package docx

import (
	"archive/zip"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func createTestDocx(t *testing.T, path string, media []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)

	ct, err := w.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`))

	for _, name := range media {
		entry, err := w.Create("word/media/" + name)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{255, 0, 0, 255})
		if err := png.Encode(entry, img); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtract_WithImages(t *testing.T) {
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "test.docx")
	createTestDocx(t, docxPath, []string{"image1.png", "screenshot.png"})

	if err := Extract(docxPath); err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	outDir := filepath.Join(dir, "test_images")
	info, err := os.Stat(outDir)
	if err != nil {
		t.Fatalf("output dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected output to be a directory")
	}

	for _, name := range []string{"image1.png", "screenshot.png"} {
		p := filepath.Join(outDir, name)
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected image %s to exist: %v", name, err)
			continue
		}
		if fi.Size() == 0 {
			t.Errorf("expected image %s to have content", name)
		}
	}
}

func TestExtract_NoImages(t *testing.T) {
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "empty.docx")
	createTestDocx(t, docxPath, nil)

	if err := Extract(docxPath); err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	outDir := filepath.Join(dir, "empty_images")
	_, err := os.Stat(outDir)
	if !os.IsNotExist(err) {
		t.Fatal("expected no output directory when docx has no images")
	}
}

func TestExtract_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.docx")
	os.WriteFile(path, []byte("this is not a zip"), 0o644)

	err := Extract(path)
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestExtract_NonexistentFile(t *testing.T) {
	err := Extract("/nonexistent/path/file.docx")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestExtract_ReadOnlyOutputDir(t *testing.T) {
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "test.docx")
	createTestDocx(t, docxPath, []string{"image1.png"})

	// Create a file where the output directory would go to block MkdirAll
	os.WriteFile(filepath.Join(dir, "test_images"), []byte("block"), 0o444)

	err := Extract(docxPath)
	if err == nil {
		t.Fatal("expected error when output dir creation fails")
	}
}

func TestExtract_WriteToReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only chmod semantics differ on Windows")
	}

	dir := t.TempDir()
	docxPath := filepath.Join(dir, "test.docx")
	createTestDocx(t, docxPath, []string{"image1.png"})

	// Create output dir as read-only so file creation inside fails
	outDir := filepath.Join(dir, "test_images")
	os.MkdirAll(outDir, 0o755)
	os.Chmod(outDir, 0o555)
	defer os.Chmod(outDir, 0o755)

	err := Extract(docxPath)
	if err == nil {
		t.Fatal("expected error when writing to read-only output dir")
	}
}

func TestExtract_RealDocx(t *testing.T) {
	src := "../test_files/file-sample_1MB.docx"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "sample.docx")
	data, _ := os.ReadFile(src)
	os.WriteFile(dest, data, 0o644)

	if err := Extract(dest); err != nil {
		t.Fatal(err)
	}
}

type errReader struct{ err error }

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }

func TestExtractFile_CopyError(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.png")
	rc := io.NopCloser(errReader{io.ErrUnexpectedEOF})
	err := extractFile(rc, dest)
	if err == nil {
		t.Fatal("expected error from io.Copy with failing reader")
	}
}

func TestExtractFile_CreateError(t *testing.T) {
	rc := io.NopCloser(errReader{io.EOF})
	err := extractFile(rc, "/nonexistent/dir/file.png")
	if err == nil {
		t.Fatal("expected error creating file in nonexistent dir")
	}
}

func TestExtract_ZipEntryOpenError(t *testing.T) {
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "test.docx")
	createTestDocx(t, docxPath, []string{"image1.png"})

	data, err := os.ReadFile(docxPath)
	if err != nil {
		t.Fatal(err)
	}

	// Find the central directory entry for word/media/image1.png
	// and corrupt its compression method to trigger f.Open() error
	target := "word/media/image1.png"
	for i := 0; i < len(data)-4; i++ {
		if data[i] == 'P' && data[i+1] == 'K' && data[i+2] == 1 && data[i+3] == 2 {
			nameLen := int(data[i+28]) | int(data[i+29])<<8
			nameStart := i + 46
			if nameStart+nameLen <= len(data) {
				name := string(data[nameStart : nameStart+nameLen])
				if name == target {
					// Compression method at offset 10 from central dir entry
					data[i+10] = 99
					data[i+11] = 0
					break
				}
			}
		}
	}

	os.WriteFile(docxPath, data, 0o644)

	err = Extract(docxPath)
	if err == nil {
		t.Fatal("expected error opening corrupted zip entry")
	}
}

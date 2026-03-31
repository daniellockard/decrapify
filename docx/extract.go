package docx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract opens a .docx file and extracts all images from word/media/
// into a <basename>_images/ folder next to the input file.
func Extract(inputPath string) error {
	r, err := zip.OpenReader(inputPath)
	if err != nil {
		return fmt.Errorf("opening docx: %w", err)
	}
	defer r.Close()

	var mediaFiles []*zip.File
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "word/media/") && !f.FileInfo().IsDir() {
			mediaFiles = append(mediaFiles, f)
		}
	}

	if len(mediaFiles) == 0 {
		return nil
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outDir := filepath.Join(filepath.Dir(inputPath), base+"_images")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	for _, f := range mediaFiles {
		name := filepath.Base(f.Name)
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening zip entry %s: %w", f.Name, err)
		}
		if err := extractFile(rc, filepath.Join(outDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(src io.ReadCloser, destPath string) error {
	defer src.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("writing %s: %w", destPath, err)
	}
	return nil
}

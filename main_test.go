package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessFileDispatch(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "unsupported extension", path: "test.xyz", wantErr: "unsupported file type: .xyz"},
		{name: "no extension", path: "README", wantErr: "unsupported file type: "},
		{name: "case insensitive docx", path: "test.DOCX"},
		{name: "case insensitive rtf", path: "test.RTF"},
		{name: "case insensitive eml", path: "test.EML"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processFile(tt.path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
			} else {
				if err == nil {
					t.Fatal("expected error (file doesn't exist), got nil")
				}
				if strings.Contains(err.Error(), "unsupported file type") {
					t.Fatalf("dispatch failed: got %q for supported extension", err.Error())
				}
			}
		})
	}
}

func TestRun_NoArgs(t *testing.T) {
	var stderr bytes.Buffer
	code := run(nil, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("expected usage message, got %q", stderr.String())
	}
}

func TestRun_NonexistentFile(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"/nonexistent/file.docx"}, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error processing") {
		t.Errorf("expected error message, got %q", stderr.String())
	}
}

func TestRun_UnsupportedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xyz")
	os.WriteFile(path, []byte("data"), 0o644)

	var stderr bytes.Buffer
	code := run([]string{path}, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unsupported file type") {
		t.Errorf("expected unsupported error, got %q", stderr.String())
	}
}

func TestRun_RealDocx(t *testing.T) {
	src := "test_files/file-sample_1MB.docx"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "sample.docx")
	data, _ := os.ReadFile(src)
	os.WriteFile(dest, data, 0o644)

	var stderr bytes.Buffer
	code := run([]string{dest}, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}
}

func TestRun_RealRtf(t *testing.T) {
	src := "test_files/file-sample_1MB.rtf"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "sample.rtf")
	data, _ := os.ReadFile(src)
	os.WriteFile(dest, data, 0o644)

	var stderr bytes.Buffer
	code := run([]string{dest}, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}

	txtPath := filepath.Join(dir, "sample.txt")
	if _, err := os.Stat(txtPath); err != nil {
		t.Fatal("expected .txt output file")
	}
}

func TestRun_RealEml(t *testing.T) {
	src := "test_files/dec.eml"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "dec.eml")
	data, _ := os.ReadFile(src)
	os.WriteFile(dest, data, 0o644)

	var stderr bytes.Buffer
	code := run([]string{dest}, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}

	outDir := filepath.Join(dir, "dec")
	if _, err := os.Stat(filepath.Join(outDir, "body.txt")); err != nil {
		t.Fatal("expected body.txt in output")
	}
}

func TestRun_MultipleFiles(t *testing.T) {
	src := "test_files/file-sample_1MB.docx"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}

	dir := t.TempDir()
	dest1 := filepath.Join(dir, "a.docx")
	dest2 := filepath.Join(dir, "b.docx")
	data, _ := os.ReadFile(src)
	os.WriteFile(dest1, data, 0o644)
	os.WriteFile(dest2, data, 0o644)

	var stderr bytes.Buffer
	code := run([]string{dest1, dest2}, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}
}

func TestRun_MixedSuccess(t *testing.T) {
	// One valid file, one invalid — should return error code but still process the valid one
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.xyz")
	os.WriteFile(badPath, []byte("data"), 0o644)

	var stderr bytes.Buffer
	code := run([]string{badPath}, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1 for unsupported file, got %d", code)
	}
}

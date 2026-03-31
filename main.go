package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/daniellockard/decrapify/docx"
	"github.com/daniellockard/decrapify/eml"
	"github.com/daniellockard/decrapify/rtf"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: decrapify <file1> [file2] ...")
		fmt.Fprintln(stderr, "Supported formats: .docx, .rtf, .eml")
		return 1
	}

	hasError := false
	for _, path := range args {
		if err := processFile(path); err != nil {
			fmt.Fprintf(stderr, "error processing %s: %v\n", path, err)
			hasError = true
		}
	}
	if hasError {
		return 1
	}
	return 0
}

func processFile(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".docx":
		return docx.Extract(path)
	case ".rtf":
		return rtf.Convert(path)
	case ".eml":
		return eml.Parse(path, processFile)
	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}
}

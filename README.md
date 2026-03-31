# decrapify

`decrapify` is a small Go CLI that extracts useful content from common email/document formats:

- `.docx`: extracts embedded images to a sibling directory
- `.rtf`: converts formatted text to plain `.txt`
- `.eml`: extracts body + attachments, then recursively processes supported attachment types

## Install

Install from GitHub:

```bash
go install github.com/daniellockard/decrapify@latest
```

This installs the `decrapify` binary into your Go bin directory:

- Windows: `%USERPROFILE%\\go\\bin`
- macOS/Linux: `$HOME/go/bin`

Make sure that directory is in your `PATH`.

## Usage

```bash
decrapify <file1> [file2] ...
```

Supported input types:

- `.docx`
- `.rtf`
- `.eml`

## What gets written

### DOCX

Input:

- `report.docx`

Output:

- `report_images/` (created next to input file)
- each file under `word/media/` is copied into that directory

### RTF

Input:

- `notes.rtf`

Output:

- `notes.txt` (created next to input file)

### EML

Input:

- `message.eml`

Output:

- `message/` directory (created next to input file)
- `body.txt` when plain text exists
- `body.html` and stripped `body.txt` when only HTML body exists
- attachments saved with original names when available
- nested `.docx`, `.rtf`, and `.eml` attachments are processed recursively

## Examples

```bash
# Process one file
decrapify test_files/dec.eml

# Process multiple files in one run
decrapify message.eml report.docx notes.rtf
```

## Build from source

```bash
git clone https://github.com/daniellockard/decrapify.git
cd decrapify
go build -o decrapify .
```

## Test

```bash
go test ./...
```

## Exit behavior

- Returns `0` when all files are processed successfully.
- Returns `1` when called without arguments or if any file fails.

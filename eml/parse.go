package eml

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

// Parse reads an .eml file and extracts its contents to a <basename>/ folder.
func Parse(inputPath string, decrapify func(string) error) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("opening eml: %w", err)
	}
	defer f.Close()

	msg, err := mail.ReadMessage(f)
	if err != nil {
		return fmt.Errorf("parsing eml: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outDir := filepath.Join(filepath.Dir(inputPath), base)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	result := &parseResult{}
	contentType := msg.Header.Get("Content-Type")
	encoding := msg.Header.Get("Content-Transfer-Encoding")

	if contentType == "" {
		contentType = "text/plain"
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		body, _ := io.ReadAll(msg.Body)
		result.plainText = string(body)
	} else {
		if err := processPart(mediaType, params, encoding, msg.Body, outDir, result, decrapify); err != nil {
			return err
		}
	}

	return result.writeOutput(outDir)
}

type parseResult struct {
	plainText string
	htmlText  string
	imageNum  int
}

func (r *parseResult) writeOutput(outDir string) error {
	return r.doWriteOutput(outDir, StripHTMLTags)
}

func (r *parseResult) doWriteOutput(outDir string, stripFunc func(io.Reader) (string, error)) error {
	if r.plainText != "" {
		if err := os.WriteFile(filepath.Join(outDir, "body.txt"), []byte(r.plainText), 0o644); err != nil {
			return fmt.Errorf("writing body.txt: %w", err)
		}
	} else if r.htmlText != "" {
		if err := os.WriteFile(filepath.Join(outDir, "body.html"), []byte(r.htmlText), 0o644); err != nil {
			return fmt.Errorf("writing body.html: %w", err)
		}
		stripped, err := stripFunc(strings.NewReader(r.htmlText))
		if err != nil {
			return fmt.Errorf("stripping HTML: %w", err)
		}
		if err := os.WriteFile(filepath.Join(outDir, "body.txt"), []byte(stripped), 0o644); err != nil {
			return fmt.Errorf("writing body.txt: %w", err)
		}
	}
	return nil
}

func processPart(mediaType string, params map[string]string, encoding string, body io.Reader, outDir string, result *parseResult, decrapify func(string) error) error {
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return fmt.Errorf("multipart without boundary")
		}
		return processMultipart(mediaType, boundary, body, outDir, result, decrapify)
	}

	decoded := decodeTransferEncoding(body, encoding)

	if mediaType == "text/plain" {
		data, err := io.ReadAll(decoded)
		if err != nil {
			return fmt.Errorf("reading text part: %w", err)
		}
		if result.plainText == "" {
			result.plainText = string(data)
		}
		return nil
	}

	if mediaType == "text/html" {
		data, err := io.ReadAll(decoded)
		if err != nil {
			return fmt.Errorf("reading html part: %w", err)
		}
		if result.htmlText == "" {
			result.htmlText = string(data)
		}
		return nil
	}

	return saveAttachment(decoded, params, mediaType, outDir, result, decrapify)
}

func processMultipart(mediaType, boundary string, body io.Reader, outDir string, result *parseResult, decrapify func(string) error) error {
	mr := multipart.NewReader(body, boundary)
	isAlternative := mediaType == "multipart/alternative"

	var htmlPart []byte
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading multipart: %w", err)
		}

		partCT := part.Header.Get("Content-Type")
		partEncoding := part.Header.Get("Content-Transfer-Encoding")
		if partCT == "" {
			partCT = "text/plain"
		}

		partMediaType, partParams, err := mime.ParseMediaType(partCT)
		if err != nil {
			continue
		}

		if fn := part.FileName(); fn != "" {
			partParams["filename"] = fn
		}

		if isAlternative {
			switch partMediaType {
			case "text/plain":
				decoded := decodeTransferEncoding(part, partEncoding)
				data, err := io.ReadAll(decoded)
				if err != nil {
					return err
				}
				result.plainText = string(data)
			case "text/html":
				decoded := decodeTransferEncoding(part, partEncoding)
				htmlPart, err = io.ReadAll(decoded)
				if err != nil {
					return err
				}
			default:
				if err := processPart(partMediaType, partParams, partEncoding, part, outDir, result, decrapify); err != nil {
					return err
				}
			}
		} else {
			if err := processPart(partMediaType, partParams, partEncoding, part, outDir, result, decrapify); err != nil {
				return err
			}
		}
	}

	if isAlternative && result.plainText == "" && len(htmlPart) > 0 {
		result.htmlText = string(htmlPart)
	}

	return nil
}

func saveAttachment(body io.Reader, params map[string]string, mediaType string, outDir string, result *parseResult, decrapify func(string) error) error {
	filename := getFilename(params, mediaType, result)
	destPath := filepath.Join(outDir, filepath.Base(filename))

	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("reading attachment: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return fmt.Errorf("writing attachment: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if decrapify != nil && (ext == ".docx" || ext == ".rtf" || ext == ".eml") {
		if err := decrapify(destPath); err != nil {
			return fmt.Errorf("decrapifying %s: %w", filename, err)
		}
	}

	return nil
}

func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	result, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return result
}

func getFilename(params map[string]string, mediaType string, result *parseResult) string {
	if name := params["filename"]; name != "" {
		return filepath.Base(decodeRFC2047(name))
	}
	if name := params["name"]; name != "" {
		return filepath.Base(decodeRFC2047(name))
	}
	result.imageNum++
	ext := ".bin"
	switch {
	case strings.HasPrefix(mediaType, "image/png"):
		ext = ".png"
	case strings.HasPrefix(mediaType, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(mediaType, "image/gif"):
		ext = ".gif"
	case strings.Contains(mediaType, "wordprocessingml"):
		ext = ".docx"
	case mediaType == "application/rtf" || mediaType == "text/rtf":
		ext = ".rtf"
	}
	return fmt.Sprintf("attachment_%d%s", result.imageNum, ext)
}

func decodeTransferEncoding(r io.Reader, encoding string) io.Reader {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, r)
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	default:
		return r
	}
}

// StripHTMLTags removes HTML tags and decodes basic entities.
func StripHTMLTags(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	var out strings.Builder
	inTag := false
	inScript := false
	inStyle := false
	var tagName strings.Builder
	readingTagName := false

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		if inTag {
			if readingTagName {
				if b == '>' || b == ' ' || b == '/' {
					name := strings.ToLower(tagName.String())
					readingTagName = false
					switch name {
					case "script":
						inScript = true
					case "/script":
						inScript = false
					case "style":
						inStyle = true
					case "/style":
						inStyle = false
					case "p", "/p", "div", "/div":
						out.WriteString("\n\n")
					case "br", "br/":
						out.WriteByte('\n')
					}
					if b == '>' {
						inTag = false
					}
				} else {
					tagName.WriteByte(b)
				}
			} else if b == '>' {
				inTag = false
			}
			continue
		}

		if b == '<' {
			inTag = true
			readingTagName = true
			tagName.Reset()
			// Include '/' for closing tags like </script>
			if next, err := br.ReadByte(); err == nil {
				if next == '/' {
					tagName.WriteByte('/')
				} else {
					br.UnreadByte()
				}
			}
			continue
		}

		if inScript || inStyle {
			continue
		}

		if b == '&' {
			entity := readEntity(br)
			out.WriteString(decodeEntity(entity))
			continue
		}

		out.WriteByte(b)
	}

	return collapseNewlines(out.String()), nil
}

func readEntity(br *bufio.Reader) string {
	var ent strings.Builder
	ent.WriteByte('&')
	for {
		b, err := br.ReadByte()
		if err != nil {
			break
		}
		ent.WriteByte(b)
		if b == ';' || ent.Len() > 10 {
			break
		}
	}
	return ent.String()
}

func decodeEntity(entity string) string {
	switch entity {
	case "&amp;":
		return "&"
	case "&lt;":
		return "<"
	case "&gt;":
		return ">"
	case "&quot;":
		return "\""
	case "&apos;":
		return "'"
	case "&nbsp;":
		return " "
	default:
		if strings.HasPrefix(entity, "&#") && strings.HasSuffix(entity, ";") {
			numStr := entity[2 : len(entity)-1]
			var val int
			if strings.HasPrefix(numStr, "x") || strings.HasPrefix(numStr, "X") {
				fmt.Sscanf(numStr[1:], "%x", &val)
			} else {
				fmt.Sscanf(numStr, "%d", &val)
			}
			if val > 0 {
				return string(rune(val))
			}
		}
		return entity
	}
}

func collapseNewlines(s string) string {
	var out strings.Builder
	newlineCount := 0
	for _, r := range s {
		if r == '\n' {
			newlineCount++
			if newlineCount <= 2 {
				out.WriteRune(r)
			}
		} else {
			newlineCount = 0
			out.WriteRune(r)
		}
	}
	return out.String()
}

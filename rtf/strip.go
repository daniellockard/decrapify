package rtf

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// StripFormatting reads RTF from r and returns plain text.
func StripFormatting(r io.Reader) (string, error) {
	br := bufio.NewReader(r)

	// Verify RTF header: {\rtf
	first, err := br.ReadByte()
	if err != nil {
		return "", fmt.Errorf("reading RTF: %w", err)
	}
	if first != '{' {
		return "", fmt.Errorf("not an RTF file: expected '{', got %q", string(first))
	}
	second, err := br.ReadByte()
	if err != nil {
		return "", fmt.Errorf("reading RTF: %w", err)
	}
	if second != '\\' {
		return "", fmt.Errorf("not an RTF file: expected '\\' after '{', got %q", string(second))
	}
	word := readControlWordName(br)
	if word != "rtf" {
		return "", fmt.Errorf("not an RTF file: expected 'rtf', got %q", word)
	}
	skipControlWordParam(br)

	var out strings.Builder
	depth := 1
	skipDepth := 0

	skipDestinations := map[string]bool{
		"fonttbl": true, "colortbl": true, "stylesheet": true,
		"pict": true, "header": true, "footer": true,
		"info": true, "fldinst": true,
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		switch b {
		case '{':
			depth++
			if skipDepth > 0 {
				continue
			}
			peeked, _ := br.Peek(2)
			if len(peeked) >= 2 && peeked[0] == '\\' && peeked[1] == '*' {
				skipDepth = depth
			}

		case '}':
			if skipDepth == depth {
				skipDepth = 0
			}
			depth--

		case '\\':
			if skipDepth > 0 {
				next, err := br.ReadByte()
				if err != nil {
					break
				}
				if next == '{' || next == '}' || next == '\\' {
					continue
				}
				if next == '\'' {
					br.ReadByte()
					br.ReadByte()
					continue
				}
				br.UnreadByte()
				readControlWordName(br)
				skipControlWordParam(br)
				continue
			}

			next, err := br.ReadByte()
			if err != nil {
				break
			}

			switch next {
			case '\\':
				out.WriteByte('\\')
			case '{':
				out.WriteByte('{')
			case '}':
				out.WriteByte('}')
			case '\'':
				h1, _ := br.ReadByte()
				h2, _ := br.ReadByte()
				val := hexVal(h1)*16 + hexVal(h2)
				out.WriteByte(val)
			case '\n', '\r':
				out.WriteByte('\n')
			default:
				br.UnreadByte()
				ctrlWord := readControlWordName(br)
				param := readControlWordParamValue(br)

				if skipDestinations[ctrlWord] {
					skipDepth = depth
					continue
				}

				switch ctrlWord {
				case "par":
					out.WriteByte('\n')
				case "line":
					out.WriteByte('\n')
				case "tab":
					out.WriteByte('\t')
				case "u":
					if param != 0 {
						var buf [4]byte
						n := utf8.EncodeRune(buf[:], rune(param))
						out.Write(buf[:n])
					}
					br.ReadByte() // skip replacement char
				case "lquote":
					out.WriteRune('\u2018')
				case "rquote":
					out.WriteRune('\u2019')
				case "ldblquote":
					out.WriteRune('\u201C')
				case "rdblquote":
					out.WriteRune('\u201D')
				case "emdash":
					out.WriteRune('\u2014')
				case "endash":
					out.WriteRune('\u2013')
				case "bullet":
					out.WriteRune('\u2022')
				}
			}

		default:
			if skipDepth > 0 {
				continue
			}
			out.WriteByte(b)
		}
	}

	return out.String(), nil
}

func readControlWordName(br *bufio.Reader) string {
	var word strings.Builder
	for {
		b, err := br.ReadByte()
		if err != nil {
			break
		}
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
			word.WriteByte(b)
		} else {
			br.UnreadByte()
			break
		}
	}
	return word.String()
}

func skipControlWordParam(br *bufio.Reader) {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		if b == '-' || (b >= '0' && b <= '9') {
			continue
		}
		if b == ' ' {
			return
		}
		br.UnreadByte()
		return
	}
}

func readControlWordParamValue(br *bufio.Reader) int {
	negative := false
	hasDigit := false
	val := 0
	for {
		b, err := br.ReadByte()
		if err != nil {
			return val
		}
		if b == '-' && !hasDigit {
			negative = true
			continue
		}
		if b >= '0' && b <= '9' {
			hasDigit = true
			val = val*10 + int(b-'0')
			continue
		}
		if b == ' ' {
			break
		}
		br.UnreadByte()
		break
	}
	if negative {
		val = -val
	}
	return val
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	default:
		return 0
	}
}

// Convert reads an RTF file and writes a plain text .txt file next to it.
func Convert(inputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("opening RTF file: %w", err)
	}
	defer f.Close()

	text, err := StripFormatting(f)
	if err != nil {
		return err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outPath := filepath.Join(filepath.Dir(inputPath), base+".txt")
	if err := os.WriteFile(outPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

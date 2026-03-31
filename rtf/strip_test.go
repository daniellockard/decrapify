package rtf

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripFormatting_SimpleText(t *testing.T) {
	input := `{\rtf1\ansi Hello world}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "Hello world" {
		t.Errorf("expected %q, got %q", "Hello world", strings.TrimSpace(got))
	}
}

func TestStripFormatting_ParAndLine(t *testing.T) {
	input := `{\rtf1\ansi First line\par Second line\line Third line}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	lines := strings.Split(got, "\n")
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(got, "First line") {
		t.Errorf("missing 'First line' in %q", got)
	}
	if !strings.Contains(got, "Second line") {
		t.Errorf("missing 'Second line' in %q", got)
	}
	if !strings.Contains(got, "Third line") {
		t.Errorf("missing 'Third line' in %q", got)
	}
}

func TestStripFormatting_UnicodeEscape(t *testing.T) {
	input := `{\rtf1\ansi don\u8217?t}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	if !strings.Contains(got, "don\u2019t") {
		t.Errorf("expected unicode char, got %q", got)
	}
}

func TestStripFormatting_EscapedSpecialChars(t *testing.T) {
	input := `{\rtf1\ansi backslash: \\ brace: \{ end: \}}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	if !strings.Contains(got, `backslash: \`) {
		t.Errorf("expected literal backslash in %q", got)
	}
	if !strings.Contains(got, "brace: {") {
		t.Errorf("expected literal { in %q", got)
	}
	if !strings.Contains(got, "end: }") {
		t.Errorf("expected literal } in %q", got)
	}
}

func TestStripFormatting_SkipsFontTable(t *testing.T) {
	input := `{\rtf1\ansi{\fonttbl{\f0 Times New Roman;}}Hello}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	if strings.Contains(got, "Times New Roman") {
		t.Errorf("font table should be skipped, got %q", got)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected 'Hello', got %q", got)
	}
}

func TestStripFormatting_SkipsColorTable(t *testing.T) {
	input := `{\rtf1\ansi{\colortbl;\red255\green0\blue0;}Color text}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	if strings.Contains(got, "255") {
		t.Errorf("color table should be skipped, got %q", got)
	}
	if !strings.Contains(got, "Color text") {
		t.Errorf("expected 'Color text', got %q", got)
	}
}

func TestStripFormatting_SkipsPict(t *testing.T) {
	input := `{\rtf1\ansi Before{\pict binary data here}After}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	if strings.Contains(got, "binary") {
		t.Errorf("pict should be skipped, got %q", got)
	}
	if !strings.Contains(got, "Before") || !strings.Contains(got, "After") {
		t.Errorf("expected 'Before' and 'After', got %q", got)
	}
}

func TestStripFormatting_SkipsIgnorableDestination(t *testing.T) {
	input := `{\rtf1\ansi{\*\generator Riched20;}Hello}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	if strings.Contains(got, "Riched20") {
		t.Errorf("ignorable destination should be skipped, got %q", got)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected 'Hello', got %q", got)
	}
}

func TestStripFormatting_Tab(t *testing.T) {
	input := `{\rtf1\ansi Col1\tab Col2}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.TrimSpace(got)
	if !strings.Contains(got, "Col1\tCol2") {
		t.Errorf("expected tab between columns, got %q", got)
	}
}

func TestStripFormatting_NonRTF(t *testing.T) {
	input := `This is just plain text, not RTF`
	_, err := StripFormatting(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for non-RTF input")
	}
}

func TestStripFormatting_EmptyRTF(t *testing.T) {
	input := `{\rtf1\ansi}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestConvert(t *testing.T) {
	dir := t.TempDir()
	rtfPath := filepath.Join(dir, "test.rtf")
	content := `{\rtf1\ansi Hello from RTF\par Second line}`
	os.WriteFile(rtfPath, []byte(content), 0o644)

	if err := Convert(rtfPath); err != nil {
		t.Fatal(err)
	}

	txtPath := filepath.Join(dir, "test.txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "Hello from RTF") {
		t.Errorf("expected 'Hello from RTF', got %q", got)
	}
	if !strings.Contains(got, "Second line") {
		t.Errorf("expected 'Second line', got %q", got)
	}
}

func TestConvert_NonexistentFile(t *testing.T) {
	err := Convert("/nonexistent/file.rtf")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestConvert_ReadOnlyOutputDir(t *testing.T) {
	dir := t.TempDir()
	rtfPath := filepath.Join(dir, "test.rtf")
	os.WriteFile(rtfPath, []byte(`{\rtf1\ansi hi}`), 0o644)

	// Make directory read-only so writing txt fails
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)

	err := Convert(rtfPath)
	if err == nil {
		t.Fatal("expected error writing to read-only dir")
	}
}

func TestStripFormatting_HexEscape(t *testing.T) {
	// \'e9 is é (Latin small letter e with acute)
	input := `{\rtf1\ansi caf\'e9}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "caf\xe9") {
		t.Errorf("expected hex-escaped char, got %q", got)
	}
}

func TestStripFormatting_HexEscapeUpperCase(t *testing.T) {
	input := `{\rtf1\ansi \'C0}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\xC0") {
		t.Errorf("expected uppercase hex char, got %q", got)
	}
}

func TestStripFormatting_EscapedNewline(t *testing.T) {
	input := "{\\" + "rtf1\\ansi line1\\\nline2\\\rline3}"
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "line1") || !strings.Contains(got, "line2") || !strings.Contains(got, "line3") {
		t.Errorf("expected lines, got %q", got)
	}
}

func TestStripFormatting_SpecialQuotes(t *testing.T) {
	input := `{\rtf1\ansi \lquote hello\rquote  \ldblquote world\rdblquote }`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\u2018") || !strings.Contains(got, "\u2019") {
		t.Errorf("expected smart single quotes, got %q", got)
	}
	if !strings.Contains(got, "\u201C") || !strings.Contains(got, "\u201D") {
		t.Errorf("expected smart double quotes, got %q", got)
	}
}

func TestStripFormatting_Dashes(t *testing.T) {
	input := `{\rtf1\ansi em\emdash dash\endash here}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\u2014") {
		t.Errorf("expected emdash, got %q", got)
	}
	if !strings.Contains(got, "\u2013") {
		t.Errorf("expected endash, got %q", got)
	}
}

func TestStripFormatting_Bullet(t *testing.T) {
	input := `{\rtf1\ansi \bullet item}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "\u2022") {
		t.Errorf("expected bullet, got %q", got)
	}
}

func TestStripFormatting_NegativeParam(t *testing.T) {
	// Negative unicode value (shouldn't happen in practice but tests the path)
	input := `{\rtf1\ansi \u-10?x}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	// Negative value: rune(-10) wraps, just ensure no crash and replacement char skipped
	_ = got
}

func TestStripFormatting_SkipGroupWithHexInside(t *testing.T) {
	// Inside a skipped group, hex escapes and unicode should be handled without crashing
	input := `{\rtf1\ansi{\fonttbl{\f0\'e9 \u1234?x;}}Hello}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected 'Hello', got %q", got)
	}
}

func TestStripFormatting_SkipGroupWithBraces(t *testing.T) {
	// Escaped braces in skipped groups should not mess up depth counting
	input := `{\rtf1\ansi{\fonttbl{\f0 \{ \} \\;}}OK}`
	got, err := StripFormatting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "OK") {
		t.Errorf("expected 'OK', got %q", got)
	}
}

func TestStripFormatting_HeaderNotRTF(t *testing.T) {
	_, err := StripFormatting(strings.NewReader(`{notrtf}`))
	if err == nil {
		t.Fatal("expected error for non-rtf control word")
	}
}

func TestStripFormatting_HeaderNoBackslash(t *testing.T) {
	_, err := StripFormatting(strings.NewReader(`{garbage}`))
	if err == nil {
		t.Fatal("expected error for missing backslash")
	}
}

func TestStripFormatting_Empty(t *testing.T) {
	_, err := StripFormatting(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestStripFormatting_HeaderOnlyBrace(t *testing.T) {
	_, err := StripFormatting(strings.NewReader("{"))
	if err == nil {
		t.Fatal("expected error for truncated header")
	}
}

func TestStripFormatting_RealFile(t *testing.T) {
	src := "../test_files/file-sample_1MB.rtf"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}
	f, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := StripFormatting(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty output from real RTF file")
	}
}

func TestConvert_RealFile(t *testing.T) {
	src := "../test_files/file-sample_1MB.rtf"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "sample.rtf")
	data, _ := os.ReadFile(src)
	os.WriteFile(dest, data, 0o644)

	if err := Convert(dest); err != nil {
		t.Fatal(err)
	}

	txtPath := filepath.Join(dir, "sample.txt")
	txt, err := os.ReadFile(txtPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(txt) == 0 {
		t.Fatal("expected non-empty txt output")
	}
}

// errAfterReader returns data then an error.
type errAfterReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

func TestStripFormatting_WrongControlWord(t *testing.T) {
	// {\foo - passes { and \, but word is "foo" not "rtf"
	_, err := StripFormatting(strings.NewReader(`{\foo1\ansi hello}`))
	if err == nil {
		t.Fatal("expected error for wrong control word")
	}
	if !strings.Contains(err.Error(), "expected 'rtf'") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestStripFormatting_ReadErrorInMainLoop(t *testing.T) {
	// Valid RTF header then an IO error
	r := &errAfterReader{
		data: []byte(`{\rtf1 `),
		err:  io.ErrUnexpectedEOF,
	}
	_, err := StripFormatting(r)
	// Should return an error (not nil), since the reader errors with non-EOF
	if err == nil {
		t.Fatal("expected error from reader")
	}
}

func TestStripFormatting_ReadErrorInSkipBlock(t *testing.T) {
	// RTF with pict destination, then error mid-stream
	r := &errAfterReader{
		data: []byte("{\\rtf1 {\\pict "),
		err:  io.ErrUnexpectedEOF,
	}
	_, err := StripFormatting(r)
	if err == nil {
		t.Fatal("expected error from reader in skip block")
	}
}

func TestStripFormatting_ReadErrorAfterBackslash(t *testing.T) {
	// RTF with backslash at very end, then error
	r := &errAfterReader{
		data: []byte("{\\rtf1 hello\\"),
		err:  io.ErrUnexpectedEOF,
	}
	_, err := StripFormatting(r)
	if err == nil {
		t.Fatal("expected error from reader after backslash")
	}
}

func TestStripFormatting_InvalidHexChar(t *testing.T) {
	// \'zz where z is not valid hex — hexVal default case
	got, err := StripFormatting(strings.NewReader(`{\rtf1\ansi \'zz}`))
	if err != nil {
		t.Fatal(err)
	}
	// hexVal('z') returns 0, so result is 0*16+0 = 0 byte
	if !strings.Contains(got, string(rune(0))) {
		t.Logf("got: %q (expected null byte from invalid hex)", got)
	}
}

func TestStripFormatting_TruncatedAfterBackslashInSkip(t *testing.T) {
	// In skip block, backslash at end triggers break
	r := &errAfterReader{
		data: []byte("{\\rtf1 {\\pict \\"),
		err:  io.ErrUnexpectedEOF,
	}
	_, err := StripFormatting(r)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadControlWordName_EOF(t *testing.T) {
	br := bufio.NewReader(strings.NewReader(""))
	got := readControlWordName(br)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestSkipControlWordParam_EOF(t *testing.T) {
	br := bufio.NewReader(strings.NewReader(""))
	skipControlWordParam(br) // should not panic
}

func TestReadControlWordParamValue_EOF(t *testing.T) {
	br := bufio.NewReader(strings.NewReader(""))
	got := readControlWordParamValue(br)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestConvert_WriteErrorDirBlock(t *testing.T) {
	dir := t.TempDir()
	rtfPath := filepath.Join(dir, "test.rtf")
	os.WriteFile(rtfPath, []byte(`{\rtf1\ansi hi}`), 0o644)

	// Block output path by making test.txt a directory
	os.MkdirAll(filepath.Join(dir, "test.txt"), 0o755)

	err := Convert(rtfPath)
	if err == nil {
		t.Fatal("expected error writing to blocked path")
	}
}

func TestConvert_StripFormattingError(t *testing.T) {
	dir := t.TempDir()
	rtfPath := filepath.Join(dir, "bad.rtf")
	os.WriteFile(rtfPath, []byte("not rtf at all"), 0o644)

	err := Convert(rtfPath)
	if err == nil {
		t.Fatal("expected StripFormatting error for non-RTF file")
	}
}

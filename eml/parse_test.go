package eml

import (
	"archive/zip"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_PlainText(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "message.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Hello, this is plain text.\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "message", "body.txt"))
	if err != nil {
		t.Fatalf("body.txt not created: %v", err)
	}
	if !strings.Contains(string(data), "Hello, this is plain text.") {
		t.Errorf("unexpected body.txt content: %q", string(data))
	}
}

func TestParse_HTMLOnly(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "htmlonly.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: HTML Test\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body><p>Hello</p><p>World</p></body></html>\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "htmlonly")

	htmlData, err := os.ReadFile(filepath.Join(outDir, "body.html"))
	if err != nil {
		t.Fatalf("body.html not created: %v", err)
	}
	if !strings.Contains(string(htmlData), "<p>Hello</p>") {
		t.Errorf("unexpected body.html content: %q", string(htmlData))
	}

	txtData, err := os.ReadFile(filepath.Join(outDir, "body.txt"))
	if err != nil {
		t.Fatalf("body.txt not created: %v", err)
	}
	txt := string(txtData)
	if !strings.Contains(txt, "Hello") || !strings.Contains(txt, "World") {
		t.Errorf("body.txt should contain stripped text, got %q", txt)
	}
	if strings.Contains(txt, "<p>") {
		t.Errorf("body.txt should not contain HTML tags, got %q", txt)
	}
}

func TestParse_MultipartAlternative(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "multipart.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Multipart\r\n" +
		"Content-Type: multipart/alternative; boundary=boundary123\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Plain text version\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body>HTML version</body></html>\r\n" +
		"--boundary123--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "multipart")
	data, err := os.ReadFile(filepath.Join(outDir, "body.txt"))
	if err != nil {
		t.Fatalf("body.txt not created: %v", err)
	}
	if !strings.Contains(string(data), "Plain text version") {
		t.Errorf("expected plain text version, got %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(outDir, "body.html")); !os.IsNotExist(err) {
		t.Error("body.html should not exist when plain text is available")
	}
}

func TestParse_InlineImage(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "inline.eml")

	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Inline Image\r\n" +
		"Content-Type: multipart/mixed; boundary=boundary456\r\n" +
		"\r\n" +
		"--boundary456\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"See attached image\r\n" +
		"--boundary456\r\n" +
		"Content-Type: image/png; name=\"screenshot.png\"\r\n" +
		"Content-Disposition: inline; filename=\"screenshot.png\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--boundary456--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(dir, "inline", "screenshot.png")
	fi, err := os.Stat(imgPath)
	if err != nil {
		t.Fatalf("inline image not extracted: %v", err)
	}
	if fi.Size() == 0 {
		t.Error("extracted image is empty")
	}
}

func TestParse_AttachedImage(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "attached.eml")

	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Attached Image\r\n" +
		"Content-Type: multipart/mixed; boundary=b789\r\n" +
		"\r\n" +
		"--b789\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Image attached\r\n" +
		"--b789\r\n" +
		"Content-Type: image/jpeg; name=\"photo.jpg\"\r\n" +
		"Content-Disposition: attachment; filename=\"photo.jpg\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--b789--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "attached", "photo.jpg")); err != nil {
		t.Fatalf("attached image not extracted: %v", err)
	}
}

func TestParse_Base64Body(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "b64body.eml")

	encoded := base64.StdEncoding.EncodeToString([]byte("Decoded text body"))
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Base64 Body\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		encoded + "\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "b64body", "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Decoded text body") {
		t.Errorf("expected decoded text, got %q", string(data))
	}
}

func TestParse_QuotedPrintableBody(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "qp.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: QP Body\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"Hello =3D World\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "qp", "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Hello = World") {
		t.Errorf("expected decoded QP text, got %q", string(data))
	}
}

func TestParse_DocxAttachment(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "withdocx.eml")

	docxData := createTestDocxBytes(t, []string{"image1.png"})
	b64 := base64.StdEncoding.EncodeToString(docxData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: DOCX Attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=docxbnd\r\n" +
		"\r\n" +
		"--docxbnd\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"See attached docx\r\n" +
		"--docxbnd\r\n" +
		"Content-Type: application/vnd.openxmlformats-officedocument.wordprocessingml.document; name=\"report.docx\"\r\n" +
		"Content-Disposition: attachment; filename=\"report.docx\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--docxbnd--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	decrapifyCalled := false
	mockDecrapify := func(path string) error {
		decrapifyCalled = true
		if filepath.Base(path) != "report.docx" {
			t.Errorf("expected decrapify called with report.docx, got %s", path)
		}
		return nil
	}

	if err := Parse(emlPath, mockDecrapify); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "withdocx", "report.docx")); err != nil {
		t.Fatalf("docx attachment not extracted: %v", err)
	}
	if !decrapifyCalled {
		t.Error("decrapify callback was not called for .docx attachment")
	}
}

func TestParse_RtfAttachment(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "withrtf.eml")

	rtfContent := `{\rtf1\ansi Hello from RTF}`
	b64 := base64.StdEncoding.EncodeToString([]byte(rtfContent))

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: RTF Attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=rtfbnd\r\n" +
		"\r\n" +
		"--rtfbnd\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"See attached rtf\r\n" +
		"--rtfbnd\r\n" +
		"Content-Type: application/rtf; name=\"notes.rtf\"\r\n" +
		"Content-Disposition: attachment; filename=\"notes.rtf\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--rtfbnd--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	decrapifyCalled := false
	mockDecrapify := func(path string) error {
		decrapifyCalled = true
		if filepath.Base(path) != "notes.rtf" {
			t.Errorf("expected decrapify called with notes.rtf, got %s", path)
		}
		return nil
	}

	if err := Parse(emlPath, mockDecrapify); err != nil {
		t.Fatal(err)
	}

	if !decrapifyCalled {
		t.Error("decrapify callback was not called for .rtf attachment")
	}
}

func TestParse_NestedMultipart(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "nested.eml")

	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Nested\r\n" +
		"Content-Type: multipart/mixed; boundary=outer\r\n" +
		"\r\n" +
		"--outer\r\n" +
		"Content-Type: multipart/alternative; boundary=inner\r\n" +
		"\r\n" +
		"--inner\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Nested plain text\r\n" +
		"--inner\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<p>Nested HTML</p>\r\n" +
		"--inner--\r\n" +
		"--outer\r\n" +
		"Content-Type: image/png; name=\"nested.png\"\r\n" +
		"Content-Disposition: attachment; filename=\"nested.png\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--outer--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "nested")
	data, err := os.ReadFile(filepath.Join(outDir, "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Nested plain text") {
		t.Errorf("expected nested plain text, got %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(outDir, "nested.png")); err != nil {
		t.Fatalf("nested image not extracted: %v", err)
	}
}

func TestParse_NonexistentFile(t *testing.T) {
	err := Parse("/nonexistent/file.eml", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestStripHTMLTags_Basic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple paragraph",
			input: "<p>Hello</p><p>World</p>",
			want:  "Hello\n\nWorld",
		},
		{
			name:  "br tag",
			input: "Line1<br>Line2<br/>Line3",
			want:  "Line1\nLine2\nLine3",
		},
		{
			name:  "entities",
			input: "&amp; &lt; &gt; &quot;",
			want:  `& < > "`,
		},
		{
			name:  "numeric entity",
			input: "&#65;&#66;",
			want:  "AB",
		},
		{
			name:  "nested tags",
			input: "<div><b>Bold</b> text</div>",
			want:  "Bold text",
		},
		{
			name:  "strip script and style",
			input: "<style>body{}</style><script>alert(1)</script>Visible",
			want:  "Visible",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StripHTMLTags(strings.NewReader(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			got = strings.TrimSpace(got)
			want := strings.TrimSpace(tt.want)
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// --- helpers ---

func createPNGBytes(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func createTestDocxBytes(t *testing.T, media []string) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	ct, _ := w.Create("[Content_Types].xml")
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`))

	for _, name := range media {
		entry, _ := w.Create("word/media/" + name)
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{0, 255, 0, 255})
		png.Encode(entry, img)
	}
	w.Close()
	f.Sync()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStripHTMLTags_Apos(t *testing.T) {
	got, err := StripHTMLTags(strings.NewReader("it&apos;s"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "it's" {
		t.Errorf("expected it's, got %q", got)
	}
}

func TestStripHTMLTags_Nbsp(t *testing.T) {
	got, err := StripHTMLTags(strings.NewReader("a&nbsp;b"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "a b" {
		t.Errorf("expected 'a b', got %q", got)
	}
}

func TestStripHTMLTags_HexEntity(t *testing.T) {
	got, err := StripHTMLTags(strings.NewReader("&#x41;"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "A" {
		t.Errorf("expected 'A', got %q", got)
	}
}

func TestStripHTMLTags_UnknownEntity(t *testing.T) {
	got, err := StripHTMLTags(strings.NewReader("&bogus;"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "&bogus;" {
		t.Errorf("expected raw entity preserved, got %q", got)
	}
}

func TestStripHTMLTags_BadNumericEntity(t *testing.T) {
	got, err := StripHTMLTags(strings.NewReader("&#0;"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "&#0;" {
		t.Errorf("expected raw entity, got %q", got)
	}
}

func TestParse_NoContentType(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "noct.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: No CT\r\n" +
		"\r\n" +
		"Just text with no content-type header\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "noct", "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Just text with no content-type header") {
		t.Errorf("unexpected body: %q", string(data))
	}
}

func TestParse_MultipartWithoutBoundary(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "nobnd.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: No boundary\r\n" +
		"Content-Type: multipart/mixed\r\n" +
		"\r\n" +
		"body\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	err := Parse(emlPath, nil)
	if err == nil {
		t.Fatal("expected error for multipart without boundary")
	}
}

func TestParse_HTMLOnlyAlternative(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "htmlalt.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: HTML Alt\r\n" +
		"Content-Type: multipart/alternative; boundary=alt1\r\n" +
		"\r\n" +
		"--alt1\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>Only HTML here</p>\r\n" +
		"--alt1--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "htmlalt")
	if _, err := os.Stat(filepath.Join(outDir, "body.html")); err != nil {
		t.Fatal("expected body.html when only HTML in alternative")
	}
	data, err := os.ReadFile(filepath.Join(outDir, "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Only HTML here") {
		t.Errorf("bad stripped text: %q", string(data))
	}
}

func TestParse_UnnamedImagePNG(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "unnamed.eml")
	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Unnamed\r\n" +
		"Content-Type: multipart/mixed; boundary=unn\r\n" +
		"\r\n" +
		"--unn\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--unn\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--unn--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "unnamed", "attachment_1.png")); err != nil {
		t.Fatal("expected generated filename for unnamed PNG attachment")
	}
}

func TestParse_UnnamedImageJPEG(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "unnjpg.eml")
	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: JPEG\r\n" +
		"Content-Type: multipart/mixed; boundary=jj\r\n" +
		"\r\n" +
		"--jj\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--jj\r\n" +
		"Content-Type: image/jpeg\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--jj--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "unnjpg", "attachment_1.jpg")); err != nil {
		t.Fatal("expected attachment_1.jpg for unnamed JPEG")
	}
}

func TestParse_UnnamedImageGIF(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "unngif.eml")
	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: GIF\r\n" +
		"Content-Type: multipart/mixed; boundary=gg\r\n" +
		"\r\n" +
		"--gg\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--gg\r\n" +
		"Content-Type: image/gif\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--gg--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "unngif", "attachment_1.gif")); err != nil {
		t.Fatal("expected attachment_1.gif for unnamed GIF")
	}
}

func TestParse_UnnamedDocxByContentType(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "unndocx.eml")
	docxData := createTestDocxBytes(t, nil)
	b64 := base64.StdEncoding.EncodeToString(docxData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: DOCX\r\n" +
		"Content-Type: multipart/mixed; boundary=dd\r\n" +
		"\r\n" +
		"--dd\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--dd\r\n" +
		"Content-Type: application/vnd.openxmlformats-officedocument.wordprocessingml.document\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--dd--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	called := false
	if err := Parse(emlPath, func(path string) error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "unndocx", "attachment_1.docx")); err != nil {
		t.Fatal("expected attachment_1.docx for unnamed docx")
	}
	if !called {
		t.Error("decrapify not called for unnamed docx")
	}
}

func TestParse_UnnamedRtfByContentType(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "unnrtf.eml")
	rtfData := []byte(`{\rtf1\ansi hello}`)
	b64 := base64.StdEncoding.EncodeToString(rtfData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: RTF\r\n" +
		"Content-Type: multipart/mixed; boundary=rr\r\n" +
		"\r\n" +
		"--rr\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--rr\r\n" +
		"Content-Type: application/rtf\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--rr--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	called := false
	if err := Parse(emlPath, func(path string) error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "unnrtf", "attachment_1.rtf")); err != nil {
		t.Fatal("expected attachment_1.rtf for unnamed rtf")
	}
	if !called {
		t.Error("decrapify not called for unnamed rtf")
	}
}

func TestParse_UnnamedBinary(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "unnbin.eml")
	b64 := base64.StdEncoding.EncodeToString([]byte("binary data"))

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Binary\r\n" +
		"Content-Type: multipart/mixed; boundary=bb\r\n" +
		"\r\n" +
		"--bb\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--bb\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--bb--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "unnbin", "attachment_1.bin")); err != nil {
		t.Fatal("expected attachment_1.bin for unknown type")
	}
}

func TestParse_DecrapifyError(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "decerr.eml")
	docxData := createTestDocxBytes(t, nil)
	b64 := base64.StdEncoding.EncodeToString(docxData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Err\r\n" +
		"Content-Type: multipart/mixed; boundary=ee\r\n" +
		"\r\n" +
		"--ee\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--ee\r\n" +
		"Content-Type: application/vnd.openxmlformats-officedocument.wordprocessingml.document; name=\"fail.docx\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--ee--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	err := Parse(emlPath, func(path string) error {
		return fmt.Errorf("mock error")
	})
	if err == nil {
		t.Fatal("expected error from decrapify callback")
	}
	if !strings.Contains(err.Error(), "mock error") {
		t.Errorf("expected mock error, got %q", err.Error())
	}
}

func TestParse_EmptyBody(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "empty.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Empty\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}
}

func TestParse_BadContentType(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "badct.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Bad CT\r\n" +
		"Content-Type: ;;;invalid;;;\r\n" +
		"\r\n" +
		"fallback body text\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "badct", "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fallback body text") {
		t.Errorf("expected fallback body, got %q", string(data))
	}
}

func TestParse_RealEml(t *testing.T) {
	src := "../test_files/dec.eml"
	if _, err := os.Stat(src); err != nil {
		t.Skip("test fixture not available")
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "dec.eml")
	data, _ := os.ReadFile(src)
	os.WriteFile(dest, data, 0o644)

	if err := Parse(dest, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "dec")
	bodyData, err := os.ReadFile(filepath.Join(outDir, "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bodyData) == 0 {
		t.Fatal("expected non-empty body.txt from real eml")
	}
}

func TestParse_DuplicatePlainText(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "dup.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Dup\r\n" +
		"Content-Type: multipart/mixed; boundary=dp\r\n" +
		"\r\n" +
		"--dp\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"First text\r\n" +
		"--dp\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Second text\r\n" +
		"--dp--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "dup", "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "First text") {
		t.Errorf("expected first text to win, got %q", string(data))
	}
}

func TestParse_DuplicateHTML(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "duphtml.eml")
	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: DupHTML\r\n" +
		"Content-Type: multipart/mixed; boundary=dh\r\n" +
		"\r\n" +
		"--dh\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<p>First HTML</p>\r\n" +
		"--dh\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<p>Second HTML</p>\r\n" +
		"--dh--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "duphtml", "body.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "First HTML") {
		t.Errorf("expected first HTML to win, got %q", string(data))
	}
}

func TestParse_TextRtfContent(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "txtrtf.eml")
	rtfData := []byte(`{\rtf1\ansi hello}`)
	b64 := base64.StdEncoding.EncodeToString(rtfData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: TextRTF\r\n" +
		"Content-Type: multipart/mixed; boundary=tr\r\n" +
		"\r\n" +
		"--tr\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--tr\r\n" +
		"Content-Type: text/rtf\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--tr--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	called := false
	if err := Parse(emlPath, func(path string) error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "txtrtf", "attachment_1.rtf")); err != nil {
		t.Fatal("expected .rtf for text/rtf content type")
	}
	if !called {
		t.Error("decrapify not called for text/rtf")
	}
}

// errReader for error injection in tests
type errReader struct{ err error }

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }

// errAfterNReader returns data then an error
type errAfterNReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errAfterNReader) Read(p []byte) (int, error) {
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

func TestParse_InvalidHeaders(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "bad.eml")
	os.WriteFile(emlPath, []byte("\x00\x00\x00"), 0o644)
	err := Parse(emlPath, nil)
	if err == nil {
		t.Fatal("expected error for invalid email headers")
	}
}

func TestParse_MkdirAllError(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "test.eml")
	os.WriteFile(emlPath, []byte("From: a@b\r\n\r\nbody"), 0o644)
	// Block output dir with regular file
	os.WriteFile(filepath.Join(dir, "test"), []byte("block"), 0o644)
	err := Parse(emlPath, nil)
	if err == nil {
		t.Fatal("expected MkdirAll error")
	}
}

func TestWriteOutput_PlainTextError(t *testing.T) {
	result := &parseResult{plainText: "hello"}
	err := result.writeOutput("/nonexistent/path/out")
	if err == nil {
		t.Fatal("expected error writing body.txt")
	}
}

func TestWriteOutput_HTMLWriteError(t *testing.T) {
	dir := t.TempDir()
	result := &parseResult{htmlText: "<p>hi</p>"}
	// Block body.html by creating it as a directory
	os.MkdirAll(filepath.Join(dir, "body.html"), 0o755)
	err := result.writeOutput(dir)
	if err == nil {
		t.Fatal("expected error writing body.html")
	}
}

func TestWriteOutput_HTMLStripError(t *testing.T) {
	dir := t.TempDir()
	result := &parseResult{htmlText: "<p>hi</p>"}
	err := result.doWriteOutput(dir, func(r io.Reader) (string, error) {
		return "", fmt.Errorf("mock strip error")
	})
	if err == nil {
		t.Fatal("expected error from strip function")
	}
}

func TestWriteOutput_HTMLBodyTxtError(t *testing.T) {
	dir := t.TempDir()
	result := &parseResult{htmlText: "<p>hi</p>"}
	// Block body.txt by creating it as a directory
	os.MkdirAll(filepath.Join(dir, "body.txt"), 0o755)
	err := result.writeOutput(dir)
	if err == nil {
		t.Fatal("expected error writing body.txt after HTML strip")
	}
}

func TestProcessPart_ReadTextError(t *testing.T) {
	result := &parseResult{}
	err := processPart("text/plain", nil, "", errReader{io.ErrUnexpectedEOF}, "/tmp", result, nil)
	if err == nil {
		t.Fatal("expected error reading text part")
	}
}

func TestProcessPart_ReadHTMLError(t *testing.T) {
	result := &parseResult{}
	err := processPart("text/html", nil, "", errReader{io.ErrUnexpectedEOF}, "/tmp", result, nil)
	if err == nil {
		t.Fatal("expected error reading html part")
	}
}

func TestProcessMultipart_NextPartError(t *testing.T) {
	result := &parseResult{}
	err := processMultipart("multipart/mixed", "bnd", errReader{io.ErrUnexpectedEOF}, t.TempDir(), result, nil)
	if err == nil {
		t.Fatal("expected error from NextPart")
	}
}

func TestProcessMultipart_BadPartContentType(t *testing.T) {
	body := "--bnd\r\n" +
		"Content-Type: ;;;invalid\r\n" +
		"\r\n" +
		"data\r\n" +
		"--bnd--\r\n"
	result := &parseResult{}
	err := processMultipart("multipart/mixed", "bnd", strings.NewReader(body), t.TempDir(), result, nil)
	if err != nil {
		t.Fatalf("bad CT part should be skipped, got error: %v", err)
	}
}

func TestProcessMultipart_AltPlainReadError(t *testing.T) {
	body := "--bnd\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"!!!not-valid-base64!!!\r\n" +
		"--bnd--\r\n"
	result := &parseResult{}
	err := processMultipart("multipart/alternative", "bnd", strings.NewReader(body), t.TempDir(), result, nil)
	if err == nil {
		t.Fatal("expected error from base64 decode in alternative text/plain")
	}
}

func TestProcessMultipart_AltHTMLReadError(t *testing.T) {
	body := "--bnd\r\n" +
		"Content-Type: text/html\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"!!!not-valid-base64!!!\r\n" +
		"--bnd--\r\n"
	result := &parseResult{}
	err := processMultipart("multipart/alternative", "bnd", strings.NewReader(body), t.TempDir(), result, nil)
	if err == nil {
		t.Fatal("expected error from base64 decode in alternative text/html")
	}
}

func TestProcessMultipart_AltDefaultError(t *testing.T) {
	body := "--bnd\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"!!!not-valid-base64!!!\r\n" +
		"--bnd--\r\n"
	result := &parseResult{}
	err := processMultipart("multipart/alternative", "bnd", strings.NewReader(body), "/nonexistent/dir", result, nil)
	if err == nil {
		t.Fatal("expected error from processPart in alternative default")
	}
}

func TestSaveAttachment_ReadError(t *testing.T) {
	result := &parseResult{}
	err := saveAttachment(errReader{io.ErrUnexpectedEOF}, map[string]string{"name": "test.bin"}, "application/octet-stream", t.TempDir(), result, nil)
	if err == nil {
		t.Fatal("expected error reading attachment")
	}
}

func TestSaveAttachment_WriteError(t *testing.T) {
	result := &parseResult{}
	err := saveAttachment(strings.NewReader("data"), map[string]string{"name": "test.bin"}, "application/octet-stream", "/nonexistent/dir", result, nil)
	if err == nil {
		t.Fatal("expected error writing attachment")
	}
}

func TestStripHTMLTags_ReadError(t *testing.T) {
	_, err := StripHTMLTags(errReader{io.ErrUnexpectedEOF})
	if err == nil {
		t.Fatal("expected error from reader")
	}
}

func TestStripHTMLTags_EntityReadError(t *testing.T) {
	// Reader that returns "&" then errors — hits readEntity error branch
	r := &errAfterNReader{data: []byte("&"), err: io.ErrUnexpectedEOF}
	_, err := StripHTMLTags(r)
	if err == nil {
		t.Fatal("expected error after entity read")
	}
}

func TestProcessMultipart_PartWithNoContentType(t *testing.T) {
	// Part with no Content-Type should default to text/plain
	body := "--bnd\r\n" +
		"\r\n" +
		"Just plain text with no CT header\r\n" +
		"--bnd--\r\n"
	result := &parseResult{}
	err := processMultipart("multipart/mixed", "bnd", strings.NewReader(body), t.TempDir(), result, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.plainText, "Just plain text with no CT header") {
		t.Errorf("expected plain text body, got %q", result.plainText)
	}
}

func TestParse_RFC2047EncodedFilename(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "enc.eml")
	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Encoded Filename\r\n" +
		"Content-Type: multipart/mixed; boundary=enc1\r\n" +
		"\r\n" +
		"--enc1\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--enc1\r\n" +
		"Content-Type: image/gif; name=\"nyan cat =?UTF-8?Q?=E2=9C=94=2Egif?=\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--enc1--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "enc")
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "✔") && strings.HasSuffix(e.Name(), ".gif") {
			found = true
		}
		if strings.Contains(e.Name(), "=?") {
			t.Errorf("filename still has encoded-word syntax: %s", e.Name())
		}
	}
	if !found {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected decoded filename with ✔.gif, got: %v", names)
	}
}

func TestParse_ContentDispositionFilename(t *testing.T) {
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "disp.eml")
	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Disposition\r\n" +
		"Content-Type: multipart/mixed; boundary=disp1\r\n" +
		"\r\n" +
		"--disp1\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--disp1\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-Disposition: attachment; filename=\"photo.png\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--disp1--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "disp", "photo.png")); err != nil {
		t.Fatal("expected photo.png from Content-Disposition filename")
	}
}

func TestParse_RFC2231EncodedFilename(t *testing.T) {
	// Reproduces the exact encoding from dec.eml:
	// Content-Type has RFC 2047 encoded name (common but non-standard)
	// Content-Disposition has RFC 2231 encoded filename*0*
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "rfc2231.eml")
	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: RFC2231 test\r\n" +
		"Content-Type: multipart/mixed; boundary=rfc\r\n" +
		"\r\n" +
		"--rfc\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--rfc\r\n" +
		"Content-Type: image/gif; name=\"nyan cat =?UTF-8?Q?=E2=9C=94=2Egif?=\"\r\n" +
		"Content-Disposition: attachment;\r\n" +
		" filename*0*=utf-8''nyan%20cat%20%E2%9C%94.gif\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--rfc--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "rfc2231")
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "=?") {
			t.Fatalf("filename still has encoded-word syntax: %s", e.Name())
		}
	}
	expected := "nyan cat ✔.gif"
	if _, err := os.Stat(filepath.Join(outDir, expected)); err != nil {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("expected %q, got files: %v", expected, names)
	}
}

func TestParse_RFC2047OnlyNoDisposition(t *testing.T) {
	// Content-Type name with RFC 2047 encoding, NO Content-Disposition header.
	// Tests the decodeRFC2047 fallback in getFilename.
	dir := t.TempDir()
	emlPath := filepath.Join(dir, "rfc2047only.eml")
	pngData := createPNGBytes(t)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	content := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: RFC2047 only\r\n" +
		"Content-Type: multipart/mixed; boundary=r47\r\n" +
		"\r\n" +
		"--r47\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"body\r\n" +
		"--r47\r\n" +
		"Content-Type: image/gif; name=\"nyan cat =?UTF-8?Q?=E2=9C=94=2Egif?=\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		b64 + "\r\n" +
		"--r47--\r\n"
	os.WriteFile(emlPath, []byte(content), 0o644)

	if err := Parse(emlPath, nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "rfc2047only")
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "=?") {
			t.Fatalf("filename still has encoded-word syntax: %s", e.Name())
		}
	}
	expected := "nyan cat ✔.gif"
	if _, err := os.Stat(filepath.Join(outDir, expected)); err != nil {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("expected %q, got files: %v", expected, names)
	}
}

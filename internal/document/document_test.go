package document

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func makeDOCX(t *testing.T, xmlBody string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(xmlBody)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseDOCX(t *testing.T) {
	data := makeDOCX(t, `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Закон Ома:</w:t></w:r><w:r><w:t xml:space="preserve"> I = U/R</w:t></w:r></w:p>
<w:p><w:r><w:t>Короткое   замыкание</w:t></w:r></w:p>
</w:body></w:document>`)
	kind, err := Kind("конспект.docx", data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(kind, data)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := doc.Text(), "Закон Ома: I = U/R\nКороткое замыкание"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestParseText(t *testing.T) {
	data := []byte("Строка 1\r\n\r\n\r\n   Строка   2  \n")
	kind, err := Kind("notes.txt", data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(kind, data)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Text(); got != "Строка 1\n\nСтрока 2" {
		t.Errorf("text = %q", got)
	}
}

func TestKindRejectsUnknown(t *testing.T) {
	if _, err := Kind("virus.exe", []byte("MZ\x90\x00binary")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("err = %v", err)
	}
	// Расширение .docx у не-zip файла не помогает.
	if _, err := Kind("fake.docx", []byte("hello")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("err = %v", err)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse("text/plain", []byte("  \n ")); !errors.Is(err, ErrNoText) {
		t.Errorf("err = %v", err)
	}
}

func TestParseImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	kind, err := Kind("photo.png", png)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(kind, png)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Images) != 1 || doc.PageCount() != 1 {
		t.Errorf("doc = %+v", doc)
	}
}

// testdata/notes.pdf — конспект из testdata/notes/electricity.txt, напечатанный Chrome в PDF.
func TestParsePDF(t *testing.T) {
	data, err := os.ReadFile("testdata/notes.pdf")
	if err != nil {
		t.Fatal(err)
	}
	kind, err := Kind("notes.pdf", data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(kind, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Pages) < 2 {
		t.Fatalf("pages = %d, want at least 2", len(doc.Pages))
	}
	text := doc.Text()
	for _, want := range []string{"Закон Ома для участка цепи", "Короткое замыкание", "предохранител"} {
		if !strings.Contains(text, want) {
			t.Errorf("text does not contain %q", want)
		}
	}
	if doc.Pages[0].Number != 1 || doc.Pages[1].Number != 2 {
		t.Errorf("page numbers = %d, %d", doc.Pages[0].Number, doc.Pages[1].Number)
	}
}

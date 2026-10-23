// Package document извлекает текст из загруженного конспекта.
package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// MaxSize — максимальный размер загружаемого файла.
const MaxSize = 25 << 20

var (
	ErrUnsupported = errors.New("unsupported file type")
	// ErrNoText — в документе нет текстового слоя (например, PDF из сканов).
	ErrNoText = errors.New("document has no text")
)

// Page — текст одной страницы. Number = 0, если у формата нет страниц.
type Page struct {
	Number int
	Text   string
}

// Image — изображение, которое нужно прочитать модели со зрением.
type Image struct {
	MIME string
	Data []byte
}

// Document — содержимое конспекта: текст по страницам или изображения.
type Document struct {
	Pages  []Page
	Images []Image
}

// Text — весь текст документа одной строкой.
func (d Document) Text() string {
	parts := make([]string, 0, len(d.Pages))
	for _, p := range d.Pages {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "\n\n")
}

// PageCount — число страниц или изображений.
func (d Document) PageCount() int {
	return len(d.Pages) + len(d.Images)
}

// Kind определяет тип файла по содержимому, а при неоднозначности — по расширению.
// Возвращает MIME-тип или ErrUnsupported.
func Kind(filename string, data []byte) (string, error) {
	sniffed := http.DetectContentType(data)
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case sniffed == "application/pdf":
		return "application/pdf", nil
	case sniffed == "image/jpeg", sniffed == "image/png":
		return sniffed, nil
	case sniffed == "application/zip" && ext == ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
	case strings.HasPrefix(sniffed, "text/plain") && (ext == ".txt" || ext == ".md"):
		return "text/plain", nil
	}
	return "", ErrUnsupported
}

// Parse извлекает содержимое файла типа mime (результат Kind).
func Parse(mime string, data []byte) (Document, error) {
	switch {
	case mime == "application/pdf":
		return parsePDF(data)
	case strings.HasPrefix(mime, "image/"):
		return Document{Images: []Image{{MIME: mime, Data: data}}}, nil
	case strings.Contains(mime, "wordprocessingml"):
		return parseDOCX(data)
	case mime == "text/plain":
		if !utf8.Valid(data) {
			return Document{}, fmt.Errorf("text file is not UTF-8: %w", ErrUnsupported)
		}
		return nonEmpty(Document{Pages: []Page{{Text: normalize(string(data))}}})
	}
	return Document{}, ErrUnsupported
}

// minTextPerPage — если текста меньше, считаем PDF сканом без текстового слоя.
const minTextPerPage = 40

func parsePDF(data []byte) (doc Document, err error) {
	// Библиотека разбора PDF может паниковать на повреждённых файлах.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("broken PDF: %v", r)
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Document{}, fmt.Errorf("read PDF: %w", err)
	}
	total := 0
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			return Document{}, fmt.Errorf("PDF page %d: %w", i, err)
		}
		text = normalize(text)
		total += utf8.RuneCountInString(text)
		doc.Pages = append(doc.Pages, Page{Number: i, Text: text})
	}
	if r.NumPage() == 0 || total < minTextPerPage*r.NumPage() {
		return Document{}, ErrNoText
	}
	return doc, nil
}

// parseDOCX собирает текст абзацев из word/document.xml.
func parseDOCX(data []byte) (Document, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Document{}, fmt.Errorf("read DOCX: %w", err)
	}
	var body io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			body, err = f.Open()
			if err != nil {
				return Document{}, err
			}
			break
		}
	}
	if body == nil {
		return Document{}, fmt.Errorf("DOCX without word/document.xml: %w", ErrUnsupported)
	}
	defer body.Close()

	var sb strings.Builder
	dec := xml.NewDecoder(io.LimitReader(body, 50<<20))
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Document{}, fmt.Errorf("parse DOCX: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				sb.WriteByte('\t')
			case "br":
				sb.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				sb.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}
	return nonEmpty(Document{Pages: []Page{{Text: normalize(sb.String())}}})
}

func nonEmpty(d Document) (Document, error) {
	if strings.TrimSpace(d.Text()) == "" {
		return Document{}, ErrNoText
	}
	return d, nil
}

// normalize убирает лишние пробелы и пустые строки, сохраняя абзацы.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, l := range lines {
		l = strings.Join(strings.Fields(l), " ")
		if l == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		blank = false
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

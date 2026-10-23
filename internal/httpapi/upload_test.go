package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/storage"
)

type fakeUploads struct {
	created map[string][]storage.File // hash → файлы
	ids     map[string]string         // hash → id графа
	metas   map[string]domain.GraphMeta
}

func newFakeUploads() *fakeUploads {
	return &fakeUploads{created: map[string][]storage.File{}, ids: map[string]string{}, metas: map[string]domain.GraphMeta{}}
}

func (f *fakeUploads) CreateFromUpload(_ context.Context, meta domain.GraphMeta, hash string, files []storage.File) error {
	f.created[hash], f.ids[hash], f.metas[hash] = files, meta.ID, meta
	return nil
}

func (f *fakeUploads) GraphBySourceHash(_ context.Context, hash string) (string, error) {
	if id, ok := f.ids[hash]; ok {
		return id, nil
	}
	return "", storage.ErrNotFound
}

type part struct {
	name string
	data []byte
}

func postFile(t *testing.T, h http.Handler, name string, data []byte) (int, map[string]any) {
	t.Helper()
	return postFiles(t, h, part{name, data})
}

func postFiles(t *testing.T, h http.Handler, parts ...part) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, p := range parts {
		fw, err := mw.CreateFormFile("file", p.name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(p.data)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/graphs", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestUploadDisabled(t *testing.T) {
	h := New(fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	if code, _ := postFile(t, h, "a.txt", []byte("Закон Ома")); code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", code)
	}
}

func TestUpload(t *testing.T) {
	store := newFakeUploads()
	notified := 0
	h := New(fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, &Uploads{
		Store: store, Notify: func() { notified++ }, PerHour: 2,
	})

	code, body := postFile(t, h, "Механика_лекция_03.txt", []byte("Второй закон Ньютона: F = ma"))
	if code != http.StatusAccepted {
		t.Fatalf("code = %d, body = %v", code, body)
	}
	id, _ := body["id"].(string)
	if !regexp.MustCompile(`^n[a-z0-9]{11}$`).MatchString(id) {
		t.Errorf("id = %q", id)
	}
	if notified != 1 || len(store.created) != 1 {
		t.Errorf("notified = %d, created = %d", notified, len(store.created))
	}
	for _, files := range store.created {
		if len(files) != 1 || files[0].MIME != "text/plain" || files[0].Filename != "Механика_лекция_03.txt" {
			t.Errorf("upload = %+v", files)
		}
	}

	// Тот же файл повторно — возвращается существующий граф, лимит не тратится.
	code, body = postFile(t, h, "copy.txt", []byte("Второй закон Ньютона: F = ma"))
	if code != http.StatusOK || body["id"] != id || body["existing"] != true {
		t.Errorf("repeat: code = %d, body = %v", code, body)
	}

	if code, _ := postFile(t, h, "virus.exe", []byte("MZ\x90\x00")); code != http.StatusUnsupportedMediaType {
		t.Errorf("exe: code = %d, want 415", code)
	}

	// Лимит 2 в час: второй новый файл проходит, третий — нет.
	if code, _ := postFile(t, h, "b.txt", []byte("Закон сохранения энергии")); code != http.StatusAccepted {
		t.Errorf("second upload: code = %d", code)
	}
	if code, body := postFile(t, h, "c.txt", []byte("Закон Кулона")); code != http.StatusTooManyRequests {
		t.Errorf("third upload: code = %d, body = %v", code, body)
	}
}

func TestLimiterWindow(t *testing.T) {
	l := newLimiter(1, time.Hour)
	now := time.Now()
	if !l.allow("ip", now) || l.allow("ip", now.Add(time.Minute)) {
		t.Error("second hit within window should be denied")
	}
	if !l.allow("other", now) {
		t.Error("limits are per key")
	}
	if !l.allow("ip", now.Add(61*time.Minute)) {
		t.Error("hit after window should be allowed")
	}
}

func TestTitleFromFilename(t *testing.T) {
	if got := titleFromFilename("Механика_лекция_03.pdf"); got != "Механика лекция 03" {
		t.Errorf("title = %q", got)
	}
	if got := titleFromFilename(".pdf"); got != "Конспект" {
		t.Errorf("title = %q", got)
	}
}

func TestUploadSeveralPhotos(t *testing.T) {
	store := newFakeUploads()
	h := New(fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, &Uploads{Store: store, PerHour: 10})
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	jpg := []byte("\xff\xd8\xff\xe0" + strings.Repeat("\x00", 32))

	code, body := postFiles(t, h, part{"IMG_0412.jpg", jpg}, part{"IMG_0413.png", png}, part{"IMG_0414.jpg", jpg})
	if code != http.StatusAccepted {
		t.Fatalf("code = %d, body = %v", code, body)
	}
	for hash, files := range store.created {
		if len(files) != 3 || files[0].Filename != "IMG_0412.jpg" || files[1].MIME != "image/png" {
			t.Errorf("files = %+v", files)
		}
		if m := store.metas[hash]; m.Title != "IMG 0412" || m.Source != "IMG_0412.jpg и ещё 2 фото" {
			t.Errorf("meta = %+v", m)
		}
	}

	// Документ вперемешку с фото — нельзя.
	if code, _ := postFiles(t, h, part{"a.jpg", jpg}, part{"notes.txt", []byte("Закон Ома")}); code != http.StatusUnsupportedMediaType {
		t.Errorf("mixed: code = %d, want 415", code)
	}
	// Больше MaxFiles фото — нельзя.
	many := make([]part, 11)
	for i := range many {
		many[i] = part{fmt.Sprintf("p%d.jpg", i), jpg}
	}
	if code, _ := postFiles(t, h, many...); code != http.StatusBadRequest {
		t.Errorf("too many: code = %d, want 400", code)
	}
}

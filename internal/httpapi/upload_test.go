package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/storage"
)

type fakeUploads struct {
	created map[string]storage.Upload // hash → файл
	ids     map[string]string         // hash → id графа
}

func (f *fakeUploads) CreateFromUpload(_ context.Context, meta domain.GraphMeta, hash string, up storage.Upload) error {
	f.created[hash], f.ids[hash] = up, meta.ID
	return nil
}

func (f *fakeUploads) GraphBySourceHash(_ context.Context, hash string) (string, error) {
	if id, ok := f.ids[hash]; ok {
		return id, nil
	}
	return "", storage.ErrNotFound
}

func postFile(t *testing.T, h http.Handler, name string, data []byte) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
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
	store := &fakeUploads{created: map[string]storage.Upload{}, ids: map[string]string{}}
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
	for _, up := range store.created {
		if up.MIME != "text/plain" || up.Filename != "Механика_лекция_03.txt" {
			t.Errorf("upload = %+v", up)
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

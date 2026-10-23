package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"graph_of_physical_knowledge/internal/explain"
	"graph_of_physical_knowledge/internal/llm"
	"graph_of_physical_knowledge/internal/storage"
)

type memStories struct {
	mu   sync.Mutex
	data map[string]string
}

func (m *memStories) Story(_ context.Context, graphID, slug string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.data[graphID+"/"+slug]; ok {
		return t, nil
	}
	return "", storage.ErrNotFound
}

func (m *memStories) SaveStory(_ context.Context, graphID, slug, text, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[graphID+"/"+slug] = text
	return nil
}

type slowTeller struct {
	calls atomic.Int32
	delay time.Duration
}

func (t *slowTeller) Tell(_ context.Context, exp explain.Explanation) (string, llm.Usage, error) {
	t.calls.Add(1)
	time.Sleep(t.delay)
	return "Рассказ про «" + exp.Target.Title + "»", llm.Usage{}, nil
}

func TestStory(t *testing.T) {
	store := &memStories{data: map[string]string{}}
	teller := &slowTeller{delay: 50 * time.Millisecond}
	h := New(fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, &Stories{Store: store, Teller: teller, PerHour: 10})
	do := func(method, path string) (int, storyResponse) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		var body storyResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}
	const path = "/api/graphs/g/nodes/phen/story"

	if code, _ := do(http.MethodGet, path); code != http.StatusNotFound {
		t.Fatalf("GET before writing: code = %d, want 404", code)
	}
	// Два одновременных запроса — одна модель.
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, body := do(http.MethodPost, path); code != http.StatusOK || body.Text == "" {
				t.Errorf("POST: code = %d, body = %+v", code, body)
			}
		}()
	}
	wg.Wait()
	if code, body := do(http.MethodGet, path); code != http.StatusOK || body.Text != "Рассказ про «Явление»" {
		t.Errorf("GET after writing: code = %d, body = %+v", code, body)
	}
	if code, _ := do(http.MethodPost, path); code != http.StatusOK {
		t.Errorf("cached POST: code = %d", code)
	}
	if n := teller.calls.Load(); n != 1 {
		t.Errorf("model called %d times, want 1", n)
	}
	if code, _ := do(http.MethodPost, "/api/graphs/g/nodes/nope/story"); code != http.StatusNotFound {
		t.Errorf("unknown node: code = %d, want 404", code)
	}
}

func TestStoryDisabled(t *testing.T) {
	h := New(fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/graphs/g/nodes/phen/story", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", rec.Code)
	}
}

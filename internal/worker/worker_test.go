package worker

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"graph_of_physical_knowledge/internal/builder"
	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/llm"
	"graph_of_physical_knowledge/internal/storage"
)

type memStore struct {
	mu       sync.Mutex
	jobs     []storage.Upload
	stages   []string
	done     map[string]domain.Graph
	titles   map[string]string
	failures map[string]string
}

func newMemStore(jobs ...storage.Upload) *memStore {
	return &memStore{jobs: jobs, done: map[string]domain.Graph{}, titles: map[string]string{}, failures: map[string]string{}}
}

func (m *memStore) ClaimJob(context.Context) (storage.Upload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.jobs) == 0 {
		return storage.Upload{}, storage.ErrNotFound
	}
	j := m.jobs[0]
	m.jobs = m.jobs[1:]
	return j, nil
}

func (m *memStore) SetStage(_ context.Context, _, stage, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.stages) == 0 || m.stages[len(m.stages)-1] != stage {
		m.stages = append(m.stages, stage)
	}
	return nil
}

func (m *memStore) CompleteGraph(_ context.Context, id, title, _ string, _ int, g domain.Graph) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.done[id], m.titles[id] = g, title
	return nil
}

func (m *memStore) FailGraph(_ context.Context, id, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[id] = reason
	return nil
}

// runUntilIdle запускает воркер и останавливает его, когда все задания завершены.
func runUntilIdle(t *testing.T, w *Worker, s *memStore) {
	t.Helper()
	total := len(s.jobs)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		finished := len(s.done)+len(s.failures) == total
		s.mu.Unlock()
		if finished || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
}

func TestWorkerBuildsGraph(t *testing.T) {
	notes, err := os.ReadFile("../../testdata/notes/electricity.txt")
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore(storage.Upload{GraphID: "g1", Filename: "Конспект урока.txt", MIME: "text/plain", Data: notes})
	w := New(s, &builder.Builder{LLM: llm.Stub{}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w.Poll = 10 * time.Millisecond
	runUntilIdle(t, w, s)

	if reason, failed := s.failures["g1"]; failed {
		t.Fatalf("failed: %s", reason)
	}
	if len(s.done["g1"].Nodes) == 0 {
		t.Fatal("graph not saved")
	}
	if s.stages[0] != builder.StageReading || s.stages[1] != builder.StageExtracting {
		t.Errorf("stages = %v", s.stages)
	}
}

func TestWorkerReportsUserFriendlyErrors(t *testing.T) {
	s := newMemStore(
		storage.Upload{GraphID: "empty", Filename: "empty.txt", MIME: "text/plain", Data: []byte("   ")},
		storage.Upload{GraphID: "nophysics", Filename: "poem.txt", MIME: "text/plain", Data: []byte("Мороз и солнце; день чудесный!")},
	)
	w := New(s, &builder.Builder{LLM: llm.Stub{}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w.Poll = 10 * time.Millisecond
	runUntilIdle(t, w, s)

	if s.failures["empty"] != msgNoText {
		t.Errorf("empty: %q", s.failures["empty"])
	}
	if s.failures["nophysics"] != msgNoPhysics {
		t.Errorf("nophysics: %q", s.failures["nophysics"])
	}
}

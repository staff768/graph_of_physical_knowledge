package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/storage"
)

// fakeStore хранит один граф «g» из двух узлов: law → phen.
type fakeStore struct{}

var fakeGraph = domain.Graph{
	Nodes: []domain.Node{
		{Slug: "law", Type: domain.NodeLaw, Title: "Закон"},
		{Slug: "phen", Type: domain.NodePhenomenon, Title: "Явление"},
	},
	Edges: []domain.Edge{{From: "law", To: "phen", Type: domain.EdgeExplains}},
}

func (fakeStore) Ping(context.Context) error { return nil }

func (fakeStore) ListGraphs(context.Context, string) ([]domain.GraphMeta, error) {
	return []domain.GraphMeta{{ID: "g", Title: "Граф"}}, nil
}

func (fakeStore) GraphMeta(_ context.Context, id string) (domain.GraphMeta, error) {
	if id != "g" {
		return domain.GraphMeta{}, storage.ErrNotFound
	}
	return domain.GraphMeta{ID: "g", Title: "Граф", NodeCount: 2, EdgeCount: 1}, nil
}

func (fakeStore) Graph(context.Context, string) (domain.Graph, error) { return fakeGraph, nil }

func (fakeStore) Node(_ context.Context, _, slug string) (domain.Node, error) {
	for _, n := range fakeGraph.Nodes {
		if n.Slug == slug {
			return n, nil
		}
	}
	return domain.Node{}, storage.ErrNotFound
}

func (fakeStore) Neighbors(context.Context, string, string) ([]domain.Link, []domain.Link, error) {
	return []domain.Link{}, []domain.Link{}, nil
}

func (s fakeStore) Ancestors(ctx context.Context, graphID, slug string) (domain.Graph, error) {
	if _, err := s.Node(ctx, graphID, slug); err != nil {
		return domain.Graph{}, err
	}
	return fakeGraph, nil
}

func (fakeStore) DeleteGraph(_ context.Context, id string) error {
	if id == "demo" {
		return storage.ErrDemoGraph
	}
	return nil
}

func (fakeStore) Search(context.Context, string, string, int) ([]domain.Node, error) {
	return fakeGraph.Nodes[:1], nil
}

func get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	h := New(fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: invalid JSON: %v", path, err)
	}
	obj, _ := body.(map[string]any)
	return rec.Code, obj
}

func TestRoutes(t *testing.T) {
	cases := []struct {
		path   string
		status int
		errMsg string
	}{
		{"/api/graphs", http.StatusOK, ""},
		{"/api/graphs/g", http.StatusOK, ""},
		{"/api/graphs/g/nodes/law", http.StatusOK, ""},
		{"/api/graphs/g/nodes/phen/chain", http.StatusOK, ""},
		{"/api/graphs/g/search?q=зак", http.StatusOK, ""},
		{"/api/graphs/missing", http.StatusNotFound, "graph not found"},
		{"/api/graphs/missing/nodes/law/chain", http.StatusNotFound, "graph not found"},
		{"/api/graphs/g/nodes/missing", http.StatusNotFound, "node not found"},
		{"/api/graphs/g/nodes/missing/chain", http.StatusNotFound, "node not found"},
		{"/api/unknown", http.StatusNotFound, "not found"},
	}
	for _, tc := range cases {
		status, body := get(t, tc.path)
		if status != tc.status {
			t.Errorf("%s: status = %d, want %d", tc.path, status, tc.status)
		}
		if tc.errMsg != "" && body["error"] != tc.errMsg {
			t.Errorf("%s: error = %v, want %q", tc.path, body["error"], tc.errMsg)
		}
	}
}

func TestGraphResponseShape(t *testing.T) {
	_, body := get(t, "/api/graphs/g")
	meta, _ := body["meta"].(map[string]any)
	if meta["id"] != "g" || meta["nodeCount"] != float64(2) {
		t.Errorf("meta = %v", meta)
	}
	if nodes, _ := body["nodes"].([]any); len(nodes) != 2 {
		t.Errorf("nodes = %v", body["nodes"])
	}
}

func TestChainResponse(t *testing.T) {
	_, body := get(t, "/api/graphs/g/nodes/phen/chain")
	if steps, _ := body["steps"].([]any); len(steps) != 2 {
		t.Errorf("steps = %v", body["steps"])
	}
}

func TestDeleteGraph(t *testing.T) {
	h := New(fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	del := func(path string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, path, nil))
		return rec.Code
	}
	if code := del("/api/graphs/g"); code != http.StatusNoContent {
		t.Errorf("delete g: %d, want 204", code)
	}
	if code := del("/api/graphs/missing"); code != http.StatusNotFound {
		t.Errorf("delete missing: %d, want 404", code)
	}
}

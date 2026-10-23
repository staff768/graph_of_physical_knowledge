package storage

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"graph_of_physical_knowledge/internal/domain"
)

// Интеграционные тесты: запускаются, только если задан TEST_DATABASE_URL.
// ВНИМАНИЕ: тесты перезаписывают графы test-a и test-b в указанной базе.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	g := domain.Graph{
		Nodes: []domain.Node{
			{Slug: "law", Type: domain.NodeLaw, Title: "Закон"},
			{Slug: "qty", Type: domain.NodeQuantity, Title: "Величина"},
			{Slug: "formula", Type: domain.NodeFormula, Title: "Формула"},
			{Slug: "phen", Type: domain.NodePhenomenon, Title: "Явление", Summary: "Описание 100%"},
			{Slug: "other", Type: domain.NodeLaw, Title: "Посторонний закон"},
		},
		Edges: []domain.Edge{
			{From: "law", To: "formula", Type: domain.EdgeFormulates},
			{From: "qty", To: "formula", Type: domain.EdgeUsedIn},
			{From: "formula", To: "phen", Type: domain.EdgeExplains, Description: "потому что"},
		},
	}
	if err := s.ReplaceGraph(ctx, domain.GraphMeta{ID: "test-a", Title: "Тестовый граф А", Section: "Механика"}, g); err != nil {
		t.Fatal(err)
	}
	// Второй граф с теми же slug, но другим содержимым — проверяет изоляцию.
	other := domain.Graph{Nodes: []domain.Node{{Slug: "law", Type: domain.NodeLaw, Title: "Закон из графа Б"}}}
	if err := s.ReplaceGraph(ctx, domain.GraphMeta{ID: "test-b", Title: "Тестовый граф Б"}, other); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAncestors(t *testing.T) {
	s := newTestStore(t)
	sub, err := s.Ancestors(context.Background(), "test-a", "phen")
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, n := range sub.Nodes {
		slugs = append(slugs, n.Slug)
	}
	slices.Sort(slugs)
	if want := []string{"formula", "law", "phen", "qty"}; !slices.Equal(slugs, want) {
		t.Errorf("ancestors = %v, want %v", slugs, want)
	}
	if len(sub.Edges) != 3 {
		t.Errorf("edges = %d, want 3", len(sub.Edges))
	}

	if _, err := s.Ancestors(context.Background(), "test-a", "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if _, err := s.Ancestors(context.Background(), "test-b", "phen"); !errors.Is(err, ErrNotFound) {
		t.Errorf("node of another graph: err = %v, want ErrNotFound", err)
	}
}

func TestGraphIsolation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a, err := s.Node(ctx, "test-a", "law")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Node(ctx, "test-b", "law")
	if err != nil {
		t.Fatal(err)
	}
	if a.Title == b.Title {
		t.Errorf("same slug in different graphs returned the same node: %q", a.Title)
	}

	gb, err := s.Graph(ctx, "test-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(gb.Nodes) != 1 || len(gb.Edges) != 0 {
		t.Errorf("graph test-b = %d nodes, %d edges; want 1, 0", len(gb.Nodes), len(gb.Edges))
	}

	// Повторная загрузка графа Б не трогает граф А.
	if err := s.ReplaceGraph(ctx, domain.GraphMeta{ID: "test-b", Title: "Б"}, domain.Graph{}); err != nil {
		t.Fatal(err)
	}
	ga, err := s.Graph(ctx, "test-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(ga.Nodes) != 5 || len(ga.Edges) != 3 {
		t.Errorf("graph test-a after replacing test-b = %d nodes, %d edges; want 5, 3", len(ga.Nodes), len(ga.Edges))
	}
}

func TestListGraphs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	all, err := s.ListGraphs(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var a *domain.GraphMeta
	for i := range all {
		if all[i].ID == "test-a" {
			a = &all[i]
		}
	}
	if a == nil {
		t.Fatal("test-a not listed")
	}
	if a.NodeCount != 5 || a.EdgeCount != 3 || a.Section != "Механика" {
		t.Errorf("meta = %+v", *a)
	}
	// В превью первым идёт самое связанное понятие — формула (3 связи).
	if len(a.Preview) != previewSize || a.Preview[0] != "Формула" {
		t.Errorf("preview = %v", a.Preview)
	}

	// Поиск находит граф и по названию, и по понятию внутри него.
	for _, q := range []string{"граф а", "явление"} {
		res, err := s.ListGraphs(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(res, func(m domain.GraphMeta) bool { return m.ID == "test-a" }) {
			t.Errorf("query %q did not find test-a", q)
		}
		if slices.ContainsFunc(res, func(m domain.GraphMeta) bool { return m.ID == "test-b" }) {
			t.Errorf("query %q unexpectedly found test-b", q)
		}
	}

	if _, err := s.GraphMeta(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNeighbors(t *testing.T) {
	s := newTestStore(t)
	in, out, err := s.Neighbors(context.Background(), "test-a", "formula")
	if err != nil {
		t.Fatal(err)
	}
	if len(in) != 2 || len(out) != 1 {
		t.Fatalf("in = %d, out = %d", len(in), len(out))
	}
	if out[0].Slug != "phen" || out[0].Description != "потому что" || out[0].NodeType != domain.NodePhenomenon {
		t.Errorf("unexpected outgoing link: %+v", out[0])
	}
}

func TestSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	res, err := s.Search(ctx, "test-a", "ЗАКОН", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Slug != "law" {
		t.Errorf("search = %+v, want law first", res)
	}
	// Символы LIKE экранируются.
	res, err = s.Search(ctx, "test-a", "100%", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Slug != "phen" {
		t.Errorf("search 100%% = %+v", res)
	}
	res, err = s.Search(ctx, "test-a", "%", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Errorf("search %% matched %d nodes, want 1", len(res))
	}
}

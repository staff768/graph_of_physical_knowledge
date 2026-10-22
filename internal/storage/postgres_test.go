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
// ВНИМАНИЕ: тесты полностью перезаписывают граф в указанной базе.
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
	if err := s.ReplaceGraph(ctx, g); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAncestors(t *testing.T) {
	s := newTestStore(t)
	sub, err := s.Ancestors(context.Background(), "phen")
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

	if _, err := s.Ancestors(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNeighbors(t *testing.T) {
	s := newTestStore(t)
	in, out, err := s.Neighbors(context.Background(), "formula")
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
	res, err := s.Search(ctx, "ЗАКОН", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Slug != "law" {
		t.Errorf("search = %+v, want law first", res)
	}
	// Символы LIKE экранируются.
	res, err = s.Search(ctx, "100%", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Slug != "phen" {
		t.Errorf("search 100%% = %+v", res)
	}
	res, err = s.Search(ctx, "%", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Errorf("search %% matched %d nodes, want 1", len(res))
	}
}

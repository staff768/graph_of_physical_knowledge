package explain

import (
	"slices"
	"testing"

	"graph_of_physical_knowledge/internal/domain"
)

func testGraph() domain.Graph {
	return domain.Graph{
		Nodes: []domain.Node{
			{Slug: "short-circuit", Type: domain.NodePhenomenon, Title: "Короткое замыкание"},
			{Slug: "overheating", Type: domain.NodePhenomenon, Title: "Перегрев"},
			{Slug: "ohm", Type: domain.NodeLaw, Title: "Закон Ома"},
			{Slug: "ohm-formula", Type: domain.NodeFormula, Title: "I = U/R"},
			{Slug: "current", Type: domain.NodeQuantity, Title: "Сила тока"},
			{Slug: "joule", Type: domain.NodeLaw, Title: "Закон Джоуля–Ленца"},
		},
		Edges: []domain.Edge{
			{From: "ohm", To: "ohm-formula", Type: domain.EdgeFormulates},
			{From: "current", To: "ohm-formula", Type: domain.EdgeUsedIn},
			{From: "ohm-formula", To: "short-circuit", Type: domain.EdgeExplains},
			{From: "short-circuit", To: "overheating", Type: domain.EdgeCauses},
			{From: "joule", To: "overheating", Type: domain.EdgeExplains},
		},
	}
}

func TestBuildOrdersStepsTopologically(t *testing.T) {
	exp, err := Build("overheating", testGraph())
	if err != nil {
		t.Fatal(err)
	}
	pos := make(map[string]int)
	for i, s := range exp.Steps {
		pos[s.Node.Slug] = i
		if s.Index != i+1 {
			t.Errorf("step %d has index %d", i, s.Index)
		}
	}
	for _, e := range testGraph().Edges {
		if pos[e.From] >= pos[e.To] {
			t.Errorf("edge %s -> %s: base appears after consequence", e.From, e.To)
		}
	}
	if last := exp.Steps[len(exp.Steps)-1].Node.Slug; last != "overheating" {
		t.Errorf("last step = %s, want target", last)
	}
	if exp.Levels != 4 {
		t.Errorf("levels = %d, want 4", exp.Levels)
	}
}

func TestBuildCausalChain(t *testing.T) {
	exp, err := Build("overheating", testGraph())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"short-circuit", "overheating"}
	if !slices.Equal(exp.CausalChain, want) {
		t.Errorf("causal chain = %v, want %v", exp.CausalChain, want)
	}
}

func TestBuildReasons(t *testing.T) {
	exp, err := Build("overheating", testGraph())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range exp.Steps {
		if s.Node.Slug != "ohm-formula" {
			continue
		}
		if len(s.Reasons) != 2 {
			t.Fatalf("reasons = %d, want 2", len(s.Reasons))
		}
		// Закон идёт раньше величины.
		if s.Reasons[0].Text != "Математическая запись закона «Закон Ома»" {
			t.Errorf("first reason = %q", s.Reasons[0].Text)
		}
		return
	}
	t.Fatal("ohm-formula step not found")
}

func TestBuildRootNode(t *testing.T) {
	g := testGraph()
	exp, err := Build("ohm", domain.Graph{Nodes: g.Nodes[2:3]})
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.Steps) != 1 || exp.CausalChain != nil || exp.Levels != 1 {
		t.Errorf("unexpected explanation for root: %+v", exp)
	}
}

func TestBuildUnknownTarget(t *testing.T) {
	if _, err := Build("nope", testGraph()); err == nil {
		t.Error("expected error")
	}
}

func TestPlural(t *testing.T) {
	cases := map[int]string{1: "понятие", 2: "понятия", 5: "понятий", 11: "понятий", 21: "понятие", 104: "понятия"}
	for n, want := range cases {
		if got := plural(n, "понятие", "понятия", "понятий"); got != want {
			t.Errorf("plural(%d) = %s, want %s", n, got, want)
		}
	}
}

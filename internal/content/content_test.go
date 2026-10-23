package content

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"graph_of_physical_knowledge/internal/domain"
)

// TestRepositoryContent проверяет, что контент из репозитория проходит валидацию.
func TestRepositoryContent(t *testing.T) {
	b, err := LoadDir(os.DirFS("../../content"), ".")
	if err != nil {
		t.Fatal(err)
	}
	if b.Meta.ID != "demo" || !b.Meta.Demo {
		t.Errorf("meta = %+v, want demo graph", b.Meta)
	}
	g := b.Graph
	if len(g.Nodes) == 0 || len(g.Edges) == 0 {
		t.Fatal("content is empty")
	}
	for _, n := range g.Nodes {
		if n.Section == "" {
			t.Errorf("node %s: section not inherited from file", n.Slug)
		}
		if n.Summary == "" {
			t.Errorf("node %s: empty summary", n.Slug)
		}
	}
}

func TestLoadDirRejectsUnknownFields(t *testing.T) {
	fsys := fstest.MapFS{"a.yaml": {Data: []byte("graph: {id: g, title: G}\nnodes:\n  - slug: a\n    type: law\n    title: A\n    colour: red\n")}}
	if _, err := LoadDir(fsys, "."); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestLoadDirGraphBlock(t *testing.T) {
	node := "nodes:\n  - {slug: a, type: law, title: A}\n"
	cases := map[string]fstest.MapFS{
		"missing":   {"a.yaml": {Data: []byte(node)}},
		"duplicate": {"a.yaml": {Data: []byte("graph: {id: g, title: G}\n" + node)}, "b.yaml": {Data: []byte("graph: {id: h, title: H}\n")}},
		"bad id":    {"a.yaml": {Data: []byte("graph: {id: Bad Id, title: G}\n" + node)}},
	}
	for name, fsys := range cases {
		if _, err := LoadDir(fsys, "."); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	b, err := LoadDir(fstest.MapFS{"a.yaml": {Data: []byte("graph: {id: g, title: G}\n" + node)}}, ".")
	if err != nil || b.Meta.ID != "g" || len(b.Graph.Nodes) != 1 {
		t.Errorf("valid bundle: %+v, %v", b, err)
	}
}

func TestValidate(t *testing.T) {
	law := domain.Node{Slug: "law", Type: domain.NodeLaw, Title: "Закон"}
	formula := domain.Node{Slug: "formula", Type: domain.NodeFormula, Title: "Формула"}
	phen := domain.Node{Slug: "phen", Type: domain.NodePhenomenon, Title: "Явление"}
	phen2 := domain.Node{Slug: "phen-2", Type: domain.NodePhenomenon, Title: "Явление 2"}

	cases := []struct {
		name    string
		g       domain.Graph
		wantErr string
	}{
		{
			name: "valid",
			g: domain.Graph{
				Nodes: []domain.Node{law, formula, phen},
				Edges: []domain.Edge{
					{From: "law", To: "formula", Type: domain.EdgeFormulates},
					{From: "formula", To: "phen", Type: domain.EdgeExplains},
				},
			},
		},
		{
			name:    "bad slug",
			g:       domain.Graph{Nodes: []domain.Node{{Slug: "Bad Slug", Type: domain.NodeLaw, Title: "X"}}},
			wantErr: "invalid slug",
		},
		{
			name:    "duplicate slug",
			g:       domain.Graph{Nodes: []domain.Node{law, law}},
			wantErr: "duplicate slug",
		},
		{
			name: "unknown node",
			g: domain.Graph{
				Nodes: []domain.Node{law},
				Edges: []domain.Edge{{From: "law", To: "missing", Type: domain.EdgeFormulates}},
			},
			wantErr: "unknown node",
		},
		{
			name: "disallowed pair",
			g: domain.Graph{
				Nodes: []domain.Node{law, phen},
				Edges: []domain.Edge{{From: "phen", To: "law", Type: domain.EdgeCauses}},
			},
			wantErr: "not allowed",
		},
		{
			name: "cycle",
			g: domain.Graph{
				Nodes: []domain.Node{phen, phen2},
				Edges: []domain.Edge{
					{From: "phen", To: "phen-2", Type: domain.EdgeCauses},
					{From: "phen-2", To: "phen", Type: domain.EdgeCauses},
				},
			},
			wantErr: "cycle",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.g)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

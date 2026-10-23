package story

import (
	"context"
	"strings"
	"testing"

	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/explain"
	"graph_of_physical_knowledge/internal/llm"
)

type fixed struct{ req llm.Request }

func (f *fixed) Name() string { return "fixed" }
func (f *fixed) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	f.req = req
	return llm.Response{Text: "  Текст рассказа.  "}, nil
}

func TestTell(t *testing.T) {
	g := domain.Graph{
		Nodes: []domain.Node{
			{Slug: "joule", Type: domain.NodeLaw, Title: "Закон Джоуля–Ленца", Formula: "Q = I^2 R t"},
			{Slug: "heat", Type: domain.NodePhenomenon, Title: "Нагрев проводника", Warning: "в конспекте Q = IRt"},
		},
		Edges: []domain.Edge{{From: "joule", To: "heat", Type: domain.EdgeExplains, Description: "ток выделяет тепло"}},
	}
	exp, err := explain.Build("heat", g)
	if err != nil {
		t.Fatal(err)
	}
	m := &fixed{}
	text, _, err := (&Teller{LLM: m}).Tell(context.Background(), exp)
	if err != nil || text != "Текст рассказа." {
		t.Fatalf("text = %q, err = %v", text, err)
	}
	prompt := m.req.Messages[1].Text
	for _, want := range []string{"Цель: «Нагрев проводника» (явление)", "закон «Закон Джоуля–Ленца», формула $Q = I^2 R t$", "ток выделяет тепло", "Ошибка в конспекте: в конспекте Q = IRt"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
}

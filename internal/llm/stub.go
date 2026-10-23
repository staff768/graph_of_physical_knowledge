package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Stub — заглушка без сети для разработки и тестов пайплайна. Строит граф по
// простым правилам: пронумерованные заголовки конспекта становятся законами,
// строки с формулами под ними — формулами. Цитаты берутся из текста дословно.
type Stub struct{}

func (Stub) Name() string { return "stub" }

var (
	stubHeading = regexp.MustCompile(`(?m)^\d+\.\s+(.+)$`)
	stubFormula = regexp.MustCompile(`[^.:\n]*\b[A-Za-zΑ-Ωα-ωℰ][\wℰ]*\s*=\s*[^.,;\n]+`)
)

func (Stub) Complete(_ context.Context, req Request) (Response, error) {
	if req.Task != "extract" {
		// Проверка формул и исправления: заглушке нечего исправлять.
		return Response{Text: `{"issues": []}`}, nil
	}
	text := ""
	for _, m := range req.Messages {
		if m.Role == "user" {
			text = m.Text
		}
	}
	type node struct {
		Slug    string `json:"slug"`
		Type    string `json:"type"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
		Formula string `json:"formula,omitempty"`
		Origin  string `json:"origin"`
		Quote   string `json:"quote"`
	}
	type edge struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Type   string `json:"type"`
		Origin string `json:"origin"`
	}
	var nodes []node
	var edges []edge
	sections := stubHeading.FindAllStringSubmatchIndex(text, -1)
	for i, loc := range sections {
		title := strings.TrimSpace(text[loc[2]:loc[3]])
		end := len(text)
		if i+1 < len(sections) {
			end = sections[i+1][0]
		}
		law := fmt.Sprintf("topic-%d", i+1)
		nodes = append(nodes, node{Slug: law, Type: "law", Title: title, Summary: title, Origin: "explicit", Quote: title})
		for j, f := range stubFormula.FindAllString(text[loc[1]:end], 3) {
			f = strings.TrimSpace(f)
			slug := fmt.Sprintf("formula-%d-%d", i+1, j+1)
			nodes = append(nodes, node{Slug: slug, Type: "formula", Title: f, Summary: f, Formula: f, Origin: "explicit", Quote: f})
			edges = append(edges, edge{From: law, To: slug, Type: "formulates", Origin: "explicit"})
		}
	}
	out, err := json.Marshal(map[string]any{"title": "Конспект", "section": "Физика", "nodes": nodes, "edges": edges})
	if err != nil {
		return Response{}, err
	}
	return Response{Text: string(out)}, nil
}

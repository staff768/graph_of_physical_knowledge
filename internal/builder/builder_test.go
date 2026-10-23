package builder

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"graph_of_physical_knowledge/internal/content"
	"graph_of_physical_knowledge/internal/document"
	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/llm"
)

// scripted — фейковая модель: на каждую задачу отвечает следующим ответом из очереди.
type scripted struct {
	replies map[string][]any // task → ответы (string или error)
	calls   []llm.Request
}

func (s *scripted) Name() string { return "scripted" }

func (s *scripted) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	s.calls = append(s.calls, req)
	queue := s.replies[req.Task]
	if len(queue) == 0 {
		return llm.Response{}, errors.New("unexpected call: " + req.Task)
	}
	s.replies[req.Task] = queue[1:]
	if err, ok := queue[0].(error); ok {
		return llm.Response{}, err
	}
	return llm.Response{Text: queue[0].(string), Usage: llm.Usage{InputTokens: 100, OutputTokens: 50}}, nil
}

func (s *scripted) tasks() []string {
	var out []string
	for _, c := range s.calls {
		out = append(out, c.Task)
	}
	return out
}

var notes = document.Document{Pages: []document.Page{
	{Number: 1, Text: "Закон Ома для участка цепи: I = U / R.\nСила тока измеряется в амперах."},
	{Number: 2, Text: "Короткое замыкание — резкое увеличение тока при малом сопротивлении."},
}}

const goodDraft = `Вот граф:
` + "```json" + `
{
  "title": "Закон Ома", "section": "Электродинамика",
  "nodes": [
    {"slug": "ohm-law", "type": "law", "title": "Закон Ома", "summary": "Связь тока и напряжения.", "origin": "explicit", "quote": "Закон Ома для участка цепи"},
    {"slug": "ohm-formula", "type": "formula", "title": "I = U/R", "summary": "Формула закона Ома.", "formula": "I = \\dfrac{U}{R}", "origin": "explicit", "quote": "I=U/R"},
    {"slug": "short-circuit", "type": "phenomenon", "title": "Короткое замыкание", "summary": "Резкий рост тока.", "origin": "explicit", "quote": "короткое  замыкание"},
    {"slug": "joule", "type": "law", "title": "Закон Джоуля–Ленца", "summary": "Нагрев проводника.", "origin": "explicit", "quote": "Q = I^2 R t"}
  ],
  "edges": [
    {"from": "ohm-law", "to": "ohm-formula", "type": "formulates", "origin": "explicit"},
    {"from": "ohm-formula", "to": "short-circuit", "type": "explains", "origin": "inferred", "note": "При R→0 ток растёт."},
    {"from": "joule", "to": "short-circuit", "type": "explains", "origin": "inferred"}
  ]
}
` + "```"

func TestBuildGroundsQuotes(t *testing.T) {
	m := &scripted{replies: map[string][]any{"extract": {goodDraft}}}
	res, err := (&Builder{LLM: m}).Build(context.Background(), notes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := content.Validate(res.Graph); err != nil {
		t.Fatalf("graph is invalid: %v", err)
	}
	byslug := map[string]domain.Node{}
	for _, n := range res.Graph.Nodes {
		byslug[n.Slug] = n
	}
	// Цитата с другими пробелами и регистром находится, страница определяется.
	if n := byslug["ohm-formula"]; n.Origin != domain.OriginExplicit || n.SourcePage != 1 {
		t.Errorf("ohm-formula = %+v", n)
	}
	if n := byslug["short-circuit"]; n.Origin != domain.OriginExplicit || n.SourcePage != 2 {
		t.Errorf("short-circuit = %+v", n)
	}
	// Цитаты нет в конспекте — понятие становится дополненным, с пояснением.
	if n := byslug["joule"]; n.Origin != domain.OriginInferred || n.SourceQuote != "" || n.Note == "" {
		t.Errorf("joule = %+v", n)
	}
	if res.Title != "Закон Ома" || res.Usage.InputTokens != 100 {
		t.Errorf("result = %+v", res)
	}
}

func TestBuildRepairsInvalidGraph(t *testing.T) {
	broken := `{"title": "T", "section": "S", "nodes": [
		{"slug": "a", "type": "law", "title": "Закон А", "origin": "inferred"},
		{"slug": "f", "type": "formula", "title": "Формула Ф", "origin": "inferred"},
		{"slug": "p", "type": "phenomenon", "title": "Явление П", "origin": "inferred"},
		{"slug": "q", "type": "phenomenon", "title": "Явление Q", "origin": "inferred"}
	], "edges": [
		{"from": "f", "to": "a", "type": "formulates", "origin": "explicit"},
		{"from": "p", "to": "a", "type": "causes", "origin": "explicit"},
		{"from": "p", "to": "q", "type": "causes", "origin": "explicit"},
		{"from": "q", "to": "p", "type": "causes", "origin": "inferred"},
		{"from": "a", "to": "ghost", "type": "explains", "origin": "explicit"}
	]}`
	// Модель не справляется с исправлением — остаток чинится детерминированно.
	m := &scripted{replies: map[string][]any{"extract": {broken}, "repair": {"не JSON"}}}
	res, err := (&Builder{LLM: m}).Build(context.Background(), notes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := content.Validate(res.Graph); err != nil {
		t.Fatalf("graph is still invalid: %v", err)
	}
	has := func(from, to string) bool {
		for _, e := range res.Graph.Edges {
			if e.From == from && e.To == to {
				return true
			}
		}
		return false
	}
	if !has("a", "f") {
		t.Error("reversed formulates edge should be turned around")
	}
	if has("p", "a") {
		t.Error("disallowed causes edge should be dropped")
	}
	// Из цикла p⇄q удаляется дополненная связь, явная остаётся.
	if !has("p", "q") || has("q", "p") {
		t.Errorf("cycle broken wrong: %+v", res.Graph.Edges)
	}
	if len(res.Fixes) == 0 {
		t.Error("fixes should be reported")
	}
}

func TestBuildMergesChunks(t *testing.T) {
	part1 := `{"title": "Ток", "section": "Электродинамика", "nodes": [
		{"slug": "current", "type": "quantity", "title": "Сила тока", "origin": "inferred"},
		{"slug": "ohm", "type": "law", "title": "Закон Ома", "origin": "inferred"}
	], "edges": [{"from": "current", "to": "ohm", "type": "used_in", "origin": "inferred"}]}`
	// Во второй части модель назвала ту же величину другим slug.
	part2 := `{"nodes": [
		{"slug": "electric-current", "type": "quantity", "title": "сила  тока", "origin": "inferred"},
		{"slug": "short", "type": "phenomenon", "title": "Короткое замыкание", "origin": "inferred"},
		{"slug": "ohm-f", "type": "formula", "title": "I = U/R", "origin": "inferred"}
	], "edges": [
		{"from": "electric-current", "to": "ohm-f", "type": "used_in", "origin": "inferred"},
		{"from": "ohm-f", "to": "short", "type": "explains", "origin": "inferred"}
	]}`
	m := &scripted{replies: map[string][]any{"extract": {part1, part2}}}
	res, err := (&Builder{LLM: m, ChunkRunes: 60}).Build(context.Background(), notes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 2 {
		t.Fatalf("calls = %v, want two extract calls", m.tasks())
	}
	if !strings.Contains(m.calls[1].Messages[1].Text, "current (quantity): Сила тока") {
		t.Error("second chunk should list nodes found so far")
	}
	if len(res.Graph.Nodes) != 4 {
		t.Errorf("nodes = %d, want 4 (duplicate merged)", len(res.Graph.Nodes))
	}
	found := false
	for _, e := range res.Graph.Edges {
		if e.From == "current" && e.To == "ohm-f" {
			found = true
		}
	}
	if !found {
		t.Errorf("edge of merged node should be renamed: %+v", res.Graph.Edges)
	}
}

func TestBuildNoPhysics(t *testing.T) {
	m := &scripted{replies: map[string][]any{"extract": {`{"title": "", "nodes": [], "edges": []}`}}}
	if _, err := (&Builder{LLM: m}).Build(context.Background(), notes, nil); !errors.Is(err, ErrNoPhysics) {
		t.Errorf("err = %v, want ErrNoPhysics", err)
	}
}

func TestBuildRetriesAndChecksPhysics(t *testing.T) {
	check := `{"issues": [{"slug": "ohm-formula", "issue": "Возможно, в конспекте ошибка."}, {"slug": "missing", "issue": "x"}]}`
	m := &scripted{replies: map[string][]any{
		"extract": {&llm.APIError{Status: 429, Body: "rate limit"}, goodDraft},
		"check":   {check},
	}}
	var stages []string
	res, err := (&Builder{LLM: m, CheckPhysics: true}).Build(context.Background(), notes, func(stage, _ string) {
		stages = append(stages, stage)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(m.tasks(), ","); got != "extract,extract,check" {
		t.Errorf("calls = %s", got)
	}
	if got := strings.Join(stages, ","); got != "extracting,linking,checking" {
		t.Errorf("stages = %s", got)
	}
	for _, n := range res.Graph.Nodes {
		if (n.Slug == "ohm-formula") != (n.Warning != "") {
			t.Errorf("node %s warning = %q", n.Slug, n.Warning)
		}
	}
}

func TestBuildDoesNotRetryClientErrors(t *testing.T) {
	m := &scripted{replies: map[string][]any{"extract": {&llm.APIError{Status: 401, Body: "bad key"}}}}
	_, err := (&Builder{LLM: m}).Build(context.Background(), notes, nil)
	var apiErr *llm.APIError
	if !errors.As(err, &apiErr) || len(m.calls) != 1 {
		t.Errorf("err = %v, calls = %d", err, len(m.calls))
	}
}

// Заглушка строит валидный граф по настоящему тестовому конспекту.
func TestStubOnRealNotes(t *testing.T) {
	text, err := os.ReadFile("../../testdata/notes/electricity.txt")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := document.Parse("text/plain", text)
	if err != nil {
		t.Fatal(err)
	}
	res, err := (&Builder{LLM: llm.Stub{}, CheckPhysics: true}).Build(context.Background(), doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := content.Validate(res.Graph); err != nil {
		t.Fatalf("invalid graph: %v", err)
	}
	explicit := 0
	for _, n := range res.Graph.Nodes {
		if n.Origin == domain.OriginExplicit {
			explicit++
		}
	}
	if len(res.Graph.Nodes) < 10 || explicit < len(res.Graph.Nodes)/2 {
		t.Errorf("nodes = %d, explicit = %d", len(res.Graph.Nodes), explicit)
	}
}

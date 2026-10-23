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

func TestBuildRetypesEdgesInsteadOfDropping(t *testing.T) {
	// Живые конспекты: «электризация вызывает заряд», «заряд объясняет электроскоп»,
	// «сила тока вводит ампер» — смысл верный, тип не тот. Связи должны сохраниться.
	draft := `{"title": "T", "nodes": [
		{"slug": "electrification", "type": "phenomenon", "title": "Электризация", "origin": "inferred"},
		{"slug": "charge", "type": "quantity", "title": "Заряд", "origin": "inferred"},
		{"slug": "electroscope", "type": "phenomenon", "title": "Электроскоп", "origin": "inferred"},
		{"slug": "current", "type": "quantity", "title": "Сила тока", "origin": "inferred"},
		{"slug": "ampere", "type": "quantity", "title": "Ампер", "origin": "inferred"},
		{"slug": "law", "type": "law", "title": "Закон", "origin": "inferred"}
	], "edges": [
		{"from": "electrification", "to": "charge", "type": "causes", "origin": "inferred"},
		{"from": "charge", "to": "electroscope", "type": "used_in", "origin": "inferred"},
		{"from": "current", "to": "ampere", "type": "defines", "origin": "inferred"},
		{"from": "charge", "to": "current", "type": "explains", "origin": "inferred"},
		{"from": "electroscope", "to": "law", "type": "causes", "origin": "inferred"}
	]}`
	m := &scripted{replies: map[string][]any{"extract": {draft}}}
	res, err := (&Builder{LLM: m}).Build(context.Background(), notes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(m.tasks(), ","); got != "extract" {
		t.Errorf("calls = %s (no repair or link expected)", got)
	}
	if err := content.Validate(res.Graph); err != nil {
		t.Fatal(err)
	}
	want := map[string]domain.EdgeType{
		"electrification>charge": domain.EdgeDefines,
		"charge>electroscope":    domain.EdgeExplains,
		"current>ampere":         domain.EdgeDerives,
		"charge>current":         domain.EdgeDerives,
		"law>electroscope":       domain.EdgeExplains, // развёрнута и переименована
	}
	got := map[string]domain.EdgeType{}
	for _, e := range res.Graph.Edges {
		got[e.From+">"+e.To] = e.Type
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("edge %s = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
}

func TestEnforceBreaksDefinesCycleFirst(t *testing.T) {
	g := domain.Graph{
		Nodes: []domain.Node{{Slug: "f", Type: domain.NodeFormula, Title: "I = q/t"}, {Slug: "i", Type: domain.NodeQuantity, Title: "Сила тока"}},
		Edges: []domain.Edge{
			{From: "f", To: "i", Type: domain.EdgeDefines, Origin: domain.OriginExplicit},
			{From: "i", To: "f", Type: domain.EdgeUsedIn, Origin: domain.OriginInferred},
		},
	}
	out := enforce(g, &Result{})
	if len(out.Edges) != 1 || out.Edges[0].Type != domain.EdgeUsedIn {
		t.Errorf("edges = %+v, want only used_in", out.Edges)
	}
}

func TestBuildMultiPagePhotos(t *testing.T) {
	page1 := `{"title": "Ток", "section": "Электродинамика", "nodes": [
		{"slug": "ohm", "type": "law", "title": "Закон Ома", "origin": "explicit", "quote": "Закон Ома", "page": 1},
		{"slug": "ohm-f", "type": "formula", "title": "I = U/R", "origin": "explicit", "quote": "I = U/R", "page": 1}
	], "edges": [{"from": "ohm", "to": "ohm-f", "type": "formulates", "origin": "explicit"}]}`
	page2 := `{"nodes": [
		{"slug": "short", "type": "phenomenon", "title": "Короткое замыкание", "origin": "explicit", "quote": "КЗ", "page": 2}
	], "edges": [{"from": "ohm-f", "to": "short", "type": "explains", "origin": "explicit"}]}`
	m := &scripted{replies: map[string][]any{"extract": {page1, page2}}}
	doc := document.Document{Images: []document.Image{{MIME: "image/jpeg", Data: []byte("a")}, {MIME: "image/jpeg", Data: []byte("b")}}}
	var notes []string
	res, err := (&Builder{LLM: m}).Build(context.Background(), doc, func(stage, note string) {
		if stage == StageExtracting {
			notes = append(notes, note)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 2 || len(m.calls[1].Messages[1].Images) != 1 {
		t.Fatalf("calls = %d", len(m.calls))
	}
	if !strings.Contains(m.calls[1].Messages[1].Text, "Страница 2 из 2") || !strings.Contains(m.calls[1].Messages[1].Text, "page = 2") {
		t.Errorf("second request should name page 2: %q", m.calls[1].Messages[1].Text)
	}
	if !strings.HasPrefix(notes[0], "страница 1 из 2") || !strings.HasPrefix(notes[1], "страница 2 из 2") {
		t.Errorf("progress notes = %q", notes)
	}
	pages := map[string]int{}
	for _, n := range res.Graph.Nodes {
		pages[n.Slug] = n.SourcePage
	}
	if pages["ohm"] != 1 || pages["short"] != 2 || len(res.Graph.Edges) != 2 {
		t.Errorf("pages = %v, edges = %d", pages, len(res.Graph.Edges))
	}
}

func TestBuildLinksIsolatedConcepts(t *testing.T) {
	extract := `{"title": "T", "nodes": [
		{"slug": "ohm", "type": "law", "title": "Закон Ома", "origin": "inferred"},
		{"slug": "ohm-f", "type": "formula", "title": "I = U/R", "origin": "inferred"},
		{"slug": "current", "type": "quantity", "title": "Сила тока", "origin": "inferred"}
	], "edges": [{"from": "ohm", "to": "ohm-f", "type": "formulates", "origin": "inferred"}]}`
	link := `{"edges": [{"from": "current", "to": "ohm-f", "type": "used_in", "origin": "inferred", "note": "I входит в формулу"}]}`
	m := &scripted{replies: map[string][]any{"extract": {extract}, "link": {link}}}
	res, err := (&Builder{LLM: m}).Build(context.Background(), notes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(m.tasks(), ","); got != "extract,link" {
		t.Errorf("calls = %s", got)
	}
	if !strings.Contains(m.calls[1].Messages[1].Text, "current (quantity)") || !strings.Contains(m.calls[1].Messages[1].Text, "Текст конспекта") {
		t.Error("link request should list concepts and include the notes text")
	}
	if len(res.Graph.Edges) != 2 {
		t.Errorf("edges = %+v", res.Graph.Edges)
	}
}

func TestConnectivity(t *testing.T) {
	d := Draft{
		Nodes: []DraftNode{{Slug: "a"}, {Slug: "b"}, {Slug: "c"}, {Slug: "d"}},
		Edges: []DraftEdge{{From: "a", To: "b"}, {From: "c", To: "ghost"}},
	}
	if iso, parts := connectivity(d); iso != 2 || parts != 3 {
		t.Errorf("isolated = %d, parts = %d; want 2, 3", iso, parts)
	}
}

func TestLinkAcceptsOnlyConnectingEdges(t *testing.T) {
	d := Draft{
		Nodes: []DraftNode{{Slug: "a", Type: "law"}, {Slug: "f", Type: "formula"}, {Slug: "q", Type: "quantity"}},
		Edges: []DraftEdge{{From: "a", To: "f", Type: "formulates"}},
	}
	link := `{"edges": [
		{"from": "a", "to": "f", "type": "formulates"},
		{"from": "q", "to": "f", "type": "used_in"},
		{"from": "q", "to": "a", "type": "used_in"},
		{"from": "ghost", "to": "a", "type": "used_in"}
	]}`
	m := &scripted{replies: map[string][]any{"link": {link}}}
	var res Result
	(&Builder{LLM: m}).link(context.Background(), &d, document.Document{}, &res)
	// Принята только q→f: a→f уже есть, q→a соединяет уже соединённое, ghost неизвестен.
	if len(d.Edges) != 2 || d.Edges[1].From != "q" || d.Edges[1].To != "f" {
		t.Errorf("edges = %+v", d.Edges)
	}
}

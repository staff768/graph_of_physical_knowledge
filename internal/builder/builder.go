// Package builder строит граф знаний по тексту конспекта с помощью языковой модели.
//
// Схема работы: конспект режется на части → модель извлекает узлы и связи в JSON →
// цитаты сверяются с текстом → граф проверяется валидатором контента, ошибки
// отправляются модели на исправление, остаток исправляется детерминированно →
// отдельный проход ищет ошибки в формулах.
package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"graph_of_physical_knowledge/internal/content"
	"graph_of_physical_knowledge/internal/document"
	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/llm"
)

// ErrNoPhysics — модель не нашла в конспекте физических понятий.
var ErrNoPhysics = errors.New("no physics concepts found")

// Этапы построения для отображения пользователю.
const (
	StageReading    = "reading"
	StageExtracting = "extracting"
	StageLinking    = "linking"
	StageChecking   = "checking"
)

// Progress сообщает о смене этапа; note — подробность для пользователя.
type Progress func(stage, note string)

type Builder struct {
	LLM llm.Client
	Log *slog.Logger
	// CheckPhysics включает отдельный проход проверки формул.
	CheckPhysics bool
	// ChunkRunes — размер части конспекта в символах для одного запроса.
	ChunkRunes int
}

type Result struct {
	Title   string
	Section string
	Graph   domain.Graph
	Usage   llm.Usage
	// Fixes — что пришлось исправить автоматически (для логов и eval).
	Fixes []string
}

// Draft — граф в том виде, в котором его возвращает модель.
type Draft struct {
	Title   string      `json:"title"`
	Section string      `json:"section"`
	Nodes   []DraftNode `json:"nodes"`
	Edges   []DraftEdge `json:"edges"`
}

type DraftNode struct {
	Slug    string `json:"slug"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Formula string `json:"formula,omitempty"`
	Unit    string `json:"unit,omitempty"`
	Origin  string `json:"origin"`
	Quote   string `json:"quote,omitempty"`
	Page    int    `json:"page,omitempty"`
	Note    string `json:"note,omitempty"`
}

type DraftEdge struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Origin      string `json:"origin"`
	Quote       string `json:"quote,omitempty"`
	Note        string `json:"note,omitempty"`
}

func (b *Builder) log() *slog.Logger {
	if b.Log != nil {
		return b.Log
	}
	return slog.New(slog.DiscardHandler)
}

// Build строит граф по документу. progress может быть nil.
func (b *Builder) Build(ctx context.Context, doc document.Document, progress Progress) (Result, error) {
	if progress == nil {
		progress = func(string, string) {}
	}
	var res Result
	var draft Draft

	parts := b.split(doc)
	for i, part := range parts {
		note := "ищу понятия и формулы"
		if len(parts) > 1 {
			note = fmt.Sprintf("часть %d из %d", i+1, len(parts))
		}
		if n := len(draft.Nodes); n > 0 {
			note += fmt.Sprintf(" · найдено %d %s", n, plural(n, "понятие", "понятия", "понятий"))
		}
		progress(StageExtracting, note)

		user := part.text
		if len(draft.Nodes) > 0 {
			user = knownNodes(draft) + "\n\nКонспект (продолжение):\n" + part.text
		}
		var d Draft
		if err := b.ask(ctx, &res, "extract", extractPrompt, user, part.images, &d); err != nil {
			return Result{}, err
		}
		merge(&draft, d)
	}
	if len(draft.Nodes) < 2 {
		return Result{}, ErrNoPhysics
	}

	progress(StageLinking, fmt.Sprintf("найдено %d %s, проверяю связи", len(draft.Nodes), plural(len(draft.Nodes), "понятие", "понятия", "понятий")))
	ground(&draft, doc, &res)
	g := b.toGraph(&draft, &res)
	if err := content.Validate(g); err != nil {
		b.log().Info("graph needs repair", "err", err)
		payload, _ := json.Marshal(draft)
		var fixed Draft
		user := "Граф:\n" + string(payload) + "\n\nОшибки проверки:\n" + err.Error()
		if askErr := b.ask(ctx, &res, "repair", repairPrompt, user, nil, &fixed); askErr == nil && len(fixed.Nodes) > 0 {
			ground(&fixed, doc, &res)
			draft = fixed
			g = b.toGraph(&draft, &res)
		} else if askErr != nil {
			res.Fixes = append(res.Fixes, "исправление моделью не удалось: "+askErr.Error())
		}
		g = enforce(g, &res)
	}
	if len(g.Nodes) < 2 {
		return Result{}, ErrNoPhysics
	}

	if b.CheckPhysics {
		progress(StageChecking, "проверяю формулы")
		b.checkPhysics(ctx, &g, &res)
	}

	res.Title = strings.TrimSpace(draft.Title)
	res.Section = strings.TrimSpace(draft.Section)
	res.Graph = g
	return res, nil
}

type part struct {
	text   string
	images []llm.Image
}

// split режет документ на части: изображения — по одному, текст — по страницам
// до ChunkRunes символов. Страницы помечаются [стр. N], чтобы модель указывала их в цитатах.
func (b *Builder) split(doc document.Document) []part {
	var parts []part
	for _, img := range doc.Images {
		parts = append(parts, part{
			text:   "Конспект — на изображении. Прочитай его внимательно, включая рукописный текст и формулы.",
			images: []llm.Image{{MIME: img.MIME, Data: img.Data}},
		})
	}
	limit := b.ChunkRunes
	if limit <= 0 {
		limit = 12000
	}
	var cur strings.Builder
	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			parts = append(parts, part{text: "Конспект:\n" + cur.String()})
		}
		cur.Reset()
	}
	for _, p := range doc.Pages {
		text := p.Text
		if p.Number > 0 {
			text = fmt.Sprintf("[стр. %d]\n%s", p.Number, p.Text)
		}
		if cur.Len() > 0 && utf8.RuneCountInString(cur.String())+utf8.RuneCountInString(text) > limit {
			flush()
		}
		cur.WriteString(text)
		cur.WriteString("\n\n")
	}
	flush()
	return parts
}

func knownNodes(d Draft) string {
	var sb strings.Builder
	sb.WriteString(knownNodesIntro)
	for _, n := range d.Nodes {
		fmt.Fprintf(&sb, "\n- %s (%s): %s", n.Slug, n.Type, n.Title)
	}
	return sb.String()
}

// ask отправляет запрос модели и разбирает JSON-ответ в out.
// Сбои провайдера и лимиты повторяются с паузой.
func (b *Builder) ask(ctx context.Context, res *Result, task, system, user string, images []llm.Image, out any) error {
	req := llm.Request{
		Task: task,
		Messages: []llm.Message{
			{Role: "system", Text: system},
			{Role: "user", Text: user, Images: images},
		},
		JSON:        true,
		MaxTokens:   8000,
		Temperature: 0.1,
	}
	var resp llm.Response
	var err error
	for attempt, delay := 0, 2*time.Second; attempt < 3; attempt, delay = attempt+1, delay*3 {
		start := time.Now()
		resp, err = b.LLM.Complete(ctx, req)
		b.log().Info("llm call", "task", task, "model", b.LLM.Name(), "duration", time.Since(start).Round(time.Millisecond),
			"in", resp.Usage.InputTokens, "out", resp.Usage.OutputTokens, "err", err)
		var apiErr *llm.APIError
		if err == nil || !errors.As(err, &apiErr) || !apiErr.Retryable() {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	if err != nil {
		return err
	}
	res.Usage.InputTokens += resp.Usage.InputTokens
	res.Usage.OutputTokens += resp.Usage.OutputTokens
	if err := parseJSON(resp.Text, out); err != nil {
		return fmt.Errorf("%s: model returned invalid JSON: %w", task, err)
	}
	return nil
}

// parseJSON достаёт JSON-объект из ответа, даже если модель обернула его в ```json.
func parseJSON(text string, out any) error {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return errors.New("no JSON object in response")
	}
	return json.Unmarshal([]byte(text[start:end+1]), out)
}

// merge добавляет в draft узлы и связи из следующей части конспекта.
// Узел с тем же slug или тем же названием считается тем же понятием.
func merge(draft *Draft, next Draft) {
	if draft.Title == "" {
		draft.Title, draft.Section = next.Title, next.Section
	}
	bySlug := map[string]string{} // slug или ключ названия → итоговый slug
	for _, n := range draft.Nodes {
		bySlug[n.Slug] = n.Slug
		bySlug[titleKey(n.Title)] = n.Slug
	}
	rename := map[string]string{}
	for _, n := range next.Nodes {
		if existing, ok := bySlug[n.Slug]; ok {
			rename[n.Slug] = existing
			continue
		}
		if existing, ok := bySlug[titleKey(n.Title)]; ok {
			rename[n.Slug] = existing
			continue
		}
		draft.Nodes = append(draft.Nodes, n)
		bySlug[n.Slug] = n.Slug
		bySlug[titleKey(n.Title)] = n.Slug
	}
	for _, e := range next.Edges {
		if r, ok := rename[e.From]; ok {
			e.From = r
		}
		if r, ok := rename[e.To]; ok {
			e.To = r
		}
		draft.Edges = append(draft.Edges, e)
	}
}

func titleKey(title string) string {
	return "title:" + squash(title)
}

// squash приводит текст к виду для нечувствительного сравнения: нижний регистр,
// ё→е, без пробелов и с унифицированными тире и кавычками. PDF и модели по-разному
// расставляют пробелы («I = q / t» и «I=q/t»), поэтому пробелы убираются совсем.
func squash(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsSpace(r), r == '­':
			continue
		case r == 'ё':
			r = 'е'
		case r == '—' || r == '–' || r == '−':
			r = '-'
		case r == '«' || r == '»' || r == '“' || r == '”' || r == '„':
			r = '"'
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// ground сверяет цитаты с текстом конспекта. «Из конспекта» без подтверждённой
// цитаты становится «дополнено моделью»; номер страницы определяется по тексту.
// Для изображений текста нет, и цитаты проверить нельзя — они принимаются как есть.
func ground(d *Draft, doc document.Document, res *Result) {
	if len(doc.Pages) == 0 {
		return
	}
	pages := make([]string, len(doc.Pages))
	for i, p := range doc.Pages {
		pages[i] = squash(p.Text)
	}
	find := func(quote string) (page int, ok bool) {
		q := squash(quote)
		if utf8.RuneCountInString(q) < 3 {
			return 0, false
		}
		for i, p := range pages {
			if strings.Contains(p, q) {
				return doc.Pages[i].Number, true
			}
		}
		return 0, false
	}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if n.Origin != string(domain.OriginExplicit) {
			n.Origin, n.Quote = string(domain.OriginInferred), ""
			continue
		}
		page, ok := find(n.Quote)
		if !ok {
			res.Fixes = append(res.Fixes, fmt.Sprintf("цитата узла %q не найдена в конспекте", n.Slug))
			n.Origin, n.Quote, n.Page = string(domain.OriginInferred), "", 0
			if n.Note == "" {
				n.Note = "Понятие добавлено для связности: дословно в конспекте его не нашлось."
			}
			continue
		}
		n.Page = page
	}
	for i := range d.Edges {
		e := &d.Edges[i]
		if e.Origin != string(domain.OriginExplicit) {
			e.Origin = string(domain.OriginInferred)
			continue
		}
		// У связи цитата необязательна: связь, видимая из структуры конспекта, тоже явная.
		if e.Quote != "" {
			if _, ok := find(e.Quote); !ok {
				e.Origin = string(domain.OriginInferred)
			}
		}
	}
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// toGraph превращает черновик в граф, исправляя то, что можно исправить без модели:
// неверные slug, неизвестные типы, дубли, связи на несуществующие узлы и петли.
func (b *Builder) toGraph(d *Draft, res *Result) domain.Graph {
	var g domain.Graph
	slugs := map[string]string{} // slug модели → итоговый
	used := map[string]bool{}
	for i, n := range d.Nodes {
		t := domain.NodeType(strings.ToLower(strings.TrimSpace(n.Type)))
		if !t.Valid() || strings.TrimSpace(n.Title) == "" {
			res.Fixes = append(res.Fixes, fmt.Sprintf("узел %q пропущен: тип %q", n.Slug, n.Type))
			continue
		}
		slug := cleanSlug(n.Slug)
		if slug == "" || used[slug] {
			slug = fmt.Sprintf("node-%d", i+1)
		}
		used[slug] = true
		slugs[n.Slug] = slug
		summary := strings.TrimSpace(n.Summary)
		if summary == "" {
			summary = n.Title
		}
		g.Nodes = append(g.Nodes, domain.Node{
			Slug:        slug,
			Type:        t,
			Title:       strings.TrimSpace(n.Title),
			Section:     strings.TrimSpace(d.Section),
			Summary:     summary,
			Formula:     strings.TrimSpace(n.Formula),
			Unit:        strings.TrimSpace(n.Unit),
			Origin:      domain.Origin(n.Origin),
			SourceQuote: strings.TrimSpace(n.Quote),
			SourcePage:  n.Page,
			Note:        strings.TrimSpace(n.Note),
		})
		d.Nodes[i].Slug = slug
	}
	type key struct{ from, to string }
	seen := map[key]bool{}
	for _, e := range d.Edges {
		from, to := slugs[e.From], slugs[e.To]
		et := domain.EdgeType(strings.ToLower(strings.TrimSpace(e.Type)))
		if from == "" || to == "" || from == to || !et.Valid() || seen[key{from, to}] {
			continue
		}
		seen[key{from, to}] = true
		desc := strings.TrimSpace(e.Description)
		if desc == "" {
			desc = strings.TrimSpace(e.Note)
		}
		g.Edges = append(g.Edges, domain.Edge{From: from, To: to, Type: et, Description: desc, Origin: domain.Origin(e.Origin)})
	}
	return g
}

func cleanSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		default:
			sb.WriteRune('-')
		}
	}
	out := strings.Trim(regexp.MustCompile(`-+`).ReplaceAllString(sb.String(), "-"), "-")
	if !slugRe.MatchString(out) {
		return ""
	}
	return out
}

// enforce делает граф валидным, удаляя недопустимые связи и разрывая циклы.
// При разрыве цикла сначала удаляются связи, дополненные моделью.
func enforce(g domain.Graph, res *Result) domain.Graph {
	types := map[string]domain.NodeType{}
	for _, n := range g.Nodes {
		types[n.Slug] = n.Type
	}
	kept := g.Edges[:0:0]
	for _, e := range g.Edges {
		if content.EdgeAllowed(e.Type, types[e.From], types[e.To]) {
			kept = append(kept, e)
			continue
		}
		// Частая ошибка — связь в обратную сторону.
		if content.EdgeAllowed(e.Type, types[e.To], types[e.From]) {
			e.From, e.To = e.To, e.From
			kept = append(kept, e)
			res.Fixes = append(res.Fixes, fmt.Sprintf("связь %s→%s развёрнута", e.To, e.From))
			continue
		}
		res.Fixes = append(res.Fixes, fmt.Sprintf("связь %s -[%s]-> %s удалена: недопустима", e.From, e.Type, e.To))
	}
	g.Edges = kept
	for {
		cycle := content.FindCycle(g)
		if cycle == nil {
			break
		}
		drop := -1
		for i, e := range g.Edges {
			if !onCycle(cycle, e) {
				continue
			}
			if drop < 0 || (e.Origin == domain.OriginInferred && g.Edges[drop].Origin != domain.OriginInferred) {
				drop = i
			}
		}
		res.Fixes = append(res.Fixes, fmt.Sprintf("связь %s→%s удалена: цикл", g.Edges[drop].From, g.Edges[drop].To))
		g.Edges = append(g.Edges[:drop:drop], g.Edges[drop+1:]...)
	}
	// Повторяющиеся связи между той же парой после разворота.
	type key struct{ from, to string }
	seen := map[key]bool{}
	uniq := g.Edges[:0:0]
	for _, e := range g.Edges {
		if !seen[key{e.From, e.To}] {
			seen[key{e.From, e.To}] = true
			uniq = append(uniq, e)
		}
	}
	g.Edges = uniq
	return g
}

func onCycle(cycle []string, e domain.Edge) bool {
	for i := 0; i+1 < len(cycle); i++ {
		if cycle[i] == e.From && cycle[i+1] == e.To {
			return true
		}
	}
	return false
}

// checkPhysics отмечает узлы с вероятными ошибками в формулах. Сбой проверки
// не ломает построение графа: граф полезен и без неё.
func (b *Builder) checkPhysics(ctx context.Context, g *domain.Graph, res *Result) {
	var sb strings.Builder
	for _, n := range g.Nodes {
		if n.Formula == "" || n.Type == domain.NodeQuantity {
			continue
		}
		fmt.Fprintf(&sb, "- %s (%s): %s — %s\n", n.Slug, n.Title, n.Formula, n.Summary)
	}
	if sb.Len() == 0 {
		return
	}
	var out struct {
		Issues []struct {
			Slug  string `json:"slug"`
			Issue string `json:"issue"`
		} `json:"issues"`
	}
	if err := b.ask(ctx, res, "check", checkPrompt, "Формулы из конспекта:\n"+sb.String(), nil, &out); err != nil {
		res.Fixes = append(res.Fixes, "проверка формул не удалась: "+err.Error())
		return
	}
	for _, is := range out.Issues {
		for i := range g.Nodes {
			if g.Nodes[i].Slug == is.Slug && strings.TrimSpace(is.Issue) != "" {
				g.Nodes[i].Warning = strings.TrimSpace(is.Issue)
			}
		}
	}
}

func plural(n int, one, few, many string) string {
	m := n % 100
	if m >= 11 && m <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	}
	return many
}

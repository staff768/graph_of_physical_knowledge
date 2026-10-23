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

	all, err := s.ListGraphs(ctx, "", "")
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
		res, err := s.ListGraphs(ctx, "", q)
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

func TestUploadJobLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.pool.Exec(ctx, `DELETE FROM graphs WHERE id LIKE 'test-job-%'`); err != nil {
		t.Fatal(err)
	}
	// Другие незавершённые задания из прошлых прогонов не должны мешать.
	if _, err := s.pool.Exec(ctx, `UPDATE graphs SET status = 'failed' WHERE status = 'processing'`); err != nil {
		t.Fatal(err)
	}
	meta := domain.GraphMeta{ID: "test-job-1", Title: "lecture.pdf", Source: "lecture.pdf"}
	up := []File{{Filename: "page1.jpg", MIME: "image/jpeg", Data: []byte("page-1")}, {Filename: "page2.jpg", MIME: "image/jpeg", Data: []byte("page-2")}}
	if err := s.CreateFromUpload(ctx, meta, "hash-1", up); err != nil {
		t.Fatal(err)
	}
	if id, err := s.GraphBySourceHash(ctx, "", "hash-1"); err != nil || id != "test-job-1" {
		t.Errorf("by hash = %q, %v", id, err)
	}

	job, err := s.ClaimJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Файлы возвращаются в порядке страниц.
	if job.GraphID != "test-job-1" || len(job.Files) != 2 || string(job.Files[0].Data) != "page-1" || job.Files[1].Filename != "page2.jpg" {
		t.Errorf("job = %+v", job)
	}
	// Взятое задание повторно не выдаётся.
	if _, err := s.ClaimJob(ctx); !errors.Is(err, ErrNotFound) {
		t.Errorf("second claim err = %v, want ErrNotFound", err)
	}
	if err := s.SetStage(ctx, "test-job-1", "extracting", "найдено 3 понятия"); err != nil {
		t.Fatal(err)
	}
	m, err := s.GraphMeta(ctx, "test-job-1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != domain.StatusProcessing || m.Stage != "extracting" || m.StageNote != "найдено 3 понятия" {
		t.Errorf("meta = %+v", m)
	}

	g := domain.Graph{
		Nodes: []domain.Node{
			{Slug: "a", Type: domain.NodeLaw, Title: "Закон", Origin: domain.OriginExplicit, SourceQuote: "цитата", SourcePage: 2},
			{Slug: "b", Type: domain.NodeFormula, Title: "Формула", Origin: domain.OriginInferred, Note: "почему", Warning: "ошибка?"},
		},
		Edges: []domain.Edge{{From: "a", To: "b", Type: domain.EdgeFormulates, Origin: domain.OriginInferred}},
	}
	if err := s.CompleteGraph(ctx, "test-job-1", "Механика", "Механика", 8, g); err != nil {
		t.Fatal(err)
	}
	m, _ = s.GraphMeta(ctx, "test-job-1")
	if m.Status != domain.StatusReady || m.Title != "Механика" || m.Pages != 8 || m.NodeCount != 2 {
		t.Errorf("meta after complete = %+v", m)
	}
	var uploads int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM uploads WHERE graph_id = 'test-job-1'`).Scan(&uploads)
	if uploads != 0 {
		t.Error("upload should be deleted after completion")
	}
	saved, err := s.Graph(ctx, "test-job-1")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Nodes[0].SourceQuote != "цитата" || saved.Nodes[0].SourcePage != 2 || saved.Nodes[1].Warning != "ошибка?" ||
		saved.Edges[0].Origin != domain.OriginInferred {
		t.Errorf("saved graph lost fields: %+v", saved)
	}

	// Неудачная обработка: статус failed, файл удалён, по хэшу граф больше не находится.
	if err := s.CreateFromUpload(ctx, domain.GraphMeta{ID: "test-job-2", Title: "x"}, "hash-2", up); err != nil {
		t.Fatal(err)
	}
	if err := s.FailGraph(ctx, "test-job-2", "В файле нет физики"); err != nil {
		t.Fatal(err)
	}
	m, _ = s.GraphMeta(ctx, "test-job-2")
	if m.Status != domain.StatusFailed || m.Error != "В файле нет физики" {
		t.Errorf("failed meta = %+v", m)
	}
	if _, err := s.GraphBySourceHash(ctx, "", "hash-2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("failed graph found by hash: %v", err)
	}
}

func TestClaimJobWithoutUpload(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.pool.Exec(ctx, `UPDATE graphs SET status = 'failed' WHERE status = 'processing'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM graphs WHERE id = 'test-job-orphan'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO graphs (id, title, status) VALUES ('test-job-orphan', 'x', 'processing')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimJob(ctx); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	m, err := s.GraphMeta(ctx, "test-job-orphan")
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != domain.StatusFailed {
		t.Errorf("status = %s, want failed", m.Status)
	}
}

func TestDeleteGraph(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.DeleteGraph(ctx, "test-b", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GraphMeta(ctx, "test-b"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: %v", err)
	}
	if err := s.DeleteGraph(ctx, "test-b", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	if err := s.ReplaceGraph(ctx, domain.GraphMeta{ID: "test-demo", Title: "Демо", Demo: true}, domain.Graph{}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteGraph(ctx, "test-demo", ""); !errors.Is(err, ErrDemoGraph) {
		t.Errorf("demo delete: %v", err)
	}
	// Граф test-a не пострадал.
	if g, _ := s.Graph(ctx, "test-a"); len(g.Nodes) != 5 {
		t.Errorf("test-a nodes = %d", len(g.Nodes))
	}
}

func TestStories(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.Story(ctx, "test-a", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing story: %v", err)
	}
	if err := s.SaveStory(ctx, "test-a", "x", "первый", "m"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveStory(ctx, "test-a", "x", "второй", "m"); err != nil {
		t.Fatal(err)
	}
	if text, err := s.Story(ctx, "test-a", "x"); err != nil || text != "второй" {
		t.Errorf("story = %q, %v", text, err)
	}
	// Повторная загрузка того же содержимого (seed) рассказы не трогает.
	g, _ := s.Graph(ctx, "test-a")
	if err := s.ReplaceGraph(ctx, domain.GraphMeta{ID: "test-a", Title: "A"}, g); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGraph(ctx, domain.GraphMeta{ID: "test-a", Title: "A"}, g); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Story(ctx, "test-a", "x"); err != nil {
		t.Errorf("story after same-content replace: %v", err)
	}
	// Изменённый граф — рассказы сбрасываются.
	g.Nodes[0].Summary = "новое описание"
	if err := s.ReplaceGraph(ctx, domain.GraphMeta{ID: "test-a", Title: "A"}, g); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Story(ctx, "test-a", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("story after replace: %v", err)
	}
}

func TestOwnerSpaces(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.pool.Exec(ctx, `DELETE FROM graphs WHERE id LIKE 'test-own-%'`); err != nil {
		t.Fatal(err)
	}
	files := []File{{Filename: "a.txt", MIME: "text/plain", Data: []byte("x")}}
	for _, g := range []struct{ id, owner string }{{"test-own-alice", "alice"}, {"test-own-bob", "bob"}} {
		if err := s.CreateFromUpload(ctx, domain.GraphMeta{ID: g.id, Title: g.id, Owner: g.owner}, "same-hash", files); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(owner string) map[string]bool {
		list, err := s.ListGraphs(ctx, owner, "")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, m := range list {
			out[m.ID] = true
		}
		return out
	}
	if a := ids("alice"); !a["test-own-alice"] || a["test-own-bob"] || !a["test-a"] {
		t.Errorf("alice sees %v: want own graph and ownerless test-a, not bob's", a)
	}
	// Тот же файл у другого владельца — свой граф, а не чужой.
	if id, err := s.GraphBySourceHash(ctx, "bob", "same-hash"); err != nil || id != "test-own-bob" {
		t.Errorf("bob's hash lookup = %q, %v", id, err)
	}
	if _, err := s.GraphBySourceHash(ctx, "carol", "same-hash"); !errors.Is(err, ErrNotFound) {
		t.Errorf("carol's hash lookup: %v", err)
	}
	if err := s.DeleteGraph(ctx, "test-own-bob", "alice"); !errors.Is(err, ErrNotOwner) {
		t.Errorf("alice deletes bob's graph: %v", err)
	}
	if m, _ := s.GraphMeta(ctx, "test-own-bob"); m.Owner != "bob" {
		t.Errorf("owner = %q", m.Owner)
	}
	if err := s.DeleteGraph(ctx, "test-own-bob", "bob"); err != nil {
		t.Errorf("bob deletes own graph: %v", err)
	}
	if st, err := s.OwnerStats(ctx, "alice"); err != nil || st.Graphs != 0 {
		// Граф Алисы ещё строится — в сводку попадают только готовые.
		t.Errorf("stats = %+v, %v", st, err)
	}
	if err := s.CompleteGraph(ctx, "test-own-alice", "Ток", "Электродинамика", 1, domain.Graph{
		Nodes: []domain.Node{{Slug: "a", Type: domain.NodeLaw, Title: "A"}, {Slug: "b", Type: domain.NodeFormula, Title: "B"}},
		Edges: []domain.Edge{{From: "a", To: "b", Type: domain.EdgeFormulates}},
	}); err != nil {
		t.Fatal(err)
	}
	st, err := s.OwnerStats(ctx, "alice")
	if err != nil || st.Graphs != 1 || st.Concepts != 2 || st.Links != 1 || len(st.Sections) != 1 || st.Sections[0].Section != "Электродинамика" {
		t.Errorf("stats = %+v, %v", st, err)
	}
}

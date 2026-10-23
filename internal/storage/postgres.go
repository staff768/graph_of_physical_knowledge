// Package storage реализует хранение графа в PostgreSQL.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/migrations"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Migrate применяет ещё не применённые SQL-миграции в лексикографическом порядке.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var applied bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		sqlText, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return err
		}
		err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sqlText)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// ReplaceGraph атомарно создаёт граф или полностью заменяет его содержимое.
// Другие графы не затрагиваются.
func (s *Store) ReplaceGraph(ctx context.Context, meta domain.GraphMeta, g domain.Graph) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO graphs (id, title, section, source, is_demo) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE
			SET title = EXCLUDED.title, section = EXCLUDED.section, source = EXCLUDED.source, is_demo = EXCLUDED.is_demo`,
			meta.ID, meta.Title, meta.Section, meta.Source, meta.Demo)
		if err != nil {
			return fmt.Errorf("upsert graph %q: %w", meta.ID, err)
		}
		// Связи удалятся каскадно вместе с узлами.
		if _, err := tx.Exec(ctx, `DELETE FROM nodes WHERE graph_id = $1`, meta.ID); err != nil {
			return err
		}
		ids := make(map[string]int64, len(g.Nodes))
		for _, n := range g.Nodes {
			var id int64
			err := tx.QueryRow(ctx, `
				INSERT INTO nodes (graph_id, slug, type, title, section, summary, formula, unit, body, example)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				RETURNING id`,
				meta.ID, n.Slug, n.Type, n.Title, n.Section, n.Summary, n.Formula, n.Unit, n.Body, n.Example,
			).Scan(&id)
			if err != nil {
				return fmt.Errorf("insert node %q: %w", n.Slug, err)
			}
			ids[n.Slug] = id
		}
		batch := &pgx.Batch{}
		for _, e := range g.Edges {
			batch.Queue(`INSERT INTO edges (graph_id, from_id, to_id, type, description) VALUES ($1, $2, $3, $4, $5)`,
				meta.ID, ids[e.From], ids[e.To], e.Type, e.Description)
		}
		return tx.SendBatch(ctx, batch).Close()
	})
}

// previewSize — сколько понятий показывать в превью графа.
const previewSize = 4

// ListGraphs возвращает графы, начиная с самых новых. Если query не пуст,
// остаются графы, в названии или понятиях которых встречается query.
func (s *Store) ListGraphs(ctx context.Context, query string) ([]domain.GraphMeta, error) {
	pattern := "%" + escapeLike(strings.ToLower(strings.TrimSpace(query))) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.title, g.section, g.source, g.is_demo, g.created_at,
			(SELECT count(*) FROM nodes n WHERE n.graph_id = g.id),
			(SELECT count(*) FROM edges e WHERE e.graph_id = g.id),
			ARRAY(
				SELECT n.title FROM nodes n
				LEFT JOIN edges e ON e.from_id = n.id OR e.to_id = n.id
				WHERE n.graph_id = g.id
				GROUP BY n.id
				ORDER BY count(e.id) DESC, n.id
				LIMIT $2
			)
		FROM graphs g
		WHERE $1 = '%%'
			OR lower(g.title) LIKE $1
			OR EXISTS (SELECT 1 FROM nodes n WHERE n.graph_id = g.id AND lower(n.title) LIKE $1)
		ORDER BY g.created_at DESC, g.id`, pattern, previewSize)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.GraphMeta, error) {
		var m domain.GraphMeta
		err := r.Scan(&m.ID, &m.Title, &m.Section, &m.Source, &m.Demo, &m.CreatedAt, &m.NodeCount, &m.EdgeCount, &m.Preview)
		return m, err
	})
}

// GraphMeta возвращает сведения об одном графе.
func (s *Store) GraphMeta(ctx context.Context, graphID string) (domain.GraphMeta, error) {
	var m domain.GraphMeta
	err := s.pool.QueryRow(ctx, `
		SELECT g.id, g.title, g.section, g.source, g.is_demo, g.created_at,
			(SELECT count(*) FROM nodes n WHERE n.graph_id = g.id),
			(SELECT count(*) FROM edges e WHERE e.graph_id = g.id)
		FROM graphs g WHERE g.id = $1`, graphID,
	).Scan(&m.ID, &m.Title, &m.Section, &m.Source, &m.Demo, &m.CreatedAt, &m.NodeCount, &m.EdgeCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GraphMeta{}, ErrNotFound
	}
	return m, err
}

const nodeColumns = `n.slug, n.type, n.title, n.section, n.summary, n.formula, n.unit, n.body, n.example`

const edgeSelect = `
	SELECT f.slug, t.slug, e.type, e.description
	FROM edges e
	JOIN nodes f ON f.id = e.from_id
	JOIN nodes t ON t.id = e.to_id`

func scanNodes(rows pgx.Rows) ([]domain.Node, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Node, error) {
		var n domain.Node
		err := r.Scan(&n.Slug, &n.Type, &n.Title, &n.Section, &n.Summary, &n.Formula, &n.Unit, &n.Body, &n.Example)
		return n, err
	})
}

func scanEdges(rows pgx.Rows) ([]domain.Edge, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Edge, error) {
		var e domain.Edge
		err := r.Scan(&e.From, &e.To, &e.Type, &e.Description)
		return e, err
	})
}

// Graph возвращает все узлы и связи графа.
func (s *Store) Graph(ctx context.Context, graphID string) (domain.Graph, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+nodeColumns+` FROM nodes n WHERE n.graph_id = $1 ORDER BY n.id`, graphID)
	if err != nil {
		return domain.Graph{}, err
	}
	nodes, err := scanNodes(rows)
	if err != nil {
		return domain.Graph{}, err
	}
	rows, err = s.pool.Query(ctx, edgeSelect+` WHERE e.graph_id = $1 ORDER BY e.id`, graphID)
	if err != nil {
		return domain.Graph{}, err
	}
	edges, err := scanEdges(rows)
	if err != nil {
		return domain.Graph{}, err
	}
	return domain.Graph{Nodes: nodes, Edges: edges}, nil
}

// Node возвращает узел графа по slug.
func (s *Store) Node(ctx context.Context, graphID, slug string) (domain.Node, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+nodeColumns+` FROM nodes n WHERE n.graph_id = $1 AND n.slug = $2`, graphID, slug)
	if err != nil {
		return domain.Node{}, err
	}
	nodes, err := scanNodes(rows)
	if err != nil {
		return domain.Node{}, err
	}
	if len(nodes) == 0 {
		return domain.Node{}, ErrNotFound
	}
	return nodes[0], nil
}

// Neighbors возвращает входящие (основания) и исходящие (следствия) связи узла.
func (s *Store) Neighbors(ctx context.Context, graphID, slug string) (incoming, outgoing []domain.Link, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT true, f.slug, f.title, f.type, e.type, e.description
		FROM edges e JOIN nodes f ON f.id = e.from_id JOIN nodes t ON t.id = e.to_id
		WHERE t.graph_id = $1 AND t.slug = $2
		UNION ALL
		SELECT false, t.slug, t.title, t.type, e.type, e.description
		FROM edges e JOIN nodes f ON f.id = e.from_id JOIN nodes t ON t.id = e.to_id
		WHERE f.graph_id = $1 AND f.slug = $2`, graphID, slug)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	incoming, outgoing = []domain.Link{}, []domain.Link{}
	for rows.Next() {
		var in bool
		var l domain.Link
		if err := rows.Scan(&in, &l.Slug, &l.Title, &l.NodeType, &l.EdgeType, &l.Description); err != nil {
			return nil, nil, err
		}
		if in {
			incoming = append(incoming, l)
		} else {
			outgoing = append(outgoing, l)
		}
	}
	return incoming, outgoing, rows.Err()
}

// Ancestors возвращает подграф, состоящий из узла и всех его предков
// (узлов, из которых он достижим), вместе с рёбрами между ними.
// Связи существуют только внутри графа, поэтому обход не выходит за его пределы.
func (s *Store) Ancestors(ctx context.Context, graphID, slug string) (domain.Graph, error) {
	const ancestorsCTE = `
		WITH RECURSIVE anc(id) AS (
			SELECT id FROM nodes WHERE graph_id = $1 AND slug = $2
			UNION
			SELECT e.from_id FROM edges e JOIN anc ON e.to_id = anc.id
		)`
	rows, err := s.pool.Query(ctx, ancestorsCTE+`
		SELECT `+nodeColumns+` FROM nodes n JOIN anc ON anc.id = n.id ORDER BY n.id`, graphID, slug)
	if err != nil {
		return domain.Graph{}, err
	}
	nodes, err := scanNodes(rows)
	if err != nil {
		return domain.Graph{}, err
	}
	if len(nodes) == 0 {
		return domain.Graph{}, ErrNotFound
	}
	rows, err = s.pool.Query(ctx, ancestorsCTE+edgeSelect+`
		WHERE e.from_id IN (SELECT id FROM anc) AND e.to_id IN (SELECT id FROM anc)
		ORDER BY e.id`, graphID, slug)
	if err != nil {
		return domain.Graph{}, err
	}
	edges, err := scanEdges(rows)
	if err != nil {
		return domain.Graph{}, err
	}
	return domain.Graph{Nodes: nodes, Edges: edges}, nil
}

// Search ищет узлы графа по подстроке в названии или кратком описании.
// Совпадения в названии идут первыми.
func (s *Store) Search(ctx context.Context, graphID, query string, limit int) ([]domain.Node, error) {
	pattern := "%" + escapeLike(strings.ToLower(query)) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT `+nodeColumns+` FROM nodes n
		WHERE n.graph_id = $1 AND (lower(n.title) LIKE $2 OR lower(n.summary) LIKE $2 OR lower(n.slug) LIKE $2)
		ORDER BY (lower(n.title) LIKE $2) DESC, length(n.title), n.title
		LIMIT $3`, graphID, pattern, limit)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

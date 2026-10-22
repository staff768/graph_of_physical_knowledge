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

// ReplaceGraph атомарно заменяет всё содержимое графа.
func (s *Store) ReplaceGraph(ctx context.Context, g domain.Graph) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `TRUNCATE edges, nodes RESTART IDENTITY`); err != nil {
			return err
		}
		ids := make(map[string]int64, len(g.Nodes))
		for _, n := range g.Nodes {
			var id int64
			err := tx.QueryRow(ctx, `
				INSERT INTO nodes (slug, type, title, section, summary, formula, unit, body, example)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				RETURNING id`,
				n.Slug, n.Type, n.Title, n.Section, n.Summary, n.Formula, n.Unit, n.Body, n.Example,
			).Scan(&id)
			if err != nil {
				return fmt.Errorf("insert node %q: %w", n.Slug, err)
			}
			ids[n.Slug] = id
		}
		batch := &pgx.Batch{}
		for _, e := range g.Edges {
			batch.Queue(`INSERT INTO edges (from_id, to_id, type, description) VALUES ($1, $2, $3, $4)`,
				ids[e.From], ids[e.To], e.Type, e.Description)
		}
		return tx.SendBatch(ctx, batch).Close()
	})
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

// Graph возвращает весь граф.
func (s *Store) Graph(ctx context.Context) (domain.Graph, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+nodeColumns+` FROM nodes n ORDER BY n.id`)
	if err != nil {
		return domain.Graph{}, err
	}
	nodes, err := scanNodes(rows)
	if err != nil {
		return domain.Graph{}, err
	}
	rows, err = s.pool.Query(ctx, edgeSelect+` ORDER BY e.id`)
	if err != nil {
		return domain.Graph{}, err
	}
	edges, err := scanEdges(rows)
	if err != nil {
		return domain.Graph{}, err
	}
	return domain.Graph{Nodes: nodes, Edges: edges}, nil
}

// Node возвращает узел по slug.
func (s *Store) Node(ctx context.Context, slug string) (domain.Node, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+nodeColumns+` FROM nodes n WHERE n.slug = $1`, slug)
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
func (s *Store) Neighbors(ctx context.Context, slug string) (incoming, outgoing []domain.Link, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT true, f.slug, f.title, f.type, e.type, e.description
		FROM edges e JOIN nodes f ON f.id = e.from_id JOIN nodes t ON t.id = e.to_id
		WHERE t.slug = $1
		UNION ALL
		SELECT false, t.slug, t.title, t.type, e.type, e.description
		FROM edges e JOIN nodes f ON f.id = e.from_id JOIN nodes t ON t.id = e.to_id
		WHERE f.slug = $1`, slug)
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
func (s *Store) Ancestors(ctx context.Context, slug string) (domain.Graph, error) {
	const ancestorsCTE = `
		WITH RECURSIVE anc(id) AS (
			SELECT id FROM nodes WHERE slug = $1
			UNION
			SELECT e.from_id FROM edges e JOIN anc ON e.to_id = anc.id
		)`
	rows, err := s.pool.Query(ctx, ancestorsCTE+`
		SELECT `+nodeColumns+` FROM nodes n JOIN anc ON anc.id = n.id ORDER BY n.id`, slug)
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
		ORDER BY e.id`, slug)
	if err != nil {
		return domain.Graph{}, err
	}
	edges, err := scanEdges(rows)
	if err != nil {
		return domain.Graph{}, err
	}
	return domain.Graph{Nodes: nodes, Edges: edges}, nil
}

// Search ищет узлы по подстроке в названии или кратком описании.
// Совпадения в названии идут первыми.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]domain.Node, error) {
	pattern := "%" + escapeLike(strings.ToLower(query)) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT `+nodeColumns+` FROM nodes n
		WHERE lower(n.title) LIKE $1 OR lower(n.summary) LIKE $1 OR lower(n.slug) LIKE $1
		ORDER BY (lower(n.title) LIKE $1) DESC, length(n.title), n.title
		LIMIT $2`, pattern, limit)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

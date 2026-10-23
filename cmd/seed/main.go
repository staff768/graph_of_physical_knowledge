// Команда seed валидирует контент из YAML и создаёт или полностью заменяет
// этот граф в БД (остальные графы не затрагиваются).
//
//	go run ./cmd/seed               # загрузить content/*.yaml в БД
//	go run ./cmd/seed -check        # только проверить контент
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"graph_of_physical_knowledge/internal/content"
	"graph_of_physical_knowledge/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "content", "directory with *.yaml content files")
	check := flag.Bool("check", false, "only validate content, do not touch the database")
	flag.Parse()

	b, err := content.LoadDir(os.DirFS(*dir), ".")
	if err != nil {
		return err
	}
	fmt.Printf("content OK: graph %q, %d nodes, %d edges\n", b.Meta.ID, len(b.Graph.Nodes), len(b.Graph.Edges))
	if *check {
		return nil
	}

	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://physics:physics@localhost:5432/physics?sslmode=disable"
	}
	store, err := storage.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}
	if err := store.ReplaceGraph(ctx, b.Meta, b.Graph); err != nil {
		return err
	}
	fmt.Printf("graph %q seeded\n", b.Meta.ID)
	return nil
}

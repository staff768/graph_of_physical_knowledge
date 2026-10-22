// Команда seed валидирует контент из YAML и полностью заменяет им граф в БД.
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

	g, err := content.LoadDir(os.DirFS(*dir), ".")
	if err != nil {
		return err
	}
	fmt.Printf("content OK: %d nodes, %d edges\n", len(g.Nodes), len(g.Edges))
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
	if err := store.ReplaceGraph(ctx, g); err != nil {
		return err
	}
	fmt.Println("database seeded")
	return nil
}

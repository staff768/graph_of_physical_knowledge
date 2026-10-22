// Команда api запускает HTTP-сервер платформы.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"graph_of_physical_knowledge/internal/httpapi"
	"graph_of_physical_knowledge/internal/storage"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(ctx, env("DATABASE_URL", "postgres://physics:physics@localhost:5432/physics?sslmode=disable"))
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}

	var static fs.FS
	staticDir := env("STATIC_DIR", "web/dist")
	if _, err := os.Stat(staticDir + "/index.html"); err == nil {
		static = os.DirFS(staticDir)
		log.Info("serving frontend", "dir", staticDir)
	}

	srv := &http.Server{
		Addr:              env("HTTP_ADDR", ":8080"),
		Handler:           httpapi.New(store, log, static),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

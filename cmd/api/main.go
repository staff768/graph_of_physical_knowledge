// Команда api запускает HTTP-сервер платформы.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"graph_of_physical_knowledge/internal/builder"
	"graph_of_physical_knowledge/internal/httpapi"
	"graph_of_physical_knowledge/internal/llm"
	"graph_of_physical_knowledge/internal/storage"
	"graph_of_physical_knowledge/internal/story"
	"graph_of_physical_knowledge/internal/worker"
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
	// Переменные из .env (если файл есть); уже заданные в окружении не перезаписываются.
	_ = godotenv.Load()

	store, err := storage.New(ctx, env("DATABASE_URL", "postgres://physics:physics@localhost:5432/physics?sslmode=disable"))
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}

	uploads, stories, err := startWorker(ctx, store, log)
	if err != nil {
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
		Handler:           httpapi.New(store, log, static, uploads, stories),
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

// startWorker запускает построение графов по конспектам, если настроена языковая модель.
// и рассказы по цепочкам. Без модели приложение работает, но обе возможности выключены.
func startWorker(ctx context.Context, store *storage.Store, log *slog.Logger) (*httpapi.Uploads, *httpapi.Stories, error) {
	client, err := llm.FromEnv()
	if errors.Is(err, llm.ErrNotConfigured) {
		log.Warn("загрузка конспектов и рассказы выключены: языковая модель не настроена (см. .env.example)", "reason", err)
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	b := &builder.Builder{LLM: client, Log: log, CheckPhysics: env("LLM_CHECK_PHYSICS", "true") == "true"}
	w := worker.New(store, b, log)
	go w.Run(ctx)
	log.Info("построение графов включено", "model", client.Name())
	perHour, err := strconv.Atoi(env("UPLOADS_PER_HOUR", "10"))
	if err != nil {
		return nil, nil, fmt.Errorf("UPLOADS_PER_HOUR: %w", err)
	}
	storiesPerHour, err := strconv.Atoi(env("STORIES_PER_HOUR", "60"))
	if err != nil {
		return nil, nil, fmt.Errorf("STORIES_PER_HOUR: %w", err)
	}
	uploads := &httpapi.Uploads{Store: store, Notify: w.Notify, PerHour: perHour}
	stories := &httpapi.Stories{Store: store, Teller: &story.Teller{LLM: client}, Model: client.Name(), PerHour: storiesPerHour}
	return uploads, stories, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

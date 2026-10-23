// Package worker в фоне строит графы по загруженным конспектам.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"graph_of_physical_knowledge/internal/builder"
	"graph_of_physical_knowledge/internal/document"
	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/llm"
	"graph_of_physical_knowledge/internal/storage"
)

// Store — то, что воркеру нужно от хранилища.
type Store interface {
	ClaimJob(ctx context.Context) (storage.Upload, error)
	SetStage(ctx context.Context, graphID, stage, note string) error
	CompleteGraph(ctx context.Context, graphID, title, section string, pages int, g domain.Graph) error
	FailGraph(ctx context.Context, graphID, reason string) error
}

type Worker struct {
	Store   Store
	Builder *builder.Builder
	Log     *slog.Logger
	// Poll — как часто проверять очередь, если никто не разбудил через Notify.
	Poll time.Duration
	// JobTimeout — предел времени на один конспект.
	JobTimeout time.Duration

	wake chan struct{}
}

func New(store Store, b *builder.Builder, log *slog.Logger) *Worker {
	return &Worker{Store: store, Builder: b, Log: log, Poll: 5 * time.Second, JobTimeout: 10 * time.Minute, wake: make(chan struct{}, 1)}
}

// Notify будит воркер сразу после загрузки нового конспекта.
func (w *Worker) Notify() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run обрабатывает очередь, пока не отменён ctx.
func (w *Worker) Run(ctx context.Context) {
	for {
		up, err := w.Store.ClaimJob(ctx)
		switch {
		case err == nil:
			w.process(ctx, up)
			continue
		case errors.Is(err, storage.ErrNotFound):
		case ctx.Err() != nil:
			return
		default:
			w.Log.Error("claim job", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-time.After(w.Poll):
		}
	}
}

// Сообщения об ошибках для ученика.
const (
	msgNoText      = "В файле не нашлось текста. Если это скан или фото, загрузите страницы как изображения JPG или PNG."
	msgUnsupported = "Этот формат не поддерживается. Подойдут PDF, DOCX, TXT, JPG и PNG."
	msgNoPhysics   = "В этом файле не нашлось физики. Похоже, это конспект по другому предмету или пустая страница."
	msgService     = "Сервис распознавания сейчас недоступен. Попробуйте загрузить конспект чуть позже."
	msgInternal    = "Не удалось построить граф. Попробуйте загрузить конспект ещё раз."
)

func (w *Worker) process(ctx context.Context, up storage.Upload) {
	first := ""
	if len(up.Files) > 0 {
		first = up.Files[0].Filename
	}
	log := w.Log.With("graph", up.GraphID, "file", first, "files", len(up.Files))
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, w.JobTimeout)
	defer cancel()
	start := time.Now()

	fail := func(reason string, err error) {
		if parent.Err() != nil {
			// Сервер останавливается: задание не провалено, его возьмут снова после перезапуска.
			log.Info("graph build interrupted by shutdown")
			return
		}
		log.Warn("graph build failed", "reason", reason, "err", err, "duration", time.Since(start).Round(time.Second))
		// Ошибку записываем даже после отмены ctx: иначе граф навсегда останется «в обработке».
		if ferr := w.Store.FailGraph(context.WithoutCancel(ctx), up.GraphID, reason); ferr != nil {
			log.Error("mark failed", "err", ferr)
		}
	}

	_ = w.Store.SetStage(ctx, up.GraphID, builder.StageReading, "")
	inputs := make([]document.Input, len(up.Files))
	for i, f := range up.Files {
		inputs[i] = document.Input{MIME: f.MIME, Data: f.Data}
	}
	doc, err := document.ParseAll(inputs)
	switch {
	case errors.Is(err, document.ErrNoText):
		fail(msgNoText, err)
		return
	case errors.Is(err, document.ErrUnsupported):
		fail(msgUnsupported, err)
		return
	case err != nil:
		fail(msgNoText, err)
		return
	}
	pages := doc.PageCount()
	_ = w.Store.SetStage(ctx, up.GraphID, builder.StageReading, fmt.Sprintf("%d %s", pages, plural(pages, "страница", "страницы", "страниц")))

	res, err := w.Builder.Build(ctx, doc, func(stage, note string) {
		if err := w.Store.SetStage(ctx, up.GraphID, stage, note); err != nil {
			log.Warn("set stage", "err", err)
		}
	})
	var apiErr *llm.APIError
	switch {
	case errors.Is(err, builder.ErrNoPhysics):
		fail(msgNoPhysics, err)
		return
	case errors.As(err, &apiErr), errors.Is(err, context.DeadlineExceeded):
		fail(msgService, err)
		return
	case err != nil:
		fail(msgInternal, err)
		return
	}

	title := res.Title
	if title == "" {
		title = strings.TrimSuffix(first, filepath.Ext(first))
	}
	if err := w.Store.CompleteGraph(ctx, up.GraphID, title, res.Section, pages, res.Graph); err != nil {
		fail(msgInternal, err)
		return
	}
	log.Info("graph built", "nodes", len(res.Graph.Nodes), "edges", len(res.Graph.Edges),
		"tokens_in", res.Usage.InputTokens, "tokens_out", res.Usage.OutputTokens,
		"fixes", len(res.Fixes), "duration", time.Since(start).Round(time.Second))
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

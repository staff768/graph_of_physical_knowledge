package httpapi

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"graph_of_physical_knowledge/internal/explain"
	"graph_of_physical_knowledge/internal/llm"
	"graph_of_physical_knowledge/internal/storage"
)

// StoryStore — кэш рассказов.
type StoryStore interface {
	Story(ctx context.Context, graphID, slug string) (string, error)
	SaveStory(ctx context.Context, graphID, slug, text, model string) error
}

// Teller пишет рассказ по разбору цепочки.
type Teller interface {
	Tell(ctx context.Context, exp explain.Explanation) (string, llm.Usage, error)
}

// Stories включает связные рассказы моделью. Nil — выключены (не настроена модель).
type Stories struct {
	Store  StoryStore
	Teller Teller
	// Model — имя модели, сохраняется рядом с рассказом.
	Model string
	// PerHour — сколько новых рассказов можно заказать с одного IP за час.
	PerHour int

	limiter  *limiter
	mu       sync.Mutex
	inflight map[string]*storyCall
}

// storyCall — рассказ, который уже пишется: повторные запросы ждут его, а не зовут модель снова.
type storyCall struct {
	done chan struct{}
	text string
	err  error
}

type storyResponse struct {
	Text string `json:"text"`
}

const msgStoriesOff = "Рассказы пока не настроены на сервере."

// story отдаёт сохранённый рассказ: 404, если его ещё не писали.
func (h *handler) story(w http.ResponseWriter, r *http.Request) {
	st := h.stories
	if st == nil {
		writeError(w, http.StatusServiceUnavailable, msgStoriesOff)
		return
	}
	text, err := st.Store.Story(r.Context(), graphFrom(r).ID, chi.URLParam(r, "slug"))
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, "story not found")
	case err != nil:
		h.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, storyResponse{Text: text})
	}
}

// tellStory пишет рассказ моделью (или отдаёт уже написанный) и сохраняет его.
func (h *handler) tellStory(w http.ResponseWriter, r *http.Request) {
	st := h.stories
	if st == nil {
		writeError(w, http.StatusServiceUnavailable, msgStoriesOff)
		return
	}
	graphID, slug := graphFrom(r).ID, chi.URLParam(r, "slug")
	if text, err := st.Store.Story(r.Context(), graphID, slug); err == nil {
		writeJSON(w, http.StatusOK, storyResponse{Text: text})
		return
	}
	sub, err := h.store.Ancestors(r.Context(), graphID, slug)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	exp, err := explain.Build(slug, sub)
	if err != nil {
		h.internalError(w, err)
		return
	}

	key := graphID + "\x00" + slug
	st.mu.Lock()
	call, running := st.inflight[key]
	if !running {
		if !st.limiter.allow(clientIP(r), time.Now()) {
			st.mu.Unlock()
			writeError(w, http.StatusTooManyRequests, "Слишком много рассказов за час. Попробуйте позже.")
			return
		}
		call = &storyCall{done: make(chan struct{})}
		st.inflight[key] = call
		go h.writeStory(st, key, graphID, slug, exp, call)
	}
	st.mu.Unlock()

	select {
	case <-call.done:
	case <-r.Context().Done():
		return
	}
	if call.err != nil {
		h.log.Error("story failed", "graph", graphID, "slug", slug, "err", call.err)
		writeError(w, http.StatusBadGateway, "Не получилось написать рассказ. Попробуйте ещё раз.")
		return
	}
	writeJSON(w, http.StatusOK, storyResponse{Text: call.text})
}

// writeStory зовёт модель вне контекста запроса: если школьник закрыл вкладку,
// рассказ всё равно допишется и сохранится.
func (h *handler) writeStory(st *Stories, key, graphID, slug string, exp explain.Explanation, call *storyCall) {
	ctx, cancel := context.WithTimeout(context.Background(), storyTimeout)
	defer cancel()
	start := time.Now()
	text, usage, err := st.Teller.Tell(ctx, exp)
	if err == nil {
		err = st.Store.SaveStory(ctx, graphID, slug, text, st.Model)
		h.log.Info("story written", "graph", graphID, "slug", slug, "in", usage.InputTokens, "out", usage.OutputTokens, "took", time.Since(start).Round(100*time.Millisecond))
	}
	call.text, call.err = text, err
	st.mu.Lock()
	delete(st.inflight, key)
	st.mu.Unlock()
	close(call.done)
}

// storyTimeout — сколько ждать модель.
const storyTimeout = 90 * time.Second

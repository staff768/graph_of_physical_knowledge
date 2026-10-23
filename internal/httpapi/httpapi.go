// Package httpapi реализует REST API платформы.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/explain"
	"graph_of_physical_knowledge/internal/storage"
)

// Store — то, что API требует от хранилища.
type Store interface {
	Ping(ctx context.Context) error
	ListGraphs(ctx context.Context, query string) ([]domain.GraphMeta, error)
	GraphMeta(ctx context.Context, graphID string) (domain.GraphMeta, error)
	Graph(ctx context.Context, graphID string) (domain.Graph, error)
	Node(ctx context.Context, graphID, slug string) (domain.Node, error)
	Neighbors(ctx context.Context, graphID, slug string) (incoming, outgoing []domain.Link, err error)
	Ancestors(ctx context.Context, graphID, slug string) (domain.Graph, error)
	Search(ctx context.Context, graphID, query string, limit int) ([]domain.Node, error)
}

type handler struct {
	store   Store
	log     *slog.Logger
	uploads *Uploads
}

// New собирает роутер. Если static не nil, он раздаётся как SPA:
// неизвестные пути вне /api отдают index.html. uploads == nil выключает загрузку конспектов.
func New(store Store, log *slog.Logger, static fs.FS, uploads *Uploads) http.Handler {
	if uploads != nil {
		uploads.limiter = newLimiter(uploads.PerHour, time.Hour)
	}
	h := &handler{store: store, log: log, uploads: uploads}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(h.logRequests)

	r.Route("/api", func(r chi.Router) {
		// Загрузка файла до 25 МБ может идти дольше обычного запроса.
		r.With(middleware.Timeout(2*time.Minute)).Post("/graphs", h.upload)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(15 * time.Second))
			r.Get("/health", h.health)
			r.Get("/graphs", h.listGraphs)
			r.Route("/graphs/{graphID}", func(r chi.Router) {
				r.Use(h.requireGraph)
				r.Get("/", h.graph)
				r.Get("/search", h.search)
				r.Get("/nodes/{slug}", h.node)
				r.Get("/nodes/{slug}/chain", h.chain)
			})
		})
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not found")
		})
	})
	if static != nil {
		r.Handle("/*", spaHandler(static))
	}
	return r
}

func (h *handler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		if strings.HasPrefix(r.URL.Path, "/api") {
			h.log.Info("request", "method", r.Method, "path", r.URL.Path,
				"status", ww.Status(), "duration", time.Since(start))
		}
	})
}

type graphMetaKey struct{}

// requireGraph отвечает 404, если графа нет, и кладёт его сведения в контекст запроса.
func (h *handler) requireGraph(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		meta, err := h.store.GraphMeta(r.Context(), chi.URLParam(r, "graphID"))
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "graph not found")
			return
		}
		if err != nil {
			h.internalError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), graphMetaKey{}, meta)))
	})
}

func graphFrom(r *http.Request) domain.GraphMeta {
	return r.Context().Value(graphMetaKey{}).(domain.GraphMeta)
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) listGraphs(w http.ResponseWriter, r *http.Request) {
	graphs, err := h.store.ListGraphs(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, graphs)
}

type graphResponse struct {
	Meta domain.GraphMeta `json:"meta"`
	domain.Graph
}

func (h *handler) graph(w http.ResponseWriter, r *http.Request) {
	meta := graphFrom(r)
	g, err := h.store.Graph(r.Context(), meta.ID)
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, graphResponse{Meta: meta, Graph: g})
}

type nodeResponse struct {
	Node     domain.Node   `json:"node"`
	Incoming []domain.Link `json:"incoming"`
	Outgoing []domain.Link `json:"outgoing"`
}

func (h *handler) node(w http.ResponseWriter, r *http.Request) {
	graphID, slug := graphFrom(r).ID, chi.URLParam(r, "slug")
	node, err := h.store.Node(r.Context(), graphID, slug)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	incoming, outgoing, err := h.store.Neighbors(r.Context(), graphID, slug)
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nodeResponse{Node: node, Incoming: incoming, Outgoing: outgoing})
}

func (h *handler) chain(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	sub, err := h.store.Ancestors(r.Context(), graphFrom(r).ID, slug)
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
	writeJSON(w, http.StatusOK, exp)
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []domain.Node{})
		return
	}
	nodes, err := h.store.Search(r.Context(), graphFrom(r).ID, q, 20)
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (h *handler) internalError(w http.ResponseWriter, err error) {
	h.log.Error("internal error", "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func spaHandler(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if _, err := fs.Stat(static, name); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFileFS(w, r, static, "index.html")
	})
}

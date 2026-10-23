package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"graph_of_physical_knowledge/internal/document"
	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/storage"
)

// UploadStore — то, что загрузке нужно от хранилища.
type UploadStore interface {
	CreateFromUpload(ctx context.Context, meta domain.GraphMeta, sourceHash string, files []storage.File) error
	GraphBySourceHash(ctx context.Context, owner, hash string) (string, error)
}

// Uploads включает загрузку конспектов. Nil — загрузка выключена (не настроена модель).
type Uploads struct {
	Store UploadStore
	// Notify будит воркер после загрузки.
	Notify func()
	// PerHour — сколько конспектов можно загрузить с одного IP за час.
	PerHour int

	limiter *limiter
}

type uploadResponse struct {
	ID string `json:"id"`
	// Existing — эти файлы уже загружали, возвращён существующий граф.
	Existing bool `json:"existing,omitempty"`
}

const msgUnsupported = "Этот формат не поддерживается. Подойдут PDF, DOCX, TXT, JPG и PNG."

// upload принимает конспект: один документ (PDF, DOCX, TXT) или до MaxFiles фото страниц
// в полях file — в порядке страниц.
func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	u := h.uploads
	if u == nil {
		writeError(w, http.StatusServiceUnavailable, "Построение графов по конспектам пока не настроено на сервере.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, document.MaxSize+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "Файлы вместе больше 25 МБ.")
			return
		}
		writeError(w, http.StatusBadRequest, "Прикрепите файл конспекта в поле file.")
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	headers := r.MultipartForm.File["file"]
	switch {
	case len(headers) == 0:
		writeError(w, http.StatusBadRequest, "Прикрепите файл конспекта в поле file.")
		return
	case len(headers) > document.MaxFiles:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Можно загрузить не больше %d фото за раз.", document.MaxFiles))
		return
	}

	files := make([]storage.File, 0, len(headers))
	total := 0
	hasher := sha256.New()
	for _, fh := range headers {
		data, err := readPart(fh)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Не удалось прочитать файл.")
			return
		}
		if total += len(data); total > document.MaxSize {
			writeError(w, http.StatusRequestEntityTooLarge, "Файлы вместе больше 25 МБ.")
			return
		}
		mime, err := document.Kind(fh.Filename, data)
		if err != nil {
			writeError(w, http.StatusUnsupportedMediaType, msgUnsupported)
			return
		}
		if len(headers) > 1 && !strings.HasPrefix(mime, "image/") {
			writeError(w, http.StatusUnsupportedMediaType, "Несколько файлов можно загрузить, только если это фото страниц (JPG или PNG). Документ загружайте по одному.")
			return
		}
		files = append(files, storage.File{Filename: filepath.Base(fh.Filename), MIME: mime, Data: data})
		// Хэш учитывает границы файлов, чтобы разные наборы страниц не совпадали случайно.
		fmt.Fprintf(hasher, "%d:", len(data))
		hasher.Write(data)
	}

	// Те же файлы уже обрабатывали — отдаём готовый граф, не тратя запросы к модели.
	hash := hex.EncodeToString(hasher.Sum(nil))
	if id, err := u.Store.GraphBySourceHash(r.Context(), ownerFrom(r), hash); err == nil {
		writeJSON(w, http.StatusOK, uploadResponse{ID: id, Existing: true})
		return
	} else if !errors.Is(err, storage.ErrNotFound) {
		h.internalError(w, err)
		return
	}

	if !u.limiter.allow(clientIP(r), time.Now()) {
		writeError(w, http.StatusTooManyRequests, "Слишком много загрузок подряд. Попробуйте через час.")
		return
	}

	first := files[0].Filename
	source := first
	if len(files) > 1 {
		source = fmt.Sprintf("%s и ещё %d фото", first, len(files)-1)
	}
	meta := domain.GraphMeta{ID: newGraphID(), Title: titleFromFilename(first), Source: source, Owner: ownerFrom(r)}
	if err := u.Store.CreateFromUpload(r.Context(), meta, hash, files); err != nil {
		h.internalError(w, err)
		return
	}
	if u.Notify != nil {
		u.Notify()
	}
	writeJSON(w, http.StatusAccepted, uploadResponse{ID: meta.ID})
}

func readPart(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, document.MaxSize+1))
}

func titleFromFilename(name string) string {
	title := strings.TrimSuffix(name, filepath.Ext(name))
	title = strings.NewReplacer("_", " ").Replace(title)
	if utf8.RuneCountInString(title) > 80 {
		title = string([]rune(title)[:80])
	}
	if strings.TrimSpace(title) == "" {
		return "Конспект"
	}
	return title
}

// newGraphID — случайный id вида «n» + 11 символов [a-z0-9]: подходит под формат slug
// и не угадывается перебором.
func newGraphID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var b [11]byte
	_, _ = rand.Read(b[:])
	out := []byte{'n'}
	for _, c := range b {
		out = append(out, alphabet[int(c)%len(alphabet)])
	}
	return string(out)
}

// clientIP — адрес клиента; middleware.RealIP уже подставил X-Forwarded-For.
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	return host
}

// limiter — скользящее окно в один час на ключ. Хранится в памяти: при нескольких
// экземплярах сервиса лимит считается на каждом отдельно, для MVP этого достаточно.
type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

func (l *limiter) allow(key string, now time.Time) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	// Не даём карте расти бесконечно.
	if len(l.hits) > 10000 {
		for k, ts := range l.hits {
			if len(ts) == 0 || now.Sub(ts[len(ts)-1]) >= l.window {
				delete(l.hits, k)
			}
		}
	}
	return true
}

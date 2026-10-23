package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"graph_of_physical_knowledge/internal/domain"
)

// Личное пространство без регистрации: при первом заходе браузер получает случайный ключ
// в cookie, а графы привязываются к хэшу этого ключа. Сам ключ на сервере не хранится.
const (
	ownerCookie = "fg_owner"
	ownerTTL    = 400 * 24 * time.Hour // столько браузеры разрешают хранить cookie
)

type ownerKey struct{}

// withOwner кладёт в контекст владельца запроса, при необходимости выдавая новый ключ.
func withOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ""
		if c, err := r.Cookie(ownerCookie); err == nil && validOwnerKey(c.Value) {
			key = c.Value
		} else {
			b := make([]byte, 32)
			_, _ = rand.Read(b)
			key = hex.EncodeToString(b)
		}
		// Cookie продлевается при каждом заходе, чтобы активный школьник её не потерял.
		http.SetCookie(w, &http.Cookie{
			Name:     ownerCookie,
			Value:    key,
			Path:     "/",
			MaxAge:   int(ownerTTL.Seconds()),
			HttpOnly: true,
			Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteLaxMode,
		})
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerKey{}, ownerID(key))))
	})
}

func validOwnerKey(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// ownerID — то, что хранится в БД: хэш ключа, чтобы по базе нельзя было войти в чужое пространство.
func ownerID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func ownerFrom(r *http.Request) string {
	id, _ := r.Context().Value(ownerKey{}).(string)
	return id
}

// markMine отмечает графы, которые спрашивающий может удалить: свои и старые без владельца.
func markMine(m *domain.GraphMeta, owner string) {
	m.Mine = !m.Demo && (m.Owner == "" || m.Owner == owner)
}

// me отдаёт сводку личного пространства для профиля.
func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.OwnerStats(r.Context(), ownerFrom(r))
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

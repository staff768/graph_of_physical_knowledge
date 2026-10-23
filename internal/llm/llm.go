// Package llm — минимальный клиент языковых моделей, не привязанный к провайдеру.
package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Image — картинка во входном сообщении (для моделей со зрением).
type Image struct {
	MIME string
	Data []byte
}

type Message struct {
	Role   string // "system", "user" или "assistant"
	Text   string
	Images []Image
}

type Request struct {
	// Task — тип задачи («extract», «repair», «check»): для логов и заглушки.
	Task     string
	Messages []Message
	// JSON просит модель вернуть один JSON-объект.
	JSON        bool
	MaxTokens   int
	Temperature float64
}

type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

type Response struct {
	Text  string
	Usage Usage
}

// Client — языковая модель.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
	// Name — провайдер и модель, для логов и отчётов.
	Name() string
}

// ErrNotConfigured — провайдер не задан в окружении.
var ErrNotConfigured = errors.New("llm provider is not configured")

// FromEnv создаёт клиента по переменным окружения (см. .env.example).
func FromEnv() (Client, error) {
	httpClient := &http.Client{Timeout: 4 * time.Minute}
	switch provider := strings.ToLower(os.Getenv("LLM_PROVIDER")); provider {
	case "":
		return nil, ErrNotConfigured
	case "openai":
		c := &OpenAI{
			BaseURL: os.Getenv("LLM_BASE_URL"),
			APIKey:  os.Getenv("LLM_API_KEY"),
			Model:   os.Getenv("LLM_MODEL"),
			HTTP:    httpClient,
		}
		if c.BaseURL == "" || c.APIKey == "" || c.Model == "" {
			return nil, fmt.Errorf("LLM_BASE_URL, LLM_API_KEY and LLM_MODEL must be set: %w", ErrNotConfigured)
		}
		return c, nil
	case "gigachat":
		return NewGigaChat(GigaChatConfig{
			AuthKey:            os.Getenv("GIGACHAT_AUTH_KEY"),
			Scope:              envOr("GIGACHAT_SCOPE", "GIGACHAT_API_PERS"),
			Model:              envOr("GIGACHAT_MODEL", "GigaChat-2-Max"),
			CAFile:             os.Getenv("GIGACHAT_CA_FILE"),
			InsecureSkipVerify: os.Getenv("GIGACHAT_INSECURE_SKIP_VERIFY") == "true",
		})
	case "stub":
		return Stub{}, nil
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER %q", provider)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// APIError — ответ провайдера с ошибкой.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("llm: HTTP %d: %s", e.Status, e.Body)
}

// Retryable — стоит ли повторить запрос позже (лимиты и сбои провайдера).
func (e *APIError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

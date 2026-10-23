package llm

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// GigaChat — клиент GigaChat API (Сбер).
//
// Сертификаты Сбера выпущены «Russian Trusted Root CA», которого нет в системных
// хранилищах: укажите путь к сертификату в GIGACHAT_CA_FILE (скачивается с
// gosuslugi.ru/crt). GIGACHAT_INSECURE_SKIP_VERIFY=true отключает проверку — только для разработки.
type GigaChat struct {
	cfg  GigaChatConfig
	http *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type GigaChatConfig struct {
	AuthKey            string // Authorization key из личного кабинета (Base64 client_id:secret)
	Scope              string // GIGACHAT_API_PERS для физлиц
	Model              string
	CAFile             string
	InsecureSkipVerify bool

	// Адреса API; переопределяются в тестах.
	OAuthURL string
	APIURL   string
}

func NewGigaChat(cfg GigaChatConfig) (*GigaChat, error) {
	if cfg.AuthKey == "" {
		return nil, fmt.Errorf("GIGACHAT_AUTH_KEY must be set: %w", ErrNotConfigured)
	}
	if cfg.OAuthURL == "" {
		cfg.OAuthURL = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "https://gigachat.devices.sberbank.ru/api/v1"
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.InsecureSkipVerify} //nolint:gosec // явная настройка для разработки
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read GIGACHAT_CA_FILE: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("GIGACHAT_CA_FILE: no PEM certificates found")
		}
		tlsCfg.RootCAs = pool
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsCfg
	return &GigaChat{cfg: cfg, http: &http.Client{Timeout: 4 * time.Minute, Transport: transport}}, nil
}

func (c *GigaChat) Name() string { return "gigachat/" + c.cfg.Model }

// accessToken возвращает действующий токен, обновляя его за минуту до истечения.
func (c *GigaChat) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.expiresAt) > time.Minute {
		return c.token, nil
	}
	form := url.Values{"scope": {c.cfg.Scope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+c.cfg.AuthKey)
	req.Header.Set("RqUID", newUUID())
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"` // миллисекунды Unix
	}
	if err := c.do(req, &out); err != nil {
		return "", fmt.Errorf("gigachat oauth: %w", err)
	}
	c.token, c.expiresAt = out.AccessToken, time.UnixMilli(out.ExpiresAt)
	return c.token, nil
}

// upload загружает картинку в хранилище GigaChat и возвращает её id для attachments.
func (c *GigaChat) upload(ctx context.Context, token string, img Image) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	ext := map[string]string{"image/png": "png", "image/jpeg": "jpg"}[img.MIME]
	part, err := mw.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="page.%s"`, ext)},
		"Content-Type":        {img.MIME},
	})
	if err != nil {
		return "", err
	}
	if _, err := part.Write(img.Data); err != nil {
		return "", err
	}
	if err := mw.WriteField("purpose", "general"); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIURL+"/files", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(req, &out); err != nil {
		return "", fmt.Errorf("gigachat upload: %w", err)
	}
	return out.ID, nil
}

type gcMessage struct {
	Role        string   `json:"role"`
	Content     string   `json:"content"`
	Attachments []string `json:"attachments,omitempty"`
}

func (c *GigaChat) Complete(ctx context.Context, req Request) (Response, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return Response{}, err
	}
	body := struct {
		Model       string      `json:"model"`
		Messages    []gcMessage `json:"messages"`
		MaxTokens   int         `json:"max_tokens,omitempty"`
		Temperature float64     `json:"temperature,omitempty"`
	}{Model: c.cfg.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature}
	for _, m := range req.Messages {
		msg := gcMessage{Role: m.Role, Content: m.Text}
		for _, img := range m.Images {
			id, err := c.upload(ctx, token, img)
			if err != nil {
				return Response{}, err
			}
			msg.Attachments = append(msg.Attachments, id)
		}
		body.Messages = append(body.Messages, msg)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	var out oaResponse // формат ответа совпадает с OpenAI
	if err := c.do(httpReq, &out); err != nil {
		return Response{}, err
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("gigachat: empty response")
	}
	return Response{
		Text:      out.Choices[0].Message.Content,
		Usage:     Usage{InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens},
		Truncated: out.Choices[0].FinishReason == "length",
	}, nil
}

func (c *GigaChat) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return &APIError{Status: resp.StatusCode, Body: truncate(string(raw), 500)}
	}
	return json.Unmarshal(raw, out)
}

// newUUID — случайный UUID v4 для заголовка RqUID.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

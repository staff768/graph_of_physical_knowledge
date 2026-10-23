package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAI — любой провайдер с OpenAI-совместимым /chat/completions:
// DeepSeek, российские агрегаторы, OpenRouter, Groq и другие.
type OpenAI struct {
	BaseURL string
	APIKey  string
	Model   string
	// ReasoningEffort — рассуждения «думающих» моделей: off (выключить), low, medium, high.
	// Пусто — не передаётся, решает провайдер. Рассуждения расходуют лимит токенов ответа.
	// Формат параметра — как у OpenRouter и совместимых агрегаторов (ProxyAPI).
	ReasoningEffort string
	HTTP            *http.Client
}

func (c *OpenAI) Name() string { return "openai-compatible/" + c.Model }

type oaContentPart struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *oaImageURL `json:"image_url,omitempty"`
}

type oaImageURL struct {
	URL string `json:"url"`
}

type oaMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // строка или []oaContentPart
}

type oaRequest struct {
	Model          string      `json:"model"`
	Messages       []oaMessage `json:"messages"`
	MaxTokens      int         `json:"max_tokens,omitempty"`
	Temperature    float64     `json:"temperature"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
	Reasoning *oaReasoning `json:"reasoning,omitempty"`
}

type oaReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

type oaResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (c *OpenAI) Complete(ctx context.Context, req Request) (Response, error) {
	body := oaRequest{Model: c.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature}
	for _, m := range req.Messages {
		if len(m.Images) == 0 {
			body.Messages = append(body.Messages, oaMessage{Role: m.Role, Content: m.Text})
			continue
		}
		parts := []oaContentPart{{Type: "text", Text: m.Text}}
		for _, img := range m.Images {
			url := "data:" + img.MIME + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
			parts = append(parts, oaContentPart{Type: "image_url", ImageURL: &oaImageURL{URL: url}})
		}
		body.Messages = append(body.Messages, oaMessage{Role: m.Role, Content: parts})
	}
	if req.JSON {
		body.ResponseFormat = &struct {
			Type string `json:"type"`
		}{Type: "json_object"}
	}
	switch c.ReasoningEffort {
	case "":
	case "off":
		disabled := false
		body.Reasoning = &oaReasoning{Enabled: &disabled}
	default:
		body.Reasoning = &oaReasoning{Effort: c.ReasoningEffort}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, &APIError{Status: resp.StatusCode, Body: truncate(string(raw), 500)}
	}
	var out oaResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, fmt.Errorf("llm: invalid response: %w", err)
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("llm: empty response")
	}
	return Response{
		Text:      out.Choices[0].Message.Content,
		Usage:     Usage{InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens},
		Truncated: out.Choices[0].FinishReason == "length",
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

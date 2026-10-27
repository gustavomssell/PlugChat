package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client interface {
	Complete(ctx context.Context, prompt string, variables map[string]any) (string, error)
}

type OpenAI struct {
	endpoint string
	apiKey   string
	model    string
	http     *http.Client
}

func FromEnv() Client {
	endpoint := strings.TrimSpace(os.Getenv("LLM_API_URL"))
	if endpoint == "" {
		return nil
	}

	apiKey := strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	model := strings.TrimSpace(os.Getenv("LLM_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}

	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	return &OpenAI{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (o *OpenAI) Complete(ctx context.Context, prompt string, variables map[string]any) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("llm: prompt vazio")
	}

	content := prompt
	if len(variables) > 0 {
		contextJSON, err := json.Marshal(variables)
		if err == nil {
			content += "\n\nContexto da conversa (JSON): " + string(contextJSON)
		}
	}

	payload, err := json.Marshal(map[string]any{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
		"temperature": 0.2,
	})
	if err != nil {
		return "", fmt.Errorf("llm: encode request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("llm: request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	response, err := o.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("llm: request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("llm: status %d", response.StatusCode)
	}

	var body struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	if len(body.Choices) == 0 {
		return "", errors.New("llm: resposta vazia")
	}

	text := strings.TrimSpace(body.Choices[0].Message.Content)
	if text == "" {
		return "", errors.New("llm: conteúdo vazio")
	}

	return text, nil
}

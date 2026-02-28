// Governing: SPEC-0017 REQ "LLM Provider Abstraction", ADR-0017
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/joestump/joe-links/internal/config"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicVersion    = "2023-06-01"
	defaultAnthropicModel = "claude-haiku-4-5-20251001"
)

type anthropicSuggester struct {
	apiKey       string
	model        string
	promptCustom string
	client       *http.Client
}

func newAnthropicSuggester(cfg *config.Config) *anthropicSuggester {
	model := cfg.LLM.Model
	if model == "" {
		model = defaultAnthropicModel
	}
	return &anthropicSuggester{
		apiKey:       cfg.LLM.APIKey,
		model:        model,
		promptCustom: cfg.LLM.Prompt,
		client:       &http.Client{},
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (a *anthropicSuggester) Suggest(ctx context.Context, req SuggestRequest) (*SuggestResponse, error) {
	prompt, err := renderPrompt(a.promptCustom, PromptData(req))
	if err != nil {
		return nil, fmt.Errorf("render prompt: %w", err)
	}

	body := anthropicRequest{
		Model:     a.model,
		MaxTokens: 256,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, respBody)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from anthropic")
	}

	var suggestion SuggestResponse
	if err := json.Unmarshal([]byte(apiResp.Content[0].Text), &suggestion); err != nil {
		return nil, fmt.Errorf("decode suggestion JSON: %w", err)
	}

	return &suggestion, nil
}

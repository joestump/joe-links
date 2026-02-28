// Governing: SPEC-0017 REQ "LLM Provider Abstraction", ADR-0017
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/joestump/joe-links/internal/config"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com"
	defaultOpenAIModel   = "gpt-4o-mini"
)

type openaiSuggester struct {
	apiKey       string
	model        string
	baseURL      string
	promptCustom string
	client       *http.Client
}

func newOpenAISuggester(cfg *config.Config) *openaiSuggester {
	model := cfg.LLM.Model
	if model == "" {
		model = defaultOpenAIModel
	}
	baseURL := cfg.LLM.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &openaiSuggester{
		apiKey:       cfg.LLM.APIKey,
		model:        model,
		baseURL:      baseURL,
		promptCustom: cfg.LLM.Prompt,
		client:       &http.Client{},
	}
}

type openaiRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (o *openaiSuggester) Suggest(ctx context.Context, req SuggestRequest) (*SuggestResponse, error) {
	prompt, err := renderPrompt(o.promptCustom, PromptData{
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("render prompt: %w", err)
	}

	body := openaiRequest{
		Model:     o.model,
		MaxTokens: 256,
		Messages:  []openaiMessage{{Role: "user", Content: prompt}},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := o.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API returned %d: %s", resp.StatusCode, respBody)
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from openai")
	}

	var suggestion SuggestResponse
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &suggestion); err != nil {
		return nil, fmt.Errorf("decode suggestion JSON: %w", err)
	}

	return &suggestion, nil
}

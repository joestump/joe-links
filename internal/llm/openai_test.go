// Governing: SPEC-0017 REQ "LLM Provider Configuration", REQ "LLM Provider Abstraction", REQ "Default Prompt Template"
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joestump/joe-links/internal/config"
)

func openaiEnvelope(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
	})
	return string(b)
}

func newOpenAITestSuggester(t *testing.T, provider, apiKey string, handler http.HandlerFunc) *openaiSuggester {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := &config.Config{}
	cfg.LLM.Provider = provider
	cfg.LLM.APIKey = apiKey
	cfg.LLM.BaseURL = srv.URL
	cfg.LLM.Model = "test-model"
	return newOpenAISuggester(cfg)
}

// Governing: SPEC-0017 REQ "LLM Provider Configuration" scenario "Ollama / custom endpoint" (#172)
// When no API key is configured, the Authorization header MUST be omitted.
func TestOpenAI_NoAPIKey_OmitsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	var hadAuth bool
	sg := newOpenAITestSuggester(t, "openai-compatible", "", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, hadAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(openaiEnvelope(`{"slug":"x","title":"","description":"","tags":[]}`)))
	})

	_, err := sg.Suggest(context.Background(), SuggestRequest{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if hadAuth || gotAuth != "" {
		t.Errorf("Authorization header was sent (%q); want it omitted for keyless provider", gotAuth)
	}
}

// When an API key IS configured, the Authorization: Bearer header MUST be present.
func TestOpenAI_WithAPIKey_SetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	sg := newOpenAITestSuggester(t, "openai", "sk-test-123", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(openaiEnvelope(`{"slug":"x","title":"","description":"","tags":[]}`)))
	})

	_, err := sg.Suggest(context.Background(), SuggestRequest{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if gotAuth != "Bearer sk-test-123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-test-123")
	}
}

// Governing: SPEC-0017 REQ "Default Prompt Template" scenario "LLM returns malformed JSON" (#169)
// A malformed model content must surface as *MalformedResponseError carrying the raw text.
func TestOpenAI_MalformedContent_ReturnsRawError(t *testing.T) {
	const raw = "definitely not json {oops"
	sg := newOpenAITestSuggester(t, "openai", "sk", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(openaiEnvelope(raw)))
	})

	_, err := sg.Suggest(context.Background(), SuggestRequest{URL: "https://example.com"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var mre *MalformedResponseError
	if !errors.As(err, &mre) {
		t.Fatalf("error type = %T, want *MalformedResponseError", err)
	}
	if mre.Raw != raw {
		t.Errorf("Raw = %q, want %q", mre.Raw, raw)
	}
}

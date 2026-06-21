// Governing: SPEC-0017 REQ "LLM Provider Abstraction", REQ "Default Prompt Template"
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

func anthropicEnvelope(text string) string {
	b, _ := json.Marshal(map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	})
	return string(b)
}

// Governing: SPEC-0017 REQ "Default Prompt Template" scenario "LLM returns malformed JSON" (#169)
// The anthropic suggester must also surface raw text via *MalformedResponseError.
func TestAnthropic_MalformedContent_ReturnsRawError(t *testing.T) {
	const raw = "<<<not json>>>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(anthropicEnvelope(raw)))
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{}
	cfg.LLM.Provider = "anthropic"
	cfg.LLM.APIKey = "sk"
	cfg.LLM.Model = "test-model"
	sg := newAnthropicSuggester(cfg)
	// Point the suggester at the fake server.
	sg.client = srv.Client()
	sg.apiURL = srv.URL

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

// Governing: SPEC-0017 REQ "Suggest API Endpoint", REQ "Default Prompt Template", REQ "LLM Provider Configuration"
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joestump/joe-links/internal/api"
	"github.com/joestump/joe-links/internal/config"
	"github.com/joestump/joe-links/internal/llm"
)

// newFakeLLMSuggester spins up an httptest.Server that mimics an OpenAI
// chat-completions endpoint and returns an openai-compatible Suggester wired to
// it. The handler receives the request and writes whatever status/body it wants,
// letting tests drive provider behaviour deterministically. The server is
// registered for cleanup via t.Cleanup.
func newFakeLLMSuggester(t *testing.T, handler http.HandlerFunc) llm.Suggester {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := &config.Config{}
	cfg.LLM.Provider = "openai-compatible"
	cfg.LLM.BaseURL = srv.URL // suggester appends /v1/chat/completions
	cfg.LLM.Model = "test-model"

	sg, err := llm.New(cfg)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	if sg == nil {
		t.Fatal("llm.New returned nil suggester for openai-compatible provider")
	}
	return sg
}

// openaiContentResponse builds the JSON envelope an OpenAI-style API returns,
// embedding content (the model's "text") as the assistant message.
func openaiContentResponse(content string) string {
	body := map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
	}
	b, _ := json.Marshal(body)
	return string(b)
}

func postSuggest(t *testing.T, env *testEnv, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/links/suggest", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		authRequest(req, token)
	}
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	return rec
}

// Scenario: Feature disabled (no provider configured) -> 503.
func TestSuggest_FeatureDisabled_503(t *testing.T) {
	env := newTestEnvWithSuggester(t, nil)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	rec := postSuggest(t, env, token, `{"url":"https://example.com"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

// Scenario: Unauthenticated request -> 401, before any LLM call.
func TestSuggest_Unauthenticated_401(t *testing.T) {
	called := false
	sg := newFakeLLMSuggester(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(openaiContentResponse(`{"slug":"x"}`)))
	})
	env := newTestEnvWithSuggester(t, sg)

	rec := postSuggest(t, env, "", `{"url":"https://example.com"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("LLM backend was contacted on an unauthenticated request")
	}
}

// Scenario: Missing url in request -> 400.
func TestSuggest_MissingURL_400(t *testing.T) {
	called := false
	sg := newFakeLLMSuggester(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(openaiContentResponse(`{"slug":"x"}`)))
	})
	env := newTestEnvWithSuggester(t, sg)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	rec := postSuggest(t, env, token, `{"title":"no url here"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if called {
		t.Error("LLM backend was contacted despite missing url")
	}
}

// Scenario: Provider returns a non-2xx error -> 502.
func TestSuggest_ProviderError_502(t *testing.T) {
	sg := newFakeLLMSuggester(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	env := newTestEnvWithSuggester(t, sg)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	rec := postSuggest(t, env, token, `{"url":"https://example.com"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

// Scenario: Provider returns malformed JSON -> 502 (and no panic; raw text logged).
func TestSuggest_MalformedJSON_502(t *testing.T) {
	sg := newFakeLLMSuggester(t, func(w http.ResponseWriter, r *http.Request) {
		// Valid OpenAI envelope, but the model's content is not the expected JSON.
		_, _ = w.Write([]byte(openaiContentResponse("this is not json at all")))
	})
	env := newTestEnvWithSuggester(t, sg)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	rec := postSuggest(t, env, token, `{"url":"https://example.com"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

// Scenario: Happy path -> 200 with fields populated.
func TestSuggest_HappyPath_200(t *testing.T) {
	sg := newFakeLLMSuggester(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(openaiContentResponse(
			`{"slug":"go-docs","title":"Go Docs","description":"The Go docs.","tags":["go","docs"]}`)))
	})
	env := newTestEnvWithSuggester(t, sg)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	rec := postSuggest(t, env, token, `{"url":"https://go.dev","title":"Go","description":"docs"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.SuggestResponseBody
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "go-docs" {
		t.Errorf("slug = %q, want %q", resp.Slug, "go-docs")
	}
	if resp.Title != "Go Docs" {
		t.Errorf("title = %q, want %q", resp.Title, "Go Docs")
	}
	if resp.Description != "The Go docs." {
		t.Errorf("description = %q, want %q", resp.Description, "The Go docs.")
	}
	if len(resp.Tags) != 2 {
		t.Errorf("len(tags) = %d, want 2", len(resp.Tags))
	}
}

// Scenario: Happy path, but the suggested slug is invalid -> slug is omitted
// (blanked) while the other fields are still returned. Governing: #170.
func TestSuggest_InvalidSlug_Omitted(t *testing.T) {
	sg := newFakeLLMSuggester(t, func(w http.ResponseWriter, r *http.Request) {
		// "Go Docs!" has uppercase, spaces, and punctuation — invalid slug.
		_, _ = w.Write([]byte(openaiContentResponse(
			`{"slug":"Go Docs!","title":"Go Docs","description":"The Go docs.","tags":["go"]}`)))
	})
	env := newTestEnvWithSuggester(t, sg)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	rec := postSuggest(t, env, token, `{"url":"https://go.dev"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.SuggestResponseBody
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "" {
		t.Errorf("slug = %q, want empty (invalid slug must be omitted)", resp.Slug)
	}
	if resp.Title != "Go Docs" {
		t.Errorf("title = %q, want %q (other fields must survive)", resp.Title, "Go Docs")
	}
}

// Scenario: A reserved slug (e.g. "admin") is also treated as invalid and omitted.
func TestSuggest_ReservedSlug_Omitted(t *testing.T) {
	sg := newFakeLLMSuggester(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(openaiContentResponse(
			`{"slug":"admin","title":"Admin","description":"","tags":[]}`)))
	})
	env := newTestEnvWithSuggester(t, sg)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	rec := postSuggest(t, env, token, `{"url":"https://example.com/admin"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.SuggestResponseBody
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "" {
		t.Errorf("slug = %q, want empty (reserved slug must be omitted)", resp.Slug)
	}
}

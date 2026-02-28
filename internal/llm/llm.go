// Governing: SPEC-0017 REQ "LLM Provider Abstraction", "LLM Provider Configuration", ADR-0017
package llm

import (
	"context"
	"fmt"

	"github.com/joestump/joe-links/internal/config"
)

// SuggestRequest is the input to the Suggester.
type SuggestRequest struct {
	URL         string
	Title       string
	Description string
}

// SuggestResponse is the structured suggestion returned by the LLM.
type SuggestResponse struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// Suggester generates link metadata suggestions via an LLM provider.
type Suggester interface {
	Suggest(ctx context.Context, req SuggestRequest) (*SuggestResponse, error)
}

// New creates a Suggester based on the config. Returns nil when LLMProvider is
// unset, meaning LLM suggestions are disabled.
func New(cfg *config.Config) (Suggester, error) {
	switch cfg.LLM.Provider {
	case "":
		return nil, nil
	case "anthropic":
		return newAnthropicSuggester(cfg), nil
	case "openai", "openai-compatible":
		return newOpenAISuggester(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %q", cfg.LLM.Provider)
	}
}

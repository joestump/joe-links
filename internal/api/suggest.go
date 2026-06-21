// Governing: SPEC-0017 REQ "Suggest API Endpoint", ADR-0017, ADR-0008, ADR-0009, ADR-0010
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/llm"
	"github.com/joestump/joe-links/internal/store"
)

// maxRawLogBytes bounds how much of a malformed LLM response we log, to avoid
// flooding the logs with a pathologically large model response.
// Governing: SPEC-0017 REQ "Default Prompt Template" scenario "LLM returns malformed JSON"
const maxRawLogBytes = 2048

// SuggestRequest is the body for POST /api/v1/links/suggest.
// Governing: SPEC-0017 REQ "Suggest API Endpoint"
type SuggestRequest struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// SuggestResponse is the suggested metadata for a link.
// Governing: SPEC-0017 REQ "Suggest API Endpoint"
type SuggestResponseBody struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// suggestAPIHandler provides the POST /api/v1/links/suggest endpoint.
type suggestAPIHandler struct {
	suggester llm.Suggester
}

// Suggest generates link metadata suggestions via the configured LLM provider.
// POST /api/v1/links/suggest
// Governing: SPEC-0017 REQ "Suggest API Endpoint", ADR-0017, ADR-0008, ADR-0009, ADR-0010
//
// @Summary      Suggest link metadata
// @Description  Uses an LLM to suggest slug, title, description, and tags for a URL.
// @Tags         Links
// @Accept       json
// @Produce      json
// @Param        body  body      SuggestRequest       true  "URL to generate suggestions for"
// @Success      200   {object}  SuggestResponseBody
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      502   {object}  ErrorResponse  "LLM provider error"
// @Failure      503   {object}  ErrorResponse  "LLM not configured"
// @Security     BearerToken
// @Router       /links/suggest [post]
func (h *suggestAPIHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	if h.suggester == nil {
		writeError(w, http.StatusServiceUnavailable, "LLM suggestions are not configured", "LLM_NOT_CONFIGURED")
		return
	}

	var req SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required", "BAD_REQUEST")
		return
	}

	resp, err := h.suggester.Suggest(r.Context(), llm.SuggestRequest{
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		// Governing: SPEC-0017 REQ "Default Prompt Template" scenario "LLM returns malformed JSON"
		// On a JSON parse failure the server MUST log the RAW LLM response text.
		var malformed *llm.MalformedResponseError
		if errors.As(err, &malformed) {
			raw := malformed.Raw
			if len(raw) > maxRawLogBytes {
				raw = raw[:maxRawLogBytes] + "...(truncated)"
			}
			log.Printf("api: LLM suggest malformed response: %v; raw=%q", malformed.Err, raw)
		} else {
			log.Printf("api: LLM suggest error: %v", err)
		}
		writeError(w, http.StatusBadGateway, "LLM provider error", "LLM_ERROR")
		return
	}

	// Governing: SPEC-0017 REQ "Default Prompt Template"
	// Suggested slugs MUST follow the same validation rules as user-supplied slugs.
	// Degrade gracefully: if the model returns an invalid slug, blank it out rather
	// than failing the whole request — the other suggested fields are still useful.
	slug := resp.Slug
	if slug != "" {
		if err := store.ValidateSlugFormat(slug); err != nil {
			log.Printf("api: LLM suggested invalid slug %q, omitting: %v", slug, err)
			slug = ""
		}
	}

	// Likewise drop title/description that violate length limits rather than
	// rejecting the whole suggestion. Validate each field independently so an
	// over-long title doesn't mask an over-long description (and vice versa).
	title, description := resp.Title, resp.Description
	if err := store.ValidateLinkText(title, ""); err != nil {
		log.Printf("api: LLM suggested title failed validation, omitting: %v", err)
		title = ""
	}
	if err := store.ValidateLinkText("", description); err != nil {
		log.Printf("api: LLM suggested description failed validation, omitting: %v", err)
		description = ""
	}

	writeJSON(w, http.StatusOK, SuggestResponseBody{
		Slug:        slug,
		Title:       title,
		Description: description,
		Tags:        resp.Tags,
	})
}

// Governing: SPEC-0017 REQ "Suggest API Endpoint", ADR-0017, ADR-0008, ADR-0009, ADR-0010
package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/llm"
)

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
		log.Printf("api: LLM suggest error: %v", err)
		writeError(w, http.StatusBadGateway, "LLM provider error", "LLM_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, SuggestResponseBody{
		Slug:        resp.Slug,
		Title:       resp.Title,
		Description: resp.Description,
		Tags:        resp.Tags,
	})
}

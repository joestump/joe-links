// Governing: SPEC-0017 REQ "Suggest API Endpoint", ADR-0008, ADR-0009
package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/joestump/joe-links/internal/llm"
)

// suggestAPIHandler provides the POST /api/v1/links/suggest endpoint.
type suggestAPIHandler struct {
	suggester llm.Suggester
}

// Suggest uses the configured LLM to suggest slug, title, description, and tags for a URL.
// POST /api/v1/links/suggest
// Governing: SPEC-0017 REQ "Suggest API Endpoint"
//
// @Summary      Suggest link metadata
// @Description  Uses the configured LLM to suggest slug, title, description, and tags for a URL
// @Tags         Links
// @Accept       json
// @Produce      json
// @Security     BearerToken
// @Param        request  body      SuggestRequest  true  "URL to get suggestions for"
// @Success      200      {object}  SuggestResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Failure      502      {object}  ErrorResponse
// @Failure      503      {object}  ErrorResponse
// @Router       /links/suggest [post]
func (h *suggestAPIHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	var req SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required", "BAD_REQUEST")
		return
	}

	if h.suggester == nil {
		writeError(w, http.StatusServiceUnavailable, "LLM suggestions not configured", "LLM_NOT_CONFIGURED")
		return
	}

	resp, err := h.suggester.Suggest(r.Context(), llm.SuggestRequest{
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		log.Printf("api: suggest LLM error: %v", err)
		writeError(w, http.StatusBadGateway, "LLM suggestion failed", "LLM_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, SuggestResponse{
		Slug:        resp.Slug,
		Title:       resp.Title,
		Description: resp.Description,
		Tags:        resp.Tags,
	})
}

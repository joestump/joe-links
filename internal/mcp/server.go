// Package mcp exposes joe-links to AI agents over the Model Context Protocol.
//
// The server is an in-process Streamable HTTP endpoint mounted at /mcp,
// running in stateless JSON mode and authenticated with the same personal
// access tokens as /api/v1. Tool handlers are thin adapters over the shared
// store layer so authorization can never diverge between surfaces.
//
// Governing: ADR-0018, SPEC-0018 REQ "MCP Endpoint"
package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/build"
	"github.com/joestump/joe-links/internal/llm"
	"github.com/joestump/joe-links/internal/metrics"
	"github.com/joestump/joe-links/internal/store"
)

// maxBodyBytes bounds every /mcp request body.
// Governing: SPEC-0018 "Security Requirements" — Request Body Size Limits
const maxBodyBytes = 1 << 20 // 1 MB

// Deps holds the shared stores and services MCP tools delegate to.
// Tools MUST call these rather than reimplementing authorization rules.
// Governing: SPEC-0018 REQ "Authorization Parity with the REST API"
type Deps struct {
	LinkStore      *store.LinkStore
	OwnershipStore *store.OwnershipStore
	TagStore       *store.TagStore
	UserStore      *store.UserStore
	KeywordStore   *store.KeywordStore
	ClickStore     *store.ClickStore
	Suggester      llm.Suggester // nil when LLM is not configured
	ShortKeyword   string        // optional short-prefix override (e.g. "go")
}

// NewServer constructs the MCP server and registers the v1 tool inventory.
// Governing: SPEC-0018 REQ "MCP Endpoint"
func NewServer(deps Deps) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{
		Name:    "joe-links",
		Version: build.Version,
	}, nil)
	registerTools(s, deps)
	return s
}

// NewHandler wraps the MCP server in the Streamable HTTP transport plus the
// endpoint middleware: PAT bearer authentication, security headers, a
// WWW-Authenticate challenge on 401, and a request body size limit.
//
// The handler is stateless: every request is self-contained, no session
// affinity is required, and the SDK passes each HTTP request's context into
// tool handlers — which is how the bearer middleware's authenticated user
// reaches the store layer.
//
// Governing: ADR-0018, SPEC-0018 REQ "MCP Endpoint", REQ "Bearer Token Authentication"
func NewHandler(deps Deps, bearer *auth.BearerTokenMiddleware) http.Handler {
	server := NewServer(deps)
	streamable := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return server },
		&sdk.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
		},
	)
	limited := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		// Stash the request's base URL so tool handlers can build canonical
		// short URLs (the SDK threads this context into tool calls).
		// Governing: SPEC-0018 REQ "Tool Inventory" — short URL in results
		ctx := context.WithValue(r.Context(), baseURLKey{}, baseURLFromRequest(r))
		streamable.ServeHTTP(w, r.WithContext(ctx))
	})
	return securityHeaders(bearer.Authenticate(limited))
}

// baseURLKey is the context key for the per-request scheme://host.
type baseURLKey struct{}

// baseURLFromRequest derives scheme://host the same way the web UI's SiteURL
// does: X-Forwarded-Proto from the reverse proxy, else TLS state.
func baseURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xf := r.Header.Get("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	return scheme + "://" + r.Host
}

// shortURL builds the canonical short URL for a slug from the request-scoped
// base URL. Falls back to a bare path when no base is in context (never the
// case for real requests).
func shortURL(ctx context.Context, slug string) string {
	if base, ok := ctx.Value(baseURLKey{}).(string); ok && base != "" {
		return base + "/" + slug
	}
	return "/" + slug
}

// securityHeaders sets the response headers required by SPEC-0018 and adds a
// WWW-Authenticate challenge to 401 responses (the shared bearer middleware
// writes the 401 itself, so the header is injected via a wrapped writer).
// Governing: SPEC-0018 "Security Requirements", REQ "Bearer Token Authentication"
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(&challengeWriter{ResponseWriter: w}, r)
	})
}

// challengeWriter injects WWW-Authenticate on 401 status writes.
type challengeWriter struct {
	http.ResponseWriter
}

func (cw *challengeWriter) WriteHeader(status int) {
	if status == http.StatusUnauthorized {
		cw.Header().Set("WWW-Authenticate", `Bearer realm="joe-links"`)
	}
	cw.ResponseWriter.WriteHeader(status)
}

// toolError is the machine-readable payload embedded in error tool results.
// Codes reuse the REST API vocabulary so one documented table serves both
// surfaces.
// Governing: SPEC-0018 REQ "Structured Tool Errors"
type toolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorResult builds an MCP tool result flagged as an error, carrying a
// stable code plus a human-readable message as JSON text content.
// Governing: SPEC-0018 REQ "Structured Tool Errors"
func errorResult(code, message string) *sdk.CallToolResult {
	payload, err := json.Marshal(toolError{Code: code, Message: message})
	if err != nil {
		payload = []byte(`{"code":"internal_error","message":"failed to encode error"}`)
	}
	return &sdk.CallToolResult{
		IsError: true,
		Content: []sdk.Content{&sdk.TextContent{Text: string(payload)}},
	}
}

// addTool registers a typed tool wrapped with per-call metrics and structured
// logging. All v1 tools MUST be registered through this helper.
// Governing: SPEC-0018 REQ "Observability", REQ "Error Handling Standards"
func addTool[In, Out any](s *sdk.Server, t *sdk.Tool, h sdk.ToolHandlerFor[In, Out]) {
	sdk.AddTool(s, t, func(ctx context.Context, req *sdk.CallToolRequest, in In) (*sdk.CallToolResult, Out, error) {
		res, out, err := h(ctx, req, in)

		outcome := "success"
		if err != nil || (res != nil && res.IsError) {
			outcome = "error"
		}
		metrics.MCPToolCallsTotal.WithLabelValues(t.Name, outcome).Inc()

		// Log the acting user id, never the token.
		// Governing: SPEC-0018 REQ "Observability"
		userID := "unknown"
		if u := auth.UserFromContext(ctx); u != nil {
			userID = u.ID
		}
		slog.Info("mcp tool call", "tool", t.Name, "user_id", userID, "outcome", outcome)

		return res, out, err
	})
}

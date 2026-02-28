# SPEC-0017: LLM-Powered Metadata Suggestions for Link Creation

## Overview

When creating a go-link, users must manually supply a slug, title, description, and tags. For most URLs these values are predictable from the page content, but the friction of filling in four fields slows link creation — especially in the browser extension popup. This spec formalises the requirements for a server-side LLM suggestion endpoint and the corresponding browser extension integration. See ADR-0017 for the architectural decision.

## Requirements

### Requirement: LLM Provider Configuration

The server MUST support configuration of an LLM provider via environment variables. When `JOE_LLM_PROVIDER` is unset, the LLM feature MUST be completely disabled and the server MUST behave identically to a deployment with no LLM config. No code paths that contact an external LLM MUST execute when the feature is disabled.

The following environment variables MUST be supported:

| Variable | Default | Description |
|----------|---------|-------------|
| `JOE_LLM_PROVIDER` | *(unset — disabled)* | `anthropic`, `openai`, or `openai-compatible` |
| `JOE_LLM_API_KEY` | — | API key for the chosen provider |
| `JOE_LLM_MODEL` | *(provider default)* | Model name (e.g. `claude-haiku-4-5-20251001`, `gpt-4o-mini`, `llama3`) |
| `JOE_LLM_BASE_URL` | *(provider default)* | Base URL override for Ollama or any OpenAI-compatible endpoint |
| `JOE_LLM_PROMPT` | *(built-in default)* | Override the system prompt sent to the LLM |

#### Scenario: Feature disabled by default

- **WHEN** `JOE_LLM_PROVIDER` is not set
- **THEN** the server starts normally, `POST /api/v1/links/suggest` returns HTTP 503, and no LLM client is initialised

#### Scenario: Valid provider configured

- **WHEN** `JOE_LLM_PROVIDER` is set to a supported value and `JOE_LLM_API_KEY` is provided
- **THEN** the server initialises the appropriate LLM client and the suggest endpoint becomes active

#### Scenario: Ollama / custom endpoint

- **WHEN** `JOE_LLM_PROVIDER` is `openai-compatible` and `JOE_LLM_BASE_URL` is set to a local Ollama URL (e.g. `http://localhost:11434/v1`)
- **THEN** the server routes LLM calls to that URL without requiring an API key

---

### Requirement: Suggest API Endpoint

The server MUST expose `POST /api/v1/links/suggest` following the REST conventions in ADR-0008. The endpoint MUST require bearer token authentication per ADR-0009. The request body MUST accept `url`, `title`, and `description` fields. The response MUST return `slug`, `title`, `description`, and `tags` when the LLM call succeeds. The endpoint MUST be documented in the OpenAPI / Swagger spec per ADR-0010.

#### Scenario: Successful suggestion

- **WHEN** an authenticated request is posted with a valid URL, title, and description and the LLM is configured
- **THEN** the server returns HTTP 200 with a JSON body containing `slug`, `title`, `description`, and `tags`

#### Scenario: LLM not configured

- **WHEN** `JOE_LLM_PROVIDER` is unset and the endpoint is called
- **THEN** the server returns HTTP 503 Service Unavailable with a JSON error body

#### Scenario: LLM call fails

- **WHEN** the configured LLM provider returns an error or times out
- **THEN** the server returns HTTP 502 Bad Gateway with a JSON error body; the caller SHOULD surface no error to the end-user

#### Scenario: Missing required fields

- **WHEN** the request body is missing the `url` field
- **THEN** the server returns HTTP 400 Bad Request

#### Scenario: Unauthenticated request

- **WHEN** no `Authorization: Bearer` header is present
- **THEN** the server returns HTTP 401 Unauthorized

---

### Requirement: LLM Provider Abstraction

The implementation MUST define an `internal/llm` package containing a `Suggester` interface. All LLM communication MUST go through this interface so that provider-specific code is isolated. The package MUST NOT import provider SDKs; it MUST use only the Go standard library `net/http` for all outbound calls.

Two implementations MUST be provided:
- `anthropicSuggester` — calls the Anthropic Messages API (`api.anthropic.com/v1/messages`)
- `openaiSuggester` — calls the OpenAI Chat Completions API; MUST also serve as the implementation for `openai-compatible` providers by accepting an overridden base URL

#### Scenario: Anthropic provider routes correctly

- **WHEN** `JOE_LLM_PROVIDER=anthropic`
- **THEN** outbound LLM calls use `POST https://api.anthropic.com/v1/messages` with the `x-api-key` and `anthropic-version` headers

#### Scenario: OpenAI provider routes correctly

- **WHEN** `JOE_LLM_PROVIDER=openai`
- **THEN** outbound LLM calls use `POST https://api.openai.com/v1/chat/completions` with an `Authorization: Bearer` header

#### Scenario: OpenAI-compatible with base URL override

- **WHEN** `JOE_LLM_PROVIDER=openai-compatible` and `JOE_LLM_BASE_URL` is set
- **THEN** the openaiSuggester routes calls to the overridden base URL instead of `api.openai.com`

---

### Requirement: Default Prompt Template

The server MUST embed a default prompt template in the binary. The template MUST use Go `text/template` syntax with `{{.URL}}`, `{{.Title}}`, and `{{.Description}}` variables. The template MUST instruct the LLM to respond with valid JSON only, using the structure `{"slug":"...","title":"...","description":"...","tags":["..."]}`. `JOE_LLM_PROMPT` MUST override the default when set.

Slug suggestions MUST follow the same validation rules as user-supplied slugs: lowercase letters, digits, and hyphens only; max 30 characters; no leading or trailing hyphens.

#### Scenario: Default prompt used

- **WHEN** `JOE_LLM_PROMPT` is not set
- **THEN** the server renders the built-in prompt template with the request's URL, title, and description

#### Scenario: Custom prompt override

- **WHEN** `JOE_LLM_PROMPT` is set to a valid Go template string
- **THEN** the server renders that template instead of the built-in default

#### Scenario: LLM returns malformed JSON

- **WHEN** the LLM response cannot be parsed as the expected JSON structure
- **THEN** the server returns HTTP 502 and logs the raw LLM response for debugging

---

### Requirement: Extension Meta Extraction

The browser extension MUST extract the active tab's `<title>` and `<meta name="description">` content before calling the suggest endpoint. The `manifest.json` MUST include the `"scripting"` permission. Extraction MUST use `chrome.scripting.executeScript` targeting the active tab. If extraction fails (e.g. on a `chrome://` page or a tab with no URL), the extension MUST still call the suggest endpoint with whatever values are available, omitting missing fields.

#### Scenario: Page has title and meta description

- **WHEN** the popup opens on a normal web page
- **THEN** both `title` and `description` are extracted and sent in the suggest request

#### Scenario: Page has no meta description

- **WHEN** the active tab's page has no `<meta name="description">` tag
- **THEN** the suggest request is sent with `description` omitted or empty; the server MUST still return a valid suggestion

#### Scenario: Scripting fails on privileged page

- **WHEN** `chrome.scripting.executeScript` throws (e.g. `chrome://` URLs)
- **THEN** the extension falls back to using only the tab title already available via `chrome.tabs.query`, and still fires the suggest request

---

### Requirement: Extension Suggestion Strip

After the popup form is interactive, the extension MUST fire the suggest request asynchronously. The form MUST remain fully usable while the request is in flight. When suggestions are returned, the extension MUST render a "✦ Suggested" strip above the form fields. Each suggested field (slug, title, description, tags) MUST have a one-click "Use" button that fills the corresponding form input. The strip MUST be dismissible. If the suggest endpoint returns 503 or any non-200 response, the extension MUST silently hide the suggestion UI — no error MUST be shown to the user for a missing or failed LLM.

#### Scenario: Suggestions arrive successfully

- **WHEN** the suggest endpoint returns HTTP 200 with a valid JSON body
- **THEN** the extension renders the suggestion strip with "Use" buttons for each suggested field

#### Scenario: Partial suggestions

- **WHEN** the LLM returns suggestions for only some fields (e.g. slug and title but not tags)
- **THEN** the extension renders "Use" buttons only for the fields that have non-empty values

#### Scenario: User accepts a suggestion

- **WHEN** the user clicks a "Use" button next to a suggested value
- **THEN** the corresponding form field is populated with the suggestion; existing user input in that field is replaced

#### Scenario: User dismisses the strip

- **WHEN** the user clicks the dismiss (×) button on the suggestion strip
- **THEN** the strip is hidden and the form is unaffected

#### Scenario: LLM not configured (503)

- **WHEN** the suggest endpoint returns HTTP 503
- **THEN** no suggestion strip is rendered; the popup behaves as if the feature does not exist

#### Scenario: Suggestion request fails or times out

- **WHEN** the network request to the suggest endpoint fails or times out
- **THEN** no suggestion strip is rendered; no error is shown to the user

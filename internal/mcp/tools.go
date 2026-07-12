// Tool inventory for the joe-links MCP server. Every tool is a thin adapter
// over the shared store layer: authorization rules are the REST API's,
// enforced by the same store calls — never reimplemented here.
//
// Governing: ADR-0018, SPEC-0018 REQ "Tool Inventory", REQ "Authorization Parity with the REST API"
package mcp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/llm"
	"github.com/joestump/joe-links/internal/store"
)

// Error codes shared with the REST API vocabulary (one casing).
// Governing: SPEC-0018 REQ "Structured Tool Errors"
const (
	codeUnauthorized   = "unauthorized"
	codeForbidden      = "forbidden"
	codeNotFound       = "not_found"
	codeValidation     = "validation_failed"
	codeDuplicateSlug  = "duplicate_slug"
	codeUnknownUser    = "unknown_user"
	codeDuplicateShare = "duplicate_share"
	codeDuplicateOwner = "duplicate_owner"
	codeLLMError       = "llm_error"
	codeInternal       = "internal_error"
)

// ownerPayload mirrors the REST OwnerResponse shape.
type ownerPayload struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	IsPrimary bool   `json:"is_primary"`
}

// linkPayload is the structured result for tools returning a full link.
// ShortURL is always populated so agents can hand humans a working URL.
// Governing: SPEC-0018 REQ "Tool Inventory" — short URL in create/get/update results
type linkPayload struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	ShortURL    string         `json:"short_url" jsonschema:"canonical short URL for this link"`
	URL         string         `json:"url"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Visibility  string         `json:"visibility"`
	Tags        []string       `json:"tags,omitempty"`
	Owners      []ownerPayload `json:"owners,omitempty"`
	SharedWith  []string       `json:"shared_with,omitempty" jsonschema:"emails with share access; only populated for owners/admins"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// listItemPayload is the compact per-row shape for list_links.
type listItemPayload struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	ShortURL    string `json:"short_url"`
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility"`
}

// registerTools registers the SPEC-0018 v1 tool inventory — exactly these
// tools, no admin capabilities. suggest_link_metadata is registered only when
// an LLM provider is configured.
// Governing: SPEC-0018 REQ "Tool Inventory", REQ "Conditional Suggestion Tool"
func registerTools(s *sdk.Server, deps Deps) {
	addTool(s, &sdk.Tool{
		Name:        "create_link",
		Description: "Create a go-link. Defaults to private visibility; pass share_with emails to create a secure link shared with those users in one atomic call. Returns the working short URL.",
	}, createLinkTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "get_link",
		Description: "Fetch one link by slug or id, including visibility, tags, owners, and (for owners) share grants.",
	}, getLinkTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "list_links",
		Description: "List links visible to you. filter: mine (default), shared (shared with you), or public (browsable by everyone). Optional q search and tag filter apply to mine/public.",
	}, listLinksTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "update_link",
		Description: "Update url, title, description, tags, or visibility on a link you own. Omitted fields are left unchanged; slug is immutable.",
	}, updateLinkTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "delete_link",
		Description: "Permanently delete a link you own.",
	}, deleteLinkTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "share_link",
		Description: "Grant a user access to a secure link by email. The user must already have a joe-links account.",
	}, shareLinkTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "unshare_link",
		Description: "Revoke a user's share access to a link by email.",
	}, unshareLinkTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "add_co_owner",
		Description: "Add a co-owner to a link you own, by email. Co-owners can edit and delete the link.",
	}, addCoOwnerTool(deps))

	addTool(s, &sdk.Tool{
		Name:        "get_link_stats",
		Description: "Click totals (all-time, 7d, 30d) and recent clicks for a link you own.",
	}, getLinkStatsTool(deps))

	// Governing: SPEC-0018 REQ "Conditional Suggestion Tool"
	if deps.Suggester != nil {
		addTool(s, &sdk.Tool{
			Name:        "suggest_link_metadata",
			Description: "Ask the configured LLM to suggest a slug, title, description, and tags for a destination URL.",
		}, suggestTool(deps))
	}

	addTool(s, &sdk.Tool{
		Name:        "list_keywords",
		Description: "List keyword templates configured on this server (used for keyword host routing like jira/ABC-123).",
	}, listKeywordsTool(deps))
}

// --- shared helpers -------------------------------------------------------

// requireUser returns the authenticated user from the request context. The
// bearer middleware guarantees presence; the nil check is defense in depth.
// Governing: SPEC-0018 REQ "Bearer Token Authentication"
func requireUser(ctx context.Context) (*store.User, *sdk.CallToolResult) {
	if u := auth.UserFromContext(ctx); u != nil {
		return u, nil
	}
	return nil, errorResult(codeUnauthorized, "no authenticated user in request context")
}

// resolveLink looks a link up by slug first, then by id.
func resolveLink(ctx context.Context, deps Deps, ref string) (*store.Link, *sdk.CallToolResult) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, errorResult(codeValidation, "link reference (slug or id) is required")
	}
	link, err := deps.LinkStore.GetBySlug(ctx, ref)
	if err == nil {
		return link, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		slog.Error("mcp: resolve link by slug", "ref", ref, "error", err)
		return nil, errorResult(codeInternal, "failed to look up link")
	}
	link, err = deps.LinkStore.GetByID(ctx, ref)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrNotFound) {
			return nil, errorResult(codeNotFound, fmt.Sprintf("no link with slug or id %q", ref))
		}
		slog.Error("mcp: resolve link by id", "ref", ref, "error", err)
		return nil, errorResult(codeInternal, "failed to look up link")
	}
	return link, nil
}

// isOwnerOrAdmin mirrors the REST mutation rule: link owners (incl. co-owners)
// and admins.
// Governing: SPEC-0018 REQ "Authorization Parity with the REST API"
func isOwnerOrAdmin(deps Deps, user *store.User, linkID string) (bool, error) {
	if user.Role == "admin" {
		return true, nil
	}
	return deps.OwnershipStore.IsOwner(linkID, user.ID)
}

// canRead mirrors REST GET /links/{id}: owner OR share recipient OR admin.
// Governing: SPEC-0018 REQ "Authorization Parity with the REST API"
func canRead(ctx context.Context, deps Deps, user *store.User, linkID string) (bool, error) {
	ok, err := isOwnerOrAdmin(deps, user, linkID)
	if err != nil || ok {
		return ok, err
	}
	return deps.LinkStore.HasShare(ctx, linkID, user.ID)
}

// buildLinkPayload assembles the full link result, including owners and — for
// owners/admins — the share list resolved to emails.
func buildLinkPayload(ctx context.Context, deps Deps, link *store.Link, includeShares bool) (*linkPayload, error) {
	p := &linkPayload{
		ID:          link.ID,
		Slug:        link.Slug,
		ShortURL:    shortURL(ctx, link.Slug),
		URL:         link.URL,
		Title:       link.Title,
		Description: link.Description,
		Visibility:  link.Visibility,
		CreatedAt:   link.CreatedAt,
		UpdatedAt:   link.UpdatedAt,
	}

	tags, err := deps.LinkStore.ListTags(ctx, link.ID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	for _, t := range tags {
		p.Tags = append(p.Tags, t.Name)
	}

	owners, err := deps.OwnershipStore.ListOwnerUsers(link.ID)
	if err != nil {
		return nil, fmt.Errorf("list owners: %w", err)
	}
	for _, o := range owners {
		p.Owners = append(p.Owners, ownerPayload{ID: o.ID, Email: o.Email, IsPrimary: o.IsPrimary})
	}

	if includeShares {
		shares, err := deps.LinkStore.ListShares(ctx, link.ID)
		if err != nil {
			return nil, fmt.Errorf("list shares: %w", err)
		}
		for _, sh := range shares {
			u, err := deps.UserStore.GetByID(ctx, sh.UserID)
			if err != nil {
				continue // dangling share row; skip rather than fail the read
			}
			p.SharedWith = append(p.SharedWith, u.Email)
		}
		sort.Strings(p.SharedWith)
	}
	return p, nil
}

// resolveEmails maps share_with emails to user IDs. All emails must resolve;
// unknown ones fail the whole call, named.
// Governing: SPEC-0018 REQ "Agent-Oriented Creation Defaults"
func resolveEmails(ctx context.Context, deps Deps, emails []string) ([]string, *sdk.CallToolResult) {
	ids := make([]string, 0, len(emails))
	var unknown []string
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		u, err := deps.UserStore.GetByEmail(ctx, e)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				unknown = append(unknown, e)
				continue
			}
			slog.Error("mcp: resolve email", "error", err)
			return nil, errorResult(codeInternal, "failed to resolve user email")
		}
		ids = append(ids, u.ID)
	}
	if len(unknown) > 0 {
		return nil, errorResult(codeUnknownUser,
			fmt.Sprintf("no joe-links account for: %s — users must sign in once before links can be shared with them", strings.Join(unknown, ", ")))
	}
	return ids, nil
}

func internalError(action string, err error) *sdk.CallToolResult {
	slog.Error("mcp: "+action, "error", err)
	return errorResult(codeInternal, action+" failed")
}

// --- create_link ----------------------------------------------------------

type createLinkIn struct {
	Slug        string   `json:"slug" jsonschema:"short slug, [a-z0-9][a-z0-9-]*[a-z0-9]"`
	URL         string   `json:"url" jsonschema:"destination URL; may contain $variable placeholders"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Visibility  string   `json:"visibility,omitempty" jsonschema:"public, private, or secure; defaults to private (secure when share_with is set)"`
	ShareWith   []string `json:"share_with,omitempty" jsonschema:"emails of existing users to grant access; implies secure visibility unless overridden"`
}

func createLinkTool(deps Deps) sdk.ToolHandlerFor[createLinkIn, *linkPayload] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in createLinkIn) (*sdk.CallToolResult, *linkPayload, error) {
		user, denied := requireUser(ctx)
		if denied != nil {
			return denied, nil, nil
		}

		if strings.TrimSpace(in.URL) == "" {
			return errorResult(codeValidation, "url is required"), nil, nil
		}
		if err := store.ValidateSlugFormat(in.Slug); err != nil {
			return errorResult(codeValidation, err.Error()), nil, nil
		}
		if err := store.ValidateURLVariables(in.URL); err != nil {
			return errorResult(codeValidation, err.Error()), nil, nil
		}
		if err := store.ValidateLinkText(in.Title, in.Description); err != nil {
			return errorResult(codeValidation, err.Error()), nil, nil
		}

		// Agent-surface defaults: private unless sharing, which implies secure.
		// Explicit visibility always wins. Deliberate divergence from the REST
		// default of public — see SPEC-0018.
		// Governing: SPEC-0018 REQ "Agent-Oriented Creation Defaults"
		visibility := in.Visibility
		if visibility == "" {
			if len(in.ShareWith) > 0 {
				visibility = "secure"
			} else {
				visibility = "private"
			}
		}
		if err := store.ValidateVisibility(visibility); err != nil {
			return errorResult(codeValidation, err.Error()), nil, nil
		}

		shareIDs, denied := resolveEmails(ctx, deps, in.ShareWith)
		if denied != nil {
			return denied, nil, nil
		}

		// Atomic: link + owner + tags + shares in one transaction.
		// Governing: SPEC-0018 REQ "Database Operation Standards"
		link, err := deps.LinkStore.CreateFull(ctx, in.Slug, in.URL, user.ID, in.Title, in.Description, visibility, in.Tags, shareIDs, user.ID)
		if err != nil {
			if errors.Is(err, store.ErrSlugTaken) {
				return errorResult(codeDuplicateSlug, fmt.Sprintf("slug %q already exists", in.Slug)), nil, nil
			}
			return internalError("create link", err), nil, nil
		}

		p, err := buildLinkPayload(ctx, deps, link, true)
		if err != nil {
			return internalError("build link result", err), nil, nil
		}
		return nil, p, nil
	}
}

// --- get_link ---------------------------------------------------------------

type linkRefIn struct {
	Link string `json:"link" jsonschema:"link slug or id"`
}

func getLinkTool(deps Deps) sdk.ToolHandlerFor[linkRefIn, *linkPayload] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in linkRefIn) (*sdk.CallToolResult, *linkPayload, error) {
		user, denied := requireUser(ctx)
		if denied != nil {
			return denied, nil, nil
		}
		link, errRes := resolveLink(ctx, deps, in.Link)
		if errRes != nil {
			return errRes, nil, nil
		}

		ok, err := canRead(ctx, deps, user, link.ID)
		if err != nil {
			return internalError("authorize read", err), nil, nil
		}
		if !ok {
			return errorResult(codeForbidden, "you do not have access to this link"), nil, nil
		}

		owner, err := isOwnerOrAdmin(deps, user, link.ID)
		if err != nil {
			return internalError("authorize read", err), nil, nil
		}
		p, err := buildLinkPayload(ctx, deps, link, owner)
		if err != nil {
			return internalError("build link result", err), nil, nil
		}
		return nil, p, nil
	}
}

// --- list_links ---------------------------------------------------------------

type listLinksIn struct {
	Q      string `json:"q,omitempty" jsonschema:"search slugs, URLs, titles, and descriptions"`
	Tag    string `json:"tag,omitempty" jsonschema:"filter by tag slug (mine filter only)"`
	Filter string `json:"filter,omitempty" jsonschema:"mine (default), shared, or public"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max results, default 50, cap 200"`
}

type listLinksOut struct {
	Links []listItemPayload `json:"links"`
	Count int               `json:"count"`
}

func listLinksTool(deps Deps) sdk.ToolHandlerFor[listLinksIn, *listLinksOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in listLinksIn) (*sdk.CallToolResult, *listLinksOut, error) {
		user, denied := requireUser(ctx)
		if denied != nil {
			return denied, nil, nil
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}

		out := &listLinksOut{Links: []listItemPayload{}}
		appendLink := func(id, slug, url, title, description, visibility string) {
			out.Links = append(out.Links, listItemPayload{
				ID: id, Slug: slug, ShortURL: shortURL(ctx, slug),
				URL: url, Title: title, Description: description, Visibility: visibility,
			})
		}

		switch in.Filter {
		case "", "mine":
			var links []*store.Link
			var err error
			switch {
			case in.Q != "":
				links, err = deps.LinkStore.SearchByOwner(ctx, user.ID, in.Q)
			case in.Tag != "":
				links, err = deps.LinkStore.ListByOwnerAndTag(ctx, user.ID, in.Tag)
			default:
				links, err = deps.LinkStore.ListByOwner(ctx, user.ID)
			}
			if err != nil {
				return internalError("list links", err), nil, nil
			}
			for _, l := range links {
				if len(out.Links) >= limit {
					break
				}
				appendLink(l.ID, l.Slug, l.URL, l.Title, l.Description, l.Visibility)
			}
		case "shared":
			links, err := deps.LinkStore.ListSharedWithUser(ctx, user.ID)
			if err != nil {
				return internalError("list shared links", err), nil, nil
			}
			for _, l := range links {
				if len(out.Links) >= limit {
					break
				}
				appendLink(l.ID, l.Slug, l.URL, l.Title, l.Description, l.Visibility)
			}
		case "public":
			// Governing: SPEC-0018 REQ "Authorization Parity with the REST API"
			// — public listing never exposes other users' private/secure links.
			links, _, err := deps.LinkStore.ListPublic(ctx, user.ID, in.Q, 1, limit)
			if err != nil {
				return internalError("list public links", err), nil, nil
			}
			for _, l := range links {
				appendLink(l.ID, l.Slug, l.URL, l.Title, l.Description, l.Visibility)
			}
		default:
			return errorResult(codeValidation, `filter must be "mine", "shared", or "public"`), nil, nil
		}

		out.Count = len(out.Links)
		return nil, out, nil
	}
}

// --- update_link ---------------------------------------------------------------

type updateLinkIn struct {
	Link        string    `json:"link" jsonschema:"link slug or id"`
	URL         *string   `json:"url,omitempty"`
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Tags        *[]string `json:"tags,omitempty" jsonschema:"replaces the full tag set; [] clears all tags"`
	Visibility  *string   `json:"visibility,omitempty" jsonschema:"public, private, or secure"`
}

func updateLinkTool(deps Deps) sdk.ToolHandlerFor[updateLinkIn, *linkPayload] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in updateLinkIn) (*sdk.CallToolResult, *linkPayload, error) {
		user, denied := requireUser(ctx)
		if denied != nil {
			return denied, nil, nil
		}
		link, errRes := resolveLink(ctx, deps, in.Link)
		if errRes != nil {
			return errRes, nil, nil
		}
		ok, err := isOwnerOrAdmin(deps, user, link.ID)
		if err != nil {
			return internalError("authorize update", err), nil, nil
		}
		if !ok {
			return errorResult(codeForbidden, "only owners and admins may update a link"), nil, nil
		}

		// Overlay provided fields on current values; omitted fields unchanged.
		url, title, description, visibility := link.URL, link.Title, link.Description, link.Visibility
		if in.URL != nil {
			url = *in.URL
		}
		if in.Title != nil {
			title = *in.Title
		}
		if in.Description != nil {
			description = *in.Description
		}
		if in.Visibility != nil {
			visibility = *in.Visibility
		}
		if strings.TrimSpace(url) == "" {
			return errorResult(codeValidation, "url cannot be empty"), nil, nil
		}
		if err := store.ValidateURLVariables(url); err != nil {
			return errorResult(codeValidation, err.Error()), nil, nil
		}
		if err := store.ValidateLinkText(title, description); err != nil {
			return errorResult(codeValidation, err.Error()), nil, nil
		}
		if err := store.ValidateVisibility(visibility); err != nil {
			return errorResult(codeValidation, err.Error()), nil, nil
		}

		updated, err := deps.LinkStore.Update(ctx, link.ID, url, title, description, visibility)
		if err != nil {
			return internalError("update link", err), nil, nil
		}

		if in.Tags != nil {
			// Dedupe case-insensitively so duplicate spellings cannot roll the
			// tag write back (tags upsert by derived slug).
			seen := map[string]bool{}
			var tags []string
			for _, tag := range *in.Tags {
				k := strings.ToLower(strings.TrimSpace(tag))
				if k == "" || seen[k] {
					continue
				}
				seen[k] = true
				tags = append(tags, tag)
			}
			if err := deps.LinkStore.SetTags(ctx, updated.ID, tags); err != nil {
				return internalError("update tags", err), nil, nil
			}
		}

		p, err := buildLinkPayload(ctx, deps, updated, true)
		if err != nil {
			return internalError("build link result", err), nil, nil
		}
		return nil, p, nil
	}
}

// --- delete_link ---------------------------------------------------------------

type deleteLinkOut struct {
	Deleted bool   `json:"deleted"`
	Slug    string `json:"slug"`
}

func deleteLinkTool(deps Deps) sdk.ToolHandlerFor[linkRefIn, *deleteLinkOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in linkRefIn) (*sdk.CallToolResult, *deleteLinkOut, error) {
		user, denied := requireUser(ctx)
		if denied != nil {
			return denied, nil, nil
		}
		link, errRes := resolveLink(ctx, deps, in.Link)
		if errRes != nil {
			return errRes, nil, nil
		}
		ok, err := isOwnerOrAdmin(deps, user, link.ID)
		if err != nil {
			return internalError("authorize delete", err), nil, nil
		}
		if !ok {
			return errorResult(codeForbidden, "only owners and admins may delete a link"), nil, nil
		}
		if err := deps.LinkStore.Delete(ctx, link.ID); err != nil {
			return internalError("delete link", err), nil, nil
		}
		return nil, &deleteLinkOut{Deleted: true, Slug: link.Slug}, nil
	}
}

// --- share_link / unshare_link / add_co_owner ---------------------------------

type shareIn struct {
	Link  string `json:"link" jsonschema:"link slug or id"`
	Email string `json:"email" jsonschema:"email of an existing joe-links user"`
}

type shareOut struct {
	Link       string   `json:"link"`
	SharedWith []string `json:"shared_with"`
}

// requireShareTarget resolves the link + target user and checks owner/admin.
func requireShareTarget(ctx context.Context, deps Deps, in shareIn) (*store.Link, *store.User, *sdk.CallToolResult) {
	user, denied := requireUser(ctx)
	if denied != nil {
		return nil, nil, denied
	}
	link, errRes := resolveLink(ctx, deps, in.Link)
	if errRes != nil {
		return nil, nil, errRes
	}
	ok, err := isOwnerOrAdmin(deps, user, link.ID)
	if err != nil {
		return nil, nil, internalError("authorize share management", err)
	}
	if !ok {
		return nil, nil, errorResult(codeForbidden, "only owners and admins may manage shares")
	}
	target, err := deps.UserStore.GetByEmail(ctx, strings.TrimSpace(in.Email))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, errorResult(codeUnknownUser,
				fmt.Sprintf("no joe-links account for %s — users must sign in once before links can be shared with them", in.Email))
		}
		return nil, nil, internalError("resolve user email", err)
	}
	return link, target, nil
}

func sharesResult(ctx context.Context, deps Deps, link *store.Link) (*sdk.CallToolResult, *shareOut, error) {
	p, err := buildLinkPayload(ctx, deps, link, true)
	if err != nil {
		return internalError("list shares", err), nil, nil
	}
	shared := p.SharedWith
	if shared == nil {
		shared = []string{}
	}
	return nil, &shareOut{Link: link.Slug, SharedWith: shared}, nil
}

func shareLinkTool(deps Deps) sdk.ToolHandlerFor[shareIn, *shareOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in shareIn) (*sdk.CallToolResult, *shareOut, error) {
		user := auth.UserFromContext(ctx)
		link, target, errRes := requireShareTarget(ctx, deps, in)
		if errRes != nil {
			return errRes, nil, nil
		}
		has, err := deps.LinkStore.HasShare(ctx, link.ID, target.ID)
		if err != nil {
			return internalError("check existing share", err), nil, nil
		}
		if has {
			return errorResult(codeDuplicateShare, fmt.Sprintf("%s already has access to %s", in.Email, link.Slug)), nil, nil
		}
		if err := deps.LinkStore.AddShare(ctx, link.ID, target.ID, user.ID); err != nil {
			return internalError("add share", err), nil, nil
		}
		return sharesResult(ctx, deps, link)
	}
}

func unshareLinkTool(deps Deps) sdk.ToolHandlerFor[shareIn, *shareOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in shareIn) (*sdk.CallToolResult, *shareOut, error) {
		link, target, errRes := requireShareTarget(ctx, deps, in)
		if errRes != nil {
			return errRes, nil, nil
		}
		if err := deps.LinkStore.RemoveShare(ctx, link.ID, target.ID); err != nil {
			return internalError("remove share", err), nil, nil
		}
		return sharesResult(ctx, deps, link)
	}
}

type coOwnerOut struct {
	Link   string         `json:"link"`
	Owners []ownerPayload `json:"owners"`
}

func addCoOwnerTool(deps Deps) sdk.ToolHandlerFor[shareIn, *coOwnerOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in shareIn) (*sdk.CallToolResult, *coOwnerOut, error) {
		link, target, errRes := requireShareTarget(ctx, deps, in)
		if errRes != nil {
			return errRes, nil, nil
		}
		if err := deps.LinkStore.AddOwner(ctx, link.ID, target.ID); err != nil {
			if errors.Is(err, store.ErrDuplicateOwner) || errors.Is(err, store.ErrAlreadyOwner) {
				return errorResult(codeDuplicateOwner, fmt.Sprintf("%s already owns %s", in.Email, link.Slug)), nil, nil
			}
			return internalError("add co-owner", err), nil, nil
		}
		p, err := buildLinkPayload(ctx, deps, link, false)
		if err != nil {
			return internalError("list owners", err), nil, nil
		}
		return nil, &coOwnerOut{Link: link.Slug, Owners: p.Owners}, nil
	}
}

// --- get_link_stats -------------------------------------------------------------

type statsIn struct {
	Link  string `json:"link" jsonschema:"link slug or id"`
	Limit int    `json:"limit,omitempty" jsonschema:"max recent clicks, default 20, cap 100"`
}

type recentClickPayload struct {
	ClickedAt time.Time `json:"clicked_at"`
	Referrer  string    `json:"referrer,omitempty"`
	User      string    `json:"user,omitempty" jsonschema:"display name of the authenticated clicker, empty for anonymous"`
}

type statsOut struct {
	Link    string               `json:"link"`
	Total   int64                `json:"total"`
	Last7d  int64                `json:"last_7d"`
	Last30d int64                `json:"last_30d"`
	Recent  []recentClickPayload `json:"recent"`
}

func getLinkStatsTool(deps Deps) sdk.ToolHandlerFor[statsIn, *statsOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in statsIn) (*sdk.CallToolResult, *statsOut, error) {
		user, denied := requireUser(ctx)
		if denied != nil {
			return denied, nil, nil
		}
		link, errRes := resolveLink(ctx, deps, in.Link)
		if errRes != nil {
			return errRes, nil, nil
		}
		// REST stats rule: owners and admins only.
		// Governing: SPEC-0018 REQ "Authorization Parity with the REST API"
		ok, err := isOwnerOrAdmin(deps, user, link.ID)
		if err != nil {
			return internalError("authorize stats", err), nil, nil
		}
		if !ok {
			return errorResult(codeForbidden, "only owners and admins may view stats"), nil, nil
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		stats, err := deps.ClickStore.GetClickStats(ctx, link.ID)
		if err != nil {
			return internalError("load click stats", err), nil, nil
		}
		recent, err := deps.ClickStore.ListRecentClicks(ctx, link.ID, limit)
		if err != nil {
			return internalError("load recent clicks", err), nil, nil
		}

		out := &statsOut{Link: link.Slug, Total: stats.Total, Last7d: stats.Last7d, Last30d: stats.Last30d, Recent: []recentClickPayload{}}
		for _, c := range recent {
			out.Recent = append(out.Recent, recentClickPayload{ClickedAt: c.ClickedAt, Referrer: c.Referrer, User: c.DisplayName})
		}
		return nil, out, nil
	}
}

// --- suggest_link_metadata -------------------------------------------------------

type suggestIn struct {
	URL string `json:"url" jsonschema:"destination URL to suggest metadata for"`
}

type suggestOut struct {
	Slug        string   `json:"slug,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func suggestTool(deps Deps) sdk.ToolHandlerFor[suggestIn, *suggestOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, in suggestIn) (*sdk.CallToolResult, *suggestOut, error) {
		if _, denied := requireUser(ctx); denied != nil {
			return denied, nil, nil
		}
		if strings.TrimSpace(in.URL) == "" {
			return errorResult(codeValidation, "url is required"), nil, nil
		}
		resp, err := deps.Suggester.Suggest(ctx, llm.SuggestRequest{URL: in.URL})
		if err != nil {
			var malformed *llm.MalformedResponseError
			if errors.As(err, &malformed) {
				slog.Error("mcp: LLM suggest malformed response", "error", malformed.Err)
			} else {
				slog.Error("mcp: LLM suggest", "error", err)
			}
			return errorResult(codeLLMError, "LLM provider error"), nil, nil
		}
		// Mirror the REST behavior: blank out invalid slugs, keep the rest.
		// Governing: SPEC-0017 REQ "Default Prompt Template"
		slug := resp.Slug
		if slug != "" && store.ValidateSlugFormat(slug) != nil {
			slug = ""
		}
		return nil, &suggestOut{Slug: slug, Title: resp.Title, Description: resp.Description, Tags: resp.Tags}, nil
	}
}

// --- list_keywords ----------------------------------------------------------------

type keywordsOut struct {
	Keywords []string `json:"keywords"`
}

func listKeywordsTool(deps Deps) sdk.ToolHandlerFor[any, *keywordsOut] {
	return func(ctx context.Context, req *sdk.CallToolRequest, _ any) (*sdk.CallToolResult, *keywordsOut, error) {
		if _, denied := requireUser(ctx); denied != nil {
			return denied, nil, nil
		}
		list, err := deps.KeywordStore.List(ctx)
		if err != nil {
			return internalError("list keywords", err), nil, nil
		}
		out := &keywordsOut{Keywords: []string{}}
		for _, k := range list {
			out.Keywords = append(out.Keywords, k.Keyword)
		}
		return nil, out, nil
	}
}

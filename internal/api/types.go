// Governing: SPEC-0005 REQ "API Response Structures", SPEC-0007 REQ "Request/Response Type Declarations", ADR-0008
package api

import "time"

// ErrorResponse is the standard error shape.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// OwnerResponse represents a link owner.
type OwnerResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	IsPrimary bool   `json:"is_primary"`
}

// LinkResponse is the full link resource.
// Governing: SPEC-0005 REQ "API Response Structures"
type LinkResponse struct {
	ID          string          `json:"id"`
	Slug        string          `json:"slug"`
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Owners      []OwnerResponse `json:"owners"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// LinkListResponse wraps a paginated list of links.
// Governing: SPEC-0005 REQ "Pagination"
type LinkListResponse struct {
	Links      []*LinkResponse `json:"links"`
	NextCursor *string         `json:"next_cursor"`
}

// CreateLinkRequest is the body for POST /api/v1/links.
// Governing: SPEC-0005 REQ "Links Collection"
type CreateLinkRequest struct {
	Slug        string   `json:"slug"`
	URL         string   `json:"url"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// UpdateLinkRequest is the body for PUT /api/v1/links/{id}.
// Governing: SPEC-0005 REQ "Link Resource" — slug is intentionally omitted (immutable).
type UpdateLinkRequest struct {
	URL         string   `json:"url"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// AddOwnerRequest is the body for POST /api/v1/links/{id}/owners.
// Governing: SPEC-0005 REQ "Co-Owner Management"
type AddOwnerRequest struct {
	Email string `json:"email"`
}

// TagResponse represents a tag with its link count.
// Governing: SPEC-0005 REQ "API Response Structures"
type TagResponse struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	LinkCount int    `json:"link_count"`
}

// TagListResponse wraps a paginated list of tags.
// Governing: SPEC-0005 REQ "Pagination"
type TagListResponse struct {
	Tags       []*TagResponse `json:"tags"`
	NextCursor *string        `json:"next_cursor"`
}

// UserResponse represents a user profile.
type UserResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserListResponse wraps a paginated list of users.
// Governing: SPEC-0005 REQ "Pagination"
type UserListResponse struct {
	Users      []*UserResponse `json:"users"`
	NextCursor *string         `json:"next_cursor"`
}

// UpdateRoleRequest is the body for PUT /api/v1/admin/users/{id}/role.
// Governing: SPEC-0005 REQ "Admin Endpoints"
type UpdateRoleRequest struct {
	Role string `json:"role"`
}

// TokenResponse is the API token representation (never includes token_hash).
type TokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// TokenCreatedResponse is returned only on POST /api/v1/tokens — includes plaintext once.
type TokenCreatedResponse struct {
	TokenResponse
	Token string `json:"token"`
}

// TokenListResponse wraps a list of tokens.
type TokenListResponse struct {
	Tokens []*TokenResponse `json:"tokens"`
}

// CreateTokenRequest is the body for POST /api/v1/tokens.
type CreateTokenRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

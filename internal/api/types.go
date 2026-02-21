// Governing: SPEC-0005 REQ "API Response Structures", ADR-0008
package api

import "time"

// --- Link types ---

// CreateLinkRequest is the request body for POST /api/v1/links.
// Governing: SPEC-0005 REQ "Links Collection"
type CreateLinkRequest struct {
	Slug        string   `json:"slug"`
	URL         string   `json:"url"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// UpdateLinkRequest is the request body for PUT /api/v1/links/{id}.
// Governing: SPEC-0005 REQ "Link Resource" — slug is intentionally omitted (immutable).
type UpdateLinkRequest struct {
	URL         string   `json:"url"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// OwnerResponse represents a link owner in API responses.
type OwnerResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	IsPrimary bool   `json:"is_primary"`
}

// LinkResponse is the JSON representation of a single link.
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

// LinkListResponse is the paginated response for link list endpoints.
// Governing: SPEC-0005 REQ "Pagination"
type LinkListResponse struct {
	Links      []LinkResponse `json:"links"`
	NextCursor *string        `json:"next_cursor"`
}

// --- Token types ---

// CreateTokenRequest is the request body for POST /api/v1/tokens.
type CreateTokenRequest struct {
	Name      string `json:"name"`
	ExpiresIn string `json:"expires_in,omitempty"`
}

// TokenResponse is the JSON representation of an API token.
type TokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Token      string     `json:"token,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// TokenListResponse is the paginated response for token list endpoints.
// Governing: SPEC-0005 REQ "Pagination"
type TokenListResponse struct {
	Tokens     []TokenResponse `json:"tokens"`
	NextCursor *string         `json:"next_cursor"`
}

// --- User types ---

// UserResponse is the JSON representation of a user.
type UserResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserListResponse is the paginated response for user list endpoints.
// Governing: SPEC-0005 REQ "Pagination"
type UserListResponse struct {
	Users      []UserResponse `json:"users"`
	NextCursor *string        `json:"next_cursor"`
}

// UpdateRoleRequest is the request body for PUT /api/v1/admin/users/{id}/role.
// Governing: SPEC-0005 REQ "Admin Endpoints"
type UpdateRoleRequest struct {
	Role string `json:"role"`
}

// --- Tag types ---

// TagResponse is the JSON representation of a tag.
// Governing: SPEC-0005 REQ "API Response Structures"
type TagResponse struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	LinkCount int    `json:"link_count"`
}

// TagListResponse is the paginated response for tag list endpoints.
// Governing: SPEC-0005 REQ "Pagination"
type TagListResponse struct {
	Tags       []TagResponse `json:"tags"`
	NextCursor *string       `json:"next_cursor"`
}

// --- Co-owner types ---

// AddOwnerRequest is the request body for POST /api/v1/links/{id}/owners.
// Governing: SPEC-0005 REQ "Co-Owner Management"
type AddOwnerRequest struct {
	Email string `json:"email"`
}

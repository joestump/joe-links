// Governing: SPEC-0002 REQ "Link Store Interface", ADR-0005
package store

import (
	"context"
	"errors"
)

var (
	// ErrNotFound is returned when a requested entity does not exist.
	ErrNotFound = errors.New("not found")

	// ErrDuplicateOwner is returned when attempting to add an owner that already exists.
	ErrDuplicateOwner = errors.New("user is already an owner of this link")
)

// LinkStoreIface exposes all link data operations.
// No handler MAY query the DB directly; all access goes through this interface.
// Governing: SPEC-0002 REQ "Link Store Interface"
type LinkStoreIface interface {
	Create(ctx context.Context, slug, url, ownerID, title, description string) (*Link, error)
	GetBySlug(ctx context.Context, slug string) (*Link, error)
	GetByID(ctx context.Context, id string) (*Link, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*Link, error)
	ListAll(ctx context.Context) ([]*Link, error)
	Update(ctx context.Context, id, url, title, description string) (*Link, error)
	Delete(ctx context.Context, id string) error
	AddOwner(ctx context.Context, linkID, userID string) error
	RemoveOwner(ctx context.Context, linkID, userID string) error
	SetTags(ctx context.Context, linkID string, tagNames []string) error
	ListTags(ctx context.Context, linkID string) ([]*Tag, error)
	ListByTag(ctx context.Context, tagSlug string) ([]*Link, error)
}

// TagStoreIface exposes tag operations.
// Governing: SPEC-0002 REQ "Link Store Interface"
type TagStoreIface interface {
	Upsert(ctx context.Context, name string) (*Tag, error)
	GetBySlug(ctx context.Context, slug string) (*Tag, error)
	ListAll(ctx context.Context) ([]*Tag, error)
}

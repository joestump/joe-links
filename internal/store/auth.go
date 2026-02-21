// Governing: SPEC-0002 REQ "Authorization Based on Ownership", ADR-0005
package store

// IsOwnerOrAdmin returns true if userID appears in link_owners for linkID, OR role == "admin".
// Governing: SPEC-0002 REQ "Authorization Based on Ownership"
func IsOwnerOrAdmin(ownerStore *OwnershipStore, linkID, userID, role string) (bool, error) {
	if role == "admin" {
		return true, nil
	}
	return ownerStore.IsOwner(linkID, userID)
}

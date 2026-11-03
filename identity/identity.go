// Package identity defines opaque identities shared by canonical IR and
// display output.
package identity

// NodeID is an opaque host-assigned semantic node identity.
//
// The zero value means unassigned. Untie never derives a NodeID from source
// offsets. A host that needs identity across edits must preserve the same
// nonzero ID on the semantically corresponding node when it constructs a new
// tree. Cloning, validation, and layout preserve assigned IDs.
type NodeID uint64

// Assigned reports whether the identity is nonzero.
func (id NodeID) Assigned() bool {
	return id != 0
}

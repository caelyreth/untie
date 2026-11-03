package identity

import "testing"

func TestNodeIDAssigned(t *testing.T) {
	t.Parallel()

	if NodeID(0).Assigned() {
		t.Fatal("zero NodeID reported assigned")
	}
	if !NodeID(1).Assigned() {
		t.Fatal("nonzero NodeID reported unassigned")
	}
}

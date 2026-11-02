package display

import (
	"testing"

	"github.com/caelyreth/untie/geometry"
)

func TestCloneIndependent(t *testing.T) {
	t.Parallel()

	box := &Box{
		Width: 10,
		Items: []Item{{
			Kind: ItemGlyphPath,
			Path: geometry.Path{{Op: geometry.SegMoveTo}},
		}},
	}
	got := box.Clone()
	got.Items[0].Path[0].Op = geometry.SegClose
	if box.Items[0].Path[0].Op != geometry.SegMoveTo {
		t.Fatal("Clone shared path storage")
	}
}

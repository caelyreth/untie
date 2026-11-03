package typst

import (
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestArenaHintHonorsNodeBudget(t *testing.T) {
	t.Parallel()

	if got := arenaHint(1<<20, source.Limits{MaxNodes: 1}); got != 1 {
		t.Fatalf("arenaHint(huge, MaxNodes=1) = %d, want 1", got)
	}
}

package untie

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/caelyreth/untie/source"
)

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if expr, err := ParseContext(ctx, "x", TeX, Inline, ParseOptions{}); expr != nil ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("ParseContext(canceled) = %v, %v", expr, err)
	}
	//nolint:staticcheck // This assertion intentionally verifies nil-context handling.
	if expr, err := ParseContext(nil, "x", TeX, Inline, ParseOptions{}); expr != nil ||
		!errors.Is(err, ErrInvalidContext) {
		t.Fatalf("ParseContext(nil context) = %v, %v", expr, err)
	}

	lastCheck := &cancelAfterContext{cancelAt: 2}
	if expr, err := ParseContext(lastCheck, "x", TeX, Inline, ParseOptions{}); expr != nil ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("ParseContext(end cancellation) = %v, %v", expr, err)
	}
}

type cancelAfterContext struct {
	calls    int
	cancelAt int
}

func (c *cancelAfterContext) Deadline() (deadlineTime time.Time, ok bool) {
	return deadlineTime, false
}

func (c *cancelAfterContext) Done() <-chan struct{} {
	return nil
}

func (c *cancelAfterContext) Err() error {
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func (c *cancelAfterContext) Value(any) any {
	return nil
}

func TestSnapshotParseOptionsCopiesMacrosAndNormalizesLimits(t *testing.T) {
	t.Parallel()

	macros := map[string]Macro{"a": {Body: "A"}}
	opts := snapshotParseOptions(ParseOptions{
		Macros: macros,
		Limits: source.Limits{MaxNodes: -1},
	})
	macros["a"] = Macro{Body: "changed"}
	if opts.Macros["a"].Body != "A" {
		t.Fatal("snapshotParseOptions shared its macro map")
	}
	if opts.Limits != source.DefaultLimits() {
		t.Fatalf("snapshot limits = %+v, want defaults", opts.Limits)
	}
}

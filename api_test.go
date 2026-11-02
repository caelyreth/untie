package untie

import (
	"context"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestParseCopiesOptionsAndDiagnostics(t *testing.T) {
	t.Parallel()

	macros := map[string]Macro{
		"R": {Body: `\mathbb{R}`, Args: 0},
	}
	opts := ParseOptions{Macros: macros, Preamble: "x", Limits: source.Limits{MaxSourceBytes: 100}}
	expr := Parse("x", TeX, Inline, opts)
	macros["R"] = Macro{Body: "changed"}
	opts.Limits.MaxSourceBytes = 1

	if expr.Key() == 0 {
		t.Fatal("Key() = 0")
	}
	if got := expr.Root(); got == nil {
		t.Fatal("Root() = nil")
	}
	diagnostics := expr.Diagnostics()
	if len(diagnostics) > 0 {
		diagnostics[0].Message = "mutated"
		if expr.Diagnostics()[0].Message == "mutated" {
			t.Fatal("Diagnostics() returned shared storage")
		}
	}
}

func TestParseRecoversPanics(t *testing.T) {
	t.Parallel()

	// An invalid dialect exercises the normal diagnostic path; this test mainly
	// protects the public panic boundary when a frontend regresses.
	expr := Parse("x", Dialect(99), Inline, ParseOptions{})
	if expr == nil || len(expr.Diagnostics()) == 0 {
		t.Fatal("Parse() did not return a diagnosed fallback")
	}
	if !strings.Contains(expr.Diagnostics()[0].Message, "dialect") {
		t.Fatalf("diagnostic = %+v", expr.Diagnostics()[0])
	}
}

func TestKeyIncludesLimits(t *testing.T) {
	t.Parallel()

	a := Parse("x", Typst, Inline, ParseOptions{Limits: source.Limits{MaxNodes: 10}})
	b := Parse("x", Typst, Inline, ParseOptions{Limits: source.Limits{MaxNodes: 11}})
	if a.Key() == b.Key() {
		t.Fatal("parse key does not include limits")
	}
}

func TestKeyIgnoresMapInsertionOrder(t *testing.T) {
	t.Parallel()

	a := Parse("x", TeX, Inline, ParseOptions{Macros: map[string]Macro{
		"a": {Body: "A"},
		"b": {Body: "B"},
	}})
	b := Parse("x", TeX, Inline, ParseOptions{Macros: map[string]Macro{
		"b": {Body: "B"},
		"a": {Body: "A"},
	}})
	if a.Key() != b.Key() {
		t.Fatalf("keys differ: %d != %d", a.Key(), b.Key())
	}
}

func TestParseLimitDiagnostic(t *testing.T) {
	t.Parallel()

	expr := Parse("abcdef", Typst, Inline, ParseOptions{Limits: source.Limits{MaxSourceBytes: 3}})
	diagnostics := expr.Diagnostics()
	if len(diagnostics) != 1 || diagnostics[0].Code != source.CodeLimitExceeded {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestLayoutContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LayoutContext(ctx, Parse("x", TeX, Inline, ParseOptions{}), nil, Style{Size: 16}); err == nil {
		t.Fatal("LayoutContext() error = nil, want cancellation")
	}
}

func TestFallbackIRValidates(t *testing.T) {
	t.Parallel()

	expr := Parse(`\frac{1}{2}`, TeX, Inline, ParseOptions{})
	if err := ir.Validate(expr.Root(), source.Limits{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

package untie

import (
	"context"
	"errors"
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

	expr := parseRecovered(
		"x",
		TeX,
		Inline,
		ParseOptions{},
		source.DefaultLimits(),
		func(source.Limits) frontendResult {
			panic("injected frontend failure")
		},
	)
	if expr == nil {
		t.Fatal("parseRecovered() = nil")
	}
	diagnostics := expr.Diagnostics()
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "injected frontend failure") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	if expr.Key() == 0 {
		t.Fatal("recovered expression has zero key")
	}
	if err := ir.ValidateSource(expr.Root(), "x", source.Limits{}); err != nil {
		t.Fatalf("recovered Root() invalid: %v", err)
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

func TestLayoutContextRejectsNilAndDoesNotReportFalseSuccess(t *testing.T) {
	t.Parallel()

	expr := Parse("x", TeX, Inline, ParseOptions{})
	//nolint:staticcheck // This assertion intentionally verifies nil-context handling.
	if _, err := LayoutContext(nil, expr, nil, Style{}); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("nil context error = %v", err)
	}
	if _, err := LayoutContext(context.Background(), nil, nil, Style{}); !errors.Is(err, ErrInvalidExpr) {
		t.Fatalf("nil expression error = %v", err)
	}
	if _, err := LayoutContext(context.Background(), expr, nil, Style{}); !errors.Is(err, ErrInvalidFonts) {
		t.Fatalf("nil fonts error = %v", err)
	}
	if box, err := LayoutContext(context.Background(), expr, &FontSet{}, Style{}); !errors.Is(err, ErrLayoutNotImplemented) || box != nil {
		t.Fatalf("LayoutContext() = %v, %v, want nil, ErrLayoutNotImplemented", box, err)
	}
	if box := Layout(expr, &FontSet{}, Style{}); box != nil {
		t.Fatalf("Layout() = %v, want nil while backend is unavailable", box)
	}
}

func TestRootReturnsIndependentSnapshots(t *testing.T) {
	t.Parallel()

	expr := Parse(`\frac{1}{2}`, TeX, Inline, ParseOptions{})
	key := expr.Key()
	first, err := expr.RootSnapshot()
	if err != nil {
		t.Fatalf("RootSnapshot() error = %v", err)
	}
	row := first.Root.(*ir.Row)
	ir.Walk(first.Root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok {
			glyph.Text = "changed"
			return false
		}
		return true
	})
	row.Items = row.Items[:0]

	second := expr.Root()
	if second == nil {
		t.Fatal("Root() = nil after snapshot mutation")
	}
	secondRow := second.Root.(*ir.Row)
	if len(secondRow.Items) != 1 {
		t.Fatalf("second snapshot items = %d, want 1", len(secondRow.Items))
	}
	restored := false
	ir.Walk(second.Root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok && glyph.Text == "1" {
			restored = true
		}
		return true
	})
	if !restored {
		t.Fatalf("second snapshot was mutated: %#v", secondRow.Items[0])
	}
	if expr.Key() != key {
		t.Fatalf("Key changed from %d to %d", key, expr.Key())
	}
	if second == expr.root || secondRow == expr.root.Root {
		t.Fatal("Root() exposed internal storage")
	}
	if expr.rootForLayout() != expr.root {
		t.Fatal("internal layout path unexpectedly copied the root")
	}
}

func TestTextLimitAffectsParsedFallback(t *testing.T) {
	t.Parallel()

	const src = "abcdefgh"
	expr := Parse(src, Typst, Inline, ParseOptions{Limits: source.Limits{
		MaxSourceBytes: len(src),
		MaxTextBytes:   3,
	}})
	root := expr.Root()
	var texts []string
	ir.Walk(root.Root, func(node ir.Node) bool {
		if text, ok := node.(*ir.ErrorNode); ok {
			texts = append(texts, text.Text)
		}
		return true
	})
	if len(texts) != 1 || len(texts[0]) > 3 {
		t.Fatalf("fallback texts = %q, want one <= 3 bytes", texts)
	}
}

func TestNodeLimitAffectsParse(t *testing.T) {
	t.Parallel()

	expr := Parse("x+y", TeX, Inline, ParseOptions{Limits: source.Limits{MaxNodes: 1}})
	foundLimit := false
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Code == source.CodeLimitExceeded {
			foundLimit = true
		}
	}
	if !foundLimit {
		t.Fatalf("diagnostics = %+v, want limit diagnostic", expr.Diagnostics())
	}
}

func TestFallbackIRValidates(t *testing.T) {
	t.Parallel()

	expr := Parse(`\frac{1}{2}`, TeX, Inline, ParseOptions{})
	if err := ir.Validate(expr.Root(), source.Limits{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

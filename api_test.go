package untie

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
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

func TestKeyNormalizesDefaultLimits(t *testing.T) {
	t.Parallel()

	zero := Parse("x", TeX, Inline, ParseOptions{})
	defaults := Parse("x", TeX, Inline, ParseOptions{Limits: source.DefaultLimits()})
	if zero.Key() != defaults.Key() {
		t.Fatalf("zero and explicit default limits have different keys: %d != %d", zero.Key(), defaults.Key())
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
	fonts := loadTestFonts(t)
	//nolint:staticcheck // This assertion intentionally verifies nil-context handling.
	if _, err := LayoutContext(nil, expr, fonts, Style{Size: 16}); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("nil context error = %v", err)
	}
	if _, err := LayoutContext(context.Background(), nil, nil, Style{}); !errors.Is(err, ErrInvalidExpr) {
		t.Fatalf("nil expression error = %v", err)
	}
	if _, err := LayoutContext(context.Background(), expr, nil, Style{}); !errors.Is(err, ErrInvalidFonts) {
		t.Fatalf("nil fonts error = %v", err)
	}
	if box, err := LayoutContext(context.Background(), expr, fonts, Style{}); !errors.Is(err, ErrInvalidStyle) || box != nil {
		t.Fatalf("LayoutContext(invalid style) = %v, %v, want nil, ErrInvalidStyle", box, err)
	}
	box, err := LayoutContext(context.Background(), expr, fonts, Style{Size: 16})
	if err != nil {
		t.Fatalf("LayoutContext() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("LayoutContext() box invalid: %v", err)
	}
	if box, err := Layout(expr, fonts, Style{Size: 16}); err != nil || box == nil {
		t.Fatalf("Layout() = %v, %v", box, err)
	}
}

func loadTestFonts(t testing.TB) *FontSet {
	t.Helper()
	data := testFontBytes(t)
	fonts, err := LoadFonts(Fonts{Math: FontData{Bytes: data, ID: "NewCMMath-Book"}})
	if err != nil {
		t.Fatalf("LoadFonts() error = %v", err)
	}
	return fonts
}

func testFontBytes(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatalf("read test font: %v", err)
	}
	return data
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

func TestParseBoundaryRejectsOutOfSourceFrontendIR(t *testing.T) {
	t.Parallel()

	expr := parseRecovered(
		"x",
		TeX,
		Inline,
		ParseOptions{},
		source.DefaultLimits(),
		func(source.Limits) frontendResult {
			return frontendResult{root: ir.NewGlyph(source.NewRange(0, 99), "bad")}
		},
	)
	diagnostics := expr.Diagnostics()
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "invalid IR") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	if err := ir.ValidateSource(expr.Root(), "x", source.Limits{}); err != nil {
		t.Fatalf("fallback Root() invalid: %v", err)
	}
}

func TestLayoutLimitsFlowThroughFacade(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	tests := []struct {
		name   string
		limits source.Limits
	}{
		{name: "output items", limits: source.Limits{MaxOutputItems: 1}},
		{name: "path segments", limits: source.Limits{MaxPathSegments: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expr := Parse("x+y", TeX, Inline, ParseOptions{Limits: tc.limits})
			if box, err := LayoutContext(context.Background(), expr, fonts, Style{Size: 16}); err == nil || box != nil {
				t.Fatalf("LayoutContext() = %v, %v, want limit error", box, err)
			}
		})
	}
}

func TestFallbackIRValidates(t *testing.T) {
	t.Parallel()

	expr := Parse(`\frac{1}{2}`, TeX, Inline, ParseOptions{})
	if err := ir.Validate(expr.Root(), source.Limits{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLayoutEmitsSourceAndMacroOriginHits(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	expr := ParseTeX(`\sq{x}`, Inline, ParseOptions{
		Macros: map[string]Macro{"sq": {Body: `#1^2`, Args: 1}},
	})
	box, err := LayoutContext(context.Background(), expr, fonts, Style{Size: 16})
	if err != nil {
		t.Fatal(err)
	}
	var foundArgument, foundScript bool
	for _, hit := range box.Hits {
		if hit.Source == source.NewRange(4, 5) && hit.Kind == display.HitAtom {
			foundArgument = true
		}
		if hit.Source == source.NewRange(0, 6) && hit.Kind == display.HitScript {
			foundScript = true
		}
	}
	if !foundArgument || !foundScript {
		t.Fatalf("hits = %#v, want macro argument atom and call-origin script", box.Hits)
	}
	hit, ok := box.At(geometry.Point{X: 4, Y: 0})
	if !ok || hit.Source != source.NewRange(4, 5) || hit.Kind != display.HitAtom {
		t.Fatalf("At() = (%#v, %v), want innermost macro argument", hit, ok)
	}
}

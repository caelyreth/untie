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
	opts := ParseOptions{Macros: macros, Limits: source.Limits{MaxSourceBytes: 100}}
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

func TestLayoutPopulatesAltWithoutChangingGeometry(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	expr := ParseTeX(`x_i^2+\frac{1}{2}`, Inline, ParseOptions{})
	box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16})
	if err != nil {
		t.Fatalf("LayoutContext() error = %v", err)
	}
	if box.Alt != expr.Alt() {
		t.Fatalf("Box.Alt = %#v, want %#v", box.Alt, expr.Alt())
	}
	if box.Alt.Text == "" || box.Alt.Speech == "" {
		t.Fatalf("Box.Alt = %#v, want visible text and speech", box.Alt)
	}
	if err := box.Alt.Validate(); err != nil {
		t.Fatalf("Box.Alt.Validate() error = %v", err)
	}

	plain := box.Clone()
	plain.Alt = Alt{}
	if len(plain.Items) != len(box.Items) || plain.Width != box.Width || plain.Ascent != box.Ascent {
		t.Fatal("alt population changed geometry")
	}

	limited := ParseTeX(`(x)`, Inline, ParseOptions{Limits: Limits{
		MaxSourceBytes: 3,
		MaxTextBytes:   5,
	}})
	box, err = engine.LayoutContext(context.Background(), limited, Style{Size: 16})
	if err != nil {
		t.Fatalf("LayoutContext(limited alt) error = %v", err)
	}
	if box.Alt.Text == "" || box.Alt.Speech == "" || len(box.Alt.Text) > 5 || len(box.Alt.Speech) > 5 {
		t.Fatalf("bounded alt fallback = %#v", box.Alt)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("layout with bounded alt fallback is invalid: %v", err)
	}
}

func TestLayoutAltUsesStricterEngineLimits(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{MaxTextBytes: 1})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	expr := ParseTeX(`x_i^2`, Inline, ParseOptions{})
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Code == source.CodeInternal {
			t.Fatalf("parse introduced internal diagnostic: %+v", diagnostic)
		}
	}

	box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16})
	if err != nil {
		t.Fatalf("LayoutContext() error = %v", err)
	}
	if box.Alt.Text == "" || box.Alt.Speech == "" {
		t.Fatalf("Box.Alt = %#v, want visible bounded fallback", box.Alt)
	}
	if len(box.Alt.Text) > 1 || len(box.Alt.Speech) > 1 {
		t.Fatalf("Box.Alt exceeds engine MaxTextBytes=1: %#v", box.Alt)
	}
	if err := box.Alt.Validate(); err != nil {
		t.Fatalf("Box.Alt.Validate() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Box.Validate() error = %v", err)
	}
	if box.Alt == expr.Alt() {
		t.Fatalf("Box.Alt = %#v, want stricter Engine limits to affect output", box.Alt)
	}
}

func TestIntersectLimitsUsesPerFieldMinimum(t *testing.T) {
	t.Parallel()

	left := Limits{
		MaxSourceBytes:     20,
		MaxTokens:          21,
		MaxNesting:         22,
		MaxNodes:           23,
		MaxMacroExpansions: 24,
		MaxExpandedTokens:  25,
		MaxCallDepth:       26,
		MaxTableCells:      27,
		MaxAssemblyRepeats: 28,
		MaxTextBytes:       29,
		MaxOutputItems:     30,
		MaxPathSegments:    31,
		MaxDimension:       32,
	}
	right := Limits{
		MaxSourceBytes:     19,
		MaxTokens:          22,
		MaxNesting:         21,
		MaxNodes:           24,
		MaxMacroExpansions: 23,
		MaxExpandedTokens:  26,
		MaxCallDepth:       25,
		MaxTableCells:      28,
		MaxAssemblyRepeats: 27,
		MaxTextBytes:       30,
		MaxOutputItems:     29,
		MaxPathSegments:    32,
		MaxDimension:       31,
	}
	want := Limits{
		MaxSourceBytes:     19,
		MaxTokens:          21,
		MaxNesting:         21,
		MaxNodes:           23,
		MaxMacroExpansions: 23,
		MaxExpandedTokens:  25,
		MaxCallDepth:       25,
		MaxTableCells:      27,
		MaxAssemblyRepeats: 27,
		MaxTextBytes:       29,
		MaxOutputItems:     29,
		MaxPathSegments:    31,
		MaxDimension:       31,
	}
	if got := intersectLimits(left, right); got != want {
		t.Fatalf("intersectLimits() = %+v, want %+v", got, want)
	}
}

func TestLayoutAltRecognizesExactTeXSpaceLengths(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	tests := []struct {
		source string
		text   string
		speech string
	}{
		{source: `a\enspace b`, text: "a\u2002b", speech: "a en space b"},
		{source: `a\quad b`, text: "a\u2003b", speech: "a quad space b"},
		{source: `a\qquad b`, text: "a\u2003\u2003b", speech: "a double quad space b"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			expr := ParseTeX(tc.source, Inline, ParseOptions{})
			exprAlt := expr.Alt()
			if exprAlt.Text != tc.text || exprAlt.Speech != tc.speech {
				t.Fatalf("Expr.Alt() = %#v, want Text=%q Speech=%q", exprAlt, tc.text, tc.speech)
			}
			box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16})
			if err != nil {
				t.Fatalf("LayoutContext() error = %v", err)
			}
			if box.Alt != exprAlt {
				t.Fatalf("Box.Alt = %#v, want %#v", box.Alt, exprAlt)
			}
		})
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
	if _, err := expr.RootSnapshot(); err != nil {
		t.Fatalf("fallback RootSnapshot() error = %v", err)
	}
}

func TestFallbackHonorsExactOneNodeBudget(t *testing.T) {
	t.Parallel()

	limits := source.Limits{MaxNodes: 1}
	recovered := parseRecovered(
		"xy",
		TeX,
		Inline,
		ParseOptions{},
		limits.WithDefaults(),
		func(source.Limits) frontendResult {
			panic("frontend failure")
		},
	)
	tests := []struct {
		name string
		expr *Expr
	}{
		{name: "byte limit", expr: Parse("xy", TeX, Inline, ParseOptions{Limits: source.Limits{MaxSourceBytes: 1, MaxNodes: 1}})},
		{name: "unknown dialect", expr: Parse("xy", Dialect(255), Inline, ParseOptions{Limits: limits})},
		{name: "recovered panic", expr: recovered},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.expr.Diagnostics()) == 0 {
				t.Fatal("expected a diagnostic")
			}
			if _, err := test.expr.RootSnapshot(); err != nil {
				t.Fatalf("RootSnapshot() error = %v", err)
			}
		})
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

func TestParsedLayoutRetainsStricterEngineTraversalLimit(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{MaxNodes: 1})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	expr := ParseTeX(`x+y`, Inline, ParseOptions{})
	if box, err := engine.Layout(expr, Style{Size: 16}); err == nil || box != nil {
		t.Fatalf("Layout() = %v, %v, want engine traversal-limit error", box, err)
	}
}

func TestPublicLayoutErrorClassification(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	missing := ParseTeX("中", Inline, ParseOptions{})
	if _, err := engine.Layout(missing, Style{Size: 16}); !errors.Is(err, ErrMissingGlyph) {
		t.Fatalf("missing-glyph layout error = %v", err)
	}

	limited, err := NewEngine(fonts, Limits{MaxOutputItems: 1})
	if err != nil {
		t.Fatalf("NewEngine(limited) error = %v", err)
	}
	expr := ParseTeX("x+y", Inline, ParseOptions{})
	if _, err := limited.Layout(expr, Style{Size: 16}); !errors.Is(err, ErrLimit) {
		t.Fatalf("limited layout error = %v", err)
	}
}

func TestExprHasErrorsUsesRootDiagnosticFacade(t *testing.T) {
	t.Parallel()

	clean := ParseTeX(`x+1`, Inline, ParseOptions{})
	if clean.HasErrors() {
		t.Fatalf("clean diagnostics = %+v", clean.Diagnostics())
	}
	bad := ParseTeX(`\definitelyunknown`, Inline, ParseOptions{})
	if !bad.HasErrors() {
		t.Fatalf("bad diagnostics = %+v", bad.Diagnostics())
	}
	for _, diagnostic := range bad.Diagnostics() {
		if diagnostic.Severity != SeverityError {
			continue
		}
		if diagnostic.Code != CodeUnknownCommand {
			t.Fatalf("diagnostic code = %q, want %q", diagnostic.Code, CodeUnknownCommand)
		}
		return
	}
	t.Fatal("missing error diagnostic")
}

func TestLayoutFormulaContextAcceptsCanonicalIR(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	expr := ParseTeX(`x_i^2`, Inline, ParseOptions{})
	formula := expr.Root()
	box, err := engine.LayoutFormulaContext(context.Background(), formula, Inline, Style{Size: 16})
	if err != nil {
		t.Fatalf("LayoutFormulaContext() error = %v", err)
	}
	if box == nil || box.Alt.Text == "" || box.Alt.Speech == "" {
		t.Fatalf("LayoutFormulaContext() box = %#v", box)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Box.Validate() error = %v", err)
	}
	if box.Alt != expr.Alt() {
		t.Fatalf("Box.Alt = %#v, want %#v", box.Alt, expr.Alt())
	}
	if _, err := engine.LayoutFormulaContext(context.Background(), nil, Inline, Style{Size: 16}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil formula error = %v, want ErrInvalidInput", err)
	}
}

func TestFallbackIRValidates(t *testing.T) {
	t.Parallel()

	expr := Parse(`\frac{1}{2}`, TeX, Inline, ParseOptions{})
	if err := ir.Validate(expr.Root(), source.Limits{}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestHostMacroURLProducesValidSourceIR(t *testing.T) {
	t.Parallel()

	const src = `\site`
	expr := ParseTeX(src, Inline, ParseOptions{
		Macros: map[string]Macro{
			"site": {Body: `\url{https://example.org/a%20b}`},
		},
	})
	if err := ir.ValidateSource(expr.Root(), src, source.Limits{}); err != nil {
		t.Fatalf("ValidateSource() error = %v", err)
	}
	var text *ir.Text
	ir.Walk(expr.Root().Root, func(node ir.Node) bool {
		if candidate, ok := node.(*ir.Text); ok {
			text = candidate
		}
		return true
	})
	if text == nil {
		t.Fatalf("host macro URL root = %T, want text", expr.Root())
	}
	if text.Text != "https://example.org/a%20b" {
		t.Fatalf("host macro URL text = %q", text.Text)
	}
	if text.Span != source.NewRange(0, len(src)) || text.Origin != text.Span {
		t.Fatalf("host macro URL span/origin = %v/%v, want invocation", text.Span, text.Origin)
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

func TestExprAltIsSemanticAndDeterministic(t *testing.T) {
	t.Parallel()

	tex := ParseTeX(`x_i^2`, Inline, ParseOptions{})
	typst := ParseTypst(`x_i^2`, Inline, ParseOptions{})
	texAlt := tex.Alt()
	typstAlt := typst.Alt()
	if texAlt.Text != "x_i²" || texAlt.Speech != "x sub i squared" {
		t.Fatalf("TeX Alt() = %#v", texAlt)
	}
	if typstAlt != texAlt {
		t.Fatalf("equivalent source produced different alt text:\nTeX:   %#v\nTypst: %#v", texAlt, typstAlt)
	}
	if again := tex.Alt(); again != texAlt {
		t.Fatalf("Alt() is nondeterministic: %#v != %#v", texAlt, again)
	}
	if err := texAlt.Validate(); err != nil {
		t.Fatalf("Alt().Validate() error = %v", err)
	}
}

func TestExprAltIgnoresSourceSpans(t *testing.T) {
	t.Parallel()

	plain := ParseTeX(`x`, Inline, ParseOptions{}).Alt()
	grouped := ParseTeX(`{x}`, Inline, ParseOptions{}).Alt()
	if plain != grouped {
		t.Fatalf("source grouping changed alternative text:\nplain:   %#v\ngrouped: %#v", plain, grouped)
	}
}

func TestExprAltKeepsTextRunsReadable(t *testing.T) {
	t.Parallel()

	alt := ParseTeX(`\text{hello world}`, Inline, ParseOptions{}).Alt()
	if alt.Text != "hello world" {
		t.Fatalf("Alt().Text = %q, want %q", alt.Text, "hello world")
	}
	if alt.Speech != "hello world" {
		t.Fatalf("Alt().Speech = %q, want %q", alt.Speech, "hello world")
	}
}

func TestExprAltShowsRecoverableErrors(t *testing.T) {
	t.Parallel()

	alt := ParseTeX(`\definitelyunknown`, Inline, ParseOptions{}).Alt()
	if !strings.Contains(alt.Text, "error") || !strings.Contains(alt.Text, "definitelyunknown") {
		t.Fatalf("Alt().Text = %q, want visible error", alt.Text)
	}
	if !strings.Contains(alt.Speech, "error") || !strings.Contains(alt.Speech, "definitelyunknown") {
		t.Fatalf("Alt().Speech = %q, want visible error", alt.Speech)
	}
}

func TestExprAltContextCancellationAndNil(t *testing.T) {
	t.Parallel()

	expr := ParseTeX(`x`, Inline, ParseOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if alt, err := expr.AltContext(ctx); !errors.Is(err, context.Canceled) || alt != (Alt{}) {
		t.Fatalf("AltContext() = %#v, %v, want zero, context.Canceled", alt, err)
	}
	//nolint:staticcheck // This assertion intentionally verifies nil-context handling.
	if _, err := expr.AltContext(nil); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("nil AltContext() error = %v", err)
	}
	var nilExpr *Expr
	if got := nilExpr.Alt(); got != (Alt{}) {
		t.Fatalf("nil Expr.Alt() = %#v, want zero", got)
	}
	if _, err := nilExpr.AltContext(context.Background()); !errors.Is(err, ErrInvalidExpr) {
		t.Fatalf("nil Expr.AltContext() error = %v, want ErrInvalidExpr", err)
	}
}

func TestExprAltSemanticRoleRegressions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     string
		dialect    Dialect
		wantText   string
		wantSpeech string
	}{
		{
			name:       "integral bounds",
			source:     `\int_0^1 x\,dx`,
			dialect:    TeX,
			wantSpeech: "integral from 0 to 1",
		},
		{
			name:       "TeX nolimits keeps sum bounds",
			source:     `\sum\nolimits_{i=1}^n`,
			dialect:    TeX,
			wantText:   "∑_{i=1}ⁿ",
			wantSpeech: "sum from i equals 1 to n",
		},
		{
			name:       "Typst scripts keeps sum bounds",
			source:     `scripts(sum)_(i=1)^n`,
			dialect:    Typst,
			wantText:   "∑_{i=1}ⁿ",
			wantSpeech: "sum from i equals 1 to n",
		},
		{
			name:       "ordinary Typst attach is a right script",
			source:     `attach(x, t: a)`,
			dialect:    Typst,
			wantText:   "x^a",
			wantSpeech: "x to the power a",
		},
		{
			name:       "Typst corner attachment",
			source:     `attach(x, tr: a)`,
			dialect:    Typst,
			wantText:   "x^a",
			wantSpeech: "x to the power a",
		},
		{
			name:       "Typst full attachment example",
			source:     `attach(Pi, t: alpha, b: beta, tl: 1, tr: 2+3, bl: 4+5, br: 6)`,
			dialect:    Typst,
			wantText:   "attach(Π, t: α, b: β, tl: 1, bl: 4+5, tr: 2+3, br: 6)",
			wantSpeech: "Π with top attachment α, bottom attachment β, top-left attachment 1, bottom-left attachment 4 plus 5, top-right attachment 2 plus 3, bottom-right attachment 6",
		},
		{
			name:       "overset",
			source:     `\overset{a}{b}`,
			dialect:    TeX,
			wantText:   "overset(a, b)",
			wantSpeech: "b with a above",
		},
		{
			name:       "stackrel",
			source:     `\stackrel{a}{=}`,
			dialect:    TeX,
			wantText:   "overset(a, =)",
			wantSpeech: "equals with a above",
		},
		{
			name:       "brace label",
			source:     `\overbrace{x+y}^{n}`,
			dialect:    TeX,
			wantText:   "labeled(overbrace(x+y), n)",
			wantSpeech: "x plus y with overbrace labeled n",
		},
		{
			name:       "TeX binomial",
			source:     `\binom nk`,
			dialect:    TeX,
			wantText:   "(binom(n, k))",
			wantSpeech: "left parenthesis binomial coefficient n choose k right parenthesis",
		},
		{
			name:       "Typst binomial",
			source:     `binom(n, k)`,
			dialect:    Typst,
			wantText:   "(binom(n, k))",
			wantSpeech: "left parenthesis binomial coefficient n choose k right parenthesis",
		},
		{
			name:       "generic stack",
			source:     `a\atop b`,
			dialect:    TeX,
			wantText:   "stack(a, b)",
			wantSpeech: "a over b",
		},
		{
			name:       "TeX cases",
			source:     `\begin{cases}x&x>0\\-x&x\le0\end{cases}`,
			dialect:    TeX,
			wantText:   "{cases(x, x>0; −x, x≤0)",
			wantSpeech: "left brace piecewise definition with 2 rows: row 1: x, x greater than 0; row 2: minus x, x less than or equal to 0",
		},
		{
			name:       "Typst cases",
			source:     `cases(x & "if" x > 0, -x & "otherwise")`,
			dialect:    Typst,
			wantText:   "{cases(x, if x>0; −x, otherwise)",
			wantSpeech: "left brace piecewise definition with 2 rows: row 1: x, if x greater than 0; row 2: minus x, otherwise",
		},
		{
			name:       "array",
			source:     `\begin{array}{cc}a&b\\c&d\end{array}`,
			dialect:    TeX,
			wantText:   "array(a, b; c, d)",
			wantSpeech: "array with 2 rows: row 1: a, b; row 2: c, d",
		},
		{
			name:       "Typst vector",
			source:     `vec(1, 2, 3)`,
			dialect:    Typst,
			wantText:   "(vector(1; 2; 3))",
			wantSpeech: "left parenthesis column vector with 3 entries: row 1: 1; row 2: 2; row 3: 3 right parenthesis",
		},
		{
			name:       "named operator boundary",
			source:     `\sin x`,
			dialect:    TeX,
			wantText:   "sin x",
			wantSpeech: "sine x",
		},
		{
			name:       "text boundary",
			source:     `\text{hello} x`,
			dialect:    TeX,
			wantText:   "hello x",
			wantSpeech: "hello x",
		},
		{
			name:       "negated equals",
			source:     `\not=`,
			dialect:    TeX,
			wantText:   "≠",
			wantSpeech: "not equal to",
		},
		{
			name:       "negated membership",
			source:     `\not\in`,
			dialect:    TeX,
			wantText:   "∉",
			wantSpeech: "not an element of",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expr := Parse(tc.source, tc.dialect, Display, ParseOptions{})
			for _, diagnostic := range expr.Diagnostics() {
				if diagnostic.Severity == SeverityError {
					t.Fatalf("parse diagnostic: %v", diagnostic)
				}
			}
			alt := expr.Alt()
			if tc.wantText != "" && alt.Text != tc.wantText {
				t.Fatalf("Text = %q, want %q", alt.Text, tc.wantText)
			}
			if tc.wantSpeech != "" && alt.Speech != tc.wantSpeech && !strings.HasPrefix(alt.Speech, tc.wantSpeech) {
				t.Fatalf("Speech = %q, want %q", alt.Speech, tc.wantSpeech)
			}
			if strings.Contains(strings.ToLower(alt.Text+" "+alt.Speech), "matrix") {
				t.Fatalf("semantic output incorrectly says matrix: %#v", alt)
			}
			if strings.Contains(alt.Speech, "sub 0 to the power 1") {
				t.Fatalf("integral rendered as scripts: %q", alt.Speech)
			}
			if err := alt.Validate(); err != nil {
				t.Fatalf("Alt.Validate() error = %v", err)
			}
		})
	}
}

func TestExprAltContextLimitContract(t *testing.T) {
	t.Parallel()

	expr := ParseTeX(`(x)`, Inline, ParseOptions{Limits: Limits{
		MaxSourceBytes: 3,
		MaxTextBytes:   5,
	}})
	alt, err := expr.AltContext(context.Background())
	if !errors.Is(err, ErrAltLimitExceeded) {
		t.Fatalf("AltContext() error = %v, want ErrAltLimitExceeded", err)
	}
	if alt.Text == "" || alt.Speech == "" {
		t.Fatalf("AltContext() fallback = %#v, want visible text", alt)
	}
	if len(alt.Text) > 5 || len(alt.Speech) > 5 {
		t.Fatalf("AltContext() fallback exceeds limit: text=%d speech=%d", len(alt.Text), len(alt.Speech))
	}
	if err := alt.Validate(); err != nil {
		t.Fatalf("fallback Validate() error = %v", err)
	}
	again, againErr := expr.AltContext(context.Background())
	if again != alt || !errors.Is(againErr, ErrAltLimitExceeded) {
		t.Fatalf("AltContext() is nondeterministic: %#v/%v != %#v/%v", alt, err, again, againErr)
	}
}

func TestTexDepthLimitRecoveryStaysPubliclyValid(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxNesting = 1
	limits.MaxNodes = 4
	expr := ParseTeX("abcdefghij", Inline, ParseOptions{Limits: limits})

	if _, err := expr.RootSnapshot(); err != nil {
		t.Fatalf("RootSnapshot() error = %v", err)
	}
	limitDiagnostics := 0
	for _, diagnostic := range expr.Diagnostics() {
		if diagnostic.Code == source.CodeInternal {
			t.Fatalf("depth-limit recovery introduced internal diagnostic: %+v", diagnostic)
		}
		if diagnostic.Code == source.CodeLimitExceeded {
			limitDiagnostics++
		}
	}
	if limitDiagnostics != 1 {
		t.Fatalf("limit diagnostics = %d (%+v), want exactly one", limitDiagnostics, expr.Diagnostics())
	}
}

func TestTinyTextLimitStillReturnsValidExpression(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"é", "😀", "abc"} {
		expr := Parse(src, Typst, Inline, ParseOptions{Limits: source.Limits{
			MaxSourceBytes: len(src),
			MaxTextBytes:   1,
		}})
		root, err := expr.RootSnapshot()
		if err != nil {
			t.Fatalf("RootSnapshot(%q) error = %v", src, err)
		}
		if err := ir.ValidateSource(root, src, source.Limits{MaxTextBytes: 1}); err != nil {
			t.Fatalf("fallback IR for %q is invalid: %v", src, err)
		}
		for _, diagnostic := range expr.Diagnostics() {
			if diagnostic.Code == source.CodeInternal {
				t.Fatalf("tiny text limit for %q leaked an internal diagnostic: %+v", src, diagnostic)
			}
		}
	}
	if got := truncateUTF8("é", 1); got != "?" {
		t.Fatalf("truncateUTF8(é,1) = %q, want ?", got)
	}
	if got := truncateUTF8("😀", 2); got != "?" {
		t.Fatalf("truncateUTF8(😀,2) = %q, want ?", got)
	}
}

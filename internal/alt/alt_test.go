package alt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var testSpan = source.NewRange(0, 1)

func TestGenerateScripts(t *testing.T) {
	t.Parallel()

	scripts := ir.NewScripts(testSpan, glyph("x"))
	scripts.Bottom = glyph("i")
	scripts.Top = glyph("2")

	got := Generate(scripts, ir.Inline, source.Limits{})
	if got.Text != "x_i²" {
		t.Fatalf("Text = %q, want %q", got.Text, "x_i²")
	}
	if got.Speech != "x sub i squared" {
		t.Fatalf("Speech = %q, want %q", got.Speech, "x sub i squared")
	}
	if got.MathML != "" {
		t.Fatalf("MathML = %q, want empty", got.MathML)
	}

	left := ir.NewScripts(testSpan, glyph("x"))
	left.TopLeft = glyph("a")
	left.BottomLeft = glyph("b")
	got = Generate(left, ir.Inline, source.Limits{})
	if got.Text != "{}^a_bx" {
		t.Fatalf("prescript Text = %q", got.Text)
	}
	if got.Speech != "x with pre-subscript b with pre-superscript a" {
		t.Fatalf("prescript Speech = %q", got.Speech)
	}
}

func TestGenerateLimitsDependOnSemanticMode(t *testing.T) {
	t.Parallel()

	sum := ir.NewOperator(testSpan, "∑")
	sum.Class = ir.ClassLarge
	sum.Limits = ir.LimitsAuto
	scripts := ir.NewScripts(testSpan, sum)
	scripts.Bottom = glyph("i")
	scripts.Top = glyph("n")
	scripts.Limits = ir.LimitsAuto

	inline := Generate(scripts, ir.Inline, source.Limits{})
	if inline.Speech != "sum sub i to the power n" {
		t.Fatalf("inline Speech = %q", inline.Speech)
	}
	display := Generate(scripts, ir.Display, source.Limits{})
	if display.Speech != "sum from i to n" {
		t.Fatalf("display Speech = %q", display.Speech)
	}
	if inline.Text != display.Text {
		t.Fatalf("linear notation changed by placement: %q != %q", inline.Text, display.Text)
	}
}

func TestGenerateStructuralConstructs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		node       ir.Node
		wantText   string
		wantSpeech string
	}{
		{
			name:       "fraction",
			node:       ir.NewFraction(testSpan, glyph("1"), glyph("2")),
			wantText:   "1/2",
			wantSpeech: "1 over 2",
		},
		{
			name:       "fraction groups compound operands",
			node:       ir.NewFraction(testSpan, row(glyph("a"), glyph("+"), glyph("b")), glyph("c")),
			wantText:   "(a+b)/c",
			wantSpeech: "a plus b over c",
		},
		{
			name:       "square root",
			node:       ir.NewRadical(testSpan, glyph("x"), nil),
			wantText:   "√x",
			wantSpeech: "square root of x",
		},
		{
			name:       "cube root",
			node:       ir.NewRadical(testSpan, glyph("x"), glyph("3")),
			wantText:   "∛x",
			wantSpeech: "cube root of x",
		},
		{
			name:       "generic root",
			node:       ir.NewRadical(testSpan, glyph("x"), glyph("n")),
			wantText:   "root(n, x)",
			wantSpeech: "root of order n of x",
		},
		{
			name: "fence",
			node: func() ir.Node {
				fence := ir.NewFenced(testSpan, glyph("x"))
				fence.Open = glyph("(")
				fence.Close = glyph(")")
				fence.Balanced = true
				return fence
			}(),
			wantText:   "(x)",
			wantSpeech: "left parenthesis x right parenthesis",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Generate(tc.node, ir.Inline, source.Limits{})
			if got.Text != tc.wantText || got.Speech != tc.wantSpeech {
				t.Fatalf("Generate() = %#v, want Text=%q Speech=%q", got, tc.wantText, tc.wantSpeech)
			}
		})
	}
}

func TestGenerateDecorationsAndWrappers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		node       ir.Node
		wantText   string
		wantSpeech string
	}{
		{
			name:       "accent",
			node:       ir.NewAccent(testSpan, glyph("x"), glyph("^"), ir.PositionAbove),
			wantText:   "hat(x)",
			wantSpeech: "x with hat",
		},
		{
			name:       "line",
			node:       ir.NewLine(testSpan, glyph("x"), ir.PositionAbove),
			wantText:   "overline(x)",
			wantSpeech: "x with overline",
		},
		{
			name:       "cancel",
			node:       ir.NewCancel(testSpan, glyph("x")),
			wantText:   "cancel(x)",
			wantSpeech: "x canceled",
		},
		{
			name: "reserved box",
			node: func() ir.Node {
				box := ir.NewBox(testSpan, glyph("x"))
				box.HasBorder = true
				return box
			}(),
			wantText:   "box(x)",
			wantSpeech: "boxed x",
		},
		{
			name: "empty filled box",
			node: func() ir.Node {
				box := ir.NewBox(testSpan, ir.NewRow(testSpan))
				box.HasFill = true
				box.Fill = ir.Paint{Role: ir.PaintInk}
				return box
			}(),
			wantText:   "filled-rectangle",
			wantSpeech: "filled rectangle",
		},
		{
			name:       "phantom",
			node:       ir.NewPhantom(testSpan, glyph("x")),
			wantText:   "phantom(x)",
			wantSpeech: "phantom of x",
		},
		{
			name: "style and color are semantic pass-throughs",
			node: func() ir.Node {
				colored := ir.NewColor(testSpan, glyph("x"), ir.Paint{HasExplicit: true, Explicit: ir.RGBA{A: 255}})
				styled := ir.NewStyle(testSpan, colored)
				styled.SetSize = true
				styled.NewSize = ir.SizeDisplay
				return styled
			}(),
			wantText:   "x",
			wantSpeech: "x",
		},
		{
			name: "variant",
			node: func() ir.Node {
				bold := glyph("x")
				bold.Variant = ir.VariantBold
				return bold
			}(),
			wantText:   "bold(x)",
			wantSpeech: "bold x",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Generate(tc.node, ir.Inline, source.Limits{})
			if got.Text != tc.wantText || got.Speech != tc.wantSpeech {
				t.Fatalf("Generate() = %#v, want Text=%q Speech=%q", got, tc.wantText, tc.wantSpeech)
			}
		})
	}
}

func TestGenerateTablesMultilineBreaksAndMarkers(t *testing.T) {
	t.Parallel()

	table := ir.NewTable(testSpan,
		ir.TableRow{Cells: []ir.Node{glyph("x"), glyph("y")}},
		ir.TableRow{Cells: []ir.Node{glyph("1"), glyph("2")}},
	)
	got := Generate(table, ir.Display, source.Limits{})
	if got.Text != "matrix(x, y; 1, 2)" {
		t.Fatalf("table Text = %q", got.Text)
	}
	if got.Speech != "matrix with 2 rows: row 1: x, y; row 2: 1, 2" {
		t.Fatalf("table Speech = %q", got.Speech)
	}

	multiline := ir.NewMultiline(testSpan,
		[]ir.Node{glyph("x"), glyph("="), glyph("1")},
		[]ir.Node{glyph("y"), glyph("="), glyph("2")},
	)
	got = Generate(multiline, ir.Display, source.Limits{})
	if got.Text != "multiline(x=1; y=2)" {
		t.Fatalf("multiline Text = %q", got.Text)
	}
	if got.Speech != "multiline expression: row 1: x equals 1; row 2: y equals 2" {
		t.Fatalf("multiline Speech = %q", got.Speech)
	}

	row := ir.NewRow(testSpan,
		glyph("x"),
		ir.NewBreak(testSpan, ir.BreakRelation, 0),
		glyph("y"),
		ir.NewAlignMarker(testSpan),
		glyph("z"),
	)
	got = Generate(row, ir.Display, source.Limits{})
	if got.Text != `x \\ y & z` {
		t.Fatalf("row Text = %q", got.Text)
	}
	if got.Speech != "x line break y alignment point z" {
		t.Fatalf("row Speech = %q", got.Speech)
	}
}

func TestGenerateErrorsPlaceholdersAndSpacesRemainVisible(t *testing.T) {
	t.Parallel()

	row := ir.NewRow(testSpan,
		ir.NewError(testSpan, `\bad`, source.CodeUnknownCommand),
		ir.NewSpace(testSpan, ir.SpaceExplicit, ir.RawLength(ir.LengthMu, 3_000_000)),
		ir.NewPlaceholder(testSpan, "missing argument"),
	)
	got := Generate(row, ir.Inline, source.Limits{})
	if got.Text != `[error: \bad] [missing: missing argument]` {
		t.Fatalf("Text = %q", got.Text)
	}
	if got.Speech != `error: \bad thin space missing content: missing argument` {
		t.Fatalf("Speech = %q", got.Speech)
	}
}

func TestGenerateRejectsUnknownNodesVisibly(t *testing.T) {
	t.Parallel()

	out, err := GenerateContext(context.Background(), unknownNode{}, ir.Inline, source.Limits{})
	if !errors.Is(err, ErrUnsupportedNode) {
		t.Fatalf("GenerateContext() error = %v, want ErrUnsupportedNode", err)
	}
	if out.Text != "[unsupported node: kind-255]" {
		t.Fatalf("Text = %q", out.Text)
	}
	if out.Speech != "unsupported node: kind-255" {
		t.Fatalf("Speech = %q", out.Speech)
	}
	if out.MathML != "" {
		t.Fatalf("MathML = %q, want empty", out.MathML)
	}

	out, err = GenerateContext(context.Background(), row(glyph("x"), unknownNode{}, glyph("y")), ir.Inline, source.Limits{})
	if !errors.Is(err, ErrUnsupportedNode) {
		t.Fatalf("nested GenerateContext() error = %v, want ErrUnsupportedNode", err)
	}
	if out.Text != "x[unsupported node: kind-255]y" {
		t.Fatalf("nested Text = %q", out.Text)
	}
}

func TestGenerateRecoversMalformedNodePanics(t *testing.T) {
	t.Parallel()

	out, err := GenerateContext(context.Background(), panicNode{}, ir.Inline, source.Limits{})
	if !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("GenerateContext() error = %v, want ErrInvalidRoot", err)
	}
	if out.Text == "" || out.Speech == "" {
		t.Fatalf("panic fallback = %#v", out)
	}
}

func TestGenerateRejectsCyclesAndLimits(t *testing.T) {
	t.Parallel()

	cycle := ir.NewRow(testSpan)
	cycle.Items = []ir.Node{cycle}
	if _, err := GenerateContext(context.Background(), cycle, ir.Inline, source.Limits{}); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("cycle error = %v, want ErrInvalidRoot", err)
	}
	if _, err := GenerateContext(context.Background(), row(glyph("x"), glyph("y")), ir.Inline,
		source.Limits{MaxNodes: 1}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("node limit error = %v, want ErrLimitExceeded", err)
	}
	if _, err := GenerateContext(context.Background(), glyph("abcd"), ir.Inline,
		source.Limits{MaxTextBytes: 3}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("text limit error = %v, want ErrLimitExceeded", err)
	}
}

func TestGenerateCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GenerateContext(ctx, glyph("x"), ir.Inline, source.Limits{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("GenerateContext() error = %v, want context.Canceled", err)
	}
	//nolint:staticcheck // This assertion intentionally verifies nil-context handling.
	if _, err := GenerateContext(nil, glyph("x"), ir.Inline, source.Limits{}); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("nil context error = %v, want ErrInvalidContext", err)
	}
}

func TestGenerateIsSourceIndependentAndDeterministic(t *testing.T) {
	t.Parallel()

	first := ir.NewFraction(source.NewRange(0, 9), glyphAt(source.NewRange(1, 2), "a"), glyphAt(source.NewRange(3, 4), "b"))
	second := ir.NewFraction(source.Detached(), glyphAt(source.Detached(), "a"), glyphAt(source.Detached(), "b"))
	firstAlt := Generate(first, ir.Inline, source.Limits{})
	secondAlt := Generate(second, ir.Inline, source.Limits{})
	if firstAlt != secondAlt {
		t.Fatalf("source ranges changed semantics: %#v != %#v", firstAlt, secondAlt)
	}
	if again := Generate(first, ir.Inline, source.Limits{}); again != firstAlt {
		t.Fatalf("Generate() is nondeterministic: %#v != %#v", firstAlt, again)
	}
}

func TestGenerateRejectsInvalidUTF8WithoutReturningIt(t *testing.T) {
	t.Parallel()

	out, err := GenerateContext(context.Background(), ir.NewGlyph(testSpan, string([]byte{0xff})), ir.Inline, source.Limits{})
	if err == nil {
		t.Fatal("GenerateContext() error = nil")
	}
	if out.Text == "" || out.Speech == "" {
		t.Fatalf("visible fallback = %#v", out)
	}
	if strings.ContainsRune(out.Text, '\ufffd') {
		t.Fatalf("fallback invented replacement text: %q", out.Text)
	}
}

type unknownNode struct {
	ir.Node
}

func (unknownNode) Kind() ir.NodeKind {
	return ir.NodeKind(255)
}

type panicNode struct {
	ir.Node
}

func (panicNode) Kind() ir.NodeKind {
	panic("invalid kind")
}

func glyph(text string) *ir.Glyph {
	return glyphAt(testSpan, text)
}

func glyphAt(span source.Range, text string) *ir.Glyph {
	return ir.NewGlyph(span, text)
}

func row(nodes ...ir.Node) *ir.Row {
	return ir.NewRow(testSpan, nodes...)
}

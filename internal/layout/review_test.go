package layout

import (
	"context"
	"errors"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestScriptAndLimitPlacementPolicy(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)

	tests := []struct {
		name       string
		base       *ir.Glyph
		script     ir.Node
		position   bool // true puts the script above
		limits     ir.LimitsPlacement
		mode       ir.Mode
		wantLimits bool
	}{
		{"x squared", glyph("x", ir.ClassAlphabetic), glyph("2", ir.ClassNormal), true, ir.LimitsAuto, ir.Inline, false},
		{"x subscript", glyph("x", ir.ClassAlphabetic), glyph("i", ir.ClassNormal), false, ir.LimitsAuto, ir.Inline, false},
		{"display sum", glyph("sum", ir.ClassLarge), glyph("wide", ir.ClassNormal), true, ir.LimitsAuto, ir.Display, true},
		{"sum nolimits", glyph("sum", ir.ClassLarge), glyph("wide", ir.ClassNormal), true, ir.LimitsNever, ir.Display, false},
		{"display lim", glyph("lim", ir.ClassLarge), glyph("wide", ir.ClassNormal), true, ir.LimitsAuto, ir.Display, true},
		{"forced limits", glyph("x", ir.ClassAlphabetic), glyph("wide", ir.ClassNormal), true, ir.LimitsAlways, ir.Inline, true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			scripts := ir.NewScripts(source.NewRange(0, 3), test.base)
			if test.position {
				scripts.Top = test.script
			} else {
				scripts.Bottom = test.script
			}
			scripts.Limits = test.limits
			box, err := renderer.Layout(scripts, test.mode, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if len(box.Items) != 2 {
				t.Fatalf("items = %d, want base and script", len(box.Items))
			}
			scriptX := box.Items[1].At.X
			baseWidth := box.Items[0].Bound.Width
			if test.wantLimits {
				if scriptX >= baseWidth {
					t.Fatalf("limit x = %v, base width = %v; want centered limit", scriptX, baseWidth)
				}
			} else if scriptX < baseWidth {
				t.Fatalf("side script x = %v, base width = %v; want side placement", scriptX, baseWidth)
			}
		})
	}
}

func TestForcedSideScriptCoexistsWithLimit(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	scripts := ir.NewScripts(source.NewRange(0, 3), glyph("sum", ir.ClassLarge))
	scripts.Top = glyph("wide", ir.ClassNormal)
	scripts.TopRight = glyph("2", ir.ClassNormal)
	scripts.Limits = ir.LimitsAlways
	box, err := renderer.Layout(scripts, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) != 3 {
		t.Fatalf("items = %d, want base, limit, and forced side script", len(box.Items))
	}
	baseWidth := box.Items[0].Bound.Width
	sawLimit, sawSide := false, false
	for _, item := range box.Items[1:] {
		if item.At.X < baseWidth {
			sawLimit = true
		}
		if item.At.X > baseWidth {
			sawSide = true
		}
	}
	if !sawLimit || !sawSide {
		t.Fatalf("items = %+v, want centered limit and forced side script", box.Items)
	}
}

func TestSignedExplicitSpacing(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name  string
		space *ir.Space
		want  float32
	}{
		{"TeX negative thin space", ir.NewSpace(source.NewRange(1, 2), ir.SpaceExplicit, irEm(-1.0/6)), float32(16 * 5.0 / 6.0)},
		{"negative mkern", ir.NewSpace(source.NewRange(1, 2), ir.SpaceExplicit, mustMuLength(-3)), float32(16 * 5.0 / 6.0)},
		{"zero explicit space", ir.NewSpace(source.NewRange(1, 2), ir.SpaceExplicit, irEm(0)), 16},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			root := ir.NewRow(source.NewRange(0, 3), glyph("x", ir.ClassNormal), test.space, glyph("y", ir.ClassNormal))
			box, err := renderer.Layout(root, ir.Inline, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if !closeHost(box.Width, test.want) {
				t.Fatalf("Width = %v, want %v", box.Width, test.want)
			}
		})
	}
}

func TestBinaryClassLookahead(t *testing.T) {
	t.Parallel()

	alphabetic, binary, relation := ir.ClassAlphabetic, ir.ClassBinary, ir.ClassRelation
	opening, closing, punctuation := ir.ClassOpening, ir.ClassClosing, ir.ClassPunctuation
	tests := []struct {
		name    string
		classes []ir.MathClass
		index   int
		want    ir.MathClass
	}{
		{"trailing binary", []ir.MathClass{alphabetic, binary}, 1, ir.ClassUnary},
		{"binary before relation", []ir.MathClass{alphabetic, binary, relation, alphabetic}, 1, ir.ClassUnary},
		{"binary before closing", []ir.MathClass{opening, alphabetic, binary, closing}, 2, ir.ClassUnary},
		{"binary before punctuation", []ir.MathClass{alphabetic, binary, punctuation}, 1, ir.ClassUnary},
		{"valid binary", []ir.MathClass{alphabetic, binary, alphabetic}, 1, ir.ClassBinary},
	}
	for _, test := range tests {
		got := normalizeRowClasses(test.classes)
		if got[test.index] != test.want {
			t.Fatalf("%s: normalized class = %v, want %v", test.name, got[test.index], test.want)
		}
	}

	renderer, _, _ := newTestRenderer(t)
	for _, test := range []struct {
		name  string
		nodes []ir.Node
		width float32
	}{
		{"trailing x+", []ir.Node{glyph("x", ir.ClassAlphabetic), glyph("+", ir.ClassBinary)}, 16},
		{"(x+)", []ir.Node{glyph("(", ir.ClassOpening), glyph("x", ir.ClassAlphabetic), glyph("+", ir.ClassBinary), glyph(")", ir.ClassClosing)}, 32},
		{"x+ before right delimiter", []ir.Node{glyph("x", ir.ClassAlphabetic), glyph("+", ir.ClassBinary), glyph(")", ir.ClassClosing)}, 24},
	} {
		root := ir.NewRow(source.NewRange(0, 1), test.nodes...)
		box, err := renderer.Layout(root, ir.Inline, testStyle(16))
		if err != nil {
			t.Fatalf("%s: Layout() error = %v", test.name, err)
		}
		if !closeHost(box.Width, test.width) {
			t.Fatalf("%s: Width = %v, want %v", test.name, box.Width, test.width)
		}
		if err := box.Validate(); err != nil {
			t.Fatalf("%s: display validation error = %v", test.name, err)
		}
	}

	root := ir.NewRow(source.NewRange(0, 4),
		glyph("x", ir.ClassAlphabetic),
		glyph("+", ir.ClassBinary),
		glyph("=", ir.ClassRelation),
		glyph("y", ir.ClassAlphabetic),
	)
	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if got := box.Items[1].At.X; !closeHost(got, 8) {
		t.Fatalf("binary x = %v, want 8 after demotion; items = %+v", got, box.Items)
	}
}

func TestLayoutChecksContextBeforeValidation(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := renderer.LayoutContext(ctx, ir.NewRow(source.NewRange(99, 100)), ir.Inline, testStyle(16))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LayoutContext() error = %v, want cancellation", err)
	}
}

func TestMathKernInfoSelectionAndScaling(t *testing.T) {
	t.Parallel()

	base, err := newFragment(Metrics{Width: em(1, 1), Ascent: em(5, 10), Descent: em(2, 10)}, ir.ClassNormal, Text, geometry.Em(geometry.EmScale))
	if err != nil {
		t.Fatalf("newFragment(base) error = %v", err)
	}
	script, err := newFragment(Metrics{Width: em(1, 2), Ascent: em(3, 10), Descent: em(1, 10)}, ir.ClassNormal, Script, geometry.Em(geometry.EmScale))
	if err != nil {
		t.Fatalf("newFragment(script) error = %v", err)
	}
	base.Kerns.TopRight = &MathKern{
		Heights: []geometry.Em{em(2, 10)},
		Kerns:   []geometry.Em{em(-1, 10), em(-2, 10)},
	}
	script.Kerns.BottomLeft = &MathKern{
		Heights: []geometry.Em{em(2, 10)},
		Kerns:   []geometry.Em{em(-3, 10), em(-4, 10)},
	}
	if got, want := mathKern(base, script, em(4, 10), KernTopRight), em(-4, 10); got < want-2 || got > want+2 {
		t.Fatalf("mathKern() = %d, want %d (+/-2)", got, want)
	}
	if KernTopRight.inverse() != KernBottomLeft || KernTopLeft.inverse() != KernBottomRight {
		t.Fatal("kern corner inverse mapping is incorrect")
	}

	scaled, err := scaleKernInfo(base.Kerns, em(8, 10))
	if err != nil {
		t.Fatalf("scaleKernInfo() error = %v", err)
	}
	if scaled.TopRight == nil || scaled.TopRight.Kerns[1] != em(-16, 100) {
		t.Fatalf("scaled kern = %+v", scaled.TopRight)
	}
}

func TestDistinctHorizontalAndSkewedFractions(t *testing.T) {
	t.Parallel()

	horizontalRenderer, _, horizontalStretch := newTestRenderer(t)
	horizontal := ir.NewFraction(source.NewRange(0, 3), glyph("1", ir.ClassNormal), glyph("2", ir.ClassNormal))
	horizontal.Style = ir.FractionHorizontal
	horizontal.Bar = false
	horizontalBox, err := horizontalRenderer.Layout(horizontal, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(horizontal fraction) error = %v", err)
	}
	if horizontalStretch.calls != 0 {
		t.Fatalf("horizontal fraction stretch calls = %d, want 0", horizontalStretch.calls)
	}
	if len(horizontalBox.Items) != 3 || horizontalBox.Items[0].At.Y != horizontalBox.Items[2].At.Y {
		t.Fatalf("horizontal fraction items = %+v", horizontalBox.Items)
	}

	skewedRenderer, _, skewedStretch := newTestRenderer(t)
	skewed := ir.NewFraction(source.NewRange(0, 3), glyph("1", ir.ClassNormal), glyph("2", ir.ClassNormal))
	skewed.Style = ir.FractionSkewed
	skewed.Bar = false
	skewedBox, err := skewedRenderer.Layout(skewed, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout(skewed fraction) error = %v", err)
	}
	if skewedStretch.calls == 0 {
		t.Fatal("skewed fraction did not stretch its slash")
	}
	if len(skewedBox.Items) != 3 || skewedBox.Items[0].At.Y == skewedBox.Items[1].At.Y {
		t.Fatalf("skewed fraction items = %+v", skewedBox.Items)
	}
}

func TestConfiguredNodeLimitCountsEachNodeOnce(t *testing.T) {
	t.Parallel()

	root := ir.NewRow(source.NewRange(0, 2), glyph("x", ir.ClassNormal), glyph("y", ir.ClassNormal))
	renderer, err := New(Fonts{Glyphs: &fakeGlyphSource{glyphs: map[string]Glyph{}}, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxNodes: 3},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := renderer.Layout(root, ir.Inline, testStyle(16)); err != nil {
		t.Fatalf("Layout() with exact node budget error = %v", err)
	}
}

func TestConfiguredTextLimitAndGlyphRefCoordinates(t *testing.T) {
	t.Parallel()

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"x": {
			ID:      1,
			FontID:  "test-math",
			Advance: em(1, 2),
			Ascent:  em(7, 10),
			Descent: em(1, 5),
		},
	}, strict: true}
	renderer, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{
		Limits: source.Limits{MaxTextBytes: 1},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	box, err := renderer.Layout(glyph("x", ir.ClassNormal), ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) != 1 || box.Items[0].Kind != display.ItemGlyphRef {
		t.Fatalf("items = %+v", box.Items)
	}
	if box.Items[0].At.X != box.Items[0].Bound.X || !closeHost(box.Items[0].At.Y-box.Items[0].Bound.Y, 11.2) {
		t.Fatalf("glyph ref At = %+v, Bound = %+v", box.Items[0].At, box.Items[0].Bound)
	}
	if _, err := renderer.Layout(glyph("too-long", ir.ClassNormal), ir.Inline, testStyle(16)); !errors.Is(err, ErrLimit) {
		t.Fatalf("text limit error = %v", err)
	}
}

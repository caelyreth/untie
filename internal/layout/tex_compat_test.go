package layout

import (
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestExplicitFontSizeScalesTextAndMathExactlyOnce(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	styledGlyph := func(body ir.Node, percent float64) *ir.Style {
		node := ir.NewStyle(source.NewRange(0, 1), body)
		node.SetSize = true
		node.NewSize = ir.SizeText
		node.SetFontSize = true
		node.FontSize = mustPercentLength(percent)
		return node
	}

	math := styledGlyph(glyph("x", ir.ClassNormal), 150)
	mathBox, err := renderer.Layout(math, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("math Layout() error = %v", err)
	}
	if !closeHost(mathBox.Width, 12) || !closeHost(mathBox.Ascent, 16.8) || len(mathBox.Items) != 1 {
		t.Fatalf("math box = width:%v ascent:%v items:%d, want 12/16.8/1",
			mathBox.Width, mathBox.Ascent, len(mathBox.Items))
	}
	if !closeHost(mathBox.Items[0].Em, 24) {
		t.Fatalf("math item em = %v, want 24", mathBox.Items[0].Em)
	}

	text := ir.NewText(source.NewRange(0, 1), "x")
	textBox, err := renderer.Layout(styledGlyph(text, 50), ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("text Layout() error = %v", err)
	}
	if !closeHost(textBox.Width, 4) || len(textBox.Items) != 1 || !closeHost(textBox.Items[0].Em, 8) {
		t.Fatalf("text box = width:%v items:%d em:%v, want 4/1/8",
			textBox.Width, len(textBox.Items), textBox.Items[0].Em)
	}
}

func TestExplicitFontSizeComposesWithNestedScriptScale(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	style := ir.NewStyle(source.NewRange(0, 1), nil)
	style.SetSize = true
	style.NewSize = ir.SizeText
	style.SetFontSize = true
	style.FontSize = mustPercentLength(150)
	scripts := ir.NewScripts(source.NewRange(0, 1), glyph("x", ir.ClassNormal))
	scripts.Top = glyph("2", ir.ClassNormal)
	style.Body = scripts

	box, err := renderer.Layout(style, ir.Display, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) != 2 {
		t.Fatalf("items = %d, want base and superscript", len(box.Items))
	}
	if !closeHost(box.Items[0].Em, 24) || !closeHost(box.Items[1].Em, 19.2) {
		t.Fatalf("base/script em = %v/%v, want 24/19.2", box.Items[0].Em, box.Items[1].Em)
	}
}

func TestOverlayAlignmentConsumesNoHorizontalAdvance(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name      string
		alignment ir.OverlayAlignment
		wantX     float32
	}{
		{name: "start", alignment: ir.OverlayStart, wantX: 0},
		{name: "center", alignment: ir.OverlayCenter, wantX: -4},
		{name: "end", alignment: ir.OverlayEnd, wantX: -8},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := ir.NewRow(
				source.NewRange(0, 3),
				glyph("a", ir.ClassNormal),
				ir.NewOverlay(source.NewRange(1, 2), glyph("x", ir.ClassNormal), test.alignment),
				glyph("b", ir.ClassNormal),
			)
			box, err := renderer.Layout(root, ir.Display, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if !closeHost(box.Width, 16) || len(box.Items) != 3 {
				t.Fatalf("box = width:%v items:%d, want 16/3", box.Width, len(box.Items))
			}
			if got := box.Items[1].At.X - 8; !closeHost(got, test.wantX) {
				t.Fatalf("overlay x = %v, want %v", got, test.wantX)
			}
			if got := box.Items[2].At.X; !closeHost(got, 8) {
				t.Fatalf("following glyph x = %v, want 8", got)
			}
			if err := box.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestRaiseShiftsInkAndClampsBaselineMetrics(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	tests := []struct {
		name              string
		amount            ir.Length
		wantAscent        float32
		wantDescent       float32
		wantAtY           float32
		wantRaisedAscent  float32
		wantRaisedDescent float32
	}{
		{
			name:              "up",
			amount:            irEm(0.25),
			wantAscent:        15.2,
			wantDescent:       3.2,
			wantAtY:           -4,
			wantRaisedAscent:  15.2,
			wantRaisedDescent: 0,
		},
		{
			name:              "down",
			amount:            irEm(-0.5),
			wantAscent:        11.2,
			wantDescent:       11.2,
			wantAtY:           8,
			wantRaisedAscent:  3.2,
			wantRaisedDescent: 11.2,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := ir.NewRow(
				source.NewRange(0, 2),
				glyph("a", ir.ClassNormal),
				ir.NewRaise(source.NewRange(1, 2), glyph("x", ir.ClassNormal), test.amount),
			)
			box, err := renderer.Layout(root, ir.Display, testStyle(16))
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if !closeHost(box.Width, 16) {
				t.Fatalf("width = %v, want unchanged 16", box.Width)
			}
			if !closeHost(box.Ascent, test.wantAscent) || !closeHost(box.Descent, test.wantDescent) {
				t.Fatalf("metrics = %v/%v, want %v/%v",
					box.Ascent, box.Descent, test.wantAscent, test.wantDescent)
			}
			if !closeHost(box.Items[1].At.Y, test.wantAtY) {
				t.Fatalf("raised glyph y = %v, want %v", box.Items[1].At.Y, test.wantAtY)
			}
			if err := box.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			standalone, err := renderer.Layout(
				ir.NewRaise(source.NewRange(0, 1), glyph("x", ir.ClassNormal), test.amount),
				ir.Display,
				testStyle(16),
			)
			if err != nil {
				t.Fatalf("standalone Layout() error = %v", err)
			}
			if !closeHost(standalone.Ascent, test.wantRaisedAscent) ||
				!closeHost(standalone.Descent, test.wantRaisedDescent) {
				t.Fatalf("standalone raise metrics = %v/%v, want %v/%v",
					standalone.Ascent, standalone.Descent,
					test.wantRaisedAscent, test.wantRaisedDescent)
			}
		})
	}
}

func TestOverlayAndRaisePreserveDisplayValidation(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	root := ir.NewRow(
		source.NewRange(0, 5),
		glyph("x", ir.ClassNormal),
		ir.NewOverlay(source.NewRange(1, 3), glyph("y", ir.ClassNormal), ir.OverlayCenter),
		ir.NewRaise(source.NewRange(3, 5), glyph("z", ir.ClassNormal), irEm(0.1)),
	)
	box, err := renderer.Layout(root, ir.Display, display.Style{Size: 16})
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if err := box.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	canonical, err := box.MarshalCanonical()
	if err != nil || len(canonical) == 0 {
		t.Fatalf("MarshalCanonical() = %q, %v", canonical, err)
	}
}

func mustPercentLength(value float64) ir.Length {
	length, err := ir.PercentLength(value)
	if err != nil {
		panic(err)
	}
	return length
}

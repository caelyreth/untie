package crosscheck

import (
	"context"
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/engine"
	"github.com/caelyreth/untie/internal/fixture"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// toleranceDIP is Typlet's 0.0015pt rounding tolerance converted once to DIP.
const toleranceDIP = fixture.TypletFrameToleranceDIP

type layoutHarness struct {
	engine *engine.Engine
}

func newLayoutHarness(fontBytes []byte) (*layoutHarness, error) {
	fonts, err := font.Load(font.Fonts{
		Math: font.FontData{Bytes: fontBytes, ID: "NewCMMath-Book"},
	})
	if err != nil {
		return nil, err
	}
	renderer, err := engine.New(fonts, source.DefaultLimits())
	if err != nil {
		return nil, err
	}
	return &layoutHarness{engine: renderer}, nil
}

func (h *layoutHarness) layout(ctx context.Context, root ir.Node, mode ir.Mode, size float64) (*display.Box, error) {
	return h.layoutFormula(ctx, &ir.Formula{Root: root}, mode, size)
}

func (h *layoutHarness) layoutFormula(ctx context.Context, formula *ir.Formula, mode ir.Mode, size float64) (*display.Box, error) {
	return h.engine.LayoutFormula(ctx, formula, mode, display.Style{Size: float32(size)})
}

type glyphView struct {
	Font string
	ID   uint16
	X    float64
	Y    float64
	Size float64
}

type layoutView struct {
	Width     float64
	Ascent    float64
	Descent   float64
	Italic    float64
	Axis      float64
	Overflow  float64
	TopHeight float64
	TopAnchor bool
	Glyphs    []glyphView
	Items     []itemView
}

type itemView struct {
	Kind  display.ItemKind
	Paint paintView
	Bound rectView
	Path  []segmentView
	Rect  rectView
	Font  string
	Glyph uint16
	At    pointView
	Em    float64
}

type paintView struct {
	Role        display.Role
	Explicit    display.RGBA
	HasExplicit bool
}

type pointView struct {
	X float64
	Y float64
}

type rectView struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type segmentView struct {
	Op     geometry.SegOp
	Points [3]pointView
}

func outputView(box *display.Box) layoutView {
	view := layoutView{
		Width: float64(box.Width), Ascent: float64(box.Ascent), Descent: float64(box.Descent),
		Italic: float64(box.Italic), Axis: float64(box.AxisHeight), Overflow: float64(box.Overflow),
		TopHeight: float64(box.Ascent + box.Descent),
	}
	for _, item := range box.Items {
		view.Items = append(view.Items, itemToView(item))
		isGlyph := item.Kind == display.ItemGlyphRef ||
			(item.Kind == display.ItemGlyphPath && item.FontID != "" && item.Glyph != 0)
		if !isGlyph {
			continue
		}
		view.Glyphs = append(view.Glyphs, glyphView{
			Font: item.FontID, ID: item.Glyph,
			X: float64(item.At.X), Y: float64(item.At.Y), Size: float64(item.Em),
		})
	}
	return view
}

func itemToView(item display.Item) itemView {
	view := itemView{
		Kind:  item.Kind,
		Paint: paintToView(item.Paint),
	}
	view.Bound = rectToView(item.Bound)
	view.Rect = rectToView(item.Rect)
	view.Font = item.FontID
	view.Glyph = item.Glyph
	view.At = pointView{X: float64(item.At.X), Y: float64(item.At.Y)}
	view.Em = float64(item.Em)
	view.Path = make([]segmentView, len(item.Path))
	for i, segment := range item.Path {
		view.Path[i].Op = segment.Op
		for j, point := range segment.Points {
			view.Path[i].Points[j] = pointView{X: float64(point.X), Y: float64(point.Y)}
		}
	}
	return view
}

func paintToView(paint display.Paint) paintView {
	return paintView{
		Role:        paint.Role,
		Explicit:    paint.Explicit,
		HasExplicit: paint.HasExplicit,
	}
}

func rectToView(rect geometry.Rect) rectView {
	return rectView{
		X: float64(rect.X), Y: float64(rect.Y),
		Width: float64(rect.Width), Height: float64(rect.Height),
	}
}

func fixtureView(frame fixture.Frame) (layoutView, error) {
	converted, err := fixture.FrameToDIP(frame)
	if err != nil {
		return layoutView{}, err
	}
	if converted.Box == nil {
		return layoutView{}, fmt.Errorf("fixture frame has no box")
	}
	view := layoutView{Width: *converted.Box.Width}
	switch converted.Box.Anchor {
	case "baseline":
		view.Ascent = *converted.Box.Ascent
		view.Descent = *converted.Box.Descent
		view.TopHeight = view.Ascent + view.Descent
	case "top":
		view.TopAnchor = true
		view.TopHeight = *converted.Box.Height
	}
	view.Glyphs = make([]glyphView, len(converted.Glyphs))
	for i, glyph := range converted.Glyphs {
		if glyph.HasTransform {
			return layoutView{}, fmt.Errorf("fixture glyph %d has an unsupported transform", i)
		}
		view.Glyphs[i] = glyphView{
			Font: converted.Fonts[glyph.Font], ID: uint16(glyph.Glyph),
			X: glyph.X, Y: glyph.Y, Size: glyph.Size,
		}
	}
	return view, nil
}

// CompareOutputs compares two renderer outputs in baseline coordinates.
func CompareOutputs(got, want *display.Box) string {
	if got == nil || want == nil {
		return "output: both boxes are required"
	}
	return compareLayoutViews("output", outputView(got), outputView(want), false, true)
}

// CompareOutputToFixture compares renderer output to a Typlet frame. Both
// values are first expressed in the fixture frame's anchor coordinate system.
func CompareOutputToFixture(got *display.Box, want fixture.Frame) string {
	if got == nil {
		return "fixture: renderer output is nil"
	}
	actual := outputView(got)
	expected, err := fixtureView(want)
	if err != nil {
		return err.Error()
	}
	if expected.TopAnchor {
		actual.YToTop(actual.Ascent)
	}
	return compareLayoutViews("fixture", actual, expected, true, false)
}

func (v *layoutView) YToTop(baseline float64) {
	for i := range v.Glyphs {
		v.Glyphs[i].Y = baseline + v.Glyphs[i].Y
	}
	v.TopAnchor = true
}

func compareLayoutViews(prefix string, got, want layoutView, fixtureCoords, compareItems bool) string {
	for _, field := range []struct {
		name string
		got  float64
		want float64
	}{
		{"width", got.Width, want.Width},
	} {
		if !near(field.got, field.want) {
			return fmt.Sprintf("%s.%s: expected %.6f, got %.6f", prefix, field.name, field.want, field.got)
		}
	}
	if fixtureCoords && want.TopAnchor {
		if !near(got.TopHeight, want.TopHeight) {
			return fmt.Sprintf("%s.top-height: expected %.6f, got %.6f", prefix, want.TopHeight, got.TopHeight)
		}
	} else {
		type field struct {
			name string
			got  float64
			want float64
		}
		fields := []field{
			{"ascent", got.Ascent, want.Ascent},
			{"descent", got.Descent, want.Descent},
		}
		if !fixtureCoords {
			fields = append(fields,
				field{"italic", got.Italic, want.Italic},
				field{"axis-height", got.Axis, want.Axis},
				field{"overflow", got.Overflow, want.Overflow})
		}
		for _, field := range fields {
			if !near(field.got, field.want) {
				return fmt.Sprintf("%s.%s: expected %.6f, got %.6f", prefix, field.name, field.want, field.got)
			}
		}
	}
	if len(got.Glyphs) != len(want.Glyphs) {
		return fmt.Sprintf("%s.glyphs: expected %d, got %d", prefix, len(want.Glyphs), len(got.Glyphs))
	}
	for i := range want.Glyphs {
		g := got.Glyphs[i]
		w := want.Glyphs[i]
		if g.Font != w.Font || g.ID != w.ID {
			return fmt.Sprintf("%s.glyphs[%d]: expected %s/%d, got %s/%d",
				prefix, i, w.Font, w.ID, g.Font, g.ID)
		}
		for _, field := range []struct {
			name string
			got  float64
			want float64
		}{
			{"x", g.X, w.X},
			{"y", g.Y, w.Y},
			{"size", g.Size, w.Size},
		} {
			if !near(field.got, field.want) {
				return fmt.Sprintf("%s.glyphs[%d].%s: expected %.6f, got %.6f",
					prefix, i, field.name, field.want, field.got)
			}
		}
	}
	if compareItems {
		if len(got.Items) != len(want.Items) {
			return fmt.Sprintf("%s.items: expected %d, got %d", prefix, len(want.Items), len(got.Items))
		}
		for i := range want.Items {
			if difference := compareItemViews(fmt.Sprintf("%s.items[%d]", prefix, i), got.Items[i], want.Items[i]); difference != "" {
				return difference
			}
		}
	}
	return ""
}

func compareItemViews(prefix string, got, want itemView) string {
	if got.Kind != want.Kind {
		return fmt.Sprintf("%s.kind: expected %d, got %d", prefix, want.Kind, got.Kind)
	}
	if got.Paint != want.Paint {
		return fmt.Sprintf("%s.paint: expected %+v, got %+v", prefix, want.Paint, got.Paint)
	}
	if difference := compareRect(prefix+".bound", got.Bound, want.Bound); difference != "" {
		return difference
	}
	switch want.Kind {
	case display.ItemGlyphPath:
		if len(got.Path) != len(want.Path) {
			return fmt.Sprintf("%s.path: expected %d segments, got %d", prefix, len(want.Path), len(got.Path))
		}
		for i := range want.Path {
			if got.Path[i].Op != want.Path[i].Op {
				return fmt.Sprintf("%s.path[%d].op: expected %d, got %d", prefix, i, want.Path[i].Op, got.Path[i].Op)
			}
			for j := range want.Path[i].Points {
				if !near(got.Path[i].Points[j].X, want.Path[i].Points[j].X) ||
					!near(got.Path[i].Points[j].Y, want.Path[i].Points[j].Y) {
					return fmt.Sprintf("%s.path[%d].point[%d]: expected (%v,%v), got (%v,%v)",
						prefix, i, j,
						want.Path[i].Points[j].X, want.Path[i].Points[j].Y,
						got.Path[i].Points[j].X, got.Path[i].Points[j].Y)
				}
			}
		}
	case display.ItemRule:
		return compareRect(prefix+".rect", got.Rect, want.Rect)
	case display.ItemGlyphRef:
		if got.Font != want.Font || got.Glyph != want.Glyph {
			return fmt.Sprintf("%s.glyph: expected %s/%d, got %s/%d",
				prefix, want.Font, want.Glyph, got.Font, got.Glyph)
		}
		if !near(got.At.X, want.At.X) || !near(got.At.Y, want.At.Y) || !near(got.Em, want.Em) {
			return fmt.Sprintf("%s.placement: expected (%v,%v,%v), got (%v,%v,%v)",
				prefix, want.At.X, want.At.Y, want.Em, got.At.X, got.At.Y, got.Em)
		}
	default:
		return fmt.Sprintf("%s: unknown item kind %d", prefix, want.Kind)
	}
	return ""
}

func compareRect(prefix string, got, want rectView) string {
	for _, field := range []struct {
		name string
		got  float64
		want float64
	}{
		{"x", got.X, want.X},
		{"y", got.Y, want.Y},
		{"width", got.Width, want.Width},
		{"height", got.Height, want.Height},
	} {
		if !near(field.got, field.want) {
			return fmt.Sprintf("%s.%s: expected %.6f, got %.6f", prefix, field.name, field.want, field.got)
		}
	}
	return ""
}

func near(got, want float64) bool {
	return !math.IsNaN(got) && !math.IsInf(got, 0) && math.Abs(got-want) <= toleranceDIP
}

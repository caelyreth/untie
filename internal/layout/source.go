package layout

import (
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

// Axis is a glyph stretch axis.
type Axis uint8

const (
	// Horizontal stretches a glyph or assembly to the right.
	Horizontal Axis = iota
	// Vertical stretches a glyph or assembly up and down.
	Vertical
)

// Point is a fixed-point point in local ems.
type Point struct {
	X geometry.Em
	Y geometry.Em
}

// Segment is one outline command in local ems. The Op field uses the public
// path operation vocabulary so flattening does not invent a second one.
type Segment struct {
	Op     geometry.SegOp
	Points [3]Point
}

// Path is a glyph outline in local ems, y down, with its baseline at y=0.
type Path []Segment

// GlyphRequest identifies one glyph lookup or shaping operation.
type GlyphRequest struct {
	Text     string
	Style    Style
	Variant  ir.Variant
	Font     ir.FontRole
	Features []string
}

// Glyph is a shaped glyph in local ems. Advance is the final layout advance,
// including any font-specific italic correction. Outline may be empty when the
// source only exposes a host glyph reference.
type Glyph struct {
	ID              uint16
	FontID          string
	Text            string
	Advance         geometry.Em
	Ascent          geometry.Em
	Descent         geometry.Em
	Italic          geometry.Em
	TopAccent       geometry.Em
	HasTopAccent    bool
	BottomAccent    geometry.Em
	HasBottomAccent bool
	// InkBottom is the signed y coordinate of the outline's lower edge relative
	// to the baseline. It may be negative for accents whose ink sits above the
	// baseline. Fill Outline or set HasInkBottom to preserve exact placement.
	InkBottom    geometry.Em
	HasInkBottom bool
	Kerns        MathKernInfo
	TextLike     bool
	Outline      Path
}

// KernCorner identifies one of the four corners in an OpenType MATH
// MathKernInfo table.
type KernCorner uint8

const (
	KernTopLeft KernCorner = iota
	KernTopRight
	KernBottomRight
	KernBottomLeft
)

func (c KernCorner) inverse() KernCorner {
	switch c {
	case KernTopLeft:
		return KernBottomRight
	case KernTopRight:
		return KernBottomLeft
	case KernBottomRight:
		return KernTopLeft
	default:
		return KernTopRight
	}
}

// MathKern is one corner's MathKernInfo table. Heights are strictly
// increasing and Kerns has exactly one more element.
type MathKern struct {
	Heights []geometry.Em
	Kerns   []geometry.Em
}

// MathKernInfo groups the optional kern tables attached to a glyph.
type MathKernInfo struct {
	TopLeft     *MathKern
	TopRight    *MathKern
	BottomRight *MathKern
	BottomLeft  *MathKern
}

func (k MathKernInfo) at(corner KernCorner) *MathKern {
	switch corner {
	case KernTopLeft:
		return k.TopLeft
	case KernTopRight:
		return k.TopRight
	case KernBottomRight:
		return k.BottomRight
	default:
		return k.BottomLeft
	}
}

// GlyphSource is the only glyph-facing dependency of layout.
type GlyphSource interface {
	Glyph(req GlyphRequest) (Glyph, bool)
}

// StretchRequest asks for a MATH size variant or assembly in local ems.
type StretchRequest struct {
	GlyphRequest
	Axis      Axis
	Target    geometry.Em
	ShortFall geometry.Em
}

// GlyphStretcher is an optional extension port for OpenType MATH variants and
// assemblies. Without it, an ordinary glyph is used only if it meets the
// target; otherwise layout returns ErrInsufficientStretch.
type GlyphStretcher interface {
	Stretch(req StretchRequest) (Glyph, bool)
}

// TextRequest identifies a shaped text run.
type TextRequest struct {
	Text     string
	Style    Style
	Variant  ir.Variant
	Tracking geometry.Em
	Features []string
}

// PositionedGlyph is one glyph in a shaped text run, in local ems relative to
// the run baseline.
type PositionedGlyph struct {
	Glyph Glyph
	At    Point
}

// TextRun is a shaped one-line text run in local ems.
type TextRun struct {
	Width   geometry.Em
	Ascent  geometry.Em
	Descent geometry.Em
	Italic  geometry.Em
	Glyphs  []PositionedGlyph
}

// TextSource is an optional extension port for deterministic text shaping.
type TextSource interface {
	ShapeText(req TextRequest) (TextRun, bool)
}

// TableLine is a resolved table boundary passed to a TableLayouter.
type TableLine struct {
	Position  int
	Count     int
	Dashed    bool
	Thickness geometry.Em
}

// TableRow contains measured cells and an optional measured tag.
type TableRow struct {
	Cells []Fragment
	Tag   *Fragment
}

// TableRequest is the measured-cell extension point for table layout. Row
// metrics and cell fragments are already deterministic when this is called.
type TableRequest struct {
	Style      Style
	Rows       []TableRow
	Columns    []ir.ColumnAlign
	RowGap     geometry.Em
	ColumnGap  geometry.Em
	Horizontal []TableLine
	Vertical   []TableLine
}

// TableLayouter is an optional extension port for table and matrix layout.
type TableLayouter interface {
	LayoutTable(req TableRequest) (Fragment, bool)
}

// Fonts are the ports required by the layout engine.
type Fonts struct {
	Glyphs  GlyphSource
	Metrics StyleFont
	Stretch GlyphStretcher
	Text    TextSource
}

package layout

import (
	"context"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
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

// Path is a glyph outline in local ems, y down, relative to the font glyph
// origin. Glyph.OriginY places that origin relative to the fragment baseline.
type Path []Segment

// GlyphRequest identifies one glyph lookup or shaping operation.
type GlyphRequest struct {
	Context  context.Context
	Text     string
	Style    Style
	Variant  ir.Variant
	Font     ir.FontRole
	Features []string
}

// Glyph is a shaped glyph in local ems. Advance is the final layout advance.
// AdvanceIncludesItalic reports that Italic has already been incorporated into
// Advance. Outline may be empty when the source only exposes a host glyph
// reference.
type Glyph struct {
	ID      uint16
	FontID  string
	Text    string
	Advance geometry.Em
	Ascent  geometry.Em
	Descent geometry.Em
	Italic  geometry.Em
	// OriginY is the glyph origin's signed offset from the fragment baseline.
	// It is non-zero when a vertical MATH construction is centered on the
	// math axis while its outline stays at the font's original baseline.
	OriginY               geometry.Em
	AdvanceIncludesItalic bool
	TopAccent             geometry.Em
	HasTopAccent          bool
	BottomAccent          geometry.Em
	HasBottomAccent       bool
	// InkBottom is the signed y coordinate of the outline's lower edge relative
	// to the baseline. It may be negative for accents whose ink sits above the
	// baseline. Fill Outline or set HasInkBottom to preserve exact placement.
	InkBottom    geometry.Em
	HasInkBottom bool
	Kerns        MathKernInfo
	// StretchableHorizontal reports a MATH horizontal construction. Script
	// layout uses it to select the stretch-stack limit constants.
	StretchableHorizontal bool
	// ExactGlyphRef reports that ID, FontID, Advance, ascent, and descent
	// describe the complete drawn glyph, so hosts may cache and draw it by ID.
	ExactGlyphRef bool
	TextLike      bool
	Outline       Path
	// Parts carries the individual glyphs of a MATH assembly. Positions are
	// baseline-relative local ems. Outline is empty for a part-backed glyph.
	Parts []PositionedGlyph
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
	Glyph(req GlyphRequest) (Glyph, error)
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
	Stretch(req StretchRequest) (Glyph, error)
}

// TextRequest identifies a shaped text run.
type TextRequest struct {
	Context context.Context
	Text    string
	Style   Style
	Variant ir.Variant
	Font    ir.FontRole
	// Tracking is unscaled base ems, independent of Style.Size. Shaping
	// produces base-em glyph positions; layout applies the style scale once.
	Tracking geometry.Em
	Features []string
}

// OperatorRequest identifies one named operator run. Operator text is set as
// a single upright run so scripts and limits attach to the complete name.
type OperatorRequest struct {
	Context  context.Context
	Text     string
	Style    Style
	Variant  ir.Variant
	Features []string
}

// TextCluster preserves the source extent of one shaping cluster. Byte offsets
// are relative to the text passed in TextRequest or OperatorRequest, not to the
// complete document source.
type TextCluster struct {
	RuneStart int
	RuneEnd   int
	Source    source.Range
	Missing   bool
}

// PositionedGlyph is one glyph in a shaped text run, in local ems relative to
// the run baseline. Cluster indexes TextRun.Clusters and is meaningful only
// when that slice is non-empty.
type PositionedGlyph struct {
	Glyph Glyph
	At    Point
	// Cluster is the zero-based index into TextRun.Clusters.
	Cluster int
	// Missing marks one synthetic glyph substituting for an unavailable text
	// cluster. Layout paints it with the error role.
	Missing bool
}

// TextRun is a shaped one-line text run in local ems.
type TextRun struct {
	Width   geometry.Em
	Ascent  geometry.Em
	Descent geometry.Em
	Italic  geometry.Em
	// Clusters is in visual glyph order and preserves source extents for editor
	// hit testing. It may be empty for non-text providers and synthetic runs.
	Clusters []TextCluster
	Glyphs   []PositionedGlyph
}

// TextSource is an optional extension port for deterministic text shaping.
type TextSource interface {
	ShapeText(req TextRequest) (TextRun, error)
}

// OperatorSource is an optional extension port for named operator runs.
type OperatorSource interface {
	ShapeOperator(req OperatorRequest) (TextRun, error)
}

// TableLine is a resolved table boundary used by the built-in table grid.
type TableLine struct {
	Position  int
	Count     int
	Dashed    bool
	Thickness geometry.Em
}

// Fonts are the ports required by the layout engine.
type Fonts struct {
	Glyphs    GlyphSource
	Metrics   StyleFont
	Stretch   GlyphStretcher
	Text      TextSource
	Operators OperatorSource
}

// baselineAtTop converts a desired child top edge in a parent's top-origin
// frame to the child baseline offset expected by Fragment.pushGroup.
func baselineAtTop(top, childAscent, parentAscent geometry.Em) (geometry.Em, error) {
	value, err := add(top, childAscent)
	if err != nil {
		return 0, err
	}
	return sub(value, parentAscent)
}

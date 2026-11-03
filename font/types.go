package font

import (
	"math"
	"strconv"

	"github.com/caelyreth/untie/geometry"
)

// FaceRef identifies a face in a loaded FontSet. Fallback faces are numbered
// from fallbackFaceBase in their input order.
type FaceRef uint8

const (
	// MathFace is the primary math face supplied in FontSetConfig.Math.
	MathFace FaceRef = iota
	// TextFace is the optional text face supplied in FontSetConfig.Text.
	TextFace

	fallbackFaceBase FaceRef = 2
	maxFallbackFaces         = int(math.MaxUint8) - int(fallbackFaceBase) + 1
)

// FallbackFace returns the FaceRef for fallback index i.
func FallbackFace(i int) (FaceRef, bool) {
	if i < 0 || i >= maxFallbackFaces {
		return 0, false
	}
	return fallbackFaceBase + FaceRef(i), true
}

// GlyphID is an OpenType glyph index.
type GlyphID uint16

// Glyph identifies a glyph and the face that owns it.
type Glyph struct {
	Face FaceRef
	ID   GlyphID
}

// FaceInfo is immutable metadata for one loaded face.
type FaceInfo struct {
	ID             string
	Index          int
	PostScriptName string
	UnitsPerEm     uint16
	GlyphCount     int
	HasMath        bool
}

// Bounds is a design-unit control bounding box in the same y-down coordinate
// space as Outline. It includes Bezier control points, matching OpenType
// MATH/font tooling bounds. YMin is the top edge and YMax is the bottom edge.
type Bounds struct {
	XMin int32
	YMin int32
	XMax int32
	YMax int32
}

// Empty reports whether the bounds contain no ink.
func (b Bounds) Empty() bool {
	return b.XMin == 0 && b.YMin == 0 && b.XMax == 0 && b.YMax == 0
}

// Width returns the horizontal extent.
func (b Bounds) Width() int32 {
	return b.XMax - b.XMin
}

// Height returns the vertical extent.
func (b Bounds) Height() int32 {
	return b.YMax - b.YMin
}

// GlyphMetrics contains unhinted design-unit metrics for one glyph.
type GlyphMetrics struct {
	Advance    int32
	Bounds     Bounds
	HasOutline bool
}

// Segment is one command in a glyph outline. Coordinates are font design units
// in y-down orientation with the baseline at y=0. MoveTo and LineTo use
// Points[0], QuadTo uses Points[0:2], CubeTo uses all three points, and Close
// uses none.
type Segment struct {
	Op     geometry.SegOp
	Points [3]Point
}

// Point is a whole design-unit outline coordinate. Fractional CFF coordinates
// are truncated toward zero once when the font is loaded, matching the pinned
// Typst/ttf-parser oracle.
type Point struct {
	X int32
	Y int32
}

// Outline is an immutable snapshot of a glyph's unhinted outline. Coordinates
// are whole design units; sub-design-unit CFF precision is not represented by
// this API. The returned value owns its Segments slice, so mutation cannot
// affect the font cache.
type Outline struct {
	Glyph    Glyph
	Bounds   Bounds
	Segments []Segment
}

// VerticalMetrics are design-unit font metrics.
type VerticalMetrics struct {
	Ascender   int32
	Descender  int32
	LineGap    int32
	CapHeight  int32
	XHeight    int32
	FontBounds Bounds
}

// MathConstant identifies a value from the OpenType MATH constants table.
// Lengths are returned in design units. The three percentage constants are
// returned as their literal percentage value.
type MathConstant uint8

const (
	ScriptPercentScaleDown MathConstant = iota
	ScriptScriptPercentScaleDown
	DelimitedSubFormulaMinHeight
	DisplayOperatorMinHeight
	MathLeading
	AxisHeight
	AccentBaseHeight
	FlattenedAccentBaseHeight
	SubscriptShiftDown
	SubscriptTopMax
	SubscriptBaselineDropMin
	SuperscriptShiftUp
	SuperscriptShiftUpCramped
	SuperscriptBottomMin
	SuperscriptBaselineDropMax
	SubSuperscriptGapMin
	SuperscriptBottomMaxWithSubscript
	SpaceAfterScript
	UpperLimitGapMin
	UpperLimitBaselineRiseMin
	LowerLimitGapMin
	LowerLimitBaselineDropMin
	StackTopShiftUp
	StackTopDisplayStyleShiftUp
	StackBottomShiftDown
	StackBottomDisplayStyleShiftDown
	StackGapMin
	StackDisplayStyleGapMin
	StretchStackTopShiftUp
	StretchStackBottomShiftDown
	StretchStackGapAboveMin
	StretchStackGapBelowMin
	FractionNumeratorShiftUp
	FractionNumeratorDisplayStyleShiftUp
	FractionDenominatorShiftDown
	FractionDenominatorDisplayStyleShiftDown
	FractionNumeratorGapMin
	FractionNumDisplayStyleGapMin
	FractionRuleThickness
	FractionDenominatorGapMin
	FractionDenomDisplayStyleGapMin
	SkewedFractionHorizontalGap
	SkewedFractionVerticalGap
	OverbarVerticalGap
	OverbarRuleThickness
	OverbarExtraAscender
	UnderbarVerticalGap
	UnderbarRuleThickness
	UnderbarExtraDescender
	RadicalVerticalGap
	RadicalDisplayStyleVerticalGap
	RadicalRuleThickness
	RadicalExtraAscender
	RadicalKernBeforeDegree
	RadicalKernAfterDegree
	RadicalDegreeBottomRaisePercent

	// MathConstantCount is the number of MATH constants, not a constant.
	MathConstantCount
)

var mathConstantNames = [...]string{
	"scriptPercentScaleDown",
	"scriptScriptPercentScaleDown",
	"delimitedSubFormulaMinHeight",
	"displayOperatorMinHeight",
	"mathLeading",
	"axisHeight",
	"accentBaseHeight",
	"flattenedAccentBaseHeight",
	"subscriptShiftDown",
	"subscriptTopMax",
	"subscriptBaselineDropMin",
	"superscriptShiftUp",
	"superscriptShiftUpCramped",
	"superscriptBottomMin",
	"superscriptBaselineDropMax",
	"subSuperscriptGapMin",
	"superscriptBottomMaxWithSubscript",
	"spaceAfterScript",
	"upperLimitGapMin",
	"upperLimitBaselineRiseMin",
	"lowerLimitGapMin",
	"lowerLimitBaselineDropMin",
	"stackTopShiftUp",
	"stackTopDisplayStyleShiftUp",
	"stackBottomShiftDown",
	"stackBottomDisplayStyleShiftDown",
	"stackGapMin",
	"stackDisplayStyleGapMin",
	"stretchStackTopShiftUp",
	"stretchStackBottomShiftDown",
	"stretchStackGapAboveMin",
	"stretchStackGapBelowMin",
	"fractionNumeratorShiftUp",
	"fractionNumeratorDisplayStyleShiftUp",
	"fractionDenominatorShiftDown",
	"fractionDenominatorDisplayStyleShiftDown",
	"fractionNumeratorGapMin",
	"fractionNumDisplayStyleGapMin",
	"fractionRuleThickness",
	"fractionDenominatorGapMin",
	"fractionDenomDisplayStyleGapMin",
	"skewedFractionHorizontalGap",
	"skewedFractionVerticalGap",
	"overbarVerticalGap",
	"overbarRuleThickness",
	"overbarExtraAscender",
	"underbarVerticalGap",
	"underbarRuleThickness",
	"underbarExtraDescender",
	"radicalVerticalGap",
	"radicalDisplayStyleVerticalGap",
	"radicalRuleThickness",
	"radicalExtraAscender",
	"radicalKernBeforeDegree",
	"radicalKernAfterDegree",
	"radicalDegreeBottomRaisePercent",
}

// String returns the OpenType name for c.
func (c MathConstant) String() string {
	if int(c) < len(mathConstantNames) {
		return mathConstantNames[c]
	}
	return "MathConstant(" + strconv.Itoa(int(c)) + ")"
}

// MathKernCorner identifies one of the four corners in a MATH cut-in kerning
// record.
type MathKernCorner uint8

const (
	MathKernTopRight MathKernCorner = iota
	MathKernTopLeft
	MathKernBottomRight
	MathKernBottomLeft
)

// KernCurve is a MATH cut-in kerning step function. Heights contains n values
// in ascending order and Kerns contains n+1 values.
type KernCurve struct {
	Heights []int32
	Kerns   []int32
}

// At returns the kern value for correction height h.
func (k KernCurve) At(h int32) int32 {
	if len(k.Kerns) == 0 {
		return 0
	}
	lo, hi := 0, len(k.Heights)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if h < k.Heights[mid] {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if lo >= len(k.Kerns) {
		return k.Kerns[len(k.Kerns)-1]
	}
	return k.Kerns[lo]
}

// MathAxis selects a glyph construction axis.
type MathAxis uint8

const (
	VerticalAxis MathAxis = iota
	HorizontalAxis
)

// SizeVariant is one pre-drawn size of a stretchy glyph.
type SizeVariant struct {
	Glyph   GlyphID
	Advance int32
}

// AssemblyPart is one component of a stretchy-glyph assembly.
type AssemblyPart struct {
	Glyph          GlyphID
	StartConnector int32
	EndConnector   int32
	FullAdvance    int32
	Extender       bool
}

// Assembly is the complete recipe for constructing a stretchy glyph. Parts
// retain OpenType order, and the first and last parts are the endpoint glyphs.
type Assembly struct {
	ItalicCorrection    int32
	MinConnectorOverlap int32
	Parts               []AssemblyPart
}

// MathConstruction contains the pre-drawn variants and optional assembly for a
// glyph along one axis.
type MathConstruction struct {
	Variants []SizeVariant
	Assembly *Assembly
}

// Feature is one GSUB feature request. Value follows OpenType alternate-feature
// numbering: ssty=1 selects the first alternate and ssty=2 the second.
type Feature struct {
	Tag   string
	Value uint16
}

// MathScriptStyle selects the ssty alternate used by ShapeMath.
type MathScriptStyle uint8

const (
	NoScriptVariant MathScriptStyle = iota
	ScriptVariant
	ScriptScriptVariant
)

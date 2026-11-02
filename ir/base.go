// Package ir defines Untie's language-neutral math tree.
package ir

import "github.com/caelyreth/untie/source"

// Mode selects inline/text style or display style at the root.
type Mode uint8

const (
	Inline Mode = iota
	Display
)

// NodeKind is the stable discriminator for a node family.
type NodeKind uint8

const (
	KindRow NodeKind = iota + 1
	KindGlyph
	KindText
	KindOperator
	KindSpace
	KindFraction
	KindRadical
	KindScripts
	KindFenced
	KindAccent
	KindLine
	KindCancel
	KindTable
	KindMultiline
	KindStyle
	KindColor
	KindBox
	KindPhantom
	KindError
	KindPlaceholder
	KindBreak
	KindAlignMarker
	KindSizedDelimiter
)

// MathClass controls automatic spacing and operator behavior.
type MathClass uint8

const (
	ClassNormal MathClass = iota
	ClassAlphabetic
	ClassBinary
	ClassRelation
	ClassOpening
	ClassClosing
	ClassFence
	ClassPunctuation
	ClassLarge
	ClassUnary
	ClassVarying
	ClassInner
	ClassDiacritic
	ClassSpace
	ClassSpecial
)

// MathSize is the current typographic style size.
type MathSize uint8

const (
	SizeInherit MathSize = iota
	SizeDisplay
	SizeText
	SizeScript
	SizeScriptScript
)

// DelimiterSize is an explicit source-requested delimiter size tier. The four
// tiers correspond to the maximum heights 1.2em, 1.8em, 2.4em, and 3.0em in
// layout and are independent of the current math style.
type DelimiterSize uint8

const (
	DelimiterSize1 DelimiterSize = iota + 1
	DelimiterSize2
	DelimiterSize3
	DelimiterSize4
)

// Valid reports whether the delimiter size is one of the four explicit tiers.
func (s DelimiterSize) Valid() bool {
	return s >= DelimiterSize1 && s <= DelimiterSize4
}

// LimitsPlacement controls visual placement only: whether scripts are rendered
// beside a base or above/below it. It does not change ScriptRole semantics.
type LimitsPlacement uint8

const (
	LimitsAuto LimitsPlacement = iota
	LimitsAlways
	LimitsNever
	// LimitsDisplay places scripts above/below only in display style. Unlike
	// Auto, it does not depend on the base atom's class.
	LimitsDisplay
)

// Position is above or below for accents and lines.
type Position uint8

const (
	PositionAbove Position = iota
	PositionBelow
)

// Variant is a logical math alphabet. Concrete faces and glyph choices are font
// concerns.
type Variant uint8

const (
	VariantAuto Variant = iota
	VariantNormal
	VariantItalic
	VariantBold
	VariantBoldItalic
	VariantDoubleStruck
	VariantScript
	VariantBoldScript
	VariantFraktur
	VariantBoldFraktur
	VariantSans
	VariantBoldSans
	VariantMonospace
)

// PaintRole is a host-resolved semantic color.
type PaintRole uint8

const (
	PaintInk PaintRole = iota
	PaintError
	PaintPlaceholder
)

// RGBA is an explicit 8-bit source color.
type RGBA struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

// Paint is either a semantic role or an explicit source color.
type Paint struct {
	Role        PaintRole
	Explicit    RGBA
	HasExplicit bool
}

// FontRole identifies which host-provided font role text uses.
type FontRole uint8

const (
	FontMath FontRole = iota
	FontText
)

// SpaceKind identifies special space nodes.
type SpaceKind uint8

const (
	SpaceExplicit SpaceKind = iota
	SpaceInterword
	SpaceIgnored
)

// FractionStyle selects fraction construction.
type FractionStyle uint8

const (
	FractionVertical FractionStyle = iota
	FractionNoBar
	FractionSkewed
	FractionHorizontal
)

// FractionRole records the semantic operation represented by a fraction-shaped
// node. FractionStyle continues to control geometry.
type FractionRole uint8

const (
	FractionRoleFraction FractionRole = iota
	FractionRoleBinomial
	FractionRoleStacked
)

// TableRole identifies the semantic construct represented by a table grid.
type TableRole uint8

const (
	TableRoleMatrix TableRole = iota
	TableRoleArray
	TableRoleCases
	TableRoleVector
)

// ScriptRole identifies the semantic purpose of attached scripts independently
// of visual placement. For example, explicit \nolimits or Typst scripts() may
// place operator bounds beside the base without changing them from limits.
type ScriptRole uint8

const (
	ScriptRoleOrdinary ScriptRole = iota
	ScriptRoleLimits
	ScriptRoleOverlay
	ScriptRoleBraceLabel
)

// CancelRole distinguishes cancellation strokes from logical negation, which
// can share the same geometric overlay.
type CancelRole uint8

const (
	CancelRoleStrike CancelRole = iota
	CancelRoleNegation
)

// ColumnAlign is the alignment of a table column.
type ColumnAlign uint8

const (
	AlignCenter ColumnAlign = iota
	AlignLeft
	AlignRight
)

// BreakKind describes why a break point is legal.
type BreakKind uint8

const (
	BreakRelation BreakKind = iota
	BreakBinary
	BreakExplicit
)

// Base contains source and semantic properties shared by all IR nodes.
//
// Base is embedded by every concrete node. Its unexported kind and marker
// methods make Node a sealed public interface.
type Base struct {
	kind NodeKind

	Span   source.Range
	Origin source.Range

	Class   MathClass
	Size    MathSize
	Cramped bool
	Variant Variant
	Paint   Paint

	Limits        LimitsPlacement
	LeftSpacing   Length
	RightSpacing  Length
	IgnoreAsSpace bool
}

// NewBase constructs a Base with deterministic defaults.
func NewBase(kind NodeKind, span source.Range) Base {
	return Base{
		kind:    kind,
		Span:    span,
		Origin:  span,
		Class:   ClassNormal,
		Size:    SizeInherit,
		Variant: VariantAuto,
		Paint:   Paint{Role: PaintInk},
	}
}

// Kind returns the node discriminator.
func (b *Base) Kind() NodeKind {
	return b.kind
}

// NodeBase exposes shared fields.
func (b *Base) NodeBase() *Base {
	return b
}

func (*Base) node() {}

// Node is a sealed language-neutral math node.
type Node interface {
	NodeBase() *Base
	Kind() NodeKind
	node()
}

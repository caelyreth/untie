// Package ir defines Untie's language-neutral math tree.
package ir

import (
	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/source"
)

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
	KindOverlay
	KindRaise
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

// DelimiterSizingKind selects how an explicitly sized delimiter resolves its
// vertical target.
type DelimiterSizingKind uint8

const (
	// DelimiterSizingContent fits delimiters to their wrapped content.
	DelimiterSizingContent DelimiterSizingKind = iota
	// DelimiterSizingRelative multiplies the content-derived target by Value,
	// whose unit must be percent.
	DelimiterSizingRelative
	// DelimiterSizingAbsolute uses Value as an absolute target. Font-relative
	// units still resolve at the active typographic size.
	DelimiterSizingAbsolute
)

// DelimiterSizing is a source-independent delimiter target policy. It never
// encodes a dialect's command names or discrete size tiers.
type DelimiterSizing struct {
	Kind  DelimiterSizingKind
	Value Length
}

// ContentDelimiterSizing returns the default content-relative policy.
func ContentDelimiterSizing() DelimiterSizing {
	return DelimiterSizing{Kind: DelimiterSizingContent}
}

// RelativeDelimiterSizing returns a percentage-of-content policy.
func RelativeDelimiterSizing(value Length) DelimiterSizing {
	return DelimiterSizing{Kind: DelimiterSizingRelative, Value: value}
}

// AbsoluteDelimiterSizing returns a target-length policy.
func AbsoluteDelimiterSizing(value Length) DelimiterSizing {
	return DelimiterSizing{Kind: DelimiterSizingAbsolute, Value: value}
}

// Valid reports whether the policy is internally consistent.
func (s DelimiterSizing) Valid() bool {
	switch s.Kind {
	case DelimiterSizingContent:
		return s.Value.Unit == LengthZero && s.Value.Value == 0
	case DelimiterSizingRelative:
		return s.Value.Unit == LengthPercent && s.Value.Value >= 0
	case DelimiterSizingAbsolute:
		return s.Value.Unit >= LengthZero && s.Value.Unit < LengthPercent &&
			s.Value.Value >= 0 && (s.Value.Unit != LengthZero || s.Value.Value == 0)
	default:
		return false
	}
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

// OverlayAlignment positions zero-width content relative to its insertion
// point. It controls horizontal placement only; overlay content preserves its
// own vertical metrics and ink.
type OverlayAlignment uint8

const (
	// OverlayStart places the body's left edge at the insertion point.
	OverlayStart OverlayAlignment = iota
	// OverlayCenter centers the body on the insertion point.
	OverlayCenter
	// OverlayEnd places the body's right edge at the insertion point.
	OverlayEnd
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

	// NodeID is an optional, opaque host-assigned semantic identity. Zero means
	// unassigned. Nonzero IDs must be unique within a formula. Untie preserves
	// the ID through cloning and layout and never derives it from source
	// offsets.
	NodeID identity.NodeID

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

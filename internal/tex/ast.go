package tex

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type astKind uint8

const (
	astRow astKind = iota
	astGlyph
	astText
	astOperator
	astSpace
	astFraction
	astRadical
	astScripts
	astFenced
	astSizedDelimiter
	astAccent
	astLine
	astCancel
	astColor
	astBox
	astPhantom
	astClass
	astRule
	astStyle
	astVariant
	astTable
	astMultiline
	astError
	astPlaceholder
	astBreak
	astAlign
)

type astBase struct {
	kind   astKind
	span   source.Range
	origin source.Range
}

type astNode interface {
	base() *astBase
}

func (b *astBase) base() *astBase {
	return b
}

type astRowNode struct {
	astBase
	items []astNode
}

type astGlyphNode struct {
	astBase
	text    string
	class   ir.MathClass
	variant ir.Variant
	limits  ir.LimitsPlacement
}

type astTextNode struct {
	astBase
	text    string
	variant ir.Variant
}

type astOperatorNode struct {
	astBase
	text           string
	variant        ir.Variant
	limits         ir.LimitsPlacement
	semanticLimits bool
}

type astSpaceNode struct {
	astBase
	kind   ir.SpaceKind
	amount ir.Length
}

type astFractionNode struct {
	astBase
	numerator   astNode
	denominator astNode
	bar         bool
	style       ir.FractionStyle
	role        ir.FractionRole
	size        ir.MathSize
	infix       bool
}

type astRadicalNode struct {
	astBase
	radicand astNode
	index    astNode
}

type astScriptsNode struct {
	astBase
	main        astNode
	top         astNode
	bottom      astNode
	topLeft     astNode
	bottomLeft  astNode
	topRight    astNode
	bottomRight astNode
	limits      ir.LimitsPlacement
	role        ir.ScriptRole
}

type astFencedNode struct {
	astBase
	open     astNode
	close    astNode
	body     []astNode
	balanced bool
	size     ir.MathSize
}

type astSizedDelimiterNode struct {
	astBase
	text  string
	class ir.MathClass
	size  ir.DelimiterSize
}

type astAccentNode struct {
	astBase
	body       astNode
	mark       astNode
	position   ir.Position
	stretchy   bool
	dotless    bool
	exactWidth bool
}

type astLineNode struct {
	astBase
	body     astNode
	position ir.Position
}

type astCancelNode struct {
	astBase
	body     astNode
	cross    bool
	inverted bool
	role     ir.CancelRole
}

type astColorNode struct {
	astBase
	body  astNode
	paint ir.Paint
}

type astBoxNode struct {
	astBase
	body       astNode
	fill       ir.Paint
	hasFill    bool
	border     ir.Paint
	hasBorder  bool
	borderSize ir.Length
	radius     ir.Length
}

type astPhantomNode struct {
	astBase
	body   astNode
	width  bool
	height bool
	depth  bool
}

type astClassNode struct {
	astBase
	body      astNode
	class     ir.MathClass
	limits    ir.LimitsPlacement
	setLimits bool
}

type astRuleNode struct {
	astBase
	width  ir.Length
	height ir.Length
}

type astStyleNode struct {
	astBase
	body    astNode
	size    ir.MathSize
	cramped bool
}

type astVariantNode struct {
	astBase
	body    astNode
	variant ir.Variant
}

type astTableRow struct {
	cells []astNode
}

type astTableNode struct {
	astBase
	rows       []astTableRow
	columns    []ir.ColumnAlign
	rowGap     ir.Length
	columnGap  ir.Length
	vertical   []ir.TableLine
	horizontal []ir.TableLine
	role       ir.TableRole
	size       ir.MathSize
}

type astMultilineNode struct {
	astBase
	rows     [][]astNode
	centered bool
}

type astErrorNode struct {
	astBase
	text string
	code source.Code
}

type astPlaceholderNode struct {
	astBase
	reason string
}

type astBreakNode struct {
	astBase
	kind ir.BreakKind
}

type astAlignNode struct {
	astBase
}

func newASTBase(kind astKind, span source.Range) astBase {
	return astBase{kind: kind, span: span, origin: span}
}

func newASTBaseAt(kind astKind, span, origin source.Range) astBase {
	if origin.IsDetached() {
		origin = span
	}
	return astBase{kind: kind, span: span, origin: origin}
}

func tokenOrigin(t token) source.Range {
	if t.origin.IsDetached() {
		return t.span
	}
	return t.origin
}

func containerOrigin(open token, body astNode, span source.Range) source.Range {
	if open.origin.IsDetached() || open.origin == open.span {
		return span
	}
	origin := open.origin
	if body != nil && !body.base().origin.IsDetached() {
		origin = source.Merge(origin, body.base().origin)
	}
	return origin
}

func astSpan(nodes ...astNode) source.Range {
	var span source.Range
	first := true
	for _, n := range nodes {
		if n == nil {
			continue
		}
		b := n.base()
		if first {
			span = b.span
			first = false
		} else {
			span = source.Merge(span, b.span)
		}
	}
	if first {
		return source.Detached()
	}
	return span
}

func astOrigin(nodes ...astNode) source.Range {
	var origin source.Range
	first := true
	for _, n := range nodes {
		if n == nil {
			continue
		}
		b := n.base()
		if first {
			origin = b.origin
			first = false
		} else {
			origin = source.Merge(origin, b.origin)
		}
	}
	if first {
		return source.Detached()
	}
	return origin
}

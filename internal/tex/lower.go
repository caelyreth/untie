package tex

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type lowerContext struct {
	variant ir.Variant
	size    ir.MathSize
	cramped bool
}

type lowerer struct {
	limits     source.Limits
	diagnostic func(source.Diagnostic)
	count      int
	limitHit   bool
	exhausted  bool
}

func newLowerer(limits source.Limits, diagnostic func(source.Diagnostic)) *lowerer {
	return &lowerer{limits: limits, diagnostic: diagnostic}
}

func (l *lowerer) lower(node astNode, ctx lowerContext) ir.Node {
	if node == nil {
		return nil
	}
	if l.count >= l.limits.MaxNodes {
		if !l.limitHit {
			l.limitHit = true
			l.diagnostic(source.Diagnostic{
				Range:    nonEmptyRange(node.base().span),
				Severity: source.SeverityError,
				Code:     source.CodeLimitExceeded,
				Message:  "TeX IR node limit exceeded",
			})
		}
		l.exhausted = true
		return l.placeholder(node.base().span, "IR node limit exceeded")
	}
	l.count++

	switch n := node.(type) {
	case *astRowNode:
		items := make([]ir.Node, 0, len(n.items))
		for _, item := range n.items {
			if l.exhausted {
				break
			}
			child := l.lower(item, ctx)
			if child != nil {
				items = append(items, child)
			}
		}
		out := ir.NewRow(n.span, items...)
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astGlyphNode:
		out := ir.NewGlyph(n.span, n.text)
		variant := n.variant
		if variant == ir.VariantAuto {
			variant = ctx.variant
		}
		out.Base.Variant = variant
		out.Base.Limits = n.limits
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, n.class)
		return out
	case *astTextNode:
		out := ir.NewText(n.span, n.text)
		out.Base.Variant = n.variant
		if out.Base.Variant == ir.VariantAuto {
			out.Base.Variant = ctx.variant
		}
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astOperatorNode:
		out := ir.NewOperator(n.span, n.text)
		out.Base.Limits = n.limits
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassLarge)
		return out
	case *astSpaceNode:
		out := ir.NewSpace(n.span, n.kind, n.amount)
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassSpace)
		return out
	case *astFractionNode:
		numerator := l.lower(n.numerator, ctx)
		denominator := l.lower(n.denominator, ctx)
		out := ir.NewFraction(n.span, numerator, denominator)
		out.Bar = n.bar
		out.Style = n.style
		if n.size == ir.SizeInherit {
			out.Base.Size = ctx.size
		} else {
			out.Base.Size = n.size
		}
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astRadicalNode:
		radicand := l.lower(n.radicand, ctx)
		var index ir.Node
		if n.index != nil {
			index = l.lower(n.index, ctx)
		}
		out := ir.NewRadical(n.span, radicand, index)
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astScriptsNode:
		base := l.lower(n.main, ctx)
		var top, bottom ir.Node
		if n.top != nil {
			top = l.lower(n.top, ctx)
		}
		if n.bottom != nil {
			bottom = l.lower(n.bottom, ctx)
		}
		out := ir.NewScripts(n.span, base)
		out.Top = top
		out.Bottom = bottom
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		if base != nil {
			out.Base.Class = base.NodeBase().Class
		}
		if n.limits != ir.LimitsAuto {
			out.Base.Limits = n.limits
			if base != nil {
				base.NodeBase().Limits = n.limits
			}
		}
		return out
	case *astFencedNode:
		var open, close ir.Node
		if n.open != nil {
			open = l.lower(n.open, ctx)
		}
		if n.close != nil {
			close = l.lower(n.close, ctx)
		}
		// Fenced permits omitted delimiters semantically, but the current
		// neutral IR validator expects non-nil children in the flattened fence.
		// Empty rows are the zero-ink representation of a null delimiter.
		if open == nil {
			open = ir.NewRow(n.span)
			open.NodeBase().Class = ir.ClassOpening
		}
		if close == nil {
			close = ir.NewRow(n.span)
			close.NodeBase().Class = ir.ClassClosing
		}
		body := make([]ir.Node, 0, len(n.body))
		for _, item := range n.body {
			if l.exhausted {
				break
			}
			child := l.lower(item, ctx)
			if child != nil {
				body = append(body, child)
			}
		}
		if len(body) == 0 {
			body = append(body, ir.NewRow(n.span))
		}
		out := ir.NewFenced(n.span, body...)
		out.Open = open
		out.Close = close
		out.Balanced = n.balanced
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astAccentNode:
		body := l.lower(n.body, ctx)
		mark := l.lower(n.mark, ctx)
		out := ir.NewAccent(n.span, body, mark, n.position)
		out.Stretchy = n.stretchy
		out.Dotless = n.dotless
		out.ExactFrameWidth = n.exactWidth
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astLineNode:
		body := l.lower(n.body, ctx)
		out := ir.NewLine(n.span, body, n.position)
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astCancelNode:
		body := l.lower(n.body, ctx)
		out := ir.NewCancel(n.span, body)
		out.Cross = n.cross
		out.Inverted = n.inverted
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astStyleNode:
		childCtx := ctx
		if n.size != ir.SizeInherit {
			childCtx.size = n.size
		}
		childCtx.cramped = n.cramped
		body := l.lower(n.body, childCtx)
		out := ir.NewStyle(n.span, body)
		out.SetSize = n.size != ir.SizeInherit
		out.NewSize = n.size
		out.SetCramped = n.cramped
		out.Cramped = n.cramped
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astVariantNode:
		childCtx := ctx
		childCtx.variant = n.variant
		body := l.lower(n.body, childCtx)
		// Variant is inherited by leaves; a redundant row wrapper would add
		// no layout semantics.
		return body
	case *astTableNode:
		rows := make([]ir.TableRow, 0, len(n.rows))
		for _, row := range n.rows {
			if l.exhausted {
				break
			}
			cells := make([]ir.Node, 0, len(row.cells))
			for _, cell := range row.cells {
				if l.exhausted {
					break
				}
				cells = append(cells, l.lower(cell, ctx))
			}
			rows = append(rows, ir.TableRow{Cells: cells})
		}
		out := ir.NewTable(n.span, rows...)
		out.Columns = append([]ir.ColumnAlign(nil), n.columns...)
		out.RowGap = n.rowGap
		out.ColumnGap = n.columnGap
		out.Horizontal = append([]ir.TableLine(nil), n.horizontal...)
		out.Vertical = append([]ir.TableLine(nil), n.vertical...)
		if n.size == ir.SizeInherit {
			out.Base.Size = ctx.size
		} else {
			out.Base.Size = n.size
		}
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astMultilineNode:
		rows := make([][]ir.Node, 0, len(n.rows))
		for _, row := range n.rows {
			if l.exhausted {
				break
			}
			outRow := make([]ir.Node, 0, len(row))
			for _, item := range row {
				if l.exhausted {
					break
				}
				outRow = append(outRow, l.lower(item, ctx))
			}
			rows = append(rows, outRow)
		}
		out := ir.NewMultiline(n.span, rows...)
		out.Centered = n.centered
		out.Base.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astErrorNode:
		out := ir.NewError(n.span, n.text, n.code)
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astPlaceholderNode:
		return l.placeholder(n.span, n.reason)
	case *astBreakNode:
		out := ir.NewBreak(n.span, n.kind, 0)
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassSpecial)
		return out
	case *astAlignNode:
		out := ir.NewAlignMarker(n.span)
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassSpecial)
		return out
	default:
		l.diagnostic(source.Diagnostic{
			Range:    nonEmptyRange(node.base().span),
			Severity: source.SeverityError,
			Code:     source.CodeInternal,
			Message:  "unknown TeX AST node during lowering",
		})
		return l.placeholder(node.base().span, "unknown AST node")
	}
}

func (l *lowerer) placeholder(span source.Range, reason string) ir.Node {
	l.count++
	return ir.NewPlaceholder(nonEmptyRange(span), reason)
}

func (l *lowerer) applyBase(base *ir.Base, ast *astBase, ctx lowerContext, class ir.MathClass) {
	base.Span = ast.span
	base.Origin = ast.origin
	if base.Origin.IsDetached() {
		base.Origin = base.Span
	}
	base.Class = class
	if base.Variant == ir.VariantAuto {
		base.Variant = ctx.variant
	}
	if base.Size == ir.SizeInherit {
		base.Size = ctx.size
	}
	base.Cramped = ctx.cramped
}

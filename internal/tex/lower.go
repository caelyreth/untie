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

	// A variant wrapper is inherited by leaves and does not allocate an IR node.
	if n, ok := node.(*astVariantNode); ok {
		childCtx := ctx
		childCtx.variant = n.variant
		return l.lower(n.body, childCtx)
	}

	if !l.reserve(node.base().span) {
		return nil
	}

	switch n := node.(type) {
	case *astRowNode:
		items := make([]ir.Node, 0, len(n.items))
		for _, item := range n.items {
			child := l.lower(item, ctx)
			if l.exhausted {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
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
		out.Variant = variant
		out.Limits = n.limits
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, n.class)
		return out
	case *astTextNode:
		out := ir.NewText(n.span, n.text)
		out.Variant = n.variant
		if out.Variant == ir.VariantAuto {
			out.Variant = ctx.variant
		}
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astOperatorNode:
		out := ir.NewOperator(n.span, n.text)
		out.Limits = n.limits
		out.Size = ctx.size
		out.Variant = n.variant
		if out.Variant == ir.VariantAuto {
			out.Variant = ir.VariantNormal
		}
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassLarge)
		return out
	case *astSpaceNode:
		out := ir.NewSpace(n.span, n.kind, n.amount)
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassSpace)
		return out
	case *astFractionNode:
		numerator := l.lower(n.numerator, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		denominator := l.lower(n.denominator, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		if numerator == nil || denominator == nil {
			return l.placeholder(n.span, "missing fraction part")
		}
		out := ir.NewFraction(n.span, numerator, denominator)
		out.Bar = n.bar
		out.Style = n.style
		if n.size == ir.SizeInherit {
			out.Size = ctx.size
		} else {
			out.Size = n.size
		}
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astRadicalNode:
		radicand := l.lower(n.radicand, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		var index ir.Node
		if n.index != nil {
			index = l.lower(n.index, ctx)
			if l.exhausted {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
		}
		if radicand == nil {
			return l.placeholder(n.span, "missing radicand")
		}
		out := ir.NewRadical(n.span, radicand, index)
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astScriptsNode:
		base := l.lower(n.main, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		var top, bottom ir.Node
		if n.top != nil {
			top = l.lower(n.top, ctx)
			if l.exhausted {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
		}
		if n.bottom != nil {
			bottom = l.lower(n.bottom, ctx)
			if l.exhausted {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
		}
		if base == nil {
			return l.placeholder(n.span, "missing script base")
		}
		out := ir.NewScripts(n.span, base)
		out.Top = top
		out.Bottom = bottom
		out.Size = ctx.size
		out.Limits = n.limits
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		out.Class = base.NodeBase().Class
		base.NodeBase().Limits = n.limits
		return out
	case *astFencedNode:
		var open, close ir.Node
		if n.open != nil {
			open = l.lower(n.open, ctx)
			if l.exhausted {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
		}
		if n.close != nil {
			close = l.lower(n.close, ctx)
			if l.exhausted {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
		}
		// Fenced permits omitted delimiters semantically, but the current
		// neutral IR validator expects non-nil children in the flattened fence.
		// Empty rows are the zero-ink representation of a null delimiter.
		if open == nil {
			if !l.reserve(n.span) {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
			open = ir.NewRow(n.span)
			open.NodeBase().Class = ir.ClassOpening
		}
		if close == nil {
			if !l.reserve(n.span) {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
			close = ir.NewRow(n.span)
			close.NodeBase().Class = ir.ClassClosing
		}
		body := make([]ir.Node, 0, len(n.body))
		for _, item := range n.body {
			child := l.lower(item, ctx)
			if l.exhausted {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
			if child != nil {
				body = append(body, child)
			}
		}
		if len(body) == 0 {
			if !l.reserve(n.span) {
				return l.placeholder(n.span, "IR node limit exceeded")
			}
			body = append(body, ir.NewRow(n.span))
		}
		out := ir.NewFenced(n.span, body...)
		out.Open = open
		out.Close = close
		out.Balanced = n.balanced
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astAccentNode:
		body := l.lower(n.body, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		mark := l.lower(n.mark, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		if body == nil || mark == nil {
			return l.placeholder(n.span, "missing accent part")
		}
		out := ir.NewAccent(n.span, body, mark, n.position)
		out.Stretchy = n.stretchy
		out.Dotless = n.dotless
		out.ExactFrameWidth = n.exactWidth
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astLineNode:
		body := l.lower(n.body, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		if body == nil {
			return l.placeholder(n.span, "missing line body")
		}
		out := ir.NewLine(n.span, body, n.position)
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astCancelNode:
		body := l.lower(n.body, ctx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		if body == nil {
			return l.placeholder(n.span, "missing cancel body")
		}
		out := ir.NewCancel(n.span, body)
		out.Cross = n.cross
		out.Inverted = n.inverted
		out.Size = ctx.size
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassNormal)
		return out
	case *astStyleNode:
		childCtx := ctx
		if n.size != ir.SizeInherit {
			childCtx.size = n.size
		}
		childCtx.cramped = n.cramped
		body := l.lower(n.body, childCtx)
		if l.exhausted {
			return l.placeholder(n.span, "IR node limit exceeded")
		}
		if body == nil {
			return l.placeholder(n.span, "missing style body")
		}
		out := ir.NewStyle(n.span, body)
		out.SetSize = n.size != ir.SizeInherit
		out.NewSize = n.size
		out.SetCramped = n.cramped
		out.Cramped = n.cramped
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astTableNode:
		rows := make([]ir.TableRow, 0, len(n.rows))
		totalCells := 0
		for _, row := range n.rows {
			rowCells := make([]ir.Node, 0, len(row.cells))
			for _, cell := range row.cells {
				if totalCells >= l.limits.MaxTableCells {
					l.limit(n.span, "TeX table cell limit exceeded")
					return l.placeholder(n.span, "table cell limit exceeded")
				}
				totalCells++
				child := l.lower(cell, ctx)
				if l.exhausted {
					return l.placeholder(n.span, "IR node limit exceeded")
				}
				if child == nil {
					return l.placeholder(n.span, "missing table cell")
				}
				rowCells = append(rowCells, child)
			}
			rows = append(rows, ir.TableRow{Cells: rowCells})
		}
		out := ir.NewTable(n.span, rows...)
		out.Columns = append([]ir.ColumnAlign(nil), n.columns...)
		out.RowGap = n.rowGap
		out.ColumnGap = n.columnGap
		out.Horizontal = append([]ir.TableLine(nil), n.horizontal...)
		out.Vertical = append([]ir.TableLine(nil), n.vertical...)
		if n.size == ir.SizeInherit {
			out.Size = ctx.size
		} else {
			out.Size = n.size
		}
		l.applyBase(&out.Base, &n.astBase, ctx, ir.ClassInner)
		return out
	case *astMultilineNode:
		rows := make([][]ir.Node, 0, len(n.rows))
		for _, row := range n.rows {
			outRow := make([]ir.Node, 0, len(row))
			for _, item := range row {
				child := l.lower(item, ctx)
				if l.exhausted {
					return l.placeholder(n.span, "IR node limit exceeded")
				}
				if child == nil {
					return l.placeholder(n.span, "missing multiline item")
				}
				outRow = append(outRow, child)
			}
			rows = append(rows, outRow)
		}
		if len(rows) == 0 {
			return l.placeholder(n.span, "empty multiline")
		}
		out := ir.NewMultiline(n.span, rows...)
		out.Centered = n.centered
		out.Size = ctx.size
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
			Range:    normalizeRange(node.base().span),
			Severity: source.SeverityError,
			Code:     source.CodeInternal,
			Message:  "unknown TeX AST node during lowering",
		})
		return l.placeholder(node.base().span, "unknown AST node")
	}
}

func (l *lowerer) reserve(span source.Range) bool {
	if l.count >= l.limits.MaxNodes {
		l.limit(span, "TeX IR node limit exceeded")
		l.exhausted = true
		return false
	}
	l.count++
	return true
}

func (l *lowerer) limit(span source.Range, message string) {
	if l.limitHit {
		return
	}
	l.limitHit = true
	l.diagnostic(source.Diagnostic{
		Range:    normalizeRange(span),
		Severity: source.SeverityError,
		Code:     source.CodeLimitExceeded,
		Message:  message,
	})
}

func (l *lowerer) placeholder(span source.Range, reason string) ir.Node {
	return ir.NewPlaceholder(normalizeRange(span), reason)
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

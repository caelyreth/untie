package typst

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// reserveNode claims one slot in the exact IR output budget. Once the budget
// is exhausted, lowering aborts and the caller replaces the partial tree with
// a bounded fallback. Constructors must reserve before allocating a node.
func (l *lowerer) reserveNode(span source.Range) bool {
	if l.exhausted {
		return false
	}
	if l.nodes >= l.limits.MaxNodes {
		l.exhausted = true
		if !l.nodeLimitReported {
			l.nodeLimitReported = true
			l.errorAt(span, source.CodeLimitExceeded, "node limit exceeded")
		}
		return false
	}
	l.nodes++
	return true
}

// releaseNode returns a reservation for an intermediate node that is replaced
// rather than attached to the final tree.
func (l *lowerer) releaseNode() {
	if l.nodes > 0 {
		l.nodes--
	}
}

func boundedFallback(span source.Range, text string, code source.Code, maxNodes int) ir.Node {
	text = sanitizeFrontendText(text)
	if text == "" {
		text = "?"
	}
	errNode := ir.NewError(span, text, code)
	if maxNodes < 2 {
		return errNode
	}
	return ir.NewRow(span, errNode)
}

func (l *lowerer) fallbackWithinBudget(span source.Range) ir.Node {
	return boundedFallback(span, "node limit exceeded", source.CodeLimitExceeded, l.limits.MaxNodes)
}

// inspectText lowers content only to recover its textual value. That scratch
// tree is not part of the result, so it uses an isolated bounded budget rather
// than consuming or exhausting the exact output-node budget.
func (l *lowerer) inspectText(n *astNode) (text string) {
	nodes := l.nodes
	exhausted := l.exhausted
	nodeLimitReported := l.nodeLimitReported
	tableLimit := l.tableLimit
	l.nodes = 0
	l.exhausted = false
	l.nodeLimitReported = true
	l.tableLimit = false
	defer func() {
		l.nodes = nodes
		l.exhausted = exhausted
		l.nodeLimitReported = nodeLimitReported
		l.tableLimit = tableLimit
	}()
	return nodeText(l.lower(n))
}

func (l *lowerer) newRow(span source.Range, items ...ir.Node) *ir.Row {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewRow(span, items...)
}

func (l *lowerer) newGlyph(span source.Range, text string) *ir.Glyph {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewGlyph(span, text)
}

func (l *lowerer) newText(span source.Range, text string) *ir.Text {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewText(span, text)
}

func (l *lowerer) newOperator(span source.Range, text string) *ir.Operator {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewOperator(span, text)
}

func (l *lowerer) newSpace(span source.Range, kind ir.SpaceKind, amount ir.Length) *ir.Space {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewSpace(span, kind, amount)
}

func (l *lowerer) newFraction(span source.Range, numerator, denominator ir.Node) *ir.Fraction {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewFraction(span, numerator, denominator)
}

func (l *lowerer) newRadical(span source.Range, radicand, index ir.Node) *ir.Radical {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewRadical(span, radicand, index)
}

func (l *lowerer) newScripts(span source.Range, base ir.Node) *ir.Scripts {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewScripts(span, base)
}

func (l *lowerer) newFenced(span source.Range, body ...ir.Node) *ir.Fenced {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewFenced(span, body...)
}

func (l *lowerer) newSizedDelimiter(span source.Range, text string, sizing ir.DelimiterSizing) *ir.SizedDelimiter {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewSizedDelimiter(span, text, sizing)
}

func (l *lowerer) newAccent(span source.Range, body, accent ir.Node, position ir.Position) *ir.Accent {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewAccent(span, body, accent, position)
}

func (l *lowerer) newLine(span source.Range, body ir.Node, position ir.Position) *ir.Line {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewLine(span, body, position)
}

func (l *lowerer) newCancel(span source.Range, body ir.Node) *ir.Cancel {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewCancel(span, body)
}

func (l *lowerer) newTable(span source.Range, rows ...ir.TableRow) *ir.Table {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewTable(span, rows...)
}

func (l *lowerer) newMultiline(span source.Range, rows ...[]ir.Node) *ir.Multiline {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewMultiline(span, rows...)
}

func (l *lowerer) newStyle(span source.Range, body ir.Node) *ir.Style {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewStyle(span, body)
}

func (l *lowerer) newColor(span source.Range, body ir.Node, paint ir.Paint) *ir.Color {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewColor(span, body, paint)
}

func (l *lowerer) newBox(span source.Range, body ir.Node) *ir.Box {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewBox(span, body)
}

func (l *lowerer) newPhantom(span source.Range, body ir.Node) *ir.Phantom {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewPhantom(span, body)
}

func (l *lowerer) newError(span source.Range, text string, code source.Code) *ir.ErrorNode {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewError(span, text, code)
}

func (l *lowerer) newPlaceholder(span source.Range, reason string) *ir.Placeholder {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewPlaceholder(span, reason)
}

func (l *lowerer) newBreak(span source.Range, kind ir.BreakKind) *ir.Break {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewBreak(span, kind)
}

func (l *lowerer) newAlignMarker(span source.Range) *ir.AlignMarker {
	if !l.reserveNode(span) {
		return nil
	}
	return l.arena.NewAlignMarker(span)
}

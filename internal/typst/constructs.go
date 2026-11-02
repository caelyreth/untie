package typst

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (l *lowerer) lowerSqrt(call *astNode) ir.Node {
	body := l.contentArg(call, "radicand", 0, true)
	if l.exhausted {
		return nil
	}
	return l.newRadical(call.span, body, nil)
}

func (l *lowerer) lowerRootCall(call *astNode) ir.Node {
	positional := l.positionalArguments(call)
	index := l.contentNamed(call, "index")
	if index == nil && len(positional) >= 2 {
		index = l.lower(positional[0].node)
		positional = positional[1:]
	}
	radicand := l.contentNamed(call, "radicand")
	if radicand == nil && len(positional) > 0 {
		radicand = l.lower(positional[len(positional)-1].node)
	}
	if radicand == nil {
		l.errorAt(call.span, source.CodeMissingArgument, "missing argument: radicand")
		radicand = l.placeholder(call.span, "missing radicand")
	}
	if l.exhausted {
		return nil
	}
	return l.newRadical(call.span, radicand, index)
}

func (l *lowerer) lowerFracCall(call *astNode) ir.Node {
	num := l.contentArg(call, "num", 0, true)
	denom := l.contentArg(call, "denom", 1, true)
	if l.exhausted {
		return nil
	}
	fr := l.newFraction(call.span, num, denom)
	if fr == nil {
		return nil
	}
	fr.Padding = builtinEmLength(0.1)
	if styleArg := namedArgument(call, "style"); styleArg != nil {
		style, ok := l.valueString(styleArg.node)
		if !ok {
			l.errorAt(styleArg.span, source.CodeInvalidValue, "expected string, found %s", l.valueType(styleArg.node))
		} else {
			switch style {
			case "vertical":
				fr.Style = ir.FractionVertical
			case "skewed":
				fr.Style = ir.FractionSkewed
				fr.Bar = false
			case "horizontal":
				fr.Style = ir.FractionHorizontal
				fr.Bar = false
			default:
				l.errorAt(styleArg.span, source.CodeInvalidValue, "unknown fraction style %q", style)
			}
		}
	}
	return fr
}

func (l *lowerer) lowerBinom(call *astNode) ir.Node {
	upper := l.contentArg(call, "upper", 0, true)
	lowerArgs := l.positionalArguments(call)
	if len(lowerArgs) > 1 {
		lowerArgs = lowerArgs[1:]
	} else {
		lowerArgs = nil
	}
	var lower ir.Node
	if named := l.contentNamed(call, "lower"); named != nil {
		lower = named
	} else if len(lowerArgs) > 0 {
		lower = l.lower(lowerRowNode(lowerArgs))
	} else {
		l.errorAt(call.span, source.CodeMissingArgument, "missing argument: lower")
		lower = l.placeholder(call.span, "missing lower")
	}
	if l.exhausted {
		return nil
	}
	fr := l.newFraction(call.span, upper, lower)
	if fr == nil {
		return nil
	}
	fr.Bar = false
	fr.Style = ir.FractionNoBar
	fr.Role = ir.FractionRoleBinomial
	fr.Padding = builtinEmLength(0.1)
	fenced := l.newFenced(call.span, fr)
	if fenced == nil {
		return nil
	}
	open := l.glyph(source.NewRange(call.span.Start, call.span.Start), "(")
	if open == nil {
		return nil
	}
	closeGlyph := l.glyph(source.NewRange(call.span.End, call.span.End), ")")
	if closeGlyph == nil {
		return nil
	}
	open.Class = ir.ClassOpening
	closeGlyph.Class = ir.ClassClosing
	fenced.Open = open
	fenced.Close = closeGlyph
	fenced.Class = ir.ClassInner
	return fenced
}

func (l *lowerer) lowerAttachCall(call *astNode) ir.Node {
	base := l.contentArg(call, "base", 0, true)
	if l.exhausted {
		return nil
	}
	scripts := l.newScripts(call.span, base)
	if scripts == nil {
		return nil
	}
	scripts.Top = l.contentNamed(call, "t")
	scripts.Bottom = l.contentNamed(call, "b")
	scripts.TopLeft = l.contentNamed(call, "tl")
	scripts.BottomLeft = l.contentNamed(call, "bl")
	scripts.TopRight = l.contentNamed(call, "tr")
	scripts.BottomRight = l.contentNamed(call, "br")
	if l.exhausted {
		return nil
	}
	if base, ok := scripts.BaseItem.(*ir.Scripts); ok && base.Role != ir.ScriptRoleOrdinary {
		scripts.Role = base.Role
	}
	if scripts.Role == ir.ScriptRoleOrdinary {
		scripts.Role = attachmentScriptRole(scripts.BaseItem, scripts.Top != nil || scripts.Bottom != nil)
	}
	inheritScriptClass(scripts)
	// Corner-only attachments decorate the base on both sides; they do not
	// acquire the base's limit placement. This mirrors \sideset, whose outer
	// corner-script layer remains auto while its inner base uses limits.
	if scripts.Top != nil || scripts.Bottom != nil {
		inheritScriptLimits(scripts)
	} else if inner, ok := scripts.BaseItem.(*ir.Scripts); ok && scriptless(inner) {
		// A script-less limits wrapper has no placement to perform. Its base
		// still retains the explicit semantic limits, while the empty wrapper
		// stays context-sensitive for the corner-script layer above it.
		inner.Limits = ir.LimitsAuto
	}
	return scripts
}

func (l *lowerer) lowerLimitsCall(call *astNode, name string) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	if l.exhausted {
		return nil
	}
	scripts := l.newScripts(call.span, body)
	if scripts == nil {
		return nil
	}
	if name == "scripts" {
		scripts.Limits = ir.LimitsNever
		scripts.Role = baseScriptRole(body)
	} else {
		scripts.Limits = ir.LimitsAlways
		scripts.Role = baseScriptRole(body)
		if inlineArg := namedArgument(call, "inline"); inlineArg != nil {
			if inline, ok := l.valueBool(inlineArg.node); ok && !inline {
				scripts.Limits = ir.LimitsDisplay
			}
		}
	}
	if body != nil {
		body.NodeBase().Limits = scripts.Limits
	}
	inheritScriptMetadata(scripts)
	return scripts
}

func baseScriptRole(node ir.Node) ir.ScriptRole {
	if hasLimitSemantics(node) {
		return ir.ScriptRoleLimits
	}
	// limits() and scripts() only choose visual placement. Whether a non-operator
	// becomes an overlay is decided by the attachment's selected script slots.
	return ir.ScriptRoleOrdinary
}

func scriptless(node *ir.Scripts) bool {
	return node != nil &&
		node.Top == nil && node.Bottom == nil &&
		node.TopLeft == nil && node.BottomLeft == nil &&
		node.TopRight == nil && node.BottomRight == nil
}

func attachmentScriptRole(node ir.Node, smart bool) ir.ScriptRole {
	if hasLimitSemantics(node) {
		return ir.ScriptRoleLimits
	}
	// Typst's t/b attachments are smart-positioned. They are overlays only when
	// the base's relation/limit semantics place them above or below; otherwise
	// they are ordinary right scripts. Explicit corner attachments do not alter
	// the role.
	if smart && nodeUsesLimits(node) {
		return ir.ScriptRoleOverlay
	}
	return ir.ScriptRoleOrdinary
}

func nodeUsesLimits(node ir.Node) bool {
	if node == nil || node.NodeBase() == nil {
		return false
	}
	base := node.NodeBase()
	switch effectiveNodeLimits(node) {
	case ir.LimitsAlways, ir.LimitsDisplay:
		return true
	case ir.LimitsNever:
		return false
	}
	return base.Class == ir.ClassRelation
}

func (l *lowerer) lowerLR(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	if l.exhausted {
		return nil
	}
	sizing := l.delimiterSizing(call, "lr")
	if fenced, ok := body.(*ir.Fenced); ok {
		fenced.Class = ir.ClassInner
		fenced.Sizing = sizing
		return fenced
	}
	if row, ok := body.(*ir.Row); ok && len(row.Items) == 1 {
		if fenced, ok := row.Items[0].(*ir.Fenced); ok {
			fenced.Class = ir.ClassInner
			fenced.Sizing = sizing
			l.releaseNode()
			return fenced
		}
	}
	if node, ok := l.standaloneSizedDelimiter(body, sizing); ok {
		return node
	}
	if row, ok := body.(*ir.Row); ok && len(row.Items) >= 2 {
		first, firstOK := delimiterNode(row.Items[0])
		last, lastOK := delimiterNode(row.Items[len(row.Items)-1])
		if firstOK && lastOK {
			l.releaseNode()
			fenced := l.newFenced(call.span, row.Items[1:len(row.Items)-1]...)
			if fenced == nil {
				return nil
			}
			if glyph, ok := first.(*ir.Glyph); ok {
				glyph.Class = ir.ClassOpening
			}
			if glyph, ok := last.(*ir.Glyph); ok {
				glyph.Class = ir.ClassClosing
			}
			fenced.Open = first
			fenced.Close = last
			fenced.Balanced = true
			fenced.Class = ir.ClassInner
			fenced.Sizing = sizing
			return fenced
		}
	}
	// lr() makes delimiters already present in its body stretch. If no fence is
	// present, the body itself is the complete result.
	return body
}

func (l *lowerer) standaloneSizedDelimiter(node ir.Node, sizing ir.DelimiterSizing) (ir.Node, bool) {
	released := 0
	if row, ok := node.(*ir.Row); ok && len(row.Items) == 1 {
		node = row.Items[0]
		released++
	}
	glyph, ok := node.(*ir.Glyph)
	if !ok || !isFenceGlyph(glyph) || sizing.Kind == ir.DelimiterSizingContent {
		return nil, false
	}
	for range released + 1 {
		l.releaseNode()
	}
	out := l.newSizedDelimiter(glyph.Span, glyph.Text, sizing)
	if out == nil {
		return nil, true
	}
	out.Origin = glyph.Origin
	out.Variant = glyph.Variant
	out.Paint = glyph.Paint
	switch glyph.Class {
	case ir.ClassOpening, ir.ClassClosing, ir.ClassRelation:
		out.Class = glyph.Class
	default:
		out.Class = ir.ClassNormal
	}
	return out, true
}

func delimiterNode(n ir.Node) (ir.Node, bool) {
	if glyph, ok := n.(*ir.Glyph); ok {
		return glyph, isFenceGlyph(glyph)
	}
	if row, ok := n.(*ir.Row); ok && len(row.Items) == 0 {
		return row, true
	}
	return nil, false
}

func isFenceGlyph(g *ir.Glyph) bool {
	return g.Class == ir.ClassOpening || g.Class == ir.ClassClosing || g.Class == ir.ClassFence
}

func (l *lowerer) lowerMid(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	if g, ok := body.(*ir.Glyph); ok {
		g.Class = ir.ClassFence
		return g
	}
	return body
}

func (l *lowerer) lowerFenceFunction(call *astNode, name string) ir.Node {
	sizing := l.delimiterSizing(call, name)
	pairs := map[string][2]string{
		"abs": {"|", "|"}, "norm": {"‖", "‖"}, "floor": {"⌊", "⌋"},
		"ceil": {"⌈", "⌉"}, "round": {"(", ")"},
	}
	pair := pairs[name]
	body := l.contentArg(call, "body", 0, true)
	if l.exhausted {
		return nil
	}
	if _, ok := body.(*ir.Row); ok {
		l.releaseNode()
	}
	fenced := l.newFenced(call.span, l.flattenFenceBody(body)...)
	if fenced == nil {
		return nil
	}
	open := l.glyph(source.NewRange(call.span.Start, call.span.Start), pair[0])
	if open == nil {
		return nil
	}
	closeGlyph := l.glyph(source.NewRange(call.span.End, call.span.End), pair[1])
	if closeGlyph == nil {
		return nil
	}
	open.Class = ir.ClassOpening
	closeGlyph.Class = ir.ClassClosing
	if name == "abs" || name == "norm" {
		open.Class = ir.ClassFence
		closeGlyph.Class = ir.ClassFence
	}
	fenced.Open = open
	fenced.Close = closeGlyph
	fenced.Balanced = true
	fenced.Class = ir.ClassInner
	fenced.Sizing = sizing
	return fenced
}

func (l *lowerer) lowerDelimiterFunction(call *astNode, def symbolDef) ir.Node {
	body := l.contentNamed(call, "body")
	if body == nil {
		body = l.contentArg(call, "body", 0, true)
	}
	if l.exhausted {
		return nil
	}
	if _, ok := body.(*ir.Row); ok {
		l.releaseNode()
	}
	closeText := matchingDelimiter(def.text)
	fenced := l.newFenced(call.span, l.flattenFenceBody(body)...)
	if fenced == nil {
		return nil
	}
	open := l.glyph(source.NewRange(call.span.Start, call.span.Start), def.text)
	if open == nil {
		return nil
	}
	closeClass := ir.ClassClosing
	if def.class == ir.ClassFence {
		open.Class = ir.ClassFence
		closeClass = ir.ClassFence
	} else {
		open.Class = ir.ClassOpening
	}
	fenced.Open = open
	if closeText != "" {
		closeGlyph := l.glyph(source.NewRange(call.span.End, call.span.End), closeText)
		if closeGlyph == nil {
			return nil
		}
		closeGlyph.Class = closeClass
		fenced.Close = closeGlyph
	}
	fenced.Balanced = true
	fenced.Class = ir.ClassInner
	fenced.Sizing = l.delimiterSizing(call, callName(call.base))
	return fenced
}

func matchingDelimiter(open string) string {
	pairs := map[string]string{
		"(": ")", "⟮": "⟯", "⦇": "⦈", "⦅": "⦆", "⦓": "⦔", "⦕": "⦖",
		"{": "}", "⦃": "⦄", "[": "]", "⦍": "⦐", "⦏": "⦎", "⟦": "⟧",
		"⦋": "⦌", "❲": "❳", "⟬": "⟭", "⦗": "⦘", "⟅": "⟆", "⎰": "⎱",
		"⎱": "⎰", "⧘": "⧙", "⧚": "⧛", "⟨": "⟩", "⧼": "⧽", "⦑": "⦒",
		"⦉": "⦊", "⟪": "⟫", "⌜": "⌝", "⌞": "⌟", "|": "|", "‖": "‖",
		"⦀": "⦀", "⦙": "⦙", "⦚": "⦚", "⌈": "⌉", "⌊": "⌋",
	}
	if close, ok := pairs[open]; ok {
		return close
	}
	return ""
}

func (l *lowerer) lowerLine(call *astNode, name string) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	position := ir.PositionAbove
	if name == "underline" {
		position = ir.PositionBelow
	}
	if l.exhausted {
		return nil
	}
	line := l.newLine(call.span, body, position)
	if line == nil {
		return nil
	}
	line.Thickness = builtinEmLength(0.05)
	return line
}

func (l *lowerer) lowerCancel(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	if l.exhausted {
		return nil
	}
	cancel := l.newCancel(call.span, body)
	if cancel == nil {
		return nil
	}
	cancel.Thickness = builtinEmLength(0.05)
	if arg := namedArgument(call, "cross"); arg != nil {
		if cross, ok := l.valueBool(arg.node); ok {
			cancel.Cross = cross
		}
	}
	if arg := namedArgument(call, "inverted"); arg != nil {
		if inverted, ok := l.valueBool(arg.node); ok {
			cancel.Inverted = inverted
		}
	}
	for _, name := range []string{"length", "stroke", "angle"} {
		if arg := namedArgument(call, name); arg != nil {
			l.warningAt(arg.span, source.CodeUnsupported, "cancel %s is not represented by the neutral Cancel node", name)
		}
	}
	return cancel
}

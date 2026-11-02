package typst

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (l *lowerer) lowerSqrt(call *astNode) ir.Node {
	body := l.contentArg(call, "radicand", 0, true)
	return ir.NewRadical(call.span, body, nil)
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
	return ir.NewRadical(call.span, radicand, index)
}

func (l *lowerer) lowerFracCall(call *astNode) ir.Node {
	num := l.contentArg(call, "num", 0, true)
	denom := l.contentArg(call, "denom", 1, true)
	fr := ir.NewFraction(call.span, num, denom)
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
	fr := ir.NewFraction(call.span, upper, lower)
	fr.Bar = false
	fr.Style = ir.FractionNoBar
	fr.Role = ir.FractionRoleBinomial
	fr.Padding = builtinEmLength(0.1)
	fenced := ir.NewFenced(call.span, fr)
	open := l.glyph(source.NewRange(call.span.Start, call.span.Start), "(")
	closeGlyph := l.glyph(source.NewRange(call.span.End, call.span.End), ")")
	open.Class = ir.ClassOpening
	closeGlyph.Class = ir.ClassClosing
	fenced.Open = open
	fenced.Close = closeGlyph
	fenced.Class = ir.ClassInner
	return fenced
}

func (l *lowerer) lowerAttachCall(call *astNode) ir.Node {
	base := l.contentArg(call, "base", 0, true)
	scripts := ir.NewScripts(call.span, base)
	scripts.Top = l.contentNamed(call, "t")
	scripts.Bottom = l.contentNamed(call, "b")
	scripts.TopLeft = l.contentNamed(call, "tl")
	scripts.BottomLeft = l.contentNamed(call, "bl")
	scripts.TopRight = l.contentNamed(call, "tr")
	scripts.BottomRight = l.contentNamed(call, "br")
	if base, ok := scripts.BaseItem.(*ir.Scripts); ok && base.Role != ir.ScriptRoleOrdinary {
		scripts.Role = base.Role
	}
	if scripts.Role == ir.ScriptRoleOrdinary {
		if hasLimitSemantics(scripts.BaseItem) {
			scripts.Role = ir.ScriptRoleLimits
		} else {
			scripts.Role = ir.ScriptRoleOverlay
		}
	}
	inheritScriptMetadata(scripts)
	return scripts
}

func (l *lowerer) lowerLimitsCall(call *astNode, name string) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	scripts := ir.NewScripts(call.span, body)
	if name == "scripts" {
		scripts.Limits = ir.LimitsNever
		scripts.Role = ir.ScriptRoleOrdinary
	} else {
		scripts.Limits = ir.LimitsAlways
		scripts.Role = ir.ScriptRoleLimits
		if inlineArg := namedArgument(call, "inline"); inlineArg != nil {
			if inline, ok := l.valueBool(inlineArg.node); ok && !inline {
				scripts.Limits = ir.LimitsAuto
			}
		}
	}
	inheritScriptMetadata(scripts)
	return scripts
}

func (l *lowerer) lowerLR(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	if size := namedArgument(call, "size"); size != nil && !isDefaultPercent(l, size.node, 100) {
		l.warningAt(size.span, source.CodeUnsupported, "lr size is not represented by the neutral Fenced node")
	}
	if fenced, ok := body.(*ir.Fenced); ok {
		fenced.Class = ir.ClassInner
		return fenced
	}
	if row, ok := body.(*ir.Row); ok && len(row.Items) == 1 {
		if fenced, ok := row.Items[0].(*ir.Fenced); ok {
			return fenced
		}
	}
	if row, ok := body.(*ir.Row); ok && len(row.Items) >= 2 {
		first, firstOK := delimiterNode(row.Items[0])
		last, lastOK := delimiterNode(row.Items[len(row.Items)-1])
		if firstOK && lastOK {
			fenced := ir.NewFenced(call.span, row.Items[1:len(row.Items)-1]...)
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
			return fenced
		}
	}
	if row, ok := body.(*ir.Row); ok && len(row.Items) == 1 {
		if g, ok := row.Items[0].(*ir.Glyph); ok && isFenceGlyph(g) {
			l.warningAt(call.span, source.CodeUnsupported, "a single stretchy delimiter is not represented by the neutral Fenced node")
		}
	}
	// lr() makes delimiters already present in its body stretch. If no fence is
	// present, the body itself is the complete result.
	return body
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
	if size := namedArgument(call, "size"); size != nil && !isDefaultPercent(l, size.node, 100) {
		l.warningAt(size.span, source.CodeUnsupported, "%s size is not represented by the neutral Fenced node", name)
	}
	pairs := map[string][2]string{
		"abs": {"|", "|"}, "norm": {"‖", "‖"}, "floor": {"⌊", "⌋"},
		"ceil": {"⌈", "⌉"}, "round": {"(", ")"},
	}
	pair := pairs[name]
	body := l.contentArg(call, "body", 0, true)
	fenced := ir.NewFenced(call.span, l.flattenFenceBody(body)...)
	open := l.glyph(source.NewRange(call.span.Start, call.span.Start), pair[0])
	closeGlyph := l.glyph(source.NewRange(call.span.End, call.span.End), pair[1])
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
	return fenced
}

func (l *lowerer) lowerDelimiterFunction(call *astNode, def symbolDef) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	if named := l.contentNamed(call, "body"); named != nil {
		body = named
	}
	closeText := matchingDelimiter(def.text)
	fenced := ir.NewFenced(call.span, l.flattenFenceBody(body)...)
	open := l.glyph(source.NewRange(call.span.Start, call.span.Start), def.text)
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
		closeGlyph.Class = closeClass
		fenced.Close = closeGlyph
	}
	fenced.Balanced = true
	fenced.Class = ir.ClassInner
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
	line := ir.NewLine(call.span, body, position)
	line.Thickness = builtinEmLength(0.05)
	return line
}

func (l *lowerer) lowerCancel(call *astNode) ir.Node {
	body := l.contentArg(call, "body", 0, true)
	cancel := ir.NewCancel(call.span, body)
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

package typst

import (
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (l *lowerer) lowerAccentCall(call *astNode) ir.Node {
	if size := namedArgument(call, "size"); size != nil && !isDefaultPercent(l, size.node, 100) {
		l.warningAt(size.span, source.CodeUnsupported, "accent size is not represented by the neutral Accent node")
	}
	positional := l.positionalArguments(call)
	var accentNode *astNode
	if len(positional) >= 2 {
		accentNode = positional[1].node
	}
	if named := namedArgument(call, "accent"); named != nil {
		accentNode = named.node
	}
	mark := ""
	if accentNode != nil {
		mark = l.accentText(accentNode)
	}
	if mark == "" {
		l.errorAt(call.span, source.CodeInvalidValue, "expected exactly one accent character")
		mark = "̂"
	}
	body := l.contentArg(call, "base", 0, true)
	accent := l.glyph(accentNodeSpan(accentNode, call.span), normalizeAccent(mark))
	position := ir.PositionAbove
	if accentIsBelow(normalizeAccent(mark)) {
		position = ir.PositionBelow
	}
	out := ir.NewAccent(call.span, body, accent, position)
	out.Stretchy = true
	if arg := namedArgument(call, "dotless"); arg != nil {
		if dotless, ok := l.valueBool(arg.node); ok {
			out.Dotless = dotless
		}
	}
	return out
}

func accentNodeSpan(n *astNode, fallback source.Range) source.Range {
	if n == nil || !n.span.IsValid() {
		return fallback
	}
	return n.span
}

func (l *lowerer) lowerAccentWithMark(call *astNode, mark string) ir.Node {
	body := l.contentArg(call, "base", 0, true)
	accent := l.glyph(call.span, mark)
	position := ir.PositionAbove
	if accentIsBelow(mark) {
		position = ir.PositionBelow
	}
	out := ir.NewAccent(call.span, body, accent, position)
	out.Stretchy = true
	if arg := namedArgument(call, "dotless"); arg != nil {
		if dotless, ok := l.valueBool(arg.node); ok {
			out.Dotless = dotless
		}
	}
	return out
}

func (l *lowerer) accentText(n *astNode) string {
	if n == nil {
		return ""
	}
	if n.kind == astString {
		return n.text
	}
	if n.kind == astIdentifier || n.kind == astField {
		if mark, ok := accentAliases[n.text]; ok {
			return mark
		}
		if def, ok := symbol(n.text); ok {
			return def.text
		}
		return n.text
	}
	if n.kind == astRow && len(n.items) == 1 {
		return l.accentText(n.items[0])
	}
	if n.kind == astCode && len(n.items) == 1 {
		return l.accentText(n.items[0])
	}
	if n.kind == astGlyph || n.kind == astShorthand {
		return n.text
	}
	return ""
}

func normalizeAccent(s string) string {
	names := map[string]string{
		"^": "̂", "ˆ": "̂", "~": "̃", "∼": "̃", "˜": "̃", "¯": "̄", "-": "̄",
		"−": "̄", "˘": "̆", ".": "̇", "⋅": "̇", "¨": "̈", "○": "̊", "∘": "̊",
		"←": "⃖", "→": "⃗", "↔": "⃡", "⟷": "⃡", "`": "̀", "´": "́", "ˇ": "̌",
	}
	if mark, ok := names[s]; ok {
		return mark
	}
	return s
}

func accentIsBelow(s string) bool {
	if strings.ContainsAny(s, "⏟⎵⏝⏡") {
		return true
	}
	for _, r := range s {
		if r >= '\u0316' && r <= '\u0334' {
			return true
		}
	}
	return false
}

func (l *lowerer) lowerStructuralAccent(call *astNode, name string) ir.Node {
	marks := map[string]string{
		"overbrace": "⏞", "underbrace": "⏟", "overbracket": "⎴", "underbracket": "⎵",
		"overparen": "⏜", "underparen": "⏝", "overshell": "⏠", "undershell": "⏡",
	}
	position := ir.PositionAbove
	if strings.HasPrefix(name, "under") {
		position = ir.PositionBelow
	}
	body := l.contentArg(call, "body", 0, true)
	accent := l.glyph(call.span, marks[name])
	accent.Class = ir.ClassDiacritic
	decor := ir.NewAccent(call.span, body, accent, position)
	decor.Stretchy = true
	var annotation ir.Node
	if named := l.contentNamed(call, "annotation"); named != nil {
		annotation = named
	} else {
		positional := l.positionalArguments(call)
		if len(positional) >= 2 {
			annotation = l.lower(positional[1].node)
		}
	}
	if annotation == nil {
		return decor
	}
	scripts := ir.NewScripts(call.span, decor)
	if position == ir.PositionAbove {
		scripts.Top = annotation
	} else {
		scripts.Bottom = annotation
	}
	scripts.Limits = ir.LimitsAlways
	return scripts
}

package tex

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var extensibleArrows = map[string]string{
	"\\xleftarrow":         "←",
	"\\xrightarrow":        "→",
	"\\xLeftarrow":         "⇐",
	"\\xRightarrow":        "⇒",
	"\\xleftrightarrow":    "↔",
	"\\xLeftrightarrow":    "⇔",
	"\\xhookleftarrow":     "↩",
	"\\xhookrightarrow":    "↪",
	"\\xmapsto":            "↦",
	"\\xlongequal":         "=",
	"\\xleftharpoondown":   "↽",
	"\\xleftharpoonup":     "↼",
	"\\xrightharpoondown":  "⇁",
	"\\xrightharpoonup":    "⇀",
	"\\xleftrightharpoons": "⇋",
	"\\xrightleftharpoons": "⇌",
	"\\xtofrom":            "⇄",
	"\\xtwoheadleftarrow":  "↞",
	"\\xtwoheadrightarrow": "↠",
}

func (p *parser) parseOverUnderAccent(command token, position ir.Position, mark string, stretchy bool) astNode {
	body := p.parseRequiredArgument(command)
	return &astAccentNode{
		astBase:  newASTBaseAt(astAccent, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:     body,
		mark:     p.newGlyphAST(command.span, mark, ir.ClassDiacritic),
		position: position,
		stretchy: stretchy,
	}
}

func (p *parser) parseStructuralAccent(command token, position ir.Position, mark string) astNode {
	accent := p.parseOverUnderAccent(command, position, mark, true)
	// KaTeX horizBrace.ts emits minner for braces and brackets; annotations on
	// these structures are always over/under limits.
	return &astClassNode{
		astBase:   newASTBaseAt(astClass, accent.base().span, tokenOrigin(command)),
		body:      accent,
		class:     ir.ClassInner,
		limits:    ir.LimitsAlways,
		setLimits: true,
	}
}

func (p *parser) parseStacked(command token, position ir.Position, forceRelation bool) astNode {
	annotation := p.parseRequiredArgument(command)
	body := compactAST(p.parseRequiredArgument(command))
	scripts := &astScriptsNode{
		astBase: newASTBaseAt(astScripts, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		main: &astScriptsNode{
			astBase: newASTBaseAt(astScripts, body.base().span, tokenOrigin(command)),
			main:    body,
			limits:  ir.LimitsAlways,
			role:    ir.ScriptRoleOrdinary,
		},
		limits: ir.LimitsAlways,
		role:   ir.ScriptRoleOverlay,
	}
	if position == ir.PositionAbove {
		scripts.top = annotation
	} else {
		scripts.bottom = annotation
	}
	if forceRelation {
		return &astClassNode{
			astBase: newASTBaseAt(astClass, scripts.span, tokenOrigin(command)),
			body:    scripts,
			class:   ir.ClassRelation,
		}
	}
	return scripts
}

func compactAST(node astNode) astNode {
	if row, ok := node.(*astRowNode); ok && len(row.items) == 1 {
		return row.items[0]
	}
	return node
}

func (p *parser) parseExtensibleArrow(command token) astNode {
	arrowText := extensibleArrows[command.text]
	below := p.parseOptionalArgument(command)
	above := p.parseRequiredArgument(command)
	arrow := p.newGlyphAST(command.span, arrowText, ir.ClassRelation)
	scripts := &astScriptsNode{
		astBase: newASTBaseAt(astScripts, source.Merge(command.span, above.base().span), tokenOrigin(command)),
		main:    arrow,
		limits:  ir.LimitsAlways,
		role:    ir.ScriptRoleOverlay,
	}
	if astHasContent(above) {
		scripts.top = above
	}
	if astHasContent(below) {
		scripts.bottom = below
		scripts.span = source.Merge(scripts.span, below.base().span)
	}
	scripts.span = source.Merge(command.span, scripts.span)
	scripts.origin = tokenOrigin(command)
	return scripts
}

func astHasContent(node astNode) bool {
	if node == nil {
		return false
	}
	if row, ok := node.(*astRowNode); ok {
		return len(row.items) > 0
	}
	if placeholder, ok := node.(*astPlaceholderNode); ok {
		return placeholder.reason != ""
	}
	return true
}

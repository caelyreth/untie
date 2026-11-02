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
		mark:     newGlyphAST(command.span, mark, ir.ClassDiacritic),
		position: position,
		stretchy: stretchy,
	}
}

func (p *parser) parseStacked(command token, position ir.Position) astNode {
	annotation := p.parseRequiredArgument(command)
	body := p.parseRequiredArgument(command)
	return &astAccentNode{
		astBase:    newASTBaseAt(astAccent, source.Merge(command.span, body.base().span), tokenOrigin(command)),
		body:       body,
		mark:       annotation,
		position:   position,
		exactWidth: true,
	}
}

func (p *parser) parseExtensibleArrow(command token) astNode {
	arrowText := extensibleArrows[command.text]
	below := p.parseOptionalArgument(command)
	above := p.parseRequiredArgument(command)
	arrow := newGlyphAST(command.span, arrowText, ir.ClassRelation)
	var body astNode = arrow
	if astHasContent(above) {
		body = &astAccentNode{
			astBase:    newASTBaseAt(astAccent, source.Merge(command.span, above.base().span), tokenOrigin(command)),
			body:       body,
			mark:       above,
			position:   ir.PositionAbove,
			exactWidth: true,
		}
	}
	if astHasContent(below) {
		body = &astAccentNode{
			astBase:    newASTBaseAt(astAccent, source.Merge(command.span, below.base().span), tokenOrigin(command)),
			body:       body,
			mark:       below,
			position:   ir.PositionBelow,
			exactWidth: true,
		}
	}
	body.base().span = source.Merge(command.span, body.base().span)
	body.base().origin = tokenOrigin(command)
	return body
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

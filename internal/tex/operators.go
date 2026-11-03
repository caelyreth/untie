package tex

import (
	"github.com/caelyreth/untie/ir"
)

type namedOperatorSpec struct {
	parts    []string
	limits   ir.LimitsPlacement
	hasLine  bool
	line     ir.Position
	accent   string
	accentAt ir.Position
}

var namedOperatorOverrides = map[string]namedOperatorSpec{
	// KaTeX macros.ts defines these as \operatorname* composites. The explicit
	// always-limit contract matters even in inline style.
	"\\liminf":  {parts: []string{"lim", "inf"}, limits: ir.LimitsAlways},
	"\\limsup":  {parts: []string{"lim", "sup"}, limits: ir.LimitsAlways},
	"\\argmin":  {parts: []string{"arg", "min"}, limits: ir.LimitsAlways},
	"\\argmax":  {parts: []string{"arg", "max"}, limits: ir.LimitsAlways},
	"\\injlim":  {parts: []string{"inj", "lim"}, limits: ir.LimitsAlways},
	"\\projlim": {parts: []string{"proj", "lim"}, limits: ir.LimitsAlways},
	"\\plim":    {parts: []string{"plim"}, limits: ir.LimitsAlways},
	"\\varlimsup": {
		parts: []string{"lim"}, limits: ir.LimitsAlways, hasLine: true, line: ir.PositionAbove,
	},
	"\\varliminf": {
		parts: []string{"lim"}, limits: ir.LimitsAlways, hasLine: true, line: ir.PositionBelow,
	},
	"\\varinjlim": {
		parts: []string{"lim"}, limits: ir.LimitsAlways,
		accent: "→", accentAt: ir.PositionBelow,
	},
	"\\varprojlim": {
		parts: []string{"lim"}, limits: ir.LimitsAlways,
		accent: "←", accentAt: ir.PositionBelow,
	},
}

func (p *parser) parseNamedOperator(command token) astNode {
	if spec, ok := namedOperatorOverrides[command.text]; ok {
		return p.buildNamedOperator(command, spec)
	}
	op, ok := namedOperators[command.text]
	if !ok {
		return nil
	}
	limits := ir.LimitsNever
	if limitOperators[command.text] {
		limits = ir.LimitsAuto
	}
	return &astOperatorNode{
		astBase:        newASTBaseAt(astOperator, command.span, tokenOrigin(command)),
		text:           op,
		limits:         limits,
		semanticLimits: limitOperators[command.text],
	}
}

func (p *parser) buildNamedOperator(command token, spec namedOperatorSpec) astNode {
	items := make([]astNode, 0, len(spec.parts)*2-1)
	for i, part := range spec.parts {
		if i > 0 {
			items = append(items, explicitEmSpaceAST(command.span, 1.0/6.0))
		}
		items = append(items, &astOperatorNode{
			astBase: newASTBaseAt(astOperator, command.span, tokenOrigin(command)),
			text:    part,
			limits:  ir.LimitsNever,
		})
	}
	var body astNode = &astRowNode{
		astBase: newASTBaseAt(astRow, command.span, tokenOrigin(command)),
		items:   items,
	}
	if spec.hasLine || spec.accent != "" {
		position := spec.line
		if spec.accent != "" {
			position = spec.accentAt
		}
		if spec.accent != "" {
			body = &astAccentNode{
				astBase:  newASTBaseAt(astAccent, command.span, tokenOrigin(command)),
				body:     body,
				mark:     p.newGlyphAST(command.span, spec.accent, ir.ClassDiacritic),
				position: position,
				stretchy: true,
			}
		} else {
			body = &astLineNode{
				astBase:  newASTBaseAt(astLine, command.span, tokenOrigin(command)),
				body:     body,
				position: position,
			}
		}
	}
	return &astClassNode{
		astBase:   newASTBaseAt(astClass, command.span, tokenOrigin(command)),
		body:      body,
		class:     ir.ClassLarge,
		limits:    spec.limits,
		setLimits: true,
	}
}

package typst

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type lowerer struct {
	src        string
	limits     source.Limits
	diags      []source.Diagnostic
	nodes      int
	depth      int
	callDepth  int
	tableLimit bool
}

func lower(root *astNode, src string, limits source.Limits) Result {
	l := lowerer{src: src, limits: limits.WithDefaults()}
	node := l.lower(root)
	if node == nil {
		node = ir.NewRow(source.NewRange(0, len(src)))
	}
	return Result{
		Root:        node,
		Diagnostics: source.NormalizeDiagnostics(l.diags),
	}
}

func (l *lowerer) lower(n *astNode) ir.Node {
	if n == nil {
		return l.placeholder(source.Detached(), "missing expression")
	}
	if !n.span.IsValid() {
		n.span = source.Detached()
	}
	if !l.enter(n.span) {
		return l.errNode(n.span, "nesting limit exceeded", source.CodeLimitExceeded)
	}
	defer l.leave()
	l.nodes++
	if l.nodes > l.limits.MaxNodes {
		if l.nodes == l.limits.MaxNodes+1 {
			l.errorAt(n.span, source.CodeLimitExceeded, "node limit exceeded")
		}
		return l.errNode(n.span, "node limit exceeded", source.CodeLimitExceeded)
	}

	switch n.kind {
	case astRow:
		return l.lowerRow(n)
	case astGlyph:
		if n.raw == "primes" {
			return l.primeRow(n.span, utf8.RuneCountInString(n.text))
		}
		return l.glyph(n.span, n.text)
	case astIdentifier, astField:
		return l.lowerIdentifier(n)
	case astString:
		return l.text(n.span, n.text)
	case astNumber:
		return l.lowerNumber(n)
	case astShorthand:
		return l.glyph(n.span, n.text)
	case astAttach:
		return l.lowerAttach(n)
	case astFraction:
		return l.lowerFraction(n)
	case astRoot:
		return l.lowerRoot(n)
	case astDelimited:
		return l.lowerDelimited(n)
	case astCall:
		return l.lowerCall(n)
	case astCode:
		if len(n.items) == 0 {
			return l.errNode(n.span, "#", source.CodeMissingArgument)
		}
		inner := n.items[0]
		if (inner.kind == astIdentifier || inner.kind == astField) && !strings.Contains(inner.text, ".") {
			if v, ok := namedColor(inner.text); ok {
				return l.lowerValueContent(inner.span, v)
			}
		}
		return l.lower(inner)
	case astArray, astDict, astLiteral, astUnary:
		v, ok := l.evalValue(n)
		if !ok {
			return l.errNode(n.span, l.sourceText(n.span), source.CodeInvalidValue)
		}
		return l.lowerValueContent(n.span, v)
	case astAlign:
		marker := ir.NewAlignMarker(n.span)
		marker.Class = ir.ClassSpecial
		return marker
	case astLinebreak:
		return ir.NewBreak(n.span, ir.BreakExplicit, 0)
	case astEquation:
		if len(n.items) == 0 {
			return ir.NewRow(n.span)
		}
		return l.lower(n.items[0])
	case astError:
		text := n.raw
		if text == "" {
			text = l.sourceText(n.span)
		}
		if text == "" {
			text = "?"
		}
		return l.errNode(n.span, text, source.CodeUnsupported)
	default:
		text := l.sourceText(n.span)
		if text == "" {
			text = "unsupported expression"
		}
		l.errorAt(n.span, source.CodeUnsupported, "unsupported Typst expression")
		return l.errNode(n.span, text, source.CodeUnsupported)
	}
}

func (l *lowerer) enter(span source.Range) bool {
	if l.depth >= l.limits.MaxNesting {
		l.errorAt(span, source.CodeLimitExceeded, "nesting limit exceeded")
		return false
	}
	l.depth++
	return true
}

func (l *lowerer) leave() {
	l.depth--
}

func (l *lowerer) errorAt(span source.Range, code source.Code, format string, args ...any) {
	if span.IsDetached() {
		span = source.NewRange(0, 0)
	}
	l.diags = append(l.diags, source.Diagnostic{
		Range:    span,
		Severity: source.SeverityError,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (l *lowerer) tableLimitError(span source.Range) {
	if l.tableLimit {
		return
	}
	l.tableLimit = true
	l.errorAt(span, source.CodeLimitExceeded, "table cell limit exceeded")
}

func (l *lowerer) warningAt(span source.Range, code source.Code, format string, args ...any) {
	if span.IsDetached() {
		span = source.NewRange(0, 0)
	}
	l.diags = append(l.diags, source.Diagnostic{
		Range:    span,
		Severity: source.SeverityWarning,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (l *lowerer) sourceText(span source.Range) string {
	if span.IsDetached() || !span.IsValid() || span.End > len(l.src) {
		return ""
	}
	return l.src[span.Start:span.End]
}

func (l *lowerer) errNode(span source.Range, text string, code source.Code) *ir.ErrorNode {
	text = sanitizeFrontendText(text)
	if text == "" {
		text = "?"
	}
	return ir.NewError(span, text, code)
}

func (l *lowerer) placeholder(span source.Range, reason string) *ir.Placeholder {
	return ir.NewPlaceholder(span, sanitizeFrontendText(reason))
}

func sanitizeFrontendText(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	return strings.ToValidUTF8(text, "\ufffd")
}

func (l *lowerer) glyph(span source.Range, text string) *ir.Glyph {
	text = sanitizeFrontendText(text)
	g := ir.NewGlyph(span, text)
	g.Class = classForText(text)
	return g
}

func (l *lowerer) lowerNumber(n *astNode) ir.Node {
	if n == nil || n.text == "" {
		return l.glyph(n.span, n.text)
	}
	runes := []rune(n.text)
	if len(runes) == 1 {
		return l.glyph(n.span, n.text)
	}
	row := ir.NewRow(n.span)
	offset := n.span.Start
	for _, r := range runes {
		size := utf8.RuneLen(r)
		span := source.NewRange(offset, offset+size)
		l.nodes++
		if l.nodes > l.limits.MaxNodes {
			if l.nodes == l.limits.MaxNodes+1 {
				l.errorAt(span, source.CodeLimitExceeded, "node limit exceeded")
			}
			return l.errNode(n.span, "node limit exceeded", source.CodeLimitExceeded)
		}
		g := l.glyph(span, string(r))
		g.Class = ir.ClassNormal
		row.Items = append(row.Items, g)
		offset += size
	}
	return row
}

func (l *lowerer) text(span source.Range, text string) ir.Node {
	text = sanitizeFrontendText(text)
	if text == "" {
		return l.placeholder(span, "empty text")
	}
	if len(text) > l.limits.MaxTextBytes {
		l.errorAt(span, source.CodeLimitExceeded, "text length limit exceeded")
		text = text[:l.limits.MaxTextBytes]
		for !utf8.ValidString(text) && len(text) > 0 {
			text = text[:len(text)-1]
		}
		if text == "" {
			return l.placeholder(span, "text too long")
		}
	}
	return ir.NewText(span, text)
}

func (l *lowerer) lowerRow(n *astNode) ir.Node {
	hasLinebreak := false
	hasAlign := false
	for _, item := range n.items {
		switch item.kind {
		case astLinebreak:
			hasLinebreak = true
		case astAlign:
			hasAlign = true
		}
	}
	if !hasLinebreak && !hasAlign {
		row := ir.NewRow(n.span)
		row.Items = l.lowerItems(n.items)
		return row
	}

	rows := make([][]ir.Node, 0, 4)
	current := make([]*astNode, 0, 4)
	flush := func() {
		rows = append(rows, l.lowerMultilineRow(current, n.span))
		current = make([]*astNode, 0, 4)
	}
	for _, item := range n.items {
		if item.kind == astLinebreak {
			flush()
			continue
		}
		current = append(current, item)
	}
	// Typst ignores a trailing linebreak in a multiline sequence.
	if len(current) > 0 {
		flush()
	}
	if len(rows) == 0 {
		return ir.NewRow(n.span)
	}
	if len(rows) == 1 && !hasAlign {
		if len(rows[0]) == 1 {
			return rows[0][0]
		}
		row := ir.NewRow(n.span)
		row.Items = rows[0]
		return row
	}
	multiline := ir.NewMultiline(n.span, rows...)
	multiline.Centered = true
	return multiline
}

func (l *lowerer) lowerMultilineRow(items []*astNode, fallback source.Range) []ir.Node {
	parts := make([]ir.Node, 0, 3)
	segment := make([]*astNode, 0, len(items))
	flush := func() {
		start, end := fallback.Start, fallback.Start
		if len(segment) > 0 {
			start = segment[0].span.Start
			end = segment[len(segment)-1].span.End
		}
		row := ir.NewRow(source.NewRange(start, end))
		row.Items = l.lowerItems(segment)
		parts = append(parts, row)
		segment = make([]*astNode, 0, len(items))
	}
	for _, item := range items {
		if item.kind == astAlign {
			flush()
			parts = append(parts, l.lower(item))
			continue
		}
		segment = append(segment, item)
	}
	flush()
	return parts
}

func (l *lowerer) lowerItems(items []*astNode) []ir.Node {
	out := make([]ir.Node, 0, len(items))
	for _, item := range items {
		if item == nil || item.kind == astLinebreak {
			continue
		}
		out = append(out, l.lower(item))
	}
	return out
}

func (l *lowerer) lowerIdentifier(n *astNode) ir.Node {
	resolved := resolveIdent(n)
	if resolved.unknownRoot != "" {
		return l.unknownVariable(resolved.rootSpan, resolved.unknownRoot)
	}
	name := resolved.name
	if name == "zws" {
		return ir.NewRow(n.span)
	}
	if name == "dif" || name == "Dif" {
		return l.lowerDifferential(n.span, strings.ToUpper(name[:1]))
	}
	if hasCombiningMark(name) {
		return l.lowerCombinedIdentifier(n)
	}
	if amount, ok := spacingSymbols[name]; ok {
		space := ir.NewSpace(n.span, ir.SpaceExplicit, amount)
		space.Class = ir.ClassSpace
		return space
	}
	if def, ok := symbol(name); ok {
		g := l.glyph(n.span, def.text)
		g.Class = def.class
		if def.class == ir.ClassLarge {
			g.Limits = limitsForSymbol(name, def.class)
		}
		return g
	}
	if textOperators[name] {
		op := ir.NewOperator(n.span, name)
		op.Class = ir.ClassLarge
		op.Variant = ir.VariantNormal
		op.Limits = ir.LimitsAuto
		if name != "lim" && name != "liminf" && name != "limsup" {
			op.Limits = ir.LimitsNever
		}
		return op
	}
	if n.inCode {
		if kind, ok := codeKeywordValue(name); ok {
			return l.lowerValueContent(n.span, kind)
		}
	}
	if resolved.module != "" {
		l.errorAt(resolved.fieldSpan, source.CodeUnknownVariable,
			"module `%s` does not contain `%s`", resolved.module, name)
		return l.errNode(n.span, name, source.CodeUnknownVariable)
	}
	if !n.inCode && utf8.RuneCountInString(name) == 1 {
		return l.glyph(n.span, name)
	}
	if strings.Contains(name, ".") {
		return l.unknownSymbolField(n, name)
	}
	return l.unknownVariable(n.span, name)
}

func (l *lowerer) lowerDifferential(span source.Range, letter string) ir.Node {
	space := ir.NewSpace(span, ir.SpaceExplicit, builtinMuLength(3))
	space.Class = ir.ClassSpace
	space.Weak = true
	d := l.glyph(span, letter)
	d.Class = ir.ClassUnary
	d.Variant = ir.VariantNormal
	return ir.NewRow(span, space, d)
}

func (l *lowerer) unknownVariable(span source.Range, name string) *ir.ErrorNode {
	if span.IsDetached() || !span.IsValid() {
		span = source.NewRange(0, 0)
	}
	l.diags = append(l.diags, source.Diagnostic{
		Range:    span,
		Severity: source.SeverityError,
		Code:     source.CodeUnknownVariable,
		Message:  "unknown variable: " + name,
		Hints:    []string{"if you meant to display this as text, put it in quotes"},
	})
	return l.errNode(span, name, source.CodeUnknownVariable)
}

func (l *lowerer) unknownSymbolField(n *astNode, name string) *ir.ErrorNode {
	root := name
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		root = name[:dot]
	}
	if _, ok := symbol(root); !ok {
		return l.unknownVariable(resolveIdent(n).rootSpan, root)
	}
	field := name
	fieldSpan := n.span
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 && dot+1 < len(name) {
		field = name[dot+1:]
		fieldSpan = source.NewRange(n.span.Start+dot+1, n.span.End)
	}
	l.errorAt(fieldSpan, source.CodeUnknownVariable,
		"symbol `%s` does not contain `%s`", root, field)
	return l.errNode(n.span, name, source.CodeUnknownVariable)
}

func codeKeywordValue(name string) (value, bool) {
	switch name {
	case "none":
		return noneValue(), true
	case "auto":
		return autoValue(), true
	case "true":
		return boolValue(true), true
	case "false":
		return boolValue(false), true
	default:
		return value{}, false
	}
}

func (l *lowerer) lowerAttach(n *astNode) ir.Node {
	baseNode := l.lower(n.base)
	scripts := ir.NewScripts(n.span, baseNode)
	if base, ok := baseNode.(*ir.Scripts); ok && base.Top == nil && base.Bottom == nil &&
		base.TopLeft == nil && base.BottomLeft == nil && base.TopRight == nil && base.BottomRight == nil {
		scripts.BaseItem = base.BaseItem
		scripts.Limits = base.Limits
		scripts.Role = base.Role
	}
	scripts.Top = l.lowerOptional(n.top)
	scripts.Bottom = l.lowerOptional(n.bottom)
	scripts.TopLeft = l.lowerOptional(n.topLeft)
	scripts.BottomLeft = l.lowerOptional(n.bottomLeft)
	scripts.TopRight = l.lowerOptional(n.topRight)
	scripts.BottomRight = l.lowerOptional(n.bottomRight)
	if scripts.Role == ir.ScriptRoleOrdinary {
		scripts.Role = attachmentScriptRole(scripts.BaseItem, scripts.Top != nil || scripts.Bottom != nil)
	}
	inheritScriptMetadata(scripts)
	return scripts
}

func hasLimitSemantics(node ir.Node) bool {
	if node == nil || node.NodeBase() == nil {
		return false
	}
	base := node.NodeBase()
	if base.Class != ir.ClassLarge {
		return false
	}
	if base.Limits != ir.LimitsNever {
		return true
	}
	return containsIntegralText(nodeText(node))
}

func containsIntegralText(text string) bool {
	for _, r := range text {
		switch r {
		case '∫', '∬', '∭', '∮', '∯', '∰':
			return true
		}
	}
	return false
}

func inheritScriptMetadata(scripts *ir.Scripts) {
	inheritScriptClass(scripts)
	inheritScriptLimits(scripts)
}

func inheritScriptClass(scripts *ir.Scripts) {
	if scripts == nil || scripts.BaseItem == nil {
		return
	}
	base := scripts.BaseItem.NodeBase()
	scripts.Class = base.Class
}

func inheritScriptLimits(scripts *ir.Scripts) {
	if scripts == nil || scripts.BaseItem == nil {
		return
	}
	base := scripts.BaseItem.NodeBase()
	if scripts.Limits == ir.LimitsAuto {
		scripts.Limits = effectiveNodeLimits(scripts.BaseItem)
	}
	if scripts.Limits == ir.LimitsAuto && base.Class == ir.ClassRelation {
		scripts.Limits = ir.LimitsAlways
		if scripts.Top != nil || scripts.Bottom != nil {
			base.Limits = ir.LimitsAlways
		}
	}
}

func effectiveNodeLimits(node ir.Node) ir.LimitsPlacement {
	if node == nil || node.NodeBase() == nil {
		return ir.LimitsAuto
	}
	base := node.NodeBase()
	if base.Limits != ir.LimitsAuto {
		return base.Limits
	}
	if scripts, ok := node.(*ir.Scripts); ok {
		return effectiveNodeLimits(scripts.BaseItem)
	}
	return ir.LimitsAuto
}

func (l *lowerer) lowerOptional(n *astNode) ir.Node {
	if n == nil {
		return nil
	}
	if isNoneAST(n) {
		return nil
	}
	return l.lower(n)
}

func (l *lowerer) lowerFraction(n *astNode) ir.Node {
	if len(n.items) != 2 {
		return l.errNode(n.span, l.sourceText(n.span), source.CodeInternal)
	}
	fr := ir.NewFraction(n.span, l.lower(n.items[0]), l.lower(n.items[1]))
	fr.Padding = builtinEmLength(0.1)
	return fr
}

func (l *lowerer) lowerRoot(n *astNode) ir.Node {
	if len(n.items) == 0 {
		return l.errNode(n.span, l.sourceText(n.span), source.CodeMissingArgument)
	}
	var index ir.Node
	if len(n.items) > 1 {
		index = l.lower(n.items[1])
	}
	return ir.NewRadical(n.span, l.lower(n.items[0]), index)
}

func (l *lowerer) lowerDelimited(n *astNode) ir.Node {
	if len(n.items) == 0 {
		return ir.NewRow(n.span)
	}
	if n.open == "" && n.close == "" && n.raw == "content" {
		return l.lowerTextBody(n.items[0])
	}
	body := l.lower(n.items[0])
	if n.open == "" && n.close == "" {
		return body
	}
	if n.open == "{" && n.close == "}" {
		return body
	}
	items := make([]ir.Node, 0, len(l.flattenFenceBody(body))+2)
	open := l.glyph(source.NewRange(n.span.Start, n.span.Start+len(n.open)), n.open)
	open.Class = ir.ClassOpening
	items = append(items, open)
	items = append(items, l.flattenFenceBody(body)...)
	if n.close != "" {
		end := n.span.End
		closeGlyph := l.glyph(source.NewRange(max(0, end-len(n.close)), end), n.close)
		closeGlyph.Class = ir.ClassClosing
		items = append(items, closeGlyph)
	}
	row := ir.NewRow(n.span, items...)
	return row
}

func (l *lowerer) flattenFenceBody(body ir.Node) []ir.Node {
	if row, ok := body.(*ir.Row); ok {
		return append([]ir.Node(nil), row.Items...)
	}
	return []ir.Node{body}
}

func (l *lowerer) lowerCall(call *astNode) ir.Node {
	if call.base == nil {
		return l.errNode(call.span, l.sourceText(call.span), source.CodeInternal)
	}
	resolved := resolveIdent(call.base)
	if resolved.unknownRoot != "" {
		return l.unknownVariable(resolved.rootSpan, resolved.unknownRoot)
	}
	name := resolved.name
	if name == "" {
		return l.errNode(call.span, l.sourceText(call.span), source.CodeUnsupported)
	}
	if l.callDepth >= l.limits.MaxCallDepth {
		l.errorAt(call.span, source.CodeLimitExceeded, "maximum function call depth exceeded")
		return l.errNode(call.span, l.sourceText(call.span), source.CodeLimitExceeded)
	}
	l.callDepth++
	defer func() { l.callDepth-- }()
	l.validateSignature(call, name)

	switch name {
	case "sqrt":
		return l.lowerSqrt(call)
	case "root":
		return l.lowerRootCall(call)
	case "frac":
		return l.lowerFracCall(call)
	case "binom":
		return l.lowerBinom(call)
	case "attach":
		return l.lowerAttachCall(call)
	case "scripts", "limits":
		return l.lowerLimitsCall(call, name)
	case "lr":
		return l.lowerLR(call)
	case "mid":
		return l.lowerMid(call)
	case "abs", "norm", "floor", "ceil", "round":
		return l.lowerFenceFunction(call, name)
	case "mat":
		return l.lowerMatrix(call, "mat")
	case "vec":
		return l.lowerMatrix(call, "vec")
	case "cases":
		return l.lowerMatrix(call, "cases")
	case "accent":
		return l.lowerAccentCall(call)
	case "overline", "underline":
		return l.lowerLine(call, name)
	case "cancel":
		return l.lowerCancel(call)
	case "h":
		return l.lowerHSpace(call)
	case "text":
		return l.lowerText(call)
	case "strong", "emph":
		return l.lowerContentEmphasis(call, name)
	case "link":
		return l.lowerLink(call)
	case "box":
		return l.lowerBox(call)
	case "hide":
		return l.lowerHide(call)
	case "display", "inline", "script", "sscript":
		return l.lowerStyle(call, name)
	case "bold", "italic", "upright", "serif", "sans", "mono", "bb", "cal", "scr", "frak":
		return l.lowerAlphabet(call, name)
	case "class":
		return l.lowerClass(call)
	case "op":
		return l.lowerOperator(call)
	case "stretch":
		return l.lowerStretch(call)
	case "primes":
		return l.lowerPrimes(call)
	case "overbrace", "underbrace", "overbracket", "underbracket", "overparen", "underparen", "overshell", "undershell":
		return l.lowerStructuralAccent(call, name)
	case "equation":
		return l.lowerEquation(call)
	}
	if mark, ok := accentAliases[name]; ok {
		return l.lowerAccentWithMark(call, mark)
	}
	if def, ok := symbol(name); ok {
		if def.class == ir.ClassOpening || def.class == ir.ClassFence {
			return l.lowerDelimiterFunction(call, def)
		}
		if def.class == ir.ClassDiacritic {
			return l.lowerAccentWithMark(call, def.text)
		}
		return l.lowerContentCall(call)
	}
	if call.base.inCode && (name == "rgb" || name == "luma") {
		if v, ok := l.evalValue(call); ok {
			return l.lowerValueContent(call.span, v)
		}
	}
	if !call.base.inCode && call.base.kind == astIdentifier && utf8.RuneCountInString(name) == 1 {
		return l.lowerContentCall(call)
	}
	if resolved.module != "" {
		l.errorAt(resolved.fieldSpan, source.CodeUnknownVariable,
			"module `%s` does not contain `%s`", resolved.module, name)
		return l.errNode(call.span, l.sourceText(call.span), source.CodeUnknownVariable)
	}
	if strings.Contains(name, ".") {
		return l.unknownSymbolField(call.base, name)
	}
	l.diags = append(l.diags, source.Diagnostic{
		Range:    call.base.span,
		Severity: source.SeverityError,
		Code:     source.CodeUnknownVariable,
		Message:  "unknown variable: " + name,
		Hints:    []string{"this function is not available in Untie's Typst math subset"},
	})
	return l.errNode(call.span, l.sourceText(call.span), source.CodeUnknownVariable)
}

// lowerContentCall handles Typst's rule that calling non-function content
// displays the callee followed by its parenthesized argument list.
func (l *lowerer) lowerContentCall(call *astNode) ir.Node {
	base := l.lower(call.base)
	body := make([]ir.Node, 0, len(call.args)*3)
	for i, arg := range call.args {
		if i > 0 {
			separator := ","
			if call.args[i-1].semicolonAfter {
				separator = ";"
			}
			sep := l.glyph(source.NewRange(arg.span.Start, arg.span.Start), separator)
			sep.Class = ir.ClassPunctuation
			body = append(body, sep)
		}
		switch arg.kind {
		case argNamed:
			l.errorAt(arg.span, source.CodeSyntax,
				"named-argument syntax can only be used with functions")
			name := l.glyph(source.NewRange(arg.span.Start, arg.span.Start), arg.name)
			colon := l.glyph(source.NewRange(arg.span.Start, arg.span.Start), ":")
			colon.Class = ir.ClassRelation
			body = append(body, name, colon)
			if arg.node != nil {
				body = append(body, l.lowerArgument(arg.node))
			}
		case argSpread:
			l.errorAt(arg.span, source.CodeSyntax,
				"spread-argument syntax can only be used with functions")
			dots := l.glyph(source.NewRange(arg.span.Start, arg.span.Start), "..")
			body = append(body, dots)
			if arg.node != nil {
				body = append(body, l.lowerArgument(arg.node))
			}
		default:
			if arg.node != nil {
				body = append(body, l.lowerArgument(arg.node))
			}
		}
	}
	fenced := ir.NewFenced(call.span, body...)
	open := l.glyph(source.NewRange(call.span.Start, call.span.Start), "(")
	closeGlyph := l.glyph(source.NewRange(call.span.End, call.span.End), ")")
	open.Class = ir.ClassOpening
	closeGlyph.Class = ir.ClassClosing
	fenced.Open = open
	fenced.Close = closeGlyph
	fenced.Balanced = true
	fenced.Class = ir.ClassInner
	row := ir.NewRow(call.span, base, fenced)
	return row
}

func callName(n *astNode) string {
	if n == nil {
		return ""
	}
	switch n.kind {
	case astIdentifier, astField:
		return resolveIdent(n).name
	default:
		return ""
	}
}

func (l *lowerer) contentArg(call *astNode, name string, index int, required bool) ir.Node {
	if named := l.contentNamed(call, name); named != nil {
		return named
	}
	positional := l.positionalArguments(call)
	if index < len(positional) && positional[index].node != nil {
		return l.lowerArgument(positional[index].node)
	}
	if required {
		l.errorAt(call.span, source.CodeMissingArgument, "missing argument: %s", name)
		return l.placeholder(call.span, "missing "+name)
	}
	return nil
}

func (l *lowerer) contentNamed(call *astNode, name string) ir.Node {
	arg := namedArgument(call, name)
	if arg == nil || isNoneAST(arg.node) {
		return nil
	}
	return l.lowerArgument(arg.node)
}

func (l *lowerer) lowerArgument(n *astNode) ir.Node {
	if n == nil {
		return l.placeholder(source.Detached(), "missing argument")
	}
	if n.kind == astRow && len(n.items) == 1 {
		return l.lower(n.items[0])
	}
	if n.kind == astCode && len(n.items) == 1 {
		inner := n.items[0]
		if inner.kind == astLiteral {
			return l.lowerValueContent(n.span, inner.value)
		}
		return l.lower(inner)
	}
	return l.lower(n)
}

func callPositional(call *astNode) []astArg {
	out := make([]astArg, 0, len(call.args))
	for _, arg := range call.args {
		if arg.kind == argPositional {
			out = append(out, arg)
		}
	}
	return out
}

func (l *lowerer) positionalArguments(call *astNode) []astArg {
	for _, arg := range call.args {
		if arg.kind == argSpread {
			out, _ := l.expandPositional(call)
			return out
		}
	}
	return callPositional(call)
}

func (l *lowerer) delimiterSizing(call *astNode, name string) ir.DelimiterSizing {
	arg := namedArgument(call, "size")
	if arg == nil {
		return ir.ContentDelimiterSizing()
	}
	length, ok := l.valueLength(arg.node)
	if !ok || length.Value < 0 {
		l.errorAt(arg.span, source.CodeInvalidValue, "%s size expects a non-negative relative length, found %s",
			name, l.valueType(arg.node))
		return ir.ContentDelimiterSizing()
	}
	if length.Unit == ir.LengthPercent {
		if math.Abs(length.Float()-100) < 1e-9 {
			return ir.ContentDelimiterSizing()
		}
		return ir.RelativeDelimiterSizing(length)
	}
	return ir.AbsoluteDelimiterSizing(length)
}

func isDefaultPercent(l *lowerer, n *astNode, want float64) bool {
	length, ok := l.valueLength(n)
	if !ok {
		return false
	}
	return length.Unit == ir.LengthPercent && math.Abs(length.Float()-want) < 1e-9
}

func namedArgument(call *astNode, name string) *astArg {
	for i := range call.args {
		if call.args[i].kind == argNamed && call.args[i].name == name {
			return &call.args[i]
		}
	}
	return nil
}

func isNoneAST(n *astNode) bool {
	if n == nil {
		return true
	}
	switch n.kind {
	case astIdentifier:
		return n.text == "none"
	case astLiteral:
		return n.value.kind == valueNone
	case astCode:
		return len(n.items) == 1 && isNoneAST(n.items[0])
	case astRow:
		return len(n.items) == 1 && isNoneAST(n.items[0])
	default:
		return false
	}
}

func lowerRowNode(args []astArg) *astNode {
	items := make([]*astNode, 0, len(args))
	r := source.Detached()
	for _, arg := range args {
		if arg.node != nil {
			items = append(items, arg.node)
			r = source.Merge(r, arg.span)
		}
	}
	row := newAST(astRow, r)
	row.items = items
	return row
}

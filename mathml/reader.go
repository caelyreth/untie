package mathml

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func unmarshal(ctx context.Context, data []byte, options Options) (Result, error) {
	if ctx == nil {
		return Result{}, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	options = options.normalized()
	if len(data) > options.Limits.MaxSourceBytes {
		result := Result{Diagnostics: []source.Diagnostic{diagnostic(
			inputSpan(data),
			source.CodeLimitExceeded,
			fmt.Errorf("%w: input exceeds %d bytes", ErrLimitExceeded, options.Limits.MaxSourceBytes),
		)}}
		return result, fmt.Errorf("%w: input exceeds %d bytes", ErrLimitExceeded, options.Limits.MaxSourceBytes)
	}

	root, err := parseXML(ctx, data, options.Limits)
	if err != nil {
		result := Result{Diagnostics: []source.Diagnostic{diagnostic(inputSpan(data), codeForError(err), err)}}
		if ctxErr := ctx.Err(); ctxErr != nil {
			result.Diagnostics = []source.Diagnostic{diagnostic(inputSpan(data), source.CodeCanceled, ctxErr)}
			return result, ctxErr
		}
		return result, err
	}
	result, err := lowerDocument(ctx, root, options.Limits)
	if err != nil {
		span := inputSpan(data)
		if root != nil && root.span.IsValid() {
			span = root.span
		}
		var elementErr *elementError
		if errors.As(err, &elementErr) {
			span = elementErr.span
		}
		result.Diagnostics = []source.Diagnostic{diagnostic(span, codeForError(err), err)}
		if ctxErr := ctx.Err(); ctxErr != nil {
			result.Diagnostics = []source.Diagnostic{diagnostic(span, source.CodeCanceled, ctxErr)}
			return result, ctxErr
		}
		return result, err
	}
	result.Diagnostics = source.NormalizeDiagnostics(result.Diagnostics)
	return result, nil
}

func inputSpan(data []byte) source.Range {
	if len(data) == 0 {
		return source.Detached()
	}
	return source.NewRange(0, len(data))
}

func codeForError(err error) source.Code {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return source.CodeCanceled
	case errors.Is(err, ErrLimitExceeded):
		return source.CodeLimitExceeded
	case errors.Is(err, ErrUnsupported):
		return source.CodeUnsupported
	case errors.Is(err, ErrInvalidMathML):
		return source.CodeSyntax
	default:
		return source.CodeInternal
	}
}

type elementError struct {
	err  error
	span source.Range
}

func (e *elementError) Error() string { return e.err.Error() }
func (e *elementError) Unwrap() error { return e.err }

func atElement(err error, e *xmlElement) error {
	if err == nil || e == nil {
		return err
	}
	return &elementError{err: err, span: e.span}
}

type xmlElement struct {
	name     xml.Name
	attrs    []xml.Attr
	children []*xmlElement
	text     string
	span     source.Range
}

type xmlParser struct {
	ctx    context.Context
	limits source.Limits
	root   *xmlElement
	stack  []*xmlElement
	nodes  int
	tokens int
	text   int
}

func parseXML(ctx context.Context, data []byte, limits source.Limits) (*xmlElement, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	decoder.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		return nil, fmt.Errorf("%w: unsupported XML encoding %q", ErrUnsupported, label)
	}
	state := &xmlParser{ctx: ctx, limits: limits}
	sawRoot := false
	sawDeclaration := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		before := decoder.InputOffset()
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var syntax *xml.SyntaxError
			if errors.As(err, &syntax) {
				return nil, fmt.Errorf("%w: line %d: %s", ErrInvalidMathML, syntax.Line, syntax.Msg)
			}
			return nil, fmt.Errorf("%w: %v", ErrInvalidMathML, err)
		}
		after := decoder.InputOffset()
		state.tokens++
		if state.tokens > limits.MaxTokens {
			return nil, fmt.Errorf("%w: token limit exceeded", ErrLimitExceeded)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if sawRoot && len(state.stack) == 0 {
				return nil, fmt.Errorf("%w: multiple root elements", ErrInvalidMathML)
			}
			state.nodes++
			state.tokens += len(token.Attr)
			if state.nodes > limits.MaxNodes || state.tokens > limits.MaxTokens {
				return nil, fmt.Errorf("%w: element limit exceeded", ErrLimitExceeded)
			}
			if len(state.stack)+1 > limits.MaxNesting {
				return nil, fmt.Errorf("%w: nesting limit exceeded", ErrLimitExceeded)
			}
			if err := validateUniqueAttributes(token.Attr); err != nil {
				return nil, err
			}
			element := &xmlElement{
				name:  token.Name,
				attrs: append([]xml.Attr(nil), token.Attr...),
				span:  source.NewRange(int(before), int(after)),
			}
			if len(state.stack) == 0 {
				state.root = element
				sawRoot = true
			} else {
				parent := state.stack[len(state.stack)-1]
				parent.children = append(parent.children, element)
			}
			state.stack = append(state.stack, element)
		case xml.EndElement:
			if len(state.stack) == 0 {
				return nil, fmt.Errorf("%w: unexpected closing element", ErrInvalidMathML)
			}
			current := state.stack[len(state.stack)-1]
			if current.name != token.Name {
				return nil, fmt.Errorf("%w: mismatched closing element", ErrInvalidMathML)
			}
			current.span.End = int(after)
			state.stack = state.stack[:len(state.stack)-1]
		case xml.CharData:
			if len(state.stack) == 0 {
				if strings.TrimSpace(string(token)) != "" {
					return nil, fmt.Errorf("%w: text outside root element", ErrInvalidMathML)
				}
				continue
			}
			state.text += len(token)
			if state.text > limits.MaxTextBytes {
				return nil, fmt.Errorf("%w: text limit exceeded", ErrLimitExceeded)
			}
			state.stack[len(state.stack)-1].text += string(token)
		case xml.Comment:
			// Comments have no canonical semantics.
		case xml.ProcInst:
			if token.Target == "xml" && !sawDeclaration && state.tokens == 1 {
				sawDeclaration = true
				continue
			}
			return nil, fmt.Errorf("%w: processing instruction %q", ErrUnsupported, token.Target)
		case xml.Directive:
			return nil, fmt.Errorf("%w: XML directives are not accepted", ErrUnsupported)
		}
	}
	if len(state.stack) != 0 {
		return nil, fmt.Errorf("%w: unclosed element", ErrInvalidMathML)
	}
	if state.root == nil {
		return nil, fmt.Errorf("%w: empty document", ErrInvalidMathML)
	}
	return state.root, nil
}

func validateUniqueAttributes(attrs []xml.Attr) error {
	seen := make(map[xml.Name]struct{}, len(attrs))
	for _, attr := range attrs {
		if _, ok := seen[attr.Name]; ok {
			return fmt.Errorf("%w: duplicate attribute %q", ErrInvalidMathML, attr.Name.Local)
		}
		seen[attr.Name] = struct{}{}
	}
	return nil
}

func lowerDocument(ctx context.Context, root *xmlElement, limits source.Limits) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if root.name.Space != Namespace || root.name.Local != "math" {
		return Result{}, atElement(fmt.Errorf("%w: root must be MathML math", ErrInvalidMathML), root)
	}
	view := newAttrView(root)
	mode := ir.Inline
	if display, ok := view.take("display"); ok {
		switch display {
		case "block":
			mode = ir.Display
		case "inline":
			mode = ir.Inline
		default:
			return Result{}, atElement(fmt.Errorf("%w: invalid display %q", ErrInvalidMathML, display), root)
		}
	}
	if version, ok := view.take(attrVersion); ok && version != strconv.Itoa(SchemaVersion) {
		return Result{}, atElement(fmt.Errorf("%w: metadata version %q", ErrUnsupported, version), root)
	}
	view.ignore("alttext")
	view.ignore(attrAltText)
	view.ignoreNonSemantic()
	suppressed, err := view.optionalNonNegativeInt(attrSuppressed)
	if err != nil {
		return Result{}, atElement(err, root)
	}
	if suppressed > limits.MaxNodes {
		return Result{}, atElement(fmt.Errorf("%w: suppressed tag count exceeds node limit", ErrLimitExceeded), root)
	}
	if err := view.finish(); err != nil {
		return Result{}, atElement(err, root)
	}
	if err := requireContainerText(root); err != nil {
		return Result{}, atElement(err, root)
	}

	lower := lowerer{ctx: ctx, limits: limits, active: make(map[*xmlElement]bool)}
	children, err := lower.elementChildren(root, 0)
	if err != nil {
		return Result{}, err
	}
	var expression ir.Node
	switch len(children) {
	case 0:
		expression = ir.NewRow(source.Detached())
	case 1:
		expression = children[0]
	default:
		expression = ir.NewRow(source.Detached(), children...)
	}
	formula := &ir.Formula{Root: expression}
	for range suppressed {
		formula.Tags = append(formula.Tags, ir.EquationTag{
			Span:       source.Detached(),
			Origin:     source.Detached(),
			Suppressed: true,
			Line:       ir.FormulaTagLine,
		})
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := ir.Validate(formula, limits); err != nil {
		return Result{}, atElement(fmt.Errorf("%w: imported IR is invalid: %v", ErrInvalidMathML, err), root)
	}
	return Result{Formula: formula, Mode: mode}, nil
}

type lowerer struct {
	ctx    context.Context
	limits source.Limits
	count  int
	active map[*xmlElement]bool
}

func (l *lowerer) element(e *xmlElement) (ir.Node, error) {
	if err := l.ctx.Err(); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("%w: nil element", ErrInvalidMathML)
	}
	if e.name.Space != Namespace {
		return nil, atElement(fmt.Errorf("%w: foreign namespace %q", ErrUnsupported, e.name.Space), e)
	}
	if l.active[e] {
		return nil, atElement(fmt.Errorf("%w: element cycle", ErrInvalidMathML), e)
	}
	l.active[e] = true
	defer delete(l.active, e)

	l.count++
	if l.count > l.limits.MaxNodes {
		return nil, atElement(fmt.Errorf("%w: node limit exceeded", ErrLimitExceeded), e)
	}
	view := newAttrView(e)
	kind, hasKind := view.take(attrKind)
	var (
		node ir.Node
		err  error
	)
	switch e.name.Local {
	case "mrow":
		switch kind {
		case "", "row", "multiline-row":
			node, err = l.lowerRow(e, view)
		case "fenced":
			node, err = l.lowerWrappedFence(e, view)
		default:
			err = fmt.Errorf("%w: mrow metadata kind %q", ErrUnsupported, kind)
		}
	case "mfenced":
		if hasKind && kind != "fenced" {
			err = fmt.Errorf("%w: mfenced metadata kind %q", ErrUnsupported, kind)
		} else {
			node, err = l.lowerMFenced(e, view)
		}
	case "mi", "mn":
		if hasKind && kind != "glyph" {
			err = fmt.Errorf("%w: %s metadata kind %q", ErrUnsupported, e.name.Local, kind)
		} else {
			node, err = l.lowerGlyph(e, view, e.name.Local)
		}
	case "mo":
		node, err = l.lowerMO(e, view, kind)
	case "mtext":
		if hasKind && kind != "text" {
			err = fmt.Errorf("%w: mtext metadata kind %q", ErrUnsupported, kind)
		} else {
			node, err = l.lowerText(e, view)
		}
	case "mspace":
		node, err = l.lowerSpace(e, view, kind)
	case "mfrac":
		if hasKind && kind != "fraction" {
			err = fmt.Errorf("%w: mfrac metadata kind %q", ErrUnsupported, kind)
		} else {
			node, err = l.lowerFraction(e, view)
		}
	case "msqrt", "mroot":
		if hasKind && kind != "radical" {
			err = fmt.Errorf("%w: %s metadata kind %q", ErrUnsupported, e.name.Local, kind)
		} else {
			node, err = l.lowerRadical(e, view)
		}
	case "msub", "msup", "msubsup", "munder", "mover", "munderover":
		node, err = l.lowerScripts(e, view, kind)
	case "mtable":
		node, err = l.lowerTable(e, view, kind)
	case "mtr", "mtd":
		err = fmt.Errorf("%w: %s outside mtable", ErrInvalidMathML, e.name.Local)
	default:
		err = fmt.Errorf("%w: element %q", ErrUnsupported, e.name.Local)
	}
	if err != nil {
		return nil, atElement(err, e)
	}
	if err := view.finish(); err != nil {
		return nil, atElement(err, e)
	}
	return node, nil
}

func (l *lowerer) elementChildren(e *xmlElement, depth int) ([]ir.Node, error) {
	if err := requireContainerText(e); err != nil {
		return nil, atElement(err, e)
	}
	if depth > l.limits.MaxNesting {
		return nil, atElement(fmt.Errorf("%w: nesting limit exceeded", ErrLimitExceeded), e)
	}
	out := make([]ir.Node, 0, len(e.children))
	for _, child := range e.children {
		node, err := l.element(child)
		if err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, nil
}

func (l *lowerer) lowerRow(e *xmlElement, view *attrView) (ir.Node, error) {
	node := ir.NewRow(source.Detached())
	if err := view.applyBase(node, ir.ClassNormal); err != nil {
		return nil, err
	}
	children, err := l.elementChildren(e, 1)
	if err != nil {
		return nil, err
	}
	node.Items = children
	return node, nil
}

func (l *lowerer) lowerWrappedFence(e *xmlElement, view *attrView) (ir.Node, error) {
	children, err := l.elementChildren(e, 1)
	if err != nil {
		return nil, err
	}
	hasOpen, err := view.boolDefault(attrHasOpen, true)
	if err != nil {
		return nil, err
	}
	hasClose, err := view.boolDefault(attrHasClose, true)
	if err != nil {
		return nil, err
	}
	if hasOpen && len(children) == 0 || hasClose && len(children) == 0 {
		return nil, fmt.Errorf("%w: fenced mrow is missing an advertised delimiter", ErrInvalidMathML)
	}
	node := ir.NewFenced(source.Detached())
	if err := view.applyBase(node, ir.ClassInner); err != nil {
		return nil, err
	}
	if hasOpen {
		node.Open = children[0]
		children = children[1:]
	}
	if hasClose {
		node.Close = children[len(children)-1]
		children = children[:len(children)-1]
	}
	node.Body = children
	node.Balanced, err = view.boolDefault(attrBalanced, node.Open != nil && node.Close != nil)
	if err != nil {
		return nil, err
	}
	node.Sizing, err = view.delimiterSizing(ir.ContentDelimiterSizing(), true)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (l *lowerer) lowerMFenced(e *xmlElement, view *attrView) (ir.Node, error) {
	open, _ := view.takeDefault("open", "(")
	closeText, _ := view.takeDefault("close", ")")
	separators, _ := view.takeDefault("separators", ",")
	children, err := l.elementChildren(e, 1)
	if err != nil {
		return nil, err
	}
	node := ir.NewFenced(source.Detached())
	if err := view.applyBase(node, ir.ClassInner); err != nil {
		return nil, err
	}
	if open != "" {
		glyph := ir.NewGlyph(source.Detached(), open)
		glyph.Class = ir.ClassOpening
		node.Open = glyph
	}
	if closeText != "" {
		glyph := ir.NewGlyph(source.Detached(), closeText)
		glyph.Class = ir.ClassClosing
		node.Close = glyph
	}
	node.Balanced = node.Open != nil && node.Close != nil
	node.Sizing = ir.ContentDelimiterSizing()
	if len(children) > 1 && separators != "" {
		separatorRunes := []rune(separators)
		body := make([]ir.Node, 0, len(children)*2-1)
		for i, child := range children {
			if i > 0 {
				operator := ir.NewOperator(source.Detached(), string(separatorRunes[(i-1)%len(separatorRunes)]))
				operator.Class = ir.ClassPunctuation
				body = append(body, operator)
			}
			body = append(body, child)
		}
		node.Body = body
	} else {
		node.Body = children
	}
	return node, nil
}

func (l *lowerer) lowerGlyph(e *xmlElement, view *attrView, elementName string) (ir.Node, error) {
	text, err := textContent(e)
	if err != nil {
		return nil, err
	}
	node := ir.NewGlyph(source.Detached(), text)
	defaultClass := ir.ClassNormal
	switch elementName {
	case "mi":
		defaultClass = inferredMIClass(text)
	case "mo":
		defaultClass = inferredMOClass(text)
	}
	if err := view.applyBase(node, defaultClass); err != nil {
		return nil, err
	}
	if elementName == "mo" {
		if err := consumeMOPresentation(view); err != nil {
			return nil, err
		}
	}
	return node, nil
}

func (l *lowerer) lowerMO(e *xmlElement, view *attrView, kind string) (ir.Node, error) {
	if kind == "" {
		kind = "operator"
	}
	switch kind {
	case "glyph":
		return l.lowerGlyph(e, view, "mo")
	case "operator":
		text, err := textContent(e)
		if err != nil {
			return nil, err
		}
		node := ir.NewOperator(source.Detached(), text)
		if err := view.applyBase(node, inferredMOClass(text)); err != nil {
			return nil, err
		}
		if err := consumeMOPresentation(view); err != nil {
			return nil, err
		}
		return node, nil
	case "sized-delimiter":
		text, err := textContent(e)
		if err != nil {
			return nil, err
		}
		node := ir.NewSizedDelimiter(source.Detached(), text, ir.AbsoluteDelimiterSizing(ir.Length{}))
		if err := view.applyBase(node, inferredMOClass(text)); err != nil {
			return nil, err
		}
		hasMetadataSizing := view.has(attrSizingKind)
		sizing, err := view.delimiterSizing(ir.AbsoluteDelimiterSizing(ir.Length{}), false)
		if err != nil {
			return nil, err
		}
		if !hasMetadataSizing {
			if minsize, ok := view.take("minsize"); ok {
				value, err := nonNegativeLength(minsize, "minsize")
				if err != nil {
					return nil, err
				}
				sizing = ir.AbsoluteDelimiterSizing(value)
			} else {
				return nil, fmt.Errorf("%w: sized delimiter has no target policy", ErrInvalidMathML)
			}
		}
		node.Sizing = sizing
		view.ignore("minsize")
		if err := consumeMOPresentation(view); err != nil {
			return nil, err
		}
		return node, nil
	default:
		return nil, fmt.Errorf("%w: mo metadata kind %q", ErrUnsupported, kind)
	}
}

func (l *lowerer) lowerText(e *xmlElement, view *attrView) (ir.Node, error) {
	text, err := textContent(e)
	if err != nil {
		return nil, err
	}
	node := ir.NewText(source.Detached(), text)
	if err := view.applyBase(node, ir.ClassNormal); err != nil {
		return nil, err
	}
	if font, ok := view.take(attrFont); ok {
		value, valid := enumValue[ir.FontRole](fontNames[:], font)
		if !valid {
			return nil, fmt.Errorf("%w: text font %q", ErrInvalidMathML, font)
		}
		node.Font = value
	}
	if tracking, ok := view.take(attrTracking); ok {
		node.Tracking, err = nonNegativeLength(tracking, "tracking")
		if err != nil {
			return nil, err
		}
	}
	if style, ok := view.take("style"); ok {
		tracking, err := letterSpacing(style)
		if err != nil {
			return nil, err
		}
		if !node.Tracking.IsZero() && tracking != node.Tracking {
			return nil, fmt.Errorf("%w: tracking metadata and letter-spacing disagree", ErrInvalidMathML)
		}
		node.Tracking = tracking
	}
	return node, nil
}

func (l *lowerer) lowerSpace(e *xmlElement, view *attrView, kind string) (ir.Node, error) {
	if strings.TrimSpace(e.text) != "" || len(e.children) > 0 {
		return nil, fmt.Errorf("%w: mspace must be empty", ErrInvalidMathML)
	}
	width := ir.Length{}
	var err error
	if value, ok := view.take("width"); ok {
		width, err = nonNegativeLength(value, "width")
		if err != nil {
			return nil, err
		}
	}
	if kind == "" {
		kind = "space"
	}
	switch kind {
	case "break":
		if !width.IsZero() {
			return nil, fmt.Errorf("%w: break mspace width must be zero", ErrInvalidMathML)
		}
		node := ir.NewBreak(source.Detached(), ir.BreakExplicit)
		if value, ok := view.take(attrBreakKind); ok {
			parsedBreak, valid := enumValue[ir.BreakKind](breakKindNames[:], value)
			if !valid {
				return nil, fmt.Errorf("%w: break kind %q", ErrInvalidMathML, value)
			}
			node.Break = parsedBreak
		}
		if err := view.applyBase(node, ir.ClassNormal); err != nil {
			return nil, err
		}
		return node, nil
	case "align-marker":
		if !width.IsZero() {
			return nil, fmt.Errorf("%w: alignment marker width must be zero", ErrInvalidMathML)
		}
		node := ir.NewAlignMarker(source.Detached())
		if err := view.applyBase(node, ir.ClassNormal); err != nil {
			return nil, err
		}
		return node, nil
	case "space":
		spaceType := ir.SpaceExplicit
		if value, ok := view.take(attrSpaceType); ok {
			parsed, valid := enumValue[ir.SpaceKind](spaceTypeNames[:], value)
			if !valid {
				return nil, fmt.Errorf("%w: space type %q", ErrInvalidMathML, value)
			}
			spaceType = parsed
		}
		amount := width
		if value, ok := view.take(attrSpaceAmount); ok {
			amount, err = nonNegativeLength(value, "space amount")
			if err != nil {
				return nil, err
			}
			if _, hasWidth := view.values[xml.Name{Local: "width"}]; hasWidth &&
				spaceType == ir.SpaceExplicit && amount != width {
				return nil, fmt.Errorf("%w: space metadata amount and width disagree", ErrInvalidMathML)
			}
		}
		if spaceType != ir.SpaceExplicit {
			amount = ir.Length{}
		}
		node := ir.NewSpace(source.Detached(), spaceType, amount)
		if weak, ok := view.take(attrSpaceWeak); ok {
			node.Weak, err = parseBool("space weak", weak)
			if err != nil {
				return nil, err
			}
		}
		if err := view.applyBase(node, ir.ClassSpace); err != nil {
			return nil, err
		}
		return node, nil
	default:
		return nil, fmt.Errorf("%w: mspace metadata kind %q", ErrUnsupported, kind)
	}
}

func (l *lowerer) lowerFraction(e *xmlElement, view *attrView) (ir.Node, error) {
	children, err := l.elementChildren(e, 1)
	if err != nil {
		return nil, err
	}
	if len(children) != 2 {
		return nil, fmt.Errorf("%w: mfrac requires exactly two children", ErrInvalidMathML)
	}
	node := ir.NewFraction(source.Detached(), children[0], children[1])
	if err := view.applyBase(node, ir.ClassNormal); err != nil {
		return nil, err
	}
	if style, ok := view.take(attrFractionStyle); ok {
		parsed, valid := enumValue[ir.FractionStyle](fractionStyleNames[:], style)
		if !valid {
			return nil, fmt.Errorf("%w: fraction style %q", ErrInvalidMathML, style)
		}
		node.Style = parsed
	}
	if role, ok := view.take(attrFractionRole); ok {
		parsed, valid := enumValue[ir.FractionRole](fractionRoleNames[:], role)
		if !valid {
			return nil, fmt.Errorf("%w: fraction role %q", ErrInvalidMathML, role)
		}
		node.Role = parsed
	}
	if padding, ok := view.take(attrFractionPad); ok {
		node.Padding, err = nonNegativeLength(padding, "fraction padding")
		if err != nil {
			return nil, err
		}
	}
	if thickness, ok := view.take("linethickness"); ok {
		value, err := parseLength(thickness)
		if err != nil {
			return nil, err
		}
		node.Bar = !value.IsZero()
	}
	if bevelled, ok := view.take("bevelled"); ok {
		value, err := parseBool("bevelled", bevelled)
		if err != nil {
			return nil, err
		}
		if value {
			node.Style = ir.FractionSkewed
			node.Bar = false
		}
	}
	return node, nil
}

func (l *lowerer) lowerRadical(e *xmlElement, view *attrView) (ir.Node, error) {
	children, err := l.elementChildren(e, 1)
	if err != nil {
		return nil, err
	}
	if e.name.Local == "mroot" && len(children) != 2 {
		return nil, fmt.Errorf("%w: mroot requires exactly two children", ErrInvalidMathML)
	}
	if e.name.Local == "msqrt" && len(children) > 1 {
		children = []ir.Node{ir.NewRow(source.Detached(), children...)}
	}
	var radicand ir.Node
	if len(children) == 0 {
		radicand = ir.NewRow(source.Detached())
	} else {
		radicand = children[0]
	}
	var index ir.Node
	if len(children) == 2 {
		index = children[1]
	}
	node := ir.NewRadical(source.Detached(), radicand, index)
	if err := view.applyBase(node, ir.ClassNormal); err != nil {
		return nil, err
	}
	return node, nil
}

func (l *lowerer) lowerScripts(e *xmlElement, view *attrView, kind string) (ir.Node, error) {
	children, err := l.elementChildren(e, 1)
	if err != nil {
		return nil, err
	}
	if kind != "" && kind != "scripts" && kind != "accent" {
		return nil, fmt.Errorf("%w: %s metadata kind %q", ErrUnsupported, e.name.Local, kind)
	}
	if kind == "accent" {
		if len(children) != 2 {
			return nil, fmt.Errorf("%w: accent requires two children", ErrInvalidMathML)
		}
		position := ir.PositionAbove
		if e.name.Local == "munder" {
			position = ir.PositionBelow
		}
		if value, ok := view.take(attrPosition); ok {
			switch value {
			case "above":
				position = ir.PositionAbove
			case "below":
				position = ir.PositionBelow
			default:
				return nil, fmt.Errorf("%w: accent position %q", ErrInvalidMathML, value)
			}
		}
		node := ir.NewAccent(source.Detached(), children[0], children[1], position)
		node.Stretchy, err = view.boolDefault(attrStretchy, node.Stretchy)
		if err != nil {
			return nil, err
		}
		node.NaturalFit, err = view.boolDefault(attrNaturalFit, false)
		if err != nil {
			return nil, err
		}
		node.Dotless, err = view.boolDefault(attrDotless, false)
		if err != nil {
			return nil, err
		}
		node.ExactFrameWidth, err = view.boolDefault(attrExactWidth, false)
		if err != nil {
			return nil, err
		}
		if value, ok := view.take("accent"); ok {
			if _, err := parseBool("accent", value); err != nil {
				return nil, err
			}
		}
		if value, ok := view.take("accentunder"); ok {
			if _, err := parseBool("accentunder", value); err != nil {
				return nil, err
			}
		}
		if err := view.applyBase(node, ir.ClassNormal); err != nil {
			return nil, err
		}
		return node, nil
	}

	expected := map[string]int{"msub": 2, "msup": 2, "msubsup": 3, "munder": 2, "mover": 2, "munderover": 3}[e.name.Local]
	if len(children) != expected {
		return nil, fmt.Errorf("%w: %s requires %d children", ErrInvalidMathML, e.name.Local, expected)
	}
	node := ir.NewScripts(source.Detached(), children[0])
	switch e.name.Local {
	case "msub", "munder":
		node.Bottom = children[1]
	case "msup", "mover":
		node.Top = children[1]
	case "msubsup", "munderover":
		node.Bottom = children[1]
		node.Top = children[2]
	}
	if err := view.applyBase(node, ir.ClassNormal); err != nil {
		return nil, err
	}
	if role, ok := view.take(attrScriptRole); ok {
		parsed, valid := enumValue[ir.ScriptRole](scriptRoleNames[:], role)
		if !valid {
			return nil, fmt.Errorf("%w: script role %q", ErrInvalidMathML, role)
		}
		node.Role = parsed
	} else if strings.HasPrefix(e.name.Local, "munder") || strings.HasPrefix(e.name.Local, "mover") {
		if baseClass(children[0]) == ir.ClassLarge {
			node.Role = ir.ScriptRoleLimits
		} else {
			node.Role = ir.ScriptRoleOverlay
		}
	}
	if layout, ok := view.take(attrScriptLayout); ok {
		switch layout {
		case "side", "limits":
		default:
			return nil, fmt.Errorf("%w: script layout %q", ErrUnsupported, layout)
		}
	}
	return node, nil
}

func (l *lowerer) lowerTable(e *xmlElement, view *attrView, kind string) (ir.Node, error) {
	if kind == "" {
		kind = "table"
	}
	switch kind {
	case "table":
		return l.lowerTableNode(e, view)
	case "multiline":
		return l.lowerMultiline(e, view)
	default:
		return nil, fmt.Errorf("%w: mtable metadata kind %q", ErrUnsupported, kind)
	}
}

func (l *lowerer) lowerTableNode(e *xmlElement, view *attrView) (ir.Node, error) {
	if err := requireContainerText(e); err != nil {
		return nil, err
	}
	node := ir.NewTable(source.Detached())
	if err := view.applyBase(node, ir.ClassNormal); err != nil {
		return nil, err
	}
	if role, ok := view.take(attrTableRole); ok {
		parsed, valid := enumValue[ir.TableRole](tableRoleNames[:], role)
		if !valid {
			return nil, fmt.Errorf("%w: table role %q", ErrInvalidMathML, role)
		}
		node.Role = parsed
	}
	if value, ok := view.take("columnalign"); ok {
		for _, alignment := range strings.Fields(value) {
			parsed, valid := enumValue[ir.ColumnAlign]([]string{"center", "left", "right"}, alignment)
			if !valid {
				return nil, fmt.Errorf("%w: column alignment %q", ErrInvalidMathML, alignment)
			}
			node.Columns = append(node.Columns, parsed)
		}
	}
	var err error
	node.RowGap, err = view.lengthEither(attrRowGap, "rowspacing")
	if err != nil {
		return nil, err
	}
	node.ColumnGap, err = view.lengthEither(attrColumnGap, "columnspacing")
	if err != nil {
		return nil, err
	}
	node.Horizontal, err = view.tableLines(attrHSeparators)
	if err != nil {
		return nil, err
	}
	node.Vertical, err = view.tableLines(attrVSeparators)
	if err != nil {
		return nil, err
	}
	view.ignore("rowlines")
	view.ignore("columnlines")
	for _, child := range e.children {
		if child.name.Local != "mtr" || child.name.Space != Namespace {
			return nil, atElement(fmt.Errorf("%w: mtable child %q", ErrUnsupported, child.name.Local), child)
		}
		row, err := l.lowerTableRow(child)
		if err != nil {
			return nil, err
		}
		node.Rows = append(node.Rows, row)
	}
	return node, nil
}

func (l *lowerer) lowerTableRow(e *xmlElement) (ir.TableRow, error) {
	view := newAttrView(e)
	view.ignoreNonSemantic()
	if err := view.finish(); err != nil {
		return ir.TableRow{}, atElement(err, e)
	}
	if err := requireContainerText(e); err != nil {
		return ir.TableRow{}, atElement(err, e)
	}
	row := ir.TableRow{}
	for _, child := range e.children {
		if child.name.Local != "mtd" || child.name.Space != Namespace {
			return ir.TableRow{}, atElement(fmt.Errorf("%w: mtr child %q", ErrUnsupported, child.name.Local), child)
		}
		cellView := newAttrView(child)
		cellView.ignoreNonSemantic()
		if kind, ok := cellView.take(attrKind); ok && kind != "table-cell" && kind != "multiline-row" {
			return ir.TableRow{}, atElement(fmt.Errorf("%w: mtd metadata kind %q", ErrUnsupported, kind), child)
		}
		if err := cellView.finish(); err != nil {
			return ir.TableRow{}, atElement(err, child)
		}
		nodes, err := l.elementChildren(child, 2)
		if err != nil {
			return ir.TableRow{}, err
		}
		var cell ir.Node
		if len(nodes) == 0 {
			cell = ir.NewRow(source.Detached())
		} else if len(nodes) == 1 {
			cell = nodes[0]
		} else {
			cell = ir.NewRow(source.Detached(), nodes...)
		}
		row.Cells = append(row.Cells, cell)
	}
	return row, nil
}

func (l *lowerer) lowerMultiline(e *xmlElement, view *attrView) (ir.Node, error) {
	if err := requireContainerText(e); err != nil {
		return nil, err
	}
	node := ir.NewMultiline(source.Detached())
	if err := view.applyBase(node, ir.ClassNormal); err != nil {
		return nil, err
	}
	var err error
	node.Centered, err = view.boolDefault(attrCentered, false)
	if err != nil {
		return nil, err
	}
	for _, child := range e.children {
		if child.name.Local != "mtr" || child.name.Space != Namespace {
			return nil, atElement(fmt.Errorf("%w: multiline child %q", ErrUnsupported, child.name.Local), child)
		}
		rowView := newAttrView(child)
		rowView.ignoreNonSemantic()
		if err := rowView.finish(); err != nil {
			return nil, atElement(err, child)
		}
		if err := requireContainerText(child); err != nil {
			return nil, atElement(err, child)
		}
		if len(child.children) != 1 || child.children[0].name.Local != "mtd" {
			return nil, atElement(fmt.Errorf("%w: multiline mtr requires one mtd", ErrInvalidMathML), child)
		}
		cell := child.children[0]
		cellView := newAttrView(cell)
		cellView.ignoreNonSemantic()
		if kind, ok := cellView.take(attrKind); ok && kind != "multiline-row" {
			return nil, atElement(fmt.Errorf("%w: multiline mtd metadata kind %q", ErrUnsupported, kind), cell)
		}
		if err := cellView.finish(); err != nil {
			return nil, atElement(err, cell)
		}
		nodes, err := l.elementChildren(cell, 2)
		if err != nil {
			return nil, err
		}
		node.Rows = append(node.Rows, nodes)
	}
	return node, nil
}

func requireContainerText(e *xmlElement) error {
	if strings.TrimSpace(e.text) != "" {
		return fmt.Errorf("%w: unexpected text in %s", ErrInvalidMathML, e.name.Local)
	}
	return nil
}

func textContent(e *xmlElement) (string, error) {
	if len(e.children) > 0 {
		return "", fmt.Errorf("%w: nested element in %s", ErrInvalidMathML, e.name.Local)
	}
	if e.text == "" {
		return "", fmt.Errorf("%w: empty %s", ErrInvalidMathML, e.name.Local)
	}
	return e.text, nil
}

func inferredMIClass(text string) ir.MathClass {
	runes := []rune(text)
	if len(runes) == 1 && unicode.IsLetter(runes[0]) {
		return ir.ClassAlphabetic
	}
	return ir.ClassNormal
}

func inferredMOClass(text string) ir.MathClass {
	switch text {
	case "(", "[", "{", "⟨", "⌊", "⌈":
		return ir.ClassOpening
	case ")", "]", "}", "⟩", "⌋", "⌉":
		return ir.ClassClosing
	case ",", ";", ":":
		return ir.ClassPunctuation
	case "+", "-", "−", "±", "∓", "×", "÷", "·":
		return ir.ClassBinary
	case "=", "<", ">", "≤", "≥", "≠", "≈", "≡", "∼", "→", "←", "↔", "⇒", "⇐", "⇔":
		return ir.ClassRelation
	case "∑", "∏", "∐", "∫", "∬", "∭", "∮", "⋃", "⋂":
		return ir.ClassLarge
	case "|", "‖":
		return ir.ClassFence
	default:
		return ir.ClassNormal
	}
}

func baseClass(n ir.Node) ir.MathClass {
	if n == nil || n.NodeBase() == nil {
		return ir.ClassNormal
	}
	return n.NodeBase().Class
}

type attrView struct {
	element *xmlElement
	values  map[xml.Name]string
	used    map[xml.Name]bool
}

func newAttrView(e *xmlElement) *attrView {
	values := make(map[xml.Name]string, len(e.attrs))
	for _, attr := range e.attrs {
		values[attr.Name] = attr.Value
	}
	return &attrView{element: e, values: values, used: make(map[xml.Name]bool)}
}

func (v *attrView) take(name string) (string, bool) {
	key := xml.Name{Local: name}
	value, ok := v.values[key]
	if ok {
		v.used[key] = true
	}
	return value, ok
}

func (v *attrView) has(name string) bool {
	_, ok := v.values[xml.Name{Local: name}]
	return ok
}

func (v *attrView) takeDefault(name, fallback string) (string, bool) {
	value, ok := v.take(name)
	if !ok {
		return fallback, false
	}
	return value, true
}

func (v *attrView) ignore(name string) {
	v.take(name)
}

func (v *attrView) ignoreNonSemantic() {
	for _, name := range []string{"class", "id", "aria-label"} {
		v.ignore(name)
	}
	for key := range v.values {
		if key.Space == "xmlns" || key.Local == "xmlns" ||
			(key.Space == "http://www.w3.org/XML/1998/namespace" && key.Local == "lang") {
			v.used[key] = true
		}
	}
}

func (v *attrView) boolDefault(name string, fallback bool) (bool, error) {
	value, ok := v.take(name)
	if !ok {
		return fallback, nil
	}
	return parseBool(name, value)
}

func (v *attrView) optionalNonNegativeInt(name string) (int, error) {
	value, ok := v.take(name)
	if !ok {
		return 0, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("%w: %s must be a non-negative integer", ErrInvalidMathML, name)
	}
	return number, nil
}

func (v *attrView) applyBase(node ir.Node, defaultClass ir.MathClass) error {
	if node == nil || node.NodeBase() == nil {
		return fmt.Errorf("%w: node has no base", ErrInvalidMathML)
	}
	base := node.NodeBase()
	base.Span = source.Detached()
	base.Origin = source.Detached()
	base.Class = defaultClass
	if value, ok := v.take(attrNodeID); ok {
		id, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("%w: node ID %q", ErrInvalidMathML, value)
		}
		base.NodeID = ir.NodeID(id)
	}
	if value, ok := v.take(attrClass); ok {
		parsed, valid := enumValue[ir.MathClass](classNames[:], value)
		if !valid {
			return fmt.Errorf("%w: math class %q", ErrInvalidMathML, value)
		}
		base.Class = parsed
	}
	var (
		dataSize    ir.MathSize
		hasDataSize bool
	)
	if value, ok := v.take(attrBaseSize); ok {
		parsed, valid := enumValue[ir.MathSize](sizeNames[:], value)
		if !valid {
			return fmt.Errorf("%w: math size %q", ErrInvalidMathML, value)
		}
		dataSize, hasDataSize = parsed, true
		base.Size = parsed
	}
	if value, ok := v.take("displaystyle"); ok {
		display, err := parseBool("displaystyle", value)
		if err != nil {
			return err
		}
		if display {
			if hasDataSize && dataSize != ir.SizeDisplay {
				return fmt.Errorf("%w: displaystyle conflicts with node size", ErrInvalidMathML)
			}
			base.Size = ir.SizeDisplay
		} else if hasDataSize && dataSize != ir.SizeText {
			return fmt.Errorf("%w: displaystyle conflicts with node size", ErrInvalidMathML)
		} else {
			base.Size = ir.SizeText
		}
	}
	if value, ok := v.take("scriptlevel"); ok {
		level, err := strconv.Atoi(value)
		if err != nil || level < 0 || level > 2 {
			return fmt.Errorf("%w: scriptlevel must be 0, 1, or 2", ErrInvalidMathML)
		}
		size := [...]ir.MathSize{ir.SizeDisplay, ir.SizeScript, ir.SizeScriptScript}[level]
		if hasDataSize && dataSize != size {
			return fmt.Errorf("%w: scriptlevel conflicts with node size", ErrInvalidMathML)
		}
		base.Size = size
	}
	if value, ok := v.take(attrBaseCramped); ok {
		cramped, err := parseBool("node cramped", value)
		if err != nil {
			return err
		}
		base.Cramped = cramped
	}
	if value, ok := v.take("mathvariant"); ok {
		variant, valid := variantsByName[value]
		if !valid {
			return fmt.Errorf("%w: mathvariant %q", ErrInvalidMathML, value)
		}
		base.Variant = variant
	}
	if color, ok := v.take("mathcolor"); ok {
		if paint, ok := v.take(attrPaintRole); ok {
			_ = paint
			return fmt.Errorf("%w: mathcolor conflicts with paint role", ErrInvalidMathML)
		}
		paint, err := parseColor(color)
		if err != nil {
			return err
		}
		base.Paint = paint
	} else if role, ok := v.take(attrPaintRole); ok {
		parsed, valid := enumValue[ir.PaintRole](paintRoleNames[:], role)
		if !valid {
			return fmt.Errorf("%w: paint role %q", ErrInvalidMathML, role)
		}
		base.Paint = ir.Paint{Role: parsed}
	}
	if value, ok := v.take(attrLimits); ok {
		parsed, valid := enumValue[ir.LimitsPlacement](limitsNames[:], value)
		if !valid {
			return fmt.Errorf("%w: limits placement %q", ErrInvalidMathML, value)
		}
		base.Limits = parsed
	}
	var err error
	if value, ok := v.take("lspace"); ok {
		base.LeftSpacing, err = parseLength(value)
		if err != nil {
			return err
		}
	}
	if value, ok := v.take("rspace"); ok {
		base.RightSpacing, err = parseLength(value)
		if err != nil {
			return err
		}
	}
	if value, ok := v.take(attrIgnoreSpace); ok {
		base.IgnoreAsSpace, err = parseBool("ignore-as-space", value)
		if err != nil {
			return err
		}
	}
	v.ignoreNonSemantic()
	return nil
}

func (v *attrView) delimiterSizing(fallback ir.DelimiterSizing, contentAllowed bool) (ir.DelimiterSizing, error) {
	kindText, hasKind := v.take(attrSizingKind)
	valueText, hasValue := v.take(attrSizingValue)
	if !hasKind && !hasValue {
		return fallback, nil
	}
	if hasKind != hasValue {
		return ir.DelimiterSizing{}, fmt.Errorf("%w: incomplete delimiter sizing", ErrInvalidMathML)
	}
	kind, valid := enumValue[ir.DelimiterSizingKind](sizingKindNames[:], kindText)
	if !valid {
		return ir.DelimiterSizing{}, fmt.Errorf("%w: delimiter sizing kind %q", ErrInvalidMathML, kindText)
	}
	value, err := nonNegativeLength(valueText, "delimiter sizing")
	if err != nil {
		return ir.DelimiterSizing{}, err
	}
	sizing := ir.DelimiterSizing{Kind: kind, Value: value}
	if !sizing.Valid() || (!contentAllowed && kind == ir.DelimiterSizingContent) {
		return ir.DelimiterSizing{}, fmt.Errorf("%w: invalid delimiter sizing", ErrInvalidMathML)
	}
	return sizing, nil
}

func (v *attrView) lengthEither(primary, secondary string) (ir.Length, error) {
	primaryValue, hasPrimary := v.take(primary)
	secondaryValue, hasSecondary := v.take(secondary)
	if !hasPrimary && !hasSecondary {
		return ir.Length{}, nil
	}
	if !hasPrimary {
		return nonNegativeLength(secondaryValue, secondary)
	}
	length, err := nonNegativeLength(primaryValue, primary)
	if err != nil {
		return ir.Length{}, err
	}
	if hasSecondary {
		other, err := nonNegativeLength(secondaryValue, secondary)
		if err != nil {
			return ir.Length{}, err
		}
		if other != length {
			return ir.Length{}, fmt.Errorf("%w: %s and %s disagree", ErrInvalidMathML, primary, secondary)
		}
	}
	return length, nil
}

func (v *attrView) tableLines(name string) ([]ir.TableLine, error) {
	value, ok := v.take(name)
	if !ok || value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ";")
	lines := make([]ir.TableLine, 0, len(parts))
	for _, part := range parts {
		fields := strings.Split(part, ",")
		if len(fields) != 4 {
			return nil, fmt.Errorf("%w: malformed table-line metadata", ErrInvalidMathML)
		}
		position, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("%w: table-line position", ErrInvalidMathML)
		}
		count, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("%w: table-line count", ErrInvalidMathML)
		}
		dashed, err := parseBool("table-line dashed", fields[2])
		if err != nil {
			return nil, err
		}
		thickness, err := nonNegativeLength(fields[3], "table-line thickness")
		if err != nil {
			return nil, err
		}
		lines = append(lines, ir.TableLine{
			Position: position, Count: count, Dashed: dashed, Thickness: thickness,
		})
	}
	return lines, nil
}

func (v *attrView) finish() error {
	for key := range v.values {
		if v.used[key] {
			continue
		}
		if key.Local == "xmlns" || key.Space == "xmlns" ||
			(key.Space == "http://www.w3.org/XML/1998/namespace" && key.Local == "lang") {
			continue
		}
		if key.Space != "" {
			return fmt.Errorf("%w: attribute namespace %q", ErrUnsupported, key.Space)
		}
		return fmt.Errorf("%w: attribute %q", ErrUnsupported, key.Local)
	}
	return nil
}

func consumeMOPresentation(view *attrView) error {
	if value, ok := view.take("form"); ok {
		switch value {
		case "prefix", "infix", "postfix":
		default:
			return fmt.Errorf("%w: operator form %q", ErrInvalidMathML, value)
		}
	}
	for _, name := range []string{"fence", "separator", "largeop", "stretchy", "symmetric"} {
		if value, ok := view.take(name); ok {
			if _, err := parseBool(name, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseBool(field, value string) (bool, error) {
	switch value {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%w: %s must be true or false", ErrInvalidMathML, field)
	}
}

func parseColor(text string) (ir.Paint, error) {
	if text == "currentColor" {
		return ir.Paint{Role: ir.PaintInk}, nil
	}
	if !strings.HasPrefix(text, "#") {
		return ir.Paint{}, fmt.Errorf("%w: color %q; only hex and currentColor are supported", ErrUnsupported, text)
	}
	digits := strings.TrimPrefix(text, "#")
	expand := func(value byte) byte {
		return value<<4 | value
	}
	var color ir.RGBA
	switch len(digits) {
	case 3, 4:
		values := make([]byte, len(digits))
		for i := range digits {
			values[i] = hexNibble(digits[i])
			if values[i] == 0xff {
				return ir.Paint{}, fmt.Errorf("%w: color %q", ErrInvalidMathML, text)
			}
		}
		color = ir.RGBA{
			R: expand(values[0]),
			G: expand(values[1]),
			B: expand(values[2]),
			A: 0xff,
		}
		if len(values) == 4 {
			color.A = expand(values[3])
		}
	case 6:
		raw, ok := parseHexBytes(digits)
		if !ok {
			return ir.Paint{}, fmt.Errorf("%w: color %q", ErrInvalidMathML, text)
		}
		color = ir.RGBA{R: raw[0], G: raw[1], B: raw[2], A: 0xff}
	case 8:
		raw, ok := parseHexBytes(digits)
		if !ok {
			return ir.Paint{}, fmt.Errorf("%w: color %q", ErrInvalidMathML, text)
		}
		color = ir.RGBA{R: raw[0], G: raw[1], B: raw[2], A: raw[3]}
	default:
		return ir.Paint{}, fmt.Errorf("%w: color %q", ErrInvalidMathML, text)
	}
	return ir.Paint{Explicit: color, HasExplicit: true}, nil
}

func hexNibble(value byte) byte {
	switch {
	case value >= '0' && value <= '9':
		return value - '0'
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10
	default:
		return 0xff
	}
}

func parseHexBytes(text string) ([4]byte, bool) {
	var out [4]byte
	for i := 0; i < len(text)/2; i++ {
		high, low := hexNibble(text[i*2]), hexNibble(text[i*2+1])
		if high == 0xff || low == 0xff {
			return out, false
		}
		out[i] = high<<4 | low
	}
	return out, true
}

func letterSpacing(style string) (ir.Length, error) {
	for _, declaration := range strings.Split(style, ";") {
		name, value, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(name) != "letter-spacing" {
			return ir.Length{}, fmt.Errorf("%w: style property %q", ErrUnsupported, strings.TrimSpace(name))
		}
		return nonNegativeLength(strings.TrimSpace(value), "letter-spacing")
	}
	return ir.Length{}, fmt.Errorf("%w: empty style attribute", ErrInvalidMathML)
}

func validXMLText(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0xfffe || r == 0xffff {
			return false
		}
	}
	return true
}

package layout

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// hostSubunits matches geometry.EmScale so font metrics and host conversion do
// not introduce a coarser rounding step before flattening.
const hostSubunits int64 = 1 << 16

const transientReserveLimit = 4096

// Options configures a Renderer. Zero limits use source.DefaultLimits.
type Options struct {
	Limits source.Limits
}

// LayoutOptions controls recoverable behavior for one layout call.
type LayoutOptions struct {
	RecoverVisible bool
}

// Renderer is an immutable layout engine. It is safe for concurrent use if its
// supplied font ports are safe for concurrent use.
type Renderer struct {
	fonts       Fonts
	limits      source.Limits
	scratchPool *layoutScratchPool
}

// New validates the required font ports and constructs a renderer.
func New(fonts Fonts, options Options) (*Renderer, error) {
	if fonts.Glyphs == nil {
		return nil, fmt.Errorf("%w: nil glyph source", ErrInvalidInput)
	}
	if fonts.Metrics == nil {
		return nil, fmt.Errorf("%w: nil style font", ErrInvalidInput)
	}
	return &Renderer{
		fonts:       fonts,
		limits:      options.Limits.WithDefaults(),
		scratchPool: defaultLayoutScratchPool,
	}, nil
}

// Layout lays out root with mode and display style.
func (r *Renderer) Layout(root ir.Node, mode ir.Mode, style display.Style) (*display.Box, error) {
	return r.LayoutContext(context.Background(), root, mode, style)
}

// LayoutContext lays out root with cancellation and deterministic resource
// limits.
func (r *Renderer) LayoutContext(ctx context.Context, root ir.Node, mode ir.Mode, style display.Style) (*display.Box, error) {
	return r.LayoutFormulaContextOptions(ctx, &ir.Formula{Root: root}, mode, style, LayoutOptions{
		RecoverVisible: true,
	})
}

// LayoutFormulaContext lays out a complete formula with cancellation and
// deterministic resource limits.
func (r *Renderer) LayoutFormulaContext(ctx context.Context, formula *ir.Formula, mode ir.Mode, style display.Style) (*display.Box, error) {
	return r.LayoutFormulaContextOptions(ctx, formula, mode, style, LayoutOptions{
		RecoverVisible: true,
	})
}

// LayoutFormulaContextOptions lays out a formula with per-call recovery
// behavior.
func (r *Renderer) LayoutFormulaContextOptions(
	ctx context.Context,
	formula *ir.Formula,
	mode ir.Mode,
	style display.Style,
	options LayoutOptions,
) (*display.Box, error) {
	return r.layoutFormulaContext(ctx, formula, mode, style, true, nil, options)
}

// LayoutParsedFormulaContext lays out a formula that was structurally
// validated and frozen by the parser boundary. Engine-configured limits and
// final output invariants are still enforced.
func (r *Renderer) LayoutParsedFormulaContext(ctx context.Context, formula *ir.Formula, stats ir.Stats, mode ir.Mode, style display.Style) (*display.Box, error) {
	return r.LayoutParsedFormulaContextOptions(ctx, formula, stats, mode, style, LayoutOptions{
		RecoverVisible: true,
	})
}

// LayoutParsedFormulaContextOptions lays out parser-validated formula IR with
// per-call recovery behavior.
func (r *Renderer) LayoutParsedFormulaContextOptions(
	ctx context.Context,
	formula *ir.Formula,
	stats ir.Stats,
	mode ir.Mode,
	style display.Style,
	options LayoutOptions,
) (*display.Box, error) {
	return r.layoutFormulaContext(ctx, formula, mode, style, false, &stats, options)
}

func (r *Renderer) layoutFormulaContext(
	ctx context.Context,
	formula *ir.Formula,
	mode ir.Mode,
	style display.Style,
	validateStructure bool,
	preparedStats *ir.Stats,
	options LayoutOptions,
) (*display.Box, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nil renderer", ErrInvalidInput)
	}
	if ctx == nil {
		return nil, fmt.Errorf("%w: nil context", ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if formula == nil || formula.Root == nil {
		return nil, fmt.Errorf("%w: nil formula", ErrInvalidInput)
	}
	root := formula.Root
	if err := validateConfiguredLimits(ctx, formula, r.limits, preparedStats); err != nil {
		return nil, err
	}
	if validateStructure {
		if err := ir.Validate(formula, structuralValidationLimits()); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
		}
	}
	hostSize, err := hostSizeFromDisplay(style.Size)
	if err != nil {
		return nil, err
	}
	if math.IsNaN(float64(style.MaxWidth)) || math.IsInf(float64(style.MaxWidth), 0) || style.MaxWidth < 0 {
		return nil, fmt.Errorf("%w: invalid max width", ErrInvalidInput)
	}
	if style.Align > display.AlignEnd {
		return nil, fmt.Errorf("%w: invalid alignment", ErrInvalidInput)
	}
	rootStyle, err := rootStyle(mode, style.ScriptLevel)
	if err != nil {
		return nil, err
	}
	metrics := r.fonts.Metrics.MathMetrics(rootStyle, ir.VariantAuto).normalized()
	scratch := r.scratchPool.acquire()
	state := &layoutState{
		ctx:            ctx,
		renderer:       r,
		request:        style,
		mode:           mode,
		rootStyle:      rootStyle,
		metrics:        metrics,
		hostSize:       hostSize,
		limits:         r.limits,
		scratchPool:    r.scratchPool,
		scratch:        scratch,
		fragments:      scratch.fragments,
		glyphs:         scratch.glyphs,
		pathCache:      scratch.pathCache,
		recoverVisible: options.RecoverVisible,
		diagnosticKeys: make(map[recoveryDiagnosticKey]struct{}),
	}
	defer state.releaseScratch()
	if preparedStats != nil {
		hint := boundedCapacity(preparedStats.GlyphCapacity, r.limits.MaxOutputItems)
		state.glyphs.reserve(min(hint, transientReserveLimit))
	}
	env := environment{
		Style:    rootStyle,
		Variant:  ir.VariantAuto,
		Font:     ir.FontMath,
		Features: append([]string(nil), style.Features...),
	}
	var rootFragment *Fragment
	if isEmptyRoot(root) {
		rootFragment, err = state.layoutEmptyRoot(root, env)
	} else {
		rootFragment, err = state.layoutNode(root, env, 0)
	}
	if err != nil {
		return nil, err
	}
	tag, err := state.prepareSingleLineTag(formula.Tags, env)
	if err != nil {
		code, recoverable := recoveryDiagnosticCode(err)
		if !state.recoverVisible || !recoverable {
			return nil, err
		}
		span, text := tagRecoverySource(formula.Tags)
		state.recordRecoveryDiagnosticText(span, text, err, code)
		tag = nil
	}
	var box *display.Box
	if tag != nil {
		// Tagged display output is single-line phase-1 metadata. Avoid a
		// multi-line wrap that cannot own one unambiguous tag.
		if mode == ir.Display && style.MaxWidth > 0 {
			width, widthErr := state.host(rootFragment.Width)
			if widthErr != nil {
				return nil, widthErr
			}
			formulaFrame := maxFloat(0, style.MaxWidth-tag.width)
			offsetX, offsetErr := state.displayLineOffsetWithin(width, formulaFrame)
			if offsetErr != nil {
				return nil, offsetErr
			}
			box, err = state.flattenSingleDisplayLine(rootFragment, offsetX)
		} else {
			box, err = state.flattenFragment(rootFragment)
		}
	} else {
		box, err = state.flatten(rootFragment)
	}
	if err != nil {
		return nil, err
	}
	if tag != nil {
		box, err = state.attachPreparedSingleLineTag(box, tag)
		if err != nil {
			return nil, err
		}
	}
	box.FontUsage = finalizeFontUsage(state.fontUsage)
	if err := box.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	box.Diagnostics = source.NormalizeDiagnostics(state.diagnostics)
	return box, nil
}

func tagRecoverySource(tags []ir.EquationTag) (source.Range, string) {
	span := source.Detached()
	for i := range tags {
		if tags[i].Suppressed {
			continue
		}
		if !span.IsValid() || span.IsDetached() {
			span = tags[i].Span
		}
	}
	if !span.IsValid() || span.IsDetached() {
		return source.NewRange(0, 0), "equation tag"
	}
	return span, "equation tag"
}

type layoutState struct {
	ctx            context.Context
	renderer       *Renderer
	request        display.Style
	mode           ir.Mode
	rootStyle      Style
	metrics        MathMetrics
	hostSize       int64
	limits         source.Limits
	nodes          int
	pathSegments   int
	plannedOutput  int
	fragments      fragmentArena
	glyphs         glyphArena
	pathCache      flattenPathCache
	scratchPool    *layoutScratchPool
	scratch        *layoutScratch
	recoverVisible bool
	diagnostics    []source.Diagnostic
	diagnosticKeys map[recoveryDiagnosticKey]struct{}
	fontUsage      display.FontUsage
}

type recoveryDiagnosticKey struct {
	span source.Range
	code source.Code
	text string
}

func isEmptyRoot(node ir.Node) bool {
	row, ok := node.(*ir.Row)
	return ok && len(row.Items) == 0
}

func (s *layoutState) releaseScratch() {
	if s == nil || s.scratch == nil {
		return
	}
	scratch := s.scratch
	scratch.fragments = s.fragments
	scratch.glyphs = s.glyphs
	scratch.pathCache = s.pathCache
	s.scratch = nil
	s.scratchPool.release(scratch)
}

type environment struct {
	Style           Style
	Variant         ir.Variant
	Font            ir.FontRole
	Paint           ir.Paint
	Features        []string
	LeadingClass    ir.MathClass
	HasLeadingClass bool
	AlignmentInfix  bool
}

func rootStyle(mode ir.Mode, scriptLevel int) (Style, error) {
	if mode != ir.Inline && mode != ir.Display {
		return Style{}, fmt.Errorf("%w: invalid mode %d", ErrInvalidInput, mode)
	}
	style := Style{Size: Text}
	if mode == ir.Display {
		style.Size = Display
	}
	switch {
	case scriptLevel == 0:
	case scriptLevel == 1:
		style.Size = Script
	case scriptLevel >= 2:
		style.Size = ScriptScript
	default:
		return Style{}, fmt.Errorf("%w: negative script level", ErrInvalidInput)
	}
	return style, nil
}

func hostSizeFromDisplay(size float32) (int64, error) {
	if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
		return 0, fmt.Errorf("%w: style size must be finite and positive", ErrInvalidInput)
	}
	scaled := math.Round(float64(size) * float64(hostSubunits))
	if scaled > math.MaxInt64 {
		return 0, fmt.Errorf("%w: style size overflow", ErrInvalidInput)
	}
	subunits := int64(scaled)
	if subunits == 0 {
		return 0, fmt.Errorf("%w: style size below output precision", ErrInvalidInput)
	}
	return subunits, nil
}

func (s *layoutState) check(depth int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if depth > s.limits.MaxNesting {
		return fmt.Errorf("%w: nesting depth %d", ErrLimit, depth)
	}
	s.nodes++
	if s.nodes > s.limits.MaxNodes {
		return fmt.Errorf("%w: node count %d", ErrLimit, s.nodes)
	}
	return nil
}

func (s *layoutState) reservePathSegments(count int) error {
	if count < 0 || count > s.limits.MaxPathSegments-s.pathSegments {
		return fmt.Errorf("%w: path segment count would exceed %d", ErrLimit, s.limits.MaxPathSegments)
	}
	s.pathSegments += count
	return nil
}

func validateConfiguredLimits(ctx context.Context, formula *ir.Formula, limits source.Limits, stats *ir.Stats) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	limits = limits.WithDefaults()
	if stats != nil && stats.WithinLimits(limits) {
		return nil
	}
	nodes := 0
	var visit func(ir.Node, int) error
	visit = func(node ir.Node, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if node == nil {
			return nil
		}
		nodes++
		if nodes > limits.MaxNodes {
			return withNodeSourceRange(node, fmt.Errorf("%w: node count %d", ErrLimit, nodes))
		}
		if depth > limits.MaxNesting {
			return withNodeSourceRange(node, fmt.Errorf("%w: nesting depth %d", ErrLimit, depth))
		}
		if err := checkConfiguredNodeLimits(node, limits); err != nil {
			return withNodeSourceRange(node, err)
		}
		if table, ok := node.(*ir.Table); ok {
			cells := 0
			for _, row := range table.Rows {
				if len(row.Cells) > limits.MaxTableCells-cells {
					return withNodeSourceRange(
						node,
						fmt.Errorf("%w: table cell count exceeds %d", ErrLimit, limits.MaxTableCells),
					)
				}
				cells += len(row.Cells)
			}
		}
		for _, child := range ir.Children(node) {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(formula.Root, 0); err != nil {
		return err
	}
	for i, tag := range formula.Tags {
		nodes++
		if nodes > limits.MaxNodes {
			return withSourceRange(
				fmt.Errorf("%w: tag %d metadata count %d", ErrLimit, i, nodes),
				tagSourceRange(tag),
			)
		}
		if tag.Body == nil {
			continue
		}
		if err := visit(tag.Body, 0); err != nil {
			return err
		}
	}
	return nil
}

func tagSourceRange(tag ir.EquationTag) source.Range {
	if tag.Origin.IsValid() && !tag.Origin.IsDetached() {
		return tag.Origin
	}
	if tag.Span.IsValid() {
		return tag.Span
	}
	return source.Detached()
}

func checkConfiguredNodeLimits(node ir.Node, limits source.Limits) error {
	base := node.NodeBase()
	if base != nil {
		if exceedsLength(base.LeftSpacing, limits.MaxDimension) || exceedsLength(base.RightSpacing, limits.MaxDimension) {
			return fmt.Errorf("%w: node spacing exceeds %d", ErrLimit, limits.MaxDimension)
		}
	}
	checkText := func(text string) error {
		if len(text) > limits.MaxTextBytes {
			return fmt.Errorf("%w: text byte count %d", ErrLimit, len(text))
		}
		return nil
	}
	checkLength := func(length ir.Length) error {
		if exceedsLength(length, limits.MaxDimension) {
			return fmt.Errorf("%w: length exceeds %d", ErrLimit, limits.MaxDimension)
		}
		return nil
	}

	switch value := node.(type) {
	case *ir.Glyph:
		return checkText(value.Text)
	case *ir.SizedDelimiter:
		if err := checkText(value.Text); err != nil {
			return err
		}
		return checkLength(value.Sizing.Value)
	case *ir.Text:
		if err := checkText(value.Text); err != nil {
			return err
		}
		return checkLength(value.Tracking)
	case *ir.Operator:
		return checkText(value.Text)
	case *ir.Space:
		return checkLength(value.Amount)
	case *ir.Fraction:
		return checkLength(value.Padding)
	case *ir.Line:
		if err := checkLength(value.Thickness); err != nil {
			return err
		}
		return checkLength(value.Gap)
	case *ir.Cancel:
		return checkLength(value.Thickness)
	case *ir.Fenced:
		return checkLength(value.Sizing.Value)
	case *ir.Table:
		if err := checkLength(value.RowGap); err != nil {
			return err
		}
		if err := checkLength(value.ColumnGap); err != nil {
			return err
		}
		for _, line := range value.Horizontal {
			if err := checkLength(line.Thickness); err != nil {
				return err
			}
		}
		for _, line := range value.Vertical {
			if err := checkLength(line.Thickness); err != nil {
				return err
			}
		}
	case *ir.Box:
		for _, length := range value.Padding {
			if err := checkLength(length); err != nil {
				return err
			}
		}
		if err := checkLength(value.BorderWidth); err != nil {
			return err
		}
		return checkLength(value.Radius)
	case *ir.ErrorNode:
		return checkText(value.Text)
	case *ir.Placeholder:
		return checkText(value.Reason)
	}
	return nil
}

func exceedsLength(length ir.Length, maxDimension int64) bool {
	if length.Unit == ir.LengthZero || maxDimension <= 0 {
		return false
	}
	const lengthScale = int64(1_000_000)
	if maxDimension > math.MaxInt64/lengthScale {
		return false
	}
	maxValue := maxDimension * lengthScale
	return length.Value > maxValue || length.Value < -maxValue
}

func structuralValidationLimits() source.Limits {
	return source.Limits{
		MaxSourceBytes:     math.MaxInt,
		MaxTokens:          math.MaxInt,
		MaxNesting:         math.MaxInt,
		MaxNodes:           math.MaxInt,
		MaxMacroExpansions: math.MaxInt,
		MaxExpandedTokens:  math.MaxInt,
		MaxCallDepth:       math.MaxInt,
		MaxTableCells:      math.MaxInt,
		MaxAssemblyRepeats: math.MaxInt,
		MaxTextBytes:       math.MaxInt,
		MaxOutputItems:     math.MaxInt,
		MaxPathSegments:    math.MaxInt,
		MaxDimension:       math.MaxInt64,
	}
}

func (s *layoutState) fontMetrics(style Style, variant ir.Variant) MathMetrics {
	return s.renderer.fonts.Metrics.MathMetrics(style, variant).normalized()
}

func (s *layoutState) nodeEnvironment(n ir.Node, env environment) environment {
	base := n.NodeBase()
	if base == nil {
		return env
	}
	if size, ok := styleFromIR(base.Size); ok {
		env.Style.Size = size
	}
	if base.Cramped {
		env.Style.Cramped = true
	}
	if base.Variant != ir.VariantAuto {
		env.Variant = base.Variant
	}
	if base.Paint.HasExplicit || base.Paint.Role != ir.PaintInk {
		env.Paint = base.Paint
	}
	return env
}

func (s *layoutState) layoutNode(n ir.Node, env environment, depth int) (fragment *Fragment, err error) {
	if err := s.check(depth); err != nil {
		return nil, err
	}
	if n == nil {
		return nil, fmt.Errorf("%w: nil node", ErrInvalidInput)
	}
	if err := s.checkTextLimit(n); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && s.recoverVisible {
			code, recoverable := recoveryDiagnosticCode(err)
			if recoverable {
				s.recordRecoveryDiagnostic(n, err, code)
				replacement, recoveryErr := s.layoutRecoveryPlaceholder(n, env)
				if recoveryErr == nil {
					fragment = replacement
					err = nil
				}
			}
		}
		if err == nil && fragment != nil {
			if hitErr := s.attachNodeHit(fragment, n, depth); hitErr != nil {
				fragment = nil
				err = hitErr
			}
		}
		if err != nil {
			err = withNodeSourceRange(n, err)
		}
	}()
	env = s.nodeEnvironment(n, env)

	switch node := n.(type) {
	case *ir.Row:
		fragment, err = s.layoutRow(node.Items, env, depth+1)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	case *ir.Glyph:
		fragment, err = s.layoutClassGlyphText(node.Text, node.Class, env, nil)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	case *ir.Operator:
		fragment, err = s.layoutOperator(node, env)
		if err != nil {
			return nil, err
		}
		return fragment, nil
	case *ir.ErrorNode:
		env.Paint = ir.Paint{Role: ir.PaintError}
		fragment, err = s.layoutGlyphText(node.Text, env, nil)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	case *ir.Text:
		fragment, err = s.layoutText(node, env, depth)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	case *ir.Space:
		return s.layoutSpace(node, env)
	case *ir.Fraction:
		return s.layoutFraction(node, env, depth+1)
	case *ir.Radical:
		return s.layoutRadical(node, env, depth+1)
	case *ir.Scripts:
		return s.layoutScripts(node, env, depth+1)
	case *ir.Fenced:
		return s.layoutFenced(node, env, depth+1)
	case *ir.SizedDelimiter:
		return s.layoutSizedDelimiter(node, env)
	case *ir.Style:
		return s.layoutStyle(node, env, depth+1)
	case *ir.Color:
		child := env
		child.Paint = node.Paint
		fragment, err = s.layoutNode(node.Body, child, depth+1)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, n.NodeBase().Class), nil
	case *ir.Line:
		return s.layoutLine(node, env, depth+1)
	case *ir.Accent:
		return s.layoutAccent(node, env, depth+1)
	case *ir.Cancel:
		return s.layoutCancel(node, env, depth+1)
	case *ir.Box:
		return s.layoutBox(node, env, depth+1)
	case *ir.Phantom:
		return s.layoutPhantom(node, env, depth+1)
	case *ir.Table:
		return s.layoutTable(node, env, depth+1)
	case *ir.Multiline:
		return s.layoutMultiline(node, env, depth+1)
	case *ir.Placeholder:
		return s.layoutPlaceholder(node, env)
	case *ir.Break, *ir.AlignMarker:
		return s.emptyFragment(env.Style.Size, s.scale(env.Style)), nil
	default:
		return nil, fmt.Errorf("%w: %T", ErrUnsupported, n)
	}
}

func recoveryDiagnosticCode(err error) (source.Code, bool) {
	switch {
	case errors.Is(err, ErrMissingGlyph):
		return source.CodeMissingGlyph, true
	case errors.Is(err, ErrInsufficientStretch):
		return source.CodeInsufficientStretch, true
	case errors.Is(err, ErrUnsupported):
		return source.CodeUnsupported, true
	default:
		return "", false
	}
}

func (s *layoutState) recordRecoveryDiagnostic(node ir.Node, cause error, code source.Code) {
	base := node.NodeBase()
	if base == nil {
		return
	}
	s.recordRecoveryDiagnosticText(base.Span, nodeText(node), cause, code)
}

func (s *layoutState) recordRecoveryDiagnosticText(
	span source.Range,
	text string,
	cause error,
	code source.Code,
) {
	key := recoveryDiagnosticKey{span: span, code: code, text: text}
	if _, exists := s.diagnosticKeys[key]; exists {
		return
	}
	s.diagnosticKeys[key] = struct{}{}

	message := string(code)
	hints := []string{"The recoverable item is drawn as an error-colored box."}
	switch code {
	case source.CodeMissingGlyph:
		message = "missing glyph"
		hints = append([]string{"Add a fallback font that covers this text or symbol."}, hints...)
	case source.CodeInsufficientStretch:
		message = "insufficient stretch"
		hints = append([]string{"Add a font with a larger stretchy variant or assembly."}, hints...)
	case source.CodeUnsupported:
		message = "unsupported layout"
		hints = append([]string{"Simplify or rewrite this construct for the current renderer."}, hints...)
	}
	args := []string(nil)
	if text != "" {
		message = fmt.Sprintf("%s: %q", message, text)
		args = []string{text}
	}
	if cause != nil {
		hints = append(hints, cause.Error())
	}
	s.diagnostics = append(s.diagnostics, source.Diagnostic{
		Range:    span,
		Severity: source.SeverityError,
		Code:     code,
		Message:  message,
		Args:     args,
		Hints:    hints,
	})
}

func (s *layoutState) layoutRecoveryPlaceholder(node ir.Node, env environment) (*Fragment, error) {
	fragment, err := s.layoutMissingFragment(env, ir.Paint{Role: ir.PaintError})
	if err != nil {
		return nil, err
	}
	base := node.NodeBase()
	if base != nil {
		fragment = withClass(fragment, base.Class)
	}
	return fragment, nil
}

func (s *layoutState) layoutEmptyRoot(node ir.Node, env environment) (*Fragment, error) {
	fragment, err := s.layoutMissingFragment(env, ir.Paint{Role: ir.PaintPlaceholder})
	if err != nil {
		return nil, err
	}
	if base := node.NodeBase(); base != nil {
		fragment = withClass(fragment, base.Class)
	}
	if err := s.attachNodeHit(fragment, node, 0); err != nil {
		return nil, err
	}
	return fragment, nil
}

func (s *layoutState) checkTextLimit(node ir.Node) error {
	text := nodeText(node)
	if len(text) > s.limits.MaxTextBytes {
		return fmt.Errorf("%w: text byte count %d", ErrLimit, len(text))
	}
	return nil
}

func nodeText(node ir.Node) string {
	var text string
	switch node := node.(type) {
	case *ir.Glyph:
		text = node.Text
	case *ir.Text:
		text = node.Text
	case *ir.Operator:
		text = node.Text
	case *ir.SizedDelimiter:
		text = node.Text
	case *ir.ErrorNode:
		text = node.Text
	case *ir.Placeholder:
		text = node.Reason
	default:
		return ""
	}
	return text
}

package layout

import (
	"context"
	"fmt"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const hostSubunits int64 = 1024

// Options configures a Renderer. Zero limits use source.DefaultLimits.
type Options struct {
	Limits source.Limits
	Tables TableLayouter
}

// Renderer is an immutable layout engine. It is safe for concurrent use if its
// supplied ports are safe for concurrent use.
type Renderer struct {
	fonts  Fonts
	tables TableLayouter
	limits source.Limits
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
		fonts:  fonts,
		tables: options.Tables,
		limits: options.Limits.WithDefaults(),
	}, nil
}

// Layout lays out root with mode and display style.
func (r *Renderer) Layout(root ir.Node, mode ir.Mode, style display.Style) (*display.Box, error) {
	return r.LayoutContext(context.Background(), root, mode, style)
}

// LayoutContext lays out root with cancellation and deterministic resource
// limits.
func (r *Renderer) LayoutContext(ctx context.Context, root ir.Node, mode ir.Mode, style display.Style) (*display.Box, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nil renderer", ErrInvalidInput)
	}
	if ctx == nil {
		return nil, fmt.Errorf("%w: nil context", ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("%w: nil root", ErrInvalidInput)
	}
	if err := validateConfiguredLimits(ctx, root, r.limits); err != nil {
		return nil, err
	}
	if err := ir.Validate(&ir.Formula{Root: root}, structuralValidationLimits()); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
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
	state := &layoutState{
		ctx:       ctx,
		renderer:  r,
		request:   style,
		mode:      mode,
		rootStyle: rootStyle,
		metrics:   metrics,
		hostSize:  hostSize,
		limits:    r.limits,
	}
	env := environment{
		Style:    rootStyle,
		Variant:  ir.VariantAuto,
		Font:     ir.FontMath,
		Features: append([]string(nil), style.Features...),
	}
	rootFragment, err := state.layoutNode(root, env, 0)
	if err != nil {
		return nil, err
	}
	return state.flatten(rootFragment)
}

type layoutState struct {
	ctx          context.Context
	renderer     *Renderer
	request      display.Style
	mode         ir.Mode
	rootStyle    Style
	metrics      MathMetrics
	hostSize     int64
	limits       source.Limits
	nodes        int
	pathSegments int
}

type environment struct {
	Style    Style
	Variant  ir.Variant
	Font     ir.FontRole
	Paint    ir.Paint
	Features []string
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

func validateConfiguredLimits(ctx context.Context, root ir.Node, limits source.Limits) error {
	limits = limits.WithDefaults()
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
			return fmt.Errorf("%w: node count %d", ErrLimit, nodes)
		}
		if depth > limits.MaxNesting {
			return fmt.Errorf("%w: nesting depth %d", ErrLimit, depth)
		}
		if table, ok := node.(*ir.Table); ok {
			cells := 0
			for _, row := range table.Rows {
				cells += len(row.Cells)
			}
			if cells > limits.MaxTableCells {
				return fmt.Errorf("%w: table cell count %d", ErrLimit, cells)
			}
		}
		for _, child := range ir.Children(node) {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(root, 0)
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

func (s *layoutState) layoutNode(n ir.Node, env environment, depth int) (*Fragment, error) {
	if err := s.check(depth); err != nil {
		return nil, err
	}
	if n == nil {
		return nil, fmt.Errorf("%w: nil node", ErrInvalidInput)
	}
	if err := s.checkTextLimit(n); err != nil {
		return nil, err
	}
	env = s.nodeEnvironment(n, env)

	switch node := n.(type) {
	case *ir.Row:
		fragment, err := s.layoutRow(node.Items, env, depth+1)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	case *ir.Glyph:
		fragment, err := s.layoutGlyphText(node.Text, env, nil)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	case *ir.Operator:
		fragment, err := s.layoutOperator(node, env)
		if err != nil {
			return nil, err
		}
		return fragment, nil
	case *ir.ErrorNode:
		env.Paint = ir.Paint{Role: ir.PaintError}
		fragment, err := s.layoutGlyphText(node.Text, env, nil)
		if err != nil {
			return nil, err
		}
		return withClass(fragment, node.Class), nil
	case *ir.Text:
		fragment, err := s.layoutText(node, env)
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
	case *ir.Style:
		return s.layoutStyle(node, env, depth+1)
	case *ir.Color:
		child := env
		child.Paint = node.Paint
		fragment, err := s.layoutNode(node.Body, child, depth+1)
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
	case *ir.Placeholder:
		return s.layoutPlaceholder(node, env)
	case *ir.Break, *ir.AlignMarker:
		return emptyFragment(env.Style.Size, s.scale(env.Style)), nil
	default:
		return nil, fmt.Errorf("%w: %T", ErrUnsupported, n)
	}
}

func (s *layoutState) checkTextLimit(node ir.Node) error {
	var text string
	switch node := node.(type) {
	case *ir.Glyph:
		text = node.Text
	case *ir.Text:
		text = node.Text
	case *ir.Operator:
		text = node.Text
	case *ir.ErrorNode:
		text = node.Text
	case *ir.Placeholder:
		text = node.Reason
	default:
		return nil
	}
	if len(text) > s.limits.MaxTextBytes {
		return fmt.Errorf("%w: text byte count %d", ErrLimit, len(text))
	}
	return nil
}

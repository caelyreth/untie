package ir

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/untie/source"
)

// Stats is the immutable resource-envelope summary produced while validating a
// formula. It lets parser-owned snapshots answer stricter configured-limit
// checks without traversing the full IR again. Values are extrema, not totals,
// except Nodes and Tags.
type Stats struct {
	Nodes             int
	Tags              int
	MaxDepth          int
	MaxTextBytes      int
	MaxDimensionValue int64 // Absolute fixed-point value in millionths of a unit.
	MaxTableCells     int
	// GlyphCapacity is a conservative hint for materialized glyph fragments.
	// Text runs count source bytes, so the value may exceed the shaped output.
	GlyphCapacity int
}

// WithinLimits reports whether a formula with these extrema satisfies limits.
// It is a conservative envelope: a true result is sufficient to skip the full
// configured-limit traversal, while false requires a diagnostic-producing pass.
func (s Stats) WithinLimits(limits source.Limits) bool {
	limits = limits.WithDefaults()
	if s.Nodes > limits.MaxNodes || s.MaxDepth > limits.MaxNesting ||
		s.MaxTextBytes > limits.MaxTextBytes || s.MaxTableCells > limits.MaxTableCells {
		return false
	}
	if s.MaxDimensionValue <= 0 || limits.MaxDimension > math.MaxInt64/lengthScale {
		return true
	}
	return s.MaxDimensionValue <= limits.MaxDimension*lengthScale
}

var (
	ErrInvalidIR      = errors.New("ir: invalid")
	ErrCycle          = errors.New("ir: cycle detected")
	ErrTraversalLimit = errors.New("ir: traversal limit exceeded")
)

// Validate checks the full tree against structural invariants and resource
// limits. It does not have source text, so it can only check that ranges are
// structurally valid. Use ValidateSource at a parser or file-inspection
// boundary. A zero-value limits uses source.DefaultLimits.
func Validate(f *Formula, limits source.Limits) error {
	_, err := ValidateWithStats(f, limits)
	return err
}

// ValidateSource checks a formula and verifies that every attached range lies
// inside src. Detached ranges are permitted.
func ValidateSource(f *Formula, src string, limits source.Limits) error {
	_, err := ValidateSourceWithStats(f, src, limits)
	return err
}

// ValidateWithStats validates f and returns its resource envelope.
func ValidateWithStats(f *Formula, limits source.Limits) (Stats, error) {
	return validate(f, -1, limits)
}

// ValidateSourceWithStats validates f against src and returns its resource
// envelope.
func ValidateSourceWithStats(f *Formula, src string, limits source.Limits) (Stats, error) {
	return validate(f, len(src), limits)
}

func validate(f *Formula, sourceLen int, limits source.Limits) (Stats, error) {
	if f == nil {
		return Stats{}, fmt.Errorf("%w: nil formula", ErrInvalidIR)
	}
	if nodeIsNil(f.Root) {
		return Stats{}, fmt.Errorf("%w: nil root", ErrInvalidIR)
	}
	limits = limits.WithDefaults()
	if sourceLen >= 0 && sourceLen > limits.MaxSourceBytes {
		return Stats{}, fmt.Errorf("%w: source exceeds byte limit", ErrInvalidIR)
	}
	state := validationState{
		limits:    limits,
		sourceLen: sourceLen,
	}
	state.active = state.activeInline[:0]
	state.path.frames = state.pathInline[:0]
	state.path.pushText("root")
	if err := state.visit(f.Root, 0); err != nil {
		return Stats{}, err
	}
	var manualLines map[int]bool
	var rowLines map[int]bool
	previousRowLine := -1
	hasFormulaTag := false
	hasRowTag := false
	if len(f.Tags) > 0 {
		manualLines = make(map[int]bool)
		rowLines = make(map[int]bool)
	}
	for i, tag := range f.Tags {
		state.path.reset()
		state.path.pushTag(i)
		state.count++
		if state.count > state.limits.MaxNodes {
			return Stats{}, fmt.Errorf("%w: %w: %s: metadata limit exceeded", ErrInvalidIR, ErrTraversalLimit, &state.path)
		}
		if err := state.validateRangeField(tag.Span, ".span"); err != nil {
			return Stats{}, err
		}
		if err := state.validateRangeField(tag.Origin, ".origin"); err != nil {
			return Stats{}, err
		}
		if tag.Line < -1 {
			return Stats{}, fmt.Errorf("%w: %s: invalid line %d", ErrInvalidIR, &state.path, tag.Line)
		}
		if tag.Line == FormulaTagLine {
			if hasRowTag {
				return Stats{}, fmt.Errorf("%w: %s: formula and row tags cannot be mixed", ErrInvalidIR, &state.path)
			}
			hasFormulaTag = true
		} else {
			if hasFormulaTag {
				return Stats{}, fmt.Errorf("%w: %s: formula and row tags cannot be mixed", ErrInvalidIR, &state.path)
			}
			if tag.Line <= previousRowLine {
				return Stats{}, fmt.Errorf("%w: %s: row tags must be in strictly increasing line order", ErrInvalidIR, &state.path)
			}
			if rowLines[tag.Line] {
				return Stats{}, fmt.Errorf("%w: %s: multiple tag events for row %d", ErrInvalidIR, &state.path, tag.Line)
			}
			rowLines[tag.Line] = true
			previousRowLine = tag.Line
			hasRowTag = true
		}
		if tag.Starred && !tag.Manual {
			return Stats{}, fmt.Errorf("%w: %s: starred tag must be manual", ErrInvalidIR, &state.path)
		}
		if tag.Suppressed {
			if tag.Manual {
				return Stats{}, fmt.Errorf("%w: %s: tag cannot be both manual and suppressed", ErrInvalidIR, &state.path)
			}
			if tag.Body != nil {
				return Stats{}, fmt.Errorf("%w: %s: suppression event has a body", ErrInvalidIR, &state.path)
			}
			continue
		}
		if !tag.Manual {
			return Stats{}, fmt.Errorf("%w: %s: tag needs a manual body or suppression", ErrInvalidIR, &state.path)
		}
		if tag.Body == nil {
			return Stats{}, fmt.Errorf("%w: %s: manual tag has no body", ErrInvalidIR, &state.path)
		}
		if manualLines[tag.Line] {
			return Stats{}, fmt.Errorf("%w: %s: multiple manual tags for line %d", ErrInvalidIR, &state.path, tag.Line)
		}
		manualLines[tag.Line] = true
		state.path.pushText(".body")
		err := state.visit(tag.Body, 0)
		state.path.pop()
		if err != nil {
			return Stats{}, err
		}
	}
	state.stats.Nodes = state.count
	state.stats.Tags = len(f.Tags)
	return state.stats, nil
}

type validationState struct {
	limits    source.Limits
	sourceLen int
	count     int
	active    []Node
	stats     Stats
	path      validationPath
	// Common ancestor depths avoid allocating a map for cycle detection.
	activeInline [16]Node
	// Common nesting depths avoid a path allocation; deeper trees grow normally.
	pathInline [8]validationPathFrame
}

func (s *validationState) visit(n Node, depth int) error {
	if nodeIsNil(n) {
		return fmt.Errorf("%w: %s: nil node", ErrInvalidIR, &s.path)
	}
	for _, ancestor := range s.active {
		if ancestor == n {
			return fmt.Errorf("%w: %w: %s", ErrInvalidIR, ErrCycle, &s.path)
		}
	}
	if depth > s.stats.MaxDepth {
		s.stats.MaxDepth = depth
	}
	s.active = append(s.active, n)
	defer func() { s.active = s.active[:len(s.active)-1] }()
	s.stats.observe(n)
	s.count++
	if s.count > s.limits.MaxNodes {
		return fmt.Errorf("%w: %w: %s: node limit exceeded", ErrInvalidIR, ErrTraversalLimit, &s.path)
	}
	if depth > s.limits.MaxNesting {
		return fmt.Errorf("%w: %w: %s: nesting limit exceeded", ErrInvalidIR, ErrTraversalLimit, &s.path)
	}

	base := n.NodeBase()
	if base == nil {
		return fmt.Errorf("%w: %s: nil base", ErrInvalidIR, &s.path)
	}
	expected, ok := kindForNode(n)
	if !ok {
		return fmt.Errorf("%w: %s: unsupported node %T", ErrInvalidIR, &s.path, n)
	}
	if base.kind == 0 {
		return fmt.Errorf("%w: %s: zero node kind", ErrInvalidIR, &s.path)
	}
	if base.kind != expected {
		return fmt.Errorf("%w: %s: node type %T has kind %s, want %s",
			ErrInvalidIR, &s.path, n, kindName(base.kind), kindName(expected))
	}
	if err := s.validateRangeField(base.Span, ".span"); err != nil {
		return err
	}
	if err := s.validateRangeField(base.Origin, ".origin"); err != nil {
		return err
	}
	if base.Class > ClassSpecial {
		return fmt.Errorf("%w: %s: invalid math class %d", ErrInvalidIR, &s.path, base.Class)
	}
	if base.Size > SizeScriptScript {
		return fmt.Errorf("%w: %s: invalid math size", ErrInvalidIR, &s.path)
	}
	if base.Variant > VariantMonospace {
		return fmt.Errorf("%w: %s: invalid variant", ErrInvalidIR, &s.path)
	}
	if base.Limits > LimitsDisplay {
		return fmt.Errorf("%w: %s: invalid limits placement", ErrInvalidIR, &s.path)
	}
	if err := s.validatePaintField(base.Paint, ".paint"); err != nil {
		return err
	}
	if err := base.LeftSpacing.ValidateWithLimit(s.limits.MaxDimension); err != nil {
		return fmt.Errorf("%w: %s: left spacing: %v", ErrInvalidIR, &s.path, err)
	}
	if err := base.RightSpacing.ValidateWithLimit(s.limits.MaxDimension); err != nil {
		return fmt.Errorf("%w: %s: right spacing: %v", ErrInvalidIR, &s.path, err)
	}

	if err := validateNode(n, s); err != nil {
		return err
	}
	return s.visitChildren(n, depth)
}

func (s *validationState) validateRangeField(r source.Range, field string) error {
	s.path.pushText(field)
	err := s.validateRange(r)
	s.path.pop()
	return err
}

func (s *validationState) validateRange(r source.Range) error {
	if !r.IsValid() {
		return fmt.Errorf("%w: %s: invalid range", ErrInvalidIR, &s.path)
	}
	if s.sourceLen >= 0 && !r.IsValidFor(s.sourceLen) {
		return fmt.Errorf("%w: %s: range %d:%d exceeds source length %d",
			ErrInvalidIR, &s.path, r.Start, r.End, s.sourceLen)
	}
	return nil
}

func (s *validationState) visitChildren(n Node, depth int) error {
	switch n := n.(type) {
	case *Row:
		return s.visitNodeSlice(n.Kind(), n.Items, depth)
	case *Glyph, *Text, *Operator, *Space, *SizedDelimiter, *ErrorNode, *Placeholder, *Break, *AlignMarker:
		return nil
	case *Fraction:
		if err := s.visitChild(n.Kind(), n.Numerator, 0, depth); err != nil {
			return err
		}
		return s.visitChild(n.Kind(), n.Denominator, 1, depth)
	case *Radical:
		if err := s.visitChild(n.Kind(), n.Radicand, 0, depth); err != nil {
			return err
		}
		if n.Index != nil {
			return s.visitChild(n.Kind(), n.Index, 1, depth)
		}
		return nil
	case *Scripts:
		index := 0
		for _, child := range [...]Node{
			n.BaseItem, n.Top, n.Bottom, n.TopLeft, n.BottomLeft, n.TopRight, n.BottomRight,
		} {
			if child == nil {
				continue
			}
			if err := s.visitChild(n.Kind(), child, index, depth); err != nil {
				return err
			}
			index++
		}
		return nil
	case *Fenced:
		index := 0
		if n.Open != nil {
			if err := s.visitChild(n.Kind(), n.Open, index, depth); err != nil {
				return err
			}
			index++
		}
		for _, child := range n.Body {
			if err := s.visitChild(n.Kind(), child, index, depth); err != nil {
				return err
			}
			index++
		}
		if n.Close != nil {
			return s.visitChild(n.Kind(), n.Close, index, depth)
		}
		return nil
	case *Accent:
		if err := s.visitChild(n.Kind(), n.Body, 0, depth); err != nil {
			return err
		}
		return s.visitChild(n.Kind(), n.Accent, 1, depth)
	case *Line:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	case *Cancel:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	case *Table:
		index := 0
		for _, row := range n.Rows {
			for _, child := range row.Cells {
				if err := s.visitChild(n.Kind(), child, index, depth); err != nil {
					return err
				}
				index++
			}
		}
		return nil
	case *Multiline:
		index := 0
		for _, row := range n.Rows {
			for _, child := range row {
				if err := s.visitChild(n.Kind(), child, index, depth); err != nil {
					return err
				}
				index++
			}
		}
		return nil
	case *Style:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	case *Overlay:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	case *Raise:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	case *Color:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	case *Box:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	case *Phantom:
		return s.visitChild(n.Kind(), n.Body, 0, depth)
	default:
		return nil
	}
}

func (s *validationState) visitNodeSlice(kind NodeKind, children []Node, depth int) error {
	for i, child := range children {
		if err := s.visitChild(kind, child, i, depth); err != nil {
			return err
		}
	}
	return nil
}

func (s *validationState) visitChild(kind NodeKind, child Node, index, depth int) error {
	s.path.pushChild(kind, index)
	if child == nil {
		err := fmt.Errorf("%w: %s: nil child", ErrInvalidIR, &s.path)
		s.path.pop()
		return err
	}
	err := s.visit(child, depth+1)
	s.path.pop()
	return err
}

func validateNode(n Node, s *validationState) error {
	limits := s.limits
	switch n := n.(type) {
	case *Row:
		// Empty rows are meaningful zero-ink roots and null-delimiter placeholders.
	case *Glyph:
		if err := validateTextBytes(n.Text, limits, &s.path, "glyph"); err != nil {
			return err
		}
	case *Text:
		if err := validateTextBytes(n.Text, limits, &s.path, "text"); err != nil {
			return err
		}
		if n.Font > FontText {
			return fmt.Errorf("%w: %s: invalid font role", ErrInvalidIR, &s.path)
		}
		if err := s.validateNonNegativeField(n.Tracking, ".tracking"); err != nil {
			return err
		}
		if err := validateTextClusterSources(n, s); err != nil {
			return err
		}
	case *Operator:
		if err := validateTextBytes(n.Text, limits, &s.path, "operator"); err != nil {
			return err
		}
	case *Space:
		if n.Type > SpaceIgnored {
			return fmt.Errorf("%w: %s: invalid space kind", ErrInvalidIR, &s.path)
		}
		if err := n.Amount.ValidateWithLimit(limits.MaxDimension); err != nil {
			return fmt.Errorf("%w: %s: space amount: %v", ErrInvalidIR, &s.path, err)
		}
		if n.Type != SpaceExplicit && !n.Amount.IsZero() {
			return fmt.Errorf("%w: %s: non-explicit space has an explicit amount", ErrInvalidIR, &s.path)
		}
	case *Fraction:
		if n.Numerator == nil || n.Denominator == nil {
			return fmt.Errorf("%w: %s: missing fraction part", ErrInvalidIR, &s.path)
		}
		if n.Style > FractionHorizontal {
			return fmt.Errorf("%w: %s: invalid fraction style", ErrInvalidIR, &s.path)
		}
		if n.Role > FractionRoleStacked {
			return fmt.Errorf("%w: %s: invalid fraction role", ErrInvalidIR, &s.path)
		}
		if n.Role != FractionRoleFraction && n.Style != FractionNoBar {
			return fmt.Errorf("%w: %s: semantic fraction role requires no-bar style", ErrInvalidIR, &s.path)
		}
		if n.Style != FractionVertical && n.Bar {
			return fmt.Errorf("%w: %s: non-vertical fraction has a bar", ErrInvalidIR, &s.path)
		}
		if err := s.validateNonNegativeField(n.Padding, ".padding"); err != nil {
			return err
		}
	case *Radical:
		if n.Radicand == nil {
			return fmt.Errorf("%w: %s: missing radicand", ErrInvalidIR, &s.path)
		}
	case *Scripts:
		if n.BaseItem == nil {
			return fmt.Errorf("%w: %s: missing script base", ErrInvalidIR, &s.path)
		}
		if n.Role > ScriptRoleBraceLabel {
			return fmt.Errorf("%w: %s: invalid script role", ErrInvalidIR, &s.path)
		}
	case *Fenced:
		if len(n.Body) == 0 && n.Open == nil && n.Close == nil {
			return fmt.Errorf("%w: %s: empty fence", ErrInvalidIR, &s.path)
		}
		if n.Balanced && (n.Open == nil || n.Close == nil) {
			return fmt.Errorf("%w: %s: balanced fence has a missing delimiter", ErrInvalidIR, &s.path)
		}
		if err := s.validateDelimiterSizingField(n.Sizing, ".sizing", true); err != nil {
			return err
		}
	case *SizedDelimiter:
		if err := validateTextBytes(n.Text, limits, &s.path, "sized delimiter"); err != nil {
			return err
		}
		if utf8.RuneCountInString(n.Text) != 1 {
			return fmt.Errorf("%w: %s: sized delimiter text must be one rune", ErrInvalidIR, &s.path)
		}
		if err := s.validateDelimiterSizingField(n.Sizing, ".sizing", false); err != nil {
			return err
		}
		switch n.Class {
		case ClassNormal, ClassOpening, ClassClosing, ClassRelation:
		default:
			return fmt.Errorf("%w: %s: invalid sized delimiter class", ErrInvalidIR, &s.path)
		}
	case *Accent:
		if n.Body == nil || n.Accent == nil {
			return fmt.Errorf("%w: %s: missing accent part", ErrInvalidIR, &s.path)
		}
		if n.Position > PositionBelow {
			return fmt.Errorf("%w: %s: invalid accent position", ErrInvalidIR, &s.path)
		}
	case *Line:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing line body", ErrInvalidIR, &s.path)
		}
		if n.Position > PositionBelow {
			return fmt.Errorf("%w: %s: invalid line position", ErrInvalidIR, &s.path)
		}
		if err := s.validateNonNegativeField(n.Thickness, ".thickness"); err != nil {
			return err
		}
		if err := s.validateNonNegativeField(n.Gap, ".gap"); err != nil {
			return err
		}
	case *Cancel:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing cancel body", ErrInvalidIR, &s.path)
		}
		if err := s.validateNonNegativeField(n.Thickness, ".thickness"); err != nil {
			return err
		}
		if n.Role > CancelRoleNegation {
			return fmt.Errorf("%w: %s: invalid cancel role", ErrInvalidIR, &s.path)
		}
		if n.Role == CancelRoleNegation && (n.Cross || n.Inverted) {
			return fmt.Errorf("%w: %s: negation cannot use cross or inverted cancellation", ErrInvalidIR, &s.path)
		}
	case *Table:
		if err := validateTable(n, s); err != nil {
			return err
		}
	case *Multiline:
		if len(n.Rows) == 0 {
			return fmt.Errorf("%w: %s: empty multiline", ErrInvalidIR, &s.path)
		}
	case *Style:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing style body", ErrInvalidIR, &s.path)
		}
		if n.SetSize && n.NewSize == SizeInherit {
			return fmt.Errorf("%w: %s: inherit size cannot be explicit", ErrInvalidIR, &s.path)
		}
		if n.NewSize > SizeScriptScript {
			return fmt.Errorf("%w: %s: invalid style size", ErrInvalidIR, &s.path)
		}
		if !n.SetCramped && n.Cramped {
			return fmt.Errorf("%w: %s: cramped value set without SetCramped", ErrInvalidIR, &s.path)
		}
		if !n.SetFontSize && !n.FontSize.IsZero() {
			return fmt.Errorf("%w: %s: font size set without SetFontSize", ErrInvalidIR, &s.path)
		}
		if n.SetFontSize {
			if n.FontSize.Unit != LengthPercent {
				return fmt.Errorf("%w: %s: font size must be a positive percentage", ErrInvalidIR, &s.path)
			}
			if err := n.FontSize.ValidateWithLimit(limits.MaxDimension); err != nil {
				return fmt.Errorf("%w: %s: font size: %v", ErrInvalidIR, &s.path, err)
			}
			if n.FontSize.Value <= 0 {
				return fmt.Errorf("%w: %s: font size must be positive", ErrInvalidIR, &s.path)
			}
		}
	case *Overlay:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing overlay body", ErrInvalidIR, &s.path)
		}
		if n.Alignment > OverlayEnd {
			return fmt.Errorf("%w: %s: invalid overlay alignment", ErrInvalidIR, &s.path)
		}
	case *Raise:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing raise body", ErrInvalidIR, &s.path)
		}
		if n.Amount.Unit == LengthPercent {
			return fmt.Errorf("%w: %s: raise amount cannot be percentage-based", ErrInvalidIR, &s.path)
		}
		if err := n.Amount.ValidateWithLimit(limits.MaxDimension); err != nil {
			return fmt.Errorf("%w: %s: raise amount: %v", ErrInvalidIR, &s.path, err)
		}
	case *Color:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing color body", ErrInvalidIR, &s.path)
		}
		if err := s.validatePaintField(n.Paint, ".paint"); err != nil {
			return err
		}
	case *Box:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing box body", ErrInvalidIR, &s.path)
		}
		for i, length := range n.Padding {
			s.path.pushPadding(i)
			err := validateNonNegative(length, limits, &s.path)
			s.path.pop()
			if err != nil {
				return err
			}
		}
		if !n.HasFill && !paintIsZero(n.Fill) {
			return fmt.Errorf("%w: %s: fill paint set without HasFill", ErrInvalidIR, &s.path)
		}
		if err := s.validatePaintField(n.Fill, ".fill"); err != nil {
			return err
		}
		if !n.HasBorder && (!paintIsZero(n.Border) || !n.BorderWidth.IsZero()) {
			return fmt.Errorf("%w: %s: border fields set without HasBorder", ErrInvalidIR, &s.path)
		}
		if err := s.validatePaintField(n.Border, ".border"); err != nil {
			return err
		}
		if err := s.validateNonNegativeField(n.BorderWidth, ".border-width"); err != nil {
			return err
		}
		if err := s.validateNonNegativeField(n.Radius, ".radius"); err != nil {
			return err
		}
	case *Phantom:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing phantom body", ErrInvalidIR, &s.path)
		}
	case *ErrorNode:
		if err := validateTextBytes(n.Text, limits, &s.path, "error"); err != nil {
			return err
		}
	case *Placeholder:
		if n.Reason == "" {
			return fmt.Errorf("%w: %s: empty placeholder reason", ErrInvalidIR, &s.path)
		}
		if !utf8.ValidString(n.Reason) {
			return fmt.Errorf("%w: %s: placeholder reason is not valid UTF-8", ErrInvalidIR, &s.path)
		}
		if len(n.Reason) > limits.MaxTextBytes {
			return fmt.Errorf("%w: %s: placeholder reason too long", ErrInvalidIR, &s.path)
		}
	case *Break:
		if n.Break > BreakExplicit {
			return fmt.Errorf("%w: %s: invalid break kind", ErrInvalidIR, &s.path)
		}
	case *AlignMarker:
		// No extra fields.
	default:
		return fmt.Errorf("%w: %s: unsupported node %T", ErrInvalidIR, &s.path, n)
	}
	return nil
}

func validateTextClusterSources(n *Text, s *validationState) error {
	if len(n.Clusters) == 0 {
		return nil
	}
	textEnd := 0
	var previousSource source.Range
	for i, cluster := range n.Clusters {
		s.path.pushChild(KindText, i)
		err := func() error {
			if cluster.TextStart != textEnd {
				return fmt.Errorf("%w: %s: text cluster must partition Text at byte %d",
					ErrInvalidIR, &s.path, textEnd)
			}
			if cluster.TextEnd <= cluster.TextStart || cluster.TextEnd > len(n.Text) {
				return fmt.Errorf("%w: %s: invalid text byte range %d:%d",
					ErrInvalidIR, &s.path, cluster.TextStart, cluster.TextEnd)
			}
			if !utf8.RuneStart(n.Text[cluster.TextStart]) ||
				(cluster.TextEnd < len(n.Text) && !utf8.RuneStart(n.Text[cluster.TextEnd])) {
				return fmt.Errorf("%w: %s: text cluster splits a UTF-8 rune", ErrInvalidIR, &s.path)
			}
			if cluster.Source.IsDetached() || !cluster.Source.IsValid() ||
				cluster.Source.End <= cluster.Source.Start {
				return fmt.Errorf("%w: %s: invalid source range %+v",
					ErrInvalidIR, &s.path, cluster.Source)
			}
			if i > 0 && cluster.Source.Start < previousSource.End {
				return fmt.Errorf("%w: %s: source ranges overlap or are out of order",
					ErrInvalidIR, &s.path)
			}
			s.path.pushText(".source")
			err := s.validateRange(cluster.Source)
			s.path.pop()
			return err
		}()
		s.path.pop()
		if err != nil {
			return err
		}
		textEnd = cluster.TextEnd
		previousSource = cluster.Source
	}
	if textEnd != len(n.Text) {
		s.path.pushText(".clusters")
		err := fmt.Errorf("%w: %s: source map does not cover Text", ErrInvalidIR, &s.path)
		s.path.pop()
		return err
	}
	return nil
}

func (s *validationState) validateNonNegativeField(length Length, field string) error {
	s.path.pushText(field)
	err := validateNonNegative(length, s.limits, &s.path)
	s.path.pop()
	return err
}

func (s *validationState) validatePaintField(paint Paint, field string) error {
	s.path.pushText(field)
	err := validatePaint(paint, &s.path)
	s.path.pop()
	return err
}

func (s *validationState) validateDelimiterSizingField(sizing DelimiterSizing, field string, contentAllowed bool) error {
	s.path.pushText(field)
	err := validateDelimiterSizing(sizing, s.limits, &s.path, contentAllowed)
	s.path.pop()
	return err
}

func validateTextBytes(text string, limits source.Limits, path *validationPath, label string) error {
	if text == "" {
		return fmt.Errorf("%w: %s: empty %s", ErrInvalidIR, path, label)
	}
	if !utf8.ValidString(text) {
		return fmt.Errorf("%w: %s: %s is not valid UTF-8", ErrInvalidIR, path, label)
	}
	if len(text) > limits.MaxTextBytes {
		return fmt.Errorf("%w: %s: %s text too long", ErrInvalidIR, path, label)
	}
	return nil
}

func validateDelimiterSizing(sizing DelimiterSizing, limits source.Limits, path *validationPath, contentAllowed bool) error {
	if sizing.Kind > DelimiterSizingAbsolute {
		return fmt.Errorf("%w: %s: invalid delimiter sizing kind %d", ErrInvalidIR, path, sizing.Kind)
	}
	if !contentAllowed && sizing.Kind == DelimiterSizingContent {
		return fmt.Errorf("%w: %s: standalone delimiter requires a number or length target", ErrInvalidIR, path)
	}
	if err := sizing.Value.ValidateWithLimit(limits.MaxDimension); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidIR, path, err)
	}
	if !sizing.Valid() {
		switch sizing.Kind {
		case DelimiterSizingContent:
			return fmt.Errorf("%w: %s: content sizing cannot carry a value", ErrInvalidIR, path)
		case DelimiterSizingRelative:
			return fmt.Errorf("%w: %s: relative sizing requires a percent length", ErrInvalidIR, path)
		default:
			return fmt.Errorf("%w: %s: absolute sizing requires a non-negative non-percent length", ErrInvalidIR, path)
		}
	}
	return nil
}

func validateNonNegative(length Length, limits source.Limits, path *validationPath) error {
	if err := length.ValidateWithLimit(limits.MaxDimension); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidIR, path, err)
	}
	if length.Value < 0 {
		return fmt.Errorf("%w: %s: negative length", ErrInvalidIR, path)
	}
	return nil
}

func validatePaint(paint Paint, path *validationPath) error {
	if paint.Role > PaintPlaceholder {
		return fmt.Errorf("%w: %s: invalid paint role %d", ErrInvalidIR, path, paint.Role)
	}
	if paint.HasExplicit && paint.Role != PaintInk {
		return fmt.Errorf("%w: %s: explicit paint also selects role %d", ErrInvalidIR, path, paint.Role)
	}
	if !paint.HasExplicit && paint.Explicit != (RGBA{}) {
		return fmt.Errorf("%w: %s: explicit color set without HasExplicit", ErrInvalidIR, path)
	}
	return nil
}

func paintIsZero(paint Paint) bool {
	return !paint.HasExplicit && paint.Role == PaintInk && paint.Explicit == (RGBA{})
}

func validateTable(table *Table, s *validationState) error {
	limits := s.limits
	if table.Role > TableRoleVector {
		return fmt.Errorf("%w: %s: invalid table role", ErrInvalidIR, &s.path)
	}
	cells := 0
	maxColumns := len(table.Columns)
	for _, row := range table.Rows {
		cells += len(row.Cells)
		maxColumns = max(maxColumns, len(row.Cells))
	}
	if cells > limits.MaxTableCells {
		return fmt.Errorf("%w: %s: table cell limit exceeded", ErrInvalidIR, &s.path)
	}
	for i, column := range table.Columns {
		if column > AlignRight {
			return fmt.Errorf("%w: %s: invalid column alignment at %d", ErrInvalidIR, &s.path, i)
		}
	}
	if err := s.validateNonNegativeField(table.RowGap, ".row-gap"); err != nil {
		return err
	}
	if err := s.validateNonNegativeField(table.ColumnGap, ".column-gap"); err != nil {
		return err
	}
	for i, line := range table.Horizontal {
		if err := validateTableLine(line, len(table.Rows), "horizontal", limits); err != nil {
			s.path.pushTableLine("horizontal", i)
			err = fmt.Errorf("%w: %s: %v", ErrInvalidIR, &s.path, err)
			s.path.pop()
			return err
		}
	}
	for i, line := range table.Vertical {
		if err := validateTableLine(line, maxColumns, "vertical", limits); err != nil {
			s.path.pushTableLine("vertical", i)
			err = fmt.Errorf("%w: %s: %v", ErrInvalidIR, &s.path, err)
			s.path.pop()
			return err
		}
	}
	return nil
}

func validateTableLine(line TableLine, boundaryCount int, axis string, limits source.Limits) error {
	if line.Position < 0 || line.Position > boundaryCount {
		return fmt.Errorf("%s line position %d outside 0..%d", axis, line.Position, boundaryCount)
	}
	if line.Count <= 0 {
		return fmt.Errorf("%s line count must be positive", axis)
	}
	if err := line.Thickness.ValidateWithLimit(limits.MaxDimension); err != nil || line.Thickness.Value < 0 {
		var frames [1]validationPathFrame
		path := validationPath{frames: frames[:0]}
		path.pushText("thickness")
		return validateNonNegative(line.Thickness, limits, &path)
	}
	return nil
}

// validationPath stores structural segments without rendering diagnostic text.
type validationPath struct {
	frames []validationPathFrame
}

type validationPathFrame struct {
	text      string
	kind      NodeKind
	index     int
	frameKind validationPathFrameKind
}

type validationPathFrameKind uint8

const (
	validationPathText validationPathFrameKind = iota
	validationPathChild
	validationPathTag
	validationPathPadding
	validationPathTableLine
)

func (p *validationPath) reset() {
	p.frames = p.frames[:0]
}

func (p *validationPath) pushText(text string) {
	p.frames = append(p.frames, validationPathFrame{frameKind: validationPathText, text: text})
}

func (p *validationPath) pushChild(kind NodeKind, index int) {
	p.frames = append(p.frames, validationPathFrame{
		frameKind: validationPathChild,
		kind:      kind,
		index:     index,
	})
}

func (p *validationPath) pushTag(index int) {
	p.frames = append(p.frames, validationPathFrame{frameKind: validationPathTag, index: index})
}

func (p *validationPath) pushPadding(index int) {
	p.frames = append(p.frames, validationPathFrame{frameKind: validationPathPadding, index: index})
}

func (p *validationPath) pushTableLine(axis string, index int) {
	p.frames = append(p.frames, validationPathFrame{
		frameKind: validationPathTableLine,
		text:      axis,
		index:     index,
	})
}

func (p *validationPath) pop() {
	p.frames = p.frames[:len(p.frames)-1]
}

func (p *validationPath) String() string {
	var b strings.Builder
	for _, frame := range p.frames {
		switch frame.frameKind {
		case validationPathText:
			b.WriteString(frame.text)
		case validationPathChild:
			b.WriteByte('.')
			b.WriteString(kindName(frame.kind))
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(frame.index))
			b.WriteByte(']')
		case validationPathTag:
			b.WriteString("tags[")
			b.WriteString(strconv.Itoa(frame.index))
			b.WriteByte(']')
		case validationPathPadding:
			b.WriteString(".padding[")
			b.WriteString(strconv.Itoa(frame.index))
			b.WriteByte(']')
		case validationPathTableLine:
			b.WriteByte('.')
			b.WriteString(frame.text)
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(frame.index))
			b.WriteByte(']')
		}
	}
	return b.String()
}

func kindForNode(n Node) (NodeKind, bool) {
	switch n.(type) {
	case *Row:
		return KindRow, true
	case *Glyph:
		return KindGlyph, true
	case *Text:
		return KindText, true
	case *Operator:
		return KindOperator, true
	case *Space:
		return KindSpace, true
	case *Fraction:
		return KindFraction, true
	case *Radical:
		return KindRadical, true
	case *Scripts:
		return KindScripts, true
	case *Fenced:
		return KindFenced, true
	case *SizedDelimiter:
		return KindSizedDelimiter, true
	case *Accent:
		return KindAccent, true
	case *Line:
		return KindLine, true
	case *Cancel:
		return KindCancel, true
	case *Table:
		return KindTable, true
	case *Multiline:
		return KindMultiline, true
	case *Style:
		return KindStyle, true
	case *Overlay:
		return KindOverlay, true
	case *Raise:
		return KindRaise, true
	case *Color:
		return KindColor, true
	case *Box:
		return KindBox, true
	case *Phantom:
		return KindPhantom, true
	case *ErrorNode:
		return KindError, true
	case *Placeholder:
		return KindPlaceholder, true
	case *Break:
		return KindBreak, true
	case *AlignMarker:
		return KindAlignMarker, true
	default:
		return 0, false
	}
}

func kindName(kind NodeKind) string {
	names := [...]string{
		"invalid",
		"row",
		"glyph",
		"text",
		"operator",
		"space",
		"fraction",
		"radical",
		"scripts",
		"fenced",
		"accent",
		"line",
		"cancel",
		"table",
		"multiline",
		"style",
		"color",
		"box",
		"phantom",
		"error",
		"placeholder",
		"break",
		"align-marker",
		"sized-delimiter",
		"overlay",
		"raise",
	}
	if int(kind) >= len(names) {
		return "unknown"
	}
	return names[kind]
}

// KindName returns the canonical serialization name for kind.
func KindName(kind NodeKind) string {
	return kindName(kind)
}

// FormatPath joins a path segment safely for diagnostics.
func FormatPath(parts ...string) string {
	return strings.Join(parts, ".")
}

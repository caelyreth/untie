package ir

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caelyreth/untie/source"
)

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
	return validate(f, -1, limits)
}

// ValidateSource checks a formula and verifies that every attached range lies
// inside src. Detached ranges are permitted.
func ValidateSource(f *Formula, src string, limits source.Limits) error {
	return validate(f, len(src), limits)
}

func validate(f *Formula, sourceLen int, limits source.Limits) error {
	if f == nil {
		return fmt.Errorf("%w: nil formula", ErrInvalidIR)
	}
	if f.Root == nil {
		return fmt.Errorf("%w: nil root", ErrInvalidIR)
	}
	limits = limits.WithDefaults()
	if sourceLen >= 0 && sourceLen > limits.MaxSourceBytes {
		return fmt.Errorf("%w: source exceeds byte limit", ErrInvalidIR)
	}
	state := validationState{
		limits:    limits,
		sourceLen: sourceLen,
		active:    make(map[Node]bool),
	}
	return state.visit(f.Root, "root", 0)
}

type validationState struct {
	limits    source.Limits
	sourceLen int
	count     int
	active    map[Node]bool
}

func (s *validationState) visit(n Node, path string, depth int) error {
	if n == nil {
		return fmt.Errorf("%w: %s: nil node", ErrInvalidIR, path)
	}
	if s.active[n] {
		return fmt.Errorf("%w: %w: %s", ErrInvalidIR, ErrCycle, path)
	}
	s.count++
	if s.count > s.limits.MaxNodes {
		return fmt.Errorf("%w: %w: %s: node limit exceeded", ErrInvalidIR, ErrTraversalLimit, path)
	}
	if depth > s.limits.MaxNesting {
		return fmt.Errorf("%w: %w: %s: nesting limit exceeded", ErrInvalidIR, ErrTraversalLimit, path)
	}

	base := n.NodeBase()
	if base == nil {
		return fmt.Errorf("%w: %s: nil base", ErrInvalidIR, path)
	}
	expected, ok := kindForNode(n)
	if !ok {
		return fmt.Errorf("%w: %s: unsupported node %T", ErrInvalidIR, path, n)
	}
	if base.kind == 0 {
		return fmt.Errorf("%w: %s: zero node kind", ErrInvalidIR, path)
	}
	if base.kind != expected {
		return fmt.Errorf("%w: %s: node type %T has kind %s, want %s",
			ErrInvalidIR, path, n, kindName(base.kind), kindName(expected))
	}
	if err := s.validateRange(base.Span, path+".span"); err != nil {
		return err
	}
	if err := s.validateRange(base.Origin, path+".origin"); err != nil {
		return err
	}
	if base.Class > ClassSpecial {
		return fmt.Errorf("%w: %s: invalid math class %d", ErrInvalidIR, path, base.Class)
	}
	if base.Size > SizeScriptScript {
		return fmt.Errorf("%w: %s: invalid math size", ErrInvalidIR, path)
	}
	if base.Variant > VariantMonospace {
		return fmt.Errorf("%w: %s: invalid variant", ErrInvalidIR, path)
	}
	if base.Limits > LimitsNever {
		return fmt.Errorf("%w: %s: invalid limits placement", ErrInvalidIR, path)
	}
	if err := validatePaint(base.Paint, path+".paint"); err != nil {
		return err
	}
	if err := base.LeftSpacing.ValidateWithLimit(s.limits.MaxDimension); err != nil {
		return fmt.Errorf("%w: %s: left spacing: %v", ErrInvalidIR, path, err)
	}
	if err := base.RightSpacing.ValidateWithLimit(s.limits.MaxDimension); err != nil {
		return fmt.Errorf("%w: %s: right spacing: %v", ErrInvalidIR, path, err)
	}

	s.active[n] = true
	defer delete(s.active, n)
	if err := validateNode(n, path, s.limits); err != nil {
		return err
	}
	children := Children(n)
	for i, child := range children {
		childPath := fmt.Sprintf("%s.%s[%d]", path, kindName(n.Kind()), i)
		if child == nil {
			return fmt.Errorf("%w: %s: nil child", ErrInvalidIR, childPath)
		}
		if err := s.visit(child, childPath, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (s *validationState) validateRange(r source.Range, path string) error {
	if !r.IsValid() {
		return fmt.Errorf("%w: %s: invalid range", ErrInvalidIR, path)
	}
	if s.sourceLen >= 0 && !r.IsValidFor(s.sourceLen) {
		return fmt.Errorf("%w: %s: range %d:%d exceeds source length %d",
			ErrInvalidIR, path, r.Start, r.End, s.sourceLen)
	}
	return nil
}

func validateNode(n Node, path string, limits source.Limits) error {
	switch n := n.(type) {
	case *Row:
		// Empty rows are meaningful zero-ink roots and null-delimiter placeholders.
	case *Glyph:
		if err := validateTextBytes(n.Text, limits, path, "glyph"); err != nil {
			return err
		}
	case *Text:
		if err := validateTextBytes(n.Text, limits, path, "text"); err != nil {
			return err
		}
		if n.Font > FontText {
			return fmt.Errorf("%w: %s: invalid font role", ErrInvalidIR, path)
		}
		if err := validateNonNegative(n.Tracking, limits, path+".tracking"); err != nil {
			return err
		}
	case *Operator:
		if err := validateTextBytes(n.Text, limits, path, "operator"); err != nil {
			return err
		}
	case *Space:
		if n.Type > SpaceIgnored {
			return fmt.Errorf("%w: %s: invalid space kind", ErrInvalidIR, path)
		}
		if err := n.Amount.ValidateWithLimit(limits.MaxDimension); err != nil {
			return fmt.Errorf("%w: %s: space amount: %v", ErrInvalidIR, path, err)
		}
		if n.Type != SpaceExplicit && !n.Amount.IsZero() {
			return fmt.Errorf("%w: %s: non-explicit space has an explicit amount", ErrInvalidIR, path)
		}
	case *Fraction:
		if n.Numerator == nil || n.Denominator == nil {
			return fmt.Errorf("%w: %s: missing fraction part", ErrInvalidIR, path)
		}
		if n.Style > FractionHorizontal {
			return fmt.Errorf("%w: %s: invalid fraction style", ErrInvalidIR, path)
		}
		if err := validateNonNegative(n.Padding, limits, path+".padding"); err != nil {
			return err
		}
	case *Radical:
		if n.Radicand == nil {
			return fmt.Errorf("%w: %s: missing radicand", ErrInvalidIR, path)
		}
	case *Scripts:
		if n.BaseItem == nil {
			return fmt.Errorf("%w: %s: missing script base", ErrInvalidIR, path)
		}
	case *Fenced:
		if len(n.Body) == 0 && n.Open == nil && n.Close == nil {
			return fmt.Errorf("%w: %s: empty fence", ErrInvalidIR, path)
		}
		if n.Balanced && (n.Open == nil || n.Close == nil) {
			return fmt.Errorf("%w: %s: balanced fence has a missing delimiter", ErrInvalidIR, path)
		}
	case *Accent:
		if n.Body == nil || n.Accent == nil {
			return fmt.Errorf("%w: %s: missing accent part", ErrInvalidIR, path)
		}
		if n.Position > PositionBelow {
			return fmt.Errorf("%w: %s: invalid accent position", ErrInvalidIR, path)
		}
	case *Line:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing line body", ErrInvalidIR, path)
		}
		if n.Position > PositionBelow {
			return fmt.Errorf("%w: %s: invalid line position", ErrInvalidIR, path)
		}
		if err := validateNonNegative(n.Thickness, limits, path+".thickness"); err != nil {
			return err
		}
		if err := validateNonNegative(n.Gap, limits, path+".gap"); err != nil {
			return err
		}
	case *Cancel:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing cancel body", ErrInvalidIR, path)
		}
		if err := validateNonNegative(n.Thickness, limits, path+".thickness"); err != nil {
			return err
		}
	case *Table:
		if err := validateTable(n, path, limits); err != nil {
			return err
		}
	case *Multiline:
		if len(n.Rows) == 0 {
			return fmt.Errorf("%w: %s: empty multiline", ErrInvalidIR, path)
		}
		for i, row := range n.Rows {
			if len(row) == 0 {
				return fmt.Errorf("%w: %s: empty multiline row %d", ErrInvalidIR, path, i)
			}
		}
	case *Style:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing style body", ErrInvalidIR, path)
		}
		if n.SetSize && n.NewSize == SizeInherit {
			return fmt.Errorf("%w: %s: inherit size cannot be explicit", ErrInvalidIR, path)
		}
		if n.NewSize > SizeScriptScript {
			return fmt.Errorf("%w: %s: invalid style size", ErrInvalidIR, path)
		}
		if !n.SetCramped && n.Cramped {
			return fmt.Errorf("%w: %s: cramped value set without SetCramped", ErrInvalidIR, path)
		}
	case *Color:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing color body", ErrInvalidIR, path)
		}
		if err := validatePaint(n.Paint, path+".paint"); err != nil {
			return err
		}
	case *Box:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing box body", ErrInvalidIR, path)
		}
		for i, length := range n.Padding {
			if err := validateNonNegative(length, limits, fmt.Sprintf("%s.padding[%d]", path, i)); err != nil {
				return err
			}
		}
		if !n.HasFill && !paintIsZero(n.Fill) {
			return fmt.Errorf("%w: %s: fill paint set without HasFill", ErrInvalidIR, path)
		}
		if err := validatePaint(n.Fill, path+".fill"); err != nil {
			return err
		}
		if !n.HasBorder && (!paintIsZero(n.Border) || !n.BorderWidth.IsZero()) {
			return fmt.Errorf("%w: %s: border fields set without HasBorder", ErrInvalidIR, path)
		}
		if err := validatePaint(n.Border, path+".border"); err != nil {
			return err
		}
		if err := validateNonNegative(n.BorderWidth, limits, path+".border-width"); err != nil {
			return err
		}
		if err := validateNonNegative(n.Radius, limits, path+".radius"); err != nil {
			return err
		}
	case *Phantom:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing phantom body", ErrInvalidIR, path)
		}
	case *ErrorNode:
		if err := validateTextBytes(n.Text, limits, path, "error"); err != nil {
			return err
		}
	case *Placeholder:
		if n.Reason == "" {
			return fmt.Errorf("%w: %s: empty placeholder reason", ErrInvalidIR, path)
		}
		if len(n.Reason) > limits.MaxTextBytes {
			return fmt.Errorf("%w: %s: placeholder reason too long", ErrInvalidIR, path)
		}
	case *Break:
		if n.Break > BreakExplicit {
			return fmt.Errorf("%w: %s: invalid break kind", ErrInvalidIR, path)
		}
	case *AlignMarker:
		// No extra fields.
	default:
		return fmt.Errorf("%w: %s: unsupported node %T", ErrInvalidIR, path, n)
	}
	return nil
}

func validateTextBytes(text string, limits source.Limits, path, label string) error {
	if text == "" {
		return fmt.Errorf("%w: %s: empty %s", ErrInvalidIR, path, label)
	}
	if len(text) > limits.MaxTextBytes {
		return fmt.Errorf("%w: %s: %s text too long", ErrInvalidIR, path, label)
	}
	return nil
}

func validateNonNegative(length Length, limits source.Limits, path string) error {
	if err := length.ValidateWithLimit(limits.MaxDimension); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidIR, path, err)
	}
	if length.Value < 0 {
		return fmt.Errorf("%w: %s: negative length", ErrInvalidIR, path)
	}
	return nil
}

func validatePaint(paint Paint, path string) error {
	if paint.Role > PaintPlaceholder {
		return fmt.Errorf("%w: %s: invalid paint role %d", ErrInvalidIR, path, paint.Role)
	}
	if paint.HasExplicit && paint.Role != PaintInk {
		return fmt.Errorf("%w: %s: explicit paint also selects role %d", ErrInvalidIR, path, paint.Role)
	}
	return nil
}

func paintIsZero(paint Paint) bool {
	return !paint.HasExplicit && paint.Role == PaintInk && paint.Explicit == (RGBA{})
}

func validateTable(table *Table, path string, limits source.Limits) error {
	cells := 0
	maxColumns := len(table.Columns)
	for i, row := range table.Rows {
		if len(row.Cells) == 0 {
			return fmt.Errorf("%w: %s: row %d has no cells", ErrInvalidIR, path, i)
		}
		cells += len(row.Cells)
		maxColumns = max(maxColumns, len(row.Cells))
	}
	if cells > limits.MaxTableCells {
		return fmt.Errorf("%w: %s: table cell limit exceeded", ErrInvalidIR, path)
	}
	for i, column := range table.Columns {
		if column > AlignRight {
			return fmt.Errorf("%w: %s: invalid column alignment at %d", ErrInvalidIR, path, i)
		}
	}
	if err := validateNonNegative(table.RowGap, limits, path+".row-gap"); err != nil {
		return err
	}
	if err := validateNonNegative(table.ColumnGap, limits, path+".column-gap"); err != nil {
		return err
	}
	for i, line := range table.Horizontal {
		if err := validateTableLine(line, len(table.Rows), "horizontal", limits); err != nil {
			return fmt.Errorf("%w: %s.horizontal[%d]: %v", ErrInvalidIR, path, i, err)
		}
	}
	for i, line := range table.Vertical {
		if err := validateTableLine(line, maxColumns, "vertical", limits); err != nil {
			return fmt.Errorf("%w: %s.vertical[%d]: %v", ErrInvalidIR, path, i, err)
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
	if err := validateNonNegative(line.Thickness, limits, "thickness"); err != nil {
		return err
	}
	return nil
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

package ir

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caelyreth/untie/source"
)

var (
	ErrInvalidIR = errors.New("ir: invalid")
)

// Validate checks the full tree against structural invariants and resource
// limits. A zero-value limits uses source.DefaultLimits.
func Validate(f *Formula, limits source.Limits) error {
	if f == nil {
		return fmt.Errorf("%w: nil formula", ErrInvalidIR)
	}
	if f.Root == nil {
		return fmt.Errorf("%w: nil root", ErrInvalidIR)
	}
	limits = limits.WithDefaults()
	state := validationState{limits: limits}
	if err := state.visit(f.Root, "root", 0); err != nil {
		return err
	}
	return nil
}

type validationState struct {
	limits source.Limits
	count  int
}

func (s *validationState) visit(n Node, path string, depth int) error {
	if n == nil {
		return fmt.Errorf("%w: %s: nil node", ErrInvalidIR, path)
	}
	s.count++
	if s.count > s.limits.MaxNodes {
		return fmt.Errorf("%w: %s: node limit exceeded", ErrInvalidIR, path)
	}
	if depth > s.limits.MaxNesting {
		return fmt.Errorf("%w: %s: nesting limit exceeded", ErrInvalidIR, path)
	}

	base := n.NodeBase()
	if base == nil {
		return fmt.Errorf("%w: %s: nil base", ErrInvalidIR, path)
	}
	if base.kind == 0 {
		return fmt.Errorf("%w: %s: zero node kind", ErrInvalidIR, path)
	}
	if !base.Span.IsValid() {
		return fmt.Errorf("%w: %s: invalid span", ErrInvalidIR, path)
	}
	if !base.Origin.IsValid() {
		return fmt.Errorf("%w: %s: invalid origin", ErrInvalidIR, path)
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
	if err := base.LeftSpacing.Validate(); err != nil {
		return fmt.Errorf("%w: %s: left spacing: %v", ErrInvalidIR, path, err)
	}
	if err := base.RightSpacing.Validate(); err != nil {
		return fmt.Errorf("%w: %s: right spacing: %v", ErrInvalidIR, path, err)
	}
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

func validateNode(n Node, path string, limits source.Limits) error {
	switch n := n.(type) {
	case *Row:
		if len(n.Items) == 0 {
			return nil
		}
	case *Glyph:
		if n.Text == "" {
			return fmt.Errorf("%w: %s: empty glyph", ErrInvalidIR, path)
		}
		if len(n.Text) > limits.MaxTextBytes {
			return fmt.Errorf("%w: %s: glyph text too long", ErrInvalidIR, path)
		}
	case *Text:
		if n.Text == "" {
			return fmt.Errorf("%w: %s: empty text", ErrInvalidIR, path)
		}
		if len(n.Text) > limits.MaxTextBytes {
			return fmt.Errorf("%w: %s: text too long", ErrInvalidIR, path)
		}
		if n.Font > FontText {
			return fmt.Errorf("%w: %s: invalid font role", ErrInvalidIR, path)
		}
		if err := n.Tracking.Validate(); err != nil {
			return fmt.Errorf("%w: %s: tracking: %v", ErrInvalidIR, path, err)
		}
	case *Operator:
		if n.Text == "" {
			return fmt.Errorf("%w: %s: empty operator", ErrInvalidIR, path)
		}
	case *Space:
		if n.Type > SpaceIgnored {
			return fmt.Errorf("%w: %s: invalid space kind", ErrInvalidIR, path)
		}
		if err := n.Amount.Validate(); err != nil {
			return fmt.Errorf("%w: %s: space amount: %v", ErrInvalidIR, path, err)
		}
	case *Fraction:
		if n.Numerator == nil || n.Denominator == nil {
			return fmt.Errorf("%w: %s: missing fraction part", ErrInvalidIR, path)
		}
		if n.Style > FractionHorizontal {
			return fmt.Errorf("%w: %s: invalid fraction style", ErrInvalidIR, path)
		}
		if err := n.Padding.Validate(); err != nil {
			return fmt.Errorf("%w: %s: fraction padding: %v", ErrInvalidIR, path, err)
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
		if err := n.Thickness.Validate(); err != nil {
			return fmt.Errorf("%w: %s: line thickness: %v", ErrInvalidIR, path, err)
		}
		if err := n.Gap.Validate(); err != nil {
			return fmt.Errorf("%w: %s: line gap: %v", ErrInvalidIR, path, err)
		}
	case *Cancel:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing cancel body", ErrInvalidIR, path)
		}
		if err := n.Thickness.Validate(); err != nil {
			return fmt.Errorf("%w: %s: cancel thickness: %v", ErrInvalidIR, path, err)
		}
	case *Table:
		cells := 0
		for _, row := range n.Rows {
			cells += len(row.Cells)
		}
		if cells > limits.MaxTableCells {
			return fmt.Errorf("%w: %s: table cell limit exceeded", ErrInvalidIR, path)
		}
		for _, col := range n.Columns {
			if col > AlignRight {
				return fmt.Errorf("%w: %s: invalid column alignment", ErrInvalidIR, path)
			}
		}
		if err := n.RowGap.Validate(); err != nil {
			return fmt.Errorf("%w: %s: row gap: %v", ErrInvalidIR, path, err)
		}
		if err := n.ColumnGap.Validate(); err != nil {
			return fmt.Errorf("%w: %s: column gap: %v", ErrInvalidIR, path, err)
		}
	case *Multiline:
		if len(n.Rows) == 0 {
			return fmt.Errorf("%w: %s: empty multiline", ErrInvalidIR, path)
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
	case *Color:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing color body", ErrInvalidIR, path)
		}
	case *Box:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing box body", ErrInvalidIR, path)
		}
		for _, length := range n.Padding {
			if err := length.Validate(); err != nil {
				return fmt.Errorf("%w: %s: box padding: %v", ErrInvalidIR, path, err)
			}
		}
		if err := n.BorderWidth.Validate(); err != nil {
			return fmt.Errorf("%w: %s: border width: %v", ErrInvalidIR, path, err)
		}
		if err := n.Radius.Validate(); err != nil {
			return fmt.Errorf("%w: %s: box radius: %v", ErrInvalidIR, path, err)
		}
	case *Phantom:
		if n.Body == nil {
			return fmt.Errorf("%w: %s: missing phantom body", ErrInvalidIR, path)
		}
	case *ErrorNode:
		if n.Text == "" {
			return fmt.Errorf("%w: %s: empty error text", ErrInvalidIR, path)
		}
	case *Placeholder:
		if n.Reason == "" {
			return fmt.Errorf("%w: %s: empty placeholder reason", ErrInvalidIR, path)
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

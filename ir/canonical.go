package ir

import (
	"encoding/json"
	"fmt"

	"github.com/caelyreth/untie/source"
)

// MarshalCanonical returns deterministic JSON for golden tests and cache
// diagnostics. It is not the final public protocol. An optional limits argument
// overrides source.DefaultLimits.
func MarshalCanonical(f *Formula, limits ...source.Limits) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("ir: nil formula")
	}
	effective := source.DefaultLimits()
	if len(limits) > 0 {
		effective = limits[0]
	}
	if err := Validate(f, effective); err != nil {
		return nil, err
	}
	state := canonicalState{
		limits: effective.WithDefaults(),
		active: make(map[Node]bool),
	}
	snapshot, err := state.node(f.Root, 0)
	if err != nil {
		return nil, err
	}
	return json.Marshal(snapshot)
}

type canonicalState struct {
	limits source.Limits
	count  int
	active map[Node]bool
}

func (s *canonicalState) node(n Node, depth int) (any, error) {
	if nodeIsNil(n) {
		return nil, nil
	}
	if s.active[n] {
		return nil, fmt.Errorf("%w: %w", ErrInvalidIR, ErrCycle)
	}
	s.count++
	if s.count > s.limits.MaxNodes {
		return nil, fmt.Errorf("%w: %w: node limit exceeded", ErrInvalidIR, ErrTraversalLimit)
	}
	if depth > s.limits.MaxNesting {
		return nil, fmt.Errorf("%w: %w: nesting limit exceeded", ErrInvalidIR, ErrTraversalLimit)
	}
	s.active[n] = true
	defer delete(s.active, n)

	base := snapshotBase(n.NodeBase())
	switch n := n.(type) {
	case *Row:
		items, err := s.nodes(n.Items, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "items": items}, nil
	case *Glyph:
		return map[string]any{"base": base, "text": n.Text}, nil
	case *Text:
		return map[string]any{"base": base, "text": n.Text, "font": n.Font, "tracking": snapshotLength(n.Tracking)}, nil
	case *Operator:
		return map[string]any{"base": base, "text": n.Text}, nil
	case *Space:
		return map[string]any{"base": base, "space": n.Type, "amount": snapshotLength(n.Amount), "weak": n.Weak}, nil
	case *Fraction:
		numerator, err := s.node(n.Numerator, depth+1)
		if err != nil {
			return nil, err
		}
		denominator, err := s.node(n.Denominator, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "numerator": numerator, "denominator": denominator, "bar": n.Bar, "style": n.Style, "padding": snapshotLength(n.Padding)}, nil
	case *Radical:
		radicand, err := s.node(n.Radicand, depth+1)
		if err != nil {
			return nil, err
		}
		index, err := s.node(n.Index, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "radicand": radicand, "index": index}, nil
	case *Scripts:
		children := make(map[string]any, 7)
		for name, child := range map[string]Node{
			"main": n.BaseItem, "top": n.Top, "bottom": n.Bottom,
			"top-left": n.TopLeft, "bottom-left": n.BottomLeft,
			"top-right": n.TopRight, "bottom-right": n.BottomRight,
		} {
			snapshot, err := s.node(child, depth+1)
			if err != nil {
				return nil, err
			}
			children[name] = snapshot
		}
		children["base"] = base
		return children, nil
	case *Fenced:
		open, err := s.node(n.Open, depth+1)
		if err != nil {
			return nil, err
		}
		closeNode, err := s.node(n.Close, depth+1)
		if err != nil {
			return nil, err
		}
		body, err := s.nodes(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "open": open, "close": closeNode, "body": body, "balanced": n.Balanced}, nil
	case *Accent:
		body, err := s.node(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		accent, err := s.node(n.Accent, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "body": body, "accent": accent, "position": n.Position, "stretchy": n.Stretchy, "dotless": n.Dotless, "exact-width": n.ExactFrameWidth}, nil
	case *Line:
		body, err := s.node(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "body": body, "position": n.Position, "thickness": snapshotLength(n.Thickness), "gap": snapshotLength(n.Gap)}, nil
	case *Cancel:
		body, err := s.node(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "body": body, "cross": n.Cross, "inverted": n.Inverted, "thickness": snapshotLength(n.Thickness)}, nil
	case *Table:
		rows := make([]any, len(n.Rows))
		for i, row := range n.Rows {
			cells, err := s.nodes(row.Cells, depth+1)
			if err != nil {
				return nil, err
			}
			tag, err := s.node(row.Tag, depth+1)
			if err != nil {
				return nil, err
			}
			rows[i] = map[string]any{"cells": cells, "tag": tag}
		}
		return map[string]any{"base": base, "rows": rows, "columns": n.Columns, "row-gap": snapshotLength(n.RowGap), "column-gap": snapshotLength(n.ColumnGap), "horizontal": n.Horizontal, "vertical": n.Vertical}, nil
	case *Multiline:
		rows := make([]any, len(n.Rows))
		for i, row := range n.Rows {
			items, err := s.nodes(row, depth+1)
			if err != nil {
				return nil, err
			}
			rows[i] = items
		}
		return map[string]any{"base": base, "rows": rows, "centered": n.Centered}, nil
	case *Style:
		body, err := s.node(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "body": body, "set-size": n.SetSize, "size": n.NewSize, "set-cramped": n.SetCramped, "cramped": n.Cramped}, nil
	case *Color:
		body, err := s.node(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "body": body, "paint": snapshotPaint(n.Paint)}, nil
	case *Box:
		body, err := s.node(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "body": body, "padding": snapshotLengths(n.Padding[:]), "fill": snapshotPaint(n.Fill), "has-fill": n.HasFill, "border": snapshotPaint(n.Border), "has-border": n.HasBorder, "border-width": snapshotLength(n.BorderWidth), "radius": snapshotLength(n.Radius)}, nil
	case *Phantom:
		body, err := s.node(n.Body, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base": base, "body": body, "width": n.Width, "height": n.Height, "depth": n.Depth}, nil
	case *ErrorNode:
		return map[string]any{"base": base, "text": n.Text, "code": n.Code}, nil
	case *Placeholder:
		return map[string]any{"base": base, "reason": n.Reason}, nil
	case *Break:
		return map[string]any{"base": base, "break": n.Break, "penalty": n.Penalty}, nil
	case *AlignMarker:
		return map[string]any{"base": base}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported node %T", ErrInvalidIR, n)
	}
}

func (s *canonicalState) nodes(nodes []Node, depth int) ([]any, error) {
	out := make([]any, len(nodes))
	for i, n := range nodes {
		snapshot, err := s.node(n, depth)
		if err != nil {
			return nil, err
		}
		out[i] = snapshot
	}
	return out, nil
}

func snapshotBase(b *Base) map[string]any {
	return map[string]any{
		"kind":            KindName(b.kind),
		"span":            []int{b.Span.Start, b.Span.End},
		"origin":          []int{b.Origin.Start, b.Origin.End},
		"class":           b.Class,
		"size":            b.Size,
		"cramped":         b.Cramped,
		"variant":         b.Variant,
		"paint":           snapshotPaint(b.Paint),
		"limits":          b.Limits,
		"left-spacing":    snapshotLength(b.LeftSpacing),
		"right-spacing":   snapshotLength(b.RightSpacing),
		"ignore-as-space": b.IgnoreAsSpace,
	}
}

func snapshotPaint(p Paint) map[string]any {
	if p.HasExplicit {
		return map[string]any{"rgba": []uint8{p.Explicit.R, p.Explicit.G, p.Explicit.B, p.Explicit.A}}
	}
	return map[string]any{"role": p.Role}
}

func snapshotLength(l Length) []any {
	if l.Unit == LengthZero {
		return nil
	}
	return []any{l.Unit, l.Value}
}

func snapshotLengths(lengths []Length) []any {
	out := make([]any, len(lengths))
	for i, l := range lengths {
		out[i] = snapshotLength(l)
	}
	return out
}

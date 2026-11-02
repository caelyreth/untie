package ir

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/caelyreth/untie/source"
)

// Children returns direct child nodes in deterministic semantic order.
// Returned slices never alias node-owned storage.
func Children(n Node) []Node {
	if nodeIsNil(n) {
		return nil
	}
	switch n := n.(type) {
	case *Row:
		return slices.Clone(n.Items)
	case *Glyph, *Text, *Operator, *Space, *SizedDelimiter, *ErrorNode, *Placeholder, *Break, *AlignMarker:
		return nil
	case *Fraction:
		return compact(n.Numerator, n.Denominator)
	case *Radical:
		return compact(n.Radicand, n.Index)
	case *Scripts:
		return compact(n.BaseItem, n.Top, n.Bottom, n.TopLeft, n.BottomLeft, n.TopRight, n.BottomRight)
	case *Fenced:
		out := make([]Node, 0, len(n.Body)+2)
		if n.Open != nil {
			out = append(out, n.Open)
		}
		out = append(out, n.Body...)
		if n.Close != nil {
			out = append(out, n.Close)
		}
		return out
	case *Accent:
		return compact(n.Body, n.Accent)
	case *Line:
		return compact(n.Body)
	case *Cancel:
		return compact(n.Body)
	case *Table:
		var out []Node
		for _, row := range n.Rows {
			out = append(out, row.Cells...)
		}
		return out
	case *Multiline:
		var out []Node
		for _, row := range n.Rows {
			out = append(out, row...)
		}
		return out
	case *Style:
		return compact(n.Body)
	case *Color:
		return compact(n.Body)
	case *Box:
		return compact(n.Body)
	case *Phantom:
		return compact(n.Body)
	default:
		return nil
	}
}

// WalkFormula visits the formula root followed by every equation tag body in
// source order. Traversal budgets and cycle checks are shared across all
// entries, so tags cannot bypass the formula's node or nesting limits.
func WalkFormula(f *Formula, limits source.Limits, fn func(Node) bool) error {
	if f == nil {
		return fmt.Errorf("%w: nil formula", ErrInvalidIR)
	}
	if nodeIsNil(f.Root) {
		return fmt.Errorf("%w: nil root", ErrInvalidIR)
	}
	if fn == nil {
		return fmt.Errorf("%w: nil walk callback", ErrInvalidIR)
	}
	state := walkState{
		limits: limits.WithDefaults(),
		active: make(map[Node]bool),
	}
	if err := state.visit(f.Root, 0, fn); err != nil {
		return err
	}
	for i, tag := range f.Tags {
		state.count++
		if state.count > state.limits.MaxNodes {
			return fmt.Errorf("%w: %w: tag %d metadata limit exceeded", ErrInvalidIR, ErrTraversalLimit, i)
		}
		if tag.Body == nil {
			continue
		}
		if err := state.visit(tag.Body, 0, fn); err != nil {
			return fmt.Errorf("tag %d: %w", i, err)
		}
	}
	return nil
}

// Walk visits n and its descendants depth-first in deterministic order. If fn
// returns false, that node's children are skipped. Walk stops on cycles or
// when the default traversal budget is exhausted. Call WalkWithLimits when the
// caller needs the traversal error.
func Walk(n Node, fn func(Node) bool) {
	_ = WalkWithLimits(n, source.Limits{}, fn)
}

// WalkWithLimits visits n with cycle detection and deterministic resource
// limits.
func WalkWithLimits(n Node, limits source.Limits, fn func(Node) bool) error {
	if fn == nil {
		return fmt.Errorf("%w: nil walk callback", ErrInvalidIR)
	}
	state := walkState{
		limits: limits.WithDefaults(),
		active: make(map[Node]bool),
	}
	return state.visit(n, 0, fn)
}

type walkState struct {
	limits source.Limits
	count  int
	active map[Node]bool
}

func (s *walkState) visit(n Node, depth int, fn func(Node) bool) error {
	if nodeIsNil(n) {
		return nil
	}
	if s.active[n] {
		return fmt.Errorf("%w: %w", ErrInvalidIR, ErrCycle)
	}
	s.count++
	if s.count > s.limits.MaxNodes {
		return fmt.Errorf("%w: %w: node limit exceeded", ErrInvalidIR, ErrTraversalLimit)
	}
	if depth > s.limits.MaxNesting {
		return fmt.Errorf("%w: %w: nesting limit exceeded", ErrInvalidIR, ErrTraversalLimit)
	}
	s.active[n] = true
	defer delete(s.active, n)
	if !fn(n) {
		return nil
	}
	for _, child := range Children(n) {
		if err := s.visit(child, depth+1, fn); err != nil {
			return err
		}
	}
	return nil
}

func nodeIsNil(n Node) bool {
	if n == nil {
		return true
	}
	value := reflect.ValueOf(n)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

func compact(nodes ...Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

package ir

import (
	"fmt"

	"github.com/caelyreth/untie/source"
)

// CloneFormula returns an independent copy of f. The input is validated first,
// so cycles, excessive depth, and oversized trees are rejected rather than
// copied.
func CloneFormula(f *Formula, limits ...source.Limits) (*Formula, error) {
	effective := source.DefaultLimits()
	if len(limits) > 0 {
		effective = limits[0]
	}
	if err := Validate(f, effective); err != nil {
		return nil, err
	}
	state := cloneState{
		limits: effective.WithDefaults(),
		active: make(map[Node]bool),
		done:   make(map[Node]Node),
	}
	root, err := state.clone(f.Root, 0)
	if err != nil {
		return nil, err
	}
	out := &Formula{
		Root:   root,
		Tags:   make([]EquationTag, len(f.Tags)),
		Labels: append([]EquationLabel(nil), f.Labels...),
	}
	for i, tag := range f.Tags {
		state.count++
		if state.count > state.limits.MaxNodes {
			return nil, fmt.Errorf("%w: %w: tag %d metadata limit exceeded", ErrInvalidIR, ErrTraversalLimit, i)
		}
		out.Tags[i] = tag
		if tag.Body == nil {
			continue
		}
		out.Tags[i].Body, err = state.clone(tag.Body, 0)
		if err != nil {
			return nil, fmt.Errorf("tag %d: %w", i, err)
		}
	}
	return out, nil
}

// CloneNode returns an independent copy of n without wrapping it in a Formula.
// Callers that need full validation should call Validate or ValidateSource
// first.
func CloneNode(n Node, limits ...source.Limits) (Node, error) {
	effective := source.DefaultLimits()
	if len(limits) > 0 {
		effective = limits[0]
	}
	state := cloneState{
		limits: effective.WithDefaults(),
		active: make(map[Node]bool),
		done:   make(map[Node]Node),
	}
	return state.clone(n, 0)
}

type cloneState struct {
	limits source.Limits
	count  int
	active map[Node]bool
	done   map[Node]Node
}

func (s *cloneState) clone(n Node, depth int) (Node, error) {
	if nodeIsNil(n) {
		return nil, nil
	}
	if s.active[n] {
		return nil, fmt.Errorf("%w: %w", ErrInvalidIR, ErrCycle)
	}
	if out, ok := s.done[n]; ok {
		return out, nil
	}
	s.count++
	if s.count > s.limits.MaxNodes {
		return nil, fmt.Errorf("%w: %w: node limit exceeded", ErrInvalidIR, ErrTraversalLimit)
	}
	if depth > s.limits.MaxNesting {
		return nil, fmt.Errorf("%w: %w: nesting limit exceeded", ErrInvalidIR, ErrTraversalLimit)
	}
	s.active[n] = true
	out, err := s.cloneNode(n, depth)
	delete(s.active, n)
	if err != nil {
		return nil, err
	}
	s.done[n] = out
	return out, nil
}

func (s *cloneState) cloneNode(n Node, depth int) (Node, error) {
	switch n := n.(type) {
	case *Row:
		out := *n
		out.Items = make([]Node, len(n.Items))
		for i, child := range n.Items {
			cloned, err := s.clone(child, depth+1)
			if err != nil {
				return nil, err
			}
			out.Items[i] = cloned
		}
		return &out, nil
	case *Glyph:
		out := *n
		return &out, nil
	case *SizedDelimiter:
		out := *n
		return &out, nil
	case *Text:
		out := *n
		out.Clusters = append([]TextClusterSource(nil), n.Clusters...)
		return &out, nil
	case *Operator:
		out := *n
		return &out, nil
	case *Space:
		out := *n
		return &out, nil
	case *Fraction:
		out := *n
		var err error
		if out.Numerator, err = s.clone(n.Numerator, depth+1); err != nil {
			return nil, err
		}
		if out.Denominator, err = s.clone(n.Denominator, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Radical:
		out := *n
		var err error
		if out.Radicand, err = s.clone(n.Radicand, depth+1); err != nil {
			return nil, err
		}
		if out.Index, err = s.clone(n.Index, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Scripts:
		out := *n
		children := []struct {
			src *Node
			dst *Node
		}{
			{&n.BaseItem, &out.BaseItem},
			{&n.Top, &out.Top},
			{&n.Bottom, &out.Bottom},
			{&n.TopLeft, &out.TopLeft},
			{&n.BottomLeft, &out.BottomLeft},
			{&n.TopRight, &out.TopRight},
			{&n.BottomRight, &out.BottomRight},
		}
		for _, child := range children {
			cloned, err := s.clone(*child.src, depth+1)
			if err != nil {
				return nil, err
			}
			*child.dst = cloned
		}
		return &out, nil
	case *Fenced:
		out := *n
		var err error
		if out.Open, err = s.clone(n.Open, depth+1); err != nil {
			return nil, err
		}
		if out.Close, err = s.clone(n.Close, depth+1); err != nil {
			return nil, err
		}
		out.Body = make([]Node, len(n.Body))
		for i, child := range n.Body {
			if out.Body[i], err = s.clone(child, depth+1); err != nil {
				return nil, err
			}
		}
		return &out, nil
	case *Accent:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		if out.Accent, err = s.clone(n.Accent, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Line:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Cancel:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Table:
		out := *n
		out.Rows = make([]TableRow, len(n.Rows))
		for i, row := range n.Rows {
			out.Rows[i].Cells = make([]Node, len(row.Cells))
			for j, child := range row.Cells {
				cloned, err := s.clone(child, depth+1)
				if err != nil {
					return nil, err
				}
				out.Rows[i].Cells[j] = cloned
			}
		}
		out.Columns = append([]ColumnAlign(nil), n.Columns...)
		out.Horizontal = append([]TableLine(nil), n.Horizontal...)
		out.Vertical = append([]TableLine(nil), n.Vertical...)
		return &out, nil
	case *Multiline:
		out := *n
		out.Rows = make([][]Node, len(n.Rows))
		for i, row := range n.Rows {
			out.Rows[i] = make([]Node, len(row))
			for j, child := range row {
				cloned, err := s.clone(child, depth+1)
				if err != nil {
					return nil, err
				}
				out.Rows[i][j] = cloned
			}
		}
		return &out, nil
	case *Style:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Overlay:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Raise:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Color:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Box:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *Phantom:
		out := *n
		var err error
		if out.Body, err = s.clone(n.Body, depth+1); err != nil {
			return nil, err
		}
		return &out, nil
	case *ErrorNode:
		out := *n
		return &out, nil
	case *Placeholder:
		out := *n
		return &out, nil
	case *Break:
		out := *n
		return &out, nil
	case *AlignMarker:
		out := *n
		return &out, nil
	default:
		return nil, fmt.Errorf("%w: unsupported node %T", ErrInvalidIR, n)
	}
}

package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) alignmentPartCount(node ir.Node) (int, error) {
	row, ok := node.(*ir.Row)
	if !ok {
		return 1, nil
	}
	count := 1
	for i, item := range row.Items {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return 0, err
			}
		}
		if _, ok := item.(*ir.AlignMarker); ok {
			count++
		}
	}
	return count, nil
}

func (s *layoutState) alignmentPartCountInItems(items []ir.Node) (int, error) {
	count := 1
	for i, item := range items {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return 0, err
			}
		}
		if _, ok := item.(*ir.AlignMarker); ok {
			count++
		}
	}
	return count, nil
}

// splitAlignmentParts preserves empty pieces and the original row base so a
// cell's alignment markers become independently measurable sub-columns.
func splitAlignmentParts(node ir.Node) []ir.Node {
	row, ok := node.(*ir.Row)
	if !ok {
		return []ir.Node{node}
	}
	hasMarker := false
	for _, item := range row.Items {
		if _, ok := item.(*ir.AlignMarker); ok {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return []ir.Node{node}
	}

	parts := make([]ir.Node, 0, 2)
	current := make([]ir.Node, 0, len(row.Items))
	flush := func() {
		part := *row
		part.Items = append([]ir.Node(nil), current...)
		parts = append(parts, &part)
		current = make([]ir.Node, 0, len(row.Items))
	}
	for _, item := range row.Items {
		if _, ok := item.(*ir.AlignMarker); ok {
			flush()
			continue
		}
		current = append(current, item)
	}
	flush()
	return parts
}

func (s *layoutState) layoutAlignedParts(node ir.Node, env environment, depth int) ([]measuredPart, error) {
	return s.layoutAlignmentNodes(splitAlignmentParts(node), env, depth)
}

func (s *layoutState) layoutAlignmentNodes(nodes []ir.Node, env environment, depth int) ([]measuredPart, error) {
	transfer := make([]geometry.Em, len(nodes))
	for i := 0; i+1 < len(nodes); i += 2 {
		spacing, err := s.alignmentLeadingSpacing(nodes[i+1], env)
		if err != nil {
			return nil, err
		}
		transfer[i] = spacing
	}
	parts := make([]measuredPart, len(nodes))
	for i, part := range nodes {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		partEnv := env
		if i > 0 && transfer[i-1] > 0 {
			partEnv.SkipLeadingExplicit = true
		}
		fragment, err := s.layoutNode(part, partEnv, depth)
		if err != nil {
			return nil, err
		}
		if transfer[i] > 0 {
			space := s.emptyFragment(env.Style.Size, s.scale(env.Style))
			space.Width = transfer[i]
			if err := fragment.AddGroup(fragment.Width, 0, space); err != nil {
				return nil, err
			}
			fragment.Width, err = add(fragment.Width, transfer[i])
			if err != nil {
				return nil, err
			}
		}
		parts[i] = measuredPart{
			fragment: fragment,
			width:    fragment.Width,
			ascent:   fragment.Ascent,
			descent:  fragment.Descent,
		}
	}
	return parts, nil
}

func (s *layoutState) alignmentLeadingSpacing(node ir.Node, env environment) (geometry.Em, error) {
	first := alignmentFirstNode(node)
	if first == nil {
		return 0, nil
	}
	firstEnv := s.nodeEnvironment(first, env)
	value, ok, err := s.leftSpacing(first, firstEnv)
	if err != nil || !ok {
		return 0, err
	}
	return value, nil
}

func alignmentFirstNode(node ir.Node) ir.Node {
	row, ok := node.(*ir.Row)
	if !ok {
		return node
	}
	for _, item := range row.Items {
		switch item.(type) {
		case *ir.AlignMarker, *ir.Break:
			continue
		}
		return item
	}
	return nil
}

type alignedRun struct {
	parts []measuredPart
}

func (s *layoutState) addAlignedRun(fragment *Fragment, x, baseline geometry.Em, run alignedRun, widths []geometry.Em, alternatorRight bool) error {
	if len(run.parts) == 0 {
		return nil
	}
	if len(widths) <= 1 {
		return fragment.AddGroup(x, baseline, run.parts[0].fragment)
	}
	var point geometry.Em
	previousPoint := geometry.Em(0)
	for i, part := range run.parts {
		if i >= len(widths) {
			break
		}
		point, err := add(point, widths[i])
		if err != nil {
			return err
		}
		partX := previousPoint
		if alternatorRight {
			partX, err = sub(point, part.width)
			if err != nil {
				return err
			}
		}
		partAbsX, err := add(x, partX)
		if err != nil {
			return err
		}
		if err := fragment.AddGroup(partAbsX, baseline, part.fragment); err != nil {
			return err
		}
		previousPoint = point
		alternatorRight = !alternatorRight
	}
	return nil
}

func alignOffset(alignment ir.ColumnAlign, free geometry.Em) geometry.Em {
	if free <= 0 {
		return 0
	}
	switch alignment {
	case ir.AlignRight:
		return free
	case ir.AlignCenter:
		return free / 2
	default:
		return 0
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func validateAlignment(alignment ir.ColumnAlign) error {
	if alignment > ir.AlignRight {
		return fmt.Errorf("%w: invalid column alignment %d", ErrInvalidInput, alignment)
	}
	return nil
}

package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func (s *layoutState) layoutMultiline(node *ir.Multiline, env environment, depth int) (*Fragment, error) {
	if len(node.Rows) == 0 {
		return nil, fmt.Errorf("%w: empty multiline", ErrInvalidInput)
	}
	if len(node.Rows) > s.limits.MaxTableCells {
		return nil, fmt.Errorf("%w: multiline row count %d", ErrLimit, len(node.Rows))
	}

	totalParts := 0
	maxColumns := 0
	for rowIndex, items := range node.Rows {
		if rowIndex%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
		}
		count, err := s.alignmentPartCountInItems(items)
		if err != nil {
			return nil, err
		}
		if count > s.limits.MaxTableCells-totalParts {
			return nil, fmt.Errorf("%w: multiline cell count exceeds %d", ErrLimit, s.limits.MaxTableCells)
		}
		totalParts += count
		maxColumns = maxInt(maxColumns, count)
	}
	if maxColumns > s.limits.MaxTableCells {
		return nil, fmt.Errorf("%w: multiline column count %d", ErrLimit, maxColumns)
	}
	rows := make([][]measuredPart, len(node.Rows))
	rowAscent := make([]geometry.Em, len(node.Rows))
	rowDescent := make([]geometry.Em, len(node.Rows))
	rowWidth := make([]geometry.Em, len(node.Rows))
	for rowIndex, items := range node.Rows {
		parts := splitRowAlignmentParts(items)
		measured, err := s.layoutAlignmentNodes(parts, env, depth)
		if err != nil {
			return nil, err
		}
		rows[rowIndex] = measured
		for _, cell := range measured {
			rowAscent[rowIndex] = max(rowAscent[rowIndex], cell.ascent)
			rowDescent[rowIndex] = max(rowDescent[rowIndex], cell.descent)
			rowWidth[rowIndex], err = add(rowWidth[rowIndex], cell.width)
			if err != nil {
				return nil, err
			}
		}
	}

	columnWidths := make([]geometry.Em, maxColumns)
	for _, row := range rows {
		for columnIndex, cell := range row {
			columnWidths[columnIndex] = max(columnWidths[columnIndex], cell.width)
		}
	}
	totalWidth := geometry.Em(0)
	for _, width := range rowWidth {
		totalWidth = max(totalWidth, width)
	}
	if maxColumns > 1 {
		totalWidth = 0
		for _, width := range columnWidths {
			var err error
			totalWidth, err = add(totalWidth, width)
			if err != nil {
				return nil, err
			}
		}
	}

	var leadingConstant geometry.Em
	if env.Style.Size <= Text {
		leadingConstant = geometry.Em(13 * geometry.EmScale / 20) // Typst's 0.65em leading.
	} else {
		leadingConstant = geometry.Em(geometry.EmScale / 4) // Typst's 0.25em tight leading.
	}
	leading, err := s.metricAt(leadingConstant, env.Style)
	if err != nil {
		return nil, err
	}

	rowTops := make([]geometry.Em, len(rows))
	totalHeight := geometry.Em(0)
	for rowIndex := range rows {
		rowTops[rowIndex] = totalHeight
		totalHeight, err = sum(totalHeight, rowAscent[rowIndex], rowDescent[rowIndex])
		if err != nil {
			return nil, err
		}
		if rowIndex+1 < len(rows) {
			totalHeight, err = add(totalHeight, leading)
			if err != nil {
				return nil, err
			}
		}
	}
	baseline := rowAscent[0]
	descent, err := sub(totalHeight, baseline)
	if err != nil {
		return nil, err
	}
	fragment, err := s.newFragment(Metrics{
		Width:   totalWidth,
		Ascent:  baseline,
		Descent: descent,
	}, node.Class, env.Style.Size, s.scale(env.Style))
	if err != nil {
		return nil, err
	}
	fragment.BaseAscent = baseline
	fragment.BaseDescent = descent
	fragment.TextLike = true

	for rowIndex, row := range rows {
		rowBaseline, err := sum(rowTops[rowIndex], rowAscent[rowIndex], -baseline)
		if err != nil {
			return nil, err
		}
		if maxColumns <= 1 {
			x := geometry.Em(0)
			if node.Centered && len(row) > 0 {
				x = (totalWidth - row[0].width) / 2
			}
			if len(row) > 0 {
				if err := fragment.AddGroup(x, rowBaseline, row[0].fragment); err != nil {
					return nil, err
				}
			}
			continue
		}
		run := alignedRun{
			parts: row,
		}
		if err := s.addAlignedRun(fragment, 0, rowBaseline, run, columnWidths, true); err != nil {
			return nil, err
		}
	}
	return fragment, nil
}

func splitRowAlignmentParts(items []ir.Node) []ir.Node {
	parts := make([]ir.Node, 0, 2)
	current := make([]ir.Node, 0, len(items))
	flush := func() {
		// Parser cell segments are already rows. Preserve their atom metadata
		// instead of hiding it behind an extra ClassNormal row.
		if len(current) == 1 {
			if row, ok := current[0].(*ir.Row); ok {
				parts = append(parts, row)
				current = make([]ir.Node, 0, len(items))
				return
			}
		}
		row := ir.NewRow(source.Detached(), current...)
		row.Class = ir.ClassInner
		parts = append(parts, row)
		current = make([]ir.Node, 0, len(items))
	}
	for _, item := range items {
		if _, ok := item.(*ir.AlignMarker); ok {
			flush()
			continue
		}
		current = append(current, item)
	}
	flush()
	return parts
}

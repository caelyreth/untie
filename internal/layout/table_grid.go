package layout

import (
	"fmt"

	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutTableGrid(
	node *ir.Table,
	env environment,
	rows []measuredTableRow,
	rowGap, columnGap geometry.Em,
	horizontal, vertical []TableLine,
) (*Fragment, error) {
	columnWidths, columnTotals, err := tableColumnWidths(rows, len(node.Columns))
	if err != nil {
		return nil, err
	}
	columnLefts, boundaryX, gridWidth, err := tableColumnPositions(columnTotals, columnGap, vertical)
	if err != nil {
		return nil, err
	}

	totalWidth := gridWidth

	topPad := geometry.Em(0)
	if hasTableBoundary(horizontal, 0) {
		topPad = rowGap
	}
	bottomPad := geometry.Em(0)
	if len(rows) > 0 && hasTableBoundary(horizontal, len(rows)) {
		bottomPad = rowGap
	}
	totalHeight, rowTops, err := tableRowPositions(rows, rowGap, topPad, bottomPad)
	if err != nil {
		return nil, err
	}
	axis, err := s.metricAt(s.fontMetrics(env.Style, env.Variant).AxisHeight, env.Style)
	if err != nil {
		return nil, err
	}
	baseline, err := tableBaseline(totalHeight, axis)
	if err != nil {
		return nil, err
	}
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

	rowBaselines := make([]geometry.Em, len(rows))
	for rowIndex := range rows {
		rowBaseline, err := sum(rowTops[rowIndex], rows[rowIndex].ascent, -baseline)
		if err != nil {
			return nil, err
		}
		rowBaselines[rowIndex] = rowBaseline
	}
	// Typst emits cells column by column. Paint order is part of the display
	// contract, even when the resulting ink is geometrically identical.
	for columnIndex, columnLeft := range columnLefts {
		for rowIndex := range rows {
			if rowIndex%64 == 0 {
				if err := s.ctx.Err(); err != nil {
					return nil, err
				}
			}
			if columnIndex >= len(rows[rowIndex].cells) {
				continue
			}
			cell := rows[rowIndex].cells[columnIndex]
			if len(cell.parts) == 0 {
				continue
			}
			if len(columnWidths[columnIndex]) <= 1 {
				x := columnLeft
				free, err := sub(columnTotals[columnIndex], cell.parts[0].width)
				if err != nil {
					return nil, err
				}
				alignment, err := tableColumnAlignment(node.Columns, columnIndex)
				if err != nil {
					return nil, err
				}
				x += alignOffset(alignment, free)
				if err := fragment.AddGroup(x, rowBaselines[rowIndex], cell.parts[0].fragment); err != nil {
					return nil, err
				}
				continue
			}
			run := alignedRun{
				parts: cell.parts,
			}
			if err := s.addAlignedRun(fragment, columnLeft, rowBaselines[rowIndex], run, columnWidths[columnIndex], true); err != nil {
				return nil, err
			}
		}
	}
	for _, line := range horizontal {
		y, err := horizontalLineCenter(line, rowTops, rows, rowGap, topPad, bottomPad, totalHeight)
		if err != nil {
			return nil, err
		}
		y, err = sub(y, baseline)
		if err != nil {
			return nil, err
		}
		if err := s.addTableRule(fragment, 0, y, totalWidth, line, env.Paint, false, env.Style); err != nil {
			return nil, err
		}
	}
	for _, line := range vertical {
		if line.Position < 0 || line.Position >= len(boundaryX) {
			return nil, fmt.Errorf("%w: vertical line boundary %d", ErrInvalidInput, line.Position)
		}
		if err := s.addTableRule(fragment, boundaryX[line.Position], -baseline, totalHeight, line, env.Paint, true, env.Style); err != nil {
			return nil, err
		}
	}
	return withClass(fragment, node.Class), nil
}

func tableColumnWidths(rows []measuredTableRow, declared int) ([][]geometry.Em, []geometry.Em, error) {
	columns := declared
	for _, row := range rows {
		columns = maxInt(columns, len(row.cells))
	}
	if columns == 0 {
		return nil, nil, nil
	}
	widths := make([][]geometry.Em, columns)
	for _, row := range rows {
		for columnIndex, cell := range row.cells {
			parts := len(cell.parts)
			if parts > len(widths[columnIndex]) {
				grown := make([]geometry.Em, parts)
				copy(grown, widths[columnIndex])
				widths[columnIndex] = grown
			}
			for partIndex, part := range cell.parts {
				if part.width > widths[columnIndex][partIndex] {
					widths[columnIndex][partIndex] = part.width
				}
			}
		}
	}
	totals := make([]geometry.Em, len(widths))
	for columnIndex, parts := range widths {
		for _, width := range parts {
			var err error
			totals[columnIndex], err = add(totals[columnIndex], width)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return widths, totals, nil
}

func tableColumnPositions(columnTotals []geometry.Em, gap geometry.Em, lines []TableLine) ([]geometry.Em, []geometry.Em, geometry.Em, error) {
	lefts := make([]geometry.Em, len(columnTotals))
	boundaries := make([]geometry.Em, len(columnTotals)+1)
	halfGap := gap / 2
	x := geometry.Em(0)
	if hasTableBoundaryLines(lines, 0) {
		boundaries[0] = halfGap
		var err error
		x, err = add(x, gap)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	for columnIndex, width := range columnTotals {
		lefts[columnIndex] = x
		var err error
		x, err = add(x, width)
		if err != nil {
			return nil, nil, 0, err
		}
		if hasTableBoundaryLines(lines, columnIndex+1) {
			boundaries[columnIndex+1], err = add(x, halfGap)
			if err != nil {
				return nil, nil, 0, err
			}
		}
		x, err = add(x, gap)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	total := x
	if len(columnTotals) > 0 && !hasTableBoundaryLines(lines, len(columnTotals)) {
		var err error
		total, err = sub(total, gap)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	return lefts, boundaries, total, nil
}

func tableRowPositions(rows []measuredTableRow, gap, topPad, bottomPad geometry.Em) (geometry.Em, []geometry.Em, error) {
	tops := make([]geometry.Em, len(rows))
	height := topPad
	for rowIndex, row := range rows {
		tops[rowIndex] = height
		var err error
		height, err = sum(height, row.ascent, row.descent)
		if err != nil {
			return 0, nil, err
		}
		if rowIndex+1 < len(rows) {
			height, err = add(height, gap)
			if err != nil {
				return 0, nil, err
			}
		}
	}
	height, err := add(height, bottomPad)
	if err != nil {
		return 0, nil, err
	}
	return height, tops, nil
}

func tableBaseline(height, axis geometry.Em) (geometry.Em, error) {
	if height == 0 {
		return 0, nil
	}
	half := height / 2
	baseline, err := add(half, axis)
	if err != nil {
		return 0, err
	}
	return clamp(baseline, 0, height), nil
}

func tableColumnAlignment(columns []ir.ColumnAlign, index int) (ir.ColumnAlign, error) {
	if index < len(columns) {
		alignment := columns[index]
		if err := validateAlignment(alignment); err != nil {
			return ir.AlignCenter, err
		}
		return alignment, nil
	}
	return ir.AlignCenter, nil
}

func horizontalLineCenter(
	line TableLine,
	rowTops []geometry.Em,
	rows []measuredTableRow,
	rowGap, topPad, bottomPad, totalHeight geometry.Em,
) (geometry.Em, error) {
	switch line.Position {
	case 0:
		return topPad / 2, nil
	case len(rows):
		if bottomPad > 0 {
			return sub(totalHeight, bottomPad/2)
		}
		return totalHeight, nil
	default:
		if line.Position < 0 || line.Position > len(rows) {
			return 0, fmt.Errorf("%w: horizontal line boundary %d", ErrInvalidInput, line.Position)
		}
		return sum(rowTops[line.Position], -(rowGap / 2))
	}
}

func (s *layoutState) addTableRule(fragment *Fragment, x, y, length geometry.Em, line TableLine, paint ir.Paint, vertical bool, style Style) error {
	dash, gap, segments := length, geometry.Em(0), 1
	if line.Dashed {
		var err error
		dash, err = s.metricAt(geometry.Em(geometry.EmScale/5), style)
		if err != nil {
			return err
		}
		minimumDash, err := mulDiv(line.Thickness, 3, 1)
		if err != nil {
			return err
		}
		dash = max(dash, minimumDash)
		gap = dash / 2
		segments, err = ruleSegmentCount(length, dash, gap)
		if err != nil {
			return err
		}
	}
	if length <= 0 {
		segments = 0
	}
	if segments > s.limits.MaxOutputItems {
		return fmt.Errorf("%w: table rule segment count %d", ErrLimit, segments)
	}
	for stroke := 0; stroke < line.Count; stroke++ {
		if err := s.reservePlannedOutput(segments); err != nil {
			return err
		}
		// Count is a multiplicity for stacked rule strokes at the same boundary.
		offset, err := mulDiv(line.Thickness, int64(2*stroke-(line.Count-1)), 1)
		if err != nil {
			return err
		}
		for position := geometry.Em(0); position < length; {
			end, err := add(position, dash)
			if err != nil {
				return err
			}
			end = min(end, length)
			segmentLength := end - position
			if vertical {
				center, err := add(x, offset)
				if err != nil {
					return err
				}
				left, err := sub(center, line.Thickness/2)
				if err != nil {
					return err
				}
				top, err := add(y, position)
				if err != nil {
					return err
				}
				if err := fragment.AddRule(left, top, segmentLength, line.Thickness, paint, true); err != nil {
					return err
				}
			} else {
				center, err := add(y, offset)
				if err != nil {
					return err
				}
				top, err := sub(center, line.Thickness/2)
				if err != nil {
					return err
				}
				left, err := add(x, position)
				if err != nil {
					return err
				}
				if err := fragment.AddRule(left, top, segmentLength, line.Thickness, paint, false); err != nil {
					return err
				}
			}
			if end == length {
				break
			}
			position, err = add(end, gap)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ruleSegmentCount(length, dash, gap geometry.Em) (int, error) {
	if length <= 0 || dash <= 0 || gap < 0 {
		return 0, nil
	}
	step, err := add(dash, gap)
	if err != nil || step <= 0 {
		return 0, fmt.Errorf("%w: invalid dash pattern", ErrInvalidInput)
	}
	count := length / step
	if length%step != 0 {
		count++
	}
	if count > geometry.Em(1<<30) {
		return 0, fmt.Errorf("%w: table rule segment count overflow", ErrLimit)
	}
	return int(count), nil
}

func hasTableBoundary(lines []TableLine, position int) bool {
	return hasTableBoundaryLines(lines, position)
}

func hasTableBoundaryLines(lines []TableLine, position int) bool {
	for _, line := range lines {
		if line.Position == position && line.Count > 0 {
			return true
		}
	}
	return false
}

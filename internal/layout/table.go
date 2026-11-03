package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutTable(node *ir.Table, env environment, depth int) (*Fragment, error) {
	shape, err := s.tableShape(node)
	if err != nil {
		return nil, err
	}
	return s.layoutTableBuiltIn(node, env, depth, shape)
}

type tableShape struct {
	rows        int
	columns     int
	lineStrokes int
}

func (s *layoutState) tableShape(node *ir.Table) (tableShape, error) {
	shape := tableShape{rows: len(node.Rows)}
	// Source Limits has one table budget rather than separate row and column
	// fields, so rows, columns, cells, and alignment sub-columns share it.
	if shape.rows > s.limits.MaxTableCells {
		return tableShape{}, fmt.Errorf("%w: table row count %d", ErrLimit, shape.rows)
	}
	if len(node.Columns) > s.limits.MaxTableCells {
		return tableShape{}, fmt.Errorf("%w: table column count %d", ErrLimit, len(node.Columns))
	}
	shape.columns = len(node.Columns)
	cells := 0
	parts := 0
	for rowIndex, row := range node.Rows {
		if rowIndex%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return tableShape{}, err
			}
		}
		if len(row.Cells) > s.limits.MaxTableCells {
			return tableShape{}, fmt.Errorf("%w: table cell count %d", ErrLimit, len(row.Cells))
		}
		shape.columns = maxInt(shape.columns, len(row.Cells))
		if len(row.Cells) > s.limits.MaxTableCells-cells {
			return tableShape{}, fmt.Errorf("%w: table cell count exceeds %d", ErrLimit, s.limits.MaxTableCells)
		}
		cells += len(row.Cells)
		for _, cell := range row.Cells {
			count, err := s.alignmentPartCount(cell)
			if err != nil {
				return tableShape{}, err
			}
			if count > s.limits.MaxTableCells-parts {
				return tableShape{}, fmt.Errorf("%w: table sub-column count exceeds %d", ErrLimit, s.limits.MaxTableCells)
			}
			parts += count
		}
	}
	if shape.columns > s.limits.MaxTableCells {
		return tableShape{}, fmt.Errorf("%w: table column count %d", ErrLimit, shape.columns)
	}
	strokes := 0
	for i, line := range node.Horizontal {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return tableShape{}, err
			}
		}
		if line.Count > s.limits.MaxTableCells-strokes {
			return tableShape{}, fmt.Errorf("%w: table rule count exceeds %d", ErrLimit, s.limits.MaxTableCells)
		}
		strokes += line.Count
	}
	for i, line := range node.Vertical {
		if i%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return tableShape{}, err
			}
		}
		if line.Count > s.limits.MaxTableCells-strokes {
			return tableShape{}, fmt.Errorf("%w: table rule count exceeds %d", ErrLimit, s.limits.MaxTableCells)
		}
		strokes += line.Count
	}
	shape.lineStrokes = strokes
	if strokes > s.limits.MaxOutputItems {
		return tableShape{}, fmt.Errorf("%w: table output item count exceeds %d", ErrLimit, s.limits.MaxOutputItems)
	}
	return shape, nil
}

type measuredPart struct {
	fragment *Fragment
	width    geometry.Em
	ascent   geometry.Em
	descent  geometry.Em
}

type measuredCell struct {
	parts   []measuredPart
	ascent  geometry.Em
	descent geometry.Em
}

type measuredTableRow struct {
	cells   []measuredCell
	ascent  geometry.Em
	descent geometry.Em
}

func (s *layoutState) layoutTableBuiltIn(node *ir.Table, env environment, depth int, shape tableShape) (*Fragment, error) {
	if shape.rows == 0 || shape.columns == 0 {
		fragment := s.emptyFragment(env.Style.Size, s.scale(env.Style))
		fragment.Class = node.Class
		return fragment, nil
	}
	cellEnv := env
	cellEnv.Style = env.Style.Denominator()
	rows := make([]measuredTableRow, len(node.Rows))
	for rowIndex, row := range node.Rows {
		if rowIndex%64 == 0 {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
		}
		rows[rowIndex].cells = make([]measuredCell, len(row.Cells))
		for columnIndex, cell := range row.Cells {
			if err := s.ctx.Err(); err != nil {
				return nil, err
			}
			parts, err := s.layoutAlignedParts(cell, cellEnv, depth)
			if err != nil {
				return nil, err
			}
			for _, part := range parts {
				setOwnHitKind(part.fragment, display.HitTableCell)
			}
			measured := measuredCell{parts: parts}
			for _, part := range parts {
				measured.ascent = max(measured.ascent, part.ascent)
				measured.descent = max(measured.descent, part.descent)
			}
			rows[rowIndex].cells[columnIndex] = measured
			rows[rowIndex].ascent = max(rows[rowIndex].ascent, measured.ascent)
			rows[rowIndex].descent = max(rows[rowIndex].descent, measured.descent)
		}
	}
	paren, err := s.layoutClassGlyphText("(", ir.ClassOpening, cellEnv, nil)
	if err != nil {
		return nil, fmt.Errorf("layout table baseline parenthesis: %w", err)
	}
	for i := range rows {
		rows[i].ascent = max(rows[i].ascent, paren.Ascent)
		rows[i].descent = max(rows[i].descent, paren.Descent)
	}

	metrics := s.fontMetrics(env.Style, env.Variant)
	rowGap, err := s.length(node.RowGap, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	columnGap, err := s.length(node.ColumnGap, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	horizontal, err := s.tableLines(node.Horizontal, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	vertical, err := s.tableLines(node.Vertical, env.Style, metrics)
	if err != nil {
		return nil, err
	}
	return s.layoutTableGrid(node, env, rows, rowGap, columnGap, horizontal, vertical)
}

func (s *layoutState) reservePlannedOutput(count int) error {
	// Only deterministic rule items are reserved here. Cell/glyph output is
	// checked against the same limit during flattening, after exact expansion.
	if count < 0 || count > s.limits.MaxOutputItems-s.plannedOutput {
		return fmt.Errorf("%w: output item count would exceed %d", ErrLimit, s.limits.MaxOutputItems)
	}
	s.plannedOutput += count
	return nil
}

func (s *layoutState) tableLines(lines []ir.TableLine, style Style, metrics MathMetrics) ([]TableLine, error) {
	out := make([]TableLine, len(lines))
	for i, line := range lines {
		thickness, err := s.orMetric(line.Thickness, geometry.Em(geometry.EmScale/20), style, metrics)
		if err != nil {
			return nil, err
		}
		out[i] = TableLine{
			Position:  line.Position,
			Count:     line.Count,
			Dashed:    line.Dashed,
			Thickness: thickness,
		}
	}
	return out, nil
}

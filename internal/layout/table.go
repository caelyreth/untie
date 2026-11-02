package layout

import (
	"fmt"

	"github.com/caelyreth/untie/ir"
)

func (s *layoutState) layoutTable(node *ir.Table, env environment, depth int) (*Fragment, error) {
	if s.renderer.tables == nil {
		return nil, fmt.Errorf("%w: table layout extension is not configured", ErrUnsupported)
	}
	cellEnv := env
	rows := make([]TableRow, len(node.Rows))
	for rowIndex, row := range node.Rows {
		rows[rowIndex].Cells = make([]Fragment, len(row.Cells))
		for columnIndex, cell := range row.Cells {
			fragment, err := s.layoutNode(cell, cellEnv, depth)
			if err != nil {
				return nil, err
			}
			rows[rowIndex].Cells[columnIndex] = *fragment
		}
		if row.Tag != nil {
			tag, err := s.layoutNode(row.Tag, cellEnv, depth)
			if err != nil {
				return nil, err
			}
			rows[rowIndex].Tag = tag
		}
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
	fragment, ok := s.renderer.tables.LayoutTable(TableRequest{
		Style:      env.Style,
		Rows:       rows,
		Columns:    append([]ir.ColumnAlign(nil), node.Columns...),
		RowGap:     rowGap,
		ColumnGap:  columnGap,
		Horizontal: horizontal,
		Vertical:   vertical,
	})
	if !ok {
		return nil, fmt.Errorf("%w: table extension rejected input", ErrUnsupported)
	}
	return withClass(&fragment, node.Class), nil
}

func (s *layoutState) tableLines(lines []ir.TableLine, style Style, metrics MathMetrics) ([]TableLine, error) {
	out := make([]TableLine, len(lines))
	for i, line := range lines {
		thickness, err := s.orMetric(line.Thickness, metrics.FractionRuleThickness, style, metrics)
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

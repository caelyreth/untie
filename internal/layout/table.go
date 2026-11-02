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
	cellEnv.Style = env.Style.Denominator()
	rows := make([][]Fragment, len(node.Rows))
	for rowIndex, row := range node.Rows {
		rows[rowIndex] = make([]Fragment, len(row.Cells))
		for columnIndex, cell := range row.Cells {
			fragment, err := s.layoutNode(cell, cellEnv, depth)
			if err != nil {
				return nil, err
			}
			rows[rowIndex][columnIndex] = *fragment
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
	fragment, ok := s.renderer.tables.LayoutTable(TableRequest{
		Style:     env.Style,
		Rows:      rows,
		Columns:   append([]ir.ColumnAlign(nil), node.Columns...),
		RowGap:    rowGap,
		ColumnGap: columnGap,
	})
	if !ok {
		return nil, fmt.Errorf("%w: table extension rejected input", ErrUnsupported)
	}
	return withClass(&fragment, node.Class), nil
}

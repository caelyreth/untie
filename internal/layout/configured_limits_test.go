package layout

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestPreparedStatsMatchFullConfiguredValidation(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 6)
	scripts := ir.NewScripts(span, ir.NewGlyph(span, "x"))
	scripts.Top = ir.NewGlyph(span, "2")
	boxed := ir.NewBox(span, ir.NewText(span, "hello"))
	boxed.Padding[0] = ir.RawLength(ir.LengthEm, 2_000_000)
	table := ir.NewTable(span,
		ir.TableRow{Cells: []ir.Node{ir.NewGlyph(span, "a")}},
		ir.TableRow{Cells: []ir.Node{ir.NewGlyph(span, "b"), ir.NewGlyph(span, "c")}},
	)
	formula := &ir.Formula{
		Root: ir.NewRow(span, scripts, boxed, table),
		Tags: []ir.EquationTag{{
			Body:   ir.NewGlyph(span, "1"),
			Span:   span,
			Origin: span,
			Manual: true,
			Line:   ir.FormulaTagLine,
		}},
	}
	stats, err := ir.ValidateWithStats(formula, source.DefaultLimits())
	if err != nil {
		t.Fatalf("ValidateWithStats() error = %v", err)
	}

	tests := []struct {
		name   string
		limits source.Limits
	}{
		{name: "within", limits: source.DefaultLimits()},
		{name: "nodes", limits: source.Limits{MaxNodes: stats.Nodes - 1}},
		{name: "nesting", limits: source.Limits{MaxNesting: stats.MaxDepth - 1}},
		{name: "text", limits: source.Limits{MaxTextBytes: stats.MaxTextBytes - 1}},
		{name: "dimension", limits: source.Limits{MaxDimension: 1}},
		{name: "table cells", limits: source.Limits{MaxTableCells: stats.MaxTableCells - 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fullErr := validateConfiguredLimits(context.Background(), formula, tc.limits, nil)
			fastErr := validateConfiguredLimits(context.Background(), formula, tc.limits, &stats)
			if (fullErr == nil) != (fastErr == nil) {
				t.Fatalf("full error = %v, stats error = %v", fullErr, fastErr)
			}
			if fullErr != nil && fullErr.Error() != fastErr.Error() {
				t.Fatalf("full error = %q, stats error = %q", fullErr, fastErr)
			}
		})
	}
}

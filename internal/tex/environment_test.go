package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestMatrixEnvironment(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\begin{matrix}a&b\\c&d\end{matrix}`, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
	table := requireKind[*ir.Table](t, rowItems(t, result.Root)[0])
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	for i, row := range table.Rows {
		if len(row.Cells) != 2 {
			t.Errorf("row %d cells = %d, want 2", i, len(row.Cells))
		}
	}
}

func TestFencedMatrixAndArray(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\begin{pmatrix}1&2\\3&4\end{pmatrix}`, Options{})
	fenced := requireKind[*ir.Fenced](t, rowItems(t, result.Root)[0])
	if fenced.Open == nil || fenced.Close == nil {
		t.Fatal("missing matrix fences")
	}

	result = parseTest(t, `\begin{array}{|l|c|r|}a&b&c\end{array}`, Options{})
	table := requireKind[*ir.Table](t, rowItems(t, result.Root)[0])
	if len(table.Columns) != 3 || len(table.Vertical) != 4 {
		t.Errorf("columns = %d, vertical lines = %d", len(table.Columns), len(table.Vertical))
	}
}

func TestAlignedEnvironmentLowersToAlignedTable(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\begin{aligned}x&=1\\y&=2\end{aligned}`, Options{})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", result.Diagnostics)
	}
	table := requireKind[*ir.Table](t, rowItems(t, result.Root)[0])
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if len(table.Columns) != 2 || table.Columns[0] != ir.AlignRight || table.Columns[1] != ir.AlignLeft {
		t.Errorf("columns = %v, want right/left", table.Columns)
	}
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			t.Errorf("row has %d cells, want 2", len(row.Cells))
		}
	}
}

func TestEnvironmentRecovery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		code   source.Code
	}{
		{`\begin{unknown}x\end{unknown}`, source.CodeInvalidEnvironment},
		{`\begin{matrix}x`, source.CodeInvalidEnvironment},
		{`\begin{matrix}x\end{pmatrix}`, source.CodeInvalidEnvironment},
		{`\begin{array}a\end{array}`, source.CodeMissingArgument},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tc.source, Options{})
			if !hasDiagnostic(result.Diagnostics, tc.code) {
				t.Fatalf("diagnostics = %v, want %s", diagnosticCodes(result.Diagnostics), tc.code)
			}
		})
	}
}

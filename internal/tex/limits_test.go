package tex

import (
	"strconv"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestNodeLimitRecoveryStaysWithinBudget(t *testing.T) {
	t.Parallel()

	sources := []string{
		"x+y",
		`\frac{a+b}{c+d}`,
		`\left.\frac{x}{y}\right.`,
		`\begin{matrix}a&b\\c&d\end{matrix}`,
		`\begin{aligned}x&=1\\y&=2\end{aligned}`,
	}
	for _, src := range sources {
		baseline := countIRNodes(Parse(src, ir.Display, Options{}).Root)
		for maxNodes := 0; maxNodes <= 8; maxNodes++ {
			limits := source.DefaultLimits()
			limits.MaxNodes = maxNodes
			t.Run(src+"/"+strconv.Itoa(maxNodes), func(t *testing.T) {
				result := Parse(src, ir.Display, Options{Limits: limits})
				if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
					t.Fatalf("invalid recovery IR at MaxNodes=%d: %v", maxNodes, err)
				}
				count := countIRNodes(result.Root)
				effective := limits.WithDefaults().MaxNodes
				if count > effective {
					t.Fatalf("node count = %d, limit = %d", count, effective)
				}
				if maxNodes > 0 && count > maxNodes {
					t.Fatalf("node count = %d exceeds explicit limit %d", count, maxNodes)
				}
				if maxNodes > 0 && baseline > maxNodes && !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
					t.Fatalf("MaxNodes=%d did not report limit: %v", maxNodes, diagnosticCodes(result.Diagnostics))
				}
			})
		}
	}
}

func TestLoweringNestingBudgetStaysValid(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxNesting = 32
	src := strings.Repeat("^", 96) + "x"
	result := Parse(src, ir.Inline, Options{Limits: limits})
	if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
		t.Fatalf("invalid deep-script recovery IR: %v", err)
	}
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want nesting limit", diagnosticCodes(result.Diagnostics))
	}
}

func TestTableCellLimitTruncatesBeforeLowering(t *testing.T) {
	t.Parallel()

	const src = `\begin{matrix}a&b\\c&d\end{matrix}`
	for _, maxCells := range []int{0, 1, 2, 3, 4, 5} {
		limits := source.DefaultLimits()
		limits.MaxTableCells = maxCells
		t.Run(strconv.Itoa(maxCells), func(t *testing.T) {
			result := Parse(src, ir.Display, Options{Limits: limits})
			if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
				t.Fatalf("invalid IR at MaxTableCells=%d: %v", maxCells, err)
			}
			table := firstTable(result.Root)
			if table == nil {
				t.Fatal("table was unexpectedly replaced")
			}
			got := tableCellCount(table)
			effective := limits.WithDefaults().MaxTableCells
			want := min(4, effective)
			if got != want {
				t.Fatalf("cells = %d, want %d", got, want)
			}
			shouldLimit := maxCells > 0 && maxCells < 4
			if shouldLimit != hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
				t.Fatalf("MaxTableCells=%d diagnostics = %v", maxCells, diagnosticCodes(result.Diagnostics))
			}
		})
	}
}

func countIRNodes(root ir.Node) int {
	count := 0
	ir.Walk(root, func(ir.Node) bool {
		count++
		return true
	})
	return count
}

func firstTable(root ir.Node) *ir.Table {
	var found *ir.Table
	ir.Walk(root, func(node ir.Node) bool {
		if table, ok := node.(*ir.Table); ok && found == nil {
			found = table
		}
		return found == nil
	})
	return found
}

func tableCellCount(table *ir.Table) int {
	count := 0
	for _, row := range table.Rows {
		count += len(row.Cells)
	}
	return count
}

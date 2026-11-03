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

func TestLoweringDepthGuardChargesNodeBudget(t *testing.T) {
	t.Parallel()

	limits := source.DefaultLimits()
	limits.MaxNesting = 1
	limits.MaxNodes = 4
	result := Parse("abcdefghij", ir.Inline, Options{Limits: limits})

	if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
		t.Fatalf("invalid depth-limit recovery IR: %v", err)
	}
	if count := countIRNodes(result.Root); count > limits.MaxNodes {
		t.Fatalf("node count = %d, limit = %d", count, limits.MaxNodes)
	}
	limitDiagnostics := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == source.CodeLimitExceeded {
			limitDiagnostics++
		}
	}
	if limitDiagnostics != 1 {
		t.Fatalf("limit diagnostics = %d (%v), want exactly one", limitDiagnostics, diagnosticCodes(result.Diagnostics))
	}
}

func TestLoweringLimitBoundariesStayValid(t *testing.T) {
	t.Parallel()

	for maxNesting := 1; maxNesting <= 4; maxNesting++ {
		for maxNodes := 1; maxNodes <= 12; maxNodes++ {
			limits := source.DefaultLimits()
			limits.MaxNesting = maxNesting
			limits.MaxNodes = maxNodes
			t.Run("n"+strconv.Itoa(maxNesting)+"/nodes"+strconv.Itoa(maxNodes), func(t *testing.T) {
				result := Parse("abcdefghij", ir.Inline, Options{Limits: limits})
				if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
					t.Fatalf("invalid recovery IR: %v", err)
				}
				if count := countIRNodes(result.Root); count > maxNodes {
					t.Fatalf("node count = %d exceeds limit %d", count, maxNodes)
				}
				limitDiagnostics := 0
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Code == source.CodeLimitExceeded {
						limitDiagnostics++
					}
				}
				if limitDiagnostics > 1 {
					t.Fatalf("duplicate limit diagnostics: %v", diagnosticCodes(result.Diagnostics))
				}
			})
		}
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

func TestTableCellLimitBoundsParsingWork(t *testing.T) {
	t.Parallel()

	const (
		maxCells  = 8
		tailCells = 8_192
	)
	var src strings.Builder
	src.Grow(64 * 1024)
	src.WriteString(`\begin{matrix}`)
	for i := 0; i < maxCells+tailCells; i++ {
		if i != 0 {
			src.WriteByte('&')
		}
		switch i {
		case maxCells:
			src.WriteString(`\begin{pmatrix}{x}&{y}\end{pmatrix}`)
		case maxCells + 1:
			src.WriteString(`{\unknowncommand}`)
		default:
			src.WriteString(`{x}`)
		}
	}
	src.WriteString(`\end{matrix}+z`)

	limits := source.DefaultLimits()
	limits.MaxTableCells = maxCells
	p := newParser(src.String(), ir.Display, nil, limits)
	root, _, _, _ := p.parse()

	if p.parsedTableCells != maxCells {
		t.Fatalf("parsed cells = %d, want %d", p.parsedTableCells, maxCells)
	}
	if err := ir.Validate(&ir.Formula{Root: root}, limits); err != nil {
		t.Fatalf("invalid IR: %v", err)
	}
	table := firstTable(root)
	if table == nil {
		t.Fatal("table was unexpectedly removed")
	}
	if got := tableCellCount(table); got != maxCells {
		t.Fatalf("table cells = %d, want %d", got, maxCells)
	}
	if hasDiagnostic(p.diagnostics, source.CodeUnknownCommand) {
		t.Fatalf("discarded tail was semantically parsed: %v", diagnosticCodes(p.diagnostics))
	}
	items := rowItems(t, root)
	if len(items) < 3 {
		t.Fatalf("root items = %d, want trailing +z", len(items))
	}
	if glyph := requireKind[*ir.Glyph](t, items[len(items)-1]); glyph.Text != "z" {
		t.Fatalf("trailing glyph = %q, want z", glyph.Text)
	}
}

func TestSubstackLimitBoundsParsingWork(t *testing.T) {
	t.Parallel()

	const tailGroups = 4_096
	var src strings.Builder
	src.Grow(64 * 1024)
	src.WriteString(`\substack{a\\`)
	for i := 0; i < tailGroups; i++ {
		src.WriteString(`{\unknowncommand&x}`)
	}
	src.WriteString(`}+z`)

	limits := source.DefaultLimits()
	limits.MaxTableCells = 1
	p := newParser(src.String(), ir.Display, nil, limits)
	root, _, _, _ := p.parse()

	if p.parsedTableCells != limits.MaxTableCells {
		t.Fatalf("parsed cells = %d, want %d", p.parsedTableCells, limits.MaxTableCells)
	}
	if err := ir.Validate(&ir.Formula{Root: root}, limits); err != nil {
		t.Fatalf("invalid IR: %v", err)
	}
	if hasDiagnostic(p.diagnostics, source.CodeUnknownCommand) {
		t.Fatalf("discarded tail was semantically parsed: %v", diagnosticCodes(p.diagnostics))
	}
	items := rowItems(t, root)
	if len(items) < 3 {
		t.Fatalf("root items = %d, want trailing +z", len(items))
	}
	if glyph := requireKind[*ir.Glyph](t, items[len(items)-1]); glyph.Text != "z" {
		t.Fatalf("trailing glyph = %q, want z", glyph.Text)
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

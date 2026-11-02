package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestRecoveryRangesStayWithinSource(t *testing.T) {
	t.Parallel()

	sources := []string{
		"{a",
		`\left(x`,
		`\begin{matrix}x`,
		`\def\foo#1{#1}\foo{`,
		`\def\foo#1{#1}\foo`,
		`\sqrt[`,
		`\text{unterminated`,
	}
	for _, src := range sources {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := Parse(src, ir.Display, Options{})
			for _, diagnostic := range result.Diagnostics {
				assertRangeWithin(t, diagnostic.Range, len(src), "diagnostic")
			}
			ir.Walk(result.Root, func(node ir.Node) bool {
				assertRangeWithin(t, node.NodeBase().Span, len(src), "node span")
				assertRangeWithin(t, node.NodeBase().Origin, len(src), "node origin")
				return true
			})
		})
	}
}

func TestEOFCaretRangeRemainsEmpty(t *testing.T) {
	t.Parallel()

	const src = `\left(x`
	result := Parse(src, ir.Display, Options{})
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Range == source.NewRange(len(src), len(src)) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no EOF caret diagnostic in %v", result.Diagnostics)
	}
}

func assertRangeWithin(t *testing.T, r source.Range, sourceLen int, label string) {
	t.Helper()
	if r.IsDetached() {
		return
	}
	if r.Start < 0 || r.End < r.Start || r.End > sourceLen {
		t.Fatalf("%s range %v is outside source length %d", label, r, sourceLen)
	}
}

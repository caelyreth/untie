package crosscheck

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/internal/quality/fixture"
	"github.com/caelyreth/untie/ir"
)

func BenchmarkParseBothDialects(b *testing.B) {
	root, _ := benchmarkPaths(b)
	corpus, err := LoadPairedCorpus(root)
	if err != nil {
		b.Fatal(err)
	}
	for _, dialect := range []string{"tex", "typst"} {
		b.Run(dialect, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for _, testCase := range corpus.Cases {
					if dialect == "tex" {
						if testCase.TeX == "" {
							continue
						}
						_ = untie.ParseTeX(testCase.TeX, caseMode(testCase), untie.ParseOptions{})
					} else {
						_ = untie.ParseTypst(testCase.Source, caseMode(testCase), untie.ParseOptions{})
					}
				}
			}
		})
	}
}

func BenchmarkLayoutBothDialects(b *testing.B) {
	root, _ := benchmarkPaths(b)
	harness := benchmarkHarness(b, root)
	testCase := mustCase(b, root, "quadratic")
	mode := caseMode(testCase)
	typstRoot := requireValidRootBenchmark(b, untie.ParseTypst(testCase.Source, mode, untie.ParseOptions{}))
	texRoot := requireValidRootBenchmark(b, untie.ParseTeX(testCase.TeX, mode, untie.ParseOptions{}))
	for _, side := range []struct {
		name string
		root *ir.Formula
	}{
		{"typst", typstRoot},
		{"tex", texRoot},
	} {
		b.Run(side.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := harness.layout(b.Context(), side.root.Root, mode, 16); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkPaths(tb testing.TB) (string, string) {
	tb.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	pkg := filepath.Dir(file)
	return filepath.Join(pkg, "..", "..", "..", "testdata", "upstream"),
		filepath.Join(pkg, "testdata", "paired-baseline.json")
}

func benchmarkHarness(tb testing.TB, root string) *layoutHarness {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join(root, "fonts", "NewCMMath-Book.otf"))
	if err != nil {
		tb.Fatal(err)
	}
	harness, err := newLayoutHarness(data)
	if err != nil {
		tb.Fatal(err)
	}
	return harness
}

func mustCase(tb testing.TB, root, id string) fixture.Case {
	tb.Helper()
	corpus, err := LoadPairedCorpus(root)
	if err != nil {
		tb.Fatal(err)
	}
	for _, testCase := range corpus.Cases {
		if testCase.ID == id {
			return testCase
		}
	}
	tb.Fatalf("missing case %q", id)
	return fixture.Case{}
}

func caseMode(testCase fixture.Case) ir.Mode {
	if testCase.Display {
		return ir.Display
	}
	return ir.Inline
}

func requireValidRootBenchmark(b *testing.B, expr *untie.Expr) *ir.Formula {
	b.Helper()
	root, err := expr.RootSnapshot()
	if err != nil {
		b.Fatal(err)
	}
	return root
}

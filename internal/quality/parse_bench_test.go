package quality_test

import (
	"path/filepath"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/internal/quality/fixture"
	"github.com/caelyreth/untie/source"
)

type parseBenchCase struct {
	source  string
	dialect untie.Dialect
	mode    untie.Mode
}

type parseBenchCorpus struct {
	name         string
	cases        []parseBenchCase
	sourceBytes  int64
	diagnostics  int64
	errorCount   int64
	wellFormed   int64
	malformed    int64
	dialectLabel string
}

var benchmarkParsedExpr *untie.Expr

func BenchmarkParseTrackedCorpora(b *testing.B) {
	corpora := loadTrackedParseBenchmarks(b)
	for _, corpus := range corpora {
		b.Run(corpus.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(corpus.sourceBytes)
			b.ResetTimer()
			for b.Loop() {
				for _, testCase := range corpus.cases {
					expr := untie.Parse(testCase.source, testCase.dialect, testCase.mode, untie.ParseOptions{})
					benchmarkParsedExpr = expr
				}
			}
			reportParseBenchmarkMetrics(b, corpus)
		})
	}
}

func reportParseBenchmarkMetrics(b *testing.B, corpus parseBenchCorpus) {
	elapsed := b.Elapsed().Seconds()
	if elapsed <= 0 {
		return
	}
	cases := int64(len(corpus.cases))
	b.ReportMetric(float64(cases)/elapsed, "cases/s")
	b.ReportMetric(float64(corpus.sourceBytes)/elapsed, "source-B/s")
	b.ReportMetric(float64(cases), "cases/corpus")
	b.ReportMetric(float64(corpus.sourceBytes), "source-B/corpus")
	b.ReportMetric(float64(corpus.diagnostics), "diagnostics/corpus")
	b.ReportMetric(float64(corpus.errorCount), "errors/corpus")
	b.ReportMetric(float64(corpus.wellFormed), "well-formed/corpus")
	b.ReportMetric(float64(corpus.malformed), "malformed/corpus")
}

func loadTrackedParseBenchmarks(b *testing.B) []parseBenchCorpus {
	b.Helper()
	root := filepath.Join(repositoryRoot(b), "testdata", "upstream")
	manifest, err := fixture.LoadCorpusManifest(filepath.Join(root, "manifests", "corpora.json"))
	if err != nil {
		b.Fatal(err)
	}
	corpora := make([]parseBenchCorpus, 0, len(manifest.Corpora))
	for _, spec := range manifest.Corpora {
		corpus, err := fixture.LoadCorpus(root, spec)
		if err != nil {
			b.Fatal(err)
		}
		benchCorpus := parseBenchCorpus{
			name:         spec.Name,
			cases:        make([]parseBenchCase, len(corpus.Cases)),
			dialectLabel: trackedCorpusDialect(spec.Name),
		}
		for i, testCase := range corpus.Cases {
			mode := untie.Inline
			if testCase.Display {
				mode = untie.Display
			}
			dialect := untie.TeX
			if benchCorpus.dialectLabel == "typst" {
				dialect = untie.Typst
			}
			benchCorpus.cases[i] = parseBenchCase{
				source:  testCase.Source,
				dialect: dialect,
				mode:    mode,
			}
			benchCorpus.sourceBytes += int64(len(testCase.Source))
			expr := untie.Parse(testCase.Source, dialect, mode, untie.ParseOptions{})
			diagnostics := expr.Diagnostics()
			benchCorpus.diagnostics += int64(len(diagnostics))
			hasError := false
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == source.SeverityError {
					hasError = true
					benchCorpus.errorCount++
				}
			}
			if hasError {
				benchCorpus.malformed++
			} else {
				benchCorpus.wellFormed++
			}
		}
		corpora = append(corpora, benchCorpus)
	}
	return corpora
}

func trackedCorpusDialect(name string) string {
	switch name {
	case "ratex-core", "katex-cases":
		return "tex"
	default:
		return "typst"
	}
}

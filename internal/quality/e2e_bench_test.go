package quality_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/internal/quality/daily"
)

type corpusBenchCase struct {
	source  string
	dialect untie.Dialect
	mode    untie.Mode
	limits  untie.Limits
	style   untie.Style
}

type corpusBenchStats struct {
	corpora      int64
	cases        int64
	sourceBytes  int64
	successes    int64
	items        int64
	hits         int64
	pathSegments int64
	layoutErrors int64
}

var (
	benchmarkCorpusExpr *untie.Expr
	benchmarkCorpusBox  *untie.Box
)

// Every layout variant parses and lays out the same daily corpus with the same
// preloaded font bytes. Caller-owned and warm package-cache engines are built
// before timing; the cold variant clears the package cache for every corpus
// pass; the no-cache variant builds an Engine for every case. Reported item,
// hit, and path totals cover successful layouts only. Their /case values use
// all attempts, while /success values use only cases that produced a box.
func BenchmarkEndToEndDailyCorpus(b *testing.B) {
	cases, fonts := loadDailyBenchmarkCorpus(b)
	sourceBytes := int64(0)
	uniqueLimits := make(map[untie.Limits]struct{})
	for _, testCase := range cases {
		sourceBytes += int64(len(testCase.source))
		uniqueLimits[testCase.limits] = struct{}{}
	}
	b.Run("parse-only", func(b *testing.B) {
		runCorpusBenchmark(b, cases, sourceBytes, len(uniqueLimits), nil, nil)
	})

	b.Run("e2e/caller-engine", func(b *testing.B) {
		engines := make(map[untie.Limits]*untie.Engine, len(uniqueLimits))
		for limits := range uniqueLimits {
			prepared, err := untie.NewEngine(fonts, limits)
			if err != nil {
				b.Fatal(err)
			}
			engines[limits] = prepared
		}
		runCorpusBenchmark(b, cases, sourceBytes, len(uniqueLimits), func(expr *untie.Expr, limits untie.Limits, style untie.Style) (*untie.Box, error) {
			return engines[limits].Layout(expr, style)
		}, nil)
	})

	accessibleCases := make([]corpusBenchCase, len(cases))
	copy(accessibleCases, cases)
	for i := range accessibleCases {
		accessibleCases[i].style.GenerateAlt = true
	}
	b.Run("e2e/caller-engine-accessible", func(b *testing.B) {
		engines := make(map[untie.Limits]*untie.Engine, len(uniqueLimits))
		for limits := range uniqueLimits {
			prepared, err := untie.NewEngine(fonts, limits)
			if err != nil {
				b.Fatal(err)
			}
			engines[limits] = prepared
		}
		runCorpusBenchmark(b, accessibleCases, sourceBytes, len(uniqueLimits), func(expr *untie.Expr, limits untie.Limits, style untie.Style) (*untie.Box, error) {
			return engines[limits].Layout(expr, style)
		}, nil)
	})

	b.Run("e2e/package-cache-warm", func(b *testing.B) {
		untie.ClearLayoutEngineCache()
		b.Cleanup(untie.ClearLayoutEngineCache)
		warmPackageCache(b, cases, fonts)
		runCorpusBenchmark(b, cases, sourceBytes, len(uniqueLimits), func(expr *untie.Expr, _ untie.Limits, style untie.Style) (*untie.Box, error) {
			return untie.LayoutContext(context.Background(), expr, fonts, style)
		}, nil)
	})

	b.Run("e2e/package-cache-cold", func(b *testing.B) {
		b.Cleanup(untie.ClearLayoutEngineCache)
		runCorpusBenchmark(b, cases, sourceBytes, len(uniqueLimits), func(expr *untie.Expr, _ untie.Limits, style untie.Style) (*untie.Box, error) {
			return untie.LayoutContext(context.Background(), expr, fonts, style)
		}, untie.ClearLayoutEngineCache)
	})

	b.Run("e2e/no-cache", func(b *testing.B) {
		runCorpusBenchmark(b, cases, sourceBytes, len(uniqueLimits), func(expr *untie.Expr, limits untie.Limits, style untie.Style) (*untie.Box, error) {
			prepared, err := untie.NewEngine(fonts, limits)
			if err != nil {
				return nil, err
			}
			return prepared.Layout(expr, style)
		}, nil)
	})
}

type corpusLayoutFunc func(expr *untie.Expr, limits untie.Limits, style untie.Style) (*untie.Box, error)

func runCorpusBenchmark(
	b *testing.B,
	cases []corpusBenchCase,
	sourceBytes int64,
	uniqueConfigs int,
	layout corpusLayoutFunc,
	beforeIteration func(),
) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(sourceBytes)
	b.ResetTimer()
	observed := corpusBenchStats{}
	for b.Loop() {
		observed.corpora++
		if beforeIteration != nil {
			beforeIteration()
		}
		for _, testCase := range cases {
			expr := parseBenchmarkCase(testCase)
			observed.cases++
			observed.sourceBytes += int64(len(testCase.source))
			if layout == nil {
				benchmarkCorpusExpr = expr
				continue
			}
			box, err := layout(expr, testCase.limits, testCase.style)
			if err != nil {
				observed.layoutErrors++
				continue
			}
			observed.successes++
			benchmarkCorpusBox = box
			observed.items += int64(len(box.Items))
			observed.hits += int64(len(box.Hits))
			for _, item := range box.Items {
				observed.pathSegments += int64(len(item.Path))
			}
		}
	}
	reportCorpusBenchmarkMetrics(b, observed, uniqueConfigs)
}

func reportCorpusBenchmarkMetrics(b *testing.B, stats corpusBenchStats, uniqueConfigs int) {
	elapsed := b.Elapsed().Seconds()
	if elapsed <= 0 {
		return
	}
	perSecond := func(value int64) float64 { return float64(value) / elapsed }
	perCorpus := func(value int64) float64 { return float64(value) / float64(stats.corpora) }
	perCase := func(value int64) float64 { return float64(value) / float64(stats.cases) }
	perSuccess := func(value int64) float64 { return float64(value) / float64(stats.successes) }
	b.ReportMetric(perSecond(stats.corpora), "corpora/s")
	b.ReportMetric(perSecond(stats.cases), "cases/s")
	b.ReportMetric(perSecond(stats.sourceBytes), "source-B/s")
	b.ReportMetric(perSecond(stats.items), "items/s")
	b.ReportMetric(perSecond(stats.hits), "hits/s")
	b.ReportMetric(perSecond(stats.pathSegments), "path-seg/s")
	b.ReportMetric(perCorpus(stats.cases), "cases/corpus")
	b.ReportMetric(perCorpus(stats.sourceBytes), "source-B/corpus")
	b.ReportMetric(perCorpus(stats.successes), "successes/corpus")
	b.ReportMetric(perCase(stats.items), "items/case")
	b.ReportMetric(perCase(stats.hits), "hits/case")
	b.ReportMetric(perCase(stats.pathSegments), "path-seg/case")
	if stats.successes > 0 {
		b.ReportMetric(perSuccess(stats.items), "items/success")
		b.ReportMetric(perSuccess(stats.hits), "hits/success")
		b.ReportMetric(perSuccess(stats.pathSegments), "path-seg/success")
	}
	b.ReportMetric(perCase(stats.successes), "successes/case")
	b.ReportMetric(perCase(stats.layoutErrors), "layout-errors/case")
	b.ReportMetric(float64(uniqueConfigs), "engine-configs")
}

func parseBenchmarkCase(testCase corpusBenchCase) *untie.Expr {
	options := untie.ParseOptions{Limits: testCase.limits}
	if testCase.dialect == untie.Typst {
		return untie.ParseTypst(testCase.source, testCase.mode, options)
	}
	return untie.ParseTeX(testCase.source, testCase.mode, options)
}

func warmPackageCache(b *testing.B, cases []corpusBenchCase, fonts *untie.FontSet) {
	b.Helper()
	for _, testCase := range cases {
		expr := parseBenchmarkCase(testCase)
		if _, err := untie.LayoutContext(context.Background(), expr, fonts, testCase.style); err != nil {
			continue
		}
	}
}

func loadDailyBenchmarkCorpus(b *testing.B) ([]corpusBenchCase, *untie.FontSet) {
	b.Helper()
	root := repositoryRoot(b)
	corpusRoot := filepath.Join(root, "testdata", "daily-stem")
	cases, err := daily.LoadCaseFile(filepath.Join(corpusRoot, "cases.jsonl"))
	if err != nil {
		b.Fatal(err)
	}
	manifest, err := daily.LoadManifest(filepath.Join(corpusRoot, "manifest.json"))
	if err != nil {
		b.Fatal(err)
	}
	if err := daily.ValidateCorpus(cases, manifest); err != nil {
		b.Fatal(err)
	}
	fonts, err := daily.LoadFonts(filepath.Join(root, "testdata", "upstream", "fonts", "NewCMMath-Book.otf"))
	if err != nil {
		b.Fatal(err)
	}

	benchCases := make([]corpusBenchCase, len(cases))
	for i, testCase := range cases {
		benchCase, err := benchmarkCase(testCase)
		if err != nil {
			b.Fatal(err)
		}
		benchCases[i] = benchCase
	}
	return benchCases, fonts
}

func benchmarkCase(testCase daily.Case) (corpusBenchCase, error) {
	mode := untie.Inline
	if testCase.Mode == "display" {
		mode = untie.Display
	}
	dialect := untie.TeX
	if testCase.Dialect == "typst" {
		dialect = untie.Typst
	}

	limits := untie.Limits{}
	if testCase.Limits != nil {
		limits = untie.Limits{
			MaxSourceBytes:     testCase.Limits.MaxSourceBytes,
			MaxTokens:          testCase.Limits.MaxTokens,
			MaxNesting:         testCase.Limits.MaxNesting,
			MaxNodes:           testCase.Limits.MaxNodes,
			MaxMacroExpansions: testCase.Limits.MaxMacroExpansions,
			MaxExpandedTokens:  testCase.Limits.MaxExpandedTokens,
			MaxCallDepth:       testCase.Limits.MaxCallDepth,
			MaxTableCells:      testCase.Limits.MaxTableCells,
			MaxAssemblyRepeats: testCase.Limits.MaxAssemblyRepeats,
			MaxTextBytes:       testCase.Limits.MaxTextBytes,
			MaxOutputItems:     testCase.Limits.MaxOutputItems,
			MaxPathSegments:    testCase.Limits.MaxPathSegments,
			MaxDimension:       testCase.Limits.MaxDimension,
		}
	}
	limits = limits.WithDefaults()

	style := untie.Style{Size: 16}
	if testCase.Style != nil {
		if testCase.Style.Size > 0 {
			style.Size = testCase.Style.Size
		}
		style.MaxWidth = testCase.Style.MaxWidth
		switch testCase.Style.Align {
		case "", "start":
			style.Align = untie.AlignStart
		case "center":
			style.Align = untie.AlignCenter
		case "end":
			style.Align = untie.AlignEnd
		default:
			return corpusBenchCase{}, &benchmarkConfigError{field: "align", value: testCase.Style.Align, id: testCase.ID}
		}
	}
	return corpusBenchCase{
		source:  testCase.Source,
		dialect: dialect,
		mode:    mode,
		limits:  limits,
		style:   style,
	}, nil
}

type benchmarkConfigError struct {
	field string
	value string
	id    string
}

func (e *benchmarkConfigError) Error() string {
	return "benchmark case " + e.id + ": invalid " + e.field + " " + e.value
}

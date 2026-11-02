package quality_test

import (
	"context"
	"testing"

	"github.com/caelyreth/untie"
)

type constrainedBenchStats struct {
	corpora            int64
	cases              int64
	sourceBytes        int64
	parentItems        int64
	parentHits         int64
	parentPathSegments int64
	lineItems          int64
	lineHits           int64
	linePathSegments   int64
	tagItems           int64
	tagHits            int64
	tagPathSegments    int64
	successes          int64
	layoutErrors       int64
}

var benchmarkConstrainedBox *untie.Box

func BenchmarkEndToEndConstrainedDailyCorpus(b *testing.B) {
	allCases, fonts := loadDailyBenchmarkCorpus(b)
	cases := make([]corpusBenchCase, 0, len(allCases))
	sourceBytes := int64(0)
	uniqueLimits := make(map[untie.Limits]struct{})
	for _, testCase := range allCases {
		if testCase.style.MaxWidth <= 0 {
			continue
		}
		cases = append(cases, testCase)
		sourceBytes += int64(len(testCase.source))
		uniqueLimits[testCase.limits] = struct{}{}
	}
	if len(cases) == 0 {
		b.Fatal("daily corpus has no constrained display cases")
	}

	engines := make(map[untie.Limits]*untie.Engine, len(uniqueLimits))
	for limits := range uniqueLimits {
		engine, err := untie.NewEngine(fonts, limits)
		if err != nil {
			b.Fatal(err)
		}
		engines[limits] = engine
	}

	b.ReportAllocs()
	b.SetBytes(sourceBytes)
	b.ResetTimer()
	observed := constrainedBenchStats{}
	for b.Loop() {
		observed.corpora++
		for _, testCase := range cases {
			expr := parseBenchmarkCase(testCase)
			observed.cases++
			observed.sourceBytes += int64(len(testCase.source))
			box, err := engines[testCase.limits].LayoutContext(context.Background(), expr, testCase.style)
			if err != nil {
				observed.layoutErrors++
				continue
			}
			observed.successes++
			benchmarkConstrainedBox = box
			observeBoxStorage(box, &observed)
		}
	}
	reportConstrainedBenchmarkMetrics(b, observed)
}

func observeBoxStorage(box *untie.Box, stats *constrainedBenchStats) {
	stats.parentItems += int64(len(box.Items))
	stats.parentHits += int64(len(box.Hits))
	for _, item := range box.Items {
		stats.parentPathSegments += int64(len(item.Path))
	}
	for _, line := range box.Lines {
		if line.Box == nil {
			continue
		}
		stats.lineItems += int64(len(line.Box.Items))
		stats.lineHits += int64(len(line.Box.Hits))
		for _, item := range line.Box.Items {
			stats.linePathSegments += int64(len(item.Path))
		}
	}
	if box.Tag != nil && box.Tag.Box != nil {
		stats.tagItems += int64(len(box.Tag.Box.Items))
		stats.tagHits += int64(len(box.Tag.Box.Hits))
		for _, item := range box.Tag.Box.Items {
			stats.tagPathSegments += int64(len(item.Path))
		}
	}
}

func reportConstrainedBenchmarkMetrics(b *testing.B, stats constrainedBenchStats) {
	elapsed := b.Elapsed().Seconds()
	if elapsed <= 0 || stats.corpora == 0 {
		return
	}
	perCorpus := func(value int64) float64 {
		return float64(value) / float64(stats.corpora)
	}
	physicalPathSegments := stats.parentPathSegments + stats.linePathSegments + stats.tagPathSegments
	physicalItems := stats.parentItems + stats.lineItems + stats.tagItems
	b.ReportMetric(float64(stats.corpora)/elapsed, "corpora/s")
	b.ReportMetric(perCorpus(stats.cases), "cases/corpus")
	b.ReportMetric(perCorpus(stats.successes), "successes/corpus")
	b.ReportMetric(perCorpus(stats.layoutErrors), "layout-errors/corpus")
	b.ReportMetric(perCorpus(stats.parentItems), "parent-items/corpus")
	b.ReportMetric(perCorpus(stats.lineItems), "line-items/corpus")
	b.ReportMetric(perCorpus(stats.tagItems), "tag-items/corpus")
	b.ReportMetric(perCorpus(physicalItems), "physical-items/corpus")
	b.ReportMetric(perCorpus(stats.parentPathSegments), "parent-path-seg/corpus")
	b.ReportMetric(perCorpus(stats.linePathSegments), "line-path-seg/corpus")
	b.ReportMetric(perCorpus(stats.tagPathSegments), "tag-path-seg/corpus")
	b.ReportMetric(perCorpus(physicalPathSegments), "physical-path-seg/corpus")
	b.ReportMetric(perCorpus(stats.parentHits), "parent-hits/corpus")
	b.ReportMetric(perCorpus(stats.lineHits), "line-hits/corpus")
	b.ReportMetric(perCorpus(stats.tagHits), "tag-hits/corpus")
	if stats.parentItems > 0 {
		b.ReportMetric(float64(physicalItems)/float64(stats.parentItems), "item-storage-x")
	}
	if stats.parentPathSegments > 0 {
		b.ReportMetric(float64(physicalPathSegments)/float64(stats.parentPathSegments), "path-storage-x")
	}
}

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
	lineRangeItems     int64
	lineRangeHits      int64
	lineRangePaths     int64
	tagRangeItems      int64
	tagRangeHits       int64
	tagRangePaths      int64
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
		stats.lineRangeItems += int64(line.Items.Count)
		stats.lineRangeHits += int64(line.Hits.Count)
		for _, item := range box.Items[line.Items.Start:line.Items.End()] {
			stats.lineRangePaths += int64(len(item.Path))
		}
	}
	if box.Tag != nil {
		stats.tagRangeItems += int64(box.Tag.Items.Count)
		stats.tagRangeHits += int64(box.Tag.Hits.Count)
		for _, item := range box.Items[box.Tag.Items.Start:box.Tag.Items.End()] {
			stats.tagRangePaths += int64(len(item.Path))
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
	physicalPathSegments := stats.parentPathSegments
	physicalItems := stats.parentItems
	b.ReportMetric(float64(stats.corpora)/elapsed, "corpora/s")
	b.ReportMetric(perCorpus(stats.cases), "cases/corpus")
	b.ReportMetric(perCorpus(stats.successes), "successes/corpus")
	b.ReportMetric(perCorpus(stats.layoutErrors), "layout-errors/corpus")
	b.ReportMetric(perCorpus(stats.parentItems), "parent-items/corpus")
	b.ReportMetric(perCorpus(stats.lineRangeItems), "line-range-items/corpus")
	b.ReportMetric(perCorpus(stats.tagRangeItems), "tag-range-items/corpus")
	b.ReportMetric(perCorpus(physicalItems), "physical-items/corpus")
	b.ReportMetric(perCorpus(stats.parentPathSegments), "parent-path-seg/corpus")
	b.ReportMetric(perCorpus(stats.lineRangePaths), "line-range-path-seg/corpus")
	b.ReportMetric(perCorpus(stats.tagRangePaths), "tag-range-path-seg/corpus")
	b.ReportMetric(perCorpus(physicalPathSegments), "physical-path-seg/corpus")
	b.ReportMetric(perCorpus(stats.parentHits), "parent-hits/corpus")
	b.ReportMetric(perCorpus(stats.lineRangeHits), "line-range-hits/corpus")
	b.ReportMetric(perCorpus(stats.tagRangeHits), "tag-range-hits/corpus")
	if stats.parentItems > 0 {
		b.ReportMetric(float64(physicalItems)/float64(stats.parentItems), "item-storage-x")
	}
	if stats.parentPathSegments > 0 {
		b.ReportMetric(float64(physicalPathSegments)/float64(stats.parentPathSegments), "path-storage-x")
	}
}

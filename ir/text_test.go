package ir

import (
	"reflect"
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestNormalizeTextClusters(t *testing.T) {
	t.Parallel()

	text := "a中"
	spans := []TextClusterSource{
		{TextStart: 0, TextEnd: 1, Source: source.NewRange(5, 6)},
		{TextStart: 1, TextEnd: 4, Source: source.NewRange(8, 11)},
	}
	got := NormalizeTextClusters(text, spans)
	if !reflect.DeepEqual(got, spans) {
		t.Fatalf("NormalizeTextClusters() = %#v, want %#v", got, spans)
	}

	merged := NormalizeTextClusters("ab", []TextClusterSource{
		{TextStart: 0, TextEnd: 1, Source: source.NewRange(10, 12)},
		{TextStart: 1, TextEnd: 2, Source: source.NewRange(10, 12)},
	})
	want := []TextClusterSource{{
		TextStart: 0,
		TextEnd:   2,
		Source:    source.NewRange(10, 12),
	}}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("coalesced clusters = %#v, want %#v", merged, want)
	}
}

func TestNormalizeTextClustersRejectsUnprovableMaps(t *testing.T) {
	t.Parallel()

	tests := map[string][]TextClusterSource{
		"gap": {
			{TextStart: 1, TextEnd: 2, Source: source.NewRange(1, 2)},
		},
		"incomplete": {
			{TextStart: 0, TextEnd: 1, Source: source.NewRange(1, 2)},
		},
		"overlap": {
			{TextStart: 0, TextEnd: 1, Source: source.NewRange(1, 3)},
			{TextStart: 1, TextEnd: 2, Source: source.NewRange(2, 4)},
		},
		"out of order": {
			{TextStart: 0, TextEnd: 1, Source: source.NewRange(5, 6)},
			{TextStart: 1, TextEnd: 2, Source: source.NewRange(2, 3)},
		},
		"split rune": {
			{TextStart: 0, TextEnd: 2, Source: source.NewRange(1, 3)},
			{TextStart: 2, TextEnd: 3, Source: source.NewRange(3, 4)},
		},
		"detached": {
			{TextStart: 0, TextEnd: 1, Source: source.Detached()},
		},
	}
	for name, clusters := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeTextClusters("a中", clusters); got != nil {
				t.Fatalf("NormalizeTextClusters() = %#v, want nil", got)
			}
		})
	}
}

func TestTextClusterValidationCloneAndCanonical(t *testing.T) {
	t.Parallel()

	text := NewText(source.NewRange(6, 13), "a中")
	text.Origin = source.NewRange(6, 13)
	text.Clusters = []TextClusterSource{
		{TextStart: 0, TextEnd: 1, Source: source.NewRange(6, 7)},
		{TextStart: 1, TextEnd: 4, Source: source.NewRange(9, 12)},
	}
	formula := &Formula{Root: text}
	src := strings.Repeat("x", 13)
	if err := ValidateSource(formula, src, source.Limits{}); err != nil {
		t.Fatalf("ValidateSource() error = %v", err)
	}

	encoded, err := MarshalCanonical(formula)
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"clusters":[{"source":[6,7],"text":[0,1]}`) {
		t.Fatalf("canonical output lost cluster map: %s", encoded)
	}

	clone, err := CloneFormula(formula)
	if err != nil {
		t.Fatalf("CloneFormula() error = %v", err)
	}
	cloned := clone.Root.(*Text)
	cloned.Clusters[0].Source = source.NewRange(0, 1)
	if text.Clusters[0].Source != (source.Range{Start: 6, End: 7}) {
		t.Fatal("CloneFormula() shared text cluster storage")
	}
}

func TestTextClusterValidationRejectsBadSourceRanges(t *testing.T) {
	t.Parallel()

	text := NewText(source.NewRange(0, 1), "x")
	text.Clusters = []TextClusterSource{{
		TextStart: 0,
		TextEnd:   1,
		Source:    source.NewRange(2, 3),
	}}
	if err := ValidateSource(&Formula{Root: text}, "x", source.Limits{}); err == nil {
		t.Fatal("ValidateSource() accepted a cluster source range beyond the input")
	}
}

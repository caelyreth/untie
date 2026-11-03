package display

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestFontUsageCloneAndQueries(t *testing.T) {
	t.Parallel()

	usage := FontUsage{
		Faces: []FaceUse{{
			Source:   source.NewRange(0, 1),
			FaceID:   "fallback",
			Fallback: true,
		}},
		Missing: []MissingFallback{{
			Source:     source.NewRange(1, 4),
			Text:       "中",
			TriedFaces: []string{"math", "fallback"},
		}},
	}
	if !usage.UsesFallback() || !usage.NeedsFallback() {
		t.Fatalf("usage queries = fallback %v missing %v", usage.UsesFallback(), usage.NeedsFallback())
	}
	clone := usage.Clone()
	clone.Faces[0].FaceID = "changed"
	clone.Missing[0].TriedFaces[0] = "changed"
	if usage.Faces[0].FaceID != "fallback" || usage.Missing[0].TriedFaces[0] != "math" {
		t.Fatal("FontUsage.Clone() retained shared storage")
	}
}

func TestFontUsageValidation(t *testing.T) {
	t.Parallel()

	valid := FontUsage{
		Faces: []FaceUse{{
			Source: source.NewRange(0, 1),
			FaceID: "math",
		}},
		Missing: []MissingFallback{{
			Source:     source.NewRange(1, 4),
			Text:       "中",
			TriedFaces: []string{"math"},
		}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid usage error = %v", err)
	}
	canonicalBox := &Box{FontUsage: valid}
	canonical, err := canonicalBox.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	for _, fragment := range []string{`"fontUsage"`, `"faceID":"math"`, `"triedFaces":["math"]`} {
		if !strings.Contains(string(canonical), fragment) {
			t.Fatalf("canonical output missing %s: %s", fragment, canonical)
		}
	}

	tests := []struct {
		name  string
		usage FontUsage
	}{
		{name: "face source", usage: FontUsage{Faces: []FaceUse{{FaceID: "math"}}}},
		{name: "face id", usage: FontUsage{Faces: []FaceUse{{Source: source.NewRange(0, 1)}}}},
		{
			name: "duplicate face",
			usage: FontUsage{Faces: []FaceUse{
				{Source: source.NewRange(0, 1), FaceID: "math"},
				{Source: source.NewRange(0, 1), FaceID: "math"},
			}},
		},
		{
			name: "unordered faces",
			usage: FontUsage{Faces: []FaceUse{
				{Source: source.NewRange(2, 3), FaceID: "math"},
				{Source: source.NewRange(0, 1), FaceID: "math"},
			}},
		},
		{
			name: "missing text",
			usage: FontUsage{Missing: []MissingFallback{{
				Source: source.NewRange(0, 1), TriedFaces: []string{"math"},
			}}},
		},
		{
			name: "missing faces",
			usage: FontUsage{Missing: []MissingFallback{{
				Source: source.NewRange(0, 1), Text: "x",
			}}},
		},
		{
			name: "duplicate tried face",
			usage: FontUsage{Missing: []MissingFallback{{
				Source: source.NewRange(0, 1), Text: "x",
				TriedFaces: []string{"math", "math"},
			}}},
		},
		{
			name: "duplicate missing",
			usage: FontUsage{Missing: []MissingFallback{
				{Source: source.NewRange(0, 1), Text: "x", TriedFaces: []string{"math"}},
				{Source: source.NewRange(0, 1), Text: "x", TriedFaces: []string{"math"}},
			}},
		},
		{
			name: "unordered missing",
			usage: FontUsage{Missing: []MissingFallback{
				{Source: source.NewRange(2, 3), Text: "x", TriedFaces: []string{"math"}},
				{Source: source.NewRange(0, 1), Text: "y", TriedFaces: []string{"math"}},
			}},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if err := test.usage.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

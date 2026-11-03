package font

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/caelyreth/untie/source"
)

const (
	emojiSubsetPath      = "../testdata/upstream/fonts/NotoEmoji-ShapingSubset.ttf"
	arabicSubsetPath     = "../testdata/upstream/fonts/NotoNaskhArabic-ShapingSubset.ttf"
	devanagariSubsetPath = "../testdata/upstream/fonts/NotoSansDevanagari-ShapingSubset.ttf"
)

func TestShapeTextPreservesGraphemeClusters(t *testing.T) {
	t.Parallel()

	set := loadSharedTestFont(t)
	tests := []struct {
		name         string
		text         string
		wantClusters []ShapedCluster
		wantGlyphs   []int
	}{
		{
			name: "combining mark",
			text: "a\u0301b",
			wantClusters: []ShapedCluster{
				{RuneStart: 0, RuneEnd: 2, ByteStart: 0, ByteEnd: 3, Face: MathFace},
				{RuneStart: 2, RuneEnd: 3, ByteStart: 3, ByteEnd: 4, Face: MathFace},
			},
			wantGlyphs: []int{1, 1},
		},
		{
			name: "devanagari conjunct",
			text: "क्ष",
			wantClusters: []ShapedCluster{
				{RuneStart: 0, RuneEnd: 2, ByteStart: 0, ByteEnd: 6, Face: MathFace},
				{RuneStart: 2, RuneEnd: 3, ByteStart: 6, ByteEnd: 9, Face: MathFace},
			},
			wantGlyphs: []int{2, 1},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			shaped, err := set.ShapeText(TextShapeRequest{
				Context: context.Background(),
				Text:    test.text,
				Faces:   []FaceRef{MathFace},
			})
			if err != nil {
				t.Fatalf("ShapeText: %v", err)
			}
			if !reflect.DeepEqual(shaped.Clusters, test.wantClusters) {
				t.Fatalf("clusters = %#v, want %#v", shaped.Clusters, test.wantClusters)
			}
			if got := glyphCounts(shaped); !reflect.DeepEqual(got, test.wantGlyphs) {
				t.Fatalf("glyph counts = %v, want %v", got, test.wantGlyphs)
			}
		})
	}
}

func TestShapeTextRTLOrderAndArabicScript(t *testing.T) {
	t.Parallel()

	set := loadSharedTestFont(t)
	shaped, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    "\U0001EE00\U0001EE01\U0001EE02",
		Faces:   []FaceRef{MathFace},
	})
	if err != nil {
		t.Fatalf("ShapeText: %v", err)
	}
	starts := make([]int, len(shaped.Clusters))
	for i, cluster := range shaped.Clusters {
		starts[i] = cluster.RuneStart
	}
	if want := []int{2, 1, 0}; !reflect.DeepEqual(starts, want) {
		t.Fatalf("cluster order = %v, want visual RTL order %v", starts, want)
	}
}

func TestShapeTextMixedBidiVisualOrder(t *testing.T) {
	t.Parallel()

	set := loadComplexTextFont(t)
	arabic, ok := FallbackFace(1)
	if !ok {
		t.Fatal("Arabic fallback is unavailable")
	}
	const text = "Aسلام123عالمB"
	shaped, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    text,
		Faces:   []FaceRef{MathFace, arabic},
	})
	if err != nil {
		t.Fatalf("ShapeText: %v", err)
	}
	starts := make([]int, len(shaped.Clusters))
	for i, cluster := range shaped.Clusters {
		starts[i] = cluster.RuneStart
	}
	want := []int{0, 11, 10, 9, 8, 5, 6, 7, 4, 3, 2, 1, 12}
	if !reflect.DeepEqual(starts, want) {
		t.Fatalf("mixed bidi cluster order = %v, want visual order %v", starts, want)
	}
}

func TestShapeTextShapesArabicAndDevanagari(t *testing.T) {
	t.Parallel()

	set := loadComplexTextFont(t)
	arabic, ok := FallbackFace(1)
	if !ok {
		t.Fatal("Arabic fallback is unavailable")
	}
	devanagari, ok := FallbackFace(2)
	if !ok {
		t.Fatal("Devanagari fallback is unavailable")
	}

	const arabicText = "سلام"
	arabicRun, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    arabicText,
		Faces:   []FaceRef{MathFace, arabic},
	})
	if err != nil {
		t.Fatalf("ShapeText Arabic: %v", err)
	}
	starts := make([]int, len(arabicRun.Clusters))
	for i, cluster := range arabicRun.Clusters {
		starts[i] = cluster.RuneStart
		if cluster.Face != arabic {
			t.Errorf("Arabic cluster %d face = %d, want %d", i, cluster.Face, arabic)
		}
	}
	if want := []int{3, 2, 1, 0}; !reflect.DeepEqual(starts, want) {
		t.Fatalf("Arabic cluster order = %v, want %v", starts, want)
	}
	runes := []rune(arabicText)
	contextual := false
	for _, glyph := range arabicRun.Glyphs {
		source := runes[arabicRun.Clusters[glyph.Cluster].RuneStart]
		nominal, ok := set.MapRune(arabic, source)
		if !ok {
			t.Fatalf("fallback does not map %q", string(source))
		}
		contextual = contextual || glyph.Glyph.ID != nominal.ID
	}
	if !contextual {
		t.Fatal("Arabic output contains only nominal, unjoined glyphs")
	}

	const devanagariText = "कर्मक्ष"
	devanagariRun, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    devanagariText,
		Faces:   []FaceRef{MathFace, devanagari},
	})
	if err != nil {
		t.Fatalf("ShapeText Devanagari: %v", err)
	}
	counts := glyphCounts(devanagariRun)
	if len(devanagariRun.Clusters) != 3 || !reflect.DeepEqual(counts, []int{1, 2, 1}) {
		t.Fatalf("Devanagari clusters/glyphs = %#v/%v, want three clusters [1,2,1]", devanagariRun.Clusters, counts)
	}
	if cluster := devanagariRun.Clusters[1]; cluster.RuneStart != 1 || cluster.RuneEnd != 4 || cluster.Face != devanagari {
		t.Fatalf("Devanagari reordered cluster = %#v", cluster)
	}
}

func TestShapeTextFallbackIsPerGrapheme(t *testing.T) {
	t.Parallel()

	set := loadComplexTextFont(t)
	fallback, ok := FallbackFace(0)
	if !ok {
		t.Fatal("FallbackFace(0) is unavailable")
	}
	shaped, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    "A👩‍💻B",
		Faces:   []FaceRef{MathFace, fallback},
	})
	if err != nil {
		t.Fatalf("ShapeText: %v", err)
	}
	if len(shaped.Clusters) != 3 {
		t.Fatalf("cluster count = %d, want 3", len(shaped.Clusters))
	}
	wantFaces := []FaceRef{MathFace, fallback, MathFace}
	for i, cluster := range shaped.Clusters {
		if cluster.Face != wantFaces[i] {
			t.Errorf("cluster %d face = %d, want %d", i, cluster.Face, wantFaces[i])
		}
		if cluster.Missing {
			t.Errorf("cluster %d unexpectedly missing", i)
		}
	}
	emoji := shaped.Clusters[1]
	if emoji.RuneStart != 1 || emoji.RuneEnd != 4 || emoji.ByteStart != 1 || emoji.ByteEnd != 12 {
		t.Fatalf("emoji cluster = %#v", emoji)
	}
	if got := glyphCounts(shaped); !reflect.DeepEqual(got, []int{1, 1, 1}) {
		t.Fatalf("glyph counts = %v, want one ligature glyph per cluster", got)
	}
}

func TestShapeTextMissingLimitAndCancellation(t *testing.T) {
	t.Parallel()

	set := loadSharedTestFont(t)
	shaped, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    "😀",
		Faces:   []FaceRef{MathFace},
	})
	if err != nil {
		t.Fatalf("ShapeText missing: %v", err)
	}
	if len(shaped.Clusters) != 1 || !shaped.Clusters[0].Missing {
		t.Fatalf("missing cluster = %#v", shaped.Clusters)
	}

	if _, err := set.ShapeText(TextShapeRequest{
		Context:   context.Background(),
		Text:      "ab",
		Faces:     []FaceRef{MathFace},
		MaxGlyphs: 1,
	}); !errors.Is(err, ErrTextLimit) {
		t.Fatalf("limit error = %v, want ErrTextLimit", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := set.ShapeText(TextShapeRequest{
		Context: ctx,
		Text:    "a",
		Faces:   []FaceRef{MathFace},
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v, want context.Canceled", err)
	}
}

func TestShapeTextValidatesFacesAndUTF8(t *testing.T) {
	t.Parallel()

	set := loadSharedTestFont(t)
	tests := []struct {
		name string
		req  TextShapeRequest
	}{
		{
			name: "no faces",
			req:  TextShapeRequest{Context: context.Background(), Text: "a"},
		},
		{
			name: "duplicate face",
			req: TextShapeRequest{
				Context: context.Background(), Text: "a",
				Faces: []FaceRef{MathFace, MathFace},
			},
		},
		{
			name: "unknown face",
			req: TextShapeRequest{
				Context: context.Background(), Text: "a",
				Faces: []FaceRef{TextFace},
			},
		},
		{
			name: "invalid utf8",
			req: TextShapeRequest{
				Context: context.Background(), Text: string([]byte{0xff}),
				Faces: []FaceRef{MathFace},
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := set.ShapeText(test.req); !errors.Is(err, ErrInvalidText) {
				t.Fatalf("ShapeText error = %v, want ErrInvalidText", err)
			}
		})
	}
}

func TestShapeTextAcceptsShortFeatureTags(t *testing.T) {
	t.Parallel()

	set := loadSharedTestFont(t)
	shaped, err := set.ShapeText(TextShapeRequest{
		Context:  context.Background(),
		Text:     "abc",
		Faces:    []FaceRef{MathFace},
		Features: []Feature{{Tag: "ss1"}},
	})
	if err != nil {
		t.Fatalf("ShapeText(ss1): %v", err)
	}
	if len(shaped.Clusters) != 3 {
		t.Fatalf("clusters = %d, want 3", len(shaped.Clusters))
	}
}

func TestShapeTextRequiresVariationSequenceCoverage(t *testing.T) {
	t.Parallel()

	set := loadComplexTextFont(t)
	faces := []FaceRef{MathFace}
	for i := 0; ; i++ {
		face, ok := FallbackFace(i)
		if !ok {
			break
		}
		if _, exists := set.Face(face); exists {
			faces = append(faces, face)
		}
	}
	for _, face := range faces {
		if _, ok := set.MapVariation(face, '\u2600', '\uFE0F'); ok {
			t.Skip("test font set unexpectedly covers the sun variation sequence")
		}
	}
	shaped, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    "\u2600\uFE0F",
		Faces:   faces,
	})
	if err != nil {
		t.Fatalf("ShapeText: %v", err)
	}
	if len(shaped.Clusters) != 1 || !shaped.Clusters[0].Missing {
		t.Fatalf("variation-selector clusters = %#v, want one missing cluster", shaped.Clusters)
	}
}

func TestShapedTextSourceRangesMatchText(t *testing.T) {
	t.Parallel()

	set := loadSharedTestFont(t)
	const text = "a\u0301b"
	shaped, err := set.ShapeText(TextShapeRequest{
		Context: context.Background(),
		Text:    text,
		Faces:   []FaceRef{MathFace},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, cluster := range shaped.Clusters {
		r := source.NewRange(cluster.ByteStart, cluster.ByteEnd)
		if r.Slice(text) == nil {
			t.Fatalf("invalid source range %+v", r)
		}
	}
}

func loadComplexTextFont(t testing.TB) *FontSet {
	t.Helper()
	mathData := readTestFont(t)
	emojiData, err := os.ReadFile(emojiSubsetPath)
	if err != nil {
		t.Fatalf("read emoji subset: %v", err)
	}
	arabicData, err := os.ReadFile(arabicSubsetPath)
	if err != nil {
		t.Fatalf("read Arabic subset: %v", err)
	}
	devanagariData, err := os.ReadFile(devanagariSubsetPath)
	if err != nil {
		t.Fatalf("read Devanagari subset: %v", err)
	}
	set, err := LoadFontSet(FontSetConfig{
		Math: FontFace{Bytes: mathData, ID: "newcm-math-book"},
		Fallbacks: []FontFace{
			{Bytes: emojiData, ID: "noto-emoji-shaping-subset"},
			{Bytes: arabicData, ID: "noto-naskh-arabic-shaping-subset"},
			{Bytes: devanagariData, ID: "noto-sans-devanagari-shaping-subset"},
		},
	})
	if err != nil {
		t.Fatalf("load shaping font set: %v", err)
	}
	return set
}

func glyphCounts(shaped ShapedText) []int {
	counts := make([]int, len(shaped.Clusters))
	for _, glyph := range shaped.Glyphs {
		if glyph.Cluster >= 0 && glyph.Cluster < len(counts) {
			counts[glyph.Cluster]++
		}
	}
	return counts
}

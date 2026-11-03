package untie

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestRendererFontUsageMergesFallbackClusters(t *testing.T) {
	t.Parallel()

	fonts := loadTextFallbackFonts(t)
	renderer, err := NewRenderer(fonts, RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	const glyphs = "👩‍💻👩‍💻"
	sourceText := `\text{` + glyphs + `}`
	start := strings.Index(sourceText, glyphs)
	result, err := renderer.Render(context.Background(), sourceText, RenderRequest{
		Dialect: TeX,
		Style:   Style{Size: 24},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	usage := result.Box.FontUsage
	if !usage.UsesFallback() || usage.NeedsFallback() {
		t.Fatalf("font usage = %#v, want used fallback without missing coverage", usage)
	}
	if len(usage.Faces) != 1 {
		t.Fatalf("merged face uses = %#v, want one contiguous emoji fallback span", usage.Faces)
	}
	want := source.NewRange(start, start+len(glyphs))
	if usage.Faces[0].Source != want || usage.Faces[0].FaceID != "emoji-fallback" ||
		!usage.Faces[0].Fallback || usage.Faces[0].Missing {
		t.Fatalf("face use = %#v, want source %v emoji fallback", usage.Faces[0], want)
	}
}

func TestRendererFontUsageReportsRTLFallbackInSourceOrder(t *testing.T) {
	t.Parallel()

	fonts := loadTextFallbackFonts(t)
	renderer, err := NewRenderer(fonts, RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	const mixed = "AسلامB"
	sourceText := `\text{` + mixed + `}`
	start := strings.Index(sourceText, mixed)
	result, err := renderer.Render(context.Background(), sourceText, RenderRequest{
		Dialect: TeX,
		Style:   Style{Size: 24},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	usage := result.Box.FontUsage
	if !usage.UsesFallback() || usage.NeedsFallback() {
		t.Fatalf("font usage = %#v, want Arabic fallback", usage)
	}
	arabicStart := start + len("A")
	arabicEnd := arabicStart + len("سلام")
	foundArabic := false
	previousEnd := -1
	for _, face := range usage.Faces {
		if face.Source.Start < start || face.Source.End > start+len(mixed) {
			t.Fatalf("face source %v is outside text range", face.Source)
		}
		if previousEnd > face.Source.Start {
			t.Fatalf("face uses are not source ordered: %#v", usage.Faces)
		}
		previousEnd = face.Source.End
		if face.Source == source.NewRange(arabicStart, arabicEnd) {
			foundArabic = face.FaceID == "arabic-fallback" && face.Fallback
		}
	}
	if !foundArabic {
		t.Fatalf("font usage = %#v, want Arabic fallback at %v", usage.Faces,
			source.NewRange(arabicStart, arabicEnd))
	}
}

func TestRendererFontUsageMergesMissingFallbackWithoutDuplicatingDiagnostics(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer(loadTestFonts(t), RendererOptions{})
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	const missingText = "中中"
	sourceText := `\text{` + missingText + `}`
	start := strings.Index(sourceText, missingText)
	result, err := renderer.Render(context.Background(), sourceText, RenderRequest{
		Dialect: TeX,
		Style:   Style{Size: 24},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	usage := result.Box.FontUsage
	if !usage.NeedsFallback() {
		t.Fatalf("font usage = %#v, want missing fallback", usage)
	}
	if len(usage.Missing) != 1 {
		t.Fatalf("missing fallback records = %#v, want one merged record", usage.Missing)
	}
	missing := usage.Missing[0]
	if missing.Text != missingText ||
		missing.Source != source.NewRange(start, start+len(missingText)) ||
		len(missing.TriedFaces) == 0 || missing.TriedFaces[0] != "NewCMMath-Book" {
		t.Fatalf("missing fallback = %#v", missing)
	}
	var missingDiagnostics []source.Range
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == CodeMissingGlyph {
			missingDiagnostics = append(missingDiagnostics, diagnostic.Range)
		}
	}
	if len(missingDiagnostics) != 2 ||
		missingDiagnostics[0] == missingDiagnostics[1] {
		t.Fatalf("missing diagnostics = %#v, want two distinct per-cluster ranges", missingDiagnostics)
	}
}

func loadTextFallbackFonts(t testing.TB) *FontSet {
	t.Helper()
	emoji, err := os.ReadFile("testdata/upstream/fonts/NotoEmoji-ShapingSubset.ttf")
	if err != nil {
		t.Fatalf("read emoji fallback: %v", err)
	}
	arabic, err := os.ReadFile("testdata/upstream/fonts/NotoNaskhArabic-ShapingSubset.ttf")
	if err != nil {
		t.Fatalf("read Arabic fallback: %v", err)
	}
	fonts, err := LoadFontSet(FontSetConfig{
		Math: FontFace{Bytes: testFontBytes(t), ID: "NewCMMath-Book"},
		Fallbacks: []FontFace{
			{Bytes: emoji, ID: "emoji-fallback"},
			{Bytes: arabic, ID: "arabic-fallback"},
		},
	})
	if err != nil {
		t.Fatalf("LoadFontSet() error = %v", err)
	}
	return fonts
}

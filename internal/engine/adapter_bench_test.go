package engine

import (
	"context"
	"os"
	"testing"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

var benchmarkGlyph layout.Glyph
var benchmarkRun layout.TextRun

func BenchmarkGlyphLookup(b *testing.B) {
	adapter := benchmarkAdapter(b, source.Limits{}, 0)
	req := layout.GlyphRequest{
		Context: context.Background(), Text: "x", Style: layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto, Font: ir.FontMath,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		glyph, err := adapter.Glyph(req)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkGlyph = glyph
	}
}

func BenchmarkGlyphFallbackMiss(b *testing.B) {
	adapter := benchmarkAdapter(b, source.Limits{}, 4)
	req := layout.GlyphRequest{
		Context: context.Background(), Text: "\U000F0000", Style: layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto, Font: ir.FontMath,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := adapter.Glyph(req); err == nil {
			b.Fatal("unexpected fallback glyph")
		}
	}
}

func BenchmarkTrackedText(b *testing.B) {
	adapter := benchmarkAdapter(b, source.Limits{}, 0)
	req := layout.TextRequest{
		Context: context.Background(), Text: "ab", Style: layout.Style{Size: layout.Text},
		Font: ir.FontText, Tracking: geometry.Em(geometry.EmScale / 10),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		run, err := adapter.ShapeText(req)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkRun = run
	}
}

func BenchmarkVariantStretch(b *testing.B) {
	benchmarkStretch(b, 11*geometry.Em(geometry.EmScale))
}

func BenchmarkAssembly(b *testing.B) {
	benchmarkStretch(b, 20*geometry.Em(geometry.EmScale))
}

func benchmarkStretch(b *testing.B, target geometry.Em) {
	adapter := benchmarkAdapter(b, source.Limits{}, 0)
	req := layout.StretchRequest{
		GlyphRequest: layout.GlyphRequest{
			Context: context.Background(), Text: "(", Style: layout.Style{Size: layout.Text},
			Variant: ir.VariantAuto, Font: ir.FontMath,
		},
		Axis:   layout.Vertical,
		Target: target,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		glyph, err := adapter.Stretch(req)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkGlyph = glyph
	}
}

func benchmarkAdapter(b *testing.B, limits source.Limits, fallbackCount int) *adapter {
	b.Helper()
	data, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		b.Fatal(err)
	}
	face := font.FontFace{Bytes: data, ID: "NewCMMath-Book"}
	fallbacks := make([]font.FontFace, fallbackCount)
	for i := range fallbacks {
		fallbacks[i] = face
	}
	set, err := font.LoadFontSet(font.FontSetConfig{Math: face, Fallbacks: fallbacks})
	if err != nil {
		b.Fatal(err)
	}
	adapter, err := newAdapter(set, limits)
	if err != nil {
		b.Fatal(err)
	}
	return adapter
}

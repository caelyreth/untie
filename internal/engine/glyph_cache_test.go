package engine

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/layout"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestGlyphTemplateCacheIsolationAndLifetime(t *testing.T) {
	data := readEngineTestFont(t)
	firstSet, err := font.Load(font.Fonts{
		Math: font.FontData{Bytes: data, ID: "first-math"},
		Fallbacks: []font.FontData{{
			Bytes: data,
			ID:    "first-fallback",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondSet, err := font.Load(font.Fonts{
		Math: font.FontData{Bytes: data, ID: "second-math"},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := newAdapter(firstSet, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := newAdapter(secondSet, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}

	mapped, ok := firstSet.MapRune(font.MathFace, 'x')
	if !ok {
		t.Fatal("math face does not map x")
	}
	fallbackRef, ok := font.FallbackFace(0)
	if !ok {
		t.Fatal("fallback face is unavailable")
	}
	fallback, ok := firstSet.MapRune(fallbackRef, 'x')
	if !ok || fallback.ID != mapped.ID {
		t.Fatalf("fallback x = %+v, %v; want glyph %d", fallback, ok, mapped.ID)
	}

	firstGlyph := mustGlyphFromFont(t, first, mapped, true, "x")
	secondGlyph := mustGlyphFromFont(t, second, mapped, true, "x")
	if firstGlyph.FontID != "first-math" || secondGlyph.FontID != "second-math" {
		t.Fatalf("font IDs = %q, %q; caches crossed FontSet lifetimes", firstGlyph.FontID, secondGlyph.FontID)
	}
	fallbackGlyph := mustGlyphFromFont(t, first, fallback, false, "x")
	if fallbackGlyph.FontID != "first-fallback" {
		t.Fatalf("fallback FontID = %q, want first-fallback", fallbackGlyph.FontID)
	}

	mathGlyph := mustGlyphFromFont(t, first, mapped, true, "x")
	textGlyph := mustGlyphFromFont(t, first, mapped, false, "x")
	if mathGlyph.Italic == 0 || !mathGlyph.AdvanceIncludesItalic {
		t.Fatalf("math x lacks italic semantics: %+v", mathGlyph)
	}
	if textGlyph.Italic != 0 || textGlyph.AdvanceIncludesItalic {
		t.Fatalf("text x inherited math semantics: %+v", textGlyph)
	}
	if !mathGlyph.TextLike || textGlyph.TextLike {
		t.Fatalf("TextLike math/text = %v/%v, want true/false", mathGlyph.TextLike, textGlyph.TextLike)
	}

	otherText := mustGlyphFromFont(t, first, mapped, true, "+")
	if otherText.Text != "+" || otherText.TextLike {
		t.Fatalf("request text leaked into template: %+v", otherText)
	}
	key := glyphTemplateKey{glyph: mapped, mathText: true}
	cached, ok := first.glyphTemplates.Load(key)
	if !ok {
		t.Fatal("math glyph template was not cached")
	}
	template := cached.(*glyphTemplate)
	if template.glyph.Text != "" || template.glyph.TextLike {
		t.Fatalf("cache retained request-specific fields: %+v", template.glyph)
	}
	repeated := mustGlyphFromFont(t, first, mapped, true, "x")
	if len(mathGlyph.Outline) != 0 && &mathGlyph.Outline[0] != &repeated.Outline[0] {
		t.Fatal("cached outline storage was copied on a cache hit")
	}
	if mathGlyph.Kerns != repeated.Kerns {
		t.Fatal("cached kern storage changed between cache hits")
	}
}

func TestGlyphTemplateCacheConcurrentUse(t *testing.T) {
	data := readEngineTestFont(t)
	set, err := font.Load(font.Fonts{Math: font.FontData{Bytes: data, ID: "concurrent-math"}})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := newAdapter(set, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	x, ok := set.MapRune(font.MathFace, 'x')
	if !ok {
		t.Fatal("math face does not map x")
	}
	y, ok := set.MapRune(font.MathFace, 'y')
	if !ok {
		t.Fatal("math face does not map y")
	}

	requests := []struct {
		glyph    font.Glyph
		mathText bool
		text     string
	}{
		{glyph: x, mathText: true, text: "x"},
		{glyph: x, mathText: false, text: "x"},
		{glyph: y, mathText: true, text: "y"},
		{glyph: y, mathText: false, text: "y"},
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 250 {
				for _, req := range requests {
					glyph, err := adapter.glyphFromFont(req.glyph, layout.Style{}, req.mathText, req.text)
					if err != nil {
						errs <- err
						return
					}
					if glyph.Text != req.text || glyph.TextLike != (req.mathText && isTextLike(req.text)) {
						errs <- fmt.Errorf("request fields for %q: %+v", req.text, glyph)
						return
					}
					if req.mathText && (!glyph.AdvanceIncludesItalic || len(glyph.Outline) == 0) {
						errs <- fmt.Errorf("math template for %q is incomplete: %+v", req.text, glyph)
						return
					}
					if !req.mathText && (glyph.AdvanceIncludesItalic || glyph.Italic != 0) {
						errs <- fmt.Errorf("text template for %q inherited math fields: %+v", req.text, glyph)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestGlyphLookupUsesCachedTemplate(t *testing.T) {
	data := readEngineTestFont(t)
	set, err := font.Load(font.Fonts{Math: font.FontData{Bytes: data, ID: "cached-lookup-math"}})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := newAdapter(set, source.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	req := layout.GlyphRequest{
		Context: context.Background(),
		Text:    "x",
		Style:   layout.Style{Size: layout.Text},
		Variant: ir.VariantAuto,
		Font:    ir.FontMath,
	}
	first, err := adapter.Glyph(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Glyph(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Text != second.Text || first.FontID != second.FontID ||
		first.Advance != second.Advance || first.TextLike != second.TextLike {
		t.Fatalf("cached lookup changed glyph: first=%+v second=%+v", first, second)
	}
}

func readEngineTestFont(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/upstream/fonts/NewCMMath-Book.otf")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustGlyphFromFont(t *testing.T, adapter *adapter, mapped font.Glyph, mathText bool, text string) layout.Glyph {
	t.Helper()
	glyph, err := adapter.glyphFromFont(mapped, layout.Style{}, mathText, text)
	if err != nil {
		t.Fatal(err)
	}
	return glyph
}

package engine

import (
	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/layout"
)

// glyphTemplate contains font-derived fields that are immutable for one
// adapter. Request-specific text and text-like state are applied after lookup.
type glyphTemplate struct {
	glyph    layout.Glyph
	mathText bool
}

type glyphTemplateKey struct {
	glyph    font.Glyph
	mathText bool
}

type constructionKey struct {
	glyph font.Glyph
	axis  font.MathAxis
}

// constructionTemplate is private adapter state. Its slices are treated as
// immutable after insertion.
type constructionTemplate struct {
	construction font.MathConstruction
	present      bool
}

func materializeGlyph(template *glyphTemplate, text string) layout.Glyph {
	glyph := template.glyph
	glyph.Text = text
	glyph.TextLike = template.mathText && isTextLike(text)
	return glyph
}

func (a *adapter) cachedMathConstruction(glyph font.Glyph, axis font.MathAxis) (font.MathConstruction, bool) {
	key := constructionKey{glyph: glyph, axis: axis}
	if cached, ok := a.constructions.Load(key); ok {
		template := cached.(*constructionTemplate)
		return template.construction, template.present
	}
	construction, present := a.fonts.MathConstruction(glyph, axis)
	template := &constructionTemplate{construction: construction, present: present}
	actual, _ := a.constructions.LoadOrStore(key, template)
	stored := actual.(*constructionTemplate)
	return stored.construction, stored.present
}

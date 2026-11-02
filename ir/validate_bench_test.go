package ir

import (
	"testing"

	"github.com/caelyreth/untie/source"
)

func BenchmarkValidateSuccessfulFormula(b *testing.B) {
	span := source.NewRange(0, 1)
	glyph := func(text string) Node { return NewGlyph(span, text) }

	items := make([]Node, 0, 84)
	for range 32 {
		items = append(items, NewFraction(span, glyph("a"), glyph("b")))
	}
	for range 16 {
		fenced := NewFenced(span, glyph("x"))
		fenced.Open = glyph("(")
		fenced.Close = glyph(")")
		fenced.Balanced = true
		items = append(items, fenced)
	}
	for range 16 {
		scripts := NewScripts(span, glyph("x"))
		scripts.Top = glyph("2")
		scripts.Bottom = glyph("i")
		items = append(items, scripts)
	}
	items = append(items, NewBox(span, glyph("boxed")))
	formula := &Formula{
		Root: NewRow(span, items...),
		Tags: []EquationTag{
			{Body: glyph("1"), Span: span, Origin: span, Manual: true, Line: FormulaTagLine},
			{Body: glyph("2"), Span: span, Origin: span, Manual: true, Line: 0},
		},
	}

	if err := Validate(formula, source.Limits{}); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := Validate(formula, source.Limits{}); err != nil {
			b.Fatal(err)
		}
	}
}

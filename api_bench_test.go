package untie

import (
	"context"
	"strings"
	"testing"

	"github.com/caelyreth/untie/source"
)

var (
	benchmarkExpr     *Expr
	benchmarkBox      *Box
	benchmarkEngine   *Engine
	benchmarkCacheKey layoutEngineCacheKey
)

func BenchmarkParse(b *testing.B) {
	large := strings.Repeat(`x^2+\frac{1}{2}+`, 40)
	cases := []struct {
		name      string
		source    string
		dialect   Dialect
		mode      Mode
		parseOpts ParseOptions
	}{
		{name: "tex-small", source: "x", dialect: TeX, mode: Inline},
		{name: "tex-medium", source: `x^2+\frac{1}{2}+\sqrt{y}`, dialect: TeX, mode: Inline},
		{name: "tex-large", source: large, dialect: TeX, mode: Display},
		{name: "typst-medium", source: `x^2 + 1/2 + sqrt(y)`, dialect: Typst, mode: Inline},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkExpr = Parse(tc.source, tc.dialect, tc.mode, tc.parseOpts)
			}
		})
	}
}

func BenchmarkLayoutContext(b *testing.B) {
	fonts := loadTestFonts(b)
	small := Parse("x", TeX, Inline, ParseOptions{})
	medium := Parse(`x^2+\frac{1}{2}+\sqrt{y}`, TeX, Inline, ParseOptions{})
	large := Parse(strings.Repeat(`x^2+\frac{1}{2}+`, 40), TeX, Display, ParseOptions{})

	cases := []struct {
		name string
		expr *Expr
		size float32
	}{
		{name: "small-12", expr: small, size: 12},
		{name: "small-16", expr: small, size: 16},
		{name: "small-24", expr: small, size: 24},
		{name: "medium-16", expr: medium, size: 16},
		{name: "large-16", expr: large, size: 16},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			style := Style{Size: tc.size}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				box, err := LayoutContext(context.Background(), tc.expr, fonts, style)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkBox = box
			}
		})
	}
}

func BenchmarkPreparedLayout(b *testing.B) {
	fonts := loadTestFonts(b)
	prepared, err := NewEngine(fonts, source.Limits{})
	if err != nil {
		b.Fatal(err)
	}
	small := Parse("x", TeX, Inline, ParseOptions{})
	medium := Parse(`x^2+\frac{1}{2}+\sqrt{y}`, TeX, Inline, ParseOptions{})
	large := Parse(strings.Repeat(`x^2+\frac{1}{2}+`, 40), TeX, Display, ParseOptions{})

	cases := []struct {
		name string
		expr *Expr
		size float32
	}{
		{name: "small-12", expr: small, size: 12},
		{name: "small-16", expr: small, size: 16},
		{name: "small-24", expr: small, size: 24},
		{name: "medium-16", expr: medium, size: 16},
		{name: "large-16", expr: large, size: 16},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			style := Style{Size: tc.size}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				box, err := prepared.Layout(tc.expr, style)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkBox = box
			}
		})
	}
}

func BenchmarkNewEngine(b *testing.B) {
	fonts := loadTestFonts(b)
	b.ReportAllocs()
	for b.Loop() {
		prepared, err := NewEngine(fonts, source.Limits{})
		if err != nil {
			b.Fatal(err)
		}
		benchmarkEngine = prepared
	}
}

func BenchmarkLayoutEngineCacheKey(b *testing.B) {
	fonts := loadTestFonts(b)
	limits := source.Limits{MaxOutputItems: 1234}
	b.ReportAllocs()
	for b.Loop() {
		benchmarkCacheKey = makeLayoutEngineCacheKey(fonts, limits, Version())
	}
}

func BenchmarkLayoutEngineCacheHit(b *testing.B) {
	fonts := loadTestFonts(b)
	cache := newLayoutEngineCache(4)
	ctx := context.Background()
	if _, err := cache.get(ctx, fonts, source.Limits{}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		prepared, err := cache.get(ctx, fonts, source.Limits{})
		if err != nil {
			b.Fatal(err)
		}
		benchmarkEngine = prepared
	}
}

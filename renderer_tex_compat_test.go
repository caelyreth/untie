package untie

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/source"
)

func TestTeXCompatibilityEndToEnd(t *testing.T) {
	t.Parallel()

	fonts := loadTestFonts(t)
	engine, err := NewEngine(fonts, Limits{})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	t.Run("sizing", func(t *testing.T) {
		expr := ParseTeX(`\small x`, Display, ParseOptions{})
		for _, diagnostic := range expr.Diagnostics() {
			if diagnostic.Severity == source.SeverityError {
				t.Fatalf("parse diagnostics = %v", expr.Diagnostics())
			}
		}
		box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16})
		if err != nil {
			t.Fatalf("LayoutContext() error = %v", err)
		}
		if len(box.Items) == 0 || !hostNear(box.Items[0].Em, 14.4) {
			t.Fatalf("sized item em = %v, want 14.4", box.Items[0].Em)
		}
		if err := box.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("math overlap", func(t *testing.T) {
		plain := ParseTeX(`a123b`, Display, ParseOptions{})
		clapped := ParseTeX(`a\mathclap{123}b`, Display, ParseOptions{})
		plainBox, err := engine.LayoutContext(context.Background(), plain, Style{Size: 16})
		if err != nil {
			t.Fatalf("plain LayoutContext() error = %v", err)
		}
		clappedBox, err := engine.LayoutContext(context.Background(), clapped, Style{Size: 16})
		if err != nil {
			t.Fatalf("clapped LayoutContext() error = %v", err)
		}
		if clappedBox.Width >= plainBox.Width {
			t.Fatalf("clapped width = %v, plain width = %v; zero-width overlap did not remove advance",
				clappedBox.Width, plainBox.Width)
		}
		if clappedBox.Overflow <= 0 {
			t.Fatal("clapped box did not report out-of-frame overlay ink")
		}
	})

	t.Run("row tags", func(t *testing.T) {
		expr := ParseTeX(
			`\begin{align}x&=1\tag{A}\\y&=2\tag*{B}\end{align}`,
			Display,
			ParseOptions{},
		)
		for _, diagnostic := range expr.Diagnostics() {
			if diagnostic.Severity == source.SeverityError {
				t.Fatalf("parse diagnostics = %v", expr.Diagnostics())
			}
		}
		box, err := engine.LayoutContext(context.Background(), expr, Style{Size: 16})
		if err != nil {
			t.Fatalf("LayoutContext() error = %v", err)
		}
		if box.Tag != nil || len(box.Tags) != 2 || len(box.Lines) != 0 {
			t.Fatalf("tags = formula:%+v rows:%+v lines:%d", box.Tag, box.Tags, len(box.Lines))
		}
		if box.Tags[0].Line != 0 || box.Tags[1].Line != 1 {
			t.Fatalf("row tag lines = %d/%d", box.Tags[0].Line, box.Tags[1].Line)
		}
		if err := box.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})
}

func hostNear(got, want float32) bool {
	const tolerance = float32(1) / 1024
	diff := got - want
	return diff <= tolerance && diff >= -tolerance
}

package layout

import (
	"errors"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSizedDelimiterHeightTiers(t *testing.T) {
	t.Parallel()

	for size := ir.DelimiterSize1; size <= ir.DelimiterSize4; size++ {
		got, err := sizedDelimiterHeight(size)
		if err != nil {
			t.Fatalf("sizedDelimiterHeight(%d) error = %v", size, err)
		}
		want, err := geometry.EmFromParts(3*int64(size+1), 5)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("sizedDelimiterHeight(%d) = %v, want %v", size, got, want)
		}
	}
	if _, err := sizedDelimiterHeight(0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("zero size error = %v", err)
	}
}

func TestSizedDelimiterStretchesIndependentlyOfMathStyle(t *testing.T) {
	t.Parallel()

	for size := ir.DelimiterSize1; size <= ir.DelimiterSize4; size++ {
		size := size
		t.Run(string(rune('0'+size)), func(t *testing.T) {
			t.Parallel()
			renderer, _, stretcher := newTestRenderer(t)
			span := source.NewRange(0, 7)
			node := ir.NewSizedDelimiter(span, "(", size)
			node.Class = ir.ClassOpening

			inline, err := renderer.Layout(node, ir.Display, testStyle(16))
			if err != nil {
				t.Fatalf("inline layout error = %v", err)
			}
			style := ir.NewStyle(span, node)
			style.SetSize = true
			style.NewSize = ir.SizeScript
			script, err := renderer.Layout(style, ir.Display, testStyle(16))
			if err != nil {
				t.Fatalf("script layout error = %v", err)
			}
			wantEm, err := sizedDelimiterHeight(size)
			if err != nil {
				t.Fatal(err)
			}
			wantExtent := float32(wantEm.Float() * 16)
			for name, box := range map[string]*display.Box{"inline": inline, "script": script} {
				if !closeHost(box.Ascent+box.Descent, wantExtent) {
					t.Fatalf("%s extent = %v, want %v", name, box.Ascent+box.Descent, wantExtent)
				}
				if box.Width <= 0 {
					t.Fatalf("%s width = %v, want positive", name, box.Width)
				}
			}
			if len(stretcher.targets) != 2 {
				t.Fatalf("stretch targets = %v, want inline and script requests", stretcher.targets)
			}
			wantInlineTarget, _ := geometry.EmFromParts(3*int64(size+1), 5)
			if stretcher.targets[0] != wantInlineTarget {
				t.Fatalf("inline target = %v, want %v", stretcher.targets[0], wantInlineTarget)
			}
			if stretcher.targets[1] != wantInlineTarget {
				t.Fatalf("script target = %v, want style-independent %v", stretcher.targets[1], wantInlineTarget)
			}
		})
	}
}

func TestSizedDelimiterPreservesClassSpacingAndSourceHit(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	x := glyph("x", ir.ClassNormal)
	x.Span = source.NewRange(0, 1)
	x.Origin = x.Span
	relation := ir.NewSizedDelimiter(source.NewRange(1, 7), ".", ir.DelimiterSize1)
	relation.Origin = source.NewRange(2, 7)
	relation.Class = ir.ClassRelation
	y := glyph("y", ir.ClassNormal)
	y.Span = source.NewRange(8, 9)
	y.Origin = y.Span
	root := ir.NewRow(source.NewRange(0, 9), x, relation, y)

	box, err := renderer.Layout(root, ir.Inline, testStyle(16))
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if len(box.Items) != 2 {
		t.Fatalf("items = %#v, want only x and y glyphs", box.Items)
	}
	thick, err := geometry.EmFromParts(5, 18)
	if err != nil {
		t.Fatal(err)
	}
	wantY := float32(thick.Float()*16*2) + 8
	if !closeHost(box.Items[1].At.X, wantY) {
		t.Fatalf("relation spacing y.x = %v, want %v", box.Items[1].At.X, wantY)
	}

	var hit *display.Hit
	for i := range box.Hits {
		if box.Hits[i].Kind == display.HitFence && box.Hits[i].Source == relation.Origin {
			hit = &box.Hits[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("hits = %#v, want sized-delimiter fence hit for %v", box.Hits, relation.Origin)
	}
	if hit.Area.Width != 0 {
		t.Fatalf("null delimiter hit width = %v, want 0", hit.Area.Width)
	}
}

func TestSizedDelimiterRejectsMalformedInputAndMissingStretch(t *testing.T) {
	t.Parallel()

	renderer, _, _ := newTestRenderer(t)
	for name, node := range map[string]*ir.SizedDelimiter{
		"empty": {
			Base: ir.NewBase(ir.KindSizedDelimiter, source.NewRange(0, 1)),
			Size: ir.DelimiterSize1,
		},
		"invalid size": ir.NewSizedDelimiter(source.NewRange(0, 1), "(", ir.DelimiterSize4+1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := renderer.Layout(node, ir.Inline, testStyle(16)); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Layout() error = %v, want invalid input", err)
			}
		})
	}

	glyphs := &fakeGlyphSource{glyphs: map[string]Glyph{
		"(": {
			ID:      1,
			FontID:  "test-math",
			Advance: em(1, 4),
			Ascent:  em(1, 2),
			Descent: em(1, 10),
		},
	}, strict: true}
	noStretch, err := New(Fonts{Glyphs: glyphs, Metrics: fakeStyleFont{}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	node := ir.NewSizedDelimiter(source.NewRange(0, 4), "(", ir.DelimiterSize1)
	node.Class = ir.ClassOpening
	if _, err := noStretch.Layout(node, ir.Inline, testStyle(16)); !errors.Is(err, ErrInsufficientStretch) {
		t.Fatalf("Layout() error = %v, want insufficient stretch", err)
	}
}

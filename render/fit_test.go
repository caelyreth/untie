package render_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/render"
)

func TestFitIncludesOverflowAndAppliesAllAlignments(t *testing.T) {
	t.Parallel()

	box := &display.Box{
		Width:    20,
		Ascent:   10,
		Descent:  4,
		Overflow: 3,
	}
	before := box.Clone()
	viewport := geometry.Rect{X: 10, Y: 20, Width: 60, Height: 40}
	bounds := box.Bounds()
	tests := []struct {
		name      string
		align     render.Align
		wantLeft  float32
		wantRight float32
	}{
		{name: "start", align: render.AlignStart, wantLeft: 15, wantRight: 54},
		{name: "center", align: render.AlignCenter, wantLeft: 20.5, wantRight: 59.5},
		{name: "end", align: render.AlignEnd, wantLeft: 26, wantRight: 65},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transform, err := render.Fit(box, viewport, render.FitOptions{
				Padding: 5,
				Align:   test.align,
			})
			if err != nil {
				t.Fatalf("Fit() error = %v", err)
			}
			closeFloat(t, "Scale", transform.Scale, 1.5)
			placed := transform.ApplyRect(bounds)
			closeFloat(t, "placed width", placed.Width, 39)
			closeFloat(t, "placed height", placed.Height, 30)
			closeFloat(t, "placed left", placed.X, test.wantLeft)
			closeFloat(t, "placed right", placed.X+placed.Width, test.wantRight)
			closeFloat(t, "placed top", placed.Y, 25)
			closeFloat(t, "placed bottom", placed.Y+placed.Height, 55)
		})
	}
	if !reflect.DeepEqual(box, before) {
		t.Fatal("Fit() mutated the box")
	}
}

func TestFitUsesTightArbitraryScale(t *testing.T) {
	t.Parallel()

	box := &display.Box{Width: 100, Ascent: 50}
	transform, err := render.Fit(box, geometry.Rect{Width: 100, Height: 100}, render.FitOptions{Padding: 10})
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}
	closeFloat(t, "Scale", transform.Scale, 0.8)
	placed := transform.ApplyRect(box.Bounds())
	closeFloat(t, "left", placed.X, 10)
	closeFloat(t, "top", placed.Y, 30)
	closeFloat(t, "width", placed.Width, 80)
	closeFloat(t, "height", placed.Height, 40)
}

func TestFitHandlesZeroSizedContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		box       *display.Box
		wantScale float32
	}{
		{name: "point", box: &display.Box{}, wantScale: 1},
		{name: "zero width", box: &display.Box{Ascent: 10, Descent: 10}, wantScale: 4},
		{name: "zero height", box: &display.Box{Width: 20}, wantScale: 4},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transform, err := render.Fit(test.box, geometry.Rect{Width: 100, Height: 100}, render.FitOptions{
				Padding: 10,
				Align:   render.AlignCenter,
			})
			if err != nil {
				t.Fatalf("Fit() error = %v", err)
			}
			closeFloat(t, "Scale", transform.Scale, test.wantScale)
			placed := transform.ApplyRect(test.box.Bounds())
			closeFloat(t, "center x", placed.X+placed.Width/2, 50)
			closeFloat(t, "center y", placed.Y+placed.Height/2, 50)
		})
	}
}

func TestFitRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	validBox := &display.Box{Width: 10, Ascent: 5}
	validViewport := geometry.Rect{Width: 100, Height: 100}
	tests := []struct {
		name     string
		box      *display.Box
		viewport geometry.Rect
		options  render.FitOptions
		want     error
	}{
		{name: "nil box", viewport: validViewport, want: render.ErrInvalidBox},
		{name: "nan box", box: &display.Box{Width: float32(math.NaN())}, viewport: validViewport, want: render.ErrInvalidBox},
		{name: "negative box width", box: &display.Box{Width: -1}, viewport: validViewport, want: render.ErrInvalidBox},
		{name: "infinite box", box: &display.Box{Ascent: float32(math.Inf(1))}, viewport: validViewport, want: render.ErrInvalidBox},
		{name: "zero viewport width", box: validBox, viewport: geometry.Rect{Height: 100}, want: render.ErrInvalidViewport},
		{name: "negative viewport height", box: validBox, viewport: geometry.Rect{Width: 100, Height: -1}, want: render.ErrInvalidViewport},
		{name: "nan viewport", box: validBox, viewport: geometry.Rect{Width: float32(math.NaN()), Height: 100}, want: render.ErrInvalidViewport},
		{name: "negative padding", box: validBox, viewport: validViewport, options: render.FitOptions{Padding: -1}, want: render.ErrInvalidFitOptions},
		{name: "infinite padding", box: validBox, viewport: validViewport, options: render.FitOptions{Padding: float32(math.Inf(1))}, want: render.ErrInvalidFitOptions},
		{name: "padding leaves no width", box: validBox, viewport: validViewport, options: render.FitOptions{Padding: 50}, want: render.ErrInvalidFitOptions},
		{name: "unknown alignment", box: validBox, viewport: validViewport, options: render.FitOptions{Align: render.Align(255)}, want: render.ErrInvalidFitOptions},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transform, err := render.Fit(test.box, test.viewport, test.options)
			if !errors.Is(err, test.want) {
				t.Fatalf("Fit() error = %v, want %v", err, test.want)
			}
			if transform != (render.Transform{}) {
				t.Fatalf("Fit() transform = %+v, want zero", transform)
			}
		})
	}
}

func TestTransformApplyAndApplyRect(t *testing.T) {
	t.Parallel()

	transform := render.Transform{
		Scale:  2,
		Offset: geometry.Point{X: 3, Y: 4},
	}
	if got := transform.Apply(geometry.Point{X: 5, Y: 6}); got != (geometry.Point{X: 13, Y: 16}) {
		t.Fatalf("Apply() = %+v", got)
	}
	want := geometry.Rect{X: 7, Y: 10, Width: 16, Height: 20}
	if got := transform.ApplyRect(geometry.Rect{X: 2, Y: 3, Width: 8, Height: 10}); got != want {
		t.Fatalf("ApplyRect() = %+v, want %+v", got, want)
	}
}

func closeFloat(t *testing.T, name string, got, want float32) {
	t.Helper()
	if delta := got - want; delta < -1e-5 || delta > 1e-5 {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

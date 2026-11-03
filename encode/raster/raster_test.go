package raster_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/encode/raster"
	"github.com/caelyreth/untie/geometry"
)

func TestEncodePaintOrderRolesAndExplicitColors(t *testing.T) {
	t.Parallel()

	palette := raster.Palette{
		Ink:         color.NRGBA{B: 0xff, A: 0xff},
		Error:       color.NRGBA{R: 0xff, A: 0xff},
		Placeholder: color.NRGBA{G: 0xff, A: 0xff},
	}
	box := makeBox(4, 4, 0, 0, []display.Item{
		rule(display.Paint{Role: display.RoleInk}, geometry.Rect{Y: -4, Width: 4, Height: 4}),
		rule(display.Paint{Role: display.RoleError}, geometry.Rect{Y: -4, Width: 2, Height: 2}),
		rule(explicitPaint(0xff, 0xff, 0, 0xff),
			geometry.Rect{X: 1, Y: -3, Width: 2, Height: 2}),
		rule(display.Paint{Role: display.RolePlaceholder},
			geometry.Rect{X: 3, Y: -1, Width: 1, Height: 1}),
	}, nil)
	img := encodeOK(t, box, &raster.Options{Scale: 1, Palette: palette})

	assertSize(t, img, 4, 4)
	assertPixel(t, img, 3, 0, palette.Ink)
	assertPixel(t, img, 0, 0, palette.Error)
	assertPixel(t, img, 1, 0, palette.Error)
	assertPixel(t, img, 1, 1, color.NRGBA{R: 0xff, G: 0xff, A: 0xff})
	assertPixel(t, img, 2, 2, color.NRGBA{R: 0xff, G: 0xff, A: 0xff})
	assertPixel(t, img, 3, 3, palette.Placeholder)
}

func TestEncodeBackgroundAndTransparentDefault(t *testing.T) {
	t.Parallel()

	box := makeBox(2, 2, 0, 0, []display.Item{
		rule(display.Paint{Role: display.RoleInk},
			geometry.Rect{Y: -2, Width: 1, Height: 1}),
	}, nil)
	background := color.NRGBA{R: 0xf0, G: 0xe0, B: 0xd0, A: 0xff}
	img := encodeOK(t, box, &raster.Options{
		Scale:      1,
		Background: &background,
		Palette:    raster.Palette{Ink: color.NRGBA{A: 0xff}},
	})
	assertPixel(t, img, 0, 0, color.NRGBA{A: 0xff})
	assertPixel(t, img, 1, 1, background)

	transparent := encodeOK(t, box, &raster.Options{
		Scale:   1,
		Palette: raster.Palette{Ink: color.NRGBA{A: 0xff}},
	})
	assertPixel(t, transparent, 1, 1, color.NRGBA{})
}

func TestEncodeCurvesHolesAndAntialiasing(t *testing.T) {
	t.Parallel()

	template := display.PathTemplate{
		Bound: geometry.Rect{Width: 8, Height: 8},
		Segments: []geometry.Segment{
			{Op: geometry.SegMoveTo, Points: points(1, 1)},
			{Op: geometry.SegQuadTo, Points: points(4, 1, 7, 1)},
			{Op: geometry.SegLineTo, Points: points(7, 7)},
			{Op: geometry.SegLineTo, Points: points(1, 7)},
			{Op: geometry.SegClose},
			// Counter-clockwise circle in screen coordinates, opposite the
			// outer contour, so nonzero winding leaves a transparent hole.
			{Op: geometry.SegMoveTo, Points: points(2, 4)},
			{Op: geometry.SegCubeTo, Points: points(2, 5.1046, 3, 6, 4, 6)},
			{Op: geometry.SegCubeTo, Points: points(5, 6, 6, 5.1046, 6, 4)},
			{Op: geometry.SegCubeTo, Points: points(6, 2.8954, 5, 2, 4, 2)},
			{Op: geometry.SegCubeTo, Points: points(3, 2, 2, 2.8954, 2, 4)},
			{Op: geometry.SegClose},
		},
	}
	box := makeBox(8, 8, 0, 0, []display.Item{
		pathItem(0, geometry.Point{Y: -8}, 1, geometry.Rect{Y: -8, Width: 8, Height: 8}),
	}, []display.PathTemplate{template})
	background := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	img := encodeOK(t, box, &raster.Options{
		Scale:      1,
		Background: &background,
		Palette:    raster.Palette{Ink: color.NRGBA{A: 0xff}},
	})

	assertPixel(t, img, 1, 2, color.NRGBA{A: 0xff})
	assertPixel(t, img, 4, 4, background)
	assertPixel(t, img, 0, 0, background)
	if !hasAntialiasedGray(img) {
		t.Fatal("curved edge produced no antialiased intermediate pixels")
	}
}

func TestEncodeUsesNonzeroWinding(t *testing.T) {
	t.Parallel()

	template := display.PathTemplate{
		Bound: geometry.Rect{Width: 3, Height: 3},
		Segments: []geometry.Segment{
			// Two overlapping contours with the same winding. Even-odd filling
			// would punch a hole through their overlap; nonzero filling must not.
			{Op: geometry.SegMoveTo, Points: points(0, 0)},
			{Op: geometry.SegLineTo, Points: points(2, 0)},
			{Op: geometry.SegLineTo, Points: points(2, 2)},
			{Op: geometry.SegLineTo, Points: points(0, 2)},
			{Op: geometry.SegClose},
			{Op: geometry.SegMoveTo, Points: points(1, 1)},
			{Op: geometry.SegLineTo, Points: points(3, 1)},
			{Op: geometry.SegLineTo, Points: points(3, 3)},
			{Op: geometry.SegLineTo, Points: points(1, 3)},
			{Op: geometry.SegClose},
		},
	}
	box := makeBox(3, 3, 0, 0, []display.Item{
		pathItem(0, geometry.Point{Y: -3}, 1, geometry.Rect{Y: -3, Width: 3, Height: 3}),
	}, []display.PathTemplate{template})
	img := encodeOK(t, box, &raster.Options{
		Scale:   1,
		Palette: raster.Palette{Ink: color.NRGBA{A: 0xff}},
	})
	assertPixel(t, img, 1, 1, color.NRGBA{A: 0xff})
}

func TestEncodeOverflowPaddingAndDeviceScale(t *testing.T) {
	t.Parallel()

	background := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	t.Run("overflow and padding", func(t *testing.T) {
		t.Parallel()

		bounds := geometry.Rect{X: -1, Y: -3, Width: 4, Height: 4}
		box := makeBox(2, 2, 0, 1, []display.Item{
			rule(display.Paint{Role: display.RoleInk}, bounds),
		}, nil)
		img := encodeOK(t, box, &raster.Options{
			Scale:      1,
			Padding:    2,
			Background: &background,
			Palette:    raster.Palette{Ink: color.NRGBA{A: 0xff}},
		})
		assertSize(t, img, 8, 8)
		assertPixel(t, img, 1, 2, background)
		assertPixel(t, img, 2, 2, color.NRGBA{A: 0xff})
		assertPixel(t, img, 5, 5, color.NRGBA{A: 0xff})
		assertPixel(t, img, 7, 6, background)
	})
	t.Run("device pixel ratio", func(t *testing.T) {
		t.Parallel()

		box := makeBox(2, 2, 0, 0, []display.Item{
			rule(display.Paint{Role: display.RoleInk}, geometry.Rect{Y: -2, Width: 2, Height: 2}),
		}, nil)
		img := encodeOK(t, box, &raster.Options{
			Scale:      2,
			Padding:    1,
			Background: &background,
			Palette:    raster.Palette{Ink: color.NRGBA{A: 0xff}},
		})
		assertSize(t, img, 6, 6)
		assertPixel(t, img, 0, 0, background)
		assertPixel(t, img, 1, 1, color.NRGBA{A: 0xff})
		assertPixel(t, img, 4, 4, color.NRGBA{A: 0xff})
		assertPixel(t, img, 5, 5, background)
	})
}

func TestEncodeEmptyBoxAndDefaults(t *testing.T) {
	t.Parallel()

	img := encodeOK(t, &display.Box{}, &raster.Options{Scale: 1})
	assertSize(t, img, 1, 1)
	assertPixel(t, img, 0, 0, color.NRGBA{})

	defaults := encodeOK(t, &display.Box{}, nil)
	assertSize(t, defaults, 16, 16)
	assertPixel(t, defaults, 8, 8, color.NRGBA{})
}

func TestEncodeRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	validBox := makeBox(1, 1, 0, 0, nil, nil)
	tests := []struct {
		name    string
		box     *display.Box
		options *raster.Options
		want    error
	}{
		{name: "nil box", want: raster.ErrInvalidBox},
		{name: "zero scale", box: validBox, options: &raster.Options{}, want: raster.ErrInvalidOptions},
		{name: "negative scale", box: validBox, options: &raster.Options{Scale: -1}, want: raster.ErrInvalidOptions},
		{name: "nan scale", box: validBox, options: &raster.Options{Scale: float32(math.NaN())}, want: raster.ErrInvalidOptions},
		{name: "infinite scale", box: validBox, options: &raster.Options{Scale: float32(math.Inf(1))}, want: raster.ErrInvalidOptions},
		{name: "negative padding", box: validBox, options: &raster.Options{Scale: 1, Padding: -1}, want: raster.ErrInvalidOptions},
		{name: "nan padding", box: validBox, options: &raster.Options{Scale: 1, Padding: float32(math.NaN())}, want: raster.ErrInvalidOptions},
		{name: "infinite padding", box: validBox, options: &raster.Options{Scale: 1, Padding: float32(math.Inf(1))}, want: raster.ErrInvalidOptions},
		{name: "dimension overflow", box: validBox, options: &raster.Options{Scale: math.MaxFloat32}, want: raster.ErrImageTooLarge},
		{
			name: "bounding overflow",
			box: &display.Box{
				Width:    1,
				Overflow: math.MaxFloat32,
			},
			options: &raster.Options{Scale: 1},
			want:    raster.ErrInvalidBox,
		},
		{
			name: "unknown paint role",
			box: &display.Box{
				Width:  1,
				Ascent: 1,
				Items: []display.Item{{
					Kind:      display.ItemRule,
					Paint:     display.Paint{Role: display.Role(255)},
					Bound:     geometry.Rect{Y: -1, Width: 1, Height: 1},
					PathIndex: display.NoPath,
					Rect:      geometry.Rect{Y: -1, Width: 1, Height: 1},
				}},
			},
			options: &raster.Options{Scale: 1},
			want:    raster.ErrInvalidBox,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			img, err := raster.Encode(test.box, test.options)
			if !errors.Is(err, test.want) {
				t.Fatalf("Encode() error = %v, want %v", err, test.want)
			}
			if img != nil {
				t.Fatalf("Encode() image = %#v, want nil", img)
			}
		})
	}
}

func TestEncodePNGDeterministicRoundTrip(t *testing.T) {
	t.Parallel()

	box := makeBox(2, 2, 0, 0, []display.Item{
		rule(display.Paint{Role: display.RoleInk}, geometry.Rect{Y: -2, Width: 2, Height: 2}),
	}, nil)
	options := &raster.Options{
		Scale:   1,
		Palette: raster.Palette{Ink: color.NRGBA{R: 10, G: 20, B: 30, A: 0xff}},
	}
	var first, second bytes.Buffer
	if err := raster.EncodePNG(&first, box, options); err != nil {
		t.Fatalf("first EncodePNG() error = %v", err)
	}
	if err := raster.EncodePNG(&second, box, options); err != nil {
		t.Fatalf("second EncodePNG() error = %v", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("EncodePNG() output is nondeterministic")
	}
	decoded, err := png.Decode(bytes.NewReader(first.Bytes()))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	direct := encodeOK(t, box, options)
	if decoded.Bounds() != direct.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", decoded.Bounds(), direct.Bounds())
	}
	for y := direct.Bounds().Min.Y; y < direct.Bounds().Max.Y; y++ {
		for x := direct.Bounds().Min.X; x < direct.Bounds().Max.X; x++ {
			if !colorEqual(decoded.At(x, y), direct.At(x, y)) {
				t.Fatalf("decoded pixel (%d,%d) = %v, want %v",
					x, y, decoded.At(x, y), direct.At(x, y))
			}
		}
	}
	if err := raster.EncodePNG(nil, box, options); !errors.Is(err, raster.ErrInvalidWriter) {
		t.Fatalf("EncodePNG(nil writer) error = %v", err)
	}
}

func TestEncodeStablePixelHash(t *testing.T) {
	t.Parallel()

	box := makeBox(2, 2, 0, 0, []display.Item{
		rule(display.Paint{Role: display.RoleInk}, geometry.Rect{Y: -2, Width: 2, Height: 2}),
	}, nil)
	img := encodeOK(t, box, &raster.Options{
		Scale:   1,
		Palette: raster.Palette{Ink: color.NRGBA{R: 10, G: 20, B: 30, A: 0xff}},
	})
	const want = "850ddc69d6a093082c28269fd232b7543f8133b013b4820a4f15ebfe769d18d4"
	if got := stableHash(img); got != want {
		t.Fatalf("pixel hash = %s, want %s", got, want)
	}
}

func makeBox(
	width float32,
	ascent float32,
	descent float32,
	overflow float32,
	items []display.Item,
	paths []display.PathTemplate,
) *display.Box {
	box := &display.Box{
		Width:    width,
		Ascent:   ascent,
		Descent:  descent,
		Overflow: overflow,
		Items:    items,
		Paths:    paths,
	}
	if err := box.Validate(); err != nil {
		panic(err)
	}
	return box
}

func rule(paint display.Paint, rect geometry.Rect) display.Item {
	return display.Item{
		Kind:      display.ItemRule,
		Paint:     paint,
		Bound:     rect,
		PathIndex: display.NoPath,
		Rect:      rect,
	}
}

func pathItem(
	index int,
	at geometry.Point,
	em float32,
	bound geometry.Rect,
) display.Item {
	return display.Item{
		Kind:      display.ItemGlyphPath,
		Paint:     display.Paint{Role: display.RoleInk},
		Bound:     bound,
		PathIndex: index,
		At:        at,
		Em:        em,
	}
}

func explicitPaint(r, g, b, a uint8) display.Paint {
	return display.Paint{
		Role:        display.RoleInk,
		Explicit:    display.RGBA{R: r, G: g, B: b, A: a},
		HasExplicit: true,
	}
}

func points(values ...float32) [3]geometry.Point {
	var out [3]geometry.Point
	for i := 0; i+1 < len(values); i += 2 {
		out[i/2] = geometry.Point{X: values[i], Y: values[i+1]}
	}
	return out
}

func encodeOK(t *testing.T, box *display.Box, options *raster.Options) *image.RGBA {
	t.Helper()
	img, err := raster.Encode(box, options)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if img == nil {
		t.Fatal("Encode() image = nil")
	}
	return img
}

func assertSize(t *testing.T, img image.Image, width, height int) {
	t.Helper()
	want := image.Rect(0, 0, width, height)
	if got := img.Bounds(); got != want {
		t.Fatalf("image bounds = %v, want %v", got, want)
	}
}

func assertPixel(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
	t.Helper()
	got := img.At(x, y)
	if !colorEqual(got, want) {
		t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

func colorEqual(got color.Color, want color.Color) bool {
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()
	return gr == wr && gg == wg && gb == wb && ga == wa
}

func hasAntialiasedGray(img *image.RGBA) bool {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel := img.RGBAAt(x, y)
			if pixel.R == pixel.G && pixel.G == pixel.B &&
				pixel.R > 0 && pixel.R < 0xff && pixel.A == 0xff {
				return true
			}
		}
	}
	return false
}

func stableHash(img *image.RGBA) string {
	return hexString(sha256.Sum256(img.Pix))
}

func hexString(bytes [32]byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(bytes)*2)
	for i, value := range bytes {
		out[2*i] = digits[value>>4]
		out[2*i+1] = digits[value&0x0f]
	}
	return string(out)
}

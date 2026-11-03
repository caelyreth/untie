package raster

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/render"
	"golang.org/x/image/vector"
)

var (
	// ErrInvalidBox reports a malformed display box.
	ErrInvalidBox = errors.New("raster: invalid box")
	// ErrInvalidOptions reports an invalid scale, padding, or output size.
	ErrInvalidOptions = errors.New("raster: invalid options")
	// ErrInvalidPath reports a path segment that cannot be rasterized.
	ErrInvalidPath = errors.New("raster: invalid path")
	// ErrInvalidWriter reports a nil PNG destination.
	ErrInvalidWriter = errors.New("raster: invalid writer")
	// ErrImageTooLarge reports dimensions that cannot form an image.RGBA.
	ErrImageTooLarge = errors.New("raster: image too large")
)

// Palette resolves semantic display paints to concrete colors.
type Palette struct {
	Ink         color.NRGBA
	Error       color.NRGBA
	Placeholder color.NRGBA
}

// Options controls raster output.
//
// Scale is the number of output pixels per host unit and must be finite and
// positive. Padding is measured in output pixels and must be finite and
// nonnegative. A nil Background creates a transparent image; a non-nil
// Background fills the complete image before visible commands are drawn.
type Options struct {
	Scale      float32
	Padding    float32
	Background *color.NRGBA
	Palette    Palette
}

// DefaultOptions returns a transparent one-pixel-per-unit image with eight
// pixels of padding and conventional black, red, and muted-gray paints.
func DefaultOptions() Options {
	return Options{
		Scale:   1,
		Padding: 8,
		Palette: Palette{
			Ink:         color.NRGBA{A: 0xff},
			Error:       color.NRGBA{R: 0xc0, A: 0xff},
			Placeholder: color.NRGBA{R: 0x70, G: 0x70, B: 0x70, A: 0xff},
		},
	}
}

// Encode rasterizes box into a caller-owned RGBA image.
//
// The image includes the complete Box.Bounds, including certified overflow.
// Each dimension is ceil(boundsDimension*Scale + 2*Padding), with a minimum of
// one pixel. Box coordinates map to output pixels as
// (boundsCoordinate-bounds.Min)*Scale + Padding.
func Encode(box *display.Box, options *Options) (*image.RGBA, error) {
	resolved, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	if err := validateBox(box); err != nil {
		return nil, err
	}
	canvas, err := newCanvas(box, resolved)
	if err != nil {
		return nil, err
	}

	img := image.NewRGBA(image.Rect(0, 0, canvas.width, canvas.height))
	if resolved.Background != nil {
		background := *resolved.Background
		draw.Draw(img, img.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)
	}

	rasterizer := vector.NewRasterizer(canvas.width, canvas.height)
	err = render.Walk(box, func(command render.Command) error {
		paint, err := resolvePaint(command.Paint, resolved.Palette)
		if err != nil {
			return err
		}
		rasterizer.Reset(canvas.width, canvas.height)
		switch command.Kind {
		case render.CommandPath:
			if err := canvas.drawPath(rasterizer, command); err != nil {
				return err
			}
		case render.CommandRule:
			if err := canvas.drawRule(rasterizer, command.Rect); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: command kind %d", ErrInvalidBox, command.Kind)
		}
		uniform := image.Uniform{C: paint}
		rasterizer.Draw(img, img.Bounds(), &uniform, image.Point{})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return img, nil
}

// EncodePNG rasterizes box and encodes the result as a PNG stream.
func EncodePNG(w io.Writer, box *display.Box, options *Options) error {
	if w == nil {
		return ErrInvalidWriter
	}
	img, err := Encode(box, options)
	if err != nil {
		return err
	}
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("raster: encode PNG: %w", err)
	}
	return nil
}

type canvas struct {
	width   int
	height  int
	scale   float64
	originX float64
	originY float64
}

func resolveOptions(options *Options) (Options, error) {
	if options == nil {
		return DefaultOptions(), nil
	}
	resolved := *options
	if !finite(float64(resolved.Scale)) || resolved.Scale <= 0 {
		return Options{}, fmt.Errorf("%w: scale %v is not finite and positive",
			ErrInvalidOptions, resolved.Scale)
	}
	if !finite(float64(resolved.Padding)) || resolved.Padding < 0 {
		return Options{}, fmt.Errorf("%w: padding %v is not finite and nonnegative",
			ErrInvalidOptions, resolved.Padding)
	}
	return resolved, nil
}

func validateBox(box *display.Box) error {
	if box == nil {
		return fmt.Errorf("%w: nil box", ErrInvalidBox)
	}
	if err := box.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBox, err)
	}
	return nil
}

func newCanvas(box *display.Box, options Options) (canvas, error) {
	bounds := box.Bounds()
	for _, field := range []struct {
		name  string
		value float32
		zero  bool
	}{
		{name: "x", value: bounds.X},
		{name: "y", value: bounds.Y},
		{name: "width", value: bounds.Width, zero: true},
		{name: "height", value: bounds.Height, zero: true},
	} {
		value := float64(field.value)
		if !finite(value) || field.zero && value < 0 {
			return canvas{}, fmt.Errorf("%w: bounds %s is %v",
				ErrInvalidBox, field.name, field.value)
		}
	}

	scale := float64(options.Scale)
	padding := float64(options.Padding)
	pixelWidth := math.Ceil(float64(bounds.Width)*scale + 2*padding)
	pixelHeight := math.Ceil(float64(bounds.Height)*scale + 2*padding)
	if !finite(pixelWidth) || !finite(pixelHeight) {
		return canvas{}, fmt.Errorf("%w: output dimensions overflow", ErrImageTooLarge)
	}
	pixelWidth = max(1, pixelWidth)
	pixelHeight = max(1, pixelHeight)
	maxInt := int(^uint(0) >> 1)
	if pixelWidth > float64(maxInt) || pixelHeight > float64(maxInt) {
		return canvas{}, fmt.Errorf("%w: output %vx%v exceeds int", ErrImageTooLarge,
			pixelWidth, pixelHeight)
	}
	width := int(pixelWidth)
	height := int(pixelHeight)
	if width > maxInt/height || width*height > maxInt/4 {
		return canvas{}, fmt.Errorf("%w: output %dx%d exceeds image.RGBA storage",
			ErrImageTooLarge, width, height)
	}

	originX := padding - float64(bounds.X)*scale
	originY := padding - float64(bounds.Y)*scale
	if !finite(originX) || !finite(originY) ||
		math.Abs(originX) > math.MaxFloat32 || math.Abs(originY) > math.MaxFloat32 {
		return canvas{}, fmt.Errorf("%w: coordinate transform overflows", ErrImageTooLarge)
	}
	return canvas{
		width: width, height: height,
		scale: scale, originX: originX, originY: originY,
	}, nil
}

func (c canvas) drawPath(rasterizer *vector.Rasterizer, command render.Command) error {
	haveContour := false
	closed := false
	for i, segment := range command.Path.Segments {
		switch segment.Op {
		case geometry.SegMoveTo:
			x, y, err := c.pathPoint(command, segment.Points[0], i, 0)
			if err != nil {
				return err
			}
			if haveContour && !closed {
				rasterizer.ClosePath()
			}
			rasterizer.MoveTo(x, y)
			haveContour = true
			closed = false
		case geometry.SegLineTo:
			x, y, err := c.pathPoint(command, segment.Points[0], i, 0)
			if err != nil {
				return err
			}
			rasterizer.LineTo(x, y)
		case geometry.SegQuadTo:
			x1, y1, err := c.pathPoint(command, segment.Points[0], i, 0)
			if err != nil {
				return err
			}
			x2, y2, err := c.pathPoint(command, segment.Points[1], i, 1)
			if err != nil {
				return err
			}
			rasterizer.QuadTo(x1, y1, x2, y2)
		case geometry.SegCubeTo:
			x1, y1, err := c.pathPoint(command, segment.Points[0], i, 0)
			if err != nil {
				return err
			}
			x2, y2, err := c.pathPoint(command, segment.Points[1], i, 1)
			if err != nil {
				return err
			}
			x3, y3, err := c.pathPoint(command, segment.Points[2], i, 2)
			if err != nil {
				return err
			}
			rasterizer.CubeTo(x1, y1, x2, y2, x3, y3)
		case geometry.SegClose:
			if haveContour && !closed {
				rasterizer.ClosePath()
				closed = true
			}
		default:
			return fmt.Errorf("%w: segment %d has unknown op %d",
				ErrInvalidPath, i, segment.Op)
		}
	}
	if haveContour && !closed {
		rasterizer.ClosePath()
	}
	return nil
}

func (c canvas) pathPoint(
	command render.Command,
	point geometry.Point,
	segmentIndex int,
	pointIndex int,
) (float32, float32, error) {
	x := c.originX + (float64(command.At.X)+float64(point.X)*float64(command.Em))*c.scale
	y := c.originY + (float64(command.At.Y)+float64(point.Y)*float64(command.Em))*c.scale
	return finiteFloat32(x, y, segmentIndex, pointIndex)
}

func (c canvas) drawRule(rasterizer *vector.Rasterizer, rect geometry.Rect) error {
	if rect.Width == 0 || rect.Height == 0 {
		return nil
	}
	left, top, err := c.rulePoint(rect.X, rect.Y)
	if err != nil {
		return err
	}
	right, bottom, err := c.rulePoint(rect.X+rect.Width, rect.Y+rect.Height)
	if err != nil {
		return err
	}
	rasterizer.MoveTo(left, top)
	rasterizer.LineTo(right, top)
	rasterizer.LineTo(right, bottom)
	rasterizer.LineTo(left, bottom)
	rasterizer.ClosePath()
	return nil
}

func (c canvas) rulePoint(x, y float32) (float32, float32, error) {
	placedX := c.originX + float64(x)*c.scale
	placedY := c.originY + float64(y)*c.scale
	return finiteFloat32(placedX, placedY, -1, -1)
}

func finiteFloat32(x, y float64, segmentIndex, pointIndex int) (float32, float32, error) {
	if !finite(x) || !finite(y) ||
		math.Abs(x) > math.MaxFloat32 || math.Abs(y) > math.MaxFloat32 {
		if segmentIndex < 0 {
			return 0, 0, fmt.Errorf("%w: rule placement overflows", ErrInvalidBox)
		}
		return 0, 0, fmt.Errorf("%w: segment %d point %d overflows",
			ErrInvalidPath, segmentIndex, pointIndex)
	}
	return float32(x), float32(y), nil
}

func resolvePaint(paint display.Paint, palette Palette) (color.NRGBA, error) {
	if paint.HasExplicit {
		return color.NRGBA{
			R: paint.Explicit.R,
			G: paint.Explicit.G,
			B: paint.Explicit.B,
			A: paint.Explicit.A,
		}, nil
	}
	switch paint.Role {
	case display.RoleInk:
		return palette.Ink, nil
	case display.RoleError:
		return palette.Error, nil
	case display.RolePlaceholder:
		return palette.Placeholder, nil
	default:
		return color.NRGBA{}, fmt.Errorf("%w: unknown paint role %d",
			ErrInvalidBox, paint.Role)
	}
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

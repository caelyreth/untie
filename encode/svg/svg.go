package svg

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"strconv"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/geometry"
	"github.com/caelyreth/untie/render"
)

var (
	// ErrInvalidBox reports a nil or structurally invalid display box.
	ErrInvalidBox = errors.New("svg: invalid box")
	// ErrInvalidOptions reports unsupported padding or numeric precision.
	ErrInvalidOptions = errors.New("svg: invalid options")
	// ErrInvalidWriter reports a nil output writer.
	ErrInvalidWriter = errors.New("svg: invalid writer")
)

// Palette supplies concrete colors for semantic display roles.
type Palette struct {
	Ink         color.NRGBA
	Error       color.NRGBA
	Placeholder color.NRGBA
}

// DefaultPalette returns the default light-background role colors.
func DefaultPalette() Palette {
	return Palette{
		Ink:         color.NRGBA{A: 0xff},
		Error:       color.NRGBA{R: 0xb4, G: 0x23, B: 0x18, A: 0xff},
		Placeholder: color.NRGBA{R: 0x66, G: 0x70, B: 0x85, A: 0xff},
	}
}

// Options controls SVG sizing, colors, and metadata.
//
// Padding is added on every side of Box.Bounds(). Precision is the maximum
// number of fractional digits; zero emits the shortest exact float32 form.
// A nil Background leaves the background transparent. An all-zero Palette uses
// DefaultPalette. Title and Description are escaped XML text.
type Options struct {
	Padding     float32
	Precision   int
	Background  *color.NRGBA
	Palette     Palette
	Title       string
	Description string
}

// DefaultOptions returns the default encoder options.
func DefaultOptions() Options {
	return Options{
		Precision: 3,
		Palette:   DefaultPalette(),
	}
}

// Marshal encodes box as a standalone SVG document.
//
// A nil options pointer uses DefaultOptions. Marshal does not mutate box.
func Marshal(box *display.Box, options *Options) ([]byte, error) {
	var output bytes.Buffer
	if err := Write(&output, box, options); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// Write encodes box as a standalone SVG document to writer.
//
// A nil options pointer uses DefaultOptions. Writer errors are returned
// unchanged, so an error after writing has begun can leave partial output.
func Write(writer io.Writer, box *display.Box, options *Options) error {
	if writer == nil {
		return fmt.Errorf("%w: nil writer", ErrInvalidWriter)
	}
	opts, err := normalizeOptions(options)
	if err != nil {
		return err
	}
	if box == nil {
		return fmt.Errorf("%w: nil box", ErrInvalidBox)
	}
	if err := box.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBox, err)
	}
	for i, path := range box.Paths {
		if path.Segments[0].Op != geometry.SegMoveTo {
			return fmt.Errorf("%w: path %d does not begin with MoveTo", ErrInvalidBox, i)
		}
	}

	bounds := box.Bounds()
	width := bounds.Width + 2*opts.Padding
	height := bounds.Height + 2*opts.Padding
	if !finite(width) || !finite(height) || width < 0 || height < 0 {
		return fmt.Errorf("%w: output dimensions are not finite", ErrInvalidOptions)
	}
	offset := geometry.Point{
		X: opts.Padding - bounds.X,
		Y: opts.Padding - bounds.Y,
	}
	if !finite(offset.X) || !finite(offset.Y) {
		return fmt.Errorf("%w: output translation is not finite", ErrInvalidOptions)
	}

	encoder := svgEncoder{
		writer:    writer,
		options:   opts,
		offset:    offset,
		width:     width,
		height:    height,
		scratch:   make([]byte, 0, 64),
		pathFirst: true,
	}
	encoder.encode(box)
	return encoder.err
}

type svgEncoder struct {
	writer  io.Writer
	options Options
	offset  geometry.Point
	width   float32
	height  float32
	scratch []byte
	err     error

	pathFirst bool
}

func (e *svgEncoder) encode(box *display.Box) {
	e.writeString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	e.writeString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"")
	e.writeNumber(e.width)
	e.writeString("\" height=\"")
	e.writeNumber(e.height)
	e.writeString("\" viewBox=\"0 0 ")
	e.writeNumber(e.width)
	e.writeByte(' ')
	e.writeNumber(e.height)
	e.writeString("\">\n")
	e.writeMetadata("title", e.options.Title)
	e.writeMetadata("desc", e.options.Description)
	if e.options.Background != nil {
		e.writeString("<rect x=\"0\" y=\"0\" width=\"")
		e.writeNumber(e.width)
		e.writeString("\" height=\"")
		e.writeNumber(e.height)
		e.writeString("\" fill=\"")
		e.writeColor(*e.options.Background)
		e.writeString("\"/>\n")
	}
	if err := render.Walk(box, e.writeCommand); err != nil && e.err == nil {
		e.err = err
	}
	e.writeString("</svg>\n")
}

func (e *svgEncoder) writeMetadata(element, text string) {
	if text == "" {
		return
	}
	e.writeByte('<')
	e.writeString(element)
	e.writeByte('>')
	e.writeEscaped(text)
	e.writeString("</")
	e.writeString(element)
	e.writeString(">\n")
}

func (e *svgEncoder) writeCommand(command render.Command) error {
	if e.err != nil {
		return e.err
	}
	switch command.Kind {
	case render.CommandPath:
		e.writeString("<path d=\"")
		e.pathFirst = true
		for _, segment := range command.Path.Segments {
			e.writeSegment(command, segment)
		}
		e.writeString("\" fill=\"")
		e.writePaint(command.Paint)
		e.writeString("\" fill-rule=\"nonzero\"/>\n")
	case render.CommandRule:
		rect := translateRect(command.Rect, e.offset)
		e.writeString("<rect x=\"")
		e.writeNumber(rect.X)
		e.writeString("\" y=\"")
		e.writeNumber(rect.Y)
		e.writeString("\" width=\"")
		e.writeNumber(rect.Width)
		e.writeString("\" height=\"")
		e.writeNumber(rect.Height)
		e.writeString("\" fill=\"")
		e.writePaint(command.Paint)
		e.writeString("\"/>\n")
	default:
		return fmt.Errorf("%w: unknown render command %d", ErrInvalidBox, command.Kind)
	}
	return e.err
}

func (e *svgEncoder) writeSegment(command render.Command, segment geometry.Segment) {
	var op byte
	var count int
	switch segment.Op {
	case geometry.SegMoveTo:
		op, count = 'M', 1
	case geometry.SegLineTo:
		op, count = 'L', 1
	case geometry.SegQuadTo:
		op, count = 'Q', 2
	case geometry.SegCubeTo:
		op, count = 'C', 3
	case geometry.SegClose:
		if !e.pathFirst {
			e.writeString("Z")
			e.pathFirst = false
		}
		return
	default:
		if e.err == nil {
			e.err = fmt.Errorf("%w: unknown path operation %d", ErrInvalidBox, segment.Op)
		}
		return
	}
	if e.pathFirst && op != 'M' {
		if e.err == nil {
			e.err = fmt.Errorf("%w: path does not begin with MoveTo", ErrInvalidBox)
		}
		return
	}
	if !e.pathFirst {
		e.writeByte(' ')
	}
	e.writeByte(op)
	for i := 0; i < count; i++ {
		point := command.At
		point.X += segment.Points[i].X * command.Em
		point.Y += segment.Points[i].Y * command.Em
		point.X += e.offset.X
		point.Y += e.offset.Y
		e.writeByte(' ')
		e.writeNumber(point.X)
		e.writeByte(' ')
		e.writeNumber(point.Y)
	}
	e.pathFirst = false
}

func (e *svgEncoder) writePaint(paint display.Paint) {
	if paint.HasExplicit {
		e.writeColor(color.NRGBA{
			R: paint.Explicit.R,
			G: paint.Explicit.G,
			B: paint.Explicit.B,
			A: paint.Explicit.A,
		})
		return
	}
	switch paint.Role {
	case display.RoleInk:
		e.writeColor(e.options.Palette.Ink)
	case display.RoleError:
		e.writeColor(e.options.Palette.Error)
	case display.RolePlaceholder:
		e.writeColor(e.options.Palette.Placeholder)
	}
}

func (e *svgEncoder) writeColor(value color.NRGBA) {
	if value.A == 0xff {
		const hex = "0123456789abcdef"
		e.writeByte('#')
		e.writeByte(hex[value.R>>4])
		e.writeByte(hex[value.R&0x0f])
		e.writeByte(hex[value.G>>4])
		e.writeByte(hex[value.G&0x0f])
		e.writeByte(hex[value.B>>4])
		e.writeByte(hex[value.B&0x0f])
		return
	}
	e.writeString("rgba(")
	e.writeUint(uint64(value.R))
	e.writeByte(',')
	e.writeUint(uint64(value.G))
	e.writeByte(',')
	e.writeUint(uint64(value.B))
	e.writeByte(',')
	e.writeNumber(float32(value.A) / 255)
	e.writeByte(')')
}

func (e *svgEncoder) writeEscaped(text string) {
	if e.err != nil {
		return
	}
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(text)); err != nil {
		e.err = err
		return
	}
	e.writeBytes(escaped.Bytes())
}

func (e *svgEncoder) writeNumber(value float32) {
	if value == 0 {
		value = 0
	}
	precision := -1
	if e.options.Precision > 0 {
		precision = e.options.Precision
	}
	e.scratch = strconv.AppendFloat(e.scratch[:0], float64(value), 'f', precision, 32)
	if precision > 0 {
		for len(e.scratch) > 0 && e.scratch[len(e.scratch)-1] == '0' {
			e.scratch = e.scratch[:len(e.scratch)-1]
		}
		if len(e.scratch) > 0 && e.scratch[len(e.scratch)-1] == '.' {
			e.scratch = e.scratch[:len(e.scratch)-1]
		}
	}
	if len(e.scratch) == 0 {
		e.scratch = append(e.scratch, '0')
	}
	e.writeBytes(e.scratch)
}

func (e *svgEncoder) writeUint(value uint64) {
	e.scratch = strconv.AppendUint(e.scratch[:0], value, 10)
	e.writeBytes(e.scratch)
}

func (e *svgEncoder) writeByte(value byte) {
	if e.err != nil {
		return
	}
	var one [1]byte
	one[0] = value
	e.writeBytes(one[:])
}

func (e *svgEncoder) writeString(value string) {
	if e.err != nil {
		return
	}
	count, err := io.WriteString(e.writer, value)
	if err == nil && count != len(value) {
		err = io.ErrShortWrite
	}
	e.err = err
}

func (e *svgEncoder) writeBytes(value []byte) {
	if e.err != nil {
		return
	}
	count, err := e.writer.Write(value)
	if err == nil && count != len(value) {
		err = io.ErrShortWrite
	}
	e.err = err
}

func normalizeOptions(options *Options) (Options, error) {
	if options == nil {
		return DefaultOptions(), nil
	}
	out := *options
	if !finite(out.Padding) || out.Padding < 0 {
		return Options{}, fmt.Errorf("%w: padding %v is not finite and nonnegative",
			ErrInvalidOptions, out.Padding)
	}
	if out.Precision < 0 || out.Precision > 9 {
		return Options{}, fmt.Errorf("%w: precision %d is outside [0,9]",
			ErrInvalidOptions, out.Precision)
	}
	if out.Palette == (Palette{}) {
		out.Palette = DefaultPalette()
	}
	return out, nil
}

func translateRect(rect geometry.Rect, offset geometry.Point) geometry.Rect {
	rect.X += offset.X
	rect.Y += offset.Y
	return rect
}

func finite(value float32) bool {
	number := float64(value)
	return !math.IsNaN(number) && !math.IsInf(number, 0)
}

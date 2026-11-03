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
	return marshal(box, options)
}

// Write encodes box as a standalone SVG document to writer.
//
// A nil options pointer uses DefaultOptions. Writer errors are returned
// unchanged, so an error after writing has begun can leave partial output.
func Write(writer io.Writer, box *display.Box, options *Options) error {
	if writer == nil {
		return fmt.Errorf("%w: nil writer", ErrInvalidWriter)
	}
	output, err := marshal(box, options)
	if err != nil {
		return err
	}
	count, err := writer.Write(output)
	if err == nil && count != len(output) {
		err = io.ErrShortWrite
	}
	return err
}

func marshal(box *display.Box, options *Options) ([]byte, error) {
	opts, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	if box == nil {
		return nil, fmt.Errorf("%w: nil box", ErrInvalidBox)
	}
	if err := box.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidBox, err)
	}
	for i, path := range box.Paths {
		if path.Segments[0].Op != geometry.SegMoveTo {
			return nil, fmt.Errorf("%w: path %d does not begin with MoveTo", ErrInvalidBox, i)
		}
	}

	bounds := box.Bounds()
	width := bounds.Width + 2*opts.Padding
	height := bounds.Height + 2*opts.Padding
	if !finite(width) || !finite(height) || width < 0 || height < 0 {
		return nil, fmt.Errorf("%w: output dimensions are not finite", ErrInvalidOptions)
	}
	offset := geometry.Point{
		X: opts.Padding - bounds.X,
		Y: opts.Padding - bounds.Y,
	}
	if !finite(offset.X) || !finite(offset.Y) {
		return nil, fmt.Errorf("%w: output translation is not finite", ErrInvalidOptions)
	}

	usedPaths := make([]bool, len(box.Paths))
	for _, item := range box.Items {
		if item.Kind == display.ItemGlyphPath {
			usedPaths[item.PathIndex] = true
		}
	}
	capacity := 256 + len(box.Items)*64
	for i, path := range box.Paths {
		if !usedPaths[i] {
			continue
		}
		capacity += len(path.Segments) * 24
	}
	encoder := svgEncoder{
		dst:       make([]byte, 0, capacity),
		options:   opts,
		offset:    offset,
		width:     width,
		height:    height,
		pathFirst: true,
	}
	encoder.encode(box, usedPaths)
	return encoder.dst, encoder.err
}

type svgEncoder struct {
	dst     []byte
	options Options
	offset  geometry.Point
	width   float32
	height  float32
	err     error

	pathFirst  bool
	pathOp     byte
	pathPrefix uint64
}

func (e *svgEncoder) encode(box *display.Box, usedPaths []bool) {
	e.pathPrefix = pathPrefix(box, usedPaths)
	e.writeString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	e.writeString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"")
	e.writeNumber(e.width)
	e.writeString("\" height=\"")
	e.writeNumber(e.height)
	e.writeString("\" viewBox=\"0 0 ")
	e.writeNumber(e.width)
	e.writeByte(' ')
	e.writeNumber(e.height)
	e.writeString("\" fill=\"")
	e.writeColor(e.options.Palette.Ink)
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
	e.writeDefinitions(box, usedPaths)
	for _, item := range box.Items {
		switch item.Kind {
		case display.ItemGlyphPath:
			e.writePathUse(item)
		case display.ItemRule:
			e.writeRule(item)
		case display.ItemGlyphRef:
		default:
			if e.err == nil {
				e.err = fmt.Errorf("%w: unknown item kind %d", ErrInvalidBox, item.Kind)
				return
			}
		}
	}
	e.writeString("</svg>\n")
}

func (e *svgEncoder) writeDefinitions(box *display.Box, used []bool) {
	wrote := false
	for i, path := range box.Paths {
		if !used[i] {
			continue
		}
		if !wrote {
			e.writeString("<defs>\n")
			wrote = true
		}
		e.writeString("<path id=\"")
		e.writePathID(i)
		e.writeString("\" d=\"")
		e.pathFirst = true
		e.pathOp = 0
		for _, segment := range path.Segments {
			e.writeTemplateSegment(segment)
		}
		e.writeString("\"/>\n")
	}
	if wrote {
		e.writeString("</defs>\n")
	}
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

func (e *svgEncoder) writePathUse(item display.Item) {
	e.writeString("<use href=\"#")
	e.writePathID(item.PathIndex)
	e.writeString("\" transform=\"translate(")
	e.writeNumber(item.At.X + e.offset.X)
	e.writeByte(' ')
	e.writeNumber(item.At.Y + e.offset.Y)
	e.writeString(")scale(")
	e.writeNumber(item.Em)
	e.writeByte(')')
	e.writeByte('"')
	e.writeFill(item.Paint)
	e.writeString("/>\n")
}

func (e *svgEncoder) writePathID(index int) {
	e.writeByte('u')
	e.dst = strconv.AppendUint(e.dst, e.pathPrefix, 16)
	e.writeByte('p')
	e.writeUint(uint64(index))
}

func pathPrefix(box *display.Box, used []bool) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	hash := uint64(offset)
	mix := func(value uint64) {
		hash ^= value
		hash *= prime
	}
	for index, path := range box.Paths {
		if !used[index] {
			continue
		}
		mix(uint64(index))
		mix(uint64(len(path.Segments)))
		for _, segment := range path.Segments {
			mix(uint64(segment.Op))
			for _, point := range segment.Points {
				mix(uint64(math.Float32bits(point.X)))
				mix(uint64(math.Float32bits(point.Y)))
			}
		}
	}
	return hash
}

func (e *svgEncoder) writeRule(item display.Item) {
	rect := translateRect(item.Rect, e.offset)
	e.writeString("<rect x=\"")
	e.writeNumber(rect.X)
	e.writeString("\" y=\"")
	e.writeNumber(rect.Y)
	e.writeString("\" width=\"")
	e.writeNumber(rect.Width)
	e.writeString("\" height=\"")
	e.writeNumber(rect.Height)
	e.writeByte('"')
	e.writeFill(item.Paint)
	e.writeString("/>\n")
}

func (e *svgEncoder) writeFill(paint display.Paint) {
	if !paint.HasExplicit && paint.Role == display.RoleInk {
		return
	}
	e.writeString(" fill=\"")
	e.writePaint(paint)
	e.writeByte('"')
}

func (e *svgEncoder) writeTemplateSegment(segment geometry.Segment) {
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
			e.pathOp = 0
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
	wroteCommand := e.pathFirst || op != e.pathOp
	if wroteCommand {
		e.writeByte(op)
	}
	for i := 0; i < count; i++ {
		if (i > 0 || !wroteCommand) &&
			(segment.Points[i].X == 0 || !math.Signbit(float64(segment.Points[i].X))) {
			e.writeByte(' ')
		}
		e.writeNumber(segment.Points[i].X)
		if segment.Points[i].Y == 0 || !math.Signbit(float64(segment.Points[i].Y)) {
			e.writeByte(' ')
		}
		e.writeNumber(segment.Points[i].Y)
	}
	e.pathFirst = false
	e.pathOp = op
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
	if e.options.Precision > 0 &&
		e.options.Precision <= 6 &&
		math.Abs(float64(value)) <= 1_000_000 {
		e.dst = appendFixedFloat32(e.dst, value, e.options.Precision)
		return
	}
	precision := -1
	if e.options.Precision > 0 {
		precision = e.options.Precision
	}
	start := len(e.dst)
	e.dst = strconv.AppendFloat(e.dst, float64(value), 'f', precision, 32)
	if precision > 0 {
		for len(e.dst) > start && e.dst[len(e.dst)-1] == '0' {
			e.dst = e.dst[:len(e.dst)-1]
		}
		if len(e.dst) > start && e.dst[len(e.dst)-1] == '.' {
			e.dst = e.dst[:len(e.dst)-1]
		}
	}
	if len(e.dst) == start {
		e.dst = append(e.dst, '0')
	}
}

func (e *svgEncoder) writeUint(value uint64) {
	e.dst = appendUint(e.dst, value)
}

var fixedScales = [...]uint64{
	1,
	10,
	100,
	1_000,
	10_000,
	100_000,
	1_000_000,
}

func appendFixedFloat32(dst []byte, value float32, precision int) []byte {
	negative := math.Signbit(float64(value))
	if negative {
		value = -value
	}
	if precision == 3 {
		return appendFixed3(dst, value, negative)
	}
	scale := fixedScales[precision]
	rounded := uint64(math.RoundToEven(float64(value) * float64(scale)))
	if negative {
		dst = append(dst, '-')
	}
	dst = appendUint(dst, rounded/scale)
	fraction := rounded % scale
	if fraction == 0 {
		return dst
	}
	dst = append(dst, '.')
	start := len(dst)
	for divisor := scale / 10; divisor > 0; divisor /= 10 {
		dst = append(dst, byte('0'+fraction/divisor))
		fraction %= divisor
	}
	for len(dst) > start && dst[len(dst)-1] == '0' {
		dst = dst[:len(dst)-1]
	}
	return dst
}

func appendFixed3(dst []byte, value float32, negative bool) []byte {
	rounded := uint64(math.RoundToEven(float64(value) * 1_000))
	if negative {
		dst = append(dst, '-')
	}
	whole := rounded / 1_000
	fraction := rounded % 1_000
	if fraction == 0 {
		return appendUint(dst, whole)
	}
	if whole != 0 {
		dst = appendUint(dst, whole)
	}
	dst = append(dst,
		'.',
		byte('0'+fraction/100),
		byte('0'+(fraction/10)%10),
		byte('0'+fraction%10),
	)
	for dst[len(dst)-1] == '0' {
		dst = dst[:len(dst)-1]
	}
	return dst
}

func appendUint(dst []byte, value uint64) []byte {
	if value == 0 {
		return append(dst, '0')
	}
	var digits [20]byte
	index := len(digits)
	for value >= 10 {
		quotient := value / 10
		index--
		digits[index] = byte('0' + value - quotient*10)
		value = quotient
	}
	index--
	digits[index] = byte('0' + value)
	return append(dst, digits[index:]...)
}

func (e *svgEncoder) writeByte(value byte) {
	if e.err != nil {
		return
	}
	e.dst = append(e.dst, value)
}

func (e *svgEncoder) writeString(value string) {
	if e.err != nil {
		return
	}
	e.dst = append(e.dst, value...)
}

func (e *svgEncoder) writeBytes(value []byte) {
	if e.err != nil {
		return
	}
	e.dst = append(e.dst, value...)
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

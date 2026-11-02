package fixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// Frame is Typlet's flattened layout frame, in fixture points.
type Frame struct {
	Box      *FrameBox    `json:"box"`
	FontSize float64      `json:"fontSize"`
	Fonts    []string     `json:"fonts"`
	Glyphs   []FrameGlyph `json:"glyphs"`
	Page     [2]float64   `json:"page"`
	Shapes   []FrameShape `json:"shapes"`
}

// FrameBox is either baseline-anchored (width/ascent/descent) or
// top-anchored (width/height).
type FrameBox struct {
	Anchor  string   `json:"anchor"`
	Width   *float64 `json:"width"`
	Height  *float64 `json:"height,omitempty"`
	Ascent  *float64 `json:"ascent,omitempty"`
	Descent *float64 `json:"descent,omitempty"`
}

// UnmarshalJSON decodes and validates a frame box.
func (b *FrameBox) UnmarshalJSON(data []byte) error {
	if err := rejectUnknownFields(data, "anchor", "width", "height", "ascent", "descent"); err != nil {
		return err
	}
	type plain FrameBox
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*b = FrameBox(decoded)
	return b.validate()
}

func (b *FrameBox) validate() error {
	if b == nil {
		return errors.New("nil frame box")
	}
	if b.Width == nil || !finite(*b.Width) || *b.Width < 0 {
		return errors.New("box width must be a finite non-negative number")
	}
	for name, value := range map[string]*float64{
		"height": b.Height, "ascent": b.Ascent, "descent": b.Descent,
	} {
		if value != nil && (!finite(*value) || *value < 0) {
			return fmt.Errorf("box %s must be a finite non-negative number", name)
		}
	}
	switch b.Anchor {
	case "baseline":
		if b.Height != nil || b.Ascent == nil || b.Descent == nil {
			return errors.New("baseline box requires ascent and descent, and forbids height")
		}
	case "top":
		if b.Height == nil || b.Ascent != nil || b.Descent != nil {
			return errors.New("top box requires height and forbids ascent and descent")
		}
	default:
		return fmt.Errorf("unknown box anchor %q", b.Anchor)
	}
	return nil
}

// FrameGlyph is one entry of Typlet's compact glyph tuple:
// [font, glyph, x, y, size, text, fill?, transform?].
type FrameGlyph struct {
	Font         int
	Glyph        uint32
	X            float64
	Y            float64
	Size         float64
	Text         string
	Fill         string
	HasFill      bool
	Transform    [4]float64
	HasTransform bool
}

// UnmarshalJSON decodes the compact array representation.
func (g *FrameGlyph) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) != 6 && len(values) != 7 && len(values) != 8 {
		return fmt.Errorf("glyph tuple has %d fields; want 6, 7, or 8", len(values))
	}
	font, err := parseInt(values[0], "font")
	if err != nil {
		return err
	}
	id, err := parseInt(values[1], "glyph")
	if err != nil {
		return err
	}
	if id < 0 || id > math.MaxUint16 {
		return fmt.Errorf("glyph ID %d is outside uint16", id)
	}
	x, err := parseNumber(values[2], "x")
	if err != nil {
		return err
	}
	y, err := parseNumber(values[3], "y")
	if err != nil {
		return err
	}
	size, err := parseNumber(values[4], "size")
	if err != nil {
		return err
	}
	var text string
	if err := json.Unmarshal(values[5], &text); err != nil {
		return fmt.Errorf("text: %w", err)
	}

	decoded := FrameGlyph{Font: font, Glyph: uint32(id), X: x, Y: y, Size: size, Text: text}
	if len(values) >= 7 {
		if err := json.Unmarshal(values[6], &decoded.Fill); err != nil {
			return fmt.Errorf("fill: %w", err)
		}
		decoded.HasFill = true
	}
	if len(values) == 8 {
		var transform []float64
		if err := json.Unmarshal(values[7], &transform); err != nil {
			return fmt.Errorf("transform: %w", err)
		}
		if len(transform) != 4 {
			return fmt.Errorf("transform has %d values; want 4", len(transform))
		}
		copy(decoded.Transform[:], transform)
		decoded.HasTransform = true
	}
	*g = decoded
	return g.validate()
}

// MarshalJSON restores the compact array representation.
func (g FrameGlyph) MarshalJSON() ([]byte, error) {
	if err := g.validate(); err != nil {
		return nil, err
	}
	values := []any{g.Font, g.Glyph, g.X, g.Y, g.Size, g.Text}
	if g.HasFill {
		values = append(values, g.Fill)
	}
	if g.HasTransform {
		values = append(values, g.Transform[:])
	}
	return json.Marshal(values)
}

func (g FrameGlyph) validate() error {
	if g.Font < 0 {
		return errors.New("glyph font index must be non-negative")
	}
	if g.Glyph > math.MaxUint16 {
		return fmt.Errorf("glyph ID %d exceeds uint16", g.Glyph)
	}
	if !finite(g.X) || !finite(g.Y) || !finite(g.Size) || g.Size < 0 {
		return errors.New("glyph position and size must be finite, and size non-negative")
	}
	if g.HasFill && g.Fill == "" {
		return errors.New("glyph fill is empty")
	}
	if g.HasTransform {
		if !g.HasFill {
			return errors.New("explicit glyph transform requires its explicit fill field")
		}
		for _, value := range g.Transform {
			if !finite(value) {
				return errors.New("glyph transform contains a non-finite value")
			}
		}
	}
	return nil
}

// ShapeStroke is a fixture stroke: color and thickness in points.
type ShapeStroke struct {
	Color     string
	Thickness float64
}

// MarshalJSON restores the compact [color, thickness] representation.
func (s ShapeStroke) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{s.Color, s.Thickness})
}

// FrameShape is one flattened line, rectangle, or box shape.
type FrameShape struct {
	Line    []float64
	Rect    []float64
	Box     []float64
	Fill    string
	HasFill bool
	Stroke  *ShapeStroke
}

// UnmarshalJSON decodes and validates a frame shape.
func (s *FrameShape) UnmarshalJSON(data []byte) error {
	if err := rejectUnknownFields(data, "line", "rect", "box", "fill", "stroke"); err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	geometryCount := 0
	var decoded FrameShape
	for _, name := range []string{"line", "rect", "box"} {
		raw, ok := object[name]
		if !ok {
			continue
		}
		geometryCount++
		var values []float64
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		switch name {
		case "line":
			decoded.Line = values
		case "rect":
			decoded.Rect = values
		case "box":
			decoded.Box = values
		}
	}
	if geometryCount != 1 {
		return fmt.Errorf("shape has %d geometries; want exactly one", geometryCount)
	}
	if raw, ok := object["fill"]; ok {
		if err := json.Unmarshal(raw, &decoded.Fill); err != nil {
			return fmt.Errorf("fill: %w", err)
		}
		decoded.HasFill = true
	}
	if raw, ok := object["stroke"]; ok {
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("stroke: %w", err)
		}
		if len(values) != 2 {
			return fmt.Errorf("stroke has %d values; want 2", len(values))
		}
		var stroke ShapeStroke
		if err := json.Unmarshal(values[0], &stroke.Color); err != nil {
			return fmt.Errorf("stroke color: %w", err)
		}
		thickness, err := parseNumber(values[1], "stroke thickness")
		if err != nil {
			return err
		}
		stroke.Thickness = thickness
		decoded.Stroke = &stroke
	}
	*s = decoded
	return s.validate()
}

// MarshalJSON restores a deterministic frame shape object.
func (s FrameShape) MarshalJSON() ([]byte, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	wire := struct {
		Box    []float64    `json:"box,omitempty"`
		Fill   *string      `json:"fill,omitempty"`
		Line   []float64    `json:"line,omitempty"`
		Rect   []float64    `json:"rect,omitempty"`
		Stroke *ShapeStroke `json:"stroke,omitempty"`
	}{
		Box: s.Box, Line: s.Line, Rect: s.Rect, Stroke: s.Stroke,
	}
	if s.HasFill {
		wire.Fill = &s.Fill
	}
	return json.Marshal(wire)
}

func (s FrameShape) validate() error {
	count := 0
	for name, values := range map[string][]float64{"line": s.Line, "rect": s.Rect, "box": s.Box} {
		if values == nil {
			continue
		}
		count++
		if len(values) != 4 {
			return fmt.Errorf("%s has %d values; want 4", name, len(values))
		}
		for _, value := range values {
			if !finite(value) {
				return fmt.Errorf("%s contains a non-finite value", name)
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("shape has %d geometries; want exactly one", count)
	}
	if s.HasFill && s.Fill == "" {
		return errors.New("shape fill is empty")
	}
	if s.Stroke != nil {
		if s.Stroke.Color == "" {
			return errors.New("shape stroke color is empty")
		}
		if !finite(s.Stroke.Thickness) || s.Stroke.Thickness < 0 {
			return errors.New("shape stroke thickness must be finite and non-negative")
		}
	}
	return nil
}

// MarshalJSON makes the custom array and object encoders reachable through
// encoding/json without losing the outer frame shape.
func (f Frame) MarshalJSON() ([]byte, error) {
	type plain Frame
	return json.Marshal(plain(f))
}

// UnmarshalJSON decodes a frame and validates that every glyph references a
// declared font.
func (f *Frame) UnmarshalJSON(data []byte) error {
	if err := rejectUnknownFields(data, "box", "fontSize", "fonts", "glyphs", "page", "shapes"); err != nil {
		return err
	}
	type plain Frame
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*f = Frame(decoded)
	return f.Validate()
}

// Validate checks frame invariants needed by the fixture adapter and diff.
func (f Frame) Validate() error {
	if _, err := FrameToDIP(f); err != nil {
		return err
	}
	if f.Box == nil {
		return errors.New("frame has no box")
	}
	if err := f.Box.validate(); err != nil {
		return err
	}
	if !(f.FontSize > 0) {
		return errors.New("fontSize must be positive")
	}
	for i, value := range f.Page {
		if !finite(value) || value < 0 {
			return fmt.Errorf("page[%d] must be a finite non-negative number", i)
		}
	}
	for i, glyph := range f.Glyphs {
		if err := glyph.validate(); err != nil {
			return fmt.Errorf("glyph %d: %w", i, err)
		}
		if glyph.Font >= len(f.Fonts) {
			return fmt.Errorf("glyph %d references font %d of %d", i, glyph.Font, len(f.Fonts))
		}
	}
	for i, shape := range f.Shapes {
		if err := shape.validate(); err != nil {
			return fmt.Errorf("shape %d: %w", i, err)
		}
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// ShapeGeometry reports which geometry a shape carries.
func (s FrameShape) ShapeGeometry() string {
	switch {
	case s.Line != nil:
		return "line"
	case s.Rect != nil:
		return "rect"
	case s.Box != nil:
		return "box"
	default:
		return ""
	}
}

// ShapeGeometryValues returns the selected geometry values.
func (s FrameShape) ShapeGeometryValues() []float64 {
	switch {
	case s.Line != nil:
		return s.Line
	case s.Rect != nil:
		return s.Rect
	case s.Box != nil:
		return s.Box
	default:
		return nil
	}
}

var _ json.Unmarshaler = (*Frame)(nil)
var _ json.Marshaler = Frame{}

package fixture

import "fmt"

// GlyphPlacement is one already-positioned glyph in fixture points. Font is a
// face name because Typlet fixture glyphs refer to faces by an index.
type GlyphPlacement struct {
	Font      string
	Glyph     uint32
	X         float64
	Y         float64
	Size      float64
	Text      string
	Fill      *string
	Transform *[4]float64
}

// FrameLayout is an already-flattened paint-order glyph and shape stream. It is
// the fixture-side interface used by layout tests before encoding to JSON.
type FrameLayout struct {
	Box      FrameBox
	FontSize float64
	Page     [2]float64
	Glyphs   []GlyphPlacement
	Shapes   []FrameShape
}

// FlattenFrame converts named fonts to Typlet's first-use font indices and
// validates the resulting fixture frame. It preserves glyph and shape order.
func FlattenFrame(layout FrameLayout) (Frame, error) {
	fonts := make([]string, 0, 2)
	fontIndex := make(map[string]int)
	glyphs := make([]FrameGlyph, len(layout.Glyphs))
	for i, placement := range layout.Glyphs {
		if placement.Font == "" {
			return Frame{}, fmt.Errorf("glyph %d has an empty font name", i)
		}
		index, ok := fontIndex[placement.Font]
		if !ok {
			index = len(fonts)
			fontIndex[placement.Font] = index
			fonts = append(fonts, placement.Font)
		}
		glyph := FrameGlyph{
			Font: index, Glyph: placement.Glyph,
			X: placement.X, Y: placement.Y, Size: placement.Size,
			Text: placement.Text,
		}
		if placement.Fill != nil {
			glyph.Fill = *placement.Fill
			glyph.HasFill = true
		}
		if placement.Transform != nil {
			if placement.Fill == nil {
				// Typlet always writes the fill field when it writes a
				// transform. Black is the fixture's default fill.
				glyph.Fill = "#000000"
				glyph.HasFill = true
			}
			glyph.Transform = *placement.Transform
			glyph.HasTransform = true
		}
		glyphs[i] = glyph
	}
	frame := Frame{
		Box:      &layout.Box,
		FontSize: layout.FontSize,
		Fonts:    fonts,
		Glyphs:   glyphs,
		Page:     layout.Page,
		Shapes:   append([]FrameShape(nil), layout.Shapes...),
	}
	if err := frame.Validate(); err != nil {
		return Frame{}, err
	}
	return frame, nil
}

// Validate checks a frame layout without encoding it.
func (l FrameLayout) Validate() error {
	_, err := FlattenFrame(l)
	return err
}

package fixture

import (
	"encoding/json"
	"fmt"
	"slices"
)

// FontDump is Typlet's NewCM metric and shaping dump.
type FontDump struct {
	Header *FontHeader
	Glyphs []FontGlyph
	CMap   *FontCMap
	Shapes []FontShape
}

// FontHeader holds face-wide metrics and the OpenType MATH constants observed
// by Typst. MATH constants are in ems; glyph metrics are in font design units.
type FontHeader struct {
	Kind                string             `json:"kind"`
	GlyphCount          int                `json:"glyphCount"`
	GSUBFeatures        []string           `json:"gsubFeatures"`
	Math                map[string]float64 `json:"math"`
	Metrics             FontMetrics        `json:"metrics"`
	MinConnectorOverlap int                `json:"minConnectorOverlap"`
	PostScriptName      string             `json:"postScriptName"`
	UnitsPerEm          int                `json:"unitsPerEm"`
}

// FontMetrics is the text metric subset recorded by Typlet.
type FontMetrics struct {
	Ascender      float64          `json:"ascender"`
	CapHeight     float64          `json:"capHeight"`
	Descender     float64          `json:"descender"`
	Overline      FontLineMetric   `json:"overline"`
	Strikethrough FontLineMetric   `json:"strikethrough"`
	Subscript     FontScriptMetric `json:"subscript"`
	Superscript   FontScriptMetric `json:"superscript"`
	Underline     FontLineMetric   `json:"underline"`
	XHeight       float64          `json:"xHeight"`
}

// FontLineMetric is a decoration position and thickness.
type FontLineMetric struct {
	Position  float64 `json:"position"`
	Thickness float64 `json:"thickness"`
}

// FontScriptMetric is an OpenType script-line metric.
type FontScriptMetric struct {
	Height           float64 `json:"height"`
	HorizontalOffset float64 `json:"horizontalOffset"`
	VerticalOffset   float64 `json:"verticalOffset"`
	Width            float64 `json:"width"`
}

// FontGlyph is one glyph record.
type FontGlyph struct {
	Kind       string              `json:"kind"`
	ID         uint16              `json:"id"`
	Advance    *float64            `json:"advance,omitempty"`
	BBox       *[4]float64         `json:"bbox,omitempty"`
	Italic     *float64            `json:"italic,omitempty"`
	Accent     *float64            `json:"accent,omitempty"`
	Extended   bool                `json:"extended,omitempty"`
	Vertical   *GlyphConstruction  `json:"vertical,omitempty"`
	Horizontal *GlyphConstruction  `json:"horizontal,omitempty"`
	Kern       map[string]FontKern `json:"kern,omitempty"`
}

// FontKern is one OpenType MATH kern table.
type FontKern struct {
	Heights []float64 `json:"heights"`
	Kerns   []float64 `json:"kerns"`
}

// GlyphConstruction holds size variants and an optional assembly.
type GlyphConstruction struct {
	Variants [][2]int      `json:"variants"`
	Assembly *FontAssembly `json:"assembly"`
}

// FontAssembly is a MATH glyph assembly.
type FontAssembly struct {
	Italic float64            `json:"italic"`
	Parts  []FontAssemblyPart `json:"parts"`
}

// FontAssemblyPart is one connector part of an assembly.
type FontAssemblyPart struct {
	Glyph          uint16
	StartConnector float64
	EndConnector   float64
	FullAdvance    float64
	Extender       bool
}

// UnmarshalJSON decodes Typst's compact five-element assembly part tuple.
func (p *FontAssemblyPart) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) != 5 {
		return fmt.Errorf("assembly part has %d values; want 5", len(values))
	}
	glyph, err := parseInt(values[0], "assembly glyph")
	if err != nil {
		return err
	}
	start, err := parseNumber(values[1], "assembly start connector")
	if err != nil {
		return err
	}
	end, err := parseNumber(values[2], "assembly end connector")
	if err != nil {
		return err
	}
	advance, err := parseNumber(values[3], "assembly full advance")
	if err != nil {
		return err
	}
	var extender bool
	if err := json.Unmarshal(values[4], &extender); err != nil {
		return fmt.Errorf("assembly extender: %w", err)
	}
	if glyph < 0 || glyph > 0xffff {
		return fmt.Errorf("assembly glyph %d is outside uint16", glyph)
	}
	if !finite(start) || !finite(end) || !finite(advance) {
		return fmt.Errorf("assembly part contains a non-finite metric")
	}
	*p = FontAssemblyPart{
		Glyph: uint16(glyph), StartConnector: start, EndConnector: end,
		FullAdvance: advance, Extender: extender,
	}
	return nil
}

// FontCMap is the font's character and variation-sequence mapping.
type FontCMap struct {
	Kind      string      `json:"kind"`
	Chars     []CMapEntry `json:"chars"`
	Sequences []CMapEntry `json:"sequences"`
}

// CMapEntry maps text to one glyph ID.
type CMapEntry struct {
	Text  string
	Glyph uint16
}

// UnmarshalJSON decodes a [text, glyph] tuple.
func (e *CMapEntry) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) != 2 {
		return fmt.Errorf("cmap entry has %d values; want 2", len(values))
	}
	var text string
	if err := json.Unmarshal(values[0], &text); err != nil {
		return fmt.Errorf("cmap text: %w", err)
	}
	glyph, err := parseInt(values[1], "cmap glyph")
	if err != nil {
		return err
	}
	if glyph < 0 || glyph > 0xffff {
		return fmt.Errorf("cmap glyph %d is outside uint16", glyph)
	}
	*e = CMapEntry{Text: text, Glyph: uint16(glyph)}
	return nil
}

// FontShape is one HarfBuzz shaping result observed by Typst.
type FontShape struct {
	Kind     string              `json:"kind"`
	Features []string            `json:"features"`
	Results  []FontShapingResult `json:"results"`
}

// FontShapingResult maps input text to an output glyph run.
type FontShapingResult struct {
	Text   string
	Glyphs []ShapedGlyph
}

// UnmarshalJSON decodes a [text, glyph run] tuple.
func (r *FontShapingResult) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) != 2 {
		return fmt.Errorf("shaping result has %d values; want 2", len(values))
	}
	var text string
	if err := json.Unmarshal(values[0], &text); err != nil {
		return fmt.Errorf("shaping text: %w", err)
	}
	var runs [][]int
	if err := json.Unmarshal(values[1], &runs); err != nil {
		return fmt.Errorf("shaping glyph run: %w", err)
	}
	glyphs := make([]ShapedGlyph, len(runs))
	for i, run := range runs {
		if len(run) != 5 {
			return fmt.Errorf("shaping glyph %d has %d values; want 5", i, len(run))
		}
		if run[0] < 0 || run[0] > 0xffff {
			return fmt.Errorf("shaping glyph %d ID %d is outside uint16", i, run[0])
		}
		glyphs[i] = ShapedGlyph{
			ID: uint16(run[0]), XAdvance: run[1], XOffset: run[2],
			YOffset: run[3], YAdvance: run[4],
		}
	}
	r.Text = text
	r.Glyphs = glyphs
	return nil
}

// ShapedGlyph is one glyph and its shaping displacement in design units.
type ShapedGlyph struct {
	ID       uint16
	XAdvance int
	XOffset  int
	YOffset  int
	YAdvance int
}

// LoadFontDump decodes a Typlet font JSON Lines dump.
func LoadFontDump(path string) (*FontDump, error) {
	dump := &FontDump{}
	glyphIDs := make(map[uint16]int)
	err := readJSONLines(path, func(line int, data []byte) error {
		var envelope struct {
			Kind string `json:"kind"`
		}
		if err := decodeJSONObject(data, &envelope, false); err != nil {
			return err
		}
		switch envelope.Kind {
		case "font":
			if dump.Header != nil {
				return fmt.Errorf("duplicate font header")
			}
			var header FontHeader
			if err := decodeJSONObject(data, &header, true); err != nil {
				return err
			}
			dump.Header = &header
		case "glyph":
			var glyph FontGlyph
			if err := decodeJSONObject(data, &glyph, true); err != nil {
				return err
			}
			if previous, ok := glyphIDs[glyph.ID]; ok {
				return fmt.Errorf("duplicate glyph ID %d (first at record %d)", glyph.ID, previous)
			}
			glyphIDs[glyph.ID] = line
			dump.Glyphs = append(dump.Glyphs, glyph)
		case "cmap":
			if dump.CMap != nil {
				return fmt.Errorf("duplicate cmap record")
			}
			var cmap FontCMap
			if err := decodeJSONObject(data, &cmap, true); err != nil {
				return err
			}
			dump.CMap = &cmap
		case "shape":
			var shape FontShape
			if err := decodeJSONObject(data, &shape, true); err != nil {
				return err
			}
			dump.Shapes = append(dump.Shapes, shape)
		default:
			return fmt.Errorf("unknown font record kind %q", envelope.Kind)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := dump.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return dump, nil
}

// Validate checks the font dump's structural invariants.
func (d *FontDump) Validate() error {
	if d.Header == nil {
		return fmt.Errorf("missing font header")
	}
	h := d.Header
	if h.Kind != "font" || h.PostScriptName == "" {
		return fmt.Errorf("invalid font header identity")
	}
	if h.UnitsPerEm <= 0 {
		return fmt.Errorf("unitsPerEm must be positive")
	}
	if h.MinConnectorOverlap < 0 {
		return fmt.Errorf("minConnectorOverlap must be non-negative")
	}
	if h.GlyphCount <= 0 || h.GlyphCount != len(d.Glyphs) {
		return fmt.Errorf("header glyphCount %d does not match %d glyph records", h.GlyphCount, len(d.Glyphs))
	}
	if !slices.Contains(h.GSUBFeatures, "ssty") {
		return fmt.Errorf("font has no ssty GSUB feature")
	}
	if !slices.Contains(h.GSUBFeatures, "dtls") {
		return fmt.Errorf("font has no dtls GSUB feature")
	}
	for name, value := range h.Math {
		if name == "" || !finite(value) {
			return fmt.Errorf("invalid MATH constant %q", name)
		}
	}
	if err := validateFontMetrics(h.Metrics); err != nil {
		return err
	}
	for i, glyph := range d.Glyphs {
		if glyph.Kind != "glyph" {
			return fmt.Errorf("glyph %d has kind %q", i, glyph.Kind)
		}
		if int(glyph.ID) >= h.GlyphCount {
			return fmt.Errorf("glyph record ID %d is outside glyphCount %d", glyph.ID, h.GlyphCount)
		}
		if glyph.Advance != nil && (!finite(*glyph.Advance) || *glyph.Advance < 0) {
			return fmt.Errorf("glyph %d has invalid advance", glyph.ID)
		}
		if glyph.Italic != nil && !finite(*glyph.Italic) {
			return fmt.Errorf("glyph %d has non-finite italic correction", glyph.ID)
		}
		if glyph.Accent != nil && !finite(*glyph.Accent) {
			return fmt.Errorf("glyph %d has non-finite accent attachment", glyph.ID)
		}
		if glyph.BBox != nil {
			for _, value := range glyph.BBox {
				if !finite(value) {
					return fmt.Errorf("glyph %d has non-finite bbox", glyph.ID)
				}
			}
		}
		for corner, kern := range glyph.Kern {
			if corner != "topRight" && corner != "bottomRight" {
				return fmt.Errorf("glyph %d has unknown kern corner %q", glyph.ID, corner)
			}
			if len(kern.Kerns) != len(kern.Heights)+1 {
				return fmt.Errorf("glyph %d kern %s has %d kerns for %d heights", glyph.ID, corner, len(kern.Kerns), len(kern.Heights))
			}
			for i := range kern.Heights {
				if !finite(kern.Heights[i]) || (i > 0 && kern.Heights[i] <= kern.Heights[i-1]) {
					return fmt.Errorf("glyph %d kern %s heights are not strictly increasing", glyph.ID, corner)
				}
			}
			for i, value := range kern.Kerns {
				if !finite(value) {
					return fmt.Errorf("glyph %d kern %s value %d is non-finite", glyph.ID, corner, i)
				}
			}
		}
		if err := validateConstruction(glyph.ID, h.GlyphCount, "vertical", glyph.Vertical); err != nil {
			return err
		}
		if err := validateConstruction(glyph.ID, h.GlyphCount, "horizontal", glyph.Horizontal); err != nil {
			return err
		}
	}
	if d.CMap == nil {
		return fmt.Errorf("missing cmap record")
	}
	if d.CMap.Kind != "cmap" || len(d.CMap.Chars) == 0 {
		return fmt.Errorf("invalid cmap record")
	}
	for _, group := range []struct {
		name    string
		entries []CMapEntry
	}{{"chars", d.CMap.Chars}, {"sequences", d.CMap.Sequences}} {
		seen := make(map[string]struct{}, len(group.entries))
		for j, entry := range group.entries {
			if int(entry.Glyph) >= h.GlyphCount {
				return fmt.Errorf("cmap %s entry %d maps to glyph %d outside glyphCount %d", group.name, j, entry.Glyph, h.GlyphCount)
			}
			if _, ok := seen[entry.Text]; ok {
				return fmt.Errorf("cmap %s entry %d repeats text %q", group.name, j, entry.Text)
			}
			seen[entry.Text] = struct{}{}
		}
	}
	for i, shape := range d.Shapes {
		if shape.Kind != "shape" {
			return fmt.Errorf("shape %d has kind %q", i, shape.Kind)
		}
		for j, result := range shape.Results {
			for k, glyph := range result.Glyphs {
				if int(glyph.ID) >= h.GlyphCount {
					return fmt.Errorf("shape %d result %d glyph %d maps to glyph %d outside glyphCount %d", i, j, k, glyph.ID, h.GlyphCount)
				}
			}
		}
	}
	return nil
}

func validateConstruction(glyph uint16, glyphCount int, direction string, construction *GlyphConstruction) error {
	if construction == nil {
		return nil
	}
	for i, variant := range construction.Variants {
		if variant[0] < 0 || variant[0] > 0xffff || variant[0] >= glyphCount {
			return fmt.Errorf("glyph %d %s variant %d glyph %d is outside glyphCount %d", glyph, direction, i, variant[0], glyphCount)
		}
		if !finite(float64(variant[1])) {
			return fmt.Errorf("glyph %d %s variant %d advance is non-finite", glyph, direction, i)
		}
	}
	if construction.Assembly == nil {
		return nil
	}
	if len(construction.Assembly.Parts) == 0 {
		return fmt.Errorf("glyph %d %s assembly is empty", glyph, direction)
	}
	if !finite(construction.Assembly.Italic) {
		return fmt.Errorf("glyph %d %s assembly italic correction is non-finite", glyph, direction)
	}
	for i, part := range construction.Assembly.Parts {
		if int(part.Glyph) >= glyphCount {
			return fmt.Errorf("glyph %d %s assembly part %d glyph %d is outside glyphCount %d", glyph, direction, i, part.Glyph, glyphCount)
		}
		if !finite(part.StartConnector) || !finite(part.EndConnector) || !finite(part.FullAdvance) {
			return fmt.Errorf("glyph %d %s assembly part %d is non-finite", glyph, direction, i)
		}
	}
	return nil
}

func validateFontMetrics(metrics FontMetrics) error {
	values := map[string]float64{
		"ascender": metrics.Ascender, "capHeight": metrics.CapHeight,
		"descender": metrics.Descender, "xHeight": metrics.XHeight,
		"overline.position": metrics.Overline.Position, "overline.thickness": metrics.Overline.Thickness,
		"strikethrough.position": metrics.Strikethrough.Position, "strikethrough.thickness": metrics.Strikethrough.Thickness,
		"underline.position": metrics.Underline.Position, "underline.thickness": metrics.Underline.Thickness,
		"subscript.height": metrics.Subscript.Height, "subscript.horizontalOffset": metrics.Subscript.HorizontalOffset,
		"subscript.verticalOffset": metrics.Subscript.VerticalOffset, "subscript.width": metrics.Subscript.Width,
		"superscript.height": metrics.Superscript.Height, "superscript.horizontalOffset": metrics.Superscript.HorizontalOffset,
		"superscript.verticalOffset": metrics.Superscript.VerticalOffset, "superscript.width": metrics.Superscript.Width,
	}
	for name, value := range values {
		if !finite(value) {
			return fmt.Errorf("font metric %s is non-finite", name)
		}
	}
	for name, value := range map[string]float64{
		"overline.thickness":      metrics.Overline.Thickness,
		"strikethrough.thickness": metrics.Strikethrough.Thickness,
		"underline.thickness":     metrics.Underline.Thickness,
	} {
		if value < 0 {
			return fmt.Errorf("font metric %s is negative", name)
		}
	}
	return nil
}

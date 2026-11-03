// Package font defines host-supplied font inputs and parsed font sets.
package font

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// FontFace is one face from a host-provided SFNT or TTC file.
//
// ID is optional when the face is passed to LoadFontSet, LoadMathFont, or
// another high-level source helper. Empty IDs are derived deterministically.
type FontFace struct {
	ID    string
	Bytes []byte
	Index int
}

// FontSetConfig configures a validated font collection. Math must contain
// OpenType MATH constants. Fallback order is deterministic.
type FontSetConfig struct {
	Math      FontFace
	Text      *FontFace
	Fallbacks []FontFace
}

// FontOption configures one face passed to LoadMathFont.
type FontOption func(*FontFace)

// WithFontID sets an explicit stable face ID. A blank ID is treated like an
// omitted ID and is derived by LoadMathFont.
func WithFontID(id string) FontOption {
	return func(face *FontFace) {
		face.ID = id
	}
}

// WithFontIndex selects a face in a TrueType/OpenType collection.
func WithFontIndex(index int) FontOption {
	return func(face *FontFace) {
		face.Index = index
	}
}

// FontData is one face from a host-provided SFNT or TTC file.
//
// New integrations should use FontFace.
type FontData struct {
	Bytes []byte
	Index int
	ID    string
}

// Fonts are host-supplied faces. Fallback order is deterministic.
//
// New integrations should use FontSetConfig.
type Fonts struct {
	Math      FontData
	Text      *FontData
	Fallbacks []FontData
}

// FontSet is an immutable parsed font collection with a concurrency-safe glyph
// outline cache.
type FontSet struct {
	fonts   Fonts
	faces   []*parsedFace
	hasText bool
	key     [32]byte
}

// Load validates and prepares host fonts. It accepts SFNT and TTC data. The
// primary math face must contain OpenType MATH constants. All font bytes are
// copied before parsing, and every glyph outline is validated here so later
// layout cannot fail because of a malformed glyph program.
//
// New integrations should use LoadFontSet or LoadMathFont.
func Load(fonts Fonts) (set *FontSet, err error) {
	return load(fonts, false)
}

// LoadMathFont prepares one math face. The face ID is derived when opts do not
// provide one. The face must contain OpenType MATH constants.
func LoadMathFont(data []byte, opts ...FontOption) (*FontSet, error) {
	face := FontFace{Bytes: data}
	for _, opt := range opts {
		if opt != nil {
			opt(&face)
		}
	}
	return LoadFontSet(FontSetConfig{Math: face})
}

// LoadFontSet validates and prepares host fonts. It accepts SFNT and TTC data.
// Empty face IDs are derived deterministically. Explicit IDs must be nonblank
// and unique. All font bytes are copied before parsing.
func LoadFontSet(config FontSetConfig) (*FontSet, error) {
	return load(config.toLegacy(), true)
}

func (config FontSetConfig) toLegacy() Fonts {
	fonts := Fonts{Math: fontData(config.Math)}
	if config.Text != nil {
		text := fontData(*config.Text)
		fonts.Text = &text
	}
	fonts.Fallbacks = make([]FontData, len(config.Fallbacks))
	for i, face := range config.Fallbacks {
		fonts.Fallbacks[i] = fontData(face)
	}
	return fonts
}

func fontData(face FontFace) FontData {
	return FontData{Bytes: face.Bytes, Index: face.Index, ID: face.ID}
}

func load(fonts Fonts, deriveIDs bool) (set *FontSet, err error) {
	defer func() {
		if r := recover(); r != nil {
			set = nil
			err = fmt.Errorf("font: malformed input: %v", r)
		}
	}()

	if len(fonts.Math.Bytes) == 0 {
		return nil, errors.New("font: math face is required")
	}
	if len(fonts.Fallbacks) > maxFallbackFaces {
		return nil, fmt.Errorf("font: %d fallback faces exceed limit %d", len(fonts.Fallbacks), maxFallbackFaces)
	}
	if err := validateExplicitFontIDs(fonts, deriveIDs); err != nil {
		return nil, err
	}

	owned := Fonts{Math: copyFontData(fonts.Math)}
	hasText := fonts.Text != nil
	if hasText {
		text := copyFontData(*fonts.Text)
		owned.Text = &text
	}
	owned.Fallbacks = make([]FontData, len(fonts.Fallbacks))
	for i, face := range fonts.Fallbacks {
		owned.Fallbacks[i] = copyFontData(face)
	}

	faces := make([]*parsedFace, 0, 1+len(owned.Fallbacks)+boolInt(hasText))
	mathInput := owned.Math
	mathFace, err := parseSelectedFace(labelFace(mathInput, "math"))
	if err != nil {
		return nil, fmt.Errorf("font: math face: %w", err)
	}
	restoreFaceID(mathFace, mathInput)
	if !mathFace.info.HasMath || !mathFace.math.hasConstants {
		return nil, errors.New("font: primary math face has no OpenType MATH constants")
	}
	faces = append(faces, mathFace)
	if hasText {
		textInput := *owned.Text
		textFace, err := parseSelectedFace(labelFace(textInput, "text"))
		if err != nil {
			return nil, fmt.Errorf("font: text face: %w", err)
		}
		restoreFaceID(textFace, textInput)
		faces = append(faces, textFace)
	}
	for i, data := range owned.Fallbacks {
		face, err := parseSelectedFace(labelFace(data, fmt.Sprintf("fallback-%d", i)))
		if err != nil {
			return nil, fmt.Errorf("font: fallback face %d: %w", i, err)
		}
		restoreFaceID(face, data)
		faces = append(faces, face)
	}
	if deriveIDs {
		if err := resolveFontIDs(&owned, faces); err != nil {
			return nil, err
		}
	}

	set = &FontSet{
		fonts:   owned,
		faces:   faces,
		hasText: hasText,
	}
	set.key = canonicalFontKey(owned)
	return set, nil
}

func validateExplicitFontIDs(fonts Fonts, allowEmpty bool) error {
	ids := make(map[string]struct{}, 1+len(fonts.Fallbacks)+boolInt(fonts.Text != nil))
	check := func(role string, face FontData) error {
		if strings.TrimSpace(face.ID) == "" {
			if allowEmpty {
				return nil
			}
			return fmt.Errorf("font: %s face has empty ID", role)
		}
		if _, exists := ids[face.ID]; exists {
			return fmt.Errorf("font: duplicate face ID %q", face.ID)
		}
		ids[face.ID] = struct{}{}
		return nil
	}
	if err := check("math", fonts.Math); err != nil {
		return err
	}
	if fonts.Text != nil {
		if err := check("text", *fonts.Text); err != nil {
			return err
		}
	}
	for i, face := range fonts.Fallbacks {
		if err := check(fmt.Sprintf("fallback %d", i), face); err != nil {
			return err
		}
	}
	return nil
}

func resolveFontIDs(fonts *Fonts, faces []*parsedFace) error {
	ids := make(map[string]struct{}, len(faces))
	for i, face := range faces {
		if strings.TrimSpace(face.data.ID) == "" {
			role := faceRole(i, fonts.Text != nil)
			face.data.ID = derivedFaceID(role, face)
			face.info.ID = face.data.ID
		}
		if _, exists := ids[face.data.ID]; exists {
			return fmt.Errorf("font: duplicate face ID %q", face.data.ID)
		}
		ids[face.data.ID] = struct{}{}
	}
	fonts.Math = faces[0].data
	if fonts.Text != nil {
		text := faces[1].data
		fonts.Text = &text
	}
	for i := range fonts.Fallbacks {
		faceIndex := i + 1
		if fonts.Text != nil {
			faceIndex++
		}
		fonts.Fallbacks[i] = faces[faceIndex].data
	}
	return nil
}

func faceRole(index int, hasText bool) string {
	switch {
	case index == 0:
		return "math"
	case index == 1 && hasText:
		return "text"
	}
	if hasText {
		index--
	}
	return fmt.Sprintf("fallback-%d", index-1)
}

func derivedFaceID(role string, face *parsedFace) string {
	h := sha256.New()
	_, _ = h.Write([]byte("untie-font-face-id\x00v1\x00"))
	_, _ = h.Write([]byte(role))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(face.info.PostScriptName))
	_, _ = h.Write([]byte{0})
	var index [8]byte
	binary.BigEndian.PutUint64(index[:], uint64(face.data.Index))
	_, _ = h.Write(index[:])
	_, _ = h.Write(face.data.Bytes)
	sum := h.Sum(nil)

	name := safeIDSegment(face.info.PostScriptName)
	if name == "" {
		name = "face"
	}
	return role + "-" + name + "-" + hex.EncodeToString(sum[:8])
}

func safeIDSegment(value string) string {
	const maxLength = 32
	var out strings.Builder
	out.Grow(min(len(value), maxLength))
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			r += 'a' - 'A'
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		default:
			if lastDash || out.Len() == 0 {
				continue
			}
			r = '-'
		}
		if r == '-' {
			lastDash = true
		} else {
			lastDash = false
		}
		if out.Len()+1 > maxLength {
			break
		}
		out.WriteRune(r)
	}
	return strings.Trim(out.String(), "-")
}

func labelFace(face FontData, role string) FontData {
	if strings.TrimSpace(face.ID) == "" {
		face.ID = role
	}
	return face
}

func restoreFaceID(face *parsedFace, input FontData) {
	face.data.ID = input.ID
	face.info.ID = input.ID
}

// Key returns a stable 64-bit cache key. Use KeySHA256 when collision-resistant
// identity is required.
func (f *FontSet) Key() uint64 {
	if f == nil {
		return 0
	}
	return binary.BigEndian.Uint64(f.key[:8])
}

// KeySHA256 returns the canonical SHA-256 identity of the font set.
func (f *FontSet) KeySHA256() [32]byte {
	if f == nil {
		return [32]byte{}
	}
	return f.key
}

// Fonts returns an independent copy of the input font data.
func (f *FontSet) Fonts() Fonts {
	if f == nil {
		return Fonts{}
	}
	out := f.fonts
	out.Math = copyFontData(f.fonts.Math)
	if f.fonts.Text != nil {
		text := copyFontData(*f.fonts.Text)
		out.Text = &text
	}
	out.Fallbacks = make([]FontData, len(f.fonts.Fallbacks))
	for i, face := range f.fonts.Fallbacks {
		out.Fallbacks[i] = copyFontData(face)
	}
	return out
}

// Face returns immutable metadata for ref.
func (f *FontSet) Face(ref FaceRef) (FaceInfo, bool) {
	face := f.face(ref)
	if face == nil {
		return FaceInfo{}, false
	}
	return face.info, true
}

// FaceCount returns the number of loaded faces.
func (f *FontSet) FaceCount() int {
	if f == nil {
		return 0
	}
	return len(f.faces)
}

// VerticalMetrics returns design-unit vertical metrics for ref.
func (f *FontSet) VerticalMetrics(ref FaceRef) (VerticalMetrics, bool) {
	face := f.face(ref)
	if face == nil {
		return VerticalMetrics{}, false
	}
	return face.vertical, true
}

// MapRune maps r through ref's cmap.
func (f *FontSet) MapRune(ref FaceRef, r rune) (Glyph, bool) {
	face := f.face(ref)
	if face == nil {
		return Glyph{}, false
	}
	gid, ok := face.mapRune(r)
	if !ok {
		return Glyph{}, false
	}
	return Glyph{Face: ref, ID: gid}, true
}

// MapVariation maps a base rune and Unicode variation selector.
func (f *FontSet) MapVariation(ref FaceRef, r, selector rune) (Glyph, bool) {
	face := f.face(ref)
	if face == nil {
		return Glyph{}, false
	}
	gid, ok := face.opentype.GlyphIndexVariation(r, selector)
	if !ok {
		return Glyph{}, false
	}
	return Glyph{Face: ref, ID: GlyphID(gid)}, true
}

// GlyphMetrics returns design-unit metrics for one glyph.
func (f *FontSet) GlyphMetrics(g Glyph) (GlyphMetrics, bool) {
	face := f.face(g.Face)
	if face == nil || int(g.ID) >= len(face.metrics) {
		return GlyphMetrics{}, false
	}
	return face.metrics[g.ID], true
}

// GlyphAdvance returns the design-unit advance width of one glyph.
func (f *FontSet) GlyphAdvance(g Glyph) (int32, bool) {
	m, ok := f.GlyphMetrics(g)
	return m.Advance, ok
}

// Outline returns a defensive copy of a glyph's unhinted y-down outline.
func (f *FontSet) Outline(g Glyph) (Outline, bool) {
	face := f.face(g.Face)
	if face == nil {
		return Outline{}, false
	}
	data, ok := face.outline(g.ID)
	if !ok {
		return Outline{}, false
	}
	return Outline{
		Glyph:    g,
		Bounds:   data.bounds,
		Segments: append([]Segment(nil), data.segments...),
	}, true
}

// MathConstant returns one design-unit MATH constant. Percentage constants are
// returned as literal percentages.
func (f *FontSet) MathConstant(ref FaceRef, c MathConstant) (int32, bool) {
	face := f.face(ref)
	if face == nil || !face.math.hasConstants || c >= MathConstantCount {
		return 0, false
	}
	return face.math.constants[c], true
}

// ItalicCorrection returns the MATH italic correction in design units.
func (f *FontSet) ItalicCorrection(g Glyph) (int32, bool) {
	face := f.face(g.Face)
	if face == nil || !face.math.present {
		return 0, false
	}
	v, ok := face.math.italic[g.ID]
	return v, ok
}

// TopAccentAttachment returns the MATH top-accent attachment point in design
// units from the glyph origin.
func (f *FontSet) TopAccentAttachment(g Glyph) (int32, bool) {
	face := f.face(g.Face)
	if face == nil || !face.math.present {
		return 0, false
	}
	v, ok := face.math.topAccent[g.ID]
	return v, ok
}

// IsExtendedShape reports whether a glyph is in MATH ExtendedShapeCoverage.
func (f *FontSet) IsExtendedShape(g Glyph) (bool, bool) {
	face := f.face(g.Face)
	if face == nil || !face.math.present {
		return false, false
	}
	_, ok := face.math.extendedShape[g.ID]
	return ok, true
}

// MathKernCurve returns a defensive copy of a glyph's MATH cut-in kern curve.
func (f *FontSet) MathKernCurve(g Glyph, corner MathKernCorner) (KernCurve, bool) {
	face := f.face(g.Face)
	if face == nil || !face.math.present || corner > MathKernBottomLeft {
		return KernCurve{}, false
	}
	record := face.math.kerns[g.ID]
	if record == nil || record.corners[corner] == nil {
		return KernCurve{}, false
	}
	curve := record.corners[corner]
	return KernCurve{
		Heights: append([]int32(nil), curve.Heights...),
		Kerns:   append([]int32(nil), curve.Kerns...),
	}, true
}

// MathKern evaluates a glyph's MATH cut-in kern at correctionHeight design
// units.
func (f *FontSet) MathKern(g Glyph, corner MathKernCorner, correctionHeight int32) int32 {
	curve, ok := f.MathKernCurve(g, corner)
	if !ok {
		return 0
	}
	return curve.At(correctionHeight)
}

// MathConstruction returns size variants and an optional assembly recipe.
func (f *FontSet) MathConstruction(g Glyph, axis MathAxis) (MathConstruction, bool) {
	face := f.face(g.Face)
	if face == nil || !face.math.present {
		return MathConstruction{}, false
	}
	table := face.math.vertical
	if axis == HorizontalAxis {
		table = face.math.horizontal
	} else if axis != VerticalAxis {
		return MathConstruction{}, false
	}
	construction, ok := table[g.ID]
	if !ok {
		return MathConstruction{}, false
	}
	out := MathConstruction{Variants: append([]SizeVariant(nil), construction.variants...)}
	if construction.assembly != nil {
		assembly := *construction.assembly
		assembly.Parts = append([]AssemblyPart(nil), assembly.Parts...)
		out.Assembly = &assembly
	}
	return out, true
}

// Features returns the supported math GSUB feature tags in lexical order.
func (f *FontSet) Features(ref FaceRef) ([]string, bool) {
	face := f.face(ref)
	if face == nil {
		return nil, false
	}
	return face.gsub.featureTags(), true
}

// HasFeature reports whether ref has a supported ssty or dtls rule.
func (f *FontSet) HasFeature(ref FaceRef, tag string) bool {
	face := f.face(ref)
	if face == nil {
		return false
	}
	_, ok := face.gsub.features[tag]
	return ok
}

// Shape applies one-to-one math GSUB features in request order and returns a
// new glyph slice. Tags must be exact, case-sensitive four-byte OpenType tags.
// Disabled (zero-valued) and unsupported features are ignored.
func (f *FontSet) Shape(ref FaceRef, glyphs []GlyphID, features ...Feature) ([]GlyphID, bool) {
	face := f.face(ref)
	if face == nil {
		return nil, false
	}
	return face.gsub.shape(glyphs, features), true
}

// ShapeMath applies the standard math substitutions in Typst's canonical
// order: optional dtls first, then the style-appropriate ssty alternate.
func (f *FontSet) ShapeMath(ref FaceRef, glyphs []GlyphID, style MathScriptStyle, flatten bool) ([]GlyphID, bool) {
	face := f.face(ref)
	if face == nil {
		return nil, false
	}
	features := make([]Feature, 0, 2)
	if flatten {
		features = append(features, Feature{Tag: "dtls", Value: 1})
	}
	switch style {
	case ScriptVariant:
		features = append(features, Feature{Tag: "ssty", Value: 1})
	case ScriptScriptVariant:
		features = append(features, Feature{Tag: "ssty", Value: 2})
	case NoScriptVariant:
	default:
		return nil, false
	}
	return face.gsub.shape(glyphs, features), true
}

func (f *FontSet) face(ref FaceRef) *parsedFace {
	if f == nil || len(f.faces) == 0 {
		return nil
	}
	switch ref {
	case MathFace:
		return f.faces[0]
	case TextFace:
		if !f.hasText {
			return nil
		}
		return f.faces[1]
	}
	if ref < fallbackFaceBase {
		return nil
	}
	index := int(ref-fallbackFaceBase) + 1
	if f.hasText {
		index++
	}
	if index < 0 || index >= len(f.faces) {
		return nil
	}
	return f.faces[index]
}

func canonicalFontKey(fonts Fonts) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte("untie-font-set\x00v1\x00"))
	writeKeyFace(h, 1, fonts.Math)
	writeKeyFace(h, boolByte(fonts.Text != nil), valueOrZero(fonts.Text))
	writeUint32(h, uint32(len(fonts.Fallbacks)))
	for _, face := range fonts.Fallbacks {
		writeKeyFace(h, 2, face)
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writeKeyFace(h hashWriter, role byte, face FontData) {
	_, _ = h.Write([]byte{role})
	writeUint32(h, uint32(face.Index))
	writeUint32(h, uint32(len(face.ID)))
	_, _ = h.Write([]byte(face.ID))
	writeUint32(h, uint32(len(face.Bytes)))
	_, _ = h.Write(face.Bytes)
}

func writeUint32(h hashWriter, v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	_, _ = h.Write(b[:])
}

func copyFontData(in FontData) FontData {
	out := in
	out.Bytes = append([]byte(nil), in.Bytes...)
	return out
}

func valueOrZero(v *FontData) FontData {
	if v == nil {
		return FontData{}
	}
	return *v
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

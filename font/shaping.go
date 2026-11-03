package font

import (
	"context"
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	tsfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/segmenter"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

var (
	// ErrInvalidText reports a malformed text-shaping request.
	ErrInvalidText = errors.New("font: invalid text shaping request")
	// ErrTextLimit reports that shaped output exceeded the caller's budget.
	ErrTextLimit = errors.New("font: text glyph limit exceeded")
)

// TextShapeRequest describes one bidirectional, fallback-aware text run.
//
// Faces are tried in order. Fallback is selected per extended grapheme cluster,
// so combining marks and emoji ZWJ sequences are never split merely to change
// fonts. MaxGlyphs bounds the total number of output glyph records; zero leaves
// the request unbounded and is intended only for trusted callers.
type TextShapeRequest struct {
	Context   context.Context
	Text      string
	Faces     []FaceRef
	Features  []Feature
	MaxGlyphs int
}

// ShapedCluster identifies the source runes represented by a shaping cluster.
// Rune and byte offsets are relative to TextShapeRequest.Text.
type ShapedCluster struct {
	RuneStart int
	RuneEnd   int
	ByteStart int
	ByteEnd   int
	Face      FaceRef
	// Missing reports that the selected face does not cover every significant
	// rune in the cluster, or that OpenType shaping produced .notdef.
	Missing bool
}

// ShapedGlyph is one positioned glyph in font design units. Cluster indexes
// ShapedText.Clusters. Invisible records carry advances for default-ignorable
// characters but have no drawable glyph.
type ShapedGlyph struct {
	Glyph     Glyph
	Advance   int32
	XOffset   int32
	YOffset   int32
	Cluster   int
	Invisible bool
}

// ShapedText is a visually ordered, one-line text run.
type ShapedText struct {
	Clusters []ShapedCluster
	Glyphs   []ShapedGlyph
}

type indexedFace struct {
	ref  FaceRef
	face *tsfont.Face
}

type textShaper struct {
	faces            []indexedFace
	paragraphSegment shaping.Segmenter
	graphemeSegment  segmenter.Segmenter
	shaper           shaping.HarfbuzzShaper
}

type singleFaceMap struct {
	face *tsfont.Face
}

func (m singleFaceMap) ResolveFace(rune) *tsfont.Face {
	return m.face
}

func newTextShaper(set *FontSet) *textShaper {
	refs := make([]FaceRef, 0, len(set.faces))
	refs = append(refs, MathFace)
	if set.hasText {
		refs = append(refs, TextFace)
	}
	for i := 0; ; i++ {
		ref, ok := FallbackFace(i)
		if !ok {
			break
		}
		if set.face(ref) == nil {
			break
		}
		refs = append(refs, ref)
	}
	faces := make([]indexedFace, 0, len(refs))
	for _, ref := range refs {
		parsed := set.face(ref)
		if parsed == nil {
			continue
		}
		faces = append(faces, indexedFace{ref: ref, face: tsfont.NewFace(parsed.glyphFont)})
	}
	return &textShaper{faces: faces}
}

func (s *textShaper) face(ref FaceRef) (*indexedFace, bool) {
	for i := range s.faces {
		if s.faces[i].ref == ref {
			return &s.faces[i], true
		}
	}
	return nil, false
}

func (s *textShaper) reset() {
	// Segmenter retains the last input until its next Split. Flush those
	// references before returning the shaper to the pool.
	s.paragraphSegment.Split(shaping.Input{}, singleFaceMap{})
	s.graphemeSegment.Init(nil)
}

// ShapeText shapes one text run using the supplied ordered face list.
func (f *FontSet) ShapeText(req TextShapeRequest) (ShapedText, error) {
	if f == nil || len(f.faces) == 0 {
		return ShapedText{}, fmt.Errorf("%w: empty font set", ErrInvalidText)
	}
	if req.Context == nil {
		return ShapedText{}, fmt.Errorf("%w: nil context", ErrInvalidText)
	}
	if err := req.Context.Err(); err != nil {
		return ShapedText{}, err
	}
	if !utf8.ValidString(req.Text) {
		return ShapedText{}, fmt.Errorf("%w: invalid UTF-8", ErrInvalidText)
	}
	if len(req.Faces) == 0 {
		return ShapedText{}, fmt.Errorf("%w: no faces", ErrInvalidText)
	}
	if req.MaxGlyphs < 0 {
		return ShapedText{}, fmt.Errorf("%w: negative glyph limit", ErrInvalidText)
	}
	features, err := openTypeFeatures(req.Features)
	if err != nil {
		return ShapedText{}, err
	}

	shaper, _ := f.shapers.Get().(*textShaper)
	if shaper == nil {
		shaper = newTextShaper(f)
	}
	defer func() {
		shaper.reset()
		f.shapers.Put(shaper)
	}()

	faces := make([]indexedFace, len(req.Faces))
	seen := make(map[FaceRef]struct{}, len(req.Faces))
	for i, ref := range req.Faces {
		if _, duplicate := seen[ref]; duplicate {
			return ShapedText{}, fmt.Errorf("%w: duplicate face %d", ErrInvalidText, ref)
		}
		seen[ref] = struct{}{}
		face, ok := shaper.face(ref)
		if !ok {
			return ShapedText{}, fmt.Errorf("%w: unknown face %d", ErrInvalidText, ref)
		}
		faces[i] = *face
	}

	text := []rune(req.Text)
	if len(text) == 0 {
		return ShapedText{}, nil
	}
	byteOffsets := runeByteOffsets(text)
	input := shaping.Input{
		Text:      text,
		RunStart:  0,
		RunEnd:    len(text),
		Direction: di.DirectionLTR,
		Face:      faces[0].face,
	}
	paragraphs := shaper.paragraphSegment.Split(input, singleFaceMap{face: faces[0].face})
	faceIndex, missingRune, err := segmentClusterFaces(
		req.Context,
		text,
		faces,
		paragraphs,
		&shaper.graphemeSegment,
	)
	if err != nil {
		return ShapedText{}, err
	}
	result := ShapedText{}
	clusterIndexes := make(map[[2]int]int)
	for _, part := range paragraphs {
		if err := req.Context.Err(); err != nil {
			return ShapedText{}, err
		}
		start := part.RunStart
		for start < part.RunEnd {
			if err := req.Context.Err(); err != nil {
				return ShapedText{}, err
			}
			selected := faceIndex[start]
			end := start + 1
			for end < part.RunEnd && faceIndex[end] == selected {
				end++
			}
			run := part
			run.RunStart = start
			run.RunEnd = end
			run.Face = faces[selected].face
			run.Size = fixed.I(int(faces[selected].face.Upem()))
			run.FontFeatures = features
			output := shaper.shaper.Shape(run)
			if err := appendShapedOutput(
				req.Context,
				&result,
				output,
				faces[selected].ref,
				text,
				byteOffsets,
				missingRune,
				clusterIndexes,
				req.MaxGlyphs,
			); err != nil {
				return ShapedText{}, err
			}
			start = end
		}
	}
	return result, nil
}

func appendShapedOutput(
	ctx context.Context,
	result *ShapedText,
	output shaping.Output,
	face FaceRef,
	text []rune,
	byteOffsets []int,
	missingRune []bool,
	clusterIndexes map[[2]int]int,
	maxGlyphs int,
) error {
	for i, glyph := range output.Glyphs {
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		start := glyph.TextIndex()
		end := start + glyph.RunesCount()
		if start < 0 || end <= start || end > len(text) {
			return fmt.Errorf("%w: invalid cluster [%d,%d)", ErrInvalidText, start, end)
		}
		key := [2]int{start, end}
		clusterIndex, ok := clusterIndexes[key]
		if !ok {
			clusterIndex = len(result.Clusters)
			clusterIndexes[key] = clusterIndex
			missing := glyph.GlyphID == 0
			for i := start; i < end; i++ {
				missing = missing || missingRune[i]
			}
			result.Clusters = append(result.Clusters, ShapedCluster{
				RuneStart: start,
				RuneEnd:   end,
				ByteStart: byteOffsets[start],
				ByteEnd:   byteOffsets[end],
				Face:      face,
				Missing:   missing,
			})
		}
		if glyph.GlyphID == 0 {
			result.Clusters[clusterIndex].Missing = true
		}
		if maxGlyphs > 0 && len(result.Glyphs) >= maxGlyphs {
			return fmt.Errorf("%w: limit %d", ErrTextLimit, maxGlyphs)
		}
		advance, err := roundFontUnits(glyph.Advance)
		if err != nil {
			return err
		}
		xOffset, err := roundFontUnits(glyph.XOffset)
		if err != nil {
			return err
		}
		yOffset, err := roundFontUnits(glyph.YOffset)
		if err != nil {
			return err
		}
		shaped := ShapedGlyph{
			Glyph:   Glyph{Face: face},
			Advance: advance,
			XOffset: xOffset,
			YOffset: yOffset,
			Cluster: clusterIndex,
		}
		switch {
		case glyph.GlyphID == tsfont.EmptyGlyph:
			shaped.Invisible = true
		case uint32(glyph.GlyphID) > uint32(^uint16(0)):
			return fmt.Errorf("%w: glyph id %d", ErrInvalidText, glyph.GlyphID)
		default:
			shaped.Glyph.ID = GlyphID(glyph.GlyphID)
		}
		result.Glyphs = append(result.Glyphs, shaped)
	}
	return nil
}

func segmentClusterFaces(
	ctx context.Context,
	text []rune,
	faces []indexedFace,
	paragraphs []shaping.Input,
	seg *segmenter.Segmenter,
) ([]int, []bool, error) {
	facesByRune := make([]int, len(text))
	for i := range facesByRune {
		facesByRune[i] = -1
	}
	missing := make([]bool, len(text))
	seg.Init(text)
	iter := seg.GraphemeIterator()
	seen := 0
	for iter.Next() {
		if seen%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
		}
		grapheme := iter.Grapheme()
		selected, supported := selectClusterFace(grapheme.Text, faces)
		for i := grapheme.Offset; i < grapheme.Offset+len(grapheme.Text); i++ {
			facesByRune[i] = selected
			missing[i] = !supported
		}
		seen++
	}
	if seen == 0 && len(text) != 0 {
		return nil, nil, fmt.Errorf("%w: grapheme segmentation produced no clusters", ErrInvalidText)
	}
	for i, selected := range facesByRune {
		if selected < 0 {
			return nil, nil, fmt.Errorf("%w: grapheme coverage is missing rune %d", ErrInvalidText, i)
		}
	}
	// A single face covering a complete bidi/script run preserves joining,
	// reordering, and ligatures inside that run. Per-grapheme fallback is only
	// used when no supplied face covers the run as a whole.
	for _, paragraph := range paragraphs {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if paragraph.RunStart < 0 || paragraph.RunEnd <= paragraph.RunStart || paragraph.RunEnd > len(text) {
			return nil, nil, fmt.Errorf("%w: invalid paragraph range", ErrInvalidText)
		}
		selected, supported := selectRunFace(text[paragraph.RunStart:paragraph.RunEnd], paragraph.Script, faces)
		if !supported {
			continue
		}
		for i := paragraph.RunStart; i < paragraph.RunEnd; i++ {
			facesByRune[i] = selected
			missing[i] = false
		}
	}
	return facesByRune, missing, nil
}

func selectRunFace(run []rune, script language.Script, faces []indexedFace) (int, bool) {
	firstFull := -1
	preferTextFace := script.Strong() &&
		script != language.Latin &&
		script != language.Greek &&
		script != language.Cyrillic
	for i, candidate := range faces {
		coverage := 0
		required := 0
		for _, r := range run {
			if !significantTextRune(r) {
				continue
			}
			required++
			if _, ok := candidate.face.NominalGlyph(r); ok {
				coverage++
			}
		}
		if required == 0 {
			return 0, true
		}
		if coverage == required {
			if firstFull < 0 {
				firstFull = i
			}
			if !preferTextFace || candidate.ref != MathFace {
				return i, true
			}
		}
	}
	if firstFull >= 0 {
		return firstFull, true
	}
	return 0, false
}

func selectClusterFace(cluster []rune, faces []indexedFace) (int, bool) {
	best := 0
	bestCoverage := -1
	for i, candidate := range faces {
		coverage := 0
		required := 0
		for _, r := range cluster {
			if !significantTextRune(r) {
				continue
			}
			required++
			if _, ok := candidate.face.NominalGlyph(r); ok {
				coverage++
			}
		}
		if required == 0 {
			return 0, true
		}
		if coverage == required {
			return i, true
		}
		if coverage > bestCoverage {
			best = i
			bestCoverage = coverage
		}
	}
	return best, false
}

func significantTextRune(r rune) bool {
	switch {
	case r >= 0xFE00 && r <= 0xFE0F:
		return false
	case r >= 0xE0100 && r <= 0xE01EF:
		return false
	case r == '\u200C' || r == '\u200D' || r == '\uFEFF':
		return false
	case unicode.Is(unicode.Cc, r), unicode.Is(unicode.Cf, r):
		return false
	case unicode.Is(unicode.Zl, r), unicode.Is(unicode.Zp, r):
		return false
	default:
		return true
	}
}

func openTypeFeatures(features []Feature) ([]shaping.FontFeature, error) {
	out := make([]shaping.FontFeature, len(features))
	for i, feature := range features {
		if len(feature.Tag) != 4 {
			return nil, fmt.Errorf("%w: feature tag %q", ErrInvalidText, feature.Tag)
		}
		for _, r := range feature.Tag {
			if r < 0x20 || r > 0x7e {
				return nil, fmt.Errorf("%w: feature tag %q", ErrInvalidText, feature.Tag)
			}
		}
		out[i] = shaping.FontFeature{
			Tag:   ot.NewTag(feature.Tag[0], feature.Tag[1], feature.Tag[2], feature.Tag[3]),
			Value: uint32(feature.Value),
		}
	}
	return out, nil
}

func runeByteOffsets(text []rune) []int {
	offsets := make([]int, len(text)+1)
	offset := 0
	for i, r := range text {
		offsets[i] = offset
		offset += utf8.RuneLen(r)
	}
	offsets[len(text)] = offset
	return offsets
}

func roundFontUnits(value fixed.Int26_6) (int32, error) {
	units := int64(value)
	if units >= 0 {
		units = (units + 32) / 64
	} else {
		units = (units - 32) / 64
	}
	const (
		minInt32 = -1 << 31
		maxInt32 = 1<<31 - 1
	)
	if units < minInt32 || units > maxInt32 {
		return 0, fmt.Errorf("%w: shaped metric is out of range", ErrInvalidText)
	}
	return int32(units), nil
}

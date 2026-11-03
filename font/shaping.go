package font

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/bidi"
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
	Direction TextDirection
	Language  string
	MaxGlyphs int
}

// TextDirection selects the paragraph direction used by bidi reordering.
type TextDirection uint8

const (
	TextDirectionLTR TextDirection = iota
	TextDirectionRTL
)

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
	set              *FontSet
	faces            []indexedFace
	paragraphSegment shaping.Segmenter
	graphemeSegment  segmenter.Segmenter
	shaper           shaping.HarfbuzzShaper
	bidiParagraph    bidi.Paragraph
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
	return &textShaper{set: set, faces: faces}
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
	s.bidiParagraph.Segment(nil, bidi.Neutral)
}

func visualBidiOrder(
	parts []shaping.Input,
	text []rune,
	defaultDirection bidi.Direction,
	paragraph *bidi.Paragraph,
) []shaping.Input {
	runs := paragraph.Segment(text, defaultDirection)
	if runs.NumRuns() == 0 {
		return parts
	}
	levels := make([]bidi.Level, len(text))
	maxLevel := bidi.Level(0)
	for i := 0; i < runs.NumRuns(); i++ {
		run := runs.Run(i)
		if run.Start < 0 || run.End <= run.Start || run.End > len(levels) {
			return parts
		}
		if run.Level > maxLevel {
			maxLevel = run.Level
		}
		for j := run.Start; j < run.End; j++ {
			levels[j] = run.Level
		}
	}
	if maxLevel == 0 {
		return parts
	}
	visual := make([]int, len(text))
	for i := range visual {
		visual[i] = i
	}
	for level := maxLevel; level > 0; level-- {
		for start := 0; start < len(visual); {
			if levels[visual[start]] < level {
				start++
				continue
			}
			end := start + 1
			for end < len(visual) && levels[visual[end]] >= level {
				end++
			}
			for left, right := start, end-1; left < right; left, right = left+1, right-1 {
				visual[left], visual[right] = visual[right], visual[left]
			}
			start = end
		}
	}
	position := make([]int, len(text))
	for i, runeIndex := range visual {
		position[runeIndex] = i
	}
	ordered := append([]shaping.Input(nil), parts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return visualStart(ordered[i], position) < visualStart(ordered[j], position)
	})
	return ordered
}

func visualStart(part shaping.Input, position []int) int {
	start := int(^uint(0) >> 1)
	if part.RunStart < 0 || part.RunStart >= len(position) {
		return start
	}
	end := min(part.RunEnd, len(position))
	for i := part.RunStart; i < end; i++ {
		start = min(start, position[i])
	}
	return start
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
	if req.Direction > TextDirectionRTL {
		return ShapedText{}, fmt.Errorf("%w: unknown direction %d", ErrInvalidText, req.Direction)
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
	direction := di.DirectionLTR
	baseDirection := bidi.LeftToRight
	if req.Direction == TextDirectionRTL {
		direction = di.DirectionRTL
		baseDirection = bidi.RightToLeft
	}
	input := shaping.Input{
		Text:      text,
		RunStart:  0,
		RunEnd:    len(text),
		Direction: direction,
		Language:  language.NewLanguage(req.Language),
		Face:      faces[0].face,
	}
	paragraphs := shaper.paragraphSegment.Split(input, singleFaceMap{face: faces[0].face})
	paragraphs = visualBidiOrder(paragraphs, text, baseDirection, &shaper.bidiParagraph)
	faceIndex, missingRune, err := segmentClusterFaces(
		req.Context,
		text,
		faces,
		paragraphs,
		&shaper.graphemeSegment,
		shaper.set,
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
		var ranges []textShapeRange
		for start < part.RunEnd {
			if err := req.Context.Err(); err != nil {
				return ShapedText{}, err
			}
			selected := faceIndex[start]
			end := start + 1
			for end < part.RunEnd && faceIndex[end] == selected {
				end++
			}
			ranges = append(ranges, textShapeRange{start: start, end: end, face: selected})
			start = end
		}
		if part.Direction.Progression() == di.TowardTopLeft {
			for left, right := 0, len(ranges)-1; left < right; left, right = left+1, right-1 {
				ranges[left], ranges[right] = ranges[right], ranges[left]
			}
		}
		for _, current := range ranges {
			start, end, selected := current.start, current.end, current.face
			run := part
			run.RunStart = start
			run.RunEnd = end
			run.Face = faces[selected].face
			run.Size = fixed.I(int(faces[selected].face.Upem()))
			run.FontFeatures = features
			output := shaper.shaper.Shape(run)
			bestFace := selected
			bestOutput := output
			if runNeedsShapingProbe(text[start:end], part.Script) ||
				missingGlyphCount(bestOutput.Glyphs) > 0 {
				bestMissing := missingGlyphCount(bestOutput.Glyphs)
				bestGlyphs := len(bestOutput.Glyphs)
				for candidateIndex, candidate := range faces {
					if candidateIndex == selected {
						continue
					}
					candidateRun := run
					candidateRun.Face = candidate.face
					candidateRun.Size = fixed.I(int(candidate.face.Upem()))
					candidateOutput := shaper.shaper.Shape(candidateRun)
					candidateMissing := missingGlyphCount(candidateOutput.Glyphs)
					candidateGlyphs := len(candidateOutput.Glyphs)
					if candidateMissing < bestMissing ||
						candidateMissing == bestMissing && candidateGlyphs < bestGlyphs {
						bestFace = candidateIndex
						bestOutput = candidateOutput
						bestMissing = candidateMissing
						bestGlyphs = candidateGlyphs
					}
				}
				output = bestOutput
				selected = bestFace
				for i := start; i < end; i++ {
					faceIndex[i] = selected
				}
				required, covered := faceCoverage(f, faces[selected], text[start:end])
				for i := start; i < end; i++ {
					missingRune[i] = required != covered
				}
			}
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
		}
	}
	return result, nil
}

type textShapeRange struct {
	start int
	end   int
	face  int
}

func missingGlyphCount(glyphs []shaping.Glyph) int {
	count := 0
	for _, glyph := range glyphs {
		if glyph.GlyphID == 0 {
			count++
		}
	}
	return count
}

func runNeedsShapingProbe(run []rune, script language.Script) bool {
	if script.Strong() &&
		script != language.Latin &&
		script != language.Greek &&
		script != language.Cyrillic {
		return true
	}
	for _, r := range run {
		if isVariationSelectorRune(r) || r == '\u200D' || r >= 0x1F000 {
			return true
		}
	}
	return false
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
	set *FontSet,
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
		selected, supported := selectClusterFace(set, grapheme.Text, faces)
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
		selected, supported := selectRunFace(
			set,
			text[paragraph.RunStart:paragraph.RunEnd],
			paragraph.Script,
			faces,
		)
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

func selectRunFace(set *FontSet, run []rune, script language.Script, faces []indexedFace) (int, bool) {
	firstFull := -1
	preferTextFace := script.Strong() &&
		script != language.Latin &&
		script != language.Greek &&
		script != language.Cyrillic
	for i, candidate := range faces {
		required, coverage := faceCoverage(set, candidate, run)
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

func selectClusterFace(set *FontSet, cluster []rune, faces []indexedFace) (int, bool) {
	best := 0
	bestCoverage := -1
	for i, candidate := range faces {
		required, coverage := faceCoverage(set, candidate, cluster)
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

func faceCoverage(set *FontSet, candidate indexedFace, run []rune) (required, covered int) {
	for i := 0; i < len(run); i++ {
		r := run[i]
		if isVariationSelectorRune(r) {
			continue
		}
		if !significantTextRune(r) {
			continue
		}
		required++
		if i+1 < len(run) && isVariationSelectorRune(run[i+1]) {
			if _, ok := set.MapVariation(candidate.ref, r, run[i+1]); ok {
				covered++
			}
			i++
			continue
		}
		if _, ok := candidate.face.NominalGlyph(r); ok {
			covered++
		}
	}
	return required, covered
}

func isVariationSelectorRune(r rune) bool {
	return r >= 0xFE00 && r <= 0xFE0F || r >= 0xE0100 && r <= 0xE01EF
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
		tag := feature.Tag
		if len(tag) == 0 || len(tag) > 4 {
			return nil, fmt.Errorf("%w: feature tag %q", ErrInvalidText, feature.Tag)
		}
		for len(tag) < 4 {
			tag += " "
		}
		for _, r := range tag {
			if r < 0x20 || r > 0x7e {
				return nil, fmt.Errorf("%w: feature tag %q", ErrInvalidText, feature.Tag)
			}
		}
		out[i] = shaping.FontFeature{
			Tag:   ot.NewTag(tag[0], tag[1], tag[2], tag[3]),
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

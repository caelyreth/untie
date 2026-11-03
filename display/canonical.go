package display

import (
	"encoding/json"
	"fmt"

	"github.com/caelyreth/untie/geometry"
)

// MarshalCanonical returns deterministic JSON for alternative-text tests.
// It is not a stable cross-language protocol.
func (a Alt) MarshalCanonical() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(canonicalAlt(a))
}

// MarshalCanonical returns deterministic JSON for golden tests and diagnostics.
// It is not a stable cross-language protocol.
func (b *Box) MarshalCanonical() ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("display: nil box")
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(canonicalBoxValue(b, true))
}

type canonicalBoxSnapshot struct {
	Width      float32           `json:"width"`
	Ascent     float32           `json:"ascent"`
	Descent    float32           `json:"descent"`
	Italic     float32           `json:"italic"`
	AxisHeight float32           `json:"axisHeight"`
	Overflow   float32           `json:"overflow"`
	Items      []canonicalItem   `json:"items"`
	Paths      []canonicalPath   `json:"paths,omitempty"`
	Hits       []canonicalHit    `json:"hits,omitempty"`
	Breaks     []canonicalBreak  `json:"breaks,omitempty"`
	Alt        *canonicalAlt     `json:"alt,omitempty"`
	Tag        *canonicalTagBox  `json:"tag,omitempty"`
	Tags       []canonicalTagBox `json:"tags,omitempty"`
	Lines      []canonicalLine   `json:"lines,omitempty"`
}

type canonicalLine struct {
	Baseline float32          `json:"baseline"`
	OffsetX  float32          `json:"offsetX"`
	Width    float32          `json:"width"`
	Ascent   float32          `json:"ascent"`
	Descent  float32          `json:"descent"`
	Overflow float32          `json:"overflow"`
	Items    canonicalRange   `json:"items"`
	Hits     canonicalRange   `json:"hits"`
	Tag      *canonicalTagBox `json:"tag,omitempty"`
}

type canonicalTagBox struct {
	Source  [2]int         `json:"source"`
	Manual  bool           `json:"manual"`
	Starred bool           `json:"starred"`
	Line    int            `json:"line"`
	X       float32        `json:"x"`
	Y       float32        `json:"y"`
	Width   float32        `json:"width"`
	Ascent  float32        `json:"ascent"`
	Descent float32        `json:"descent"`
	Italic  float32        `json:"italic"`
	Items   canonicalRange `json:"items"`
	Hits    canonicalRange `json:"hits"`
}

type canonicalRange struct {
	Start int `json:"start"`
	Count int `json:"count"`
}

func canonicalBoxValue(b *Box, includeLines bool) canonicalBoxSnapshot {
	items := make([]canonicalItem, len(b.Items))
	for i, item := range b.Items {
		items[i] = canonicalItem{
			Kind:      itemKindName(item.Kind),
			Paint:     canonicalPaint(item.Paint),
			Bound:     []float32{item.Bound.X, item.Bound.Y, item.Bound.Width, item.Bound.Height},
			PathIndex: canonicalPathIndex(item),
			Rect:      []float32{item.Rect.X, item.Rect.Y, item.Rect.Width, item.Rect.Height},
			FontID:    item.FontID,
			Glyph:     item.Glyph,
			At:        []float32{item.At.X, item.At.Y},
			Em:        item.Em,
		}
	}
	var alt *canonicalAlt
	if b.Alt != (Alt{}) {
		snapshot := canonicalAlt(b.Alt)
		alt = &snapshot
	}
	out := canonicalBoxSnapshot{
		Width:      b.Width,
		Ascent:     b.Ascent,
		Descent:    b.Descent,
		Italic:     b.Italic,
		AxisHeight: b.AxisHeight,
		Overflow:   b.Overflow,
		Items:      items,
		Paths:      canonicalPaths(b.Paths),
		Hits:       canonicalHits(b.Hits),
		Breaks:     canonicalBreaks(b.Breaks),
		Alt:        alt,
	}
	if b.Tag != nil {
		tag := canonicalTag(b.Tag)
		out.Tag = &tag
	}
	if len(b.Tags) > 0 {
		out.Tags = make([]canonicalTagBox, len(b.Tags))
		for i := range b.Tags {
			out.Tags[i] = canonicalTag(&b.Tags[i])
		}
	}
	if includeLines && len(b.Lines) > 0 {
		out.Lines = make([]canonicalLine, len(b.Lines))
		for i, line := range b.Lines {
			out.Lines[i] = canonicalLine{
				Baseline: line.Baseline,
				OffsetX:  line.OffsetX,
				Width:    line.Width,
				Ascent:   line.Ascent,
				Descent:  line.Descent,
				Overflow: line.Overflow,
				Items:    canonicalItemRange(line.Items),
				Hits:     canonicalHitRange(line.Hits),
			}
			if line.Tag != nil {
				tag := canonicalTag(line.Tag)
				out.Lines[i].Tag = &tag
			}
		}
	}
	return out
}

func canonicalTag(tag *TagBox) canonicalTagBox {
	return canonicalTagBox{
		Source:  [2]int{tag.Source.Start, tag.Source.End},
		Manual:  tag.Manual,
		Starred: tag.Starred,
		Line:    tag.Line,
		X:       tag.X,
		Y:       tag.Y,
		Width:   tag.Width,
		Ascent:  tag.Ascent,
		Descent: tag.Descent,
		Italic:  tag.Italic,
		Items:   canonicalItemRange(tag.Items),
		Hits:    canonicalHitRange(tag.Hits),
	}
}

func canonicalItemRange(itemRange ItemRange) canonicalRange {
	return canonicalRange(itemRange)
}

func canonicalHitRange(hitRange HitRange) canonicalRange {
	return canonicalRange(hitRange)
}

type canonicalAlt struct {
	Speech string `json:"speech"`
	Text   string `json:"text"`
}

type canonicalHit struct {
	Source [2]int    `json:"source"`
	Area   []float32 `json:"area"`
	Depth  int       `json:"depth"`
	Kind   string    `json:"kind"`
}

type canonicalBreak struct {
	X    float32 `json:"x"`
	Kind string  `json:"kind"`
}

func canonicalHits(hits []Hit) []canonicalHit {
	if len(hits) == 0 {
		return nil
	}
	out := make([]canonicalHit, len(hits))
	for i, hit := range hits {
		out[i] = canonicalHit{
			Source: [2]int{hit.Source.Start, hit.Source.End},
			Area:   rectValues(hit.Area),
			Depth:  hit.Depth,
			Kind:   hitKindName(hit.Kind),
		}
	}
	return out
}

func canonicalBreaks(breaks []Break) []canonicalBreak {
	if len(breaks) == 0 {
		return nil
	}
	out := make([]canonicalBreak, len(breaks))
	for i, breakPoint := range breaks {
		out[i] = canonicalBreak{
			X:    breakPoint.X,
			Kind: breakKindName(breakPoint.Kind),
		}
	}
	return out
}

func rectValues(rect geometry.Rect) []float32 {
	return []float32{rect.X, rect.Y, rect.Width, rect.Height}
}

type canonicalItem struct {
	Kind      string    `json:"kind"`
	Paint     []any     `json:"paint"`
	Bound     []float32 `json:"bound"`
	PathIndex *int      `json:"pathIndex,omitempty"`
	Rect      []float32 `json:"rect,omitempty"`
	FontID    string    `json:"fontID,omitempty"`
	Glyph     uint16    `json:"glyph,omitempty"`
	At        []float32 `json:"at,omitempty"`
	Em        float32   `json:"em,omitempty"`
}

type canonicalPath struct {
	Bound    []float32 `json:"bound"`
	Segments [][]any   `json:"segments"`
}

func canonicalPathIndex(item Item) *int {
	if item.Kind != ItemGlyphPath {
		return nil
	}
	index := item.PathIndex
	return &index
}

func canonicalPaths(paths []PathTemplate) []canonicalPath {
	if len(paths) == 0 {
		return nil
	}
	out := make([]canonicalPath, len(paths))
	for i, path := range paths {
		out[i] = canonicalPath{
			Bound:    rectValues(path.Bound),
			Segments: canonicalSegments(path.Segments),
		}
	}
	return out
}

func canonicalSegments(path []geometry.Segment) [][]any {
	out := make([][]any, len(path))
	for i, seg := range path {
		row := make([]any, 0, 7)
		row = append(row, seg.Op)
		for _, p := range seg.Points {
			row = append(row, p.X, p.Y)
		}
		out[i] = row
	}
	return out
}

func canonicalPaint(p Paint) []any {
	if p.HasExplicit {
		return []any{"rgba", p.Explicit.R, p.Explicit.G, p.Explicit.B, p.Explicit.A}
	}
	return []any{"role", p.Role}
}

func itemKindName(kind ItemKind) string {
	switch kind {
	case ItemGlyphPath:
		return "glyph-path"
	case ItemRule:
		return "rule"
	case ItemGlyphRef:
		return "glyph-ref"
	default:
		return "unknown"
	}
}

func hitKindName(kind HitKind) string {
	switch kind {
	case HitAtom:
		return "atom"
	case HitGroup:
		return "group"
	case HitScript:
		return "script"
	case HitFractionNumerator:
		return "fraction-numerator"
	case HitFractionDenominator:
		return "fraction-denominator"
	case HitRadical:
		return "radical"
	case HitFence:
		return "fence"
	case HitTableCell:
		return "table-cell"
	case HitRow:
		return "row"
	case HitOther:
		return "other"
	default:
		return "unknown"
	}
}

func breakKindName(kind BreakKind) string {
	switch kind {
	case BreakRelation:
		return "relation"
	case BreakBinary:
		return "binary"
	case BreakExplicit:
		return "explicit"
	default:
		return "unknown"
	}
}

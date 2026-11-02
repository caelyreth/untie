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
	Width      float32          `json:"width"`
	Ascent     float32          `json:"ascent"`
	Descent    float32          `json:"descent"`
	Italic     float32          `json:"italic"`
	AxisHeight float32          `json:"axisHeight"`
	Overflow   float32          `json:"overflow"`
	Items      []canonicalItem  `json:"items"`
	Hits       []canonicalHit   `json:"hits,omitempty"`
	Breaks     []canonicalBreak `json:"breaks,omitempty"`
	Alt        *canonicalAlt    `json:"alt,omitempty"`
	Lines      []canonicalLine  `json:"lines,omitempty"`
}

type canonicalLine struct {
	Baseline float32              `json:"baseline"`
	Box      canonicalBoxSnapshot `json:"box"`
}

func canonicalBoxValue(b *Box, includeLines bool) canonicalBoxSnapshot {
	items := make([]canonicalItem, len(b.Items))
	for i, item := range b.Items {
		items[i] = canonicalItem{
			Kind:   itemKindName(item.Kind),
			Paint:  canonicalPaint(item.Paint),
			Bound:  []float32{item.Bound.X, item.Bound.Y, item.Bound.Width, item.Bound.Height},
			Path:   canonicalPath(item.Path),
			Rect:   []float32{item.Rect.X, item.Rect.Y, item.Rect.Width, item.Rect.Height},
			FontID: item.FontID,
			Glyph:  item.Glyph,
			At:     []float32{item.At.X, item.At.Y},
			Em:     item.Em,
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
		Hits:       canonicalHits(b.Hits),
		Breaks:     canonicalBreaks(b.Breaks),
		Alt:        alt,
	}
	if includeLines && len(b.Lines) > 0 {
		out.Lines = make([]canonicalLine, len(b.Lines))
		for i, line := range b.Lines {
			out.Lines[i] = canonicalLine{
				Baseline: line.Baseline,
				Box:      canonicalBoxValue(line.Box, false),
			}
		}
	}
	return out
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
	X       float32 `json:"x"`
	Penalty int     `json:"penalty"`
	Kind    string  `json:"kind"`
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
			X:       breakPoint.X,
			Penalty: breakPoint.Penalty,
			Kind:    breakKindName(breakPoint.Kind),
		}
	}
	return out
}

func rectValues(rect geometry.Rect) []float32 {
	return []float32{rect.X, rect.Y, rect.Width, rect.Height}
}

type canonicalItem struct {
	Kind   string    `json:"kind"`
	Paint  []any     `json:"paint"`
	Bound  []float32 `json:"bound"`
	Path   [][]any   `json:"path,omitempty"`
	Rect   []float32 `json:"rect,omitempty"`
	FontID string    `json:"fontID,omitempty"`
	Glyph  uint16    `json:"glyph,omitempty"`
	At     []float32 `json:"at,omitempty"`
	Em     float32   `json:"em,omitempty"`
}

func canonicalPath(path []geometry.Segment) [][]any {
	if len(path) == 0 {
		return nil
	}
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

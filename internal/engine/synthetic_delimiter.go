package engine

import (
	"context"
	"fmt"

	"github.com/caelyreth/untie/font"
	"github.com/caelyreth/untie/internal/layout"
)

type syntheticDelimiterPart struct {
	text     rune
	extender bool
}

// KaTeX makeStackedDelim uses these Unicode pieces when a font has no native
// MATH construction for moustache delimiters.
func moustachePieces(text string) ([]syntheticDelimiterPart, bool) {
	switch text {
	case "⎰":
		return []syntheticDelimiterPart{
			{text: '\u23a7'},
			{text: '\u23aa', extender: true},
			{text: '\u23ad'},
		}, true
	case "⎱":
		return []syntheticDelimiterPart{
			{text: '\u23ab'},
			{text: '\u23aa', extender: true},
			{text: '\u23a9'},
		}, true
	default:
		return nil, false
	}
}

func (a *adapter) syntheticDelimiterAssembly(
	ctx context.Context,
	mapped font.Glyph,
	text string,
) (font.MathConstruction, bool, error) {
	pieces, ok := moustachePieces(text)
	if !ok {
		return font.MathConstruction{}, false, nil
	}
	parts := make([]font.AssemblyPart, len(pieces))
	for i, piece := range pieces {
		if err := checkEvery(ctx, i, 16); err != nil {
			return font.MathConstruction{}, false, err
		}
		mappedPart, ok := a.fonts.MapRune(mapped.Face, piece.text)
		if !ok {
			return font.MathConstruction{}, false, fmt.Errorf(
				"%w: synthetic delimiter piece U+%04X",
				layout.ErrMissingGlyph,
				piece.text,
			)
		}
		outline, ok := a.fonts.Outline(mappedPart)
		if !ok || outline.Bounds.Empty() {
			return font.MathConstruction{}, false, fmt.Errorf(
				"%w: synthetic delimiter outline U+%04X",
				layout.ErrMissingGlyph,
				piece.text,
			)
		}
		advance := int64(outline.Bounds.YMax) - int64(outline.Bounds.YMin)
		if advance <= 0 {
			return font.MathConstruction{}, false, fmt.Errorf(
				"%w: synthetic delimiter piece U+%04X has no vertical advance",
				layout.ErrInvalidInput,
				piece.text,
			)
		}
		if advance > 1<<31-1 {
			return font.MathConstruction{}, false, fmt.Errorf(
				"%w: synthetic delimiter piece U+%04X advance overflows",
				layout.ErrInvalidInput,
				piece.text,
			)
		}
		parts[i] = font.AssemblyPart{
			Glyph:       mappedPart.ID,
			FullAdvance: int32(advance),
			Extender:    piece.extender,
		}
	}
	return font.MathConstruction{Assembly: &font.Assembly{Parts: parts}}, true, nil
}

package typst

import (
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// hasCombiningMark detects identifiers that the lexer intentionally keeps
// together as one math identifier. Typst treats the marks as accents on the
// preceding base character rather than as an unknown multi-letter variable.
func hasCombiningMark(text string) bool {
	for _, r := range text {
		if unicode.IsMark(r) {
			return true
		}
	}
	return false
}

func (l *lowerer) lowerCombinedIdentifier(n *astNode) ir.Node {
	type runeSpan struct {
		r     rune
		start int
		end   int
	}
	runes := make([]runeSpan, 0, utf8.RuneCountInString(n.text))
	offset := n.span.Start
	for _, r := range n.text {
		size := utf8.RuneLen(r)
		runes = append(runes, runeSpan{r: r, start: offset, end: offset + size})
		offset += size
	}

	items := make([]ir.Node, 0, len(runes))
	for i := 0; i < len(runes); {
		base := runes[i]
		i++
		if unicode.IsMark(base.r) {
			items = append(items, l.glyph(source.NewRange(base.start, base.end), string(base.r)))
			continue
		}
		var current ir.Node = l.glyph(source.NewRange(base.start, base.end), string(base.r))
		for i < len(runes) && unicode.IsMark(runes[i].r) {
			mark := runes[i]
			markGlyph := l.glyph(source.NewRange(mark.start, mark.end), string(mark.r))
			markGlyph.Class = ir.ClassDiacritic
			position := ir.PositionAbove
			if accentIsBelow(string(mark.r)) {
				position = ir.PositionBelow
			}
			current = ir.NewAccent(source.NewRange(base.start, mark.end), current, markGlyph, position)
			i++
		}
		items = append(items, current)
	}
	if len(items) == 1 {
		return items[0]
	}
	row := ir.NewRow(n.span, items...)
	row.Class = ir.ClassAlphabetic
	return row
}

package ir

import "math"

func (s *Stats) observe(node Node) {
	base := node.NodeBase()
	if base != nil {
		s.observeLength(base.LeftSpacing)
		s.observeLength(base.RightSpacing)
	}

	switch value := node.(type) {
	case *Glyph:
		s.observeText(value.Text)
		s.GlyphCapacity++
	case *SizedDelimiter:
		s.observeText(value.Text)
		s.GlyphCapacity++
	case *Text:
		s.observeText(value.Text)
		s.GlyphCapacity = saturatedAddInt(s.GlyphCapacity, len(value.Text))
		s.observeLength(value.Tracking)
	case *Operator:
		s.observeText(value.Text)
		s.GlyphCapacity = saturatedAddInt(s.GlyphCapacity, len(value.Text))
	case *Space:
		s.observeLength(value.Amount)
	case *Fraction:
		s.observeLength(value.Padding)
	case *Line:
		s.observeLength(value.Thickness)
		s.observeLength(value.Gap)
	case *Cancel:
		s.observeLength(value.Thickness)
	case *Table:
		s.observeLength(value.RowGap)
		s.observeLength(value.ColumnGap)
		for _, line := range value.Horizontal {
			s.observeLength(line.Thickness)
		}
		for _, line := range value.Vertical {
			s.observeLength(line.Thickness)
		}
		cells := 0
		for _, row := range value.Rows {
			cells += len(row.Cells)
		}
		if cells > s.MaxTableCells {
			s.MaxTableCells = cells
		}
	case *Box:
		for _, length := range value.Padding {
			s.observeLength(length)
		}
		s.observeLength(value.BorderWidth)
		s.observeLength(value.Radius)
	case *ErrorNode:
		s.observeText(value.Text)
		s.GlyphCapacity++
	case *Placeholder:
		s.observeText(value.Reason)
	}
}

func saturatedAddInt(left, right int) int {
	if right > 0 && left > math.MaxInt-right {
		return math.MaxInt
	}
	return left + right
}

func (s *Stats) observeText(text string) {
	if len(text) > s.MaxTextBytes {
		s.MaxTextBytes = len(text)
	}
}

func (s *Stats) observeLength(length Length) {
	if length.Unit == LengthZero {
		return
	}
	value := length.Value
	if value < 0 {
		if value == math.MinInt64 {
			value = math.MaxInt64
		} else {
			value = -value
		}
	}
	if value > s.MaxDimensionValue {
		s.MaxDimensionValue = value
	}
}

package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
)

type preparedSingleLineTag struct {
	fragment *Fragment
	tag      ir.EquationTag
	width    float32
	ascent   float32
	descent  float32
	italic   float32
}

func (s *layoutState) prepareSingleLineTag(
	tags []ir.EquationTag,
	env environment,
) (*preparedSingleLineTag, error) {
	var manual *ir.EquationTag
	for i := range tags {
		tag := &tags[i]
		if tag.Suppressed {
			continue
		}
		if manual != nil {
			return nil, fmt.Errorf("%w: multiple equation tags are deferred", ErrUnsupported)
		}
		manual = tag
	}
	if manual == nil {
		return nil, nil
	}
	if s.mode != ir.Display {
		return nil, fmt.Errorf("%w: equation tags require display mode", ErrUnsupported)
	}
	if manual.Line != ir.FormulaTagLine {
		return nil, fmt.Errorf("%w: equation tag line %d is deferred", ErrUnsupported, manual.Line)
	}

	tagBody := manual.Body
	if !manual.Starred {
		open := ir.NewGlyph(manual.Span, "(")
		open.NodeBase().Class = ir.ClassOpening
		closeNode := ir.NewGlyph(manual.Span, ")")
		closeNode.NodeBase().Class = ir.ClassClosing
		fenced := ir.NewFenced(manual.Span, manual.Body)
		fenced.Open = open
		fenced.Close = closeNode
		fenced.Balanced = true
		tagBody = fenced
	}
	fragment, err := s.layoutNode(tagBody, env, 0)
	if err != nil {
		return nil, err
	}
	width, ascent, descent, italic, err := s.fragmentHostMetrics(fragment)
	if err != nil {
		return nil, err
	}
	return &preparedSingleLineTag{
		fragment: fragment,
		tag:      *manual,
		width:    width,
		ascent:   ascent,
		descent:  descent,
		italic:   italic,
	}, nil
}

func (s *layoutState) attachPreparedSingleLineTag(
	base *display.Box,
	prepared *preparedSingleLineTag,
) (*display.Box, error) {
	if base == nil || prepared == nil || prepared.fragment == nil {
		return nil, fmt.Errorf("%w: nil equation tag", ErrInvalidInput)
	}
	if len(base.Lines) > 1 {
		return nil, fmt.Errorf("%w: tagged layout produced multiple lines", ErrInvalidInput)
	}

	x := base.Width
	if len(base.Lines) == 1 {
		line := &base.Lines[0]
		formulaFrame, err := s.subHost(base.Width, prepared.width)
		if err != nil {
			return nil, err
		}
		formulaFrame = maxFloat(0, formulaFrame)
		wantOffset, err := s.displayLineOffsetWithin(line.Width, formulaFrame)
		if err != nil {
			return nil, err
		}
		if line.OffsetX != wantOffset {
			return nil, fmt.Errorf("%w: tagged line offset was not prepared for the tag frame", ErrInvalidInput)
		}
		lineRight, err := s.addHost(line.Width, line.OffsetX)
		if err != nil {
			return nil, err
		}
		x = maxFloat(formulaFrame, lineRight)
	}

	itemStart := len(base.Items)
	hitStart := len(base.Hits)
	flatten := newFlattenState(s, base, x, 0)
	base.Items = reserveDisplayItems(base.Items, prepared.fragment.flatItems, s.limits.MaxOutputItems)
	base.Hits = reserveDisplayHits(base.Hits, prepared.fragment.flatHits, s.limits.MaxNodes)
	if err := flatten.visit(prepared.fragment, 0, 0, 0); err != nil {
		return nil, err
	}
	tag := &display.TagBox{
		Source:  prepared.tag.Span,
		Manual:  prepared.tag.Manual,
		Starred: prepared.tag.Starred,
		Line:    prepared.tag.Line,
		X:       x,
		Y:       0,
		Width:   prepared.width,
		Ascent:  prepared.ascent,
		Descent: prepared.descent,
		Italic:  prepared.italic,
		Items: display.ItemRange{
			Start: itemStart,
			Count: len(base.Items) - itemStart,
		},
		Hits: display.HitRange{
			Start: hitStart,
			Count: len(base.Hits) - hitStart,
		},
	}
	base.Tag = tag
	if len(base.Lines) == 1 {
		base.Lines[0].Tag = tag
	} else {
		base.Width = maxFloat(base.Width, tag.X+tag.Width)
	}
	base.Ascent = maxFloat(base.Ascent, tag.Ascent)
	base.Descent = maxFloat(base.Descent, tag.Descent)
	base.Italic = tag.Italic

	overflow, err := s.displayItemsOverflow(base.Items, base.Width, base.Ascent, base.Descent)
	if err != nil {
		return nil, err
	}
	tagLogicalOverflow := maxFloat(0, tag.X+tag.Width-base.Width)
	base.Overflow = maxFloat(base.Overflow, maxFloat(overflow, tagLogicalOverflow))
	return base, nil
}

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

type preparedRowTag struct {
	fragment *Fragment
	tag      ir.EquationTag
	width    float32
	ascent   float32
	descent  float32
	italic   float32
	baseline float32
}

type preparedEquationTags struct {
	formula  *preparedSingleLineTag
	rows     []preparedRowTag
	maxWidth float32
}

func (s *layoutState) prepareEquationTags(
	tags []ir.EquationTag,
	root *Fragment,
	env environment,
) (*preparedEquationTags, error) {
	var formulaTags, rowTags int
	for i := range tags {
		tag := &tags[i]
		if tag.Suppressed {
			continue
		}
		switch {
		case tag.Line == ir.FormulaTagLine:
			formulaTags++
		case tag.Line >= 0:
			rowTags++
		default:
			return nil, fmt.Errorf("%w: invalid equation tag line %d", ErrUnsupported, tag.Line)
		}
	}
	switch {
	case formulaTags == 0 && rowTags == 0:
		return nil, nil
	case formulaTags > 0 && rowTags > 0:
		return nil, fmt.Errorf("%w: formula and row equation tags cannot be mixed", ErrUnsupported)
	case formulaTags > 0:
		formula, err := s.prepareSingleLineTag(tags, env)
		if err != nil {
			return nil, err
		}
		return &preparedEquationTags{formula: formula}, nil
	}
	if s.mode != ir.Display {
		return nil, fmt.Errorf("%w: equation tags require display mode", ErrUnsupported)
	}
	if root == nil || len(root.rows) == 0 {
		return nil, fmt.Errorf("%w: row tags require a multiline display root", ErrUnsupported)
	}
	prepared := &preparedEquationTags{
		rows: make([]preparedRowTag, 0, rowTags),
	}
	for i := range tags {
		tag := &tags[i]
		if tag.Suppressed {
			continue
		}
		if tag.Line >= len(root.rows) {
			return nil, fmt.Errorf("%w: equation tag row %d exceeds %d rows", ErrUnsupported, tag.Line, len(root.rows))
		}
		fragment, err := s.tagFragment(tag, env)
		if err != nil {
			return nil, err
		}
		width, ascent, descent, italic, err := s.fragmentHostMetrics(fragment)
		if err != nil {
			return nil, err
		}
		baseline, err := s.host(root.rows[tag.Line].baseline)
		if err != nil {
			return nil, err
		}
		prepared.rows = append(prepared.rows, preparedRowTag{
			fragment: fragment,
			tag:      *tag,
			width:    width,
			ascent:   ascent,
			descent:  descent,
			italic:   italic,
			baseline: baseline,
		})
		prepared.maxWidth = maxFloat(prepared.maxWidth, width)
	}
	return prepared, nil
}

func (s *layoutState) tagFragment(tag *ir.EquationTag, env environment) (*Fragment, error) {
	tagBody := tag.Body
	if tagBody == nil {
		return nil, fmt.Errorf("%w: nil equation tag body", ErrInvalidInput)
	}
	if !tag.Starred {
		open := ir.NewGlyph(tag.Span, "(")
		open.NodeBase().Class = ir.ClassOpening
		closeNode := ir.NewGlyph(tag.Span, ")")
		closeNode.NodeBase().Class = ir.ClassClosing
		fenced := ir.NewFenced(tag.Span, tag.Body)
		fenced.Open = open
		fenced.Close = closeNode
		fenced.Balanced = true
		tagBody = fenced
	}
	return s.layoutNode(tagBody, env, 0)
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

	fragment, err := s.tagFragment(manual, env)
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

func (s *layoutState) attachPreparedRowTags(
	base *display.Box,
	prepared *preparedEquationTags,
) (*display.Box, error) {
	if base == nil || prepared == nil || len(prepared.rows) == 0 {
		return nil, fmt.Errorf("%w: empty row equation tags", ErrInvalidInput)
	}
	if base.Tag != nil {
		return nil, fmt.Errorf("%w: formula and row equation tags cannot be mixed", ErrInvalidInput)
	}
	formulaRight := base.Width
	if len(base.Lines) == 1 {
		formulaRight, _ = s.addHost(base.Lines[0].OffsetX, base.Lines[0].Width)
	}
	columnRight := maxFloat(base.Width, formulaRight+prepared.maxWidth)
	base.Tags = make([]display.TagBox, 0, len(prepared.rows))
	for i := range prepared.rows {
		preparedTag := &prepared.rows[i]
		itemStart := len(base.Items)
		hitStart := len(base.Hits)
		x := columnRight - preparedTag.width
		flatten := newFlattenState(s, base, x, preparedTag.baseline)
		base.Items = reserveDisplayItems(base.Items, preparedTag.fragment.flatItems, s.limits.MaxOutputItems)
		base.Hits = reserveDisplayHits(base.Hits, preparedTag.fragment.flatHits, s.limits.MaxNodes)
		if err := flatten.visit(preparedTag.fragment, 0, 0); err != nil {
			return nil, err
		}
		base.Tags = append(base.Tags, display.TagBox{
			Source:  preparedTag.tag.Span,
			Manual:  preparedTag.tag.Manual,
			Starred: preparedTag.tag.Starred,
			Line:    preparedTag.tag.Line,
			X:       x,
			Y:       preparedTag.baseline,
			Width:   preparedTag.width,
			Ascent:  preparedTag.ascent,
			Descent: preparedTag.descent,
			Italic:  preparedTag.italic,
			Items: display.ItemRange{
				Start: itemStart,
				Count: len(base.Items) - itemStart,
			},
			Hits: display.HitRange{
				Start: hitStart,
				Count: len(base.Hits) - hitStart,
			},
		})
		top := preparedTag.baseline - preparedTag.ascent
		if top < 0 {
			base.Ascent = maxFloat(base.Ascent, -top)
		}
		base.Descent = maxFloat(base.Descent, preparedTag.baseline+preparedTag.descent)
	}
	if len(base.Lines) == 0 {
		base.Width = columnRight
	}
	base.Italic = prepared.rows[len(prepared.rows)-1].italic
	overflow, err := s.displayItemsOverflow(base.Items, base.Width, base.Ascent, base.Descent)
	if err != nil {
		return nil, err
	}
	for i := range base.Tags {
		overflow = maxFloat(overflow, maxFloat(0, base.Tags[i].X+base.Tags[i].Width-base.Width))
	}
	base.Overflow = maxFloat(base.Overflow, overflow)
	return base, nil
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
	if err := flatten.visit(prepared.fragment, 0, 0); err != nil {
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

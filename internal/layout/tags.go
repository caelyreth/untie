package layout

import (
	"fmt"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
)

func hasManualTag(tags []ir.EquationTag) bool {
	for _, tag := range tags {
		if !tag.Suppressed && tag.Manual {
			return true
		}
	}
	return false
}

func (s *layoutState) attachSingleLineTag(
	base *display.Box,
	tags []ir.EquationTag,
	env environment,
) (*display.Box, error) {
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
		return base, nil
	}
	if s.mode != ir.Display {
		return nil, fmt.Errorf("%w: equation tags require display mode", ErrUnsupported)
	}
	if manual.Line != 0 && manual.Line != -1 {
		return nil, fmt.Errorf("%w: equation tag line %d is deferred", ErrUnsupported, manual.Line)
	}
	if len(base.Lines) > 1 {
		return nil, fmt.Errorf("%w: tagged layout produced multiple lines", ErrInvalidInput)
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
	tagBox, err := s.flattenFragment(fragment)
	if err != nil {
		return nil, err
	}
	if err := tagBox.Validate(); err != nil {
		return nil, fmt.Errorf("%w: equation tag: %v", ErrInvalidInput, err)
	}
	if len(base.Items)+len(tagBox.Items) > s.limits.MaxOutputItems {
		return nil, fmt.Errorf("%w: equation tag output item count", ErrLimit)
	}
	pathSegments := 0
	for _, item := range tagBox.Items {
		pathSegments += len(item.Path)
	}
	if pathSegments > s.limits.MaxPathSegments-s.pathSegments {
		return nil, fmt.Errorf("%w: equation tag path segment count", ErrLimit)
	}
	if len(base.Hits)+len(tagBox.Hits) > s.limits.MaxNodes {
		return nil, fmt.Errorf("%w: equation tag source hit count", ErrLimit)
	}

	x := float32(0)
	if len(base.Lines) == 1 {
		line := &base.Lines[0]
		rightEdge, err := s.subHost(base.Width, tagBox.Width)
		if err != nil {
			return nil, err
		}
		if rightEdge < 0 {
			rightEdge = 0
		}
		formulaRight, err := s.addHost(line.Box.Width, line.OffsetX)
		if err != nil {
			return nil, err
		}
		afterFormula, err := s.addHost(formulaRight, 0)
		if err != nil {
			return nil, err
		}
		x = maxFloat(rightEdge, afterFormula)
	} else {
		x, err = s.addHost(base.Width, 0)
		if err != nil {
			return nil, err
		}
	}
	y := float32(0)
	tag := &display.TagBox{
		Box:     tagBox,
		Source:  manual.Span,
		Manual:  manual.Manual,
		Starred: manual.Starred,
		Line:    manual.Line,
		X:       x,
		Y:       y,
	}
	if err := tag.Box.Validate(); err != nil {
		return nil, fmt.Errorf("%w: equation tag: %v", ErrInvalidInput, err)
	}

	for _, item := range tagBox.Items {
		translated, err := s.translateItem(item, tag.X, tag.Y)
		if err != nil {
			return nil, err
		}
		base.Items = append(base.Items, translated)
	}
	for _, hit := range tagBox.Hits {
		translated := hit
		translated.Area.X, err = s.addHost(hit.Area.X, tag.X)
		if err != nil {
			return nil, err
		}
		translated.Area.Y, err = s.addHost(hit.Area.Y, tag.Y)
		if err != nil {
			return nil, err
		}
		base.Hits = append(base.Hits, translated)
	}
	base.Tag = tag
	if len(base.Lines) == 1 {
		base.Lines[0].Tag = tag
	} else {
		base.Width = maxFloat(base.Width, tag.X+tag.Box.Width)
	}
	base.Ascent = maxFloat(base.Ascent, -tag.Y+tag.Box.Ascent)
	base.Descent = maxFloat(base.Descent, tag.Y+tag.Box.Descent)
	base.Italic = tag.Box.Italic
	overflow, err := s.displayItemsOverflow(base.Items, base.Width, base.Ascent, base.Descent)
	if err != nil {
		return nil, err
	}
	tagLogicalOverflow := maxFloat(0, tag.X+tag.Box.Width-base.Width)
	overflow = maxFloat(overflow, tagLogicalOverflow)
	base.Overflow = maxFloat(base.Overflow, overflow)
	return base, nil
}

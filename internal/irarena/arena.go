// Package irarena allocates immutable IR nodes for a single frontend parse.
//
// Large frontend inputs opt into block allocation for the two highest-volume
// node families. Small inputs keep one allocation per node so they do not pay a
// block-sized penalty. Blocks are never reused: every returned pointer remains
// stable for the lifetime of the parsed expression snapshot.
package irarena

import (
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const (
	arenaMinHint = 32
)

// Arena owns the node blocks allocated through it. Callers must treat every
// returned node as immutable after the frontend finishes lowering.
type Arena struct {
	rows   nodes[ir.Row]
	glyphs nodes[ir.Glyph]
}

// New returns an empty node arena.
func New(hint int) *Arena {
	a := &Arena{}
	if hint < arenaMinHint {
		return a
	}
	a.glyphs.enable(max(hint*3/8, 16))
	a.rows.enable(max(hint/8+4, 8))
	return a
}

type nodes[T any] struct {
	block   []T
	next    int
	size    int
	enabled bool
}

func (a *nodes[T]) alloc(value T) *T {
	if !a.enabled {
		out := new(T)
		*out = value
		return out
	}
	if a.next >= len(a.block) {
		a.size = max(a.size*2, 8)
		a.block = make([]T, a.size)
		a.next = 0
	}
	out := &a.block[a.next]
	a.next++
	*out = value
	return out
}

func (a *nodes[T]) enable(size int) {
	a.block = make([]T, size)
	a.size = size
	a.enabled = true
}

// NewRow takes ownership of items without cloning it.
func (a *Arena) NewRow(span source.Range, items ...ir.Node) *ir.Row {
	return a.rows.alloc(ir.Row{Base: ir.NewBase(ir.KindRow, span), Items: items})
}

// NewGlyph constructs a glyph.
func (a *Arena) NewGlyph(span source.Range, text string) *ir.Glyph {
	return a.glyphs.alloc(ir.Glyph{Base: ir.NewBase(ir.KindGlyph, span), Text: text})
}

// NewText constructs a text run in the text face.
func (a *Arena) NewText(span source.Range, text string) *ir.Text {
	return &ir.Text{Base: ir.NewBase(ir.KindText, span), Text: text, Font: ir.FontText}
}

// NewOperator constructs an operator.
func (a *Arena) NewOperator(span source.Range, text string) *ir.Operator {
	return &ir.Operator{Base: ir.NewBase(ir.KindOperator, span), Text: text}
}

// NewSpace constructs an explicit or semantic space.
func (a *Arena) NewSpace(span source.Range, kind ir.SpaceKind, amount ir.Length) *ir.Space {
	return &ir.Space{Base: ir.NewBase(ir.KindSpace, span), Type: kind, Amount: amount}
}

// NewFraction constructs a vertical fraction with the canonical defaults.
func (a *Arena) NewFraction(span source.Range, numerator, denominator ir.Node) *ir.Fraction {
	return &ir.Fraction{
		Base:        ir.NewBase(ir.KindFraction, span),
		Numerator:   numerator,
		Denominator: denominator,
		Bar:         true,
		Style:       ir.FractionVertical,
	}
}

// NewRadical constructs a radical.
func (a *Arena) NewRadical(span source.Range, radicand, index ir.Node) *ir.Radical {
	return &ir.Radical{Base: ir.NewBase(ir.KindRadical, span), Radicand: radicand, Index: index}
}

// NewScripts constructs a scripts node.
func (a *Arena) NewScripts(span source.Range, base ir.Node) *ir.Scripts {
	return &ir.Scripts{Base: ir.NewBase(ir.KindScripts, span), BaseItem: base}
}

// NewFenced takes ownership of body without cloning it.
func (a *Arena) NewFenced(span source.Range, body ...ir.Node) *ir.Fenced {
	return &ir.Fenced{Base: ir.NewBase(ir.KindFenced, span), Body: body}
}

// NewSizedDelimiter constructs a standalone explicitly sized delimiter.
func (a *Arena) NewSizedDelimiter(span source.Range, text string, sizing ir.DelimiterSizing) *ir.SizedDelimiter {
	return &ir.SizedDelimiter{
		Base:   ir.NewBase(ir.KindSizedDelimiter, span),
		Text:   text,
		Sizing: sizing,
	}
}

// NewAccent constructs a stretchy accent with the canonical defaults.
func (a *Arena) NewAccent(span source.Range, body, accent ir.Node, position ir.Position) *ir.Accent {
	return &ir.Accent{
		Base:     ir.NewBase(ir.KindAccent, span),
		Body:     body,
		Accent:   accent,
		Position: position,
		Stretchy: true,
	}
}

// NewLine constructs a line decoration.
func (a *Arena) NewLine(span source.Range, body ir.Node, position ir.Position) *ir.Line {
	return &ir.Line{Base: ir.NewBase(ir.KindLine, span), Body: body, Position: position}
}

// NewCancel constructs a cancellation decoration.
func (a *Arena) NewCancel(span source.Range, body ir.Node) *ir.Cancel {
	return &ir.Cancel{Base: ir.NewBase(ir.KindCancel, span), Body: body}
}

// NewTable takes ownership of rows without cloning it. Cell slices are also
// owned by the returned table.
func (a *Arena) NewTable(span source.Range, rows ...ir.TableRow) *ir.Table {
	return &ir.Table{Base: ir.NewBase(ir.KindTable, span), Rows: rows}
}

// NewMultiline takes ownership of rows and their element slices.
func (a *Arena) NewMultiline(span source.Range, rows ...[]ir.Node) *ir.Multiline {
	return &ir.Multiline{Base: ir.NewBase(ir.KindMultiline, span), Rows: rows}
}

// NewStyle constructs an explicit style wrapper.
func (a *Arena) NewStyle(span source.Range, body ir.Node) *ir.Style {
	return &ir.Style{Base: ir.NewBase(ir.KindStyle, span), Body: body}
}

// NewColor constructs a color wrapper.
func (a *Arena) NewColor(span source.Range, body ir.Node, paint ir.Paint) *ir.Color {
	return &ir.Color{Base: ir.NewBase(ir.KindColor, span), Body: body, Paint: paint}
}

// NewBox constructs an undecorated box.
func (a *Arena) NewBox(span source.Range, body ir.Node) *ir.Box {
	return &ir.Box{Base: ir.NewBase(ir.KindBox, span), Body: body}
}

// NewPhantom constructs a phantom with every extent preserved.
func (a *Arena) NewPhantom(span source.Range, body ir.Node) *ir.Phantom {
	return &ir.Phantom{
		Base:   ir.NewBase(ir.KindPhantom, span),
		Body:   body,
		Width:  true,
		Height: true,
		Depth:  true,
	}
}

// NewError constructs an error-role node.
func (a *Arena) NewError(span source.Range, text string, code source.Code) *ir.ErrorNode {
	base := ir.NewBase(ir.KindError, span)
	base.Paint = ir.Paint{Role: ir.PaintError}
	return &ir.ErrorNode{Base: base, Text: text, Code: code}
}

// NewPlaceholder constructs a placeholder node.
func (a *Arena) NewPlaceholder(span source.Range, reason string) *ir.Placeholder {
	base := ir.NewBase(ir.KindPlaceholder, span)
	base.Paint = ir.Paint{Role: ir.PaintPlaceholder}
	return &ir.Placeholder{Base: base, Reason: reason}
}

// NewBreak constructs a line-break marker.
func (a *Arena) NewBreak(span source.Range, kind ir.BreakKind) *ir.Break {
	return &ir.Break{Base: ir.NewBase(ir.KindBreak, span), Break: kind}
}

// NewAlignMarker constructs an alignment marker.
func (a *Arena) NewAlignMarker(span source.Range) *ir.AlignMarker {
	return &ir.AlignMarker{Base: ir.NewBase(ir.KindAlignMarker, span)}
}

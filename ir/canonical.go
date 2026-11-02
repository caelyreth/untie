package ir

import (
	"encoding/json"
	"fmt"
)

// MarshalCanonical returns deterministic JSON for golden tests and cache
// diagnostics. It is not the final public protocol.
func MarshalCanonical(f *Formula) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("ir: nil formula")
	}
	return json.Marshal(snapshotNode(f.Root))
}

type canonicalNode = any

func snapshotNode(n Node) canonicalNode {
	if n == nil {
		return nil
	}
	base := snapshotBase(n.NodeBase())
	switch n := n.(type) {
	case *Row:
		return map[string]any{"base": base, "items": snapshotNodes(n.Items)}
	case *Glyph:
		return map[string]any{"base": base, "text": n.Text}
	case *Text:
		return map[string]any{"base": base, "text": n.Text, "font": n.Font, "tracking": snapshotLength(n.Tracking)}
	case *Operator:
		return map[string]any{"base": base, "text": n.Text}
	case *Space:
		return map[string]any{"base": base, "space": n.Type, "amount": snapshotLength(n.Amount), "weak": n.Weak}
	case *Fraction:
		return map[string]any{"base": base, "numerator": snapshotNode(n.Numerator), "denominator": snapshotNode(n.Denominator), "bar": n.Bar, "style": n.Style, "padding": snapshotLength(n.Padding)}
	case *Radical:
		return map[string]any{"base": base, "radicand": snapshotNode(n.Radicand), "index": snapshotNode(n.Index)}
	case *Scripts:
		return map[string]any{
			"base": base, "main": snapshotNode(n.BaseItem), "top": snapshotNode(n.Top), "bottom": snapshotNode(n.Bottom),
			"top-left": snapshotNode(n.TopLeft), "bottom-left": snapshotNode(n.BottomLeft),
			"top-right": snapshotNode(n.TopRight), "bottom-right": snapshotNode(n.BottomRight),
		}
	case *Fenced:
		return map[string]any{"base": base, "open": snapshotNode(n.Open), "close": snapshotNode(n.Close), "body": snapshotNodes(n.Body), "balanced": n.Balanced}
	case *Accent:
		return map[string]any{"base": base, "body": snapshotNode(n.Body), "accent": snapshotNode(n.Accent), "position": n.Position, "stretchy": n.Stretchy, "dotless": n.Dotless, "exact-width": n.ExactFrameWidth}
	case *Line:
		return map[string]any{"base": base, "body": snapshotNode(n.Body), "position": n.Position, "thickness": snapshotLength(n.Thickness), "gap": snapshotLength(n.Gap)}
	case *Cancel:
		return map[string]any{"base": base, "body": snapshotNode(n.Body), "cross": n.Cross, "inverted": n.Inverted, "thickness": snapshotLength(n.Thickness)}
	case *Table:
		rows := make([]any, len(n.Rows))
		for i, row := range n.Rows {
			rows[i] = map[string]any{"cells": snapshotNodes(row.Cells), "tag": snapshotNode(row.Tag)}
		}
		return map[string]any{"base": base, "rows": rows, "columns": n.Columns, "row-gap": snapshotLength(n.RowGap), "column-gap": snapshotLength(n.ColumnGap), "horizontal": n.Horizontal, "vertical": n.Vertical}
	case *Multiline:
		rows := make([]any, len(n.Rows))
		for i, row := range n.Rows {
			rows[i] = snapshotNodes(row)
		}
		return map[string]any{"base": base, "rows": rows, "centered": n.Centered}
	case *Style:
		return map[string]any{"base": base, "body": snapshotNode(n.Body), "set-size": n.SetSize, "size": n.NewSize, "set-cramped": n.SetCramped, "cramped": n.Cramped}
	case *Color:
		return map[string]any{"base": base, "body": snapshotNode(n.Body), "paint": snapshotPaint(n.Paint)}
	case *Box:
		return map[string]any{"base": base, "body": snapshotNode(n.Body), "padding": snapshotLengths(n.Padding[:]), "fill": snapshotPaint(n.Fill), "has-fill": n.HasFill, "border": snapshotPaint(n.Border), "has-border": n.HasBorder, "border-width": snapshotLength(n.BorderWidth), "radius": snapshotLength(n.Radius)}
	case *Phantom:
		return map[string]any{"base": base, "body": snapshotNode(n.Body), "width": n.Width, "height": n.Height, "depth": n.Depth}
	case *ErrorNode:
		return map[string]any{"base": base, "text": n.Text, "code": n.Code}
	case *Placeholder:
		return map[string]any{"base": base, "reason": n.Reason}
	case *Break:
		return map[string]any{"base": base, "break": n.Break, "penalty": n.Penalty}
	case *AlignMarker:
		return map[string]any{"base": base}
	default:
		return map[string]any{"base": base, "unsupported": fmt.Sprintf("%T", n)}
	}
}

func snapshotNodes(nodes []Node) []any {
	out := make([]any, len(nodes))
	for i, n := range nodes {
		out[i] = snapshotNode(n)
	}
	return out
}

func snapshotBase(b *Base) map[string]any {
	return map[string]any{
		"kind":            KindName(b.kind),
		"span":            []int{b.Span.Start, b.Span.End},
		"origin":          []int{b.Origin.Start, b.Origin.End},
		"class":           b.Class,
		"size":            b.Size,
		"cramped":         b.Cramped,
		"variant":         b.Variant,
		"paint":           snapshotPaint(b.Paint),
		"limits":          b.Limits,
		"left-spacing":    snapshotLength(b.LeftSpacing),
		"right-spacing":   snapshotLength(b.RightSpacing),
		"ignore-as-space": b.IgnoreAsSpace,
	}
}

func snapshotPaint(p Paint) map[string]any {
	if p.HasExplicit {
		return map[string]any{"rgba": []uint8{p.Explicit.R, p.Explicit.G, p.Explicit.B, p.Explicit.A}}
	}
	return map[string]any{"role": p.Role}
}

func snapshotLength(l Length) []any {
	if l.Unit == LengthZero {
		return nil
	}
	return []any{l.Unit, l.Value}
}

func snapshotLengths(lengths []Length) []any {
	out := make([]any, len(lengths))
	for i, l := range lengths {
		out[i] = snapshotLength(l)
	}
	return out
}

package ir

// Children returns direct child nodes in deterministic semantic order.
func Children(n Node) []Node {
	switch n := n.(type) {
	case nil:
		return nil
	case *Row:
		return n.Items
	case *Glyph, *Text, *Operator, *Space, *ErrorNode, *Placeholder, *Break, *AlignMarker:
		return nil
	case *Fraction:
		return compact(n.Numerator, n.Denominator)
	case *Radical:
		return compact(n.Radicand, n.Index)
	case *Scripts:
		return compact(n.BaseItem, n.Top, n.Bottom, n.TopLeft, n.BottomLeft, n.TopRight, n.BottomRight)
	case *Fenced:
		return append(compact(n.Open), append(append([]Node(nil), n.Body...), n.Close)...)
	case *Accent:
		return compact(n.Body, n.Accent)
	case *Line:
		return compact(n.Body)
	case *Cancel:
		return compact(n.Body)
	case *Table:
		var out []Node
		for _, row := range n.Rows {
			out = append(out, row.Cells...)
			if row.Tag != nil {
				out = append(out, row.Tag)
			}
		}
		return out
	case *Multiline:
		var out []Node
		for _, row := range n.Rows {
			out = append(out, row...)
		}
		return out
	case *Style:
		return compact(n.Body)
	case *Color:
		return compact(n.Body)
	case *Box:
		return compact(n.Body)
	case *Phantom:
		return compact(n.Body)
	default:
		return nil
	}
}

// Walk visits n and its descendants depth-first in deterministic order. If fn
// returns false, that node's children are skipped.
func Walk(n Node, fn func(Node) bool) {
	if n == nil || !fn(n) {
		return
	}
	for _, child := range Children(n) {
		Walk(child, fn)
	}
}

func compact(nodes ...Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

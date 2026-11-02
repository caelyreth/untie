package alt

import (
	"context"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func FuzzGenerateTerminatesAndIsDeterministic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{255, 254, 253, 252, 251, 250})
	f.Fuzz(func(t *testing.T, data []byte) {
		root := randomNode(data, 0)
		limits := source.Limits{
			MaxSourceBytes: 4_096,
			MaxNodes:       128,
			MaxNesting:     24,
			MaxTextBytes:   4_096,
		}
		first, firstErr := GenerateContext(context.Background(), root, ir.Display, limits)
		second, secondErr := GenerateContext(context.Background(), root, ir.Display, limits)
		if (firstErr == nil) != (secondErr == nil) || firstErr != nil && firstErr.Error() != secondErr.Error() {
			t.Fatalf("nondeterministic errors: %v, %v", firstErr, secondErr)
		}
		if first != second {
			t.Fatalf("nondeterministic output:\n%#v\n%#v", first, second)
		}
		if firstErr != nil && (first.Text == "" || first.Speech == "") {
			t.Fatalf("error did not include a visible fallback: %#v", first)
		}
		if err := first.Validate(); err != nil {
			t.Fatalf("invalid generated Alt: %v", err)
		}
	})
}

func randomNode(data []byte, depth int) ir.Node {
	next := func() byte {
		if len(data) == 0 {
			return 0
		}
		value := data[0]
		data = data[1:]
		return value
	}
	text := func() string {
		n := int(next() % 4)
		out := make([]byte, n)
		for i := range out {
			out[i] = 'a' + next()%26
		}
		if len(out) == 0 {
			return "x"
		}
		return string(out)
	}
	span := source.NewRange(0, 1)
	if depth >= 6 {
		return ir.NewGlyph(span, text())
	}
	children := func(count int) []ir.Node {
		out := make([]ir.Node, count)
		for i := range out {
			out[i] = randomNode(data, depth+1)
		}
		return out
	}
	switch next() % 12 {
	case 0:
		return ir.NewRow(span, children(int(next()%4))...)
	case 1:
		return ir.NewGlyph(span, text())
	case 2:
		return ir.NewText(span, text())
	case 3:
		return ir.NewOperator(span, text())
	case 4:
		return ir.NewSpace(span, ir.SpaceKind(next()%3), ir.RawLength(ir.LengthMu, int64(next()%11-5)*1_000_000))
	case 5:
		return ir.NewFraction(span, randomNode(data, depth+1), randomNode(data, depth+1))
	case 6:
		radical := ir.NewRadical(span, randomNode(data, depth+1), nil)
		if next()%2 == 0 {
			radical.Index = randomNode(data, depth+1)
		}
		return radical
	case 7:
		scripts := ir.NewScripts(span, randomNode(data, depth+1))
		scripts.Bottom = randomNode(data, depth+1)
		scripts.Top = randomNode(data, depth+1)
		scripts.BottomLeft = randomNode(data, depth+1)
		scripts.TopRight = randomNode(data, depth+1)
		return scripts
	case 8:
		fence := ir.NewFenced(span, children(int(next()%3))...)
		fence.Open = randomNode(data, depth+1)
		fence.Close = randomNode(data, depth+1)
		return fence
	case 9:
		return ir.NewAccent(span, randomNode(data, depth+1), ir.NewGlyph(span, text()), ir.Position(next()%2))
	case 10:
		return ir.NewTable(span, ir.TableRow{Cells: children(int(next() % 3))})
	default:
		switch next() % 7 {
		case 0:
			return ir.NewMultiline(span, children(int(next()%3)))
		case 1:
			return ir.NewStyle(span, randomNode(data, depth+1))
		case 2:
			return ir.NewColor(span, randomNode(data, depth+1), ir.Paint{})
		case 3:
			return ir.NewBox(span, randomNode(data, depth+1))
		case 4:
			return ir.NewPhantom(span, randomNode(data, depth+1))
		case 5:
			return ir.NewPlaceholder(span, text())
		default:
			return ir.NewError(span, text(), source.CodeSyntax)
		}
	}
}

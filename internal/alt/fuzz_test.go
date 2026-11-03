package alt

import (
	"context"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func FuzzGenerateTerminatesAndIsDeterministic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{255, 254, 253, 252, 251, 250})
	f.Fuzz(func(t *testing.T, data []byte) {
		cursor := &fuzzCursor{data: data}
		root := randomNode(cursor, 0)
		limits := source.Limits{
			MaxSourceBytes: 4_096,
			MaxNodes:       128,
			MaxNesting:     24,
			MaxTextBytes:   4_096,
		}
		for _, full := range []bool{false, true} {
			root, values := attachmentFixture(data, full)
			first, firstErr := GenerateContext(context.Background(), root, ir.Display, limits)
			second, secondErr := GenerateContext(context.Background(), root, ir.Display, limits)
			if firstErr != nil || secondErr != nil {
				t.Fatalf("attachment fixture error = %v, %v", firstErr, secondErr)
			}
			if !first.Equal(second) {
				t.Fatalf("attachment fixture is nondeterministic:\n%#v\n%#v", first, second)
			}
			if err := first.Validate(); err != nil {
				t.Fatalf("attachment fixture is invalid: %v", err)
			}
			assertAttachmentValuesVisible(t, first, values)
		}

		first, firstErr := GenerateContext(context.Background(), root, ir.Display, limits)
		second, secondErr := GenerateContext(context.Background(), root, ir.Display, limits)
		if (firstErr == nil) != (secondErr == nil) || firstErr != nil && firstErr.Error() != secondErr.Error() {
			t.Fatalf("nondeterministic errors: %v, %v", firstErr, secondErr)
		}
		if !first.Equal(second) {
			t.Fatalf("nondeterministic output:\n%#v\n%#v", first, second)
		}
		if firstErr != nil && (first.Text == "" || first.Speech == "") {
			t.Fatalf("error did not include a visible fallback: %#v", first)
		}
		if err := first.Validate(); err != nil {
			t.Fatalf("invalid generated Alt: %v", err)
		}
		if first.Description == nil {
			t.Fatal("generated Alt has no structured description")
		}
		descriptionJSON, err := first.Description.MarshalCanonical()
		if err != nil {
			t.Fatalf("description MarshalCanonical() error = %v", err)
		}
		cloneJSON, err := first.Description.Clone().MarshalCanonical()
		if err != nil {
			t.Fatalf("cloned description MarshalCanonical() error = %v", err)
		}
		if string(descriptionJSON) != string(cloneJSON) {
			t.Fatal("cloned description changed canonical JSON")
		}
		if len(first.Text) > limits.MaxTextBytes || len(first.Speech) > limits.MaxTextBytes {
			t.Fatalf("generated Alt exceeds MaxTextBytes: text=%d speech=%d limit=%d",
				len(first.Text), len(first.Speech), limits.MaxTextBytes)
		}
	})
}

type fuzzCursor struct {
	data []byte
}

func attachmentFixture(data []byte, full bool) (*ir.Scripts, []string) {
	span := source.NewRange(0, 1)
	scripts := ir.NewScripts(span, ir.NewGlyph(span, "x"))
	if len(data) > 0 {
		scripts.Role = ir.ScriptRole(data[0] % 4)
	}
	if full {
		scripts.Top = ir.NewGlyph(span, "TOP_ONLY")
		scripts.Bottom = ir.NewGlyph(span, "BOTTOM_ONLY")
	}
	scripts.TopLeft = ir.NewGlyph(span, "TOP_LEFT")
	scripts.BottomLeft = ir.NewGlyph(span, "BOTTOM_LEFT")
	scripts.TopRight = ir.NewGlyph(span, "TOP_RIGHT")
	scripts.BottomRight = ir.NewGlyph(span, "BOTTOM_RIGHT")
	values := []string{"TOP_LEFT", "BOTTOM_LEFT", "TOP_RIGHT", "BOTTOM_RIGHT"}
	if full {
		values = append(values, "TOP_ONLY", "BOTTOM_ONLY")
	}
	return scripts, values
}

func assertAttachmentValuesVisible(t *testing.T, alt display.Alt, values []string) {
	t.Helper()
	visible := alt.Text + "\n" + alt.Speech
	unsupported := strings.Contains(visible, "unsupported node") ||
		strings.Contains(visible, "[unsupported")
	for _, value := range values {
		if !strings.Contains(visible, value) && !unsupported {
			t.Fatalf("attachment %q was silently lost: %#v", value, alt)
		}
	}
}

func (c *fuzzCursor) next() byte {
	if len(c.data) == 0 {
		return 0
	}
	value := c.data[0]
	c.data = c.data[1:]
	return value
}

func randomNode(cursor *fuzzCursor, depth int) ir.Node {
	text := func() string {
		n := int(cursor.next() % 4)
		out := make([]byte, n)
		for i := range out {
			out[i] = 'a' + cursor.next()%26
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
			out[i] = randomNode(cursor, depth+1)
		}
		return out
	}
	switch cursor.next() % 17 {
	case 0:
		return ir.NewRow(span, children(int(cursor.next()%4))...)
	case 1:
		return ir.NewGlyph(span, text())
	case 2:
		return ir.NewText(span, text())
	case 3:
		return ir.NewOperator(span, text())
	case 4:
		return ir.NewSpace(span, ir.SpaceKind(cursor.next()%3), ir.RawLength(ir.LengthMu, int64(cursor.next()%11-5)*1_000_000))
	case 5:
		fraction := ir.NewFraction(span, randomNode(cursor, depth+1), randomNode(cursor, depth+1))
		switch ir.FractionRole(cursor.next() % 3) {
		case ir.FractionRoleBinomial:
			fraction.Role = ir.FractionRoleBinomial
			fraction.Style = ir.FractionNoBar
			fraction.Bar = false
		case ir.FractionRoleStacked:
			fraction.Role = ir.FractionRoleStacked
			fraction.Style = ir.FractionNoBar
			fraction.Bar = false
		}
		return fraction
	case 6:
		radical := ir.NewRadical(span, randomNode(cursor, depth+1), nil)
		if cursor.next()%2 == 0 {
			radical.Index = randomNode(cursor, depth+1)
		}
		return radical
	case 7:
		scripts := ir.NewScripts(span, randomNode(cursor, depth+1))
		scripts.Bottom = randomNode(cursor, depth+1)
		scripts.Top = randomNode(cursor, depth+1)
		scripts.BottomLeft = randomNode(cursor, depth+1)
		scripts.TopRight = randomNode(cursor, depth+1)
		scripts.Role = ir.ScriptRole(cursor.next() % 4)
		return scripts
	case 8:
		fence := ir.NewFenced(span, children(int(cursor.next()%3))...)
		fence.Open = randomNode(cursor, depth+1)
		fence.Close = randomNode(cursor, depth+1)
		return fence
	case 9:
		return ir.NewAccent(span, randomNode(cursor, depth+1), ir.NewGlyph(span, text()), ir.Position(cursor.next()%2))
	case 10:
		table := ir.NewTable(span,
			ir.TableRow{Cells: children(int(cursor.next() % 3))},
			ir.TableRow{Cells: children(int(cursor.next() % 3))},
		)
		table.Role = ir.TableRole(cursor.next() % 4)
		return table
	case 11:
		return ir.NewLine(span, randomNode(cursor, depth+1), ir.Position(cursor.next()%2))
	case 12:
		cancel := ir.NewCancel(span, randomNode(cursor, depth+1))
		cancel.Role = ir.CancelRole(cursor.next() % 2)
		if cancel.Role == ir.CancelRoleStrike {
			cancel.Cross = cursor.next()%2 == 0
			cancel.Inverted = !cancel.Cross && cursor.next()%2 == 0
		}
		return cancel
	case 13:
		return ir.NewMultiline(span, children(int(cursor.next()%3)))
	case 14:
		switch cursor.next() % 4 {
		case 0:
			return ir.NewStyle(span, randomNode(cursor, depth+1))
		case 1:
			return ir.NewColor(span, randomNode(cursor, depth+1), ir.Paint{})
		case 2:
			return ir.NewBox(span, randomNode(cursor, depth+1))
		default:
			return ir.NewPhantom(span, randomNode(cursor, depth+1))
		}
	case 15:
		texts := []string{"(", ".", "", "()", string([]byte{0xff})}
		text := texts[int(cursor.next())%len(texts)]
		sizings := []ir.DelimiterSizing{
			ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 1_200_000)),
			ir.RelativeDelimiterSizing(ir.RawLength(ir.LengthPercent, 150_000_000)),
			ir.ContentDelimiterSizing(),
			{Kind: ir.DelimiterSizingAbsolute + 1},
			ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthPercent, 100_000_000)),
		}
		node := ir.NewSizedDelimiter(span, text, sizings[int(cursor.next())%len(sizings)])
		switch cursor.next() % 4 {
		case 0:
			node.Class = ir.ClassOpening
		case 1:
			node.Class = ir.ClassClosing
		case 2:
			node.Class = ir.ClassRelation
		}
		return node
	default:
		switch cursor.next() % 4 {
		case 0:
			return ir.NewPlaceholder(span, text())
		case 1:
			return ir.NewError(span, text(), source.CodeSyntax)
		case 2:
			return ir.NewBreak(span, ir.BreakKind(cursor.next()%3))
		default:
			return ir.NewAlignMarker(span)
		}
	}
}

package mathml_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/mathml"
	"github.com/caelyreth/untie/source"
)

func TestMarshalGolden(t *testing.T) {
	t.Parallel()
	span := source.Detached()
	x := ir.NewGlyph(span, "x")
	x.Class = ir.ClassAlphabetic
	plus := ir.NewGlyph(span, "+")
	plus.Class = ir.ClassBinary
	one := ir.NewGlyph(span, "1")
	formula := &ir.Formula{Root: ir.NewRow(span, x, plus, one)}

	got, err := mathml.Marshal(context.Background(), formula, ir.Inline, mathml.Options{})
	if err != nil {
		t.Fatal(err)
	}
	const want = `<math xmlns="http://www.w3.org/1998/Math/MathML" display="inline" data-untie-version="1" alttext="x plus 1" data-untie-alt-text="x+1"><mrow data-untie-kind="row" data-untie-class="normal"><mi data-untie-kind="glyph" data-untie-class="alphabetic">x</mi><mo data-untie-kind="glyph" data-untie-class="binary" form="infix">+</mo><mn data-untie-kind="glyph" data-untie-class="normal">1</mn></mrow></math>`
	if string(got) != want {
		t.Fatalf("Marshal golden changed\nwant: %s\ngot:  %s", want, got)
	}
}

func TestMarshalCoversEveryIRNodeKind(t *testing.T) {
	t.Parallel()
	span := source.Detached()
	glyph := func(text string, class ir.MathClass) *ir.Glyph {
		node := ir.NewGlyph(span, text)
		node.Class = class
		return node
	}

	text := ir.NewText(span, "text")
	text.Tracking = ir.RawLength(ir.LengthEm, 125_000)
	text.Font = ir.FontText

	fraction := ir.NewFraction(span, glyph("1", ir.ClassNormal), glyph("2", ir.ClassNormal))
	relative, err := ir.PercentLength(125)
	if err != nil {
		t.Fatal(err)
	}
	fraction.Padding = ir.RawLength(ir.LengthEm, 100_000)

	radical := ir.NewRadical(span, glyph("x", ir.ClassAlphabetic), glyph("3", ir.ClassNormal))
	fenced := ir.NewFenced(span, glyph("x", ir.ClassAlphabetic))
	fenced.Open = glyph("(", ir.ClassOpening)
	fenced.Close = glyph(")", ir.ClassClosing)
	fenced.Balanced = true
	fenced.Sizing = ir.RelativeDelimiterSizing(relative)

	scripts := ir.NewScripts(span, glyph("x", ir.ClassAlphabetic))
	scripts.Top = glyph("2", ir.ClassNormal)
	scripts.Bottom = glyph("i", ir.ClassAlphabetic)
	scripts.TopLeft = glyph("a", ir.ClassAlphabetic)
	scripts.BottomLeft = glyph("b", ir.ClassAlphabetic)
	scripts.TopRight = glyph("c", ir.ClassAlphabetic)
	scripts.BottomRight = glyph("d", ir.ClassAlphabetic)
	scripts.Role = ir.ScriptRoleLimits

	accent := ir.NewAccent(span, glyph("x", ir.ClassAlphabetic), glyph("^", ir.ClassDiacritic), ir.PositionAbove)
	line := ir.NewLine(span, glyph("x", ir.ClassAlphabetic), ir.PositionBelow)
	line.Thickness = ir.RawLength(ir.LengthEm, 20_000)
	line.Gap = ir.RawLength(ir.LengthEm, 30_000)
	cancel := ir.NewCancel(span, glyph("=", ir.ClassRelation))
	cancel.Cross = true
	cancel.Inverted = true
	cancel.Thickness = ir.RawLength(ir.LengthEm, 20_000)

	table := ir.NewTable(span, ir.TableRow{Cells: []ir.Node{
		glyph("a", ir.ClassAlphabetic),
		glyph("b", ir.ClassAlphabetic),
	}})
	table.Role = ir.TableRoleCases
	table.Columns = []ir.ColumnAlign{ir.AlignLeft, ir.AlignRight}
	table.RowGap = ir.RawLength(ir.LengthEm, 100_000)
	table.ColumnGap = ir.RawLength(ir.LengthEm, 200_000)
	table.Horizontal = []ir.TableLine{{Position: 0, Count: 2, Dashed: true, Thickness: ir.RawLength(ir.LengthPt, 500_000)}}

	multiline := ir.NewMultiline(span, []ir.Node{
		glyph("x", ir.ClassAlphabetic),
		ir.NewAlignMarker(span),
		glyph("y", ir.ClassAlphabetic),
	})
	multiline.Centered = true

	style := ir.NewStyle(span, glyph("x", ir.ClassAlphabetic))
	style.SetSize = true
	style.NewSize = ir.SizeScript
	style.SetCramped = true
	style.Cramped = true
	style.Size = ir.SizeDisplay
	style.Paint = ir.Paint{
		Explicit: ir.RGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff}, HasExplicit: true,
	}

	color := ir.NewColor(span, glyph("x", ir.ClassAlphabetic), ir.Paint{
		Explicit: ir.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x78}, HasExplicit: true,
	})
	color.Paint = ir.Paint{
		Explicit: ir.RGBA{R: 0x98, G: 0x76, B: 0x54, A: 0x32}, HasExplicit: true,
	}

	box := ir.NewBox(span, glyph("x", ir.ClassAlphabetic))
	box.Padding = [4]ir.Length{
		ir.RawLength(ir.LengthEm, 10_000),
		ir.RawLength(ir.LengthEm, 20_000),
		ir.RawLength(ir.LengthEm, 30_000),
		ir.RawLength(ir.LengthEm, 40_000),
	}
	box.HasFill = true
	box.Fill = ir.Paint{Explicit: ir.RGBA{R: 1, G: 2, B: 3, A: 0xff}, HasExplicit: true}
	box.HasBorder = true
	box.Border = ir.Paint{Role: ir.PaintError}
	box.BorderWidth = ir.RawLength(ir.LengthEm, 10_000)
	box.Radius = ir.RawLength(ir.LengthEm, 20_000)

	phantom := ir.NewPhantom(span, glyph("x", ir.ClassAlphabetic))
	phantom.Height = false

	items := []ir.Node{
		glyph("x", ir.ClassAlphabetic),
		text,
		ir.NewOperator(span, "+"),
		ir.NewSpace(span, ir.SpaceExplicit, ir.RawLength(ir.LengthEm, 250_000)),
		fraction,
		radical,
		scripts,
		fenced,
		ir.NewSizedDelimiter(span, "(", ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 2_000_000))),
		accent,
		line,
		cancel,
		table,
		multiline,
		style,
		color,
		box,
		phantom,
		ir.NewError(span, "bad", source.CodeUnknownCommand),
		ir.NewPlaceholder(span, "missing font"),
		ir.NewBreak(span, ir.BreakRelation),
		ir.NewAlignMarker(span),
	}
	root := ir.NewRow(span, items...)
	root.NodeID = identity.NodeID(1)
	for i, item := range items {
		item.NodeBase().NodeID = identity.NodeID(i + 2)
	}
	tag := glyph("1", ir.ClassNormal)
	tag.NodeID = identity.NodeID(100)
	formula := &ir.Formula{
		Root: root,
		Tags: []ir.EquationTag{{
			Body: tag, Span: span, Origin: span, Manual: true, Line: ir.FormulaTagLine,
		}},
	}

	output, err := mathml.Marshal(context.Background(), formula, ir.Display, mathml.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for kind := ir.KindRow; kind <= ir.KindSizedDelimiter; kind++ {
		want := `data-untie-kind="` + ir.KindName(kind) + `"`
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("output does not contain %s", want)
		}
	}
	for _, element := range []string{
		"mmultiscripts", "mroot", "menclose", "mtable", "mstyle", "mphantom",
		"merror", "mlabeledtr", "mspace",
	} {
		if !bytes.Contains(output, []byte("<"+element)) {
			t.Errorf("output does not contain <%s>", element)
		}
	}
	if err := validateXML(output); err != nil {
		t.Fatalf("output is not well-formed XML: %v\n%s", err, output)
	}
}

func TestMarshalDeterministicEscapingAndAccessibility(t *testing.T) {
	t.Parallel()
	text := ir.NewText(source.Detached(), `a<&>"'`)
	formula := &ir.Formula{Root: ir.NewRow(source.Detached(), text)}
	options := mathml.Options{}
	first, err := mathml.Marshal(context.Background(), formula, ir.Inline, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := mathml.Marshal(context.Background(), formula, ir.Inline, options)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Marshal output is not deterministic")
	}
	for _, fragment := range []string{
		`a&lt;&amp;&gt;&#34;&#39;`,
		`alttext=`,
		`display="inline"`,
		`xmlns="http://www.w3.org/1998/Math/MathML"`,
	} {
		if !strings.Contains(string(first), fragment) {
			t.Errorf("output does not contain %q:\n%s", fragment, first)
		}
	}
}

func TestWriteMatchesMarshal(t *testing.T) {
	t.Parallel()
	formula := &ir.Formula{Root: ir.NewRow(source.Detached(), ir.NewGlyph(source.Detached(), "x"))}
	got, err := mathml.Marshal(context.Background(), formula, ir.Display, mathml.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var streamed bytes.Buffer
	if err := mathml.Write(context.Background(), &streamed, formula, ir.Display, mathml.Options{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(streamed.Bytes(), got) {
		t.Fatalf("Write and Marshal differ\nWrite:   %s\nMarshal: %s", streamed.Bytes(), got)
	}
}

func TestMarshalHonorsCancellationAndOutputLimit(t *testing.T) {
	t.Parallel()
	formula := &ir.Formula{Root: ir.NewRow(source.Detached(), ir.NewGlyph(source.Detached(), "x"))}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mathml.Marshal(ctx, formula, ir.Inline, mathml.Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Marshal error = %v", err)
	}
	if _, err := mathml.Marshal(context.Background(), formula, ir.Inline, mathml.Options{MaxOutputBytes: 16}); !errors.Is(err, mathml.ErrLimitExceeded) {
		t.Fatalf("limited Marshal error = %v, want ErrLimitExceeded", err)
	}
}

func TestMarshalRejectsInlineEquationTags(t *testing.T) {
	t.Parallel()
	formula := &ir.Formula{
		Root: ir.NewRow(source.Detached(), ir.NewGlyph(source.Detached(), "x")),
		Tags: []ir.EquationTag{{
			Body: ir.NewGlyph(source.Detached(), "1"), Manual: true, Line: ir.FormulaTagLine,
		}},
	}
	if _, err := mathml.Marshal(context.Background(), formula, ir.Inline, mathml.Options{}); !errors.Is(err, mathml.ErrInvalidFormula) {
		t.Fatalf("Marshal error = %v, want ErrInvalidFormula", err)
	}
}

func TestMarshalRejectsXMLInvalidText(t *testing.T) {
	t.Parallel()
	formula := &ir.Formula{Root: ir.NewRow(source.Detached(), ir.NewText(source.Detached(), "bad\x00text"))}
	if _, err := mathml.Marshal(context.Background(), formula, ir.Inline, mathml.Options{}); !errors.Is(err, mathml.ErrInvalidFormula) {
		t.Fatalf("Marshal error = %v, want ErrInvalidFormula", err)
	}
}

func validateXML(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if start, ok := token.(xml.StartElement); ok {
			seen := make(map[xml.Name]struct{}, len(start.Attr))
			for _, attr := range start.Attr {
				if _, exists := seen[attr.Name]; exists {
					return errors.New("duplicate XML attribute")
				}
				seen[attr.Name] = struct{}{}
			}
		}
	}
}

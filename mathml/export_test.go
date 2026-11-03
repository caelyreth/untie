package mathml_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/caelyreth/untie"
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
	fontSize, err := ir.PercentLength(150)
	if err != nil {
		t.Fatal(err)
	}
	style.SetFontSize = true
	style.FontSize = fontSize
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
	overlay := ir.NewOverlay(span, glyph("x", ir.ClassAlphabetic), ir.OverlayCenter)
	raise := ir.NewRaise(span, glyph("x", ir.ClassAlphabetic), ir.RawLength(ir.LengthEm, 250_000))

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
		overlay,
		raise,
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
	for kind := ir.KindRow; kind <= ir.KindRaise; kind++ {
		want := `data-untie-kind="` + ir.KindName(kind) + `"`
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("output does not contain %s", want)
		}
	}
	for _, element := range []string{
		"mmultiscripts", "mroot", "menclose", "mpadded", "mtable", "mstyle",
		"mphantom", "merror", "mlabeledtr", "mspace",
	} {
		if !bytes.Contains(output, []byte("<"+element)) {
			t.Errorf("output does not contain <%s>", element)
		}
	}
	if err := validateXML(output); err != nil {
		t.Fatalf("output is not well-formed XML: %v\n%s", err, output)
	}
	if !bytes.Contains(output, []byte(`mathsize="150%"`)) ||
		!bytes.Contains(output, []byte(`data-untie-overlay-alignment="center"`)) ||
		!bytes.Contains(output, []byte(`data-untie-raise-amount="0.25em"`)) {
		t.Fatalf("new IR metadata missing from output:\n%s", output)
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

func TestMarshalRowTagsAsLabeledRows(t *testing.T) {
	t.Parallel()
	span := source.Detached()
	multiline := ir.NewMultiline(span,
		[]ir.Node{ir.NewGlyph(span, "x")},
		[]ir.Node{ir.NewGlyph(span, "y")},
		[]ir.Node{ir.NewGlyph(span, "z")},
	)
	root := ir.NewRow(span, multiline)
	formula := &ir.Formula{
		Root: root,
		Tags: []ir.EquationTag{
			{Body: ir.NewGlyph(span, "A"), Span: span, Origin: span, Manual: true, Line: 0},
			{Body: ir.NewGlyph(span, "B"), Span: span, Origin: span, Manual: true, Starred: true, Line: 1},
			{Span: span, Origin: span, Suppressed: true, Line: 2},
		},
	}
	output, err := mathml.Marshal(context.Background(), formula, ir.Display, mathml.Options{})
	if err != nil {
		t.Fatal(err)
	}
	document := string(output)
	for _, want := range []string{
		`<mlabeledtr data-untie-tag-line="0"`,
		`data-untie-tag-starred="false"`,
		`data-untie-tag-line="1"`,
		`data-untie-tag-starred="true"`,
		`<mtr data-untie-tag-line="2"`,
		`data-untie-tag-suppressed="true"`,
		`data-untie-suppressed-tag-lines="2"`,
		`data-untie-kind="tag-label"`,
		`<mo fence="true" form="prefix">(</mo>`,
	} {
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("row-tag output does not contain %q:\n%s", want, document)
		}
	}
	if err := validateXML(output); err != nil {
		t.Fatalf("row-tag output is not well-formed XML: %v\n%s", err, output)
	}
}

func TestMarshalTeXRowTags(t *testing.T) {
	t.Parallel()
	expression := untie.ParseTeX(
		`\begin{align}x&=1\tag{A}\\y&=2\tag*{B}\\z&=3\nonumber\end{align}`,
		untie.Display,
		untie.ParseOptions{},
	)
	if diagnostics := expression.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("ParseTeX diagnostics = %#v", diagnostics)
	}
	formula, err := expression.RootSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	output, err := mathml.Marshal(context.Background(), formula, untie.Display, mathml.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-untie-tag-line="0"`,
		`data-untie-tag-line="1"`,
		`data-untie-tag-line="2"`,
		`data-untie-suppressed-tag-lines="2"`,
	} {
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("TeX row-tag output does not contain %q:\n%s", want, output)
		}
	}
}

func TestMarshalSuppressedRowTagsWithoutMultilineRoot(t *testing.T) {
	t.Parallel()
	span := source.Detached()
	formula := &ir.Formula{
		Root: ir.NewRow(span, ir.NewGlyph(span, "x")),
		Tags: []ir.EquationTag{{
			Span: span, Origin: span, Suppressed: true, Line: 3,
		}},
	}
	output, err := mathml.Marshal(context.Background(), formula, ir.Display, mathml.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`data-untie-suppressed-tags="1"`)) ||
		!bytes.Contains(output, []byte(`data-untie-suppressed-tag-lines="3"`)) {
		t.Fatalf("suppressed row metadata missing:\n%s", output)
	}
}

func TestMarshalRowTagsRequireMultilineDisplayRoot(t *testing.T) {
	t.Parallel()
	span := source.Detached()
	formula := &ir.Formula{
		Root: ir.NewRow(span, ir.NewGlyph(span, "x")),
		Tags: []ir.EquationTag{{
			Body: ir.NewGlyph(span, "A"), Span: span, Origin: span, Manual: true, Line: 0,
		}},
	}
	if _, err := mathml.Marshal(context.Background(), formula, ir.Display, mathml.Options{}); !errors.Is(err, mathml.ErrInvalidFormula) {
		t.Fatalf("Marshal error = %v, want ErrInvalidFormula", err)
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

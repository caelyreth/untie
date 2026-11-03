package mathml_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/caelyreth/untie"
	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/mathml"
	"github.com/caelyreth/untie/source"
)

func TestRoundTripGroundedInKaTeXFixtures(t *testing.T) {
	t.Parallel()
	wantIDs := map[string]bool{
		"parser/sup-sub": false,
		"parser/frac":    false,
		"parser/root":    false,
		"parser/matrix":  false,
		"parser/text":    false,
	}
	records := readKaTeXFixtures(t)
	for _, record := range records {
		_, ok := wantIDs[record.ID]
		if !ok {
			continue
		}
		wantIDs[record.ID] = true
		record := record
		t.Run(record.ID, func(t *testing.T) {
			expression := untie.ParseTeX(record.Source, untie.Display, untie.ParseOptions{})
			if diagnostics := expression.Diagnostics(); len(diagnostics) != 0 {
				t.Fatalf("ParseTeX diagnostics = %#v", diagnostics)
			}
			formula, err := expression.RootSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			assertRoundTrip(t, formula, ir.Display)
		})
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("fixture %q was not found", id)
		}
	}
}

func TestRoundTripSupportedMetadataAndNodes(t *testing.T) {
	t.Parallel()
	span := source.Detached()
	baseClass := ir.ClassAlphabetic
	baseSize := ir.SizeScript
	glyph := func(id uint64, text string, class ir.MathClass) *ir.Glyph {
		node := ir.NewGlyph(span, text)
		node.NodeID = identity.NodeID(id)
		node.Class = class
		node.Size = baseSize
		node.Cramped = true
		node.Variant = ir.VariantBold
		node.Paint = ir.Paint{Explicit: ir.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44}, HasExplicit: true}
		node.Limits = ir.LimitsNever
		node.LeftSpacing = ir.RawLength(ir.LengthEm, 125_000)
		node.RightSpacing = ir.RawLength(ir.LengthEx, -62_500)
		node.IgnoreAsSpace = true
		return node
	}
	text := ir.NewText(span, "hello")
	text.NodeID = identity.NodeID(10)
	text.Class = baseClass
	text.Tracking = ir.RawLength(ir.LengthEm, 50_000)
	text.Font = ir.FontText

	fraction := ir.NewFraction(span, glyph(11, "1", ir.ClassNormal), glyph(12, "2", ir.ClassNormal))
	fraction.NodeID = identity.NodeID(13)
	fraction.Style = ir.FractionNoBar
	fraction.Bar = false
	fraction.Role = ir.FractionRoleBinomial
	fraction.Padding = ir.RawLength(ir.LengthEm, 75_000)

	scripts := ir.NewScripts(span, glyph(14, "∑", ir.ClassLarge))
	scripts.NodeID = identity.NodeID(15)
	scripts.Top = glyph(16, "n", ir.ClassAlphabetic)
	scripts.Bottom = glyph(17, "i", ir.ClassAlphabetic)
	scripts.Limits = ir.LimitsAlways
	scripts.Role = ir.ScriptRoleLimits

	space := ir.NewSpace(span, ir.SpaceExplicit, ir.RawLength(ir.LengthMu, 2_500_000))
	space.NodeID = identity.NodeID(18)
	space.Weak = true

	breakNode := ir.NewBreak(span, ir.BreakBinary)
	breakNode.NodeID = identity.NodeID(19)
	marker := ir.NewAlignMarker(span)
	marker.NodeID = identity.NodeID(20)
	fenced := ir.NewFenced(span, text, fraction)
	fenced.NodeID = identity.NodeID(21)
	fenced.Open = glyph(22, "(", ir.ClassOpening)
	fenced.Close = glyph(23, ")", ir.ClassClosing)
	fenced.Balanced = true
	fenced.Sizing = ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 2_000_000))
	formula := &ir.Formula{Root: ir.NewRow(span, glyph(1, "x", baseClass), scripts, space, breakNode, marker, fenced)}
	formula.Root.NodeBase().NodeID = identity.NodeID(2)
	assertRoundTrip(t, formula, ir.Display)
}

func TestRoundTripGeneratedSupportedTrees(t *testing.T) {
	t.Parallel()
	for seed := int64(1); seed <= 40; seed++ {
		generator := treeGenerator{random: rand.New(rand.NewSource(seed)), nextID: 1}
		root := generator.row(0)
		formula := &ir.Formula{Root: root}
		mode := ir.Inline
		if seed%2 == 0 {
			mode = ir.Display
		}
		if err := ir.Validate(formula, source.Limits{}); err != nil {
			t.Fatalf("seed %d generated invalid IR: %v", seed, err)
		}
		assertRoundTrip(t, formula, mode)
	}
}

func assertRoundTrip(t *testing.T, formula *ir.Formula, mode ir.Mode) {
	t.Helper()
	output, err := mathml.Marshal(context.Background(), formula, mode, mathml.Options{})
	if err != nil {
		t.Fatalf("Marshal: %v\n%s", err, output)
	}
	result, err := mathml.Unmarshal(context.Background(), output, mathml.Options{})
	if err != nil {
		t.Fatalf("Unmarshal: %v diagnostics=%+v\n%s", err, result.Diagnostics, output)
	}
	if result.Mode != mode {
		t.Fatalf("mode = %v, want %v", result.Mode, mode)
	}
	got := normalizedCanonical(t, result.Formula)
	want := normalizedCanonical(t, formula)
	if !bytes.Equal(got, want) {
		t.Fatalf("round-trip changed canonical IR\nwant: %s\ngot:  %s\nMathML: %s", want, got, output)
	}
}

func normalizedCanonical(t *testing.T, formula *ir.Formula) []byte {
	t.Helper()
	data, err := normalizedCanonicalValue(formula)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func normalizedCanonicalValue(formula *ir.Formula) ([]byte, error) {
	cloned, err := ir.CloneFormula(formula)
	if err != nil {
		return nil, err
	}
	if err := ir.WalkFormula(cloned, source.Limits{}, func(node ir.Node) bool {
		base := node.NodeBase()
		base.Span = source.Detached()
		base.Origin = source.Detached()
		if text, ok := node.(*ir.Text); ok {
			text.Clusters = nil
		}
		return true
	}); err != nil {
		return nil, err
	}
	for i := range cloned.Tags {
		cloned.Tags[i].Span = source.Detached()
		cloned.Tags[i].Origin = source.Detached()
	}
	data, err := ir.MarshalCanonical(cloned)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type katexFixture struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Kind   string `json:"kind"`
}

func readKaTeXFixtures(t *testing.T) []katexFixture {
	t.Helper()
	path := filepath.Join("..", "testdata", "upstream", "katex", "cases.jsonl")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close %s: %v", path, err)
		}
	}()
	var out []katexFixture
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		var record katexFixture
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		out = append(out, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

type treeGenerator struct {
	random *rand.Rand
	nextID uint64
}

func (g *treeGenerator) id() identity.NodeID {
	id := identity.NodeID(g.nextID)
	g.nextID++
	return id
}

func (g *treeGenerator) base(node ir.Node) {
	base := node.NodeBase()
	base.NodeID = g.id()
	base.Class = ir.MathClass(g.random.Intn(len(classNamesForTest)))
	base.Size = ir.MathSize(g.random.Intn(5))
	base.Cramped = g.random.Intn(2) == 0
	base.Variant = ir.Variant(g.random.Intn(13))
	if g.random.Intn(3) == 0 {
		base.Paint = ir.Paint{Explicit: ir.RGBA{
			R: uint8(g.random.Intn(256)),
			G: uint8(g.random.Intn(256)),
			B: uint8(g.random.Intn(256)),
			A: uint8(g.random.Intn(256)),
		}, HasExplicit: true}
	} else if g.random.Intn(4) == 0 {
		base.Paint = ir.Paint{Role: ir.PaintRole(g.random.Intn(3))}
	}
	base.Limits = ir.LimitsPlacement(g.random.Intn(4))
	if g.random.Intn(3) == 0 {
		base.LeftSpacing = ir.RawLength(ir.LengthUnit(1+g.random.Intn(5)), int64(g.random.Intn(500_001)-250_000))
	}
	if g.random.Intn(3) == 0 {
		base.RightSpacing = ir.RawLength(ir.LengthUnit(1+g.random.Intn(5)), int64(g.random.Intn(500_001)-250_000))
	}
	base.IgnoreAsSpace = g.random.Intn(2) == 0
}

func (g *treeGenerator) row(depth int) *ir.Row {
	count := 1 + g.random.Intn(4)
	items := make([]ir.Node, count)
	for i := range items {
		items[i] = g.node(depth + 1)
	}
	row := ir.NewRow(source.Detached(), items...)
	g.base(row)
	return row
}

func (g *treeGenerator) node(depth int) ir.Node {
	if depth >= 4 {
		return g.leaf()
	}
	switch g.random.Intn(12) {
	case 0:
		return g.row(depth)
	case 1:
		fraction := ir.NewFraction(source.Detached(), g.node(depth+1), g.node(depth+1))
		style := ir.FractionStyle(g.random.Intn(4))
		fraction.Style = style
		fraction.Bar = style == ir.FractionVertical
		fraction.Role = ir.FractionRoleFraction
		if fraction.Style != ir.FractionVertical {
			fraction.Bar = false
		}
		if g.random.Intn(2) == 0 {
			fraction.Style = ir.FractionNoBar
			fraction.Bar = false
			fraction.Role = ir.FractionRole(g.random.Intn(3))
		}
		g.base(fraction)
		return fraction
	case 2:
		var index ir.Node
		if g.random.Intn(2) == 0 {
			index = g.node(depth + 1)
		}
		radical := ir.NewRadical(source.Detached(), g.node(depth+1), index)
		g.base(radical)
		return radical
	case 3:
		scripts := ir.NewScripts(source.Detached(), g.node(depth+1))
		if g.random.Intn(2) == 0 {
			scripts.Top = g.node(depth + 1)
		}
		if g.random.Intn(2) == 0 {
			scripts.Bottom = g.node(depth + 1)
		}
		if scripts.Top == nil && scripts.Bottom == nil {
			scripts.Top = g.node(depth + 1)
		}
		scripts.Role = ir.ScriptRole(g.random.Intn(4))
		g.base(scripts)
		return scripts
	case 4:
		fenced := ir.NewFenced(source.Detached(), g.node(depth+1))
		fenced.Open = g.leaf()
		fenced.Close = g.leaf()
		fenced.Open.NodeBase().Class = ir.ClassOpening
		fenced.Close.NodeBase().Class = ir.ClassClosing
		fenced.Balanced = true
		switch g.random.Intn(3) {
		case 0:
			fenced.Sizing = ir.ContentDelimiterSizing()
		case 1:
			value, _ := ir.PercentLength(float64(50 + g.random.Intn(151)))
			fenced.Sizing = ir.RelativeDelimiterSizing(value)
		case 2:
			fenced.Sizing = ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, int64(g.random.Intn(4_000_001))))
		}
		g.base(fenced)
		return fenced
	case 5:
		accent := ir.NewAccent(source.Detached(), g.node(depth+1), g.leaf(), ir.Position(g.random.Intn(2)))
		accent.Stretchy = g.random.Intn(2) == 0
		accent.NaturalFit = g.random.Intn(2) == 0
		accent.Dotless = g.random.Intn(2) == 0
		accent.ExactFrameWidth = g.random.Intn(2) == 0
		g.base(accent)
		return accent
	case 6:
		table := ir.NewTable(source.Detached(),
			ir.TableRow{Cells: []ir.Node{g.node(depth + 1), g.node(depth + 1)}},
			ir.TableRow{Cells: []ir.Node{g.node(depth + 1), g.node(depth + 1)}},
		)
		table.Role = ir.TableRole(g.random.Intn(4))
		for range 2 {
			table.Columns = append(table.Columns, ir.ColumnAlign(g.random.Intn(3)))
		}
		table.RowGap = ir.RawLength(ir.LengthEm, int64(g.random.Intn(200_001)))
		table.ColumnGap = ir.RawLength(ir.LengthEm, int64(g.random.Intn(200_001)))
		g.base(table)
		return table
	case 7:
		multiline := ir.NewMultiline(source.Detached(), []ir.Node{g.node(depth + 1)}, []ir.Node{g.node(depth + 1)})
		multiline.Centered = g.random.Intn(2) == 0
		g.base(multiline)
		return multiline
	case 8:
		spaceKind := ir.SpaceKind(g.random.Intn(3))
		amount := ir.Length{}
		if spaceKind == ir.SpaceExplicit {
			amount = ir.RawLength(ir.LengthEm, int64(g.random.Intn(1_000_001)))
		}
		space := ir.NewSpace(source.Detached(), spaceKind, amount)
		space.Weak = g.random.Intn(2) == 0
		g.base(space)
		return space
	case 9:
		delimiter := ir.NewSizedDelimiter(source.Detached(), "(", ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, int64(1+g.random.Intn(4_000_000)))))
		g.base(delimiter)
		delimiter.Class = []ir.MathClass{ir.ClassNormal, ir.ClassOpening, ir.ClassClosing, ir.ClassRelation}[g.random.Intn(4)]
		return delimiter
	case 10:
		breakNode := ir.NewBreak(source.Detached(), ir.BreakKind(g.random.Intn(3)))
		g.base(breakNode)
		return breakNode
	default:
		return g.leaf()
	}
}

func (g *treeGenerator) leaf() ir.Node {
	switch g.random.Intn(5) {
	case 0, 1:
		classes := []ir.MathClass{
			ir.ClassAlphabetic, ir.ClassNormal, ir.ClassBinary, ir.ClassRelation,
			ir.ClassOpening, ir.ClassClosing, ir.ClassFence, ir.ClassPunctuation,
		}
		node := ir.NewGlyph(source.Detached(), []string{"x", "1", "+", "=", "(", ")", "a"}[g.random.Intn(7)])
		node.Class = classes[g.random.Intn(len(classes))]
		g.base(node)
		return node
	case 2:
		node := ir.NewOperator(source.Detached(), []string{"+", "=", ",", "∑"}[g.random.Intn(4)])
		g.base(node)
		return node
	case 3:
		node := ir.NewText(source.Detached(), []string{"abc", "a b", "中"}[g.random.Intn(3)])
		node.Font = ir.FontRole(g.random.Intn(2))
		if g.random.Intn(3) == 0 {
			node.Tracking = ir.RawLength(ir.LengthEm, int64(g.random.Intn(200_001)))
		}
		g.base(node)
		return node
	default:
		node := ir.NewAlignMarker(source.Detached())
		g.base(node)
		return node
	}
}

var classNamesForTest = [...]string{
	"normal", "alphabetic", "binary", "relation", "opening", "closing",
	"fence", "punctuation", "large", "unary", "varying", "inner",
	"diacritic", "space", "special",
}

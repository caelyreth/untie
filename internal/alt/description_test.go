package alt

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestDescriptionCoversEveryPublicIRNodeKind(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	glyph := func(text string) ir.Node { return ir.NewGlyph(span, text) }
	fraction := ir.NewFraction(span, glyph("1"), glyph("2"))
	fraction.Role = ir.FractionRoleBinomial
	fraction.Style = ir.FractionNoBar
	scripts := ir.NewScripts(span, glyph("x"))
	scripts.Role = ir.ScriptRoleLimits
	scripts.Top = glyph("n")
	table := ir.NewTable(span, ir.TableRow{Cells: []ir.Node{glyph("x")}})
	table.Role = ir.TableRoleCases
	table.Columns = []ir.ColumnAlign{ir.AlignLeft}

	tests := map[string]struct {
		node ir.Node
		kind display.AltNodeKind
	}{
		"row":             {ir.NewRow(span, glyph("x"), glyph("y")), display.AltKindRow},
		"glyph":           {glyph("x"), display.AltKindGlyph},
		"text":            {ir.NewText(span, "text"), display.AltKindText},
		"operator":        {ir.NewOperator(span, "sin"), display.AltKindOperator},
		"space":           {ir.NewSpace(span, ir.SpaceInterword, ir.RawLength(ir.LengthMu, 3_000_000)), display.AltKindSpace},
		"fraction":        {fraction, display.AltKindFraction},
		"radical":         {ir.NewRadical(span, glyph("x"), glyph("3")), display.AltKindRadical},
		"scripts":         {scripts, display.AltKindScripts},
		"fenced":          {validDescriptionFence(span, glyph), display.AltKindFenced},
		"sized-delimiter": {ir.NewSizedDelimiter(span, "(", ir.ContentDelimiterSizing()), display.AltKindSizedDelimiter},
		"accent":          {ir.NewAccent(span, glyph("x"), glyph("^"), ir.PositionAbove), display.AltKindAccent},
		"line":            {ir.NewLine(span, glyph("x"), ir.PositionBelow), display.AltKindLine},
		"cancel":          {ir.NewCancel(span, glyph("x")), display.AltKindCancel},
		"table":           {table, display.AltKindTable},
		"multiline":       {ir.NewMultiline(span, []ir.Node{glyph("x")}), display.AltKindMultiline},
		"style":           {ir.NewStyle(span, glyph("x")), display.AltKindStyle},
		"overlay":         {ir.NewOverlay(span, glyph("x"), ir.OverlayCenter), display.AltKindOverlay},
		"raise":           {ir.NewRaise(span, glyph("x"), ir.RawLength(ir.LengthEm, 500_000)), display.AltKindRaise},
		"color":           {ir.NewColor(span, glyph("x"), ir.Paint{Role: ir.PaintInk}), display.AltKindColor},
		"box":             {ir.NewBox(span, glyph("x")), display.AltKindBox},
		"phantom":         {ir.NewPhantom(span, glyph("x")), display.AltKindPhantom},
		"error":           {ir.NewError(span, "bad", source.CodeSyntax), display.AltKindError},
		"placeholder":     {ir.NewPlaceholder(span, "missing"), display.AltKindPlaceholder},
		"break":           {ir.NewBreak(span, ir.BreakExplicit), display.AltKindBreak},
		"align-marker":    {ir.NewAlignMarker(span), display.AltKindAlignMarker},
	}
	if len(tests) != 25 {
		t.Fatalf("fixture covers %d node kinds, want 25", len(tests))
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := GenerateContext(context.Background(), tc.node, ir.Inline, source.Limits{})
			if err != nil {
				t.Fatalf("GenerateContext() error = %v", err)
			}
			if out.Description == nil {
				t.Fatal("Description = nil")
			}
			if out.Description.Root.Kind != tc.kind {
				t.Fatalf("Description.Root.Kind = %q, want %q",
					out.Description.Root.Kind, tc.kind)
			}
			if err := out.Description.Validate(); err != nil {
				t.Fatalf("Description.Validate() error = %v", err)
			}
		})
	}
}

func TestDescriptionPreservesSemanticFields(t *testing.T) {
	t.Parallel()

	span := source.NewRange(20, 40)
	glyph := func(text string) ir.Node { return ir.NewGlyph(span, text) }

	fraction := ir.NewFraction(span, glyph("n"), glyph("k"))
	fraction.Role = ir.FractionRoleBinomial
	fraction.Style = ir.FractionNoBar
	fraction.Bar = false
	binom := Generate(fraction, ir.Inline, source.Limits{}).Description.Root
	if binom.FractionRole != display.AltFractionBinomial ||
		binom.FractionStyle != display.AltFractionNoBar {
		t.Fatalf("fraction semantics = %#v", binom)
	}
	if hasAltFlag(binom.Flags, display.AltFlagBar) {
		t.Fatalf("barless fraction has bar flag: %v", binom.Flags)
	}
	assertDescriptionPart(t, binom, display.AltPartNumerator, display.AltKindGlyph, "n")
	assertDescriptionPart(t, binom, display.AltPartDenominator, display.AltKindGlyph, "k")

	scripts := ir.NewScripts(span, ir.NewOperator(span, "sum"))
	scripts.Role = ir.ScriptRoleLimits
	scripts.Bottom = glyph("i")
	scripts.Top = glyph("n")
	limits := Generate(scripts, ir.Inline, source.Limits{}).Description.Root
	if limits.ScriptRole != display.AltScriptLimits {
		t.Fatalf("scripts role = %q", limits.ScriptRole)
	}
	assertDescriptionPart(t, limits, display.AltPartBase, display.AltKindOperator, "sum")
	assertDescriptionPart(t, limits, display.AltPartBottom, display.AltKindGlyph, "i")
	assertDescriptionPart(t, limits, display.AltPartTop, display.AltKindGlyph, "n")

	fence := validDescriptionFence(span, glyph)
	fenced := Generate(fence, ir.Inline, source.Limits{}).Description.Root
	assertDescriptionPart(t, fenced, display.AltPartOpen, display.AltKindGlyph, "(")
	assertDescriptionPart(t, fenced, display.AltPartContent, display.AltKindGlyph, "x")
	assertDescriptionPart(t, fenced, display.AltPartClose, display.AltKindGlyph, ")")
	if !hasAltFlag(fenced.Flags, display.AltFlagBalanced) {
		t.Fatalf("fence flags = %v, want balanced", fenced.Flags)
	}

	accent := ir.NewAccent(span, glyph("x"), glyph("^"), ir.PositionBelow)
	accent.Dotless = true
	accented := Generate(accent, ir.Inline, source.Limits{}).Description.Root
	if accented.Position != display.AltPositionBelow ||
		!hasAltFlag(accented.Flags, display.AltFlagDotless) {
		t.Fatalf("accent semantics = %#v", accented)
	}
	assertDescriptionPart(t, accented, display.AltPartBody, display.AltKindGlyph, "x")
	assertDescriptionPart(t, accented, display.AltPartAccent, display.AltKindGlyph, "^")

	cancel := ir.NewCancel(span, glyph("="))
	cancel.Role = ir.CancelRoleNegation
	cancel.Cross = true
	canceled := Generate(cancel, ir.Inline, source.Limits{}).Description.Root
	if canceled.CancelRole != display.AltCancelNegation ||
		!hasAltFlag(canceled.Flags, display.AltFlagCross) {
		t.Fatalf("cancel semantics = %#v", canceled)
	}
	slashedNode := ir.NewCancel(span, glyph("p"))
	slashedNode.Role = ir.CancelRoleSlashed
	slashed := Generate(slashedNode, ir.Inline, source.Limits{}).Description.Root
	if slashed.CancelRole != display.AltCancelSlashed {
		t.Fatalf("slashed semantics = %#v", slashed)
	}

	table := ir.NewTable(span,
		ir.TableRow{Cells: []ir.Node{glyph("x"), glyph("1")}},
		ir.TableRow{Cells: []ir.Node{glyph("y"), glyph("2")}},
	)
	table.Role = ir.TableRoleCases
	table.Columns = []ir.ColumnAlign{ir.AlignLeft, ir.AlignRight}
	tableDescription := Generate(table, ir.Inline, source.Limits{}).Description.Root
	if tableDescription.TableRole != display.AltTableCases ||
		len(tableDescription.Rows) != 2 ||
		len(tableDescription.Rows[0].Items) != 2 ||
		len(tableDescription.Columns) != 2 ||
		tableDescription.Columns[0].Align != display.AltColumnLeft ||
		tableDescription.Columns[1].Align != display.AltColumnRight {
		t.Fatalf("table semantics = %#v", tableDescription)
	}

	thinSpace := Generate(
		ir.NewSpace(span, ir.SpaceExplicit, ir.RawLength(ir.LengthMu, 3_000_000)),
		ir.Inline,
		source.Limits{},
	).Description.Root
	if thinSpace.Space != display.AltSpaceExplicit ||
		thinSpace.SpaceWidth != display.AltSpaceWidthThin {
		t.Fatalf("thin space semantics = %#v", thinSpace)
	}
	customSpace := Generate(
		ir.NewSpace(span, ir.SpaceExplicit, ir.RawLength(ir.LengthPt, 2_500_000)),
		ir.Inline,
		source.Limits{},
	).Description.Root
	if customSpace.SpaceWidth != display.AltSpaceWidthCustom {
		t.Fatalf("custom space semantics = %#v", customSpace)
	}
}

func TestDescriptionTagsAndRecoveryNodes(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	formula := &ir.Formula{
		Root: ir.NewRow(span,
			ir.NewGlyph(span, "x"),
			ir.NewPlaceholder(span, "missing argument"),
			ir.NewError(span, "bad", source.CodeUnknownCommand),
		),
		Tags: []ir.EquationTag{
			{
				Body:   ir.NewGlyph(span, "1"),
				Manual: true,
				Line:   ir.FormulaTagLine,
			},
			{
				Suppressed: true,
				Line:       ir.FormulaTagLine,
			},
		},
	}
	out := GenerateFormula(formula, ir.Display, source.Limits{})
	if out.Description == nil || len(out.Description.Tags) != 2 {
		t.Fatalf("Description = %#v", out.Description)
	}
	tag := out.Description.Tags[0]
	if !tag.Manual || tag.Line != ir.FormulaTagLine || tag.Body == nil ||
		tag.Body.Kind != display.AltKindGlyph || tag.Body.Text != "1" {
		t.Fatalf("manual tag = %#v", tag)
	}
	if !out.Description.Tags[1].Suppressed || out.Description.Tags[1].Body != nil {
		t.Fatalf("suppressed tag = %#v", out.Description.Tags[1])
	}
	placeholder := out.Description.Root.Parts[1].Node
	if placeholder.Kind != display.AltKindPlaceholder || placeholder.Text != "missing argument" {
		t.Fatalf("placeholder = %#v", placeholder)
	}
	errorNode := out.Description.Root.Parts[2].Node
	if errorNode.Kind != display.AltKindError ||
		errorNode.Code != string(source.CodeUnknownCommand) ||
		errorNode.Text != "bad" {
		t.Fatalf("error node = %#v", errorNode)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDescriptionTeXCompatibilityFields(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	fontSize, err := ir.PercentLength(248.8)
	if err != nil {
		t.Fatal(err)
	}
	style := ir.NewStyle(span, ir.NewGlyph(span, "x"))
	style.SetFontSize = true
	style.FontSize = fontSize

	styled, err := GenerateContext(context.Background(), style, ir.Inline, source.Limits{})
	if err != nil {
		t.Fatalf("GenerateContext(style) error = %v", err)
	}
	if styled.Description.Root.FontSize == nil ||
		styled.Description.Root.FontSize.Percent != 248.8 {
		t.Fatalf("style font size = %#v", styled.Description.Root.FontSize)
	}

	overlay := ir.NewOverlay(span, ir.NewGlyph(span, "x"), ir.OverlayEnd)
	overlaid, err := GenerateContext(context.Background(), overlay, ir.Inline, source.Limits{})
	if err != nil {
		t.Fatalf("GenerateContext(overlay) error = %v", err)
	}
	if overlaid.Description.Root.Kind != display.AltKindOverlay ||
		overlaid.Description.Root.OverlayAlignment != display.AltOverlayEnd {
		t.Fatalf("overlay description = %#v", overlaid.Description.Root)
	}
	assertDescriptionPart(t, overlaid.Description.Root, display.AltPartBody, display.AltKindGlyph, "x")
	if overlaid.Text != "x" || overlaid.Speech != "x" {
		t.Fatalf("overlay Alt = %#v, want visible body content", overlaid)
	}

	for _, tc := range []struct {
		amount ir.Length
		want   display.AltRaiseDirection
	}{
		{ir.RawLength(ir.LengthEm, 500_000), display.AltRaiseUp},
		{ir.RawLength(ir.LengthEm, -500_000), display.AltRaiseDown},
		{ir.RawLength(ir.LengthEm, 0), display.AltRaiseZero},
	} {
		raise := ir.NewRaise(span, ir.NewGlyph(span, "x"), tc.amount)
		raised, err := GenerateContext(context.Background(), raise, ir.Inline, source.Limits{})
		if err != nil {
			t.Fatalf("GenerateContext(raise) error = %v", err)
		}
		if raised.Description.Root.Kind != display.AltKindRaise ||
			raised.Description.Root.RaiseDirection != tc.want {
			t.Fatalf("raise description = %#v, want %q", raised.Description.Root, tc.want)
		}
		assertDescriptionPart(t, raised.Description.Root, display.AltPartBody, display.AltKindGlyph, "x")
		if raised.Text != "x" || raised.Speech != "x" {
			t.Fatalf("raise Alt = %#v, want visible body content", raised)
		}
	}
}

func TestDescriptionRowTags(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	formula := &ir.Formula{
		Root: ir.NewMultiline(span,
			[]ir.Node{ir.NewGlyph(span, "x")},
			[]ir.Node{ir.NewGlyph(span, "y")},
		),
		Tags: []ir.EquationTag{
			{
				Body:   ir.NewGlyph(span, "1"),
				Manual: true,
				Line:   0,
			},
			{
				Suppressed: true,
				Line:       1,
			},
		},
	}
	out, err := GenerateFormulaContext(context.Background(), formula, ir.Display, source.Limits{})
	if err != nil {
		t.Fatalf("GenerateFormulaContext() error = %v", err)
	}
	if out.Description == nil || len(out.Description.Tags) != 2 {
		t.Fatalf("Description = %#v", out.Description)
	}
	if out.Description.Tags[0].Line != 0 || out.Description.Tags[0].Body == nil ||
		out.Description.Tags[1].Line != 1 || !out.Description.Tags[1].Suppressed {
		t.Fatalf("row tags = %#v", out.Description.Tags)
	}
	canonical, err := out.Description.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical() error = %v", err)
	}
	if !strings.Contains(string(canonical), `"line":0`) ||
		!strings.Contains(string(canonical), `"line":1`) {
		t.Fatalf("canonical row tags = %s", canonical)
	}
}

func TestDescriptionIsSourceIndependentAndSerializable(t *testing.T) {
	t.Parallel()

	firstRoot := ir.NewRow(source.NewRange(0, 5),
		ir.NewGlyph(source.NewRange(0, 1), "x"),
		ir.NewFraction(source.NewRange(1, 5),
			ir.NewGlyph(source.NewRange(2, 3), "1"),
			ir.NewGlyph(source.NewRange(4, 5), "2"),
		),
	)
	firstRoot.NodeID = 71
	firstRoot.Paint = ir.Paint{Explicit: ir.RGBA{R: 255, A: 255}, HasExplicit: true}
	first := Generate(firstRoot, ir.Inline, source.Limits{})

	secondRoot := ir.NewRow(source.Detached(),
		ir.NewGlyph(source.Detached(), "x"),
		ir.NewFraction(source.Detached(),
			ir.NewGlyph(source.Detached(), "1"),
			ir.NewGlyph(source.Detached(), "2"),
		),
	)
	secondRoot.NodeID = 900
	secondRoot.Paint = ir.Paint{Role: ir.PaintPlaceholder}
	second := Generate(secondRoot, ir.Inline, source.Limits{})

	if !first.Equal(second) {
		t.Fatalf("source metadata changed description:\nfirst:  %#v\nsecond: %#v",
			first, second)
	}
	firstJSON, err := first.Description.MarshalCanonical()
	if err != nil {
		t.Fatalf("first MarshalCanonical() error = %v", err)
	}
	secondJSON, err := second.Description.MarshalCanonical()
	if err != nil {
		t.Fatalf("second MarshalCanonical() error = %v", err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("source metadata changed JSON:\n%s\n%s", firstJSON, secondJSON)
	}
	for _, excluded := range []string{`"source"`, `"span"`, `"nodeID"`, `"paint"`, `"font"`} {
		if strings.Contains(string(firstJSON), excluded) {
			t.Fatalf("description JSON leaked %s: %s", excluded, firstJSON)
		}
	}
}

func TestDescriptionLimitsCancellationAndFallback(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	root := ir.NewRow(span,
		ir.NewGlyph(span, "x"),
		ir.NewGlyph(span, "y"),
	)
	out, err := GenerateContext(
		context.Background(),
		root,
		ir.Inline,
		source.Limits{MaxNodes: 1},
	)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("GenerateContext() error = %v, want ErrLimitExceeded", err)
	}
	if out.Description == nil {
		t.Fatal("limit fallback Description = nil")
	}
	if out.Description.Root.Kind != display.AltKindError ||
		out.Description.Root.Code != string(source.CodeInternal) {
		t.Fatalf("limit fallback description = %#v", out.Description.Root)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("fallback Validate() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if description, err := buildDescription(ctx, &ir.Formula{Root: root}, source.Limits{}); description != nil ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("canceled buildDescription() = %#v, %v", description, err)
	}
}

func TestDescriptionBuilderRejectsCycle(t *testing.T) {
	t.Parallel()

	span := source.NewRange(0, 1)
	row := &ir.Row{Base: ir.NewBase(ir.KindRow, span)}
	row.Items = []ir.Node{row}
	if description, err := buildDescription(context.Background(),
		&ir.Formula{Root: row}, source.Limits{}); description != nil ||
		!errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("cyclic buildDescription() = %#v, %v", description, err)
	}
}

func validDescriptionFence(span source.Range, glyph func(string) ir.Node) *ir.Fenced {
	fence := ir.NewFenced(span, glyph("x"))
	fence.Open = glyph("(")
	fence.Close = glyph(")")
	fence.Balanced = true
	fence.Sizing = ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 1_500_000))
	return fence
}

func assertDescriptionPart(
	t *testing.T,
	node display.AltNode,
	role display.AltPartRole,
	kind display.AltNodeKind,
	text string,
) {
	t.Helper()
	for _, part := range node.Parts {
		if part.Role == role {
			if part.Node.Kind != kind || part.Node.Text != text {
				t.Fatalf("part %q = %#v, want kind %q text %q",
					role, part.Node, kind, text)
			}
			return
		}
	}
	t.Fatalf("part %q not found in %#v", role, node.Parts)
}

func hasAltFlag(flags []display.AltFlag, want display.AltFlag) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}

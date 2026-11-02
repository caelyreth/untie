package typst

import (
	"reflect"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func parse(t *testing.T, src string) Result {
	t.Helper()
	result := Parse(src, ir.Inline, Options{})
	if result.Root == nil {
		t.Fatalf("Parse(%q).Root = nil", src)
	}
	if err := ir.Validate(&ir.Formula{Root: result.Root}, source.Limits{}); err != nil {
		t.Fatalf("Parse(%q) produced invalid IR: %v", src, err)
	}
	return result
}

func requireNoDiagnostics(t *testing.T, src string, result Result) {
	t.Helper()
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Parse(%q) diagnostics = %#v", src, result.Diagnostics)
	}
}

func find[T ir.Node](t *testing.T, root ir.Node) T {
	t.Helper()
	var zero T
	var found T
	ir.Walk(root, func(n ir.Node) bool {
		if v, ok := n.(T); ok && reflect.ValueOf(found).IsNil() {
			found = v
		}
		return true
	})
	if reflect.ValueOf(found).IsNil() {
		t.Fatalf("did not find %T in %T", zero, root)
	}
	return found
}

func builtinPercentLength(t *testing.T, value float64) ir.Length {
	t.Helper()
	length, err := ir.PercentLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}

func TestCoreConstructs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want any
	}{
		{"glyph", "x", (*ir.Glyph)(nil)},
		{"plain subscript", "x_i", (*ir.Scripts)(nil)},
		{"script", "x^2_i", (*ir.Scripts)(nil)},
		{"fraction", "1/2", (*ir.Fraction)(nil)},
		{"sqrt", "sqrt(x)", (*ir.Radical)(nil)},
		{"root", "root(3, x)", (*ir.Radical)(nil)},
		{"fence", "lr([x])", (*ir.Fenced)(nil)},
		{"accent", "hat(x)", (*ir.Accent)(nil)},
		{"line", "overline(x)", (*ir.Line)(nil)},
		{"cancel", "cancel(x, cross: #true)", (*ir.Cancel)(nil)},
		{"matrix", "mat(1, 2; 3, 4)", (*ir.Table)(nil)},
		{"vector", "vec(1, 2, 3)", (*ir.Table)(nil)},
		{"cases", `cases(1 "if" x > 0, 0 "else")`, (*ir.Table)(nil)},
		{"text", "text(fill: #red, \"time\")", (*ir.Color)(nil)},
		{"style", "display(1/2)", (*ir.Style)(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parse(t, tt.src)
			requireNoDiagnostics(t, tt.src, result)
			switch tt.want.(type) {
			case *ir.Glyph:
				find[*ir.Glyph](t, result.Root)
			case *ir.Scripts:
				find[*ir.Scripts](t, result.Root)
			case *ir.Fraction:
				find[*ir.Fraction](t, result.Root)
			case *ir.Radical:
				find[*ir.Radical](t, result.Root)
			case *ir.Fenced:
				find[*ir.Fenced](t, result.Root)
			case *ir.Accent:
				find[*ir.Accent](t, result.Root)
			case *ir.Line:
				find[*ir.Line](t, result.Root)
			case *ir.Cancel:
				find[*ir.Cancel](t, result.Root)
			case *ir.Table:
				find[*ir.Table](t, result.Root)
			case *ir.Color:
				find[*ir.Color](t, result.Root)
			case *ir.Style:
				find[*ir.Style](t, result.Root)
			}
		})
	}
}

func TestAttachmentsMergeAndPreserveLimits(t *testing.T) {
	t.Parallel()

	result := parse(t, "limits(sum)_1^2 + x'^2")
	requireNoDiagnostics(t, "limits(sum)_1^2 + x'^2", result)
	var scripts []*ir.Scripts
	ir.Walk(result.Root, func(n ir.Node) bool {
		if s, ok := n.(*ir.Scripts); ok {
			scripts = append(scripts, s)
		}
		return true
	})
	if len(scripts) != 2 {
		t.Fatalf("found %d Scripts nodes, want 2", len(scripts))
	}
	if scripts[0].Limits != ir.LimitsAlways || scripts[0].Top == nil || scripts[0].Bottom == nil {
		t.Fatalf("limits attachment = %#v", scripts[0])
	}
	if scripts[1].TopRight == nil || scripts[1].Top == nil {
		t.Fatalf("prime/superscript attachment = %#v", scripts[1])
	}
}

func TestSemanticLoweringDefaults(t *testing.T) {
	t.Parallel()

	t.Run("relation attachments use limits", func(t *testing.T) {
		result := parse(t, `arrow.r^1_2`)
		requireNoDiagnostics(t, "relation attachment", result)
		scripts := find[*ir.Scripts](t, result.Root)
		if scripts.Limits != ir.LimitsAlways {
			t.Fatalf("relation limits = %v, want always", scripts.Limits)
		}
	})

	t.Run("directional fence classes", func(t *testing.T) {
		result := parse(t, `floor(x)`)
		requireNoDiagnostics(t, "floor", result)
		fenced := find[*ir.Fenced](t, result.Root)
		if fenced.Open.NodeBase().Class != ir.ClassOpening || fenced.Close.NodeBase().Class != ir.ClassClosing {
			t.Fatalf("floor classes = %v/%v", fenced.Open.NodeBase().Class, fenced.Close.NodeBase().Class)
		}

		result = parse(t, `norm(x)`)
		requireNoDiagnostics(t, "norm", result)
		fenced = find[*ir.Fenced](t, result.Root)
		if fenced.Open.NodeBase().Class != ir.ClassFence || fenced.Close.NodeBase().Class != ir.ClassFence {
			t.Fatalf("norm classes = %v/%v", fenced.Open.NodeBase().Class, fenced.Close.NodeBase().Class)
		}
	})

	t.Run("binom padding", func(t *testing.T) {
		result := parse(t, `binom(n, k)`)
		requireNoDiagnostics(t, "binom", result)
		fraction := find[*ir.Fraction](t, result.Root)
		if fraction.Padding != builtinEmLength(0.1) {
			t.Fatalf("binom padding = %v, want %v", fraction.Padding, builtinEmLength(0.1))
		}
	})

	t.Run("alignment marker class", func(t *testing.T) {
		result := parse(t, `a &= b \ c &= d`)
		requireNoDiagnostics(t, "alignment", result)
		marker := find[*ir.AlignMarker](t, result.Root)
		if marker.Class != ir.ClassSpecial {
			t.Fatalf("align marker class = %v, want special", marker.Class)
		}
	})
}

func TestLRPromotesBodyDelimiters(t *testing.T) {
	t.Parallel()

	result := parse(t, `lr(chevron.l a mid(bar.v) b chevron.r)`)
	requireNoDiagnostics(t, "lr delimiters", result)
	fenced := find[*ir.Fenced](t, result.Root)
	if nodeText(fenced.Open) != "⟨" || nodeText(fenced.Close) != "⟩" {
		t.Fatalf("lr delimiters = %q/%q", nodeText(fenced.Open), nodeText(fenced.Close))
	}
	if len(fenced.Body) != 3 {
		t.Fatalf("lr body items = %d, want 3", len(fenced.Body))
	}
}

func TestFunctionArgumentsAndEvaluator(t *testing.T) {
	t.Parallel()

	result := parse(t, `mat(
		delim: "[",
		augment: #1,
		row-gap: #1em,
		column-gap: #2em,
		1 & 2;
		3 & 4,
	)`)
	requireNoDiagnostics(t, "matrix arguments", result)
	table := find[*ir.Table](t, result.Root)
	if table.RowGap != builtinEmLength(1) || table.ColumnGap != builtinEmLength(2) {
		t.Fatalf("gaps = %v/%v", table.RowGap, table.ColumnGap)
	}
	if len(table.Vertical) != 1 || table.Vertical[0].Position != 1 {
		t.Fatalf("vertical augmentation = %#v", table.Vertical)
	}
	fenced := find[*ir.Fenced](t, result.Root)
	if got := nodeText(fenced.Open); got != "[" {
		t.Fatalf("open delimiter = %q", got)
	}

	dictAugment := parse(t, `mat(augment: #(vline: 1), 1, 2; 3, 4)`)
	requireNoDiagnostics(t, "dictionary augment", dictAugment)
	dictTable := find[*ir.Table](t, dictAugment.Root)
	if len(dictTable.Vertical) != 1 || dictTable.Vertical[0].Position != 1 {
		t.Fatalf("dictionary vertical augmentation = %#v", dictTable.Vertical)
	}
	hlineAugment := parse(t, `mat(augment: #(hline: (1, -1)), 1, 2; 3, 4; 5, 6)`)
	requireNoDiagnostics(t, "horizontal augmentation", hlineAugment)
	hlineTable := find[*ir.Table](t, hlineAugment.Root)
	if len(hlineTable.Horizontal) != 2 ||
		hlineTable.Horizontal[0].Position != 1 ||
		hlineTable.Horizontal[1].Position != 2 {
		t.Fatalf("horizontal augmentation = %#v", hlineTable.Horizontal)
	}

	empty := parse(t, `mat(1, , 3; , 5, )`)
	requireNoDiagnostics(t, "empty matrix cells", empty)
	emptyTable := find[*ir.Table](t, empty.Root)
	if len(emptyTable.Rows) != 2 || len(emptyTable.Rows[0].Cells) != 3 || len(emptyTable.Rows[1].Cells) != 3 {
		t.Fatalf("empty matrix shape = %#v", emptyTable.Rows)
	}

	cases := parse(t, `cases(reverse: #true, a, b)`)
	requireNoDiagnostics(t, "cases", cases)
	casesFence := find[*ir.Fenced](t, cases.Root)
	if nodeText(casesFence.Open) != "" || nodeText(casesFence.Close) != "}" {
		t.Fatalf("reversed cases delimiters = %q/%q", nodeText(casesFence.Open), nodeText(casesFence.Close))
	}

	caseTable := find[*ir.Table](t, cases.Root)
	if len(caseTable.Columns) != 1 {
		t.Fatalf("plain cases columns = %d, want 1", len(caseTable.Columns))
	}
	aligned := parse(t, `cases(x & "if" y, z & "else")`)
	requireNoDiagnostics(t, "aligned cases", aligned)
	alignedTable := find[*ir.Table](t, aligned.Root)
	if len(alignedTable.Columns) != 2 {
		t.Fatalf("aligned cases columns = %d, want 2", len(alignedTable.Columns))
	}
}

func TestEmptyPositionalArgumentsAreDiagnosed(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`frac(,1)`,
		`frac(1,,2)`,
		`sqrt(,)`,
		`root(,x)`,
		`attach(,t:2)`,
	} {
		result := parse(t, src)
		comma := strings.LastIndexByte(src, ',')
		if comma < 0 {
			t.Fatalf("%q has no comma", src)
		}
		want := source.NewRange(comma, comma+1)
		found := false
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code == source.CodeMissingArgument && diagnostic.Range == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%q diagnostics = %#v, want missing argument at comma %v", src, result.Diagnostics, want)
		}
	}
}

func TestMatrixLikeCallsAllowEmptyCells(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`mat(,1)`,
		`mat(1,,2;3,,4)`,
		`vec(,1)`,
		`cases(,x)`,
	} {
		result := parse(t, src)
		requireNoDiagnostics(t, src, result)
	}
}

func TestUnsupportedStatementsConsumeCompleteBoundary(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`#let x = 1; x`,
		`#let x = (1 + (2 - 3)); x`,
		`#let x = {1 + 2}; x`,
		`#set text(fill: red); x`,
	} {
		result := parse(t, src)
		var unsupported []source.Diagnostic
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code == source.CodeUnsupported {
				unsupported = append(unsupported, diagnostic)
			}
		}
		if len(unsupported) != 1 {
			t.Fatalf("%q unsupported diagnostics = %#v, want exactly one", src, unsupported)
		}
		wantEnd := strings.IndexByte(src, ';') + 1
		if got := unsupported[0].Range.Slice(src); len(got) == 0 || got[len(got)-1] != ';' {
			t.Fatalf("%q unsupported span = %q, want complete statement through semicolon", src, got)
		}
		if unsupported[0].Range.End != wantEnd {
			t.Fatalf("%q unsupported end = %d, want %d", src, unsupported[0].Range.End, wantEnd)
		}

		var visible strings.Builder
		ir.Walk(result.Root, func(node ir.Node) bool {
			if glyph, ok := node.(*ir.Glyph); ok {
				visible.WriteString(glyph.Text)
			}
			return true
		})
		if visible.String() != "x" {
			t.Fatalf("%q visible glyphs = %q, want only trailing x", src, visible.String())
		}
	}
}

func TestSyntaxGroupingRowsStayNeutral(t *testing.T) {
	t.Parallel()

	result := parse(t, `a+b^2`)
	requireNoDiagnostics(t, "neutral grouping", result)
	root, ok := result.Root.(*ir.Row)
	if !ok || root.Class != ir.ClassNormal {
		t.Fatalf("root = %#v, want a normal row", result.Root)
	}
	parenthesized := parse(t, `(a)`)
	requireNoDiagnostics(t, "literal delimiters", parenthesized)
	literal, ok := parenthesized.Root.(*ir.Row)
	if !ok || literal.Class != ir.ClassNormal {
		t.Fatalf("literal delimiter row = %#v, want normal", parenthesized.Root)
	}

	primes := parse(t, `f''`)
	requireNoDiagnostics(t, "grouped primes", primes)
	scripts := find[*ir.Scripts](t, primes.Root)
	prime, ok := scripts.TopRight.(*ir.Glyph)
	if !ok || prime.Text != "″" || prime.Class != ir.ClassNormal {
		t.Fatalf("grouped prime = %#v, want normal double prime", scripts.TopRight)
	}
}

func TestEvaluatorValuesSpreadAndHSpace(t *testing.T) {
	t.Parallel()

	result := parse(t, `frac(..#("a", "b"), style: "skewed") + h(#1.5em, weak: #true)`)
	requireNoDiagnostics(t, "spread and h", result)
	fraction := find[*ir.Fraction](t, result.Root)
	if fraction.Style != ir.FractionSkewed || fraction.Bar {
		t.Fatalf("fraction = %#v", fraction)
	}
	space := find[*ir.Space](t, result.Root)
	if space.Amount != builtinEmLength(1.5) || !space.Weak {
		t.Fatalf("space = %#v", space)
	}
	percent := parse(t, `lr([x], size: #150%)`)
	requireNoDiagnostics(t, "percentage size", percent)
	fenced := find[*ir.Fenced](t, percent.Root)
	if fenced.Sizing.Kind != ir.DelimiterSizingRelative || fenced.Sizing.Value.Float() != 150 {
		t.Fatalf("percentage sizing = %#v", fenced.Sizing)
	}

	text := parse(t, `text(#red, "time"^2)`)
	requireNoDiagnostics(t, "positional text fill", text)
	color := find[*ir.Color](t, text.Root)
	if !color.Paint.HasExplicit || color.Paint.Explicit.R != 0xff {
		t.Fatalf("text color = %#v", color.Paint)
	}
}

func TestLRSizeDiagnosticBoundary(t *testing.T) {
	t.Parallel()

	valid := []struct {
		source string
		want   ir.DelimiterSizing
	}{
		{source: `lr([x])`, want: ir.ContentDelimiterSizing()},
		{source: `lr([x], size: #100%)`, want: ir.ContentDelimiterSizing()},
		{source: `lr([x], size: #150%)`, want: ir.RelativeDelimiterSizing(builtinPercentLength(t, 150))},
		{source: `lr([x], size: #3em)`, want: ir.AbsoluteDelimiterSizing(builtinEmLength(3))},
	}
	for _, tt := range valid {
		tt := tt
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			result := parse(t, tt.source)
			requireNoDiagnostics(t, tt.source, result)
			fenced := find[*ir.Fenced](t, result.Root)
			if fenced.Sizing != tt.want {
				t.Fatalf("%s sizing = %#v, want %#v", tt.source, fenced.Sizing, tt.want)
			}
		})
	}

	invalid := []string{
		`lr([x], size: #1)`,
		`lr([x], size: #"150%")`,
		`lr([x], size: #auto)`,
		`lr([x], size: #none)`,
		`lr([x], size: #-1em)`,
	}
	for _, src := range invalid {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parse(t, src)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeInvalidValue ||
				result.Diagnostics[0].Severity != source.SeverityError {
				t.Fatalf("%s diagnostics = %#v, want invalid-value error", src, result.Diagnostics)
			}
			fenced := find[*ir.Fenced](t, result.Root)
			if fenced.Sizing != ir.ContentDelimiterSizing() {
				t.Fatalf("%s fallback sizing = %#v", src, fenced.Sizing)
			}
		})
	}
}

func TestLRSingleDelimiterUsesStandaloneSizing(t *testing.T) {
	t.Parallel()

	result := parse(t, `lr(|, size: #2em)`)
	requireNoDiagnostics(t, "single sized delimiter", result)
	node := find[*ir.SizedDelimiter](t, result.Root)
	if node.Text != "|" || node.Sizing != ir.AbsoluteDelimiterSizing(builtinEmLength(2)) ||
		node.Size != ir.SizeInherit {
		t.Fatalf("single sized delimiter = %#v", node)
	}

	defaultResult := parse(t, `lr(|)`)
	requireNoDiagnostics(t, "single content delimiter", defaultResult)
	if hasKind[*ir.SizedDelimiter](defaultResult.Root) {
		t.Fatal("content-sized single delimiter unexpectedly became an explicit SizedDelimiter")
	}
}

func hasKind[T ir.Node](root ir.Node) bool {
	found := false
	ir.Walk(root, func(node ir.Node) bool {
		if _, ok := node.(T); ok {
			found = true
			return false
		}
		return !found
	})
	return found
}

func TestNonFunctionContentCallPreservesArguments(t *testing.T) {
	t.Parallel()

	result := parse(t, `f(x, key: #1)`)
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeSyntax {
		t.Fatalf("content call diagnostics = %#v, want named-argument syntax error", result.Diagnostics)
	}
	var glyphs []string
	ir.Walk(result.Root, func(n ir.Node) bool {
		if glyph, ok := n.(*ir.Glyph); ok {
			glyphs = append(glyphs, glyph.Text)
		}
		return true
	})
	joined := strings.Join(glyphs, "")
	for _, want := range []string{"f", "x", "key", ":", "1", ","} {
		if !strings.Contains(joined, want) {
			t.Fatalf("content call glyphs %q lack %q", joined, want)
		}
	}
}

func TestNonFunctionContentCallPunctuationSpans(t *testing.T) {
	t.Parallel()

	src := `f(x,y)`
	result := parse(t, src)
	want := map[string]source.Range{
		"(": source.NewRange(1, 2),
		",": source.NewRange(3, 4),
		")": source.NewRange(5, 6),
	}
	found := make(map[string]source.Range)
	ir.Walk(result.Root, func(node ir.Node) bool {
		glyph, ok := node.(*ir.Glyph)
		if !ok {
			return true
		}
		if _, ok := want[glyph.Text]; ok {
			found[glyph.Text] = glyph.Span
		}
		return true
	})
	for text, span := range want {
		if found[text] != span {
			t.Fatalf("punctuation %q span = %v, want %v; all glyph spans = %#v",
				text, found[text], span, found)
		}
	}
}

func TestUnsupportedParametersAreDiagnosed(t *testing.T) {
	t.Parallel()

	result := Parse(`lr([x], size: #150%) + frac(a, b, unknown: 1)`, ir.Inline, Options{})
	if result.Root == nil {
		t.Fatal("nil root")
	}
	var unexpected bool
	for _, diagnostic := range result.Diagnostics {
		switch diagnostic.Code {
		case source.CodeSyntax:
			if strings.Contains(diagnostic.Message, "unexpected argument") {
				unexpected = true
			}
		}
	}
	if len(result.Diagnostics) != 1 || !unexpected {
		t.Fatalf("diagnostics = %#v, want only unexpected frac argument", result.Diagnostics)
	}
}

func TestShorthandsSymbolsAndMultiline(t *testing.T) {
	t.Parallel()

	result := parse(t, "a -> b <= c != d\nx &= 1 \\ &= 2")
	requireNoDiagnostics(t, "shorthands", result)
	var text []string
	ir.Walk(result.Root, func(n ir.Node) bool {
		if g, ok := n.(*ir.Glyph); ok {
			text = append(text, g.Text)
		}
		return true
	})
	joined := strings.Join(text, "")
	for _, want := range []string{"→", "≤", "≠", "="} {
		if !strings.Contains(joined, want) {
			t.Fatalf("glyph text %q lacks %q", joined, want)
		}
	}
	if _, ok := result.Root.(*ir.Multiline); !ok {
		t.Fatalf("root = %T, want *ir.Multiline", result.Root)
	}
}

func TestSpansAreUTF8ByteOffsets(t *testing.T) {
	t.Parallel()

	src := "α + sqrt(β)"
	result := parse(t, src)
	requireNoDiagnostics(t, src, result)
	var alpha, beta *ir.Glyph
	ir.Walk(result.Root, func(n ir.Node) bool {
		if g, ok := n.(*ir.Glyph); ok {
			switch g.Text {
			case "α":
				alpha = g
			case "β":
				beta = g
			}
		}
		return true
	})
	if alpha == nil || alpha.Span != source.NewRange(0, len("α")) {
		t.Fatalf("alpha span = %#v", alpha)
	}
	if beta == nil || string(beta.Span.Slice(src)) != "β" {
		t.Fatalf("beta span = %#v", beta)
	}
}

func TestUnknownAndUnsupportedDiagnosticsRecover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src  string
		code source.Code
	}{
		{"unknownThing + x", source.CodeUnknownVariable},
		{"notAFunction(x)", source.CodeUnknownVariable},
		{"#let x = 1", source.CodeUnsupported},
		{"sqrt(", source.CodeMissingArgument},
	}
	for _, tt := range tests {
		result := Parse(tt.src, ir.Inline, Options{})
		if result.Root == nil {
			t.Fatalf("%q: nil root", tt.src)
		}
		if err := ir.Validate(&ir.Formula{Root: result.Root}, source.Limits{}); err != nil {
			t.Fatalf("%q: invalid IR: %v", tt.src, err)
		}
		foundCode := false
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code == tt.code {
				foundCode = true
				if diagnostic.Range.Start < 0 || diagnostic.Range.End > len(tt.src) {
					t.Fatalf("%q: invalid diagnostic range: %+v", tt.src, diagnostic.Range)
				}
			}
		}
		if !foundCode {
			t.Fatalf("%q: diagnostics = %#v, want code %q", tt.src, result.Diagnostics, tt.code)
		}
	}
}

func TestLimitsAndDeterminism(t *testing.T) {
	t.Parallel()

	opts := Options{Limits: source.Limits{MaxNesting: 4, MaxNodes: 8, MaxCallDepth: 2, MaxTableCells: 2}}
	result := Parse("sqrt(sqrt(sqrt(x))) + mat(1,2,3)", ir.Inline, opts)
	if result.Root == nil {
		t.Fatal("limited parse returned nil root")
	}
	if err := ir.Validate(&ir.Formula{Root: result.Root}, opts.Limits); err != nil {
		t.Fatalf("limited parse produced invalid IR: %v", err)
	}
	var limited bool
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == source.CodeLimitExceeded {
			limited = true
		}
	}
	if !limited {
		t.Fatalf("diagnostics = %#v, want limit diagnostic", result.Diagnostics)
	}

	a := Parse("frac(a, b, style: \"skewed\") + mat(1,2;3,4)", ir.Display, Options{})
	b := Parse("frac(a, b, style: \"skewed\") + mat(1,2;3,4)", ir.Display, Options{})
	aj, err := ir.MarshalCanonical(&ir.Formula{Root: a.Root})
	if err != nil {
		t.Fatal(err)
	}
	bj, err := ir.MarshalCanonical(&ir.Formula{Root: b.Root})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(aj, bj) || !reflect.DeepEqual(a.Diagnostics, b.Diagnostics) {
		t.Fatal("repeated parse is not deterministic")
	}
}

func TestMalformedInputsNeverPanic(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"", "(", ")", "[", "{", "#", "#(", "sqrt(", "x^", "x_", "x/",
		`"unterminated`, "mat(,,,;,,,)", "accent(x)", "text()",
		"root(,)", "lr()", "cases(", "\xff", "/* unterminated",
	}
	for _, input := range inputs {
		result := Parse(input, ir.Inline, Options{})
		if result.Root == nil {
			t.Fatalf("Parse(%q).Root = nil", input)
		}
		if err := ir.Validate(&ir.Formula{Root: result.Root}, source.Limits{}); err != nil {
			t.Fatalf("Parse(%q) invalid IR: %v", input, err)
		}
	}
}

func TestStaticTypstFeatureCorpus(t *testing.T) {
	t.Parallel()

	// These examples are taken from the pinned Typst 0.15.1 math suite and
	// Typlet's statically generated feature corpus. They exercise syntax and
	// lowering without requiring either project at test time.
	examples := []string{
		`x + y <= z`,
		`x_i^2 + f''`,
		`1/2 + (a+b)/(c+d)`,
		`sqrt(x) + root(3, x) + √y + ∛z`,
		`sum_(i=0)^n i + integral_0^1 x dif x`,
		`lim_(x -> 0) f(x)`,
		`lr([x/y]) + lr(chevron.l a mid(bar.v) b chevron.r)`,
		`abs(x) + norm(y) + floor(z) + ceil(w) + round(v)`,
		`hat(x) + tilde(a) + macron(b) + dot(c) + dot.double(d) + arrow(e) + breve(f)`,
		`accent(x, arrow) + accent(y, "→")`,
		`overline(x) + underline(y)`,
		`cancel(x) + cancel(y, cross: #true)`,
		`text(fill: #red, "time"^2) + text(#blue, "place")`,
		`text(fill: #red)[word] + #[plain text]`,
		`bold(A) + italic(Gamma) + upright(alpha) + bb(R) + cal(A) + scr(B) + frak(g) + sans(x) + mono(y) + serif(z)`,
		`display(1/2) + inline(sum_i x_i) + script(x^2) + sscript(y_1)`,
		`class("relation", +) + class("large", x)`,
		`op("rank", limits: #true) + attach(Pi, t: alpha, b: beta, tl: 1, tr: 2)`,
		`scripts(sum)_1^2 + limits(A)_1^2`,
		`frac(a, b, style: "skewed") + binom(n, k)`,
		`#math.frac($a$, $b$) + #sym.arrow.r`,
		`mat(delim: "[", augment: #1, 1, 2; 3, 4)`,
		`vec(1, 2, 3) + cases(reverse: #true, a, b)`,
		`overbrace(a+b, n) + underbrace(c, m) + overparen(d) + underbracket(e, k)`,
		`h(#1em, weak: #true) + a thin b med c thick d quad e wide f`,
	}
	for _, src := range examples {
		result := Parse(src, ir.Inline, Options{})
		if err := ir.Validate(&ir.Formula{Root: result.Root}, source.Limits{}); err != nil {
			t.Errorf("Parse(%q) invalid IR: %v", src, err)
		}
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Severity == source.SeverityError {
				t.Errorf("Parse(%q): %s", src, diagnostic.Error())
			}
		}
	}
}

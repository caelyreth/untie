package tex

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSizedDelimiterCommandsLowerAllClassesAndSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command string
		class   ir.MathClass
		sizing  ir.DelimiterSizing
	}{
		{`\big`, ir.ClassNormal, texDelimiterTier(1)},
		{`\Big`, ir.ClassNormal, texDelimiterTier(2)},
		{`\bigg`, ir.ClassNormal, texDelimiterTier(3)},
		{`\Bigg`, ir.ClassNormal, texDelimiterTier(4)},
		{`\bigl`, ir.ClassOpening, texDelimiterTier(1)},
		{`\Bigl`, ir.ClassOpening, texDelimiterTier(2)},
		{`\biggl`, ir.ClassOpening, texDelimiterTier(3)},
		{`\Biggl`, ir.ClassOpening, texDelimiterTier(4)},
		{`\bigr`, ir.ClassClosing, texDelimiterTier(1)},
		{`\Bigr`, ir.ClassClosing, texDelimiterTier(2)},
		{`\biggr`, ir.ClassClosing, texDelimiterTier(3)},
		{`\Biggr`, ir.ClassClosing, texDelimiterTier(4)},
		{`\bigm`, ir.ClassRelation, texDelimiterTier(1)},
		{`\Bigm`, ir.ClassRelation, texDelimiterTier(2)},
		{`\biggm`, ir.ClassRelation, texDelimiterTier(3)},
		{`\Biggm`, ir.ClassRelation, texDelimiterTier(4)},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			src := tt.command + "("
			result := parseTest(t, src, Options{})
			requireNoDiagnostics(t, result)
			node := requireKind[*ir.SizedDelimiter](t, rowItems(t, result.Root)[0])
			if node.Text != "(" || node.Class != tt.class || node.Sizing != tt.sizing {
				t.Fatalf("%s = text %q class %v sizing %#v", src, node.Text, node.Class, node.Sizing)
			}
			if node.Size != ir.SizeText {
				t.Fatalf("%s base size = %v, want text style", src, node.Size)
			}
			wantSpan := source.NewRange(0, len(src))
			if node.Span != wantSpan || node.Origin != wantSpan {
				t.Fatalf("%s span/origin = %v/%v, want %v", src, node.Span, node.Origin, wantSpan)
			}
		})
	}
}

func TestSizedDelimiterAcceptedDelimiterTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		text   string
	}{
		{"(", "("}, {`\lparen`, "("}, {")", ")"}, {`\rparen`, ")"},
		{"[", "["}, {`\lbrack`, "["}, {"]", "]"}, {`\rbrack`, "]"},
		{`\{`, "{"}, {`\lbrace`, "{"}, {`\}`, "}"}, {`\rbrace`, "}"},
		{`\lfloor`, "⌊"}, {`\rfloor`, "⌋"}, {"⌊", "⌊"}, {"⌋", "⌋"},
		{`\lceil`, "⌈"}, {`\rceil`, "⌉"}, {"⌈", "⌈"}, {"⌉", "⌉"},
		{"<", "⟨"}, {`\langle`, "⟨"}, {"⟨", "⟨"},
		{">", "⟩"}, {`\rangle`, "⟩"}, {"⟩", "⟩"},
		{`\lt`, "⟨"}, {`\gt`, "⟩"},
		{`\lvert`, "|"}, {`\rvert`, "|"}, {`\lVert`, "‖"}, {`\rVert`, "‖"},
		{`\lgroup`, "⟮"}, {`\rgroup`, "⟯"}, {"⟮", "⟮"}, {"⟯", "⟯"},
		{`\lmoustache`, "⎰"}, {`\rmoustache`, "⎱"}, {"⎰", "⎰"}, {"⎱", "⎱"},
		{"/", "/"}, {`\backslash`, `\`},
		{"|", "|"}, {`\vert`, "|"}, {`\|`, "‖"}, {`\Vert`, "‖"},
		{`\uparrow`, "↑"}, {`\Uparrow`, "⇑"},
		{`\downarrow`, "↓"}, {`\Downarrow`, "⇓"},
		{`\updownarrow`, "↕"}, {`\Updownarrow`, "⇕"},
		{".", "."},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			src := `\big` + tt.source
			result := parseTest(t, src, Options{})
			requireNoDiagnostics(t, result)
			node := requireKind[*ir.SizedDelimiter](t, rowItems(t, result.Root)[0])
			if node.Text != tt.text || node.Class != ir.ClassNormal || node.Sizing != texDelimiterTier(1) {
				t.Fatalf("%s = text %q class %v sizing %#v", src, node.Text, node.Class, node.Sizing)
			}
			if node.Size != ir.SizeText {
				t.Fatalf("%s base size = %v, want text style", src, node.Size)
			}
		})
	}
}

func TestSizedDelimiterInvalidArgumentsRemainDiagnostics(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`\bigl 1`,
		`\bigr=`,
		`\Big\alpha`,
		`\big \VERT`,
	} {
		src := src
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, src, Options{})
			if !hasDiagnostic(result.Diagnostics, source.CodeInvalidValue) {
				t.Fatalf("%s diagnostics = %v, want invalid-value", src, diagnosticCodes(result.Diagnostics))
			}
			if hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
				t.Fatalf("%s diagnostics = %v, must not remain unsupported", src, diagnosticCodes(result.Diagnostics))
			}
			if _, ok := result.Root.(*ir.Row).Items[0].(*ir.ErrorNode); !ok {
				t.Fatalf("%s root item = %T, want visible ErrorNode", src, result.Root.(*ir.Row).Items[0])
			}
		})
	}

	for _, src := range []string{`\bigg`, `{\bigl}`} {
		result := parseTest(t, src, Options{})
		if !hasDiagnostic(result.Diagnostics, source.CodeMissingArgument) {
			t.Fatalf("%s diagnostics = %v, want missing-argument", src, diagnosticCodes(result.Diagnostics))
		}
		if hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
			t.Fatalf("%s diagnostics = %v, must not remain unsupported", src, diagnosticCodes(result.Diagnostics))
		}
	}
}

func TestSizedDelimiterGroupedArgumentsDoNotLeak(t *testing.T) {
	t.Parallel()

	result := parseTest(t, `\bigl{a+b}x`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeInvalidValue) {
		t.Fatalf("diagnostics = %v, want invalid-value", diagnosticCodes(result.Diagnostics))
	}
	if hasDiagnostic(result.Diagnostics, source.CodeUnexpectedToken) ||
		hasDiagnostic(result.Diagnostics, source.CodeUnclosedGroup) {
		t.Fatalf("diagnostics = %v, invalid delimiter group leaked tokens", diagnosticCodes(result.Diagnostics))
	}
	var texts []string
	ir.Walk(result.Root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok {
			texts = append(texts, glyph.Text)
		}
		return true
	})
	if strings.Join(texts, "") != "x" {
		t.Fatalf("grouped invalid argument leaked content: glyphs = %v", texts)
	}

	scoped := parseTest(t, `\bigl{\def\foo{x}}\foo`, Options{})
	if !hasDiagnostic(scoped.Diagnostics, source.CodeInvalidValue) {
		t.Fatalf("scoped diagnostics = %v, want invalid-value", diagnosticCodes(scoped.Diagnostics))
	}
	if !hasDiagnostic(scoped.Diagnostics, source.CodeUnknownCommand) {
		t.Fatalf("definition escaped invalid delimiter group: diagnostics = %v", diagnosticCodes(scoped.Diagnostics))
	}

	function := parseTest(t, `\bigl\frac{a}{b}x`, Options{})
	if !hasDiagnostic(function.Diagnostics, source.CodeInvalidValue) {
		t.Fatalf("function diagnostics = %v, want invalid-value", diagnosticCodes(function.Diagnostics))
	}
	var functionGlyphs []string
	ir.Walk(function.Root, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok {
			functionGlyphs = append(functionGlyphs, glyph.Text)
		}
		return true
	})
	if strings.Join(functionGlyphs, "") != "x" {
		t.Fatalf("function argument leaked after invalid delimiter: glyphs = %v", functionGlyphs)
	}
}

func TestSizedDelimiterMacroSpansUseContiguousCallSiteRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source string
		start  string
		end    string
	}{
		{source: `\def\foo{(} \bigl\foo x`, start: `\bigl`, end: `\foo`},
		{source: `\def\foo#1{\bigl#1}\foo(`, start: `\foo(`, end: `\foo(`},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			result := parseTest(t, tt.source, Options{})
			var sized *ir.SizedDelimiter
			ir.Walk(result.Root, func(node ir.Node) bool {
				if candidate, ok := node.(*ir.SizedDelimiter); ok && sized == nil {
					sized = candidate
				}
				return sized == nil
			})
			if sized == nil {
				t.Fatalf("no sized delimiter in %#v", result.Root)
			}
			start := strings.LastIndex(tt.source, tt.start)
			end := strings.LastIndex(tt.source, tt.end) + len(tt.end)
			want := source.NewRange(start, end)
			if sized.Span != want || sized.Origin != want {
				t.Fatalf("span/origin = %v/%v, want call-site %v", sized.Span, sized.Origin, want)
			}
		})
	}

	grouped := parseTest(t, `\def\foo#1{\bigl#1}\foo{+}`, Options{})
	if !hasDiagnostic(grouped.Diagnostics, source.CodeInvalidValue) {
		t.Fatalf("grouped macro diagnostics = %v, want invalid-value", diagnosticCodes(grouped.Diagnostics))
	}
}

func requireNoDiagnostics(t *testing.T, result Result) {
	t.Helper()
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

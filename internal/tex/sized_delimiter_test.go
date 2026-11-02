package tex

import (
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestSizedDelimiterCommandsLowerAllClassesAndSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command string
		class   ir.MathClass
		size    ir.DelimiterSize
	}{
		{`\big`, ir.ClassNormal, ir.DelimiterSize1},
		{`\Big`, ir.ClassNormal, ir.DelimiterSize2},
		{`\bigg`, ir.ClassNormal, ir.DelimiterSize3},
		{`\Bigg`, ir.ClassNormal, ir.DelimiterSize4},
		{`\bigl`, ir.ClassOpening, ir.DelimiterSize1},
		{`\Bigl`, ir.ClassOpening, ir.DelimiterSize2},
		{`\biggl`, ir.ClassOpening, ir.DelimiterSize3},
		{`\Biggl`, ir.ClassOpening, ir.DelimiterSize4},
		{`\bigr`, ir.ClassClosing, ir.DelimiterSize1},
		{`\Bigr`, ir.ClassClosing, ir.DelimiterSize2},
		{`\biggr`, ir.ClassClosing, ir.DelimiterSize3},
		{`\Biggr`, ir.ClassClosing, ir.DelimiterSize4},
		{`\bigm`, ir.ClassRelation, ir.DelimiterSize1},
		{`\Bigm`, ir.ClassRelation, ir.DelimiterSize2},
		{`\biggm`, ir.ClassRelation, ir.DelimiterSize3},
		{`\Biggm`, ir.ClassRelation, ir.DelimiterSize4},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			src := tt.command + "("
			result := parseTest(t, src, Options{})
			requireNoDiagnostics(t, result)
			node := requireKind[*ir.SizedDelimiter](t, rowItems(t, result.Root)[0])
			if node.Text != "(" || node.Class != tt.class || node.Size != tt.size {
				t.Fatalf("%s = text %q class %v size %d", src, node.Text, node.Class, node.Size)
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
			if node.Text != tt.text || node.Class != ir.ClassNormal || node.Size != ir.DelimiterSize1 {
				t.Fatalf("%s = text %q class %v size %d", src, node.Text, node.Class, node.Size)
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

func requireNoDiagnostics(t *testing.T, result Result) {
	t.Helper()
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

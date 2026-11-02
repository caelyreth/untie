package tex

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTextModeCommandsTerminate(t *testing.T) {
	sources := []string{
		`\text{\'{a}}`,
		`\text{\(\frac a b\)}`,
		`\text{\"{a}}`,
		`\text{\.{a}}`,
		`\text{\={a}}`,
		`\text{\~{a}}`,
		`\text{\^{a}}`,
		`\text{\textdollar}`,
		"\\text{\\`{a}}",
		`\text{\AA}`,
		`\text{\aa}`,
		`\text{\AE}`,
		`\text{\ae}`,
		`\text{\H{a}}`,
		`\text{\i}`,
		`\text{\j}`,
		`\text{\O}`,
		`\text{\o}`,
		`\text{\OE}`,
		`\text{\oe}`,
		`\text{\P}`,
		`\text{\r{a}}`,
		`\text{\S}`,
		`\text{\sect}`,
		`\text{\ss}`,
		`\text{\textgreater}`,
		`\text{\textless}`,
		`\text{\textquotedblleft}`,
		`\text{\textquotedblright}`,
		`\text{\textquoteleft}`,
		`\text{\textquoteright}`,
		`\text{\textregistered}`,
		`\text{\textsterling}`,
		`\text{\textunderscore}`,
		`\text{\u{a}}`,
		`\text{\v{a}}`,
		`\text{\sout{abc}}`,
		`\text{before $x^2$ after}`,
	}
	for _, src := range sources {
		src := src
		t.Run(src, func(t *testing.T) {
			result := parseWithin(t, src, 2*time.Second)
			if err := ir.Validate(&ir.Formula{Root: result.Root}, source.DefaultLimits()); err != nil {
				t.Fatalf("invalid IR: %v", err)
			}
		})
	}
}

func TestTextModeIterationLimitTerminates(t *testing.T) {
	limits := source.DefaultLimits()
	limits.MaxTokens = 32
	result := parseWithinOptions(t, `\text{`+strings.Repeat(`\AA `, 100)+`}`, 2*time.Second, Options{Limits: limits})
	if !hasDiagnostic(result.Diagnostics, source.CodeLimitExceeded) {
		t.Fatalf("diagnostics = %v, want limit-exceeded", diagnosticCodes(result.Diagnostics))
	}
}

func TestSharedTeXMetadataResolvesCommonSymbols(t *testing.T) {
	result := parseTest(t, `\approxeq \yen \Finv \vDash \And`, Options{})
	var glyphs int
	ir.Walk(result.Root, func(node ir.Node) bool {
		if _, ok := node.(*ir.Glyph); ok {
			glyphs++
		}
		return true
	})
	if glyphs != 5 {
		t.Fatalf("glyphs = %d, want 5", glyphs)
	}
}

func TestEnvironmentFamiliesLowerToTables(t *testing.T) {
	tests := []struct {
		source      string
		kind        ir.NodeKind
		rows        int
		columnCount int
	}{
		{`\begin{align*}a&=b\\c&=d\end{align*}`, ir.KindTable, 2, 2},
		{`\begin{alignat*}{2}a&=b&c&=d\end{alignat*}`, ir.KindTable, 1, 4},
		{`\begin{gather*}a=b\\c=d\end{gather*}`, ir.KindTable, 2, 1},
		{`\begin{equation*}a=b+c\end{equation*}`, ir.KindTable, 1, 1},
		{`\begin{Bmatrix*}[r]0&-1\\-1&0\end{Bmatrix*}`, ir.KindFenced, 0, 0},
		{`\begin{darray}{cc}a&b\\c&d\end{darray}`, ir.KindTable, 2, 2},
		{`\begin{dcases}a&b\\c&d\end{dcases}`, ir.KindFenced, 0, 0},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			result := parseTest(t, tc.source, Options{})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics: %v", result.Diagnostics)
			}
			node := rowItems(t, result.Root)[0]
			if node.NodeBase().Kind() != tc.kind {
				t.Fatalf("kind = %v, want %v", node.NodeBase().Kind(), tc.kind)
			}
			if tc.kind != ir.KindTable {
				return
			}
			table := requireKind[*ir.Table](t, node)
			if len(table.Rows) != tc.rows {
				t.Fatalf("rows = %d, want %d", len(table.Rows), tc.rows)
			}
			maxColumns := len(table.Columns)
			for _, row := range table.Rows {
				maxColumns = max(maxColumns, len(row.Cells))
			}
			if maxColumns != tc.columnCount {
				t.Fatalf("columns = %d, want %d", maxColumns, tc.columnCount)
			}
		})
	}

	result := parseTest(t, `\begin{CD}A@>f>>B\end{CD}`, Options{})
	if !hasDiagnostic(result.Diagnostics, source.CodeUnsupported) {
		t.Fatalf("CD diagnostics = %v, want unsupported", diagnosticCodes(result.Diagnostics))
	}
}

func TestTableRowBreaksAndRules(t *testing.T) {
	result := parseTest(t, `\begin{array}{c:c}a&b\cr\hline c&d\end{array}`, Options{})
	table := requireKind[*ir.Table](t, rowItems(t, result.Root)[0])
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if len(table.Vertical) != 1 || !table.Vertical[0].Dashed {
		t.Fatalf("vertical rules = %+v, want one dashed rule", table.Vertical)
	}
	if len(table.Horizontal) != 1 {
		t.Fatalf("horizontal rules = %+v, want one", table.Horizontal)
	}

	result = parseTest(t, `\begin{matrix}a\newline b\end{matrix}`, Options{})
	table = requireKind[*ir.Table](t, rowItems(t, result.Root)[0])
	if len(table.Rows) != 2 {
		t.Fatalf("newline rows = %d, want 2", len(table.Rows))
	}
}

func TestColorBoxPhantomRuleAndClassCommands(t *testing.T) {
	tests := []struct {
		source string
		kind   ir.NodeKind
	}{
		{`\textcolor{red}{x}`, ir.KindColor},
		{`\colorbox{aqua}{$F=ma$}`, ir.KindBox},
		{`\fcolorbox{red}{aqua}{x}`, ir.KindBox},
		{`\boxed{x}`, ir.KindBox},
		{`\phantom{x}`, ir.KindPhantom},
		{`\hphantom{x}`, ir.KindPhantom},
		{`\vphantom{x}`, ir.KindPhantom},
		{`\rule{1em}{0.5ex}`, ir.KindBox},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			result := parseTest(t, tc.source, Options{})
			if len(result.Diagnostics) != 0 {
				t.Fatalf("diagnostics: %v", result.Diagnostics)
			}
			if node := rowItems(t, result.Root)[0]; node.NodeBase().Kind() != tc.kind {
				t.Fatalf("kind = %v, want %v", node.NodeBase().Kind(), tc.kind)
			}
		})
	}

	result := parseTest(t, `\mathrel{=}`, Options{})
	if class := rowItems(t, result.Root)[0].NodeBase().Class; class != ir.ClassRelation {
		t.Fatalf("mathrel class = %v, want relation", class)
	}

	result = parseTest(t, `\genfrac{}{}{0pt}{0}{a}{b}`, Options{})
	fraction := requireKind[*ir.Fraction](t, rowItems(t, result.Root)[0])
	if fraction.Bar {
		t.Fatal("zero-width genfrac rule should produce a barless fraction")
	}
}

func TestConformanceDiagnosticRegressions(t *testing.T) {
	tests := []struct {
		source string
		code   source.Code
	}{
		{`1\over2\over3`, source.CodeUnexpectedToken},
		{`\limits\omega`, source.CodeUnexpectedToken},
		{`1^\sqrt{2}`, source.CodeMissingArgument},
		{`\textcolor#ffffff{text}`, source.CodeInvalidValue},
		{`\rule{1au}{2em}`, source.CodeInvalidValue},
		{"A\u0328", source.CodeUnsupported},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.source, func(t *testing.T) {
			result := parseTest(t, tc.source, Options{})
			if !hasDiagnostic(result.Diagnostics, tc.code) {
				t.Fatalf("diagnostics = %v, want %s", diagnosticCodes(result.Diagnostics), tc.code)
			}
		})
	}
}

func TestExpandingDefinitionsFreezeReplacementText(t *testing.T) {
	for _, command := range []string{"edef", "xdef"} {
		source := `\def\foo{a}\` + command + `\fcopy{\foo}\def\foo{}\fcopy`
		result := parseTest(t, source, Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("%s diagnostics: %v", command, result.Diagnostics)
		}
		if got := collectGlyphText(result.Root); got != "a" {
			t.Fatalf("%s expansion = %q, want a", command, got)
		}
	}
}

func TestCommonConformanceCommandFamilies(t *testing.T) {
	tests := []string{
		`\bmod b`,
		`x\pmod n`,
		`x\pod n`,
		`\bra{\psi}`,
		`\ket{\psi}`,
		`\braket{\phi|\psi}`,
		`\set{x|x<5}`,
		`\xrightarrow[below]{above}`,
		`\overbrace{x+1}^{n}`,
		`\underbrace{x+1}_{n}`,
		`\genfrac(){2pt}{0}a{b}`,
		`\char"263a`,
		`\verb!\frac a b!`,
		`a\newline b`,
	}
	for _, src := range tests {
		src := src
		t.Run(src, func(t *testing.T) {
			result := parseTest(t, src, Options{})
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == source.SeverityError {
					t.Fatalf("diagnostics: %v", result.Diagnostics)
				}
			}
		})
	}
}

func parseWithin(t *testing.T, src string, timeout time.Duration) Result {
	t.Helper()
	return parseWithinOptions(t, src, timeout, Options{})
}

func parseWithinOptions(t *testing.T, src string, timeout time.Duration, options Options) Result {
	t.Helper()
	type response struct {
		result Result
	}
	done := make(chan response, 1)
	go func() {
		done <- response{result: Parse(src, ir.Display, options)}
	}()
	select {
	case got := <-done:
		return got.result
	case <-time.After(timeout):
		t.Fatalf("Parse(%q) did not terminate within %s", src, timeout)
		return Result{}
	}
}

func collectGlyphText(node ir.Node) string {
	var builder strings.Builder
	ir.Walk(node, func(node ir.Node) bool {
		if glyph, ok := node.(*ir.Glyph); ok {
			builder.WriteString(glyph.Text)
		}
		return true
	})
	return builder.String()
}

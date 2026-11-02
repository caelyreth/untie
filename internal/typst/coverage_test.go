package typst

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
)

func TestCatalogueSymbolResolution(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"nabla":            "∇",
		"partial":          "∂",
		"planck":           "ħ",
		"ell":              "ℓ",
		"Re":               "ℜ",
		"pee":              "℘",
		"dots.c":           "⋯",
		"integral.surf":    "∯",
		"arrow.r.hook":     "↪︎",
		"arrow.r.squiggly": "⇝",
		"supset.neq":       "⊋",
		"without":          "∖",
		"inter":            "∩",
		"forall":           "∀",
		"exists":           "∃",
		"not":              "¬",
		"tack.r":           "⊢",
		"equiv.not":        "≢",
		"lt.eq.not":        "≰",
		"subset.eq.not":    "⊈",
		"angle":            "∠",
		"ast.op":           "∗",
		"complement":       "∁",
		"dot.op":           "⋅",
		"eq.not":           "≠",
		"lt.eq":            "≤",
		"RR":               "ℝ",
		"NN":               "ℕ",
		"ZZ":               "ℤ",
	}
	for source, want := range tests {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			result := parse(t, source)
			requireNoDiagnostics(t, source, result)
			if got := nodeText(result.Root); got != want {
				t.Fatalf("node text = %q, want %q", got, want)
			}
		})
	}
}

func TestSymbolAliasesKeepTypstValues(t *testing.T) {
	t.Parallel()

	result := parse(t, "epsilon epsilon.alt phi phi.alt")
	requireNoDiagnostics(t, "Greek aliases", result)
	if got := nodeText(result.Root); got != "εϵφϕ" {
		t.Fatalf("Greek aliases = %q", got)
	}
}

func TestCallableSymbolsAndContentFunctions(t *testing.T) {
	t.Parallel()

	triple := parse(t, "dot.triple(x)")
	requireNoDiagnostics(t, "triple dot", triple)
	accent := find[*ir.Accent](t, triple.Root)
	if got := nodeText(accent.Accent); got != "⃛" || accent.Accent.NodeBase().Class != ir.ClassDiacritic {
		t.Fatalf("triple dot accent = %q class %v", got, accent.Accent.NodeBase().Class)
	}

	callable := parse(t, "Psi(bold(r), t)")
	requireNoDiagnostics(t, "callable symbol", callable)
	for _, want := range []string{"Ψ", "(", "r", ",", "t", ")"} {
		if !strings.Contains(nodeText(callable.Root), want) {
			t.Errorf("callable symbol text %q lacks %q", nodeText(callable.Root), want)
		}
	}

	hidden := parse(t, "a + #hide[$x y z$] + b")
	requireNoDiagnostics(t, "hide content", hidden)
	find[*ir.Phantom](t, hidden.Root)

	strong := parse(t, `x "and" #strong[bold]`)
	requireNoDiagnostics(t, "strong content", strong)
	var bold ir.Node
	ir.Walk(strong.Root, func(node ir.Node) bool {
		if text, ok := node.(*ir.Text); ok && text.Variant == ir.VariantBold {
			bold = text
		}
		return true
	})
	if bold == nil || nodeText(bold) != "bold" {
		t.Fatalf("strong text = %#v", bold)
	}

	link := parse(t, `#link("https://typst.app", $x^2$)`)
	find[*ir.Scripts](t, link.Root)
	if len(link.Diagnostics) != 1 || string(link.Diagnostics[0].Code) != "unsupported" {
		t.Fatalf("link diagnostics = %#v", link.Diagnostics)
	}
}

func TestLengthsBoxesAndCodeValueDisplay(t *testing.T) {
	t.Parallel()

	spacing := parse(t, "a #h(1em) b #h(-1em/6) c")
	requireNoDiagnostics(t, "h space", spacing)
	if spaces := countNodes[*ir.Space](spacing.Root); spaces != 2 {
		t.Fatalf("space count = %d, want 2", spaces)
	}

	boxed := parse(t, `#box(stroke: 0.5pt, inset: 3pt, $E=m c^2$)`)
	requireNoDiagnostics(t, "box stroke", boxed)
	box := find[*ir.Box](t, boxed.Root)
	if !box.HasBorder || box.BorderWidth.Float() <= 0 {
		t.Fatalf("box border = %#v", box)
	}
	if box.Padding[0].Float() <= 0 {
		t.Fatalf("box padding = %#v", box.Padding)
	}

	for _, source := range []string{"#1em", "#(1, 2)", "#red"} {
		result := parse(t, source)
		requireNoDiagnostics(t, source, result)
		if nodeText(result.Root) == "" {
			t.Fatalf("%s lowered to no visible representation", source)
		}
	}

	color := parse(t, "#red")
	if got := nodeText(color.Root); got != `rgb("#ff0000")` {
		t.Fatalf("code color = %q", got)
	}
	plain := Parse("red", ir.Inline, Options{})
	if len(plain.Diagnostics) != 1 || string(plain.Diagnostics[0].Code) != "unknown-variable" {
		t.Fatalf("plain red diagnostics = %#v", plain.Diagnostics)
	}

	sided := parse(t, `#box(inset: (x: 2pt, y: 1pt), stroke: (paint: red, thickness: 0.5pt), $x$)`)
	requireNoDiagnostics(t, "side and stroke dictionaries", sided)
	sidedBox := find[*ir.Box](t, sided.Root)
	if sidedBox.Padding[0] != sidedBox.Padding[2] || sidedBox.Padding[0] == sidedBox.Padding[1] {
		t.Fatalf("side padding = %#v", sidedBox.Padding)
	}
}

func TestNullableAndOneSidedMatrixDelimiters(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`mat(delim: "]", 1; 2)`,
		`mat(delim: #(none, "]"), 1; 2)`,
		`mat(delim: #(none, none), 1; 2)`,
	} {
		result := parse(t, source)
		requireNoDiagnostics(t, source, result)
	}

	result := parse(t, `lr(zws (dif f)/(dif x) |)_(x=0)`)
	requireNoDiagnostics(t, "null delimiter", result)
	fenced := find[*ir.Fenced](t, result.Root)
	if _, ok := fenced.Open.(*ir.Row); !ok {
		t.Fatalf("open delimiter = %T, want empty row", fenced.Open)
	}
}

func TestCombiningMarksBecomeAccents(t *testing.T) {
	t.Parallel()

	result := parse(t, "x̂ + é + abć")
	requireNoDiagnostics(t, "combining marks", result)
	if accents := countNodes[*ir.Accent](result.Root); accents != 2 {
		t.Fatalf("accent count = %d, want 2", accents)
	}
}

func TestLiteralFencesStayNeutralRowsUntilLR(t *testing.T) {
	t.Parallel()

	result := parse(t, "(x) + lr((y))")
	requireNoDiagnostics(t, "literal and stretchy fences", result)
	var fenced int
	ir.Walk(result.Root, func(node ir.Node) bool {
		if _, ok := node.(*ir.Fenced); ok {
			fenced++
		}
		return true
	})
	if fenced != 1 {
		t.Fatalf("fenced node count = %d, want 1", fenced)
	}
}

func TestNeutralAtomMetadata(t *testing.T) {
	t.Parallel()

	result := parse(t, `op("tr") cal(L) hat(x) integral.surf`)
	requireNoDiagnostics(t, "neutral metadata", result)
	operator := find[*ir.Operator](t, result.Root)
	if operator.Class != ir.ClassLarge {
		t.Fatalf("operator class = %v, want large", operator.Class)
	}
	accent := find[*ir.Accent](t, result.Root)
	if accent.Accent.NodeBase().Class != ir.ClassDiacritic {
		t.Fatalf("accent mark class = %v", accent.Accent.NodeBase().Class)
	}
}

func countNodes[T ir.Node](root ir.Node) int {
	count := 0
	ir.Walk(root, func(node ir.Node) bool {
		if _, ok := node.(T); ok {
			count++
		}
		return true
	})
	return count
}

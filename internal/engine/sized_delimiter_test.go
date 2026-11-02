package engine

import (
	"strings"
	"testing"

	"github.com/caelyreth/untie/display"
	"github.com/caelyreth/untie/internal/typst"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestMoustachePiecesAreScopedToMoustacheDelimiters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text  string
		runes string
	}{
		{text: "⎰", runes: "\u23a7\u23aa\u23ad"},
		{text: "⎱", runes: "\u23ab\u23aa\u23a9"},
	}
	for _, tt := range tests {
		parts, ok := moustachePieces(tt.text)
		if !ok {
			t.Fatalf("moustachePieces(%q) = unsupported", tt.text)
		}
		var got strings.Builder
		for _, part := range parts {
			got.WriteRune(part.text)
		}
		if got.String() != tt.runes || !parts[1].extender {
			t.Fatalf("moustachePieces(%q) = %q (repeat=%v), want %q with one extender",
				tt.text, got.String(), parts[1].extender, tt.runes)
		}
	}
	for _, text := range []string{"(", ")", "/", `\`, "⟮", "∥"} {
		if _, ok := moustachePieces(text); ok {
			t.Fatalf("moustachePieces(%q) accepted a non-moustache delimiter", text)
		}
	}
}

func TestNewCMAcceptedDelimiterTierMatrix(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.DefaultLimits())
	commands := []string{`\big`, `\Big`, `\bigg`, `\Bigg`}
	delimiters := []string{
		"(", `\lparen`, ")", `\rparen`,
		"[", `\lbrack`, "]", `\rbrack`,
		`\{`, `\lbrace`, `\}`, `\rbrace`,
		`\lfloor`, `\rfloor`, "⌊", "⌋",
		`\lceil`, `\rceil`, "⌈", "⌉",
		"<", `\langle`, "⟨", ">", `\rangle`, "⟩", `\lt`, `\gt`,
		`\lvert`, `\rvert`, `\lVert`, `\rVert`,
		`\lgroup`, `\rgroup`, "⟮", "⟯",
		`\lmoustache`, `\rmoustache`, "⎰", "⎱",
		"/", `\backslash`, "|", `\vert`, `\|`, `\Vert`,
		`\uparrow`, `\Uparrow`, `\downarrow`, `\Downarrow`,
		`\updownarrow`, `\Updownarrow`, ".",
	}
	for _, command := range commands {
		for _, delimiter := range delimiters {
			sourceText := command + delimiter
			box := layoutTeX(t, engine, sourceText, ir.Display)
			if len(box.Hits) == 0 {
				t.Fatalf("%s produced no source hit", sourceText)
			}
			if delimiter != "." && len(box.Items) == 0 {
				t.Fatalf("%s produced no drawable output", sourceText)
			}
		}
	}
}

func TestNewCMTallMoustachesUseSyntheticAssembly(t *testing.T) {
	t.Parallel()

	tests := []string{
		`\left\lmoustache\frac{\frac{a}{b}}{\frac{c}{d}}\right\rmoustache`,
		`\left\rmoustache\frac{\frac{a}{b}}{\frac{c}{d}}\right\lmoustache`,
	}
	for _, sourceText := range tests {
		_, engine := newNewCMEngine(t, source.DefaultLimits())
		box := layoutTeX(t, engine, sourceText, ir.Display)
		if box.Ascent+box.Descent < 40 {
			t.Fatalf("%s extent = %v, want a synthetic moustache assembly", sourceText, box.Ascent+box.Descent)
		}
		if len(box.Items) < 8 {
			t.Fatalf("%s items = %d, want assembled moustache pieces", sourceText, len(box.Items))
		}
	}
}

func TestTypstZeroSizedDelimitersUseOrdinaryGlyphs(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.DefaultLimits())
	for _, src := range []string{
		`lr(|, size: #0em)`,
		`lr((frac(a,b)), size: #0%)`,
	} {
		result := typst.Parse(src, ir.Inline, typst.Options{})
		if len(result.Diagnostics) != 0 {
			t.Fatalf("%s diagnostics = %#v", src, result.Diagnostics)
		}
		box, err := engine.Layout(t.Context(), result.Root, ir.Inline, display.Style{Size: 16})
		if err != nil {
			t.Fatalf("%s layout error = %v", src, err)
		}
		if len(box.Items) == 0 || box.Width <= 0 {
			t.Fatalf("%s box = %#v, want visible ordinary delimiters", src, box)
		}
	}
}

func TestNewCMSizedDelimiterGeometry(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.DefaultLimits())
	tests := []struct {
		source      string
		width       float32
		ascent      float32
		descent     float32
		glyphRefs   int
		outputItems int
	}{
		{source: `\bigl(`, width: 8.367919922, ascent: 15.552001953, descent: 7.552001953, glyphRefs: 1, outputItems: 2},
		{source: `\Bigl(`, width: 10.607910156, ascent: 20.736083984, descent: 12.736083984, glyphRefs: 1, outputItems: 2},
		{source: `\biggl(`, width: 14, ascent: 27.919921875, descent: 19.919921875, glyphRefs: 1, outputItems: 2},
		{source: `\Biggl(`, width: 14, ascent: 28, descent: 20, outputItems: 3},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			box := layoutTeX(t, engine, tt.source, ir.Display)
			assertBoxMetrics(t, box, tt.width, tt.ascent, tt.descent)
			if refs := glyphRefs(box); len(refs) != tt.glyphRefs {
				t.Fatalf("%s glyph refs = %d (%+v), want %d", tt.source, len(refs), refs, tt.glyphRefs)
			}
			if len(box.Items) != tt.outputItems {
				t.Fatalf("%s output items = %d, want %d", tt.source, len(box.Items), tt.outputItems)
			}
			var hit *display.Hit
			for i := range box.Hits {
				if box.Hits[i].Kind == display.HitFence && box.Hits[i].Source == source.NewRange(0, len(tt.source)) {
					hit = &box.Hits[i]
					break
				}
			}
			if hit == nil {
				t.Fatalf("%s hits = %+v, want full-source fence hit", tt.source, box.Hits)
			}
		})
	}
}

func TestNewCMSizedDelimiterIgnoresCurrentMathStyleForTarget(t *testing.T) {
	t.Parallel()

	_, engine := newNewCMEngine(t, source.DefaultLimits())
	textBox := layoutTeX(t, engine, `\bigl(`, ir.Display)
	scriptBox := layoutTeX(t, engine, `\scriptstyle\bigl(`, ir.Display)
	assertBoxMetrics(t, scriptBox, textBox.Width, textBox.Ascent, textBox.Descent)
}

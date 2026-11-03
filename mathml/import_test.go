package mathml_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/untie/identity"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/mathml"
	"github.com/caelyreth/untie/source"
)

func TestUnmarshalUsefulMathMLSubset(t *testing.T) {
	t.Parallel()
	const input = `<math xmlns="http://www.w3.org/1998/Math/MathML" display="block">
		<mrow>
			<msup><mi>x</mi><mn>2</mn></msup>
			<mo>+</mo>
			<mfrac><mn>1</mn><mroot><mi>y</mi><mn>3</mn></mroot></mfrac>
			<msubsup><mo>∑</mo><mi>i</mi><mi>n</mi></msubsup>
			<munderover><mo>∫</mo><mn>0</mn><mn>1</mn></munderover>
			<msqrt><mtext>hello</mtext></msqrt>
		</mrow>
	</math>`
	result, err := mathml.UnmarshalString(context.Background(), input, mathml.Options{})
	if err != nil {
		t.Fatalf("Unmarshal: %v diagnostics=%+v", err, result.Diagnostics)
	}
	if result.Mode != ir.Display || result.Formula == nil {
		t.Fatalf("result = %+v", result)
	}
	row, ok := result.Formula.Root.(*ir.Row)
	if !ok || len(row.Items) != 6 {
		t.Fatalf("root = %#v", result.Formula.Root)
	}
	sup, ok := row.Items[0].(*ir.Scripts)
	if !ok || sup.Top == nil || sup.Bottom != nil || sup.Role != ir.ScriptRoleOrdinary {
		t.Fatalf("msup = %#v", row.Items[0])
	}
	operator, ok := row.Items[1].(*ir.Operator)
	if !ok || operator.Text != "+" || operator.Class != ir.ClassBinary {
		t.Fatalf("mo = %#v", row.Items[1])
	}
	fraction, ok := row.Items[2].(*ir.Fraction)
	if !ok {
		t.Fatalf("mfrac = %#v", row.Items[2])
	}
	root, ok := fraction.Denominator.(*ir.Radical)
	if !ok || root.Index == nil {
		t.Fatalf("mroot = %#v", fraction.Denominator)
	}
	subsup, ok := row.Items[3].(*ir.Scripts)
	if !ok || subsup.Top == nil || subsup.Bottom == nil || subsup.BaseItem.NodeBase().Class != ir.ClassLarge {
		t.Fatalf("msubsup = %#v", row.Items[3])
	}
	underover, ok := row.Items[4].(*ir.Scripts)
	if !ok || underover.Top == nil || underover.Bottom == nil || underover.Role != ir.ScriptRoleLimits {
		t.Fatalf("munderover = %#v", row.Items[4])
	}
	radical, ok := row.Items[5].(*ir.Radical)
	if !ok || radical.Index != nil {
		t.Fatalf("msqrt = %#v", row.Items[5])
	}
	text, ok := radical.Radicand.(*ir.Text)
	if !ok || text.Text != "hello" || text.Font != ir.FontText {
		t.Fatalf("mtext = %#v", radical.Radicand)
	}
}

func TestUnmarshalMFencedAndCustomFencedRow(t *testing.T) {
	t.Parallel()
	const input = `<math xmlns="http://www.w3.org/1998/Math/MathML">
		<mfenced open="[" close="]" separators=";,"><mi>a</mi><mi>b</mi><mi>c</mi></mfenced>
		<mrow data-untie-kind="fenced" data-untie-has-open="true" data-untie-has-close="true" data-untie-balanced="true">
			<mo>(</mo><mi>x</mi><mo>)</mo>
		</mrow>
	</math>`
	result, err := mathml.UnmarshalString(context.Background(), input, mathml.Options{})
	if err != nil {
		t.Fatalf("Unmarshal: %v diagnostics=%+v", err, result.Diagnostics)
	}
	row := result.Formula.Root.(*ir.Row)
	if len(row.Items) != 2 {
		t.Fatalf("items = %d", len(row.Items))
	}
	mfenced := row.Items[0].(*ir.Fenced)
	if !mfenced.Balanced || mfenced.Open == nil || mfenced.Close == nil || len(mfenced.Body) != 5 {
		t.Fatalf("mfenced = %#v", mfenced)
	}
	custom := row.Items[1].(*ir.Fenced)
	if !custom.Balanced || custom.Open == nil || custom.Close == nil || len(custom.Body) != 1 {
		t.Fatalf("custom fenced = %#v", custom)
	}
}

func TestUnmarshalMetadataIDsColorAndSourceIndependence(t *testing.T) {
	t.Parallel()
	const input = `<math xmlns="http://www.w3.org/1998/Math/MathML" display="inline" data-untie-suppressed-tags="2">
		<mrow data-untie-node-id="7" data-untie-class="inner" data-untie-node-size="text"
			data-untie-node-cramped="true" mathvariant="bold" mathcolor="#12345678"
			data-untie-limits="display" lspace="0.25em" rspace="-0.125em"
			data-untie-ignore-as-space="true" aria-label="ignore me">
			<mi>x</mi>
		</mrow>
	</math>`
	result, err := mathml.UnmarshalString(context.Background(), input, mathml.Options{})
	if err != nil {
		t.Fatalf("Unmarshal: %v diagnostics=%+v", err, result.Diagnostics)
	}
	row := result.Formula.Root.(*ir.Row)
	base := row.NodeBase()
	if base.NodeID != identity.NodeID(7) || base.Class != ir.ClassInner ||
		base.Size != ir.SizeText || !base.Cramped || base.Variant != ir.VariantBold ||
		base.Limits != ir.LimitsDisplay || !base.IgnoreAsSpace {
		t.Fatalf("base = %#v", base)
	}
	if !base.Paint.HasExplicit || base.Paint.Explicit != (ir.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x78}) {
		t.Fatalf("paint = %#v", base.Paint)
	}
	if base.LeftSpacing != ir.RawLength(ir.LengthEm, 250_000) ||
		base.RightSpacing != ir.RawLength(ir.LengthEm, -125_000) {
		t.Fatalf("spacing = %v/%v", base.LeftSpacing, base.RightSpacing)
	}
	if base.Span != source.Detached() || base.Origin != source.Detached() {
		t.Fatalf("source metadata leaked into IR: %#v", base)
	}
	if len(result.Formula.Tags) != 2 || !result.Formula.Tags[0].Suppressed || !result.Formula.Tags[1].Suppressed {
		t.Fatalf("tags = %#v", result.Formula.Tags)
	}
}

func TestUnmarshalTableMultilineAndSpaceMarkers(t *testing.T) {
	t.Parallel()
	const input = `<math xmlns="http://www.w3.org/1998/Math/MathML">
		<mtable data-untie-table-role="cases" columnalign="left right" rowspacing="0.2em" columnspacing="0.3em">
			<mtr><mtd><mi>a</mi></mtd><mtd><mi>b</mi></mtd></mtr>
			<mtr><mtd><mi>c</mi></mtd><mtd><mi>d</mi></mtd></mtr>
		</mtable>
		<mtable data-untie-kind="multiline" data-untie-centered="true">
			<mtr><mtd><mi>x</mi><mspace data-untie-kind="align-marker" width="0"/><mi>y</mi></mtd></mtr>
		</mtable>
		<mspace data-untie-kind="break" data-untie-break-kind="relation" width="0"/>
	</math>`
	result, err := mathml.UnmarshalString(context.Background(), input, mathml.Options{})
	if err != nil {
		t.Fatalf("Unmarshal: %v diagnostics=%+v", err, result.Diagnostics)
	}
	row := result.Formula.Root.(*ir.Row)
	if len(row.Items) != 3 {
		t.Fatalf("items = %d", len(row.Items))
	}
	table := row.Items[0].(*ir.Table)
	if table.Role != ir.TableRoleCases || len(table.Rows) != 2 || len(table.Columns) != 2 {
		t.Fatalf("table = %#v", table)
	}
	if table.RowGap != ir.RawLength(ir.LengthEm, 200_000) || table.ColumnGap != ir.RawLength(ir.LengthEm, 300_000) {
		t.Fatalf("table gaps = %v/%v", table.RowGap, table.ColumnGap)
	}
	multiline := row.Items[1].(*ir.Multiline)
	if !multiline.Centered || len(multiline.Rows) != 1 || len(multiline.Rows[0]) != 3 {
		t.Fatalf("multiline = %#v", multiline)
	}
	if _, ok := multiline.Rows[0][1].(*ir.AlignMarker); !ok {
		t.Fatalf("align marker = %#v", multiline.Rows[0][1])
	}
	breakNode := row.Items[2].(*ir.Break)
	if breakNode.Break != ir.BreakRelation {
		t.Fatalf("break = %#v", breakNode)
	}
}

func TestUnmarshalRejectsUnknownAndMalformedConstructs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		xml  string
		want error
		code source.Code
	}{
		{
			name: "unknown element",
			xml:  `<math xmlns="http://www.w3.org/1998/Math/MathML"><ms>x</ms></math>`,
			want: mathml.ErrUnsupported,
			code: source.CodeUnsupported,
		},
		{
			name: "unknown attribute",
			xml:  `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi data-unknown="x">x</mi></math>`,
			want: mathml.ErrUnsupported,
			code: source.CodeUnsupported,
		},
		{
			name: "unknown metadata kind",
			xml:  `<math xmlns="http://www.w3.org/1998/Math/MathML"><mrow data-untie-kind="magic"/></math>`,
			want: mathml.ErrUnsupported,
			code: source.CodeUnsupported,
		},
		{
			name: "wrong namespace",
			xml:  `<math xmlns="urn:not-math"><mi>x</mi></math>`,
			want: mathml.ErrInvalidMathML,
			code: source.CodeSyntax,
		},
		{
			name: "foreign child",
			xml:  `<math xmlns="http://www.w3.org/1998/Math/MathML"><mrow><x:mi xmlns:x="urn:not-math">x</x:mi></mrow></math>`,
			want: mathml.ErrUnsupported,
			code: source.CodeUnsupported,
		},
		{
			name: "malformed",
			xml:  `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</math>`,
			want: mathml.ErrInvalidMathML,
			code: source.CodeSyntax,
		},
		{
			name: "directive",
			xml:  `<!DOCTYPE math><math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`,
			want: mathml.ErrUnsupported,
			code: source.CodeUnsupported,
		},
		{
			name: "duplicate attribute",
			xml:  `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi class="a" class="b">x</mi></math>`,
			want: mathml.ErrInvalidMathML,
			code: source.CodeSyntax,
		},
		{
			name: "custom fraction thickness",
			xml:  `<math xmlns="http://www.w3.org/1998/Math/MathML"><mfrac linethickness="2pt"><mi>a</mi><mi>b</mi></mfrac></math>`,
			want: mathml.ErrUnsupported,
			code: source.CodeUnsupported,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := mathml.UnmarshalString(context.Background(), test.xml, mathml.Options{})
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if result.Formula != nil {
				t.Fatalf("formula = %#v, want nil", result.Formula)
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != test.code ||
				result.Diagnostics[0].Severity != source.SeverityError {
				t.Fatalf("diagnostics = %#v", result.Diagnostics)
			}
		})
	}
}

func TestUnmarshalFencedDelimiterCombinations(t *testing.T) {
	t.Parallel()

	for _, hasOpen := range []bool{false, true} {
		for _, hasClose := range []bool{false, true} {
			for childCount := 0; childCount <= 2; childCount++ {
				hasOpen, hasClose, childCount := hasOpen, hasClose, childCount
				t.Run(fmt.Sprintf("open=%t/close=%t/children=%d", hasOpen, hasClose, childCount), func(t *testing.T) {
					t.Parallel()
					var children strings.Builder
					for i := 0; i < childCount; i++ {
						fmt.Fprintf(&children, "<mi>%c</mi>", 'a'+i)
					}
					input := fmt.Sprintf(
						`<math xmlns="http://www.w3.org/1998/Math/MathML"><mrow data-untie-kind="fenced" data-untie-has-open="%t" data-untie-has-close="%t">%s</mrow></math>`,
						hasOpen,
						hasClose,
						children.String(),
					)
					result, err := mathml.UnmarshalString(context.Background(), input, mathml.Options{})
					delimiters := 0
					if hasOpen {
						delimiters++
					}
					if hasClose {
						delimiters++
					}
					if childCount == 0 || childCount < delimiters {
						if !errors.Is(err, mathml.ErrInvalidMathML) {
							t.Fatalf("error = %v, want ErrInvalidMathML", err)
						}
						return
					}
					if err != nil {
						t.Fatalf("UnmarshalString() error = %v", err)
					}
					if result.Formula == nil {
						t.Fatal("nil imported formula")
					}
				})
			}
		}
	}
}

func TestUnmarshalExportOnlyElementsAreExplicitlyUnsupported(t *testing.T) {
	t.Parallel()
	for _, element := range []string{
		"mmultiscripts", "mpadded", "mstyle", "menclose", "mphantom", "merror", "semantics", "mlabeledtr",
	} {
		element := element
		t.Run(element, func(t *testing.T) {
			t.Parallel()
			input := `<math xmlns="http://www.w3.org/1998/Math/MathML"><` + element + `><mi>x</mi></` + element + `></math>`
			result, err := mathml.UnmarshalString(context.Background(), input, mathml.Options{})
			if !errors.Is(err, mathml.ErrUnsupported) {
				t.Fatalf("error = %v, want ErrUnsupported", err)
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeUnsupported {
				t.Fatalf("diagnostics = %#v", result.Diagnostics)
			}
		})
	}
}

func TestUnmarshalLimitsCancellationAndContext(t *testing.T) {
	t.Parallel()
	input := []byte(`<math xmlns="http://www.w3.org/1998/Math/MathML"><mrow><mi>x</mi></mrow></math>`)
	result, err := mathml.Unmarshal(context.Background(), input, mathml.Options{
		Limits: source.Limits{MaxSourceBytes: 16, MaxTokens: 100, MaxNodes: 100},
	})
	if !errors.Is(err, mathml.ErrLimitExceeded) || len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != source.CodeLimitExceeded {
		t.Fatalf("limited result = %+v, err=%v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = mathml.Unmarshal(ctx, input, mathml.Options{})
	if !errors.Is(err, context.Canceled) || len(result.Diagnostics) != 0 {
		t.Fatalf("canceled result = %+v, err=%v", result, err)
	}
	if _, err := mathml.Unmarshal(nilContext(), input, mathml.Options{}); !errors.Is(err, mathml.ErrInvalidContext) {
		t.Fatalf("nil context error = %v", err)
	}
}

func nilContext() context.Context {
	return nil
}

func TestUnmarshalSizedDelimiterAndConflictDiagnostics(t *testing.T) {
	t.Parallel()
	const sized = `<math xmlns="http://www.w3.org/1998/Math/MathML"><mo data-untie-kind="sized-delimiter" data-untie-sizing-kind="absolute" data-untie-sizing-value="2em">(</mo></math>`
	result, err := mathml.UnmarshalString(context.Background(), sized, mathml.Options{})
	if err != nil {
		t.Fatal(err)
	}
	delimiter := result.Formula.Root.(*ir.SizedDelimiter)
	if delimiter.Sizing != ir.AbsoluteDelimiterSizing(ir.RawLength(ir.LengthEm, 2_000_000)) {
		t.Fatalf("sizing = %#v", delimiter.Sizing)
	}

	const conflict = `<math xmlns="http://www.w3.org/1998/Math/MathML"><mspace data-untie-space-type="explicit" data-untie-space-amount="1em" width="2em"/></math>`
	result, err = mathml.UnmarshalString(context.Background(), conflict, mathml.Options{})
	if !errors.Is(err, mathml.ErrInvalidMathML) || len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != source.CodeSyntax {
		t.Fatalf("conflict result=%+v err=%v", result, err)
	}
}

func TestUnmarshalCancelsDuringTraversal(t *testing.T) {
	t.Parallel()
	input := []byte(`<math xmlns="http://www.w3.org/1998/Math/MathML"><mrow><mi>a</mi><mi>b</mi><mi>c</mi></mrow></math>`)
	ctx := &cancelAfter{remaining: 4}
	result, err := mathml.Unmarshal(ctx, input, mathml.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != source.CodeCanceled {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

type cancelAfter struct {
	remaining int
}

func (*cancelAfter) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*cancelAfter) Done() <-chan struct{}       { return nil }
func (c *cancelAfter) Err() error {
	if c.remaining <= 0 {
		return context.Canceled
	}
	c.remaining--
	return nil
}
func (*cancelAfter) Value(any) any { return nil }

func TestUnmarshalLargeInputReportsStructuredLimit(t *testing.T) {
	t.Parallel()
	input := []byte(strings.Repeat(" ", 64) + `<math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`)
	result, err := mathml.Unmarshal(context.Background(), input, mathml.Options{
		Limits: source.Limits{MaxSourceBytes: 32, MaxTokens: 100, MaxNodes: 100},
	})
	if !errors.Is(err, mathml.ErrLimitExceeded) || len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != source.CodeLimitExceeded {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

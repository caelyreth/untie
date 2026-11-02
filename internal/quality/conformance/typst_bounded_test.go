package conformance

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestBoundedGeneratedTypst(t *testing.T) {
	random := rand.New(rand.NewSource(0x7479707374))
	fragments := []string{
		"x", "y", "0", "1", "+", "-", "=", "(", ")", "[", "]", "{", "}",
		"^", "_", "&", "/", "\\", "#", "\"text\"", "frac(a,b)", "sqrt(x)",
		"root(3,x)", "mat(1,2;3,4)", "attach(x,t:2)", "lr((", "arrow.r", "α",
		"😀", "#math.frac(1,2)", "#let x = 1", "/*", "\\u{1F600}",
	}
	for caseIndex := 0; caseIndex < 128; caseIndex++ {
		var builder strings.Builder
		for count := 1 + random.Intn(20); count > 0; count-- {
			fragment := fragments[random.Intn(len(fragments))]
			if builder.Len()+len(fragment) > 384 {
				break
			}
			builder.WriteString(fragment)
		}
		sourceText := builder.String()
		t.Run(fmt.Sprintf("case-%03d", caseIndex), func(t *testing.T) {
			first := runObservedTypstFrontend(t, sourceText, source.DefaultLimits())
			repeated := runObservedTypstFrontend(t, sourceText, source.DefaultLimits())
			explicit := runObservedTypstFrontend(t, sourceText, ExplicitTypstLimits())
			if !bytes.Equal(first.Observation.CanonicalJSON, repeated.Observation.CanonicalJSON) {
				t.Fatalf("canonical IR is not deterministic for %q", sourceText)
			}
			if !bytes.Equal(first.Observation.CanonicalJSON, explicit.Observation.CanonicalJSON) {
				t.Fatalf("IR differs between limit profiles for %q", sourceText)
			}
			if got, want := diagnosticSignature(explicit.Diagnostics), diagnosticSignature(first.Diagnostics); got != want {
				t.Fatalf("diagnostics differ for %q: %q vs %q", sourceText, got, want)
			}
			for _, diagnostic := range explicit.Diagnostics {
				if diagnostic.Code == source.CodeInternal {
					t.Fatalf("internal diagnostic for %q: %v", sourceText, diagnostic)
				}
			}
		})
	}
}

func runObservedTypstFrontend(t *testing.T, src string, limits source.Limits) FrontendResult {
	t.Helper()
	result, err := ParseTypst(context.Background(), Request{Source: src, Mode: ir.Inline, Limits: limits})
	if err != nil {
		t.Fatalf("frontend failed for %q: %v", src, err)
	}
	canonical, problem := summarizeIR(result, src, limits)
	if problem != "" {
		t.Fatalf("invalid IR for %q: %s", src, problem)
	}
	result.Observation = &IRSummary{Valid: true, CanonicalJSON: canonical}
	return result
}

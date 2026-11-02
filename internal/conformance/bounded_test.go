package conformance

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestBoundedGeneratedTeX(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	frontend := testProcessFrontend()
	random := rand.New(rand.NewSource(0x756e746965))
	fragments := []string{
		"x", "y", "0", "1", "+", "-", "=", "{", "}", "^", "_", "&", `\\`,
		`\frac`, `\sqrt`, `\alpha`, `\beta`, `\left(`, `\right)`, `\sum`,
		`\int`, `\begin{matrix}`, `\end{matrix}`, `\mathrm`, `\mathbf`, `\,`, `\quad`,
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
			first := runObservedFrontend(t, ctx, frontend, sourceText, source.DefaultLimits())
			second := runObservedFrontend(t, ctx, frontend, sourceText, ExplicitTeXLimits())
			if !bytes.Equal(first.Observation.CanonicalJSON, second.Observation.CanonicalJSON) {
				t.Fatalf("IR differs between limit profiles for %q", sourceText)
			}
			if got, want := diagnosticSignature(second.Diagnostics), diagnosticSignature(first.Diagnostics); got != want {
				t.Fatalf("diagnostics differ for %q: %q vs %q", sourceText, got, want)
			}
			for _, diagnostic := range first.Diagnostics {
				if diagnostic.Code == source.CodeInternal {
					t.Fatalf("internal diagnostic for %q: %v", sourceText, diagnostic)
				}
			}
			for _, diagnostic := range second.Diagnostics {
				if diagnostic.Code == source.CodeInternal {
					t.Fatalf("internal diagnostic for %q: %v", sourceText, diagnostic)
				}
			}
		})
	}
}

func runObservedFrontend(t *testing.T, parent context.Context, frontend FrontendFunc, src string, limits source.Limits) FrontendResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond)
	defer cancel()
	result, err := frontend(ctx, Request{Source: src, Mode: ir.Inline, Limits: limits})
	if err != nil {
		t.Fatalf("frontend failed for %q: %v", src, err)
	}
	if result.Observation == nil || !result.Observation.Valid {
		t.Fatalf("invalid IR for %q: %#v", src, result.Observation)
	}
	return result
}

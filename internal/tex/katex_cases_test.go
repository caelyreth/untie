package tex

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

type katexCase struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Provenance string `json:"provenance"`
	Diagnostic string `json:"diagnostic"`
}

func TestStaticKaTeXCases(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../testdata/tex/katex_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []katexCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 20 {
		t.Fatalf("only %d cases loaded", len(cases))
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			result := Parse(tc.Source, ir.Inline, Options{})
			if err := ir.Validate(&ir.Formula{Root: result.Root}, source.DefaultLimits()); err != nil {
				t.Fatalf("%s: invalid IR: %v", tc.Provenance, err)
			}
			if tc.Diagnostic == "" {
				if len(result.Diagnostics) != 0 {
					t.Fatalf("%s: unexpected diagnostics: %v", tc.Provenance, result.Diagnostics)
				}
				return
			}
			if !hasDiagnostic(result.Diagnostics, source.Code(tc.Diagnostic)) {
				t.Fatalf("%s: diagnostics = %v, want %s", tc.Provenance, diagnosticCodes(result.Diagnostics), tc.Diagnostic)
			}
		})
	}
}

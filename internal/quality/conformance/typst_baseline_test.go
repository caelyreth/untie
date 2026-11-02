package conformance

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/untie/internal/quality/fixture"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTypstConformanceBaseline(t *testing.T) {
	if raceEnabled {
		t.Skip("process-per-case corpus is covered by non-race CI; run package race tests without baseline workers")
	}
	packageRoot := conformancePackageRoot(t)
	report, err := BuildTypstReport(context.Background(), SuiteOptions{
		FixtureRoot: filepath.Join(packageRoot, "..", "..", "..", "testdata", "upstream"),
		PolicyPath:  filepath.Join(packageRoot, "testdata", "typst-policy.json"),
		Frontend:    testTypstProcessFrontend(),
		CaseTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("actual report: %v", err)
	}
	if err := CheckTypstCorpusSet(report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Unexplained != 0 {
		t.Fatalf("report has %d unexplained cases", report.Summary.Unexplained)
	}
	if report.Summary.Total != 171 {
		t.Fatalf("total = %d, want 171", report.Summary.Total)
	}

	want, err := LoadReport(filepath.Join(packageRoot, "testdata", "typst-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if difference := CompareReports(want, report); difference != "" {
		t.Fatalf("Typst conformance baseline changed; review the behavior before explicitly regenerating with cmd/typstreport: %s", difference)
	}
}

func TestTypstSupplementarySpansUseUTF8ByteOffsets(t *testing.T) {
	packageRoot := conformancePackageRoot(t)
	root := filepath.Join(packageRoot, "..", "..", "..", "testdata", "upstream")
	manifest, err := fixture.LoadCorpusManifest(filepath.Join(root, "manifests", "corpora.json"))
	if err != nil {
		t.Fatal(err)
	}
	var syntax fixture.CorpusSpec
	for _, spec := range manifest.Corpora {
		if spec.Name == typletSyntaxCorpus {
			syntax = spec
			break
		}
	}
	corpus, err := fixture.LoadCorpus(root, syntax)
	if err != nil {
		t.Fatal(err)
	}
	var astral fixture.Case
	for _, testCase := range corpus.Cases {
		if testCase.ID == "astral-letters" {
			astral = testCase
			break
		}
	}
	if astral.ID == "" {
		t.Fatal("astral-letters is absent from the imported syntax corpus")
	}

	result, err := ParseTypst(t.Context(), Request{
		Source: astral.Source,
		Mode:   ir.Inline,
		Limits: ExplicitTypstLimits(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if errorsFound := errorDiagnostics(result.Diagnostics); len(errorsFound) != 0 {
		t.Fatalf("astral-letters diagnostics: %#v", errorsFound)
	}
	if problem := validateIRBoundaries(result.Root, astral.Source); problem != "" {
		t.Fatal(problem)
	}
	if _, problem := summarizeIR(result, astral.Source, ExplicitTypstLimits()); problem != "" {
		t.Fatal(problem)
	}

	wantSpans := map[string]source.Range{
		"𝑥": spanOf(astral.Source, "𝑥"),
		"𝔸": spanOf(astral.Source, "𝔸"),
		"𝑛": spanOf(astral.Source, "𝑛"),
		"😀": spanOf(astral.Source, "😀"),
	}
	found := make(map[string]bool)
	ir.Walk(result.Root, func(node ir.Node) bool {
		glyph, ok := node.(*ir.Glyph)
		if !ok {
			return true
		}
		want, ok := wantSpans[glyph.Text]
		if !ok {
			return true
		}
		found[glyph.Text] = true
		if glyph.Span != want {
			t.Errorf("glyph %q span = %#v, want UTF-8 bytes %#v", glyph.Text, glyph.Span, want)
		}
		if !utf8.Valid(glyph.Span.Slice(astral.Source)) {
			t.Errorf("glyph %q span does not slice a valid code point", glyph.Text)
		}
		return true
	})
	for text := range wantSpans {
		if !found[text] {
			t.Errorf("supplementary glyph %q was not observed", text)
		}
	}

	workerResult, err := testTypstProcessFrontend()(t.Context(), Request{
		Source: astral.Source,
		Mode:   ir.Inline,
		Limits: ExplicitTypstLimits(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if workerResult.Observation == nil || !workerResult.Observation.Valid || len(workerResult.Observation.CanonicalJSON) == 0 {
		t.Fatalf("unexpected isolated observation: %#v", workerResult.Observation)
	}
	for _, span := range wantSpans {
		encoded := []byte(`[` + strconv.Itoa(span.Start) + `,` + strconv.Itoa(span.End) + `]`)
		if !bytes.Contains(workerResult.Observation.CanonicalJSON, encoded) {
			t.Errorf("isolated canonical IR lacks UTF-8 span %s", encoded)
		}
	}
}

func TestTypstSourceFamilies(t *testing.T) {
	families, commands, environments := typstSourceFamilies(`frac(a, b) + P(A mid(|) B) + dots.c + x_i`)
	for _, want := range []string{
		"function:frac",
		"function:mid",
		"field:dots.c",
		"call:P",
		"feature:fraction",
		"feature:scripts",
		"feature:group",
	} {
		if !slices.Contains(families, want) {
			t.Errorf("families %v do not contain %q", families, want)
		}
	}
	if slices.Contains(commands, "P") {
		t.Errorf("juxtaposed uppercase P was misclassified as a function: %v", commands)
	}
	if !slices.Equal(commands, []string{"frac", "mid"}) {
		t.Errorf("commands = %v", commands)
	}
	if len(environments) != 0 {
		t.Errorf("environments = %v", environments)
	}
}

func TestUTF8BoundaryValidationRejectsSplitCodePoint(t *testing.T) {
	src := "𝑥"
	if !isRangeOnUTF8Boundaries(source.NewRange(0, len(src)), src) {
		t.Fatal("complete supplementary code point was rejected")
	}
	if isRangeOnUTF8Boundaries(source.NewRange(1, len(src)), src) {
		t.Fatal("span starting inside a supplementary code point was accepted")
	}
	if isRangeOnUTF8Boundaries(source.NewRange(0, 2), src) {
		t.Fatal("span ending inside a supplementary code point was accepted")
	}
}

func spanOf(src, text string) source.Range {
	start := strings.Index(src, text)
	if start < 0 {
		return source.Detached()
	}
	return source.NewRange(start, start+len(text))
}

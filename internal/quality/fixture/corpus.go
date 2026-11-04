package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Case is one loaded corpus record. Index is one-based and follows source
// order. A case without a frame fixture is still runnable; it is never omitted.
type Case struct {
	Index       int
	ID          string
	Source      string
	Display     bool
	Preamble    string
	TeX         string
	Category    string
	Fixture     *Fixture
	CST         *SourceCSTNode
	Diagnostics []ResolvedDiagnostic
	Meta        json.RawMessage
}

// Corpus is a manifest-backed, ordered set of cases.
type Corpus struct {
	Name  string
	Spec  CorpusSpec
	Cases []Case
}

// LoadCorpus loads every case declared by spec and enforces exact counts.
func LoadCorpus(root string, spec CorpusSpec) (*Corpus, error) {
	corpusPath, err := safeJoin(root, spec.CorpusFile)
	if err != nil {
		return nil, err
	}
	var cases []Case
	switch spec.Format {
	case TypletJSONL:
		cases, err = loadTypletCases(root, corpusPath, spec)
	case RawLines:
		cases, err = loadRawCases(corpusPath, spec.ExpectedCount, spec.CommentPrefix)
	case JSONFormulas:
		cases, err = loadJSONFormulaCases(corpusPath, spec.ExpectedCount)
	case KaTeXJSONL:
		var records []KaTeXCase
		records, err = LoadKaTeXCases(corpusPath, spec.ExpectedCount)
		if err == nil {
			cases = make([]Case, len(records))
			for i, record := range records {
				meta, marshalErr := json.Marshal(record)
				if marshalErr != nil {
					return nil, fmt.Errorf("%s: encode case %q: %w", corpusPath, record.ID, marshalErr)
				}
				cases[i] = Case{
					Index: i + 1, ID: record.ID, Source: record.Source,
					Meta: meta,
				}
			}
		}
	default:
		err = fmt.Errorf("unsupported corpus format %q", spec.Format)
	}
	if err != nil {
		return nil, err
	}
	if len(cases) != spec.ExpectedCount {
		return nil, fmt.Errorf("corpus %q loaded %d cases, want %d", spec.Name, len(cases), spec.ExpectedCount)
	}
	return &Corpus{Name: spec.Name, Spec: spec, Cases: cases}, nil
}

// LoadAllCorpora loads every corpus in declaration order.
func LoadAllCorpora(root string, manifest *CorpusManifest) ([]*Corpus, error) {
	if manifest == nil {
		return nil, errors.New("nil corpus manifest")
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	corpora := make([]*Corpus, len(manifest.Corpora))
	for i, spec := range manifest.Corpora {
		corpus, err := LoadCorpus(root, spec)
		if err != nil {
			return nil, fmt.Errorf("corpus %q: %w", spec.Name, err)
		}
		corpora[i] = corpus
	}
	return corpora, nil
}

func loadTypletCases(root, corpusPath string, spec CorpusSpec) ([]Case, error) {
	formulas, err := LoadFormulas(corpusPath)
	if err != nil {
		return nil, err
	}
	if len(formulas) != spec.ExpectedCount {
		return nil, fmt.Errorf("%s: decoded %d formulas, want %d", corpusPath, len(formulas), spec.ExpectedCount)
	}
	fixturePath, err := safeJoin(root, spec.FixtureFile)
	if err != nil {
		return nil, err
	}
	fixtures, err := LoadFixtures(fixturePath)
	if err != nil {
		return nil, err
	}
	if len(fixtures) != spec.ExpectedFixtureCount {
		return nil, fmt.Errorf("%s: decoded %d fixtures, want %d", fixturePath, len(fixtures), spec.ExpectedFixtureCount)
	}
	if spec.ExpectedFrameCount == nil {
		return nil, fmt.Errorf("%s: missing expectedFrameCount", spec.Name)
	}
	frameCount := 0
	for _, record := range fixtures {
		if record.Frame != nil {
			frameCount++
		}
	}
	if frameCount != *spec.ExpectedFrameCount {
		return nil, fmt.Errorf("%s: decoded %d frames, want %d", fixturePath, frameCount, *spec.ExpectedFrameCount)
	}

	fixtureByID := make(map[string]*Fixture, len(fixtures))
	for i := range fixtures {
		fixtureByID[fixtures[i].ID] = &fixtures[i]
	}
	formulaIDs := make(map[string]struct{}, len(formulas))
	for _, formula := range formulas {
		formulaIDs[formula.ID] = struct{}{}
		if _, ok := fixtureByID[formula.ID]; !ok {
			return nil, fmt.Errorf("%s: formula %q has no fixture", spec.Name, formula.ID)
		}
	}
	var missing []string
	for id := range fixtureByID {
		if _, ok := formulaIDs[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("%s: fixtures without formulas: %s", spec.Name, strings.Join(missing, ", "))
	}

	if spec.SelectionFile != "" {
		selectionPath, err := safeJoin(root, spec.SelectionFile)
		if err != nil {
			return nil, err
		}
		selection, err := LoadTypletSelection(selectionPath)
		if err != nil {
			return nil, err
		}
		selected, ok := selection.IDs(spec.SelectionKey)
		if !ok {
			return nil, fmt.Errorf("%s: selection has no dataset %q", selectionPath, spec.SelectionKey)
		}
		if len(selected) != spec.ExpectedCount {
			return nil, fmt.Errorf("%s: dataset %q has %d IDs, want %d", selectionPath, spec.SelectionKey, len(selected), spec.ExpectedCount)
		}
		selectedSet := make(map[string]struct{}, len(selected))
		for _, id := range selected {
			selectedSet[id] = struct{}{}
		}
		for id := range formulaIDs {
			if _, ok := selectedSet[id]; !ok {
				return nil, fmt.Errorf("%s: formula %q is absent from selection %q", spec.Name, id, spec.SelectionKey)
			}
		}
		for id := range selectedSet {
			if _, ok := formulaIDs[id]; !ok {
				return nil, fmt.Errorf("%s: selected ID %q is absent from corpus", spec.Name, id)
			}
		}
	}

	cases := make([]Case, len(formulas))
	for i, formula := range formulas {
		fixture := fixtureByID[formula.ID]
		resolved, err := ResolveFixture(formula.Source, formula.Preamble, fixture)
		if err != nil {
			return nil, fmt.Errorf("%s: resolve formula %q: %w", spec.Name, formula.ID, err)
		}
		cases[i] = Case{
			Index: i + 1, ID: formula.ID, Source: formula.Source,
			Display: formula.Display, Preamble: formula.Preamble,
			TeX: formula.TeX, Category: formula.Category,
			Fixture: fixture, CST: resolved.CST,
			Diagnostics: resolved.Diagnostics, Meta: formula.Meta,
		}
	}
	return cases, nil
}

func loadRawCases(filePath string, expected int, commentPrefix string) ([]Case, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := splitRawLines(data, commentPrefix)
	if len(lines) != expected {
		return nil, fmt.Errorf("%s: decoded %d lines, want %d", filePath, len(lines), expected)
	}
	cases := make([]Case, len(lines))
	for i, source := range lines {
		cases[i] = Case{Index: i + 1, ID: fmt.Sprintf("line:%06d", i+1), Source: source}
	}
	return cases, nil
}

type formulaDocument struct {
	Count    int      `json:"count"`
	Formulas []string `json:"formulas"`
}

func loadJSONFormulaCases(filePath string, expected int) ([]Case, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var document formulaDocument
	if err := decodeJSONObject(data, &document, true); err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	if document.Count != expected || len(document.Formulas) != expected {
		return nil, fmt.Errorf(
			"%s: declared count %d, decoded %d formulas, want %d",
			filePath,
			document.Count,
			len(document.Formulas),
			expected,
		)
	}
	cases := make([]Case, len(document.Formulas))
	for i, source := range document.Formulas {
		if strings.TrimSpace(source) == "" {
			return nil, fmt.Errorf("%s: formula %d is empty", filePath, i+1)
		}
		cases[i] = Case{Index: i + 1, ID: fmt.Sprintf("formula:%06d", i+1), Source: source}
	}
	return cases, nil
}

func splitRawLines(data []byte, commentPrefix string) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" || commentPrefix != "" && strings.HasPrefix(line, commentPrefix) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// Classification is the required disposition of every corpus case.
type Classification string

const (
	Pass               Classification = "pass"
	DiagnosticExpected Classification = "diagnostic-expected"
	UnsupportedByScope Classification = "unsupported-by-scope"
	KnownDifference    Classification = "known-difference"
	Bug                Classification = "bug"
)

func (c Classification) valid() bool {
	switch c {
	case Pass, DiagnosticExpected, UnsupportedByScope, KnownDifference, Bug:
		return true
	default:
		return false
	}
}

// Outcome is returned for every case. There is deliberately no skip state.
type Outcome struct {
	Classification Classification
	Frame          *Frame
	Detail         string
}

// RunFunc processes exactly one case.
type RunFunc func(context.Context, Case) (Outcome, error)

// RunOptions controls frame comparison and report size.
type RunOptions struct {
	Tolerance float64
}

// Result is the deterministic result for one case.
type Result struct {
	Index          int
	ID             string
	Classification Classification
	Detail         string
}

// Report accounts for every attempted case. Validate proves that the counts
// sum to Total and that no result is missing or duplicated.
type Report struct {
	Corpus             string
	Total              int
	Pass               int
	DiagnosticExpected int
	UnsupportedByScope int
	KnownDifference    int
	Bug                int
	Results            []Result
}

// Validate checks report conservation.
func (r Report) Validate() error {
	if r.Total == 0 {
		return errors.New("empty corpus report")
	}
	if r.Total != len(r.Results) {
		return fmt.Errorf("report total %d does not match %d results", r.Total, len(r.Results))
	}
	counts := make(map[Classification]int, 5)
	seen := make(map[string]struct{}, len(r.Results))
	for _, result := range r.Results {
		if !result.Classification.valid() {
			return fmt.Errorf("result %q has invalid classification %q", result.ID, result.Classification)
		}
		counts[result.Classification]++
		if _, ok := seen[result.ID]; ok {
			return fmt.Errorf("duplicate result ID %q", result.ID)
		}
		seen[result.ID] = struct{}{}
	}
	for _, field := range []struct {
		name  string
		class Classification
		got   int
	}{
		{"Pass", Pass, r.Pass},
		{"DiagnosticExpected", DiagnosticExpected, r.DiagnosticExpected},
		{"UnsupportedByScope", UnsupportedByScope, r.UnsupportedByScope},
		{"KnownDifference", KnownDifference, r.KnownDifference},
		{"Bug", Bug, r.Bug},
	} {
		if field.got != counts[field.class] {
			return fmt.Errorf("report %s count is %d, but results contain %d", field.name, field.got, counts[field.class])
		}
	}
	return nil
}

// RunCorpus invokes run for every case in source order. A callback error and a
// wrong frame become explicit Bug results rather than omitted cases. Invalid
// classifications and undeclared known differences are programmer errors and
// stop the run.
func RunCorpus(ctx context.Context, corpus *Corpus, options RunOptions, run RunFunc) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if corpus == nil {
		return Report{}, errors.New("nil corpus")
	}
	if run == nil {
		return Report{}, errors.New("nil corpus runner")
	}
	if len(corpus.Cases) != corpus.Spec.ExpectedCount {
		return Report{}, fmt.Errorf("corpus %q has %d cases, manifest declares %d", corpus.Name, len(corpus.Cases), corpus.Spec.ExpectedCount)
	}
	seen := make(map[string]struct{}, len(corpus.Cases))
	for _, testCase := range corpus.Cases {
		if testCase.ID == "" {
			return Report{}, errors.New("corpus contains an empty case ID")
		}
		if _, ok := seen[testCase.ID]; ok {
			return Report{}, fmt.Errorf("corpus contains duplicate case ID %q", testCase.ID)
		}
		seen[testCase.ID] = struct{}{}
	}

	report := Report{Corpus: corpus.Name, Total: len(corpus.Cases)}
	report.Results = make([]Result, 0, len(corpus.Cases))
	for _, testCase := range corpus.Cases {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		outcome, err := run(ctx, testCase)
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return report, contextErr
			}
			addResult(&report, testCase, Bug, err.Error())
			continue
		}
		if !outcome.Classification.valid() {
			return report, fmt.Errorf("case %q returned invalid classification %q", testCase.ID, outcome.Classification)
		}
		if outcome.Classification == KnownDifference {
			reason, ok := corpus.Spec.KnownDifferenceFor(testCase.ID)
			if !ok {
				return report, fmt.Errorf("case %q returned undeclared known difference", testCase.ID)
			}
			detail := reason
			if outcome.Detail != "" {
				detail += ": " + outcome.Detail
			}
			addResult(&report, testCase, KnownDifference, detail)
			continue
		}
		if outcome.Classification == Pass && testCase.Fixture != nil && testCase.Fixture.Frame != nil {
			if outcome.Frame == nil {
				addResult(&report, testCase, Bug, "passing case produced no frame")
				continue
			}
			if diff := CompareFrames(testCase.ID, *outcome.Frame, *testCase.Fixture.Frame, options.Tolerance); diff != nil {
				addResult(&report, testCase, Bug, diff.String())
				continue
			}
		}
		addResult(&report, testCase, outcome.Classification, outcome.Detail)
	}
	if err := report.Validate(); err != nil {
		return report, fmt.Errorf("corpus %q: %w", corpus.Name, err)
	}
	return report, nil
}

func addResult(report *Report, testCase Case, classification Classification, detail string) {
	report.Results = append(report.Results, Result{
		Index: testCase.Index, ID: testCase.ID, Classification: classification, Detail: detail,
	})
	switch classification {
	case Pass:
		report.Pass++
	case DiagnosticExpected:
		report.DiagnosticExpected++
	case UnsupportedByScope:
		report.UnsupportedByScope++
	case KnownDifference:
		report.KnownDifference++
	case Bug:
		report.Bug++
	}
}

// CorpusRoot resolves the tracked fixture root relative to any working
// directory by finding the nearest go.mod.
func CorpusRoot(start string) (string, error) {
	absolute, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(absolute, "go.mod")); err == nil {
			root := filepath.Join(absolute, "testdata", "upstream")
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				return root, nil
			}
			return "", fmt.Errorf("go.mod found at %s but %s is missing", absolute, root)
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			return "", errors.New("go.mod not found")
		}
		absolute = parent
	}
}

package conformance

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/caelyreth/untie/internal/fixture"
	"github.com/caelyreth/untie/source"
)

const reportSchemaVersion = 1

// Dialect identifies the source language evaluated by a report.
type Dialect string

const (
	DialectTeX   Dialect = "tex"
	DialectTypst Dialect = "typst"
)

// Status is the required disposition of a conformance case.
type Status string

const (
	StatusPass        Status = "pass"
	StatusDiagnostic  Status = "diagnostic"
	StatusUnsupported Status = "unsupported"
	StatusBug         Status = "bug"
)

// ReviewKind records why a non-passing case is accepted in a reviewed
// baseline. An empty review on a non-passing case is an unexplained failure.
type ReviewKind string

const (
	ReviewUpstream ReviewKind = "upstream-diagnostic"
	ReviewScope    ReviewKind = "scope-exclusion"
	ReviewGap      ReviewKind = "known-gap"
)

// Report is the deterministic result for one dialect and its corpora.
type Report struct {
	SchemaVersion int             `json:"schemaVersion"`
	Dialect       Dialect         `json:"dialect"`
	Corpora       []CorpusSummary `json:"corpora"`
	Limits        LimitProfiles   `json:"limits"`
	Summary       Summary         `json:"summary"`
	Families      []FamilySummary `json:"families"`
	Cases         []CaseReport    `json:"cases"`
}

// CorpusSummary identifies one corpus and its expected case count.
type CorpusSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// LimitProfiles records both limit configurations exercised for every case.
type LimitProfiles struct {
	Default  LimitValues `json:"default"`
	Explicit LimitValues `json:"explicit"`
}

// LimitValues is the stable JSON representation of source.Limits.
type LimitValues struct {
	MaxSourceBytes     int   `json:"maxSourceBytes"`
	MaxTokens          int   `json:"maxTokens"`
	MaxNesting         int   `json:"maxNesting"`
	MaxNodes           int   `json:"maxNodes"`
	MaxMacroExpansions int   `json:"maxMacroExpansions"`
	MaxExpandedTokens  int   `json:"maxExpandedTokens"`
	MaxCallDepth       int   `json:"maxCallDepth"`
	MaxTableCells      int   `json:"maxTableCells"`
	MaxAssemblyRepeats int   `json:"maxAssemblyRepeats"`
	MaxTextBytes       int   `json:"maxTextBytes"`
	MaxOutputItems     int   `json:"maxOutputItems"`
	MaxPathSegments    int   `json:"maxPathSegments"`
	MaxDimension       int64 `json:"maxDimension"`
}

// Summary conserves all case dispositions.
type Summary struct {
	Total       int `json:"total"`
	Pass        int `json:"pass"`
	Diagnostic  int `json:"diagnostic"`
	Unsupported int `json:"unsupported"`
	Bug         int `json:"bug"`
	Unexplained int `json:"unexplained"`
}

// FamilySummary counts cases containing one explicit command, environment, or
// lexical feature family.
type FamilySummary struct {
	Family string `json:"family"`
	Total  int    `json:"total"`
}

// CaseReport is the stable, source-independent record for one corpus case.
type CaseReport struct {
	ID           string          `json:"id"`
	Corpus       string          `json:"corpus"`
	Index        int             `json:"index"`
	Status       Status          `json:"status"`
	Review       ReviewKind      `json:"review,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Families     []string        `json:"families"`
	Commands     []string        `json:"commands,omitempty"`
	Environments []string        `json:"environments,omitempty"`
	Codes        []source.Code   `json:"codes,omitempty"`
	Diagnostics  []DiagnosticRef `json:"diagnostics,omitempty"`
	Detail       string          `json:"detail,omitempty"`

	structural bool
}

// DiagnosticRef is a compact diagnostic representation. It intentionally
// excludes free-form hints, which are not part of the conformance contract.
type DiagnosticRef struct {
	Code     source.Code     `json:"code"`
	Severity source.Severity `json:"severity"`
	Start    int             `json:"start"`
	End      int             `json:"end"`
	Message  string          `json:"message"`
}

// Key uniquely identifies a report case across corpora.
func (r CaseReport) Key() string {
	return r.Corpus + "/" + r.ID
}

// Validate checks report conservation and review discipline.
func (r *Report) Validate() error {
	if r == nil {
		return errors.New("nil conformance report")
	}
	if r.SchemaVersion != reportSchemaVersion {
		return fmt.Errorf("unsupported report schema %d", r.SchemaVersion)
	}
	if r.Dialect != DialectTeX && r.Dialect != DialectTypst {
		return fmt.Errorf("unsupported dialect %q", r.Dialect)
	}
	if len(r.Corpora) == 0 {
		return errors.New("report has no corpora")
	}

	corpusNames := make(map[string]int, len(r.Corpora))
	corpusTotal := 0
	for _, corpus := range r.Corpora {
		if corpus.Name == "" || corpus.Count <= 0 {
			return fmt.Errorf("invalid corpus summary %#v", corpus)
		}
		if _, ok := corpusNames[corpus.Name]; ok {
			return fmt.Errorf("duplicate corpus %q", corpus.Name)
		}
		corpusNames[corpus.Name] = corpus.Count
		corpusTotal += corpus.Count
	}
	if corpusTotal != r.Summary.Total {
		return fmt.Errorf("corpus count %d does not match summary total %d", corpusTotal, r.Summary.Total)
	}
	if r.Summary.Total != len(r.Cases) {
		return fmt.Errorf("summary total %d does not match %d cases", r.Summary.Total, len(r.Cases))
	}
	if got := r.Summary.Pass + r.Summary.Diagnostic + r.Summary.Unsupported + r.Summary.Bug; got != r.Summary.Total {
		return fmt.Errorf("status counts sum to %d, want %d", got, r.Summary.Total)
	}

	seen := make(map[string]struct{}, len(r.Cases))
	perCorpus := make(map[string]int, len(r.Corpora))
	counts := Summary{Total: len(r.Cases)}
	for i := range r.Cases {
		result := &r.Cases[i]
		key := result.Key()
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate case %q", key)
		}
		seen[key] = struct{}{}
		if _, ok := corpusNames[result.Corpus]; !ok {
			return fmt.Errorf("case %q names unknown corpus %q", key, result.Corpus)
		}
		perCorpus[result.Corpus]++
		if result.Index <= 0 {
			return fmt.Errorf("case %q has invalid index %d", key, result.Index)
		}
		if !validStatus(result.Status) {
			return fmt.Errorf("case %q has invalid status %q", key, result.Status)
		}
		if !slices.IsSorted(result.Families) {
			return fmt.Errorf("case %q families are not sorted", key)
		}
		if len(result.Families) == 0 {
			return fmt.Errorf("case %q has no families", key)
		}
		switch result.Status {
		case StatusPass:
			counts.Pass++
		case StatusDiagnostic:
			counts.Diagnostic++
			if len(result.Diagnostics) == 0 {
				return fmt.Errorf("diagnostic case %q has no diagnostics", key)
			}
		case StatusUnsupported:
			counts.Unsupported++
		case StatusBug:
			counts.Bug++
		}
		if result.Status == StatusPass {
			if result.Review != "" || result.Reason != "" {
				return fmt.Errorf("passing case %q has review metadata", key)
			}
			continue
		}
		if result.Review == "" || strings.TrimSpace(result.Reason) == "" {
			counts.Unexplained++
			return fmt.Errorf("non-passing case %q is unexplained", key)
		}
		switch result.Status {
		case StatusDiagnostic:
			if result.Review != ReviewUpstream && result.Review != ReviewGap {
				return fmt.Errorf("diagnostic case %q has review %q", key, result.Review)
			}
		case StatusUnsupported:
			if result.Review != ReviewScope && result.Review != ReviewGap {
				return fmt.Errorf("unsupported case %q has review %q", key, result.Review)
			}
		case StatusBug:
			if result.Review != ReviewGap {
				return fmt.Errorf("bug case %q is not a reviewed known gap", key)
			}
		}
	}
	if counts.Pass != r.Summary.Pass || counts.Diagnostic != r.Summary.Diagnostic ||
		counts.Unsupported != r.Summary.Unsupported || counts.Bug != r.Summary.Bug {
		return fmt.Errorf("summary status counts do not match cases: got %+v", r.Summary)
	}
	if counts.Unexplained != r.Summary.Unexplained {
		return fmt.Errorf("summary unexplained count %d, want %d", r.Summary.Unexplained, counts.Unexplained)
	}
	for name, count := range corpusNames {
		if perCorpus[name] != count {
			return fmt.Errorf("corpus %q has %d cases, declared %d", name, perCorpus[name], count)
		}
	}
	if err := validateFamilySummaries(r.Families, r.Cases); err != nil {
		return err
	}
	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusPass, StatusDiagnostic, StatusUnsupported, StatusBug:
		return true
	default:
		return false
	}
}

func validateFamilySummaries(summaries []FamilySummary, cases []CaseReport) error {
	want := make(map[string]int)
	for _, result := range cases {
		for _, family := range result.Families {
			want[family]++
		}
	}
	if !slices.IsSortedFunc(summaries, func(a, b FamilySummary) int {
		return cmp.Compare(a.Family, b.Family)
	}) {
		return errors.New("family summaries are not sorted")
	}
	seen := make(map[string]struct{}, len(summaries))
	for _, summary := range summaries {
		if _, ok := seen[summary.Family]; ok {
			return fmt.Errorf("duplicate family summary %q", summary.Family)
		}
		seen[summary.Family] = struct{}{}
		if summary.Total != want[summary.Family] {
			return fmt.Errorf("family %q summary %d, want %d", summary.Family, summary.Total, want[summary.Family])
		}
	}
	if len(seen) != len(want) {
		return fmt.Errorf("family summary has %d entries, want %d", len(seen), len(want))
	}
	return nil
}

// CanonicalJSON returns deterministic reviewed-baseline JSON.
func (r *Report) CanonicalJSON() ([]byte, error) {
	return fixture.CanonicalJSON(r)
}

// LoadReport loads a strict reviewed baseline.
func LoadReport(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var report Report
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := report.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &report, nil
}

// CompareReports returns the first stable difference between two reports.
func CompareReports(want, got *Report) string {
	wantJSON, err := want.CanonicalJSON()
	if err != nil {
		return "encode expected report: " + err.Error()
	}
	gotJSON, err := got.CanonicalJSON()
	if err != nil {
		return "encode actual report: " + err.Error()
	}
	if bytes.Equal(wantJSON, gotJSON) {
		return ""
	}
	wantLines := strings.Split(strings.TrimSuffix(string(wantJSON), "\n"), "\n")
	gotLines := strings.Split(strings.TrimSuffix(string(gotJSON), "\n"), "\n")
	for i := 0; i < max(len(wantLines), len(gotLines)); i++ {
		var wantLine, gotLine string
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if wantLine != gotLine {
			return fmt.Sprintf("report differs at line %d: expected %q, got %q", i+1, wantLine, gotLine)
		}
	}
	return "report differs"
}

func finalizeReport(report *Report) {
	report.SchemaVersion = reportSchemaVersion
	slices.SortFunc(report.Corpora, func(a, b CorpusSummary) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(report.Cases, func(a, b CaseReport) int {
		if c := cmp.Compare(a.Corpus, b.Corpus); c != 0 {
			return c
		}
		return cmp.Compare(a.Index, b.Index)
	})
	report.Summary = Summary{Total: len(report.Cases)}
	familyCounts := make(map[string]int)
	for i := range report.Cases {
		result := &report.Cases[i]
		sortCaseFamilies(result)
		switch result.Status {
		case StatusPass:
			report.Summary.Pass++
		case StatusDiagnostic:
			report.Summary.Diagnostic++
		case StatusUnsupported:
			report.Summary.Unsupported++
		case StatusBug:
			report.Summary.Bug++
		}
		if result.Review == "" && result.Status != StatusPass {
			report.Summary.Unexplained++
		}
		for _, family := range result.Families {
			familyCounts[family]++
		}
	}
	report.Families = report.Families[:0]
	for family, count := range familyCounts {
		report.Families = append(report.Families, FamilySummary{Family: family, Total: count})
	}
	slices.SortFunc(report.Families, func(a, b FamilySummary) int {
		return cmp.Compare(a.Family, b.Family)
	})
}

func sortCaseFamilies(result *CaseReport) {
	result.Families = uniqueSorted(result.Families)
	result.Commands = uniqueSorted(result.Commands)
	result.Environments = uniqueSorted(result.Environments)
	result.Codes = uniqueSorted(result.Codes)
	slices.SortFunc(result.Diagnostics, func(a, b DiagnosticRef) int {
		if c := cmp.Compare(a.Start, b.Start); c != 0 {
			return c
		}
		if c := cmp.Compare(a.End, b.End); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Code, b.Code); c != 0 {
			return c
		}
		return cmp.Compare(a.Message, b.Message)
	})
	result.Diagnostics = slices.CompactFunc(result.Diagnostics, func(a, b DiagnosticRef) bool {
		return a == b
	})
}

func uniqueSorted[T cmp.Ordered](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	out := slices.Clone(values)
	slices.Sort(out)
	return slices.Compact(out)
}

func limitValues(limits source.Limits) LimitValues {
	return LimitValues{
		MaxSourceBytes:     limits.MaxSourceBytes,
		MaxTokens:          limits.MaxTokens,
		MaxNesting:         limits.MaxNesting,
		MaxNodes:           limits.MaxNodes,
		MaxMacroExpansions: limits.MaxMacroExpansions,
		MaxExpandedTokens:  limits.MaxExpandedTokens,
		MaxCallDepth:       limits.MaxCallDepth,
		MaxTableCells:      limits.MaxTableCells,
		MaxAssemblyRepeats: limits.MaxAssemblyRepeats,
		MaxTextBytes:       limits.MaxTextBytes,
		MaxOutputItems:     limits.MaxOutputItems,
		MaxPathSegments:    limits.MaxPathSegments,
		MaxDimension:       limits.MaxDimension,
	}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}

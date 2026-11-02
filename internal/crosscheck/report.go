package crosscheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// Kind classifies the first observable difference in a paired case.
type Kind string

const (
	Pass                     Kind = "pass"
	SemanticIRMismatch       Kind = "semantic-ir-mismatch"
	LayoutMismatch           Kind = "layout-mismatch"
	FixtureMismatch          Kind = "fixture-mismatch"
	DocumentedPairDifference Kind = "documented-pair-difference"
	Unexplained              Kind = "unexplained"
)

func (k Kind) Valid() bool {
	switch k {
	case Pass, SemanticIRMismatch, LayoutMismatch, FixtureMismatch, DocumentedPairDifference, Unexplained:
		return true
	default:
		return false
	}
}

// CaseResult records the complete disposition of one corpus case.
type CaseResult struct {
	ID          string `json:"id"`
	Index       int    `json:"index"`
	Category    string `json:"category"`
	Kind        Kind   `json:"kind"`
	Detail      string `json:"detail,omitempty"`
	KnownReason string `json:"reason,omitempty"`
}

// Report is a deterministic, complete account of a paired corpus.
type Report struct {
	SchemaVersion  int          `json:"schemaVersion"`
	Corpus         string       `json:"corpus"`
	ToleranceDIP   float64      `json:"toleranceDIP"`
	Total          int          `json:"total"`
	Pass           int          `json:"pass"`
	SemanticIR     int          `json:"semanticIRMismatch"`
	Layout         int          `json:"layoutMismatch"`
	Fixture        int          `json:"fixtureMismatch"`
	PairDifference int          `json:"documentedPairDifference"`
	Unexplained    int          `json:"unexplained"`
	Cases          []CaseResult `json:"cases"`
}

// Validate checks conservation and requires every non-pass result to have an
// explicit reviewed reason. It does not compare a caller's expectation file.
func (r Report) Validate() error {
	if r.SchemaVersion != 1 {
		return fmt.Errorf("unsupported report schema %d", r.SchemaVersion)
	}
	if r.Corpus == "" {
		return errors.New("report corpus is empty")
	}
	if r.ToleranceDIP <= 0 {
		return errors.New("report tolerance must be positive")
	}
	if r.Total == 0 || r.Total != len(r.Cases) {
		return fmt.Errorf("report total %d does not match %d cases", r.Total, len(r.Cases))
	}
	counts := make(map[Kind]int, 6)
	seen := make(map[string]struct{}, len(r.Cases))
	for i, result := range r.Cases {
		if result.ID == "" {
			return fmt.Errorf("case %d has empty ID", i)
		}
		if result.Index != i+1 {
			return fmt.Errorf("case %q has index %d, want %d", result.ID, result.Index, i+1)
		}
		if _, ok := seen[result.ID]; ok {
			return fmt.Errorf("duplicate case %q", result.ID)
		}
		seen[result.ID] = struct{}{}
		if !result.Kind.Valid() {
			return fmt.Errorf("case %q has invalid kind %q", result.ID, result.Kind)
		}
		if result.Kind != Pass && strings.TrimSpace(result.KnownReason) == "" {
			return fmt.Errorf("case %q has no known-difference reason", result.ID)
		}
		if result.Kind == Pass && result.Detail != "" {
			return fmt.Errorf("passing case %q has failure detail", result.ID)
		}
		if len(result.Detail) > 2048 {
			return fmt.Errorf("case %q detail is %d bytes, maximum is 2048", result.ID, len(result.Detail))
		}
		counts[result.Kind]++
	}
	checks := []struct {
		name string
		kind Kind
		got  int
	}{
		{"pass", Pass, r.Pass},
		{"semantic IR mismatch", SemanticIRMismatch, r.SemanticIR},
		{"layout mismatch", LayoutMismatch, r.Layout},
		{"fixture mismatch", FixtureMismatch, r.Fixture},
		{"documented pair difference", DocumentedPairDifference, r.PairDifference},
	}
	for _, check := range checks {
		if check.got != counts[check.kind] {
			return fmt.Errorf("%s count is %d, cases contain %d", check.name, check.got, counts[check.kind])
		}
	}
	if r.Unexplained != counts[Unexplained] {
		return fmt.Errorf("unexplained count is %d, cases contain %d", r.Unexplained, counts[Unexplained])
	}
	if r.Unexplained != 0 {
		return fmt.Errorf("report contains %d unexplained cases", r.Unexplained)
	}
	return nil
}

// MarshalCanonical returns stable, indented JSON with a trailing newline.
func (r Report) MarshalCanonical() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// LoadReport decodes and validates a committed baseline.
func LoadReport(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var report Report
	if err := decoder.Decode(&report); err != nil {
		return Report{}, fmt.Errorf("%s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Report{}, fmt.Errorf("%s: trailing JSON value", path)
		}
		return Report{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := report.Validate(); err != nil {
		return Report{}, fmt.Errorf("%s: %w", path, err)
	}
	return report, nil
}

// CompareReports checks exact case order, kinds, and reviewed reasons.
func CompareReports(want, got Report) error {
	if err := want.Validate(); err != nil {
		return fmt.Errorf("expected report: %w", err)
	}
	if err := got.Validate(); err != nil {
		return fmt.Errorf("actual report: %w", err)
	}
	if want.Corpus != got.Corpus || want.Total != got.Total ||
		want.ToleranceDIP != got.ToleranceDIP {
		return fmt.Errorf("report header changed from corpus=%q total=%d tolerance=%g to corpus=%q total=%d tolerance=%g",
			want.Corpus, want.Total, want.ToleranceDIP,
			got.Corpus, got.Total, got.ToleranceDIP)
	}
	for i := range want.Cases {
		w := want.Cases[i]
		g := got.Cases[i]
		if w.ID != g.ID || w.Index != g.Index || w.Category != g.Category {
			return fmt.Errorf("case %d identity changed from %#v to %#v", i+1, w, g)
		}
		if w.Kind != g.Kind {
			return fmt.Errorf("case %q kind changed from %q to %q: %s", w.ID, w.Kind, g.Kind, g.Detail)
		}
		if w.Detail != g.Detail {
			return fmt.Errorf("case %q detail changed from %q to %q", w.ID, w.Detail, g.Detail)
		}
		if w.KnownReason != g.KnownReason {
			return fmt.Errorf("case %q reason changed from %q to %q", w.ID, w.KnownReason, g.KnownReason)
		}
	}
	return nil
}

// knownDifferences maps a case ID to its reviewed reason and expected kind.
type knownDifferences map[string]KnownDifference

type KnownDifference struct {
	Kind   Kind   `json:"kind"`
	Reason string `json:"reason"`
}

func loadKnownDifferences(path string) (knownDifferences, error) {
	if path == "" {
		return knownDifferences{}, nil
	}
	report, err := LoadReport(path)
	if err != nil {
		return nil, err
	}
	out := make(knownDifferences, len(report.Cases))
	for _, result := range report.Cases {
		if result.Kind == Pass {
			continue
		}
		out[result.ID] = KnownDifference{Kind: result.Kind, Reason: result.KnownReason}
	}
	return out, nil
}

func summarize(results []CaseResult) Report {
	report := Report{
		SchemaVersion: 1,
		Corpus:        "typlet-paired",
		ToleranceDIP:  toleranceDIP,
		Total:         len(results),
		Cases:         slices.Clone(results),
	}
	for i := range report.Cases {
		report.Cases[i].Index = i + 1
		switch report.Cases[i].Kind {
		case Pass:
			report.Pass++
		case SemanticIRMismatch:
			report.SemanticIR++
		case LayoutMismatch:
			report.Layout++
		case FixtureMismatch:
			report.Fixture++
		case DocumentedPairDifference:
			report.PairDifference++
		case Unexplained:
			report.Unexplained++
		}
	}
	return report
}

// ReviewedRefresh applies the explicit baseline review policy to observed
// cases. Details may change under an unchanged kind; a kind transition or new
// non-passing case requires a reviewer-supplied reason.
func ReviewedRefresh(results []CaseResult, previous Report, reasons map[string]string) (Report, error) {
	previousCases := make(map[string]CaseResult, len(previous.Cases))
	for _, result := range previous.Cases {
		previousCases[result.ID] = result
	}
	used := make(map[string]bool, len(reasons))
	refreshed := make([]CaseResult, len(results))
	for i, result := range results {
		result.Index = i + 1
		result.KnownReason = ""
		if result.Kind == Pass {
			result.Detail = ""
			refreshed[i] = result
			continue
		}
		if reason, ok := reasons[result.ID]; ok {
			used[result.ID] = true
			if strings.TrimSpace(reason) == "" {
				return Report{}, fmt.Errorf("case %q override reason is blank", result.ID)
			}
			result.KnownReason = reason
			refreshed[i] = result
			continue
		}
		old, ok := previousCases[result.ID]
		if !ok {
			return Report{}, fmt.Errorf("case %q has no previous entry or reviewed reason", result.ID)
		}
		if old.Kind != result.Kind {
			return Report{}, fmt.Errorf("case %q kind changed from %q to %q and requires a reviewed reason",
				result.ID, old.Kind, result.Kind)
		}
		if strings.TrimSpace(old.KnownReason) == "" {
			return Report{}, fmt.Errorf("case %q has no previous reviewed reason", result.ID)
		}
		result.KnownReason = old.KnownReason
		refreshed[i] = result
	}
	for id := range reasons {
		if !used[id] {
			return Report{}, fmt.Errorf("reviewed reason for %q was not needed", id)
		}
	}
	report := summarize(refreshed)
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

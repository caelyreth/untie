package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/caelyreth/untie/source"
)

const policySchemaVersion = 1

// Policy is the reviewed, data-driven set of scope exclusions and known gaps.
// Rules must identify cases by explicit IDs or source families; broad status
// heuristics are intentionally unavailable.
type Policy struct {
	SchemaVersion          int          `json:"schemaVersion"`
	DiagnosticExpectations []PolicyRule `json:"diagnosticExpectations,omitempty"`
	Exclusions             []PolicyRule `json:"exclusions,omitempty"`
	KnownGaps              []PolicyRule `json:"knownGaps,omitempty"`

	usedExpectations []bool
	usedExclusions   []bool
	usedKnownGaps    []bool
}

// PolicyRule matches within one corpus. Fields are ANDed across dimensions and
// ORed within each list.
type PolicyRule struct {
	IDs      []string      `json:"ids,omitempty"`
	Families []string      `json:"families,omitempty"`
	Codes    []source.Code `json:"codes,omitempty"`
	Reason   string        `json:"reason"`
	Corpus   string        `json:"corpus"`
}

// LoadPolicy loads and validates reviewed disposition data.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var policy Policy
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	policy.usedExpectations = make([]bool, len(policy.DiagnosticExpectations))
	policy.usedExclusions = make([]bool, len(policy.Exclusions))
	policy.usedKnownGaps = make([]bool, len(policy.KnownGaps))
	return &policy, nil
}

// Validate checks rule structure before a suite is evaluated.
func (p *Policy) Validate() error {
	if p == nil {
		return errors.New("nil conformance policy")
	}
	if p.SchemaVersion != policySchemaVersion {
		return fmt.Errorf("unsupported policy schema %d", p.SchemaVersion)
	}
	if err := validateRules("exclusion", p.Exclusions, false); err != nil {
		return err
	}
	if err := validateRules("known gap", p.KnownGaps, true); err != nil {
		return err
	}
	if err := validateExpectations(p.DiagnosticExpectations); err != nil {
		return err
	}
	return nil
}

func validateExpectations(rules []PolicyRule) error {
	seen := make(map[string]struct{})
	for i, rule := range rules {
		if rule.Corpus == "" || len(rule.IDs) == 0 || len(rule.Codes) == 0 || rule.Reason == "" {
			return fmt.Errorf("diagnostic expectation %d must have corpus, IDs, codes, and reason", i)
		}
		if duplicate := firstDuplicate(rule.IDs); duplicate != "" {
			return fmt.Errorf("diagnostic expectation %d repeats ID %q", i, duplicate)
		}
		if duplicate := firstDuplicate(rule.Codes); duplicate != "" {
			return fmt.Errorf("diagnostic expectation %d repeats code %q", i, duplicate)
		}
		for _, id := range rule.IDs {
			key := rule.Corpus + "/" + id
			if _, ok := seen[key]; ok {
				return fmt.Errorf("duplicate diagnostic expectation for %q", key)
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func validateRules(kind string, rules []PolicyRule, allowCodesOnly bool) error {
	for i, rule := range rules {
		if rule.Corpus == "" {
			return fmt.Errorf("%s rule %d has no corpus", kind, i)
		}
		if len(rule.IDs) == 0 && len(rule.Families) == 0 && (!allowCodesOnly || len(rule.Codes) == 0) {
			return fmt.Errorf("%s rule %d must name IDs or families", kind, i)
		}
		if len(rule.Reason) == 0 {
			return fmt.Errorf("%s rule %d has no reason", kind, i)
		}
		if duplicate := firstDuplicate(rule.IDs); duplicate != "" {
			return fmt.Errorf("%s rule %d repeats ID %q", kind, i, duplicate)
		}
		if duplicate := firstDuplicate(rule.Families); duplicate != "" {
			return fmt.Errorf("%s rule %d repeats family %q", kind, i, duplicate)
		}
		if duplicate := firstDuplicate(rule.Codes); duplicate != "" {
			return fmt.Errorf("%s rule %d repeats code %q", kind, i, duplicate)
		}
	}
	return nil
}

func firstDuplicate[T comparable](values []T) T {
	var zero T
	seen := make(map[T]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return value
		}
		seen[value] = struct{}{}
	}
	return zero
}

func (p *Policy) apply(result *CaseReport) {
	if p == nil || result.Status == StatusPass {
		return
	}
	if result.Status == StatusDiagnostic && result.Review == ReviewUpstream {
		return
	}
	exclusionIndex, exclusion := p.match(p.Exclusions, result)
	if exclusion != nil && !result.structural && len(result.Diagnostics) > 0 &&
		(result.Status == StatusBug || result.Status == StatusUnsupported) {
		if result.Status == StatusBug {
			result.Status = StatusUnsupported
		}
		result.Review = ReviewScope
		result.Reason = exclusion.Reason
		p.usedExclusions[exclusionIndex] = true
		return
	}
	gapIndex, gap := p.match(p.KnownGaps, result)
	if gap != nil && (result.Status == StatusBug || result.Status == StatusUnsupported) {
		result.Review = ReviewGap
		result.Reason = gap.Reason
		p.usedKnownGaps[gapIndex] = true
	}
}

func (p *Policy) expectedCodes(corpus, id string) ([]source.Code, bool) {
	if p == nil {
		return nil, false
	}
	for i, rule := range p.DiagnosticExpectations {
		if rule.Corpus != corpus || !contains(rule.IDs, id) {
			continue
		}
		p.usedExpectations[i] = true
		return slices.Clone(rule.Codes), true
	}
	return nil, false
}

func (p *Policy) match(rules []PolicyRule, result *CaseReport) (int, *PolicyRule) {
	for i := range rules {
		rule := &rules[i]
		if rule.Corpus != result.Corpus {
			continue
		}
		if len(rule.IDs) > 0 && !contains(rule.IDs, result.ID) {
			continue
		}
		if len(rule.Families) > 0 && !intersects(rule.Families, result.Families) {
			continue
		}
		if len(rule.Codes) > 0 && !intersects(rule.Codes, result.Codes) {
			continue
		}
		return i, rule
	}
	return -1, nil
}

func (p *Policy) unused() error {
	if p == nil {
		return nil
	}
	for i, rule := range p.DiagnosticExpectations {
		if !p.usedExpectations[i] {
			return fmt.Errorf("unused diagnostic expectation for corpus %q IDs %v", rule.Corpus, rule.IDs)
		}
	}
	for i, rule := range p.Exclusions {
		if !p.usedExclusions[i] {
			return fmt.Errorf("unused scope exclusion for corpus %q IDs %v families %v", rule.Corpus, rule.IDs, rule.Families)
		}
	}
	for i, rule := range p.KnownGaps {
		if !p.usedKnownGaps[i] {
			return fmt.Errorf("unused known gap for corpus %q IDs %v families %v", rule.Corpus, rule.IDs, rule.Families)
		}
	}
	return nil
}

func contains[T comparable](values []T, target T) bool {
	return slices.Contains(values, target)
}

func intersects[T comparable](a, b []T) bool {
	for _, value := range a {
		if slices.Contains(b, value) {
			return true
		}
	}
	return false
}

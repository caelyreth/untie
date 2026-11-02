package quality_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/untie/internal/quality/conformance"
)

// Keep this review ratchet separate from the conformance report schema. The
// set and maximum move in lockstep and may only shrink. When a bug is fixed,
// remove its ID, lower the maximum, and refresh the baseline in one review.
const typstKnownBugMaximum = 8

var typstKnownBugIDs = []string{
	"typlet-paired/computed",
	"typlet-paired/loop-matrix",
	"typlet-paired/macros",
	"typlet-syntax/bom",
	"typlet-syntax/escapes",
	"typlet-syntax/number-overflow",
	"typlet-syntax/number-suffixes",
	"typlet-syntax/numbers",
}

func checkTypstBugBudget(report *conformance.Report, maximum int, known []string) error {
	if report == nil {
		return fmt.Errorf("nil Typst conformance report")
	}
	if report.Dialect != conformance.DialectTypst {
		return fmt.Errorf("dialect is %q, want %q", report.Dialect, conformance.DialectTypst)
	}
	if maximum < 0 {
		return fmt.Errorf("invalid Typst known-bug maximum %d", maximum)
	}

	expected := slices.Clone(known)
	slices.Sort(expected)
	if duplicate, ok := firstDuplicateString(expected); ok {
		return fmt.Errorf("Typst known-bug ratchet repeats ID %q", duplicate)
	}
	if maximum != len(expected) {
		return fmt.Errorf(
			"Typst known-bug ratchet is inconsistent: maximum %d, %d known IDs",
			maximum, len(expected),
		)
	}

	actual := make([]string, 0, report.Summary.Bug)
	for _, result := range report.Cases {
		if result.Status != conformance.StatusBug {
			continue
		}
		actual = append(actual, result.Key())
	}
	slices.Sort(actual)
	if duplicate, ok := firstDuplicateString(actual); ok {
		return fmt.Errorf("Typst report repeats bug case %q", duplicate)
	}
	if report.Summary.Bug != len(actual) {
		return fmt.Errorf(
			"Typst known-bug summary is %d, cases contain %d",
			report.Summary.Bug, len(actual),
		)
	}

	missing, unexpected := caseSetDifference(expected, actual)
	if len(actual) > maximum {
		return fmt.Errorf(
			"Typst known-bug budget exceeded: %d bugs, maximum %d; unexpected bug IDs: %v",
			len(actual), maximum, unexpected,
		)
	}
	if len(missing) != 0 || len(unexpected) != 0 {
		return fmt.Errorf(
			"Typst known-bug set changed: unexpected bug IDs: %v, missing bug IDs: %v",
			unexpected, missing,
		)
	}
	return nil
}

func firstDuplicateString(values []string) (string, bool) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return value, true
		}
		seen[value] = struct{}{}
	}
	return "", false
}

func caseSetDifference(expected, actual []string) (missing, unexpected []string) {
	expectedSet := make(map[string]struct{}, len(expected))
	for _, key := range expected {
		expectedSet[key] = struct{}{}
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, key := range actual {
		actualSet[key] = struct{}{}
	}
	for _, key := range expected {
		if _, ok := actualSet[key]; !ok {
			missing = append(missing, key)
		}
	}
	for _, key := range actual {
		if _, ok := expectedSet[key]; !ok {
			unexpected = append(unexpected, key)
		}
	}
	return missing, unexpected
}

func typstBugReport(keys []string) *conformance.Report {
	report := &conformance.Report{
		Dialect: conformance.DialectTypst,
		Summary: conformance.Summary{Total: len(keys), Bug: len(keys)},
		Cases:   make([]conformance.CaseReport, 0, len(keys)),
	}
	for i, key := range keys {
		corpus, id, ok := strings.Cut(key, "/")
		if !ok {
			panic("invalid test case key " + key)
		}
		report.Cases = append(report.Cases, conformance.CaseReport{
			ID: id, Corpus: corpus, Index: i + 1, Status: conformance.StatusBug,
		})
	}
	return report
}

func TestTypstBugBudgetRejectsBugCountIncrease(t *testing.T) {
	t.Parallel()

	keys := append(slices.Clone(typstKnownBugIDs), "typlet-syntax/new-bug")
	err := checkTypstBugBudget(typstBugReport(keys), typstKnownBugMaximum, typstKnownBugIDs)
	if err == nil {
		t.Fatal("added bug was accepted")
	}
	if !strings.Contains(err.Error(), "budget exceeded") ||
		!strings.Contains(err.Error(), "typlet-syntax/new-bug") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTypstBugBudgetRejectsNewBugIDAtSameCount(t *testing.T) {
	t.Parallel()

	keys := slices.Clone(typstKnownBugIDs)
	keys[0] = "typlet-syntax/new-bug"
	err := checkTypstBugBudget(typstBugReport(keys), typstKnownBugMaximum, typstKnownBugIDs)
	if err == nil {
		t.Fatal("new bug ID was accepted")
	}
	if !strings.Contains(err.Error(), "unexpected bug IDs") ||
		!strings.Contains(err.Error(), "typlet-syntax/new-bug") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTypstBugBudgetRequiresRatchetAfterFix(t *testing.T) {
	t.Parallel()

	fixed := "typlet-syntax/bom"
	remaining := slices.DeleteFunc(slices.Clone(typstKnownBugIDs), func(key string) bool {
		return key == fixed
	})

	err := checkTypstBugBudget(typstBugReport(remaining), typstKnownBugMaximum, typstKnownBugIDs)
	if err == nil {
		t.Fatal("fixed bug was accepted without ratcheting the known-bug set")
	}
	if !strings.Contains(err.Error(), "missing bug IDs") ||
		!strings.Contains(err.Error(), fixed) {
		t.Fatalf("unexpected error: %v", err)
	}

	err = checkTypstBugBudget(typstBugReport(remaining), typstKnownBugMaximum, remaining)
	if err == nil || !strings.Contains(err.Error(), "ratchet is inconsistent") {
		t.Fatalf("stale maximum was accepted: %v", err)
	}
	if err := checkTypstBugBudget(
		typstBugReport(remaining), typstKnownBugMaximum-1, remaining,
	); err != nil {
		t.Fatalf("updated ratchet was rejected: %v", err)
	}
}

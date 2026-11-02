// Command dailyreport evaluates the tracked daily-STEM product corpus.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caelyreth/untie/internal/quality/daily"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dailyreport", flag.ContinueOnError)
	flags.SetOutput(stderr)
	corpus := flags.String("corpus", filepath.Join("testdata", "daily-stem", "cases.jsonl"), "daily corpus")
	manifest := flags.String("manifest", filepath.Join("testdata", "daily-stem", "manifest.json"), "corpus manifest")
	font := flags.String("font", filepath.Join("testdata", "upstream", "fonts", "NewCMMath-Book.otf"), "OpenType MATH font")
	timeout := flags.Duration("case-timeout", 2*time.Second, "per-case time budget")
	asJSON := flags.Bool("json", false, "print the complete canonical report")
	allowKnownIssues := flags.Bool("allow-known-issues", false, "diagnostic mode: do not fail on matched tracked issues")
	updateManifest := flags.Bool("update-manifest", false, "explicitly rewrite manifest invariants and content digest after review")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: dailyreport [-corpus FILE] [-manifest FILE] [-font FILE] [-case-timeout DURATION] [-json] [-allow-known-issues] [-update-manifest]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "dailyreport: unexpected positional arguments")
		return 2
	}
	if *updateManifest {
		same, err := sameFileOrPath(*corpus, *manifest)
		if err != nil {
			fail(stderr, err)
			return 1
		}
		if same {
			fail(stderr, errors.New("corpus and manifest resolve to the same file"))
			return 1
		}
	}
	cases, err := daily.LoadCaseFile(*corpus)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	caseFile := filepath.Base(*corpus)
	if *updateManifest {
		updated, err := daily.BuildManifest(cases, caseFile)
		if err != nil {
			fail(stderr, err)
			return 1
		}
		if err := daily.WriteManifest(*manifest, updated); err != nil {
			fail(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "updated manifest: %d cases, %d coverage groups, %d source-specific, digest %s\n",
			updated.Count, updated.CoverageGroups, updated.SourceSpecific, updated.ContentSHA256)
		return 0
	}
	declared, err := daily.LoadManifest(*manifest)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if err := daily.ValidateCorpus(cases, declared); err != nil {
		fail(stderr, err)
		return 1
	}
	if caseFile != declared.CaseFile {
		fail(stderr, fmt.Errorf("corpus basename %q does not match manifest caseFile %q",
			caseFile, declared.CaseFile))
		return 1
	}
	fonts, err := daily.LoadFonts(*font)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	runner, err := daily.NewRunner(fonts, *timeout)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	report, err := runner.Run(context.Background(), cases)
	if report.SchemaVersion != 0 {
		if validationErr := report.Validate(); validationErr != nil {
			fail(stderr, validationErr)
			return 1
		}
	}
	if *asJSON {
		if report.SchemaVersion == 0 {
			fail(stderr, errors.New("runner returned no report"))
			return 1
		}
		data, err := report.MarshalCanonical()
		if err != nil {
			fail(stderr, err)
			return 1
		}
		if _, err := fmt.Fprintln(stdout, string(data)); err != nil {
			fail(stderr, err)
			return 1
		}
	} else {
		if report.SchemaVersion != 0 {
			if err := printSummary(stdout, report); err != nil {
				fail(stderr, err)
				return 1
			}
		}
	}
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if reportExitCode(report, *allowKnownIssues) != 0 {
		if !*allowKnownIssues && report.Summary.KnownIssueFailures != 0 {
			fail(stderr, errors.New("matched tracked issues require -allow-known-issues for a zero exit"))
		} else {
			fail(stderr, errors.New("unexpected corpus failures"))
		}
		return 1
	}
	return 0
}

func printSummary(writer io.Writer, report daily.Report) error {
	summary := report.Summary
	if _, err := fmt.Fprintf(writer,
		"daily STEM: %d cases, %d pass, %d known-issue failures, %d unexpected failures\n",
		summary.Total, summary.Pass, summary.KnownIssueFailures, summary.UnexpectedFailures); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "outcomes: %d rendered, %d recovered, %d missing-glyph, %d layout-error, %d failure\n",
		summary.Rendered, summary.Recovered, summary.MissingGlyph, summary.LayoutError, summary.Failure); err != nil {
		return err
	}
	parts := make([]string, 0, len(report.Categories))
	for _, category := range report.Categories {
		parts = append(parts, fmt.Sprintf("%s=%d/%d", category.Category, category.Pass, category.Total))
	}
	if _, err := fmt.Fprintf(writer, "categories: %s\n", strings.Join(parts, " ")); err != nil {
		return err
	}
	for _, result := range report.KnownIssueCases() {
		if _, err := fmt.Fprintf(writer, "KNOWN ISSUE %s (%s): %s\n",
			result.ID, result.Issue, result.Detail); err != nil {
			return err
		}
	}
	for _, result := range report.UnexpectedFailureCases() {
		if _, err := fmt.Fprintf(writer, "UNEXPECTED FAILURE %s: %s\n", result.ID, result.Detail); err != nil {
			return err
		}
	}
	return nil
}

func reportExitCode(report daily.Report, allowKnownIssues bool) int {
	if report.Blocking(allowKnownIssues) {
		return 1
	}
	return 0
}

func sameFileOrPath(left, right string) (bool, error) {
	leftAbs, err := filepath.Abs(left)
	if err != nil {
		return false, fmt.Errorf("resolve corpus path: %w", err)
	}
	rightAbs, err := filepath.Abs(right)
	if err != nil {
		return false, fmt.Errorf("resolve manifest path: %w", err)
	}
	if filepath.Clean(leftAbs) == filepath.Clean(rightAbs) {
		return true, nil
	}
	leftInfo, err := os.Stat(leftAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat corpus path: %w", err)
	}
	rightInfo, err := os.Stat(rightAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat manifest path: %w", err)
	}
	return os.SameFile(leftInfo, rightInfo), nil
}

func fail(writer io.Writer, err error) {
	_, _ = fmt.Fprintln(writer, "dailyreport:", err)
}

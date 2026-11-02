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

	"github.com/caelyreth/untie/internal/daily"
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
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: dailyreport [-corpus FILE] [-manifest FILE] [-font FILE] [-case-timeout DURATION] [-json]")
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
	cases, err := daily.LoadCaseFile(*corpus)
	if err != nil {
		fail(stderr, err)
		return 1
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
	if filepath.Base(*corpus) != declared.CaseFile {
		fail(stderr, fmt.Errorf("corpus basename %q does not match manifest caseFile %q",
			filepath.Base(*corpus), declared.CaseFile))
		return 1
	}
	engine, err := daily.LoadEngine(*font)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	runner, err := daily.NewRunner(engine, *timeout)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	report, err := runner.Run(context.Background(), cases)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if err := report.Validate(); err != nil {
		fail(stderr, err)
		return 1
	}
	if *asJSON {
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
		if err := printSummary(stdout, report); err != nil {
			fail(stderr, err)
			return 1
		}
	}
	if report.Summary.Fail != 0 {
		return 1
	}
	return 0
}

func printSummary(writer io.Writer, report daily.Report) error {
	summary := report.Summary
	if _, err := fmt.Fprintf(writer, "daily STEM: %d cases, %d pass, %d fail\n",
		summary.Total, summary.Pass, summary.Fail); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "outcomes: %d rendered, %d recovered, %d missing-glyph, %d layout-error, %d failure\n",
		summary.Rendered, summary.Recovered, summary.MissingGlyph, summary.LayoutError, summary.Failure); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "known issues: %d\n", summary.KnownIssues); err != nil {
		return err
	}
	parts := make([]string, 0, len(report.Categories))
	for _, category := range report.Categories {
		parts = append(parts, fmt.Sprintf("%s=%d/%d", category.Category, category.Pass, category.Total))
	}
	if _, err := fmt.Fprintf(writer, "categories: %s\n", strings.Join(parts, " ")); err != nil {
		return err
	}
	for _, result := range report.Cases {
		if result.Pass {
			continue
		}
		if _, err := fmt.Fprintf(writer, "FAIL %s: %s\n", result.ID, result.Detail); err != nil {
			return err
		}
	}
	return nil
}

func fail(writer io.Writer, err error) {
	_, _ = fmt.Fprintln(writer, "dailyreport:", err)
}

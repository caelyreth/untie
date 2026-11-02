// Command typstreport evaluates every imported selected Typlet Typst corpus.
//
// Tests never call this command with -write. Regeneration is an explicit review
// action and must never happen as part of a normal test run.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/caelyreth/untie/internal/conformance"
	"github.com/caelyreth/untie/internal/fixture"
)

func main() {
	root := flag.String("root", "", "fixture root (default: discover testdata/upstream)")
	policy := flag.String("policy", "", "reviewed expected-diagnostic, scope, and gap policy")
	write := flag.String("write", "", "explicitly write the baseline to this path")
	caseTimeout := flag.Duration("case-timeout", time.Second, "per-frontend-call time budget")
	worker := flag.Bool("worker", false, "run the internal isolated Typst worker")
	flag.Parse()
	if *worker {
		if err := conformance.RunWorker(conformance.ParseTypst, os.Stdin, os.Stdout); err != nil {
			fatal(err)
		}
		return
	}

	fixtureRoot := *root
	if fixtureRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fatal(err)
		}
		fixtureRoot, err = fixture.CorpusRoot(cwd)
		if err != nil {
			fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		fatal(err)
	}
	report, err := conformance.BuildTypstReport(context.Background(), conformance.SuiteOptions{
		FixtureRoot: fixtureRoot,
		PolicyPath:  *policy,
		Frontend: conformance.ProcessFrontend(conformance.ProcessConfig{
			Path: executable,
			Args: []string{"-worker"},
		}),
		CaseTimeout: *caseTimeout,
	})
	if err != nil {
		fatal(err)
	}
	if err := report.Validate(); err != nil {
		fatal(err)
	}
	data, err := report.CanonicalJSON()
	if err != nil {
		fatal(err)
	}
	if *write != "" {
		if err := writeAtomic(*write, data); err != nil {
			fatal(err)
		}
	} else {
		if _, err := os.Stdout.Write(data); err != nil {
			fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "cases=%d pass=%d diagnostic=%d unsupported=%d bug=%d unexplained=%d\n",
		report.Summary.Total, report.Summary.Pass, report.Summary.Diagnostic,
		report.Summary.Unsupported, report.Summary.Bug, report.Summary.Unexplained)
}

func writeAtomic(path string, data []byte) error {
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "typstreport:", err)
	os.Exit(1)
}

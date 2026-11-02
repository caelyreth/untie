package quality_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/caelyreth/untie/internal/quality/conformance"
)

const qualityWorkerEnv = "UNTIE_QUALITY_CONFORMANCE_WORKER"

var (
	updateDailyManifest      = flag.Bool("update-daily-manifest", false, "rewrite the reviewed daily-STEM manifest")
	updateTeXBaseline        = flag.Bool("update-tex-baseline", false, "rewrite the reviewed TeX conformance baseline")
	updateTypstBaseline      = flag.Bool("update-typst-baseline", false, "rewrite the reviewed Typst conformance baseline")
	updateCrosscheckBaseline = flag.Bool("update-crosscheck-baseline", false, "rewrite the reviewed paired crosscheck baseline")
	crosscheckReasonsPath    = flag.String("crosscheck-reasons", "", "JSON reasons for crosscheck kind changes during update")
)

func TestMain(m *testing.M) {
	if runConformanceWorker() {
		return
	}
	os.Exit(m.Run())
}

func qualityProcessFrontend(dialect string) conformance.FrontendFunc {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return conformance.ProcessFrontend(conformance.ProcessConfig{
		Path: executable,
		Env:  []string{qualityWorkerEnv + "=" + dialect},
	})
}

func runConformanceWorker() bool {
	dialect := os.Getenv(qualityWorkerEnv)
	var frontend conformance.FrontendFunc
	switch dialect {
	case "":
		return false
	case "tex":
		frontend = conformance.ParseTeX
	case "typst":
		frontend = conformance.ParseTypst
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown quality worker dialect %q\n", dialect)
		os.Exit(2)
	}
	if err := conformance.RunWorker(frontend, os.Stdin, os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "quality conformance worker: %v\n", err)
		os.Exit(2)
	}
	os.Exit(0)
	return true
}

func repositoryRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func writeReviewedFile(path string, data []byte) error {
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

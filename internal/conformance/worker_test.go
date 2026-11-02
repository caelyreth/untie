package conformance

import (
	"fmt"
	"os"
	"testing"
)

const texWorkerEnv = "UNTIE_CONFORMANCE_TEX_WORKER"

func TestMain(m *testing.M) {
	if runTestWorkerIfRequested() {
		return
	}
	os.Exit(m.Run())
}

func testProcessFrontend() FrontendFunc {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return ProcessFrontend(ProcessConfig{
		Path: executable,
		Env:  []string{texWorkerEnv + "=1"},
	})
}

func runTestWorkerIfRequested() bool {
	if os.Getenv(texWorkerEnv) != "1" {
		return false
	}
	if err := RunWorker(ParseTeX, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "conformance worker:", err)
		os.Exit(2)
	}
	os.Exit(0)
	return true
}

package conformance

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/untie/internal/quality/fixture"
	"github.com/caelyreth/untie/internal/tex"
	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

func TestTeXCorpusTerminatesInProcess(t *testing.T) {
	packageRoot := conformancePackageRoot(t)
	root := filepath.Join(packageRoot, "..", "..", "..", "testdata", "upstream")
	manifest, err := fixture.LoadCorpusManifest(filepath.Join(root, "manifests", "corpora.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"ratex-core",
		"ratex-chemistry",
		"ratex-physics",
		"ratex-centernot",
		"ratex-slashed",
		"ratex-prooftree",
		"katex-cases",
	} {
		var spec fixture.CorpusSpec
		for _, candidate := range manifest.Corpora {
			if candidate.Name == name {
				spec = candidate
				break
			}
		}
		if spec.Name == "" {
			t.Fatalf("corpus %q is missing", name)
		}
		corpus, err := fixture.LoadCorpus(root, spec)
		if err != nil {
			t.Fatal(err)
		}
		for _, testCase := range corpus.Cases {
			testCase := testCase
			t.Run(name+"/"+testCase.ID, func(t *testing.T) {
				done := make(chan struct{})
				go func() {
					defer close(done)
					_ = tex.Parse(testCase.Source, ir.Display, tex.Options{Limits: source.DefaultLimits()})
				}()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatalf("in-process parse did not terminate for %s/%s", name, testCase.ID)
				}
			})
		}
	}
}

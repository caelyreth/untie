package untie_test

import (
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

type listedPackage struct {
	ImportPath string
	Imports    []string
}

func TestArchitectureImportRules(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "list", "-json", "./...")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var pkg listedPackage
		if err := dec.Decode(&pkg); err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		checkImports(t, pkg.ImportPath, pkg.Imports)
	}
}

func checkImports(t *testing.T, pkg string, imports []string) {
	t.Helper()

	forbiddenExternal := []string{
		"fyne.io/",
		"gioui.org/",
		"github.com/go-gl/",
		"github.com/hajimehoshi/ebiten",
		"github.com/veandco/go-sdl2/",
	}
	for _, imp := range imports {
		for _, prefix := range forbiddenExternal {
			if strings.HasPrefix(imp, prefix) {
				t.Errorf("%s imports forbidden UI/platform package %s", pkg, imp)
			}
		}
	}

	switch {
	case pkg == "github.com/caelyreth/untie/source" || pkg == "github.com/caelyreth/untie/geometry":
		for _, imp := range imports {
			if strings.Contains(strings.Split(imp, "/")[0], ".") {
				t.Errorf("%s imports non-standard package %s", pkg, imp)
			}
		}
	case pkg == "github.com/caelyreth/untie/ir":
		rejectPrefixes(t, pkg, imports,
			"github.com/caelyreth/untie/font",
			"github.com/caelyreth/untie/display",
			"github.com/caelyreth/untie/internal/",
		)
	case pkg == "github.com/caelyreth/untie/font":
		rejectPrefixes(t, pkg, imports,
			"github.com/caelyreth/untie/ir",
			"github.com/caelyreth/untie/display",
			"github.com/caelyreth/untie/internal/",
		)
	case pkg == "github.com/caelyreth/untie/internal/layout" || strings.HasPrefix(pkg, "github.com/caelyreth/untie/internal/layout/"):
		rejectPrefixes(t, pkg, imports,
			"github.com/caelyreth/untie/internal/tex",
			"github.com/caelyreth/untie/internal/typst",
		)
	case pkg == "github.com/caelyreth/untie/internal/tex" || strings.HasPrefix(pkg, "github.com/caelyreth/untie/internal/tex/") ||
		pkg == "github.com/caelyreth/untie/internal/typst" || strings.HasPrefix(pkg, "github.com/caelyreth/untie/internal/typst/"):
		rejectPrefixes(t, pkg, imports,
			"github.com/caelyreth/untie/font",
			"github.com/caelyreth/untie/display",
			"github.com/caelyreth/untie/internal/layout",
		)
	case pkg == "github.com/caelyreth/untie/display":
		rejectPrefixes(t, pkg, imports,
			"github.com/caelyreth/untie/font",
			"github.com/caelyreth/untie/internal/",
		)
	}
}

func rejectPrefixes(t *testing.T, pkg string, imports []string, prefixes ...string) {
	t.Helper()
	slices.Sort(prefixes)
	for _, imp := range imports {
		for _, prefix := range prefixes {
			if imp == prefix || strings.HasPrefix(imp, prefix+"/") {
				t.Errorf("%s imports forbidden package %s", pkg, imp)
			}
		}
	}
}

package quality_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/caelyreth/untie"

type listedPackage struct {
	ImportPath   string
	ForTest      string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

type packageRules struct {
	Production []string
	Tests      []string
}

// This is an exact allowlist for every project package. Standard-library
// imports are permitted by convention; every non-standard import must appear
// here. Test-only edges are additional to Production and never grant the same
// edge to production code. Keep the table small and make dependency changes
// visible in review.
var allowedProjectImports = map[string]packageRules{
	modulePath: {
		Production: []string{
			modulePath + "/display",
			modulePath + "/font",
			modulePath + "/internal/alt",
			modulePath + "/internal/engine",
			modulePath + "/internal/tex",
			modulePath + "/internal/typst",
			modulePath + "/ir",
			modulePath + "/source",
		},
		Tests: []string{modulePath, modulePath + "/geometry"},
	},
	modulePath + "/display": {
		Production: []string{modulePath + "/geometry", modulePath + "/source"},
	},
	modulePath + "/font": {
		Production: []string{
			"github.com/go-opentype/opentype",
			"github.com/go-text/typesetting/font",
			"github.com/go-text/typesetting/font/opentype",
			modulePath + "/geometry",
		},
		Tests: []string{"github.com/go-text/typesetting/font/opentype"},
	},
	modulePath + "/geometry": {},
	modulePath + "/internal/engine": {
		Production: []string{
			modulePath + "/display",
			modulePath + "/font",
			modulePath + "/geometry",
			modulePath + "/internal/layout",
			modulePath + "/internal/mathdata",
			modulePath + "/ir",
			modulePath + "/source",
		},
		Tests: []string{modulePath + "/internal/tex", modulePath + "/internal/typst"},
	},
	modulePath + "/internal/alt": {
		Production: []string{
			modulePath + "/display",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/internal/quality": {
		Tests: []string{
			modulePath,
			modulePath + "/internal/quality/conformance",
			modulePath + "/internal/quality/crosscheck",
			modulePath + "/internal/quality/daily",
		},
	},
	modulePath + "/internal/quality/conformance": {
		Production: []string{
			modulePath + "/internal/quality/fixture",
			modulePath + "/internal/tex",
			modulePath + "/internal/typst",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/internal/quality/crosscheck": {
		Production: []string{
			modulePath,
			modulePath + "/display",
			modulePath + "/font",
			modulePath + "/geometry",
			modulePath + "/internal/engine",
			modulePath + "/internal/quality/fixture",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/internal/quality/daily": {
		Production: []string{modulePath, modulePath + "/ir", modulePath + "/source"},
	},
	modulePath + "/internal/quality/fixture": {
		Production: []string{modulePath + "/geometry", modulePath + "/source"},
	},
	modulePath + "/internal/layout": {
		Production: []string{
			modulePath + "/display",
			modulePath + "/geometry",
			modulePath + "/ir",
			modulePath + "/source",
		},
		Tests: []string{modulePath + "/internal/layout"},
	},
	modulePath + "/internal/mathdata":     {},
	modulePath + "/internal/mathdata/gen": {},
	modulePath + "/internal/tex": {
		Production: []string{
			modulePath + "/geometry",
			modulePath + "/internal/mathdata",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/internal/typst": {
		Production: []string{
			modulePath + "/geometry",
			modulePath + "/internal/mathdata",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/ir": {
		Production: []string{modulePath + "/source"},
	},
	modulePath + "/source": {},
}

func TestArchitectureImportRules(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "list", "-test", "-json", modulePath+"/...")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -test: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	seen := make(map[string]bool)
	for dec.More() {
		var pkg listedPackage
		if err := dec.Decode(&pkg); err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		// go list -test emits synthetic test-main packages. The real package
		// records include TestImports and XTestImports, so those are sufficient.
		if pkg.ForTest != "" || strings.HasSuffix(pkg.ImportPath, ".test") {
			continue
		}
		seen[pkg.ImportPath] = true
		for _, violation := range packageImportViolations(pkg.ImportPath, pkg.Imports, false, allowedProjectImports) {
			t.Error(violation)
		}
		testImports := append([]string(nil), pkg.TestImports...)
		testImports = append(testImports, pkg.XTestImports...)
		for _, violation := range packageImportViolations(pkg.ImportPath, testImports, true, allowedProjectImports) {
			t.Error(violation)
		}
	}
	for pkg := range allowedProjectImports {
		if !seen[pkg] {
			t.Errorf("allowlist package %s was not returned by go list", pkg)
		}
	}
}

func TestArchitectureRuleDetectsViolation(t *testing.T) {
	t.Parallel()

	violations := packageImportViolations(modulePath+"/font", []string{
		modulePath + "/ir",
		"github.com/example/ui-toolkit",
	}, false, allowedProjectImports)
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want two", violations)
	}
	if got := packageImportViolations(modulePath+"/font", []string{modulePath + "/geometry"}, false, allowedProjectImports); len(got) != 0 {
		t.Fatalf("font -> geometry rejected: %v", got)
	}
	if got := packageImportViolations(modulePath+"/font", []string{modulePath + "/internal/mathdata"}, false, allowedProjectImports); len(got) == 0 {
		t.Fatal("production-only import rule accepted internal/mathdata")
	}
	if got := packageImportViolations(modulePath+"/font", []string{modulePath + "/internal/mathdata"}, true, allowedProjectImports); len(got) == 0 {
		t.Fatal("test rule accepted an unused font -> internal/mathdata edge")
	}
	if got := packageImportViolations(modulePath+"/internal/engine", []string{modulePath + "/internal/tex"}, false, allowedProjectImports); len(got) == 0 {
		t.Fatal("test-only engine -> tex edge leaked into production rules")
	}
}

func packageImportViolations(pkg string, imports []string, test bool, allow map[string]packageRules) []string {
	rules, known := allow[pkg]
	if !known {
		return []string{fmt.Sprintf("%s is not in the architecture allowlist", pkg)}
	}
	allowed := rules.Production
	if test {
		allowed = append(append([]string(nil), allowed...), rules.Tests...)
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, imp := range allowed {
		allowedSet[imp] = true
	}
	slices.Sort(imports)
	var violations []string
	for _, imp := range imports {
		if imp == "C" {
			violations = append(violations, fmt.Sprintf("%s imports cgo", pkg))
			continue
		}
		if allowedSet[imp] {
			continue
		}
		if !strings.HasPrefix(imp, modulePath+"/") && imp != modulePath {
			first := strings.SplitN(imp, "/", 2)[0]
			if strings.Contains(first, ".") {
				violations = append(violations, fmt.Sprintf("%s imports external package %s", pkg, imp))
			}
			continue
		}
		violations = append(violations, fmt.Sprintf("%s imports %s outside its exact allowlist", pkg, imp))
	}
	return violations
}

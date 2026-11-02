package untie_test

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

// This is an exact allowlist for every project package. Standard-library
// imports are permitted by convention; every non-standard import must appear
// here. Keep the table small and make dependency changes visible in review.
var allowedProjectImports = map[string][]string{
	modulePath: {
		modulePath + "/display",
		modulePath + "/font",
		modulePath + "/internal/engine",
		modulePath + "/internal/tex",
		modulePath + "/internal/typst",
		modulePath + "/ir",
		modulePath + "/source",
	},
	modulePath + "/cmd/formula-check": {
		modulePath,
		modulePath + "/ir",
	},
	modulePath + "/cmd/formula-preview": {
		modulePath,
		modulePath + "/internal/testrender",
	},
	modulePath + "/cmd/crosscheck": {
		modulePath + "/internal/crosscheck",
		modulePath + "/internal/fixture",
	},
	modulePath + "/display": {
		modulePath + "/geometry",
		modulePath + "/source",
	},
	modulePath + "/font": {
		"github.com/go-opentype/opentype",
		"github.com/go-text/typesetting/font",
		"github.com/go-text/typesetting/font/opentype",
		modulePath + "/geometry",
		modulePath + "/internal/mathdata",
		modulePath + "/source",
	},
	modulePath + "/geometry": nil,
	modulePath + "/internal/engine": {
		modulePath + "/display",
		modulePath + "/font",
		modulePath + "/geometry",
		modulePath + "/internal/layout",
		modulePath + "/internal/mathdata",
		modulePath + "/internal/tex",
		modulePath + "/internal/typst",
		modulePath + "/ir",
		modulePath + "/source",
	},
	modulePath + "/internal/conformance": {
		modulePath + "/internal/fixture",
		modulePath + "/internal/tex",
		modulePath + "/internal/typst",
		modulePath + "/ir",
		modulePath + "/source",
	},
	modulePath + "/internal/conformance/cmd/texreport": {
		modulePath + "/internal/conformance",
		modulePath + "/internal/fixture",
	},
	modulePath + "/internal/conformance/cmd/typstreport": {
		modulePath + "/internal/conformance",
		modulePath + "/internal/fixture",
	},
	modulePath + "/internal/crosscheck": {
		modulePath,
		modulePath + "/display",
		modulePath + "/font",
		modulePath + "/internal/engine",
		modulePath + "/internal/fixture",
		modulePath + "/ir",
		modulePath + "/source",
	},
	modulePath + "/internal/fixture": {
		modulePath + "/geometry",
		modulePath + "/source",
	},
	modulePath + "/internal/layout": {
		modulePath + "/internal/layout",
		modulePath + "/display",
		modulePath + "/geometry",
		modulePath + "/ir",
		modulePath + "/source",
	},
	modulePath + "/internal/mathdata":     nil,
	modulePath + "/internal/mathdata/gen": nil,
	modulePath + "/internal/testrender": {
		modulePath + "/display",
		modulePath + "/geometry",
	},
	modulePath + "/internal/tex": {
		modulePath + "/geometry",
		modulePath + "/internal/mathdata",
		modulePath + "/ir",
		modulePath + "/source",
	},
	modulePath + "/internal/typst": {
		modulePath + "/geometry",
		modulePath + "/internal/mathdata",
		modulePath + "/ir",
		modulePath + "/source",
	},
	modulePath + "/ir": {
		modulePath + "/source",
	},
	modulePath + "/source": nil,
}

func TestArchitectureImportRules(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "list", "-test", "-json", "./...")
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
		imports := append([]string(nil), pkg.Imports...)
		imports = append(imports, pkg.TestImports...)
		imports = append(imports, pkg.XTestImports...)
		for _, violation := range packageImportViolations(pkg.ImportPath, imports, allowedProjectImports) {
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
	}, allowedProjectImports)
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want two", violations)
	}
	if got := packageImportViolations(modulePath+"/font", []string{modulePath + "/internal/mathdata"}, allowedProjectImports); len(got) != 0 {
		t.Fatalf("font -> internal/mathdata rejected: %v", got)
	}
}

func packageImportViolations(pkg string, imports []string, allow map[string][]string) []string {
	allowed, known := allow[pkg]
	if !known {
		return []string{fmt.Sprintf("%s is not in the architecture allowlist", pkg)}
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

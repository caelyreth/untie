package quality_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/caelyreth/untie"

const exampleModulePath = modulePath + "/examples/mygo"

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

type importViolationFunc func(string, []string, bool, map[string]packageRules) []string

type architectureModule struct {
	Name       string
	Path       string
	Dir        string
	Allowed    map[string]packageRules
	Violations importViolationFunc
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
			modulePath + "/geometry",
			modulePath + "/internal/alt",
			modulePath + "/internal/engine",
			modulePath + "/internal/layout",
			modulePath + "/internal/tex",
			modulePath + "/internal/typst",
			modulePath + "/ir",
			modulePath + "/source",
		},
		Tests: []string{modulePath},
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
			modulePath + "/internal/quality/fixture",
			modulePath + "/source",
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
	},
	modulePath + "/internal/mathdata":     {},
	modulePath + "/internal/mathdata/gen": {},
	modulePath + "/internal/tex": {
		Production: []string{
			modulePath + "/internal/mathdata",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/internal/typst": {
		Production: []string{
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

// examples/mygo is a separate consumer module. It may import Untie's public
// API, packages from its own module and the exact third-party packages declared
// for the example. Untie internals are rejected explicitly below; relying only
// on Go's internal-package rule would incorrectly permit this module because it
// shares Untie's module-path prefix.
var allowedExampleImports = map[string]packageRules{
	exampleModulePath: {
		Production: []string{
			modulePath,
			"github.com/egoist/mygo",
			"github.com/egoist/mygo/ui",
		},
	},
}

var architectureModules = []architectureModule{
	{
		Name:       "root",
		Path:       modulePath,
		Dir:        ".",
		Allowed:    allowedProjectImports,
		Violations: packageImportViolations,
	},
	{
		Name:       "examples/mygo",
		Path:       exampleModulePath,
		Dir:        filepath.Join("examples", "mygo"),
		Allowed:    allowedExampleImports,
		Violations: examplePackageImportViolations,
	},
}

func TestArchitectureImportRules(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for _, module := range architectureModules {
		module := module
		t.Run(module.Name, func(t *testing.T) {
			checkArchitectureModule(t, filepath.Join(root, module.Dir), module)
		})
	}
}

func checkArchitectureModule(t *testing.T, dir string, module architectureModule) {
	t.Helper()

	cmd := exec.Command("go", "list", "-test", "-json", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -test in %s: %v\n%s", dir, err, out)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
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
		for _, violation := range module.Violations(pkg.ImportPath, pkg.Imports, false, module.Allowed) {
			t.Error(violation)
		}
		testImports := append([]string(nil), pkg.TestImports...)
		testImports = append(testImports, pkg.XTestImports...)
		for _, violation := range module.Violations(pkg.ImportPath, testImports, true, module.Allowed) {
			t.Error(violation)
		}
		checkRuleFreshness(t, pkg.ImportPath, pkg.Imports, testImports, module.Allowed)
	}
	for pkg := range module.Allowed {
		if !seen[pkg] {
			t.Errorf("allowlist package %s in module %s was not returned by go list", pkg, module.Path)
		}
	}
}

func checkRuleFreshness(t *testing.T, pkg string, productionImports, testImports []string, allow map[string]packageRules) {
	t.Helper()

	rules := allow[pkg]
	production := importSet(productionImports)
	tests := importSet(testImports)
	for _, grant := range rules.Production {
		if !production[grant] {
			t.Errorf("%s has unused production allowlist grant %s", pkg, grant)
		}
	}
	for _, grant := range rules.Tests {
		if production[grant] {
			t.Errorf("%s repeats production grant %s in test-only rules", pkg, grant)
		}
		if !tests[grant] {
			t.Errorf("%s has unused test-only allowlist grant %s", pkg, grant)
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

func TestArchitectureExamplePolicyRejectsInternal(t *testing.T) {
	t.Parallel()

	internalImport := modulePath + "/internal/engine"
	allow := map[string]packageRules{
		exampleModulePath: {Production: []string{internalImport}},
	}
	violations := examplePackageImportViolations(exampleModulePath, []string{internalImport}, false, allow)
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want one internal-import violation", violations)
	}
	if !strings.Contains(violations[0], "internal") {
		t.Fatalf("violation = %q, want an internal-import diagnostic", violations[0])
	}

	allowed := []string{
		modulePath,
		exampleModulePath + "/futurepkg",
		"github.com/egoist/mygo/ui",
	}
	if got := examplePackageImportViolations(exampleModulePath, allowed, false, allowedExampleImports); len(got) != 0 {
		t.Fatalf("allowed example imports rejected: %v", got)
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

func examplePackageImportViolations(pkg string, imports []string, test bool, allow map[string]packageRules) []string {
	rules, known := allow[pkg]
	if !known {
		return []string{fmt.Sprintf("%s is not in the example architecture allowlist", pkg)}
	}
	allowed := rules.Production
	if test {
		allowed = append(append([]string(nil), allowed...), rules.Tests...)
	}
	allowedSet := importSet(allowed)
	slices.Sort(imports)
	var violations []string
	for _, imp := range imports {
		if imp == "C" {
			violations = append(violations, fmt.Sprintf("%s imports cgo", pkg))
			continue
		}
		if untieInternalImport(imp) {
			violations = append(violations, fmt.Sprintf("%s imports Untie internal package %s", pkg, imp))
			continue
		}
		if withinModule(imp, exampleModulePath) || allowedSet[imp] {
			continue
		}
		if withinModule(imp, modulePath) {
			violations = append(violations, fmt.Sprintf("%s imports non-public or undeclared Untie package %s", pkg, imp))
			continue
		}
		first := strings.SplitN(imp, "/", 2)[0]
		if strings.Contains(first, ".") {
			violations = append(violations, fmt.Sprintf("%s imports undeclared external package %s", pkg, imp))
		}
	}
	return violations
}

func untieInternalImport(imp string) bool {
	rest, ok := strings.CutPrefix(imp, modulePath+"/")
	if !ok {
		return false
	}
	return rest == "internal" ||
		strings.HasPrefix(rest, "internal/") ||
		strings.Contains(rest, "/internal/") ||
		strings.HasSuffix(rest, "/internal")
}

func withinModule(imp, path string) bool {
	return imp == path || strings.HasPrefix(imp, path+"/")
}

func importSet(imports []string) map[string]bool {
	set := make(map[string]bool, len(imports))
	for _, imp := range imports {
		set[imp] = true
	}
	return set
}

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

const contribModulePath = modulePath + "/contrib/mygo"

type listedPackage struct {
	ImportPath   string
	ForTest      string
	CgoFiles     []string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

type packageRules struct {
	Production []string
	Tests      []string
}

type importViolationFunc func(string, []string, bool, map[string]packageRules, stdlibPolicy) []string

// stdlibPolicy rejects a small set of standard-library trees instead of
// maintaining a package-by-package allowlist for the rest of the standard
// library. Forbidden trees either bypass Go's memory or portability guarantees,
// launch processes, install process-wide handlers, open network services, or
// exist only for tests. Platform-specific UI code is additionally blocked by
// the dotted-module rule and the exact external dependency allowlists.
//
// Exceptions are exact package/import pairs with a reason. They must be kept
// fresh by the architecture test so temporary needs cannot silently spread.
type stdlibPolicy struct {
	ProductionForbidden  []string
	TestForbidden        []string
	ProductionExceptions map[string]map[string]string
}

type architectureModule struct {
	Name       string
	Path       string
	Dir        string
	Allowed    map[string]packageRules
	Stdlib     stdlibPolicy
	Violations importViolationFunc
}

var untieStdlibPolicy = stdlibPolicy{
	ProductionForbidden: []string{
		"net",           // Rendering must not create a network side channel.
		"os/exec",       // Process execution belongs to host tooling.
		"os/signal",     // Libraries must not own process-wide signal policy.
		"plugin",        // Dynamic loading breaks portable static builds.
		"runtime/cgo",   // The renderer must remain pure Go.
		"runtime/pprof", // Profiling belongs to the embedding application.
		"runtime/trace", // Tracing belongs to the embedding application.
		"syscall",       // Includes platform-specific escapes such as syscall/js.
		"testing",       // Test dependencies must not enter production packages.
		"unsafe",        // Keep the public safety and portability boundary intact.
	},
	TestForbidden: []string{
		"plugin",
		"runtime/cgo",
		"syscall",
		"unsafe",
	},
	ProductionExceptions: map[string]map[string]string{
		modulePath + "/internal/quality/conformance": {
			"os/exec": "external reference adapters are launched only by the conformance harness",
		},
	},
}

var exampleStdlibPolicy = stdlibPolicy{
	ProductionForbidden: untieStdlibPolicy.ProductionForbidden,
	TestForbidden:       untieStdlibPolicy.TestForbidden,
}

// This is an exact allowlist for every project package's non-standard imports.
// Standard-library imports are governed separately by untieStdlibPolicy. Every
// non-standard import must appear here. Test-only edges are additional to
// Production and never grant the same edge to production code. Keep the table
// small and make dependency changes visible in review.
var allowedProjectImports = map[string]packageRules{
	modulePath: {
		Production: []string{
			modulePath + "/display",
			modulePath + "/font",
			modulePath + "/geometry",
			modulePath + "/internal/alt",
			modulePath + "/internal/engine",
			modulePath + "/internal/irarena",
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
	modulePath + "/encode/raster": {
		Production: []string{
			"golang.org/x/image/vector",
			modulePath + "/display",
			modulePath + "/geometry",
			modulePath + "/render",
		},
		Tests: []string{modulePath + "/encode/raster"},
	},
	modulePath + "/encode/svg": {
		Production: []string{
			modulePath + "/display",
			modulePath + "/geometry",
			modulePath + "/render",
		},
		Tests: []string{modulePath + "/encode/svg"},
	},
	modulePath + "/font": {
		Production: []string{
			"github.com/go-text/typesetting/font",
			"github.com/go-text/typesetting/font/opentype",
			modulePath + "/geometry",
		},
	},
	modulePath + "/fontsource": {
		Production: []string{modulePath + "/font"},
		Tests:      []string{modulePath + "/fontsource"},
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
	modulePath + "/internal/irarena": {
		Production: []string{modulePath + "/ir", modulePath + "/source"},
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
			modulePath + "/internal/irarena",
			modulePath + "/internal/mathdata",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/internal/typst": {
		Production: []string{
			modulePath + "/internal/irarena",
			modulePath + "/internal/mathdata",
			modulePath + "/ir",
			modulePath + "/source",
		},
	},
	modulePath + "/ir": {
		Production: []string{modulePath + "/source"},
	},
	modulePath + "/render": {
		Production: []string{modulePath + "/display", modulePath + "/geometry"},
		Tests:      []string{modulePath + "/render"},
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
			modulePath + "/contrib/mygo",
			modulePath + "/render",
			"github.com/egoist/mygo",
			"github.com/egoist/mygo/ui",
		},
	},
}

// contrib/mygo is a separately versionable integration module. It may import
// only the public Untie packages it uses and MyGo packages it integrates with.
// Keeping these grants exact prevents framework dependencies from leaking into
// the root module and prevents accidental use of either project's internals.
var allowedContribImports = map[string]packageRules{
	contribModulePath: {
		Production: []string{
			modulePath,
			modulePath + "/render",
			"github.com/egoist/mygo/ui",
			"github.com/egoist/mygo/ui/markdown",
			"github.com/egoist/mygo/ui/markdown/parser",
		},
		Tests: []string{contribModulePath},
	},
}

var architectureModules = []architectureModule{
	{
		Name:       "root",
		Path:       modulePath,
		Dir:        ".",
		Allowed:    allowedProjectImports,
		Stdlib:     untieStdlibPolicy,
		Violations: packageImportViolations,
	},
	{
		Name:       "examples/mygo",
		Path:       exampleModulePath,
		Dir:        filepath.Join("examples", "mygo"),
		Allowed:    allowedExampleImports,
		Stdlib:     exampleStdlibPolicy,
		Violations: examplePackageImportViolations,
	},
	{
		Name:       "contrib/mygo",
		Path:       contribModulePath,
		Dir:        filepath.Join("contrib", "mygo"),
		Allowed:    allowedContribImports,
		Stdlib:     exampleStdlibPolicy,
		Violations: contribPackageImportViolations,
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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -test in %s: %v\nstdout:\n%s\nstderr:\n%s", dir, err, out, stderr.Bytes())
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
		if len(pkg.CgoFiles) > 0 {
			t.Errorf("%s contains cgo files", pkg.ImportPath)
		}
		for _, violation := range module.Violations(pkg.ImportPath, pkg.Imports, false, module.Allowed, module.Stdlib) {
			t.Error(violation)
		}
		checkStdlibExceptionFreshness(t, pkg.ImportPath, pkg.Imports, module.Stdlib)
		testImports := append([]string(nil), pkg.TestImports...)
		testImports = append(testImports, pkg.XTestImports...)
		for _, violation := range module.Violations(pkg.ImportPath, testImports, true, module.Allowed, module.Stdlib) {
			t.Error(violation)
		}
		checkRuleFreshness(t, pkg.ImportPath, pkg.Imports, testImports, module.Allowed)
	}
	for pkg := range module.Allowed {
		if !seen[pkg] {
			t.Errorf("allowlist package %s in module %s was not returned by go list", pkg, module.Path)
		}
	}
	for pkg := range module.Stdlib.ProductionExceptions {
		if !seen[pkg] {
			t.Errorf("stdlib exception package %s in module %s was not returned by go list", pkg, module.Path)
		}
	}
}

func checkStdlibExceptionFreshness(t *testing.T, pkg string, imports []string, policy stdlibPolicy) {
	t.Helper()

	importSet := importSet(imports)
	for imp, reason := range policy.ProductionExceptions[pkg] {
		if reason == "" {
			t.Errorf("%s has a standard-library exception for %s without a reason", pkg, imp)
		}
		if !importSet[imp] {
			t.Errorf("%s has unused standard-library exception for %s", pkg, imp)
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
	}, false, allowedProjectImports, untieStdlibPolicy)
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want two", violations)
	}
	if got := packageImportViolations(modulePath+"/font", []string{modulePath + "/geometry"}, false, allowedProjectImports, untieStdlibPolicy); len(got) != 0 {
		t.Fatalf("font -> geometry rejected: %v", got)
	}
	if got := packageImportViolations(modulePath+"/font", []string{modulePath + "/internal/mathdata"}, false, allowedProjectImports, untieStdlibPolicy); len(got) == 0 {
		t.Fatal("production-only import rule accepted internal/mathdata")
	}
	if got := packageImportViolations(modulePath+"/font", []string{modulePath + "/internal/mathdata"}, true, allowedProjectImports, untieStdlibPolicy); len(got) == 0 {
		t.Fatal("test rule accepted an unused font -> internal/mathdata edge")
	}
	if got := packageImportViolations(modulePath+"/internal/engine", []string{modulePath + "/internal/tex"}, false, allowedProjectImports, untieStdlibPolicy); len(got) == 0 {
		t.Fatal("test-only engine -> tex edge leaked into production rules")
	}
}

func TestArchitectureStdlibPolicy(t *testing.T) {
	t.Parallel()

	for _, test := range []bool{false, true} {
		if got := packageImportViolations(modulePath+"/font", []string{"C"}, test, allowedProjectImports, untieStdlibPolicy); len(got) != 1 {
			t.Fatalf("cgo test=%t violations = %v, want one", test, got)
		}
	}

	allowedProduction := []string{
		"context",
		"encoding/json",
		"fmt",
		"math",
		"math/bits",
	}
	if got := packageImportViolations(modulePath+"/geometry", allowedProduction, false, allowedProjectImports, untieStdlibPolicy); len(got) != 0 {
		t.Fatalf("ordinary standard-library imports rejected: %v", got)
	}

	forbiddenProduction := []string{
		"net/http",
		"os/exec",
		"plugin",
		"runtime/cgo",
		"syscall",
		"syscall/js",
		"testing",
		"unsafe",
	}
	for _, imp := range forbiddenProduction {
		imp := imp
		t.Run("forbidden/"+imp, func(t *testing.T) {
			violations := packageImportViolations(modulePath+"/font", []string{imp}, false, allowedProjectImports, untieStdlibPolicy)
			if len(violations) != 1 {
				t.Fatalf("%s violations = %v, want one", imp, violations)
			}
		})
	}

	t.Run("documented exception", func(t *testing.T) {
		pkg := modulePath + "/internal/quality/conformance"
		if got := packageImportViolations(pkg, []string{"os/exec"}, false, allowedProjectImports, untieStdlibPolicy); len(got) != 0 {
			t.Fatalf("documented conformance exception rejected: %v", got)
		}
		if got := packageImportViolations(modulePath+"/internal/quality/daily", []string{"os/exec"}, false, allowedProjectImports, untieStdlibPolicy); len(got) == 0 {
			t.Fatal("os/exec exception leaked to another package")
		}
	})

	t.Run("standard library allowlist cannot bypass policy", func(t *testing.T) {
		allow := map[string]packageRules{
			modulePath + "/font": {Production: []string{"unsafe"}},
		}
		if got := packageImportViolations(modulePath+"/font", []string{"unsafe"}, false, allow, untieStdlibPolicy); len(got) == 0 {
			t.Fatal("exact import allowlist bypassed the restricted standard-library policy")
		}
	})

	allowedTests := []string{
		"net/http/httptest",
		"os/exec",
		"testing",
		"testing/fstest",
	}
	if got := packageImportViolations(modulePath+"/internal/quality", allowedTests, true, allowedProjectImports, untieStdlibPolicy); len(got) != 0 {
		t.Fatalf("legitimate test-only standard-library imports rejected: %v", got)
	}
	for _, imp := range []string{"plugin", "runtime/cgo", "syscall/js", "unsafe"} {
		if got := packageImportViolations(modulePath+"/internal/quality", []string{imp}, true, allowedProjectImports, untieStdlibPolicy); len(got) == 0 {
			t.Fatalf("test standard-library policy accepted %s", imp)
		}
	}
}

func TestArchitectureExamplePolicyRejectsInternal(t *testing.T) {
	t.Parallel()

	internalImport := modulePath + "/internal/engine"
	allow := map[string]packageRules{
		exampleModulePath: {Production: []string{internalImport}},
	}
	violations := examplePackageImportViolations(exampleModulePath, []string{internalImport}, false, allow, exampleStdlibPolicy)
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
	if got := examplePackageImportViolations(exampleModulePath, allowed, false, allowedExampleImports, exampleStdlibPolicy); len(got) != 0 {
		t.Fatalf("allowed example imports rejected: %v", got)
	}
	if got := examplePackageImportViolations(exampleModulePath, []string{"os/exec"}, false, allowedExampleImports, exampleStdlibPolicy); len(got) == 0 {
		t.Fatal("example policy accepted os/exec")
	}
	if got := examplePackageImportViolations(exampleModulePath, []string{"C"}, false, allowedExampleImports, exampleStdlibPolicy); len(got) == 0 {
		t.Fatal("example policy accepted cgo")
	}
}

func TestArchitectureContribPolicyRejectsInternal(t *testing.T) {
	t.Parallel()

	for _, internalImport := range []string{
		modulePath + "/internal/engine",
		"github.com/egoist/mygo/internal/l10n",
	} {
		allow := map[string]packageRules{
			contribModulePath: {Production: []string{internalImport}},
		}
		violations := contribPackageImportViolations(
			contribModulePath,
			[]string{internalImport},
			false,
			allow,
			exampleStdlibPolicy,
		)
		if len(violations) == 0 {
			t.Fatalf("contrib policy accepted internal import %s", internalImport)
		}
	}

	allowed := []string{
		modulePath,
		modulePath + "/render",
		"github.com/egoist/mygo/ui",
		"github.com/egoist/mygo/ui/markdown",
	}
	if got := contribPackageImportViolations(
		contribModulePath,
		allowed,
		false,
		allowedContribImports,
		exampleStdlibPolicy,
	); len(got) != 0 {
		t.Fatalf("allowed contrib imports rejected: %v", got)
	}
	if got := contribPackageImportViolations(
		contribModulePath,
		[]string{"os/exec"},
		false,
		allowedContribImports,
		exampleStdlibPolicy,
	); len(got) == 0 {
		t.Fatal("contrib policy accepted os/exec")
	}
}

func packageImportViolations(pkg string, imports []string, test bool, allow map[string]packageRules, stdlib stdlibPolicy) []string {
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
		if violation := stdlibViolation(pkg, imp, test, stdlib); violation != "" {
			violations = append(violations, violation)
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

func examplePackageImportViolations(pkg string, imports []string, test bool, allow map[string]packageRules, stdlib stdlibPolicy) []string {
	return consumerPackageImportViolations(
		pkg,
		imports,
		test,
		exampleModulePath,
		allow,
		stdlib,
	)
}

func contribPackageImportViolations(pkg string, imports []string, test bool, allow map[string]packageRules, stdlib stdlibPolicy) []string {
	return consumerPackageImportViolations(
		pkg,
		imports,
		test,
		contribModulePath,
		allow,
		stdlib,
	)
}

func consumerPackageImportViolations(
	pkg string,
	imports []string,
	test bool,
	consumerModulePath string,
	allow map[string]packageRules,
	stdlib stdlibPolicy,
) []string {
	rules, known := allow[pkg]
	if !known {
		return []string{fmt.Sprintf("%s is not in the consumer architecture allowlist", pkg)}
	}
	allowed := rules.Production
	if test {
		allowed = append(append([]string(nil), allowed...), rules.Tests...)
	}
	allowedSet := importSet(allowed)
	slices.Sort(imports)
	var violations []string
	for _, imp := range imports {
		if violation := stdlibViolation(pkg, imp, test, stdlib); violation != "" {
			violations = append(violations, violation)
			continue
		}
		if untieInternalImport(imp) {
			violations = append(violations, fmt.Sprintf("%s imports Untie internal package %s", pkg, imp))
			continue
		}
		if internalPackageImport(imp) {
			violations = append(violations, fmt.Sprintf("%s imports internal package %s", pkg, imp))
			continue
		}
		if withinModule(imp, consumerModulePath) || allowedSet[imp] {
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

func stdlibViolation(pkg, imp string, test bool, policy stdlibPolicy) string {
	if imp == "C" {
		return fmt.Sprintf("%s imports cgo", pkg)
	}
	if !isStandardLibraryImport(imp) {
		return ""
	}
	if !test {
		if reason := policy.ProductionExceptions[pkg][imp]; reason != "" {
			return ""
		}
	}
	forbidden := policy.ProductionForbidden
	if test {
		forbidden = policy.TestForbidden
	}
	for _, root := range forbidden {
		if withinModule(imp, root) {
			return fmt.Sprintf("%s imports restricted standard-library package %s", pkg, imp)
		}
	}
	return ""
}

func isStandardLibraryImport(imp string) bool {
	if imp == "" || imp == "C" {
		return false
	}
	first := strings.SplitN(imp, "/", 2)[0]
	return !strings.Contains(first, ".")
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

func internalPackageImport(imp string) bool {
	for _, part := range strings.Split(imp, "/") {
		if part == "internal" {
			return true
		}
	}
	return false
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

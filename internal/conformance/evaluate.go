package conformance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// ExpectationKind describes only whether the upstream case is supposed to
// parse or to produce a recoverable diagnostic. Exact diagnostic codes are
// frozen by the reviewed baseline, not by the frontend under test.
type ExpectationKind string

const (
	ExpectParse      ExpectationKind = "parse"
	ExpectDiagnostic ExpectationKind = "diagnostic"
)

// Input is one dialect-neutral conformance case.
type Input struct {
	Corpus     string
	ID         string
	Index      int
	Source     string
	Preamble   string
	Mode       ir.Mode
	Expect     ExpectationKind
	ExpectCode []source.Code
	ExpectSpan *source.Range
	Dialect    Dialect
	Families   []string
}

// FrontendResult is the observation returned by a parser adapter.
type FrontendResult struct {
	Root        ir.Node
	Tags        []ir.EquationTag
	Diagnostics []source.Diagnostic
	Observation *IRSummary
}

// IRSummary is the serializable part of an IR observation used by isolated
// workers. When present, it replaces Root for validation and parity checks.
type IRSummary struct {
	Valid           bool
	ValidationError string
	CanonicalJSON   []byte
}

// Request is the complete formula source observation passed to a frontend.
type Request struct {
	Source string
	Mode   ir.Mode
	Limits source.Limits
}

// FrontendFunc evaluates one request under the supplied limits. Frontends share
// this boundary without sharing parser or layout code.
type FrontendFunc func(context.Context, Request) (FrontendResult, error)

// ErrFrontendTimeout identifies a frontend call that exceeded its per-run
// budget. Frontends should honor ctx themselves; this guard also contains
// older APIs that cannot yet be canceled cooperatively.
var ErrFrontendTimeout = errors.New("frontend evaluation timed out")

const defaultFrontendTimeout = time.Second

// Evaluator repeats one frontend, checks both limit profiles, and verifies the
// IR, diagnostics, source ranges, and canonical-output contracts.
type Evaluator struct {
	Frontend       FrontendFunc
	ExplicitLimits source.Limits
	Timeout        time.Duration
}

// Evaluate checks parse recovery, determinism, both limit profiles, and the IR
// contract.
func (e Evaluator) Evaluate(ctx context.Context, input Input) (CaseReport, error) {
	if e.Frontend == nil {
		return CaseReport{}, errors.New("nil frontend")
	}
	if invalidInput(input) {
		return CaseReport{}, fmt.Errorf("invalid case %q", input.ID)
	}
	explicitLimits := e.ExplicitLimits.WithDefaults()
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultFrontendTimeout
	}
	defaultRun, err := invokeFrontend(ctx, e.Frontend, Request{
		Source: input.Source, Mode: input.Mode, Limits: source.DefaultLimits(),
	}, timeout)
	if err != nil {
		return reportFromInput(input, StatusBug, "default-limit parse failed: "+err.Error()), nil
	}
	if err := ctx.Err(); err != nil {
		return CaseReport{}, err
	}
	repeatedDefaultRun, err := invokeFrontend(ctx, e.Frontend, Request{
		Source: input.Source, Mode: input.Mode, Limits: source.DefaultLimits(),
	}, timeout)
	if err != nil {
		return reportFromInput(input, StatusBug, "determinism parse failed: "+err.Error()), nil
	}
	if err := ctx.Err(); err != nil {
		return CaseReport{}, err
	}
	explicitRun, err := invokeFrontend(ctx, e.Frontend, Request{
		Source: input.Source, Mode: input.Mode, Limits: explicitLimits,
	}, timeout)
	if err != nil {
		return reportFromInput(input, StatusBug, "explicit-limit parse failed: "+err.Error()), nil
	}

	result := reportFromInput(input, StatusPass, "")
	result.Diagnostics = diagnosticRefs(defaultRun.Diagnostics)
	result.Codes = diagnosticCodes(defaultRun.Diagnostics)

	detail := structuralProblem(defaultRun, repeatedDefaultRun, explicitRun, input.Source, explicitLimits)
	if detail != "" {
		result.Status = StatusBug
		result.Detail = detail
		result.structural = true
		return result, nil
	}

	errorsFound := errorDiagnostics(defaultRun.Diagnostics)
	switch input.Expect {
	case ExpectDiagnostic:
		if len(errorsFound) == 0 {
			result.Status = StatusBug
			result.Detail = "upstream case expects a diagnostic, but parsing recovered silently"
			return result, nil
		}
		if !expectedDiagnosticFound(input.ExpectCode, input.ExpectSpan, errorsFound) {
			result.Status = StatusBug
			result.Detail = "diagnostics do not include an expected code and source span"
			return result, nil
		}
		result.Status = StatusDiagnostic
		result.Review = ReviewUpstream
		result.Reason = "selected upstream case expects a recoverable diagnostic"
	case ExpectParse:
		if len(errorsFound) == 0 {
			return result, nil
		}
		if allCodes(errorsFound, source.CodeUnsupported) {
			result.Status = StatusUnsupported
			result.Detail = "frontend reports an unsupported construct"
			return result, nil
		}
		result.Status = StatusBug
		result.Detail = "upstream case expects successful parsing"
	default:
		return CaseReport{}, fmt.Errorf("case %q has invalid expectation %q", input.ID, input.Expect)
	}
	return result, nil
}

func invokeFrontend(ctx context.Context, frontend FrontendFunc, request Request, timeout time.Duration) (FrontendResult, error) {
	if err := ctx.Err(); err != nil {
		return FrontendResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type callResult struct {
		result FrontendResult
		err    error
	}
	done := make(chan callResult, 1)
	go func() {
		var result FrontendResult
		var err error
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("frontend panic: %v", recovered)
			}
			if err == nil && result.Root == nil && result.Observation == nil {
				err = errors.New("frontend returned no IR observation")
			}
			done <- callResult{result: result, err: err}
		}()
		result, err = frontend(runCtx, request)
	}()
	select {
	case call := <-done:
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return FrontendResult{}, ErrFrontendTimeout
		}
		return call.result, call.err
	case <-runCtx.Done():
		if err := ctx.Err(); err != nil {
			return FrontendResult{}, err
		}
		return FrontendResult{}, ErrFrontendTimeout
	}
}

func structuralProblem(first, repeated, explicit FrontendResult, src string, explicitLimits source.Limits) string {
	firstCanonical, firstIRProblem := summarizeIR(first, src, source.DefaultLimits())
	if firstIRProblem != "" {
		return "default-limit result violates IR contract: " + firstIRProblem
	}
	repeatedCanonical, repeatedIRProblem := summarizeIR(repeated, src, source.DefaultLimits())
	if repeatedIRProblem != "" {
		return "repeated default-limit result violates IR contract: " + repeatedIRProblem
	}
	explicitCanonical, explicitIRProblem := summarizeIR(explicit, src, explicitLimits)
	if explicitIRProblem != "" {
		return "explicit-limit result violates IR contract: " + explicitIRProblem
	}
	if !bytes.Equal(firstCanonical, repeatedCanonical) {
		return "repeated default-limit canonical IR differs"
	}
	if firstSig, repeatedSig := diagnosticSignature(first.Diagnostics), diagnosticSignature(repeated.Diagnostics); firstSig != repeatedSig {
		return "repeated default-limit diagnostics differ: " + firstSig + " vs " + repeatedSig
	}
	if !bytes.Equal(firstCanonical, explicitCanonical) {
		return "default-limit and explicit-limit IR differ"
	}
	if firstSig, explicitSig := diagnosticSignature(first.Diagnostics), diagnosticSignature(explicit.Diagnostics); firstSig != explicitSig {
		return "default-limit and explicit-limit diagnostics differ: " + firstSig + " vs " + explicitSig
	}
	return ""
}

func summarizeIR(result FrontendResult, src string, limits source.Limits) ([]byte, string) {
	if problem := validateDiagnostics(result.Diagnostics, src); problem != "" {
		return nil, problem
	}
	if result.Observation != nil {
		if !result.Observation.Valid {
			if result.Observation.ValidationError == "" {
				return nil, "isolated worker reported invalid IR without a reason"
			}
			return nil, result.Observation.ValidationError
		}
		if len(result.Observation.CanonicalJSON) == 0 {
			return nil, "isolated worker reported valid IR without canonical JSON"
		}
		return result.Observation.CanonicalJSON, ""
	}
	formula := &ir.Formula{Root: result.Root, Tags: result.Tags}
	if err := ir.ValidateSource(formula, src, limits); err != nil {
		return nil, err.Error()
	}
	if problem := validateFormulaBoundaries(formula, src); problem != "" {
		return nil, problem
	}
	canonical, err := ir.MarshalCanonical(formula)
	if err != nil {
		return nil, "canonicalize IR: " + err.Error()
	}
	return canonical, ""
}

func validateDiagnostics(diagnostics []source.Diagnostic, src string) string {
	for i, diagnostic := range diagnostics {
		if diagnostic.Code == "" {
			return fmt.Sprintf("diagnostic %d has no code", i)
		}
		if strings.TrimSpace(diagnostic.Message) == "" {
			return fmt.Sprintf("diagnostic %d has no message", i)
		}
		if !diagnostic.Range.IsValidFor(len(src)) {
			return fmt.Sprintf("diagnostic %d range %d:%d exceeds source length %d", i, diagnostic.Range.Start, diagnostic.Range.End, len(src))
		}
		if !isRangeOnUTF8Boundaries(diagnostic.Range, src) {
			return fmt.Sprintf("diagnostic %d range %d:%d splits a UTF-8 code point", i, diagnostic.Range.Start, diagnostic.Range.End)
		}
	}
	return ""
}

func validateIRBoundaries(root ir.Node, src string) string {
	if root == nil {
		return "nil IR root"
	}
	var problem string
	ir.Walk(root, func(node ir.Node) bool {
		if problem != "" {
			return false
		}
		base := node.NodeBase()
		if base == nil {
			problem = "IR node has no base"
			return false
		}
		for _, field := range []struct {
			name string
			r    source.Range
		}{
			{"span", base.Span},
			{"origin", base.Origin},
		} {
			name, r := field.name, field.r
			if r.IsDetached() {
				continue
			}
			if !r.IsValidFor(len(src)) {
				problem = fmt.Sprintf("%s range %d:%d exceeds source length %d", name, r.Start, r.End, len(src))
				return false
			}
			if !isRangeOnUTF8Boundaries(r, src) {
				problem = fmt.Sprintf("%s range %d:%d splits a UTF-8 code point", name, r.Start, r.End)
				return false
			}
		}
		return true
	})
	return problem
}

func validateFormulaBoundaries(formula *ir.Formula, src string) string {
	if formula == nil {
		return "nil IR formula"
	}
	if problem := validateIRBoundaries(formula.Root, src); problem != "" {
		return problem
	}
	for i, tag := range formula.Tags {
		for _, field := range []struct {
			name string
			r    source.Range
		}{
			{"span", tag.Span},
			{"origin", tag.Origin},
		} {
			if problem := validateRangeBoundary(field.r, field.name, src); problem != "" {
				return fmt.Sprintf("tag %d: %s", i, problem)
			}
		}
	}
	var problem string
	if err := ir.WalkFormula(formula, source.DefaultLimits(), func(node ir.Node) bool {
		if problem != "" {
			return false
		}
		base := node.NodeBase()
		if base == nil {
			problem = "IR node has no base"
			return false
		}
		if issue := validateRangeBoundary(base.Span, "span", src); issue != "" {
			problem = issue
			return false
		}
		if issue := validateRangeBoundary(base.Origin, "origin", src); issue != "" {
			problem = issue
			return false
		}
		return true
	}); err != nil {
		return "walk formula boundaries: " + err.Error()
	}
	return problem
}

func validateRangeBoundary(r source.Range, name, src string) string {
	if r.IsDetached() {
		return ""
	}
	if !r.IsValidFor(len(src)) {
		return fmt.Sprintf("%s range %d:%d exceeds source length %d", name, r.Start, r.End, len(src))
	}
	if !isRangeOnUTF8Boundaries(r, src) {
		return fmt.Sprintf("%s range %d:%d splits a UTF-8 code point", name, r.Start, r.End)
	}
	return ""
}

func isRangeOnUTF8Boundaries(r source.Range, src string) bool {
	if r.IsDetached() {
		return true
	}
	if !r.IsValidFor(len(src)) {
		return false
	}
	if r.Start > 0 && r.Start < len(src) && !utf8.RuneStart(src[r.Start]) {
		return false
	}
	return r.End == 0 || r.End == len(src) || utf8.RuneStart(src[r.End])
}

func reportFromInput(input Input, status Status, detail string) CaseReport {
	dialect := input.Dialect
	if dialect == "" {
		dialect = DialectTeX
	}
	var families, commands, environments []string
	switch dialect {
	case DialectTypst:
		families, commands, environments = typstSourceFamilies(input.Source)
	case DialectTeX:
		families, commands, environments = sourceFamilies(input.Source)
	default:
		families = []string{"invalid-dialect"}
	}
	families = append(families, input.Families...)
	slices.Sort(families)
	families = slices.Compact(families)
	return CaseReport{
		ID:           input.ID,
		Corpus:       input.Corpus,
		Index:        input.Index,
		Status:       status,
		Families:     families,
		Commands:     commands,
		Environments: environments,
		Detail:       detail,
	}
}

func invalidInput(input Input) bool {
	if input.Corpus == "" || input.ID == "" || input.Index <= 0 {
		return true
	}
	if input.Mode != ir.Inline && input.Mode != ir.Display {
		return true
	}
	if input.Dialect != "" && input.Dialect != DialectTeX && input.Dialect != DialectTypst {
		return true
	}
	switch input.Expect {
	case ExpectParse, ExpectDiagnostic:
		return false
	default:
		return true
	}
}

func diagnosticRefs(diagnostics []source.Diagnostic) []DiagnosticRef {
	if len(diagnostics) == 0 {
		return nil
	}
	refs := make([]DiagnosticRef, len(diagnostics))
	for i, diagnostic := range diagnostics {
		refs[i] = DiagnosticRef{
			Code:     diagnostic.Code,
			Severity: diagnostic.Severity,
			Start:    diagnostic.Range.Start,
			End:      diagnostic.Range.End,
			Message:  diagnostic.Message,
		}
	}
	return refs
}

func diagnosticCodes(diagnostics []source.Diagnostic) []source.Code {
	if len(diagnostics) == 0 {
		return nil
	}
	codes := make([]source.Code, len(diagnostics))
	for i, diagnostic := range diagnostics {
		codes[i] = diagnostic.Code
	}
	return codes
}

func errorDiagnostics(diagnostics []source.Diagnostic) []source.Diagnostic {
	var errorsFound []source.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == source.SeverityError {
			errorsFound = append(errorsFound, diagnostic)
		}
	}
	return errorsFound
}

func expectedDiagnosticFound(expected []source.Code, expectedSpan *source.Range, diagnostics []source.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if expectedSpan != nil && diagnostic.Range != *expectedSpan {
			continue
		}
		for _, code := range expected {
			if diagnostic.Code == code {
				return true
			}
		}
		if len(expected) == 0 {
			return true
		}
	}
	return len(expected) == 0 && expectedSpan == nil
}

func allCodes(diagnostics []source.Diagnostic, code source.Code) bool {
	if len(diagnostics) == 0 {
		return false
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != code {
			return false
		}
	}
	return true
}

func diagnosticSignature(diagnostics []source.Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}
	parts := make([]string, len(diagnostics))
	for i, diagnostic := range diagnostics {
		parts[i] = fmt.Sprintf("%s@%d:%d:%s", diagnostic.Code, diagnostic.Range.Start, diagnostic.Range.End, diagnostic.Message)
	}
	return strings.Join(parts, "|")
}

package conformance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	Mode       ir.Mode
	Expect     ExpectationKind
	ExpectCode []source.Code
}

// FrontendResult is the observation returned by a parser adapter.
type FrontendResult struct {
	Root        ir.Node
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

// FrontendFunc evaluates source under the supplied limits. A future Typst
// adapter can implement this without importing TeX or layout.
type FrontendFunc func(context.Context, string, ir.Mode, source.Limits) (FrontendResult, error)

// ErrFrontendTimeout identifies a frontend call that exceeded its per-run
// budget. Frontends should honor ctx themselves; this guard also contains
// older APIs that cannot yet be canceled cooperatively.
var ErrFrontendTimeout = errors.New("frontend evaluation timed out")

const defaultFrontendTimeout = time.Second

// Evaluator runs one frontend under two limit profiles.
type Evaluator struct {
	Frontend       FrontendFunc
	ExplicitLimits source.Limits
	Timeout        time.Duration
}

// Evaluate checks parse recovery, both limit profiles, and the IR contract.
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
	defaultRun, err := invokeFrontend(ctx, e.Frontend, input.Source, input.Mode, source.DefaultLimits(), timeout)
	if err != nil {
		return reportFromInput(input, StatusBug, "default-limit parse failed: "+err.Error()), nil
	}
	if err := ctx.Err(); err != nil {
		return CaseReport{}, err
	}
	explicitRun, err := invokeFrontend(ctx, e.Frontend, input.Source, input.Mode, explicitLimits, timeout)
	if err != nil {
		return reportFromInput(input, StatusBug, "explicit-limit parse failed: "+err.Error()), nil
	}

	result := reportFromInput(input, StatusPass, "")
	result.Diagnostics = diagnosticRefs(defaultRun.Diagnostics)
	result.Codes = diagnosticCodes(defaultRun.Diagnostics)

	detail := structuralProblem(defaultRun, explicitRun, explicitLimits)
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
		if !expectedCodeFound(input.ExpectCode, errorsFound) {
			result.Status = StatusBug
			result.Detail = "diagnostics do not include an expected code"
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

func invokeFrontend(ctx context.Context, frontend FrontendFunc, src string, mode ir.Mode, limits source.Limits, timeout time.Duration) (FrontendResult, error) {
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
		result, err = frontend(runCtx, src, mode, limits)
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

func structuralProblem(first, second FrontendResult, explicit source.Limits) string {
	firstCanonical, firstIRProblem := summarizeIR(first, source.DefaultLimits())
	if firstIRProblem != "" {
		return "default-limit result violates IR contract: " + firstIRProblem
	}
	secondCanonical, secondIRProblem := summarizeIR(second, explicit)
	if secondIRProblem != "" {
		return "explicit-limit result violates IR contract: " + secondIRProblem
	}
	if !bytes.Equal(firstCanonical, secondCanonical) {
		return "default-limit and explicit-limit IR differ"
	}
	if firstSig, secondSig := diagnosticSignature(first.Diagnostics), diagnosticSignature(second.Diagnostics); firstSig != secondSig {
		return "default-limit and explicit-limit diagnostics differ: " + firstSig + " vs " + secondSig
	}
	return ""
}

func summarizeIR(result FrontendResult, limits source.Limits) ([]byte, string) {
	if result.Observation != nil {
		if !result.Observation.Valid {
			if result.Observation.ValidationError == "" {
				return nil, "isolated worker reported invalid IR without a reason"
			}
			return nil, result.Observation.ValidationError
		}
		return result.Observation.CanonicalJSON, ""
	}
	if err := ir.Validate(&ir.Formula{Root: result.Root}, limits); err != nil {
		return nil, err.Error()
	}
	canonical, err := ir.MarshalCanonical(&ir.Formula{Root: result.Root})
	if err != nil {
		return nil, "canonicalize IR: " + err.Error()
	}
	return canonical, ""
}

func reportFromInput(input Input, status Status, detail string) CaseReport {
	families, commands, environments := sourceFamilies(input.Source)
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

func expectedCodeFound(expected []source.Code, diagnostics []source.Diagnostic) bool {
	if len(expected) == 0 {
		return true
	}
	for _, diagnostic := range diagnostics {
		for _, code := range expected {
			if diagnostic.Code == code {
				return true
			}
		}
	}
	return false
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

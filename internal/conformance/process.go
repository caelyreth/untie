package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// ProcessConfig describes an isolated frontend worker. Every case receives a
// fresh process, so a non-cooperative parser hang or fatal failure cannot poison
// the rest of the conformance run.
type ProcessConfig struct {
	Path string
	Args []string
	Env  []string
}

type workerRequest struct {
	Source   string        `json:"source"`
	Preamble string        `json:"preamble,omitempty"`
	Mode     ir.Mode       `json:"mode"`
	Limits   source.Limits `json:"limits"`
}

type workerResponse struct {
	Diagnostics     []DiagnosticRef `json:"diagnostics,omitempty"`
	Valid           bool            `json:"valid"`
	ValidationError string          `json:"validationError,omitempty"`
	CanonicalJSON   []byte          `json:"canonicalJSON,omitempty"`
	FrontendError   string          `json:"frontendError,omitempty"`
}

// ProcessFrontend returns a frontend that communicates with one worker process
// per evaluation over stdin/stdout.
func ProcessFrontend(config ProcessConfig) FrontendFunc {
	return func(ctx context.Context, request Request) (FrontendResult, error) {
		if config.Path == "" {
			return FrontendResult{}, errors.New("empty worker executable")
		}
		wireRequest, err := json.Marshal(workerRequest(request))
		if err != nil {
			return FrontendResult{}, err
		}
		command := exec.CommandContext(ctx, config.Path, config.Args...)
		command.Env = append(os.Environ(), config.Env...)
		command.Stdin = bytes.NewReader(append(wireRequest, '\n'))
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		if err := command.Run(); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return FrontendResult{}, ctxErr
			}
			message := strings.TrimSpace(stderr.String())
			if message != "" {
				return FrontendResult{}, fmt.Errorf("worker: %w: %s", err, message)
			}
			return FrontendResult{}, fmt.Errorf("worker: %w", err)
		}
		decoder := json.NewDecoder(&stdout)
		decoder.DisallowUnknownFields()
		var response workerResponse
		if err := decoder.Decode(&response); err != nil {
			return FrontendResult{}, fmt.Errorf("decode worker response: %w", err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return FrontendResult{}, fmt.Errorf("decode worker response: %w", err)
		}
		if response.FrontendError != "" {
			return FrontendResult{}, errors.New(response.FrontendError)
		}
		return FrontendResult{
			Diagnostics: diagnosticsFromRefs(response.Diagnostics),
			Observation: &IRSummary{
				Valid:           response.Valid,
				ValidationError: response.ValidationError,
				CanonicalJSON:   response.CanonicalJSON,
			},
		}, nil
	}
}

// RunWorker executes one frontend request using a process-safe protocol. The
// report commands and conformance tests use this entry point for isolation.
func RunWorker(frontend FrontendFunc, input io.Reader, output io.Writer) error {
	if frontend == nil {
		return errors.New("nil worker frontend")
	}
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	var request workerRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode worker request: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode worker request: %w", err)
	}
	response := workerResponse{}
	result, err := callFrontend(context.Background(), frontend, request)
	if err != nil {
		response.FrontendError = err.Error()
	} else {
		response.Diagnostics = diagnosticRefs(result.Diagnostics)
		canonical, problem := summarizeIR(result, request.Source, request.Limits)
		response.Valid = problem == ""
		response.ValidationError = problem
		response.CanonicalJSON = canonical
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(response)
}

func callFrontend(ctx context.Context, frontend FrontendFunc, request workerRequest) (result FrontendResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("frontend panic: %v", recovered)
		}
		if err == nil && result.Root == nil && result.Observation == nil {
			err = errors.New("frontend returned no IR observation")
		}
	}()
	return frontend(ctx, Request(request))
}

func diagnosticsFromRefs(refs []DiagnosticRef) []source.Diagnostic {
	if len(refs) == 0 {
		return nil
	}
	diagnostics := make([]source.Diagnostic, len(refs))
	for i, ref := range refs {
		diagnostics[i] = source.Diagnostic{
			Range:    source.NewRange(ref.Start, ref.End),
			Severity: ref.Severity,
			Code:     ref.Code,
			Message:  ref.Message,
		}
	}
	return diagnostics
}

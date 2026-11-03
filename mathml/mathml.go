// Package mathml converts between Untie canonical IR and Presentation MathML.
//
// Marshal and Write cover every canonical IR node family. Unmarshal is a
// deliberately smaller, strict importer: it accepts the documented subset in
// docs/mathml.md and rejects unknown elements, attributes, and semantic values
// with structured diagnostics.
//
// MathML is an interchange boundary, not a renderer shortcut. Source spans,
// text-cluster maps, and dialect syntax cannot be represented soundly in
// Presentation MathML and are never invented by the importer.
package mathml

import (
	"context"
	"errors"
	"fmt"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

const (
	// Namespace is the Presentation MathML XML namespace.
	Namespace = "http://www.w3.org/1998/Math/MathML"

	// SchemaVersion identifies Untie's optional MathML metadata conventions.
	// It is emitted as data-untie-version on the root math element.
	SchemaVersion = 1

	// DefaultMaxOutputBytes bounds all output produced by Marshal and Write.
	DefaultMaxOutputBytes = 16 << 20
)

var (
	// ErrInvalidContext reports a nil context.
	ErrInvalidContext = errors.New("mathml: nil context")
	// ErrInvalidFormula reports malformed Untie input or an invalid export mode.
	ErrInvalidFormula = errors.New("mathml: invalid formula")
	// ErrInvalidMathML reports malformed or structurally invalid MathML input.
	ErrInvalidMathML = errors.New("mathml: invalid MathML")
	// ErrUnsupported reports valid-looking MathML outside the documented subset.
	ErrUnsupported = errors.New("mathml: unsupported construct")
	// ErrLimitExceeded reports an input, traversal, or output limit.
	ErrLimitExceeded = errors.New("mathml: limit exceeded")
)

// Options controls one MathML operation. Zero fields select defaults.
type Options struct {
	// Limits bounds formula validation, input traversal, and generated
	// accessibility text.
	Limits source.Limits

	// MaxOutputBytes bounds Marshal and Write output. Import input is bounded
	// by Limits.MaxSourceBytes.
	MaxOutputBytes int
}

func (o Options) normalized() Options {
	o.Limits = o.Limits.WithDefaults()
	if o.MaxOutputBytes <= 0 {
		o.MaxOutputBytes = DefaultMaxOutputBytes
	}
	return o
}

// Result is the strict importer result.
//
// Formula is non-nil only when Diagnostics is empty and err is nil. Mode is
// meaningful whenever Formula is non-nil. Diagnostic ranges address the input
// MathML bytes; imported IR nodes always use detached source ranges.
type Result struct {
	Formula     *ir.Formula
	Mode        ir.Mode
	Diagnostics []source.Diagnostic
}

// Marshal returns deterministic Presentation MathML for f.
func Marshal(ctx context.Context, f *ir.Formula, mode ir.Mode, options Options) ([]byte, error) {
	if ctx == nil {
		return nil, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	options = options.normalized()
	var output boundedBuffer
	output.limit = options.MaxOutputBytes
	if err := Write(ctx, &output, f, mode, options); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// Unmarshal parses a bounded Presentation MathML subset into validated
// canonical IR.
//
// Malformed or unsupported input returns a Result containing deterministic
// diagnostics and a matching sentinel error. Context cancellation always
// returns the context error and takes precedence over input diagnostics.
func Unmarshal(ctx context.Context, data []byte, options Options) (Result, error) {
	return unmarshal(ctx, data, options)
}

// UnmarshalString is Unmarshal for a string input.
func UnmarshalString(ctx context.Context, text string, options Options) (Result, error) {
	return Unmarshal(ctx, []byte(text), options)
}

type boundedBuffer struct {
	data  []byte
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-len(b.data) {
		return 0, fmt.Errorf("%w: output exceeds %d bytes", ErrLimitExceeded, b.limit)
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *boundedBuffer) Bytes() []byte {
	return b.data
}

package layout

import (
	"context"
	"errors"

	"github.com/caelyreth/untie/ir"
	"github.com/caelyreth/untie/source"
)

// sourceError attaches a canonical IR source range to an internal layout
// failure. The host facade converts it to a public untie.LayoutError.
type sourceError struct {
	sourceRange source.Range
	err         error
}

// Error preserves the underlying message while keeping the locator internal.
func (e *sourceError) Error() string {
	if e == nil {
		return "layout: nil source error"
	}
	if e.err == nil {
		return "layout: source error"
	}
	return e.err.Error()
}

// Unwrap returns the unsourced layout failure.
func (e *sourceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// SourceRange returns the canonical IR node range.
func (e *sourceError) SourceRange() source.Range {
	if e == nil {
		return source.Detached()
	}
	return e.sourceRange
}

func withSourceRange(err error, r source.Range) error {
	if err == nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var located interface {
		SourceRange() source.Range
	}
	if errors.As(err, &located) {
		return err
	}
	if !r.IsValid() {
		r = source.Detached()
	}
	return &sourceError{sourceRange: r, err: err}
}

func withNodeSourceRange(node ir.Node, err error) error {
	if node == nil {
		return withSourceRange(err, source.Detached())
	}
	return withSourceRange(err, nodeSourceRange(node))
}

func nodeSourceRange(node ir.Node) source.Range {
	if node == nil {
		return source.Detached()
	}
	base := node.NodeBase()
	if base == nil {
		return source.Detached()
	}
	if base.Origin.IsValid() && !base.Origin.IsDetached() {
		return base.Origin
	}
	if base.Span.IsValid() {
		return base.Span
	}
	return source.Detached()
}

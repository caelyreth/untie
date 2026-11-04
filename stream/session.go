package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/caelyreth/untie"
)

// DefaultCommitInterval is the default spacing between accepted source
// snapshots when SessionOptions.CommitInterval is zero.
const DefaultCommitInterval = 180 * time.Millisecond

// Renderer is the subset of Untie's renderer used by a Session.
type Renderer interface {
	Render(
		context.Context,
		string,
		untie.RenderRequest,
	) (*untie.RenderResult, error)
}

// SessionOptions configure one formula's streaming renderer.
type SessionOptions struct {
	// Request selects dialect, mode, style, and parse options.
	Request untie.RenderRequest
	// TargetSource, when non-empty, is rendered once and exposed as the
	// session's stable layout stage. Hosts use it to reserve final geometry.
	TargetSource string
	// CommitInterval bounds how often changing source is laid out. Zero uses
	// DefaultCommitInterval. Negative values are invalid.
	CommitInterval time.Duration
}

// Session coalesces source updates into immutable render frames.
//
// Session is caller-owned and must not be used concurrently. The renderer it
// refers to may be shared concurrently by other sessions.
type Session struct {
	renderer       Renderer
	options        SessionOptions
	commitInterval time.Duration
	stage          *untie.Box
	frame          *Frame
	pending        string
	hasPending     bool
	lastCommit     time.Time
}

// NewSession prepares a streaming renderer and an optional completed target
// stage. The target is rendered once before the first update.
func NewSession(
	ctx context.Context,
	renderer Renderer,
	options SessionOptions,
) (*Session, error) {
	if renderer == nil {
		return nil, fmt.Errorf("%w: nil renderer", ErrInvalidOptions)
	}
	if ctx == nil {
		return nil, untie.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options.CommitInterval < 0 {
		return nil, fmt.Errorf("%w: negative commit interval %v",
			ErrInvalidOptions, options.CommitInterval)
	}
	if options.CommitInterval == 0 {
		options.CommitInterval = DefaultCommitInterval
	}
	s := &Session{
		renderer:       renderer,
		options:        options,
		commitInterval: options.CommitInterval,
	}
	if options.TargetSource != "" {
		result, err := renderer.Render(ctx, options.TargetSource, options.Request)
		if err != nil {
			return nil, err
		}
		if result == nil || result.Box == nil {
			return nil, fmt.Errorf("%w: target source produced no box",
				ErrInvalidTransition)
		}
		s.stage = result.Box
	}
	return s, nil
}

// Update offers the latest source and renders it when the previous commit
// interval has elapsed. Repetitive calls during that window coalesce to one
// layout. changed reports whether a new immutable frame was committed.
func (s *Session) Update(
	ctx context.Context,
	source string,
	now time.Time,
) (*Frame, bool, error) {
	if s == nil || s.renderer == nil {
		return nil, false, fmt.Errorf("%w: nil session", ErrInvalidOptions)
	}
	if ctx == nil {
		return s.frame, false, untie.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return s.frame, false, err
	}
	if s.frame != nil && s.frame.source == source {
		s.pending = ""
		s.hasPending = false
		return s.frame, false, nil
	}
	s.pending = source
	s.hasPending = true
	if s.frame != nil && now.Before(s.lastCommit.Add(s.commitInterval)) {
		return s.frame, false, nil
	}

	result, err := s.renderer.Render(ctx, source, s.options.Request)
	s.hasPending = false
	s.pending = ""
	if err != nil && (result == nil || result.Box == nil) {
		return s.frame, false, err
	}
	if result == nil || result.Box == nil {
		return s.frame, false, fmt.Errorf("%w: render produced no box",
			ErrInvalidTransition)
	}
	var previous *untie.Box
	if s.frame != nil {
		previous = s.frame.box
	}
	stage := s.stage
	if stage == nil {
		stage = result.Box
	}
	s.frame = &Frame{
		source:   source,
		previous: previous,
		box:      result.Box,
		stage:    stage,
		result:   result,
	}
	s.lastCommit = now
	return s.frame, true, err
}

// Frame returns the latest committed frame.
func (s *Session) Frame() *Frame {
	if s == nil {
		return nil
	}
	return s.frame
}

// Pending reports whether a newer source is waiting for the next commit.
func (s *Session) Pending() bool {
	return s != nil && s.hasPending
}

// NextCommitIn reports how long a host should wait before calling Update again
// to commit the pending source. The returned duration is zero when a commit is
// already due. ok is false when no source is pending.
//
// Non-UI hosts can use this with their timer scheduler. MyGo hosts can pass the
// duration directly to Context.After, which requests a full view rebuild rather
// than the paint-only frames used for interpolation.
func (s *Session) NextCommitIn(now time.Time) (time.Duration, bool) {
	if s == nil || !s.hasPending {
		return 0, false
	}
	if s.frame == nil {
		return 0, true
	}
	deadline := s.lastCommit.Add(s.commitInterval)
	if !deadline.After(now) {
		return 0, true
	}
	return deadline.Sub(now), true
}

// Frame is one immutable committed layout.
type Frame struct {
	source   string
	previous *untie.Box
	box      *untie.Box
	stage    *untie.Box
	result   *untie.RenderResult
}

// Source returns the partial formula source rendered by this frame.
func (f *Frame) Source() string {
	if f == nil {
		return ""
	}
	return f.source
}

// Box returns the current immutable display box.
func (f *Frame) Box() *untie.Box {
	if f == nil {
		return nil
	}
	return f.box
}

// Previous returns the immediately preceding box, or nil for the first frame.
func (f *Frame) Previous() *untie.Box {
	if f == nil {
		return nil
	}
	return f.previous
}

// Stage returns the stable completed layout, or Box when no target was given.
func (f *Frame) Stage() *untie.Box {
	if f == nil {
		return nil
	}
	return f.stage
}

// Result exposes the diagnostics and alternative text for this frame.
func (f *Frame) Result() *untie.RenderResult {
	if f == nil {
		return nil
	}
	return f.result
}

// Diagnostics returns the current frame's immutable diagnostic snapshot.
func (f *Frame) Diagnostics() []untie.Diagnostic {
	if f == nil || f.result == nil {
		return nil
	}
	return f.result.Diagnostics
}

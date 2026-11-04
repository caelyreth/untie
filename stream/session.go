package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/caelyreth/untie"
)

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
	// Transition controls interpolation and partial-source error visibility.
	Transition TransitionOptions
}

// Session coalesces source updates into immutable render frames.
//
// Session is caller-owned and must not be used concurrently. The renderer it
// refers to may be shared concurrently by other sessions.
type Session struct {
	renderer   Renderer
	options    SessionOptions
	transition TransitionOptions
	stage      *untie.Box
	frame      *Frame
	pending    string
	hasPending bool
	lastCommit time.Time
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
	transition, err := options.Transition.normalized()
	if err != nil {
		return nil, err
	}
	s := &Session{
		renderer:   renderer,
		options:    options,
		transition: transition,
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

// Update offers the latest source and renders it when the previous transition
// window has elapsed. Repetitive calls during that window coalesce to one
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
	if s.frame != nil && now.Before(s.lastCommit.Add(s.transition.Duration)) {
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
	transition, err := NewTransition(
		previous,
		result.Box,
		now,
		s.transition,
	)
	if err != nil {
		return s.frame, false, err
	}
	stage := s.stage
	if stage == nil {
		stage = result.Box
	}
	s.frame = &Frame{
		source:     source,
		box:        result.Box,
		stage:      stage,
		result:     result,
		transition: transition,
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

// Animating reports whether pending work or command interpolation needs
// another host frame.
func (s *Session) Animating(now time.Time) bool {
	if s == nil {
		return false
	}
	return s.hasPending || s.frame != nil && s.frame.Animating(now)
}

// Frame is one immutable committed layout and its transition.
type Frame struct {
	source     string
	box        *untie.Box
	stage      *untie.Box
	result     *untie.RenderResult
	transition *Transition
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

// Animating reports whether command interpolation is still in progress.
func (f *Frame) Animating(at time.Time) bool {
	return f != nil && f.transition != nil && f.transition.Animating(at)
}

// Walk samples drawing commands at time at.
func (f *Frame) Walk(at time.Time, visit func(Command) error) error {
	if f == nil || f.transition == nil {
		return nil
	}
	return f.transition.Walk(at, visit)
}

// Diagnostics returns the current frame's immutable diagnostic snapshot.
func (f *Frame) Diagnostics() []untie.Diagnostic {
	if f == nil || f.result == nil {
		return nil
	}
	return f.result.Diagnostics
}

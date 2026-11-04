# Streaming Formula Rendering

`github.com/caelyreth/untie/stream` turns append-only formula updates into
stable display transitions without depending on a UI framework.

The stream layer owns four jobs:

- coalescing rapidly changing source before layout;
- retaining the previous and current immutable `Box` values;
- matching visible paths and rules across layouts;
- sampling moved, resized, entering, exiting, and replaced commands.

It does not incrementally parse TeX or Typst. Every committed source snapshot
still receives a complete formula parse and layout. Coalescing limits that work
to the display transition cadence instead of the input callback or render frame
rate.

## Session

Create one `stream.Session` for each independently rendered formula.

```go
session, err := stream.NewSession(ctx, renderer, stream.SessionOptions{
	Request: untie.RenderRequest{
		Mode:  untie.Display,
		Style: untie.Style{Size: 30},
	},
	TargetSource: completedFormula,
	Transition: stream.TransitionOptions{
		Duration:             180 * time.Millisecond,
		AppearanceStartScale: 0.72,
		HideErrors:           true,
	},
})
if err != nil {
	return err
}
```

`TargetSource` is optional. When supplied, it is rendered once and exposed as
the session's stable stage. A host uses that stage to reserve the completed
formula's natural width and height while partial layouts change inside it. The
stage is a layout reservation, not rendered placeholder ink.

Offer the latest source during each host frame:

```go
frame, changed, err := session.Update(ctx, partialSource, now)
if err != nil && frame == nil {
	return err
}
if changed {
	hostContentSizeChanged(frame.Stage())
}
```

`Update` records the latest source immediately. Before the previous transition
window has elapsed, later updates replace its pending source and return the
existing frame without parsing or laying out a new box. After the window
elapses, the most recent source is rendered once.

Call `Update` again with the same source after the transition completes. This
is how a pending final source is committed after the in-flight frame reaches
its final state. Use `Session.Animating` or `Frame.Animating` to decide whether
the host needs another animation frame.

## Drawing

`Frame.Walk` samples draw commands at one instant:

```go
err := frame.Walk(now, func(command stream.Command) error {
	switch command.Kind {
	case stream.Path:
		drawPath(command.Path, command.At, command.Em, command.Paint, command.Opacity)
	case stream.Rule:
		drawRule(command.Rect, command.Paint, command.Opacity)
	}
	return nil
})
```

Path templates borrow immutable storage from the current box. Resolve a point
in box coordinates as `At + point*Em`. `Opacity` multiplies the host-resolved
paint color.

The transition uses stable identity in this order:

1. host-assigned `display.Item.NodeID`;
2. a source range at the item center;
3. glyph or rule identity plus a deterministic ordinal.

Exact matches are preferred. A guarded nearest-neighbor fallback handles
recovery layouts that move a source region without changing its glyph and path
template.

Shared glyph paths interpolate `At` and `Em`; they are not re-resolved into new
geometry for every animation frame. Rules interpolate their rectangles.
Entering paths and rules fade and scale around their final bounds. Unrelated
glyph replacements cross-fade instead of vertex-morphing unrelated outlines.

At or after the transition deadline, `Walk` emits exactly the current box's
visible paths and rules in original paint order, with full opacity. Interpolated
geometry never becomes the host's source for hit testing, copying, caret
placement, or accessibility; those operations continue to use `Frame.Box`.

## MyGo

`contrib/mygo.StreamingMath` adapts one session to a drawable MyGo element:

```go
streamed, err := math.NewStreamingMath(ctx, untiemygo.StreamingMathOptions{
	TargetSource: completedFormula,
	HideErrors:   true,
})
if err != nil {
	return err
}

ui.ScrollHorizontal(c).AlignSelf(ui.Stretch).Children(func() {
	streamed.Element(c, partialSource, c.Now()).MinWidthPercent(100)
})
```

The element uses the completed stage for its natural dimensions and scale. Put
it in a clipping or horizontal scroll container when it is wider than the
available area. `HideErrors` omits error and placeholder paint during partial
source recovery while retaining the stable layout reservation.

The MyGo Markdown `Math` callback does not expose a block revision, so a host
with multiple display blocks should retain one `StreamingMath` per callback
ordinal and reset that ordinal at the start of each Markdown build. The
`examples/mygo-stream` application demonstrates this pattern.

## Performance

The transition plan is compiled once per accepted source snapshot. Sampling
does not allocate path geometry or match tables.

Local Apple M5 Pro reference results from `go test ./stream -bench . -benchmem`:

```text
BenchmarkTransitionWalkFinal       119.1 ns/op   0 B/op   0 allocs/op
BenchmarkTransitionWalkMid         129.2 ns/op   0 B/op   0 allocs/op
BenchmarkSessionCoalescedUpdate      8.0 ns/op   0 B/op   0 allocs/op
```

These numbers describe transition sampling and coalescing, not parse or layout.
Measure complete streaming behavior with the host's actual formula size, frame
cadence, font set, and source rate.

## Ownership

- `Session` is caller-owned and not concurrency-safe.
- The underlying `Renderer` may be shared concurrently by other sessions.
- Frames and boxes are immutable. Do not mutate their slices.
- Path templates passed to `Frame.Walk` are callback-scoped.
- A session retains only its current frame and pending source; it is not an
  unbounded history of partial layouts.
- `HideErrors` and other transition options are fixed for a session. Changing
  them requires a new session.
- Edits that reorder formulas should use host-owned node identity. A future
  incremental frontend can extend stability across unrelated block edits.

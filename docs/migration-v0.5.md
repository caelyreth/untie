# Migrating to 0.5

Untie `0.5.0` moves animation timing and presentation policy out of the
framework-neutral stream package. Formula layout and display geometry are
unchanged.

## Stream Layer

`stream` now owns only source coalescing, immutable render frames, display-list
matching, and interpolation at normalized progress. It no longer owns wall-clock
animation time, duration, easing, appearance scaling, or error visibility.

`SessionOptions.Transition` is removed. Configure layout cadence directly:

```go
session, err := stream.NewSession(ctx, renderer, stream.SessionOptions{
	Request:        request,
	TargetSource:   completedFormula,
	CommitInterval: 180 * time.Millisecond,
})
```

`TransitionOptions`, `Timing`, `TimingEaseOutCubic`,
`TimingEaseInOutCubic`, `DefaultDuration`, `DefaultAppearanceStartScale`,
`Session.Animating`, `Frame.Animating`, and `Frame.Walk` are removed.

Transitions are now time-free:

```go
transition, err := stream.NewTransition(
	frame.Previous(),
	frame.Box(),
)
if err != nil {
	return err
}

elapsed := float32(now.Sub(started)) / float32(animationDuration)
progress := hostEase(elapsed)
err = transition.Walk(progress, appearanceStartScale,
	func(command stream.Command) error {
		draw(command)
		return nil
	},
)
```

At progress one, `Transition.Walk` returns exactly the current box's visible
paths and rules. The host owns the animation clock and decides when to request
another paint frame.

## MyGo Adapter

The MyGo adapter now owns presentation timing. `StreamingMathOptions` gains:

- `CommitInterval`;
- `AnimationDuration`;
- `Timing`;
- `AppearanceScale`;
- `HideErrors`.

Use the host-side timing constants:

```go
streamed, err := math.NewStreamingMath(ctx, untiemygo.StreamingMathOptions{
	TargetSource:      completedFormula,
	CommitInterval:    180 * time.Millisecond,
	AnimationDuration: 180 * time.Millisecond,
	Timing:            untiemygo.StreamEaseInOutCubic,
	AppearanceScale:   0.72,
	HideErrors:        true,
})
```

`StreamingMath.Element` now reads `c.Now()` itself:

```go
streamed.Element(c, partialSource)
```

It schedules layout rebuilds through `c.After`, uses `painter.Now()` for
paint-only animation frames, and applies MyGo-owned duration, easing, appearance
scaling, and paint-role filtering.

## Cache Identity

`Version()` now returns:

```text
0.5.0+layout.8
```

The layout revision is unchanged because this release changes stream
integration APIs rather than formula layout.

# Concurrency, Lifecycle, and Caching

Untie is designed for repeated formula layout in an editor or document
application. The safe model is simple: prepare immutable resources, reuse them,
and keep per-call inputs immutable for the duration of a call.

## Concurrency Summary

| Value or call | Concurrent use |
| --- | --- |
| `*FontSet` | Safe for concurrent readers. |
| `*Engine` | Safe for concurrent `Layout`/`LayoutContext` calls. |
| `*Renderer` | Safe for concurrent `Render` calls. |
| `*Expr` | Immutable and safe to share for reads. |
| `Style.Features` | Caller-owned; snapshotted per layout. |
| `*Box` | Independent per successful layout; not safe for concurrent mutation. |
| `*Formula` passed to `LayoutFormulaContext` | Safe to share only if every caller treats it as immutable for the call. |
| Package-level layout cache | Concurrency-safe, bounded LRU with shared in-flight builds. |

The engine's font adapter uses concurrency-safe font caches. A custom
substitution of public font types does not affect the root engine because the
root engine always uses its internal font adapter.

## No Close Lifecycle

There is no `Close` method on `FontSet` or `Engine`. Both are ordinary Go
objects:

1. `LoadFonts` copies and validates bytes.
2. `NewEngine` prepares immutable layout state.
3. Retain the engine for the configured font set's lifetime.
4. Let Go's garbage collector release it.

Avoid rebuilding an engine for every keystroke. It repeats font-derived setup
that does not depend on the formula.

## Recommended Application Lifecycle

```go
func NewMathRenderer(fontBytes []byte) (*untie.Renderer, error) {
	fonts, err := untie.LoadFonts(untie.Fonts{
		Math: untie.FontData{Bytes: fontBytes, ID: "host-math"},
	})
	if err != nil {
		return nil, err
	}
	return untie.NewRenderer(fonts, untie.RendererOptions{
		DefaultStyle: untie.Style{Size: 24},
		CacheEntries: 128,
	})
}
```

The same `untie.Renderer` can serve many UI workers concurrently as long as
each worker uses its own request data and does not mutate a shared `Box`.

An `Engine` retains immutable base-em design paths for glyphs it has shaped.
That cache is safe for concurrent layouts, grows only with glyphs actually
encountered, and is released with the engine. It never shares final display
paths between returned boxes.

Layout also recycles transient fragment, glyph, and path-cache scratch storage
through a process-wide hard-bounded pool. Every retained slot is cleared before
reuse, and a returned `Box` never references pooled scratch. The pool holds at
most 16 idle scratch sets, and oversized scratch is discarded rather than
retained indefinitely.

## Context Cancellation

Use `LayoutContext` and `AltContext` when input changes quickly:

```go
box, err := engine.LayoutContext(ctx, expr, style)
if errors.Is(err, context.Canceled) {
	return err
}
```

The engine checks cancellation before work and at bounded intervals. A
canceled call returns the context error and no box. Do not publish a partial
box.

Use `ParseContext` when the host needs cancellation around parsing. It checks
the context before dispatch and before returning; parser work in progress is
bounded by `ParseOptions.Limits`. `Renderer.Render` combines this boundary with
the engine's cooperative layout cancellation.

## Style Snapshotting

`Style.Features` is the only slice in `Style`. It is copied before the engine
enters layout:

```go
features := []string{"dtls=1"}
style := untie.Style{Size: 24, Features: features}

box, err := engine.Layout(expr, style)
features[0] = "bad feature" // does not affect the completed box
```

Later calls with the mutated slice see the mutation, as expected. A caller may
share one immutable `Style` value across workers.

## Box Ownership

Each successful layout returns a new `Box` and new item/path storage. Reusing
an engine does not alias boxes from earlier calls.

`Box.Clone()` copies the public display data, including paths, item and hit
slices, line/tag ranges, breaks, tags, and alternative text. It does not clone
the font set or engine.

Treat a `Box` as read-only after handing it to another goroutine. If a host
needs to annotate or transform one, clone it first.

## Expression and IR Ownership

`Expr` is immutable through its public methods. In particular:

- `RootSnapshot()` returns an independent validated formula, including tag
  metadata;
- `Diagnostics()` returns an independent slice;
- `Alt()` computes deterministic text without changing layout geometry.

`LayoutFormulaContext` is the advanced boundary where the caller supplies a
formula directly. It does not clone the formula. The caller must keep it
immutable for the entire call and must not run a mutating writer concurrently.

## Package-Level Engine Cache

The convenience functions:

```go
untie.Layout(expr, fonts, style)
untie.LayoutContext(ctx, expr, fonts, style)
```

use a process-wide cache of prepared engines.

Properties:

- eight entries;
- LRU eviction;
- keyed by library version, full SHA-256 font-set identity, and effective
  limits;
- concurrent misses for one key share a single build;
- a builder canceled from waiting does not corrupt the cache;
- `ClearLayoutEngineCache()` drops entries;
- an in-flight build when clearing does not insert its result afterward;
- layout results are never cached.

This cache is for convenience and small applications. Prefer `NewEngine` when
the application has explicit renderer lifecycle ownership, a request-scoped
font set, or many simultaneous font configurations.

`ClearLayoutEngineCache` does not affect engines retained from `NewEngine`.

## Renderer Parse Cache

`NewRenderer` can retain a bounded LRU of immutable parsed expressions:

```go
renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{
	CacheEntries: 128,
})
```

Entries are keyed by source, dialect, mode, effective parse limits, and relevant
TeX macros. The cache is concurrency-safe and hard-bounded by entry count;
`CacheEntries <= 0` disables it. Every `Render` call still performs layout and
returns a new `Box`.

## Cache Keys

### Font Cache

The package-level engine cache uses the full `FontSet.KeySHA256()`. Identical
font bytes with a different host face ID are different entries. Replacing a
fallback or text face changes the key.

### Expression Key

`Expr.Key()` is a 64-bit convenience key over:

- library version;
- dialect;
- mode;
- source;
- effective parse-affecting limits;
- TeX host macro names, bodies, and arities.

Layout-only limits and Typst-ignored TeX macros do not participate. The key is
stable for equivalent maps independent of Go map iteration order. It is not a
cryptographic content hash. Use the source and version directly if a host needs
stronger identity.

### Version

`untie.Version()` currently returns `0.1.0+layout.1`. The layout revision is
part of the engine-cache contract. Do not use a cached engine across an
incompatible version without consulting `Version()`.

## Deadlock and Blocking Behavior

`LayoutContext` does not start background workers or require the caller to
drain channels. Cancellation is cooperative within the layout pass.

Concurrent cache misses can wait for another goroutine's engine build. The wait
honors the caller's context. An unrelated slow layout does not hold a lock over
the complete render path.

## Testing Concurrent Use

The repository verifies:

- concurrent shared-font layouts with multiple styles and dialects;
- deterministic canonical output across repeated concurrent calls;
- shared concurrent engine-cache builds;
- cancellation while waiting for a build;
- cache clearing around an in-flight build;
- font cache reads under concurrent goroutines.

See the evidence links below before inventing a weaker locking scheme around
`Engine` or `FontSet`.

## Evidence

- Engine lifecycle: [`layout.go`](../layout.go)
- Cache implementation: [`layout_cache.go`](../layout_cache.go)
- Concurrent style/layout tests: [`layout_lifecycle_test.go`](../layout_lifecycle_test.go)
- Font concurrent reads: [`font/font_test.go`](../font/font_test.go)
- Display clone: [`display/types.go`](../display/types.go)
- Canonical IR validation and clone: [`ir/validate.go`](../ir/validate.go),
  [`ir/clone.go`](../ir/clone.go)

# Performance Baseline

This document records measured performance, not a compatibility claim. The
numbers below are a provisional engineering baseline for detecting regressions.
They are not release guarantees and must not be compared across machines without
the same environment.

## Environment

- Date: 2026-11-02
- CPU: Apple M5 Pro
- OS/architecture: Darwin/arm64
- Go: 1.27.3
- `CGO_ENABLED=0`
- Font: tracked `NewCMMath-Book.otf`

## Command

```sh
CGO_ENABLED=0 go test -run '^$' \
  -bench 'Benchmark(Parse|LayoutContext|PreparedLayout|NewEngine|LayoutEngineCache)' \
  -benchmem -count=1 .
```

## Baseline

| Benchmark | Time | Allocated bytes | Allocations |
|---|---:|---:|---:|
| Parse TeX `x` | 940 ns | 1,833 B | 30 |
| Parse TeX medium | 5.22 us | 10,095 B | 147 |
| Parse TeX large | 130 us | 266,197 B | 3,673 |
| Parse Typst medium | 4.75 us | 10,823 B | 113 |
| Convenience layout `x`, 16 DIP | 4.88 us | 8,531 B | 36 |
| Convenience layout medium, 16 DIP | 29.6 us | 51,883 B | 264 |
| Convenience layout large, 16 DIP | 788 us | 1,406,866 B | 6,922 |
| Prepared layout `x`, 16 DIP | 4.70 us | 8,529 B | 36 |
| Prepared layout medium, 16 DIP | 30.0 us | 51,874 B | 264 |
| Prepared layout large, 16 DIP | 787 us | 1,406,651 B | 6,921 |
| New caller-owned engine | 2.15 us | 3,118 B | 13 |
| Engine cache key | 25.0 ns | 0 B | 0 |
| Engine cache hit | 48.1 ns | 0 B | 0 |

## Observations

- Convenience and prepared layout are intentionally close after the first engine
  is cached; caching engine construction removes a small fixed cost, not layout
  work.
- Large-formula cost is dominated by clone/flatten/output allocation. It is the
  first target for allocation reduction after correctness work.
- Layout results are not cached. Hosts should cache by expression, style, font
  revision, font set, and library/layout version.
- Benchmarks must not be used to justify skipped validation, mutable shared
  output, approximate metrics, or unbounded caches.

## Regression Policy

For a same-machine, same-toolchain comparison:

- investigate any throughput regression above 10 percent;
- investigate any allocation-count increase above 10 percent;
- require a focused reason and benchmark for a deliberate tradeoff;
- compare output correctness before optimizing;
- keep benchmark setup outside timed loops where appropriate;
- report command, environment, revision, and count.

Once the layout contract stabilizes, replace this provisional policy with
per-command budgets derived from representative host workloads.

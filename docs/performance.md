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
| Parse TeX `x` | 1.00 us | 1,833 B | 30 |
| Parse TeX medium | 5.60 us | 10,111 B | 147 |
| Parse TeX large | 140 us | 266,838 B | 3,673 |
| Parse Typst medium | 4.98 us | 10,839 B | 113 |
| Convenience layout `x`, 16 DIP | 5.14 us | 9,011 B | 40 |
| Convenience layout medium, 16 DIP | 32.1 us | 53,694 B | 288 |
| Convenience layout large, 16 DIP | 826 us | 1,451,376 B | 7,307 |
| Prepared layout `x`, 16 DIP | 5.06 us | 9,009 B | 40 |
| Prepared layout medium, 16 DIP | 31.8 us | 53,683 B | 288 |
| Prepared layout large, 16 DIP | 817 us | 1,451,147 B | 7,306 |
| New caller-owned engine | 2.20 us | 3,118 B | 13 |
| Engine cache key | 25.7 ns | 0 B | 0 |
| Engine cache hit | 50.1 ns | 0 B | 0 |

## Observations

- Convenience and prepared layout are intentionally close after the first engine
  is cached; caching engine construction removes a small fixed cost, not layout
  work.
- Public layout now also derives bounded alternative text, adding four small
  allocations to the simple layout path. Large-formula cost remains dominated
  by clone, fragment flattening, and output allocation.
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

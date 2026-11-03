# Migrating To 0.4

Untie 0.4 adds editor geometry and explicit semantic identity without changing
the existing `At`, `Covering`, tag, display-line, or source-map contracts. The
runtime version is `0.4.0+layout.3`.

## Caret Stops

`Box.CaretStops` stores source-ordered caret geometry for semantic leaves and
shaped text clusters. Use:

```go
carets := box.Carets()
caret, ok := box.CaretAt(sourceByteOffset)
```

`Carets` returns an independent sorted snapshot. `CaretAt` prefers an exact
proven source boundary and otherwise chooses the nearest boundary.

The displays are in final box coordinates, including constrained display line
baselines and alignment offsets. Editors should not recover caret positions by
guessing from `Hit.Area`, because hits represent occupied regions rather than
positions between atoms.

## Inline Splitting

An untagged inline box can now be divided at one of its own breaks:

```go
left, right, err := box.Split(box.Breaks[index])
if errors.Is(err, untie.ErrCannotSplit) {
	// Keep the unsplit inline object.
}
```

The split operation:

- validates ownership of the requested break;
- copies and remaps path storage independently;
- clips hit rectangles to each side;
- keeps boundary caret stops on both sides;
- translates the right box to a zero origin;
- preserves item and hit `NodeID` values;
- clears `Alt`.

Do not split constrained display boxes or tagged boxes. Those contracts retain
line and tag ownership through `Style.MaxWidth`.

## Stable Node Identity

`ir.Base` now carries `NodeID`, also exposed as `untie.NodeID`. Zero means
unassigned. Nonzero IDs must be unique within a formula.

Identity is explicit:

- `CloneFormula` preserves assigned IDs;
- validation rejects duplicate IDs on different nodes;
- canonical JSON includes assigned IDs;
- layout copies the source node ID into every emitted `Hit` and `Item`;
- a host that edits source and rebuilds a tree must preserve IDs on the
  semantically corresponding nodes.

Untie does not infer IDs from source spans, tree indexes, or pointers. It does
not provide `Expr.Edit` or incremental parsing in 0.4. Automatic cross-edit
identity would require a syntax-preserving edit model that does not exist yet.

## Text Cluster Direction

`font.ShapedCluster` now includes an `RTL` flag. It records whether shaping
produced that cluster in a right-to-left run. The internal layout text-source
port carries the same information; external code should use
`FontSet.ShapeText`, which populates it.

## Version and Cache Schema

- `untie.Version()` is now `0.4.0+layout.3`;
- the process-wide engine cache schema is `3`.

Drop persisted layout or display caches created before 0.4.

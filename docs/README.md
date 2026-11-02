# Untie Documentation

Untie is a pure-Go math renderer for TeX and Typst math. Its target pipeline is
two dialect frontends, one language-neutral IR, an OpenType MATH layout engine,
and a UI-independent display list.

The project is under active implementation. The public renderer is not
end-to-end complete at this snapshot. Read [status.md](status.md) before using a
package or claiming feature support.

## Documents

| Document | Purpose |
|---|---|
| [architecture.md](architecture.md) | Actual package boundaries, dependency direction, data contracts, and architectural regression rules |
| [status.md](status.md) | Implemented behavior, current gaps, and scope boundary |
| [upstream.md](upstream.md) | Pinned source map, confirmation status, imported fixture locations, and update procedure |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Development commands, fixture policy, commit style, worktree coordination, and review gates |
| [../THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md) | Third-party licenses, notices, generated data provenance, and owner actions |

## Source Of Truth

Use this order when evaluating a change:

1. Current tracked code and tests.
2. The documents above.
3. Pinned upstream sources recorded in [upstream.md](upstream.md).
4. Tracked fixtures and generated-data provenance.

The ignored `references/` checkout is useful for static source inspection but is
never a build or test dependency. It must not be compiled or executed.

## Quick Verification

```sh
nix develop
gofmt -l .
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...
```

`gofmt -l .` should print nothing.

## Documentation Maintenance

- Describe implemented behavior as present tense and planned behavior as
  planned.
- Do not remove a known gap without linked code or tests.
- Update `status.md`, `upstream.md`, and `THIRD_PARTY_NOTICES.md` with the code
  change that makes them stale.
- Keep prose ASCII and use tables for exact pins, counts, licenses, and status.
- Do not use an upstream project name to imply endorsement.

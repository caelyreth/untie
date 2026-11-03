# Releasing

Untie releases use annotated Git tags on the `main` branch. The root module and
the public `contrib/mygo` module are versioned together.

## Prepare

1. Start from a clean `main` branch synchronized with `origin/main`.
2. Update `libraryVersion` in [`version.go`](../version.go).
3. Update version references in `README.md`, `docs/`, `contrib/mygo/go.mod`,
   and `examples/mygo/go.mod`.
4. Run the complete local gate described in [`CONTRIBUTING.md`](../CONTRIBUTING.md).
5. Review the changelog output locally:

   ```sh
   npx --yes changelogithub@15.0.5 --to v0.2.1 --dry
   ```

The release notes come from one-line Conventional Commit subjects. The
[`changelogithub.config.mjs`](../changelogithub.config.mjs) file disables emoji
and excludes nested module tags from root release changelogs.

## Tag

Create both module tags on the same commit:

```sh
git tag -a v0.2.1 -m "v0.2.1"
git tag -a contrib/mygo/v0.2.1 -m "contrib/mygo/v0.2.1"
git push origin v0.2.1 contrib/mygo/v0.2.1
```

The `v*` tag starts the release workflow. It first runs the complete reusable
CI suite, then verifies that both module tags point at the release commit and
that nested module requirements match `untie.Version()`. Only after those checks
pass does it publish the GitHub Release with generated changelog notes.

Do not move or reuse a published tag. Publish a new patch version instead.

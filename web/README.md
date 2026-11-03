# Untie Web

The public documentation site and browser playground live here. The project
uses Bun, SvelteKit, Svelte 5 runes, Vite, oxlint, and oxfmt, matching the
Cosmofi toolchain and visual system.

## Develop

From the repository root:

```sh
nix develop
cd web
bun install
bun run dev
```

`bun run dev` builds the Go WebAssembly module before starting Vite.

## Build

```sh
cd web
PUBLIC_SITE_URL=https://your-domain.example bun run build
```

`PUBLIC_SITE_URL` sets the canonical origin used by canonical, Open Graph, and
Twitter metadata. It defaults to `https://untie.iki.moe`.

The build script:

1. compiles `web/wasm` for `js/wasm`;
2. strips the Go binary and runs `wasm-opt -Oz`;
3. copies `wasm_exec.js` and the New Computer Modern Math font;
4. prerenders all Markdown pages under `src/content/docs`;
5. writes the static site to `web/build`.

Generated files under `static/untie` and the static adapter output are ignored.

## Verify

```sh
bun run check
bun run lint
bun run build
```

The Wasm module exposes a small browser API for font initialization and
TeX/Typst-to-SVG rendering. The homepage formula field and playground both use
that same API and renderer.

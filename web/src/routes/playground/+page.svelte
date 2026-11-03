<script lang="ts">
  import { Play, RotateCcw } from 'lucide-svelte'
  import { onMount } from 'svelte'

  import { loadUntie } from '#lib/wasm/untie'

  type Dialect = 'tex' | 'typst'

  const samples: Record<Dialect, string> = {
    tex: String.raw`S = \int_{\mathcal{M}} \left(R - 2\Lambda\right)\sqrt{-g}\,\mathrm{d}^4x`,
    typst: String.raw`integral_M (R - 2 Lambda) sqrt(-g) dif^4 x`,
  }

  let dialect = $state<Dialect>('tex')
  let display = $state(true)
  let size = $state(32)
  let source = $state(samples.tex)
  let svg = $state('')
  let error = $state('')
  let loading = $state(true)
  let renderVersion = 0

  onMount(() => {
    void render()
  })

  function setDialect(next: Dialect) {
    dialect = next
    source = samples[next]
    void render()
  }

  async function render() {
    const version = renderVersion + 1
    renderVersion = version
    loading = true
    error = ''
    try {
      const untie = await loadUntie()
      const result = untie.renderSVG(source, dialect, display, size)
      applyResult(version, result)
    } catch (cause) {
      console.error('Untie Wasm:', cause)
      applyError(version, cause)
    } finally {
      if (version === renderVersion) {
        loading = false
      }
    }
  }

  function applyResult(
    version: number,
    result: { svg: string; error: string },
  ) {
    if (version !== renderVersion) {
      return
    }
    error = result.error
    svg = result.error ? '' : result.svg
  }

  function applyError(version: number, cause: unknown) {
    if (version !== renderVersion) {
      return
    }
    error = cause instanceof Error ? cause.message : String(cause)
    svg = ''
  }
</script>

<svelte:head>
  <title>Playground — Untie</title>
  <meta
    name="description"
    content="Try Untie's TeX and Typst math renderer directly in the browser."
  />
</svelte:head>

<main id="main" class="playground">
  <header class="playground-head">
    <div>
      <span class="eyebrow">Browser playground / Untie Wasm</span>
      <h1>Write math.<br />See the display list.</h1>
    </div>
  </header>

  <div class="workbench">
    <section class="editor-panel" aria-labelledby="source-title">
      <div class="panel-head">
        <h2 id="source-title">Source</h2>
        <div class="dialect-tabs" role="group" aria-label="Input syntax">
          <button
            class:active={dialect === 'tex'}
            aria-pressed={dialect === 'tex'}
            onclick={() => setDialect('tex')}>TeX</button
          >
          <button
            class:active={dialect === 'typst'}
            aria-pressed={dialect === 'typst'}
            onclick={() => setDialect('typst')}>Typst</button
          >
        </div>
      </div>
      <textarea
        bind:value={source}
        aria-label="Math source"
        spellcheck="false"></textarea>
      <div class="editor-controls">
        <div class="mode-toggle" role="group" aria-label="Layout mode">
          <button
            class:active={display}
            aria-pressed={display}
            onclick={() => {
              display = true
              void render()
            }}>Display</button
          >
          <button
            class:active={!display}
            aria-pressed={!display}
            onclick={() => {
              display = false
              void render()
            }}>Inline</button
          >
        </div>
        <label>
          <span>Size {size}</span>
          <input type="range" min="16" max="64" bind:value={size} />
        </label>
        <button class="run" onclick={() => void render()}>
          <Play size={14} fill="currentColor" aria-hidden="true" />
          Render
        </button>
      </div>
    </section>

    <section class="preview-panel" aria-labelledby="preview-title">
      <div class="panel-head">
        <h2 id="preview-title">Preview</h2>
        <div class="status">
          {#if loading}
            <span class="spin"
              ><RotateCcw size={13} aria-hidden="true" /></span
            >
            Initializing Wasm
          {:else}
            <i aria-hidden="true"></i>
            {dialect} / {display ? 'display' : 'inline'}
          {/if}
        </div>
      </div>
      <div class="preview-surface">
        {#if error}
          <p class="error">{error}</p>
        {:else if svg}
          <div class="formula">{@html svg}</div>
        {:else}
          <p class="waiting">Preparing the renderer…</p>
        {/if}
      </div>
    </section>
  </div>
</main>

<style>
  .playground {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    gap: 24px;
    min-height: 100dvh;
    padding: 38px var(--page-inset) calc(var(--nav-clearance) + 18px);
  }

  .playground-head {
    display: flex;
    align-items: end;
    justify-content: space-between;
    gap: 24px;
  }

  h1 {
    margin-top: 5px;
    font-family: var(--font-brand);
    font-size: clamp(30px, 4vw, 48px);
    font-weight: 500;
    letter-spacing: -0.035em;
    line-height: 0.98;
  }

  .workbench {
    display: grid;
    grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr);
    min-height: 0;
    border-top: 1px solid var(--line);
  }

  section {
    display: grid;
    min-width: 0;
    min-height: 0;
  }

  .editor-panel {
    grid-template-rows: 48px minmax(260px, 1fr) auto;
    border-right: 1px solid var(--line);
  }

  .preview-panel {
    grid-template-rows: 48px minmax(260px, 1fr);
  }

  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    min-width: 0;
    padding: 0 14px;
    border-bottom: 1px solid var(--line);
    background: transparent;
  }

  h2,
  .status {
    font-family: var(--font-code);
    font-size: 10px;
    font-weight: 400;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .status {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--secondary);
  }

  .status i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--life);
  }

  .spin :global(svg) {
    animation: spin 1s linear infinite;
  }

  .dialect-tabs,
  .mode-toggle {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .dialect-tabs button,
  .mode-toggle button {
    min-height: 30px;
    padding: 0 10px;
    color: var(--muted);
    font-family: var(--font-code);
    font-size: 9px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .dialect-tabs button.active,
  .mode-toggle button.active {
    color: var(--ink);
    background: var(--raised);
  }

  textarea {
    width: 100%;
    min-height: 0;
    padding: 20px;
    resize: none;
    border: 0;
    border-radius: 0;
    outline: 0;
    color: var(--ink);
    background: var(--page);
    font-family: var(--font-code);
    font-size: 12px;
    line-height: 1.75;
    tab-size: 2;
  }

  .editor-controls {
    display: grid;
    grid-template-columns: auto minmax(120px, 1fr) auto;
    align-items: center;
    gap: 18px;
    padding: 12px 14px;
    border-top: 1px solid var(--line);
    background: transparent;
  }

  label {
    display: grid;
    grid-template-columns: auto minmax(70px, 1fr);
    align-items: center;
    gap: 12px;
    color: var(--secondary);
    font-family: var(--font-code);
    font-size: 9px;
    text-transform: uppercase;
  }

  input[type='range'] {
    width: 100%;
    accent-color: var(--halo);
  }

  .run {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-height: 38px;
    padding: 0 14px;
    color: var(--page);
    background: var(--ink);
    font-family: var(--font-code);
    font-size: 10px;
    text-transform: uppercase;
  }

  .preview-surface {
    display: grid;
    min-height: 0;
    padding: clamp(24px, 4vw, 56px);
    overflow: auto;
    place-items: center;
    background: var(--page);
  }

  .formula {
    width: min(100%, 720px);
    color: var(--ink);
  }

  .formula :global(svg) {
    display: block;
    width: 100%;
    height: auto;
  }

  .error,
  .waiting {
    max-width: 44ch;
    color: var(--secondary);
    font-family: var(--font-code);
    font-size: 11px;
    line-height: 1.7;
    text-align: center;
  }

  .error {
    color: var(--afterglow);
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (max-width: 900px) {
    .workbench {
      grid-template-columns: 1fr;
    }

    .editor-panel {
      border-right: 0;
      border-bottom: 1px solid var(--line);
    }

    .playground {
      min-height: auto;
    }
  }

  @media (max-width: 600px) {
    .playground {
      padding-top: 24px;
      padding-bottom: calc(var(--nav-clearance) + 10px);
    }

    .playground-head {
      align-items: start;
      flex-direction: column;
      gap: 16px;
    }

    .editor-controls {
      grid-template-columns: 1fr auto;
    }

    .editor-controls label {
      display: none;
    }

    .preview-surface {
      min-height: 300px;
    }
  }
</style>

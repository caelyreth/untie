<script lang="ts">
  import { LoaderCircle } from '@lucide/svelte'

  import Seo from '#lib/components/seo.svelte'
  import { loadUntie } from '#lib/wasm/untie'

  type Dialect = 'tex' | 'typst'

  const renderDelay = 180

  const samples: Record<Dialect, string> = {
    tex: String.raw`S = \int_{\mathcal{M}} \left(R - 2\Lambda\right)\sqrt{-g}\,\mathrm{d}^4x`,
    typst: String.raw`integral_M (R - 2 Lambda) sqrt(-g) dif^4 x`,
  }

  let dialect = $state<Dialect>('tex')
  let display = $state(true)
  let source = $state(samples.tex)
  let svg = $state('')
  let error = $state('')
  let loading = $state(true)
  let renderVersion = 0

  $effect(() => {
    const nextSource = source
    const nextDialect = dialect
    const nextDisplay = display
    const timer = setTimeout(() => {
      void render(nextSource, nextDialect, nextDisplay)
    }, renderDelay)

    return () => clearTimeout(timer)
  })

  function setDialect(next: Dialect) {
    dialect = next
    source = samples[next]
  }

  async function render(
    nextSource: string,
    nextDialect: Dialect,
    nextDisplay: boolean,
  ) {
    const version = renderVersion + 1
    renderVersion = version
    loading = true
    error = ''
    try {
      const untie = await loadUntie()
      const result = untie.renderSVG(nextSource, nextDialect, nextDisplay)
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

<Seo
  title="Playground"
  description="Try Untie's TeX and Typst math renderer directly in the browser, with live rendering and automatic updates."
  path="/playground/"
/>

<main id="main" class="playground">
  <header class="playground-head">
    <h1>Playground</h1>
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
            onclick={() => (display = true)}>Display</button
          >
          <button
            class:active={!display}
            aria-pressed={!display}
            onclick={() => (display = false)}>Inline</button
          >
        </div>
        <span class="auto-note">Auto-renders while you type</span>
      </div>
    </section>

    <section class="preview-panel" aria-labelledby="preview-title">
      <div class="panel-head">
        <h2 id="preview-title">Preview</h2>
        <div class="status" aria-live="polite">
          {#if loading}
            <span class="spin"
              ><LoaderCircle size={13} aria-hidden="true" /></span
            >
            Rendering
          {:else if error}
            <i class="error-dot" aria-hidden="true"></i>
            Error
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
    gap: 20px;
    width: min(100%, 1500px);
    min-height: 100dvh;
    margin-inline: auto;
    padding: clamp(28px, 4vw, 52px) var(--page-inset)
      calc(var(--nav-clearance) + 18px);
  }

  .playground-head {
    display: flex;
    align-items: flex-end;
  }

  h1 {
    font-family: var(--font-brand);
    font-size: clamp(42px, 6vw, 68px);
    font-weight: 500;
    letter-spacing: -0.045em;
    line-height: 0.9;
  }

  .workbench {
    display: grid;
    grid-template-columns: minmax(320px, 0.9fr) minmax(0, 1.1fr);
    min-height: 0;
    border: 1px solid var(--line);
    overflow: hidden;
    background: var(--panel);
  }

  section {
    display: grid;
    min-width: 0;
    min-height: 0;
    background: var(--page);
  }

  .editor-panel {
    grid-template-rows: 52px minmax(280px, 1fr) auto;
    border-right: 1px solid var(--line);
  }

  .preview-panel {
    grid-template-rows: 52px minmax(280px, 1fr);
  }

  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    min-width: 0;
    padding: 0 16px;
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--panel) 76%, var(--page));
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

  .status .error-dot {
    background: var(--afterglow);
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
    font-family: var(--font-body);
    font-size: 13px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    transition:
      color 160ms ease,
      background 160ms ease,
      transform 120ms ease;
  }

  .dialect-tabs button:hover,
  .mode-toggle button:hover {
    color: var(--ink);
  }

  .dialect-tabs button:active,
  .mode-toggle button:active {
    transform: scale(0.96);
  }

  .dialect-tabs button.active,
  .mode-toggle button.active {
    color: var(--ink);
    background: var(--track);
  }

  textarea {
    width: 100%;
    min-height: 0;
    padding: clamp(20px, 2.6vw, 32px);
    resize: none;
    border: 0;
    border-radius: 0;
    outline: 0;
    color: var(--ink);
    background: var(--page);
    font-family: var(--font-code);
    font-size: 13px;
    line-height: 1.8;
    tab-size: 2;
  }

  .editor-controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    min-height: 52px;
    padding: 10px 16px;
    border-top: 1px solid var(--line);
    background: color-mix(in srgb, var(--panel) 76%, var(--page));
  }

  .auto-note {
    color: var(--muted);
    font-family: var(--font-code);
    font-size: 9px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .preview-surface {
    display: grid;
    min-height: 0;
    padding: clamp(24px, 4vw, 56px);
    overflow: auto;
    place-items: center;
    background:
      linear-gradient(var(--line) 1px, transparent 1px),
      linear-gradient(90deg, var(--line) 1px, transparent 1px), var(--page);
    background-size: 36px 36px;
  }

  .formula {
    display: grid;
    width: min(100%, 900px);
    height: 100%;
    max-height: 100%;
    color: var(--ink);
    place-items: center;
  }

  .formula :global(svg) {
    display: block;
    width: auto;
    height: auto;
    max-width: 100%;
    max-height: 100%;
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

  @media (min-width: 901px) {
    .workbench {
      height: clamp(520px, 68svh, 760px);
    }

    .editor-panel {
      grid-template-rows: 52px minmax(0, 1fr) auto;
    }

    .preview-panel {
      grid-template-rows: 52px minmax(0, 1fr);
    }
  }

  @media (max-width: 900px) {
    .workbench {
      grid-template-columns: 1fr;
    }

    .editor-panel {
      grid-template-rows: 52px minmax(280px, 44svh) auto;
      border-right: 0;
      border-bottom: 1px solid var(--line);
    }

    .preview-panel {
      grid-template-rows: 52px minmax(340px, 52svh);
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

    .editor-controls {
      flex-wrap: wrap;
    }

    .auto-note {
      display: none;
    }

    .preview-surface {
      min-height: 300px;
    }
  }
</style>

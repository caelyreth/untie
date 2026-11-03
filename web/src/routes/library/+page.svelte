<script lang="ts">
  import { LoaderCircle, Search } from '@lucide/svelte'
  import { onMount } from 'svelte'

  import Seo from '#lib/components/seo.svelte'
  import type {
    LibraryCase,
    LibraryDialect,
    LibraryMode,
  } from '#lib/library/types'
  import { loadUntie } from '#lib/wasm/untie'

  import type { PageProps } from './$types'

  type DialectFilter = 'all' | LibraryDialect
  type ModeFilter = 'all' | LibraryMode

  let { data }: PageProps = $props()
  let query = $state('')
  let dialect = $state<DialectFilter>('all')
  let mode = $state<ModeFilter>('all')
  let category = $state('all')
  let previews = $state<Record<string, string>>({})
  let previewErrors = $state<Record<string, string>>({})
  let loading = $state(true)
  let loadError = $state('')

  const filtered = $derived.by(() => data.cases.filter(matchesFilters))
  const filtersActive = $derived(
    Boolean(query) ||
      dialect !== 'all' ||
      mode !== 'all' ||
      category !== 'all',
  )

  onMount(() => {
    let disposed = false
    void loadUntie()
      .then((untie) => {
        const rendered: Record<string, string> = {}
        const failed: Record<string, string> = {}
        for (const testCase of data.cases) {
          const result = untie.renderSVG(
            testCase.source,
            testCase.dialect,
            testCase.mode === 'display',
          )
          if (result.error) {
            failed[testCase.id] = result.error
          } else {
            rendered[testCase.id] = result.svg
          }
        }
        if (!disposed) {
          previews = rendered
          previewErrors = failed
          loading = false
        }
      })
      .catch((cause) => {
        if (!disposed) {
          loadError = cause instanceof Error ? cause.message : String(cause)
          loading = false
        }
      })
    return () => {
      disposed = true
    }
  })

  function clearFilters() {
    query = ''
    dialect = 'all'
    mode = 'all'
    category = 'all'
  }

  function matchesFilters(testCase: LibraryCase) {
    const search = query.trim().toLowerCase()
    return (
      matchesFacet(testCase.dialect, dialect) &&
      matchesFacet(testCase.mode, mode) &&
      (category === 'all' || testCase.category === category) &&
      matchesSearch(testCase, search)
    )
  }

  function matchesFacet<T extends string>(value: T, selected: 'all' | T) {
    return selected === 'all' || value === selected
  }

  function matchesSearch(testCase: LibraryCase, search: string) {
    if (!search) {
      return true
    }
    return [
      testCase.label,
      testCase.source,
      testCase.categoryLabel,
      testCase.dialect,
      testCase.mode,
    ].some((value) => value.toLowerCase().includes(search))
  }
</script>

<Seo
  title="Formula Library"
  description="Browse the formulas Untie verifies across TeX and Typst, with live renderer previews."
  path="/library/"
/>

<main id="main" class="library">
  <header class="library-head">
    <div>
      <p class="eyebrow">Daily corpus / live Wasm</p>
      <h1>Formula Library</h1>
    </div>
    <p class="intro">
      Browse formulas verified across both dialects. Every preview is
      rendered by the same Go engine that powers the playground.
    </p>
  </header>

  <section class="library-toolbar" aria-label="Library filters">
    <label class="search">
      <Search size={15} strokeWidth={2} aria-hidden="true" />
      <span class="sr-only">Search formulas</span>
      <input
        bind:value={query}
        placeholder="Search formulas, syntax, or category"
        type="search"
      />
    </label>

    <div class="filter-group" role="group" aria-label="Dialect">
      {#each ['all', 'tex', 'typst'] as value}
        <button
          class:active={dialect === value}
          aria-pressed={dialect === value}
          onclick={() => (dialect = value as DialectFilter)}
        >
          {value === 'all' ? 'All' : value === 'tex' ? 'TeX' : 'Typst'}
        </button>
      {/each}
    </div>

    <div class="filter-group" role="group" aria-label="Layout mode">
      {#each ['all', 'display', 'inline'] as value}
        <button
          class:active={mode === value}
          aria-pressed={mode === value}
          onclick={() => (mode = value as ModeFilter)}
        >
          {value === 'all' ? 'All modes' : value}
        </button>
      {/each}
    </div>

    <label class="category">
      <span class="sr-only">Category</span>
      <select bind:value={category}>
        <option value="all">All categories</option>
        {#each data.categories as item}
          <option value={item.value}>{item.label} ({item.count})</option>
        {/each}
      </select>
    </label>

    {#if filtersActive}
      <button class="clear" onclick={clearFilters}>Clear</button>
    {/if}
  </section>

  <div class="library-summary" aria-live="polite">
    <span>{filtered.length} formulas</span>
    <span>{data.categories.length} categories</span>
    <span>{loading ? 'Rendering previews…' : 'Previews ready'}</span>
  </div>

  {#if loadError}
    <p class="load-error">{loadError}</p>
  {:else if filtered.length === 0}
    <div class="empty">
      <p>No formulas match these filters.</p>
      <button onclick={clearFilters}>Reset filters</button>
    </div>
  {:else}
    <section class="formula-grid" aria-label="Formula results">
      {#each filtered as testCase (testCase.id)}
        {@render FormulaCard({
          testCase,
          svg: previews[testCase.id],
          error: previewErrors[testCase.id],
          loading,
        })}
      {/each}
    </section>
  {/if}
</main>

{#snippet FormulaCard({
  testCase,
  svg,
  error,
  loading,
}: {
  testCase: LibraryCase
  svg?: string
  error?: string
  loading: boolean
})}
  <article class="formula-card">
    <div class="formula-preview" aria-hidden="true">
      {#if svg}
        <div class="formula">{@html svg}</div>
      {:else if error}
        <p class="preview-error">Render unavailable</p>
      {:else if loading}
        <span class="spin"
          ><LoaderCircle size={18} aria-hidden="true" /></span
        >
      {:else}
        <p class="preview-error">No preview</p>
      {/if}
    </div>
    <div class="formula-copy">
      <div class="formula-meta">
        <span>{testCase.categoryLabel}</span>
        <span>{testCase.dialect} / {testCase.mode}</span>
      </div>
      <h2>{testCase.label}</h2>
      <code>{testCase.source}</code>
    </div>
  </article>
{/snippet}

<style>
  .library {
    width: 100%;
    min-height: 100dvh;
    padding: clamp(32px, 5vw, 64px) var(--page-inset)
      calc(var(--nav-clearance) + 28px);
  }

  .library-head {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(260px, 430px);
    align-items: end;
    gap: 28px;
    padding-bottom: 24px;
    border-bottom: 1px solid var(--line);
  }

  h1 {
    margin-top: 8px;
    color: var(--ink);
    font-family: var(--font-brand);
    font-size: clamp(48px, 7vw, 96px);
    font-weight: 500;
    letter-spacing: -0.055em;
    line-height: 0.86;
  }

  .library-head > p {
    max-width: 52ch;
    color: var(--secondary);
    font-size: 15px;
    line-height: 1.65;
  }

  .library-toolbar {
    position: sticky;
    top: 0;
    z-index: 20;
    display: flex;
    align-items: center;
    gap: 10px;
    margin-inline: calc(-1 * var(--page-inset));
    padding: 14px var(--page-inset);
    border-bottom: 1px solid var(--line);
    background: var(--page);
    flex-wrap: wrap;
  }

  .search {
    position: relative;
    display: flex;
    flex: 1 1 280px;
    align-items: center;
    gap: 9px;
    min-width: min(100%, 260px);
    min-height: 42px;
    padding-inline: 13px;
    border: 1px solid var(--line);
    background: var(--panel);
    color: var(--muted);
  }

  .search:focus-within {
    border-color: var(--halo);
  }

  input,
  select {
    min-width: 0;
    border: 0;
    outline: 0;
    color: var(--ink);
    background: transparent;
    font-family: var(--font-body);
    font-size: 13px;
  }

  input {
    width: 100%;
    min-height: 40px;
  }

  input::placeholder {
    color: var(--muted);
  }

  .filter-group {
    display: flex;
    gap: 3px;
    padding: 3px;
    border: 1px solid var(--line);
    background: var(--panel);
  }

  .filter-group button,
  .clear,
  .empty button {
    min-height: 34px;
    padding-inline: 10px;
    color: var(--muted);
    font-family: var(--font-body);
    font-size: 12px;
    font-weight: 600;
    text-transform: capitalize;
  }

  .filter-group button:hover,
  .clear:hover,
  .empty button:hover {
    color: var(--ink);
  }

  .filter-group button.active {
    color: var(--page);
    background: var(--halo);
  }

  .category {
    min-height: 42px;
    display: flex;
    align-items: center;
    padding-inline: 11px;
    border: 1px solid var(--line);
    background: var(--panel);
  }

  select {
    min-height: 40px;
    color: var(--secondary);
  }

  option {
    color: var(--ink);
    background: var(--panel);
  }

  .clear {
    border: 1px solid var(--line);
  }

  .library-summary {
    display: flex;
    gap: 8px 20px;
    padding-block: 12px;
    color: var(--muted);
    font-family: var(--font-code);
    font-size: 9px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    flex-wrap: wrap;
  }

  .formula-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 330px), 1fr));
    gap: 10px;
  }

  .formula-card {
    display: grid;
    grid-template-rows: minmax(168px, auto) auto;
    min-width: 0;
    border: 1px solid var(--line);
    background: var(--panel);
    transition:
      border-color 180ms ease,
      transform 180ms ease;
  }

  .formula-card:hover {
    border-color: color-mix(in srgb, var(--halo) 62%, var(--line));
    transform: translateY(-2px);
  }

  .formula-preview {
    display: grid;
    min-width: 0;
    min-height: 168px;
    padding: 20px;
    overflow: hidden;
    background:
      linear-gradient(var(--line) 1px, transparent 1px),
      linear-gradient(90deg, var(--line) 1px, transparent 1px), var(--page);
    background-size: 36px 36px;
    place-items: center;
  }

  .formula {
    width: min(100%, 520px);
    color: var(--ink);
  }

  .formula :global(svg) {
    display: block;
    width: 100%;
    max-height: 132px;
    height: auto;
  }

  .formula-copy {
    min-width: 0;
    padding: 15px 16px 17px;
    border-top: 1px solid var(--line);
  }

  .formula-meta {
    display: flex;
    justify-content: space-between;
    gap: 14px;
    color: var(--muted);
    font-family: var(--font-code);
    font-size: 8px;
    letter-spacing: 0.07em;
    text-transform: uppercase;
  }

  h2 {
    margin-top: 8px;
    color: var(--ink);
    font-family: var(--font-brand);
    font-size: 21px;
    font-weight: 600;
    letter-spacing: -0.03em;
    line-height: 1.1;
  }

  .formula-copy > code {
    display: block;
    margin-top: 10px;
    overflow: hidden;
    color: var(--secondary);
    font-family: var(--font-code);
    font-size: 10px;
    line-height: 1.55;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .spin {
    display: grid;
    color: var(--secondary);
    animation: spin 900ms linear infinite;
  }

  .preview-error,
  .load-error,
  .empty {
    color: var(--secondary);
    font-family: var(--font-code);
    font-size: 10px;
    line-height: 1.6;
  }

  .load-error,
  .empty {
    padding: 40px 0;
  }

  .empty button {
    margin-top: 12px;
    border: 1px solid var(--line);
    color: var(--halo);
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (max-width: 760px) {
    .library-head {
      grid-template-columns: 1fr;
      align-items: start;
      gap: 18px;
    }

    .library-head > p {
      font-size: 14px;
    }

    .search {
      flex-basis: 100%;
    }

    .formula-grid {
      grid-template-columns: 1fr;
    }
  }
</style>

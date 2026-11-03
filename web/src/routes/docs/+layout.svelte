<script lang="ts">
  import { page } from '$app/state'

  import type { LayoutProps } from './$types'

  let { children, data }: LayoutProps = $props()
  const current = $derived(page.url.pathname)
</script>

<main id="main" class="docs-shell">
  <aside>
    <span class="eyebrow">Documentation</span>
    <nav aria-label="Documentation">
      {#each data.docs as doc}
        <a
          href={doc.href}
          aria-current={current === doc.href ? 'page' : undefined}
        >
          {doc.label}
        </a>
      {/each}
    </nav>
  </aside>
  <div class="docs-content">
    {@render children()}
  </div>
</main>

<style>
  .docs-shell {
    display: grid;
    grid-template-columns: minmax(190px, 240px) minmax(0, 820px);
    justify-content: center;
    gap: clamp(34px, 7vw, 110px);
    min-height: 100dvh;
    padding: 42px var(--page-inset) calc(var(--nav-clearance) + 26px);
  }

  aside {
    position: sticky;
    top: 30px;
    align-self: start;
    max-height: calc(100dvh - 60px);
    overflow: auto;
    scrollbar-width: none;
    -ms-overflow-style: none;
    user-select: none;
  }

  aside::-webkit-scrollbar {
    display: none;
  }

  nav {
    display: grid;
    margin-top: 14px;
    border-top: 1px solid var(--line);
  }

  nav a {
    padding: 9px 0;
    border-bottom: 1px solid
      color-mix(in srgb, var(--line) 62%, transparent);
    color: var(--secondary);
    font-size: 13px;
    line-height: 1.45;
    text-decoration: none;
  }

  nav a:hover,
  nav a[aria-current='page'] {
    color: var(--halo);
  }

  .docs-content {
    min-width: 0;
  }

  @media (max-width: 820px) {
    .docs-shell {
      grid-template-columns: minmax(0, 1fr);
      gap: 32px;
      padding-top: 28px;
    }

    aside {
      position: static;
      max-height: none;
    }

    nav {
      display: flex;
      gap: 18px;
      overflow-x: auto;
      padding-bottom: 10px;
    }

    nav a {
      flex: 0 0 auto;
      border: 0;
      white-space: nowrap;
    }
  }
</style>

<script lang="ts">
  import { page } from '$app/state'

  import type { DocLink } from '#lib/docs/types'

  import type { LayoutProps } from './$types'

  let { children, data }: LayoutProps = $props()
  const current = $derived(page.url.pathname)
  const sections = $derived(groupDocs(data.docs))

  interface DocSection {
    docs: DocLink[]
    label: string
  }

  function groupDocs(docs: readonly DocLink[]) {
    const sections: DocSection[] = []
    for (const doc of docs) {
      let section = sections.find(
        (candidate) => candidate.label === doc.section,
      )
      if (!section) {
        section = { docs: [], label: doc.section }
        sections.push(section)
      }
      section.docs.push(doc)
    }
    return sections
  }
</script>

<main id="main" class="docs-shell">
  <aside>
    <span class="eyebrow">Documentation</span>
    <nav aria-label="Documentation">
      {#each sections as section}
        <div class="nav-section">
          <p class="section-label">{section.label}</p>
          <div class="section-links">
            {#each section.docs as doc}
              <a
                href={doc.href}
                aria-current={current === doc.href ? 'page' : undefined}
              >
                {doc.label}
              </a>
            {/each}
          </div>
        </div>
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
    grid-template-columns: minmax(190px, 230px) minmax(0, 860px);
    justify-content: center;
    align-items: start;
    gap: clamp(24px, 4vw, 54px);
    min-height: 100dvh;
    padding: 42px var(--page-inset) calc(var(--nav-clearance) + 26px);
  }

  aside {
    position: sticky;
    top: 30px;
    align-self: start;
    padding: 18px;
    border: 1px solid var(--line);
    background: color-mix(in srgb, var(--panel) 88%, var(--page));
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
    gap: 18px;
    margin-top: 12px;
  }

  .nav-section + .nav-section {
    padding-top: 16px;
    border-top: 1px solid var(--line);
  }

  .section-label {
    margin-bottom: 7px;
    color: var(--muted);
    font-family: var(--font-code);
    font-size: 8px;
    letter-spacing: 0.09em;
    line-height: 1.3;
    text-transform: uppercase;
  }

  .section-links {
    display: grid;
  }

  nav a {
    padding: 10px 2px;
    color: var(--secondary);
    font-size: 13px;
    font-weight: 600;
    line-height: 1.45;
    text-decoration: none;
  }

  nav a:hover,
  nav a[aria-current='page'] {
    color: var(--halo);
    text-decoration: underline;
    text-decoration-thickness: 1px;
    text-underline-offset: 0.2em;
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
      padding: 14px;
    }

    nav {
      display: flex;
      gap: 24px;
      overflow-x: auto;
      margin-top: 8px;
    }

    .nav-section {
      flex: none;
    }

    .nav-section + .nav-section {
      padding-top: 0;
      padding-left: 24px;
      border-top: 0;
      border-left: 1px solid var(--line);
    }

    .section-links {
      display: flex;
      gap: 14px;
    }

    nav a {
      flex: 0 0 auto;
      border: 0;
      padding-block: 6px;
      white-space: nowrap;
    }
  }
</style>

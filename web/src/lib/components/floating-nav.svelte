<script lang="ts">
  import { resolve } from '$app/paths'
  import { page } from '$app/state'
  import { ArrowUpRight } from 'lucide-svelte'

  import Wordmark from './wordmark.svelte'

  const links = [
    { href: resolve('/docs'), label: 'Docs', prefix: '/docs' },
    {
      href: resolve('/playground'),
      label: 'Playground',
      prefix: '/playground',
    },
  ] as const

  const route = $derived(page.route.id ?? '/')
</script>

<nav class="floating-nav" aria-label="Primary">
  <a class="brand" href={resolve('/')} aria-label="Untie home">
    <Wordmark symbol />
  </a>
  <span class="rule" aria-hidden="true"></span>
  {#each links as link}
    <a
      href={link.href}
      class="destination"
      aria-current={route.startsWith(link.prefix) ? 'page' : undefined}
    >
      {link.label}
    </a>
  {/each}
  <a
    class="github"
    href="https://github.com/caelyreth/untie"
    aria-label="Untie on GitHub"
  >
    <ArrowUpRight size={13} aria-hidden="true" />
  </a>
</nav>

<style>
  .floating-nav {
    position: fixed;
    bottom: max(28px, env(safe-area-inset-bottom));
    left: 50%;
    z-index: 50;
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 48px;
    padding: 5px;
    border: 1px solid color-mix(in srgb, var(--line) 86%, transparent);
    border-radius: 999px;
    background: color-mix(in srgb, var(--panel) 92%, transparent);
    backdrop-filter: blur(18px);
    transform: translateX(-50%);
    user-select: none;
    view-transition-name: floating-nav;
  }

  a {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 36px;
    padding-inline: 13px;
    border-radius: 999px;
    color: var(--secondary);
    font-family: var(--font-code);
    font-size: 9px;
    letter-spacing: 0.04em;
    line-height: 1;
    text-decoration: none;
    text-transform: uppercase;
    white-space: nowrap;
    transition:
      color 180ms ease,
      background 180ms ease;
  }

  a:hover {
    color: var(--ink);
  }

  a[aria-current='page'] {
    color: var(--page);
    background: var(--ink);
  }

  .brand {
    padding-inline: 12px;
    color: var(--ink);
    font-size: 20px;
  }

  .rule {
    width: 1px;
    height: 18px;
    margin-inline: 2px;
    background: var(--line);
  }

  .github {
    width: 36px;
    padding: 0;
    color: var(--ink);
  }

  @media (max-width: 520px) {
    .floating-nav {
      right: 14px;
      left: 14px;
      justify-content: space-between;
      transform: none;
    }

    .destination {
      padding-inline: 10px;
    }
  }
</style>

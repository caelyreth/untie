<script lang="ts">
  import { resolve } from '$app/paths'
  import { page } from '$app/state'
  import { BookOpen, FlaskConical, LibraryBig } from '@lucide/svelte'

  import GithubMark from './github-mark.svelte'
  import Wordmark from './wordmark.svelte'

  const links = [
    {
      href: resolve('/docs'),
      label: 'Docs',
      prefix: '/docs',
      icon: BookOpen,
    },
    {
      href: resolve('/library'),
      label: 'Library',
      prefix: '/library',
      icon: LibraryBig,
    },
    {
      href: resolve('/playground'),
      label: 'Playground',
      prefix: '/playground',
      icon: FlaskConical,
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
    {@const Icon = link.icon}
    <a
      href={link.href}
      class="destination"
      aria-label={link.label}
      aria-current={route.startsWith(link.prefix) ? 'page' : undefined}
    >
      <Icon size={14} strokeWidth={2} aria-hidden="true" />
      <span>{link.label}</span>
    </a>
  {/each}
  <a
    class="github"
    href="https://github.com/caelyreth/untie"
    aria-label="Untie on GitHub"
  >
    <GithubMark />
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
    padding: 5px 9px 5px 6px;
    border: 1px solid color-mix(in srgb, var(--line) 86%, transparent);
    border-radius: 999px;
    background: var(--panel);
    transform: translateX(-50%);
    user-select: none;
  }

  a {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-height: 36px;
    padding-inline: 13px;
    border-radius: 999px;
    color: var(--secondary);
    font-family: var(--font-body);
    font-size: 13px;
    font-weight: 600;
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

    .destination span {
      display: none;
    }
  }
</style>

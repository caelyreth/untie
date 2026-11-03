<script lang="ts">
  import { resolve } from '$app/paths'
  import { page } from '$app/state'
  import { BookOpen, FlaskConical } from '@lucide/svelte'

  import Wordmark from './wordmark.svelte'

  const links = [
    {
      href: resolve('/docs'),
      label: 'Docs',
      prefix: '/docs',
      icon: BookOpen,
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
    <svg
      class="github-mark"
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        d="M12 .3a12 12 0 0 0-3.8 23.4c.6.1.8-.3.8-.6v-2.2c-3.3.7-4-1.6-4-1.6-.6-1.4-1.4-1.8-1.4-1.8-1.1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1.1 1.8 2.8 1.3 3.5 1 .1-.8.4-1.3.8-1.6-2.7-.3-5.5-1.3-5.5-5.9 0-1.3.5-2.4 1.2-3.2-.1-.3-.5-1.5.1-3.2 0 0 1-.3 3.3 1.2a11.5 11.5 0 0 1 6 0c2.3-1.5 3.3-1.2 3.3-1.2.6 1.7.2 2.9.1 3.2.8.8 1.2 1.9 1.2 3.2 0 4.6-2.8 5.6-5.5 5.9.4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6A12 12 0 0 0 12 .3Z"
      />
    </svg>
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
    view-transition-name: floating-nav;
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

  .github-mark {
    width: 15px;
    height: 15px;
  }

  :global(:root[data-route-transition]) .floating-nav a {
    transition: none;
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

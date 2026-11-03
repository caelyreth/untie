<script lang="ts">
  import '../app.css'
  import favicon from '#lib/assets/favicon.svg'
  import FloatingNav from '#lib/components/floating-nav.svelte'
  import FloatingScrollbar from '#lib/components/floating-scrollbar.svelte'

  import type { LayoutProps } from './$types'

  let { children }: LayoutProps = $props()
</script>

<svelte:head>
  <link rel="icon" href={favicon} />
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link
    rel="preconnect"
    href="https://fonts.gstatic.com"
    crossorigin="anonymous"
  />
  <link
    rel="stylesheet"
    href="https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:wght@500;600&family=Manrope:wght@400;500;600&family=Martian+Mono:wght@400;500&display=swap"
  />
  <meta name="theme-color" content="#161616" />
</svelte:head>

<a class="skip-link" href="#main">Skip to content</a>
<div class="site-shell">
  <div class="route-content">
    {@render children()}
  </div>
</div>
<FloatingNav />
<FloatingScrollbar />

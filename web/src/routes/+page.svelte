<script lang="ts">
  import { ArrowUpRight } from '@lucide/svelte'

  import Seo from '#lib/components/seo.svelte'
  import Wordmark from '#lib/components/wordmark.svelte'
  import MathFlow from '#lib/home/math-flow.svelte'
</script>

<Seo
  title="One math renderer for TeX and Typst"
  description="Untie is a pure-Go renderer that turns TeX and Typst math into one UI-independent display list."
  path="/"
/>

<main id="main" class="home">
  <MathFlow />
  <section class="hero" aria-labelledby="untie-title">
    <h1 id="untie-title"><Wordmark /></h1>
    <p>
      <span>
        One math renderer for both
        <em class="unripe">TeX</em> and <em class="life">Typst</em>.
      </span>
      <span>
        <em class="afterglow">Parse once</em> and draw
        <em class="regolith">everywhere</em>, in pure Go.
      </span>
      <span
        >First-class <em class="halo">MyGo integration</em> for Markdown math
        and native drawables.</span
      >
    </p>
    <nav class="hero-actions" aria-label="Get started">
      <a href="/playground">
        Open playground
        <ArrowUpRight size={14} aria-hidden="true" />
      </a>
      <a href="/docs">
        Read docs
        <ArrowUpRight size={14} aria-hidden="true" />
      </a>
    </nav>
  </section>
</main>

<style>
  .home {
    position: relative;
    isolation: isolate;
    display: grid;
    min-height: 100dvh;
    padding: 5vh var(--page-inset) calc(var(--nav-clearance) + 2vh);
    place-items: center;
    overflow: hidden;
    background: var(--page);
  }

  .hero {
    position: relative;
    z-index: 2;
    display: grid;
    justify-items: center;
    max-width: 860px;
    text-align: center;
    user-select: none;
  }

  .hero > * {
    animation: reveal 850ms cubic-bezier(0.16, 1, 0.3, 1) both;
  }

  .hero h1 {
    animation-delay: 70ms;
  }

  .hero p {
    animation-delay: 150ms;
  }

  .hero-actions {
    animation-delay: 230ms;
  }

  h1 {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--ink);
    font-size: clamp(100px, 20vw, 244px);
    line-height: 0;
  }

  p {
    display: grid;
    justify-items: center;
    gap: 5px;
    margin-top: 4px;
    color: var(--ink);
    font-size: clamp(16px, 1.7vw, 20px);
    line-height: 1.45;
    user-select: text;
  }

  p em {
    font-style: normal;
    font-weight: 600;
  }

  p .unripe {
    color: var(--unripe);
  }

  p .life {
    color: var(--life);
  }

  p .afterglow {
    color: var(--afterglow);
  }

  p .regolith {
    color: var(--regolith);
  }

  p .halo {
    color: var(--halo);
  }

  .hero-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 28px;
    margin-top: 30px;
  }

  .hero-actions a {
    display: inline-flex;
    align-items: center;
    gap: 12px;
    min-height: 46px;
    border-bottom: 1px solid var(--line);
    color: var(--ink);
    font-family: var(--font-body);
    font-size: 13px;
    font-weight: 600;
    letter-spacing: 0.05em;
    text-decoration: none;
    text-transform: uppercase;
    transition:
      color 180ms ease,
      border-color 180ms ease;
  }

  .hero-actions a:hover {
    border-color: var(--halo);
    color: var(--halo);
  }

  @keyframes reveal {
    from {
      opacity: 0;
      transform: translateY(18px);
    }

    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @media (max-width: 560px) {
    .home {
      padding-top: 8vh;
    }

    h1 {
      font-size: clamp(88px, 28vw, 142px);
    }

    p {
      margin-top: 4px;
      font-size: 15px;
    }

    .hero-actions {
      gap: 22px;
      margin-top: 24px;
    }
  }
</style>

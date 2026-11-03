<script lang="ts">
  import { MarkdownDocument } from '@comark/svelte'

  import type { DocPage } from './types'

  let { doc }: { doc: DocPage } = $props()
</script>

<article class="docs-article">
  <header class="article-head">
    <p class="meta">
      <span>Documentation</span>
      <span>{doc.slug || 'overview'}</span>
    </p>
    <h1>{doc.title}</h1>
    <p class="summary">{doc.description}</p>
  </header>

  <div class="prose">
    <MarkdownDocument value={doc.document} />
  </div>

  <footer>
    <a href={doc.sourceHref}
      >Edit this page <span aria-hidden="true">↗</span></a
    >
  </footer>
</article>

<style>
  .docs-article {
    min-width: 0;
    border: 1px solid var(--line);
    background: var(--panel);
  }

  .article-head {
    padding: clamp(28px, 5vw, 54px) clamp(24px, 6vw, 68px)
      clamp(26px, 4vw, 42px);
    border-bottom: 1px solid var(--line);
    background: color-mix(in srgb, var(--raised) 34%, var(--panel));
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 18px;
    color: var(--muted);
    font-family: var(--font-code);
    font-size: 9px;
    letter-spacing: 0.08em;
    line-height: 1.4;
    text-transform: uppercase;
  }

  .meta span + span::before {
    margin-right: 18px;
    color: var(--line);
    content: '/';
  }

  h1 {
    max-width: 18ch;
    margin-top: 16px;
    color: var(--ink);
    font-family: var(--font-brand);
    font-size: clamp(38px, 6vw, 62px);
    font-weight: 500;
    letter-spacing: -0.045em;
    line-height: 0.95;
    overflow-wrap: anywhere;
  }

  .summary {
    max-width: 62ch;
    margin-top: 18px;
    color: var(--moonveil);
    font-size: 15px;
    line-height: 1.7;
  }

  .prose {
    --prose-gap: 14px;

    padding: clamp(20px, 3vw, 34px) clamp(24px, 6vw, 68px)
      clamp(34px, 6vw, 64px);
    color: var(--moonveil);
    font-size: 16px;
    font-weight: 400;
    line-height: 1.85;
  }

  .prose :global(.comark-content),
  .prose :global(.comark-content > *) {
    min-width: 0;
  }

  .prose :global(.comark-content > :first-child) {
    margin-top: 0;
  }

  .prose :global(h1),
  .prose :global(h2),
  .prose :global(h3),
  .prose :global(h4) {
    color: var(--ink);
    font-family: var(--font-brand);
    font-weight: 600;
    overflow-wrap: anywhere;
    scroll-margin-top: 28px;
  }

  .prose :global(h1) {
    font-size: clamp(32px, 5vw, 48px);
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .prose :global(h2) {
    margin-top: 40px;
    font-size: 29px;
    letter-spacing: -0.035em;
    line-height: 1.15;
  }

  .prose :global(h3) {
    margin-top: 28px;
    font-size: 21px;
    line-height: 1.25;
  }

  .prose :global(h4) {
    margin-top: 22px;
    font-family: var(--font-body);
    font-size: 16px;
    letter-spacing: 0;
    line-height: 1.4;
  }

  .prose :global(h2::after),
  .prose :global(h3::after) {
    display: block;
    width: 100%;
    height: 1px;
    margin-top: 10px;
    background-image: linear-gradient(
      to right,
      var(--guide) 0 5px,
      transparent 5px 10px
    );
    background-size: 10px 1px;
    content: '';
  }

  .prose :global(p),
  .prose :global(ul),
  .prose :global(ol),
  .prose :global(blockquote),
  .prose :global(pre),
  .prose :global(table) {
    margin-top: var(--prose-gap);
  }

  .prose :global(p) {
    text-wrap: pretty;
  }

  .prose :global(strong) {
    color: var(--ink);
    font-weight: 600;
  }

  .prose :global(em) {
    color: var(--ink);
  }

  .prose :global(a) {
    color: var(--halo);
    text-decoration-line: underline;
    text-decoration-color: color-mix(in srgb, var(--halo) 52%, transparent);
    text-decoration-thickness: 1px;
    text-underline-offset: 0.18em;
    overflow-wrap: anywhere;
    transition:
      color 160ms ease,
      text-decoration-color 160ms ease;
  }

  .prose :global(a::after) {
    margin-left: 0.14em;
    color: var(--muted);
    font-size: 0.72em;
    text-decoration: none;
    content: '↗';
  }

  .prose :global(a[href^='#'])::after {
    content: none;
  }

  .prose :global(a:hover) {
    color: var(--life);
    text-decoration-color: currentcolor;
  }

  .prose :global(ul) {
    padding-left: 0;
    list-style: none;
  }

  .prose :global(li) {
    position: relative;
    padding-left: 22px;
    overflow-wrap: anywhere;
  }

  .prose :global(ul > li::before) {
    position: absolute;
    top: 0.92em;
    left: 2px;
    width: 10px;
    height: 1px;
    background: var(--guide);
    content: '';
  }

  .prose :global(ol) {
    padding-left: 1.45em;
  }

  .prose :global(li + li) {
    margin-top: 8px;
  }

  .prose :global(code) {
    padding: 0.13em 0.36em;
    border: 1px solid color-mix(in srgb, var(--unripe) 28%, transparent);
    color: var(--unripe);
    background: color-mix(in srgb, var(--unripe) 9%, transparent);
    font-family: var(--font-code);
    font-size: 0.82em;
    box-decoration-break: clone;
  }

  .prose :global(pre) {
    max-width: 100%;
    padding: 18px 20px;
    overflow-x: auto;
    border: 1px solid var(--line);
    background: var(--page);
    line-height: 1.65;
    scrollbar-width: none;
  }

  .prose :global(pre::-webkit-scrollbar) {
    display: none;
  }

  .prose :global(pre code) {
    padding: 0;
    border: 0;
    color: var(--ink);
    background: transparent;
    font-size: 12.5px;
  }

  .prose :global(blockquote) {
    padding: 16px 18px;
    border: 1px solid var(--line);
    border-left: 3px solid var(--halo);
    color: var(--moonveil);
    background: var(--page);
  }

  .prose :global(blockquote > :first-child) {
    margin-top: 0;
  }

  .prose :global(table) {
    display: block;
    width: 100%;
    overflow-x: auto;
    border: 1px solid var(--line);
    border-collapse: collapse;
    background: var(--page);
    font-size: 13px;
    scrollbar-width: none;
  }

  .prose :global(table::-webkit-scrollbar) {
    display: none;
  }

  .prose :global(th),
  .prose :global(td) {
    padding: 10px 13px;
    border-bottom: 1px solid var(--line);
    text-align: left;
    vertical-align: top;
  }

  .prose :global(th) {
    color: var(--ink);
    font-family: var(--font-code);
    font-size: 9px;
    font-weight: 500;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .prose :global(hr) {
    margin: 44px 0;
    border: 0;
    border-top: 1px dashed var(--guide);
  }

  footer {
    padding: 18px clamp(24px, 6vw, 68px) 22px;
    border-top: 1px solid var(--line);
    background: color-mix(in srgb, var(--raised) 28%, var(--panel));
    font-family: var(--font-code);
    font-size: 10px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  footer a {
    display: inline-flex;
    gap: 8px;
    color: var(--secondary);
    text-decoration: none;
  }

  footer a:hover {
    color: var(--halo);
  }

  @media (max-width: 640px) {
    .prose {
      font-size: 15px;
      line-height: 1.8;
    }

    .prose :global(h2) {
      font-size: 25px;
    }
  }
</style>

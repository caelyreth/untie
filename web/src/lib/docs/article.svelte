<script lang="ts">
  import type { DocPage } from './types'

  let { doc }: { doc: DocPage } = $props()
</script>

<article class="prose">
  {@html doc.html}
  <footer>
    <a href={doc.sourceHref}>Edit this page on GitHub</a>
  </footer>
</article>

<style>
  .prose {
    color: var(--moonveil);
    font-size: 16px;
    font-weight: 400;
    line-height: 1.72;
  }

  .prose :global(h1) {
    margin-bottom: 28px;
    color: var(--ink);
    font-family: var(--font-brand);
    font-size: clamp(40px, 6vw, 58px);
    font-weight: 500;
    letter-spacing: -0.04em;
    line-height: 0.98;
  }

  .prose :global(h2) {
    margin: 48px 0 16px;
    padding-top: 18px;
    border-top: 1px solid var(--line);
    color: var(--ink);
    font-size: 29px;
    line-height: 1.15;
  }

  .prose :global(h3) {
    margin: 30px 0 10px;
    color: var(--ink);
    font-size: 20px;
  }

  .prose :global(p),
  .prose :global(ul),
  .prose :global(ol),
  .prose :global(blockquote),
  .prose :global(.table-wrap) {
    margin: 14px 0;
  }

  .prose :global(a) {
    color: var(--halo);
  }

  .prose :global(strong) {
    color: var(--ink);
    font-weight: 600;
  }

  .prose :global(code) {
    padding: 0.13em 0.36em;
    color: var(--unripe);
    background: var(--panel);
    font-size: 0.88em;
  }

  .prose :global(pre) {
    max-width: 100%;
    margin: 20px 0;
    padding: 18px 20px;
    overflow-x: auto;
    border: 1px solid var(--line);
    background:
      linear-gradient(135deg, rgb(161 158 194 / 4%), transparent 45%),
      var(--panel);
    line-height: 1.65;
  }

  .prose :global(pre code) {
    padding: 0;
    color: var(--ink);
    background: none;
    font-size: 12.5px;
  }

  .prose :global(blockquote) {
    padding-left: 18px;
    border-left: 2px solid var(--halo-muted);
    color: var(--muted);
  }

  .prose :global(table) {
    display: block;
    width: 100%;
    overflow-x: auto;
    border-collapse: collapse;
    font-size: 13px;
  }

  .prose :global(th),
  .prose :global(td) {
    padding: 9px 12px;
    border-bottom: 1px solid var(--line);
    text-align: left;
    vertical-align: top;
  }

  .prose :global(th) {
    color: var(--ink);
    font-family: var(--font-code);
    font-size: 10px;
    font-weight: 500;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .prose :global(hr) {
    margin: 44px 0;
    border: 0;
    border-top: 1px solid var(--line);
  }

  footer {
    margin-top: 64px;
    padding-top: 18px;
    border-top: 1px solid var(--line);
    font-family: var(--font-code);
    font-size: 10px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }
</style>

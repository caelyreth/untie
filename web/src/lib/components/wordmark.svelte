<script lang="ts">
  import favicon from '#lib/assets/favicon.svg'

  let { symbol = false }: { symbol?: boolean } = $props()
</script>

<span class="wordmark">
  {#if symbol}
    <img
      class="symbol"
      src={favicon}
      alt=""
      aria-hidden="true"
      draggable="false"
    />
  {/if}
  <span>untie</span>
</span>

<style>
  .wordmark {
    display: inline-flex;
    align-items: center;
    gap: 0.18em;
    color: inherit;
    font-family: var(--font-brand);
    font-size: inherit;
    font-weight: 600;
    letter-spacing: -0.045em;
    line-height: 0.86;
    text-transform: none;
    white-space: nowrap;
  }

  .symbol {
    flex: none;
    width: 0.95em;
    height: 0.95em;
    object-fit: contain;
  }
</style>

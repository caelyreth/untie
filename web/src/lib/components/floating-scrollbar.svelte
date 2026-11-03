<script lang="ts">
  import { onMount } from 'svelte'

  let visible = $state(false)
  let thumbHeight = $state(0)
  let thumbTop = $state(0)

  onMount(() => {
    let frame = 0

    function update() {
      frame = 0
      const viewport = window.innerHeight
      const total = document.documentElement.scrollHeight
      const scrollable = total - viewport
      if (scrollable <= 1) {
        visible = false
        return
      }
      const track = Math.max(1, viewport - 20)
      thumbHeight = Math.max(36, (viewport / total) * track)
      const travel = Math.max(0, track - thumbHeight)
      thumbTop = (window.scrollY / scrollable) * travel
      visible = true
    }

    function schedule() {
      if (!frame) {
        frame = requestAnimationFrame(update)
      }
    }

    const observer = new ResizeObserver(schedule)
    observer.observe(document.documentElement)
    observer.observe(document.body)
    window.addEventListener('scroll', schedule, { passive: true })
    window.addEventListener('resize', schedule)
    update()

    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      window.removeEventListener('scroll', schedule)
      window.removeEventListener('resize', schedule)
    }
  })
</script>

{#if visible}
  <div class="floating-scrollbar" aria-hidden="true">
    <span
      style:height={`${thumbHeight}px`}
      style:translate={`0 ${thumbTop}px`}
    ></span>
  </div>
{/if}

<style>
  .floating-scrollbar {
    position: fixed;
    top: 10px;
    right: 5px;
    bottom: 10px;
    z-index: 80;
    width: 4px;
    pointer-events: none;
  }

  .floating-scrollbar span {
    display: block;
    width: 100%;
    border-radius: 999px;
    background: color-mix(in srgb, var(--secondary) 48%, transparent);
  }
</style>

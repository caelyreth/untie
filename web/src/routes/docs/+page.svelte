<script lang="ts">
  import DocArticle from '#lib/docs/article.svelte'

  import type { PageProps } from './$types'

  let { data }: PageProps = $props()
</script>

<svelte:head>
  <title>{data.doc.title} — Untie</title>
</svelte:head>

<DocArticle doc={data.doc} />

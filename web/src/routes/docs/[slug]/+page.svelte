<script lang="ts">
  import Seo from '#lib/components/seo.svelte'
  import DocArticle from '#lib/docs/article.svelte'

  import type { PageProps } from './$types'

  let { data }: PageProps = $props()
</script>

<Seo
  title={data.doc.title}
  description={data.doc.description}
  path={`/docs/${data.doc.slug}/`}
  type="article"
/>

<DocArticle doc={data.doc} />

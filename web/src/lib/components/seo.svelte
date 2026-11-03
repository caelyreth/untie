<script lang="ts">
  interface SeoProps {
    title: string
    description: string
    path: string
    type?: 'article' | 'website'
  }

  let { title, description, path, type = 'website' }: SeoProps = $props()

  const siteName = 'Untie'
  const fallbackSiteURL = 'https://untie.iki.moe'
  const imagePath = '/og-untie.png'
  const imageAlt = 'Untie — one math renderer for TeX and Typst'

  const siteURL = (
    import.meta.env.PUBLIC_SITE_URL?.trim() || fallbackSiteURL
  ).replace(/\/+$/, '')
  const pageTitle = $derived(
    title.includes(siteName) ? title : `${title} — ${siteName}`,
  )
  const canonicalURL = $derived(
    new URL(
      path === '/' ? '/' : `${path.replace(/\/+$/, '')}/`,
      `${siteURL}/`,
    ).href,
  )
  const imageURL = $derived(new URL(imagePath, `${siteURL}/`).href)
</script>

<svelte:head>
  <title>{pageTitle}</title>
  <meta name="description" content={description} />
  <link rel="canonical" href={canonicalURL} />

  <meta
    name="robots"
    content="index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1"
  />
  <meta name="application-name" content={siteName} />
  <meta name="apple-mobile-web-app-title" content={siteName} />
  <meta name="author" content="Yu (Caelyreth)" />
  <meta name="color-scheme" content="dark" />
  <meta name="format-detection" content="telephone=no" />
  <meta name="referrer" content="strict-origin-when-cross-origin" />

  <meta property="og:type" content={type} />
  <meta property="og:locale" content="en_US" />
  <meta property="og:site_name" content={siteName} />
  <meta property="og:title" content={pageTitle} />
  <meta property="og:description" content={description} />
  <meta property="og:url" content={canonicalURL} />
  <meta property="og:image" content={imageURL} />
  <meta property="og:image:type" content="image/png" />
  <meta property="og:image:width" content="1280" />
  <meta property="og:image:height" content="640" />
  <meta property="og:image:alt" content={imageAlt} />
  {#if imageURL.startsWith('https://')}
    <meta property="og:image:secure_url" content={imageURL} />
  {/if}

  <meta name="twitter:card" content="summary_large_image" />
  <meta name="twitter:title" content={pageTitle} />
  <meta name="twitter:description" content={description} />
  <meta name="twitter:image" content={imageURL} />
  <meta name="twitter:image:alt" content={imageAlt} />
</svelte:head>

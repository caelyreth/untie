import { basename, posix } from 'node:path'

import { marked } from 'marked'

import type { DocLink, DocPage } from '#lib/docs/types'

const repository = 'https://github.com/caelyreth/untie/blob/main/'
const markdownFiles = import.meta.glob('/src/content/docs/*.md', {
  eager: true,
  import: 'default',
  query: '?raw',
}) as Record<string, string>
const order = [
  'README.md',
  'getting-started.md',
  'concepts.md',
  'rendering.md',
  'fonts.md',
  'parsing.md',
  'errors-and-limits.md',
  'canonical-ir.md',
  'concurrency.md',
  'accessibility.md',
  'mygo.md',
  'tex-support.md',
  'typst-support.md',
  'stability.md',
  'quality.md',
  'performance.md',
  'migration.md',
]

let catalog: readonly DocLink[] | undefined

export function listDocs(): Promise<readonly DocLink[]> {
  catalog ??= readCatalog()
  return Promise.resolve(catalog)
}

export async function loadDoc(slug: string): Promise<DocPage> {
  const file = documentFile(slug)
  const markdown = documentMarkdown(file)
  const docs = await listDocs()
  const rewritten = rewriteLinks(markdown)
  const rawHtml = marked.parse(rewritten, {
    async: false,
    gfm: true,
  })
  return {
    html: addHeadingIDs(rawHtml),
    slug,
    sourceHref: `${repository}web/src/content/docs/${file}`,
    title:
      docs.find((doc) => doc.href === `/docs/${slug}`.replace(/\/$/, ''))
        ?.title ?? extractTitle(markdown),
  }
}

function documentFile(slug: string) {
  if (slug.includes('/') || slug.includes('\\') || slug.startsWith('.')) {
    throw new Error(`Invalid document slug: ${slug}`)
  }
  return slug ? `${slug}.md` : 'README.md'
}

function documentMarkdown(file: string) {
  const markdown = markdownFiles[`/src/content/docs/${file}`]
  if (!markdown) {
    throw new Error(`Unknown document: ${file}`)
  }
  return markdown
}

function readCatalog(): readonly DocLink[] {
  const files = Object.keys(markdownFiles)
    .map((file) => basename(file))
    .sort(
      (left, right) =>
        rank(left) - rank(right) || left.localeCompare(right),
    )
  return files.map((file) => {
    const markdown = markdownFiles[`/src/content/docs/${file}`] ?? ''
    const slug = file === 'README.md' ? '' : basename(file, '.md')
    const title = extractTitle(markdown)
    return {
      href: slug ? `/docs/${slug}` : '/docs',
      label: file === 'README.md' ? 'Overview' : title,
      title,
    }
  })
}

function rank(file: string) {
  const index = order.indexOf(file)
  return index === -1 ? order.length : index
}

function extractTitle(markdown: string) {
  return (
    markdown
      .match(/^#\s+(.+)$/m)?.[1]
      ?.replaceAll('`', '')
      .trim() ?? 'Untie documentation'
  )
}

function rewriteLinks(markdown: string) {
  return markdown.replace(
    /(```[\s\S]*?```)|\[([^\]]+)\]\(([^)\s]+)([^)]*)\)/g,
    (
      match,
      fence: string | undefined,
      label: string,
      target: string,
      title,
    ) => {
      if (fence) {
        return fence
      }
      return `[${label}](${rewriteTarget(target)}${title})`
    },
  )
}

function rewriteTarget(target: string) {
  if (isExternalReference(target)) {
    return target
  }
  const [pathPart, anchor = ''] = target.split('#', 2)
  const hash = anchor ? `#${anchor}` : ''
  if (isDocumentReference(pathPart)) {
    return documentHref(pathPart, hash)
  }
  return repositoryHref(pathPart, hash)
}

function isExternalReference(target: string) {
  return ['#', 'http://', 'https://', 'mailto:'].some((prefix) =>
    target.startsWith(prefix),
  )
}

function isDocumentReference(pathPart: string) {
  return pathPart.endsWith('.md') && !pathPart.includes('/')
}

function documentHref(pathPart: string, hash: string) {
  const slug = pathPart === 'README.md' ? '' : pathPart.slice(0, -3)
  return slug ? `/docs/${slug}${hash}` : `/docs${hash}`
}

function repositoryHref(pathPart: string, hash: string) {
  const repositoryPath = posix.normalize(posix.join('docs', pathPart))
  return `${repository}${repositoryPath}${hash}`
}

function addHeadingIDs(html: string) {
  const ids = new Map<string, number>()
  return html.replace(
    /<h([1-3])>(.*?)<\/h\1>/g,
    (heading, level: string, content: string) => {
      const plain = content
        .replace(/<[^>]+>/g, '')
        .replaceAll('&amp;', 'and')
        .replaceAll('&lt;', '')
        .replaceAll('&gt;', '')
      const base =
        plain
          .toLowerCase()
          .replace(/[^a-z0-9 -]/g, '')
          .trim()
          .replace(/\s+/g, '-') || 'section'
      const count = ids.get(base) ?? 0
      ids.set(base, count + 1)
      const id = count ? `${base}-${count}` : base
      return `<h${level} id="${id}">${content}</h${level}>`
    },
  )
}

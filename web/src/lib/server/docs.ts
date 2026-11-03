import { basename, posix } from 'node:path'

import { createMarkdownParser, type MarkdownDocument } from 'comark'
import headings from 'comark/plugins/headings'
import rangi from 'comark/plugins/rangi'
import security from 'comark/plugins/security'

import type { DocLink, DocPage } from '#lib/docs/types'

import { untieRangiTheme } from './rangi-theme'

const repository = 'https://github.com/caelyreth/untie/blob/main/'
const markdownFiles = import.meta.glob('../../../../docs/*.md', {
  eager: true,
  import: 'default',
  query: '?raw',
}) as Record<string, string>
const markdownParser = createMarkdownParser({
  plugins: [
    headings({ remove: true }),
    security(),
    rangi({ preStyles: true, theme: untieRangiTheme }),
  ],
  headingIds: true,
})
const navigation = [
  {
    label: 'Start',
    files: ['README.md', 'getting-started.md', 'concepts.md'],
  },
  {
    label: 'Pipeline',
    files: [
      'rendering.md',
      'fonts.md',
      'parsing.md',
      'errors-and-limits.md',
      'canonical-ir.md',
      'mathml.md',
    ],
  },
  {
    label: 'Runtime',
    files: ['concurrency.md', 'accessibility.md', 'mygo.md'],
  },
  {
    label: 'Reference',
    files: [
      'tex-support.md',
      'typst-support.md',
      'stability.md',
      'quality.md',
      'performance.md',
      'migration.md',
    ],
  },
] as const
const fileOrder = new Map<string, number>(
  navigation
    .flatMap((section) => section.files)
    .map((file, index) => [file, index]),
)
const fileSection = new Map<string, string>(
  navigation.flatMap((section) =>
    section.files.map((file) => [file, section.label] as const),
  ),
)

let catalog: Promise<readonly DocLink[]> | undefined

export function listDocs(): Promise<readonly DocLink[]> {
  catalog ??= readCatalog()
  return catalog
}

export async function loadDoc(slug: string): Promise<DocPage> {
  const file = documentFile(slug)
  const markdown = documentMarkdown(file)
  const rewritten = rewriteLinks(markdown)
  const document = await markdownParser(rewritten)
  const title = metadataTitle(document) ?? extractTitle(markdown)
  return {
    description:
      metadataDescription(document) ?? extractDescription(markdown, title),
    document,
    slug,
    sourceHref: `${repository}docs/${file}`,
    title,
  }
}

function documentFile(slug: string) {
  if (slug.includes('/') || slug.includes('\\') || slug.startsWith('.')) {
    throw new Error(`Invalid document slug: ${slug}`)
  }
  return slug ? `${slug}.md` : 'README.md'
}

function documentMarkdown(file: string) {
  const markdown =
    markdownFiles[`../../../../docs/${file}`] ??
    markdownFiles[`/../../docs/${file}`]
  if (!markdown) {
    throw new Error(`Unknown document: ${file}`)
  }
  return markdown
}

async function readCatalog(): Promise<readonly DocLink[]> {
  const files = Object.keys(markdownFiles)
    .map((file) => basename(file))
    .sort(
      (left, right) =>
        rank(left) - rank(right) || left.localeCompare(right),
    )
  return Promise.all(
    files.map(async (file) => {
      const markdown = documentMarkdown(file)
      const slug = file === 'README.md' ? '' : basename(file, '.md')
      const document = await markdownParser(rewriteLinks(markdown))
      const title = metadataTitle(document) ?? extractTitle(markdown)
      return {
        href: slug ? `/docs/${slug}` : '/docs',
        label: file === 'README.md' ? 'Overview' : title,
        section: sectionFor(file),
        title,
      }
    }),
  )
}

function rank(file: string) {
  return fileOrder.get(file) ?? fileOrder.size
}

function sectionFor(file: string) {
  return fileSection.get(file) ?? 'Reference'
}

function extractTitle(markdown: string) {
  return (
    markdown
      .match(/^#\s+(.+)$/m)?.[1]
      ?.replaceAll('`', '')
      .trim() ?? 'Untie documentation'
  )
}

function metadataTitle(document: MarkdownDocument) {
  return typeof document.meta.title === 'string'
    ? document.meta.title
    : undefined
}

function metadataDescription(document: MarkdownDocument) {
  return typeof document.meta.description === 'string'
    ? truncateDescription(document.meta.description)
    : undefined
}

function extractDescription(markdown: string, title: string) {
  const blocks = markdown.replace(/```[\s\S]*?```/g, '').split(/\n\s*\n/)
  for (const block of blocks) {
    const trimmed = block.trim()
    if (!trimmed || /^(?:#|[-*+]\s|\d+\.\s|\||>|<)/.test(trimmed)) {
      continue
    }
    const description = trimmed
      .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
      .replace(/[`*_]/g, '')
      .replace(/\s+/g, ' ')
      .trim()
    if (description.length >= 40) {
      return truncateDescription(description)
    }
  }
  return `${title} — Untie documentation for TeX and Typst math rendering.`
}

function truncateDescription(description: string) {
  const limit = 180
  if (description.length <= limit) {
    return description
  }
  const boundary = description.lastIndexOf(' ', limit)
  return `${description.slice(0, boundary > 80 ? boundary : limit).trim()}…`
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

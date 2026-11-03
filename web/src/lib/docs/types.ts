import type { MarkdownDocument } from 'comark'

export interface DocLink {
  href: string
  label: string
  section: string
  title: string
}

export interface DocPage {
  description: string
  document: MarkdownDocument
  slug: string
  sourceHref: string
  title: string
}

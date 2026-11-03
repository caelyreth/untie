export type LibraryDialect = 'tex' | 'typst'
export type LibraryMode = 'display' | 'inline'

export interface LibraryCase {
  category: string
  categoryLabel: string
  dialect: LibraryDialect
  id: string
  label: string
  mode: LibraryMode
  source: string
}

export interface LibraryCategory {
  count: number
  label: string
  value: string
}

import type { LibraryCase, LibraryCategory } from '#lib/library/types'

import type { PageServerLoad } from './$types'

interface CorpusCase {
  category: string
  coverageGroup?: string
  dialect: 'tex' | 'typst'
  expect: {
    layout: string
    parse: string
  }
  id: string
  mode: 'display' | 'inline'
  source: string
}

const corpusFiles = import.meta.glob(
  '../../../../testdata/daily-stem/cases.jsonl',
  {
    eager: true,
    import: 'default',
    query: '?raw',
  },
) as Record<string, string>

const categoryLabels: Record<string, string> = {
  'algebra': 'Algebra',
  'calculus': 'Calculus',
  'delimiters': 'Delimiters',
  'display-wrapping': 'Display Wrapping',
  'equation-tags': 'Equation Tags',
  'linear-algebra': 'Linear Algebra',
  'multiline-cases': 'Multiline',
  'physics-engineering': 'Physics & Engineering',
  'probability-statistics': 'Probability & Statistics',
  'sets-logic': 'Sets & Logic',
  'text-and-fallback': 'Text & Fallback',
}

const source = Object.values(corpusFiles)[0]
if (!source) {
  throw new Error('Daily corpus cases.jsonl was not found')
}
const cases = source
  .split('\n')
  .filter(Boolean)
  .map((line) => JSON.parse(line) as CorpusCase)
  .filter(
    (testCase) =>
      testCase.expect.parse === 'ok' && testCase.expect.layout === 'ok',
  )
  .map((testCase): LibraryCase => ({
    category: testCase.category,
    categoryLabel:
      categoryLabels[testCase.category] ??
      titleCase(testCase.category.replaceAll('-', ' ')),
    dialect: testCase.dialect,
    id: testCase.id,
    label: titleCase(
      (
        testCase.coverageGroup ??
        testCase.id.replace(/\.(?:tex|typst)$/, '')
      )
        .split('.')
        .at(-1)
        ?.replaceAll('-', ' ') ?? 'Formula',
    ),
    mode: testCase.mode,
    source: testCase.source,
  }))

const counts = new Map<string, number>()
for (const testCase of cases) {
  counts.set(testCase.category, (counts.get(testCase.category) ?? 0) + 1)
}
const categories = [...counts]
  .map(([value, count]): LibraryCategory => ({
    count,
    label: categoryLabels[value] ?? titleCase(value.replaceAll('-', ' ')),
    value,
  }))
  .sort((left, right) => left.label.localeCompare(right.label))

export const load = (async () => ({
  cases,
  categories,
})) satisfies PageServerLoad

function titleCase(value: string) {
  return value.replace(/\b[a-z]/g, (letter) => letter.toUpperCase())
}

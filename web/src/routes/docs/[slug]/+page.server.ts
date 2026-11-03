import { listDocs, loadDoc } from '#lib/server/docs'

import type { EntryGenerator, PageServerLoad } from './$types'

export const entries: EntryGenerator = async () =>
  (await listDocs())
    .filter((doc) => doc.href !== '/docs')
    .map((doc) => ({ slug: doc.href.replace('/docs/', '') }))

export const load = (async ({ params }) => ({
  doc: await loadDoc(params.slug),
})) satisfies PageServerLoad

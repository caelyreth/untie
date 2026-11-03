import { listDocs } from '#lib/server/docs'

import type { LayoutServerLoad } from './$types'

export const load = (async () => ({
  docs: await listDocs(),
})) satisfies LayoutServerLoad

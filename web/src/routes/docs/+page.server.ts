import { loadDoc } from '#lib/server/docs'

import type { PageServerLoad } from './$types'

export const load = (async () => ({
  doc: await loadDoc(''),
})) satisfies PageServerLoad

export interface Searchable {
  id: string
  name: string
}

/**
 * Items whose name matches q, best first: a name that starts with q, then a
 * word that starts with it, then q anywhere. Ties keep their order.
 */
export function search<T extends Searchable>(items: T[], q: string, limit = 8): T[] {
  const needle = q.trim().toLowerCase()
  if (!needle) return items.slice(0, limit)
  const scored: { item: T; score: number; i: number }[] = []
  items.forEach((item, i) => {
    const name = item.name.toLowerCase()
    let score = -1
    if (name.startsWith(needle)) score = 0
    else if (name.split(/[\s\-_.]+/).some((w) => w.startsWith(needle))) score = 1
    else if (name.includes(needle)) score = 2
    if (score >= 0) scored.push({ item, score, i })
  })
  scored.sort((a, b) => a.score - b.score || a.i - b.i)
  return scored.slice(0, limit).map((s) => s.item)
}

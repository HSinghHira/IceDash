// App bundles (PLAN.md 5b.4): IceStore apps that work well together,
// installed one after the other, each with its own setup. Only the apps the
// catalog lists show; a bundle with fewer than two left is hidden. Their
// names and words are in strings.bundles.
import type { IconName } from './icons'

export interface Bundle {
  id: string
  icon: IconName
  /** IceStore ids, in the order they're set up. */
  apps: string[]
}

const s = (names: string[]) => names.map((n) => `com.icestore.${n}`)

export const BUNDLES: Bundle[] = [
  { id: 'homelab', icon: 'pulse', apps: s(['watchyourlan', 'portracker', 'homelable', 'homedex', 'disk-monitor']) },
  { id: 'livetv', icon: 'film', apps: s(['dispatcharr', 'm3u-editor', 'channel-club', 'nodecast-tv']) },
  { id: 'music', icon: 'music', apps: s(['helix', 'trackhop', 'musike']) },
  { id: 'reading', icon: 'doc', apps: s(['bookorbit', 'kovi', 'indelible']) },
  { id: 'family', icon: 'home', apps: s(['kitchen-board', 'norish', 'cartvibe', 'opensignup']) },
  { id: 'places', icon: 'map', apps: s(['dawarich', 'strava-dawarich-sync']) },
  { id: 'tasks', icon: 'checks', apps: s(['opentodo', 'habits', 'tef-notes']) },
  { id: 'paperwork', icon: 'edit', apps: s(['documenso', 'lemmary', 'vert', 'docmost']) },
  { id: 'private', icon: 'shield', apps: s(['searxng', 'poke', 'cobalt']) },
  { id: 'devdesk', icon: 'terminal', apps: s(['vscodium', 'gitsave', 'github-release-monitor', 'sniptide', 'x-tools']) },
  { id: 'automation', icon: 'refresh', apps: s(['n8n-oidc', 'pocket-id', 'ntfy']) },
  { id: 'localai', icon: 'chip', apps: s(['overtchat', 'omniroute', 'pullmd']) },
]

/** The bundles with the apps the catalog has, in their order. */
export function availableBundles<A extends { id: string }>(catalog: A[]): { bundle: Bundle; apps: A[] }[] {
  const byId = new Map(catalog.map((a) => [a.id, a]))
  return BUNDLES.map((b) => ({ bundle: b, apps: b.apps.map((id) => byId.get(id)).filter((a): a is A => !!a) })).filter((x) => x.apps.length >= 2)
}

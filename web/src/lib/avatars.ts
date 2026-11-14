// Drawn avatars for people without a picture of their own, made with
// DiceBear (MIT) from its public domain (CC0) styles. They are drawn here in
// the browser, so no name leaves the server. Each style loads only when it's
// first used.
import { Avatar, Style } from '@dicebear/core'

export interface AvatarStyle {
  id: string
  name: string
}

/** The styles offered, Critters first: it's the default. */
export const AVATAR_STYLES: AvatarStyle[] = [
  { id: 'critters', name: 'Critters' },
  { id: 'lorelei', name: 'Lorelei' },
  { id: 'thumbs', name: 'Thumbs' },
  { id: 'pixel-art', name: 'Pixel Art' },
  { id: 'pixelbot', name: 'Pixelbot' },
  { id: 'voxel-bot', name: 'Voxel Bot' },
  { id: 'moods', name: 'Moods' },
  { id: 'sprouts', name: 'Sprouts' },
  { id: 'clay', name: 'Clay' },
  { id: 'planets', name: 'Planets' },
  { id: 'marbles', name: 'Marbles' },
  { id: 'glass', name: 'Glass' },
  { id: 'shapes', name: 'Shapes' },
  { id: 'blobs', name: 'Blobs' },
]
export const DEFAULT_STYLE = 'critters'

const load: Record<string, () => Promise<unknown>> = {
  critters: () => import('@dicebear/styles/critters.json'),
  lorelei: () => import('@dicebear/styles/lorelei.json'),
  thumbs: () => import('@dicebear/styles/thumbs.json'),
  'pixel-art': () => import('@dicebear/styles/pixel-art.json'),
  pixelbot: () => import('@dicebear/styles/pixelbot.json'),
  'voxel-bot': () => import('@dicebear/styles/voxel-bot.json'),
  moods: () => import('@dicebear/styles/moods.json'),
  sprouts: () => import('@dicebear/styles/sprouts.json'),
  clay: () => import('@dicebear/styles/clay.json'),
  planets: () => import('@dicebear/styles/planets.json'),
  marbles: () => import('@dicebear/styles/marbles.json'),
  glass: () => import('@dicebear/styles/glass.json'),
  shapes: () => import('@dicebear/styles/shapes.json'),
  blobs: () => import('@dicebear/styles/blobs.json'),
}

const styles = new Map<string, Promise<Style | null>>()
const drawn = new Map<string, string>()

function styleOf(id: string): Promise<Style | null> {
  let p = styles.get(id)
  if (!p) {
    const get = load[id]
    p = get
      ? get()
          .then((m) => new Style((m as { default?: unknown }).default ?? m))
          .catch(() => null)
      : Promise.resolve(null)
    styles.set(id, p)
  }
  return p
}

/** The style a choice means: "" is the default, "initials" none. */
export const styleFor = (choice: string | undefined) => (!choice ? DEFAULT_STYLE : choice)

/** A drawn avatar as a data: URI, or "" for initials (or a style that can't load). */
export async function drawAvatar(style: string, seed: string): Promise<string> {
  if (style === 'initials') return ''
  const key = `${style}\n${seed}`
  const hit = drawn.get(key)
  if (hit !== undefined) return hit
  const s = await styleOf(style)
  let uri = ''
  if (s) {
    try {
      uri = new Avatar(s, { seed, size: 128 }).toDataUri()
    } catch {
      uri = ''
    }
  }
  drawn.set(key, uri)
  return uri
}

/** A new seed for "another face". */
export const newSeed = () => Math.random().toString(36).slice(2, 10)

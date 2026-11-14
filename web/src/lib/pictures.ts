// Getting a picture ready to send: wallpapers are shrunk to 4K when bigger,
// avatars cropped to a 512 px square. Anything the server doesn't keep
// (GIF, HEIC the browser can read, BMP…) is redrawn as JPEG or WebP.

export const KEEP = ['image/jpeg', 'image/png', 'image/webp']
export const MAX_SIDE = 3840
export const MAX_BYTES = 14 << 20
export const AVATAR_SIDE = 512

export async function decode(file: Blob): Promise<ImageBitmap> {
  try {
    return await createImageBitmap(file)
  } catch {
    throw new Error('That file isn’t a picture this browser can open.')
  }
}

function toBlob(c: HTMLCanvasElement, type: string, q: number): Promise<Blob> {
  return new Promise((resolve, reject) => c.toBlob((b) => (b ? resolve(b) : reject(new Error('The picture couldn’t be drawn.'))), type, q))
}

/** A wallpaper the server takes: the file itself when it already fits. */
export async function prepareWallpaper(file: File): Promise<Blob> {
  const img = await decode(file)
  const side = Math.max(img.width, img.height)
  if (KEEP.includes(file.type) && file.size <= MAX_BYTES && side <= MAX_SIDE) {
    img.close()
    return file
  }
  const k = Math.min(1, MAX_SIDE / side)
  const c = document.createElement('canvas')
  c.width = Math.round(img.width * k)
  c.height = Math.round(img.height * k)
  c.getContext('2d')!.drawImage(img, 0, 0, c.width, c.height)
  img.close()
  let q = 0.9
  let b = await toBlob(c, 'image/jpeg', q)
  while (b.size > MAX_BYTES && q > 0.5) b = await toBlob(c, 'image/jpeg', (q -= 0.1))
  return b
}

/** Where the picture sits in the round frame: its top-left corner and drawn size, in frame pixels. */
export interface Crop {
  x: number
  y: number
  w: number
  h: number
}

/** Draws the framed part as a square avatar (WebP, or PNG where WebP can't be made). */
export async function cropAvatar(img: ImageBitmap, crop: Crop, frame: number): Promise<Blob> {
  const c = document.createElement('canvas')
  c.width = c.height = AVATAR_SIDE
  const k = AVATAR_SIDE / frame
  const g = c.getContext('2d')!
  g.imageSmoothingQuality = 'high'
  g.drawImage(img, crop.x * k, crop.y * k, crop.w * k, crop.h * k)
  const b = await toBlob(c, 'image/webp', 0.9)
  return b.size > 1_000_000 ? toBlob(c, 'image/jpeg', 0.85) : b
}

/** Keeps the picture covering the frame: no empty edge shows. */
export function clampCrop(c: Crop, frame: number): Crop {
  return { ...c, x: Math.min(0, Math.max(frame - c.w, c.x)), y: Math.min(0, Math.max(frame - c.h, c.y)) }
}

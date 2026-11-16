// Files paths (after the release, 2026-11-16): "/a/b" is in /DATA, "USB:/a/b"
// on another disk (internal/files/disks.go). A /DATA path always starts with
// "/", a disk's never does.

const DISK = /^([A-Za-z0-9._-]{1,64}):\//

/** The path's top ("/" or "USB:/") and the rest, without a leading "/". */
export function splitPath(p: string): { root: string; rel: string } {
  const m = p.startsWith('/') ? null : DISK.exec(p)
  if (m) return { root: m[1] + ':/', rel: p.slice(m[0].length).replace(/\/+$/, '') }
  return { root: '/', rel: p.replace(/^\/+|\/+$/g, '') }
}

/** The folder p is in; null at a disk's top. */
export function parentPath(p: string): string | null {
  const { root, rel } = splitPath(p)
  if (!rel) return null
  const i = rel.lastIndexOf('/')
  return root + (i < 0 ? '' : rel.slice(0, i))
}

/** One crumb for each folder below the top. */
export function crumbPaths(p: string): { name: string; path: string }[] {
  const { root, rel } = splitPath(p)
  const parts = rel.split('/').filter(Boolean)
  return parts.map((name, i) => ({ name, path: root + parts.slice(0, i + 1).join('/') }))
}

/** Whether p is folder or inside it. */
export function within(p: string, folder: string): boolean {
  if (p === folder) return true
  return p.startsWith(folder.endsWith('/') ? folder : folder + '/')
}

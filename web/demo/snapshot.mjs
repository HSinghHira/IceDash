// Copies IceDash's official mod collections (IceColors and IceCards on GitHub)
// for the website's live demo, as the backend would list and install them:
//
//   node web/demo/snapshot.mjs _site
//
// writes _site/demo/mods.json (the collections, as GET /v2/icedash/mods/collections
// answers) and each mod's files where the dashboard loads them:
// _site/v2/icedash/mods/<id>/wallpaper for a theme, .../<id>/files/ for a widget,
// with the widget SDK beside its page. GITHUB_TOKEN or GH_TOKEN raises GitHub's
// rate limit; without one it still works a few times an hour.
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const OFFICIAL = ['github.com/HSinghHira/IceColors', 'github.com/HSinghHira/IceCards']
const WALLPAPERS = ['wallpaper.webp', 'wallpaper.jpg', 'wallpaper.png']
const SDK = join(dirname(fileURLToPath(import.meta.url)), '../../internal/mods/sdk/icedash-widget.js')

const out = process.argv[2]
if (!out) {
  console.error('usage: node web/demo/snapshot.mjs <site folder>')
  process.exit(2)
}
const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN
const api = async (path) => {
  const res = await fetch(`https://api.github.com${path}`, {
    headers: { Accept: 'application/vnd.github+json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
  })
  if (!res.ok) throw new Error(`GitHub ${path}: ${res.status}`)
  return res.json()
}
const raw = async (repo, sha, path) => {
  const res = await fetch(`https://raw.githubusercontent.com/${repo}/${sha}/${path}`)
  if (!res.ok) throw new Error(`raw ${repo}/${path}: ${res.status}`)
  return Buffer.from(await res.arrayBuffer())
}
// As internal/mods ModID: owner--repo--folder, lower case.
const slug = (s) => s.toLowerCase().replace(/[^a-z0-9-]/g, '-')
const modID = (src, folder) => {
  const [owner, repo] = src.replace('github.com/', '').split('/')
  return `${slug(owner)}--${slug(repo)}--${folder}`
}
const put = async (path, data) => {
  await mkdir(dirname(path), { recursive: true })
  await writeFile(path, data)
}

const collections = []
for (const source of OFFICIAL) {
  const repo = source.replace('github.com/', '')
  const { default_branch } = await api(`/repos/${repo}`)
  const { sha } = await api(`/repos/${repo}/commits/${default_branch}`)
  const { tree } = await api(`/repos/${repo}/git/trees/${sha}?recursive=1`)
  const folders = tree.filter((t) => t.type === 'tree' && !t.path.includes('/') && !t.path.startsWith('.'))
  const items = []
  for (const f of folders) {
    const files = tree.filter((t) => t.type === 'blob' && t.path.startsWith(f.path + '/')).map((t) => t.path.slice(f.path.length + 1))
    const id = modID(source, f.path)
    const base = { id, collection: source, folder: f.path, official: true, version: f.sha.slice(0, 12) }
    const dir = join(out, 'v2/icedash/mods', id)
    if (files.includes('theme.json')) {
      const t = JSON.parse(await raw(repo, sha, `${f.path}/theme.json`))
      const item = { ...base, kind: 'theme', name: t.name, author: t.author, description: t.description, scheme: t.scheme, colors: t.colors }
      const wp = WALLPAPERS.find((w) => files.includes(w))
      if (wp) {
        item.wallpaper = `https://raw.githubusercontent.com/${repo}/${sha}/${f.path}/${wp}`
        await put(join(dir, 'wallpaper'), await raw(repo, sha, `${f.path}/${wp}`))
      }
      items.push(item)
    } else if (files.includes('widget.json')) {
      const w = JSON.parse(await raw(repo, sha, `${f.path}/widget.json`))
      if (!files.includes(w.entry)) continue
      items.push({ ...base, kind: 'widget', name: w.name, author: w.author, description: w.description, entry: w.entry, size: w.size, permissions: w.permissions ?? {} })
      for (const rel of files) {
        let data = await raw(repo, sha, `${f.path}/${rel}`)
        // The backend puts the bridge first in the widget's page when it serves it
        // (internal/mods/serve.go withPolicy); a static copy carries it in the file.
        if (rel === w.entry) {
          const page = data.toString('utf8').replace(/^\s*/, '')
          const tag = '<script src="icedash-widget.js"></script>'
          data = /^<!doctype[^>]*>/i.test(page) ? page.replace(/^<!doctype[^>]*>/i, (d) => d + tag) : '<!doctype html>' + tag + page
        }
        await put(join(dir, 'files', rel), data)
      }
      await put(join(dir, 'files', 'icedash-widget.js'), await readFile(SDK))
    }
  }
  collections.push({ source, official: true, commit: sha, items })
  console.log(`${source}: ${items.length} mods at ${sha.slice(0, 7)}`)
}
await put(join(out, 'demo', 'mods.json'), JSON.stringify({ collections }))

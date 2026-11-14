<script setup lang="ts">
// The Files app (PLAN.md, Phase 3c): everything under /DATA, for admins.
// System folders show a lock and can't be changed; delete goes to the Trash.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ApiError } from '../api/client'
import * as api from '../api/files'
import type { Clash, Entry, Job, TrashItem, Upload } from '../api/files'
import Icon from '../components/Icon.vue'
import { bytes } from '../lib/format'
import type { IconName } from '../lib/icons'
import { filesView, toast } from '../store'
import { strings } from '../strings'

// ---- where we are ----
const view = ref<'files' | 'trash'>('files')
const cwd = ref('/')
const back: string[] = []
const listing = ref<api.Listing | null>(null)
const loading = ref(false)
const error = ref('')
const top = ref<Entry[]>([])

const readPref = (k: string, d: string) => {
  try {
    return localStorage.getItem(k) ?? d
  } catch {
    return d
  }
}
const writePref = (k: string, v: string) => {
  try {
    localStorage.setItem(k, v)
  } catch {
    // a private window: it lasts until the page closes
  }
}
const hidden = ref(readPref('icedash.files.hidden', '0') === '1')
const mode = ref<'list' | 'grid'>(readPref('icedash.files.view', 'list') === 'grid' ? 'grid' : 'list')
watch(hidden, (v) => {
  writePref('icedash.files.hidden', v ? '1' : '0')
  void load()
})
watch(mode, (v) => writePref('icedash.files.view', v))

const entries = computed(() => listing.value?.entries ?? [])
const crumbs = computed(() => {
  const parts = cwd.value.split('/').filter(Boolean)
  return parts.map((name, i) => ({ name, path: '/' + parts.slice(0, i + 1).join('/') }))
})

async function load() {
  if (view.value === 'trash') return loadTrash()
  loading.value = true
  error.value = ''
  try {
    listing.value = await api.list(cwd.value, hidden.value)
    cwd.value = listing.value.path
  } catch (e) {
    error.value = msg(e)
    listing.value = null
  } finally {
    loading.value = false
  }
}
async function loadTop() {
  try {
    top.value = (await api.list('/', false)).entries.filter((e) => e.dir && !e.protected)
  } catch {
    top.value = []
  }
}

function go(path: string, remember = true) {
  if (remember && (view.value !== 'files' || path !== cwd.value)) back.push(view.value === 'trash' ? '\0trash' : cwd.value)
  view.value = 'files'
  cwd.value = path
  sel.value = new Set()
  void load()
}
function goBack() {
  const p = back.pop()
  if (p === undefined) return
  if (p === '\0trash') return showTrash(false)
  go(p, false)
}
const parent = computed(() => (cwd.value === '/' ? null : cwd.value.slice(0, cwd.value.lastIndexOf('/')) || '/'))

watch(
  () => filesView.value.seq,
  () => {
    if (filesView.value.view === 'trash') showTrash()
    else go(filesView.value.path)
  },
)

// ---- selection ----
const sel = ref<Set<string>>(new Set())
let anchor: string | null = null
const selected = computed(() => entries.value.filter((e) => sel.value.has(e.path)))
const locked = computed(() => selected.value.some((e) => e.protected) || !!listing.value?.protected)

function click(e: MouseEvent, x: Entry) {
  const s = new Set(sel.value)
  if (e.shiftKey && anchor) {
    const paths = entries.value.map((y) => y.path)
    const i = paths.indexOf(anchor)
    const j = paths.indexOf(x.path)
    for (const p of paths.slice(Math.min(i, j), Math.max(i, j) + 1)) s.add(p)
  } else if (e.ctrlKey || e.metaKey) {
    if (s.has(x.path)) s.delete(x.path)
    else s.add(x.path)
    anchor = x.path
  } else {
    s.clear()
    s.add(x.path)
    anchor = x.path
  }
  sel.value = s
}

// ---- opening ----
const previewable = new Set(['image', 'video', 'audio', 'pdf', 'text'])
function open(x: Entry) {
  if (x.broken) return
  if (x.dir) go(x.path)
  else if (previewable.has(x.kind)) void showPreview(x)
  else void download([x])
}

const preview = ref<{ entry: Entry; url: string; text?: string; cut?: boolean } | null>(null)
async function showPreview(x: Entry) {
  try {
    const url = await api.rawUrl(x.path)
    preview.value = { entry: x, url }
    if (x.kind === 'text') {
      const res = await fetch(url, { headers: { Range: 'bytes=0-1048575' } })
      const text = await res.text()
      if (preview.value?.entry.path === x.path) preview.value = { entry: x, url, text, cut: x.size > 1048576 }
    }
  } catch (e) {
    toast({ title: strings.filesPreview, body: msg(e) })
  }
}

async function download(list: Entry[]) {
  try {
    if (list.length === 1 && !list[0]!.dir) api.startDownload(await api.rawUrl(list[0]!.path, true))
    else api.startDownload(await api.zipUrl(list.map((e) => e.path)))
  } catch (e) {
    toast({ title: strings.filesDownload, body: msg(e) })
  }
}

// ---- dialogs ----
const prompt = ref<{ kind: 'mkdir' | 'rename'; value: string; target?: Entry; error: string } | null>(null)
const promptInput = ref<HTMLInputElement | null>(null)
async function ask(kind: 'mkdir' | 'rename', target?: Entry) {
  prompt.value = { kind, value: target?.name ?? '', target, error: '' }
  await nextTick()
  const el = promptInput.value
  if (el) {
    el.focus()
    const dot = target && !target.dir ? target.name.lastIndexOf('.') : -1
    el.setSelectionRange(0, dot > 0 ? dot : el.value.length)
  }
}
async function submitPrompt() {
  const p = prompt.value
  if (!p || !p.value.trim()) return
  try {
    if (p.kind === 'mkdir') await api.mkdir(cwd.value, p.value.trim())
    else if (p.target) await api.rename(p.target.path, p.value.trim())
    prompt.value = null
    await load()
    if (cwd.value === '/') void loadTop()
  } catch (e) {
    p.error = msg(e)
  }
}

// One question for every clash; resolves to the choice, or null for cancel.
const clash = ref<{ names: string[]; resolve: (c: Clash | null) => void } | null>(null)
function askClash(names: string[]): Promise<Clash | null> {
  return new Promise((resolve) => (clash.value = { names, resolve }))
}
function answer(c: Clash | null) {
  clash.value?.resolve(c)
  clash.value = null
}

const confirmBox = ref<{ text: string; ok: string; run: () => Promise<void> } | null>(null)

// ---- clipboard, move, copy ----
const clip = ref<{ kind: 'copy' | 'move'; paths: string[] } | null>(null)
const jobs = ref<Job[]>([])
let jobTimer: ReturnType<typeof setTimeout> | undefined

function copy(kind: 'copy' | 'move') {
  if (!selected.value.length || (kind === 'move' && locked.value)) return
  clip.value = { kind, paths: selected.value.map((e) => e.path) }
}
async function paste() {
  const c = clip.value
  if (!c || listing.value?.protected) return
  await transfer(c.kind, c.paths, cwd.value)
  if (c.kind === 'move') clip.value = null
}
async function transfer(kind: 'copy' | 'move', paths: string[], to: string) {
  let choice: Clash = ''
  for (;;) {
    try {
      const j = await api.transfer(kind, paths, to, choice)
      jobs.value = [j, ...jobs.value.filter((x) => x.id !== j.id)]
      watchJobs()
      return
    } catch (e) {
      if (e instanceof ApiError && e.code === 'exists' && !choice) {
        const names = ((e as ApiError & { names?: string[] }).names ?? []).join(', ')
        const c = await askClash(names ? names.split(', ') : [])
        if (!c) return
        choice = c
        continue
      }
      toast({ title: kind === 'copy' ? strings.filesCopy : strings.filesCut, body: msg(e) })
      return
    }
  }
}
function watchJobs() {
  clearTimeout(jobTimer)
  jobTimer = setTimeout(async () => {
    try {
      const all = await api.jobs()
      const mine = new Set(jobs.value.map((j) => j.id))
      const before = jobs.value
      jobs.value = all.filter((j) => mine.has(j.id))
      for (const j of jobs.value) {
        const was = before.find((b) => b.id === j.id)
        if (was?.state === 'running' && j.state !== 'running') {
          if (j.state === 'failed') toast({ title: j.kind === 'copy' ? strings.filesCopying : strings.filesMoving, body: strings.filesJobFailed(j.error ?? '') })
          void load()
        }
      }
      jobs.value = jobs.value.filter((j) => j.state === 'running')
      if (jobs.value.length) watchJobs()
    } catch {
      watchJobs()
    }
  }, 600)
}

async function del() {
  if (!selected.value.length || locked.value) return
  const paths = selected.value.map((e) => e.path)
  try {
    await api.remove(paths)
    toast({ title: strings.filesMovedToTrash(paths.length), body: '' })
    sel.value = new Set()
    await load()
  } catch (e) {
    toast({ title: strings.filesDelete, body: msg(e) })
  }
}

// ---- uploads ----
interface Up {
  id: number
  name: string
  size: number
  sent: number
  state: 'wait' | 'up' | 'done' | 'failed' | 'skipped'
  error?: string
  file: File
  dir: string
  handle?: Upload
}
const ups = ref<Up[]>([])
let upSeq = 0
let running = false
let upChoice: Clash | null = null
const fileInput = ref<HTMLInputElement | null>(null)
const activeUps = computed(() => ups.value.filter((u) => u.state === 'wait' || u.state === 'up'))
const upTotal = computed(() => {
  const list = ups.value.filter((u) => u.state !== 'skipped')
  const size = list.reduce((n, u) => n + u.size, 0)
  const sent = list.reduce((n, u) => n + (u.state === 'done' ? u.size : u.sent), 0)
  return { size, sent }
})

function queue(files: { file: File; name: string }[]) {
  if (!files.length || listing.value?.protected) return
  if (!activeUps.value.length) {
    ups.value = []
    upChoice = null
  }
  for (const f of files) ups.value.push({ id: ++upSeq, name: f.name, size: f.file.size, sent: 0, state: 'wait', file: f.file, dir: cwd.value })
  void pump()
}
async function pump() {
  if (running) return
  running = true
  try {
    for (;;) {
      const u = ups.value.find((x) => x.state === 'wait')
      if (!u) break
      await send(u, upChoice ?? '')
    }
  } finally {
    running = false
  }
  if (ups.value.length && !ups.value.some((u) => u.state === 'failed')) setTimeout(() => !activeUps.value.length && (ups.value = []), 2500)
  void load()
}
async function send(u: Up, choice: Clash) {
  u.state = 'up'
  u.handle = api.upload(u.dir, u.name, u.file, choice, (n) => (u.sent = n))
  try {
    const r = await u.handle.promise
    u.state = r.skipped ? 'skipped' : 'done'
  } catch (e) {
    if (e instanceof ApiError && e.code === 'exists' && !choice) {
      const c = await askClash([u.name])
      if (!c) {
        u.state = 'skipped'
        return
      }
      upChoice = c
      return send(u, c)
    }
    if (e instanceof ApiError && e.code === 'aborted') {
      u.state = 'skipped'
      return
    }
    u.state = 'failed'
    u.error = msg(e)
  }
}
function cancelUploads() {
  for (const u of ups.value) {
    if (u.state === 'wait') u.state = 'skipped'
    if (u.state === 'up') u.handle?.abort()
  }
}
function picked(e: Event) {
  const input = e.target as HTMLInputElement
  queue([...(input.files ?? [])].map((file) => ({ file, name: file.name })))
  input.value = ''
}

// Dropping files or folders: folders are read entry by entry.
const dragging = ref(false)
let dragDepth = 0
function dragEnter(e: DragEvent) {
  if (!e.dataTransfer?.types.includes('Files') || listing.value?.protected || view.value !== 'files') return
  dragDepth++
  dragging.value = true
}
function dragLeave() {
  dragDepth = Math.max(0, dragDepth - 1)
  if (!dragDepth) dragging.value = false
}
async function drop(e: DragEvent) {
  dragDepth = 0
  dragging.value = false
  if (!e.dataTransfer || listing.value?.protected || view.value !== 'files') return
  const items = [...e.dataTransfer.items].map((i) => i.webkitGetAsEntry?.()).filter(Boolean) as FileSystemEntry[]
  const out: { file: File; name: string }[] = []
  const walk = async (entry: FileSystemEntry, prefix: string): Promise<void> => {
    if (entry.isFile) {
      const file = await new Promise<File>((res, rej) => (entry as FileSystemFileEntry).file(res, rej))
      out.push({ file, name: prefix + entry.name })
    } else if (entry.isDirectory) {
      const reader = (entry as FileSystemDirectoryEntry).createReader()
      for (;;) {
        const batch = await new Promise<FileSystemEntry[]>((res, rej) => reader.readEntries(res, rej))
        if (!batch.length) break
        for (const child of batch) await walk(child, `${prefix}${entry.name}/`)
      }
    }
  }
  if (items.length) for (const it of items) await walk(it, '')
  else for (const file of e.dataTransfer.files) out.push({ file, name: file.name })
  queue(out)
}

// ---- trash ----
const trash = ref<TrashItem[]>([])
const tsel = ref<Set<string>>(new Set())
function showTrash(remember = true) {
  if (remember && view.value === 'files') back.push(cwd.value)
  view.value = 'trash'
  tsel.value = new Set()
  void loadTrash()
}
async function loadTrash() {
  loading.value = true
  error.value = ''
  try {
    trash.value = await api.trash()
  } catch (e) {
    error.value = msg(e)
  } finally {
    loading.value = false
  }
}
function tclick(e: MouseEvent, it: TrashItem) {
  const s = new Set(e.ctrlKey || e.metaKey ? tsel.value : [])
  if (s.has(it.id) && (e.ctrlKey || e.metaKey)) s.delete(it.id)
  else s.add(it.id)
  tsel.value = s
}
async function restore() {
  const ids = [...tsel.value]
  if (!ids.length) return
  try {
    await api.restore(ids)
    toast({ title: strings.filesRestored(ids.length), body: '' })
    tsel.value = new Set()
    await loadTrash()
  } catch (e) {
    toast({ title: strings.filesRestore, body: msg(e) })
  }
}
function purge() {
  const ids = [...tsel.value]
  if (!ids.length) return
  confirmBox.value = {
    text: strings.filesDeleteForGoodConfirm(ids.length),
    ok: strings.filesDeleteForGood,
    run: async () => {
      await api.purge(ids)
      tsel.value = new Set()
      await loadTrash()
    },
  }
}
function empty() {
  confirmBox.value = {
    text: strings.filesEmptyTrashConfirm,
    ok: strings.filesEmptyTrash,
    run: async () => {
      await api.emptyTrash()
      await loadTrash()
    },
  }
}
async function runConfirm() {
  const c = confirmBox.value
  confirmBox.value = null
  try {
    await c?.run()
  } catch (e) {
    toast({ title: c?.ok ?? '', body: msg(e) })
  }
}

// ---- right-click menu and keys ----
const menu = ref<{ x: number; y: number; item: boolean } | null>(null)
const pane = ref<HTMLElement | null>(null)
function context(e: MouseEvent, x?: Entry) {
  if (x && !sel.value.has(x.path)) {
    sel.value = new Set([x.path])
    anchor = x.path
  }
  if (!x) sel.value = new Set()
  const r = pane.value!.getBoundingClientRect()
  menu.value = { x: Math.min(e.clientX - r.left, r.width - 210), y: Math.min(e.clientY - r.top, r.height - 280), item: !!x }
}
function closeMenu() {
  menu.value = null
}
function act(f: () => unknown) {
  closeMenu()
  void f()
}

function key(e: KeyboardEvent) {
  if (prompt.value || clash.value || preview.value || confirmBox.value || view.value !== 'files') return
  if ((e.target as HTMLElement).closest('input, textarea')) return
  const ctrl = e.ctrlKey || e.metaKey
  if (e.key === 'F2' && selected.value.length === 1 && !locked.value) void ask('rename', selected.value[0])
  else if (e.key === 'Delete') void del()
  else if (e.key === 'Enter' && selected.value.length === 1) open(selected.value[0]!)
  else if (e.key === 'Backspace' && parent.value) go(parent.value)
  else if (ctrl && e.key.toLowerCase() === 'a') sel.value = new Set(entries.value.map((x) => x.path))
  else if (ctrl && e.key.toLowerCase() === 'c') copy('copy')
  else if (ctrl && e.key.toLowerCase() === 'x') copy('move')
  else if (ctrl && e.key.toLowerCase() === 'v') void paste()
  else return
  e.preventDefault()
}

// ---- helpers ----
function msg(e: unknown) {
  return e instanceof Error ? e.message : String(e)
}
const kindIcon: Record<string, IconName> = {
  folder: 'folder',
  image: 'image',
  video: 'film',
  audio: 'music',
  text: 'doc',
  pdf: 'doc',
  other: 'file',
}
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
const pct = (done: number, total: number) => (total ? Math.min(100, Math.round((done / total) * 100)) : 100)

onMounted(() => {
  if (filesView.value.view === 'trash') showTrash(false)
  else {
    cwd.value = filesView.value.path
    void load()
  }
  void loadTop()
  document.addEventListener('pointerdown', closeMenu)
})
onBeforeUnmount(() => {
  clearTimeout(jobTimer)
  document.removeEventListener('pointerdown', closeMenu)
})
</script>

<template>
  <div
    ref="pane"
    class="files"
    tabindex="0"
    @keydown="key"
    @dragenter.prevent="dragEnter"
    @dragover.prevent
    @dragleave="dragLeave"
    @drop.prevent="drop"
  >
    <aside class="side">
      <button type="button" class="place" :class="{ on: view === 'files' && cwd === '/' }" @click="go('/')">
        <Icon name="home" :size="16" /> {{ strings.filesHome }}
      </button>
      <button
        v-for="d in top"
        :key="d.path"
        type="button"
        class="place sub"
        :class="{ on: view === 'files' && (cwd === d.path || cwd.startsWith(d.path + '/')) }"
        @click="go(d.path)"
      >
        <Icon name="folder" :size="16" /> {{ d.name }}
      </button>
      <span class="grow" />
      <button type="button" class="place" :class="{ on: view === 'trash' }" @click="showTrash()">
        <Icon name="trash" :size="16" /> {{ strings.filesTrash }}
      </button>
    </aside>

    <section class="main">
      <!-- files -->
      <template v-if="view === 'files'">
        <header class="tools">
          <button type="button" class="ic" :title="strings.filesBack" :aria-label="strings.filesBack" :disabled="!back.length" @click="goBack">
            <Icon name="arrowLeft" :size="16" />
          </button>
          <button type="button" class="ic" :title="strings.filesUp" :aria-label="strings.filesUp" :disabled="!parent" @click="parent && go(parent)">
            <Icon name="up" :size="16" />
          </button>
          <nav class="crumbs" aria-label="Path">
            <button type="button" class="crumb" @click="go('/')">/DATA</button>
            <template v-for="c in crumbs" :key="c.path">
              <Icon name="chevron" :size="12" class="sep" />
              <button type="button" class="crumb" @click="go(c.path)">{{ c.name }}</button>
            </template>
            <Icon v-if="listing?.protected" name="lock" :size="13" class="lockmark" :title="strings.filesProtected" />
          </nav>
          <button type="button" class="ic" :title="strings.filesNewFolder" :aria-label="strings.filesNewFolder" :disabled="listing?.protected" @click="ask('mkdir')">
            <Icon name="folderPlus" :size="17" />
          </button>
          <button type="button" class="ic" :title="strings.filesUpload" :aria-label="strings.filesUpload" :disabled="listing?.protected" @click="fileInput?.click()">
            <Icon name="upload" :size="17" />
          </button>
          <input ref="fileInput" type="file" multiple hidden @change="picked" />
          <span class="split" />
          <button type="button" class="ic" :class="{ on: mode === 'list' }" :title="strings.filesListView" :aria-label="strings.filesListView" @click="mode = 'list'">
            <Icon name="list" :size="16" />
          </button>
          <button type="button" class="ic" :class="{ on: mode === 'grid' }" :title="strings.filesGridView" :aria-label="strings.filesGridView" @click="mode = 'grid'">
            <Icon name="tiles" :size="15" />
          </button>
          <button type="button" class="ic" :class="{ on: hidden }" :title="strings.filesShowHidden" :aria-label="strings.filesShowHidden" :aria-pressed="hidden" @click="hidden = !hidden">
            <Icon :name="hidden ? 'eye' : 'eyeOff'" :size="16" />
          </button>
          <button type="button" class="ic" :title="strings.filesRefresh" :aria-label="strings.filesRefresh" @click="load">
            <Icon name="refresh" :size="16" />
          </button>
        </header>

        <div class="area" @click.self="sel = new Set()" @contextmenu.prevent.self="context($event)">
          <p v-if="error" class="note error" role="alert">{{ error }}</p>
          <p v-else-if="listing && !entries.length" class="note">{{ strings.filesEmpty }}</p>

          <table v-if="mode === 'list' && entries.length" class="table">
            <thead>
              <tr>
                <th>{{ strings.filesName }}</th>
                <th class="num">{{ strings.filesSize }}</th>
                <th class="when">{{ strings.filesModified }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="x in entries"
                :key="x.path"
                :class="{ sel: sel.has(x.path), dim: x.broken, cut: clip?.kind === 'move' && clip.paths.includes(x.path) }"
                @click="click($event, x)"
                @dblclick="open(x)"
                @contextmenu.prevent.stop="context($event, x)"
              >
                <td class="name">
                  <Icon :name="kindIcon[x.kind] ?? 'file'" :size="17" :class="['ki', x.kind]" />
                  <span class="label">{{ x.name }}</span>
                  <Icon v-if="x.protected" name="lock" :size="12" class="lockmark" :title="strings.filesProtected" />
                </td>
                <td class="num mono">{{ x.dir ? '—' : bytes(x.size) }}</td>
                <td class="when mono">{{ when(x.modified) }}</td>
              </tr>
            </tbody>
          </table>

          <div v-else-if="mode === 'grid' && entries.length" class="grid">
            <button
              v-for="x in entries"
              :key="x.path"
              type="button"
              class="tile"
              :class="{ sel: sel.has(x.path), dim: x.broken }"
              @click="click($event, x)"
              @dblclick="open(x)"
              @contextmenu.prevent.stop="context($event, x)"
            >
              <span class="big"><Icon :name="kindIcon[x.kind] ?? 'file'" :size="34" :class="['ki', x.kind]" /></span>
              <span class="label">{{ x.name }}</span>
            </button>
          </div>

          <div v-if="dragging" class="dropzone"><Icon name="upload" :size="28" /> {{ strings.filesDropHere }}</div>
        </div>

        <footer class="status mono">
          <span>{{ sel.size ? strings.filesSelected(sel.size) : strings.filesItems(entries.length) }}</span>
          <span v-if="listing?.hidden && !hidden" class="mut">· {{ strings.filesHiddenCount(listing.hidden) }}</span>
        </footer>
      </template>

      <!-- trash -->
      <template v-else>
        <header class="tools">
          <button type="button" class="ic" :title="strings.filesBack" :aria-label="strings.filesBack" :disabled="!back.length" @click="goBack">
            <Icon name="arrowLeft" :size="16" />
          </button>
          <h3 class="ttl"><Icon name="trash" :size="16" /> {{ strings.filesTrash }}</h3>
          <button type="button" class="pill" :disabled="!tsel.size" @click="restore"><Icon name="restore" :size="15" /> {{ strings.filesRestore }}</button>
          <button type="button" class="pill" :disabled="!tsel.size" @click="purge">{{ strings.filesDeleteForGood }}</button>
          <button type="button" class="pill warn" :disabled="!trash.length" @click="empty">{{ strings.filesEmptyTrash }}</button>
        </header>
        <div class="area">
          <p v-if="error" class="note error" role="alert">{{ error }}</p>
          <p v-else-if="!loading && !trash.length" class="note">{{ strings.filesTrashEmpty }}</p>
          <table v-else class="table">
            <tbody>
              <tr v-for="it in trash" :key="it.id" :class="{ sel: tsel.has(it.id) }" @click="tclick($event, it)">
                <td class="name">
                  <Icon :name="kindIcon[it.kind] ?? 'file'" :size="17" :class="['ki', it.kind]" />
                  <span class="label">{{ it.name }}</span>
                </td>
                <td class="from mut">{{ strings.filesTrashFrom(it.from) }}</td>
                <td class="num mono">{{ it.size ? bytes(it.size) : '—' }}</td>
                <td class="when mono">{{ when(it.deleted) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <!-- uploads and jobs -->
      <div v-if="ups.length || jobs.length" class="progress">
        <div v-if="ups.length" class="prow">
          <span>{{ activeUps.length ? strings.filesUploading(activeUps.length) : strings.filesUploadDone }}</span>
          <span class="bar"><span :style="{ width: pct(upTotal.sent, upTotal.size) + '%' }" /></span>
          <span class="mono">{{ bytes(upTotal.sent) }} / {{ bytes(upTotal.size) }}</span>
          <button v-if="activeUps.length" type="button" class="link" @click="cancelUploads">{{ strings.filesCancel }}</button>
          <button v-else type="button" class="link" @click="ups = []">{{ strings.filesClose }}</button>
        </div>
        <p v-for="u in ups.filter((x) => x.state === 'failed')" :key="u.id" class="perr">{{ strings.filesUploadFailed(u.name, u.error ?? '') }}</p>
        <div v-for="j in jobs" :key="j.id" class="prow">
          <span>{{ j.kind === 'copy' ? strings.filesCopying : strings.filesMoving }}</span>
          <span class="bar"><span :style="{ width: pct(j.done, j.total) + '%' }" /></span>
          <span class="mono">{{ j.total ? `${bytes(j.done)} / ${bytes(j.total)}` : '' }}</span>
          <button type="button" class="link" @click="api.cancelJob(j.id)">{{ strings.filesCancel }}</button>
        </div>
      </div>
    </section>

    <!-- right-click menu -->
    <ul v-if="menu" class="menu" role="menu" :style="{ left: menu.x + 'px', top: menu.y + 'px' }" @pointerdown.stop>
      <template v-if="menu.item">
        <li v-if="selected.length === 1"><button role="menuitem" type="button" @click="act(() => open(selected[0]!))"><Icon name="eye" :size="15" /> {{ selected[0]!.dir ? strings.filesOpen : strings.filesPreview }}</button></li>
        <li><button role="menuitem" type="button" @click="act(() => download(selected))"><Icon name="download" :size="15" /> {{ selected.length === 1 && !selected[0]!.dir ? strings.filesDownload : strings.filesDownloadZip }}</button></li>
        <li v-if="selected.length === 1"><button role="menuitem" type="button" :disabled="locked" @click="act(() => ask('rename', selected[0]))"><Icon name="edit" :size="15" /> {{ strings.filesRename }}</button></li>
        <li><button role="menuitem" type="button" @click="act(() => copy('copy'))"><Icon name="copy" :size="15" /> {{ strings.filesCopy }}</button></li>
        <li><button role="menuitem" type="button" :disabled="locked" @click="act(() => copy('move'))"><Icon name="cut" :size="15" /> {{ strings.filesCut }}</button></li>
        <li><button role="menuitem" type="button" class="danger" :disabled="locked" @click="act(del)"><Icon name="trash" :size="15" /> {{ strings.filesDelete }}</button></li>
      </template>
      <template v-else>
        <li><button role="menuitem" type="button" :disabled="listing?.protected" @click="act(() => ask('mkdir'))"><Icon name="folderPlus" :size="15" /> {{ strings.filesNewFolder }}</button></li>
        <li><button role="menuitem" type="button" :disabled="listing?.protected" @click="act(() => fileInput?.click())"><Icon name="upload" :size="15" /> {{ strings.filesUpload }}</button></li>
      </template>
      <li v-if="clip"><button role="menuitem" type="button" :disabled="listing?.protected" @click="act(paste)"><Icon name="paste" :size="15" /> {{ strings.filesPaste }}</button></li>
    </ul>

    <!-- name prompt -->
    <div v-if="prompt" class="over" @pointerdown.self="prompt = null" @keydown.esc.stop="prompt = null">
      <form class="box" @submit.prevent="submitPrompt">
        <h3>{{ prompt.kind === 'mkdir' ? strings.filesNewFolderTitle : strings.filesRenameTitle(prompt.target?.name ?? '') }}</h3>
        <input ref="promptInput" v-model="prompt.value" :aria-label="strings.filesNamePrompt" spellcheck="false" />
        <p v-if="prompt.error" class="perr" role="alert">{{ prompt.error }}</p>
        <div class="row">
          <button type="button" class="pill" @click="prompt = null">{{ strings.filesCancel }}</button>
          <button type="submit" class="pill primary" :disabled="!prompt.value.trim()">{{ prompt.kind === 'mkdir' ? strings.filesCreate : strings.filesSave }}</button>
        </div>
      </form>
    </div>

    <!-- clash -->
    <div v-if="clash" class="over" @keydown.esc.stop="answer(null)">
      <div class="box" role="alertdialog" :aria-label="strings.filesClashTitle">
        <h3>{{ strings.filesClashTitle }}</h3>
        <p class="mut">{{ strings.filesClashBody(clash.names.slice(0, 5).join(', ') + (clash.names.length > 5 ? '…' : '')) }}</p>
        <div class="row">
          <button type="button" class="pill" @click="answer(null)">{{ strings.filesCancel }}</button>
          <button type="button" class="pill" @click="answer('skip')">{{ strings.filesSkip }}</button>
          <button type="button" class="pill" @click="answer('keep')">{{ strings.filesKeepBoth }}</button>
          <button type="button" class="pill primary" data-autofocus @click="answer('replace')">{{ strings.filesReplace }}</button>
        </div>
      </div>
    </div>

    <!-- confirm -->
    <div v-if="confirmBox" class="over" @keydown.esc.stop="confirmBox = null">
      <div class="box" role="alertdialog">
        <p>{{ confirmBox.text }}</p>
        <div class="row">
          <button type="button" class="pill" @click="confirmBox = null">{{ strings.filesCancel }}</button>
          <button type="button" class="pill warn" @click="runConfirm">{{ confirmBox.ok }}</button>
        </div>
      </div>
    </div>

    <!-- preview -->
    <div v-if="preview" class="over dark" @pointerdown.self="preview = null" @keydown.esc.stop="preview = null">
      <div class="viewer">
        <header>
          <span class="label">{{ preview.entry.name }}</span>
          <button type="button" class="ic" :title="strings.filesDownload" :aria-label="strings.filesDownload" @click="download([preview.entry])"><Icon name="download" :size="16" /></button>
          <button type="button" class="ic" :title="strings.filesClose" :aria-label="strings.filesClose" @click="preview = null"><Icon name="close" :size="16" /></button>
        </header>
        <div class="stage">
          <img v-if="preview.entry.kind === 'image'" :src="preview.url" :alt="preview.entry.name" />
          <video v-else-if="preview.entry.kind === 'video'" :src="preview.url" controls autoplay />
          <audio v-else-if="preview.entry.kind === 'audio'" :src="preview.url" controls autoplay />
          <iframe v-else-if="preview.entry.kind === 'pdf'" :src="preview.url" :title="preview.entry.name" />
          <template v-else-if="preview.entry.kind === 'text'">
            <p v-if="preview.cut" class="mut small">{{ strings.filesPreviewTooBig }}</p>
            <pre v-if="preview.text !== undefined" class="text">{{ preview.text }}</pre>
          </template>
          <p v-else class="mut">{{ strings.filesNoPreview }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.files {
  position: relative;
  height: 100%;
  display: grid;
  grid-template-columns: 190px minmax(0, 1fr);
  outline: none;
}
.side {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px 8px;
  border-right: 1px solid var(--line);
  overflow-y: auto;
}
.grow {
  flex: 1;
}
.place {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 10px;
  border: 0;
  border-radius: 9px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.place.sub {
  padding-left: 18px;
  color: var(--mut);
}
.place:hover {
  background: rgba(var(--fg-rgb), 0.05);
}
.place.on {
  background: rgba(var(--ac-rgb), 0.12);
  color: var(--ac2);
}
.main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.tools {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--line);
}
.ic {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  flex: none;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--mut);
  cursor: pointer;
}
.ic:hover:not(:disabled) {
  background: rgba(var(--fg-rgb), 0.07);
  color: var(--text);
}
.ic.on {
  color: var(--ac);
}
.ic:disabled {
  opacity: 0.35;
  cursor: default;
}
.split {
  width: 1px;
  height: 18px;
  margin: 0 4px;
  background: var(--line);
}
.crumbs {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 2px;
  margin: 0 6px;
  overflow: hidden;
  white-space: nowrap;
}
.crumb {
  padding: 4px 6px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  font-family: var(--font-mono);
  font-size: 12.5px;
  cursor: pointer;
}
.crumb:hover {
  background: rgba(var(--fg-rgb), 0.06);
}
.sep {
  color: var(--mut);
  flex: none;
}
.lockmark {
  color: var(--warn);
  flex: none;
}
.ttl {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 6px;
  font-size: 14px;
}
.area {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 4px 8px 12px;
}
.note {
  margin: 28px auto;
  max-width: 360px;
  color: var(--mut);
  text-align: center;
  font-size: 13.5px;
}
.note.error {
  color: var(--warn);
}
.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13.5px;
  user-select: none;
}
th {
  position: sticky;
  top: 0;
  padding: 7px 10px;
  background: rgba(var(--surface-rgb), 0.96);
  color: var(--mut);
  font-size: 11.5px;
  font-weight: 600;
  text-align: left;
}
td {
  padding: 6px 10px;
  border-top: 1px solid rgba(var(--fg-rgb), 0.04);
}
tbody tr {
  cursor: default;
}
tbody tr:hover {
  background: rgba(var(--fg-rgb), 0.035);
}
tr.sel,
tr.sel:hover {
  background: rgba(var(--ac-rgb), 0.14);
}
tr.dim,
.tile.dim {
  opacity: 0.45;
}
tr.cut {
  opacity: 0.55;
}
.name {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.num {
  width: 90px;
  text-align: right;
  color: var(--mut);
}
.when {
  width: 170px;
  color: var(--mut);
  white-space: nowrap;
}
.from {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12.5px;
}
.mono {
  font-family: var(--font-mono);
  font-size: 12px;
}
.mut {
  color: var(--mut);
}
.ki {
  flex: none;
  color: var(--mut);
}
.ki.folder {
  color: var(--ac);
}
.ki.image,
.ki.video {
  color: var(--ac2);
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(112px, 1fr));
  gap: 6px;
  padding: 8px 2px;
}
.tile {
  display: grid;
  justify-items: center;
  gap: 6px;
  padding: 12px 6px 10px;
  border: 0;
  border-radius: 12px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 12.5px;
  cursor: default;
  min-width: 0;
}
.tile:hover {
  background: rgba(var(--fg-rgb), 0.04);
}
.tile.sel {
  background: rgba(var(--ac-rgb), 0.14);
}
.tile .label {
  max-width: 100%;
}
.big {
  display: grid;
  place-items: center;
  width: 56px;
  height: 56px;
  border-radius: 14px;
  background: rgba(var(--fg-rgb), 0.04);
}
.status {
  display: flex;
  gap: 8px;
  padding: 6px 14px;
  border-top: 1px solid var(--line);
  color: var(--mut);
  font-size: 11.5px;
}
.dropzone {
  position: absolute;
  inset: 8px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 8px;
  border: 2px dashed var(--ac);
  border-radius: 14px;
  background: rgba(var(--surface-rgb), 0.85);
  color: var(--ac2);
  font-size: 14px;
  pointer-events: none;
}
.progress {
  display: grid;
  gap: 6px;
  padding: 8px 14px 10px;
  border-top: 1px solid var(--line);
  font-size: 12.5px;
}
.prow {
  display: grid;
  grid-template-columns: auto 1fr auto auto;
  align-items: center;
  gap: 10px;
}
.bar {
  height: 5px;
  border-radius: 99px;
  background: rgba(var(--fg-rgb), 0.08);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  background: var(--ac);
  transition: width 0.2s;
}
.perr {
  margin: 0;
  color: var(--warn);
  font-size: 12.5px;
}
.link {
  border: 0;
  background: none;
  color: var(--ac);
  font: inherit;
  cursor: pointer;
}
.pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 13px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: rgba(var(--fg-rgb), 0.03);
  color: var(--text);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.pill:disabled {
  opacity: 0.45;
  cursor: default;
}
.pill.primary {
  border-color: transparent;
  background: var(--ac);
  color: var(--ink);
  font-weight: 600;
}
.pill.warn {
  border-color: rgba(var(--warn-rgb), 0.4);
  color: var(--warn);
}
.menu {
  position: absolute;
  z-index: 5;
  min-width: 200px;
  margin: 0;
  padding: 5px;
  list-style: none;
  border-radius: 12px;
  border: 1px solid var(--line);
  background: rgba(var(--surface-rgb), 0.98);
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.6);
}
.menu button {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 7px 10px;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}
.menu button:hover:not(:disabled) {
  background: rgba(var(--ac-rgb), 0.12);
}
.menu button:disabled {
  opacity: 0.4;
  cursor: default;
}
.menu .danger {
  color: var(--warn);
}
.over {
  position: absolute;
  inset: 0;
  z-index: 6;
  display: grid;
  place-items: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.45);
  border-radius: 0 0 18px 18px;
}
.over.dark {
  background: rgba(0, 0, 0, 0.8);
}
.box {
  display: grid;
  gap: 12px;
  width: min(420px, 100%);
  padding: 18px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: rgba(var(--surface-rgb), 0.98);
}
.box h3 {
  margin: 0;
  font-size: 15px;
}
.box p {
  margin: 0;
  font-size: 13.5px;
}
.box input {
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: rgba(var(--fg-rgb), 0.04);
  color: var(--text);
  font: inherit;
}
.row {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}
.viewer {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}
.viewer header {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 4px 8px;
  font-size: 13.5px;
}
.viewer header .label {
  flex: 1;
}
.stage {
  flex: 1;
  min-height: 0;
  display: grid;
  place-items: center;
  overflow: auto;
}
.stage img,
.stage video {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
.stage iframe {
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: 8px;
  background: #fff;
}
.text {
  align-self: start;
  justify-self: stretch;
  margin: 0;
  padding: 12px 14px;
  border-radius: 10px;
  background: #07090c;
  color: #e6edf2;
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}
.small {
  font-size: 12px;
}
@media (max-width: 700px) {
  .files {
    grid-template-columns: 1fr;
  }
  .side {
    display: none;
  }
  .when {
    display: none;
  }
}
</style>

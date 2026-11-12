// A small Markdown renderer for IceStore descriptions and release notes:
// paragraphs, **bold**, `code`, - lists, 1. lists and [links](https://…).
// Everything is HTML-escaped first, so the output is safe for v-html.

const esc = (s: string) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;')

function inline(s: string): string {
  let out = esc(s)
  out = out.replace(/`([^`]+)`/g, '<code>$1</code>')
  out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  // Links: only http(s); the URL is already escaped.
  out = out.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
  return out
}

export function markdown(src: string): string {
  const lines = (src ?? '').replace(/\r\n/g, '\n').split('\n')
  const out: string[] = []
  let para: string[] = []
  let list: { tag: 'ul' | 'ol'; items: string[] } | null = null

  const flushPara = () => {
    if (para.length) out.push(`<p>${inline(para.join(' '))}</p>`)
    para = []
  }
  const flushList = () => {
    if (list) out.push(`<${list.tag}>${list.items.map((i) => `<li>${inline(i)}</li>`).join('')}</${list.tag}>`)
    list = null
  }

  for (const raw of lines) {
    const line = raw.trim()
    const ul = /^[-*]\s+(.*)$/.exec(line)
    const ol = /^\d+\.\s+(.*)$/.exec(line)
    if (!line) {
      flushPara()
      continue // a blank line inside a list keeps the list going
    }
    if (ul || ol) {
      flushPara()
      const tag = ul ? 'ul' : 'ol'
      if (!list || list.tag !== tag) {
        flushList()
        list = { tag, items: [] }
      }
      list.items.push((ul ?? ol)![1]!)
      continue
    }
    if (list && /^\s{2,}/.test(raw)) {
      list.items[list.items.length - 1] += ' ' + line // a wrapped item
      continue
    }
    flushList()
    para.push(line)
  }
  flushPara()
  flushList()
  return out.join('')
}

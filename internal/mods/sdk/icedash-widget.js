/*
 * icedash-widget.js: the bridge between a widget and IceDash.
 *
 * A widget runs in a sandboxed iframe: it can't see IceDash's page, its
 * sign-in or its storage. IceDash sends it, by postMessage, only what its
 * mod.json asked for and the user approved:
 *
 *   IceDash.on('system', f)   CPU, memory, storage and network, every 5 s  (permission: system)
 *   IceDash.on('apps', f)     installed apps: name, icon, state             (permission: apps)
 *   IceDash.location()        a promise of { city, zone } from the clock    (permission: location)
 *   IceDash.on('theme', f)    the active theme's colours (always)
 *
 * The theme's colours are also set on this page as CSS variables (--bg,
 * --surface, --text, --mut, --line, --ac, --ac2, --warn), so a widget can
 * match IceDash with no code.
 */
(function () {
  var handlers = {}
  var last = {}
  var waiting = []
  var parentWin = window.parent

  function emit(kind, data) {
    last[kind] = data
    ;(handlers[kind] || []).forEach(function (f) {
      try {
        f(data)
      } catch (e) {
        console.error(e)
      }
    })
  }

  function applyTheme(t) {
    if (!t || !t.colors) return
    var c = t.colors
    var s = document.documentElement.style
    var rgb = function (hex) {
      var n = parseInt(hex.slice(1), 16)
      return ((n >> 16) & 255) + ', ' + ((n >> 8) & 255) + ', ' + (n & 255)
    }
    s.setProperty('--bg', c.bg)
    s.setProperty('--surface', c.surface)
    s.setProperty('--text', c.text)
    s.setProperty('--mut', 'rgba(' + rgb(c.text) + ', 0.6)')
    s.setProperty('--line', 'rgba(' + rgb(c.text) + ', 0.1)')
    s.setProperty('--ac', c.accent)
    s.setProperty('--ac2', c.accent2)
    s.setProperty('--warn', c.warn)
    s.colorScheme = t.scheme === 'light' ? 'light' : 'dark'
  }

  window.addEventListener('message', function (e) {
    if (e.source !== parentWin || !e.data || typeof e.data.icedash !== 'string') return
    var m = e.data
    if (m.icedash === 'theme') applyTheme(m.data)
    if (m.icedash === 'location') {
      var w = waiting
      waiting = []
      w.forEach(function (r) {
        r(m.data)
      })
      return
    }
    emit(m.icedash, m.data)
  })

  window.IceDash = {
    on: function (kind, f) {
      ;(handlers[kind] = handlers[kind] || []).push(f)
      if (kind in last) f(last[kind])
    },
    location: function () {
      return new Promise(function (resolve) {
        waiting.push(resolve)
        parentWin.postMessage({ icedash: 'location' }, '*')
      })
    },
  }
  parentWin.postMessage({ icedash: 'hello' }, '*')
})()

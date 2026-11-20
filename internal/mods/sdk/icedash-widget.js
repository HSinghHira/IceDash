/*
 * icedash-widget.js: the bridge between a widget and IceDash.
 *
 * A widget runs in a sandboxed iframe: it can't see IceDash's page, its
 * sign-in or its storage. IceDash sends it, by postMessage, only what its
 * mod.json asked for and the user approved:
 *
 *   IceDash.on('system', f)   CPU, memory, storage and network, every 5 s  (permission: system)
 *   IceDash.on('apps', f)     installed apps: name, icon, state             (permission: apps)
 *   IceDash.location()        a promise of { city, zone }, plus place, latitude and longitude
 *                             when the user picked a weather place in Settings (permission: location)
 *   IceDash.on('location', f) the same, again whenever the user picks another place
 *   IceDash.on('theme', f)    the active theme's colours (always)
 *   IceDash.on('units', f)    { temperature: 'c' | 'f' }, the user's choice (always)
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
      // A changed place also reaches on('location') listeners; the same
      // answer again doesn't, so a listener may call location() itself.
      var was = 'location' in last ? JSON.stringify(last.location) : null
      if (was !== null && was !== JSON.stringify(m.data)) emit('location', m.data)
      else last.location = m.data
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

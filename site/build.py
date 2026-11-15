#!/usr/bin/env python3
"""Build IceDash's website into _site/ (standard library only).

    python site/build.py               # writes _site/
    python site/build.py --out public  # somewhere else

The look is the IceStore site's: the same stylesheet, hero and terminal blocks.
Content lives in templates/ and in the lists below; the latest version comes
from the newest v* git tag.
"""
import argparse
import hashlib
import html
import json
import re
import shutil
import subprocess
from pathlib import Path
from urllib.parse import urlparse

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent

ICONS = {
    "github": '<path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.4 5.4 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4"/><path d="M9 18c-4.51 2-5-2-7-2"/>',
    "plus-circle": '<circle cx="12" cy="12" r="10"/><path d="M8 12h8M12 8v8"/>',
    "issues": '<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="1"/>',
    "grid": '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/>',
    "copy": '<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
    "search": '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
    "info": '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>',
    "arrow-left": '<path d="M19 12H5M12 19l-7-7 7-7"/>',
    "arrow-right": '<path d="M5 12h14M12 5l7 7-7 7"/>',
    "package": '<path d="M11 21.73a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73zM12 22V12M3.3 7 12 12l8.7-5"/>',
    "tag": '<path d="M12.59 2.59A2 2 0 0 0 11.17 2H4a2 2 0 0 0-2 2v7.17a2 2 0 0 0 .59 1.41l8.7 8.7a2.43 2.43 0 0 0 3.42 0l6.58-6.58a2.43 2.43 0 0 0 0-3.42z"/><circle cx="7.5" cy="7.5" r=".5"/>',
    "monitor": '<rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/>',
    "download": '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>',
    "globe": '<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20M2 12h20"/>',
    "folder": '<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>',
    "terminal": '<path d="m4 17 6-6-6-6M12 19h8"/>',
    "bell": '<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9M10.3 21a1.94 1.94 0 0 0 3.4 0"/>',
    "key": '<circle cx="7.5" cy="15.5" r="5.5"/><path d="m21 2-9.6 9.6M15.5 7.5l3 3L22 7l-3-3"/>',
    "layers": '<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65M22 12.65l-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>',
    "refresh": '<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8M21 3v5h-5M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16M8 16H3v5"/>',
    "home": '<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .71-1.53l7-6a2 2 0 0 1 2.58 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
    "boxes": '<rect x="2" y="13" width="9" height="9" rx="1"/><rect x="13" y="13" width="9" height="9" rx="1"/><rect x="7.5" y="2" width="9" height="9" rx="1"/>',
    "users": '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
}

# The tour: (tab id, tab label, image, window title, caption)
SHOTS = [
    ("shot-desktop", "Desktop", "desktop", "IceDash",
     "Your apps on the desktop and in the dock, with a status dot each. Cards for the clock, the system, the network and your apps sit on the right."),
    ("shot-store", "IceStore", "icestore", "IceStore",
     "Install from IceStore without typing an IP: the setup fills in the server's address, makes passwords and checks that ports are free."),
    ("shot-menu", "Credentials", "credentials", "Credentials",
     "Right-click any app to open, restart, update, back up or uninstall it, or to see the login it made for you."),
    ("shot-first", "First launch", "first-launch", "Welcome",
     "The first time you open a new app, IceDash shows its address, login and the steps to get started."),
    ("shot-themes", "Themes", "themes", "Settings",
     "Themes change IceDash's colors and wallpaper. Get more themes, and cards for the panel, from collections on GitHub; each card runs in a sandbox."),
]

# What you get: (icon, title, text)
FEATURES = [
    ("monitor", "A real desktop", "Apps on the desktop and in a dock, in windows you can minimise and maximise, under a top bar with CPU, RAM and network speed."),
    ("package", "IceStore built in", "Browse, install and update IceStore apps. Setup fills in your server's address and passwords, and says when an app wants full access."),
    ("boxes", "App bundles", "Sets of apps that work together, like Live TV or Paperwork, installed one after another with one button."),
    ("key", "Logins at a right-click", "Open, logs, restart, stop, update, back up and uninstall from any app's menu, and the login an app generated, read live and never stored."),
    ("folder", "Files", "Browse, upload, download, preview, move and copy everything in /DATA, with a Trash you can restore from."),
    ("terminal", "Terminal", "A real shell in a window, behind your password a second time. Admins only, and every session is logged."),
    ("layers", "Widgets and themes", "Clock, System, Network and Apps widgets, a light Frost look, and more themes and widgets from GitHub, each widget in its own sandbox."),
    ("bell", "Notifications", "App installs, updates and failures in the bell, with toasts you can turn down or off, and Do not disturb."),
    ("search", "Search everything", "Ctrl Space finds apps, files, settings, wallpapers and IceStore apps from one box."),
    ("globe", "Your language", "English, Deutsch, Español, Français and 中文, chosen per person."),
    ("refresh", "Updates itself", "Update now from the menu, or at night if you like. If an update fails, the previous version comes back by itself."),
    ("home", "At your server's address", "On CasaOS IceDash is the main page. On ZimaOS an admin can make it open at the server's address too."),
]


def e(s):
    return html.escape(str(s), quote=True)


def icon(n):
    return f'<svg class="i" viewBox="0 0 24 24" aria-hidden="true">{ICONS[n]}</svg>'


def fill(tpl, values):
    """{{{k}}} raw, {{k}} escaped, {{icon:n}} icon. Every token must be filled."""
    def sub(m):
        raw, k = m.group(1) is not None, m.group(1) or m.group(2)
        if not raw and k.startswith("icon:"):
            if k[5:] not in ICONS:
                raise ValueError(f"unknown icon in template: {k}")
            return icon(k[5:])
        if k not in values:
            raise ValueError(f"unfilled template token: {m.group(0)}")
        return str(values[k]) if raw else e(values[k])
    return re.sub(r"\{\{\{([\w:-]+)\}\}\}|\{\{([\w:-]+)\}\}", sub, tpl)


def new_tab(doc, base_url):
    """Open external links in a new tab, with rel="noopener"."""
    def sub(m):
        attrs = m.group(1)
        href = re.search(r'\shref="(https?://[^"]*)"', attrs)
        if not href or href.group(1).startswith(base_url) or re.search(r"\starget=", attrs):
            return m.group(0)
        return f'<a{attrs} rel="noopener" target="_blank">'
    return re.sub(r"<a(\s[^>]*)>", sub, doc)


def latest_version():
    try:
        out = subprocess.run(["git", "-C", str(ROOT), "tag", "--list", "v*", "--sort=-v:refname"],
                             capture_output=True, text=True, check=True).stdout.split()
        return out[0] if out else "v0"
    except (OSError, subprocess.CalledProcessError):
        return "v0"


def shot_tabs():
    return "".join(
        f'<button class="tab" type="button" role="tab" id="{t}" aria-controls="p-{t}">{e(label)}</button>'
        for t, label, *_ in SHOTS)


def shot_panels():
    out = []
    for i, (t, label, img, win, cap) in enumerate(SHOTS):
        lazy = "eager" if i == 0 else "lazy"
        out.append(
            f'<div class="panel" role="tabpanel" id="p-{t}" aria-labelledby="{t}"{" hidden" if i else ""}>'
            f'<figure class="shot"><div class="chrome" aria-hidden="true"><i></i><i></i><i></i><span>{e(win)}</span></div>'
            f'<img src="img/shots/{img}.webp" alt="IceDash: {e(label)}" width="1440" height="900" loading="{lazy}" decoding="async"></figure>'
            f'<p class="cap">{e(cap)}</p></div>')
    return "\n    ".join(out)


def features():
    return "".join(
        f'<article class="feat"><span class="hb">{icon(ic)}</span><div><h3>{e(t)}</h3><p>{e(p)}</p></div></article>'
        for ic, t, p in FEATURES)


def page(layout, content, cfg, **head):
    values = {**cfg, "content": content, "robots": "index,follow,max-image-preview:large", **head}
    return new_tab(fill(layout, values), cfg["base_url"])


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=str(ROOT / "_site"))
    out = Path(ap.parse_args().out)

    cfg = json.loads((HERE / "site.json").read_text(encoding="utf-8"))
    layout = (HERE / "templates" / "layout.html").read_text(encoding="utf-8")
    index = (HERE / "templates" / "index.html").read_text(encoding="utf-8")
    version = latest_version()

    if out.exists():
        shutil.rmtree(out)
    shutil.copytree(HERE / "static", out)
    # The one-line install and uninstall scripts, at <base_url>install.sh and
    # uninstall.sh. The Pages workflow replaces them with the latest
    # release's copies, so what people run changes only with a release.
    for name in ("install.sh", "uninstall.sh"):
        shutil.copyfile(ROOT / "packaging" / name, out / name)
    # GitHub Pages' custom domain.
    (out / "CNAME").write_text(urlparse(cfg["base_url"]).hostname + "\n", encoding="utf-8")
    css_js = b"".join(p.read_bytes() for p in sorted((HERE / "static").rglob("*")) if p.suffix in (".css", ".js"))
    cfg["asset_v"] = hashlib.sha256(css_js).hexdigest()[:10]

    base = cfg["base_url"]
    ld = {
        "@context": "https://schema.org", "@type": "SoftwareApplication", "name": cfg["site_name"],
        "description": cfg["description"], "url": base, "applicationCategory": "UtilitiesApplication",
        "operatingSystem": "ZimaOS, CasaOS", "softwareVersion": version.lstrip("v"),
        "offers": {"@type": "Offer", "price": "0", "priceCurrency": "USD"},
        "author": {"@type": "Person", "name": cfg["author"], "url": cfg["author_url"]},
        "codeRepository": cfg["repo"], "image": base + "img/og.png",
    }
    home = fill(index, {**cfg, "version": version, "shot_tabs": shot_tabs(),
                        "shot_panels": shot_panels(), "features": features()})
    (out / "index.html").write_text(
        page(layout, home, cfg, root="", url=base, jsonld=json.dumps(ld, ensure_ascii=False)), encoding="utf-8")

    # 404: served for any missing path, so its links start at the site's root
    base_nf = "/"
    nf = (
        '<div class="wrap">\n'
        f'  <div class="top"><a class="logo" href="{base_nf}">{e(cfg["site_name"])}</a>'
        f'<a class="back" href="{base_nf}">{icon("arrow-left")}Home</a></div>\n'
        '  <main id="main" tabindex="-1" data-page="404">\n'
        '    <section class="sec nf" aria-labelledby="nf-h">\n'
        '      <p class="nf-code" aria-hidden="true">404</p>\n'
        '      <h1 id="nf-h">Page not found</h1>\n'
        '      <p class="lead">This page does not exist, or it moved.</p>\n'
        f'      <p class="nf-links"><a class="btn p" href="{base_nf}">{icon("arrow-left")}Go to the homepage</a>'
        f'<a class="btn" href="{base_nf}#install">{icon("download")}Install IceDash</a></p>\n'
        '    </section>\n  </main>\n</div>')
    nf_ld = {"@context": "https://schema.org", "@type": "WebPage", "name": "Page not found", "url": base + "404.html"}
    (out / "404.html").write_text(
        page(layout, nf, {**cfg, "title": f'Page not found – {cfg["site_name"]}', "description": "This page does not exist."},
             root=base_nf, url=base + "404.html", robots="noindex", jsonld=json.dumps(nf_ld)), encoding="utf-8")

    (out / "manifest.webmanifest").write_text(json.dumps({
        "name": cfg["site_name"], "short_name": cfg["site_name"], "description": cfg["tagline"],
        "start_url": ".", "display": "standalone", "background_color": "#0b0d14", "theme_color": "#0b0d14",
        "icons": [
            {"src": "favicon/web-app-manifest-192x192.png", "sizes": "192x192", "type": "image/png"},
            {"src": "favicon/web-app-manifest-512x512.png", "sizes": "512x512", "type": "image/png"},
        ],
    }, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    (out / "robots.txt").write_text(f"User-agent: *\nAllow: /\nSitemap: {base}sitemap.xml\n", encoding="utf-8")
    (out / "sitemap.xml").write_text(
        '<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n'
        f"  <url><loc>{base}</loc></url>\n</urlset>\n", encoding="utf-8")
    (out / ".nojekyll").write_text("", encoding="utf-8")
    print(f"built {out} ({version})")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Build IceDash's website and preview it on localhost, rebuilding when files change.

    python site/serve.py               # http://localhost:8000
    python site/serve.py --port 3000
    python site/serve.py --no-watch    # build once, then serve

Watches site/ (templates, static, build.py, site.json). Refresh the browser
after a rebuild. Unknown paths get _site/404.html, like the live host.
Needs only Python 3.
"""
import argparse
import functools
import http.server
import subprocess
import sys
import threading
import time
import webbrowser
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
OUT = ROOT / "_site"


def build():
    t = time.time()
    r = subprocess.run([sys.executable, str(HERE / "build.py"), "--out", str(OUT)])
    print(f"build {'ok' if r.returncode == 0 else 'FAILED'} in {time.time() - t:.1f}s", flush=True)
    return r.returncode == 0


def snapshot():
    return {p: p.stat().st_mtime for p in HERE.rglob("*") if p.is_file() and "__pycache__" not in p.parts}


def watch():
    last = snapshot()
    while True:
        time.sleep(1)
        now = snapshot()
        if now != last:
            changed = sorted({p.relative_to(ROOT) for p in now.keys() ^ last.keys()}
                             | {p.relative_to(ROOT) for p in now.keys() & last.keys() if now[p] != last[p]})
            print("changed:", ", ".join(map(str, changed[:5])) + (" ..." if len(changed) > 5 else ""), flush=True)
            build()
            last = snapshot()


class Handler(http.server.SimpleHTTPRequestHandler):
    extensions_map = {**http.server.SimpleHTTPRequestHandler.extensions_map,
                      ".webp": "image/webp", ".webmanifest": "application/manifest+json", ".svg": "image/svg+xml"}

    def send_error(self, code, message=None, explain=None):
        page = OUT / "404.html"
        if code == 404 and page.exists():
            body = page.read_bytes()
            self.send_response(404)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            if self.command != "HEAD":
                self.wfile.write(body)
            return
        super().send_error(code, message, explain)

    def end_headers(self):
        self.send_header("Cache-Control", "no-store")
        super().end_headers()

    def log_message(self, fmt, *args):
        pass  # quiet; build messages are what matter


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--port", type=int, default=8000)
    ap.add_argument("--host", default="127.0.0.1")
    ap.add_argument("--no-watch", action="store_true")
    ap.add_argument("--no-open", action="store_true", help="don't open the browser")
    args = ap.parse_args()

    if not build():
        sys.exit(1)
    if not args.no_watch:
        threading.Thread(target=watch, daemon=True).start()

    handler = functools.partial(Handler, directory=str(OUT))
    with http.server.ThreadingHTTPServer((args.host, args.port), handler) as srv:
        url = f"http://localhost:{args.port}/"
        print(f"serving {OUT} at {url}  (Ctrl+C to stop)", flush=True)
        if not args.no_open:
            webbrowser.open(url)
        try:
            srv.serve_forever()
        except KeyboardInterrupt:
            pass


if __name__ == "__main__":
    main()

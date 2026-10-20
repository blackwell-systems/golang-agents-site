#!/usr/bin/env python3
"""Preview a loop-section variant on the real landing page.

Usage: variants/preview.py X [PORT]   (X is a, b, or c)

Builds the site into /tmp/loopv-X, replaces the landing page's current
stage cards (the <ol class="loop-stages"> list and the <p class="loop-back">
line after it) with variants/loop-X.html, and serves it on PORT
(8770 + index by default). With hero=hide in the fragment's first line
(<!-- hero=hide -->), the hero's ring figure is removed too.
"""
import os, re, subprocess, sys, http.server, functools

x = sys.argv[1]
port = int(sys.argv[2]) if len(sys.argv) > 2 else 8770 + "abc".index(x)
site = os.path.expanduser("~/code/golang-agents-site")
out = f"/tmp/loopv-{x}"
env = dict(os.environ, GOCACHE=f"/tmp/gocache-loopv-{x}")
subprocess.run(["go", "run", "./cmd/build", "-out", out], cwd=site, env=env, check=True)
frag = open(f"{site}/variants/loop-{x}.html").read()
page = open(f"{out}/index.html").read()
new, n = re.subn(r'<ol class="loop-stages">.*?</ol>\s*<p class="loop-back">.*?</p>',
                 lambda m: frag, page, count=1, flags=re.S)
if n != 1:
    sys.exit("preview: stage cards not found in index.html")
if frag.lstrip().startswith("<!-- hero=hide -->"):
    new = re.sub(r'<svg class="loop-figure".*?</svg>', "", new, count=1, flags=re.S)
open(f"{out}/index.html", "w").write(new)
print(f"preview {x}: http://127.0.0.1:{port}/", flush=True)
handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=out)
http.server.ThreadingHTTPServer(("127.0.0.1", port), handler).serve_forever()

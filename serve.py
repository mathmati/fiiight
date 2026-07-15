#!/usr/bin/env python3
"""Serve the web/ directory with correct .wasm MIME and single-range HTTP
Range support (bytes=a-b / a- / -n -> 206), which http.server lacks and the
lazy zip mount needs. No special headers (no COOP/COEP) are required -- the
shell avoids SharedArrayBuffer. Plain `python3 -m http.server` from web/
also works on Python >= 3.9 (the shell then falls back to a full download).
?norange=1 makes the server ignore Range, simulating a host without it."""
import http.server
import os
import re
import urllib.parse

os.chdir(os.path.dirname(os.path.abspath(__file__)))


class Handler(http.server.SimpleHTTPRequestHandler):
    extensions_map = {
        **http.server.SimpleHTTPRequestHandler.extensions_map,
        ".wasm": "application/wasm",
        ".mjs": "text/javascript",
    }

    def end_headers(self):
        self.send_header("Cache-Control", "no-cache")
        super().end_headers()

    def do_GET(self):
        rng = self.headers.get("Range", "")
        if "norange" in urllib.parse.urlparse(self.path).query:
            rng = ""
        m = re.fullmatch(r"bytes=(\d*)-(\d*)", rng)
        if not m or (m.group(1) == "" and m.group(2) == ""):
            super().do_GET()
            return
        path = self.translate_path(self.path)
        try:
            f = open(path, "rb")
        except OSError:
            self.send_error(404, "File not found")
            return
        with f:
            size = os.fstat(f.fileno()).st_size
            if m.group(1) == "":  # suffix form: last n bytes
                start = max(0, size - int(m.group(2)))
                end = size - 1
            else:
                start = int(m.group(1))
                end = min(int(m.group(2)), size - 1) if m.group(2) else size - 1
            if start > end or start >= size:
                self.send_response(416)
                self.send_header("Content-Range", "bytes */%d" % size)
                self.end_headers()
                return
            self.send_response(206)
            self.send_header("Content-Type", self.guess_type(path))
            self.send_header("Content-Range", "bytes %d-%d/%d" % (start, end, size))
            self.send_header("Content-Length", str(end - start + 1))
            self.send_header("Accept-Ranges", "bytes")
            self.end_headers()
            f.seek(start)
            remaining = end - start + 1
            while remaining > 0:
                chunk = f.read(min(65536, remaining))
                if not chunk:
                    break
                self.wfile.write(chunk)
                remaining -= len(chunk)


if __name__ == "__main__":
    import sys
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
    http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()

#!/usr/bin/env python3
"""Serve the web/ directory with correct .wasm MIME. No special headers
(no COOP/COEP) are required -- the shell avoids SharedArrayBuffer.
Plain `python3 -m http.server` from web/ also works on Python >= 3.9."""
import http.server
import os

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


if __name__ == "__main__":
    import sys
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
    http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()

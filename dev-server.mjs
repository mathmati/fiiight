#!/usr/bin/env node
// Tiny dependency-free static server for the web/ directory.
// Usage: node web/dev-server.mjs [port]   (port 0 picks a free port)
import http from "node:http";
import { promises as fsp } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const port = Number(process.argv[2] ?? process.env.PORT ?? 8080);
const host = process.env.HOST ?? "127.0.0.1";

const MIME = {
	".html": "text/html; charset=utf-8",
	".js": "text/javascript; charset=utf-8",
	".mjs": "text/javascript; charset=utf-8",
	".css": "text/css; charset=utf-8",
	".json": "application/json",
	".wasm": "application/wasm",
	".zip": "application/zip",
	".png": "image/png",
	".jpg": "image/jpeg",
	".svg": "image/svg+xml",
	".ico": "image/x-icon",
	".txt": "text/plain; charset=utf-8",
};

// Single-range requests (bytes=a-b / bytes=a- / bytes=-n) get a 206 so the
// lazy zip mount works locally; ?norange=1 makes the server ignore Range,
// simulating a host without support (used by web/test/run.mjs).
const server = http.createServer(async (req, res) => {
	try {
		const u = new URL(req.url, "http://x");
		let urlPath = decodeURIComponent(u.pathname);
		if (urlPath.endsWith("/")) urlPath += "index.html";
		const file = path.normalize(path.join(root, urlPath));
		if (!file.startsWith(root + path.sep) && file !== root) {
			res.writeHead(403).end("forbidden");
			return;
		}
		const type = MIME[path.extname(file).toLowerCase()] ?? "application/octet-stream";
		const noRange = u.searchParams.has("norange");
		const m = !noRange && req.headers.range ? /^bytes=(\d*)-(\d*)$/.exec(req.headers.range) : null;
		if (m && (m[1] !== "" || m[2] !== "")) {
			const size = (await fsp.stat(file)).size;
			let start, end;
			if (m[1] === "") { // suffix form: last n bytes
				start = Math.max(0, size - Number(m[2]));
				end = size - 1;
			} else {
				start = Number(m[1]);
				end = m[2] === "" ? size - 1 : Math.min(Number(m[2]), size - 1);
			}
			if (start > end || start >= size) {
				res.writeHead(416, { "Content-Range": `bytes */${size}` }).end();
				return;
			}
			// Read only the requested slice (Range requests hammer this path).
			const body = Buffer.alloc(end - start + 1);
			const fh = await fsp.open(file);
			try {
				await fh.read(body, 0, body.length, start);
			} finally {
				await fh.close();
			}
			res.writeHead(206, {
				"Content-Type": type,
				"Content-Length": body.length,
				"Content-Range": `bytes ${start}-${end}/${size}`,
				"Accept-Ranges": "bytes",
				"Cache-Control": "no-cache",
			});
			res.end(body);
			return;
		}
		const data = await fsp.readFile(file);
		res.writeHead(200, {
			"Content-Type": type,
			"Content-Length": data.length,
			...(noRange ? {} : { "Accept-Ranges": "bytes" }),
			"Cache-Control": "no-cache",
		});
		res.end(data);
	} catch (err) {
		res.writeHead(err && err.code === "ENOENT" ? 404 : 500).end("not found");
	}
});

server.listen(port, host, () => {
	const addr = server.address();
	console.log(`listening on http://${host}:${addr.port}`);
});

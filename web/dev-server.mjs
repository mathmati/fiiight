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

const server = http.createServer(async (req, res) => {
	try {
		let urlPath = decodeURIComponent(new URL(req.url, "http://x").pathname);
		if (urlPath.endsWith("/")) urlPath += "index.html";
		const file = path.normalize(path.join(root, urlPath));
		if (!file.startsWith(root + path.sep) && file !== root) {
			res.writeHead(403).end("forbidden");
			return;
		}
		const data = await fsp.readFile(file);
		res.writeHead(200, {
			"Content-Type": MIME[path.extname(file).toLowerCase()] ?? "application/octet-stream",
			"Content-Length": data.length,
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

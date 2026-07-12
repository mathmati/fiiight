#!/usr/bin/env node
// End-to-end test for the browser fs-shim:
//   1. builds web/test/fstest (GOOS=js GOARCH=wasm) into a scratch dir
//   2. generates a test content.zip (mixed stored + deflate entries)
//   3. serves web/ with dev-server.mjs
//   4. drives headless Chromium over plain CDP (no npm deps)
//   5. asserts all PASS lines in window.__ikemenBootLog, then reloads and
//      asserts the /save/ file persisted via localStorage.
// Usage: node web/test/run.mjs
import { spawn, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import zlib from "node:zlib";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const webDir = path.dirname(testDir);
const chrome = process.env.CHROME_BIN || "/opt/pw-browsers/chromium";
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "ikemen-fstest-"));

// ---- 1. build fstest.wasm --------------------------------------------------
{
	const res = spawnSync("go", ["build", "-o", path.join(testDir, "fstest.wasm"), "."], {
		cwd: path.join(testDir, "fstest"),
		env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
		stdio: "inherit",
	});
	if (res.status !== 0) fail("go build fstest failed");
}

// ---- 2. generate test content.zip ------------------------------------------
// Hand-rolled zip writer so the harness has zero dependencies and we control
// stored-vs-deflate per entry (both paths of the shim's extractor get hit).
function makeZip(entries) {
	const chunks = [];
	const central = [];
	let offset = 0;
	for (const e of entries) {
		const nameBuf = Buffer.from(e.name, "utf-8");
		const isDir = e.name.endsWith("/");
		const data = isDir ? Buffer.alloc(0) : Buffer.from(e.data);
		const crc = zlib.crc32(data) >>> 0;
		const method = e.deflate ? 8 : 0;
		const comp = e.deflate ? zlib.deflateRawSync(data) : data;
		const local = Buffer.alloc(30);
		local.writeUInt32LE(0x04034b50, 0);
		local.writeUInt16LE(20, 4);            // version needed
		local.writeUInt16LE(0, 6);             // flags
		local.writeUInt16LE(method, 8);
		local.writeUInt16LE(0, 10);            // mod time
		local.writeUInt16LE(0, 12);            // mod date
		local.writeUInt32LE(crc, 14);
		local.writeUInt32LE(comp.length, 18);
		local.writeUInt32LE(data.length, 22);
		local.writeUInt16LE(nameBuf.length, 26);
		local.writeUInt16LE(0, 28);            // extra len
		chunks.push(local, nameBuf, comp);

		const cd = Buffer.alloc(46);
		cd.writeUInt32LE(0x02014b50, 0);
		cd.writeUInt16LE(20, 4);               // version made by
		cd.writeUInt16LE(20, 6);               // version needed
		cd.writeUInt16LE(0, 8);                // flags
		cd.writeUInt16LE(method, 10);
		cd.writeUInt32LE(crc, 16);
		cd.writeUInt32LE(comp.length, 20);
		cd.writeUInt32LE(data.length, 24);
		cd.writeUInt16LE(nameBuf.length, 28);
		cd.writeUInt32LE(isDir ? 0x10 : 0, 38); // external attrs (dir bit)
		cd.writeUInt32LE(offset, 42);
		central.push(cd, nameBuf);
		offset += local.length + nameBuf.length + comp.length;
	}
	const cdBuf = Buffer.concat(central);
	const eocd = Buffer.alloc(22);
	eocd.writeUInt32LE(0x06054b50, 0);
	eocd.writeUInt16LE(entries.length, 8);
	eocd.writeUInt16LE(entries.length, 10);
	eocd.writeUInt32LE(cdBuf.length, 12);
	eocd.writeUInt32LE(offset, 16);
	return Buffer.concat([...chunks, cdBuf, eocd]);
}

fs.writeFileSync(path.join(testDir, "content.zip"), makeZip([
	{ name: "data/", data: "" },                          // explicit dir entry
	{ name: "data/hello.txt", data: "hello wasm\n" },     // stored
	{ name: "data/sub/nested.txt", data: "nested content: deflate me deflate me deflate me\n", deflate: true }, // implicit dir + deflate
	{ name: "data/empty.bin", data: "" },
	{ name: "font/foo.def", data: "[Def]\n", deflate: true },
]));

// ---- helpers -----------------------------------------------------------------
const children = [];
function fail(msg) {
	console.error("FAIL: " + msg);
	cleanup();
	process.exit(1);
}
function cleanup() {
	for (const c of children) { try { c.kill("SIGKILL"); } catch { /* gone */ } }
	try { fs.rmSync(scratch, { recursive: true, force: true }); } catch { /* busy */ }
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---- 3. dev server ----------------------------------------------------------
async function startServer() {
	const srv = spawn(process.execPath, [path.join(webDir, "dev-server.mjs"), "0"], { stdio: ["ignore", "pipe", "inherit"] });
	children.push(srv);
	let out = "";
	return await new Promise((resolve, reject) => {
		const t = setTimeout(() => reject(new Error("dev-server did not start")), 10000);
		srv.stdout.on("data", (d) => {
			out += d;
			const m = out.match(/listening on (http:\/\/[^\s]+)/);
			if (m) { clearTimeout(t); resolve(m[1]); }
		});
		srv.on("exit", () => reject(new Error("dev-server exited early")));
	});
}

// ---- 4. Chromium + minimal CDP client ----------------------------------------
async function startChrome() {
	const proc = spawn(chrome, [
		"--headless=new", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
		"--remote-debugging-port=0", `--user-data-dir=${path.join(scratch, "profile")}`,
		"about:blank",
	], { stdio: ["ignore", "ignore", "pipe"] });
	children.push(proc);
	let err = "";
	return await new Promise((resolve, reject) => {
		const t = setTimeout(() => reject(new Error("chromium did not start: " + err)), 20000);
		proc.stderr.on("data", (d) => {
			err += d;
			const m = err.match(/DevTools listening on (ws:\/\/\S+)/);
			if (m) { clearTimeout(t); resolve(m[1]); }
		});
		proc.on("exit", (code) => reject(new Error("chromium exited: " + code + "\n" + err)));
	});
}

class CDP {
	constructor(ws) {
		this.ws = ws;
		this.id = 0;
		this.pending = new Map();
		ws.addEventListener("message", (ev) => {
			const msg = JSON.parse(ev.data);
			if (msg.id !== undefined && this.pending.has(msg.id)) {
				const { resolve, reject } = this.pending.get(msg.id);
				this.pending.delete(msg.id);
				if (msg.error) reject(new Error(msg.error.message));
				else resolve(msg.result);
			}
		});
	}
	static async connect(url) {
		const ws = new WebSocket(url);
		await new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error("ws connect failed")); });
		return new CDP(ws);
	}
	send(method, params = {}, sessionId) {
		const id = ++this.id;
		this.ws.send(JSON.stringify({ id, method, params, sessionId }));
		return new Promise((resolve, reject) => this.pending.set(id, { resolve, reject }));
	}
}

async function evalInPage(cdp, session, expr) {
	const r = await cdp.send("Runtime.evaluate", { expression: expr, returnByValue: true }, session);
	if (r.exceptionDetails) throw new Error("page eval failed: " + JSON.stringify(r.exceptionDetails));
	return r.result.value;
}

async function waitForDone(cdp, session, label) {
	const deadline = Date.now() + 30000;
	while (Date.now() < deadline) {
		const state = await evalInPage(cdp, session,
			"JSON.stringify({done: !!window.__testDone, log: window.__ikemenBootLog || []})");
		const { done, log } = JSON.parse(state);
		if (done) return log;
		await sleep(200);
	}
	throw new Error(label + ": timed out waiting for __testDone");
}

// ---- main ---------------------------------------------------------------------
try {
	const base = await startServer();
	const wsUrl = await startChrome();
	const cdp = await CDP.connect(wsUrl);
	const url = base + "/test/index.html";

	const { targetId } = await cdp.send("Target.createTarget", { url });
	const { sessionId } = await cdp.send("Target.attachToTarget", { targetId, flatten: true });
	await cdp.send("Page.enable", {}, sessionId);
	await cdp.send("Runtime.enable", {}, sessionId);

	// Run 1: fresh localStorage.
	const log1 = await waitForDone(cdp, sessionId, "run 1");
	console.log("--- run 1 output ---");
	for (const line of log1) console.log(line);

	const fails1 = log1.filter((l) => l.startsWith("FAIL") || l.startsWith("HARNESS-ERROR") || l.startsWith("BOOT-ERROR"));
	if (fails1.length) fail("run 1 had failures:\n" + fails1.join("\n"));
	if (!log1.includes("FSTEST-DONE")) fail("run 1 did not complete");
	const passCount = log1.filter((l) => l.startsWith("PASS")).length;
	if (passCount < 15) fail(`run 1: expected >= 15 PASS lines, got ${passCount}`);
	if (!log1.some((l) => l.startsWith("INFO: no previous save"))) {
		fail("run 1 unexpectedly found a previous save (stale profile?)");
	}

	// Run 2: reload; /save/ files must come back from localStorage.
	await cdp.send("Page.navigate", { url }, sessionId);
	await sleep(300); // let the navigation reset window state
	const log2 = await waitForDone(cdp, sessionId, "run 2");
	console.log("--- run 2 output ---");
	for (const line of log2) console.log(line);

	const fails2 = log2.filter((l) => l.startsWith("FAIL") || l.startsWith("HARNESS-ERROR") || l.startsWith("BOOT-ERROR"));
	if (fails2.length) fail("run 2 had failures:\n" + fails2.join("\n"));
	if (!log2.includes("PASS: persisted-from-previous-run")) {
		fail("run 2: /save/ file did not persist across reload");
	}

	console.log("--- OK: all assertions passed ---");
	cleanup();
	process.exit(0);
} catch (err) {
	fail(err.stack || String(err));
}

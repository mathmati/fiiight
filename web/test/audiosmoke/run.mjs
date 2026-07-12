#!/usr/bin/env node
// End-to-end WebAudio smoke test:
//   1. builds web/test/audiosmoke (GOOS=js GOARCH=wasm)
//   2. serves web/ with dev-server.mjs
//   3. drives headless Chromium over plain CDP (no npm deps) with
//      --autoplay-policy=no-user-gesture-required
//   4. asserts a PASS line (non-silent 440Hz sine) in window.__ikemenBootLog
// Usage: node web/test/audiosmoke/run.mjs
import { spawn, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const webDir = path.dirname(path.dirname(testDir));
const chrome = process.env.CHROME_BIN || "/opt/pw-browsers/chromium";
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "ikemen-audiosmoke-"));

// ---- 1. build audiosmoke.wasm ----------------------------------------------
{
	const res = spawnSync("go", ["build", "-o", path.join(testDir, "audiosmoke.wasm"), "."], {
		cwd: testDir,
		env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
		stdio: "inherit",
	});
	if (res.status !== 0) fail("go build audiosmoke failed");
}

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

// ---- 2. dev server ----------------------------------------------------------
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

// ---- 3. Chromium + minimal CDP client ----------------------------------------
async function startChrome() {
	const proc = spawn(chrome, [
		"--headless=new", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
		"--autoplay-policy=no-user-gesture-required",
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
	const url = base + "/test/audiosmoke/index.html";

	const { targetId } = await cdp.send("Target.createTarget", { url });
	const { sessionId } = await cdp.send("Target.attachToTarget", { targetId, flatten: true });
	await cdp.send("Page.enable", {}, sessionId);
	await cdp.send("Runtime.enable", {}, sessionId);

	const log = await waitForDone(cdp, sessionId, "audiosmoke");
	console.log("--- audiosmoke output ---");
	for (const line of log) console.log(line);

	const fails = log.filter((l) => l.startsWith("FAIL") || l.startsWith("BOOT-ERROR"));
	if (fails.length) fail("audiosmoke had failures:\n" + fails.join("\n"));
	if (!log.some((l) => l.startsWith("PASS"))) fail("audiosmoke produced no PASS line");

	console.log("--- OK: audio smoke test passed ---");
	cleanup();
	process.exit(0);
} catch (err) {
	fail(err.stack || String(err));
}

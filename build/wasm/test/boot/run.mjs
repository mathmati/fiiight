#!/usr/bin/env node
// Boot harness: serve bin/Ikemen_GO-Web/ (build it first with
// ./build/build.sh Web), open index.html in headless Chromium (SwiftShader
// WebGL), and watch the real engine boot. Captures window.__ikemenBootLog,
// console messages and page errors; takes a PNG screenshot every ~5s into
// shots/; writes the log tail to lastlog.txt (both beside this script).
//
// Usage: node build/wasm/test/boot/run.mjs
//   env DURATION=<seconds>   total watch time (default 60)
//   env KEYS="40:Enter,45:z" dispatch key at t seconds (key names: Enter, z,
//                            ArrowDown, ArrowUp, Escape, ...)
// Exit code 0 = ran to completion without fatal marker, 1 = fatal detected.
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const wasmDir = path.dirname(path.dirname(testDir)); // build/wasm
const repoRoot = path.dirname(path.dirname(wasmDir));
const outDir = process.env.IKEMEN_WEB_OUT || path.join(repoRoot, "bin", "Ikemen_GO-Web");
const chrome = process.env.CHROME_BIN || "/opt/pw-browsers/chromium";
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "ikemen-boot-"));
const shotsDir = path.join(testDir, "shots");
const duration = Number(process.env.DURATION || 60) * 1000;
const keySpec = (process.env.KEYS || "").split(",").filter(Boolean).map((s) => {
	const [t, key, hold] = s.split(":");
	return { at: Number(t) * 1000, key, hold: hold ? Number(hold) : undefined, done: false };
});

fs.rmSync(shotsDir, { recursive: true, force: true });
fs.mkdirSync(shotsDir, { recursive: true });

const children = [];
function cleanup() {
	for (const c of children) { try { c.kill("SIGKILL"); } catch { /* gone */ } }
	try { fs.rmSync(scratch, { recursive: true, force: true }); } catch { /* busy */ }
}
function fail(msg) {
	console.error("FAIL: " + msg);
	cleanup();
	process.exit(1);
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---- dev server -------------------------------------------------------------
async function startServer() {
	if (!fs.existsSync(path.join(outDir, "index.html"))) {
		fail("missing " + outDir + "/index.html -- run ./build/build.sh Web first (or set IKEMEN_WEB_OUT)");
	}
	const srv = spawn(process.execPath, [path.join(wasmDir, "dev-server.mjs"), "0", outDir], { stdio: ["ignore", "pipe", "inherit"] });
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

// ---- Chromium + minimal CDP client -------------------------------------------
async function startChrome() {
	const proc = spawn(chrome, [
		// Chromium ignores http(s)_proxy env vars; needed when BASE_URL is remote.
		...(process.env.CHROME_PROXY ? [`--proxy-server=${process.env.CHROME_PROXY}`] : []),
		"--headless=new", "--no-sandbox", "--disable-dev-shm-usage",
		"--use-angle=swiftshader", "--enable-unsafe-swiftshader",
		"--autoplay-policy=no-user-gesture-required",
		"--window-size=1280,720",
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
		this.listeners = [];
		ws.addEventListener("message", (ev) => {
			const msg = JSON.parse(ev.data);
			if (msg.id !== undefined && this.pending.has(msg.id)) {
				const { resolve, reject } = this.pending.get(msg.id);
				this.pending.delete(msg.id);
				if (msg.error) reject(new Error(msg.error.message));
				else resolve(msg.result);
			} else if (msg.method) {
				for (const l of this.listeners) l(msg);
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
	on(fn) { this.listeners.push(fn); }
}

async function evalInPage(cdp, session, expr) {
	const r = await cdp.send("Runtime.evaluate", { expression: expr, returnByValue: true }, session);
	if (r.exceptionDetails) throw new Error("page eval failed: " + JSON.stringify(r.exceptionDetails));
	return r.result.value;
}

const KEYDEFS = {
	Enter: { key: "Enter", code: "Enter", windowsVirtualKeyCode: 13, text: "\r" },
	Escape: { key: "Escape", code: "Escape", windowsVirtualKeyCode: 27 },
	ArrowUp: { key: "ArrowUp", code: "ArrowUp", windowsVirtualKeyCode: 38 },
	ArrowDown: { key: "ArrowDown", code: "ArrowDown", windowsVirtualKeyCode: 40 },
	ArrowLeft: { key: "ArrowLeft", code: "ArrowLeft", windowsVirtualKeyCode: 37 },
	ArrowRight: { key: "ArrowRight", code: "ArrowRight", windowsVirtualKeyCode: 39 },
};
function keyDef(name) {
	if (KEYDEFS[name]) return KEYDEFS[name];
	// single letter
	const up = name.toUpperCase();
	return { key: name, code: "Key" + up, windowsVirtualKeyCode: up.charCodeAt(0), text: name };
}
async function pressKey(cdp, session, name, hold) {
	const d = keyDef(name);
	await cdp.send("Input.dispatchKeyEvent", { type: "keyDown", ...d }, session);
	await sleep(hold || Number(process.env.KEYHOLD || 400));
	await cdp.send("Input.dispatchKeyEvent", { type: "keyUp", key: d.key, code: d.code, windowsVirtualKeyCode: d.windowsVirtualKeyCode }, session);
	console.log(`--- pressed ${name} ---`);
}

async function screenshot(cdp, session, name) {
	try {
		const r = await cdp.send("Page.captureScreenshot", { format: "png" }, session);
		fs.writeFileSync(path.join(shotsDir, name), Buffer.from(r.data, "base64"));
		console.log("--- screenshot " + name + " ---");
	} catch (e) {
		console.log("--- screenshot failed: " + e.message + " ---");
	}
}

// ---- main ---------------------------------------------------------------------
try {
	// BASE_URL overrides the local dev-server (e.g. to boot-test the live site).
	const base = process.env.BASE_URL || (await startServer());
	const wsUrl = await startChrome();
	const cdp = await CDP.connect(wsUrl);
	const url = base + "/index.html";

	const { targetId } = await cdp.send("Target.createTarget", { url: "about:blank" });
	const { sessionId } = await cdp.send("Target.attachToTarget", { targetId, flatten: true });
	await cdp.send("Page.enable", {}, sessionId);
	await cdp.send("Runtime.enable", {}, sessionId);
	await cdp.send("Log.enable", {}, sessionId);
	await cdp.send("Network.enable", {}, sessionId);
	await cdp.send("Emulation.setDeviceMetricsOverride", { width: 1280, height: 720, deviceScaleFactor: 1, mobile: false }, sessionId);

	const consoleLines = [];
	cdp.on((msg) => {
		if (msg.sessionId !== sessionId) return;
		if (msg.method === "Runtime.consoleAPICalled") {
			const text = msg.params.args.map((a) => a.value !== undefined ? String(a.value) : (a.description || a.type)).join(" ");
			consoleLines.push(`[console.${msg.params.type}] ${text}`);
			console.log(`[console.${msg.params.type}] ${text.slice(0, 500)}`);
		} else if (msg.method === "Runtime.exceptionThrown") {
			const d = msg.params.exceptionDetails;
			const text = (d.exception && (d.exception.description || d.exception.value)) || d.text;
			consoleLines.push("[pageerror] " + text);
			console.log("[pageerror] " + String(text).slice(0, 1000));
		} else if (msg.method === "Network.responseReceived") {
			const r = msg.params.response;
			if (r.status >= 400) {
				consoleLines.push(`[net] ${r.status} ${r.url}`);
				console.log(`[net] ${r.status} ${r.url}`);
			}
		} else if (msg.method === "Log.entryAdded") {
			consoleLines.push(`[log.${msg.params.entry.level}] ${msg.params.entry.text}`);
			console.log(`[log.${msg.params.entry.level}] ${msg.params.entry.text.slice(0, 500)}`);
		}
	});

	await cdp.send("Page.navigate", { url }, sessionId);
	if (process.env.PROBE) {
		await cdp.send("Page.addScriptToEvaluateOnNewDocument", {
			source: "window.addEventListener('keydown',e=>(window.__ikemenBootLog=window.__ikemenBootLog||[]).push('PROBE keydown code='+e.code+' key='+e.key+' repeat='+e.repeat));" +
				"window.addEventListener('keyup',e=>(window.__ikemenBootLog=window.__ikemenBootLog||[]).push('PROBE keyup code='+e.code));",
		}, sessionId);
		await cdp.send("Page.navigate", { url }, sessionId);
	}

	if (process.env.DBGPOLL) {
		(async () => {
			let last = "";
			for (;;) {
				await sleep(120);
				try {
					const v = await evalInPage(cdp, sessionId, "window.__ikemenDbg || ''");
					if (v && v !== last) { last = v; console.log("[dbg] " + v); }
				} catch { /* navigating */ }
			}
		})();
	}
	const start = Date.now();
	let printed = 0;
	let lastShot = 0;
	let fatal = null;
	let log = [];
	while (Date.now() - start < duration) {
		await sleep(1000);
		const t = Date.now() - start;
		try {
			log = await evalInPage(cdp, sessionId, "JSON.stringify(window.__ikemenBootLog || [])").then(JSON.parse);
		} catch (e) {
			console.log("--- eval failed: " + e.message + " ---");
			continue;
		}
		for (; printed < log.length; printed++) console.log("[boot] " + log[printed].slice(0, 800));
		if (t - lastShot >= 5000) {
			lastShot = t;
			await screenshot(cdp, sessionId, `shot-${String(Math.round(t / 1000)).padStart(3, "0")}s.png`);
			try {
				const perf = await evalInPage(cdp, sessionId,
					"JSON.stringify({fps: window.__ikemenFPS || null, heap: (performance.memory && performance.memory.usedJSHeapSize) || null, lazy: window.__fsLazyStats || null})");
				console.log("--- perf " + perf + " ---");
			} catch { /* page busy */ }
		}
		const bad = log.find((l) => /panic:|BOOT-ERROR|Engine exited/.test(l));
		if (bad) { fatal = bad; break; }
		for (const k of keySpec) {
			if (!k.done && t >= k.at) {
				k.done = true;
				await pressKey(cdp, sessionId, k.key, k.hold);
				await sleep(1500);
				await screenshot(cdp, sessionId, `shot-key-${k.key}-${Math.round(t / 1000)}s.png`);
			}
		}
	}
	await sleep(500);
	await screenshot(cdp, sessionId, "shot-final.png");

	const tail = log.slice(-400);
	fs.writeFileSync(path.join(testDir, "lastlog.txt"),
		tail.join("\n") + "\n\n=== console ===\n" + consoleLines.slice(-200).join("\n") + "\n");
	console.log("--- log tail written to lastlog.txt (" + log.length + " lines total) ---");

	if (fatal) {
		console.error("FATAL: " + fatal);
		cleanup();
		process.exit(1);
	}
	console.log("--- OK: ran " + Math.round((Date.now() - start) / 1000) + "s without fatal marker ---");
	cleanup();
	process.exit(0);
} catch (err) {
	fail(err.stack || String(err));
}

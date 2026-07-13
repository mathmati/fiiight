#!/usr/bin/env node
// Loader harness: verifies the "load your own game" flow end to end.
//
// Phase A: boot the stock game, assert no stored zip, baseline roster
//          (derived at runtime) + randomselect, navigate to VS char select,
//          screenshot baseline.
// Phase B: build a test zip (MyGame/chars/kfmcopy/* -- a renamed copy of
//          content/chars/kfm with displayname "KFM Copy"), install it via
//          window.__ikemenLoader.installBlob (in-page File from the served
//          fixture), wait for the self-reload, boot again, assert the memfs
//          select.def gained a "kfmcopy" roster line (merge mode + wrapper
//          strip), navigate to VS char select, screenshot (8 chars + random).
// Phase C: window.__ikemenLoader.reset(), wait for reload, assert IndexedDB
//          cleared and roster back to baseline, screenshot char select again.
// Phase D: plant a corrupt stored zip, reload: boot must survive and clear it.
// Phase E: install a bare stage zip (.def + .sff at the zip root, no
//          chars/stages layout): must be mounted under stages/ and appended
//          to [ExtraStages], with the character roster untouched.
//
// Fixture sources: the harness needs a KFM-style character directory
// (containing kfm.def with displayname "Kung Fu Man") and a stage directory
// (containing stage0-720.def/.sff). Point CHAR_SRC and STAGE_SRC at them;
// without both the harness skips (exit 0), since the repo-default web build
// ships no chars or stages. Serves bin/Ikemen_GO-Web/ (build it first with
// ./build/build.sh Web).
//
// Usage: node build/wasm/test/loader/run.mjs   exit 0 = green/skip, 1 = failure.
import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const wasmDir = path.dirname(path.dirname(testDir)); // build/wasm
const repoRoot = path.dirname(path.dirname(wasmDir));
const outDir = process.env.IKEMEN_WEB_OUT || path.join(repoRoot, "bin", "Ikemen_GO-Web");
const charSrc = process.env.CHAR_SRC || path.join(repoRoot, "chars", "kfm");
const stageSrc = process.env.STAGE_SRC || path.join(repoRoot, "stages");
const chrome = process.env.CHROME_BIN || "/opt/pw-browsers/chromium";
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "ikemen-loader-"));
const shotsDir = path.join(testDir, "shots");
const fixtureDir = path.join(testDir, "fixture");

fs.rmSync(shotsDir, { recursive: true, force: true });
fs.mkdirSync(shotsDir, { recursive: true });
fs.mkdirSync(fixtureDir, { recursive: true });

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

// ---- fixture: store-only zip writer ------------------------------------------
const CRC_TABLE = (() => {
	const t = new Uint32Array(256);
	for (let n = 0; n < 256; n++) {
		let c = n;
		for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
		t[n] = c >>> 0;
	}
	return t;
})();
function crc32(buf) {
	let c = 0xffffffff;
	for (let i = 0; i < buf.length; i++) c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
	return (c ^ 0xffffffff) >>> 0;
}
function buildZip(files) {
	const localParts = [];
	const centralParts = [];
	let offset = 0;
	for (const f of files) {
		const nameBuf = Buffer.from(f.name, "utf8");
		const crc = crc32(f.data);
		const lh = Buffer.alloc(30);
		lh.writeUInt32LE(0x04034b50, 0);
		lh.writeUInt16LE(20, 4);
		lh.writeUInt32LE(crc, 14);
		lh.writeUInt32LE(f.data.length, 18);
		lh.writeUInt32LE(f.data.length, 22);
		lh.writeUInt16LE(nameBuf.length, 26);
		localParts.push(lh, nameBuf, f.data);
		const ch = Buffer.alloc(46);
		ch.writeUInt32LE(0x02014b50, 0);
		ch.writeUInt16LE(20, 4);
		ch.writeUInt16LE(20, 6);
		ch.writeUInt32LE(crc, 16);
		ch.writeUInt32LE(f.data.length, 20);
		ch.writeUInt32LE(f.data.length, 24);
		ch.writeUInt16LE(nameBuf.length, 28);
		ch.writeUInt32LE(offset, 42);
		centralParts.push(ch, nameBuf);
		offset += 30 + nameBuf.length + f.data.length;
	}
	const central = Buffer.concat(centralParts);
	const eocd = Buffer.alloc(22);
	eocd.writeUInt32LE(0x06054b50, 0);
	eocd.writeUInt16LE(files.length, 8);
	eocd.writeUInt16LE(files.length, 10);
	eocd.writeUInt32LE(central.length, 12);
	eocd.writeUInt32LE(offset, 16);
	return Buffer.concat([...localParts, central, eocd]);
}
function buildFixture() {
	const srcDir = charSrc;
	const files = [];
	for (const fname of fs.readdirSync(srcDir).sort()) {
		let data = fs.readFileSync(path.join(srcDir, fname));
		let out = fname;
		if (fname === "kfm.def") {
			out = "kfmcopy.def";
			const text = data.toString("latin1")
				.replace(/displayname\s*=\s*"Kung Fu Man"/, 'displayname = "KFM Copy"');
			if (!text.includes("KFM Copy")) throw new Error("fixture: displayname patch did not apply");
			data = Buffer.from(text, "latin1");
		}
		// Wrapper dir "MyGame/" exercises wrapper stripping; no select.def
		// exercises merge mode. LAYOUT=bare zips the char folder itself
		// (root = kfmcopy/) to exercise the bare-char-folder fallback.
		const zipPrefix = process.env.LAYOUT === "bare" ? "kfmcopy/" : "MyGame/chars/kfmcopy/";
		files.push({ name: zipPrefix + out, data });
	}
	const zip = buildZip(files);
	fs.writeFileSync(path.join(fixtureDir, "kfmcopy.zip"), zip);
	console.log("--- fixture kfmcopy.zip: " + files.length + " files, " + zip.length + " bytes ---");
}
// Bare stage zip: stage0-720 renamed to stagecopy, .def + .sff at the zip
// root (no wrapper, no stages/ prefix) — exercises the bare-stage fallback.
function buildStageFixture() {
	const srcDir = stageSrc;
	const def = fs.readFileSync(path.join(srcDir, "stage0-720.def")).toString("latin1")
		.replace(/spr\s*=\s*stage0-720\.sff/, "spr = stagecopy.sff");
	if (!def.includes("spr = stagecopy.sff")) throw new Error("stage fixture: spr patch did not apply");
	const zip = buildZip([
		{ name: "stagecopy.def", data: Buffer.from(def, "latin1") },
		{ name: "stagecopy.sff", data: fs.readFileSync(path.join(srcDir, "stage0-720.sff")) },
	]);
	fs.writeFileSync(path.join(fixtureDir, "stagecopy.zip"), zip);
	console.log("--- fixture stagecopy.zip: " + zip.length + " bytes ---");
}

// ---- dev server ---------------------------------------------------------------
async function startServer() {
	const srv = spawn(process.execPath, [path.join(wasmDir, "dev-server.mjs"), "0", outDir, wasmDir], { stdio: ["ignore", "pipe", "inherit"] });
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

// ---- Chromium + minimal CDP client ---------------------------------------------
async function startChrome() {
	const proc = spawn(chrome, [
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

let cdp, sessionId;

async function evalInPage(expr, awaitPromise) {
	const r = await cdp.send("Runtime.evaluate", { expression: expr, returnByValue: true, awaitPromise: !!awaitPromise }, sessionId);
	if (r.exceptionDetails) {
		const d = r.exceptionDetails;
		throw new Error("page eval failed: " + ((d.exception && (d.exception.description || d.exception.value)) || d.text));
	}
	return r.result.value;
}

const KEYDEFS = {
	Enter: { key: "Enter", code: "Enter", windowsVirtualKeyCode: 13, text: "\r" },
	ArrowDown: { key: "ArrowDown", code: "ArrowDown", windowsVirtualKeyCode: 40 },
	F8: { key: "F8", code: "F8", windowsVirtualKeyCode: 119 },
};
// Long hold: menu input is edge-triggered per engine frame, and short taps
// can fall between frames under SwiftShader. 1500 ms registers exactly once
// (cursor autorepeat delay is longer than this; verified empirically).
async function pressKey(name, hold) {
	const d = KEYDEFS[name];
	await cdp.send("Input.dispatchKeyEvent", { type: "keyDown", ...d }, sessionId);
	await sleep(hold || 1500);
	await cdp.send("Input.dispatchKeyEvent", { type: "keyUp", key: d.key, code: d.code, windowsVirtualKeyCode: d.windowsVirtualKeyCode }, sessionId);
	console.log(`--- pressed ${name} ---`);
}

async function screenshot(name) {
	const r = await cdp.send("Page.captureScreenshot", { format: "png" }, sessionId);
	fs.writeFileSync(path.join(shotsDir, name), Buffer.from(r.data, "base64"));
	console.log("--- screenshot " + name + " ---");
}

// Wait until the boot overlay is hidden (engine started) or fail on fatal log.
async function waitBoot(label, timeoutSec = 150) {
	for (let i = 0; i < timeoutSec; i++) {
		await sleep(1000);
		let st = null;
		try {
			st = JSON.parse(await evalInPage(
				"JSON.stringify({hidden: document.getElementById('overlay').classList.contains('hidden')," +
				" fatal: (window.__ikemenBootLog||[]).find(l=>/panic:|BOOT-ERROR|Engine exited/.test(l))||null})"));
		} catch { continue; /* mid-navigation */ }
		if (st.fatal) fail(label + ": fatal during boot: " + st.fatal);
		if (st.hidden) { console.log("--- " + label + ": engine running (t=" + (i + 1) + "s) ---"); return; }
	}
	fail(label + ": boot timeout");
}

// From a fresh engine start: dismiss the welcome infobox (first boot in this
// browser profile only -- its "shown" flag persists via the /save/
// localStorage mirror), move the title cursor down to VS MODE, open the VS
// submenu, pick 1P VS 2P, land on the VERSUS char select, then screenshot.
async function navToCharSelect(tag, firstBoot) {
	await sleep(15000); // title (plus welcome infobox on first boot) is up
	if (firstBoot) {
		await pressKey("Enter");    // dismiss infobox
		await sleep(7000);
	}
	await pressKey("ArrowDown");    // ARCADE -> VS MODE
	await sleep(7000);
	await pressKey("Enter");        // open VS submenu (1P VS 2P selected)
	await sleep(7000);
	await pressKey("Enter");        // 1P VS 2P -> VERSUS char select
	await sleep(9000);
	await screenshot(tag + "-charselect.png");
}

// Entry lines of a section of the in-memfs select.def.
async function sectionLines(section) {
	const sel = await evalInPage("window.__ikemenLoader.readFile('/ikemen/data/select.def')", true);
	const out = [];
	let inSec = false;
	for (const raw of sel.split(/\r?\n/)) {
		const t = raw.trim();
		if (t.startsWith("[")) {
			if (inSec) break;
			inSec = t.toLowerCase().startsWith("[" + section);
			continue;
		}
		if (inSec && t && !t.startsWith(";")) out.push(t.split(",")[0].trim());
	}
	return out;
}
const rosterLines = () => sectionLines("characters");
const extraStageLines = () => sectionLines("extrastages");

async function waitReload(label) {
	// Set a marker; when it disappears the page has navigated away.
	await evalInPage("window.__loaderTestMark = 1");
	for (let i = 0; i < 60; i++) {
		await sleep(1000);
		let gone = false;
		try { gone = await evalInPage("window.__loaderTestMark === undefined"); } catch { continue; }
		if (gone) { console.log("--- " + label + ": reloaded ---"); return; }
	}
	fail(label + ": page never reloaded");
}

// ---- main -----------------------------------------------------------------------
try {
	if (!fs.existsSync(path.join(charSrc, "kfm.def")) ||
		!fs.existsSync(path.join(stageSrc, "stage0-720.def"))) {
		console.log("SKIP: loader test needs fixture sources; set CHAR_SRC to a kfm char dir " +
			"and STAGE_SRC to a dir containing stage0-720.def/.sff");
		cleanup();
		process.exit(0);
	}
	if (!fs.existsSync(path.join(outDir, "index.html"))) {
		fail("missing " + outDir + "/index.html -- run ./build/build.sh Web first (or set IKEMEN_WEB_OUT)");
	}
	buildFixture();
	buildStageFixture();
	const base = await startServer();
	const wsUrl = await startChrome();
	cdp = await CDP.connect(wsUrl);

	const { targetId } = await cdp.send("Target.createTarget", { url: "about:blank" });
	({ sessionId } = await cdp.send("Target.attachToTarget", { targetId, flatten: true }));
	await cdp.send("Page.enable", {}, sessionId);
	await cdp.send("Runtime.enable", {}, sessionId);
	await cdp.send("Emulation.setDeviceMetricsOverride", { width: 1280, height: 720, deviceScaleFactor: 1, mobile: false }, sessionId);
	cdp.on((msg) => {
		if (msg.sessionId !== sessionId) return;
		if (msg.method === "Runtime.consoleAPICalled") {
			const text = msg.params.args.map((a) => a.value !== undefined ? String(a.value) : (a.description || a.type)).join(" ");
			if (/loader|error|fail/i.test(text)) console.log(`[console.${msg.params.type}] ${text.slice(0, 300)}`);
		} else if (msg.method === "Runtime.exceptionThrown") {
			const d = msg.params.exceptionDetails;
			console.log("[pageerror] " + String((d.exception && (d.exception.description || d.exception.value)) || d.text).slice(0, 500));
		}
	});

	await cdp.send("Page.navigate", { url: base + "/index.html" }, sessionId);

	// ---- Phase UI: loader controls usable while the boot overlay is up ----
	// Regression for two reported desktop bugs: the touch gamepad (z-index 40,
	// auto-enabled whenever maxTouchPoints > 0 -- i.e. many desktops) used to
	// cover the "Load game..." pill (then z-index 20) with touch-only buttons
	// that swallowed mouse clicks, so the pill was dead and the F8-shown UI
	// was click-dead too. Force the pad on (worst-case stacking) and assert
	// the pill wins the hit test everywhere and a real CDP mouse click lands.
	{
		let seen = null;
		for (let i = 0; i < 160; i++) {
			await sleep(50);
			try {
				seen = await evalInPage(
					"(() => { const ov = document.getElementById('overlay');" +
					" const pill = document.getElementById('loader-pill');" +
					" if (!ov || !pill || !window.__ikemenTouch) return null;" +
					" if (ov.classList.contains('hidden')) return 'boot-done';" +
					" return 'ready'; })()");
			} catch { continue; }
			if (seen) break;
		}
		if (seen !== "ready") fail("phase UI: never saw pill while boot overlay shown (state: " + seen + ")");
		const hit = JSON.parse(await evalInPage(
			"(() => {" +
			"  window.__ikemenTouch.show();" + // what touch-capable desktops get automatically
			"  const pill = document.getElementById('loader-pill');" +
			"  const orig = pill.textContent;" +
			"  const out = { dead: [] };" +
			"  for (const label of ['Load game\\u2026', 'Load game\\u2026 (current: mygame.zip)']) {" +
			"    pill.textContent = label;" +
			"    const r = pill.getBoundingClientRect();" +
			"    for (let fx = 0.06; fx <= 0.95; fx += 0.11) {" +
			"      for (let fy = 0.2; fy <= 0.81; fy += 0.3) {" +
			"        const x = r.left + r.width * fx, y = r.top + r.height * fy;" +
			"        const el = document.elementFromPoint(x, y);" +
			"        if (!el || !(el === pill || pill.contains(el)))" +
			"          out.dead.push([Math.round(x), Math.round(y), el ? (el.id || el.className) : null]);" +
			"      }" +
			"    }" +
			"  }" +
			"  pill.textContent = orig;" +
			"  const r = pill.getBoundingClientRect();" +
			"  out.center = { x: r.left + r.width / 2, y: r.top + r.height / 2 };" +
			"  return JSON.stringify(out);" +
			"})()"));
		if (hit.dead.length) fail("phase UI: pill covered at " + JSON.stringify(hit.dead.slice(0, 6)));
		// Real mouse click at the pill center must reach the pill and trigger
		// the hidden file input (spied: headless has no file chooser).
		await evalInPage(
			"(() => { window.__uiProbe = { pill: 0, input: 0 };" +
			" document.getElementById('loader-pill').addEventListener('click', () => window.__uiProbe.pill++);" +
			" const inp = document.querySelector('input[type=file]');" +
			" window.__uiProbeOrigClick = inp.click.bind(inp);" +
			" inp.click = () => { window.__uiProbe.input++; }; return true; })()");
		await cdp.send("Input.dispatchMouseEvent", { type: "mousePressed", x: hit.center.x, y: hit.center.y, button: "left", clickCount: 1 }, sessionId);
		await cdp.send("Input.dispatchMouseEvent", { type: "mouseReleased", x: hit.center.x, y: hit.center.y, button: "left", clickCount: 1 }, sessionId);
		await sleep(200);
		const probe = await evalInPage(
			"(() => { const p = window.__uiProbe;" +
			" const inp = document.querySelector('input[type=file]');" +
			" inp.click = window.__uiProbeOrigClick; return p; })()");
		if (!probe || probe.pill !== 1 || probe.input !== 1) {
			fail("phase UI: pill click during overlay did not reach it: " + JSON.stringify(probe));
		}
		// Drop indicator must also stack above the touch pad during boot.
		const dropOK = await evalInPage(
			"(() => {" +
			"  const dt = new DataTransfer();" +
			"  dt.items.add(new File([new Uint8Array([1])], 'x.zip'));" +
			"  window.dispatchEvent(new DragEvent('dragenter', { dataTransfer: dt }));" +
			"  const drop = document.getElementById('loader-drop');" +
			"  const active = drop.classList.contains('active');" +
			"  const above = parseInt(getComputedStyle(drop).zIndex, 10) >" +
			"    parseInt(getComputedStyle(document.getElementById('ikemen-touch')).zIndex, 10);" +
			"  window.dispatchEvent(new DragEvent('dragleave', { dataTransfer: dt }));" +
			"  const inactive = !drop.classList.contains('active');" +
			"  window.__ikemenTouch.hide();" +
			"  try { localStorage.removeItem('ikemen.touch.hidden'); } catch (e) { }" +
			"  return active && above && inactive;" +
			"})()");
		if (!dropOK) fail("phase UI: drop indicator missing or stacked under the touch pad");
		console.log("--- phase UI: pill clickable over touch pad during overlay, drop indicator above pad ---");
	}

	// ---- Phase A: baseline ----
	await waitBoot("phase A");
	const storedA = await evalInPage("window.__ikemenLoader.getStored()", true);
	if (storedA !== null && storedA !== undefined) fail("phase A: expected no stored game, got " + JSON.stringify(storedA));
	// Baseline roster is derived at runtime (the base content.zip may evolve);
	// phases B/C assert relative to it.
	const rosterA = await rosterLines();
	console.log("phase A roster: " + JSON.stringify(rosterA));
	if (rosterA.length < 2 || rosterA.includes("kfmcopy") || !rosterA.includes("randomselect")) {
		fail("phase A: unexpected baseline roster: " + JSON.stringify(rosterA));
	}
	// UI checks (post-boot, engine running): loader UI hidden, F8 toggles it.
	const uiHidden = await evalInPage("document.getElementById('loader-ui').classList.contains('hidden')");
	if (!uiHidden) fail("phase A: loader UI should be hidden after boot");
	await pressKey("F8", 100);
	const uiShown = await evalInPage("!document.getElementById('loader-ui').classList.contains('hidden')");
	if (!uiShown) fail("phase A: F8 did not show loader UI");
	await pressKey("F8", 100);
	const uiReHidden = await evalInPage("document.getElementById('loader-ui').classList.contains('hidden')");
	if (!uiReHidden) fail("phase A: F8 did not hide loader UI again");
	console.log("--- phase A: F8 toggle OK ---");

	// Drag indicator: synthetic dragenter with a Files DataTransfer shows the
	// full-page drop overlay; dragleave hides it.
	const dragOK = await evalInPage(
		"(() => {" +
		"  const dt = new DataTransfer();" +
		"  dt.items.add(new File([new Uint8Array([1])], 'x.zip'));" +
		"  window.dispatchEvent(new DragEvent('dragenter', { dataTransfer: dt }));" +
		"  const active = document.getElementById('loader-drop').classList.contains('active');" +
		"  window.dispatchEvent(new DragEvent('dragleave', { dataTransfer: dt }));" +
		"  const inactive = !document.getElementById('loader-drop').classList.contains('active');" +
		"  return active && inactive;" +
		"})()");
	if (!dragOK) fail("phase A: drag indicator did not toggle on dragenter/dragleave");
	console.log("--- phase A: drag indicator OK ---");

	// Update link: present, and with the served version.txt matching the boot
	// stamp, "Check for updates" must report up-to-date without reloading.
	const updState = await evalInPage(
		"(async () => {" +
		"  if (!document.getElementById('loader-update')) return 'missing-el';" +
		"  if (typeof window.__ikemenLoader.updateGame !== 'function') return 'missing-api';" +
		"  if (!window.__ikemenBuild) return 'no-stamp';" + // stamp-less host: skip the click
		"  const reloaded = await window.__ikemenLoader.updateGame(false);" +
		"  return reloaded === false ? 'up-to-date' : 'unexpected-reload';" +
		"})()", true);
	if (updState !== "up-to-date" && updState !== "no-stamp") {
		fail("phase A: update check state: " + updState);
	}
	console.log("--- phase A: update link OK (" + updState + ") ---");

	// Rejection paths: non-zip content and oversize files must be refused
	// with a status message and nothing stored.
	const badRejected = await evalInPage(
		"window.__ikemenLoader.installBlob(new File([new Uint8Array([1,2,3,4])], 'notazip.zip'))", true);
	if (badRejected !== false) fail("phase A: non-zip file was not rejected");
	const bigRejected = await evalInPage(
		"window.__ikemenLoader.installBlob({ name: 'big.zip', size: 501*1024*1024, arrayBuffer: async () => new ArrayBuffer(0) })", true);
	if (bigRejected !== false) fail("phase A: oversize file was not rejected");
	const statusText = await evalInPage("document.getElementById('loader-status').textContent");
	if (!statusText) fail("phase A: no status message after rejection");
	const storedAfterBad = await evalInPage("window.__ikemenLoader.getStored()", true);
	if (storedAfterBad !== null && storedAfterBad !== undefined) fail("phase A: rejected file ended up stored");
	console.log("--- phase A: non-zip + oversize rejection OK (status: " + JSON.stringify(statusText) + ") ---");

	await navToCharSelect("a-baseline", true);

	// ---- Phase B: install kfmcopy.zip ----
	const installed = await evalInPage(
		"(async () => {" +
		"  const r = await fetch('test/loader/fixture/kfmcopy.zip');" +
		"  if (!r.ok) throw new Error('fixture fetch: ' + r.status);" +
		"  const b = await r.blob();" +
		"  const f = new File([b], 'kfmcopy.zip', { type: 'application/zip' });" +
		"  return await window.__ikemenLoader.installBlob(f);" +
		"})()", true);
	if (installed !== true) fail("phase B: installBlob returned " + JSON.stringify(installed));
	await waitReload("phase B");
	// While the boot overlay is still up, the "Load game..." pill must be
	// visible (pre-boot UI). Poll: the fresh document may still be parsing.
	{
		let pillState = "unknown";
		for (let i = 0; i < 40; i++) {
			await sleep(100);
			let r = null;
			try {
				r = await evalInPage(
					"(() => { const ov = document.getElementById('overlay');" +
					" if (!ov) return 'loading';" +
					" if (ov.classList.contains('hidden')) return 'boot-done';" +
					" const ui = document.getElementById('loader-ui');" +
					" if (!ui) return 'loading';" +
					" return ui.classList.contains('hidden') ? 'hidden' : 'visible'; })()");
			} catch { continue; }
			if (r === "visible" || r === "boot-done") { pillState = r; break; }
			if (r === "hidden") fail("phase B: loader pill hidden while boot overlay shown");
		}
		console.log("--- phase B: pill during boot overlay: " + pillState + " ---");
	}
	await waitBoot("phase B");
	const storedB = await evalInPage("window.__ikemenLoader.getStored()", true);
	if (!storedB || storedB.name !== "kfmcopy.zip") fail("phase B: stored record wrong: " + JSON.stringify(storedB));
	const rosterB = await rosterLines();
	console.log("phase B roster: " + JSON.stringify(rosterB));
	if (rosterB.length !== rosterA.length + 1 || !rosterB.includes("kfmcopy") ||
		JSON.stringify(rosterB.filter((c) => c !== "kfmcopy")) !== JSON.stringify(rosterA)) {
		fail("phase B: kfmcopy not merged into roster (baseline " + rosterA.length + "): " + JSON.stringify(rosterB));
	}
	const defB = await evalInPage("window.__ikemenLoader.readFile('/ikemen/chars/kfmcopy/kfmcopy.def')", true);
	if (!defB.includes("KFM Copy")) fail("phase B: kfmcopy.def not overlaid into memfs");
	await navToCharSelect("b-installed", false);

	// ---- Phase C: reset to default ----
	await evalInPage("window.__ikemenLoader.reset()", false); // reloads; don't await promise
	await waitReload("phase C");
	await waitBoot("phase C");
	const storedC = await evalInPage("window.__ikemenLoader.getStored()", true);
	if (storedC !== null && storedC !== undefined) fail("phase C: stored game not cleared: " + JSON.stringify(storedC));
	const rosterC = await rosterLines();
	console.log("phase C roster: " + JSON.stringify(rosterC));
	if (JSON.stringify(rosterC) !== JSON.stringify(rosterA)) {
		fail("phase C: roster not back to baseline: " + JSON.stringify(rosterC));
	}
	await navToCharSelect("c-reset", false);

	// ---- Phase D: corrupt stored zip must never brick boot ----
	// Plant a garbage record directly in IndexedDB, reload: boot must
	// succeed on the default game and the bad record must be auto-cleared.
	const planted = await evalInPage(
		"(async () => {" +
		"  const db = await new Promise((res, rej) => {" +
		"    const q = indexedDB.open('ikemen-loader', 1);" +
		"    q.onupgradeneeded = () => q.result.createObjectStore('zips');" +
		"    q.onsuccess = () => res(q.result); q.onerror = () => rej(q.error);" +
		"  });" +
		"  const buf = new Uint8Array(4096); buf[0] = 0x50; buf[1] = 0x4b; buf[2] = 3; buf[3] = 4;" + // 'PK\x03\x04' then garbage
		"  await new Promise((res, rej) => {" +
		"    const tx = db.transaction('zips', 'readwrite');" +
		"    tx.objectStore('zips').put({ name: 'corrupt.zip', size: buf.length, time: Date.now(), buf: buf.buffer }, 'user-game');" +
		"    tx.oncomplete = res; tx.onerror = () => rej(tx.error);" +
		"  });" +
		"  db.close(); return true;" +
		"})()", true);
	if (planted !== true) fail("phase D: could not plant corrupt record");
	await evalInPage("setTimeout(() => location.reload(), 100); true");
	await waitReload("phase D");
	await waitBoot("phase D"); // must not fail: overlay mount error is caught
	const storedD = await evalInPage("window.__ikemenLoader.getStored()", true);
	if (storedD !== null && storedD !== undefined) fail("phase D: corrupt record not auto-cleared: " + JSON.stringify(storedD));
	const rosterD = await rosterLines();
	if (JSON.stringify(rosterD) !== JSON.stringify(rosterA)) {
		fail("phase D: roster wrong after corrupt-zip recovery: " + JSON.stringify(rosterD));
	}
	console.log("--- phase D: corrupt stored zip auto-cleared, default game booted ---");

	// ---- Phase E: bare stage zip -> stages/ + [ExtraStages] ----
	const stagesA = await extraStageLines();
	const stageInstalled = await evalInPage(
		"(async () => {" +
		"  const r = await fetch('test/loader/fixture/stagecopy.zip');" +
		"  if (!r.ok) throw new Error('fixture fetch: ' + r.status);" +
		"  const b = await r.blob();" +
		"  const f = new File([b], 'stagecopy.zip', { type: 'application/zip' });" +
		"  return await window.__ikemenLoader.installBlob(f);" +
		"})()", true);
	if (stageInstalled !== true) fail("phase E: installBlob returned " + JSON.stringify(stageInstalled));
	await waitReload("phase E");
	await waitBoot("phase E");
	const storedE = await evalInPage("window.__ikemenLoader.getStored()", true);
	if (!storedE || storedE.name !== "stagecopy.zip") fail("phase E: stored record wrong: " + JSON.stringify(storedE));
	const rosterE = await rosterLines();
	if (JSON.stringify(rosterE) !== JSON.stringify(rosterA)) {
		fail("phase E: bare stage zip changed the character roster: " + JSON.stringify(rosterE));
	}
	const stagesE = await extraStageLines();
	console.log("phase E extrastages: " + JSON.stringify(stagesE));
	if (!stagesE.includes("stages/stagecopy.def") ||
		JSON.stringify(stagesE.filter((s) => s !== "stages/stagecopy.def")) !== JSON.stringify(stagesA)) {
		fail("phase E: stagecopy not merged into [ExtraStages] (baseline " + JSON.stringify(stagesA) + "): " + JSON.stringify(stagesE));
	}
	const defE = await evalInPage("window.__ikemenLoader.readFile('/ikemen/stages/stagecopy.def')", true);
	if (!defE.includes("spr = stagecopy.sff")) fail("phase E: stagecopy.def not mounted under stages/");
	console.log("--- phase E: bare stage zip mounted under stages/ and listed in [ExtraStages] ---");

	console.log("--- OK: baseline " + rosterA.length + " entries, merge-mode install adds kfmcopy, reset restores default, corrupt zip recovered, bare stage zip lands in stages/ ---");
	cleanup();
	process.exit(0);
} catch (err) {
	fail(err.stack || String(err));
}

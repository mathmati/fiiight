#!/usr/bin/env node
// Touch overlay harness: boots the real game in headless Chromium with touch
// emulation (mobile metrics), forces the virtual gamepad visible, then drives
// it with Input.dispatchTouchEvent and asserts -- via window.__ikemenTouch
// state() and a page-level keydown/keyup probe -- that the overlay dispatches
// synthetic KeyboardEvents with the right codes. Screenshots portrait and
// landscape into shots/ beside this script. Serves bin/Ikemen_GO-Web/
// (build it first with ./build/build.sh Web).
//
// Usage: node build/wasm/test/touch/run.mjs   (env BOOT_TIMEOUT=<s>, default 90)
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
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "ikemen-touch-"));
const shotsDir = path.join(testDir, "shots");
const bootTimeout = Number(process.env.BOOT_TIMEOUT || 90) * 1000;

fs.rmSync(shotsDir, { recursive: true, force: true });
fs.mkdirSync(shotsDir, { recursive: true });

const children = [];
function cleanup() {
	for (const c of children) { try { c.kill("SIGKILL"); } catch { /* gone */ } }
	try { fs.rmSync(scratch, { recursive: true, force: true }); } catch { /* busy */ }
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

let failures = 0;
function check(name, cond, detail) {
	if (cond) console.log("PASS: " + name);
	else { failures++; console.error("FAIL: " + name + (detail ? " -- " + detail : "")); }
}

async function startServer() {
	if (!fs.existsSync(path.join(outDir, "index.html"))) {
		console.error("FAIL: missing " + outDir + "/index.html -- run ./build/build.sh Web first (or set IKEMEN_WEB_OUT)");
		process.exit(1);
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

async function startChrome() {
	const proc = spawn(chrome, [
		"--headless=new", "--no-sandbox", "--disable-dev-shm-usage",
		"--use-angle=swiftshader", "--enable-unsafe-swiftshader",
		"--autoplay-policy=no-user-gesture-required",
		"--window-size=390,844",
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
		this.ws = ws; this.id = 0; this.pending = new Map(); this.listeners = [];
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

async function main() {
	const base = process.env.BASE_URL || (await startServer());
	const cdp = await CDP.connect(await startChrome());
	const { targetId } = await cdp.send("Target.createTarget", { url: "about:blank" });
	const { sessionId: s } = await cdp.send("Target.attachToTarget", { targetId, flatten: true });
	const ev = (expr) => cdp.send("Runtime.evaluate", { expression: expr, returnByValue: true }, s)
		.then((r) => {
			if (r.exceptionDetails) throw new Error("page eval failed: " + JSON.stringify(r.exceptionDetails));
			return r.result.value;
		});

	await cdp.send("Page.enable", {}, s);
	await cdp.send("Runtime.enable", {}, s);
	await cdp.send("Emulation.setDeviceMetricsOverride", { width: 390, height: 844, deviceScaleFactor: 2, mobile: true }, s);
	await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 }, s);

	// Page-level probe: record every keydown/keyup the window sees.
	await cdp.send("Page.addScriptToEvaluateOnNewDocument", {
		source: "window.__touchProbe=[];" +
			"window.addEventListener('keydown',e=>window.__touchProbe.push('down:'+e.code+':'+e.key));" +
			"window.addEventListener('keyup',e=>window.__touchProbe.push('up:'+e.code+':'+e.key));",
	}, s);

	cdp.on((m) => {
		if (m.sessionId === s && m.method === "Runtime.exceptionThrown") {
			const d = m.params.exceptionDetails;
			console.error("[pageerror] " + ((d.exception && d.exception.description) || d.text));
		}
	});

	await cdp.send("Page.navigate", { url: base + "/index.html" }, s);

	// ---- boot the real game, wait for the title screen -------------------
	console.log("--- booting engine (up to " + bootTimeout / 1000 + "s) ---");
	const t0 = Date.now();
	let booted = false;
	while (Date.now() - t0 < bootTimeout) {
		await sleep(1000);
		let st;
		try {
			st = await ev("JSON.stringify({hidden:document.getElementById('overlay').classList.contains('hidden')," +
				"log:(window.__ikemenBootLog||[]).slice(-5)})").then(JSON.parse);
		} catch { continue; }
		const bad = st.log.find((l) => /panic:|BOOT-ERROR|Engine exited/.test(l));
		if (bad) { console.error("FATAL during boot: " + bad); cleanup(); process.exit(1); }
		if (st.hidden) { booted = true; break; }
	}
	check("engine booted (loading overlay hidden)", booted);
	if (!booted) { cleanup(); process.exit(1); }
	console.log("--- boot took " + Math.round((Date.now() - t0) / 1000) + "s; waiting for title screen ---");
	await sleep(10000); // intro/logo -> title

	// ---- overlay present & visible ---------------------------------------
	await ev("window.__ikemenTouch.show()");
	const vis = await ev("JSON.stringify({api:!!window.__ikemenTouch,visible:window.__ikemenTouch.isVisible()," +
		"els:['#ikemen-touch','.itc-dpad','.itc-btns','.itc-toggle','.itc-meta'].map(q=>!!document.querySelector(q))})").then(JSON.parse);
	check("debug API + overlay elements exist", vis.api && vis.visible && vis.els.every(Boolean), JSON.stringify(vis));

	// Element geometry for tap targets.
	const geo = await ev(`JSON.stringify((()=>{
		const r=(q)=>{const b=document.querySelector(q).getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2,w:b.width,h:b.height,l:b.left,t:b.top,rt:b.right,bt:b.bottom};};
		return {dpad:r('.itc-dpad'),A:r('[data-code=KeyZ]'),C:r('[data-code=KeyC]'),Z:r('[data-code=KeyD]'),start:r('[data-code=Enter]'),vw:innerWidth,vh:innerHeight};
	})())`).then(JSON.parse);
	for (const [name, b] of Object.entries(geo)) {
		if (typeof b !== "object") continue;
		check(`${name} fully on-screen`, b.l >= 0 && b.t >= 0 && b.rt <= geo.vw && b.bt <= geo.vh,
			JSON.stringify(b) + ` viewport ${geo.vw}x${geo.vh}`);
	}

	const touch = (type, points) =>
		cdp.send("Input.dispatchTouchEvent", { type, touchPoints: points.map((p, i) => ({ x: p.x, y: p.y, id: p.id ?? i + 1 })) }, s);
	const state = () => ev("JSON.stringify(window.__ikemenTouch.state())").then(JSON.parse);
	const probe = () => ev("JSON.stringify(window.__touchProbe.splice(0))").then(JSON.parse);

	await probe(); // drain anything from boot

	// ---- 1. D-pad "down" zone ---------------------------------------------
	const dDown = { x: geo.dpad.x, y: geo.dpad.y + geo.dpad.h * 0.35, id: 1 };
	await touch("touchStart", [dDown]);
	await sleep(120);
	check("dpad down zone -> state [ArrowDown]", JSON.stringify(await state()) === '["ArrowDown"]', JSON.stringify(await state()));
	await cdp.send("Page.captureScreenshot", { format: "png" }, s)
		.then((r) => fs.writeFileSync(path.join(shotsDir, "portrait-dpad-held.png"), Buffer.from(r.data, "base64")));

	// ---- 2. slide to down-right diagonal, then pure right ------------------
	await touch("touchMove", [{ x: geo.dpad.x + geo.dpad.w * 0.28, y: geo.dpad.y + geo.dpad.h * 0.28, id: 1 }]);
	await sleep(120);
	let st = await state();
	check("slide -> diagonal [ArrowDown,ArrowRight]", st.sort().join() === "ArrowDown,ArrowRight", JSON.stringify(st));
	await touch("touchMove", [{ x: geo.dpad.x + geo.dpad.w * 0.4, y: geo.dpad.y, id: 1 }]);
	await sleep(120);
	st = await state();
	check("slide -> [ArrowRight] only", st.join() === "ArrowRight", JSON.stringify(st));
	await touch("touchEnd", []);
	await sleep(120);
	check("release -> state empty", (await state()).length === 0);
	let pr = await probe();
	check("probe saw ArrowDown keydown+keyup", pr.includes("down:ArrowDown:ArrowDown") && pr.includes("up:ArrowDown:ArrowDown"), JSON.stringify(pr));
	check("probe saw ArrowRight keydown+keyup", pr.includes("down:ArrowRight:ArrowRight") && pr.includes("up:ArrowRight:ArrowRight"), JSON.stringify(pr));

	// ---- 3. A button tap (code KeyZ, key "z") -------------------------------
	await touch("touchStart", [{ x: geo.A.x, y: geo.A.y, id: 1 }]);
	await sleep(120);
	check("A button -> state [KeyZ]", JSON.stringify(await state()) === '["KeyZ"]', JSON.stringify(await state()));
	await touch("touchEnd", []);
	await sleep(120);
	pr = await probe();
	check("probe saw KeyZ down/up with key=z", pr.includes("down:KeyZ:z") && pr.includes("up:KeyZ:z"), JSON.stringify(pr));

	// ---- 4. multi-touch: hold dpad-down + A + C simultaneously --------------
	const pA = { x: geo.A.x, y: geo.A.y, id: 2 };
	const pC = { x: geo.C.x, y: geo.C.y, id: 3 };
	await touch("touchStart", [dDown]);
	await touch("touchStart", [dDown, pA]);
	await touch("touchStart", [dDown, pA, pC]);
	await sleep(150);
	st = (await state()).sort();
	check("multi-touch -> [ArrowDown,KeyC,KeyZ]", st.join() === "ArrowDown,KeyC,KeyZ", JSON.stringify(st));
	// CDP touchEnd semantics: touchPoints lists the points being RELEASED.
	await touch("touchEnd", [pA]); // lift A only
	await sleep(120);
	st = (await state()).sort();
	check("partial release keeps others held", st.join() === "ArrowDown,KeyC", JSON.stringify(st));
	await touch("touchEnd", [dDown, pC]);
	await sleep(120);
	check("full release -> empty", (await state()).length === 0);
	await probe();

	// ---- 5. toggle hides pad and releases keys ------------------------------
	await touch("touchStart", [{ x: geo.A.x, y: geo.A.y, id: 1 }]);
	await sleep(80);
	await ev("window.__ikemenTouch.hide()");
	await sleep(80);
	check("hide() releases held keys", (await state()).length === 0);
	check("hide() -> not visible", (await ev("window.__ikemenTouch.isVisible()")) === false);
	pr = await probe();
	check("probe saw KeyZ released on hide", pr.includes("up:KeyZ:z"), JSON.stringify(pr));
	await touch("touchEnd", []);
	await ev("window.__ikemenTouch.show()");
	check("show() -> visible again", (await ev("window.__ikemenTouch.isVisible()")) === true);

	// ---- 6. START press (Enter) so title screen reacts, then screenshots ----
	await touch("touchStart", [{ x: geo.start.x, y: geo.start.y, id: 1 }]);
	await sleep(150);
	check("START -> state [Enter]", JSON.stringify(await state()) === '["Enter"]', JSON.stringify(await state()));
	await touch("touchEnd", []);
	await sleep(1500);

	await cdp.send("Page.captureScreenshot", { format: "png" }, s)
		.then((r) => fs.writeFileSync(path.join(shotsDir, "portrait.png"), Buffer.from(r.data, "base64")));
	console.log("--- screenshot portrait.png ---");

	await cdp.send("Emulation.setDeviceMetricsOverride", { width: 844, height: 390, deviceScaleFactor: 2, mobile: true }, s);
	await sleep(1200);
	await cdp.send("Page.captureScreenshot", { format: "png" }, s)
		.then((r) => fs.writeFileSync(path.join(shotsDir, "landscape.png"), Buffer.from(r.data, "base64")));
	console.log("--- screenshot landscape.png ---");

	// Landscape bounds sanity.
	const geo2 = await ev(`JSON.stringify((()=>{
		const r=(q)=>{const b=document.querySelector(q).getBoundingClientRect();return {l:b.left,t:b.top,rt:b.right,bt:b.bottom};};
		return {dpad:r('.itc-dpad'),A:r('[data-code=KeyZ]'),Z:r('[data-code=KeyD]'),toggle:r('.itc-toggle'),vw:innerWidth,vh:innerHeight};
	})())`).then(JSON.parse);
	for (const [name, b] of Object.entries(geo2)) {
		if (typeof b !== "object") continue;
		check(`landscape ${name} on-screen`, b.l >= 0 && b.t >= 0 && b.rt <= geo2.vw && b.bt <= geo2.vh, JSON.stringify(b));
	}

	console.log(failures === 0 ? "--- ALL PASS ---" : `--- ${failures} FAILURE(S) ---`);
	cleanup();
	process.exit(failures === 0 ? 0 : 1);
}

main().catch((err) => { console.error("FAIL: " + (err.stack || err)); cleanup(); process.exit(1); });

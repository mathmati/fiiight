// loader.js -- "Load your own game": pick or drop a zip of MUGEN/Ikemen
// content (chars/, stages/, a full game with data/select.def, ...), persist
// it in IndexedDB, and overlay it onto /ikemen at every boot, after the base
// content.zip mount (user files win on path collision).
//
// Install = validate + store in IndexedDB + location.reload(); main.js calls
// window.__ikemenLoader.applyStoredOverlay(setStatus, setProgress) between
// the content.zip mount and go.run(). Zips without their own data/select.def
// are merged into the base roster by rewriting select.def in the memfs.
// A bad stored zip must never brick the site: the overlay is wrapped in
// try/catch and auto-clears the stored record on failure.
"use strict";
(() => {
	const DB_NAME = "ikemen-loader";
	const STORE = "zips";
	const KEY = "user-game";
	const MAX_BYTES = 500 * 1024 * 1024; // 500 MB cap

	// Folders that legitimately sit at the root of a content zip. A lone root
	// folder with one of these names is content, not a wrapper dir to strip.
	const CONTENT_ROOTS = new Set(["chars", "stages", "data", "font", "sound", "music", "save", "external", "modules"]);

	// ---- IndexedDB (single record, replaced on each install) ---------------
	function idbOpen() {
		return new Promise((resolve, reject) => {
			const req = indexedDB.open(DB_NAME, 1);
			req.onupgradeneeded = () => req.result.createObjectStore(STORE);
			req.onsuccess = () => resolve(req.result);
			req.onerror = () => reject(req.error);
		});
	}
	async function idbReq(mode, fn) {
		const db = await idbOpen();
		try {
			return await new Promise((resolve, reject) => {
				const tx = db.transaction(STORE, mode);
				const req = fn(tx.objectStore(STORE));
				tx.oncomplete = () => resolve(req.result);
				tx.onerror = () => reject(tx.error || new Error("IndexedDB transaction failed"));
				tx.onabort = () => reject(tx.error || new Error("IndexedDB transaction aborted"));
			});
		} finally {
			db.close();
		}
	}
	const idbGet = () => idbReq("readonly", (s) => s.get(KEY));
	const idbPut = (rec) => idbReq("readwrite", (s) => s.put(rec, KEY));
	const idbClear = () => idbReq("readwrite", (s) => s.delete(KEY));

	// ---- memfs helpers (promisified Node-callback API from fs-shim) --------
	function fsCall(name, ...args) {
		return new Promise((resolve, reject) => {
			globalThis.fs[name](...args, (err, res) => err ? reject(err) : resolve(res));
		});
	}
	async function readFileText(path) {
		const fd = await fsCall("open", path, 0, 0);
		try {
			const st = await fsCall("fstat", fd);
			const buf = new Uint8Array(st.size);
			let off = 0;
			while (off < st.size) {
				const n = await fsCall("read", fd, buf, off, st.size - off, off);
				if (n <= 0) break;
				off += n;
			}
			return new TextDecoder().decode(buf.subarray(0, off));
		} finally {
			await fsCall("close", fd);
		}
	}
	async function writeFileText(path, text) {
		const data = new TextEncoder().encode(text);
		// O_WRONLY | O_CREAT | O_TRUNC
		const fd = await fsCall("open", path, 1 | 64 | 512, 0o644);
		try {
			await fsCall("write", fd, data, 0, data.length, 0);
		} finally {
			await fsCall("close", fd);
		}
	}

	// ---- zip enumeration (reuses fs-shim's central-directory parser) -------
	// mapName -> null means "list, don't extract": nothing is inflated or
	// written, we just collect the entry names.
	async function listZipEntries(bytes) {
		const names = [];
		await globalThis.mountZipBuffer(bytes, "/", {
			mapName: (n) => { names.push(n); return null; },
		});
		return names;
	}

	// ---- layout analysis ----------------------------------------------------
	function cleanName(n) {
		n = String(n).replace(/\\/g, "/").replace(/^\/+/, "");
		if (!n || n.startsWith("__MACOSX/")) return "";
		const base = n.split("/").pop();
		if (base === ".DS_Store" || base === "Thumbs.db") return "";
		return n;
	}

	// Returns { prefix, hasSelect, charLines, stageLines, fileCount }.
	// prefix is the single wrapper folder ("MyGame/") to strip, or "".
	function analyzeZip(names) {
		const entries = names.map(cleanName).filter(Boolean);
		if (entries.length === 0) throw new Error("zip contains no usable files");

		// Wrapper detection: everything under one root folder that is not
		// itself a known content dir -> strip it.
		let prefix = "";
		const firsts = new Set(entries.map((n) => n.split("/")[0]));
		if (firsts.size === 1) {
			const f = firsts.values().next().value;
			if (!CONTENT_ROOTS.has(f.toLowerCase()) && entries.some((n) => n.startsWith(f + "/"))) {
				prefix = f + "/";
			}
		}
		const stripped = entries
			.map((n) => (prefix && n.startsWith(prefix)) ? n.slice(prefix.length) : n)
			.filter(Boolean);

		const hasSelect = stripped.some((n) => n.toLowerCase() === "data/select.def");

		// Merge-mode candidates: chars/<dir>/<file>.def and stages/<file>.def.
		const charDefs = new Map(); // dirname -> [def filenames]
		const stageLines = [];
		for (const n of stripped) {
			const parts = n.split("/");
			const l = n.toLowerCase();
			if (parts.length === 3 && l.startsWith("chars/") && l.endsWith(".def")) {
				if (!charDefs.has(parts[1])) charDefs.set(parts[1], []);
				charDefs.get(parts[1]).push(parts[2]);
			} else if (parts.length === 2 && l.startsWith("stages/") && l.endsWith(".def")) {
				stageLines.push("stages/" + parts[1]);
			}
		}
		const charLines = [];
		for (const [dir, defs] of charDefs) {
			// Prefer the .def named after the dir (the engine resolves the
			// bare dir name to it); otherwise reference the first .def.
			const own = defs.find((d) => d.toLowerCase() === dir.toLowerCase() + ".def");
			charLines.push(own ? dir : dir + "/" + defs[0]);
		}

		return {
			prefix,
			hasSelect,
			charLines,
			stageLines,
			fileCount: stripped.filter((n) => !n.endsWith("/")).length,
		};
	}

	// ---- select.def merge ---------------------------------------------------
	// Appends newLines at the end of the named section (after its last
	// non-comment entry), skipping lines whose first field already exists.
	function mergeSelectDef(text, charLines, stageLines) {
		const nl = text.includes("\r\n") ? "\r\n" : "\n";
		const lines = text.split(/\r?\n/);

		function appendToSection(section, newLines) {
			if (!newLines.length) return;
			let inSec = false;
			let insertAt = -1;
			let lastContent = -1;
			const existing = new Set();
			for (let i = 0; i < lines.length; i++) {
				const t = lines[i].trim();
				if (t.startsWith("[")) {
					if (inSec) { insertAt = i; break; }
					if (t.toLowerCase().startsWith("[" + section)) { inSec = true; insertAt = lines.length; }
					continue;
				}
				if (inSec && t && !t.startsWith(";")) {
					lastContent = i;
					existing.add(t.split(",")[0].trim().toLowerCase());
				}
			}
			if (!inSec) return; // section missing in this select.def; leave as-is
			const add = newLines.filter((l) => !existing.has(l.split(",")[0].trim().toLowerCase()));
			if (!add.length) return;
			const pos = (lastContent >= 0 && lastContent < insertAt) ? lastContent + 1 : insertAt;
			lines.splice(pos, 0, ...add);
		}

		appendToSection("characters", charLines);
		appendToSection("extrastages", stageLines);
		return lines.join(nl);
	}

	// ---- boot-time overlay (called by main.js before go.run) ----------------
	async function applyStoredOverlay(setStatus, setProgress) {
		let rec = null;
		try {
			rec = await idbGet();
		} catch (e) {
			console.warn("loader: IndexedDB unavailable, skipping user game", e);
			return;
		}
		if (!rec || !rec.buf) return;
		try {
			const bytes = new Uint8Array(rec.buf);
			const info = analyzeZip(await listZipEntries(bytes));
			if (setStatus) setStatus("Installing " + rec.name + " — " + info.fileCount + " files…");
			await globalThis.mountZipBuffer(bytes, "/ikemen", {
				mapName: (n) => {
					n = cleanName(n);
					if (!n) return null;
					return (info.prefix && n.startsWith(info.prefix)) ? n.slice(info.prefix.length) : n;
				},
				onProgress: (done, total) => {
					if (setProgress && total > 0) setProgress(done / total);
				},
			});
			if (!info.hasSelect && (info.charLines.length || info.stageLines.length)) {
				const sel = await readFileText("/ikemen/data/select.def");
				await writeFileText("/ikemen/data/select.def", mergeSelectDef(sel, info.charLines, info.stageLines));
			}
			if (setStatus) setStatus("Loaded " + rec.name + " — starting engine…");
			if (setProgress) setProgress(1);
			console.log("loader: overlaid " + rec.name + " (" + info.fileCount + " files" +
				(info.prefix ? ", stripped wrapper '" + info.prefix + "'" : "") +
				(info.hasSelect ? ", full game" : ", merged " + info.charLines.length + " chars / " + info.stageLines.length + " stages") + ")");
		} catch (e) {
			console.error("loader: stored game failed to mount; removing it", e);
			try { await idbClear(); } catch (e2) { /* ignore */ }
			showStatus("Stored game \"" + rec.name + "\" failed to load and was removed. Booting the default game.");
			if (setStatus) setStatus("Custom game failed to load — using default…");
		}
		refreshStoredUI();
	}

	// ---- install / reset -----------------------------------------------------
	function fmtMB(n) { return (n / (1024 * 1024)).toFixed(1) + " MB"; }

	async function installBlob(blob) {
		const name = (blob && blob.name) || "game.zip";
		try {
			if (!blob || typeof blob.arrayBuffer !== "function") throw new Error("not a file");
			if (blob.size > MAX_BYTES) {
				showStatus("\"" + name + "\" is " + fmtMB(blob.size) + " — the limit is 500 MB.");
				return false;
			}
			const buf = await blob.arrayBuffer();
			const bytes = new Uint8Array(buf);
			if (bytes.length < 22 || bytes[0] !== 0x50 || bytes[1] !== 0x4b) {
				showStatus("\"" + name + "\" doesn't look like a zip file. Drop a .zip of MUGEN/Ikemen content.");
				return false;
			}
			// Full central-directory parse up front: rejects corrupt zips
			// before anything is persisted.
			const info = analyzeZip(await listZipEntries(bytes));
			showStatus("Installing " + name + " — " + info.fileCount + " files…", true);
			await idbPut({ name, size: bytes.length, time: Date.now(), buf });
			showStatus("Installed " + name + " (" + info.fileCount + " files) — restarting…", true);
			setTimeout(() => location.reload(), 600);
			return true;
		} catch (e) {
			console.error("loader: install failed", e);
			showStatus("Couldn't install \"" + name + "\": " + (e && e.message ? e.message : e));
			return false;
		}
	}

	async function reset() {
		try { await idbClear(); } catch (e) { console.warn("loader: reset failed", e); }
		location.reload();
	}

	async function getStored() {
		const rec = await idbGet();
		return rec ? { name: rec.name, size: rec.size, time: rec.time } : null;
	}

	// ---- UI -------------------------------------------------------------------
	// Styling matches index.html's overlay: black bg, system-ui, #e33 accent.
	const style = document.createElement("style");
	style.textContent = [
		"#loader-ui { position: fixed; right: 12px; bottom: 26px; z-index: 20;",
		"  display: flex; flex-direction: column; align-items: flex-end; gap: 6px;",
		"  font: 12px/1.4 system-ui, sans-serif; }",
		"#loader-ui.hidden { display: none; }",
		"#loader-status { color: #999; max-width: min(60vw, 340px); text-align: right; }",
		"#loader-status:empty { display: none; }",
		"#loader-pill { background: #181818; color: #ddd; border: 1px solid #333;",
		"  border-radius: 999px; padding: 6px 14px; cursor: pointer;",
		"  font: 12px/1.2 system-ui, sans-serif; }",
		"#loader-pill:hover { border-color: #e33; color: #fff; }",
		"#loader-reset { color: #777; text-decoration: underline; cursor: pointer; display: none; }",
		"#loader-reset:hover { color: #f66; }",
		"#loader-drop { position: fixed; inset: 0; z-index: 30; display: none;",
		"  align-items: center; justify-content: center;",
		"  background: rgba(0, 0, 0, 0.72); pointer-events: none; }",
		"#loader-drop.active { display: flex; }",
		"#loader-drop > div { color: #fff; font: 20px/1.4 system-ui, sans-serif;",
		"  border: 3px dashed #e33; border-radius: 12px; padding: 40px 60px;",
		"  background: rgba(24, 8, 8, 0.6); }",
	].join("\n");
	document.head.appendChild(style);

	const ui = document.createElement("div");
	ui.id = "loader-ui";
	document.body.appendChild(ui);
	const statusEl = document.createElement("div");
	statusEl.id = "loader-status";
	const pillEl = document.createElement("button");
	pillEl.id = "loader-pill";
	pillEl.type = "button";
	pillEl.textContent = "Load game…";
	pillEl.title = "Install a zip of MUGEN/Ikemen content (F8)";
	const resetEl = document.createElement("a");
	resetEl.id = "loader-reset";
	resetEl.textContent = "Reset to default game";
	ui.append(statusEl, pillEl, resetEl);

	const dropEl = document.createElement("div");
	dropEl.id = "loader-drop";
	const dropMsg = document.createElement("div");
	dropMsg.textContent = "Drop game zip to install";
	dropEl.appendChild(dropMsg);
	document.body.appendChild(dropEl);

	const inputEl = document.createElement("input");
	inputEl.type = "file";
	inputEl.accept = ".zip,application/zip";
	inputEl.style.display = "none";
	document.body.appendChild(inputEl);

	// Visible while the boot overlay is up; afterwards toggled with F8 (or
	// forced visible while a status message is showing).
	const overlayEl = document.getElementById("overlay");
	let forcedVisible = false;
	let statusTimer = 0;
	function syncVisibility() {
		const overlayShown = overlayEl && !overlayEl.classList.contains("hidden");
		ui.classList.toggle("hidden", !(overlayShown || forcedVisible || statusEl.textContent !== ""));
	}
	if (overlayEl) {
		new MutationObserver(syncVisibility).observe(overlayEl, { attributes: true, attributeFilter: ["class"] });
	}
	window.addEventListener("keydown", (e) => {
		if (e.code === "F8") {
			e.preventDefault();
			forcedVisible = !forcedVisible;
			syncVisibility();
		}
	});

	function showStatus(text, sticky) {
		statusEl.textContent = text;
		clearTimeout(statusTimer);
		if (!sticky) {
			statusTimer = setTimeout(() => { statusEl.textContent = ""; syncVisibility(); }, 8000);
		}
		syncVisibility();
	}

	async function refreshStoredUI() {
		let rec = null;
		try { rec = await getStored(); } catch (e) { /* no IndexedDB */ }
		resetEl.style.display = rec ? "inline" : "none";
		pillEl.textContent = rec ? "Load game… (current: " + rec.name + ")" : "Load game…";
	}

	pillEl.addEventListener("click", () => inputEl.click());
	inputEl.addEventListener("change", () => {
		if (inputEl.files && inputEl.files[0]) installBlob(inputEl.files[0]);
		inputEl.value = "";
	});
	resetEl.addEventListener("click", () => {
		showStatus("Removing custom game — restarting…", true);
		reset();
	});

	// Whole-page drag & drop.
	function dragHasFiles(e) {
		return e.dataTransfer && Array.from(e.dataTransfer.types || []).includes("Files");
	}
	let dragDepth = 0;
	window.addEventListener("dragenter", (e) => {
		if (!dragHasFiles(e)) return;
		e.preventDefault();
		dragDepth++;
		dropEl.classList.add("active");
	});
	window.addEventListener("dragover", (e) => {
		if (dragHasFiles(e)) e.preventDefault();
	});
	window.addEventListener("dragleave", () => {
		if (--dragDepth <= 0) {
			dragDepth = 0;
			dropEl.classList.remove("active");
		}
	});
	window.addEventListener("drop", (e) => {
		e.preventDefault();
		dragDepth = 0;
		dropEl.classList.remove("active");
		const f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0];
		if (f) installBlob(f);
	});

	refreshStoredUI();
	syncVisibility();

	window.__ikemenLoader = {
		installBlob,
		applyStoredOverlay,
		reset,
		getStored,
		// debug/test helper
		readFile: readFileText,
	};
})();

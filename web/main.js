// main.js -- boot sequence: mount content.zip into the in-memory fs, chdir,
// then instantiate and run ikemen.wasm.
"use strict";

// Captures every stdout/stderr line emitted through fs-shim (see emitLine),
// used by test automation and for surfacing panics.
window.__ikemenBootLog = [];

(() => {
	const overlay = document.getElementById("overlay");
	const bar = document.getElementById("progress-bar");
	const status = document.getElementById("status");
	const errorBox = document.getElementById("error");

	function setStatus(text) { status.textContent = text; }
	function setProgress(frac) { bar.style.width = Math.round(frac * 100) + "%"; }
	function showError(text) {
		overlay.classList.remove("hidden");
		errorBox.style.display = "block";
		errorBox.textContent = text;
		setStatus("Failed to start");
	}

	function fmtMB(n) { return (n / (1024 * 1024)).toFixed(1) + " MB"; }

	async function boot() {
		setStatus("Downloading content…");
		await mountZip("content.zip", "/ikemen", (loaded, total) => {
			if (total > 0) {
				setProgress(loaded / total);
				setStatus("Downloading content… " + fmtMB(loaded) + " / " + fmtMB(total));
			} else {
				setStatus("Downloading content… " + fmtMB(loaded));
			}
		});
		setProgress(1);
		globalThis.process.chdir("/ikemen");

		setStatus("Loading engine…");
		const go = new Go();
		go.argv = ["ikemen"];
		let result;
		try {
			result = await WebAssembly.instantiateStreaming(fetch("ikemen.wasm"), go.importObject);
		} catch (e) {
			// Fallback for servers without application/wasm MIME.
			const resp = await fetch("ikemen.wasm");
			if (!resp.ok) throw new Error("ikemen.wasm: HTTP " + resp.status);
			result = await WebAssembly.instantiate(await resp.arrayBuffer(), go.importObject);
		}

		overlay.classList.add("hidden");
		let exitCode = 0;
		go.exit = (code) => { exitCode = code; };
		await go.run(result.instance); // resolves on normal exit, throws on panic/trap
		const msg = "Engine exited with code " + exitCode;
		window.__ikemenBootLog.push(msg);
		if (exitCode !== 0) showError(msg + "\n\nLast output:\n" + window.__ikemenBootLog.slice(-20).join("\n"));
		else setStatus(msg);
	}

	// Autoplay policy: resume WebAudio on the first user gesture. The Go side
	// (audio_js.go) registers window.__ikemenResumeAudio once audio is up.
	const resumeAudio = () => { if (window.__ikemenResumeAudio) window.__ikemenResumeAudio(); };
	window.addEventListener("pointerdown", resumeAudio);
	window.addEventListener("keydown", resumeAudio);

	boot().catch((err) => {
		const detail = (err && err.stack) ? err.stack : String(err);
		window.__ikemenBootLog.push("BOOT-ERROR: " + detail);
		console.error(err);
		showError(detail + "\n\nLast output:\n" + window.__ikemenBootLog.slice(-20).join("\n"));
	});
})();

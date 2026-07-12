// Test driver page logic: mounts the test zip with the real fs-shim, runs
// fstest.wasm, and flags completion for the CDP runner (web/test/run.mjs).
"use strict";
window.__ikemenBootLog = [];
window.__testDone = false;

(async () => {
	try {
		await mountZip("/test/content.zip", "/ikemen", () => {});
		globalThis.process.chdir("/ikemen");
		const go = new Go();
		const { instance } = await WebAssembly.instantiateStreaming(
			fetch("/test/fstest.wasm"), go.importObject);
		await go.run(instance);
	} catch (err) {
		window.__ikemenBootLog.push("HARNESS-ERROR: " + ((err && err.stack) || err));
	} finally {
		window.__testDone = true;
		document.getElementById("out").textContent = window.__ikemenBootLog.join("\n");
	}
})();

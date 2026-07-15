// fullscreen.js -- mobile fullscreen + landscape lock.
//
// Android (Chrome/Firefox): the Fullscreen API needs a user gesture, so the
// first tap after load enters fullscreen and locks landscape.
// iPhone Safari has no element Fullscreen API; those users get a chrome-less
// experience by installing to the Home Screen (see manifest + apple-* meta in
// index.html), which launches standalone. When already standalone we skip the
// fullscreen request and just keep the orientation locked.
"use strict";
(() => {
	const coarse = (window.matchMedia && window.matchMedia("(pointer: coarse)").matches) ||
		"ontouchstart" in window || (navigator.maxTouchPoints || 0) > 0;
	if (!coarse) return; // desktop: leave windowing to the user

	const standalone = (window.matchMedia && window.matchMedia("(display-mode: standalone)").matches) ||
		(window.matchMedia && window.matchMedia("(display-mode: fullscreen)").matches) ||
		window.navigator.standalone === true;

	function lockLandscape() {
		try {
			const o = screen.orientation;
			if (o && o.lock) o.lock("landscape").catch(() => { /* not allowed here */ });
		} catch (e) { /* unsupported */ }
	}

	async function goFullscreen() {
		const el = document.documentElement;
		try {
			if (!document.fullscreenElement && el.requestFullscreen) {
				await el.requestFullscreen({ navigationUI: "hide" });
			}
		} catch (e) { /* iOS Safari / denied — home-screen install is the fallback */ }
		lockLandscape();
	}

	window.addEventListener("orientationchange", lockLandscape);

	if (standalone) {
		lockLandscape();
		return;
	}

	// Enter fullscreen on the first gesture (also the gesture main.js uses to
	// resume audio, so it costs the player nothing extra).
	const once = () => {
		window.removeEventListener("touchend", once);
		window.removeEventListener("pointerdown", once);
		goFullscreen();
	};
	window.addEventListener("touchend", once, { passive: true });
	window.addEventListener("pointerdown", once);
})();

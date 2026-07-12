// touch.js -- virtual gamepad overlay for touch devices.
//
// The engine (engine/src/system_js.go) registers plain "keydown"/"keyup"
// listeners on window and reads only KeyboardEvent.code (plus .key for text
// entry and the modifier booleans); it never checks isTrusted. So this file
// simulates a gamepad by dispatching synthetic KeyboardEvents to window with
// the codes bound in the default P1 config (defaultConfig.ini [Keys_P1]):
//
//   up/down/left/right = arrows          start = RETURN (Enter)
//   A=z  B=x  C=c  X=a  Y=s  Z=d         menu/back = Escape
//
// Layout: 8-way D-pad bottom-left (radial hit zones, slide to change
// direction), six action buttons bottom-right in two arcs (XYZ over ABC),
// START/ESC pills top-center, and a gamepad toggle top-right. Multi-touch:
// every touch identifier is tracked independently, so D-pad + several
// buttons can be held at once.
//
// Debug/test hooks: window.__ikemenTouch.{show,hide,toggle,state,isVisible}.
"use strict";

(() => {
	// KeyboardEvent.code -> .key for everything we emit.
	const KEY_OF = {
		ArrowUp: "ArrowUp", ArrowDown: "ArrowDown",
		ArrowLeft: "ArrowLeft", ArrowRight: "ArrowRight",
		KeyZ: "z", KeyX: "x", KeyC: "c",
		KeyA: "a", KeyS: "s", KeyD: "d",
		Enter: "Enter", Escape: "Escape",
	};

	// Action buttons: label -> code. Two arcs of three around the corner
	// thumb; cx/cy are button-centre offsets from the bottom-right anchor in
	// units of --tu (button diameter).
	const BUTTONS = [
		{ label: "A", code: "KeyZ", cx: 2.75, cy: 0.55 },
		{ label: "B", code: "KeyX", cx: 1.70, cy: 0.80 },
		{ label: "C", code: "KeyC", cx: 0.65, cy: 1.05 },
		{ label: "X", code: "KeyA", cx: 2.90, cy: 1.60 },
		{ label: "Y", code: "KeyS", cx: 1.85, cy: 1.85 },
		{ label: "Z", code: "KeyD", cx: 0.80, cy: 2.10 },
	];

	// 8 direction sectors, 45 deg each, counter-clockwise from +x (right).
	const DIR_U = 1, DIR_D = 2, DIR_L = 4, DIR_R = 8;
	const SECTORS = [
		DIR_R, DIR_R | DIR_U, DIR_U, DIR_U | DIR_L,
		DIR_L, DIR_L | DIR_D, DIR_D, DIR_D | DIR_R,
	];
	const DIR_CODE = { [DIR_U]: "ArrowUp", [DIR_D]: "ArrowDown", [DIR_L]: "ArrowLeft", [DIR_R]: "ArrowRight" };

	const HIDDEN_LS_KEY = "ikemen.touch.hidden";
	const isTouchDevice = () =>
		(window.matchMedia && window.matchMedia("(pointer: coarse)").matches) ||
		"ontouchstart" in window ||
		(navigator.maxTouchPoints || 0) > 0;

	// ---- synthetic key dispatch (refcounted so overlapping sources never
	// emit unbalanced keydown/keyup pairs) --------------------------------
	const held = new Map(); // code -> refcount
	function fire(type, code) {
		window.dispatchEvent(new KeyboardEvent(type, {
			code, key: KEY_OF[code] || code,
			bubbles: true, cancelable: true, composed: true,
		}));
	}
	function keyDown(code) {
		const n = (held.get(code) || 0) + 1;
		held.set(code, n);
		if (n === 1) fire("keydown", code);
	}
	function keyUp(code) {
		const n = (held.get(code) || 0) - 1;
		if (n <= 0) {
			if (held.delete(code)) fire("keyup", code);
		} else {
			held.set(code, n);
		}
	}
	function releaseAll() {
		for (const code of [...held.keys()]) { held.delete(code); fire("keyup", code); }
	}

	// Autoplay: main.js already resumes audio on window pointerdown/keydown,
	// but tickle the hook directly on first touch as belt-and-braces.
	let audioTickled = false;
	function tickleAudio() {
		if (audioTickled) return;
		audioTickled = true;
		if (typeof window.__ikemenResumeAudio === "function") {
			try { window.__ikemenResumeAudio(); } catch { /* not up yet */ }
		}
	}

	// ---- DOM ------------------------------------------------------------
	let root = null, dpadEl = null, arrowEls = null;

	const CSS = `
#ikemen-touch {
	position: fixed; inset: 0; z-index: 40;
	pointer-events: none;
	user-select: none; -webkit-user-select: none;
	-webkit-touch-callout: none; -webkit-tap-highlight-color: transparent;
	font-family: system-ui, sans-serif;
	--tu: clamp(44px, 12vmin, 62px);   /* action button diameter */
	--dp: clamp(118px, 36vmin, 180px); /* d-pad diameter */
	--sal: env(safe-area-inset-left, 0px);
	--sar: env(safe-area-inset-right, 0px);
	--sat: env(safe-area-inset-top, 0px);
	--sab: env(safe-area-inset-bottom, 0px);
}
#ikemen-touch * { touch-action: none; box-sizing: border-box; }
#ikemen-touch.itc-hidden .itc-pad { display: none; }

.itc-toggle {
	position: absolute;
	top: calc(var(--sat) + 10px); right: calc(var(--sar) + 10px);
	width: 40px; height: 34px;
	display: flex; align-items: center; justify-content: center;
	font-size: 19px; line-height: 1;
	background: rgba(16, 16, 22, 0.55);
	border: 1px solid rgba(255, 255, 255, 0.18);
	border-radius: 10px;
	pointer-events: auto; cursor: pointer;
	opacity: 0.85;
}
#ikemen-touch.itc-hidden .itc-toggle { opacity: 0.5; filter: grayscale(1); }

.itc-dpad {
	position: absolute;
	left: calc(var(--sal) + 14px); bottom: calc(var(--sab) + 16px);
	width: var(--dp); height: var(--dp);
	border-radius: 50%;
	background: radial-gradient(circle at 50% 45%, rgba(40, 40, 52, 0.40), rgba(14, 14, 20, 0.42));
	border: 1px solid rgba(255, 255, 255, 0.16);
	pointer-events: auto;
}
.itc-dpad::after {
	content: ""; position: absolute; inset: 35%;
	border-radius: 50%;
	border: 1px solid rgba(255, 255, 255, 0.12);
}
.itc-ar {
	position: absolute;
	font-size: calc(var(--dp) * 0.15);
	color: rgba(255, 255, 255, 0.42);
	pointer-events: none;
	transition: color 60ms linear;
}
.itc-ar.itc-on { color: #fff; text-shadow: 0 0 10px rgba(255, 90, 90, 0.95); }

.itc-btns {
	position: absolute;
	right: calc(var(--sar) + 12px); bottom: calc(var(--sab) + 14px);
	width: calc(var(--tu) * 3.45); height: calc(var(--tu) * 2.65);
	pointer-events: none;
}
.itc-btn {
	position: absolute;
	width: var(--tu); height: var(--tu);
	display: flex; align-items: center; justify-content: center;
	border-radius: 50%;
	background: rgba(16, 16, 24, 0.44);
	border: 1px solid rgba(255, 255, 255, 0.22);
	color: rgba(255, 255, 255, 0.88);
	font-size: calc(var(--tu) * 0.34); font-weight: 600;
	pointer-events: auto;
	transition: transform 50ms linear, background-color 50ms linear;
}
.itc-btn.itc-on {
	background: rgba(226, 51, 51, 0.55);
	border-color: rgba(255, 255, 255, 0.65);
	transform: scale(0.92);
}

.itc-meta {
	position: absolute;
	top: calc(var(--sat) + 8px); left: 50%;
	transform: translateX(-50%);
	display: flex; gap: 14px;
	pointer-events: none;
}
.itc-pill {
	padding: 6px 16px;
	border-radius: 999px;
	background: rgba(16, 16, 24, 0.5);
	border: 1px solid rgba(255, 255, 255, 0.2);
	color: rgba(255, 255, 255, 0.8);
	font-size: 11px; font-weight: 600; letter-spacing: 0.12em;
	pointer-events: auto;
}
.itc-pill.itc-on {
	background: rgba(226, 51, 51, 0.55);
	border-color: rgba(255, 255, 255, 0.65);
}

/* While the pad is active, stop double-tap zoom / scroll / selection on the
   game area itself. */
html.itc-touch-active, html.itc-touch-active body,
html.itc-touch-active #stage, html.itc-touch-active #ikemen-canvas {
	touch-action: none;
	user-select: none; -webkit-user-select: none;
	overscroll-behavior: none;
}
`;

	function el(tag, className, text) {
		const e = document.createElement(tag);
		if (className) e.className = className;
		if (text !== undefined) e.textContent = text;
		return e;
	}

	// ---- multi-touch routing ---------------------------------------------
	// touch identifier -> { move(touch)?, end() }.
	const activeTouches = new Map();

	function onRootTouchMove(e) {
		let handled = false;
		for (const t of e.changedTouches) {
			const h = activeTouches.get(t.identifier);
			if (h) { handled = true; if (h.move) h.move(t); }
		}
		if (handled && e.cancelable) e.preventDefault();
	}
	function onRootTouchEnd(e) {
		for (const t of e.changedTouches) {
			const h = activeTouches.get(t.identifier);
			if (h) { activeTouches.delete(t.identifier); h.end(); }
		}
	}

	// Simple press-and-hold element (action buttons + pills).
	function bindPressable(elem, code) {
		elem.addEventListener("touchstart", (e) => {
			e.preventDefault(); // no scroll, no mouse-compat events, no dbl-tap zoom
			tickleAudio();
			for (const t of e.changedTouches) {
				if (activeTouches.has(t.identifier)) continue;
				elem.classList.add("itc-on");
				keyDown(code);
				activeTouches.set(t.identifier, {
					end: () => { elem.classList.remove("itc-on"); keyUp(code); },
				});
			}
		}, { passive: false });
	}

	// D-pad: one owning touch; radial 8-way hit zones; sliding retargets.
	function bindDpad(elem) {
		let ownerId = null;
		let mask = 0;
		let rect = null;

		function applyMask(next) {
			const changed = mask ^ next;
			if (!changed) return;
			for (const bit of [DIR_U, DIR_D, DIR_L, DIR_R]) {
				if (!(changed & bit)) continue;
				const code = DIR_CODE[bit];
				if (next & bit) { keyDown(code); arrowEls[bit].classList.add("itc-on"); }
				else { keyUp(code); arrowEls[bit].classList.remove("itc-on"); }
			}
			mask = next;
		}

		function maskFromPoint(x, y) {
			const cx = rect.left + rect.width / 2;
			const cy = rect.top + rect.height / 2;
			const dx = x - cx, dy = y - cy;
			const r = Math.hypot(dx, dy);
			if (r < rect.width * 0.14) return 0; // dead zone in the hub
			const ang = (Math.atan2(-dy, dx) * 180 / Math.PI + 360) % 360;
			return SECTORS[Math.round(ang / 45) % 8];
		}

		elem.addEventListener("touchstart", (e) => {
			e.preventDefault();
			tickleAudio();
			for (const t of e.changedTouches) {
				if (ownerId !== null || activeTouches.has(t.identifier)) continue;
				ownerId = t.identifier;
				rect = elem.getBoundingClientRect();
				applyMask(maskFromPoint(t.clientX, t.clientY));
				activeTouches.set(t.identifier, {
					move: (tt) => applyMask(maskFromPoint(tt.clientX, tt.clientY)),
					end: () => { ownerId = null; applyMask(0); },
				});
			}
		}, { passive: false });
	}

	function build() {
		if (root) return;

		const style = document.createElement("style");
		style.id = "ikemen-touch-style";
		style.textContent = CSS;
		document.head.appendChild(style);
		document.documentElement.classList.add("itc-touch-active");

		root = el("div");
		root.id = "ikemen-touch";

		// Toggle (always visible on touch devices).
		const toggle = el("div", "itc-toggle", "\u{1F3AE}");
		toggle.setAttribute("role", "button");
		toggle.setAttribute("aria-label", "Toggle touch controls");
		const doToggle = () => {
			if (root.classList.contains("itc-hidden")) api.show();
			else api.hide();
		};
		toggle.addEventListener("touchstart", (e) => { e.preventDefault(); tickleAudio(); doToggle(); }, { passive: false });
		toggle.addEventListener("click", doToggle); // mouse fallback
		root.appendChild(toggle);

		// Everything below lives in .itc-pad so the toggle can hide it.
		const pad = el("div", "itc-pad");

		// D-pad.
		dpadEl = el("div", "itc-dpad");
		arrowEls = {};
		for (const [bit, left, top, rot] of [
			[DIR_U, "50%", "16%", 0], [DIR_R, "84%", "50%", 90],
			[DIR_D, "50%", "84%", 180], [DIR_L, "16%", "50%", 270],
		]) {
			const a = el("span", "itc-ar", "▲");
			a.style.left = left;
			a.style.top = top;
			a.style.transform = `translate(-50%, -50%) rotate(${rot}deg)`;
			dpadEl.appendChild(a);
			arrowEls[bit] = a;
		}
		bindDpad(dpadEl);
		pad.appendChild(dpadEl);

		// Action buttons.
		const btns = el("div", "itc-btns");
		for (const b of BUTTONS) {
			const btn = el("div", "itc-btn", b.label);
			btn.dataset.code = b.code;
			btn.style.right = `calc(var(--tu) * ${(b.cx - 0.5).toFixed(2)})`;
			btn.style.bottom = `calc(var(--tu) * ${(b.cy - 0.5).toFixed(2)})`;
			bindPressable(btn, b.code);
			btns.appendChild(btn);
		}
		pad.appendChild(btns);

		// START / ESC pills, top-center.
		const meta = el("div", "itc-meta");
		const esc = el("div", "itc-pill", "ESC");
		esc.dataset.code = "Escape";
		bindPressable(esc, "Escape");
		const start = el("div", "itc-pill", "START");
		start.dataset.code = "Enter";
		bindPressable(start, "Enter");
		meta.appendChild(esc);
		meta.appendChild(start);
		pad.appendChild(meta);

		root.appendChild(pad);
		document.body.appendChild(root);

		// Shared move/end routing for every tracked touch.
		window.addEventListener("touchmove", onRootTouchMove, { passive: false });
		window.addEventListener("touchend", onRootTouchEnd, { passive: true });
		window.addEventListener("touchcancel", onRootTouchEnd, { passive: true });

		// Never leave keys stuck when the tab loses focus.
		const panic = () => { activeTouches.clear(); releaseAll(); clearVisualPressed(); };
		window.addEventListener("blur", panic);
		document.addEventListener("visibilitychange", () => { if (document.hidden) panic(); });

		// Double-tap-zoom guard for the game area outside the pad widgets.
		const stage = document.getElementById("stage");
		if (stage) {
			stage.addEventListener("touchend", (e) => { if (e.cancelable) e.preventDefault(); }, { passive: false });
		}
	}

	function clearVisualPressed() {
		if (!root) return;
		for (const n of root.querySelectorAll(".itc-on")) n.classList.remove("itc-on");
	}

	// ---- public/debug API -------------------------------------------------
	const api = {
		show() {
			build();
			root.classList.remove("itc-hidden");
			try { localStorage.removeItem(HIDDEN_LS_KEY); } catch { /* private mode */ }
		},
		hide() {
			if (!root) return;
			root.classList.add("itc-hidden");
			activeTouches.clear();
			releaseAll();
			clearVisualPressed();
			try { localStorage.setItem(HIDDEN_LS_KEY, "1"); } catch { /* private mode */ }
		},
		toggle() {
			if (!root || root.classList.contains("itc-hidden")) api.show();
			else api.hide();
		},
		state() { return [...held.keys()]; },
		isVisible() { return !!root && !root.classList.contains("itc-hidden"); },
	};
	window.__ikemenTouch = api;

	// Auto-enable on touch devices only; desktop gets nothing (the debug API
	// can still force it with __ikemenTouch.show()).
	if (isTouchDevice()) {
		build();
		let hidden = false;
		try { hidden = localStorage.getItem(HIDDEN_LS_KEY) === "1"; } catch { /* private mode */ }
		if (hidden) root.classList.add("itc-hidden");
	}
})();

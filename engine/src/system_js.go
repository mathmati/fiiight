//go:build js

// Browser (js/wasm) windowing backend over an HTML canvas.
// The canvas element must exist in the host page with id "ikemen-canvas".
package main

import (
	"fmt"
	"image"
	"sync"
	"syscall/js"
	"time"
)

const jsCanvasID = "ikemen-canvas"

// jsEvent is one queued browser input event, drained by pollEvents.
type jsEvent struct {
	kind int // 0 = keydown, 1 = keyup, 2 = text
	key  Key
	mod  ModifierKey
	text string
}

type Window struct {
	canvas     js.Value
	title      string
	x, y, w, h int
	fullscreen bool
	closeflag  bool

	// requestAnimationFrame present/yield machinery (see SwapBuffers)
	rafCh chan struct{}
	rafCb js.Func

	// last time (UnixNano) the smoothed FPS was mirrored to window.__ikemenFPS
	fpsLastReport uint64

	// browser event queue, filled by JS listeners, drained in pollEvents
	evMu    sync.Mutex
	events  []jsEvent
	jsFuncs []js.Func
}

func (s *System) newWindow(w, h int) (*Window, error) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return nil, fmt.Errorf("no DOM document available")
	}
	canvas := doc.Call("getElementById", jsCanvasID)
	if !canvas.Truthy() {
		return nil, fmt.Errorf("canvas element with id %q not found", jsCanvasID)
	}

	// Size the canvas backing store if the host page has not done so.
	if canvas.Get("width").Int() == 0 || canvas.Get("height").Int() == 0 {
		canvas.Set("width", w)
		canvas.Set("height", h)
	}

	_, forceWindowed := sys.cmdFlags["-windowed"]
	fullscreen := s.cfg.Video.Fullscreen && !forceWindowed

	window := &Window{
		canvas:     canvas,
		title:      s.cfg.Config.WindowTitle,
		w:          w,
		h:          h,
		fullscreen: fullscreen,
		rafCh:      make(chan struct{}, 1),
	}

	window.rafCb = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		select {
		case window.rafCh <- struct{}{}:
		default:
		}
		return nil
	})

	window.registerEventListeners()

	for i := range input.controllers {
		input.controllerstate[i] = &ControllerState{Buttons: make(map[int]byte)}
	}

	return window, nil
}

// registerEventListeners hooks keyboard and clipboard events into the
// window's event queue.
func (w *Window) registerEventListeners() {
	global := js.Global()

	push := func(ev jsEvent) {
		w.evMu.Lock()
		w.events = append(w.events, ev)
		w.evMu.Unlock()
	}

	keydown := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		code := e.Get("code").String()
		key := jsCodeToKey(code)
		mod := modifierFromEvent(e)
		if key != KeyUnknown {
			// Keep the browser from scrolling/stealing game keys, but let
			// browser-level shortcuts (F5/F11/F12, Ctrl/Meta combos) through.
			if (mod&(KModCtrl|KModGui)) == 0 && code != "F5" && code != "F11" && code != "F12" {
				e.Call("preventDefault")
			}
			if !e.Get("repeat").Truthy() {
				push(jsEvent{kind: 0, key: key, mod: mod})
			}
		}
		// Printable characters feed text entry (SDL TextInputEvent equivalent)
		if ch := e.Get("key").String(); len([]rune(ch)) == 1 && (mod&(KModCtrl|KModAlt|KModGui)) == 0 {
			push(jsEvent{kind: 2, text: ch})
		}
		return nil
	})
	keyup := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		key := jsCodeToKey(e.Get("code").String())
		if key != KeyUnknown {
			push(jsEvent{kind: 1, key: key, mod: modifierFromEvent(e)})
		}
		return nil
	})
	paste := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		if cd := e.Get("clipboardData"); cd.Truthy() {
			if text := cd.Call("getData", "text").String(); text != "" {
				push(jsEvent{kind: 2, text: text})
			}
		}
		e.Call("preventDefault")
		return nil
	})

	global.Call("addEventListener", "keydown", keydown)
	global.Call("addEventListener", "keyup", keyup)
	global.Call("addEventListener", "paste", paste)
	w.jsFuncs = append(w.jsFuncs, keydown, keyup, paste)
}

// SwapBuffers presents the frame and yields to the browser event loop by
// blocking until the next requestAnimationFrame callback fires. WebGL
// presents implicitly when control returns to the browser, so the critical
// part is the yield: without it the wasm main loop would starve the tab.
func (w *Window) SwapBuffers() {
	js.Global().Call("requestAnimationFrame", w.rafCb)
	<-w.rafCh
}

func (w *Window) SetIcon(icon []image.Image) {
	// No window icon in a browser context.
}

func (w *Window) SetSwapInterval(interval int) {
	// requestAnimationFrame paces presentation; nothing to configure.
}

func (w *Window) GetSize() (int, int) {
	return w.canvas.Get("width").Int(), w.canvas.Get("height").Int()
}

// Calculates a position and size for the viewport to fill the window while centered
// Returns x, y, width, height respectively (mirror of system_sdl.go)
func (w *Window) GetScaledViewportSize() (int32, int32, int32, int32) {
	winWidth, winHeight := w.GetSize()

	// If aspect ratio should not be kept, just return full window
	if !sys.cfg.Video.KeepAspect {
		return 0, 0, int32(winWidth), int32(winHeight)
	}

	var x, y, resizedWidth, resizedHeight int32 = 0, 0, int32(winWidth), int32(winHeight)

	// Select stage or default aspect ratio
	aspectGame := sys.getCurrentAspect()
	aspectWindow := float32(winWidth) / float32(winHeight)

	// Keep aspect ratio
	if aspectWindow > aspectGame {
		// Window is wider: black bars on sides
		resizedHeight = int32(winHeight)
		resizedWidth = int32(float32(resizedHeight) * aspectGame)
		x = (int32(winWidth) - resizedWidth) / 2
		y = 0
	} else {
		// Window is taller: black bars on top and bottom
		resizedWidth = int32(winWidth)
		resizedHeight = int32(float32(resizedWidth) / aspectGame)
		x = 0
		y = (int32(winHeight) - resizedHeight) / 2
	}

	return x, y, resizedWidth, resizedHeight
}

func (w *Window) GetClipboardString() string {
	// navigator.clipboard is async-only; paste events feed OnTextEntered instead.
	return ""
}

func (w *Window) toggleFullscreen() {
	doc := js.Global().Get("document")
	if w.fullscreen {
		if doc.Get("fullscreenElement").Truthy() {
			doc.Call("exitFullscreen")
		}
	} else {
		if w.canvas.Get("requestFullscreen").Truthy() {
			w.canvas.Call("requestFullscreen")
		}
	}
	w.fullscreen = !w.fullscreen
}

func (w *Window) UpdateDebugFPS() {
	// Mirror of the SDL implementation using time.Now (nanosecond ticks).
	now := uint64(time.Now().UnixNano())
	const freq = float32(1e9)
	diff := float32(now - sys.gameFPSprevcount)

	if diff > 0 {
		instantFPS := freq / diff
		// Use an EMA to apply smoothing
		sys.gameFPS = (sys.gameFPS * 0.95) + (instantFPS * 0.05)
	}

	sys.gameFPSprevcount = now

	// Mirror the smoothed FPS to JS once a second for test harnesses/debugging.
	if now-w.fpsLastReport >= 1e9 {
		w.fpsLastReport = now
		js.Global().Set("__ikemenFPS", float64(sys.gameFPS))
	}
}

func (w *Window) pollEvents() {
	w.evMu.Lock()
	events := w.events
	w.events = nil
	w.evMu.Unlock()

	for _, ev := range events {
		switch ev.kind {
		case 0:
			OnKeyPressed(ev.key, ev.mod)
		case 1:
			OnKeyReleased(ev.key, ev.mod)
		case 2:
			OnTextEntered(ev.text)
		}
	}

	pollGamepads()
}

func (w *Window) shouldClose() bool {
	return w.closeflag
}

func (w *Window) Close() {
	for _, f := range w.jsFuncs {
		f.Release()
	}
	w.jsFuncs = nil
	w.rafCb.Release()
}

// GLCreateContext and GLMakeCurrent exist so the "OpenGL"-named renderer
// path in system.go compiles; the WebGL2 context is owned by the renderer.
func (w *Window) GLCreateContext() (interface{}, error) {
	return nil, nil
}

func (w *Window) GLMakeCurrent(ctx interface{}) error {
	return nil
}

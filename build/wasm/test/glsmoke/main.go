//go:build js

// glsmoke: standalone WebGL2 smoke test for the Ikemen GO js/wasm renderer
// binding path. It imports nothing from the engine; instead it mirrors
// gl_js.go's init sequence with raw syscall/js calls:
//  1. acquire a webgl2 context on #ikemen-canvas with the same options the
//     engine uses (antialias:false, alpha:false, preserveDrawingBuffer:false)
//  2. report renderer caps (MAX_SAMPLES, EXT_color_buffer_float)
//  3. compile the engine's REAL sprite shader pair (sprite.vert.glsl /
//     sprite.frag.glsl, copied in by run.mjs) with the injected
//     "#version 300 es" header
//  4. draw a red|green two-texel quad through the RGBA path and read back
//  5. draw an indexed R8 texture through the palette path and read back
//
// Output: PASS/FAIL lines in window.__ikemenBootLog, terminated by
// GLSMOKE-DONE. Asserted by run.mjs over CDP in headless Chromium.
package main

import (
	_ "embed"
	"fmt"
	"strings"
	"syscall/js"
)

//go:embed sprite.vert.glsl
var vertSrc string

//go:embed sprite.frag.glsl
var fragSrc string

var (
	global = js.Global()
	gl     js.Value
	failed bool
)

func logLine(s string) {
	global.Get("console").Call("log", s)
	if log := global.Get("__ikemenBootLog"); log.Truthy() {
		log.Call("push", s)
	}
}

func pass(format string, a ...interface{}) {
	logLine("PASS: " + fmt.Sprintf(format, a...))
}

func fail(format string, a ...interface{}) {
	failed = true
	logLine("FAIL: " + fmt.Sprintf(format, a...))
}

func info(format string, a ...interface{}) {
	logLine("INFO: " + fmt.Sprintf(format, a...))
}

func u8Array(b []byte) js.Value {
	arr := global.Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(arr, b)
	return arr
}

func f32Array(f []float32) js.Value {
	arr := global.Get("Float32Array").New(len(f))
	for i, v := range f {
		arr.SetIndex(i, v)
	}
	return arr
}

func compileShader(shaderType int, src string) (js.Value, error) {
	if !strings.HasPrefix(strings.TrimSpace(src), "#version") {
		src = "#version 300 es\n" + src
	}
	sh := gl.Call("createShader", shaderType)
	gl.Call("shaderSource", sh, src)
	gl.Call("compileShader", sh)
	if !gl.Call("getShaderParameter", sh, 0x8B81 /* COMPILE_STATUS */).Bool() {
		return js.Null(), fmt.Errorf("shader compile failed: %s", gl.Call("getShaderInfoLog", sh).String())
	}
	return sh, nil
}

func readCenter(x, y int) [4]int {
	px := global.Get("Uint8Array").New(4)
	gl.Call("readPixels", x, y, 1, 1, 0x1908 /* RGBA */, 0x1401 /* UNSIGNED_BYTE */, px)
	return [4]int{px.Index(0).Int(), px.Index(1).Int(), px.Index(2).Int(), px.Index(3).Int()}
}

func near(v, want int) bool {
	d := v - want
	if d < 0 {
		d = -d
	}
	return d <= 8
}

func checkColor(label string, got [4]int, r, g, b int) {
	if near(got[0], r) && near(got[1], g) && near(got[2], b) {
		pass("%s -> rgba%v", label, got)
	} else {
		fail("%s: want ~(%d,%d,%d), got rgba%v", label, r, g, b, got)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fail("panic: %v", r)
		}
		if failed {
			logLine("GLSMOKE-FAILED")
		}
		logLine("GLSMOKE-DONE")
	}()

	doc := global.Get("document")
	canvas := doc.Call("getElementById", "ikemen-canvas")
	if !canvas.Truthy() {
		fail("canvas #ikemen-canvas not found")
		return
	}

	// Same context options as the engine binding (gl_js.go glInit).
	gl = canvas.Call("getContext", "webgl2", map[string]interface{}{
		"antialias":             false,
		"alpha":                 false,
		"preserveDrawingBuffer": false,
	})
	if !gl.Truthy() {
		fail("getContext(webgl2) returned null")
		return
	}
	pass("webgl2 context acquired")

	// Caps the engine gates on.
	info("VERSION=%s", gl.Call("getParameter", 0x1F02).String())
	info("RENDERER=%s", gl.Call("getParameter", 0x1F01).String())
	info("MAX_SAMPLES=%d", gl.Call("getParameter", 0x8D57).Int())
	info("EXT_color_buffer_float=%v", gl.Call("getExtension", "EXT_color_buffer_float").Truthy())

	// Compile the engine's actual sprite shader pair.
	vs, err := compileShader(0x8B31 /* VERTEX_SHADER */, vertSrc)
	if err != nil {
		fail("sprite.vert: %v", err)
		return
	}
	fs, err := compileShader(0x8B30 /* FRAGMENT_SHADER */, fragSrc)
	if err != nil {
		fail("sprite.frag: %v", err)
		return
	}
	prog := gl.Call("createProgram")
	gl.Call("attachShader", prog, vs)
	gl.Call("attachShader", prog, fs)
	gl.Call("linkProgram", prog)
	if !gl.Call("getProgramParameter", prog, 0x8B82 /* LINK_STATUS */).Bool() {
		fail("sprite program link: %s", gl.Call("getProgramInfoLog", prog).String())
		return
	}
	pass("engine sprite shaders compiled and linked under #version 300 es")
	gl.Call("useProgram", prog)

	uni := func(name string) js.Value { return gl.Call("getUniformLocation", prog, name) }
	set1i := func(name string, v int) { gl.Call("uniform1i", uni(name), v) }
	set1f := func(name string, v float32) { gl.Call("uniform1f", uni(name), v) }

	// Fullscreen quad, engine vertex layout: vec2 position + vec2 uv,
	// stride 16 bytes, TRIANGLE_STRIP.
	const (
		ARRAY_BUFFER   = 0x8892
		STATIC_DRAW    = 0x88E4
		FLOAT          = 0x1406
		TEXTURE_2D     = 0x0DE1
		TRIANGLE_STRIP = 0x0005
	)
	vao := gl.Call("createVertexArray")
	gl.Call("bindVertexArray", vao)
	vbo := gl.Call("createBuffer")
	gl.Call("bindBuffer", ARRAY_BUFFER, vbo)
	verts := []float32{
		-1, -1, 0, 0,
		1, -1, 1, 0,
		-1, 1, 0, 1,
		1, 1, 1, 1,
	}
	gl.Call("bufferData", ARRAY_BUFFER, f32Array(verts), STATIC_DRAW)
	posLoc := gl.Call("getAttribLocation", prog, "position").Int()
	uvLoc := gl.Call("getAttribLocation", prog, "uv").Int()
	gl.Call("enableVertexAttribArray", posLoc)
	gl.Call("vertexAttribPointer", posLoc, 2, FLOAT, false, 16, 0)
	gl.Call("enableVertexAttribArray", uvLoc)
	gl.Call("vertexAttribPointer", uvLoc, 2, FLOAT, false, 16, 8)

	// Identity modelview/projection: positions are already clip space.
	ident := []float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	gl.Call("uniformMatrix4fv", uni("modelview"), false, f32Array(ident))
	gl.Call("uniformMatrix4fv", uni("projection"), false, f32Array(ident))

	// Common sprite uniforms (neutral PalFX).
	set1i("isFlat", 0)
	set1i("isTrapez", 0)
	set1i("neg", 0)
	set1i("mask", -1)
	set1f("alpha", 1)
	set1f("gray", 0)
	set1f("hue", 0)
	gl.Call("uniform3f", uni("add"), 0, 0, 0)
	gl.Call("uniform3f", uni("mult"), 1, 1, 1)
	gl.Call("uniform4f", uni("tint"), 0, 0, 0, 0)
	gl.Call("uniform4f", uni("x1x2x4x3"), 0, 0, 0, 0)

	w := canvas.Get("width").Int()
	h := canvas.Get("height").Int()
	gl.Call("viewport", 0, 0, w, h)

	newTex := func(unit int) js.Value {
		t := gl.Call("createTexture")
		gl.Call("activeTexture", 0x84C0+unit)
		gl.Call("bindTexture", TEXTURE_2D, t)
		for _, p := range [][2]int{{0x2800, 0x2600}, {0x2801, 0x2600}, {0x2802, 0x812F}, {0x2803, 0x812F}} {
			gl.Call("texParameteri", TEXTURE_2D, p[0], p[1]) // NEAREST + CLAMP_TO_EDGE
		}
		return t
	}
	gl.Call("pixelStorei", 0x0CF5 /* UNPACK_ALIGNMENT */, 1)

	// ---- Test A: RGBA texture path (red|green gradient quad) ----
	newTex(0)
	// 2x1 RGBA: left red, right green
	gl.Call("texImage2D", TEXTURE_2D, 0, 0x8058 /* RGBA8 */, 2, 1, 0, 0x1908, 0x1401,
		u8Array([]byte{255, 0, 0, 255, 0, 255, 0, 255}))
	set1i("tex", 0)
	set1i("isRgba", 1)
	gl.Call("clearColor", 0, 0, 1, 1)
	gl.Call("clear", 0x4000)
	gl.Call("drawArrays", TRIANGLE_STRIP, 0, 4)
	checkColor("rgba path left(red)", readCenter(w/4, h/2), 255, 0, 0)
	checkColor("rgba path right(green)", readCenter(3*w/4, h/2), 0, 255, 0)

	// ---- Test B: paletted sprite path (R8 index texture + palette LUT) ----
	newTex(1)
	// 2x1 R8 index texture: indices 64 and 192
	gl.Call("texImage2D", TEXTURE_2D, 0, 0x8229 /* R8 */, 2, 1, 0, 0x1903 /* RED */, 0x1401,
		u8Array([]byte{64, 192}))
	newTex(2)
	// 256x1 RGBA palette: [64]=blue, [192]=yellow
	palData := make([]byte, 256*4)
	palData[64*4+2] = 255 // blue
	palData[64*4+3] = 255
	palData[192*4+0] = 255 // yellow
	palData[192*4+1] = 255
	palData[192*4+3] = 255
	gl.Call("texImage2D", TEXTURE_2D, 0, 0x8058, 256, 1, 0, 0x1908, 0x1401, u8Array(palData))
	set1i("tex", 1)
	set1i("pal", 2)
	set1i("isRgba", 0)
	gl.Call("clear", 0x4000)
	gl.Call("drawArrays", TRIANGLE_STRIP, 0, 4)
	checkColor("palette path left(blue)", readCenter(w/4, h/2), 0, 0, 255)
	checkColor("palette path right(yellow)", readCenter(3*w/4, h/2), 255, 255, 0)

	if e := gl.Call("getError").Int(); e != 0 {
		fail("gl.getError() = 0x%x", e)
	} else {
		pass("no GL errors")
	}
}

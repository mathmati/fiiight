//go:build js

// Thin WebGL2 binding over syscall/js for the browser renderer
// (render_webgl2.go / font_webgl2.go).
//
// Design notes:
//   - WebGL2 hands out opaque JS objects for programs/shaders/buffers/
//     textures/framebuffers/renderbuffers/VAOs. The engine stores uint32
//     handles, so the binding interns every created object in a table and
//     returns a monotonically increasing handle (0 == null object).
//   - Uniform locations are likewise opaque objects; they are interned once
//     per (program, name) at RegisterUniforms time and referenced by int32
//     ids afterwards (-1 == not found), matching the desktop backends.
//   - All bulk uploads (BufferData, TexImage2D, ReadPixels, uniform arrays)
//     go through a single grow-on-demand scratch Uint8Array via
//     js.CopyBytesToJS, with a Float32Array view over the same buffer for
//     float uploads. No per-call typed-array allocation beyond the subarray
//     view object.
//   - gl.getError polling is a performance killer; it only happens when
//     glDebug is set (Video.RendererDebugMode).
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"
)

// WebGL2 constants (numeric values match GLES 3.0).
const (
	gl_NONE                        = 0
	gl_LINES                       = 0x0001
	gl_LINE_LOOP                   = 0x0002
	gl_LINE_STRIP                  = 0x0003
	gl_TRIANGLES                   = 0x0004
	gl_TRIANGLE_STRIP              = 0x0005
	gl_TRIANGLE_FAN                = 0x0006
	gl_ZERO                        = 0
	gl_ONE                         = 1
	gl_SRC_ALPHA                   = 0x0302
	gl_ONE_MINUS_SRC_ALPHA         = 0x0303
	gl_DST_COLOR                   = 0x0306
	gl_ONE_MINUS_DST_COLOR         = 0x0307
	gl_FUNC_ADD                    = 0x8006
	gl_FUNC_REVERSE_SUBTRACT       = 0x800B
	gl_DEPTH_BUFFER_BIT            = 0x00000100
	gl_COLOR_BUFFER_BIT            = 0x00004000
	gl_LESS                        = 0x0201
	gl_CULL_FACE                   = 0x0B44
	gl_DEPTH_TEST                  = 0x0B71
	gl_BLEND                       = 0x0BE2
	gl_SCISSOR_TEST                = 0x0C11
	gl_CW                          = 0x0900
	gl_CCW                         = 0x0901
	gl_BACK                        = 0x0405
	gl_TEXTURE_2D                  = 0x0DE1
	gl_TEXTURE_CUBE_MAP            = 0x8513
	gl_TEXTURE_CUBE_MAP_POSITIVE_X = 0x8515
	gl_TEXTURE0                    = 0x84C0
	gl_TEXTURE_MAG_FILTER          = 0x2800
	gl_TEXTURE_MIN_FILTER          = 0x2801
	gl_TEXTURE_WRAP_S              = 0x2802
	gl_TEXTURE_WRAP_T              = 0x2803
	gl_NEAREST                     = 0x2600
	gl_LINEAR                      = 0x2601
	gl_NEAREST_MIPMAP_NEAREST      = 0x2700
	gl_LINEAR_MIPMAP_NEAREST       = 0x2701
	gl_NEAREST_MIPMAP_LINEAR       = 0x2702
	gl_LINEAR_MIPMAP_LINEAR        = 0x2703
	gl_CLAMP_TO_EDGE               = 0x812F
	gl_MIRRORED_REPEAT             = 0x8370
	gl_REPEAT                      = 0x2901
	gl_UNPACK_ROW_LENGTH           = 0x0CF2
	gl_UNPACK_ALIGNMENT            = 0x0CF5
	gl_UNSIGNED_BYTE               = 0x1401
	gl_INT                         = 0x1404
	gl_UNSIGNED_INT                = 0x1405
	gl_FLOAT                       = 0x1406
	gl_HALF_FLOAT                  = 0x140B
	gl_RED                         = 0x1903
	gl_RGB                         = 0x1907
	gl_RGBA                        = 0x1908
	gl_R8                          = 0x8229
	gl_RGB8                        = 0x8051
	gl_RGBA8                       = 0x8058
	gl_RGBA16F                     = 0x881A
	gl_RGB32F                      = 0x8815
	gl_RGBA32F                     = 0x8814
	gl_DEPTH_COMPONENT16           = 0x81A5
	gl_VERTEX_SHADER               = 0x8B31
	gl_FRAGMENT_SHADER             = 0x8B30
	gl_COMPILE_STATUS              = 0x8B81
	gl_LINK_STATUS                 = 0x8B82
	gl_ARRAY_BUFFER                = 0x8892
	gl_ELEMENT_ARRAY_BUFFER        = 0x8893
	gl_STATIC_DRAW                 = 0x88E4
	gl_DYNAMIC_DRAW                = 0x88E8
	gl_FRAMEBUFFER                 = 0x8D40
	gl_READ_FRAMEBUFFER            = 0x8CA8
	gl_DRAW_FRAMEBUFFER            = 0x8CA9
	gl_RENDERBUFFER                = 0x8D41
	gl_COLOR_ATTACHMENT0           = 0x8CE0
	gl_DEPTH_ATTACHMENT            = 0x8D00
	gl_FRAMEBUFFER_COMPLETE        = 0x8CD5
	gl_MAX_TEXTURE_IMAGE_UNITS     = 0x8872
	gl_MAX_SAMPLES                 = 0x8D57
	gl_VERSION                     = 0x1F02
	gl_RENDERER                    = 0x1F01
)

var (
	glCtx     js.Value // the WebGL2RenderingContext
	glObjs    map[uint32]js.Value
	glNextObj uint32
	glLocs    map[int32]js.Value
	glNextLoc int32

	// Shared scratch: one Uint8Array plus a Float32Array view over the same
	// ArrayBuffer, grown on demand, plus a Go-side staging slice for
	// float32 -> byte conversion.
	glU8         js.Value
	glF32        js.Value
	glScratchCap int
	glStage      []byte

	glHasColorBufferFloat bool
	glDebug               bool
)

// glInit acquires (once) the WebGL2 context from the #ikemen-canvas element
// and probes EXT_color_buffer_float.
func glInit() error {
	if glCtx.Truthy() {
		return nil
	}
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return fmt.Errorf("WebGL2 init: no DOM document available")
	}
	canvas := doc.Call("getElementById", jsCanvasID)
	if !canvas.Truthy() {
		return fmt.Errorf("WebGL2 init: canvas element %q not found", jsCanvasID)
	}
	// antialias:false — the engine does its own MSAA into an FBO.
	ctx := canvas.Call("getContext", "webgl2", map[string]interface{}{
		"antialias":             false,
		"alpha":                 false,
		"preserveDrawingBuffer": false,
	})
	if !ctx.Truthy() {
		return fmt.Errorf("WebGL2 init: getContext(\"webgl2\") returned null")
	}
	glCtx = ctx
	glObjs = make(map[uint32]js.Value, 64)
	glNextObj = 1
	glLocs = make(map[int32]js.Value, 256)
	glNextLoc = 1
	glHasColorBufferFloat = ctx.Call("getExtension", "EXT_color_buffer_float").Truthy()
	glEnsureScratch(1 << 16)
	return nil
}

func glEnsureScratch(n int) {
	if n <= glScratchCap {
		return
	}
	c := glScratchCap
	if c == 0 {
		c = 1 << 12
	}
	for c < n {
		c *= 2
	}
	glU8 = js.Global().Get("Uint8Array").New(c)
	glF32 = js.Global().Get("Float32Array").New(glU8.Get("buffer"), 0, c/4)
	glScratchCap = c
}

// glBytes copies b into the scratch buffer and returns a Uint8Array view.
func glBytes(b []byte) js.Value {
	glEnsureScratch(len(b))
	js.CopyBytesToJS(glU8, b)
	return glU8.Call("subarray", 0, len(b))
}

// glBytesF32 copies raw bytes and returns a Float32Array view over them
// (for FLOAT-typed uploads whose engine-side data is []byte).
func glBytesF32(b []byte) js.Value {
	glEnsureScratch(len(b))
	js.CopyBytesToJS(glU8, b)
	return glF32.Call("subarray", 0, len(b)/4)
}

// glFloats converts f to little-endian bytes (wasm is little-endian, as are
// JS typed arrays on all supported platforms) and returns a Float32Array view.
func glFloats(f []float32) js.Value {
	n := len(f) * 4
	if cap(glStage) < n {
		glStage = make([]byte, n*2)
	}
	s := glStage[:n]
	for i, v := range f {
		binary.LittleEndian.PutUint32(s[i*4:], math.Float32bits(v))
	}
	glEnsureScratch(n)
	js.CopyBytesToJS(glU8, s)
	return glF32.Call("subarray", 0, len(f))
}

// Object table helpers.
func glRegister(v js.Value) uint32 {
	h := glNextObj
	glNextObj++
	glObjs[h] = v
	return h
}

func glObj(h uint32) js.Value {
	if h == 0 {
		return js.Null()
	}
	if v, ok := glObjs[h]; ok {
		return v
	}
	return js.Null()
}

func glUnregister(h uint32) js.Value {
	v := glObj(h)
	delete(glObjs, h)
	return v
}

func glLoc(l int32) js.Value {
	if v, ok := glLocs[l]; ok {
		return v
	}
	return js.Null()
}

// glCheckError polls gl.getError when debug mode is on.
func glCheckError(tag string) {
	if !glDebug {
		return
	}
	if e := glCtx.Call("getError").Int(); e != 0 {
		Logcat(fmt.Sprintf("WebGL2 ERROR 0x%x at %s", e, tag))
	}
}

// ---- shaders / programs ----

func glCreateShader(shaderType uint32) uint32 {
	return glRegister(glCtx.Call("createShader", int(shaderType)))
}

func glShaderSource(shader uint32, src string) {
	glCtx.Call("shaderSource", glObj(shader), src)
}

func glCompileShader(shader uint32) {
	glCtx.Call("compileShader", glObj(shader))
}

func glGetShaderCompileStatus(shader uint32) bool {
	return glCtx.Call("getShaderParameter", glObj(shader), gl_COMPILE_STATUS).Bool()
}

func glGetShaderInfoLog(shader uint32) string {
	if v := glCtx.Call("getShaderInfoLog", glObj(shader)); v.Truthy() {
		return v.String()
	}
	return ""
}

func glDeleteShader(shader uint32) {
	glCtx.Call("deleteShader", glUnregister(shader))
}

func glCreateProgram() uint32 {
	return glRegister(glCtx.Call("createProgram"))
}

func glAttachShader(program, shader uint32) {
	glCtx.Call("attachShader", glObj(program), glObj(shader))
}

func glDetachShader(program, shader uint32) {
	glCtx.Call("detachShader", glObj(program), glObj(shader))
}

func glLinkProgram(program uint32) {
	glCtx.Call("linkProgram", glObj(program))
}

func glGetProgramLinkStatus(program uint32) bool {
	return glCtx.Call("getProgramParameter", glObj(program), gl_LINK_STATUS).Bool()
}

func glGetProgramInfoLog(program uint32) string {
	if v := glCtx.Call("getProgramInfoLog", glObj(program)); v.Truthy() {
		return v.String()
	}
	return ""
}

func glDeleteProgram(program uint32) {
	glCtx.Call("deleteProgram", glUnregister(program))
}

func glUseProgram(program uint32) {
	glCtx.Call("useProgram", glObj(program))
}

func glGetAttribLocation(program uint32, name string) int32 {
	return int32(glCtx.Call("getAttribLocation", glObj(program), name).Int())
}

func glGetUniformLocation(program uint32, name string) int32 {
	v := glCtx.Call("getUniformLocation", glObj(program), name)
	if !v.Truthy() {
		return -1
	}
	l := glNextLoc
	glNextLoc++
	glLocs[l] = v
	return l
}

// ---- buffers / vertex arrays ----

func glGenBuffer() uint32 {
	return glRegister(glCtx.Call("createBuffer"))
}

func glBindBuffer(target, buffer uint32) {
	glCtx.Call("bindBuffer", int(target), glObj(buffer))
}

func glBufferData(target uint32, data []byte, usage uint32) {
	glCtx.Call("bufferData", int(target), glBytes(data), int(usage))
}

func glBufferDataSize(target uint32, size int, usage uint32) {
	glCtx.Call("bufferData", int(target), size, int(usage))
}

func glBufferSubData(target uint32, offset int, data []byte) {
	glCtx.Call("bufferSubData", int(target), offset, glBytes(data))
}

func glGenVertexArray() uint32 {
	return glRegister(glCtx.Call("createVertexArray"))
}

func glBindVertexArray(vao uint32) {
	glCtx.Call("bindVertexArray", glObj(vao))
}

func glEnableVertexAttribArray(loc uint32) {
	glCtx.Call("enableVertexAttribArray", int(loc))
}

func glDisableVertexAttribArray(loc uint32) {
	glCtx.Call("disableVertexAttribArray", int(loc))
}

func glVertexAttribPointer(loc uint32, size int, xtype uint32, normalized bool, stride, offset int) {
	glCtx.Call("vertexAttribPointer", int(loc), size, int(xtype), normalized, stride, offset)
}

func glVertexAttrib2f(loc uint32, x, y float32) {
	glCtx.Call("vertexAttrib2f", int(loc), x, y)
}

func glVertexAttrib3f(loc uint32, x, y, z float32) {
	glCtx.Call("vertexAttrib3f", int(loc), x, y, z)
}

func glVertexAttrib4f(loc uint32, x, y, z, w float32) {
	glCtx.Call("vertexAttrib4f", int(loc), x, y, z, w)
}

// ---- textures ----

func glGenTexture() uint32 {
	return glRegister(glCtx.Call("createTexture"))
}

func glDeleteTexture(tex uint32) {
	if tex == 0 {
		return
	}
	glCtx.Call("deleteTexture", glUnregister(tex))
}

func glActiveTexture(unit uint32) {
	glCtx.Call("activeTexture", int(unit))
}

func glBindTexture(target, tex uint32) {
	glCtx.Call("bindTexture", int(target), glObj(tex))
}

func glTexParameteri(target, pname uint32, param int32) {
	glCtx.Call("texParameteri", int(target), int(pname), param)
}

func glPixelStorei(pname uint32, param int32) {
	glCtx.Call("pixelStorei", int(pname), param)
}

// glTexImage2D uploads byte data (or allocates storage when data is nil).
// When xtype is FLOAT the view handed to WebGL must be a Float32Array.
func glTexImage2D(target uint32, level, internalFormat, width, height int32, format, xtype uint32, data []byte) {
	var view js.Value = js.Null()
	if data != nil {
		if xtype == gl_FLOAT {
			view = glBytesF32(data)
		} else {
			view = glBytes(data)
		}
	}
	glCtx.Call("texImage2D", int(target), level, int(internalFormat), width, height, 0, int(format), int(xtype), view)
}

func glTexImage2Df(target uint32, level, internalFormat, width, height int32, format uint32, data []float32) {
	var view js.Value = js.Null()
	if data != nil {
		view = glFloats(data)
	}
	glCtx.Call("texImage2D", int(target), level, int(internalFormat), width, height, 0, int(format), gl_FLOAT, view)
}

// glTexImage2DSource uploads a DOM TexImageSource (HTMLVideoElement,
// HTMLCanvasElement, ImageBitmap, ...) into the bound texture. WebGL2
// accepts the element directly, so no per-frame CPU pixel copy is needed.
func glTexImage2DSource(target uint32, level, internalFormat, width, height int32, format, xtype uint32, source js.Value) {
	glCtx.Call("texImage2D", int(target), level, int(internalFormat), width, height, 0, int(format), int(xtype), source)
}

func glTexSubImage2D(target uint32, level, x, y, width, height int32, format, xtype uint32, data []byte) {
	if data == nil {
		return
	}
	var view js.Value
	if xtype == gl_FLOAT {
		view = glBytesF32(data)
	} else {
		view = glBytes(data)
	}
	glCtx.Call("texSubImage2D", int(target), level, x, y, width, height, int(format), int(xtype), view)
}

func glCopyTexSubImage2D(target uint32, level, xoffset, yoffset, x, y, width, height int32) {
	glCtx.Call("copyTexSubImage2D", int(target), level, xoffset, yoffset, x, y, width, height)
}

func glGenerateMipmap(target uint32) {
	glCtx.Call("generateMipmap", int(target))
}

// ---- framebuffers / renderbuffers ----

func glGenFramebuffer() uint32 {
	return glRegister(glCtx.Call("createFramebuffer"))
}

func glDeleteFramebuffer(fbo uint32) {
	if fbo == 0 {
		return
	}
	glCtx.Call("deleteFramebuffer", glUnregister(fbo))
}

func glBindFramebuffer(target, fbo uint32) {
	glCtx.Call("bindFramebuffer", int(target), glObj(fbo))
}

func glFramebufferTexture2D(target, attachment, textarget, texture uint32, level int32) {
	glCtx.Call("framebufferTexture2D", int(target), int(attachment), int(textarget), glObj(texture), level)
}

func glFramebufferRenderbuffer(target, attachment, rbtarget, rb uint32) {
	glCtx.Call("framebufferRenderbuffer", int(target), int(attachment), int(rbtarget), glObj(rb))
}

func glCheckFramebufferStatus(target uint32) uint32 {
	return uint32(glCtx.Call("checkFramebufferStatus", int(target)).Int())
}

func glGenRenderbuffer() uint32 {
	return glRegister(glCtx.Call("createRenderbuffer"))
}

func glBindRenderbuffer(target, rb uint32) {
	glCtx.Call("bindRenderbuffer", int(target), glObj(rb))
}

func glRenderbufferStorage(target, internalFormat uint32, width, height int32) {
	glCtx.Call("renderbufferStorage", int(target), int(internalFormat), width, height)
}

func glRenderbufferStorageMultisample(target uint32, samples int32, internalFormat uint32, width, height int32) {
	glCtx.Call("renderbufferStorageMultisample", int(target), samples, int(internalFormat), width, height)
}

func glBlitFramebuffer(srcX0, srcY0, srcX1, srcY1, dstX0, dstY0, dstX1, dstY1 int32, mask uint32, filter uint32) {
	glCtx.Call("blitFramebuffer", srcX0, srcY0, srcX1, srcY1, dstX0, dstY0, dstX1, dstY1, int(mask), int(filter))
}

func glReadBuffer(mode uint32) {
	glCtx.Call("readBuffer", int(mode))
}

func glReadPixels(x, y, width, height int32, format, xtype uint32, dst []byte) {
	glEnsureScratch(len(dst))
	view := glU8.Call("subarray", 0, len(dst))
	glCtx.Call("readPixels", x, y, width, height, int(format), int(xtype), view)
	js.CopyBytesToGo(dst, view)
}

// ---- global state ----

func glEnable(cap uint32) {
	glCtx.Call("enable", int(cap))
}

func glDisable(cap uint32) {
	glCtx.Call("disable", int(cap))
}

func glDepthFunc(fn uint32) {
	glCtx.Call("depthFunc", int(fn))
}

func glDepthMask(flag bool) {
	glCtx.Call("depthMask", flag)
}

func glFrontFace(mode uint32) {
	glCtx.Call("frontFace", int(mode))
}

func glCullFace(mode uint32) {
	glCtx.Call("cullFace", int(mode))
}

func glBlendEquation(mode uint32) {
	glCtx.Call("blendEquation", int(mode))
}

func glBlendFunc(sfactor, dfactor uint32) {
	glCtx.Call("blendFunc", int(sfactor), int(dfactor))
}

func glScissor(x, y, width, height int32) {
	glCtx.Call("scissor", x, y, width, height)
}

func glViewport(x, y, width, height int32) {
	glCtx.Call("viewport", x, y, width, height)
}

func glClear(mask uint32) {
	glCtx.Call("clear", int(mask))
}

func glGetParameterInt(pname uint32) int {
	return glCtx.Call("getParameter", int(pname)).Int()
}

func glGetParameterString(pname uint32) string {
	if v := glCtx.Call("getParameter", int(pname)); v.Truthy() {
		return v.String()
	}
	return ""
}

// ---- uniforms ----

func glUniform1i(loc int32, v int32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniform1i", glLoc(loc), v)
}

func glUniform1f(loc int32, x float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniform1f", glLoc(loc), x)
}

func glUniform2f(loc int32, x, y float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniform2f", glLoc(loc), x, y)
}

func glUniform3f(loc int32, x, y, z float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniform3f", glLoc(loc), x, y, z)
}

func glUniform4f(loc int32, x, y, z, w float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniform4f", glLoc(loc), x, y, z, w)
}

func glUniform1fv(loc int32, values []float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniform1fv", glLoc(loc), glFloats(values))
}

func glUniform4fv(loc int32, values []float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniform4fv", glLoc(loc), glFloats(values))
}

func glUniformMatrix3fv(loc int32, values []float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniformMatrix3fv", glLoc(loc), false, glFloats(values))
}

func glUniformMatrix4fv(loc int32, values []float32) {
	if loc < 0 {
		return
	}
	glCtx.Call("uniformMatrix4fv", glLoc(loc), false, glFloats(values))
}

// ---- draws ----

func glDrawArrays(mode uint32, first, count int32) {
	glCtx.Call("drawArrays", int(mode), first, count)
}

func glDrawElements(mode uint32, count int32, xtype uint32, offset int) {
	glCtx.Call("drawElements", int(mode), count, int(xtype), offset)
}

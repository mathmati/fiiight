//go:build js

// WebGL2 renderer for the browser (js/wasm) build. Port of
// render_gles32.go (GLES 3.2, android) onto the syscall/js binding in
// gl_js.go, per port/SPEC.md §6:
//   - shaders are compiled with an injected "#version 300 es" header;
//   - the model-shadow pipeline (geometry shader + samplerCubeArray) exceeds
//     WebGL2 and is cut: IsShadowEnabled() is always false, the shadow-map
//     methods are no-ops and the model shader is compiled without
//     ENABLE_SHADOW;
//   - MSAA uses multisampled renderbuffers + BlitFramebuffer (WebGL2 has no
//     TEXTURE_2D_MULTISAMPLE), capped to gl.MAX_SAMPLES;
//   - float render targets (env cubemaps / GGX LUT) are gated on the
//     EXT_color_buffer_float probe; basic model rendering stays enabled
//     without it;
//   - SetVSync is a no-op (requestAnimationFrame paces presentation),
//     NewWorkerThread() is false and Await() is a no-op.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	mgl "github.com/go-gl/mathgl/mgl32"
	"golang.org/x/mobile/exp/f32"
)

// Base time for the post-processing CurrentTime uniform.
var webgl2StartTime = time.Now()

// ------------------------------------------------------------------
// ShaderProgram_WebGL2

type ShaderProgram_WebGL2 struct {
	program       uint32           // binding-side handle
	attributes    map[string]int32 // Attribute name to location
	uniforms      map[string]int32 // Uniform name to location
	textures      map[string]int   // Sampler name to texture unit
	name          string           // For debugging
	needsGrabPass bool
}

func (r *Renderer_WebGL2) newShaderProgram(vert, frag, geo, name string, crashWhenFail bool) (s *ShaderProgram_WebGL2, err error) {
	if len(geo) > 0 {
		// WebGL2 has no geometry shaders (SPEC §6); the only user is the
		// shadow pipeline, which is disabled on js.
		err = fmt.Errorf("geometry shaders are not supported on WebGL2 (%s)", name)
		if chkEX(err, "Shader compilation error on "+name+"\n", crashWhenFail) {
			return nil, err
		}
	}

	var vertObj, fragObj, prog uint32

	if vertObj, err = r.compileShader(gl_VERTEX_SHADER, vert); chkEX(err, "Shader compilation error on "+name+"\n", crashWhenFail) {
		return nil, err
	}
	if fragObj, err = r.compileShader(gl_FRAGMENT_SHADER, frag); chkEX(err, "Shader compilation error on "+name+"\n", crashWhenFail) {
		return nil, err
	}
	if prog, err = r.linkProgram(vertObj, fragObj); chkEX(err, "Link program error on "+name+"\n", crashWhenFail) {
		return nil, err
	}

	s = &ShaderProgram_WebGL2{program: prog, name: name}
	s.attributes = make(map[string]int32)
	s.uniforms = make(map[string]int32)
	s.textures = make(map[string]int)

	return s, nil
}

func (s *ShaderProgram_WebGL2) RegisterAttributes(names ...string) {
	for _, name := range names {
		s.attributes[name] = glGetAttribLocation(s.program, name)
	}
}

func (s *ShaderProgram_WebGL2) RegisterUniforms(names ...string) {
	for _, name := range names {
		s.uniforms[name] = glGetUniformLocation(s.program, name)
	}
}

func (s *ShaderProgram_WebGL2) RegisterTextures(names ...string) {
	for _, name := range names {
		s.uniforms[name] = glGetUniformLocation(s.program, name)
		s.textures[name] = len(s.textures)
	}
}

func (r *Renderer_WebGL2) compileShader(shaderType uint32, src string) (uint32, error) {
	shader := glCreateShader(shaderType)

	// Version header injection: the shaders in src/shaders/*.glsl are
	// version-less bodies with #ifdef GL_ES branches; WebGL2 wants
	// "#version 300 es" as the very first line (GL_ES is then predefined,
	// which activates the precision blocks in the shaders).
	fullSrc := src
	if !strings.HasPrefix(strings.TrimSpace(src), "#version") {
		fullSrc = "#version 300 es\n" + src
	}
	// Strip the NUL terminators the cgo backends append to external shaders.
	fullSrc = strings.TrimRight(fullSrc, "\x00")

	glShaderSource(shader, fullSrc)
	glCompileShader(shader)

	if !glGetShaderCompileStatus(shader) {
		typeName := "VERTEX"
		if shaderType == gl_FRAGMENT_SHADER {
			typeName = "FRAGMENT"
		}
		log := glGetShaderInfoLog(shader)
		glDeleteShader(shader)
		return 0, fmt.Errorf("WebGL2 %s Shader Err: %s", typeName, log)
	}
	return shader, nil
}

func (r *Renderer_WebGL2) linkProgram(params ...uint32) (program uint32, err error) {
	program = glCreateProgram()
	for _, param := range params {
		glAttachShader(program, param)
	}
	glLinkProgram(program)
	// Mark shaders for deletion when the program is deleted
	for _, param := range params {
		glDetachShader(program, param)
		glDeleteShader(param)
	}

	if !glGetProgramLinkStatus(program) {
		err = fmt.Errorf("Link error: %s", glGetProgramInfoLog(program))
		glDeleteProgram(program)
		return 0, err
	}
	return program, nil
}

// ------------------------------------------------------------------
// Texture_WebGL2

type Texture_WebGL2 struct {
	width  int32
	height int32
	depth  int32
	filter bool
	handle uint32 // binding-side handle
	serial uint64 // Go side serial number
}

// Helper that wraps the actual GL call to generate a texture
func (r *Renderer_WebGL2) generateTexture(width, height, depth int32, filter bool) *Texture_WebGL2 {
	h := glGenTexture()

	// Ensure a unique ID even if GL reuses the handle
	textureSerialNumber++

	tex := &Texture_WebGL2{
		width:  width,
		height: height,
		depth:  depth,
		filter: filter,
		handle: h,
		serial: textureSerialNumber,
	}

	// Note: no runtime.SetFinalizer here. The js build is single-threaded
	// and the finalizer->mainThreadTask path of the desktop backends would
	// only reclaim handles at GC time anyway; WebGL objects are reclaimed
	// with the context when the tab goes away. Explicit deletes still work
	// through glDeleteTexture.

	return tex
}

// Creates a generic texture
func (r *Renderer_WebGL2) newTexture(width, height, depth int32, filter bool) Texture {
	r.SetActiveTexture0()
	return r.generateTexture(width, height, depth, filter)
}

func (r *Renderer_WebGL2) newPaletteTexture() Texture {
	return r.newTexture(256, 1, 32, false)
}

func (r *Renderer_WebGL2) newModelTexture(width, height, depth int32, filter bool) Texture {
	return r.newTexture(width, height, depth, filter)
}

func (r *Renderer_WebGL2) newDataTexture(width, height int32) Texture {
	r.SetActiveTexture0()

	t := r.generateTexture(width, height, 32, false)

	glBindTexture(gl_TEXTURE_2D, t.handle)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, gl_NEAREST)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, gl_NEAREST)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_S, gl_CLAMP_TO_EDGE)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_T, gl_CLAMP_TO_EDGE)
	return t
}

func (r *Renderer_WebGL2) newHDRTexture(width, height int32) Texture {
	r.SetActiveTexture0()

	t := r.generateTexture(width, height, 24, false)

	glBindTexture(gl_TEXTURE_2D, t.handle)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, gl_LINEAR)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, gl_LINEAR)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_S, gl_MIRRORED_REPEAT)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_T, gl_MIRRORED_REPEAT)
	return t
}

func (r *Renderer_WebGL2) newCubeMapTexture(widthHeight int32, mipmap bool, lowestMipLevel int32) Texture {
	r.SetActiveTexture0()

	t := r.generateTexture(widthHeight, widthHeight, 24, false)

	glBindTexture(gl_TEXTURE_CUBE_MAP, t.handle)
	for i := 0; i < 6; i++ {
		glTexImage2D(uint32(gl_TEXTURE_CUBE_MAP_POSITIVE_X+i), 0, gl_RGBA16F, widthHeight, widthHeight, gl_RGBA, gl_HALF_FLOAT, nil)
	}

	if mipmap && glHasColorBufferFloat {
		glTexParameteri(gl_TEXTURE_CUBE_MAP, gl_TEXTURE_MIN_FILTER, gl_LINEAR_MIPMAP_LINEAR)
		glGenerateMipmap(gl_TEXTURE_CUBE_MAP)
	} else {
		glTexParameteri(gl_TEXTURE_CUBE_MAP, gl_TEXTURE_MIN_FILTER, gl_LINEAR)
	}

	glTexParameteri(gl_TEXTURE_CUBE_MAP, gl_TEXTURE_MAG_FILTER, gl_LINEAR)
	glTexParameteri(gl_TEXTURE_CUBE_MAP, gl_TEXTURE_WRAP_S, gl_CLAMP_TO_EDGE)
	glTexParameteri(gl_TEXTURE_CUBE_MAP, gl_TEXTURE_WRAP_T, gl_CLAMP_TO_EDGE)
	return t
}

// Bind a texture and upload texel data to it
func (t *Texture_WebGL2) SetData(data []byte) {
	var interp int32 = gl_NEAREST
	if t.filter {
		interp = gl_LINEAR
	}

	bits := Max(t.depth, 8)
	internalFormat := t.MapSizedInternalFormat(bits)
	uploadFormat := t.MapUploadFormat(bits)
	uploadType := t.MapUploadType(bits)

	r := gfx.(*Renderer_WebGL2)
	r.SetActiveTexture0()

	glBindTexture(gl_TEXTURE_2D, t.handle)
	glPixelStorei(gl_UNPACK_ALIGNMENT, 1)
	glPixelStorei(gl_UNPACK_ROW_LENGTH, 0)

	glTexImage2D(gl_TEXTURE_2D, 0, int32(internalFormat), t.width, t.height, uploadFormat, uploadType, data)

	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, interp)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, interp)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_S, gl_CLAMP_TO_EDGE)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_T, gl_CLAMP_TO_EDGE)
}

func (t *Texture_WebGL2) SetSubData(data []byte, x, y, width, height, stride int32) {
	var interp int32 = gl_NEAREST
	if t.filter {
		interp = gl_LINEAR
	}

	r := gfx.(*Renderer_WebGL2)
	r.SetActiveTexture0()

	glBindTexture(gl_TEXTURE_2D, t.handle)
	glPixelStorei(gl_UNPACK_ALIGNMENT, 1)

	bits := Max(t.depth, 8)
	uploadFormat := t.MapUploadFormat(bits)
	uploadType := t.MapUploadType(bits)
	bytesPerPixel := t.depth / 8
	if bytesPerPixel < 1 {
		bytesPerPixel = 1
	}

	var rowLength int32 = 0
	if stride != width*bytesPerPixel {
		rowLength = stride / bytesPerPixel
	}

	glPixelStorei(gl_UNPACK_ROW_LENGTH, rowLength)

	glTexSubImage2D(gl_TEXTURE_2D, 0, x, y, width, height, uploadFormat, uploadType, data)

	glCheckError(fmt.Sprintf("SetSubData w:%d h:%d s:%d", width, height, stride))

	if rowLength != 0 {
		glPixelStorei(gl_UNPACK_ROW_LENGTH, 0)
	}

	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, interp)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, interp)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_S, gl_CLAMP_TO_EDGE)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_T, gl_CLAMP_TO_EDGE)
}

func (t *Texture_WebGL2) SetDataG(data []byte, mag, min, ws, wt TextureSamplingParam) {
	bits := Max(t.depth, 8)
	internalFormat := t.MapSizedInternalFormat(bits)
	uploadFormat := t.MapUploadFormat(bits)
	uploadType := t.MapUploadType(bits)

	r := gfx.(*Renderer_WebGL2)
	r.SetActiveTexture0()

	glBindTexture(gl_TEXTURE_2D, t.handle)
	glPixelStorei(gl_UNPACK_ALIGNMENT, 1)
	glPixelStorei(gl_UNPACK_ROW_LENGTH, 0)
	glTexImage2D(gl_TEXTURE_2D, 0, int32(internalFormat), t.width, t.height, uploadFormat, uploadType, data)
	glGenerateMipmap(gl_TEXTURE_2D)
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, t.MapTextureSamplingParam(mag))
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, t.MapTextureSamplingParam(min))
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_S, t.MapTextureSamplingParam(ws))
	glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_T, t.MapTextureSamplingParam(wt))
}

func (t *Texture_WebGL2) SetPixelData(data []float32) {
	r := gfx.(*Renderer_WebGL2)
	r.SetActiveTexture0()

	glBindTexture(gl_TEXTURE_2D, t.handle)
	glPixelStorei(gl_UNPACK_ALIGNMENT, 1)
	glPixelStorei(gl_UNPACK_ROW_LENGTH, 0)
	glTexImage2Df(gl_TEXTURE_2D, 0, gl_RGBA32F, t.width, t.height, gl_RGBA, data)
}

func (t Texture_WebGL2) CopyData(src *Texture) {
	r := gfx.(*Renderer_WebGL2)
	r.SetActiveTexture0()

	glBindTexture(gl_TEXTURE_2D, 0) // Unbind whatever is currently bound
	srcGL := (*src).(*Texture_WebGL2)
	fbo := glGenFramebuffer()
	glBindFramebuffer(gl_READ_FRAMEBUFFER, fbo)
	glFramebufferTexture2D(gl_READ_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, gl_TEXTURE_2D, srcGL.handle, 0)

	glBindTexture(gl_TEXTURE_2D, t.handle)
	// Copy the old texture data into the top-left of the new, larger texture
	glCopyTexSubImage2D(gl_TEXTURE_2D, 0, 0, 0, 0, 0, srcGL.width, srcGL.height)

	glBindFramebuffer(gl_READ_FRAMEBUFFER, 0)
	glDeleteFramebuffer(fbo)
}

// Return whether texture has a valid handle
func (t *Texture_WebGL2) IsValid() bool {
	return t.width != 0 && t.height != 0 && t.handle != 0
}

func (t *Texture_WebGL2) GetWidth() int32 {
	return t.width
}

func (t *Texture_WebGL2) GetHeight() int32 {
	return t.height
}

func (t *Texture_WebGL2) MapUploadType(i int32) uint32 {
	switch i {
	case 96, 128:
		return gl_FLOAT
	default:
		return gl_UNSIGNED_BYTE
	}
}

func (t *Texture_WebGL2) MapUploadFormat(i int32) uint32 {
	switch i {
	case 8:
		return gl_RED
	case 24:
		return gl_RGB
	case 32:
		return gl_RGBA
	case 96:
		return gl_RGB
	case 128:
		return gl_RGBA
	default:
		return gl_RGBA
	}
}

func (t *Texture_WebGL2) MapSizedInternalFormat(i int32) uint32 {
	switch i {
	case 8:
		return gl_R8
	case 24:
		return gl_RGB8
	case 32:
		return gl_RGBA8
	case 96:
		return gl_RGB32F
	case 128:
		return gl_RGBA32F
	default:
		return gl_RGBA8
	}
}

func (t *Texture_WebGL2) MapTextureSamplingParam(i TextureSamplingParam) int32 {
	switch i {
	case TextureSamplingFilterNearest:
		return gl_NEAREST
	case TextureSamplingFilterLinear:
		return gl_LINEAR
	case TextureSamplingFilterNearestMipMapNearest:
		return gl_NEAREST_MIPMAP_NEAREST
	case TextureSamplingFilterLinearMipMapNearest:
		return gl_LINEAR_MIPMAP_NEAREST
	case TextureSamplingFilterNearestMipMapLinear:
		return gl_NEAREST_MIPMAP_LINEAR
	case TextureSamplingFilterLinearMipMapLinear:
		return gl_LINEAR_MIPMAP_LINEAR
	case TextureSamplingWrapClampToEdge:
		return gl_CLAMP_TO_EDGE
	case TextureSamplingWrapMirroredRepeat:
		return gl_MIRRORED_REPEAT
	case TextureSamplingWrapRepeat:
		return gl_REPEAT
	default:
		return gl_NEAREST
	}
}

// ------------------------------------------------------------------
// Renderer_WebGL2

type Renderer_WebGL2 struct {
	fbo uint32
	// Single-sample color target
	fbo_texture uint32
	// Depth (and, with MSAA, color) renderbuffers
	rbo_depth uint32
	rbo_color uint32
	// MSAA resolve target
	fbo_f         uint32
	fbo_f_texture *Texture_WebGL2
	// Grab-pass resolve FBO (MSAA only)
	grabFbo uint32
	// Environment map FBO
	fbo_env uint32
	// Post-processing FBOs
	fbo_pp         []uint32
	fbo_pp_texture []uint32
	// Post-processing shaders
	postVertBuffer   uint32
	postShaderSelect []*ShaderProgram_WebGL2
	// Custom shaders
	customShaders   map[uint32]*ShaderProgram_WebGL2
	customShaderMap map[string]uint32
	nextShaderID    uint32
	currentProgram  *ShaderProgram_WebGL2
	grabTexture     *Texture_WebGL2
	// Shader and vertex data for primitive rendering
	spriteShader *ShaderProgram_WebGL2
	vertexBuffer uint32
	// Shader and index data for 3D model rendering
	modelShader             *ShaderProgram_WebGL2
	panoramaToCubeMapShader *ShaderProgram_WebGL2
	cubemapFilteringShader  *ShaderProgram_WebGL2
	modelVertexBuffer       [2]uint32
	modelIndexBuffer        [2]uint32
	spriteVAO               uint32
	modelVAO                uint32
	modelEnvVAO             uint32
	postVAO                 uint32

	enableModel bool
	debugMode   bool
	envWarned   bool
	WebGL2State
}

type WebGL2State struct {
	program             uint32
	depthTest           bool
	depthMask           bool
	invertFrontFace     bool
	doubleSided         bool
	blendEnabled        bool
	blendEquation       BlendEquation
	blendSrc            BlendFunc
	blendDst            BlendFunc
	scissorRect         [4]int32
	scissorEnabled      bool
	texCacheTexSerial   []uint64 // Unit to serial number. Sized per GPU
	texCacheLastUsed    []uint64 // Timer value when the slot was last used. Sized per GPU
	texCacheTimer       uint64   // Increments on every texture access
	uniformICache       map[uint32]int32
	uniformF1Cache      map[uint32]float32
	uniformF2Cache      map[uint32][2]float32
	uniformF3Cache      map[uint32][3]float32
	uniformF4Cache      map[uint32][4]float32
	useUV               bool
	useNormal           bool
	useTangent          bool
	useVertColor        bool
	useJoint0           bool
	useJoint1           bool
	useOutlineAttribute bool
}

func (r *Renderer_WebGL2) GetName() string {
	// The "OpenGL" prefix keeps system.go on the SwapBuffers/external-shader
	// code path (SPEC §1.1).
	return "OpenGL ES 3.0 (WebGL2)"
}

func (r *Renderer_WebGL2) DebugInfo() string {
	if !glCtx.Truthy() {
		return "WebGL2 (uninitialized)"
	}
	return fmt.Sprintf("WebGL2: %s (%s); MSAA=%d; EXT_color_buffer_float=%v; model=%v; shadow=false",
		glGetParameterString(gl_VERSION), glGetParameterString(gl_RENDERER),
		sys.msaa, glHasColorBufferFloat, r.enableModel)
}

// init 3D model shader
func (r *Renderer_WebGL2) InitModelShader() error {
	var err error
	// Always compiled WITHOUT ENABLE_SHADOW: the shadow path needs a
	// geometry shader and samplerCubeArray, neither of which exists in
	// WebGL2 (SPEC §6).
	r.modelShader, err = r.newShaderProgram(modelVertShader, modelFragShader, "", "Model Shader", false)
	if err != nil {
		return err
	}

	r.modelShader.RegisterAttributes("inVertexId", "position", "uv", "normalIn", "tangentIn", "vertColor", "joints_0", "joints_1", "weights_0", "weights_1", "outlineAttributeIn")

	r.modelShader.RegisterUniforms("model", "view", "projection", "normalMatrix", "unlit", "baseColorFactor", "add", "mult", "useTexture", "useNormalMap", "useMetallicRoughnessMap", "useEmissionMap", "neg", "gray", "hue",
		"enableAlpha", "alphaThreshold", "numJoints", "morphTargetWeight", "morphTargetOffset", "morphTargetTextureDimension", "numTargets", "numVertices",
		"metallicRoughness", "ambientOcclusionStrength", "emission", "environmentIntensity", "mipCount", "meshOutline",
		"cameraPosition", "environmentRotation", "texTransform", "normalMapTransform", "metallicRoughnessMapTransform", "ambientOcclusionMapTransform", "emissionMapTransform",
		"lightMatrices[0]", "lightMatrices[1]", "lightMatrices[2]", "lightMatrices[3]",
		"lights[0].direction", "lights[0].range", "lights[0].color", "lights[0].intensity", "lights[0].position", "lights[0].innerConeCos", "lights[0].outerConeCos", "lights[0].type", "lights[0].shadowBias", "lights[0].shadowMapFar",
		"lights[1].direction", "lights[1].range", "lights[1].color", "lights[1].intensity", "lights[1].position", "lights[1].innerConeCos", "lights[1].outerConeCos", "lights[1].type", "lights[1].shadowBias", "lights[1].shadowMapFar",
		"lights[2].direction", "lights[2].range", "lights[2].color", "lights[2].intensity", "lights[2].position", "lights[2].innerConeCos", "lights[2].outerConeCos", "lights[2].type", "lights[2].shadowBias", "lights[2].shadowMapFar",
		"lights[3].direction", "lights[3].range", "lights[3].color", "lights[3].intensity", "lights[3].position", "lights[3].innerConeCos", "lights[3].outerConeCos", "lights[3].type", "lights[3].shadowBias", "lights[3].shadowMapFar",
	)
	r.modelShader.RegisterTextures(
		"tex", "morphTargetValues", "jointMatrices",
		"normalMap", "metallicRoughnessMap", "ambientOcclusionMap", "emissionMap",
		"lambertianEnvSampler", "GGXEnvSampler", "GGXLUT",
	)

	r.panoramaToCubeMapShader, err = r.newShaderProgram(identVertShader, panoramaToCubeMapFragShader, "", "Panorama To Cubemap Shader", false)
	if err != nil {
		return err
	}
	r.panoramaToCubeMapShader.RegisterAttributes("VertCoord")
	r.panoramaToCubeMapShader.RegisterUniforms("currentFace")
	r.panoramaToCubeMapShader.RegisterTextures("panorama")

	r.cubemapFilteringShader, err = r.newShaderProgram(identVertShader, cubemapFilteringFragShader, "", "Cubemap Filtering Shader", false)
	if err != nil {
		return err
	}
	r.cubemapFilteringShader.RegisterAttributes("VertCoord")
	r.cubemapFilteringShader.RegisterUniforms("sampleCount", "distribution", "width", "currentFace", "roughness", "intensityScale", "isLUT")
	r.cubemapFilteringShader.RegisterTextures("cubeMap")

	// Configure modelEnvVAO
	glBindVertexArray(r.modelEnvVAO)
	glBindBuffer(gl_ARRAY_BUFFER, r.postVertBuffer)

	if loc, ok := r.cubemapFilteringShader.attributes["VertCoord"]; ok && loc >= 0 {
		glEnableVertexAttribArray(uint32(loc))
		glVertexAttribPointer(uint32(loc), 2, gl_FLOAT, false, 0, 0)
	}

	// Unbind for safety
	glBindVertexArray(0)

	return nil
}

// Render initialization.
// Creates the default shaders, the framebuffer and enables MSAA.
func (r *Renderer_WebGL2) Init() {
	chk(glInit())
	LogMessage("Using %v (%v)", glGetParameterString(gl_VERSION), glGetParameterString(gl_RENDERER))

	r.debugMode = sys.cfg.Video.RendererDebugMode
	glDebug = r.debugMode

	// Cap the configured MSAA sample count to the hardware limit.
	maxSamples := int32(glGetParameterInt(gl_MAX_SAMPLES))
	if sys.msaa > maxSamples {
		sys.cfg.SetValueUpdate("Video.MSAA", maxSamples)
		sys.msaa = maxSamples
	}

	r.customShaders = make(map[uint32]*ShaderProgram_WebGL2)
	r.customShaderMap = make(map[string]uint32)
	r.nextShaderID = 1
	r.currentProgram = nil

	// Data buffers for rendering
	postVertData := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)

	r.enableModel = sys.cfg.Video.EnableModel
	// Model shadows are unsupported on WebGL2 (SPEC §6); never enabled.

	// Generate VAO's
	r.spriteVAO = glGenVertexArray()
	r.modelVAO = glGenVertexArray()
	r.modelEnvVAO = glGenVertexArray()
	r.postVAO = glGenVertexArray()

	// Generate buffers
	r.vertexBuffer = glGenBuffer()
	r.modelVertexBuffer[0] = glGenBuffer()
	r.modelVertexBuffer[1] = glGenBuffer()
	r.modelIndexBuffer[0] = glGenBuffer()
	r.modelIndexBuffer[1] = glGenBuffer()
	r.postVertBuffer = glGenBuffer()

	// Initialize post-processing vertex buffer
	glBindBuffer(gl_ARRAY_BUFFER, r.postVertBuffer)
	glBufferData(gl_ARRAY_BUFFER, postVertData, gl_STATIC_DRAW)

	// Unbind for safety
	glBindBuffer(gl_ARRAY_BUFFER, 0)

	// Sprite shader
	r.spriteShader, _ = r.newShaderProgram(vertShader, fragShader, "", "Main Shader", true)
	r.spriteShader.RegisterAttributes("position", "uv")
	r.spriteShader.RegisterUniforms("modelview", "projection", "x1x2x4x3",
		"alpha", "tint", "mask", "neg", "gray", "add", "mult", "isFlat", "isRgba", "isTrapez", "hue")
	r.spriteShader.RegisterTextures("pal", "tex")

	// Configure spriteVAO
	glBindVertexArray(r.spriteVAO)
	glBindBuffer(gl_ARRAY_BUFFER, r.vertexBuffer)

	locPos := r.spriteShader.attributes["position"]
	glEnableVertexAttribArray(uint32(locPos))
	glVertexAttribPointer(uint32(locPos), 2, gl_FLOAT, false, 16, 0)

	locUV := r.spriteShader.attributes["uv"]
	glEnableVertexAttribArray(uint32(locUV))
	glVertexAttribPointer(uint32(locUV), 2, gl_FLOAT, false, 16, 8)

	// Unbind for safety
	glBindVertexArray(0)

	if r.enableModel {
		if err := r.InitModelShader(); err != nil {
			LogMessage("Model shader unavailable on WebGL2: %v", err)
			r.enableModel = false
		}
	}

	// Compile post-processing shaders

	// Pre-allocate the shader slice to accommodate all external shaders plus the identity shader
	r.postShaderSelect = make([]*ShaderProgram_WebGL2, len(sys.cfg.Video.ExternalShaders)+1)

	// Configure postVAO
	glBindVertexArray(r.postVAO)
	glBindBuffer(gl_ARRAY_BUFFER, r.postVertBuffer)

	// External Shaders
	for i := 0; i < len(sys.cfg.Video.ExternalShaders); i++ {
		r.postShaderSelect[i], _ = r.newShaderProgram(string(sys.externalShaders[0][i]), string(sys.externalShaders[1][i]),
			"", fmt.Sprintf("Postprocess Shader #%v", i), true)
		r.postShaderSelect[i].RegisterAttributes("VertCoord") // "TexCoord" was registered but never used
		r.postShaderSelect[i].RegisterUniforms("Texture", "TextureSize", "CurrentTime")

		// Configure postVAO for this specific shader's attribute location
		if loc, ok := r.postShaderSelect[i].attributes["VertCoord"]; ok && loc >= 0 {
			glEnableVertexAttribArray(uint32(loc))
			glVertexAttribPointer(uint32(loc), 2, gl_FLOAT, false, 0, 0)
		}
	}

	// Identity shader (no post-processing). This should be the last one in modern OpenGL
	identShader, _ := r.newShaderProgram(identVertShader, identFragShader, "", "Identity Postprocess", true)
	identShader.RegisterAttributes("VertCoord")

	// Configure postVAO for the identity shader's attribute location
	if loc, ok := identShader.attributes["VertCoord"]; ok && loc >= 0 {
		glEnableVertexAttribArray(uint32(loc))
		glVertexAttribPointer(uint32(loc), 2, gl_FLOAT, false, 0, 0)
	}

	// It should be the last one in modern OpenGL
	r.postShaderSelect[len(r.postShaderSelect)-1] = identShader

	// Unbind for safety
	glBindVertexArray(0)
	glBindBuffer(gl_ARRAY_BUFFER, 0)

	r.SetActiveTexture0()
	r.grabTexture = r.newTexture(sys.scrrect[2], sys.scrrect[3], 32, true).(*Texture_WebGL2)
	r.grabTexture.SetData(nil)

	if sys.msaa == 0 {
		// create a texture for r.fbo
		r.fbo_texture = glGenTexture()
		textureSerialNumber++

		glBindTexture(gl_TEXTURE_2D, r.fbo_texture)

		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, gl_NEAREST)
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, gl_NEAREST)
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_S, gl_CLAMP_TO_EDGE)
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_T, gl_CLAMP_TO_EDGE)

		// Don't change this from gl.RGBA.
		// It breaks mixing between subtractive and additive.
		glTexImage2D(
			gl_TEXTURE_2D,
			0,
			gl_RGBA,
			sys.scrrect[2],
			sys.scrrect[3],
			gl_RGBA,
			gl_UNSIGNED_BYTE,
			nil,
		)
	}

	r.fbo_pp = make([]uint32, 2)
	r.fbo_pp_texture = make([]uint32, 2)

	// The desktop/android backends use RGBA8_SNORM here so external shaders
	// can write signed pixels, but RGBA8_SNORM is not color-renderable in
	// WebGL2 (ES 3.0) — plain RGBA8 is used instead.
	for i := 0; i < 2; i++ {
		r.fbo_pp_texture[i] = glGenTexture()
		textureSerialNumber++

		glBindTexture(gl_TEXTURE_2D, r.fbo_pp_texture[i])
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, gl_NEAREST)
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, gl_NEAREST)
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_S, gl_CLAMP_TO_EDGE)
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_WRAP_T, gl_CLAMP_TO_EDGE)
		glTexImage2D(
			gl_TEXTURE_2D,
			0,
			gl_RGBA8,
			sys.scrrect[2],
			sys.scrrect[3],
			gl_RGBA,
			gl_UNSIGNED_BYTE,
			nil,
		)
	}

	// done with the FBO textures, unbind
	glBindTexture(gl_TEXTURE_2D, 0)

	r.rbo_depth = glGenRenderbuffer()
	glBindRenderbuffer(gl_RENDERBUFFER, r.rbo_depth)
	if sys.msaa > 0 {
		glRenderbufferStorageMultisample(gl_RENDERBUFFER, sys.msaa, gl_DEPTH_COMPONENT16, sys.scrrect[2], sys.scrrect[3])
	} else {
		glRenderbufferStorage(gl_RENDERBUFFER, gl_DEPTH_COMPONENT16, sys.scrrect[2], sys.scrrect[3])
	}

	if sys.msaa > 0 {
		// WebGL2 has no multisampled textures; the MSAA color target is a
		// renderbuffer, resolved to fbo_f_texture with BlitFramebuffer.
		r.rbo_color = glGenRenderbuffer()
		glBindRenderbuffer(gl_RENDERBUFFER, r.rbo_color)
		glRenderbufferStorageMultisample(gl_RENDERBUFFER, sys.msaa, gl_RGBA8, sys.scrrect[2], sys.scrrect[3])

		r.fbo_f_texture = r.newTexture(sys.scrrect[2], sys.scrrect[3], 32, false).(*Texture_WebGL2)
		r.fbo_f_texture.SetData(nil)
	}
	glBindRenderbuffer(gl_RENDERBUFFER, 0)

	// create an FBO for our r.fbo
	r.fbo = glGenFramebuffer()
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)

	if sys.msaa > 0 {
		glFramebufferRenderbuffer(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, gl_RENDERBUFFER, r.rbo_color)
		glFramebufferRenderbuffer(gl_FRAMEBUFFER, gl_DEPTH_ATTACHMENT, gl_RENDERBUFFER, r.rbo_depth)
		if status := glCheckFramebufferStatus(gl_FRAMEBUFFER); status != gl_FRAMEBUFFER_COMPLETE {
			LogMessage("Framebuffer creation failed: 0x%x", status)
		}
		r.fbo_f = glGenFramebuffer()
		glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_f)
		glFramebufferTexture2D(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, gl_TEXTURE_2D, r.fbo_f_texture.handle, 0)

		// Resolve FBO for ResolveBackBuffer (grab pass)
		r.grabFbo = glGenFramebuffer()
		glBindFramebuffer(gl_FRAMEBUFFER, r.grabFbo)
		glFramebufferTexture2D(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, gl_TEXTURE_2D, r.grabTexture.handle, 0)
	} else {
		glFramebufferTexture2D(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, gl_TEXTURE_2D, r.fbo_texture, 0)
		glFramebufferRenderbuffer(gl_FRAMEBUFFER, gl_DEPTH_ATTACHMENT, gl_RENDERBUFFER, r.rbo_depth)
	}

	// create our two FBOs for our post-processing needs
	for i := 0; i < 2; i++ {
		r.fbo_pp[i] = glGenFramebuffer()
		glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_pp[i])
		glFramebufferTexture2D(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, gl_TEXTURE_2D, r.fbo_pp_texture[i], 0)
	}

	// create an FBO for our model stuff (no shadow FBO on WebGL2)
	if r.enableModel {
		r.fbo_env = glGenFramebuffer()
	}

	glBindFramebuffer(gl_FRAMEBUFFER, 0)

	r.InitStateCache()
}

func (r *Renderer_WebGL2) Close() {
}

func (r *Renderer_WebGL2) InitStateCache() {
	// Match standard OpenGL hardware defaults
	r.program = 0
	r.depthTest = false
	r.depthMask = true
	r.doubleSided = true
	r.invertFrontFace = false
	r.blendEnabled = false
	r.scissorEnabled = false

	// Force hardware synchronization
	glUseProgram(0)
	glDisable(gl_DEPTH_TEST)
	glDepthMask(true)
	glDisable(gl_CULL_FACE)
	glFrontFace(gl_CCW)
	glDisable(gl_BLEND)
	glDisable(gl_SCISSOR_TEST)

	// Check hardware texture limit
	maxTex := int32(glGetParameterInt(gl_MAX_TEXTURE_IMAGE_UNITS))

	// Initialize sprite texture cache
	r.texCacheTexSerial = make([]uint64, maxTex)
	r.texCacheLastUsed = make([]uint64, maxTex)

	// Initialize uniform cache
	r.uniformICache = make(map[uint32]int32, 32)
	r.uniformF1Cache = make(map[uint32]float32, 32)
	r.uniformF2Cache = make(map[uint32][2]float32, 32)
	r.uniformF3Cache = make(map[uint32][3]float32, 32)
	r.uniformF4Cache = make(map[uint32][4]float32, 32)
}

func (r *Renderer_WebGL2) IsModelEnabled() bool {
	return r.enableModel
}

func (r *Renderer_WebGL2) IsShadowEnabled() bool {
	// Model shadows need geometry shaders + samplerCubeArray; not in WebGL2.
	return false
}

func (r *Renderer_WebGL2) BeginFrame(clearColor bool) {
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)
	glViewport(0, 0, sys.scrrect[2], sys.scrrect[3])
	if clearColor {
		glClear(gl_COLOR_BUFFER_BIT | gl_DEPTH_BUFFER_BIT)
	} else {
		glClear(gl_DEPTH_BUFFER_BIT)
	}
}

func (r *Renderer_WebGL2) EndFrame() {
	if len(r.fbo_pp) == 0 {
		return
	}

	x, y, width, height := int32(0), int32(0), int32(sys.scrrect[2]), int32(sys.scrrect[3])
	// consistent time across all shaders (seconds since renderer start)
	curTime := float32(time.Since(webgl2StartTime).Seconds())

	if sys.msaa > 0 {
		glBindFramebuffer(gl_DRAW_FRAMEBUFFER, r.fbo_f)
		glBindFramebuffer(gl_READ_FRAMEBUFFER, r.fbo)
		glBlitFramebuffer(x, y, width, height, x, y, width, height, gl_COLOR_BUFFER_BIT, gl_LINEAR)
	}

	var scaleMode int32 // GL enum
	if sys.cfg.Video.WindowScaleMode {
		scaleMode = gl_LINEAR
	} else {
		scaleMode = gl_NEAREST
	}

	// set the viewport to the unscaled bounds for post-processing
	glViewport(x, y, width, height)
	// clear both of our post-processing FBOs to make sure
	// nothing's there. the output is set later
	for i := 0; i < 2; i++ {
		glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_pp[i])
		glClear(gl_COLOR_BUFFER_BIT)
	}
	r.SetActiveTexture0()

	fbo_texture := r.fbo_texture
	if sys.msaa > 0 {
		fbo_texture = r.fbo_f_texture.handle
	}

	// Reset global state
	r.DisableScissor()
	r.DisableBlending()
	r.SetDepthTest(false)
	r.SetDepthMask(false)

	for i := 0; i < len(r.postShaderSelect); i++ {
		postShader := r.postShaderSelect[i]

		// tell GL we want to use our shader program
		r.ChangeProgram(postShader.program)

		// tell GL to use our vertex array object
		// this'll be where our quad is stored
		glBindVertexArray(r.postVAO)

		// this is here because it is undefined
		// behavior to write to the same FBO
		if i%2 == 0 {
			// ping! our first post-processing FBO is the output
			glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_pp[0])
			if i == 0 {
				// first pass, use fbo_texture
				glBindTexture(gl_TEXTURE_2D, fbo_texture)
			} else {
				// not the first pass, use the second post-processing FBO
				glBindTexture(gl_TEXTURE_2D, r.fbo_pp_texture[1])
			}
		} else {
			// pong! our second post-processing FBO is the output
			glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_pp[1])
			// our first post-processing FBO is the input
			glBindTexture(gl_TEXTURE_2D, r.fbo_pp_texture[0])
		}

		if i >= len(r.postShaderSelect)-1 {
			// this is the last shader,
			// so we ask GL to scale it and output it
			// to FB0, the default frame buffer that the user sees
			x, y, width, height := sys.window.GetScaledViewportSize()
			glViewport(x, y, width, height)
			glBindFramebuffer(gl_FRAMEBUFFER, 0)
			// clear FB0 just to make sure
			glClear(gl_COLOR_BUFFER_BIT | gl_DEPTH_BUFFER_BIT)
		}

		// set post-processing parameters
		if loc, ok := postShader.uniforms["Texture"]; ok && loc >= 0 {
			r.SetUniformISub(loc, 0)
		}
		if loc, ok := postShader.uniforms["TextureSize"]; ok && loc >= 0 {
			r.SetUniformFSub(loc, float32(width), float32(height))
		}
		if loc, ok := postShader.uniforms["CurrentTime"]; ok && loc >= 0 {
			r.SetUniformFSub(loc, curTime)
		}

		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MAG_FILTER, scaleMode)
		glTexParameteri(gl_TEXTURE_2D, gl_TEXTURE_MIN_FILTER, scaleMode)

		// construct the quad and draw it
		glDrawArrays(gl_TRIANGLE_STRIP, 0, 4)
	}

	glCheckError("EndFrame")
}

func (r *Renderer_WebGL2) Await() {
	// No-op: WebGL commands flush when control returns to the browser, and
	// gl.finish() stalls the pipeline for no benefit (SPEC §6).
}

var webgl2BlendEquationLUT = map[BlendEquation]uint32{
	BlendAdd:             gl_FUNC_ADD,
	BlendReverseSubtract: gl_FUNC_REVERSE_SUBTRACT,
}

func (r *Renderer_WebGL2) MapBlendEquation(i BlendEquation) uint32 {
	return webgl2BlendEquationLUT[i]
}

var webgl2BlendFunctionLUT = map[BlendFunc]uint32{
	BlendOne:              gl_ONE,
	BlendZero:             gl_ZERO,
	BlendSrcAlpha:         gl_SRC_ALPHA,
	BlendOneMinusSrcAlpha: gl_ONE_MINUS_SRC_ALPHA,
	BlendDstColor:         gl_DST_COLOR,
	BlendOneMinusDstColor: gl_ONE_MINUS_DST_COLOR,
}

func (r *Renderer_WebGL2) MapBlendFunction(i BlendFunc) uint32 {
	return webgl2BlendFunctionLUT[i]
}

var webgl2PrimitiveModeLUT = map[PrimitiveMode]uint32{
	LINES:          gl_LINES,
	LINE_LOOP:      gl_LINE_LOOP,
	LINE_STRIP:     gl_LINE_STRIP,
	TRIANGLES:      gl_TRIANGLES,
	TRIANGLE_STRIP: gl_TRIANGLE_STRIP,
	TRIANGLE_FAN:   gl_TRIANGLE_FAN,
}

func (r *Renderer_WebGL2) MapPrimitiveMode(i PrimitiveMode) uint32 {
	return webgl2PrimitiveModeLUT[i]
}

func (r *Renderer_WebGL2) SetDepthTest(depthTest bool) {
	if depthTest != r.depthTest {
		r.depthTest = depthTest
		if depthTest {
			glEnable(gl_DEPTH_TEST)
			glDepthFunc(gl_LESS)
		} else {
			glDisable(gl_DEPTH_TEST)
		}
	}
}

// Note: This one defaults to enable so we must sync the cache early
func (r *Renderer_WebGL2) SetDepthMask(depthMask bool) {
	if depthMask != r.depthMask {
		r.depthMask = depthMask
		glDepthMask(depthMask)
	}
}

func (r *Renderer_WebGL2) SetFrontFace(invertFrontFace bool) {
	if invertFrontFace != r.invertFrontFace {
		r.invertFrontFace = invertFrontFace
		if invertFrontFace {
			glFrontFace(gl_CW)
		} else {
			glFrontFace(gl_CCW)
		}
	}
}

func (r *Renderer_WebGL2) SetCullFace(doubleSided bool) {
	if doubleSided != r.doubleSided {
		r.doubleSided = doubleSided
		if !doubleSided {
			glEnable(gl_CULL_FACE)
			glCullFace(gl_BACK)
		} else {
			glDisable(gl_CULL_FACE)
		}
	}
}

// This should be called instead of gl.UseProgram()
func (r *Renderer_WebGL2) ChangeProgram(prog uint32) {
	// Program already in use
	if r.program == prog {
		return
	}

	// Lazy release of sprite pipeline
	// We can't tell if the next thing we will draw is also a sprite, so this prevents releasing the pipeline after every single sprite
	if r.spriteShader != nil && r.program == r.spriteShader.program {
		r.ReleasePipeline()
	}

	// Same for TTF fonts
	if fr, ok := gfxFont.(*FontRenderer_WebGL2); ok && fr.shaderProgram != nil && r.program == fr.shaderProgram.program {
		fr.ReleaseFontPipeline()
	}

	// Switch program
	glUseProgram(prog)
	r.program = prog

	// Reset sprite texture cache
	for i := range r.texCacheTexSerial {
		r.texCacheTexSerial[i] = 0
		r.texCacheLastUsed[i] = 0
	}
	r.texCacheTimer = 1
}

func (r *Renderer_WebGL2) EnableBlending(eq BlendEquation, src, dst BlendFunc) {
	if !r.blendEnabled {
		glEnable(gl_BLEND)
		r.blendEnabled = true
	}

	if eq != r.blendEquation {
		r.blendEquation = eq
		glBlendEquation(r.MapBlendEquation(eq))
	}

	if src != r.blendSrc || dst != r.blendDst {
		r.blendSrc = src
		r.blendDst = dst
		glBlendFunc(r.MapBlendFunction(src), r.MapBlendFunction(dst))
	}
}

func (r *Renderer_WebGL2) DisableBlending() {
	if r.blendEnabled {
		glDisable(gl_BLEND)
		r.blendEnabled = false
		// Do not update blend equation cache because the hardware doesn't
	}
}

func (r *Renderer_WebGL2) SetPipeline() {
	// Do nothing if we were already using the sprite shader
	if r.program == r.spriteShader.program {
		return
	}

	r.ChangeProgram(r.spriteShader.program)

	glBindVertexArray(r.spriteVAO)
}

func (r *Renderer_WebGL2) ReleasePipeline() {
	glBindVertexArray(0)
}

// ---- shadow pipeline: unsupported on WebGL2, all no-ops (SPEC §6) ----

func (r *Renderer_WebGL2) prepareShadowMapPipeline(bufferIndex uint32) {}

func (r *Renderer_WebGL2) setShadowMapPipeline(doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1 bool, numVertices, vertAttrOffset uint32) {
}

func (r *Renderer_WebGL2) ReleaseShadowPipeline() {}

func (r *Renderer_WebGL2) RenderShadowMapElements(mode PrimitiveMode, count, offset int) {}

func (r *Renderer_WebGL2) SetShadowMapUniformI(name string, val int) {}

func (r *Renderer_WebGL2) SetShadowMapUniformF(name string, values ...float32) {}

func (r *Renderer_WebGL2) SetShadowMapUniformFv(name string, values []float32) {}

func (r *Renderer_WebGL2) SetShadowMapUniformMatrix(name string, value []float32) {}

func (r *Renderer_WebGL2) SetShadowMapUniformMatrix3(name string, value []float32) {}

func (r *Renderer_WebGL2) SetShadowMapTexture(name string, tex Texture) {}

func (r *Renderer_WebGL2) SetShadowFrameTexture(i uint32) {}

func (r *Renderer_WebGL2) SetShadowFrameCubeTexture(i uint32) {}

// ---- model pipeline ----

func (r *Renderer_WebGL2) prepareModelPipeline(bufferIndex uint32, env *Environment) {
	r.ChangeProgram(r.modelShader.program)

	glBindVertexArray(r.modelVAO)
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)

	glViewport(0, 0, sys.scrrect[2], sys.scrrect[3])
	r.SetDepthMask(true)
	glClear(gl_DEPTH_BUFFER_BIT)
	// Set global state
	r.EnableBlending(r.blendEquation, r.blendSrc, r.blendDst)
	r.SetDepthTest(true)
	r.SetFrontFace(r.invertFrontFace)
	r.SetCullFace(r.doubleSided)

	glBindBuffer(gl_ARRAY_BUFFER, r.modelVertexBuffer[bufferIndex])
	glBindBuffer(gl_ELEMENT_ARRAY_BUFFER, r.modelIndexBuffer[bufferIndex])

	if env != nil {
		loc, unit := r.modelShader.uniforms["lambertianEnvSampler"], r.modelShader.textures["lambertianEnvSampler"]
		glActiveTexture(uint32(gl_TEXTURE0 + unit))
		glBindTexture(gl_TEXTURE_CUBE_MAP, env.lambertianTexture.tex.(*Texture_WebGL2).handle)
		glUniform1i(loc, int32(unit))
		loc, unit = r.modelShader.uniforms["GGXEnvSampler"], r.modelShader.textures["GGXEnvSampler"]
		glActiveTexture(uint32(gl_TEXTURE0 + unit))
		glBindTexture(gl_TEXTURE_CUBE_MAP, env.GGXTexture.tex.(*Texture_WebGL2).handle)
		glUniform1i(loc, int32(unit))
		loc, unit = r.modelShader.uniforms["GGXLUT"], r.modelShader.textures["GGXLUT"]
		glActiveTexture(uint32(gl_TEXTURE0 + unit))
		glBindTexture(gl_TEXTURE_2D, env.GGXLUT.tex.(*Texture_WebGL2).handle)
		glUniform1i(loc, int32(unit))

		loc = r.modelShader.uniforms["environmentIntensity"]
		glUniform1f(loc, env.environmentIntensity)
		loc = r.modelShader.uniforms["mipCount"]
		glUniform1i(loc, env.mipmapLevels)
		loc = r.modelShader.uniforms["environmentRotation"]
		rotationMatrix := mgl.Rotate3DX(math.Pi).Mul3(mgl.Rotate3DY(0.5 * math.Pi))
		glUniformMatrix3fv(loc, rotationMatrix[:])
	} else {
		loc, unit := r.modelShader.uniforms["lambertianEnvSampler"], r.modelShader.textures["lambertianEnvSampler"]
		glActiveTexture(uint32(gl_TEXTURE0 + unit))
		glBindTexture(gl_TEXTURE_CUBE_MAP, 0)
		glUniform1i(loc, int32(unit))
		loc, unit = r.modelShader.uniforms["GGXEnvSampler"], r.modelShader.textures["GGXEnvSampler"]
		glActiveTexture(uint32(gl_TEXTURE0 + unit))
		glBindTexture(gl_TEXTURE_CUBE_MAP, 0)
		glUniform1i(loc, int32(unit))
		loc, unit = r.modelShader.uniforms["GGXLUT"], r.modelShader.textures["GGXLUT"]
		glActiveTexture(uint32(gl_TEXTURE0 + unit))
		glBindTexture(gl_TEXTURE_2D, 0)
		glUniform1i(loc, int32(unit))
		loc = r.modelShader.uniforms["environmentIntensity"]
		glUniform1f(loc, 0)
	}

	r.SetActiveTexture0()
}

// enableModelAttrib/disableModelAttrib guard negative locations so a
// compiler-eliminated attribute cannot poison the GL error state.
func webgl2EnableAttrib(loc int32, size int, xtype uint32, offset uintptr) {
	if loc < 0 {
		return
	}
	glEnableVertexAttribArray(uint32(loc))
	glVertexAttribPointer(uint32(loc), size, xtype, false, 0, int(offset))
}

func webgl2DisableAttrib(loc int32, defaults ...float32) {
	if loc < 0 {
		return
	}
	glDisableVertexAttribArray(uint32(loc))
	switch len(defaults) {
	case 2:
		glVertexAttrib2f(uint32(loc), defaults[0], defaults[1])
	case 3:
		glVertexAttrib3f(uint32(loc), defaults[0], defaults[1], defaults[2])
	case 4:
		glVertexAttrib4f(uint32(loc), defaults[0], defaults[1], defaults[2], defaults[3])
	}
}

func (r *Renderer_WebGL2) SetModelPipeline(eq BlendEquation, src, dst BlendFunc, depthTest, depthMask, doubleSided, invertFrontFace,
	useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1, useOutlineAttribute bool, numVertices, vertAttrOffset uint32) {
	r.SetDepthTest(depthTest)
	r.SetDepthMask(depthMask)
	r.SetFrontFace(invertFrontFace)
	r.SetCullFace(doubleSided)
	r.EnableBlending(eq, src, dst)

	webgl2EnableAttrib(r.modelShader.attributes["inVertexId"], 1, gl_INT, uintptr(vertAttrOffset))
	offset := uintptr(vertAttrOffset) + 4*uintptr(numVertices)

	webgl2EnableAttrib(r.modelShader.attributes["position"], 3, gl_FLOAT, offset)
	offset += 12 * uintptr(numVertices)

	if useUV {
		r.useUV = true
		webgl2EnableAttrib(r.modelShader.attributes["uv"], 2, gl_FLOAT, offset)
		offset += 8 * uintptr(numVertices)
	} else if r.useUV {
		r.useUV = false
		webgl2DisableAttrib(r.modelShader.attributes["uv"], 0, 0)
	}

	if useNormal {
		r.useNormal = true
		webgl2EnableAttrib(r.modelShader.attributes["normalIn"], 3, gl_FLOAT, offset)
		offset += 12 * uintptr(numVertices)
	} else if r.useNormal {
		r.useNormal = false
		webgl2DisableAttrib(r.modelShader.attributes["normalIn"], 0, 0, 0)
	}
	if useTangent {
		r.useTangent = true
		webgl2EnableAttrib(r.modelShader.attributes["tangentIn"], 4, gl_FLOAT, offset)
		offset += 16 * uintptr(numVertices)
	} else if r.useTangent {
		r.useTangent = false
		webgl2DisableAttrib(r.modelShader.attributes["tangentIn"], 0, 0, 0, 0)
	}
	if useVertColor {
		r.useVertColor = true
		webgl2EnableAttrib(r.modelShader.attributes["vertColor"], 4, gl_FLOAT, offset)
		offset += 16 * uintptr(numVertices)
	} else if r.useVertColor {
		r.useVertColor = false
		webgl2DisableAttrib(r.modelShader.attributes["vertColor"], 1, 1, 1, 1)
	}
	if useJoint0 {
		r.useJoint0 = true
		webgl2EnableAttrib(r.modelShader.attributes["joints_0"], 4, gl_FLOAT, offset)
		offset += 16 * uintptr(numVertices)
		webgl2EnableAttrib(r.modelShader.attributes["weights_0"], 4, gl_FLOAT, offset)
		offset += 16 * uintptr(numVertices)
		if useJoint1 {
			r.useJoint1 = true
			webgl2EnableAttrib(r.modelShader.attributes["joints_1"], 4, gl_FLOAT, offset)
			offset += 16 * uintptr(numVertices)
			webgl2EnableAttrib(r.modelShader.attributes["weights_1"], 4, gl_FLOAT, offset)
			offset += 16 * uintptr(numVertices)
		} else if r.useJoint1 {
			r.useJoint1 = false
			webgl2DisableAttrib(r.modelShader.attributes["joints_1"], 0, 0, 0, 0)
			webgl2DisableAttrib(r.modelShader.attributes["weights_1"], 0, 0, 0, 0)
		}
	} else if r.useJoint0 {
		r.useJoint0 = false
		r.useJoint1 = false
		webgl2DisableAttrib(r.modelShader.attributes["joints_0"], 0, 0, 0, 0)
		webgl2DisableAttrib(r.modelShader.attributes["weights_0"], 0, 0, 0, 0)
		webgl2DisableAttrib(r.modelShader.attributes["joints_1"], 0, 0, 0, 0)
		webgl2DisableAttrib(r.modelShader.attributes["weights_1"], 0, 0, 0, 0)
	}
	if useOutlineAttribute {
		r.useOutlineAttribute = true
		webgl2EnableAttrib(r.modelShader.attributes["outlineAttributeIn"], 4, gl_FLOAT, offset)
		offset += 16 * uintptr(numVertices)
	} else if r.useOutlineAttribute {
		r.useOutlineAttribute = false
		webgl2DisableAttrib(r.modelShader.attributes["outlineAttributeIn"], 0, 0, 0, 0)
	}
}

func (r *Renderer_WebGL2) SetMeshOutlinePipeline(invertFrontFace bool, meshOutline float32) {
	r.SetFrontFace(invertFrontFace)
	r.SetDepthTest(true)
	r.SetDepthMask(true)

	glUniform1f(r.modelShader.uniforms["meshOutline"], meshOutline)
}

func (r *Renderer_WebGL2) ReleaseModelPipeline() {
	webgl2DisableAttrib(r.modelShader.attributes["inVertexId"])
	webgl2DisableAttrib(r.modelShader.attributes["position"])
	webgl2DisableAttrib(r.modelShader.attributes["uv"], 0, 0)
	webgl2DisableAttrib(r.modelShader.attributes["normalIn"], 0, 0, 0)
	webgl2DisableAttrib(r.modelShader.attributes["tangentIn"], 0, 0, 0, 0)
	webgl2DisableAttrib(r.modelShader.attributes["vertColor"], 1, 1, 1, 1)
	webgl2DisableAttrib(r.modelShader.attributes["joints_0"], 0, 0, 0, 0)
	webgl2DisableAttrib(r.modelShader.attributes["weights_0"], 0, 0, 0, 0)
	webgl2DisableAttrib(r.modelShader.attributes["joints_1"], 0, 0, 0, 0)
	webgl2DisableAttrib(r.modelShader.attributes["weights_1"], 0, 0, 0, 0)
	webgl2DisableAttrib(r.modelShader.attributes["outlineAttributeIn"], 0, 0, 0, 0)
	r.SetDepthMask(true)
	r.SetDepthTest(false)
	r.SetCullFace(true)
	r.useUV = false
	r.useNormal = false
	r.useTangent = false
	r.useVertColor = false
	r.useJoint0 = false
	r.useJoint1 = false
	r.useOutlineAttribute = false
}

func (r *Renderer_WebGL2) ReadPixels(data []uint8, width, height int) {
	// we defer the EndFrame(), SwapBuffers(), and BeginFrame() calls that were previously below now to
	// a single spot in order to prevent the blank screenshot bug on single digit FPS
	glBindFramebuffer(gl_READ_FRAMEBUFFER, 0)
	glReadPixels(0, 0, int32(width), int32(height), gl_RGBA, gl_UNSIGNED_BYTE, data)
}

func (r *Renderer_WebGL2) EnableScissor(x, y, width, height int32) {
	// Flip Y to OpenGL convention
	realY := sys.scrrect[3] - (y + height)

	if r.scissorEnabled &&
		r.scissorRect[0] == x && r.scissorRect[1] == realY &&
		r.scissorRect[2] == width && r.scissorRect[3] == height {
		return
	}

	if !r.scissorEnabled {
		glEnable(gl_SCISSOR_TEST)
		r.scissorEnabled = true
	}

	glScissor(x, realY, width, height)
	r.scissorRect = [4]int32{x, realY, width, height}
}

func (r *Renderer_WebGL2) DisableScissor() {
	if r.scissorEnabled {
		glDisable(gl_SCISSOR_TEST)
		r.scissorEnabled = false
		// Do not zero r.scissorRect here because the hardware retains the last rect even when the test is off
	}
}

func (r *Renderer_WebGL2) SetUniformISub(loc int32, val int32) {
	if loc < 0 {
		return
	}

	// Cached path for the sprite shader
	if r.spriteShader != nil && r.program == r.spriteShader.program {
		key := (r.program << 16) | uint32(loc)
		if old, exists := r.uniformICache[key]; exists && old == val {
			return
		}
		r.uniformICache[key] = val
	}

	glUniform1i(loc, val)
}

func (r *Renderer_WebGL2) SetUniformFSub(loc int32, values ...float32) {
	if loc < 0 || len(values) == 0 {
		return
	}

	// Cached path for the sprite shader
	if r.spriteShader != nil && r.program == r.spriteShader.program {
		key := (r.program << 16) | uint32(loc)

		switch len(values) {
		case 1:
			if old, exists := r.uniformF1Cache[key]; exists && old == values[0] {
				return
			}
			r.uniformF1Cache[key] = values[0]
		case 2:
			v2 := [2]float32{values[0], values[1]}
			if old, exists := r.uniformF2Cache[key]; exists && old == v2 {
				return
			}
			r.uniformF2Cache[key] = v2
		case 3:
			v3 := [3]float32{values[0], values[1], values[2]}
			if old, exists := r.uniformF3Cache[key]; exists && old == v3 {
				return
			}
			r.uniformF3Cache[key] = v3
		case 4:
			v4 := [4]float32{values[0], values[1], values[2], values[3]}
			if old, exists := r.uniformF4Cache[key]; exists && old == v4 {
				return
			}
			r.uniformF4Cache[key] = v4
		}
	}

	// Uncached path
	switch len(values) {
	case 1:
		glUniform1f(loc, values[0])
	case 2:
		glUniform2f(loc, values[0], values[1])
	case 3:
		glUniform3f(loc, values[0], values[1], values[2])
	case 4:
		glUniform4f(loc, values[0], values[1], values[2], values[3])
	}
}

func (r *Renderer_WebGL2) SetUniformFvSub(loc int32, values []float32) {
	if loc < 0 || len(values) == 0 {
		return
	}

	switch len(values) {
	case 1, 2, 3, 4:
		r.SetUniformFSub(loc, values...)
	case 8:
		glUniform4fv(loc, values)
	default:
		glUniform1fv(loc, values)
	}
}

func (r *Renderer_WebGL2) SetUniformI(name string, val int) {
	loc := r.currentProgram.uniforms[name]
	r.SetUniformISub(loc, int32(val))
}

func (r *Renderer_WebGL2) SetUniformF(name string, values ...float32) {
	loc := r.currentProgram.uniforms[name]
	r.SetUniformFSub(loc, values...)
}

func (r *Renderer_WebGL2) SetUniformFv(name string, values []float32) {
	loc := r.currentProgram.uniforms[name]
	r.SetUniformFvSub(loc, values)
}

// Caching matrices is as expensive as direct function calls
func (r *Renderer_WebGL2) SetUniformMatrix(name string, value []float32) {
	loc, ok := r.currentProgram.uniforms[name]
	if ok && loc >= 0 {
		glUniformMatrix4fv(loc, value)
	}
}

func (r *Renderer_WebGL2) SetModelUniformI(name string, val int) {
	loc, ok := r.modelShader.uniforms[name]
	if !ok || loc < 0 {
		return
	}
	r.SetUniformISub(loc, int32(val))
}

func (r *Renderer_WebGL2) SetModelUniformF(name string, values ...float32) {
	loc, ok := r.modelShader.uniforms[name]
	if !ok || loc < 0 {
		return
	}
	r.SetUniformFSub(loc, values...)
}

func (r *Renderer_WebGL2) SetModelUniformFv(name string, values []float32) {
	loc, ok := r.modelShader.uniforms[name]
	if !ok || loc < 0 {
		return
	}
	r.SetUniformFvSub(loc, values)
}

func (r *Renderer_WebGL2) SetModelUniformMatrix(name string, value []float32) {
	loc, ok := r.modelShader.uniforms[name]
	if !ok || loc < 0 {
		return
	}
	glUniformMatrix4fv(loc, value)
}

func (r *Renderer_WebGL2) SetModelUniformMatrix3(name string, value []float32) {
	loc, ok := r.modelShader.uniforms[name]
	if !ok || loc < 0 {
		return
	}
	glUniformMatrix3fv(loc, value)
}

// Selects texture unit 0 as active and tells the cache it's dirty
// Prevents the sprite renderer from desyncing during texture maintenance
func (r *Renderer_WebGL2) SetActiveTexture0() {
	glActiveTexture(gl_TEXTURE0)

	if len(r.texCacheTexSerial) > 0 {
		r.texCacheTexSerial[0] = 0
		r.texCacheLastUsed[0] = 0
	}
}

func (r *Renderer_WebGL2) SetTextureSub(uMap map[string]int32, tMap map[string]int, name string, tex Texture) {
	t := tex.(*Texture_WebGL2)
	loc := uMap[name]

	// Cached path for the sprite shader
	// Note: The cache doesn't care if a texture is "tex" or "pal"
	if r.spriteShader != nil && r.program == r.spriteShader.program {
		// Increment reference timer
		r.texCacheTimer++

		var oldestUnit int32 = 0
		var minTime uint64 = math.MaxUint64

		// Look for a hit or the oldest slot
		for i := range r.texCacheTexSerial {
			// If we find the texture already bound, that's a hit
			if r.texCacheTexSerial[i] == t.serial {
				r.texCacheLastUsed[i] = r.texCacheTimer
				r.SetUniformISub(loc, int32(i))
				return
			}

			// While searching, track the oldest slot in case we miss
			if r.texCacheLastUsed[i] < minTime {
				minTime = r.texCacheLastUsed[i]
				oldestUnit = int32(i)
			}
		}

		// Cache miss
		glActiveTexture(gl_TEXTURE0 + uint32(oldestUnit))
		glBindTexture(gl_TEXTURE_2D, t.handle)

		// Update cache state
		r.texCacheTexSerial[oldestUnit] = t.serial
		r.texCacheLastUsed[oldestUnit] = r.texCacheTimer

		// Update uniform
		r.SetUniformISub(loc, oldestUnit)
		return
	}

	// Uncached path
	fixedUnit := uint32(tMap[name])
	glActiveTexture(gl_TEXTURE0 + fixedUnit)
	glBindTexture(gl_TEXTURE_2D, t.handle)
	r.SetUniformISub(loc, int32(fixedUnit))
}

func (r *Renderer_WebGL2) SetTexture(name string, tex Texture) {
	r.SetTextureSub(r.currentProgram.uniforms, r.currentProgram.textures, name, tex)
}

func (r *Renderer_WebGL2) SetModelTexture(name string, tex Texture) {
	r.SetTextureSub(r.modelShader.uniforms, r.modelShader.textures, name, tex)
}

func (r *Renderer_WebGL2) SetVertexData(values ...float32) {
	data := f32.Bytes(binary.LittleEndian, values...)
	glBindBuffer(gl_ARRAY_BUFFER, r.vertexBuffer)
	glBufferData(gl_ARRAY_BUFFER, data, gl_STATIC_DRAW)
}

func (r *Renderer_WebGL2) SetModelVertexData(bufferIndex uint32, values []byte) {
	glBindBuffer(gl_ARRAY_BUFFER, r.modelVertexBuffer[bufferIndex])
	glBufferData(gl_ARRAY_BUFFER, values, gl_STATIC_DRAW)
}

func (r *Renderer_WebGL2) SetModelIndexData(bufferIndex uint32, values ...uint32) {
	data := new(bytes.Buffer)
	binary.Write(data, binary.LittleEndian, values)

	glBindBuffer(gl_ELEMENT_ARRAY_BUFFER, r.modelIndexBuffer[bufferIndex])
	glBufferData(gl_ELEMENT_ARRAY_BUFFER, data.Bytes(), gl_STATIC_DRAW)
}

func (r *Renderer_WebGL2) RenderQuad() {
	glDrawArrays(gl_TRIANGLE_STRIP, 0, 4)
}

func (r *Renderer_WebGL2) RenderElements(mode PrimitiveMode, count, offset int) {
	glDrawElements(r.MapPrimitiveMode(mode), int32(count), gl_UNSIGNED_INT, offset)
}

// warnNoFloatFBO logs (once) that a float render-target path was skipped.
func (r *Renderer_WebGL2) warnNoFloatFBO(what string) bool {
	if glHasColorBufferFloat {
		return false
	}
	if !r.envWarned {
		r.envWarned = true
		LogMessage("WebGL2: EXT_color_buffer_float unavailable; skipping %s (stage environment lighting disabled)", what)
	}
	return true
}

func (r *Renderer_WebGL2) RenderCubeMap(envTex Texture, cubeTex Texture) {
	if r.warnNoFloatFBO("RenderCubeMap") {
		return
	}
	envTexture := envTex.(*Texture_WebGL2)
	cubeTexture := cubeTex.(*Texture_WebGL2)
	textureSize := cubeTexture.width

	r.ChangeProgram(r.panoramaToCubeMapShader.program)

	glBindVertexArray(r.modelEnvVAO)
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_env)
	glViewport(0, 0, textureSize, textureSize)

	data := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)
	glBindBuffer(gl_ARRAY_BUFFER, r.vertexBuffer)
	glBufferData(gl_ARRAY_BUFFER, data, gl_STATIC_DRAW)

	loc, unit := r.panoramaToCubeMapShader.uniforms["panorama"], r.panoramaToCubeMapShader.textures["panorama"]
	glActiveTexture(uint32(gl_TEXTURE0 + unit))
	glBindTexture(gl_TEXTURE_2D, envTexture.handle)
	glUniform1i(loc, int32(unit))

	for i := 0; i < 6; i++ {
		glFramebufferTexture2D(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, uint32(gl_TEXTURE_CUBE_MAP_POSITIVE_X+i), cubeTexture.handle, 0)

		glClear(gl_COLOR_BUFFER_BIT)
		glUniform1i(r.panoramaToCubeMapShader.uniforms["currentFace"], int32(i))

		glDrawArrays(gl_TRIANGLE_STRIP, 0, 4)
	}

	glBindVertexArray(0)
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)
	glBindTexture(gl_TEXTURE_CUBE_MAP, cubeTexture.handle)
	glGenerateMipmap(gl_TEXTURE_CUBE_MAP)
}

func (r *Renderer_WebGL2) RenderFilteredCubeMap(distribution int32, cubeTex Texture, filteredTex Texture, mipmapLevel, sampleCount int32, roughness float32) {
	if r.warnNoFloatFBO("RenderFilteredCubeMap") {
		return
	}
	cubeTexture := cubeTex.(*Texture_WebGL2)
	filteredTexture := filteredTex.(*Texture_WebGL2)
	textureSize := filteredTexture.width
	currentTextureSize := textureSize >> mipmapLevel

	r.ChangeProgram(r.cubemapFilteringShader.program)

	glBindVertexArray(r.modelEnvVAO)
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_env)
	glViewport(0, 0, currentTextureSize, currentTextureSize)

	data := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)
	glBindBuffer(gl_ARRAY_BUFFER, r.vertexBuffer)
	glBufferData(gl_ARRAY_BUFFER, data, gl_STATIC_DRAW)

	loc, unit := r.cubemapFilteringShader.uniforms["cubeMap"], r.cubemapFilteringShader.textures["cubeMap"]
	glActiveTexture(uint32(gl_TEXTURE0 + unit))
	glBindTexture(gl_TEXTURE_CUBE_MAP, cubeTexture.handle)
	glUniform1i(loc, int32(unit))
	glUniform1i(r.cubemapFilteringShader.uniforms["sampleCount"], sampleCount)
	glUniform1i(r.cubemapFilteringShader.uniforms["distribution"], distribution)
	glUniform1i(r.cubemapFilteringShader.uniforms["width"], textureSize)
	glUniform1f(r.cubemapFilteringShader.uniforms["roughness"], roughness)
	glUniform1f(r.cubemapFilteringShader.uniforms["intensityScale"], 1)
	glUniform1i(r.cubemapFilteringShader.uniforms["isLUT"], 0)

	for i := 0; i < 6; i++ {
		glFramebufferTexture2D(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, uint32(gl_TEXTURE_CUBE_MAP_POSITIVE_X+i), filteredTexture.handle, mipmapLevel)

		glClear(gl_COLOR_BUFFER_BIT)
		glUniform1i(r.cubemapFilteringShader.uniforms["currentFace"], int32(i))

		glDrawArrays(gl_TRIANGLE_STRIP, 0, 4)
	}

	glBindVertexArray(0)
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)
}

func (r *Renderer_WebGL2) RenderLUT(distribution int32, cubeTex Texture, lutTex Texture, sampleCount int32) {
	if r.warnNoFloatFBO("RenderLUT") {
		return
	}
	cubeTexture := cubeTex.(*Texture_WebGL2)
	lutTexture := lutTex.(*Texture_WebGL2)
	textureSize := lutTexture.width

	r.ChangeProgram(r.cubemapFilteringShader.program)

	glBindVertexArray(r.modelEnvVAO)
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo_env)
	glViewport(0, 0, textureSize, textureSize)

	data := f32.Bytes(binary.LittleEndian, -1, -1, 1, -1, -1, 1, 1, 1)
	glBindBuffer(gl_ARRAY_BUFFER, r.vertexBuffer)
	glBufferData(gl_ARRAY_BUFFER, data, gl_STATIC_DRAW)

	loc, unit := r.cubemapFilteringShader.uniforms["cubeMap"], r.cubemapFilteringShader.textures["cubeMap"]
	glActiveTexture(uint32(gl_TEXTURE0 + unit))
	glBindTexture(gl_TEXTURE_CUBE_MAP, cubeTexture.handle)
	glUniform1i(loc, int32(unit))
	glUniform1i(r.cubemapFilteringShader.uniforms["sampleCount"], sampleCount)
	glUniform1i(r.cubemapFilteringShader.uniforms["distribution"], distribution)
	glUniform1i(r.cubemapFilteringShader.uniforms["width"], textureSize)
	glUniform1f(r.cubemapFilteringShader.uniforms["roughness"], 0)
	glUniform1f(r.cubemapFilteringShader.uniforms["intensityScale"], 1)
	glUniform1i(r.cubemapFilteringShader.uniforms["currentFace"], 0)
	glUniform1i(r.cubemapFilteringShader.uniforms["isLUT"], 1)

	glBindTexture(gl_TEXTURE_2D, lutTexture.handle)
	glTexImage2D(gl_TEXTURE_2D, 0, gl_RGBA16F, lutTexture.width, lutTexture.height, gl_RGBA, gl_HALF_FLOAT, nil)

	glFramebufferTexture2D(gl_FRAMEBUFFER, gl_COLOR_ATTACHMENT0, gl_TEXTURE_2D, lutTexture.handle, 0)
	glClear(gl_COLOR_BUFFER_BIT)
	glDrawArrays(gl_TRIANGLE_STRIP, 0, 4)

	glBindVertexArray(0)
	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)
}

func (r *Renderer_WebGL2) PerspectiveProjectionMatrix(angle, aspect, near, far float32) mgl.Mat4 {
	return mgl.Perspective(angle, aspect, near, far)
}

func (r *Renderer_WebGL2) OrthographicProjectionMatrix(left, right, bottom, top, near, far float32) mgl.Mat4 {
	return mgl.Ortho(left, right, bottom, top, near, far)
}

func (r *Renderer_WebGL2) NewWorkerThread() bool {
	// wasm is single-threaded; model textures load synchronously.
	return false
}

func (r *Renderer_WebGL2) SetVSync(interval int) {
	// No-op: requestAnimationFrame paces presentation in the browser.
}

func (r *Renderer_WebGL2) LoadCustomSpriteShader(shaderName string, shaderData []byte) uint32 {
	if id, ok := r.customShaderMap[shaderName]; ok {
		return id
	}

	fragSource := string(shaderData)

	shader, err := r.newShaderProgram(vertShader, fragSource, "", "Custom Shader: "+shaderName, false)
	if err != nil {
		Logcat(fmt.Sprintf("[WebGL2 Error] Failed to compile custom shader %s: %v", shaderName, err))
		return 0
	}

	shader.RegisterAttributes("position", "uv")
	shader.RegisterUniforms("modelview", "projection", "x1x2x4x3",
		"alpha", "tint", "mask", "neg", "gray", "add", "mult", "isFlat", "isRgba", "isTrapez", "hue",
		"iTime", "iResolution", "aspectRatio", "sTime",
		// Custom parameters: registered up front so SetCustomUniforms never
		// has to intern uniform locations per call (js.Value lookups are
		// slow and getUniformLocation returns a fresh object every time).
		"p0", "p1", "p2", "p3", "p4", "p5", "p6", "p7",
		"p8", "p9", "p10", "p11", "p12", "p13", "p14", "p15")
	shader.RegisterTextures("pal", "tex", "tex1", "tex2", "bgl_RenderedTexture")

	shader.needsGrabPass = strings.Contains(fragSource, "bgl_RenderedTexture")

	id := r.nextShaderID
	r.nextShaderID++
	r.customShaders[id] = shader
	r.customShaderMap[shaderName] = id

	sys.appendToConsole(fmt.Sprintf("Loaded Custom Shader: %s (ID: %d, NeedsGrabPass: %v)", shaderName, id, shader.needsGrabPass))
	return id
}

func (r *Renderer_WebGL2) UnloadCustomSpriteShader(shaderName string) {
	if id, exists := r.customShaderMap[shaderName]; exists {
		if shader, hasProg := r.customShaders[id]; hasProg {
			glDeleteProgram(shader.program)
			delete(r.customShaders, id)
			if r.currentProgram == shader {
				r.currentProgram = nil
			}
		}
		delete(r.customShaderMap, shaderName)
	}
}

func (r *Renderer_WebGL2) SetSpritePipeline(shaderName string) {
	targetShader := r.spriteShader
	if shaderName != "" {
		if id, ok := r.customShaderMap[shaderName]; ok {
			if shader, ok := r.customShaders[id]; ok {
				targetShader = shader
			}
		}
	}

	if r.program != targetShader.program {
		r.currentProgram = targetShader
		r.ChangeProgram(targetShader.program)
		glBindVertexArray(r.spriteVAO)
	}
}

func (r *Renderer_WebGL2) SetCustomUniforms(params [16]float32) {
	if r.currentProgram == nil {
		return
	}
	for i := 0; i < 16; i++ {
		if loc, ok := r.currentProgram.uniforms[fmt.Sprintf("p%d", i)]; ok && loc >= 0 {
			glUniform1f(loc, params[i])
		}
	}
}

func (r *Renderer_WebGL2) NeedsGrabPass() bool {
	if r.currentProgram != nil {
		return r.currentProgram.needsGrabPass
	}
	return false
}

func (r *Renderer_WebGL2) ResolveBackBuffer() Texture {
	if sys.msaa > 0 {
		// The MSAA color target is a renderbuffer; resolve it with a blit.
		glBindFramebuffer(gl_READ_FRAMEBUFFER, r.fbo)
		glBindFramebuffer(gl_DRAW_FRAMEBUFFER, r.grabFbo)

		glBlitFramebuffer(0, 0, sys.scrrect[2], sys.scrrect[3], 0, 0, sys.scrrect[2], sys.scrrect[3], gl_COLOR_BUFFER_BIT, gl_NEAREST)

		glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)
		return r.grabTexture
	}

	r.SetActiveTexture0()
	glBindTexture(gl_TEXTURE_2D, r.grabTexture.handle)

	glBindFramebuffer(gl_READ_FRAMEBUFFER, r.fbo)
	glReadBuffer(gl_COLOR_ATTACHMENT0)

	glCopyTexSubImage2D(gl_TEXTURE_2D, 0, 0, 0, 0, 0, r.grabTexture.width, r.grabTexture.height)

	glBindFramebuffer(gl_FRAMEBUFFER, r.fbo)
	return r.grabTexture
}

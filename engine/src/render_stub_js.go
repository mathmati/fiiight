//go:build js

// TEMPORARY compile stub — replaced by render_webgl2.go.
// RendererStub/FontRendererStub/FontStub implement the full Renderer,
// FontRenderer and Font interfaces as no-ops so the js build links while the
// real WebGL2 backend is developed. GetName keeps the "OpenGL" prefix that
// system.go's present/shader-loading paths key off of.
package main

import (
	mgl "github.com/go-gl/mathgl/mgl32"
)

// ---------- Texture ----------

type TextureStub struct {
	width  int32
	height int32
}

func (t *TextureStub) SetData(data []byte)                                         {}
func (t *TextureStub) SetSubData(data []byte, x, y, width, height, stride int32)   {}
func (t *TextureStub) SetDataG(data []byte, mag, min, ws, wt TextureSamplingParam) {}
func (t *TextureStub) SetPixelData(data []float32)                                 {}
func (t *TextureStub) IsValid() bool                                               { return t.width > 0 && t.height > 0 }
func (t *TextureStub) GetWidth() int32                                             { return t.width }
func (t *TextureStub) GetHeight() int32                                            { return t.height }
func (t *TextureStub) CopyData(src *Texture)                                       {}

// ---------- Renderer ----------

type RendererStub struct{}

func (r *RendererStub) GetName() string {
	return "OpenGL ES 3.0 (WebGL2)"
}

func (r *RendererStub) DebugInfo() string { return "" }

func (r *RendererStub) Init()                      {}
func (r *RendererStub) Close()                     {}
func (r *RendererStub) BeginFrame(clearColor bool) {}
func (r *RendererStub) EndFrame()                  {}
func (r *RendererStub) Await()                     {}

func (r *RendererStub) IsModelEnabled() bool  { return false }
func (r *RendererStub) IsShadowEnabled() bool { return false }

func (r *RendererStub) LoadCustomSpriteShader(shaderName string, shaderData []byte) uint32 {
	return 0
}
func (r *RendererStub) UnloadCustomSpriteShader(shaderName string) {}
func (r *RendererStub) SetSpritePipeline(shaderName string)        {}
func (r *RendererStub) SetCustomUniforms(params [16]float32)       {}
func (r *RendererStub) NeedsGrabPass() bool                        { return false }
func (r *RendererStub) ResolveBackBuffer() Texture                 { return nil }

func (r *RendererStub) EnableBlending(eq BlendEquation, src, dst BlendFunc) {}
func (r *RendererStub) DisableBlending()                                    {}

func (r *RendererStub) prepareShadowMapPipeline(bufferIndex uint32) {}
func (r *RendererStub) setShadowMapPipeline(doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1 bool, numVertices, vertAttrOffset uint32) {
}
func (r *RendererStub) ReleaseShadowPipeline()                                    {}
func (r *RendererStub) prepareModelPipeline(bufferIndex uint32, env *Environment) {}
func (r *RendererStub) SetModelPipeline(eq BlendEquation, src, dst BlendFunc, depthTest, depthMask, doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1, useOutlineAttribute bool, numVertices, vertAttrOffset uint32) {
}
func (r *RendererStub) SetMeshOutlinePipeline(invertFrontFace bool, meshOutline float32) {}
func (r *RendererStub) ReleaseModelPipeline()                                            {}

func (r *RendererStub) newTexture(width, height, depth int32, filter bool) Texture {
	return &TextureStub{width: width, height: height}
}
func (r *RendererStub) newPaletteTexture() Texture {
	return &TextureStub{width: 256, height: 1}
}
func (r *RendererStub) newModelTexture(width, height, depth int32, filter bool) Texture {
	return &TextureStub{width: width, height: height}
}
func (r *RendererStub) newDataTexture(width, height int32) Texture {
	return &TextureStub{width: width, height: height}
}
func (r *RendererStub) newHDRTexture(width, height int32) Texture {
	return &TextureStub{width: width, height: height}
}
func (r *RendererStub) newCubeMapTexture(widthHeight int32, mipmap bool, lowestMipLevel int32) Texture {
	return &TextureStub{width: widthHeight, height: widthHeight}
}

func (r *RendererStub) ReadPixels(data []uint8, width, height int) {
	for i := range data {
		data[i] = 0
	}
}
func (r *RendererStub) EnableScissor(x, y, width, height int32) {}
func (r *RendererStub) DisableScissor()                         {}

func (r *RendererStub) SetUniformI(name string, val int)                        {}
func (r *RendererStub) SetUniformF(name string, values ...float32)              {}
func (r *RendererStub) SetUniformFv(name string, values []float32)              {}
func (r *RendererStub) SetUniformMatrix(name string, value []float32)           {}
func (r *RendererStub) SetTexture(name string, tex Texture)                     {}
func (r *RendererStub) SetModelUniformI(name string, val int)                   {}
func (r *RendererStub) SetModelUniformF(name string, values ...float32)         {}
func (r *RendererStub) SetModelUniformFv(name string, values []float32)         {}
func (r *RendererStub) SetModelUniformMatrix(name string, value []float32)      {}
func (r *RendererStub) SetModelUniformMatrix3(name string, value []float32)     {}
func (r *RendererStub) SetModelTexture(name string, t Texture)                  {}
func (r *RendererStub) SetShadowMapUniformI(name string, val int)               {}
func (r *RendererStub) SetShadowMapUniformF(name string, values ...float32)     {}
func (r *RendererStub) SetShadowMapUniformFv(name string, values []float32)     {}
func (r *RendererStub) SetShadowMapUniformMatrix(name string, value []float32)  {}
func (r *RendererStub) SetShadowMapUniformMatrix3(name string, value []float32) {}
func (r *RendererStub) SetShadowMapTexture(name string, t Texture)              {}
func (r *RendererStub) SetShadowFrameTexture(i uint32)                          {}
func (r *RendererStub) SetShadowFrameCubeTexture(i uint32)                      {}
func (r *RendererStub) SetVertexData(values ...float32)                         {}
func (r *RendererStub) SetModelVertexData(bufferIndex uint32, values []byte)    {}
func (r *RendererStub) SetModelIndexData(bufferIndex uint32, values ...uint32)  {}

func (r *RendererStub) RenderQuad()                                          {}
func (r *RendererStub) RenderElements(mode PrimitiveMode, count, offset int) {}
func (r *RendererStub) RenderShadowMapElements(mode PrimitiveMode, count, offset int) {
}
func (r *RendererStub) RenderCubeMap(envTexture Texture, cubeTexture Texture) {}
func (r *RendererStub) RenderFilteredCubeMap(distribution int32, cubeTexture Texture, filteredTexture Texture, mipmapLevel, sampleCount int32, roughness float32) {
}
func (r *RendererStub) RenderLUT(distribution int32, cubeTexture Texture, lutTexture Texture, sampleCount int32) {
}

func (r *RendererStub) PerspectiveProjectionMatrix(angle, aspect, near, far float32) mgl.Mat4 {
	return mgl.Perspective(angle, aspect, near, far)
}

func (r *RendererStub) OrthographicProjectionMatrix(left, right, bottom, top, near, far float32) mgl.Mat4 {
	return mgl.Ortho(left, right, bottom, top, near, far)
}

func (r *RendererStub) SetVSync(interval int) {}
func (r *RendererStub) NewWorkerThread() bool { return false }

// ---------- FontRenderer / Font ----------

type FontRendererStub struct{}

func (fr *FontRendererStub) Init(renderer interface{}) {}

func (fr *FontRendererStub) LoadFont(file string, scale int32, windowWidth int, windowHeight int) (interface{}, error) {
	return &FontStub{}, nil
}

type FontStub struct{}

func (f *FontStub) SetColor(red float32, green float32, blue float32, alpha float32) {}
func (f *FontStub) SetPalFX(spfx ShaderPalFX)                                        {}
func (f *FontStub) UpdateResolution(windowWidth int, windowHeight int)               {}
func (f *FontStub) Printf(x, y float32, xscl, yscl float32, spacingXAdd float32, align int32, blend bool, window [4]int32,
	rxadd float32, rot Rotation, projectionMode int32, fLength float32, rcx, rcy float32,
	fs string, argv ...interface{}) error {
	return nil
}
func (f *FontStub) Width(scale float32, spacingXAdd float32, fs string, argv ...interface{}) float32 {
	return 0
}

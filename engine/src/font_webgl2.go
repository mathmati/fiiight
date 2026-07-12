//go:build js

// TrueType font rendering for the WebGL2 (js/wasm) backend. Port of
// font_gles32.go: glyph rasterization (golang/freetype) is pure Go and kept
// as-is; only the texture upload and draw calls go through the syscall/js
// WebGL2 binding. FontRenderer_WebGL2 and Renderer_WebGL2 form a coupled
// pair like the other backends (the renderer's ChangeProgram releases the
// font pipeline through a type assertion on gfxFont).
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	colour "image/color"
	"io"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/mobile/exp/f32"
)

type Font_WebGL2 struct {
	fontChar     map[rune]*character
	ttf          *truetype.Font
	scale        int32
	windowWidth  int
	windowHeight int
	textures     []*TextureAtlas
	color        color
	shaderPalFX  ShaderPalFX
}

type FontRenderer_WebGL2 struct {
	shaderProgram *ShaderProgram_WebGL2
	vao           uint32
	vbo           uint32
}

// Init sets up shaders, VAO, and VBO
func (r *FontRenderer_WebGL2) Init(renderer interface{}) {
	// Configure the font shader (compiled with the "#version 300 es" header)
	r.newProgram(300, vertexFontShader, fragmentFontShader)

	// Register attributes
	r.shaderProgram.RegisterAttributes("vert", "vertTexCoord")

	// Configure VAO/VBO for texture quads
	r.vao = glGenVertexArray()
	r.vbo = glGenBuffer()
	glBindVertexArray(r.vao)
	glBindBuffer(gl_ARRAY_BUFFER, r.vbo)

	// Pre-allocate for maximum batch size
	glBufferDataSize(gl_ARRAY_BUFFER, MaxFontBatchSize*6*4*4, gl_DYNAMIC_DRAW)

	// Configure attributes
	vLoc := uint32(r.shaderProgram.attributes["vert"])
	glEnableVertexAttribArray(vLoc)
	glVertexAttribPointer(vLoc, 2, gl_FLOAT, false, 4*4, 0)

	tLoc := uint32(r.shaderProgram.attributes["vertTexCoord"])
	glEnableVertexAttribArray(tLoc)
	glVertexAttribPointer(tLoc, 2, gl_FLOAT, false, 4*4, 2*4)

	// Unbind for safety
	glBindVertexArray(0)
	glBindBuffer(gl_ARRAY_BUFFER, 0)
}

// LoadFont builds the font structure
func (r *FontRenderer_WebGL2) LoadFont(file string, scale int32, windowWidth int, windowHeight int) (interface{}, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	f, err := r.LoadTrueTypeFont(fd, scale, 32, 127, LeftToRight)
	if err != nil {
		return nil, err
	}
	f.windowWidth = windowWidth
	f.windowHeight = windowHeight
	return f, nil
}

// SetColor allows you to set the text color to be used when you draw the text
func (f *Font_WebGL2) SetColor(red float32, green float32, blue float32, alpha float32) {
	f.color.r = red
	f.color.g = green
	f.color.b = blue
	f.color.a = alpha
}

func (f *Font_WebGL2) SetPalFX(state ShaderPalFX) {
	f.shaderPalFX = state
}

func (f *Font_WebGL2) UpdateResolution(windowWidth int, windowHeight int) {
	f.windowWidth = windowWidth
	f.windowHeight = windowHeight
}

func (r *FontRenderer_WebGL2) SetFontPipeline() {
	mr := gfx.(*Renderer_WebGL2)

	// Do nothing if we were already using the font shader
	if mr.program == r.shaderProgram.program {
		return
	}

	mr.ChangeProgram(r.shaderProgram.program)

	// Bind VAO
	glBindVertexArray(r.vao)

	// We need to bind VBO here as well
	glBindBuffer(gl_ARRAY_BUFFER, r.vbo)
}

// Printf draws a string to the screen, takes a list of arguments like printf
func (f *Font_WebGL2) Printf(x, y float32, xscl, yscl float32, spacingXAdd float32,
	align int32, blend bool, window [4]int32,
	rxadd float32, rot Rotation, projectionMode int32, fLength float32, rcx, rcy float32,
	fs string, argv ...interface{}) error {

	text := fmt.Sprintf(fs, argv...)
	indices := []rune(text)
	r := gfx.(*Renderer_WebGL2)
	fr := gfxFont.(*FontRenderer_WebGL2)

	if len(indices) == 0 {
		return nil
	}

	// Buffer to store vertex data for multiple glyphs
	batchSize := Min(MaxFontBatchSize, int32(len(indices)))
	batchVertices := make([]float32, 0, batchSize*6*4)

	// Activate corresponding render state
	fr.SetFontPipeline()
	program := fr.shaderProgram

	//setup blending mode
	if blend {
		r.EnableBlending(BlendAdd, BlendSrcAlpha, BlendOneMinusSrcAlpha)
	} else {
		r.DisableBlending()
	}

	//restrict drawing to a certain part of the window
	r.EnableScissor(window[0], window[1], window[2], window[3])

	// Set texture location
	r.SetUniformISub(program.uniforms["tex"], 0)

	//set text color
	r.SetUniformFSub(program.uniforms["textColor"], f.color.r, f.color.g, f.color.b, f.color.a)

	// Set PalFX uniforms
	r.SetUniformFSub(program.uniforms["palAdd"], f.shaderPalFX.add[0], f.shaderPalFX.add[1], f.shaderPalFX.add[2])
	r.SetUniformFSub(program.uniforms["palMul"], f.shaderPalFX.mult[0], f.shaderPalFX.mult[1], f.shaderPalFX.mult[2])
	r.SetUniformFSub(program.uniforms["palGray"], f.shaderPalFX.gray)
	r.SetUniformFSub(program.uniforms["palHue"], f.shaderPalFX.hue)
	r.SetUniformISub(program.uniforms["palNeg"], int32(Btoi(f.shaderPalFX.neg)))

	//set screen resolution
	r.SetUniformFSub(program.uniforms["resolution"], float32(f.windowWidth), float32(f.windowHeight))

	glActiveTexture(gl_TEXTURE0)

	//calculate alignment position
	alignScale := xscl
	if alignScale == 0 {
		alignScale = yscl
	}
	if align == 0 {
		x -= f.widthRunes(indices, alignScale, spacingXAdd) * 0.5
	} else if align < 0 {
		x -= f.widthRunes(indices, alignScale, spacingXAdd)
	}
	needsTransform := rxadd != 0 || !rot.IsZero()
	textureID := int32(-1)
	spacing := spacingXAdd * xscl
	renderedAny := false
	// Iterate through all characters in string
	for i := range indices {
		//get rune
		runeIndex := indices[i]

		//find rune in fontChar list
		ch, ok := f.fontChar[runeIndex]

		//load missing runes in batches of 32
		if !ok {
			low := runeIndex - (runeIndex % 32)
			f.GenerateGlyphs(low, low+31)
			ch, ok = f.fontChar[runeIndex]
		}

		//skip runes that are not in font character range
		if !ok {
			continue
		}

		if int32(len(batchVertices)/24) >= batchSize || (textureID != -1 && textureID != int32(ch.textureID)) {
			// Render the current batch
			f.renderGlyphBatch(batchVertices, uint32(textureID))
			// Clear the batch buffers
			batchVertices = batchVertices[:0]
		}
		textureID = int32(ch.textureID)

		if renderedAny {
			x += spacing
		}

		//calculate position and size for current rune
		xpos := x + float32(ch.bearingH)*xscl
		ypos := y - float32(ch.height-ch.bearingV)*yscl
		w := float32(ch.width) * xscl
		h := float32(ch.height) * yscl

		x1, y1 := xpos+w, ypos
		x2, y2 := xpos, ypos
		x3, y3 := xpos, ypos+h
		x4, y4 := xpos+w, ypos+h
		if needsTransform {
			x1, y1, x2, y2, x3, y3, x4, y4 = transformTextQuad(
				x1, y1, x2, y2, x3, y3, x4, y4,
				rxadd, rot, projectionMode, fLength, rcx, rcy,
			)
		}

		batchVertices = append(batchVertices,
			x1, y1, ch.uv[2], ch.uv[1],
			x2, y2, ch.uv[0], ch.uv[1],
			x3, y3, ch.uv[0], ch.uv[3],

			x3, y3, ch.uv[0], ch.uv[3],
			x4, y4, ch.uv[2], ch.uv[3],
			x1, y1, ch.uv[2], ch.uv[1],
		)
		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		x += float32((ch.advance >> 6)) * xscl // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))
		renderedAny = true
	}

	// Render any remaining glyphs in the batch
	if len(batchVertices) > 0 {
		f.renderGlyphBatch(batchVertices, uint32(textureID))
	}

	// Disable scissor just in case
	r.DisableScissor()

	return nil
}

func (f *Font_WebGL2) widthRunes(indices []rune, scale float32, spacingXAdd float32) float32 {
	if len(indices) == 0 {
		return 0
	}

	spacing := spacingXAdd * scale
	var width float32
	renderedAny := false

	// Iterate through all characters in string
	for i := range indices {

		//get rune
		runeIndex := indices[i]

		//find rune in fontChar list
		ch, ok := f.fontChar[runeIndex]

		//load missing runes in batches of 32
		if !ok {
			low := runeIndex - (runeIndex % 32)
			f.GenerateGlyphs(low, low+31)
			ch, ok = f.fontChar[runeIndex]
		}

		//skip runes that are not in font character range
		if !ok {
			continue
		}

		if renderedAny {
			width += spacing
		}

		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		width += float32(ch.advance>>6) * scale // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))
		renderedAny = true
	}

	return width
}

// Helper function to render a batch of glyphs
func (f *Font_WebGL2) renderGlyphBatch(vertices []float32, textureID uint32) {
	fr := gfxFont.(*FontRenderer_WebGL2)

	glBindBuffer(gl_ARRAY_BUFFER, fr.vbo)
	glBufferSubData(gl_ARRAY_BUFFER, 0, f32.Bytes(binary.LittleEndian, vertices...))

	glBindTexture(gl_TEXTURE_2D, textureID)
	glDrawArrays(gl_TRIANGLES, 0, int32(len(vertices))/4)
}

func (r *FontRenderer_WebGL2) ReleaseFontPipeline() {
	glBindVertexArray(0)
	glBindBuffer(gl_ARRAY_BUFFER, 0)
}

// Width returns the width of a piece of text in pixels
func (f *Font_WebGL2) Width(scale float32, spacingXAdd float32, fs string, argv ...interface{}) float32 {
	return f.widthRunes([]rune(fmt.Sprintf(fs, argv...)), scale, spacingXAdd)
}

// GenerateGlyphs builds a set of textures based on a ttf files glyphs
func (f *Font_WebGL2) GenerateGlyphs(low, high rune) error {
	//create a freetype context for drawing
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f.ttf)
	c.SetFontSize(float64(f.scale))
	c.SetHinting(font.HintingFull)

	//create new face to measure glyph dimensions
	ttfFace := truetype.NewFace(f.ttf, &truetype.Options{
		Size:    float64(f.scale),
		DPI:     72,
		Hinting: font.HintingFull,
	})

	// Add padding to prevent cropping
	// https://github.com/ikemen-engine/Ikemen-GO/issues/3085
	padding := 2

	//make each glyph
	for ch := low; ch <= high; ch++ {
		char := new(character)

		drawGlyph := true
		gBnd, gAdv, ok := ttfFace.GlyphBounds(ch)
		if !ok {
			// Some fonts do not provide bounds for every requested rune. This is common for control chars.
			// Keep loading the font and create an invisible placeholder so future lookups do not retry this glyph forever.
			drawGlyph = false
			if adv, advOK := ttfFace.GlyphAdvance(ch); advOK {
				gAdv = adv
			} else {
				gAdv = 0
			}
			gBnd = f.ttf.Bounds(fixed.Int26_6(f.scale))
		}

		gh := int32((gBnd.Max.Y - gBnd.Min.Y) >> 6)
		gw := int32((gBnd.Max.X - gBnd.Min.X) >> 6)

		//if glyph has no dimensions set to a max value
		if gw == 0 || gh == 0 {
			gBnd = f.ttf.Bounds(fixed.Int26_6(f.scale))
			gw = int32((gBnd.Max.X - gBnd.Min.X) >> 6)
			gh = int32((gBnd.Max.Y - gBnd.Min.Y) >> 6)

			//above can sometimes yield 0 for font smaller than 48pt, 1 is minimum
			if gw == 0 || gh == 0 {
				gw = 1
				gh = 1
			}
		}

		//The glyph's ascent and descent equal -bounds.Min.Y and +bounds.Max.Y.
		gAscent := int(-gBnd.Min.Y) >> 6
		gdescent := int(gBnd.Max.Y) >> 6

		//set w,h and adv, bearing V and bearing H in char
		char.width = int(gw) + (padding * 2)
		char.height = int(gh) + (padding * 2)
		char.advance = int(gAdv)
		char.bearingV = gdescent
		char.bearingH = (int(gBnd.Min.X) >> 6) - padding

		//create image to draw glyph
		fg := image.NewUniform(colour.RGBA{255, 255, 255, 255})
		rect := image.Rect(0, 0, char.width, char.height)
		rgba := image.NewRGBA(rect)

		//set the glyph dot
		px := padding - (int(gBnd.Min.X) >> 6)
		py := padding + gAscent
		pt := freetype.Pt(px, py)

		// Draw the text from mask to image
		c.SetClip(rgba.Bounds())
		c.SetDst(rgba)
		c.SetSrc(fg)
		if drawGlyph {
			if _, err := c.DrawString(string(ch), pt); err != nil {
				// Some fonts have broken hinting bytecode. Keep full hinting for normal fonts.
				// Retry glyph without hinting instead of failing the whole font load.
				c2 := freetype.NewContext()
				c2.SetDPI(72)
				c2.SetFont(f.ttf)
				c2.SetFontSize(float64(f.scale))
				c2.SetHinting(font.HintingNone)
				c2.SetClip(rgba.Bounds())
				c2.SetDst(rgba)
				c2.SetSrc(fg)
				if _, err := c2.DrawString(string(ch), pt); err != nil {
					Logcat(fmt.Sprintf("WebGL2: ERROR DRAWING STRING: %v", err.Error()))
					return err
				}
			}
		}

		var uv [4]float32
		textureIndex := 0
		w, h := int32(rgba.Rect.Dx()), int32(rgba.Rect.Dy())
		pix := rgba.Pix
		stride := int32(rgba.Stride) // This was added to unify desktop and Android

		for {
			if textureIndex >= len(f.textures) {
				f.textures = append(f.textures, CreateTextureAtlas(256, 256, 32, true))
			}

			var inserted bool
			uv, inserted = f.textures[textureIndex].AddImage(w, h, stride, pix)

			if inserted {
				break
			}

			textureIndex++
		}

		texAtlas := f.textures[textureIndex]

		char.uv = uv
		char.textureID = texAtlas.texture.(*Texture_WebGL2).handle

		//add char to fontChar list
		f.fontChar[ch] = char
	}

	glBindTexture(gl_TEXTURE_2D, 0)
	return nil
}

// LoadTrueTypeFont builds GL buffers and glyph textures based on a ttf file
func (r *FontRenderer_WebGL2) LoadTrueTypeFont(reader io.Reader, scale int32, low, high rune, dir Direction) (*Font_WebGL2, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Read the truetype font.
	ttf, err := truetype.Parse(data)
	if err != nil && err.Error() == "freetype: invalid TrueType format: bad kern table length" {
		ttf, err = truetype.Parse(stripKernTable(data))
	}
	if err != nil {
		return nil, err
	}

	//make Font stuct type
	f := new(Font_WebGL2)
	f.fontChar = make(map[rune]*character)
	f.ttf = ttf
	f.scale = scale
	f.SetColor(1.0, 1.0, 1.0, 1.0) //set default white
	f.SetPalFX(NewShaderPalFX())
	f.textures = append(f.textures, CreateTextureAtlas(256, 256, 32, true))

	err = f.GenerateGlyphs(low, high)
	if err != nil {
		Logcat(fmt.Sprintf("Error generating glyphs: %v", err.Error()))
		return nil, err
	}

	return f, nil
}

// newProgram links the frag and vertex shader programs
func (r *FontRenderer_WebGL2) newProgram(version uint, vertexSrc, fragmentSrc string) {
	var err error
	if r.shaderProgram, err = gfx.(*Renderer_WebGL2).newShaderProgram(vertexSrc, fragmentSrc, "", "font shader", true); err != nil {
		Logcat(fmt.Sprintf("Error loading font shader: %v", err.Error()))
	}
	r.shaderProgram.RegisterUniforms("textColor", "resolution", "tex", "palAdd", "palMul", "palGray", "palHue", "palNeg")
}

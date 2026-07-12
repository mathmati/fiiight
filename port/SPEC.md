# Ikemen GO → js/wasm PORT SPEC

Target: `GOOS=js GOARCH=wasm`, browser, WebGL2, WebAudio, no cgo.
Source: `/home/user/fiiight/engine` (vendored from ikemen-engine/Ikemen-GO develop @ 05b7d98a, go 1.20).
All paths below are relative to `engine/` unless absolute.

---

## 1. Platform seam inventory

The engine has NO formal "platform" package. The seams are: (a) two Go interfaces (`Renderer`, `Texture`, `FontRenderer`, `Font`, `AudioSink`), (b) a set of **concrete types and free functions that core code references by name** and that are today only defined in SDL/GL/platform files. A js backend must supply same-named types/functions (duck-typed seam, not interface-typed).

### 1.1 Windowing / system

Seam = concrete `*Window` stored in `System.window` (`system.go:215`) plus `(s *System) newWindow(w, h int) (*Window, error)`. Currently defined only in `src/system_sdl.go` (no build tag, imports go-sdl2). Methods/fields the rest of the codebase uses (callers: system.go, motif.go, script.go, render_vk.go):

```go
func (s *System) newWindow(w, h int) (*Window, error)
func (w *Window) SwapBuffers()
func (w *Window) SetIcon(icon []image.Image)
func (w *Window) SetSwapInterval(interval int)
func (w *Window) GetSize() (int, int)
func (w *Window) GetScaledViewportSize() (int32, int32, int32, int32)
func (w *Window) GetClipboardString() string
func (w *Window) toggleFullscreen()
func (w *Window) UpdateDebugFPS()
func (w *Window) pollEvents()
func (w *Window) shouldClose() bool
func (w *Window) Close()
// fields accessed directly: w.fullscreen (system.go), w.closeflag (internal)
```

Additionally, because `Window` embeds `*sdl.Window`, these promoted methods are called:
- `system.go:394-397`: `s.window.GLCreateContext() (ctx, error)` and `s.window.GLMakeCurrent(ctx)` — only inside `if strings.HasPrefix(renderName, "OpenGL")`. The js Window must provide compilable equivalents (any `(T, error)` / `(T)` pair; can be no-ops if the js renderer name does not start with "OpenGL", but the code must still compile).
- `render_vk.go:5092 GetWMInfo()`, `render_vk.go:1530 VulkanCreateSurface()` — Vulkan-only, excluded on js.

Renderer/present dispatch (relevant to naming the js renderer): `system.go:790` `if gfx.GetName()[:6] == "OpenGL" { s.window.SwapBuffers() } else { gfx.Await() }` and `motif.go:2811` / `system.go:393,429` `strings.HasPrefix(gfx.GetName(), "OpenGL")`. **Decision point:** name the js renderer `"OpenGL ES 3.0 (WebGL2)"` (prefix "OpenGL") so the SwapBuffers path is taken and external-shader loading (`system.go:429`, loads `.vert`/`.frag`) works; implement present/yield inside `Window.SwapBuffers`.

Keyboard event delivery into the core: `pollEvents` must call (defined in `src/input.go`, portable):
```go
func OnKeyPressed(key Key, mk ModifierKey)
func OnKeyReleased(key Key, mk ModifierKey)
func OnTextEntered(s string)
```

### 1.2 Rendering

`src/render.go` (portable, no cgo) defines the two real interfaces. Verbatim (`render.go:11-98`):

```go
type Texture interface {
	SetData(data []byte)
	SetSubData(data []byte, x, y, width, height, stride int32)
	SetDataG(data []byte, mag, min, ws, wt TextureSamplingParam)
	SetPixelData(data []float32)
	IsValid() bool
	GetWidth() int32
	GetHeight() int32
	CopyData(src *Texture)
}

type Renderer interface {
	GetName() string
	DebugInfo() string
	Init()
	Close()
	BeginFrame(clearColor bool)
	EndFrame()
	Await()

	IsModelEnabled() bool
	IsShadowEnabled() bool

	//SetPipeline()
	LoadCustomSpriteShader(shaderName string, shaderData []byte) uint32
	UnloadCustomSpriteShader(shaderName string)
	SetSpritePipeline(shaderName string)
	SetCustomUniforms(params [16]float32)
	NeedsGrabPass() bool
	ResolveBackBuffer() Texture

	EnableBlending(eq BlendEquation, src, dst BlendFunc)
	DisableBlending()

	prepareShadowMapPipeline(bufferIndex uint32)
	setShadowMapPipeline(doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1 bool, numVertices, vertAttrOffset uint32)
	ReleaseShadowPipeline()
	prepareModelPipeline(bufferIndex uint32, env *Environment)
	SetModelPipeline(eq BlendEquation, src, dst BlendFunc, depthTest, depthMask, doubleSided, invertFrontFace, useUV, useNormal, useTangent, useVertColor, useJoint0, useJoint1, useOutlineAttribute bool, numVertices, vertAttrOffset uint32)
	SetMeshOutlinePipeline(invertFrontFace bool, meshOutline float32)
	ReleaseModelPipeline()
	newTexture(width, height, depth int32, filter bool) (t Texture)
	newPaletteTexture() (t Texture)
	newModelTexture(width, height, depth int32, filter bool) (t Texture)
	newDataTexture(width, height int32) (t Texture)
	newHDRTexture(width, height int32) (t Texture)
	newCubeMapTexture(widthHeight int32, mipmap bool, lowestMipLevel int32) (t Texture)

	ReadPixels(data []uint8, width, height int)
	EnableScissor(x, y, width, height int32)
	DisableScissor()

	SetUniformI(name string, val int)
	SetUniformF(name string, values ...float32)
	SetUniformFv(name string, values []float32)
	SetUniformMatrix(name string, value []float32)
	SetTexture(name string, tex Texture)
	SetModelUniformI(name string, val int)
	SetModelUniformF(name string, values ...float32)
	SetModelUniformFv(name string, values []float32)
	SetModelUniformMatrix(name string, value []float32)
	SetModelUniformMatrix3(name string, value []float32)
	SetModelTexture(name string, t Texture)
	SetShadowMapUniformI(name string, val int)
	SetShadowMapUniformF(name string, values ...float32)
	SetShadowMapUniformFv(name string, values []float32)
	SetShadowMapUniformMatrix(name string, value []float32)
	SetShadowMapUniformMatrix3(name string, value []float32)
	SetShadowMapTexture(name string, t Texture)
	SetShadowFrameTexture(i uint32)
	SetShadowFrameCubeTexture(i uint32)
	SetVertexData(values ...float32)
	SetModelVertexData(bufferIndex uint32, values []byte)
	SetModelIndexData(bufferIndex uint32, values ...uint32)

	RenderQuad()
	RenderElements(mode PrimitiveMode, count, offset int)
	RenderShadowMapElements(mode PrimitiveMode, count, offset int)
	RenderCubeMap(envTexture Texture, cubeTexture Texture)
	RenderFilteredCubeMap(distribution int32, cubeTexture Texture, filteredTexture Texture, mipmapLevel, sampleCount int32, roughness float32)
	RenderLUT(distribution int32, cubeTexture Texture, lutTexture Texture, sampleCount int32)

	PerspectiveProjectionMatrix(angle, aspect, near, far float32) mgl.Mat4
	OrthographicProjectionMatrix(left, right, bottom, top, near, far float32) mgl.Mat4

	SetVSync(interval int)
	NewWorkerThread() bool
}
```

(`mgl` = `github.com/go-gl/mathgl/mgl32`, pure Go. `PrimitiveMode` is `type PrimitiveMode byte` at `model.go:268`; `Environment` at `model.go:332`; both portable.)

Globals to populate: `var gfx Renderer; var gfxFont FontRenderer` (`render.go:140-141`), assigned in `system.go:380` via `selectRenderer` (platform file, see 1.8).

Font seam (`src/font.go:14-29,65-73`, portable):

```go
type FontRenderer interface {
	Init(renderer interface{})
	LoadFont(file string, scale int32, windowWidth int, windowHeight int) (interface{}, error)
}

type Font interface {
	SetColor(red float32, green float32, blue float32, alpha float32)
	SetPalFX(spfx ShaderPalFX)
	UpdateResolution(windowWidth int, windowHeight int)
	Printf(x, y float32, xscl, yscl float32, spacingXAdd float32, align int32, blend bool, window [4]int32,
		rxadd float32, rot Rotation, projectionMode int32, fLength float32, rcx, rcy float32,
		fs string, argv ...interface{}) error
	Width(scale float32, spacingXAdd float32, fs string, argv ...interface{}) float32
}

type TtfFont interface {   // identical method set to Font; Fnt.ttf field type
	SetColor(red float32, green float32, blue float32, alpha float32)
	SetPalFX(spfx ShaderPalFX)
	Width(scale float32, spacingXAdd float32, fs string, argv ...interface{}) float32
	Printf(x, y float32, xscl, yscl float32, spacingXAdd float32, align int32, blend bool, window [4]int32,
		rxadd float32, rot Rotation, projectionMode int32, fLength float32, rcx, rcy float32,
		fs string, argv ...interface{}) error
	UpdateResolution(windowWidth int, windowHeight int)
}
```

Note: `render_gl33.go:1274` shows the renderer and font renderer are a coupled pair (`gfxFont.(*FontRenderer_GL33)` type assertion inside the renderer). The js pair may cross-reference each other the same way. TTF glyph rasterization is pure Go (`golang/freetype`, `font_gl33.go:477 LoadTrueTypeFont`) — port that code, only the GL upload/draw calls change.

### 1.3 Input

Seam = everything in `src/input_sdl.go` (no build tag) plus two small sdl references inside `src/input.go` (portable otherwise). Required same-named definitions for a js backend:

```go
type Key = <backend keycode type>          // currently: type Key = sdl.Keycode  (input_sdl.go:25)
type ModifierKey = <backend mod type>      // currently: type ModifierKey = sdl.Keymod
const (
	KeyUnknown, KeyEscape, KeyEnter, KeyInsert, KeyF5, KeyF12, KeyPause, KeyScrollLock  // input_sdl.go:28-37
)
var KeyToStringLUT map[Key]string
var StringToKeyLUT map[string]Key
var ButtonToStringLUT map[int]string
var buttonOrder []<button type>
var StringToButtonLUT map[string]int
var input Input

type ControllerState struct {              // input_sdl.go:14 — .Axes accessed directly from input.go:491
	Axes      [6]int8
	Buttons   map[<button type>]byte
	HasRumble bool
}
type Input struct {
	controllers     [MaxPlayerNo]<handle>
	controllerstate [MaxPlayerNo]*ControllerState   // accessed from input.go LocalAnalogInput (input.go:484-491)
}

func initLUTs()                                            // called from main.go:161
func StringToKey(s string) Key
func KeyToString(k Key) string
func NewModifierKey(ctrl, alt, shift bool) (mod ModifierKey)
func (input *Input) UpdateGamepadMappings(path string)
func (input *Input) GetMaxJoystickCount() int
func (input *Input) IsJoystickPresent(joy int) bool
func (input *Input) GetJoystickName(joy int) string
func (input *Input) GetJoystickAxes(joy int) [6]float32
func (input *Input) GetJoystickButtons(joy int) []byte
func (input *Input) GetJoystickPath(joy int) string
func (input *Input) GetJoystickGUID(joy int) string
func (input *Input) RumbleController(joy int, lo, hi uint16, ticks uint32)
func CheckAxisForDpad(axes *[6]float32, base int) string
func CheckAxisForTrigger(axes *[6]float32) string
func getJoystickKey(controllerIdx int) (string, int)
```

Key string names matter: config/save files store keys as the strings in `KeyToStringLUT` ("RETURN", "ESCAPE", "SPACE", "a".."z", "F1".."F24", "KP_1", "LCTRL", ...) and buttons as "A","B","X","Y","BACK","HOME","START","LS","RS","LB","RB","DP_U/D/L/R", plus synthetic axis tokens at indices 15-24 ("LS_Y-","LS_X-","LS_X+","LS_Y+","LT","RT","RS_Y-","RS_X-","RS_X+","RS_Y+") and 25 "Not used". Reproduce exactly (map browser `KeyboardEvent.code`/Gamepad API onto these names).

`input.go` sdl leakage to patch (only place): `input.go:96-107` `ShortcutKey.Test` uses `sdl.Keymod` / `sdl.KMOD_GUI|CTRL|ALT|SHIFT` directly. Patch upstream to use `ModifierKey` + 4 backend-provided constants (`KModGui, KModCtrl, KModAlt, KModShift`), then `input.go` is fully portable. Keyboard state store is portable: `sys.keyState map[Key]bool` (`system.go:234`).

### 1.4 Audio

Seam = `AudioSink` interface, verbatim `src/audio_sdl.go:13-22`:

```go
type AudioSink interface {
	Init(sr beep.SampleRate, bufferSize int) error
	Play(s beep.Streamer)
	Lock()
	Unlock()
	Close()
	FillAudio()
}

var speaker AudioSink
```

Construction is hardcoded at `system.go:468-470`:
```go
speaker = &SDLSpeaker{}
speaker.Init(beep.SampleRate(sys.cfg.Sound.SampleRate), audioOutLen)
speaker.Play(NewNormalizer(s.soundMixer))
```
Patch to a `newSpeaker() AudioSink` factory provided per backend (1-line upstream diff), or build-tag the two lines.

Architecture (see §5): single final mix, pull model over `beep.Mixer`.

### 1.5 Video playback (stage/bgdef background videos)

`src/video_ffmpeg.go` (no build tag) imports `github.com/ikemen-engine/reisen` (cgo → ffmpeg). Consumers (stage.go:271,676-687,1800-1801,1952-1966; bgdef.go:308-371; system.go:2175-2177) require type `bgVideo` with:

```go
type bgVideo struct { ... texture Texture; scaleMode BgVideoScaleMode ... } // fields .texture and .scaleMode read by stage.go
func (bgv *bgVideo) Open(filename string, volume int, sm BgVideoScaleMode, sf BgVideoScaleFilter, loop bool) error
func (bgv *bgVideo) Tick() error
func (bgv *bgVideo) SetPlaying(on bool)
func (bgv *bgVideo) SetVisible(on bool)
func (bgv *bgVideo) Reset()
func (bgv *bgVideo) Close()
func (bgv *bgVideo) MixerCleared()
```
(`BgVideoScaleMode`/`BgVideoScaleFilter` are portable, `stage.go:84-99`.) js plan: stub returning error from `Open` (v1); optionally later implement via `<video>` element + `texImage2D(video)`.

### 1.6 Dialogs / logging / TTF loading / renderer selection (`util_*.go` seam)

Per-platform free functions, currently in `src/util_desktop.go` (`//go:build !raw && !android`, imports sqweek/dialog + flopp/go-findfont):

```go
func NewLogWriter() io.Writer
func ShowInfoDialog(message, title string)
func ShowErrorDialog(message string)
func LoadFntTtf(f *Fnt, fontfile string, filename string, height int32)
func selectRenderer(cfgVal string) (Renderer, FontRenderer)
func Logcat(s string)
```
Plus GOOS-specific `func osPreferredLanguage() string` (util_linux.go `//go:build linux`, util_darwin.go `//go:build darwin` — uses os/exec, util_windows.go `//go:build windows` — uses syscall). **GOOS=js matches none of these**: a js file must define `osPreferredLanguage()` (return `navigator.language`).

js impls: dialogs → `js.Global().Call("alert", ...)`/console; `LoadFntTtf` → copy of util_desktop version minus the findfont fallback; `selectRenderer` → returns the WebGL2 pair.

### 1.7 Netplay

No interface seam. `src/netplay.go` (pure Go, `net` TCP: netplay.go:175-176,304,375) and `src/rollback.go` (pure Go, `github.com/ikemen-engine/ggpo` — pure-Go GGPO port over UDP). Both **compile** under js (`net` exists in the js stdlib) but every Dial/Listen fails at runtime with ENOSYS/"not supported". No exclusion needed for v1; hide/disable netplay menu entries, or later replace the transport (WebRTC DataChannel) behind these entry points: `netplay.go` `NetConnection` create/accept (`net.Listen("tcp", ...)` netplay.go:304, `net.Dialer` netplay.go:375) and ggpo's UDP socket factory.

### 1.8 main() / entry

`src/main.go` (no build tag) imports go-sdl2 but **only uses it inside `runtime.GOOS == "android"` branches** (main.go:73-78, 113-117, 155-157). Minimal patch: move those android blocks into an android-tagged helper (they duplicate util_android.go logic anyway) and drop the sdl import; then main.go is portable. It also does `runtime.LockOSThread()` in `init()` (no-op on js, harmless).

---

## 1.b Complete exclude list for GOOS=js (with current build tags)

Files importing cgo-tainted packages, with the tag they carry today and the action:

| file | cgo import(s) | current tag | js action |
|---|---|---|---|
| `src/system_sdl.go` | veandco/go-sdl2 | (none) | add `//go:build !js` |
| `src/input_sdl.go` | veandco/go-sdl2 | (none) | add `//go:build !js` |
| `src/audio_sdl.go` | veandco/go-sdl2 | (none) | add `//go:build !js` |
| `src/main.go` | veandco/go-sdl2 (android-only code paths) | (none) | **patch**: remove sdl import (move android bits) — do NOT exclude, it holds `main()`/`realMain()`/`processCommandLine()`/`handlePanic()` |
| `src/input.go` | veandco/go-sdl2 (`Keymod` consts only, lines 96-107) | (none) | **patch**: replace with `ModifierKey`+backend consts — file is otherwise core logic, must build on js |
| `src/render_gl33.go` | go-gl/gl v3.3-core, go-sdl2 | `//go:build !android` | change to `//go:build !android && !js` |
| `src/font_gl33.go` | go-gl/gl v3.3-core | `//go:build !android` | `//go:build !android && !js` |
| `src/render_gles32.go` | leonkasovan/gl v3.2/gles2 (cgo), go-sdl2 | `//go:build android` | already excluded on js (android tag) — no change |
| `src/font_gles32.go` | leonkasovan/gl | `//go:build android` | no change |
| `src/render_vk.go` | Eiton/vulkan, go-sdl2 | `//go:build !kinc && !android` | `//go:build !kinc && !android && !js` |
| `src/font_vk.go` | Eiton/vulkan | `//go:build !kinc && !android` | `//go:build !kinc && !android && !js` |
| `src/video_ffmpeg.go` | ikemen-engine/reisen (cgo/ffmpeg) | (none) | add `//go:build !js` |
| `src/util_desktop.go` | sqweek/dialog (cgo/gtk on linux), flopp/go-findfont (pure but useless) | `//go:build !raw && !android` | `//go:build !raw && !android && !js` |
| `src/sound_xm.go` | `import "C"` + libxmp | (none — implicit cgo constraint) | auto-excluded when cgo is off, **but** `sound.go:388` calls `xmpDecode` → js build needs a stub (see file plan). Recommend making the tag explicit: `//go:build cgo && !js` |
| `src/util_android.go` | `import "C"` (EGL/JNI) | `//go:build android` | no change |
| `src/util_raw.go` | `import "C"` | `//go:build raw` | no change |
| `src/util_darwin.go` | os/exec | `//go:build darwin` | no change (GOOS excluded) |
| `src/util_linux.go` | — | `//go:build linux` | no change; js needs own `osPreferredLanguage` |
| `src/util_windows.go` | syscall, unsafe | `//go:build windows` | no change |
| `src/dllsearch_windows.go` | golang.org/x/sys/windows | `//go:build windows` | no change |
| `src/stdout_windows.go` | syscall | `//go:build windows` + `// +build windows` | no change |

`golang.org/x/mobile` usage: **only** `golang.org/x/mobile/exp/f32` in `src/model.go:29`, `render_gl33.go:18`, `render_gles32.go:22` — pure Go float32 math helpers, NOT cgo. `model.go` builds fine on js; no action.

All other deps are pure Go and js-safe: gopher-lua, beep/v2 (+go-mp3, oggvorbis, flac, meltysynth midi), mathgl, freetype, x/image, x/text, x/exp, gltf, dds, hdr, gjson/sjson, ini.v1, go-colorful, bitio. `ggpo` is pure Go (uses `net`).

Net upstream-diff summary (the only files whose *content* changes): `main.go` (drop sdl import), `input.go` (4 mod-key consts), `system.go:468` (speaker factory), plus 6 one-line build-tag edits and the new `_js.go` files.

---

## 2. Main loop & frame pacing

- Top level: `main()` → `realMain()` (`main.go:58-212`) → `sys.init(w,h)` (`system.go:369`, creates window/GL/audio/Lua) → `sys.luaLState.DoFile(sys.cfg.Config.System)` (`main.go:201`, default `external/script/main.lua`). **The game loop lives in Lua**: script loops call the Go builtins `refresh` (`script.go:5810`) and `game()`/match code, which funnel into `System.update()` (`system.go:876`) → **`System.await(fps)` (`system.go:786-843`) — the single frame-pacing point**.
- `await` does: `gfx.EndFrame()` → `window.SwapBuffers()` (or `gfx.Await()` for non-"OpenGL" names, system.go:790) → `runMainThreadTask()` → computes `diff = redrawWait.nextTime - now` → **`time.Sleep(diff)` at `system.go:825` only when `0 <= diff < waitDuration+2ms`** → `eventUpdate()` (`system.go:737`, polls window events).
- **wasm implication**: when on schedule, the loop sleeps every frame → Go's js runtime returns to the browser event loop → paint happens. But when running behind (`diff < 0`), the `default:`/fallthrough branches at system.go:827-838 skip the sleep entirely (frameSkip logic), producing sleepless iterations → browser starves, canvas never presents, watchdog may kill the tab. **Required change**: guarantee a yield every frame on js. Recommended: implement js `Window.SwapBuffers()` as "block on a channel resolved by the next `requestAnimationFrame` callback". That yields to the browser (all goroutines parked), gives correct vsync pacing, and makes `time.Sleep(diff)` mostly a no-op refinement. Keep `redrawWait` logic untouched.
- Secondary loops that already sleep (fine on wasm): loader goroutine `time.Sleep(10ms)` `system.go:6263`; audio push goroutine `time.Sleep(17ms)` `audio_sdl.go:68`; netplay waits (netplay.go:382-701). Loading-screen presents at `motif.go:2800-2818` go through `SwapBuffers` too (covered by the RAF strategy).
- `runtime.LockOSThread()` (`main.go:23`, android branch line 72) — harmless no-op on js.

## 3. Filesystem usage

- **Central read path**: everything file-ish goes through `src/common.go`:
  - `OpenFile(filename string) (io.ReadSeekCloser, error)` (`common.go:1662`) — plain `os.Open`, plus transparent read of entries inside `.zip` archives (loads entry fully into memory).
  - `LoadText` (`common.go:395`), `LoadFile(file *string, dirs []string, defaultDir string, load func(string) error)` (`common.go:653`), `SearchFile` (`common.go:538`), `FileExist` (`common.go:447`).
  - `FileExist`/`SearchFile` do **case-insensitive fallback via `filepath.Glob`** (`common.go` ~510) → the wasm `fs` shim must implement `readdir` correctly (Glob = Stat + ReadDir), not just open/read/stat.
- Other direct `os.*` reads: `os.ReadFile` in main.go:146 (stats), input_sdl.go:262 (gamepad mappings), system.go:433-447 (external shaders), config/motif/storyboard INI loads via `LoadText`. Lua side uses `io.open` (gopher-lua → `os` package) e.g. `main.lua:1493` reads select.def. All satisfied by the Node-style `fs` shim on `globalThis` that `wasm_exec.js` expects, backed by an in-memory FS populated from a zip.
- **No mmap, no direct `syscall`** outside windows-tagged files (`stdout_windows.go`, `util_windows.go`) and `os/exec` only in `util_darwin.go`. Verified: only `unsafe` users are bytecode.go (bit casts), image.go, sound_xm.go (cgo, excluded), render backends (excluded). Clean.
- **Write paths** (shim needs writable in-memory dirs):
  - `save/` tree created at boot: `os.MkdirAll(save/replays, save/logs)` main.go:129-130.
  - `save/config.ini` — rewritten **every boot** (`config.go:315 c.Save(def)` → `SaveINI` iniutils.go:1851).
  - `save/stats.json` — created if missing at main.go:148; written via script.go:4240 `os.WriteFile`.
  - hiscore: `hiscore_rank.go:293 os.WriteFile`.
  - crash logs `save/logs/Ikemen_*.log` (main.go:387), match logs (`createLog` main.go:47), replays (`script.go:5877 os.Create`), screenshots (`captureScreen` image.go:2101 → `os.Create` image.go:2132, honoring `Config.ScreenshotFolder`), debug Lua dumps (bytecode.go:12602), rollback logs (rollback.go:435,866+).
  - `os.Executable()`/`os.Chdir` at system.go:402-409: error-tolerated (js returns "not implemented" → skipped). `sys.baseDir = "./"` main.go:119.

## 4. Startup requirements

`realMain()` sequence: create `save/` dirs → `processCommandLine()` (main.go:215; flags: `-config <path>`, `-stats <path>`, `-windowed`, `-r <motif>`, `-p1/-p2`, `-s <stage>`, `-nosound`, `-nomusic`, `-nojoy`, `-ai`, `-speed`, ...; none required; on js pass args via `go.argv` in wasm_exec) → ensure `save/stats.json` → `initLUTs()` → `loadConfig("save/config.ini")` — **missing config.ini is fine**: defaults come from embedded-fallback `src/resources/defaultConfig.ini` (loaded from disk if present, else `defaultConfig` embed; config.go:262-268), then the merged config is written back → open `sys.cfg.Config.System` (**must exist**: `main.go:188 os.Open` panics otherwise; default `System = external/script/main.lua`) → `sys.init()` (window+GL+audio+Lua) → `DoFile(main.lua)`.

Boot file requirements (from defaultConfig.ini + main.lua):
- `external/script/*.lua` (main/menu/options/start/storyboard/global etc.) — **present in repo**.
- `data/*.zss, common.air, common.cmd, common.const` — present in repo.
- Motif: `Motif = data/ikemen1/system.def` (defaultConfig.ini:96). **NOT in this repo** (repo `data/` has only the common .zss/.air/.cmd/.const). `loadMotif` (motif.go:1529) falls back through `LoadFile(&def, {def,"","data/"} ...)` and panics if the .def can't be read. There is an embedded `resources/defaultMotif.ini` but it only supplies *default parameter values*, not the sprite/sound/font assets a motif references.
- `select.def`: motif param `Files.Select` default `select.def` (motif.go:81), loaded by Lua at `main.lua:1493` via `main.f_fileRead(motif.files.select)` → **panics if missing**.
- Fonts: `font/` dir is present (default-3x5 fonts, Open_Sans, debug.def).
- Stage: if select.def adds no stages, main.lua:1719 falls back to `Debug.StartStage = stages/stage0-720.def` (defaultConfig.ini:178) — **also not in repo**.
- Characters: **zero chars is acceptable for reaching the title screen** — select.def parsing tolerates an empty roster (main.lua only errors on missing *files* it's told to load; `TrainingChar` default is empty). Game modes are unplayable without chars, but title/menu/options run.

**Conclusion**: the vendored repo alone does NOT reach the title screen. The browser bundle zip must add, on top of repo `data/`, `external/`, `font/`: a screenpack (`data/ikemen1/system.def` + its sprites/sounds/fonts, or any motif + `-r` flag), a `select.def` (can list nothing but should `include` at least one stage or rely on StartStage), and one stage `.def+.sff` (upstream ships `stages/stage0-720.def`). Pull these from an upstream Ikemen GO release zip (they are release-artifact assets, not in the source tree).

## 5. Audio path ("final mix")

Confirmed single-sink pull architecture:
- One global `beep.Mixer` (`sys.soundMixer`); every sound/BGM/video-audio streamer is added to it. The sink wraps it: `speaker.Play(NewNormalizer(s.soundMixer))` (system.go:470) — the sink's own internal `beep.Mixer` (audio_sdl.go:44) then contains exactly that one Normalizer stream.
- Format: `sys.cfg.Sound.SampleRate` (default 44100, defaultConfig.ini:273), **stereo**, pull chunks of `audioOutLen = 2048` frames (`sound.go:24`), samples are `[][2]float64` from beep, converted to interleaved S16 for SDL (audio_sdl.go:96-104).
- Pump: goroutine loop calling `FillAudio()` every 17 ms (audio_sdl.go:65-70); `FillAudio` checks queue depth (`> bufferSize*4` bytes → skip) then `mixer.Stream(buf[:2048])` under `s.mu` and queues.
- **A WebAudio replacement must provide exactly `AudioSink`** (§1.4): pull `[][2]float64` from its mixer under Lock/Unlock, at cfg sample rate, 2048-frame chunks. Two viable designs: (a) keep the 17ms-sleep pump goroutine and push converted Float32 frames into a ring buffer consumed by an `AudioWorkletProcessor` (SharedArrayBuffer not required if you post chunks); (b) `ScriptProcessorNode`/worklet message → resolve a channel that wakes `FillAudio`. `Lock/Unlock` are called from sound.go/music.go for streamer mutation (`sound.go:105,124` etc.) — must be a real mutex.
- Decoders all pure Go (wav/ogg/flac/mp3/midi via beep; sound.go:16-21). **Exception**: `.xm/.mod/.it/.s3m` music → `xmpDecode` (sound.go:388) lives in cgo-only `sound_xm.go` → js stub must return an error (`beep.StreamSeekCloser, beep.Format, error` signature, sound_xm.go:142).
- `sys.cfg.Sound.SampleRate` should be forced to the `AudioContext.sampleRate` (usually 48000) at js init to avoid a resample; engine already resamples per-source to cfg rate (sound.go:427-428).

## 6. Renderer choice for WebGL2

**Base the js renderer on `render_gles32.go` + `font_gles32.go`** (GLES 3.2 / android pair), not render_gl33.go:

- Same `#ifdef GL_ES` shader path: gles32 injects `#version 320 es` when missing (`render_gles32.go:128-131`); js injects `#version 300 es` instead. gl33 hardcodes `#version 330 core` (`render_gl33.go:107`). All shaders in `src/shaders/*.glsl` are version-less bodies with `#if __VERSION__ >= 450` (Vulkan) / `#else` GL/GLES branches and `#ifdef GL_ES precision` blocks — **they are already GLSL ES-compatible; no separate ES 3.00 variants exist and mostly none are needed** (`.spv` files are Vulkan-only). Verified WebGL2-clean for: sprite.vert/frag, font.vert/frag, ident.*, panoramaToCubeMap.frag, cubemapFiltering.frag, model.* (without ENABLE_SHADOW).
- gl33 uses desktop-only bits: `gl.TEXTURE_CUBE_MAP_ARRAY_ARB` (render_gl33.go:845-850), sdl swap-interval, `gl.Strs` cgo string plumbing. gles32 is structurally closer to a browser GL binding (explicit uniform/attrib caches, GetError logging, no VAO assumptions beyond one VAO).
- Features exceeding WebGL2 (≈ES 3.0) that must be cut/gated — all are **model-shadow only**:
  - Geometry shader `shaders/shadow.geo.glsl` (compiled at render_gles32.go:635 / render_gl33.go:538 via `gl.GEOMETRY_SHADER`). No geometry shaders in WebGL2.
  - `samplerCubeArray` / `TEXTURE_CUBE_MAP_ARRAY` (model.frag.glsl:77, render_gles32.go:940). Not in WebGL2.
  - Both are behind `Video.EnableModelShadow` → `r.enableShadow` (render_gles32.go:711) and `#define ENABLE_SHADOW` (render_gles32.go:607-609). **js renderer: force `enableShadow=false`, return false from `IsShadowEnabled()`, compile model.frag without ENABLE_SHADOW, and implement the four ShadowMap* methods + prepareShadowMapPipeline/setShadowMapPipeline/ReleaseShadowPipeline/RenderShadowMapElements/SetShadowFrame(Cube)Texture as no-ops.**
  - No compute, no SSBO, no UBO anywhere in the GL backends (uniforms are plain glUniform*) — nothing else exceeds WebGL2.
- WebGL2-supported features the port keeps: MSAA renderbuffers + `BlitFramebuffer` resolve (render_gles32.go:888,1067), `RGBA16F`/`RGBA32F` float textures (needs `EXT_color_buffer_float` for the HDR/env FBO attachments — probe and disable `enableModel` PBR-env path if absent), `UNPACK_ROW_LENGTH` (ES3.0 ✓, render_gles32.go:375), `CopyTexSubImage2D` texture-atlas grows (render_gles32.go:432-448), sRGB not used, `ReadBuffer/ReadPixels` for screenshots (render_gles32.go:2339).
- 3D model support (`IsModelEnabled`): keep on; glTF loading is pure Go; model shaders are ES 3.0-clean without shadows.
- Binding layer: replace `leonkasovan/gl` calls with a hand-written `syscall/js` WebGL2 wrapper exposing only the ~60 GL functions the two files use (or copy the call-site style of gles32 onto a `webgl` package). Buffer uploads: use `js.CopyBytesToJS` into a scratch `Uint8Array`.
- `SetVSync` → no-op (RAF paces). `NewWorkerThread()` → `return false` (as gl33, render_gl33.go:2249; model.go:2214 then loads textures synchronously). `Await()` → no-op or `gl.finish()`.

## 7. Other js/wasm hazards

- **Threading**: no OS-thread requirements beyond the (no-op) `LockOSThread`. Goroutines used: audio pump, loader (`system.go` Loader, sleeps 10ms), `SafeGo` wrapper (common.go:1780, forwards panics to `sys.mainThreadTask`), netplay/rollback goroutines, video decode (excluded). All fine on wasm's single-threaded scheduler *as long as the main loop yields* (§2).
- `unsafe`: bytecode.go:11 (float↔int bit casts), image.go:14 (pixel casts), sound_xm.go (excluded) — all legal on wasm.
- 64-bit: GOARCH=wasm has 64-bit int/uintptr; no assumptions found. No `syscall` outside windows files. No `os/exec` outside util_darwin.go. `net` only in netplay.go (+ ggpo dep) — compiles, fails at runtime (§1.7).
- `os.Executable`, clipboard: handled (§1.1, §3). Clipboard on js: return "" synchronously (navigator.clipboard is async); wire paste later via paste events feeding `OnTextEntered`.
- `flopp/go-findfont` (util_desktop): pure Go but scans OS font dirs — js util file simply omits the fallback.
- `time.Now/Since`: fine (performance.now backed). `sdl.GetPerformanceCounter` in `UpdateDebugFPS` → reimplement with `time.Now` in js Window; `sys.gameFPSprevcount` is `uint64` — store nanos.
- gopher-lua `os`/`io` libs opened (`l.OpenLibs()` system.go:472): `io.open`, `os.time` work over the fs shim; `os.execute` would fail but is unused by bundled scripts.
- Panic path calls `gfx.GetName()`/`gfx.DebugInfo()` (main.go:351,358) before gfx may exist on very early crashes — pre-assign `gfx` in the js `selectRenderer` path early, same as today (crash before `sys.init` would nil-deref on any platform; not a js regression).
- `config.Save` on every boot + stats writes mean the fs shim must not be read-only; persist the `save/` subtree to IndexedDB/localStorage if persistence is wanted.
- Case-insensitive `filepath.Glob` fallback (§3) makes boot do many readdirs — make the shim's readdir fast (in-memory maps).

---

## Recommended minimal js backend file plan

New files in `src/` (all `//go:build js`):

| new file | provides (mirror of) |
|---|---|
| `system_js.go` | `Window` struct + `newWindow`, `SwapBuffers` (RAF-blocking present+yield), `pollEvents` (drains a JS→Go event queue: keydown/keyup→`OnKeyPressed/OnKeyReleased`, text→`OnTextEntered`, gamepadconnected, visibility/close), `shouldClose`, `SetIcon` (no-op), `SetSwapInterval` (no-op), `GetSize`/`GetScaledViewportSize` (canvas size), `GetClipboardString` (""), `toggleFullscreen` (Fullscreen API), `UpdateDebugFPS`, `Close`, stub `GLCreateContext`/`GLMakeCurrent` (mirror of system_sdl.go) |
| `input_js.go` | `Key`/`ModifierKey` types (int32 aliases), `Key*` + `KMod*` constants, all LUTs with identical string names, `Input`/`ControllerState`, `initLUTs`, `StringToKey`, `KeyToString`, `NewModifierKey`, joystick methods over the browser Gamepad API (poll in `pollEvents`, standard-mapping→buttonOrder), `RumbleController` via `GamepadHapticActuator`, `CheckAxisForDpad`/`CheckAxisForTrigger`/`getJoystickKey` (copy from input_sdl.go) (mirror of input_sdl.go) |
| `render_webgl2.go` | `Renderer_WebGL2` implementing `Renderer` — port of render_gles32.go with `#version 300 es`, shadow pipeline no-ops, `IsShadowEnabled()==false`, `EXT_color_buffer_float` probe gating HDR/env textures; `GetName() = "OpenGL ES 3.0 (WebGL2)"` |
| `font_webgl2.go` | `FontRenderer_WebGL2` + `Font_WebGL2` implementing `FontRenderer`/`Font` — port of font_gles32.go (freetype rasterize is portable; only upload/draw changes) |
| `gl_js.go` (or subpkg `src/webgl/`) | thin `syscall/js` WebGL2 binding exposing the GL entry points render/font_webgl2 need |
| `audio_js.go` | `JSSpeaker` implementing `AudioSink` over WebAudio (AudioWorklet or buffer-queue), pull from internal `beep.Mixer`, 2048-frame chunks; sets/uses `AudioContext.sampleRate` (mirror of audio_sdl.go) |
| `video_js.go` | `bgVideo` stub: `Open` returns error; `Tick/SetPlaying/SetVisible/Reset/Close/MixerCleared` no-ops; fields `texture Texture`, `scaleMode BgVideoScaleMode` (mirror of video_ffmpeg.go) |
| `util_js.go` | `NewLogWriter` (os.Stdout→console), `ShowInfoDialog`/`ShowErrorDialog` (console+alert), `LoadFntTtf` (desktop version minus findfont), `selectRenderer` → `(&Renderer_WebGL2{}, &FontRenderer_WebGL2{})`, `Logcat`, `osPreferredLanguage` (navigator.language) (mirror of util_desktop.go + util_linux.go) |
| `sound_xm_js.go` | `func xmpDecode(f io.ReadSeekCloser) (beep.StreamSeekCloser, beep.Format, error)` returning "module music not supported" error. (Alternatively tag it `!cgo` so future nocgo desktop builds share it.) |

Upstream content patches (keep to these five):
1. `main.go` — remove `go-sdl2` import; relocate the three android-only sdl blocks.
2. `input.go:96-107` — `sdl.Keymod`/`sdl.KMOD_*` → `ModifierKey`/`KMod*` (define `KMod*` in input_sdl.go from sdl and in input_js.go natively).
3. `system.go:468` — `speaker = &SDLSpeaker{}` → `speaker = newSpeaker()` (each audio file provides `newSpeaker() AudioSink`).
4. Build-tag edits: `+ !js` on system_sdl.go, input_sdl.go, audio_sdl.go, video_ffmpeg.go, render_gl33.go, font_gl33.go, render_vk.go, font_vk.go, util_desktop.go; explicit `cgo` tag on sound_xm.go.
5. (Recommended, tiny) `system.go:825` area — no change needed if `SwapBuffers` blocks on RAF; otherwise add a js-gated minimum yield.

Loader side (outside Go): `wasm_exec.js` + `fs`/`process` shim on `globalThis` implementing open/read/write/stat/lstat/readdir/mkdir/unlink over an in-memory tree unpacked from a zip containing: repo `data/ external/ font/` + screenpack (`data/ikemen1/`) + `select.def` + at least one stage; `save/` writable (optionally IndexedDB-persisted). Boot with `go.argv = ["ikemen", ...flags]`, canvas id fixed, AudioContext resumed on first user gesture before `speaker.Init` completes (queue until resume).

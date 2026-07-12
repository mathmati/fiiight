# fiiight — Ikemen GO in the browser

[Ikemen GO](https://github.com/ikemen-engine/Ikemen-GO), the open-source
fighting-game engine compatible with MUGEN content, running **fully
client-side in the browser**: the engine is compiled from Go to WebAssembly,
renders through WebGL2, and plays sound through WebAudio. No server code, no
plugins — a static file host is all it takes.

This repository is an open-source contribution effort toward upstream
[ikemen-engine/Ikemen-GO](https://github.com/ikemen-engine/Ikemen-GO)
(see their issue
[#1606](https://github.com/ikemen-engine/Ikemen-GO/issues/1606) asking for a
browser/WebAssembly build). The engine source is vendored under `engine/`,
and the browser backend is written so the diff can be upstreamed — see
[Upstreaming intent](#upstreaming-intent).

The bundled demo boots to the title screen, character select, versus and
training with the default Ikemen screenpack and Kung Fu Man 720p.

## Quick start

Requirements: Go (the vendored engine's `go.mod` pins toolchain 1.24), `zip`,
and Node or Python for a local server.

```sh
bash build/wasm.sh          # builds web/ikemen.wasm, web/wasm_exec.js, web/content.zip
node web/dev-server.mjs 8080   # or: python3 web/serve.py 8080
# open http://localhost:8080
```

Any static server works — `main.js` falls back to non-streaming WebAssembly
instantiation if the server sends the wrong MIME type for `.wasm`, and no
cross-origin isolation headers are needed (no SharedArrayBuffer use).

Browser requirements: WebGL2 and WebAssembly, i.e. any modern Chrome, Edge,
Firefox, or Safari.

## Host your own game

This is the fun part: you can ship your own MUGEN/Ikemen game as a static
website.

1. **Drop your content into `content/`**, mirroring the usual MUGEN layout
   (`chars/`, `stages/`, `data/`, `font/`, `sound/`, …). At build time,
   `content/` is overlaid on top of the engine's stock data
   (`engine/data`, `engine/external`, `engine/font`); on path conflicts your
   files win. `content/MANIFEST.md` documents the demo bundle if you want a
   reference.
2. **Edit `content/data/select.def`** to register your characters and stages,
   exactly as you would for a desktop Ikemen GO install. Screenpack changes go
   through `content/data/` the same way (the demo motif lives in
   `content/data/ikemen1/`).
3. **Run `bash build/wasm.sh`.** It compiles the engine and packages
   everything into `web/content.zip`. If you only changed content, pass
   `--shell-only` to skip the engine compile and just repackage the zip.
4. **Upload the `web/` folder to any static host.** No server code needed:
   - **GitHub Pages** — this repo ships a workflow
     (`.github/workflows/deploy-pages.yml`) that builds and deploys on every
     push to `main`; enable Pages → Source: *GitHub Actions* in the repo
     settings.
   - **itch.io** — zip the contents of `web/` and upload it as an HTML
     project with `index.html` as the entry point.
   - **Netlify / Cloudflare Pages** — point the publish directory at `web/`
     (or drag-and-drop the folder).

Player progress and settings persist per browser: files the engine writes
under `save/` are mirrored to `localStorage` and restored on the next visit.

If you host your own game, you are responsible for the licenses of the
content you bundle — see [Credits and licenses](#credits-and-licenses).

## Architecture

The port adds a `js`-build-tagged backend beside the existing SDL/GL/Vulkan
ones: a virtual filesystem shim (`web/fs-shim.js`) implements the Node-style
API Go's `syscall/js` filesystem expects, seeded from `content.zip` and
mirroring `save/` writes to `localStorage`; the main loop presents by
blocking on `requestAnimationFrame`, which both paces to vsync and guarantees
the browser event loop gets control every frame; rendering is a WebGL2 port
of the engine's GLES backend (`engine/src/render_webgl2.go`); and audio is a
WebAudio sink that schedules `AudioBufferSourceNode` chunks against
`AudioContext.currentTime` (`engine/src/audio_js.go`). The full design —
seam inventory, decision log, file-by-file plan — is in
[`port/SPEC.md`](port/SPEC.md).

## Status and known limitations

Working: boot, title, options, character select, versus, training, arcade
flow, sprites/fonts/lifebars, sound effects and streamed music (mp3/ogg/wav),
keyboard and Gamepad-API input, persistent saves.

Not working (yet):

- **No netplay in the browser.** Raw TCP/UDP sockets are impossible in web
  pages; a WebRTC data-channel transport is future work.
- **No background videos.** `type = video` stage/storyboard backgrounds
  (`.webm`) are stubbed out; the demo screenpack's video logo storyboard is
  disabled for this reason.
- **No module music.** `.xm`/`.mod`/`.it`/`.s3m` playback needs libxmp (cgo);
  attempting to play one reports an error. Streamed formats work.
- **Shadows are disabled for 3D-model stages.** The WebGL2 renderer reports
  `IsShadowEnabled() == false`; sprite stages are unaffected.
- **Performance depends on a real GPU.** The CI/software-GL (SwiftShader)
  environment runs around 25 fps; real hardware is expected to hold 60 fps.
  At low frame rates, very short keypresses can occasionally fall between
  polled frames.

## Testing

Headless test harnesses under `web/test/`, all driving Chromium over plain
CDP with no npm dependencies:

- `node web/test/run.mjs` — filesystem shim end-to-end: mounts a generated
  zip, exercises the fs API from a Go wasm binary, verifies `save/`
  persistence across a reload.
- `node web/test/glsmoke/run.mjs` — WebGL2 smoke test: context acquisition,
  compiling the engine's real sprite shaders, draw and readback (on
  SwiftShader).
- `node web/test/audiosmoke/run.mjs` — WebAudio smoke test: asserts a
  non-silent 440 Hz sine reaches the output.
- `node web/test/boot/run.mjs` — full-game boot harness: serves `web/`,
  watches the real engine boot to title/character select, takes periodic
  screenshots, and can inject key presses (`DURATION=…`, `KEYS=…`).

## Upstreaming intent

The goal is to land this in upstream Ikemen GO, so the diff is structured to
be reviewable: the browser backend is a set of **additive `js`-build-tagged
files** (`system_js.go`, `render_webgl2.go`, `font_webgl2.go`, `gl_js.go`,
`input_js.go`, `audio_js.go`, `video_js.go`, `sound_xm_js.go`,
`util_js.go`), plus a minimal set of build-tag/guard patches to shared files.
`port/SPEC.md` records every seam and every shared-file change.

## Credits and licenses

- **Engine:** Ikemen GO, MIT license (see `engine/LICENCE.txt` and the
  bundled asset licenses it references). This repository's own glue code is
  MIT (see [`LICENSE`](LICENSE)).
- **Screenpack assets** (motif, lifebar, sounds, glyphs, logos): CC-BY 3.0,
  from
  [ikemen-engine/Ikemen-GO-Screenpack](https://github.com/ikemen-engine/Ikemen-GO-Screenpack);
  full attribution list in [`content/MANIFEST.md`](content/MANIFEST.md).
- **Kung Fu Man 720p and the Elecbyte bitmap fonts:** © Elecbyte, Creative
  Commons **NonCommercial** licenses.

**Note the NON-COMMERCIAL restriction:** the bundled demo content includes
CC-NC material (kfm720, Elecbyte fonts), so a deployment of this demo bundle
must be non-commercial. If you host your own game, replace or audit the
bundled content and respect the licenses of every character, stage, and
screenpack you ship — `content/MANIFEST.md` lists exactly what this repo
bundles and under which terms.

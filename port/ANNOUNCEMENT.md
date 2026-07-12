# Draft posts for sharing the browser port

## For ikemen-engine/Ikemen-GO issue #1606 ("WIP - Working in WebAssembly")

> Hi all — following up on this issue: we got Ikemen GO running fully in the
> browser, and there's a live demo you can try right now:
>
> **https://mathmati.github.io/fiiight/**
>
> It's the current develop engine (05b7d98a) compiled with
> `GOOS=js GOARCH=wasm` — no server side, runs entirely client-side:
>
> - New js-tagged backend files alongside the SDL ones: WebGL2 renderer
>   (ported from the GLES 3.2 backend), WebAudio sink wrapping the same
>   final-mix interface as the SDL speaker, keyboard + Gamepad API input
>   with save-file-compatible key names, and a canvas window whose
>   SwapBuffers blocks on requestAnimationFrame so the loop yields every
>   frame.
> - Game content ships as a single zip mounted on an in-memory Node-style
>   fs shim (what wasm_exec.js expects), with saves persisted to
>   localStorage.
> - The diff is deliberately structured for upstreaming: additive
>   `*_js.go` files plus five small patches to shared files (build-tag
>   gating, a speaker factory, modifier-key constants). Full seam analysis
>   is written up in the repo (`port/SPEC.md`).
>
> Source: https://github.com/mathmati/fiiight (engine vendored; the js
> backend lives in `engine/src/*_js.go`).
>
> Known gaps: netplay (raw TCP/UDP doesn't exist in browsers — a WebRTC
> DataChannel transport is the plan), background videos, and module music;
> 3D-model shadows are disabled (WebGL2 has no geometry shaders).
>
> If there's interest in taking this upstream I'm happy to open a PR and
> adapt to whatever structure you prefer.

## Shorter version (Discord #development)

> Ikemen GO running in the browser (Go→wasm + WebGL2 + WebAudio, fully
> client-side): https://mathmati.github.io/fiiight/ — current develop
> engine, structured as an additive js backend for possible upstreaming
> (context: issue #1606). Feedback very welcome, especially perf reports
> from different GPUs/browsers.

## Roster note (append to the main post above; replaces the separate license-request issue)

> Roster note: the bundle includes Takezo and Genpaku by @donswelt
> (KGenjuro) — fully original, hand-drawn, and their small animated intro
> stories helped test a few things in the port (storyboards, MIDI music,
> video). I've included them with a note and accreditation in the bundle
> manifest — please let me know if you're happy with them staying in, or
> would rather I removed them. I wanted some demo characters beyond the
> Kung Fu Man cast, but it's hard finding IP-safe things for a clean
> baseline repo — if others have open characters from original IP that
> could be included, let me know.

# content/ — playable content bundle for the browser port

These files sit on top of the vendored engine content (`engine/data`,
`engine/external`, `engine/font`) when `build/wasm.sh` packages
`web/content.zip`. On path conflicts, files in `content/` win.

## Provenance

All files were taken from the upstream screenpack repository that Ikemen GO
release zips bundle their runtime assets from:

- Repo: https://github.com/ikemen-engine/Ikemen-GO-Screenpack
- Ref: `master` @ commit `cb92767` (2026-06-29, "Merge pull request #68 from potsmugen/fix3b")

This is the exact asset source used by the upstream release workflow
(`engine/.github/workflows/releases.yml` clones this repo and copies its
`chars/ data/ font/ sound/ stages/ video/` into every release zip). The
release current at bundling time was the `nightly` prerelease of 2026-07-08.
Files were fetched individually from `raw.githubusercontent.com` (the release
zip asset host was not reachable from the build environment); contents are
identical to what the release zips ship.

## What is included (and why)

- `data/ikemen1/` — default motif (`system.def`, `system.sff`) + its bitmap
  fonts under `data/ikemen1/fonts/` (referenced by `system.def` and
  `data/fight.def`).
- `data/` — motif/lifebar companions resolved from `data/`: `fight.def`,
  `fight.sff`, `fight.snd`, `fightfx.air`, `fightfx.sff`, `common.snd`,
  `glyphs.sff` (movelist glyphs, default motif param `glyphs = glyphs.sff`),
  `system.snd`, and `select.def`.
- `data/gofx/` — engine common FX pack (`defaultConfig.ini` `Common.Fx =
  data/gofx/gofx.def`; required by `dizzy.zss`/`guardbreak.zss`/`tag.zss`).
- `data/select.def` — roster trimmed for the browser bundle: one character
  (`kfm720`, mapped to `stages/stage0-720.def`) plus `randomselect`;
  `[ExtraStages]` trimmed to the one bundled stage. Everything else is
  upstream verbatim.
- `stages/stage0-720.def` + `stages/stage0-720.sff` — default boot/fallback
  stage (`defaultConfig.ini` `Debug.StartStage`). Its .def references no
  music or sound files.
- `chars/kfm720/` — Kung Fu Man 720p (Elecbyte), the lightest 720p-native
  character upstream ships (~1.4 MB), so Versus/Training are playable.
  `Config.TrainingChar` is left empty (engine default): P2 is picked manually
  in Training, which works with a one-character roster + randomselect.
- `font/` — motif fonts not present in `engine/font`: `f-4x6.{def,sff,fnt}`,
  `infofont.def`, `Open_Sans.def` (both point at
  `engine/font/Open_Sans/OpenSans-Regular.ttf`), and the default-motif
  fallback fonts `f-6x9.{def,sff,fnt}`, `f-6x9f.fnt`, `jg.fnt` (referenced by
  `src/resources/defaultMotif.ini` defaults).

## Local modifications (all marked with "Browser port" comments)

- `data/ikemen1/system.def`: `logo.storyboard = logo.def` commented out (the
  logo storyboard is a `type = video` background playing
  `video/ik_logo.webm`; video playback is stubbed on js/wasm, and a failed
  video open is a hard error). The disabled `[Attract Mode]` storyboard
  references were commented out as well. `logo.def` / `ik_logo.webm` are
  therefore not bundled.
- `data/select.def`: roster/stage trim described above.

## Intentionally omitted upstream files

- `data/work/`, `data/ikemen1/logo.def`, `video/` — source art / video intro
  (video unsupported in the wasm port v1).
- Unreferenced legacy fonts (`num*`, `name14`, `font2`, `enter48`, `arcade`,
  `options`, `ending-bg`, `msgothic-tt36`, `mssansserif-tt36`, `f-6x8f.fnt`,
  `name1.fnt`) and the other stages/characters (`stage0`, `stage1`,
  `stage3d*`, `stageZ`, `interactivestage`, `kfm`, `kfm_zss`, `kfm_zaxis`).
- `external/gamecontrollerdb.txt` (SDL gamepad DB; the browser port uses the
  Gamepad API's standard mapping instead).

## Known dead references (safe, present upstream too)

- `data/ikemen1/system.def` `[Music]` entries (`sound/Title.mp3` etc.): the
  upstream screenpack `sound/` directory is empty; the engine documents that
  an invalid BGM filename simply plays no music.
- `system.def` `gameover.def` / `credits.def`: guarded by
  `main.f_fileExists()` in `external/script/start.lua`; skipped when absent
  (they do not exist upstream either).
- `chars/kfm720/kfm720.def` `stcommon = common1.cns`: the engine falls back
  to `engine/data/common1.cns.zss` (`src/compiler.go`).
- `chars/kfm720/kfm720.def` `ai = kfm720.ai`: legacy MUGEN AI hint file,
  unused by Ikemen GO and not shipped upstream.
- `chars/kfm720/intro.def` / `ending.def` `bgm = intro.mp3` / `ending.mp3`:
  not shipped upstream; storyboards play silently.

## Licensing / attribution

- Screenpack motif, lifebar, sounds, glyphs, logos (data/, font/ bitmap
  fonts): Creative Commons Attribution 3.0 Unported (CC-BY 3.0),
  per the screenpack repo `LICENCE.txt`. Attribution:
  - Screenpack Motif and Lifebar assets by Ohmga Shironeko
  - Screenpack Motif sounds by SuperFromND
  - Screenpack Logo by Ohmga Shironeko and President Devon
  - Lifebar messages, rank backgrounds, action icons by President Devon and Rurouni
  - Command list glyphs, order select icons by Rurouni
  - Dizzy, guard break, tag switch effects by Shiyo Kakuge
  - Title screen motif logos by Cylia Margatroid and Rurouni
  - Lifebar voicelines by Miguel Young
- Elecbyte MUGEN font files (`f-4x6`, `f-6x9`, `jg.fnt`, `infofont`):
  Creative Commons Attribution-NonCommercial 3.0 (CC-BY-NC 3.0),
  attribution optional (per screenpack `LICENCE.txt`).
- `chars/kfm720/` (Kung Fu Man 720p): (c) 2009 Elecbyte, Creative Commons
  Noncommercial license, attribution optional (see `chars/kfm720/readme.txt`).
- `stages/stage0-720.*`: Elecbyte sample stage, distributed with the
  screenpack repo under the same terms.
- Engine-side content (`engine/data`, `engine/external`, `engine/font`) is
  distributed under the Ikemen GO project's licenses (MIT source; bundled
  asset licenses per `engine/LICENCE.txt`).

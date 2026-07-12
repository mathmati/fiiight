# content/ — playable content bundle for the browser port

These files sit on top of the vendored engine content (`engine/data`,
`engine/external`, `engine/font`) when `build/wasm.sh` packages
`web/content.zip`. On path conflicts, files in `content/` win.

## Provenance

Except where a per-character section below says otherwise, all files were
taken from the upstream screenpack repository that Ikemen GO release zips
bundle their runtime assets from:

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

- `data/ikemen1/` — default motif (`system.def`, `system.sff`, boot logo
  storyboard `logo.def`) + its bitmap fonts under `data/ikemen1/fonts/`
  (referenced by `system.def` and `data/fight.def`).
- `video/ik_logo.webm` — the Ikemen GO logo video played by
  `data/ikemen1/logo.def` (`logo.storyboard` in `system.def`). Fetched from
  the screenpack repo @ `cb92767` (`video/ik_logo.webm`), unmodified.
- `data/` — motif/lifebar companions resolved from `data/`: `fight.def`,
  `fight.sff`, `fight.snd`, `fightfx.air`, `fightfx.sff`, `common.snd`,
  `glyphs.sff` (movelist glyphs, default motif param `glyphs = glyphs.sff`),
  `system.snd`, and `select.def`.
- `data/gofx/` — engine common FX pack (`defaultConfig.ini` `Common.Fx =
  data/gofx/gofx.def`; required by `dizzy.zss`/`guardbreak.zss`/`tag.zss`).
- `data/select.def` — roster edited for the browser bundle: seven characters
  (`kfm720`, `kfm`, `kfm_zss`, `suavedude`, `takezo`, `genpaku`, `Training`
  (roster-listed with `exclude=1` — kept out of demo/random pools;
  Large/Small variants removed: mismatched scaled hitboxes)) plus
  `randomselect`. All map to `stages/stage0-720.def` except `takezo`, which
  maps to `stages/takezo.def`; `[ExtraStages]` lists the two bundled stages.
  A "how to edit the roster" pointer comment was added at the top of the
  file. Everything else is upstream verbatim. See "Characters" below for
  per-character provenance.
- `stages/stage0-720.def` + `stages/stage0-720.sff` — default boot/fallback
  stage (`defaultConfig.ini` `Debug.StartStage`). Its .def references no
  music or sound files.
- `chars/` — seven-slot roster; per-character provenance, license evidence
  and sizes in the "Characters" section below. `Config.TrainingChar` is left
  empty (engine default): P2 is picked manually in Training.
- `sound/soundfont.sf2` + `sound/soundfont-license.txt` — TimGM6mb General
  MIDI soundfont (Tim Brechbill, 5.7 MB), at the engine's default
  `Sound.SoundFont` config path: enables `.mid` BGM (used by Takezo's and
  Genpaku's intro storyboards). Fetched from the Debian package
  `timgm6mb-soundfont 1.3-5` (deb.debian.org); the package's copyright file
  ships alongside as the license text.
- `font/` — motif fonts not present in `engine/font`: `f-4x6.{def,sff,fnt}`,
  `infofont.def`, `Open_Sans.def` (both point at
  `engine/font/Open_Sans/OpenSans-Regular.ttf`), and the default-motif
  fallback fonts `f-6x9.{def,sff,fnt}`, `f-6x9f.fnt`, `jg.fnt` (referenced by
  `src/resources/defaultMotif.ini` defaults).

## Characters

Roster policy: only original-IP characters with recorded evidence permitting
redistribution. No rips or fan versions of commercial characters. The
characters below are either from the MUGEN/Ikemen "sample" universe
(Elecbyte's Kung Fu Man cast) / training dummies derived from it, or
original hand-drawn characters self-published on GitHub by their author
(KGenjuro's Takezo and Genpaku).

### chars/kfm720/ — Kung Fu Man 720 (Elecbyte) — 1.5 MB

- Source: https://github.com/ikemen-engine/Ikemen-GO-Screenpack @ `cb92767`,
  `chars/kfm720/` (unchanged from the original bundle).
- License: (c) 2009 Elecbyte, Creative Commons Noncommercial, attribution
  optional. `chars/kfm720/readme.txt`: "KFM is licensed under the Creative
  Commons Noncommercial License.  Attribution is optional.  This means you
  may freely build upon or use parts of KFM for non-commercial puposes."
  (typo upstream's).

### chars/kfm/ — Kung Fu Man classic (Elecbyte) — 0.6 MB

- Source: https://github.com/ikemen-engine/Ikemen-GO-Screenpack @ `cb92767`,
  `chars/kfm/`, fetched file-by-file from raw.githubusercontent.com.
- The classic 320x240 MUGEN 1.0 version; plays alongside kfm720 as a second
  fighter. `kfm.ai` reference is dead upstream too (same as kfm720).
- License: identical to kfm720 — its `readme.txt` carries the same Elecbyte
  CC-Noncommercial statement quoted above.

### chars/kfm_zss/ — Kung Fu Man ZSS (Elecbyte / Ikemen GO team) — 0.7 MB

- Source: https://github.com/ikemen-engine/Ikemen-GO-Screenpack @ `cb92767`,
  `chars/kfm_zss/`.
- Elecbyte's KFM with states rewritten in Ikemen GO's native ZSS language by
  the ikemen-engine maintainers; useful engine-coverage character (exercises
  the ZSS compiler path in the wasm build). `stcommon = common1.cns.zss`
  resolves to `engine/data/common1.cns.zss`.
- License: Elecbyte CC-Noncommercial base (same `readme.txt` as kfm); the
  ZSS conversion is distributed by the ikemen-engine org in the same
  screenpack repo our whole bundle comes from.

### chars/suavedude/ — Suave Dude (Masukenpu-kun, 2010) — 2.0 MB

- Source: https://github.com/CableDorado2/Ikemen-Plus-Ultra @ `7c40e6b`,
  `chars/Suave Dude/` (the open-source Ikemen Plus Ultra engine repo),
  fetched file-by-file from raw.githubusercontent.com.
- Suave Dude is the villain from Elecbyte's Kung Fu Man intro storyboard —
  MUGEN sample-universe IP, not a commercial character. This playable
  version was built from scratch by Masukenpu-kun.
- License evidence, `chars/suavedude/readme.txt` (shipped, verbatim): "The
  snd file is borrowed from Kung Fu Man by Elecbyte. The copyright of other
  pictures and files belongs to his Masukenpukun. You are free to use them
  as you wish. However, the character ``Suave Dude'' itself belongs to his
  Elecbyte". Elecbyte's character IP + borrowed snd fall under the same
  CC-Noncommercial sample license as kfm720.
- Local modifications: (1) folder/def renamed `Suave Dude.def` ->
  `suavedude.def` (avoids spaces in select.def entries); (2) two
  `[State -1, Training Mode Setting]` blocks in `sd.cmd` commented out
  (marked "Browser port") — they used the Ikemen-Plus-Ultra-fork-only
  `suavemode` trigger, which Ikemen GO rejects at compile time. The blocks
  only toggled the boss/awaken var in practice mode; default behavior is
  preserved. The separate `Minion/` sub-character (different author, no
  license statement) was intentionally NOT taken; nothing in suavedude's
  files references it.
- WinMUGEN-era character (`MugenVersion = 04,14,2002`); boot-tested in the
  wasm build (char select + versus load OK).

### chars/Training/ — Training dummy (stupa) — Large/Small variants removed post-review (bad scaled hitboxes; overlays looked broken in demo mode)

- Source: https://github.com/acdgames/Ikemen-Plus @ `b1f9d6c`,
  `chars/Training/` — the official Ikemen Plus engine repository (Suehiro's
  S-SIZE Ikemen continued by K4thos), which bundled this character as its
  default training dummy alongside Elecbyte's kfm.
- stupa's Training is the de-facto standard MUGEN/Ikemen training dummy: a
  white mannequin recolor/edit of Elecbyte's Kung Fu Man sprites (Elecbyte
  CC-Noncommercial covers the sprite base and permits building upon KFM).
  No author readme ships with it upstream; the license evidence is (a) the
  Elecbyte KFM-derivative sprite base and (b) its distribution inside the
  official Ikemen engine repo we mirror. Weakest license paper trail in the
  bundle — flagged here for transparency; cut it first if policy tightens.
- Note: the character's big select portrait (sprite 9000,1) is an anime-girl
  drawing bundled with the original character; in-game the dummy is the
  white mannequin.
- Three roster slots share one folder: `Training.def`, `TrainingLarge.def`,
  `TrainingSmall.def` (large/ and small/ subfolders carry their own
  sff/air/cns). Upstream's `TrainingMedium.def` was omitted to keep the
  roster at seven slots.

### chars/takezo/ — Kibagami Takezo (KGenjuro, 2002) — 2.2 MB

- Author: KGenjuro (Torsten Hartmann, github.com/donswelt).
- Source: https://github.com/donswelt/Takezo-Kibagami-Mugen @ `7042c79`
  (`main`), the author's own repository.
- Original-IP evidence: the repo description reads "An original M.U.G.E.N.
  character I made decades ago." The bundled `README.md` (author's own
  F.A.Q., kept verbatim in the folder) says the character was "programmed
  and designed by KGenjuro" with voices and sound FX "recorded by KGenjuro";
  it is an original hand-drawn superdeformed samurai (inspired by, but not
  ripped from, Samurai Shodown — the author describes creating the sprites
  himself from a keyring figure as reference).
- Status: No formal license in the source repo; author self-publishes these
  publicly. A license request is pending (asked in the release announcement,
  ikemen-engine/Ikemen-GO#1606); will be
  removed immediately on the author's request.
- WinMUGEN-era character (`mugenversion = 14,04,2001`), 12 palettes,
  intro/ending storyboards (`takezo-i.def` / `takezo-e.def`) included.
- Local modifications: palette files renamed from upstream uppercase
  `*.ACT` to lowercase `*.act` to match the `.def` `[Files]` references
  byte-for-byte (the browser port's zip filesystem lookup is happiest with
  exact case); the `Takezo_Stage/` subfolder was installed separately as
  `stages/takezo.*` (see Stages below). File contents are unmodified.
- Storyboard `bgm = takezo-i.mid` plays through the bundled
  `sound/soundfont.sf2` (the engine's MIDI decoder is pure Go and works in
  the browser build; it only needs a soundfont at `Sound.SoundFont`).

### chars/genpaku/ — Ogata Genpaku (KGenjuro, 2002) — 4.4 MB

- Author: KGenjuro (Torsten Hartmann, github.com/donswelt).
- Source: https://github.com/donswelt/Genpaku-Mugen @ `f3b91fb` (`main`),
  the author's own repository.
- Original-IP evidence: the repo description reads "Another M.U.G.E.N.
  character I made decades ago." (companion repo to Takezo, whose
  description reads "An original M.U.G.E.N. character I made decades
  ago."); same author, same original hand-drawn cast. The author's
  `README.md` is kept verbatim in the folder.
- Status: No formal license in the source repo; author self-publishes these
  publicly. A license request is pending (asked in the release announcement,
  ikemen-engine/Ikemen-GO#1606); will be
  removed immediately on the author's request.
- WinMUGEN-era character (`mugenversion = 14,04,2002`, V1.0 BETA), 6
  palettes, intro/ending storyboards included. Files are unmodified.
- Storyboard `bgm = genpaku-i.mid`: plays through the bundled
  `sound/soundfont.sf2` (same as Takezo).

## Stages

### stages/takezo.def + stages/takezo.sff — "Takezo's BG" (KGenjuro) — 24 KB

- Author: KGenjuro (Torsten Hartmann, github.com/donswelt).
- Source: https://github.com/donswelt/Takezo-Kibagami-Mugen @ `7042c79`,
  `Takezo_Stage/` subfolder (distributed by the author together with the
  character).
- Original-IP evidence: same repo as chars/takezo/ — description "An
  original M.U.G.E.N. character I made decades ago."; the stage .def credits
  ";by KGenjuro".
- Status: No formal license in the source repo; author self-publishes these
  publicly. A license request is pending (asked in the release announcement,
  ikemen-engine/Ikemen-GO#1606); will be
  removed immediately on the author's request.
- Installed as `stages/takezo.def` + `stages/takezo.sff` (upstream folder
  already names the files `takezo.def`/`takezo.sff`, and its `spr =
  stages/takezo.sff` reference is correct as-is). Registered in
  `data/select.def` `[ExtraStages]` and as takezo's character stage.
- Local modification (marked "Browser port"): `[Music]` `bgmusic =
  sound\sound_sea.mp3` commented out — the mp3 is not distributed with the
  repo; the stage plays silently.

## Local modifications (all marked with "Browser port" comments)

- `data/ikemen1/system.def`: `logo.storyboard = logo.def` is enabled again
  (background-video playback now works on js/wasm via a hidden `<video>`
  element; `logo.def` and `video/ik_logo.webm` are bundled). The disabled
  `[Attract Mode]` storyboard references remain commented out (attract mode
  is off and `intro.def` is not bundled).
- `data/select.def`: roster/stage trim described above.

## Intentionally omitted upstream files

- `data/work/` — source art. From `video/`, only `ik_logo.webm` (used by the
  logo storyboard) is bundled; any other upstream `video/` content is
  referenced only by storyboards that are not enabled in the browser build.
- Unreferenced legacy fonts (`num*`, `name14`, `font2`, `enter48`, `arcade`,
  `options`, `ending-bg`, `msgothic-tt36`, `mssansserif-tt36`, `f-6x8f.fnt`,
  `name1.fnt`) and the other stages (`stage0`, `stage1`, `stage3d*`,
  `stageZ`, `interactivestage`) plus the `kfm_zaxis` character (needs a
  z-axis stage, none bundled).
- `external/gamecontrollerdb.txt` (SDL gamepad DB; the browser port uses the
  Gamepad API's standard mapping instead).

## Known dead references (safe, present upstream too)

- `data/ikemen1/system.def` `[Music]` entries (`sound/Title.mp3` etc.): the
  upstream screenpack `sound/` directory is empty; the engine documents that
  an invalid BGM filename simply plays no music.
- `system.def` `gameover.def` / `credits.def`: guarded by
  `main.f_fileExists()` in `external/script/start.lua`; skipped when absent
  (they do not exist upstream either).
- `stcommon = common1.cns` (kfm720, kfm, suavedude, Training, takezo,
  genpaku defs): the engine falls back to `engine/data/common1.cns.zss`
  (`src/compiler.go`).
- `ai = kfm720.ai` / `ai = kfm.ai` (kfm720, kfm, kfm_zss defs): legacy MUGEN
  AI hint files, unused by Ikemen GO and not shipped upstream.
- `intro.def` / `ending.def` `bgm = intro.mp3` / `ending.mp3` (kfm720, kfm,
  kfm_zss): not shipped upstream; storyboards play silently.

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
- `chars/kfm720/`, `chars/kfm/`, `chars/kfm_zss/` (Kung Fu Man variants):
  (c) 2009 Elecbyte, Creative Commons Noncommercial license, attribution
  optional (see each folder's `readme.txt`).
- `chars/suavedude/` (Suave Dude): sprites/code (c) Masukenpu-kun, "You are
  free to use them as you wish" (`chars/suavedude/readme.txt`); snd + the
  Suave Dude character IP: Elecbyte, CC-Noncommercial sample license.
- `chars/Training/` (Training dummy): stupa; Elecbyte KFM sprite derivative
  (CC-Noncommercial); redistributed from the official Ikemen Plus repo. See
  the Characters section for the full evidence trail.
- `stages/stage0-720.*`: Elecbyte sample stage, distributed with the
  screenpack repo under the same terms.
- `chars/takezo/`, `chars/genpaku/`, `stages/takezo.*` (KGenjuro / Torsten
  Hartmann): original characters and stage, self-published by the author at
  github.com/donswelt. No formal license in the source repos; a license
  request is pending (asked in the release announcement,
  ikemen-engine/Ikemen-GO#1606); will be removed immediately
  on the author's request.
- `sound/soundfont.sf2` (TimGM6mb): (c) 2004 Tim Brechbill, 2010 David
  Bolton; GNU GPL v2 (full text pointer in `sound/soundfont-license.txt`,
  taken verbatim from Debian's `timgm6mb-soundfont` copyright file).
- Engine-side content (`engine/data`, `engine/external`, `engine/font`) is
  distributed under the Ikemen GO project's licenses (MIT source; bundled
  asset licenses per `engine/LICENCE.txt`).

#!/usr/bin/env bash
# Build the browser (WebAssembly) version of Ikemen GO into bin/Ikemen_GO-Web/:
#   1. compile ./src to ikemen.wasm (GOEXPERIMENT=arenas CGO_ENABLED=0
#      GOOS=js GOARCH=wasm)
#   2. copy wasm_exec.js from GOROOT (lib/wasm on modern Go, misc/wasm on
#      older layouts)
#   3. copy the static browser runtime files from build/wasm/
#   4. package content.zip: from CONTENT_DIR (a complete game directory) when
#      given, else from the repo's own runtime dirs (data/ external/ font/)
#      overlaid with the Elecbyte screenpack -- the same asset set desktop
#      releases bundle next to the binary (cloned like build.sh does; skip
#      with SCREENPACK=0 for a motif-less engine-only bundle)
#   5. write a version stamp (version.txt)
#
# Usage: build/wasm/build_web.sh [CONTENT_DIR]
#   CONTENT_DIR may also be passed as an environment variable. It should
#   point at a directory laid out like a desktop install (data/, font/,
#   chars/, stages/, ...); its contents become the root of content.zip.
#
# The output directory is a self-contained static site. Serve it with any
# static file host, e.g.:
#   node bin/Ikemen_GO-Web/dev-server.mjs 8080
#   python3 bin/Ikemen_GO-Web/serve.py 8080
# Hosts that support HTTP Range requests get lazy content streaming (files
# download when first used); on hosts without it the shell falls back to
# downloading content.zip in full before boot. No cross-origin isolation
# headers are needed (the shell does not use SharedArrayBuffer).
set -euo pipefail

WASM_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$WASM_DIR/../.." && pwd -P)"
OUT="$REPO_ROOT/bin/Ikemen_GO-Web"
CONTENT_DIR="${1:-${CONTENT_DIR:-}}"

# Screenpack assets for the default bundle (same knobs as build/build.sh)
SCREENPACK="${SCREENPACK:-1}"
SCREENPACK_REPO="${SCREENPACK_REPO:-https://github.com/ikemen-engine/Ikemen-GO-Screenpack.git}"
SCREENPACK_REF="${SCREENPACK_REF:-master}"
SCREENPACK_DIR="${SCREENPACK_DIR:-$REPO_ROOT/build/screenpack}"

ensure_screenpack() {
	mkdir -p "$(dirname "$SCREENPACK_DIR")"
	if [[ ! -d "$SCREENPACK_DIR/.git" ]]; then
		rm -rf "$SCREENPACK_DIR"
		echo "==> Cloning Elecbyte screenpack: $SCREENPACK_REPO ($SCREENPACK_REF) -> $SCREENPACK_DIR"
		git clone --depth=1 -b "$SCREENPACK_REF" "$SCREENPACK_REPO" "$SCREENPACK_DIR"
	else
		echo "==> Using existing screenpack in $SCREENPACK_DIR"
	fi
}

for tool in go zip; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "ERROR: '$tool' not found on PATH (required for the Web build)." >&2
		exit 1
	fi
done

mkdir -p "$OUT"

# 1. engine -> wasm (pure Go: no cgo, no SDL/FFmpeg/libxmp)
echo "==> building $OUT/ikemen.wasm"
(cd "$REPO_ROOT" && \
	GOEXPERIMENT=arenas CGO_ENABLED=0 GOOS=js GOARCH=wasm \
	go build -trimpath \
	-ldflags "-s -w -X 'main.Version=${APP_VERSION:-nightly}' -X 'main.BuildTime=${APP_BUILDTIME:-$(date '+%Y.%m.%d')}'" \
	-o "$OUT/ikemen.wasm" ./src)

# 2. wasm_exec.js from GOROOT
GOROOT_DIR="$(go env GOROOT)"
copied=0
for candidate in "$GOROOT_DIR/lib/wasm/wasm_exec.js" "$GOROOT_DIR/misc/wasm/wasm_exec.js"; do
	if [[ -f "$candidate" ]]; then
		cp "$candidate" "$OUT/wasm_exec.js"
		echo "==> copied $candidate"
		copied=1
		break
	fi
done
if [[ "$copied" -eq 0 ]]; then
	echo "ERROR: wasm_exec.js not found under $GOROOT_DIR" >&2
	exit 1
fi

# 3. static browser runtime
echo "==> copying browser runtime files"
for f in index.html main.js fs-shim.js loader.js touch.js dev-server.mjs serve.py; do
	cp "$WASM_DIR/$f" "$OUT/$f"
done

# 4. content.zip
rm -f "$OUT/content.zip"
if [[ -n "$CONTENT_DIR" ]]; then
	if [[ ! -d "$CONTENT_DIR" ]]; then
		echo "ERROR: CONTENT_DIR is not a directory: $CONTENT_DIR" >&2
		exit 1
	fi
	echo "==> packaging content.zip from $CONTENT_DIR"
	(cd "$CONTENT_DIR" && zip -q -r "$OUT/content.zip" . -x '*.git*')
else
	# Repo-default content: the same dirs the desktop engine uses in place.
	# Excluded: Vulkan-only shader binaries (no Vulkan on WebGL2), Windows
	# .ico files and icon sources (config references the .png icons only).
	echo "==> packaging content.zip from repo defaults (data/ external/ font/)"
	(cd "$REPO_ROOT" && zip -q -r "$OUT/content.zip" data external font \
		-x '*.git*' -x '*.spv' -x 'external/icons/*.ico' -x 'external/icons/icon-src/*')
	# The repo dirs alone have no motif: the default config points at
	# data/ikemen1/system.def, which lives in the screenpack repo that
	# desktop releases bundle. Overlay it so the default build boots.
	if [[ "$SCREENPACK" != "0" ]]; then
		ensure_screenpack
		overlay_dirs=()
		for d in "$SCREENPACK_DIR"/*/; do
			[[ -d "$d" ]] && overlay_dirs+=("$(basename "$d")")
		done
		echo "==> overlaying screenpack (${overlay_dirs[*]})"
		(cd "$SCREENPACK_DIR" && zip -q -r "$OUT/content.zip" "${overlay_dirs[@]}" -x '*.git*')
	else
		echo "==> SCREENPACK=0: engine-only bundle (no motif; supply your own via CONTENT_DIR)"
	fi
fi
echo "==> content.zip: $(du -h "$OUT/content.zip" | cut -f1)"

# 5. build stamp: lets the running page tell which deploy it is (loader.js
# "Check for updates" compares this against a fresh fetch)
STAMP="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo local)-$(date -u +%Y%m%d%H%M)"
printf '%s\n' "$STAMP" > "$OUT/version.txt"
echo "==> stamped version.txt: $STAMP"

echo "==> Web build ready: $OUT"

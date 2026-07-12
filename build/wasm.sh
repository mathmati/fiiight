#!/usr/bin/env bash
# Build the Ikemen GO browser shell:
#   1. compile the engine to web/ikemen.wasm (GOOS=js GOARCH=wasm)
#   2. copy wasm_exec.js from GOROOT
#   3. package web/content.zip from the engine content dirs (data/ external/ font/)
#
# The engine's js/wasm port is not finished yet, so step 1 is expected to fail
# today; pass --shell-only to skip it and still produce steps 2-3.
#
# Serve the result with any static server, e.g.:
#   node web/dev-server.mjs 8080         # correct .wasm MIME
#   python3 web/serve.py 8080            # ditto
#   (cd web && python3 -m http.server)   # also fine; main.js falls back to
#                                        # non-streaming instantiation if the
#                                        # .wasm MIME type is wrong.
# No cross-origin isolation headers are needed (no SharedArrayBuffer use).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHELL_ONLY=0
for arg in "$@"; do
	case "$arg" in
		--shell-only) SHELL_ONLY=1 ;;
		*) echo "usage: $0 [--shell-only]" >&2; exit 2 ;;
	esac
done

mkdir -p "$ROOT/web"

# 1. engine -> wasm
if [[ "$SHELL_ONLY" -eq 1 ]]; then
	echo "==> skipping engine build (--shell-only)"
else
	echo "==> building engine to web/ikemen.wasm"
	(cd "$ROOT/engine" && GOEXPERIMENT=arenas GOOS=js GOARCH=wasm go build -trimpath -o "$ROOT/web/ikemen.wasm" ./src)
fi

# 2. wasm_exec.js from GOROOT (lib/wasm on modern Go, misc/wasm on older layouts)
GOROOT_DIR="$(go env GOROOT)"
copied=0
for candidate in "$GOROOT_DIR/lib/wasm/wasm_exec.js" "$GOROOT_DIR/misc/wasm/wasm_exec.js"; do
	if [[ -f "$candidate" ]]; then
		cp "$candidate" "$ROOT/web/wasm_exec.js"
		echo "==> copied $candidate"
		copied=1
		break
	fi
done
if [[ "$copied" -eq 0 ]]; then
	echo "error: wasm_exec.js not found under $GOROOT_DIR" >&2
	exit 1
fi

# 3. content.zip from engine content dirs overlaid with content/
#    (content/ wins on path conflicts: zip updates entries added twice)
echo "==> packaging web/content.zip"
rm -f "$ROOT/web/content.zip"
# exclusions: Vulkan-only shader binaries (no Vulkan on WebGL2), Windows .ico
# files and icon sources (config only references the IkemenCylia_*.png icons)
(cd "$ROOT/engine" && zip -q -r "$ROOT/web/content.zip" data external font \
	-x '*.git*' -x '*.spv' -x 'external/icons/*.ico' -x 'external/icons/icon-src/*')
if [[ -d "$ROOT/content" ]]; then
	overlay_dirs=()
	for d in "$ROOT/content"/*/; do
		[[ -d "$d" ]] && overlay_dirs+=("$(basename "$d")")
	done
	if [[ ${#overlay_dirs[@]} -gt 0 ]]; then
		(cd "$ROOT/content" && zip -q -r "$ROOT/web/content.zip" "${overlay_dirs[@]}" -x '*.git*' -x 'MANIFEST.md')
	fi
fi
echo "==> done: $(du -h "$ROOT/web/content.zip" | cut -f1) content.zip"

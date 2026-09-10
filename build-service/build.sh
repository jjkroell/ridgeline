#!/usr/bin/env bash
# Build one MeshCore firmware inside the builder container.
#
#   build.sh <git-tag> <pio-env> [extra -D flags...]
#
# Options are passed as build flags via PLATFORMIO_BUILD_FLAGS rather than by
# editing platformio.ini: nothing in the checkout is modified per request, so
# concurrent or repeated builds cannot contaminate each other, and the tree
# stays exactly as upstream tagged it apart from the patches applied below.
set -euo pipefail

TAG="${1:?usage: build.sh <tag> <env> [flags...]}"
ENV_NAME="${2:?usage: build.sh <tag> <env> [flags...]}"
shift 2
EXTRA_FLAGS=("$@")

SRC_DIR="${SRC_DIR:-/src}"
OUT_DIR="${OUT_DIR:-/out}"
REPO="${MESHCORE_REPO:-https://github.com/meshcore-dev/MeshCore.git}"

log() { echo "[build] $*"; }

# ── Source at the requested tag ───────────────────────────────────────────────
if [[ -d "$SRC_DIR/.git" ]]; then
  log "fetching $TAG"
  git -C "$SRC_DIR" fetch --depth 1 origin "refs/tags/${TAG}:refs/tags/${TAG}" --force -q
else
  log "cloning $TAG"
  git clone --branch "$TAG" --depth 1 -q "$REPO" "$SRC_DIR"
fi
# Only reset when the checkout is not already on the requested tag. A hard reset
# rewrites every source mtime, which defeats PlatformIO's incremental build and
# turns a 20-second flag change into a 3-minute full recompile. Staying put is
# safe because the patches below are idempotent and assert their own anchors, so
# a tree already patched is left exactly as it should be.
WANT=$(git -C "$SRC_DIR" rev-parse -q --verify "tags/${TAG}^{commit}")
HAVE=$(git -C "$SRC_DIR" rev-parse -q --verify HEAD || true)
if [[ "$WANT" != "$HAVE" ]]; then
  log "switching to $TAG"
  git -C "$SRC_DIR" reset --hard -q "tags/${TAG}"
  git -C "$SRC_DIR" clean -fdq -e .pio
else
  # Same commit: discard any stray edit outside the files we patch, so a failed
  # previous run cannot leave the tree lying about what it built.
  DIRTY=$(git -C "$SRC_DIR" diff --name-only | grep -v '^examples/companion_radio/NodePrefs.h$' || true)
  if [[ -n "$DIRTY" ]]; then
    log "unexpected local changes, resetting: $(echo "$DIRTY" | tr '\n' ' ')"
    git -C "$SRC_DIR" reset --hard -q "tags/${TAG}"
    git -C "$SRC_DIR" clean -fdq -e .pio
  else
    log "already at $TAG, keeping build cache"
  fi
fi

# ── Patches ───────────────────────────────────────────────────────────────────
# Applied with python rather than `git apply` on purpose: these are one-line
# edits that must survive upstream context drift across releases, and a failed
# hunk should be a clear error naming the file, not a rejected patch file.
python3 /patches/apply.py "$SRC_DIR"

# ── Build ─────────────────────────────────────────────────────────────────────
cd "$SRC_DIR"
if [[ ${#EXTRA_FLAGS[@]} -gt 0 ]]; then
  export PLATFORMIO_BUILD_FLAGS="${EXTRA_FLAGS[*]}"
  log "flags: $PLATFORMIO_BUILD_FLAGS"
fi
log "building $ENV_NAME @ $TAG"
pio run -e "$ENV_NAME"

# ── Merge ESP32 flash images ──────────────────────────────────────────────────
# An ESP32 build emits firmware.bin, bootloader.bin and partitions.bin as three
# images that belong at three different flash offsets — not something a person
# can sensibly flash from a download. MeshCore ships a `mergebin` PlatformIO
# target that combines them using the board's own FLASH_EXTRA_IMAGES offsets.
#
# Use that target rather than calling esptool with our own offset table: the
# bootloader sits at 0x1000 on the original ESP32 but 0x0 on the S3/C3/C6
# family, and getting it wrong produces a file that flashes cleanly and then
# fails to boot. The board config already knows.
#
# Presence of bootloader.bin + partitions.bin is the signal; nRF52 builds have
# neither and are already single-image.
mkdir -p "$OUT_DIR"
BUILD="$SRC_DIR/.pio/build/$ENV_NAME"
if [[ -f "$BUILD/bootloader.bin" && -f "$BUILD/partitions.bin" ]]; then
  log "esp32 flash layout detected, merging images"
  MERGED_BIN_PATH="$BUILD/firmware-merged.bin" pio run -e "$ENV_NAME" -t mergebin
  [[ -f "$BUILD/firmware-merged.bin" ]] || { echo "[build] mergebin produced nothing" >&2; exit 1; }
fi

# ── Convert nRF52 images to UF2 ───────────────────────────────────────────────
# An nRF52 build emits a .hex, which needs a programmer, and a .zip for serial
# DFU. What people actually want is the .uf2 they can drag onto the board's USB
# drive. MeshCore ships a `create_uf2` target for exactly this — use it rather
# than calling uf2conv ourselves, so the family id stays theirs to get right.
#
# Presence of a .hex with no ESP32 flash layout is the signal.
if [[ -f "$BUILD/firmware.hex" && ! -f "$BUILD/bootloader.bin" && ! -f "$BUILD/firmware.uf2" ]]; then
  log "nrf52 build detected, creating uf2"
  pio run -e "$ENV_NAME" -t create_uf2 || log "uf2 conversion failed (keeping .hex)"
fi

# ── Collect ───────────────────────────────────────────────────────────────────
found=0
for f in firmware-merged.bin firmware.zip firmware.uf2 firmware.bin; do
  if [[ -f "$BUILD/$f" ]]; then
    cp "$BUILD/$f" "$OUT_DIR/$f"
    log "artifact $f ($(stat -c%s "$BUILD/$f") bytes)"
    found=1
  fi
done
# .hex only as a fallback: it needs a programmer, so it is worth publishing only
# when the uf2 conversion did not produce the drag-and-drop file instead.
if [[ ! -f "$OUT_DIR/firmware.uf2" && -f "$BUILD/firmware.hex" ]]; then
  cp "$BUILD/firmware.hex" "$OUT_DIR/firmware.hex"
  log "artifact firmware.hex ($(stat -c%s "$BUILD/firmware.hex") bytes) — no uf2 was produced"
  found=1
fi
# Keep the component images beside the merged one: they are what a bench setup
# with esptool wants, and they cost a few KB.
for f in bootloader.bin partitions.bin; do
  [[ -f "$BUILD/$f" ]] && cp "$BUILD/$f" "$OUT_DIR/$f"
done
[[ $found -eq 1 ]] || { echo "[build] no artifacts produced" >&2; exit 1; }
log "done"

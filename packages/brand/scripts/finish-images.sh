#!/usr/bin/env bash
# Operator-only, never in CI. Turns raw generations into the web-size files the manual shows:
#   $BRAND_GEN_DIR/{photo,world,kit}/<id>.png → public/brand/img/<area>/<id>.jpg
# (≤1600 px, q82, prompt embedded as JPEG comment). Needs ImageMagick (`magick`). Only files
# newer than their JPG are rebuilt; keep public/ under the 15 MB budget (public-assets test).
#   bun run finish-images
set -eu
cd "$(dirname "$0")/.."
gen=${BRAND_GEN_DIR:-$HOME/Dev/compozy/_rebranding/gen}
command -v magick >/dev/null || { echo "error: ImageMagick (magick) is required" >&2; exit 1; }
for f in "$gen"/{photo,world,kit}/*.png; do
  [ -f "$f" ] || continue
  area=$(basename "$(dirname "$f")")
  id=$(basename "$f" .png)
  out=public/brand/img/$area/$id.jpg
  mkdir -p "$(dirname "$out")"
  [ "$out" -nt "$f" ] && continue
  magick "$f" -resize '1600x1600>' -strip -set comment "$(head -c 4000 "prompts/$area/$id.md" 2>/dev/null)" -quality 82 "$out"
  echo "→ $out"
done

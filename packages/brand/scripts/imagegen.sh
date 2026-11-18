#!/usr/bin/env bash
# Operator-only, paid, never in CI. Generates manual images with Codex's built-in image_gen
# (ChatGPT login), 4 at a time, from prompts/<area>/<id>.md.
#   bun run imagegen <area> [id ...]
# Raw PNGs go outside git (ADR-003): $BRAND_GEN_DIR/<area>/<id>.png, default
# ~/Dev/compozy/_rebranding/gen. Reference images: $BRAND_GEN_DIR/refs/<file>, named by a
# `refs:` line in the prompt. Existing outputs are skipped; delete one to regenerate it.
set -u
cd "$(dirname "$0")/.."
area=${1:?usage: bun run imagegen <area> [id ...]}
shift
gen=${BRAND_GEN_DIR:-$HOME/Dev/compozy/_rebranding/gen}
[ -d "prompts/$area" ] || { echo "error: no prompts/$area" >&2; exit 1; }
ids=("$@")
[ ${#ids[@]} -eq 0 ] && ids=($(ls "prompts/$area" | sed 's/\.md$//'))
mkdir -p "$gen/$area" "$gen/logs"
run() {
  local id=$1 f=prompts/$area/$1.md out=$gen/$area/$1.png
  [ -f "$out" ] && { echo "skip $id"; return; }
  local refs args=() spec task
  refs=$(grep -m1 '^refs:' "$f" | sed 's/^refs: *//')
  for r in $refs; do args+=(-i "$gen/refs/$r"); done
  spec=$(grep -v '^refs:' "$f")
  task="Generate exactly ONE image with your image generation tool following the spec below. Save the final image as a PNG file at: $out
Do not create any other files, do not write code, do not explain. If a reference image is attached it is Image 1.

SPEC:
$spec"
  codex exec -s danger-full-access --skip-git-repo-check -C "$gen" "${args[@]}" -- "$task" >"$gen/logs/$area-$id.log" 2>&1
  [ -f "$out" ] && echo "ok $id" || echo "FAIL $id"
}
export -f run
export area gen
printf '%s\n' "${ids[@]}" | xargs -P 4 -I{} bash -c 'run {}'

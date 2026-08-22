#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUNDLED_DIR="$ROOT_DIR/skills"
SOURCE_DIR="$ROOT_DIR/.agents/skills"
TARGET_DIR="$ROOT_DIR/.claude/skills"

link_skill() {
  local source_path="$1"
  local skill_name
  skill_name="$(basename "$source_path")"
  local target="$TARGET_DIR/$skill_name"

  if [ -L "$target" ] || [ -d "$target" ]; then
    rm -rf "$target"
  fi

  ln -s "$source_path" "$target"
}

# Skills shipped inside the binary (skills/<name>) are also exposed to local
# agents: each one is linked into .agents/skills with a relative symlink, so the
# loop below picks it up and mirrors it into .claude/skills like any other skill.
if [ -d "$BUNDLED_DIR" ]; then
  mkdir -p "$SOURCE_DIR"

  bundled_skills=0
  for bundled in "$BUNDLED_DIR"/*/; do
    bundled="${bundled%/}"
    [ -f "$bundled/SKILL.md" ] || continue

    skill_name="$(basename "$bundled")"
    bundled_link="$SOURCE_DIR/$skill_name"

    if [ -L "$bundled_link" ] || [ -e "$bundled_link" ]; then
      rm -rf "$bundled_link"
    fi

    ln -s "../../skills/$skill_name" "$bundled_link"
    bundled_skills=$((bundled_skills + 1))
  done

  echo "Linked $bundled_skills bundled skills from skills/ → .agents/skills"
fi

# Shared spec-cycle skills are canonical in extensions/spec-cycle/skills (the
# extension embeds them into the binary); each listed one is linked into
# .agents/skills so the loop below mirrors it into .claude/skills. Only
# byte-identical shares belong in this list — cy-create-spec, cy-create-tasks,
# and cy-final-verify keep intentionally divergent local variants (Compozy
# tooling vs the portable extension copy) and must never be linked.
SPEC_CYCLE_DIR="$ROOT_DIR/extensions/spec-cycle/skills"
SPEC_CYCLE_SHARED_SKILLS=(
  cy-execute-task
  cy-fix-reviews
  cy-review-round
  cy-workflow-memory
)

if [ -d "$SPEC_CYCLE_DIR" ]; then
  mkdir -p "$SOURCE_DIR"

  spec_cycle_skills=0
  for skill_name in "${SPEC_CYCLE_SHARED_SKILLS[@]}"; do
    [ -f "$SPEC_CYCLE_DIR/$skill_name/SKILL.md" ] || continue

    shared_link="$SOURCE_DIR/$skill_name"
    if [ -L "$shared_link" ] || [ -e "$shared_link" ]; then
      rm -rf "$shared_link"
    fi

    ln -s "../../extensions/spec-cycle/skills/$skill_name" "$shared_link"
    spec_cycle_skills=$((spec_cycle_skills + 1))
  done

  echo "Linked $spec_cycle_skills spec-cycle skills from extensions/spec-cycle/skills → .agents/skills"
fi

if [ -d "$SOURCE_DIR" ]; then
  mkdir -p "$TARGET_DIR"

  linked_skills=0
  for skill in "$SOURCE_DIR"/*/; do
    skill="${skill%/}"

    if [ -f "$skill/SKILL.md" ]; then
      link_skill "$skill"
      linked_skills=$((linked_skills + 1))
      continue
    fi

    # No SKILL.md at this level: treat the folder as a skill group and
    # symlink each nested child that contains a SKILL.md at the top level
    # (Claude Code does not load nested skill folders).
    for nested in "$skill"/*/; do
      nested="${nested%/}"
      if [ -f "$nested/SKILL.md" ]; then
        link_skill "$nested"
        linked_skills=$((linked_skills + 1))
      fi
    done
  done

  echo "Linked $linked_skills skills from .agents/skills → .claude/skills"
else
  echo "No .agents/skills directory found, skipping skill symlink."
fi

# CLAUDE.md is the authoritative file per surface; AGENTS.md is a relative
# symlink to it so the two never drift. Surfaces are listed explicitly so
# imported repos under .resources/ and other vendored trees are never touched.
SURFACES=(
  "."
  "web"
  "internal"
  "packages/site"
  "packages/slides"
)

linked_pairs=0
for surface in "${SURFACES[@]}"; do
  claude_file="$ROOT_DIR/$surface/CLAUDE.md"
  agents_file="$ROOT_DIR/$surface/AGENTS.md"

  if [ ! -f "$claude_file" ]; then
    continue
  fi

  if [ -L "$agents_file" ]; then
    # Already a symlink: refresh it so target stays correct.
    rm "$agents_file"
  elif [ -e "$agents_file" ]; then
    # Real file: replace with a symlink. Content should already be synced.
    rm "$agents_file"
  fi

  ln -s "CLAUDE.md" "$agents_file"
  linked_pairs=$((linked_pairs + 1))
done

echo "Linked $linked_pairs AGENTS.md → CLAUDE.md pair(s)"

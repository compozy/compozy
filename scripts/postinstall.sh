#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
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

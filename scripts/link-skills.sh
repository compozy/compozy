#!/bin/bash

# Script to create symbolic links for each individual skill from
# .agents/skills/<skill> to .claude/skills/<skill>
# This keeps .agents/skills as the source of truth while making
# skills accessible to Claude Code via .claude/skills

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SOURCE_DIR="$REPO_ROOT/.agents/skills"

# Agent targets
TARGET_DIRS=(".claude")

link_skill() {
  local skill_path="$1"
  local target_dir="$2"
  local skill_name
  skill_name="$(basename "$skill_path")"
  local target_path="$REPO_ROOT/$target_dir/skills/$skill_name"

  if [ -e "$target_path" ]; then
    if [ -L "$target_path" ]; then
      CURRENT_TARGET="$(readlink "$target_path")"
      if [ "$CURRENT_TARGET" = "$skill_path" ]; then
        return 0
      fi
      rm "$target_path"
    else
      rm -rf "$target_path"
    fi
  fi

  ln -s "$skill_path" "$target_path"
  echo "  Linked: $skill_name"
}

for target_dir in "${TARGET_DIRS[@]}"; do
  target_skills_dir="$REPO_ROOT/$target_dir/skills"

  if [ ! -d "$SOURCE_DIR" ]; then
    echo "Warning: $SOURCE_DIR does not exist. Skipping."
    continue
  fi

  # Remove stale whole-folder symlink if present
  if [ -L "$target_skills_dir" ]; then
    echo "Removing stale whole-folder symlink at $target_dir/skills"
    rm "$target_skills_dir"
  fi

  mkdir -p "$target_skills_dir"

  echo "Linking skills into $target_dir/skills:"
  for skill in "$SOURCE_DIR"/*/; do
    [ -d "$skill" ] || continue
    link_skill "$skill" "$target_dir"
  done
done

echo "Symlink setup complete!"

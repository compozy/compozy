#!/bin/bash

# Link agent skills into the locations each tool expects.
#
# 1) Bundled skills under skills/ are the source of truth when a matching
#    skill also lives (or used to live) under .agents/skills. Symlink those into
#    both .agents/skills and .claude/skills.
# 2) Everything else under .agents/skills is still linked into .claude/skills
#    (including cy-* skills that exist only under .agents/skills).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AGENTS_SKILLS="$REPO_ROOT/.agents/skills"
BUNDLED_SKILLS="$REPO_ROOT/skills"
CLAUDE_SKILLS="$REPO_ROOT/.claude/skills"

# Relative target from .agents/skills/<name> or .claude/skills/<name> -> skills/<name>
bundled_rel_target() {
  local skill_name="$1"
  printf '../../skills/%s' "$skill_name"
}

ensure_symlink() {
  local link_path="$1"
  local desired_target="$2"
  local label="$3"

  mkdir -p "$(dirname "$link_path")"

  if [ -L "$link_path" ]; then
    if [ "$(readlink "$link_path")" = "$desired_target" ]; then
      return 0
    fi
    rm "$link_path"
  elif [ -e "$link_path" ]; then
    echo "  Warning: replacing non-symlink path $link_path" >&2
    rm -rf "$link_path"
  fi

  ln -s "$desired_target" "$link_path"
  echo "  Linked: $label"
}

is_bundled_skill() {
  local skill_name="$1"
  [ -d "$BUNDLED_SKILLS/$skill_name" ]
}

# --- 1) skills/* -> .agents/skills and .claude/skills ------------------------

if [ -d "$BUNDLED_SKILLS" ]; then
  echo "Linking bundled skills from skills/:"
  linked_any=0
  for skill in "$BUNDLED_SKILLS"/*/; do
    [ -d "$skill" ] || continue
    skill_name="$(basename "$skill")"
    rel="$(bundled_rel_target "$skill_name")"
    ensure_symlink \
      "$AGENTS_SKILLS/$skill_name" \
      "$rel" \
      ".agents/skills/$skill_name -> skills/$skill_name"
    ensure_symlink \
      "$CLAUDE_SKILLS/$skill_name" \
      "$rel" \
      ".claude/skills/$skill_name -> skills/$skill_name"
    linked_any=1
  done
  if [ "$linked_any" -eq 0 ]; then
    echo "  (none found)"
  fi
else
  echo "Warning: $BUNDLED_SKILLS does not exist. Skipping bundled links."
fi

# --- 2) .agents/skills/* -> .claude/skills (existing behavior) ---------------

if [ ! -d "$AGENTS_SKILLS" ]; then
  echo "Warning: $AGENTS_SKILLS does not exist. Skipping .claude links."
  echo "Symlink setup complete!"
  exit 0
fi

# Remove stale whole-folder symlink if present
if [ -L "$CLAUDE_SKILLS" ]; then
  echo "Removing stale whole-folder symlink at .claude/skills"
  rm "$CLAUDE_SKILLS"
fi

mkdir -p "$CLAUDE_SKILLS"

echo "Linking skills into .claude/skills:"
for skill in "$AGENTS_SKILLS"/*/; do
  [ -d "$skill" ] || continue
  skill_name="$(basename "$skill")"

  # Already pointed at skills/<name> in step 1 — keep that direct link.
  if is_bundled_skill "$skill_name"; then
    continue
  fi

  # Preserve absolute targets (with trailing slash from the glob) so existing
  # .claude/skills links are not rewritten on every run.
  ensure_symlink \
    "$CLAUDE_SKILLS/$skill_name" \
    "$skill" \
    "$skill_name"
done

echo "Symlink setup complete!"

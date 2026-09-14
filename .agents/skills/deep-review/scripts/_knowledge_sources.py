"""Filesystem discovery and router parsing for review knowledge.

These helpers preserve lexical instruction scopes while finding canonical local
skill sources. They enumerate candidates and routes; applicability decisions and
source evidence remain in build_knowledge.
"""

from __future__ import annotations

import os
import re
from pathlib import Path

from _common import rel

INSTRUCTION_NAMES = {"AGENTS.md", "CLAUDE.md"}
SKILL_ROOTS = (".agents/skills", ".claude/skills", ".codex/skills", "skills")
IGNORED_DIRS = {".git", ".deep-review", "node_modules", "vendor", "dist", "build", ".next", "target", "__pycache__"}
REFERENCE_RE = re.compile(r"(?<![A-Za-z0-9_./:-])(?P<path>(?:\./)?(?:references|assets)/[A-Za-z0-9_./-]+\.md)", re.I)


def lexical_rel(path: Path, repo: Path) -> str:
    """Keep the location that supplies instruction scope, even for symlinks."""
    try:
        return path.relative_to(repo).as_posix()
    except ValueError:
        return str(path)


def instruction_paths(repo: Path, selected: list[str]) -> list[Path]:
    ancestors = {repo}
    for name in selected:
        path = Path(name)
        if path.is_absolute() or ".." in path.parts:
            raise ValueError(f"manifest selected path must be repository-relative: {name!r}")
        ancestors.update(repo / parent for parent in path.parents)
    found = []
    for parent in sorted(ancestors):
        for name in sorted(INSTRUCTION_NAMES):
            source = parent / name
            if source.is_symlink() and not source.exists():
                raise ValueError(f"broken instruction symlink: {source}")
            if source.is_file():
                found.append(source)
    return found


def walk_skills(repo: Path) -> list[Path]:
    """Follow installed skill symlinks, preserving aliases and stopping cycles."""
    found: set[Path] = set()

    def visit(directory: Path, ancestors: frozenset[Path]) -> None:
        canonical = directory.resolve()
        if canonical in ancestors:
            return
        with os.scandir(directory) as entries:
            children = sorted(entries, key=lambda entry: entry.name)
        for child in children:
            if child.name == "SKILL.md" and child.is_file(follow_symlinks=True):
                found.add(Path(child.path))
            elif child.name not in IGNORED_DIRS and child.is_dir(follow_symlinks=True):
                visit(Path(child.path), ancestors | {canonical})

    for name in SKILL_ROOTS:
        root = repo / name
        if root.is_symlink() and not root.exists():
            raise ValueError(f"broken skill-root symlink: {root}")
        if root.is_dir():
            visit(root, frozenset())
    return sorted(found)


def frontmatter(text: str) -> dict[str, str]:
    lines = text.splitlines()
    if not lines or lines[0] != "---":
        return {}
    try:
        end = lines.index("---", 1)
    except ValueError:
        return {}
    values: dict[str, str] = {}
    index = 1
    while index < end:
        match = re.match(r"^([a-zA-Z0-9_-]+):\s*(.*)$", lines[index])
        if not match:
            index += 1
            continue
        key, value = match.group(1), match.group(2).strip().strip("\"'")
        if value in {">", ">-", "|", "|-"}:
            block: list[str] = []
            index += 1
            while index < end and (not lines[index].strip() or lines[index].startswith((" ", "\t"))):
                if lines[index].strip():
                    block.append(lines[index].strip())
                index += 1
            values[key] = " ".join(block)
        else:
            values[key] = value
            index += 1
    return values


def direct_references(skill: Path, repo: Path, text: str) -> dict[str, list[dict]]:
    refs: dict[str, list[dict]] = {}
    for line_number, line in enumerate(text.splitlines(), 1):
        for name in sorted({match.group("path") for match in REFERENCE_RE.finditer(line)}):
            candidate = skill.parent / name
            if not candidate.is_file():
                raise ValueError(f"missing direct reference {name!r} from {lexical_rel(skill, repo)}:{line_number}; repair the route or explicitly exclude its parent before compiling")
            hint = {"line": line_number, "text": line.strip()}
            hints = refs.setdefault(rel(candidate, repo), [])
            if hint not in hints:
                hints.append(hint)
    return refs

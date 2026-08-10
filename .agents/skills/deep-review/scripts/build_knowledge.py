#!/usr/bin/env python3
"""Discover repository-local review knowledge (bootstrap; writes under --out).

Builds knowledge.json from the selected manifest paths and creates a
rules.template.json accounting skeleton. Root/nested AGENTS.md and CLAUDE.md
are directory-scoped. Repo-local SKILL.md files are candidates when an
applicable instruction explicitly dispatches them or their metadata matches
the changed paths/repository technology signals. Direct markdown references
of candidate skills are included so their load decision is also auditable.

The orchestrator reads every pending source, copies rules.template.json to
rules.json, extracts only verdict-bearing rules verbatim, and changes every
pending status to applied or not-applicable with a concrete reason.

Exit codes: 0 ok, 1 missing/invalid manifest or filesystem error.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path

sys.dont_write_bytecode = True

from _common import manifest_selected, read_json, rel, repo_root, write_json

INSTRUCTION_NAMES = {"AGENTS.md", "CLAUDE.md"}
CONFIG_SOURCES = (".deep-review.yaml", ".deep-review.yml", ".coderabbit.yaml", ".coderabbit.yml")
SKILL_ROOTS = (".agents/skills", ".claude/skills", ".codex/skills", "skills")
REPO_SIGNAL_FILES = (
    "package.json", "go.mod", "Cargo.toml", "pyproject.toml", "requirements.txt",
    "bun.lock", "pnpm-lock.yaml", "yarn.lock", "package-lock.json",
)
IGNORED_DIRS = {
    ".git", ".deep-review", "node_modules", "vendor", "dist", "build",
    ".next", "target", "__pycache__",
}
STOPWORDS = {
    "about", "after", "agent", "agents", "best", "build", "building", "code",
    "comprehensive", "create", "creating", "development", "expert", "files",
    "guide", "implement", "project", "skill", "skills", "using", "when", "with",
}
REFERENCE_RE = re.compile(
    r"(?P<path>(?:references|assets)/[A-Za-z0-9_./-]+\.md)", re.I
)


def walk_named(repo: Path, names: set[str]) -> list[Path]:
    found: list[Path] = []
    for root, dirs, files in os.walk(repo, followlinks=False):
        dirs[:] = sorted(d for d in dirs if d not in IGNORED_DIRS)
        for name in sorted(set(files) & names):
            found.append(Path(root) / name)
    return sorted(found)


def walk_skills(repo: Path) -> list[Path]:
    found: set[Path] = set()
    for root_name in SKILL_ROOTS:
        root = repo / root_name
        if not root.is_dir():
            continue
        for walk_root, dirs, files in os.walk(root, followlinks=False):
            dirs[:] = sorted(d for d in dirs if d not in IGNORED_DIRS)
            if "SKILL.md" in files:
                found.add((Path(walk_root) / "SKILL.md").resolve())
    return sorted(found)


def scope_for(repo: Path, source: Path) -> list[str]:
    parent = rel(source.parent, repo)
    return ["**/*"] if parent == "." else [f"{parent}/**"]


def path_in_scope(path: str, source: Path, repo: Path) -> bool:
    parent = rel(source.parent, repo)
    return parent == "." or path == parent or path.startswith(parent + "/")


def tokens(value: str) -> set[str]:
    return {
        token for token in re.findall(r"[a-z0-9][a-z0-9+#.-]+", value.lower())
        if len(token) >= 3 and token not in STOPWORDS
    }


def frontmatter(text: str) -> dict[str, str]:
    if not text.startswith("---\n"):
        return {}
    end = text.find("\n---", 4)
    if end < 0:
        return {}
    lines = text[4:end].splitlines()
    values: dict[str, str] = {}
    index = 0
    while index < len(lines):
        match = re.match(r"^([a-zA-Z0-9_-]+):\s*(.*)$", lines[index])
        if not match:
            index += 1
            continue
        key, value = match.group(1), match.group(2).strip().strip("\"'")
        if value in {">", ">-", "|", "|-"}:
            block: list[str] = []
            index += 1
            while index < len(lines) and (not lines[index].strip() or lines[index].startswith((" ", "\t"))):
                if lines[index].strip():
                    block.append(lines[index].strip())
                index += 1
            values[key] = " ".join(block)
            continue
        values[key] = value
        index += 1
    return values


def direct_references(skill: Path, repo: Path, text: str) -> list[str]:
    refs: set[str] = set()
    for match in REFERENCE_RE.finditer(text):
        candidate = (skill.parent / match.group("path")).resolve()
        try:
            candidate.relative_to(repo.resolve())
        except ValueError:
            continue
        if candidate.is_file():
            refs.add(rel(candidate, repo))
    return sorted(refs)


def repository_tokens(repo: Path, selected: list[str]) -> set[str]:
    signal = tokens(" ".join(selected))
    for name in REPO_SIGNAL_FILES:
        path = repo / name
        if path.is_file():
            signal |= tokens(path.read_text(encoding="utf-8", errors="replace")[:200_000])
    return signal


def explicit_dispatches(
    name: str, skill_path: str, instructions: list[dict], instruction_text: dict[str, str]
) -> tuple[list[str], list[str]]:
    sources, paths = [], []
    name_pattern = re.compile(rf"(?<![a-z0-9-])\$?{re.escape(name.lower())}(?![a-z0-9-])")
    for source in instructions:
        if not source["applies_to"]:
            continue
        text = instruction_text[source["path"]].lower()
        if name_pattern.search(text) or skill_path.lower() in text:
            sources.append(source["path"])
            paths.extend(source["applies_to"])
    return sorted(set(sources)), sorted(set(paths))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    repo = repo_root()
    out = Path(args.out).resolve()
    try:
        manifest = read_json(out / "manifest.json")
        selected = sorted(manifest_selected(manifest))
        signal_tokens = repository_tokens(repo, selected)

        instruction_text: dict[str, str] = {}
        instructions: list[dict] = []
        for source in walk_named(repo, INSTRUCTION_NAMES):
            path = rel(source, repo)
            applies_to = [item for item in selected if path_in_scope(item, source, repo)]
            instruction_text[path] = source.read_text(encoding="utf-8", errors="replace")
            instructions.append({
                "path": path,
                "kind": "instruction",
                "scope": scope_for(repo, source),
                "applies_to": applies_to,
                "precedence": len(source.relative_to(repo).parts) - 1,
                "candidate": bool(applies_to),
                "candidate_reason": (
                    f"directory-scoped instructions apply to {len(applies_to)} selected path(s)"
                    if applies_to else "directory scope contains no selected path"
                ),
            })

        supplemental: list[dict] = []
        for source_name in CONFIG_SOURCES:
            source = repo / source_name
            if source.is_file():
                supplemental.append({
                    "path": source_name,
                    "kind": "config",
                    "scope": ["**/*"],
                    "applies_to": selected,
                    "candidate": bool(selected),
                    "candidate_reason": "repository review configuration can define path instructions",
                })
        learnings = repo / ".deep-review" / "learnings.md"
        if learnings.is_file():
            supplemental.append({
                "path": rel(learnings, repo),
                "kind": "learning",
                "scope": ["**/*"],
                "applies_to": selected,
                "candidate": bool(selected),
                "candidate_reason": "repository review learnings can refine finding validity",
            })

        skills: list[dict] = []
        references: list[dict] = []
        for skill in walk_skills(repo):
            path = rel(skill, repo)
            text = skill.read_text(encoding="utf-8", errors="replace")
            metadata = frontmatter(text)
            name = metadata.get("name") or skill.parent.name
            description = metadata.get("description", "")
            dispatch_sources, dispatched_paths = explicit_dispatches(
                name, path, instructions, instruction_text
            )
            overlap = sorted(tokens(f"{name} {description}") & signal_tokens)
            candidate = bool(dispatch_sources or overlap)
            if dispatch_sources:
                reason = f"explicitly dispatched by {', '.join(dispatch_sources)}"
                applies_to = dispatched_paths
            elif overlap:
                reason = f"metadata matches repository/change signals: {', '.join(overlap[:8])}"
                applies_to = selected
            else:
                reason = "no explicit dispatch or metadata match for this change"
                applies_to = []
            ref_paths = direct_references(skill, repo, text)
            skills.append({
                "path": path,
                "kind": "skill",
                "name": name,
                "description": description,
                "scope": ["**/*"],
                "applies_to": applies_to,
                "candidate": candidate,
                "candidate_reason": reason,
                "dispatch_sources": dispatch_sources,
                "references": ref_paths,
            })
            for ref_path in ref_paths:
                references.append({
                    "path": ref_path,
                    "kind": "skill-reference",
                    "parent_skill": path,
                    "scope": ["**/*"],
                    "applies_to": applies_to,
                    "candidate": candidate,
                    "candidate_reason": (
                        f"direct reference of candidate skill {path}"
                        if candidate else f"parent skill {path} is not applicable"
                    ),
                })

        sources = sorted(
            [*instructions, *supplemental, *skills, *references],
            key=lambda item: ({
                "config": 0, "learning": 1, "instruction": 2,
                "skill": 3, "skill-reference": 4,
            }[item["kind"]], item["path"]),
        )
        knowledge = {
            "target": manifest["target"],
            "selected_paths": selected,
            "sources": sources,
            "summary": {
                "instructions": len(instructions),
                "supplemental": len(supplemental),
                "skills": len(skills),
                "skill_references": len(references),
                "pending_candidates": sum(1 for source in sources if source["candidate"]),
            },
        }
        template_sources = []
        for source in sources:
            pending = source["candidate"]
            template_sources.append({
                "source": source["path"],
                "kind": source["kind"],
                "status": "pending" if pending else "not-applicable",
                "reason": (
                    "read this source in full and classify it before building jobs"
                    if pending else source["candidate_reason"]
                ),
            })
        write_json(out / "knowledge.json", knowledge)
        write_json(out / "rules.template.json", {"sources": template_sources, "rules": []})
    except (OSError, RuntimeError, ValueError) as error:
        sys.stderr.write(f"{error}\n")
        return 1

    summary = knowledge["summary"]
    print(f"knowledge registry -> {out / 'knowledge.json'}")
    print(
        f"sources: {summary['instructions']} instructions + {summary['supplemental']} config/learnings + "
        f"{summary['skills']} skills + "
        f"{summary['skill_references']} direct references; "
        f"{summary['pending_candidates']} need an applied/not-applicable decision"
    )
    print(f"rules accounting skeleton -> {out / 'rules.template.json'}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

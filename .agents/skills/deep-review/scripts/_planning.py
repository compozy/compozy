"""Deterministic cohort partitions shared by preparation and the job gate."""
from __future__ import annotations

import re
from collections import Counter, defaultdict
from pathlib import Path

DEFAULT_MAX_COHORT_FILES = 200
MAX_COHORT_CHANGED_LINES = 15000
DEFAULT_MAX_POLISH_FILES = 200
MAX_POLISH_CHANGED_LINES = 15000

def canonical_hunk(hunk: dict) -> tuple[int, int, str]:
    return int(hunk["start"]), int(hunk["lines"]), str(hunk.get("side", "new"))



def validate_cohorts(
    cohorts: list[dict], selected: dict[str, dict], max_cohort_files: int,
    max_lines: int = MAX_COHORT_CHANGED_LINES,
) -> list[str]:
    errors: list[str] = []
    seen_ids: set[str] = set()
    full_owners: dict[str, list[str]] = defaultdict(list)
    scoped_owners: dict[str, list[tuple[str, tuple[int, int, str]]]] = defaultdict(list)
    for cohort in cohorts:
        cohort_id = cohort.get("id")
        if not isinstance(cohort_id, str) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]*", cohort_id) or cohort_id in seen_ids:
            errors.append(f"duplicate or missing cohort id {cohort_id!r}")
        seen_ids.add(cohort_id)
        if cohort.get("risk") not in {"high", "normal", "low"}:
            errors.append(f"{cohort_id}: risk must be high|normal|low")
        files = cohort.get("files", [])
        if not files or len(files) > max_cohort_files:
            errors.append(
                f"{cohort_id}: invalid file count {len(files)} (1..{max_cohort_files})"
            )
        scope = cohort.get("hunk_scope") or {}
        extra_scope = set(scope) - set(files)
        if extra_scope:
            errors.append(f"{cohort_id}: hunk_scope paths absent from files: {sorted(extra_scope)}")
        for path in files:
            if path not in selected:
                errors.append(f"{cohort_id}: non-selected or unknown path {path}")
                continue
            if path in scope:
                if not scope[path] and not selected[path]["hunks"]:
                    # Mode-only changes and empty files still need one file owner.
                    full_owners[path].append(cohort_id)
                for hunk in scope[path]:
                    scoped_owners[path].append((cohort_id, canonical_hunk(hunk)))
            else:
                full_owners[path].append(cohort_id)
        if scope:
            scoped_lines = sum(
                int(h["lines"]) for path in files if path in selected
                for h in scope.get(path, selected[path]["hunks"])
            )
            if scoped_lines > max_lines:
                errors.append(f"{cohort_id}: scoped changed lines {scoped_lines} > {max_lines}")
        else:
            changed = sum(
                int(selected[path].get("adds") or 0) + int(selected[path].get("dels") or 0)
                for path in files
                if path in selected
            )
            if changed > max_lines:
                errors.append(
                    f"{cohort_id}: changed lines {changed} > {max_lines} without hunk_scope"
                )

    for path, item in selected.items():
        full, scoped = full_owners.get(path, []), scoped_owners.get(path, [])
        if full and scoped:
            errors.append(f"{path}: mixed full and scoped ownership")
        elif full:
            if len(full) != 1:
                errors.append(f"{path}: owned by {len(full)} cohorts ({full})")
        elif scoped:
            want = Counter(
                (side, line)
                for start, lines, side in (canonical_hunk(h) for h in item["hunks"])
                for line in range(start, start + lines)
            )
            got = Counter(
                (side, line)
                for _, (start, lines, side) in scoped
                for line in range(start, start + lines)
            )
            if got != want:
                errors.append(
                    f"{path}: hunk slice mismatch missing_lines={sum((want - got).values())} "
                    f"duplicated_or_extra_lines={sum((got - want).values())}"
                )
        else:
            errors.append(f"{path}: missing cohort ownership")
    return errors



def split_hunk(hunk: dict, limit: int) -> list[dict]:
    start, remaining = int(hunk["start"]), int(hunk["lines"])
    side, chunks = str(hunk.get("side", "new")), []
    while remaining:
        size = min(limit, remaining)
        chunks.append({"start": start, "lines": size, "side": side})
        start += size
        remaining -= size
    return chunks


def polish_cohorts(
    cohorts: list[dict], selected: dict[str, dict], max_files: int, max_lines: int
) -> list[dict]:
    """Partition owned hunks without dropping files, including oversized hunks."""
    result: list[dict] = []
    for cohort in cohorts:
        units: list[tuple[str, list[dict]]] = []
        source_scope = cohort.get("hunk_scope") or {}
        for path in cohort["files"]:
            hunks = source_scope.get(path) or selected[path]["hunks"]
            expanded = [piece for hunk in hunks for piece in split_hunk(hunk, max_lines)]
            current: list[dict] = []
            current_lines = 0
            for hunk in expanded:
                lines = int(hunk["lines"])
                if current and current_lines + lines > max_lines:
                    units.append((path, current))
                    current, current_lines = [], 0
                current.append(hunk)
                current_lines += lines
            if current or not expanded:
                units.append((path, current))

        batch: dict[str, list[dict]] = {}
        batch_lines = 0

        def flush() -> None:
            nonlocal batch, batch_lines
            if not batch:
                return
            index = len([item for item in result if item["parent_id"] == cohort["id"]]) + 1
            result.append({
                "id": f"{cohort['id']}-p{index:02d}",
                "parent_id": cohort["id"],
                "name": f"{cohort['name']} — polish {index}",
                "risk": cohort["risk"],
                "files": list(batch),
                "hunk_scope": {path: hunks for path, hunks in batch.items()},
            })
            batch, batch_lines = {}, 0

        for path, hunks in units:
            unit_lines = sum(int(hunk["lines"]) for hunk in hunks)
            adds_file = path not in batch
            if batch and (
                batch_lines + unit_lines > max_lines
                or (adds_file and len(batch) >= max_files)
                or path in batch
            ):
                flush()
            batch[path] = hunks
            batch_lines += unit_lines
        flush()
    return result


def automatic_plan(selected: dict[str, dict], max_files: int = DEFAULT_MAX_COHORT_FILES,
                   repo: Path | None = None, max_context_lines: int | None = None,
                   max_lines: int = MAX_COHORT_CHANGED_LINES) -> dict:
    """Package-oriented default; semantic exceptions remain explicit plan overrides."""
    groups: dict[str, list[str]] = defaultdict(list)
    context_lines: dict[str, int] = {}
    for path in sorted(selected):
        parts = Path(path).parts
        group = "/".join(parts[:2]) if len(parts) > 2 and parts[0] in {"internal", "packages", "apps", "web"} else parts[0] if len(parts) > 1 else "root"
        groups[group].append(path)
        file = repo / path if repo else None
        context_lines[path] = len(file.read_bytes().splitlines()) if file and file.is_file() else sum(int(h["lines"]) for h in selected[path]["hunks"])
    cohorts = []
    for group, files in sorted(groups.items()):
        # Keep source and tests adjacent within each package; the planner never guesses callers.
        files.sort(key=lambda path: (re.sub(r"(?:_test|[.]test|[.]spec)(?=[.])", "", path), path))
        seeds = []
        batch, size = [], 0
        for path in files:
            if max_context_lines and batch and size + context_lines[path] > max_context_lines:
                seeds.append(batch)
                batch, size = [], 0
            batch.append(path)
            size += context_lines[path]
        if batch:
            seeds.append(batch)
        for seed in seeds:
            risk = "high" if any(re.search(r"(?:auth|security|migrat|storage|store|concurr|protocol)", p, re.I) for p in seed) else "low" if all(Path(p).suffix in {".md", ".txt", ".json", ".yaml", ".yml"} for p in seed) else "normal"
            parent = {"id": "auto", "name": group, "risk": risk, "files": seed}
            for item in polish_cohorts([parent], selected, max_files, max_lines):
                item.update(id=f"c{len(cohorts) + 1:03d}", name=group)
                item.pop("parent_id", None)
                item["context_lines"] = sum(context_lines[path] for path in item["files"])
                cohorts.append(item)
    return {"cohorts": cohorts, "sweeps": [], "generated": True,
            "limits": {"cohort_files": max_files, "cohort_changed_lines": max_lines,
                       "polish_files": DEFAULT_MAX_POLISH_FILES, "polish_changed_lines": MAX_POLISH_CHANGED_LINES,
                       "context_lines": max_context_lines}}

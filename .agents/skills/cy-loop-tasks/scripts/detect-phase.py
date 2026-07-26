#!/usr/bin/env python3
"""
detect-phase.py -- read-only.

Prints the next phase + action for ``cy-loop-tasks`` to take. The agent
calls this at the start of every iteration. The output drives the rest
of the iteration deterministically. Filesystem (not state.yaml) is the
ultimate source of truth for task and review-round status; state.yaml
only mirrors what is fast to compute.

Usage:
    detect-phase.py <slug> [--tasks-root .compozy/tasks]

Output (single line, key=value space-separated):
    phase=0 action=bootstrap
    phase=B action=execute_task task=<stem>             # mode=tasks
    phase=B action=execute_task task=<stem> lane=frontend agent=<claude|cursor>
    phase=B action=execute_free_slice                   # mode=free
    phase=C action=qa_report
    phase=C action=qa_execution
    phase=D action=peer_review round=<N>
    phase=E action=done

Exits:
    0 always (output describes the situation; missing slug => bootstrap)
    1 unrecoverable error reading state.yaml or filesystem
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import load  # noqa: E402


_FRONTMATTER = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)


def _read_frontmatter(md_path: Path) -> dict[str, str]:
    if not md_path.exists():
        return {}
    text = md_path.read_text(encoding="utf-8", errors="replace")
    m = _FRONTMATTER.match(text)
    if not m:
        return {}
    fm: dict[str, str] = {}
    for line in m.group(1).splitlines():
        if ":" in line:
            k, _, v = line.partition(":")
            fm[k.strip()] = v.strip().strip("'\"")
    return fm


def _is_qa_task(slug_dir: Path, stem: str) -> bool:
    fm = _read_frontmatter(slug_dir / f"{stem}.md")
    type_field = fm.get("type", "").lower()
    return type_field in {"qa-report", "qa-execution"}


def _qa_kind(slug_dir: Path, stem: str) -> str:
    fm = _read_frontmatter(slug_dir / f"{stem}.md")
    type_field = fm.get("type", "").lower()
    if type_field == "qa-execution":
        return "qa_execution"
    return "qa_report"


def emit(line: str) -> None:
    print(line)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("slug")
    ap.add_argument("--tasks-root", default=".compozy/tasks")
    args = ap.parse_args()

    slug_dir = Path(args.tasks_root) / args.slug
    state_path = slug_dir / "state.yaml"

    # Phase 0: bootstrap if state.yaml is missing.
    if not state_path.exists():
        emit("phase=0 action=bootstrap")
        return 0

    try:
        state = load(state_path)
    except Exception as exc:  # noqa: BLE001
        print(f"detect-phase: failed to parse {state_path}: {exc}", file=sys.stderr)
        return 1

    mode = state.get("mode")
    verify_status = state.get("verify", {}).get("last_status")
    qa = state.get("qa", {})

    # Phase B and C ordering depends on mode.
    if mode == "tasks":
        pending = list(state.get("tasks", {}).get("pending") or [])
        if pending:
            head = pending[0]
            if _is_qa_task(slug_dir, head):
                kind = _qa_kind(slug_dir, head)
                emit(f"phase=C action={kind}")
                return 0
            line = f"phase=B action=execute_task task={head}"
            frontend_agent = state.get("frontend_agent")
            if frontend_agent:
                fm = _read_frontmatter(slug_dir / f"{head}.md")
                if fm.get("type", "").lower() == "frontend":
                    line += f" lane=frontend agent={frontend_agent}"
            emit(line)
            return 0
        # No pending: fall through to QA / D
    elif mode == "free":
        progress = state.get("progress", {}) or {}
        if not progress.get("deliverables_complete", False):
            emit("phase=B action=execute_free_slice")
            return 0
    else:
        print(
            f"detect-phase: unknown mode {mode!r} in {state_path}",
            file=sys.stderr,
        )
        return 1

    # Phase C: QA artifacts not yet produced.
    if not qa.get("report_done", False):
        emit("phase=C action=qa_report")
        return 0
    if not qa.get("execution_done", False):
        emit("phase=C action=qa_execution")
        return 0

    # Phase D: QA complete but no SHIP verdict yet.
    review = state.get("review", {}) or {}
    next_round = int(review.get("rounds", 0) or 0) + 1
    if not review.get("ship", False):
        emit(f"phase=D action=peer_review round={next_round}")
        return 0

    # Phase E: QA complete AND review SHIP AND verify PASS.
    if verify_status == "PASS":
        emit("phase=E action=done")
        return 0
    # A SHIP verdict on a tree that no longer verifies is void; re-enter
    # peer review so the next round fixes the tree and re-judges it.
    emit(f"phase=D action=peer_review round={next_round}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

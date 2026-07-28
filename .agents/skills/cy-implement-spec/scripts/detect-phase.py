#!/usr/bin/env python3
"""
detect-phase.py -- read-only.

Prints the next phase + action for ``cy-implement-spec`` to take. The
agent calls this at the start of every iteration; the output drives the
rest of the iteration deterministically. There is no task queue: Phase B
re-emits ``implement`` (with criteria counters) until the agent proves
every spec criterion and flips ``progress.implementation_complete``.

Usage:
    detect-phase.py <slug> [--tasks-root .compozy/tasks]

Output (single line, key=value space-separated):
    phase=0 action=bootstrap
    phase=B action=implement pending=<n> met=<m>
    phase=C action=qa_report
    phase=C action=qa_execution
    phase=D action=peer_review round=<N>
    phase=E action=done

Exits:
    0 always (output describes the situation; missing state => bootstrap)
    1 unrecoverable error reading state.yaml (parse failure, empty
      criteria list -- state.yaml was not written by init-state.py)
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import load  # noqa: E402


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

    progress = state.get("progress", {}) or {}
    criteria = list(progress.get("criteria") or [])

    # Phase B: implementation runs until every criterion is proven.
    if not progress.get("implementation_complete", False):
        if not criteria:
            print(
                f"detect-phase: {state_path} has an empty progress.criteria "
                "list; state.yaml was not created by init-state.py -- repair "
                "or recreate it",
                file=sys.stderr,
            )
            return 1
        pending = sum(1 for c in criteria if c.get("status") != "met")
        met = len(criteria) - pending
        emit(f"phase=B action=implement pending={pending} met={met}")
        return 0

    # Phase C: QA artifacts not yet produced.
    qa = state.get("qa", {}) or {}
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
    if state.get("verify", {}).get("last_status") == "PASS":
        emit("phase=E action=done")
        return 0
    # A SHIP verdict on a tree that no longer verifies is void; re-enter
    # peer review so the next round fixes the tree and re-judges it.
    emit(f"phase=D action=peer_review round={next_round}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

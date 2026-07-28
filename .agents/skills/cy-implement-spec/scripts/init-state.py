#!/usr/bin/env python3
"""
init-state.py -- bootstrap helper (mutating).

Creates ``.compozy/tasks/<slug>/state.yaml`` for a cy-implement-spec run.
Requires at least one ``--criterion`` (the spec's exit conditions,
extracted by the agent from the spec documents at bootstrap). Refuses to
clobber an existing state.yaml and refuses slugs that carry an authored
task graph -- those belong to cy-loop-tasks. Read schema in
``references/state-schema.md``.

Usage:
    init-state.py <slug> --goal "<text>" --criterion "<text>"
                  [--criterion "<text>" ...] [--tasks-root <path>]

Exits:
    0 success
    1 generic error (stderr): empty or duplicate criterion, bad slug dir
    2 state.yaml already exists
    3 _techspec.md missing
    4 no --criterion provided
    5 task graph present (_tasks.md or task_*.md) -- use cy-loop-tasks
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import dump, now_iso  # noqa: E402


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("slug")
    ap.add_argument("--goal", required=True, help="verbatim goal_signature")
    ap.add_argument(
        "--criterion",
        action="append",
        default=[],
        help="one spec exit condition; repeat per criterion (>= 1 required)",
    )
    ap.add_argument(
        "--tasks-root",
        default=".compozy/tasks",
        help="root of feature spec dirs (default: .compozy/tasks)",
    )
    args = ap.parse_args()

    slug_dir = Path(args.tasks_root) / args.slug
    if not slug_dir.is_dir():
        print(f"init-state: slug directory not found: {slug_dir}", file=sys.stderr)
        return 1

    if not (slug_dir / "_techspec.md").exists():
        print(
            f"init-state: _techspec.md missing under {slug_dir}; author it first",
            file=sys.stderr,
        )
        return 3

    state_path = slug_dir / "state.yaml"
    if state_path.exists():
        print(
            f"init-state: {state_path} already exists; refusing to overwrite",
            file=sys.stderr,
        )
        return 2

    if (slug_dir / "_tasks.md").exists() or any(slug_dir.glob("task_*.md")):
        print(
            f"init-state: {slug_dir} carries an authored task graph "
            "(_tasks.md / task_*.md); drive it with cy-loop-tasks instead",
            file=sys.stderr,
        )
        return 5

    criteria_texts = [c.strip() for c in args.criterion]
    if not criteria_texts:
        print(
            "init-state: at least one --criterion is required; extract the "
            "spec's exit conditions from the spec documents first",
            file=sys.stderr,
        )
        return 4
    if any(not c for c in criteria_texts):
        print("init-state: --criterion text is empty after trim", file=sys.stderr)
        return 1
    if len(set(criteria_texts)) != len(criteria_texts):
        print(
            "init-state: duplicate --criterion text; each criterion must be "
            "unique so --criterion-met matches exactly one entry",
            file=sys.stderr,
        )
        return 1

    timestamp = now_iso()
    state = {
        "slug": args.slug,
        "created_at": timestamp,
        "last_updated": timestamp,
        "iteration": 0,
        "goal_signature": args.goal,
        "progress": {
            "implementation_complete": False,
            "criteria": [
                {"text": text, "status": "pending", "iteration": 0}
                for text in criteria_texts
            ],
        },
        "qa": {"report_done": False, "execution_done": False},
        "review": {"rounds": 0, "last_verdict": None, "ship": False},
        "verify": {"last_run": None, "last_status": None},
        "iterations": [],
    }
    dump(state, state_path)
    print(f"init-state: wrote {state_path} (criteria={len(criteria_texts)})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""
init-state.py -- bootstrap helper (mutating).

Creates ``.compozy/tasks/<slug>/state.yaml`` from the canonical template.
Auto-detects mode from filesystem unless overridden. Refuses to clobber
an existing state.yaml. Read schema in ``references/state-schema.md``.

Usage:
    init-state.py <slug> --goal "<text>" [--mode tasks|free]
                  [--frontend claude|cursor] [--tasks-root <path>]

Exits:
    0 success
    1 generic error (stderr)
    2 state.yaml already exists
    3 _techspec.md missing
    4 mode override conflicts with filesystem
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import dump, now_iso  # noqa: E402


def detect_mode(slug_dir: Path) -> str:
    tasks_md = slug_dir / "_tasks.md"
    has_task_files = any(slug_dir.glob("task_*.md"))
    return "tasks" if (tasks_md.exists() and has_task_files) else "free"


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("slug")
    ap.add_argument("--goal", required=True, help="verbatim goal_signature")
    ap.add_argument("--mode", choices=["tasks", "free"], default=None)
    ap.add_argument(
        "--frontend",
        choices=["claude", "cursor"],
        default=None,
        help="frontend worker agent; non-null activates the herdr frontend lane",
    )
    ap.add_argument(
        "--tasks-root",
        default=".compozy/tasks",
        help="root of feature task dirs (default: .compozy/tasks)",
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

    detected = detect_mode(slug_dir)
    chosen = args.mode or detected
    if args.mode and args.mode != detected:
        print(
            f"init-state: --mode {args.mode} conflicts with filesystem "
            f"(detected {detected}); reconcile before retrying",
            file=sys.stderr,
        )
        return 4

    completed: list[str] = []
    pending: list[str] = []
    total = 0
    if chosen == "tasks":
        task_files = sorted(p.name for p in slug_dir.glob("task_*.md"))
        total = len(task_files)
        for fname in task_files:
            stem = fname[: -len(".md")]
            pending.append(stem)

    timestamp = now_iso()
    state = {
        "slug": args.slug,
        "created_at": timestamp,
        "last_updated": timestamp,
        "mode": chosen,
        "iteration": 0,
        "goal_signature": args.goal,
        "frontend_agent": args.frontend,
        "tasks": {
            "total": total,
            "completed": completed,
            "current": None,
            "pending": pending,
        },
        "progress": {
            "deliverables_complete": False,
            "checklist": [],
        },
        "qa": {"report_done": False, "execution_done": False},
        "review": {"rounds": 0, "last_verdict": None, "ship": False},
        "verify": {"last_run": None, "last_status": None},
        "iterations": [],
    }
    dump(state, state_path)
    print(
        f"init-state: wrote {state_path} (mode={chosen}, total_tasks={total}, "
        f"frontend_agent={args.frontend or 'null'})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

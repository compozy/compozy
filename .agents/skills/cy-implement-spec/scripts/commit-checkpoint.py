#!/usr/bin/env python3
"""
commit-checkpoint.py -- mutating.

Checkpoint commit for ``cy-implement-spec``. Invoked by the orchestrator
at the end of every Phase B iteration and every Phase D peer-review round
(after update-state.py advances state) so each completed milestone or
review round becomes one atomic, restorable git commit.

Usage:
    commit-checkpoint.py <slug> --milestone "<text>"   # phase B
    commit-checkpoint.py <slug> --review-round <N>     # phase D
    commit-checkpoint.py <slug> [--coauthor "Name <email>"]
                         [--tasks-root <p>]

Behavior:
    1. Verify state.yaml exists under <tasks-root>/<slug>/.
    2. ``git status --porcelain``: empty tree -> print ``SKIP: no changes`` and exit 0.
    3. Build commit header:
         --milestone "<txt>" -> ``feat: <txt>`` (whitespace collapsed)
         --review-round N    -> ``fix: peer-review round <N> remediation``
       Hard-cap full header at 72 chars.
    4. Build body lines:
         ``Checkpoint via cy-implement-spec (iteration <N>, <phase label>).``
       plus, when --coauthor is given:
         (blank line)
         ``Co-Authored-By: <value>``
       Pass the driving agent's identity (e.g. ``Claude Fable 5
       <noreply@anthropic.com>`` or ``Codex <noreply@openai.com>``).
    5. ``git add -A`` then ``git commit -m <header> -m <body>``. No --amend,
       --no-verify, or --no-gpg-sign. Hook failures surface as exit 1.
    6. On success, print new commit SHA (``git rev-parse HEAD``) and exit 0.

Exits:
    0 success (commit created OR ``SKIP: no changes``)
    1 git command failure (commit, hook, rev-parse, ...)
    2 argument or state error (missing slug dir, state.yaml, flag misuse)
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import load  # noqa: E402


_HEADER_MAX = 72


def _collapse_ws(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip()


def _truncate(header: str, limit: int = _HEADER_MAX) -> str:
    if len(header) <= limit:
        return header
    return header[: limit - 1].rstrip() + "…"


def _build_milestone_header(text: str) -> str:
    body = _collapse_ws(text)
    if not body:
        raise SystemExit("commit-checkpoint: --milestone text is empty after trim")
    return _truncate(f"feat: {body}")


def _build_review_header(round_n: int) -> str:
    return _truncate(f"fix: peer-review round {round_n} remediation")


def _run_git(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        check=False,
        text=True,
        capture_output=True,
    )


def _tree_is_clean() -> bool:
    status = _run_git(["status", "--porcelain"])
    if status.returncode != 0:
        print(
            f"commit-checkpoint: git status failed: {status.stderr.strip()}",
            file=sys.stderr,
        )
        raise SystemExit(1)
    return status.stdout.strip() == ""


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("slug")
    ap.add_argument(
        "--milestone",
        dest="milestone_text",
        help="milestone text exactly as recorded in update-state (phase B)",
    )
    ap.add_argument(
        "--review-round",
        type=int,
        help="peer-review round number (phase D)",
    )
    ap.add_argument(
        "--coauthor",
        default=None,
        help='driving agent identity for the Co-Authored-By trailer, e.g. "Codex <noreply@openai.com>"',
    )
    ap.add_argument("--tasks-root", default=".compozy/tasks")
    args = ap.parse_args()

    provided = [
        flag
        for flag, value in (
            ("--milestone", args.milestone_text),
            ("--review-round", args.review_round),
        )
        if value is not None
    ]
    if len(provided) != 1:
        print(
            "commit-checkpoint: provide exactly one of --milestone \"<text>\" "
            "or --review-round <N>",
            file=sys.stderr,
        )
        return 2
    if args.review_round is not None and args.review_round < 1:
        print(
            "commit-checkpoint: --review-round must be >= 1",
            file=sys.stderr,
        )
        return 2

    slug_dir = Path(args.tasks_root) / args.slug
    state_path = slug_dir / "state.yaml"
    if not state_path.exists():
        print(
            f"commit-checkpoint: {state_path} missing; run init-state.py first",
            file=sys.stderr,
        )
        return 2

    if _tree_is_clean():
        print("SKIP: no changes")
        return 0

    try:
        state = load(state_path)
    except Exception as exc:  # noqa: BLE001
        print(
            f"commit-checkpoint: failed to parse {state_path}: {exc}",
            file=sys.stderr,
        )
        return 1

    iteration = int(state.get("iteration", 0))

    if args.milestone_text is not None:
        header = _build_milestone_header(args.milestone_text)
        phase_label = "phase B milestone"
    else:
        header = _build_review_header(args.review_round)
        phase_label = f"phase D review round {args.review_round}"

    body = f"Checkpoint via cy-implement-spec (iteration {iteration}, {phase_label})."
    if args.coauthor:
        body += f"\n\nCo-Authored-By: {args.coauthor.strip()}"

    add = _run_git(["add", "-A"])
    if add.returncode != 0:
        print(
            f"commit-checkpoint: git add -A failed: {add.stderr.strip()}",
            file=sys.stderr,
        )
        return 1

    commit = _run_git(["commit", "-m", header, "-m", body])
    if commit.returncode != 0:
        sys.stderr.write(commit.stderr)
        sys.stderr.write(commit.stdout)
        return 1

    sha = _run_git(["rev-parse", "HEAD"])
    if sha.returncode != 0:
        print(
            f"commit-checkpoint: git rev-parse HEAD failed: {sha.stderr.strip()}",
            file=sys.stderr,
        )
        return 1
    print(sha.stdout.strip())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

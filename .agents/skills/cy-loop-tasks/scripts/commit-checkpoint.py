#!/usr/bin/env python3
"""
commit-checkpoint.py -- mutating.

Checkpoint commit for ``cy-loop-tasks``. Invoked by the orchestrator at
the end of every Phase B iteration and every Phase D peer-review round
(after update-state.py advances state) so each completed task, slice, or
review round becomes one atomic, restorable git commit.

Usage:
    commit-checkpoint.py <slug> --task <stem>        # phase B, mode=tasks
    commit-checkpoint.py <slug> --slice "<text>"     # phase B, mode=free
    commit-checkpoint.py <slug> --review-round <N>   # phase D
    commit-checkpoint.py <slug> --task <stem> --body-note "<text>"
    commit-checkpoint.py <slug> [--tasks-root <p>]

Behavior:
    1. Verify state.yaml exists under <tasks-root>/<slug>/.
    2. ``git status --porcelain``: empty tree -> print ``SKIP: no changes`` and
       exit 0 (stacked mode with an active stack still re-submits first; see
       Stacked mode below).
    3. Build commit header:
         --task task_NN -> ``feat: <title from task_NN.md frontmatter> #<NN>``
         --slice "<txt>" -> ``feat: <txt>`` (whitespace collapsed)
         --review-round N -> ``fix: peer-review round <N> remediation``
       Hard-cap full header at 72 chars.
    4. Build body lines:
         ``Checkpoint via cy-loop-tasks (iteration <N>, <phase label>).``
         (blank line)
         optional ``--body-note`` text
         (blank line)
         ``Co-Authored-By: Codex <noreply@openai.com>``
    5. ``git add -A`` then ``git commit -m <header> -m <body>``. No --amend,
       --no-verify, or --no-gpg-sign. Hook failures surface as exit 1.
    6. On success, print new commit SHA (``git rev-parse HEAD``) and exit 0.

Stacked mode (``state.yaml`` has ``stacked: true``; set at bootstrap via
``init-state.py --stacked``, tasks mode only):
    - --task task_NN: before committing, ensure the layer branch
      ``<slug>/task-NN`` is the checked-out stack top — first layer via
      ``gh stack init <layer> --base <current branch>``, later layers via
      ``gh stack add <layer>``. The commit lands on the layer branch.
    - --review-round N: commit rides the current stack branch (no new layer);
      the current branch must already belong to a stack.
    - After every successful commit — and on a clean tree while inside a
      stack — run ``gh stack submit --auto`` so every layer's PR is created
      or refreshed. The SHA (or ``SKIP: no changes``) stays the first stdout
      line; a ``stack: submitted`` line follows.
    - Not in a stack but ``refs/heads/<slug>/task-*`` branches exist -> exit 2
      (resume drift: run ``gh stack checkout`` + ``gh stack top`` first).
    - --slice is incompatible with stacked mode -> exit 2.
    - The gh binary honors the ``CY_LOOP_GH`` env override (tests use a stub).

Exits:
    0 success (commit created OR ``SKIP: no changes``)
    1 git or gh command failure (commit, hook, rev-parse, stack submit, ...)
    2 argument or state error (missing slug dir, state.yaml, flag misuse,
      stacked-mode drift)
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import load  # noqa: E402


_FRONTMATTER = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)
_TASK_STEM = re.compile(r"^task_(\d+)$")
_HEADER_MAX = 72
_COAUTHOR = "Co-Authored-By: Codex <noreply@openai.com>"


def _read_frontmatter(md_path: Path) -> dict[str, str]:
    if not md_path.exists():
        return {}
    text = md_path.read_text(encoding="utf-8", errors="replace")
    match = _FRONTMATTER.match(text)
    if not match:
        return {}
    fm: dict[str, str] = {}
    for line in match.group(1).splitlines():
        if ":" not in line:
            continue
        key, _, value = line.partition(":")
        fm[key.strip()] = value.strip().strip("'\"")
    return fm


def _collapse_ws(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip()


def _truncate(header: str, limit: int = _HEADER_MAX) -> str:
    if len(header) <= limit:
        return header
    return header[: limit - 1].rstrip() + "…"


def _build_task_header(task_md: Path, stem: str) -> str:
    match = _TASK_STEM.match(stem)
    if not match:
        raise SystemExit(
            f"commit-checkpoint: --task value {stem!r} must look like task_NN"
        )
    nn = match.group(1)
    title = _collapse_ws(_read_frontmatter(task_md).get("title", "")) or stem
    return _truncate(f"feat: {title} #{nn}")


def _build_slice_header(text: str) -> str:
    body = _collapse_ws(text)
    if not body:
        raise SystemExit("commit-checkpoint: --slice text is empty after trim")
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


def _run_gh(args: list[str]) -> subprocess.CompletedProcess[str]:
    gh_bin = os.environ.get("CY_LOOP_GH", "gh")
    return subprocess.run(
        [gh_bin, *args],
        check=False,
        text=True,
        capture_output=True,
    )


def _current_branch() -> str:
    head = _run_git(["rev-parse", "--abbrev-ref", "HEAD"])
    if head.returncode != 0:
        print(
            f"commit-checkpoint: git rev-parse HEAD failed: {head.stderr.strip()}",
            file=sys.stderr,
        )
        raise SystemExit(1)
    branch = head.stdout.strip()
    if branch == "HEAD":
        print(
            "commit-checkpoint: detached HEAD is incompatible with stacked "
            "mode; check out a branch first",
            file=sys.stderr,
        )
        raise SystemExit(2)
    return branch


def _in_stack() -> bool:
    return _run_gh(["stack", "view"]).returncode == 0


def _layer_branches(slug: str) -> list[str]:
    refs = _run_git(
        [
            "for-each-ref",
            "--format=%(refname:short)",
            f"refs/heads/{slug}/task-*",
        ]
    )
    if refs.returncode != 0:
        print(
            f"commit-checkpoint: git for-each-ref failed: {refs.stderr.strip()}",
            file=sys.stderr,
        )
        raise SystemExit(1)
    return [line for line in refs.stdout.splitlines() if line.strip()]


def _ensure_task_layer(slug: str, stem: str) -> str:
    """Make ``<slug>/task-NN`` the checked-out stack layer; return its name."""
    nn = _TASK_STEM.match(stem).group(1)  # stem validated by _build_task_header
    layer = f"{slug}/task-{nn}"
    if _current_branch() == layer:
        return layer
    if _in_stack():
        add = _run_gh(["stack", "add", layer])
        if add.returncode != 0:
            print(
                f"commit-checkpoint: gh stack add {layer} failed: "
                f"{add.stderr.strip() or add.stdout.strip()}",
                file=sys.stderr,
            )
            raise SystemExit(1)
        return layer
    if _layer_branches(slug):
        print(
            f"commit-checkpoint: layer branches for {slug!r} exist but the "
            "current branch is not part of a stack; resume the stack first "
            "(`gh stack checkout`, then `gh stack top`) and retry",
            file=sys.stderr,
        )
        raise SystemExit(2)
    base = _current_branch()
    init = _run_gh(["stack", "init", layer, "--base", base])
    if init.returncode != 0:
        print(
            f"commit-checkpoint: gh stack init {layer} --base {base} failed: "
            f"{init.stderr.strip() or init.stdout.strip()}",
            file=sys.stderr,
        )
        raise SystemExit(1)
    return layer


def _submit_stack() -> None:
    submit = _run_gh(["stack", "submit", "--auto"])
    if submit.returncode != 0:
        print(
            "commit-checkpoint: gh stack submit --auto failed: "
            f"{submit.stderr.strip() or submit.stdout.strip()} "
            "(any local commit is preserved; rerunning the checkpoint "
            "re-submits)",
            file=sys.stderr,
        )
        raise SystemExit(1)
    print(f"stack: submitted ({_current_branch()})")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("slug")
    ap.add_argument("--task", help="task stem like task_07 (phase B, mode=tasks)")
    ap.add_argument("--slice", dest="slice_text", help="slice text (phase B, mode=free)")
    ap.add_argument(
        "--review-round",
        type=int,
        help="peer-review round number (phase D)",
    )
    ap.add_argument(
        "--body-note",
        help="optional provenance or context paragraph for the commit body",
    )
    ap.add_argument("--tasks-root", default=".compozy/tasks")
    args = ap.parse_args()

    provided = [
        flag
        for flag, value in (
            ("--task", args.task),
            ("--slice", args.slice_text),
            ("--review-round", args.review_round),
        )
        if value is not None
    ]
    if len(provided) != 1:
        print(
            "commit-checkpoint: provide exactly one of --task <stem>, "
            "--slice \"<text>\", or --review-round <N>",
            file=sys.stderr,
        )
        return 2
    if args.review_round is not None and args.review_round < 1:
        print(
            "commit-checkpoint: --review-round must be >= 1",
            file=sys.stderr,
        )
        return 2
    body_note = _collapse_ws(args.body_note or "")
    if args.body_note is not None and not body_note:
        print(
            "commit-checkpoint: --body-note is empty after trim",
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

    try:
        state = load(state_path)
    except Exception as exc:  # noqa: BLE001
        print(
            f"commit-checkpoint: failed to parse {state_path}: {exc}",
            file=sys.stderr,
        )
        return 1

    stacked = bool(state.get("stacked"))
    if stacked and args.slice_text is not None:
        print(
            "commit-checkpoint: --slice is incompatible with stacked mode "
            "(stack layers map to the authored task graph)",
            file=sys.stderr,
        )
        return 2

    if _tree_is_clean():
        print("SKIP: no changes")
        if stacked and _in_stack():
            # Heal a prior commit-ok/submit-failed run: re-submit is idempotent.
            _submit_stack()
        return 0

    iteration = int(state.get("iteration", 0))
    mode = state.get("mode") or "?"

    if args.task:
        task_md = slug_dir / f"{args.task}.md"
        header = _build_task_header(task_md, args.task)
        phase_label = f"phase B mode={mode}"
        if stacked:
            _ensure_task_layer(args.slug, args.task)
    elif args.slice_text is not None:
        header = _build_slice_header(args.slice_text)
        phase_label = f"phase B mode={mode}"
    else:
        header = _build_review_header(args.review_round)
        phase_label = f"phase D review round {args.review_round}"
        if stacked and not _in_stack():
            print(
                "commit-checkpoint: stacked review-round checkpoint requires "
                "the current branch to belong to a stack; resume it first "
                "(`gh stack checkout`, then `gh stack top`)",
                file=sys.stderr,
            )
            return 2

    body_parts = [
        f"Checkpoint via cy-loop-tasks (iteration {iteration}, {phase_label})."
    ]
    if body_note:
        body_parts.append(body_note)
    body_parts.append(_COAUTHOR)
    body = "\n\n".join(body_parts)

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
    if stacked:
        _submit_stack()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

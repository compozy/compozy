#!/usr/bin/env python3
"""
update-state.py -- mutating.

Single source of truth for state.yaml mutations after bootstrap. Every
iteration MUST end by calling this script with the flags that describe
what happened. The script also appends one entry to ``iterations[]``
(capped at the last 50) when at least one observation flag is present.

Usage:
    update-state.py <slug> [flags...]

Flags (multiple may combine in one call):
    --phase {0,B,C,D,E}              phase label for iterations[]
    --action "<text>"                action label for the iteration log entry
    --outcome {completed,partial,blocked}
    --memory-written path[,path...]  comma-separated repo-relative paths
    --blocker "<text>"               appends to iteration entry blockers[]
    --task-completed <stem>          mode=tasks: move stem from pending to completed
    --task-current <stem|->          set tasks.current; '-' clears it
    --reconcile-tasks                rebuild mode/tasks.* from task_*.md frontmatter
    --disable-frontend               switch future frontend tasks to the local lane
    --disable-stacked                switch future checkpoints to one-branch mode
    --add-progress "<text>"          mode=free: append checklist entry status=in_progress
    --complete-progress "<text>"     mode=free: flip checklist entry to completed
    --deliverables-complete          mode=free: set progress.deliverables_complete=true
    --qa-report-done                 set qa.report_done=true
    --qa-execution-done              set qa.execution_done=true
    --review-round-done VERDICT      close one peer-review round: increments
                                     review.rounds, sets review.last_verdict;
                                     SHIP also sets review.ship=true
    --verify-pass                    verify.last_status=PASS, last_run=now
    --verify-fail                    verify.last_status=FAIL, last_run=now
    --pr-url URL                     delivery PR URL; repeat for stacked PRs
    --head-sha SHA                   matching PR head SHA; repeat in PR order
    --ci-check NAME                  observed required check; repeat as needed
    --ci-pending                     delivery.ci_status=PENDING
    --ci-pass                        delivery.ci_status=PASS; requires exact-head
                                     PR/check evidence
    --ci-fail                        delivery.ci_status=FAIL
    --repo-root PATH                 git checkout used for exact-head validation
    --tasks-root <path>              default .compozy/tasks
    --max-iterations N               default 50; tail-cap iterations[]

Exits:
    0 success
    1 generic error (stderr)
    2 state.yaml missing
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import dump, load, now_iso  # noqa: E402


class StateUpdateError(ValueError):
    """Raised when a requested state mutation is invalid."""


_FRONTMATTER = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)
_IN_PROGRESS_STATUSES = {"in_progress", "in-progress", "running"}
_GITHUB_PR_URL = re.compile(r"^https://github\.com/[^/]+/[^/]+/pull/[1-9][0-9]*$")
_GIT_SHA = re.compile(r"^[0-9a-f]{40}$")


def _read_frontmatter(md_path: Path) -> dict[str, str]:
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


def _parse_args() -> argparse.Namespace:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("slug")
    ap.add_argument("--tasks-root", default=".compozy/tasks")
    ap.add_argument("--max-iterations", type=int, default=50)

    ap.add_argument("--phase", choices=["0", "B", "C", "D", "E"])
    ap.add_argument("--action")
    ap.add_argument("--outcome", choices=["completed", "partial", "blocked"])
    ap.add_argument("--memory-written", default="")
    ap.add_argument("--blocker", action="append", default=[])

    ap.add_argument("--task-completed")
    ap.add_argument("--task-current")
    ap.add_argument("--reconcile-tasks", action="store_true")
    ap.add_argument("--disable-frontend", action="store_true")
    ap.add_argument("--disable-stacked", action="store_true")
    ap.add_argument("--add-progress")
    ap.add_argument("--complete-progress")
    ap.add_argument("--deliverables-complete", action="store_true")

    ap.add_argument("--qa-report-done", action="store_true")
    ap.add_argument("--qa-execution-done", action="store_true")
    ap.add_argument(
        "--review-round-done",
        choices=["SHIP", "FIX_BEFORE_SHIP", "REWORK"],
    )

    ap.add_argument("--verify-pass", action="store_true")
    ap.add_argument("--verify-fail", action="store_true")
    ap.add_argument("--pr-url", action="append", default=[])
    ap.add_argument("--head-sha", action="append", default=[])
    ap.add_argument("--ci-check", action="append", default=[])
    ci_status = ap.add_mutually_exclusive_group()
    ci_status.add_argument("--ci-pending", action="store_true")
    ci_status.add_argument("--ci-pass", action="store_true")
    ci_status.add_argument("--ci-fail", action="store_true")
    ap.add_argument("--repo-root", default=".")
    return ap.parse_args()


def _delivery_status_arg(args: argparse.Namespace) -> str | None:
    if args.ci_pending:
        return "PENDING"
    if args.ci_pass:
        return "PASS"
    if args.ci_fail:
        return "FAIL"
    return None


def _current_head(repo_root: str) -> str:
    result = subprocess.run(
        ["git", "-C", repo_root, "rev-parse", "HEAD"],
        check=False,
        text=True,
        capture_output=True,
    )
    if result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip() or "unknown git error"
        raise StateUpdateError(f"cannot resolve current HEAD: {detail}")
    return result.stdout.strip().lower()


def _validate_delivery_args(state: dict, args: argparse.Namespace) -> None:
    status = _delivery_status_arg(args)
    has_metadata = bool(args.pr_url or args.head_sha or args.ci_check)
    if has_metadata and status is None:
        raise StateUpdateError(
            "delivery evidence requires --ci-pending, --ci-pass, or --ci-fail"
        )
    if status is None:
        return

    if status == "PASS" and not args.ci_check:
        raise StateUpdateError("CI PASS requires at least one reported check")
    if status == "PASS" and (not args.pr_url or not args.head_sha):
        raise StateUpdateError(
            "CI PASS requires fresh --pr-url and --head-sha evidence"
        )

    existing = state.get("delivery", {}) or {}
    pr_urls = list(args.pr_url or existing.get("pr_urls") or [])
    head_shas = [sha.lower() for sha in (args.head_sha or existing.get("head_shas") or [])]
    checks = list(args.ci_check or existing.get("checks") or [])

    if not pr_urls or len(pr_urls) != len(head_shas):
        raise StateUpdateError(
            "CI evidence requires one --head-sha for every --pr-url"
        )
    invalid_url = next((url for url in pr_urls if not _GITHUB_PR_URL.fullmatch(url)), None)
    if invalid_url:
        raise StateUpdateError(f"invalid GitHub PR URL: {invalid_url}")
    invalid_sha = next((sha for sha in head_shas if not _GIT_SHA.fullmatch(sha)), None)
    if invalid_sha:
        raise StateUpdateError(f"invalid 40-character git head SHA: {invalid_sha}")

    if status == "PASS":
        current_head = _current_head(args.repo_root)
        if head_shas[-1] != current_head:
            raise StateUpdateError(
                "CI PASS head does not match current HEAD: "
                f"reported={head_shas[-1]} current={current_head}"
            )


def _clear_delivery(state: dict) -> None:
    state["delivery"] = {
        "pr_urls": [],
        "head_shas": [],
        "ci_status": None,
        "checks": [],
        "observed_at": None,
    }


def _reconcile_tasks(state: dict, slug_dir: Path) -> None:
    task_files = sorted(slug_dir.glob("task_*.md"))
    tasks = state.setdefault("tasks", {})
    if not task_files:
        state["mode"] = "free"
        tasks["total"] = 0
        tasks["completed"] = []
        tasks["current"] = None
        tasks["pending"] = []
        return
    if not (slug_dir / "_tasks.md").exists():
        raise StateUpdateError(
            "--reconcile-tasks found task_*.md files but _tasks.md is missing"
        )

    completed: list[str] = []
    pending: list[str] = []
    current: str | None = None
    in_progress: list[str] = []
    for path in task_files:
        stem = path.stem
        status = _read_frontmatter(path).get("status", "pending").lower()
        if status == "completed":
            completed.append(stem)
            continue
        pending.append(stem)
        if status in _IN_PROGRESS_STATUSES:
            in_progress.append(stem)

    if len(in_progress) > 1:
        joined = ", ".join(in_progress)
        raise StateUpdateError(
            f"--reconcile-tasks found multiple in-progress tasks: {joined}"
        )
    if in_progress:
        current = in_progress[0]

    state["mode"] = "tasks"
    tasks["total"] = len(task_files)
    tasks["completed"] = completed
    tasks["current"] = current
    tasks["pending"] = pending


def _apply(state: dict, args: argparse.Namespace, slug_dir: Path) -> None:
    _validate_delivery_args(state, args)

    if args.disable_frontend:
        state["frontend_agent"] = None

    if args.disable_stacked:
        state["stacked"] = False

    if args.reconcile_tasks:
        _reconcile_tasks(state, slug_dir)

    if args.task_completed:
        tasks = state.setdefault("tasks", {})
        pending = list(tasks.get("pending") or [])
        completed = list(tasks.get("completed") or [])
        if args.task_completed in pending:
            pending.remove(args.task_completed)
        if args.task_completed not in completed:
            completed.append(args.task_completed)
        tasks["pending"] = pending
        tasks["completed"] = completed
        if tasks.get("current") == args.task_completed:
            tasks["current"] = None

    if args.task_current is not None:
        tasks = state.setdefault("tasks", {})
        tasks["current"] = None if args.task_current == "-" else args.task_current

    if args.add_progress:
        progress = state.setdefault("progress", {})
        checklist = list(progress.get("checklist") or [])
        checklist.append(
            {
                "text": args.add_progress,
                "status": "in_progress",
                "iteration": int(state.get("iteration", 0)) + 1,
            }
        )
        progress["checklist"] = checklist

    if args.complete_progress:
        progress = state.setdefault("progress", {})
        checklist = list(progress.get("checklist") or [])
        for item in checklist:
            if item.get("text") == args.complete_progress:
                item["status"] = "completed"
                item["iteration"] = int(state.get("iteration", 0)) + 1
                break
        else:
            raise StateUpdateError(
                "--complete-progress did not match any existing "
                "progress.checklist[].text exactly; run --add-progress first "
                "or pass the exact stored text"
            )
        progress["checklist"] = checklist

    if args.deliverables_complete:
        state.setdefault("progress", {})["deliverables_complete"] = True

    if args.qa_report_done:
        state.setdefault("qa", {})["report_done"] = True
    if args.qa_execution_done:
        state.setdefault("qa", {})["execution_done"] = True

    if args.review_round_done:
        review = state.setdefault("review", {})
        review["rounds"] = int(review.get("rounds", 0) or 0) + 1
        review["last_verdict"] = args.review_round_done
        if args.review_round_done == "SHIP":
            review["ship"] = True

    if args.verify_pass:
        state.setdefault("verify", {})
        state["verify"]["last_run"] = now_iso()
        state["verify"]["last_status"] = "PASS"
        if not args.ci_pass:
            _clear_delivery(state)
    if args.verify_fail:
        state.setdefault("verify", {})
        state["verify"]["last_run"] = now_iso()
        state["verify"]["last_status"] = "FAIL"
        _clear_delivery(state)

    delivery_status = _delivery_status_arg(args)
    if delivery_status is not None:
        existing = state.get("delivery", {}) or {}
        state["delivery"] = {
            "pr_urls": list(args.pr_url or existing.get("pr_urls") or []),
            "head_shas": [
                sha.lower()
                for sha in (args.head_sha or existing.get("head_shas") or [])
            ],
            "ci_status": delivery_status,
            "checks": list(args.ci_check or existing.get("checks") or []),
            "observed_at": now_iso(),
        }


def _has_observation(args: argparse.Namespace) -> bool:
    return any(
        [
            args.action,
            args.outcome,
            args.memory_written,
            args.blocker,
            args.task_completed,
            args.add_progress,
            args.complete_progress,
            args.deliverables_complete,
            args.qa_report_done,
            args.qa_execution_done,
            args.review_round_done,
            args.verify_pass,
            args.verify_fail,
            args.ci_pending,
            args.ci_pass,
            args.ci_fail,
            args.reconcile_tasks,
            args.disable_frontend,
            args.disable_stacked,
        ]
    )


def main() -> int:
    args = _parse_args()
    slug_dir = Path(args.tasks_root) / args.slug
    state_path = slug_dir / "state.yaml"
    if not state_path.exists():
        print(
            f"update-state: {state_path} missing; run init-state.py first",
            file=sys.stderr,
        )
        return 2
    try:
        state = load(state_path)
    except Exception as exc:  # noqa: BLE001
        print(f"update-state: failed to parse {state_path}: {exc}", file=sys.stderr)
        return 1

    record_iteration = _has_observation(args)
    try:
        _apply(state, args, slug_dir)
    except StateUpdateError as exc:
        print(f"update-state: {exc}", file=sys.stderr)
        return 1

    if record_iteration:
        new_n = int(state.get("iteration", 0)) + 1
        state["iteration"] = new_n
        memory_written: list[str] = []
        if args.memory_written:
            memory_written = [
                item.strip()
                for item in args.memory_written.split(",")
                if item.strip()
            ]
        entry = {
            "n": new_n,
            "timestamp": now_iso(),
            "phase": args.phase or "?",
            "action": args.action or "",
            "outcome": args.outcome or "completed",
            "memory_written": memory_written,
            "blockers": list(args.blocker or []),
        }
        iterations = list(state.get("iterations") or [])
        iterations.append(entry)
        max_n = max(1, args.max_iterations)
        if len(iterations) > max_n:
            iterations = iterations[-max_n:]
        state["iterations"] = iterations

    state["last_updated"] = now_iso()
    dump(state, state_path)
    print(
        f"update-state: {state_path} updated "
        f"(iteration={state.get('iteration', 0)}, "
        f"phase={args.phase or '?'})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

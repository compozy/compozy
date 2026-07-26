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
    --add-criterion "<text>"         append a criterion (status=pending);
                                     repeatable; for spec amendments only
    --criterion-met "<text>"         flip one criterion pending -> met by
                                     exact stored text; repeatable
    --implementation-complete        set progress.implementation_complete=true;
                                     refused while any criterion is pending
                                     or without --verify-pass in the same call
    --qa-report-done                 set qa.report_done=true
    --qa-execution-done              set qa.execution_done=true
    --review-round-done VERDICT      close one peer-review round: increments
                                     review.rounds, sets review.last_verdict;
                                     SHIP also sets review.ship=true
    --verify-pass                    verify.last_status=PASS, last_run=now
    --verify-fail                    verify.last_status=FAIL, last_run=now
    --tasks-root <path>              default .compozy/tasks
    --max-iterations N               default 50; tail-cap iterations[]

Exits:
    0 success
    1 generic error (stderr); state.yaml is left untouched
    2 state.yaml missing
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _state_io import dump, load, now_iso  # noqa: E402


class StateUpdateError(ValueError):
    """Raised when a requested state mutation is invalid."""


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

    ap.add_argument("--add-criterion", action="append", default=[])
    ap.add_argument("--criterion-met", action="append", default=[])
    ap.add_argument("--implementation-complete", action="store_true")

    ap.add_argument("--qa-report-done", action="store_true")
    ap.add_argument("--qa-execution-done", action="store_true")
    ap.add_argument(
        "--review-round-done",
        choices=["SHIP", "FIX_BEFORE_SHIP", "REWORK"],
    )

    ap.add_argument("--verify-pass", action="store_true")
    ap.add_argument("--verify-fail", action="store_true")
    return ap.parse_args()


def _apply(state: dict, args: argparse.Namespace) -> None:
    if args.verify_pass and args.verify_fail:
        raise StateUpdateError(
            "--verify-pass and --verify-fail are mutually exclusive"
        )

    progress = state.setdefault("progress", {})
    criteria = list(progress.get("criteria") or [])
    next_iteration = int(state.get("iteration", 0)) + 1

    for text in args.add_criterion:
        cleaned = text.strip()
        if not cleaned:
            raise StateUpdateError("--add-criterion text is empty after trim")
        if any(c.get("text") == cleaned for c in criteria):
            raise StateUpdateError(
                f"--add-criterion duplicates an existing criterion: {cleaned!r}"
            )
        criteria.append(
            {"text": cleaned, "status": "pending", "iteration": next_iteration}
        )

    for text in args.criterion_met:
        for item in criteria:
            if item.get("text") == text:
                if item.get("status") == "met":
                    raise StateUpdateError(
                        f"--criterion-met target is already met: {text!r}"
                    )
                item["status"] = "met"
                item["iteration"] = next_iteration
                break
        else:
            raise StateUpdateError(
                "--criterion-met did not match any progress.criteria[].text "
                f"exactly: {text!r}; pass the exact stored text"
            )

    progress["criteria"] = criteria

    if args.implementation_complete:
        if not criteria:
            raise StateUpdateError(
                "--implementation-complete refused: progress.criteria is empty"
            )
        still_pending = [
            c.get("text", "") for c in criteria if c.get("status") != "met"
        ]
        if still_pending:
            joined = "; ".join(still_pending)
            raise StateUpdateError(
                f"--implementation-complete refused: criteria still pending: {joined}"
            )
        if not args.verify_pass:
            raise StateUpdateError(
                "--implementation-complete requires --verify-pass in the same "
                "call; run cy-final-verify first"
            )
        progress["implementation_complete"] = True

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
    if args.verify_fail:
        state.setdefault("verify", {})
        state["verify"]["last_run"] = now_iso()
        state["verify"]["last_status"] = "FAIL"


def _has_observation(args: argparse.Namespace) -> bool:
    return any(
        [
            args.action,
            args.outcome,
            args.memory_written,
            args.blocker,
            args.add_criterion,
            args.criterion_met,
            args.implementation_complete,
            args.qa_report_done,
            args.qa_execution_done,
            args.review_round_done,
            args.verify_pass,
            args.verify_fail,
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
        _apply(state, args)
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
    criteria = list(state.get("progress", {}).get("criteria") or [])
    met = sum(1 for c in criteria if c.get("status") == "met")
    print(
        f"update-state: {state_path} updated "
        f"(iteration={state.get('iteration', 0)}, "
        f"phase={args.phase or '?'}, criteria={met}/{len(criteria)} met)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

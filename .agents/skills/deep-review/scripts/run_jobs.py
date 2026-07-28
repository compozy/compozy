#!/usr/bin/env python3
"""Deep-review job runner/validator (mutating helper; writes only under --out).

One resumable engine for every job kind (defect cohort, polish, sweep). Two modes:

  --validate-only            Report each job as VALID / PENDING / INVALID and
                             refresh the status file. This is how Workflow- and
                             Agent-driven rounds resume: dispatch only what is
                             not VALID, then re-run this mode until exit 0.
  --command '<template>'     Execute pending jobs via a subprocess per job
                             ({prompt} required; {output} and {label} optional
                             placeholders), bounded workers, output validation,
                             retries, and provider-block detection.

Valid outputs are always preserved — re-running never repeats finished work.
The source freeze is checked before and after execution (--no-freeze-check to
skip). Exit codes: 0 all jobs valid, 1 failures/pending remain, 2 blocked by a
provider limit (see <out>/run-blocker.json), 3 source drifted during the run.
"""

from __future__ import annotations

import argparse
import json
import shlex
import subprocess
import sys
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

sys.dont_write_bytecode = True  # keep the tracked skill tree free of __pycache__

from _common import check_freeze, load_jobs, read_json, repo_root, validate_job_output, write_json

PRINT_LOCK = threading.Lock()
STOP_EVENT = threading.Event()
STOP_REASON: dict[str, str] = {}


def say(message: str) -> None:
    with PRINT_LOCK:
        print(message, flush=True)


def job_state(repo: Path, out: Path, job: dict) -> tuple[str, str]:
    if not (repo / job["output"]).is_file():
        return "pending", "missing output"
    try:
        validate_job_output(repo, out, job)
    except ValueError as error:
        return "invalid", str(error)
    return "valid", ""


def render_command(template: str, job: dict, repo: Path) -> list[str]:
    tokens = shlex.split(template)
    substituted = [
        token.replace("{prompt}", job["prompt"])
        .replace("{output}", job["output"])
        .replace("{label}", job["label"])
        for token in tokens
    ]
    return substituted


def run_one(repo: Path, out: Path, job: dict, args) -> dict:
    label = job["label"]
    output = repo / job["output"]
    state, _ = job_state(repo, out, job)
    if state == "valid":
        say(f"SKIP {label} existing-valid-output")
        return {"label": label, "status": "pass", "attempt": 0, "preserved": True}

    runs_dir = out / "runs"
    last_error, exit_code = "not run", None
    for attempt in range(1, args.attempts + 1):
        if STOP_EVENT.is_set():
            return {"label": label, "status": "blocked", "attempt": attempt - 1,
                    "error": STOP_REASON.get("reason", "run stopped")}
        output.parent.mkdir(parents=True, exist_ok=True)
        if output.exists():
            output.unlink()
        stdout_path = runs_dir / f"{label}.attempt-{attempt}.events.jsonl"
        stderr_path = runs_dir / f"{label}.attempt-{attempt}.err"
        command = render_command(args.command, job, repo)
        with stdout_path.open("w", encoding="utf-8") as out_file, stderr_path.open(
            "w", encoding="utf-8"
        ) as err_file:
            try:
                completed = subprocess.run(
                    command, cwd=repo, stdout=out_file, stderr=err_file,
                    check=False, timeout=args.timeout_min * 60,
                )
                exit_code = completed.returncode
            except subprocess.TimeoutExpired:
                exit_code = None
                last_error = f"runner timeout after {args.timeout_min}m"
                say(f"RETRY {label} attempt={attempt} reason={last_error}")
                continue
        streams = (
            stdout_path.read_text(encoding="utf-8", errors="replace")
            + stderr_path.read_text(encoding="utf-8", errors="replace")
        )
        blocked_on = next((pattern for pattern in args.block_on if pattern in streams), None)
        if blocked_on:
            STOP_EVENT.set()
            STOP_REASON.setdefault("reason", blocked_on)
            STOP_REASON.setdefault("label", label)
            say(f"BLOCKED {label} pattern={blocked_on}")
            return {"label": label, "status": "blocked", "attempt": attempt,
                    "exit_code": exit_code, "error": blocked_on}
        if exit_code != 0:
            last_error = f"command exit {exit_code}"
        else:
            state, reason = job_state(repo, out, job)
            if state == "valid":
                say(f"PASS {label} attempt={attempt}")
                return {"label": label, "status": "pass", "attempt": attempt, "exit_code": 0}
            last_error = reason or "output invalid"
        say(f"RETRY {label} attempt={attempt} reason={last_error}")
    return {"label": label, "status": "fail", "attempt": args.attempts,
            "exit_code": exit_code, "error": last_error}


def stage_report(repo: Path, out: Path, jobs: list[dict]) -> tuple[int, list[dict]]:
    pending = []
    for job in jobs:
        state, reason = job_state(repo, out, job)
        if state != "valid":
            pending.append({"label": job["label"], "state": state, "reason": reason})
    return len(jobs) - len(pending), pending


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", required=True)
    parser.add_argument("--jobs-file", help="default: <out>/jobs.json")
    parser.add_argument("--only", nargs="*", help="exact job labels to consider")
    parser.add_argument("--validate-only", action="store_true")
    parser.add_argument("--command", help="subprocess template; {prompt} required, {output}/{label} optional")
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--attempts", type=int, default=2)
    parser.add_argument("--timeout-min", type=int, default=35)
    parser.add_argument("--block-on", action="append", default=None,
                        help="abort-the-run substring (repeatable); default: usageLimitExceeded")
    parser.add_argument("--status-file", help="default: <out>/runs/<jobs-stem>-status.json")
    parser.add_argument("--no-freeze-check", action="store_true")
    args = parser.parse_args()
    if bool(args.validate_only) == bool(args.command):
        parser.error("pass exactly one of --validate-only or --command")
    if not 1 <= args.workers <= 6:
        parser.error("--workers must be between 1 and 6")
    if not 1 <= args.attempts <= 3:
        parser.error("--attempts must be between 1 and 3")
    if args.command and "{prompt}" not in args.command:
        parser.error("--command must contain the {prompt} placeholder")
    args.block_on = args.block_on or ["usageLimitExceeded"]

    repo = repo_root()
    out = Path(args.out).resolve()
    jobs_path = Path(args.jobs_file).resolve() if args.jobs_file else out / "jobs.json"
    try:
        jobs = load_jobs(jobs_path)
    except RuntimeError as error:
        sys.stderr.write(f"{error}\n")
        return 1
    if args.only:
        unknown = set(args.only) - {job["label"] for job in jobs}
        if unknown:
            parser.error(f"unknown jobs: {sorted(unknown)}")
        jobs = [job for job in jobs if job["label"] in args.only]

    status_path = (
        Path(args.status_file).resolve() if args.status_file
        else out / "runs" / f"{jobs_path.stem}-status.json"
    )

    if args.validate_only:
        rows = []
        for job in jobs:
            state, reason = job_state(repo, out, job)
            row = {"label": job["label"], "status": state, "reason": reason or None}
            if state != "valid":  # pending rows carry the dispatch contract for the engines
                row["prompt"], row["output"] = job["prompt"], job["output"]
            rows.append(row)
            say(f"{state.upper()} {job['label']}" + (f" — {reason}" if reason else ""))
        write_json(status_path, {"mode": "validate", "jobs": rows})
        pending = [row for row in rows if row["status"] != "valid"]
        say(f"SUMMARY valid={len(rows) - len(pending)} pending={len(pending)} of {len(rows)}")
        return 0 if not pending else 1

    drift: list[str] = []
    if not args.no_freeze_check:
        try:
            drift = check_freeze(repo, out, "before run")
        except RuntimeError as error:
            sys.stderr.write(f"{error}\n")
            return 1
        if drift:
            sys.stderr.write(drift[0] + "\n")
            return 3

    (out / "runs").mkdir(parents=True, exist_ok=True)
    results: list[dict] = []
    with ThreadPoolExecutor(max_workers=args.workers) as executor:
        futures = {executor.submit(run_one, repo, out, job, args): job for job in jobs}
        for future in as_completed(futures):
            result = future.result()
            results.append(result)
            if result["status"] == "fail":
                say(f"FAIL {result['label']}: {result['error']}")
    results.sort(key=lambda item: str(item["label"]))

    valid_count, pending = stage_report(repo, out, jobs)
    failed = [item for item in results if item["status"] == "fail"]
    blocked = [item for item in results if item["status"] == "blocked"]
    write_json(status_path, {
        "mode": "run", "jobs": results,
        "summary": {"pass": len(results) - len(failed) - len(blocked),
                    "fail": len(failed), "blocked": len(blocked),
                    "stage_valid": valid_count, "stage_total": len(jobs)},
    })
    if blocked:
        write_json(out / "run-blocker.json", {
            "status": "blocked",
            "pattern": STOP_REASON.get("reason"),
            "first_label": STOP_REASON.get("label"),
            "jobs_file": str(jobs_path),
            "valid_outputs": valid_count,
            "total_jobs": len(jobs),
            "pending": [row["label"] for row in pending],
        })
    say(f"SUMMARY pass={len(results) - len(failed) - len(blocked)} fail={len(failed)} "
        f"blocked={len(blocked)}; stage {valid_count}/{len(jobs)} outputs valid")

    if not args.no_freeze_check:
        try:
            drift = check_freeze(repo, out, "after run")
        except RuntimeError as error:
            sys.stderr.write(f"{error}\n")
            return 1
        if drift:
            sys.stderr.write(drift[0] + "\n")
            return 3
    if blocked:
        say(f"resume: rerun this command after the limit clears — see {out / 'run-blocker.json'}")
        return 2
    return 1 if failed or pending else 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Observe the AGH runtime during a real-scenario QA demo without sending any prompt.

Read-only helper. Tails journey-log.jsonl for the configured duration; flags stall
(silence > stall-threshold-sec) with a diagnose block (which agent, which task seems blocked).
Writes a structured observation summary to qa-artifacts/qa/observation-summary.json so the
auditor can fold it into its report.

This script does NOT touch the AGH daemon, the SSE stream, or the network. The runtime is
expected to write to journey-log.jsonl through normal operation (the AGH daemon, agent
sessions, and the record-scenario-action helper). If your scenario lacks a journey-log
writer, that is a real-scenario bug — not something this observer should patch around.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any


def now_utc() -> datetime:
    return datetime.now(timezone.utc)


def parse_iso(value: str) -> datetime | None:
    if not value:
        return None
    try:
        if value.endswith("Z"):
            value = value[:-1] + "+00:00"
        dt = datetime.fromisoformat(value)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def read_existing(journey_log: Path) -> list[dict[str, Any]]:
    entries: list[dict[str, Any]] = []
    if not journey_log.is_file():
        return entries
    with journey_log.open("r", encoding="utf-8") as handle:
        for line in handle:
            stripped = line.strip()
            if not stripped:
                continue
            try:
                payload = json.loads(stripped)
            except json.JSONDecodeError:
                continue
            if isinstance(payload, dict):
                entries.append(payload)
    return entries


def diagnose_stall(open_tasks_path: Path, entries: list[dict[str, Any]]) -> dict[str, Any]:
    diagnose: dict[str, Any] = {
        "agents_silent": [],
        "tasks_unstarted": [],
        "tasks_in_progress_no_completion": [],
        "channels_silent": [],
    }
    actors = {str(entry.get("actor", "")).strip() for entry in entries if entry.get("actor")}
    if open_tasks_path.is_file():
        try:
            tasks = json.loads(open_tasks_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            tasks = []
        if isinstance(tasks, list):
            unstarted = [
                task["title"]
                for task in tasks
                if isinstance(task, dict)
                and task.get("owner_agent") not in actors
            ]
            diagnose["tasks_unstarted"] = unstarted
            owners = {task["owner_agent"] for task in tasks if isinstance(task, dict) and task.get("owner_agent")}
            silent = sorted(owner for owner in owners if owner not in actors)
            diagnose["agents_silent"] = silent
    actions_by_task: dict[str, set[str]] = defaultdict(set)
    for entry in entries:
        ids = entry.get("ids", [])
        if not isinstance(ids, list):
            continue
        action = str(entry.get("action", "")).lower()
        for tid in ids:
            actions_by_task[str(tid)].add(action)
    diagnose["tasks_in_progress_no_completion"] = sorted(
        tid
        for tid, actions in actions_by_task.items()
        if {"task_started", "run_started", "claim_run", "start_run"}.intersection(actions)
        and not {"task_completed", "run_completed", "complete_run"}.intersection(actions)
    )
    channel_counts = Counter()
    for entry in entries:
        channel = str(entry.get("channel", "")).strip()
        if channel:
            channel_counts[channel] += 1
    diagnose["channels_silent"] = sorted(
        channel for channel, count in channel_counts.items() if count <= 1
    )
    return diagnose


def summarize(entries: list[dict[str, Any]]) -> dict[str, Any]:
    actors = Counter()
    surfaces = Counter()
    actions = Counter()
    channels = Counter()
    for entry in entries:
        actors[str(entry.get("actor", ""))] += 1
        surfaces[str(entry.get("surface", ""))] += 1
        actions[str(entry.get("action", ""))] += 1
        channel = str(entry.get("channel", ""))
        if channel:
            channels[channel] += 1
    return {
        "entry_count": len(entries),
        "by_actor": dict(actors),
        "by_surface": dict(surfaces),
        "by_action": dict(actions),
        "by_channel": dict(channels),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--workspace", required=True, help="WORKSPACE_PATH from manifest")
    parser.add_argument("--qa-output-path", required=True, help="QA_OUTPUT_PATH from manifest")
    parser.add_argument(
        "--duration-sec", type=int, default=1800, help="Total observation window (default 1800s = 30 min)"
    )
    parser.add_argument(
        "--stall-threshold-sec",
        type=int,
        default=300,
        help="Mark stall when no new journey-log row arrives within this many seconds (default 300)",
    )
    parser.add_argument(
        "--poll-interval-sec",
        type=float,
        default=2.0,
        help="How often to poll the journey-log for new rows (default 2s)",
    )
    parser.add_argument(
        "--exit-on-stall",
        action="store_true",
        help="Exit immediately when stall is detected instead of continuing the window",
    )
    args = parser.parse_args()

    qa_output_path = Path(args.qa_output_path).resolve()
    workspace_root = Path(args.workspace).resolve()
    journey_log = qa_output_path / "qa" / "journey-log.jsonl"
    open_tasks_path = workspace_root / ".agh" / "tasks" / "open-tasks.json"
    summary_path = qa_output_path / "qa" / "observation-summary.json"

    journey_log.parent.mkdir(parents=True, exist_ok=True)
    started_at = now_utc()
    deadline = started_at.timestamp() + args.duration_sec
    last_size = journey_log.stat().st_size if journey_log.is_file() else 0
    last_change = started_at.timestamp()
    stall_detected = False
    stall_at: datetime | None = None

    try:
        while time.time() < deadline:
            time.sleep(max(0.5, args.poll_interval_sec))
            if not journey_log.is_file():
                continue
            current_size = journey_log.stat().st_size
            if current_size != last_size:
                last_size = current_size
                last_change = time.time()
                continue
            if (time.time() - last_change) >= args.stall_threshold_sec and not stall_detected:
                stall_detected = True
                stall_at = now_utc()
                if args.exit_on_stall:
                    break
    except KeyboardInterrupt:
        pass

    entries = read_existing(journey_log)
    summary = {
        "started_at": started_at.isoformat(),
        "ended_at": now_utc().isoformat(),
        "duration_sec_requested": args.duration_sec,
        "stall_detected": stall_detected,
        "stall_at": stall_at.isoformat() if stall_at else None,
        "stall_threshold_sec": args.stall_threshold_sec,
        "summary": summarize(entries),
    }
    if stall_detected:
        summary["diagnose"] = diagnose_stall(open_tasks_path, entries)
    summary_path.write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    print(json.dumps(summary, indent=2, sort_keys=True))
    return 1 if stall_detected else 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Append one structured QA scenario action to journey-log.jsonl.

Mutating helper. Used by both the AGH runtime (when wired) and by the QA observer when it
needs to record an out-of-band signal (e.g., a deliverable artifact landing on disk).
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import sys
from pathlib import Path


def parse_json_arg(value: str, label: str) -> object:
    if not value:
        return []
    try:
        return json.loads(value)
    except json.JSONDecodeError as err:
        raise ValueError(f"{label} must be valid JSON: {err}") from err


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--log", required=True, help="Path to qa/journey-log.jsonl")
    parser.add_argument("--surface", required=True, help="cli, api, web, runtime, provider, or other surface")
    parser.add_argument("--actor", required=True, help="Operator, agent, or system actor id")
    parser.add_argument("--action", required=True, help="Action performed")
    parser.add_argument("--target", required=True, help="Target object or route")
    parser.add_argument("--ids", default="[]", help="JSON array of relevant persisted IDs")
    parser.add_argument("--evidence-path", default="", help="Path to command output, screenshot, transcript, or artifact")
    parser.add_argument("--channel", default="", help="Channel id when applicable")
    parser.add_argument("--task-kind", default="", choices=["", "root", "subtask", "dependency", "run"])
    parser.add_argument("--probe-id", default="", help="Disruption probe id when applicable")
    parser.add_argument("--phase", default="", help="trigger, observed, or result for disruption probes")
    parser.add_argument(
        "--artifact-type",
        default="",
        help="Playbook deliverable_type (tsx_page, go_service_stub, etc.) when this row records a produced artifact",
    )
    parser.add_argument(
        "--produced-by-agent",
        default="",
        help="Agent id that authored the artifact (defaults to actor when unset)",
    )
    parser.add_argument(
        "--used-later-by",
        default="",
        help="JSON array of consumer agent ids or task ids that later use this artifact",
    )
    parser.add_argument(
        "--file-stats",
        default="",
        help="JSON object with artifact stats (e.g., lines, has_errors, parse_ok)",
    )
    parser.add_argument(
        "--review-cycle",
        default="",
        choices=["", "requested", "verdict_approved", "verdict_changes_requested", "resubmitted"],
        help="Mark this row as part of a review handoff cycle",
    )
    parser.add_argument(
        "--disagreement-resolved",
        action="store_true",
        help="Mark this row as the resolution point of a disagreement between agents",
    )
    parser.add_argument(
        "--kickoff",
        action="store_true",
        help="Mark this row as the operator kickoff (auditor exemption marker)",
    )
    args = parser.parse_args()

    try:
        ids = parse_json_arg(args.ids, "--ids")
    except ValueError as err:
        print(err, file=sys.stderr)
        return 2
    if not isinstance(ids, list):
        print("--ids must decode to a JSON array", file=sys.stderr)
        return 2

    used_later: list[str] = []
    if args.used_later_by:
        try:
            parsed = parse_json_arg(args.used_later_by, "--used-later-by")
        except ValueError as err:
            print(err, file=sys.stderr)
            return 2
        if not isinstance(parsed, list):
            print("--used-later-by must decode to a JSON array", file=sys.stderr)
            return 2
        used_later = [str(item) for item in parsed]

    file_stats: dict | None = None
    if args.file_stats:
        try:
            parsed = parse_json_arg(args.file_stats, "--file-stats")
        except ValueError as err:
            print(err, file=sys.stderr)
            return 2
        if not isinstance(parsed, dict):
            print("--file-stats must decode to a JSON object", file=sys.stderr)
            return 2
        file_stats = parsed

    log_path = Path(args.log)
    log_path.parent.mkdir(parents=True, exist_ok=True)
    entry = {
        "ts": datetime.now(timezone.utc).isoformat(),
        "surface": args.surface.strip().lower(),
        "actor": args.actor.strip(),
        "action": args.action.strip(),
        "target": args.target.strip(),
        "ids": ids,
        "evidence_path": args.evidence_path.strip(),
    }
    if args.channel:
        entry["channel"] = args.channel.strip()
    if args.task_kind:
        entry["task_kind"] = args.task_kind
    if args.probe_id:
        entry["probe_id"] = args.probe_id.strip()
    if args.phase:
        entry["phase"] = args.phase.strip().lower()
    if args.artifact_type:
        entry["artifact_type"] = args.artifact_type.strip()
        entry["produced_by_agent"] = (args.produced_by_agent or args.actor).strip()
    if used_later:
        entry["used_later_by"] = used_later
    if file_stats is not None:
        entry["file_stats"] = file_stats
    if args.review_cycle:
        entry["review_cycle"] = args.review_cycle
    if args.disagreement_resolved:
        entry["disagreement_resolved"] = True
    if args.kickoff:
        entry["kickoff"] = True

    with log_path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(entry, sort_keys=True) + "\n")
    print(str(log_path))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

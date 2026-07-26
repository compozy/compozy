#!/usr/bin/env python3
"""Render or confirm the single in-persona operator kickoff for a real-scenario QA playbook.

Mutating helper. Called once by the real-scenario-qa skill after the bootstrap completes.

What it does:
  1. Loads the playbook spec and renders the kickoff message.
  2. Scans the rendered message against forbidden-prompt-phrases.md and aborts on any match.
  3. Writes the rendered message to <workspace>/.agh/operator-kickoff.txt for inspection.
  4. With --confirm-posted, requires non-empty provider evidence, appends the journey row, and
     updates the manifest with KICKOFF_POSTED=true and KICKOFF_TIMESTAMP=<iso>.

It does NOT call `agh session prompt` itself. The caller renders, invokes the AGH CLI once, then
confirms that exact call with --confirm-posted and its captured provider evidence.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import re
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
from playbook_loader import (  # noqa: E402
    PlaybookError,
    load_forbidden_rules,
    load_validated_playbook,
    repo_root_from,
)


def render_kickoff(playbook: dict) -> tuple[str, str]:
    persona = playbook["operator_persona"]
    company = playbook["company"]
    actor = persona.get("name") or persona["role"]
    header = f"{actor} ({persona['role']}) — {company['name']}"
    body = playbook["kickoff_brief"].strip()
    return actor, f"{header}\n\n{body}\n"


def scan_forbidden(text: str, rules: dict[str, list[str]]) -> list[str]:
    hits: list[str] = []
    lowered = text.lower()
    for literal in rules["literals"]:
        if literal.lower() in lowered:
            hits.append(f"literal:{literal}")
    for pattern in rules["patterns"]:
        try:
            if re.search(pattern, text):
                hits.append(f"regex:{pattern}")
        except re.error as err:
            hits.append(f"invalid_regex:{pattern} ({err})")
    return hits


def append_journey_entry(journey_log: Path, entry: dict) -> None:
    journey_log.parent.mkdir(parents=True, exist_ok=True)
    with journey_log.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(entry, sort_keys=True) + "\n")


def update_manifest(manifest_path: Path, timestamp: str) -> None:
    if not manifest_path.is_file():
        raise PlaybookError(f"manifest not found: {manifest_path}")
    data = json.loads(manifest_path.read_text(encoding="utf-8"))
    env = data.setdefault("env", {})
    if str(env.get("KICKOFF_POSTED", "false")).lower() == "true":
        raise PlaybookError("operator kickoff is already confirmed; refusing a duplicate")
    env["KICKOFF_POSTED"] = "true"
    env["KICKOFF_TIMESTAMP"] = timestamp
    status = data.setdefault("status", {})
    notes = status.setdefault("notes", [])
    notes.append(f"operator kickoff posted at {timestamp}")
    manifest_path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def confirm_posted(
    manifest_path: Path,
    journey_log: Path,
    evidence: Path,
    actor: str,
    playbook_ref: str,
) -> str:
    if not evidence.is_file() or evidence.stat().st_size == 0:
        raise PlaybookError(f"operator kickoff evidence is missing or empty: {evidence}")
    timestamp = datetime.now(timezone.utc).isoformat()
    update_manifest(manifest_path, timestamp)
    append_journey_entry(
        journey_log,
        {
            "ts": timestamp,
            "surface": "runtime",
            "actor": actor,
            "action": "operator_kickoff",
            "target": f"playbook:{playbook_ref}",
            "ids": [playbook_ref],
            "evidence_path": str(evidence),
            "kickoff": True,
            "playbook_ref": playbook_ref,
        },
    )
    return timestamp


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--workspace", required=True, help="WORKSPACE_PATH from bootstrap manifest")
    parser.add_argument("--playbook", required=True, help="Playbook ref")
    parser.add_argument("--qa-output-path", required=True, help="QA_OUTPUT_PATH from manifest")
    parser.add_argument("--manifest", required=True, help="bootstrap-manifest.json path")
    parser.add_argument("--repo-root", default=".", help="Repository root")
    parser.add_argument(
        "--print-only",
        action="store_true",
        help="Print the rendered kickoff text and exit; do not mutate manifest or journey log",
    )
    parser.add_argument(
        "--confirm-posted",
        default="",
        help="Non-empty session prompt JSONL captured from the one posted kickoff",
    )
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    if not (repo_root / ".agents").is_dir():
        try:
            repo_root = repo_root_from(repo_root)
        except PlaybookError as err:
            print(str(err), file=sys.stderr)
            return 2

    try:
        playbook = load_validated_playbook(repo_root, args.playbook)
        rules = load_forbidden_rules(repo_root)
    except PlaybookError as err:
        print(str(err), file=sys.stderr)
        return 2

    actor, message = render_kickoff(playbook)
    hits = scan_forbidden(message, rules)
    if hits:
        print("operator kickoff contains forbidden prompt phrases; refusing to post:", file=sys.stderr)
        for hit in hits:
            print(f"  - {hit}", file=sys.stderr)
        return 2

    if args.print_only:
        if args.confirm_posted:
            print("--print-only cannot be combined with --confirm-posted", file=sys.stderr)
            return 2
        sys.stdout.write(message)
        return 0

    workspace_root = Path(args.workspace).resolve()
    qa_output_path = Path(args.qa_output_path).resolve()
    journey_log = qa_output_path / "qa" / "journey-log.jsonl"
    manifest_path = Path(args.manifest).resolve()

    inspect_path = workspace_root / ".agh" / "operator-kickoff.txt"
    inspect_path.parent.mkdir(parents=True, exist_ok=True)
    inspect_path.write_text(message, encoding="utf-8")

    timestamp = ""
    evidence_path = ""
    if args.confirm_posted:
        evidence = Path(args.confirm_posted).resolve()
        try:
            timestamp = confirm_posted(
                manifest_path,
                journey_log,
                evidence,
                actor,
                args.playbook,
            )
        except PlaybookError as err:
            print(str(err), file=sys.stderr)
            return 2
        evidence_path = str(evidence)

    summary = {
        "playbook_ref": args.playbook,
        "actor": actor,
        "timestamp": timestamp,
        "confirmed": bool(args.confirm_posted),
        "evidence_path": evidence_path,
        "rendered_path": str(inspect_path),
        "journey_log_path": str(journey_log),
        "manifest_path": str(manifest_path),
        "message_length": len(message),
    }
    print(json.dumps(summary, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Activate a materialized real-scenario task tree behind the single-kickoff barrier.

Mutating helper. ``prepare`` pauses scheduler dispatch and starts every declared task with an
idempotency key. ``release`` requires confirmed kickoff evidence before resuming dispatch.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
from typing import Callable


CommandRunner = Callable[[str, list[str], dict[str, str]], dict]
ActionRecorder = Callable[[Path, Path, dict], None]


def read_object(path: Path, label: str) -> dict:
    if not path.is_file():
        raise RuntimeError(f"{label} not found: {path}")
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as err:
        raise RuntimeError(f"{label} is not valid JSON: {err}") from err
    if not isinstance(payload, dict):
        raise RuntimeError(f"{label} must contain a JSON object: {path}")
    return payload


def read_tasks(path: Path) -> list[dict]:
    if not path.is_file():
        raise RuntimeError(f"open task tree not found: {path}")
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as err:
        raise RuntimeError(f"open task tree is not valid JSON: {err}") from err
    if not isinstance(payload, list) or not payload:
        raise RuntimeError("open task tree must contain at least one task")
    tasks: list[dict] = []
    seen_ids: set[str] = set()
    for index, raw in enumerate(payload):
        if not isinstance(raw, dict):
            raise RuntimeError(f"open task tree entry {index} must be an object")
        runtime_id = str(raw.get("runtime_id", "")).strip()
        if not runtime_id:
            raise RuntimeError(f"open task tree entry {index} is missing runtime_id")
        if runtime_id in seen_ids:
            raise RuntimeError(f"open task tree contains duplicate runtime_id {runtime_id!r}")
        seen_ids.add(runtime_id)
        tasks.append(raw)
    return tasks


def cli_environment(manifest: dict) -> dict[str, str]:
    manifest_env = manifest.get("env", {})
    if not isinstance(manifest_env, dict):
        raise RuntimeError("bootstrap manifest env must be an object")
    agh_home = str(manifest_env.get("AGH_HOME", "")).strip()
    if not agh_home:
        raise RuntimeError("bootstrap manifest env is missing AGH_HOME")
    env = os.environ.copy()
    for key in ("AGH_HOME", "AGH_UDS_PATH", "TMUX_BRIDGE_SOCKET"):
        value = str(manifest_env.get(key, "")).strip()
        if value:
            env[key] = value
    return env


def run_agh(agh_bin: str, args: list[str], env: dict[str, str]) -> dict:
    proc = subprocess.run(
        [agh_bin, *args, "-o", "json"],
        check=False,
        capture_output=True,
        text=True,
        env=env,
    )
    if proc.returncode != 0:
        detail = proc.stderr.strip() or proc.stdout.strip() or f"exit {proc.returncode}"
        raise RuntimeError(f"{' '.join(args)} failed: {detail}")
    try:
        payload = json.loads(proc.stdout)
    except json.JSONDecodeError as err:
        raise RuntimeError(f"{' '.join(args)} emitted invalid JSON: {err}") from err
    if not isinstance(payload, dict):
        raise RuntimeError(f"{' '.join(args)} must emit a JSON object")
    return payload


def nested_object(payload: dict, key: str) -> dict:
    value = payload.get(key)
    return value if isinstance(value, dict) else {}


def scheduler_paused(payload: dict) -> bool:
    scheduler = nested_object(payload, "scheduler")
    if "paused" in scheduler:
        return bool(scheduler["paused"])
    return bool(payload.get("paused"))


def execution_ids(payload: dict, expected_task_id: str) -> tuple[str, str]:
    task_id = str(nested_object(payload, "task").get("id", "")).strip()
    run_id = str(nested_object(payload, "run").get("id", "")).strip()
    if task_id != expected_task_id or not run_id:
        raise RuntimeError(
            f"task start for {expected_task_id!r} omitted matching task/run ids: {payload!r}"
        )
    return task_id, run_id


def write_evidence(path: Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def record_run_action(helper: Path, journey_log: Path, row: dict) -> None:
    command = [
        sys.executable,
        str(helper),
        "--log",
        str(journey_log),
        "--surface",
        "cli",
        "--actor",
        "real-scenario-bootstrap",
        "--action",
        row["action"],
        "--target",
        row["target"],
        "--ids",
        json.dumps(row.get("ids", [])),
        "--evidence-path",
        row["evidence_path"],
    ]
    if row.get("channel"):
        command.extend(["--channel", row["channel"]])
    if row.get("task_kind"):
        command.extend(["--task-kind", row["task_kind"]])
    proc = subprocess.run(command, check=False, capture_output=True, text=True)
    if proc.returncode != 0:
        detail = proc.stderr.strip() or proc.stdout.strip() or f"exit {proc.returncode}"
        raise RuntimeError(f"record scenario action failed: {detail}")


def prepare_activation(
    workspace: Path,
    qa_output_path: Path,
    manifest_path: Path,
    agh_bin: str,
    runner: CommandRunner | None = None,
    recorder: ActionRecorder | None = None,
) -> dict:
    runner = runner or run_agh
    recorder = recorder or record_run_action
    manifest = read_object(manifest_path, "bootstrap manifest")
    manifest_env = manifest.get("env", {})
    if str(manifest_env.get("KICKOFF_POSTED", "false")).lower() == "true":
        raise RuntimeError("cannot prepare task activation after the operator kickoff")
    tasks = read_tasks(workspace / ".agh" / "tasks" / "open-tasks.json")
    playbook_ref = str(tasks[0].get("playbook_ref", "")).strip()
    if not playbook_ref or any(task.get("playbook_ref") != playbook_ref for task in tasks):
        raise RuntimeError("all open tasks must belong to one non-empty playbook_ref")

    qa_root = qa_output_path / "qa"
    evidence_path = qa_root / "task-activation.json"
    journey_log = qa_root / "journey-log.jsonl"
    record_helper = Path(__file__).with_name("record-scenario-action.py")
    env = cli_environment(manifest)
    prior_evidence: dict = {}
    if evidence_path.is_file():
        prior_evidence = read_object(evidence_path, "task activation evidence")
    owns_existing_pause = bool(
        prior_evidence.get("scheduler_pause_owned")
        and prior_evidence.get("status") in {"preparing", "failed"}
        and prior_evidence.get("playbook_ref") == playbook_ref
    )
    evidence = {
        "schema_version": 1,
        "playbook_ref": playbook_ref,
        "status": "preparing",
        "scheduler_pause_owned": owns_existing_pause,
        "tasks": [],
    }
    write_evidence(evidence_path, evidence)
    try:
        status = runner(agh_bin, ["scheduler", "status"], env)
        if scheduler_paused(status) and not owns_existing_pause:
            raise RuntimeError("scheduler is already paused; refusing to take ownership of the barrier")
        if not scheduler_paused(status):
            paused = runner(
                agh_bin,
                ["scheduler", "pause", "--reason", f"real-scenario kickoff barrier: {playbook_ref}"],
                env,
            )
            evidence["scheduler_pause_owned"] = True
            write_evidence(evidence_path, evidence)
            if not scheduler_paused(paused):
                raise RuntimeError("scheduler pause command did not report paused=true")

        for task in tasks:
            runtime_id = str(task["runtime_id"])
            channel = str(task.get("channel", "")).strip()
            args = [
                "task",
                "start",
                runtime_id,
                "--idempotency-key",
                f"real-scenario:{playbook_ref}:{runtime_id}",
                "--metadata",
                json.dumps({"playbook_ref": playbook_ref, "activation": "pre_kickoff"}),
            ]
            if channel:
                args.extend(
                    [
                        "--network",
                        "live",
                        "--network-channel-strategy",
                        "named",
                        "--network-channel",
                        channel,
                    ]
                )
            output = runner(agh_bin, args, env)
            task_id, run_id = execution_ids(output, runtime_id)
            evidence["tasks"].append(
                {"task_id": task_id, "run_id": run_id, "channel": channel, "output": output}
            )
            write_evidence(evidence_path, evidence)

        evidence["status"] = "prepared"
        evidence["prepared_at"] = datetime.now(timezone.utc).isoformat()
        write_evidence(evidence_path, evidence)
        for task in evidence["tasks"]:
            recorder(
                record_helper,
                journey_log,
                {
                    "action": "task_started",
                    "target": task["task_id"],
                    "ids": [task["run_id"]],
                    "evidence_path": str(evidence_path),
                    "channel": task["channel"],
                    "task_kind": "run",
                },
            )
        return evidence
    except Exception as err:
        evidence["status"] = "failed"
        evidence["error"] = str(err)
        evidence["failed_at"] = datetime.now(timezone.utc).isoformat()
        write_evidence(evidence_path, evidence)
        raise


def release_activation(
    workspace: Path,
    qa_output_path: Path,
    manifest_path: Path,
    kickoff_evidence: Path,
    agh_bin: str,
    runner: CommandRunner | None = None,
    recorder: ActionRecorder | None = None,
) -> dict:
    del workspace
    runner = runner or run_agh
    recorder = recorder or record_run_action
    manifest = read_object(manifest_path, "bootstrap manifest")
    manifest_env = manifest.get("env", {})
    if str(manifest_env.get("KICKOFF_POSTED", "false")).lower() != "true":
        raise RuntimeError("operator kickoff is not confirmed in the bootstrap manifest")
    if not kickoff_evidence.is_file() or kickoff_evidence.stat().st_size == 0:
        raise RuntimeError(f"operator kickoff evidence is missing or empty: {kickoff_evidence}")

    qa_root = qa_output_path / "qa"
    evidence_path = qa_root / "task-activation.json"
    evidence = read_object(evidence_path, "task activation evidence")
    if evidence.get("status") != "prepared" or not evidence.get("scheduler_pause_owned"):
        raise RuntimeError("task activation is not prepared behind an owned scheduler barrier")
    env = cli_environment(manifest)
    resumed = runner(agh_bin, ["scheduler", "resume"], env)
    if scheduler_paused(resumed):
        raise RuntimeError("scheduler resume command still reports paused=true")
    status = runner(agh_bin, ["scheduler", "status"], env)
    if scheduler_paused(status):
        raise RuntimeError("scheduler remained paused after release")

    evidence["status"] = "released"
    evidence["released_at"] = datetime.now(timezone.utc).isoformat()
    evidence["kickoff_evidence"] = str(kickoff_evidence)
    evidence["scheduler_status"] = status
    write_evidence(evidence_path, evidence)
    recorder(
        Path(__file__).with_name("record-scenario-action.py"),
        qa_root / "journey-log.jsonl",
        {
            "action": "scheduler_resumed",
            "target": f"playbook:{evidence['playbook_ref']}",
            "ids": [task["run_id"] for task in evidence["tasks"]],
            "evidence_path": str(evidence_path),
        },
    )
    return evidence


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["prepare", "release"])
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--qa-output-path", required=True)
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--kickoff-evidence", default="")
    parser.add_argument("--agh-bin", default=os.environ.get("AGH_BIN", "agh"))
    args = parser.parse_args()

    workspace = Path(args.workspace).resolve()
    qa_output_path = Path(args.qa_output_path).resolve()
    manifest_path = Path(args.manifest).resolve()
    try:
        if args.phase == "prepare":
            result = prepare_activation(
                workspace, qa_output_path, manifest_path, args.agh_bin
            )
        else:
            kickoff_evidence = (
                Path(args.kickoff_evidence).resolve()
                if args.kickoff_evidence
                else qa_output_path / "qa" / "operator-kickoff.jsonl"
            )
            result = release_activation(
                workspace,
                qa_output_path,
                manifest_path,
                kickoff_evidence,
                args.agh_bin,
            )
    except RuntimeError as err:
        print(str(err), file=sys.stderr)
        return 2
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

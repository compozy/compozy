#!/usr/bin/env python3
"""Observe isolated Compozy progress through runtime-owned public reads."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
import time
from typing import Any, Callable
from urllib import error as urlerror
from urllib import parse as urlparse
from urllib import request as urlrequest


TASK_TERMINAL = {"completed", "failed", "canceled"}
LOOP_TERMINAL = {"done", "no-op", "blocked", "failed", "exhausted", "stalled", "canceled"}


class PublicReadError(RuntimeError):
    """A public CLI or API read failed or returned an invalid contract."""


def require_object(value: Any, source: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise PublicReadError(f"{source} returned a non-object JSON value")
    return value


def require_list(value: Any, source: str) -> list[Any]:
    if not isinstance(value, list):
        raise PublicReadError(f"{source} returned a non-list JSON value")
    return value


def require_matching_field(record: dict[str, Any], field: str, expected: str, source: str) -> str:
    actual = str(record.get(field, "")).strip()
    if actual != expected:
        raise PublicReadError(f"{source} {field} mismatch: got {actual or '<empty>'}, want {expected}")
    return actual


def canonical_fingerprint(snapshot: dict[str, Any]) -> str:
    return json.dumps(snapshot, sort_keys=True, separators=(",", ":"), ensure_ascii=True)


def durable_progress_fingerprint(snapshot: dict[str, Any]) -> str:
    tasks = require_object(snapshot.get("tasks"), "observer task snapshot")
    loops = require_object(snapshot.get("loop_runs", {}), "observer Loop snapshot")
    task_fields = (
        "status",
        "catalog_status",
        "catalog_matches_detail",
        "current_run_id",
        "latest_event_seq",
    )
    task_progress = {
        task_id: {key: require_object(value, f"observer task {task_id}").get(key) for key in task_fields}
        for task_id, value in tasks.items()
    }
    loop_progress: dict[str, dict[str, Any]] = {}
    for run_id, value in loops.items():
        run = require_object(value, f"observer Loop run {run_id}")
        progress = require_object(run.get("progress", {}), f"observer Loop progress {run_id}")
        attention = None
        if run.get("attention") is not None:
            record = require_object(run["attention"], f"observer Loop attention {run_id}")
            attention = {"kind": record.get("kind"), "count": record.get("count")}
        loop_progress[run_id] = {
            "status": run.get("status"),
            "progress": {key: progress.get(key) for key in ("round", "steps_done", "steps_total")},
            "attention": attention,
        }
    return canonical_fingerprint(
        {
            "tasks": task_progress,
            "loop_runs": loop_progress,
            "catalog_comparison": snapshot.get("catalog_comparison"),
        }
    )


def task_status(task: dict[str, Any]) -> str:
    return str(task.get("status", "")).strip().lower()


def all_terminal(snapshot: dict[str, Any]) -> bool:
    comparison = snapshot.get("catalog_comparison")
    if isinstance(comparison, dict) and comparison.get("matches") is not True:
        return False
    tasks = require_object(snapshot.get("tasks"), "observer task snapshot")
    if not tasks:
        return False
    if any(task_status(require_object(value, "observer task")) not in TASK_TERMINAL for value in tasks.values()):
        return False
    loops = require_object(snapshot.get("loop_runs", {}), "observer Loop snapshot")
    return all(
        str(require_object(value, "observer Loop run").get("status", "")).strip().lower() in LOOP_TERMINAL
        for value in loops.values()
    )


def diagnose_unchanged(snapshot: dict[str, Any]) -> dict[str, Any]:
    tasks = require_object(snapshot.get("tasks"), "observer task snapshot")
    loops = require_object(snapshot.get("loop_runs", {}), "observer Loop snapshot")
    return {
        "tasks_active_unchanged": sorted(
            task_id
            for task_id, value in tasks.items()
            if task_status(require_object(value, "observer task")) not in TASK_TERMINAL
        ),
        "loop_runs_active_unchanged": sorted(
            run_id
            for run_id, value in loops.items()
            if str(require_object(value, "observer Loop run").get("status", "")).strip().lower()
            not in LOOP_TERMINAL
        ),
    }


def observation_result(
    outcome: str,
    latest: dict[str, Any],
    transitions: list[dict[str, Any]],
    progress_transitions: int,
    *,
    error: str | None = None,
    diagnose: dict[str, Any] | None = None,
) -> dict[str, Any]:
    result = {
        "outcome": outcome,
        "stall_detected": outcome == "stall",
        "error": error,
        "progress_transitions": progress_transitions,
        "transitions": transitions,
        "final_snapshot": latest,
    }
    if diagnose is not None:
        result["diagnose"] = diagnose
    return result


def run_observation(
    reader: Any,
    *,
    duration_sec: float,
    stall_threshold_sec: float,
    poll_interval_sec: float,
    monotonic: Callable[[], float] = time.monotonic,
    sleep: Callable[[float], None] = time.sleep,
) -> dict[str, Any]:
    started = monotonic()
    last_progress = started
    previous_fingerprint = ""
    progress_transitions = 0
    transitions: list[dict[str, Any]] = []
    latest: dict[str, Any] = {"tasks": {}, "loop_runs": {}}

    while True:
        try:
            latest = require_object(reader.read(), "public runtime reader")
            fingerprint = durable_progress_fingerprint(latest)
        except (PublicReadError, OSError, ValueError) as exc:
            return observation_result("error", latest, transitions, progress_transitions, error=str(exc))

        observed_at = datetime.now(timezone.utc).isoformat()
        if not previous_fingerprint:
            previous_fingerprint = fingerprint
            transitions.append({"observed_at": observed_at, "snapshot": latest})
        elif fingerprint != previous_fingerprint:
            previous_fingerprint = fingerprint
            last_progress = monotonic()
            progress_transitions += 1
            transitions.append({"observed_at": observed_at, "snapshot": latest})

        if all_terminal(latest):
            return observation_result("all_terminal", latest, transitions, progress_transitions)

        current = monotonic()
        if current - last_progress >= stall_threshold_sec:
            return observation_result(
                "stall", latest, transitions, progress_transitions, diagnose=diagnose_unchanged(latest)
            )
        if current - started >= duration_sec:
            return observation_result("window_complete", latest, transitions, progress_transitions)
        sleep(max(0.05, poll_interval_sec))


class PublicRuntimeReader:
    def __init__(
        self,
        *,
        scenario_workspace: Path,
        runtime_workspace: Path,
        workspace_id: str,
        api_base_url: str,
        compozy_home: Path,
        compozy_bin: str,
        command_timeout_sec: float,
    ) -> None:
        self.scenario_workspace = scenario_workspace.resolve()
        self.runtime_workspace = runtime_workspace.resolve()
        self.workspace_id = workspace_id.strip()
        self.api_base_url = api_base_url.rstrip("/")
        self.compozy_home = compozy_home.resolve()
        self.compozy_bin = compozy_bin
        self.command_timeout_sec = command_timeout_sec
        self.declared_task_ids = self._load_declared_task_ids()
        self.task_detail_cache: dict[str, tuple[str, dict[str, Any]]] = {}
        self.loop_detail_cache: dict[str, tuple[str, dict[str, Any]]] = {}
        self._validate_workspace_registration()

    def _load_declared_task_ids(self) -> list[str]:
        path = self.scenario_workspace / ".compozy" / "tasks" / "open-tasks.json"
        try:
            payload = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            raise PublicReadError(f"declared task catalog is unreadable: {path}: {exc}") from exc
        rows = require_list(payload, f"declared task catalog {path}")
        task_ids: list[str] = []
        for index, row in enumerate(rows):
            item = require_object(row, f"declared task catalog row {index}")
            task_id = str(item.get("runtime_id", "")).strip()
            if not task_id:
                raise PublicReadError(f"declared task catalog row {index} has no runtime_id")
            task_ids.append(task_id)
        if not task_ids:
            raise PublicReadError("declared task catalog is empty")
        return task_ids

    def _run_cli(self, arguments: list[str], source: str) -> dict[str, Any]:
        env = os.environ.copy()
        env["COMPOZY_HOME"] = str(self.compozy_home)
        try:
            completed = subprocess.run(
                [self.compozy_bin, *arguments, "-o", "json"],
                check=False,
                capture_output=True,
                text=True,
                timeout=self.command_timeout_sec,
                env=env,
            )
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise PublicReadError(f"{source} read failed: {exc}") from exc
        if completed.returncode != 0:
            detail = completed.stderr.strip() or completed.stdout.strip() or "no diagnostic"
            raise PublicReadError(f"{source} read exited {completed.returncode}: {detail}")
        try:
            payload = json.loads(completed.stdout)
        except json.JSONDecodeError as exc:
            raise PublicReadError(f"{source} returned malformed JSON: {exc}") from exc
        return require_object(payload, source)

    def _get_api(self, path: str, query: dict[str, str], source: str) -> dict[str, Any]:
        url = f"{self.api_base_url}{path}"
        if query:
            url += "?" + urlparse.urlencode(query)
        try:
            with urlrequest.urlopen(url, timeout=self.command_timeout_sec) as response:
                body = response.read().decode("utf-8")
        except (OSError, urlerror.URLError) as exc:
            raise PublicReadError(f"{source} read failed: {exc}") from exc
        try:
            payload = json.loads(body)
        except json.JSONDecodeError as exc:
            raise PublicReadError(f"{source} returned malformed JSON: {exc}") from exc
        return require_object(payload, source)

    def _validate_workspace_registration(self) -> None:
        response = self._run_cli(
            ["workspace", "info", self.workspace_id],
            "public workspace registration",
        )
        workspace = require_object(response.get("workspace"), "public workspace registration workspace")
        require_matching_field(workspace, "id", self.workspace_id, "public workspace registration")
        root_dir = str(workspace.get("root_dir", "")).strip()
        if not root_dir:
            raise PublicReadError("public workspace registration root_dir is empty")
        registered_root = Path(root_dir).expanduser().resolve()
        if registered_root != self.runtime_workspace:
            raise PublicReadError(f"public workspace root mismatch: got {registered_root}, want {self.runtime_workspace}")

    def _task_catalog(self) -> dict[str, dict[str, Any]]:
        response = self._run_cli(
            ["task", "list", "--workspace", self.workspace_id, "--limit", "200"],
            "public Task catalog",
        )
        rows = require_list(response.get("tasks"), "public Task catalog tasks")
        catalog: dict[str, dict[str, Any]] = {}
        for index, row in enumerate(rows):
            item = require_object(row, f"public Task catalog row {index}")
            task_id = str(item.get("id", "")).strip()
            if not task_id:
                raise PublicReadError(f"public Task catalog row {index} has no id")
            require_matching_field(item, "workspace_id", self.workspace_id, f"public Task catalog row {task_id}")
            if task_id in catalog:
                raise PublicReadError(f"public Task catalog contains duplicate task {task_id}")
            catalog[task_id] = item
        return catalog

    def _task_account(self, catalog: dict[str, dict[str, Any]]) -> dict[str, dict[str, Any]]:
        account: dict[str, dict[str, Any]] = {}
        for task_id in self.declared_task_ids:
            item = catalog.get(task_id)
            if item is None:
                raise PublicReadError(f"declared task {task_id} is absent from the public Task catalog")
            catalog_key = canonical_fingerprint(
                {
                    key: item.get(key)
                    for key in ("status", "current_run_id", "latest_event_seq")
                }
            )
            cached = self.task_detail_cache.get(task_id)
            if cached is None or cached[0] != catalog_key:
                detail = self._run_cli(["task", "get", task_id], f"public Task detail {task_id}")
                self.task_detail_cache[task_id] = (catalog_key, detail)
            else:
                detail = cached[1]
            summary = require_object(detail.get("summary"), f"public Task detail summary {task_id}")
            task = require_object(detail.get("task"), f"public Task detail task {task_id}")
            for source, record in (("summary", summary), ("task", task)):
                prefix = f"public Task detail {source} for {task_id}"
                require_matching_field(record, "id", task_id, prefix)
                require_matching_field(record, "workspace_id", self.workspace_id, prefix)
            detail_status = str(summary.get("status", "")).strip().lower()
            task_detail_status = str(task.get("status", "")).strip().lower()
            if not detail_status or task_detail_status != detail_status:
                raise PublicReadError(
                    f"public Task detail status mismatch for {task_id}: "
                    f"summary={detail_status or '<empty>'}, task={task_detail_status or '<empty>'}"
                )
            catalog_status = str(item.get("status", "")).strip().lower()
            account[task_id] = {
                "status": detail_status,
                "catalog_status": catalog_status,
                "catalog_matches_detail": detail_status == catalog_status,
                "current_run_id": summary.get("current_run_id"),
                "latest_event_seq": summary.get("latest_event_seq"),
                "runs": detail.get("runs", []),
                "events": detail.get("events", []),
            }
        return account

    def _loop_runs(self) -> dict[str, dict[str, Any]]:
        path = f"/api/workspaces/{urlparse.quote(self.workspace_id, safe='')}/loop-runs"
        response = self._get_api(path, {"limit": "500"}, "public Loop runs")
        rows = require_list(response.get("runs"), "public Loop runs list")
        account: dict[str, dict[str, Any]] = {}
        for index, row in enumerate(rows):
            item = require_object(row, f"public Loop runs row {index}")
            run_id = str(item.get("id", "")).strip()
            if not run_id:
                raise PublicReadError(f"public Loop runs row {index} has no id")
            require_matching_field(item, "workspace_id", self.workspace_id, f"public Loop runs row {run_id}")
            run_key = canonical_fingerprint(
                {
                    key: item.get(key)
                    for key in ("status", "last_progress_at", "progress", "attention")
                }
            )
            cached = self.loop_detail_cache.get(run_id)
            if cached is None or cached[0] != run_key:
                why = self._run_cli(
                    ["loop", "why", run_id, "--workspace", self.workspace_id],
                    f"public Loop why {run_id}",
                )
                events = self._run_cli(
                    ["loop", "events", run_id, "--workspace", self.workspace_id, "--view", "all", "--limit", "500"],
                    f"public Loop events {run_id}",
                )
                detail = {"why": why, "events": events}
                self.loop_detail_cache[run_id] = (run_key, detail)
            else:
                detail = cached[1]
            account[run_id] = {
                "status": item.get("status"),
                "progress": item.get("progress"),
                "attention": item.get("attention"),
                **detail,
            }
        return account

    def read(self) -> dict[str, Any]:
        catalog = self._task_catalog()
        tasks = self._task_account(catalog)
        mismatches = sorted(
            task_id for task_id, item in tasks.items() if not item["catalog_matches_detail"]
        )
        return {
            "workspace_id": self.workspace_id,
            "tasks": tasks,
            "loop_runs": self._loop_runs(),
            "catalog_comparison": {
                "matches": not mismatches,
                "mismatched_task_ids": mismatches,
            },
        }


def validate_isolated_inputs(args: argparse.Namespace) -> None:
    scenario_workspace = Path(args.scenario_workspace).resolve()
    runtime_workspace = Path(args.runtime_workspace).resolve()
    compozy_home = Path(args.compozy_home).resolve()
    default_home = (Path.home() / ".compozy").resolve()
    parsed_url = urlparse.urlparse(args.api_base_url)
    if not scenario_workspace.is_dir():
        raise PublicReadError(f"scenario workspace does not exist: {scenario_workspace}")
    if not runtime_workspace.is_dir():
        raise PublicReadError(f"runtime workspace does not exist: {runtime_workspace}")
    if compozy_home == default_home:
        raise PublicReadError(f"COMPOZY_HOME must be isolated, got default path: {compozy_home}")
    if not compozy_home.is_dir():
        raise PublicReadError(f"isolated COMPOZY_HOME does not exist: {compozy_home}")
    if parsed_url.scheme not in {"http", "https"} or not parsed_url.hostname or not parsed_url.port:
        raise PublicReadError("api base URL must include scheme, host, and isolated port")
    if parsed_url.hostname not in {"127.0.0.1", "localhost", "::1"}:
        raise PublicReadError("api base URL must target the isolated loopback daemon")
    if not args.workspace_id.strip():
        raise PublicReadError("workspace id must be non-empty")
    if not args.compozy_bin.strip():
        raise PublicReadError("compozy CLI input must be non-empty")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scenario-workspace", required=True, help="WORKSPACE_PATH from manifest")
    parser.add_argument("--runtime-workspace", required=True, help="RUNTIME_WORKSPACE_PATH from manifest")
    parser.add_argument("--workspace-id", required=True, help="Registered runtime workspace ID")
    parser.add_argument("--api-base-url", required=True, help="Manifest-derived isolated daemon base URL")
    parser.add_argument("--compozy-home", required=True, help="Manifest-derived isolated COMPOZY_HOME")
    parser.add_argument("--compozy-bin", required=True, help="Exact Compozy CLI binary to invoke")
    parser.add_argument("--qa-output-path", required=True, help="QA_OUTPUT_PATH from manifest")
    parser.add_argument("--duration-sec", type=float, default=1800)
    parser.add_argument("--stall-threshold-sec", type=float, default=300)
    parser.add_argument("--poll-interval-sec", type=float, default=2)
    parser.add_argument("--command-timeout-sec", type=float, default=30)
    args = parser.parse_args()

    summary_path = Path(args.qa_output_path).resolve() / "qa" / "observation-summary.json"
    summary_path.parent.mkdir(parents=True, exist_ok=True)
    try:
        validate_isolated_inputs(args)
        reader = PublicRuntimeReader(
            scenario_workspace=Path(args.scenario_workspace),
            runtime_workspace=Path(args.runtime_workspace),
            workspace_id=args.workspace_id,
            api_base_url=args.api_base_url,
            compozy_home=Path(args.compozy_home),
            compozy_bin=args.compozy_bin,
            command_timeout_sec=args.command_timeout_sec,
        )
        summary = run_observation(
            reader,
            duration_sec=args.duration_sec,
            stall_threshold_sec=args.stall_threshold_sec,
            poll_interval_sec=args.poll_interval_sec,
        )
    except PublicReadError as exc:
        summary = observation_result(
            "error", {"tasks": {}, "loop_runs": {}}, [], 0, error=str(exc)
        )
    summary.update(
        {
            "scenario_workspace": str(Path(args.scenario_workspace).resolve()),
            "runtime_workspace": str(Path(args.runtime_workspace).resolve()),
            "workspace_id": args.workspace_id,
            "api_base_url": args.api_base_url,
            "compozy_home": str(Path(args.compozy_home).resolve()),
            "compozy_bin": args.compozy_bin,
            "duration_sec_requested": args.duration_sec,
            "stall_threshold_sec": args.stall_threshold_sec,
        }
    )
    summary_path.write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    diagnostic = {
        "outcome": summary["outcome"],
        "stall_detected": summary["stall_detected"],
        "error": summary.get("error"),
        "progress_transitions": summary["progress_transitions"],
        "summary_path": str(summary_path),
    }
    if summary["outcome"] == "error":
        print(json.dumps(diagnostic, sort_keys=True), file=sys.stderr)
        return 2
    if summary["stall_detected"]:
        print(json.dumps(diagnostic, sort_keys=True), file=sys.stderr)
        return 1
    print(json.dumps(diagnostic, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

"""Behavior tests for the real-scenario runtime observer.

Suite: runtime-owned real-scenario progress observer
Invariant: durable public Task and Loop transitions are the only progress clock.
Boundary IN: observer polling, terminal detection, stall diagnosis, and read errors.
Boundary OUT: real Compozy CLI/API transport, covered by the isolated one-kickoff re-walk.
"""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock


SCRIPT_PATH = Path(__file__).with_name("observe-runtime.py")
SPEC = importlib.util.spec_from_file_location("observe_runtime", SCRIPT_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"unable to load runtime observer: {SCRIPT_PATH}")
OBSERVER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(OBSERVER)


class FakeClock:
    def __init__(self) -> None:
        self.value = 0.0

    def monotonic(self) -> float:
        return self.value

    def sleep(self, seconds: float) -> None:
        self.value += seconds


def snapshot(status: str, event_seq: int, *, updated_at: str | None = None) -> dict:
    return {
        "tasks": {
            "task-1": {
                "id": "task-1",
                "status": status,
                "latest_event_seq": event_seq,
                "current_run_id": "run-1",
                "runs": [{"id": "run-1", "status": status}],
                "events": [{"sequence": event_seq}],
                **({"updated_at": updated_at} if updated_at else {}),
            }
        },
        "loop_runs": {},
    }


class SequenceReader:
    def __init__(self, snapshots: list[dict]) -> None:
        self.snapshots = snapshots
        self.index = 0

    def read(self) -> dict:
        current = self.snapshots[min(self.index, len(self.snapshots) - 1)]
        self.index += 1
        return current


class JSONResponse:
    def __init__(self, payload: object) -> None:
        self.payload = payload

    def __enter__(self) -> "JSONResponse":
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self) -> bytes:
        if isinstance(self.payload, str):
            return self.payload.encode("utf-8")
        return json.dumps(self.payload).encode("utf-8")


class PublicRuntimeReaderTest(unittest.TestCase):
    workspace_id = "ws_observer"
    task_id = "task-observer-001"
    loop_run_id = "looprun-observer-001"

    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        root = Path(self.temp_dir.name)
        self.scenario_workspace = root / "scenario"
        self.runtime_workspace = root / "runtime-workspace"
        self.compozy_home = root / "compozy-home"
        self.scenario_workspace.mkdir()
        self.runtime_workspace.mkdir()
        self.compozy_home.mkdir()
        task_dir = self.scenario_workspace / ".compozy" / "tasks"
        task_dir.mkdir(parents=True)
        (task_dir / "open-tasks.json").write_text(
            json.dumps([{"runtime_id": self.task_id}]),
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def workspace_payload(self, *, workspace_id: str | None = None, root_dir: str | None = None) -> dict:
        return {
            "workspace": {
                "id": workspace_id or self.workspace_id,
                "root_dir": root_dir or str(self.runtime_workspace),
                "add_dirs": [],
                "name": "observer-runtime",
                "created_at": "2026-08-21T13:00:00Z",
                "updated_at": "2026-08-21T13:00:00Z",
            }
        }

    def task_catalog_payload(self, *, status: str = "in_progress", updated_at: str = "2026-08-21T13:01:00Z") -> dict:
        return {
            "tasks": [
                {
                    "id": self.task_id,
                    "scope": "workspace",
                    "workspace_id": self.workspace_id,
                    "title": "Observe runtime progress",
                    "status": status,
                    "current_run_id": "run-task-001",
                    "latest_event_seq": 7,
                    "created_at": "2026-08-21T13:00:00Z",
                    "updated_at": updated_at,
                }
            ],
            "next_cursor": "",
        }

    def task_detail_payload(
        self,
        *,
        status: str = "in_progress",
        task_id: str | None = None,
        workspace_id: str | None = None,
    ) -> dict:
        detail_task_id = task_id or self.task_id
        detail_workspace_id = workspace_id or self.workspace_id
        summary = {
            "id": detail_task_id,
            "scope": "workspace",
            "workspace_id": detail_workspace_id,
            "title": "Observe runtime progress",
            "status": status,
            "current_run_id": "run-task-001",
            "latest_event_seq": 7,
        }
        return {
            "summary": summary,
            "task": {**summary, "description": "Read runtime-owned state."},
            "runs": [{"id": "run-task-001", "task_id": detail_task_id, "status": "running", "attempt": 1}],
            "events": [
                {
                    "id": "evt-task-007",
                    "task_id": detail_task_id,
                    "event_type": "task.run_started",
                    "timestamp": "2026-08-21T13:01:00Z",
                }
            ],
        }

    def loop_runs_payload(self, *, status: str = "running", workspace_id: str | None = None) -> dict:
        return {
            "runs": [
                {
                    "id": self.loop_run_id,
                    "workspace_id": workspace_id or self.workspace_id,
                    "loop_name": "observer-loop",
                    "status": status,
                    "completion_state": "completed" if status == "no-op" else "",
                    "generation": 1,
                    "last_progress_at": "2026-08-21T13:02:00Z",
                    "progress": {"round": 1, "steps_done": 1, "steps_total": 1},
                }
            ],
            "aggregates": {"total": 1, "live": 0, "terminal": 1, "succeeded": 1, "failed": 0},
        }

    def cli_result(self, payload: dict) -> subprocess.CompletedProcess[str]:
        return subprocess.CompletedProcess([], 0, stdout=json.dumps(payload), stderr="")

    def cli_router(self, responses: dict[tuple[str, ...], dict]):
        def run(arguments: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
            self.assertEqual(kwargs["env"]["COMPOZY_HOME"], str(self.compozy_home.resolve()))
            command = tuple(arguments[1:-2])
            if command not in responses:
                self.fail(f"unexpected public CLI command: {command}")
            return self.cli_result(responses[command])

        return run

    def reader(self):
        return OBSERVER.PublicRuntimeReader(
            scenario_workspace=self.scenario_workspace,
            runtime_workspace=self.runtime_workspace,
            workspace_id=self.workspace_id,
            api_base_url="http://127.0.0.1:53317",
            compozy_home=self.compozy_home,
            compozy_bin="compozy",
            command_timeout_sec=5,
        )

    def base_cli_responses(self, *, catalog_status: str = "in_progress", detail_status: str = "in_progress") -> dict:
        return {
            ("workspace", "info", self.workspace_id): self.workspace_payload(),
            ("task", "list", "--workspace", self.workspace_id, "--limit", "200"): self.task_catalog_payload(
                status=catalog_status
            ),
            ("task", "get", self.task_id): self.task_detail_payload(status=detail_status),
        }

    def test_validates_registered_workspace_id_and_canonical_root_before_polling(self) -> None:
        alias = self.runtime_workspace.parent / "runtime-alias"
        alias.symlink_to(self.runtime_workspace, target_is_directory=True)
        responses = {("workspace", "info", self.workspace_id): self.workspace_payload(root_dir=str(alias))}

        with mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(responses)) as run:
            self.reader()

        run.assert_called_once()

    def test_rejects_malformed_or_mismatched_workspace_records(self) -> None:
        cases = {
            "malformed workspace": {"workspace": []},
            "mismatched id": self.workspace_payload(workspace_id="ws_foreign"),
            "mismatched root": self.workspace_payload(root_dir=str(self.scenario_workspace)),
        }
        for name, payload in cases.items():
            with self.subTest(name=name):
                responses = {("workspace", "info", self.workspace_id): payload}
                with mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(responses)):
                    with self.assertRaises(OBSERVER.PublicReadError):
                        self.reader()

    def test_rejects_malformed_cli_and_api_json(self) -> None:
        malformed = subprocess.CompletedProcess([], 0, stdout="{", stderr="")
        with mock.patch.object(OBSERVER.subprocess, "run", return_value=malformed):
            with self.assertRaisesRegex(OBSERVER.PublicReadError, "malformed JSON"):
                self.reader()

        responses = self.base_cli_responses()
        with (
            mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(responses)),
            mock.patch.object(OBSERVER.urlrequest, "urlopen", return_value=JSONResponse("{")),
        ):
            with self.assertRaisesRegex(OBSERVER.PublicReadError, "malformed JSON"):
                self.reader().read()

    def test_parses_public_task_catalog_and_detail_contracts(self) -> None:
        responses = self.base_cli_responses()
        with (
            mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(responses)),
            mock.patch.object(OBSERVER.urlrequest, "urlopen", return_value=JSONResponse({"runs": []})),
        ):
            result = self.reader().read()

        task = result["tasks"][self.task_id]
        self.assertEqual(task["status"], "in_progress")
        self.assertEqual(task["latest_event_seq"], 7)
        self.assertTrue(task["catalog_matches_detail"])
        self.assertNotIn("updated_at", task)

    def test_reports_catalog_detail_status_mismatch(self) -> None:
        responses = self.base_cli_responses(catalog_status="in_progress", detail_status="completed")
        with (
            mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(responses)),
            mock.patch.object(OBSERVER.urlrequest, "urlopen", return_value=JSONResponse({"runs": []})),
        ):
            result = self.reader().read()

        self.assertFalse(result["catalog_comparison"]["matches"])
        self.assertEqual(result["catalog_comparison"]["mismatched_task_ids"], [self.task_id])

    def test_reads_loop_runs_why_events_and_accepts_no_op_as_terminal(self) -> None:
        responses = {
            **self.base_cli_responses(catalog_status="completed", detail_status="completed"),
            ("loop", "why", self.loop_run_id, "--workspace", self.workspace_id): {
                "run_id": self.loop_run_id,
                "status": "no-op",
                "headline": "Nothing needed to run",
                "progress": {"round": 1, "steps_done": 1, "steps_total": 1},
                "blockers": [],
            },
            (
                "loop",
                "events",
                self.loop_run_id,
                "--workspace",
                self.workspace_id,
                "--view",
                "all",
                "--limit",
                "500",
            ): {
                "run_id": self.loop_run_id,
                "view": "all",
                "head_seq": 3,
                "entries": [{"seq": 3, "kind": "run_no_op", "at": "2026-08-21T13:02:00Z"}],
            },
        }
        with (
            mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(responses)) as run,
            mock.patch.object(
                OBSERVER.urlrequest,
                "urlopen",
                return_value=JSONResponse(self.loop_runs_payload(status="no-op")),
            ) as urlopen,
        ):
            result = self.reader().read()

        self.assertTrue(OBSERVER.all_terminal(result))
        urlopen.assert_called_once_with(
            f"http://127.0.0.1:53317/api/workspaces/{self.workspace_id}/loop-runs?limit=500",
            timeout=5,
        )
        commands = [tuple(call.args[0][1:-2]) for call in run.call_args_list]
        self.assertIn(("loop", "why", self.loop_run_id, "--workspace", self.workspace_id), commands)
        self.assertIn(
            ("loop", "events", self.loop_run_id, "--workspace", self.workspace_id, "--view", "all", "--limit", "500"),
            commands,
        )

    def test_rejects_mismatched_task_and_loop_records(self) -> None:
        task_responses = self.base_cli_responses()
        task_responses[("task", "get", self.task_id)] = self.task_detail_payload(task_id="task-foreign")
        with (
            mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(task_responses)),
            mock.patch.object(OBSERVER.urlrequest, "urlopen", return_value=JSONResponse({"runs": []})),
        ):
            with self.assertRaises(OBSERVER.PublicReadError):
                self.reader().read()

        loop_responses = self.base_cli_responses()
        with (
            mock.patch.object(OBSERVER.subprocess, "run", side_effect=self.cli_router(loop_responses)),
            mock.patch.object(
                OBSERVER.urlrequest,
                "urlopen",
                return_value=JSONResponse(self.loop_runs_payload(workspace_id="ws_foreign")),
            ),
        ):
            with self.assertRaises(OBSERVER.PublicReadError):
                self.reader().read()


class ObserveRuntimeTest(unittest.TestCase):
    def test_public_advance_without_journey_log_growth_does_not_stall(self) -> None:
        clock = FakeClock()
        result = OBSERVER.run_observation(
            SequenceReader(
                [
                    snapshot("in_progress", 1),
                    snapshot("in_progress", 2),
                    snapshot("in_progress", 3),
                    snapshot("completed", 4),
                ]
            ),
            duration_sec=10,
            stall_threshold_sec=2,
            poll_interval_sec=1,
            monotonic=clock.monotonic,
            sleep=clock.sleep,
        )

        self.assertFalse(result["stall_detected"])
        self.assertEqual(result["outcome"], "all_terminal")
        self.assertGreaterEqual(result["progress_transitions"], 3)

    def test_unchanged_active_public_state_is_a_stall(self) -> None:
        clock = FakeClock()
        result = OBSERVER.run_observation(
            SequenceReader([snapshot("in_progress", 1)]),
            duration_sec=10,
            stall_threshold_sec=2,
            poll_interval_sec=1,
            monotonic=clock.monotonic,
            sleep=clock.sleep,
        )

        self.assertTrue(result["stall_detected"])
        self.assertEqual(result["outcome"], "stall")
        self.assertEqual(result["diagnose"]["tasks_active_unchanged"], ["task-1"])

    def test_timestamp_only_change_does_not_reset_the_progress_clock(self) -> None:
        clock = FakeClock()
        result = OBSERVER.run_observation(
            SequenceReader(
                [
                    snapshot("in_progress", 1, updated_at="2026-08-21T13:00:00Z"),
                    snapshot("in_progress", 1, updated_at="2026-08-21T13:00:01Z"),
                    snapshot("in_progress", 1, updated_at="2026-08-21T13:00:02Z"),
                ]
            ),
            duration_sec=10,
            stall_threshold_sec=2,
            poll_interval_sec=1,
            monotonic=clock.monotonic,
            sleep=clock.sleep,
        )

        self.assertTrue(result["stall_detected"])
        self.assertEqual(result["outcome"], "stall")
        self.assertEqual(result["progress_transitions"], 0)

    def test_heartbeat_only_change_does_not_reset_the_progress_clock(self) -> None:
        snapshots = []
        for second in range(3):
            state = snapshot("in_progress", 1)
            state["tasks"]["task-1"]["runs"][0]["heartbeat_at"] = f"2026-08-21T13:00:0{second}Z"
            snapshots.append(state)
        clock = FakeClock()

        result = OBSERVER.run_observation(
            SequenceReader(snapshots),
            duration_sec=10,
            stall_threshold_sec=2,
            poll_interval_sec=1,
            monotonic=clock.monotonic,
            sleep=clock.sleep,
        )

        self.assertTrue(result["stall_detected"])
        self.assertEqual(result["progress_transitions"], 0)

    def test_all_terminal_public_tasks_finish_cleanly(self) -> None:
        clock = FakeClock()
        result = OBSERVER.run_observation(
            SequenceReader([snapshot("completed", 9)]),
            duration_sec=10,
            stall_threshold_sec=2,
            poll_interval_sec=1,
            monotonic=clock.monotonic,
            sleep=clock.sleep,
        )

        self.assertFalse(result["stall_detected"])
        self.assertEqual(result["outcome"], "all_terminal")

    def test_public_read_failure_is_an_honest_error(self) -> None:
        class BrokenReader:
            def read(self) -> dict:
                raise OBSERVER.PublicReadError("task catalog returned malformed JSON")

        clock = FakeClock()
        result = OBSERVER.run_observation(
            BrokenReader(),
            duration_sec=10,
            stall_threshold_sec=2,
            poll_interval_sec=1,
            monotonic=clock.monotonic,
            sleep=clock.sleep,
        )

        self.assertFalse(result["stall_detected"])
        self.assertEqual(result["outcome"], "error")
        self.assertIn("malformed JSON", result["error"])


if __name__ == "__main__":
    unittest.main()

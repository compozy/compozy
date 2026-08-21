"""Behavior tests for the real-scenario runtime observer.

Suite: runtime-owned real-scenario progress observer
Invariant: durable public Task and Loop transitions are the only progress clock.
Boundary IN: observer polling, terminal detection, stall diagnosis, and read errors.
Boundary OUT: real Compozy CLI/API transport, covered by the isolated one-kickoff re-walk.
"""

from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest


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


def snapshot(status: str, event_seq: int) -> dict:
    return {
        "tasks": {
            "task-1": {
                "id": "task-1",
                "status": status,
                "latest_event_seq": event_seq,
                "current_run_id": "run-1",
                "runs": [{"id": "run-1", "status": status}],
                "events": [{"sequence": event_seq}],
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

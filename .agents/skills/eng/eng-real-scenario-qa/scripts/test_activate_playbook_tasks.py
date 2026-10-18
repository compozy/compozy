"""Regression tests for playbook task activation.

Suite: real-scenario task activation adapter
Invariant: task activation pauses dispatch and starts each declared Task with a stable idempotency key.
Boundary IN: activate-playbook-tasks argument construction and activation evidence.
Boundary OUT: the Compozy CLI parser and daemon, covered by the live real-scenario run.
"""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


SCRIPT_PATH = Path(__file__).with_name("activate-playbook-tasks.py")
SPEC = importlib.util.spec_from_file_location("activate_playbook_tasks", SCRIPT_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"unable to load activation helper: {SCRIPT_PATH}")
ACTIVATION = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ACTIVATION)


class ActivatePlaybookTasksTest(unittest.TestCase):
    def test_declared_task_starts_behind_scheduler_barrier(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            workspace = root / "workspace"
            qa_output = root / "qa-output"
            tasks_dir = workspace / ".compozy" / "tasks"
            tasks_dir.mkdir(parents=True)
            (tasks_dir / "open-tasks.json").write_text(
                json.dumps(
                    [
                        {
                            "runtime_id": "task-playbook-001",
                            "playbook_ref": "consumer-saas-growth",
                        }
                    ]
                ),
                encoding="utf-8",
            )
            manifest = root / "bootstrap-manifest.json"
            manifest.write_text(
                json.dumps(
                    {
                        "env": {
                            "COMPOZY_HOME": str(root / "runtime"),
                            "KICKOFF_POSTED": "false",
                        }
                    }
                ),
                encoding="utf-8",
            )

            calls: list[list[str]] = []

            def runner(_compozy_bin: str, args: list[str], _env: dict[str, str]) -> dict:
                calls.append(args)
                if args == ["scheduler", "status"]:
                    return {"scheduler": {"paused": False}}
                if args[:2] == ["scheduler", "pause"]:
                    return {"scheduler": {"paused": True}}
                if args[:2] == ["task", "start"]:
                    return {
                        "task": {"id": "task-playbook-001"},
                        "run": {"id": "run-playbook-001"},
                    }
                raise AssertionError(f"unexpected command: {args}")

            result = ACTIVATION.prepare_activation(
                workspace,
                qa_output,
                manifest,
                "compozy",
                runner=runner,
                recorder=lambda *_args: None,
            )

            self.assertEqual(result["status"], "prepared")
            self.assertTrue(result["scheduler_pause_owned"])
            self.assertEqual([args[:2] for args in calls], [["scheduler", "status"], ["scheduler", "pause"], ["task", "start"]])
            start_args = next(args for args in calls if args[:2] == ["task", "start"])
            self.assertEqual(
                start_args[:5],
                [
                    "task",
                    "start",
                    "task-playbook-001",
                    "--idempotency-key",
                    "real-scenario:consumer-saas-growth:task-playbook-001",
                ],
            )


if __name__ == "__main__":
    unittest.main()

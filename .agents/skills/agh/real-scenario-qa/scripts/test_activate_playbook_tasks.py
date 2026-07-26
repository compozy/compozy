"""Regression tests for playbook task activation.

Suite: real-scenario task activation adapter
Invariant: a declared channel starts a live run through the current named-channel CLI contract.
Boundary IN: activate-playbook-tasks argument construction and activation evidence.
Boundary OUT: the AGH CLI parser and daemon, covered by the live real-scenario run.
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
    def test_declared_channel_uses_named_live_network_contract(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            workspace = root / "workspace"
            qa_output = root / "qa-output"
            tasks_dir = workspace / ".agh" / "tasks"
            tasks_dir.mkdir(parents=True)
            (tasks_dir / "open-tasks.json").write_text(
                json.dumps(
                    [
                        {
                            "runtime_id": "task-playbook-001",
                            "playbook_ref": "consumer-saas-growth",
                            "channel": "growth-room",
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
                            "AGH_HOME": str(root / "runtime"),
                            "KICKOFF_POSTED": "false",
                        }
                    }
                ),
                encoding="utf-8",
            )

            calls: list[list[str]] = []

            def runner(_agh_bin: str, args: list[str], _env: dict[str, str]) -> dict:
                calls.append(args)
                if args == ["scheduler", "status"]:
                    return {"scheduler": {"paused": False}}
                if args[:2] == ["scheduler", "pause"]:
                    return {"scheduler": {"paused": True}}
                if args[:2] == ["task", "start"]:
                    if "--channel" in args:
                        raise RuntimeError("unknown flag: --channel")
                    return {
                        "task": {"id": "task-playbook-001"},
                        "run": {"id": "run-playbook-001"},
                    }
                raise AssertionError(f"unexpected command: {args}")

            ACTIVATION.prepare_activation(
                workspace,
                qa_output,
                manifest,
                "agh",
                runner=runner,
                recorder=lambda *_args: None,
            )

            start_args = next(args for args in calls if args[:2] == ["task", "start"])
            self.assertEqual(
                start_args[-6:],
                [
                    "--network",
                    "live",
                    "--network-channel-strategy",
                    "named",
                    "--network-channel",
                    "growth-room",
                ],
            )


if __name__ == "__main__":
    unittest.main()

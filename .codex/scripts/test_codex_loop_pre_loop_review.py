#!/usr/bin/env python3
# Suite: codex-loop pre-continuation target selection
# Invariant: the canonical cy-loop-tasks path selects the specialized loop target and resolves its installed state codec.
# Boundary IN: payload classification, canonical bundle layout, and state loading.
# Boundary OUT: live Compozy execution and reviewer output.

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path
from types import ModuleType


SCRIPT = Path(__file__).with_name("codex-loop-pre-loop-review.py")


def load_hook() -> ModuleType:
    spec = importlib.util.spec_from_file_location("codex_loop_pre_loop_review", SCRIPT)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"unable to load hook from {SCRIPT}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


hook = load_hook()


class CodexLoopPreLoopReviewTests(unittest.TestCase):
    def test_canonical_skill_path_selects_cy_loop_target(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            workspace = Path(tmp)
            parser_path = (
                workspace
                / ".agents/skills/cy-loop-tasks/scripts/_state_io.py"
            )
            parser_path.parent.mkdir(parents=True)
            parser_path.write_text(
                "def load(path):\n"
                "    return {'iterations': [{'action': 'slice test scope', "
                "'outcome': 'completed'}]}\n",
                encoding="utf-8",
            )
            state_path = workspace / ".compozy/tasks/demo/state.yaml"
            state_path.parent.mkdir(parents=True)
            state_path.write_text("fixture: true\n", encoding="utf-8")
            payload = {
                "loop": {
                    "activation_prompt": (
                        "Use .agents/skills/cy-loop-tasks/SKILL.md for demo"
                    ),
                    "slug": "demo",
                }
            }

            target = hook.build_cy_target(payload, workspace)

            self.assertIsNotNone(target)
            self.assertEqual(target.mode, "cy-loop-tasks-slice")
            self.assertEqual(target.slug, "demo")

    def test_unrelated_prompt_does_not_select_cy_loop_target(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            target = hook.build_cy_target(
                {"loop": {"activation_prompt": "Run an unrelated task"}},
                Path(tmp),
            )

            self.assertIsNone(target)


if __name__ == "__main__":
    unittest.main()

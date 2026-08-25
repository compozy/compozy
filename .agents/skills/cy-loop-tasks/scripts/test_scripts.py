#!/usr/bin/env python3
"""Self-test for the cy-loop-tasks helper scripts (read-only, tmpdir-scoped).

Run from anywhere:
    python3 .agents/skills/cy-loop-tasks/scripts/test_scripts.py
"""
from __future__ import annotations

import importlib.util
import io
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import ModuleType
from unittest import mock


SCRIPTS = Path(__file__).resolve().parent
SKILL_ROOT = SCRIPTS.parent


def load_module(name: str, path: Path) -> ModuleType:
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"unable to load module from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


state_io = load_module("state_io", SCRIPTS / "_state_io.py")
commit_checkpoint = load_module("commit_checkpoint", SCRIPTS / "commit-checkpoint.py")


def _git(cwd: Path, *args: str, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=cwd,
        check=False,
        text=True,
        capture_output=True,
        env=env,
    )


def _git_env() -> dict[str, str]:
    env = os.environ.copy()
    env.update(
        {
            "GIT_AUTHOR_NAME": "test-runner",
            "GIT_AUTHOR_EMAIL": "test@example.com",
            "GIT_COMMITTER_NAME": "test-runner",
            "GIT_COMMITTER_EMAIL": "test@example.com",
            "GIT_CONFIG_GLOBAL": "/dev/null",
            "GIT_CONFIG_SYSTEM": "/dev/null",
        }
    )
    return env


class CyLoopTasksScriptTests(unittest.TestCase):
    def run_script(self, script: str, *args: str, cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(SCRIPTS / script), *args],
            cwd=str(cwd) if cwd else SKILL_ROOT,
            check=False,
            text=True,
            capture_output=True,
        )

    def write_state(self, tasks_root: Path, slug: str, **overrides: object) -> Path:
        slug_dir = tasks_root / slug
        slug_dir.mkdir(parents=True, exist_ok=True)
        state = {
            "slug": slug,
            "created_at": "2026-05-05T00:00:00Z",
            "last_updated": "2026-05-05T00:00:00Z",
            "mode": "free",
            "iteration": 0,
            "goal_signature": "test goal",
            "frontend_agent": None,
            "stacked": False,
            "tasks": {"total": 0, "completed": [], "current": None, "pending": []},
            "progress": {"deliverables_complete": True, "checklist": []},
            "qa": {"report_done": True, "execution_done": True},
            "review": {"rounds": 1, "last_verdict": "SHIP", "ship": True},
            "verify": {"last_run": "2026-05-05T00:00:00Z", "last_status": "PASS"},
            "delivery": {
                "pr_urls": ["https://github.com/compozy/compozy/pull/123"],
                "head_shas": ["a" * 40],
                "ci_status": "PASS",
                "checks": ["Verify", "E2E (combined lane)"],
                "observed_at": "2026-05-05T00:00:00Z",
            },
            "iterations": [],
        }
        state.update(overrides)
        state_path = slug_dir / "state.yaml"
        state_io.dump(state, state_path)
        return state_path

    # ---- skill behavior contract ----------------------------------------

    def test_skill_contract_repairs_stale_codegen_inside_current_action(self) -> None:
        skill = (SKILL_ROOT / "SKILL.md").read_text(encoding="utf-8")
        recovery = (SKILL_ROOT / "references" / "recovery-loop.md").read_text(
            encoding="utf-8"
        )

        self.assertIn("**self-healing continue** loop", skill)
        self.assertIn("Do not write final iteration state", skill)
        self.assertIn("A failure is **repairable by default**", recovery)
        self.assertIn("A stale Daytona sidecar is this case, not a blocker", recovery)
        self.assertIn("run the canonical generator", recovery.lower())

    def test_skill_contract_reserves_blocked_for_external_dependencies(self) -> None:
        skill = (SKILL_ROOT / "SKILL.md").read_text(encoding="utf-8")
        recovery = (SKILL_ROOT / "references" / "recovery-loop.md").read_text(
            encoding="utf-8"
        )

        self.assertIn("## External-blocker test", recovery)
        self.assertIn("Stop only when all of these are true", recovery)
        self.assertIn("Every safe in-scope alternative", recovery)
        self.assertNotIn("Verify FAIL blocks the iteration", skill)
        self.assertNotIn("Two consecutive verify failures", skill)
        self.assertNotIn("a second failure is a blocker", skill)

    # ---- update-state.py -------------------------------------------------

    def test_complete_progress_requires_exact_existing_text(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "progress"
            state_path = self.write_state(
                tasks_root,
                slug,
                progress={
                    "deliverables_complete": False,
                    "checklist": [
                        {"text": "Implement slice", "status": "in_progress", "iteration": 1}
                    ],
                },
            )

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--complete-progress",
                " Implement slice ",
            )

            self.assertEqual(result.returncode, 1)
            self.assertIn("did not match", result.stderr)
            state = state_io.load(state_path)
            self.assertEqual(len(state["progress"]["checklist"]), 1)
            self.assertEqual(state["progress"]["checklist"][0]["status"], "in_progress")

            ok = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--complete-progress",
                "Implement slice",
            )

            self.assertEqual(ok.returncode, 0, ok.stderr)
            updated = state_io.load(state_path)
            self.assertEqual(updated["progress"]["checklist"][0]["status"], "completed")
            self.assertEqual(updated["iterations"][0]["phase"], "B")

    def test_update_state_drops_stale_top_level_current_phase(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "phase"
            state_path = self.write_state(tasks_root, slug)
            original = state_path.read_text(encoding="utf-8")
            state_path.write_text(
                original.replace("mode:", 'current_phase: "E"\nmode:', 1),
                encoding="utf-8",
            )

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "round closed",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            text = state_path.read_text(encoding="utf-8")
            self.assertNotIn("current_phase:", text)
            updated = state_io.load(state_path)
            self.assertEqual(updated["iterations"][0]["phase"], "B")

    def test_update_state_records_review_rounds_until_ship(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "review"
            state_path = self.write_state(
                tasks_root,
                slug,
                review={"rounds": 0, "last_verdict": None, "ship": False},
            )

            first = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "D",
                "--action",
                "peer-review round 1 (FIX_BEFORE_SHIP)",
                "--review-round-done",
                "FIX_BEFORE_SHIP",
                "--verify-fail",
            )

            self.assertEqual(first.returncode, 0, first.stderr)
            state = state_io.load(state_path)
            self.assertEqual(state["review"]["rounds"], 1)
            self.assertEqual(state["review"]["last_verdict"], "FIX_BEFORE_SHIP")
            self.assertFalse(state["review"]["ship"])
            self.assertEqual(state["iterations"][0]["phase"], "D")

            second = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "D",
                "--action",
                "peer-review round 2 (SHIP)",
                "--review-round-done",
                "SHIP",
                "--verify-pass",
            )

            self.assertEqual(second.returncode, 0, second.stderr)
            state = state_io.load(state_path)
            self.assertEqual(state["review"]["rounds"], 2)
            self.assertEqual(state["review"]["last_verdict"], "SHIP")
            self.assertTrue(state["review"]["ship"])

    def test_update_state_reconciles_task_files_into_tasks_mode(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "reconcile"
            slug_dir = tasks_root / slug
            state_path = self.write_state(
                tasks_root,
                slug,
                mode="free",
                progress={"deliverables_complete": False, "checklist": []},
                qa={"report_done": False, "execution_done": False},
            )
            (slug_dir / "_tasks.md").write_text("# tasks\n", encoding="utf-8")
            (slug_dir / "task_01.md").write_text(
                "---\nstatus: completed\ntitle: one\ntype: backend\n---\n\n# one\n",
                encoding="utf-8",
            )
            (slug_dir / "task_02.md").write_text(
                "---\nstatus: in_progress\ntitle: two\ntype: backend\n---\n\n# two\n",
                encoding="utf-8",
            )
            (slug_dir / "task_03.md").write_text(
                "---\nstatus: pending\ntitle: three\ntype: test\n---\n\n# three\n",
                encoding="utf-8",
            )

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "reconcile task graph",
                "--outcome",
                "completed",
                "--reconcile-tasks",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            updated = state_io.load(state_path)
            self.assertEqual(updated["mode"], "tasks")
            self.assertEqual(updated["tasks"]["total"], 3)
            self.assertEqual(updated["tasks"]["completed"], ["task_01"])
            self.assertEqual(updated["tasks"]["current"], "task_02")
            self.assertEqual(updated["tasks"]["pending"], ["task_02", "task_03"])

            phase = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(phase.returncode, 0, phase.stderr)
            self.assertEqual(
                phase.stdout.strip(),
                "phase=B action=execute_task task=task_02",
            )

    # ---- init-state.py ---------------------------------------------------

    def test_init_state_captures_frontend_agent_and_review_defaults(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "frontend"
            slug_dir = tasks_root / slug
            slug_dir.mkdir(parents=True)
            (slug_dir / "_spec.md").write_text("# spec\n", encoding="utf-8")

            result = self.run_script(
                "init-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
                "--frontend",
                "cursor",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            state = state_io.load(slug_dir / "state.yaml")
            self.assertEqual(state["frontend_agent"], "cursor")
            self.assertEqual(state["review"]["rounds"], 0)
            self.assertIsNone(state["review"]["last_verdict"])
            self.assertFalse(state["review"]["ship"])

    def test_init_state_preserves_multiline_goal_through_update(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "multiline-goal"
            slug_dir = tasks_root / slug
            slug_dir.mkdir(parents=True)
            (slug_dir / "_spec.md").write_text("# spec\n", encoding="utf-8")
            goal = "ship the workflow\n- delegate frontend work"

            initialized = self.run_script(
                "init-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--goal",
                goal,
            )

            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            updated = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "0",
                "--action",
                "bootstrap",
            )
            self.assertEqual(updated.returncode, 0, updated.stderr)
            state = state_io.load(slug_dir / "state.yaml")
            self.assertEqual(state["goal_signature"], goal)

    def test_update_state_disables_stacked_checkpoints_explicitly(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "leave-stack"
            state_path = self.write_state(tasks_root, slug, stacked=True)

            updated = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "switch to one branch",
                "--disable-stacked",
            )

            self.assertEqual(updated.returncode, 0, updated.stderr)
            state = state_io.load(state_path)
            self.assertIs(state["stacked"], False)
            self.assertEqual(state["iterations"][-1]["action"], "switch to one branch")

    def test_update_state_disables_frontend_delegation_explicitly(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "switch-local"
            state_path = self.write_state(tasks_root, slug, frontend_agent="claude")

            updated = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "switch to local execution",
                "--disable-frontend",
            )

            self.assertEqual(updated.returncode, 0, updated.stderr)
            state = state_io.load(state_path)
            self.assertIsNone(state["frontend_agent"])
            self.assertEqual(
                state["iterations"][-1]["action"],
                "switch to local execution",
            )

    def test_init_state_defaults_frontend_agent_to_null(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "local-only"
            slug_dir = tasks_root / slug
            slug_dir.mkdir(parents=True)
            (slug_dir / "_spec.md").write_text("# spec\n", encoding="utf-8")

            result = self.run_script(
                "init-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            state = state_io.load(slug_dir / "state.yaml")
            self.assertIsNone(state["frontend_agent"])
            self.assertFalse(state["stacked"])

    def test_init_state_records_stacked_flag_in_tasks_mode(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "stacked-tasks"
            slug_dir = tasks_root / slug
            slug_dir.mkdir(parents=True)
            (slug_dir / "_spec.md").write_text("# spec\n", encoding="utf-8")
            (slug_dir / "_tasks.md").write_text("# tasks\n", encoding="utf-8")
            (slug_dir / "task_01.md").write_text(
                "---\nstatus: pending\ntitle: first\n---\n", encoding="utf-8"
            )

            result = self.run_script(
                "init-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
                "--stacked",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("stacked=true", result.stdout)
            state = state_io.load(slug_dir / "state.yaml")
            self.assertTrue(state["stacked"])

    def test_init_state_rejects_stacked_in_free_mode(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "stacked-free"
            slug_dir = tasks_root / slug
            slug_dir.mkdir(parents=True)
            (slug_dir / "_spec.md").write_text("# spec\n", encoding="utf-8")

            result = self.run_script(
                "init-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
                "--stacked",
            )

            self.assertEqual(result.returncode, 4)
            self.assertIn("--stacked requires tasks mode", result.stderr)
            self.assertFalse((slug_dir / "state.yaml").exists())

    # ---- detect-phase.py -------------------------------------------------

    def test_detect_phase_emits_done_when_qa_review_and_verify_complete(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "done-ready"
            self.write_state(tasks_root, slug)  # defaults: qa done, SHIP, PASS

            result = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), "phase=E action=done")

    def test_detect_phase_waits_for_ci_after_local_verification(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "await-ci"
            self.write_state(
                tasks_root,
                slug,
                delivery={
                    "pr_urls": ["https://github.com/compozy/compozy/pull/123"],
                    "head_shas": ["b" * 40],
                    "ci_status": "PENDING",
                    "checks": [],
                    "observed_at": "2026-05-05T00:00:00Z",
                },
            )

            result = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), "phase=E action=await_ci")

    def test_update_state_rejects_ci_pass_without_reported_checks(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "ci-without-checks"
            self.write_state(tasks_root, slug)

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--ci-pass",
                "--pr-url",
                "https://github.com/compozy/compozy/pull/123",
                "--head-sha",
                "a" * 40,
            )

            self.assertEqual(result.returncode, 1)
            self.assertIn("at least one reported check", result.stderr)

    def test_update_state_records_ci_pass_for_exact_current_head(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            repo.mkdir()
            env = _git_env()
            self.assertEqual(_git(repo, "init", env=env).returncode, 0)
            (repo / "tracked.txt").write_text("head\n", encoding="utf-8")
            self.assertEqual(_git(repo, "add", "tracked.txt", env=env).returncode, 0)
            self.assertEqual(
                _git(repo, "commit", "-m", "test: seed", env=env).returncode,
                0,
            )
            head = _git(repo, "rev-parse", "HEAD", env=env).stdout.strip()
            tasks_root = repo / "tasks"
            slug = "ci-exact-head"
            state_path = self.write_state(tasks_root, slug)

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--repo-root",
                str(repo),
                "--ci-pass",
                "--pr-url",
                "https://github.com/compozy/compozy/pull/123",
                "--head-sha",
                head,
                "--ci-check",
                "Verify",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            delivery = state_io.load(state_path)["delivery"]
            self.assertEqual(delivery["ci_status"], "PASS")
            self.assertEqual(delivery["head_shas"], [head])
            self.assertEqual(delivery["checks"], ["Verify"])

    def test_update_state_rejects_ci_pass_for_stale_head(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            repo.mkdir()
            env = _git_env()
            self.assertEqual(_git(repo, "init", env=env).returncode, 0)
            (repo / "tracked.txt").write_text("head\n", encoding="utf-8")
            self.assertEqual(_git(repo, "add", "tracked.txt", env=env).returncode, 0)
            self.assertEqual(
                _git(repo, "commit", "-m", "test: seed", env=env).returncode,
                0,
            )
            tasks_root = repo / "tasks"
            slug = "ci-stale-head"
            self.write_state(tasks_root, slug)

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--repo-root",
                str(repo),
                "--ci-pass",
                "--pr-url",
                "https://github.com/compozy/compozy/pull/123",
                "--head-sha",
                "b" * 40,
                "--ci-check",
                "Verify",
            )

            self.assertEqual(result.returncode, 1)
            self.assertIn("does not match current HEAD", result.stderr)

    def test_detect_phase_emits_peer_review_when_qa_done_without_ship(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "needs-review"
            self.write_state(
                tasks_root,
                slug,
                review={"rounds": 0, "last_verdict": None, "ship": False},
            )

            result = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                result.stdout.strip(),
                "phase=D action=peer_review round=1",
            )

    def test_detect_phase_reenters_peer_review_when_ship_but_verify_fail(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "ship-but-broken"
            self.write_state(
                tasks_root,
                slug,
                verify={"last_run": "2026-05-05T00:00:00Z", "last_status": "FAIL"},
            )

            result = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                result.stdout.strip(),
                "phase=D action=peer_review round=2",
            )

    def test_detect_phase_routes_canonical_qa_task_types_in_order(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "qa-tail"
            slug_dir = tasks_root / slug
            state_path = self.write_state(
                tasks_root,
                slug,
                mode="tasks",
                tasks={
                    "total": 2,
                    "completed": [],
                    "current": None,
                    "pending": ["task_01", "task_02"],
                },
                qa={"report_done": False, "execution_done": False},
            )
            (slug_dir / "task_01.md").write_text(
                "---\nstatus: pending\ntitle: QA Plan and Session Charters\ntype: qa-report\n---\n",
                encoding="utf-8",
            )
            (slug_dir / "task_02.md").write_text(
                "---\nstatus: pending\ntitle: Real-User QA Execution\ntype: qa-execution\n---\n",
                encoding="utf-8",
            )

            report = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(report.returncode, 0, report.stderr)
            self.assertEqual(report.stdout.strip(), "phase=C action=qa_report")

            state = state_io.load(state_path)
            state["tasks"]["pending"] = ["task_02"]
            state["qa"]["report_done"] = True
            state_io.dump(state, state_path)

            execution = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(execution.returncode, 0, execution.stderr)
            self.assertEqual(execution.stdout.strip(), "phase=C action=qa_execution")

    def test_detect_phase_does_not_infer_qa_lane_from_title(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "qa-title-only"
            slug_dir = tasks_root / slug
            self.write_state(
                tasks_root,
                slug,
                mode="tasks",
                tasks={
                    "total": 1,
                    "completed": [],
                    "current": None,
                    "pending": ["task_01"],
                },
                qa={"report_done": False, "execution_done": False},
            )
            (slug_dir / "task_01.md").write_text(
                "---\nstatus: pending\ntitle: QA Plan and Session Charters\ntype: test\n---\n",
                encoding="utf-8",
            )

            result = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                result.stdout.strip(),
                "phase=B action=execute_task task=task_01",
            )

    def test_detect_phase_appends_frontend_lane_only_for_frontend_tasks(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "lane"
            slug_dir = tasks_root / slug
            self.write_state(
                tasks_root,
                slug,
                mode="tasks",
                frontend_agent="claude",
                tasks={
                    "total": 2,
                    "completed": [],
                    "current": None,
                    "pending": ["task_01", "task_02"],
                },
                qa={"report_done": False, "execution_done": False},
            )
            (slug_dir / "task_01.md").write_text(
                "---\nstatus: pending\ntitle: web view\ntype: frontend\n---\n",
                encoding="utf-8",
            )
            (slug_dir / "task_02.md").write_text(
                "---\nstatus: pending\ntitle: api\ntype: backend\n---\n",
                encoding="utf-8",
            )

            result = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                result.stdout.strip(),
                "phase=B action=execute_task task=task_01 lane=frontend agent=claude",
            )

    def test_detect_phase_omits_lane_when_frontend_agent_null(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "no-lane"
            slug_dir = tasks_root / slug
            self.write_state(
                tasks_root,
                slug,
                mode="tasks",
                tasks={
                    "total": 1,
                    "completed": [],
                    "current": None,
                    "pending": ["task_01"],
                },
                qa={"report_done": False, "execution_done": False},
            )
            (slug_dir / "task_01.md").write_text(
                "---\nstatus: pending\ntitle: web view\ntype: frontend\n---\n",
                encoding="utf-8",
            )

            result = self.run_script(
                "detect-phase.py",
                slug,
                "--tasks-root",
                str(tasks_root),
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                result.stdout.strip(),
                "phase=B action=execute_task task=task_01",
            )

    # ---- commit-checkpoint.py --------------------------------------------

    def test_commit_checkpoint_gh_runner_is_bounded_and_reports_tool_failures(self) -> None:
        completed = subprocess.CompletedProcess(["gh", "stack", "view"], 0, "{}", "")
        with mock.patch.object(commit_checkpoint.subprocess, "run", return_value=completed) as run:
            self.assertIs(commit_checkpoint._run_gh(["stack", "view", "--json"]), completed)
        self.assertEqual(run.call_args.kwargs["timeout"], 60)

        with mock.patch.object(commit_checkpoint.subprocess, "run", side_effect=FileNotFoundError):
            stderr = io.StringIO()
            with mock.patch.object(commit_checkpoint.sys, "stderr", stderr):
                with self.assertRaises(SystemExit) as missing:
                    commit_checkpoint._run_gh(["stack", "view", "--json"])
        self.assertEqual(missing.exception.code, 1)
        self.assertIn("not found", stderr.getvalue())

        with mock.patch.object(
            commit_checkpoint.subprocess,
            "run",
            side_effect=subprocess.TimeoutExpired(["gh", "stack", "view", "--json"], 60),
        ):
            stderr = io.StringIO()
            with mock.patch.object(commit_checkpoint.sys, "stderr", stderr):
                with self.assertRaises(SystemExit) as timed_out:
                    commit_checkpoint._run_gh(["stack", "view", "--json"])
        self.assertEqual(timed_out.exception.code, 1)
        self.assertIn("timed out after 60s", stderr.getvalue())

    def _init_git_repo(self, root: Path) -> None:
        env = _git_env()
        init = _git(root, "init", "-q", "-b", "main", env=env)
        self.assertEqual(init.returncode, 0, init.stderr)
        # one anchoring commit so HEAD exists and rev-parse works
        (root / ".gitkeep").write_text("", encoding="utf-8")
        add = _git(root, "add", ".gitkeep", env=env)
        self.assertEqual(add.returncode, 0, add.stderr)
        first = _git(root, "commit", "-q", "-m", "anchor", env=env)
        self.assertEqual(first.returncode, 0, first.stderr)

    def _setup_checkpoint_repo(self, root: Path, slug: str, **state_overrides: object) -> Path:
        self._init_git_repo(root)
        tasks_root = root / ".compozy" / "tasks"
        self.write_state(tasks_root, slug, mode="tasks", iteration=4, **state_overrides)
        # Track the state.yaml so subsequent checkpoint runs see a clean tree
        env = _git_env()
        _git(root, "add", "-A", env=env)
        _git(root, "commit", "-q", "-m", "state", env=env)
        return tasks_root

    def _run_checkpoint(
        self,
        root: Path,
        tasks_root: Path,
        slug: str,
        *flags: str,
        extra_env: dict[str, str] | None = None,
    ) -> subprocess.CompletedProcess[str]:
        env = _git_env()
        if extra_env:
            env.update(extra_env)
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPTS / "commit-checkpoint.py"),
                slug,
                *flags,
                "--tasks-root",
                str(tasks_root.relative_to(root)),
            ],
            cwd=root,
            env=env,
            check=False,
            text=True,
            capture_output=True,
        )

    def _gh_stub(self, tmp: Path) -> tuple[Path, dict[str, str]]:
        """Fake ``gh`` binary logging calls and mimicking stack init/add/view."""
        stub_dir = tmp / "gh-stub"
        stub_dir.mkdir(parents=True, exist_ok=True)
        stub = stub_dir / "gh"
        stub.write_text(
            "#!/bin/sh\n"
            'echo "$*" >> "$GH_STUB_LOG"\n'
            'case "$1 $2" in\n'
            '  "stack view")\n'
            '    [ -f "$GH_STUB_STATE/in-stack" ] || exit 1\n'
            "    ;;\n"
            '  "stack init")\n'
            '    git switch -q -c "$3" || exit 1\n'
            '    touch "$GH_STUB_STATE/in-stack"\n'
            "    ;;\n"
            '  "stack add")\n'
            '    git switch -q -c "$3" || exit 1\n'
            "    ;;\n"
            '  "stack submit")\n'
            "    ;;\n"
            "esac\n"
            "exit 0\n",
            encoding="utf-8",
        )
        stub.chmod(0o755)
        env = {
            "CY_LOOP_GH": str(stub),
            "GH_STUB_LOG": str(stub_dir / "calls.log"),
            "GH_STUB_STATE": str(stub_dir),
        }
        return stub_dir, env

    def _stub_calls(self, stub_dir: Path) -> str:
        log = stub_dir / "calls.log"
        return log.read_text(encoding="utf-8") if log.exists() else ""

    def test_commit_checkpoint_skips_when_no_changes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "noop"
            tasks_root = self._setup_checkpoint_repo(root, slug)

            result = self._run_checkpoint(root, tasks_root, slug, "--task", "task_07")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), "SKIP: no changes")

    def test_commit_checkpoint_tasks_mode_builds_message_from_frontmatter(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "with-task"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            slug_dir = tasks_root / slug
            (slug_dir / "task_07.md").write_text(
                "---\nstatus: pending\ntitle: implement backend tests\n---\n\nbody\n",
                encoding="utf-8",
            )
            (root / "feature.txt").write_text("change", encoding="utf-8")

            result = self._run_checkpoint(
                root,
                tasks_root,
                slug,
                "--task",
                "task_07",
                "--body-note",
                "Imported from source SHA abc123 on 2026-07-26.",
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            sha = result.stdout.strip()
            self.assertEqual(len(sha), 40)

            log = _git(root, "log", "-1", "--pretty=%B", env=_git_env())
            self.assertEqual(log.returncode, 0, log.stderr)
            self.assertIn("feat: implement backend tests #07", log.stdout)
            self.assertIn("Checkpoint via cy-loop-tasks (iteration 4, phase B mode=tasks).", log.stdout)
            self.assertIn(
                "Imported from source SHA abc123 on 2026-07-26.",
                log.stdout,
            )
            self.assertIn(
                "Co-Authored-By: Codex <noreply@openai.com>",
                log.stdout,
            )

    def test_commit_checkpoint_free_mode_truncates_long_slice(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "long-slice"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            (root / "change.txt").write_text("x", encoding="utf-8")

            long_slice = "implement " + ("very " * 60) + "long slice"

            result = self._run_checkpoint(root, tasks_root, slug, "--slice", long_slice)
            self.assertEqual(result.returncode, 0, result.stderr)

            header = _git(root, "log", "-1", "--pretty=%s", env=_git_env())
            self.assertEqual(header.returncode, 0, header.stderr)
            subject = header.stdout.strip()
            self.assertLessEqual(len(subject), 72)
            self.assertTrue(subject.startswith("feat: implement "))

    def test_commit_checkpoint_review_round_builds_fix_header(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "review-round"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            (root / "remediation.txt").write_text("fix", encoding="utf-8")

            result = self._run_checkpoint(root, tasks_root, slug, "--review-round", "2")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(len(result.stdout.strip()), 40)

            log = _git(root, "log", "-1", "--pretty=%B", env=_git_env())
            self.assertEqual(log.returncode, 0, log.stderr)
            self.assertIn("fix: peer-review round 2 remediation", log.stdout)
            self.assertIn("Checkpoint via cy-loop-tasks (iteration 4, phase D review round 2).", log.stdout)

    def test_commit_checkpoint_rejects_flag_misuse(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "no-flags"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            (root / "change.txt").write_text("x", encoding="utf-8")

            none_given = self._run_checkpoint(root, tasks_root, slug)
            self.assertEqual(none_given.returncode, 2)
            self.assertIn("exactly one", none_given.stderr)

            two_given = self._run_checkpoint(
                root, tasks_root, slug, "--task", "task_01", "--review-round", "1"
            )
            self.assertEqual(two_given.returncode, 2)
            self.assertIn("exactly one", two_given.stderr)

            bad_round = self._run_checkpoint(root, tasks_root, slug, "--review-round", "0")
            self.assertEqual(bad_round.returncode, 2)
            self.assertIn(">= 1", bad_round.stderr)

    def test_commit_checkpoint_includes_state_iteration_in_body(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "iteration"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            slug_dir = tasks_root / slug
            (slug_dir / "task_03.md").write_text(
                "---\nstatus: pending\ntitle: bump iteration\n---\n",
                encoding="utf-8",
            )

            # Bump iteration via update-state so state.yaml stays canonical
            bump = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "advance",
            )
            self.assertEqual(bump.returncode, 0, bump.stderr)
            # Commit the state mutation as anchor so the checkpoint diff is the new file
            env = _git_env()
            _git(root, "add", "-A", env=env)
            _git(root, "commit", "-q", "-m", "anchor-state", env=env)

            (root / "delta.txt").write_text("y", encoding="utf-8")

            result = self._run_checkpoint(root, tasks_root, slug, "--task", "task_03")
            self.assertEqual(result.returncode, 0, result.stderr)

            state = state_io.load(tasks_root / slug / "state.yaml")
            current_iter = int(state["iteration"])

            log = _git(root, "log", "-1", "--pretty=%B", env=_git_env())
            self.assertIn(f"iteration {current_iter}", log.stdout)
            self.assertIn("phase B mode=tasks", log.stdout)

    def test_commit_checkpoint_rejects_state_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._init_git_repo(root)
            (root / "change.txt").write_text("x", encoding="utf-8")

            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPTS / "commit-checkpoint.py"),
                    "missing-slug",
                    "--task",
                    "task_01",
                ],
                cwd=root,
                env=_git_env(),
                check=False,
                text=True,
                capture_output=True,
            )
            self.assertEqual(result.returncode, 2)
            self.assertIn("state.yaml", result.stderr)

    # ---- commit-checkpoint.py stacked mode -------------------------------

    def test_commit_checkpoint_stacked_inits_layer_commits_and_submits(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "repo"
            root.mkdir()
            slug = "stacked-first"
            tasks_root = self._setup_checkpoint_repo(root, slug, stacked=True)
            stub_dir, gh_env = self._gh_stub(Path(tmp))
            (tasks_root / slug / "task_07.md").write_text(
                "---\nstatus: pending\ntitle: implement backend tests\n---\n",
                encoding="utf-8",
            )
            (root / "feature.txt").write_text("change", encoding="utf-8")
            env = _git_env()
            _git(root, "add", "-A", env=env)
            _git(root, "commit", "-q", "-m", "task file", env=env)
            (root / "delta.txt").write_text("work", encoding="utf-8")

            result = self._run_checkpoint(
                root, tasks_root, slug, "--task", "task_07", extra_env=gh_env
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            lines = result.stdout.strip().splitlines()
            self.assertEqual(len(lines[0]), 40)
            self.assertEqual(lines[1], f"stack: submitted ({slug}/task-07)")

            branch = _git(root, "rev-parse", "--abbrev-ref", "HEAD", env=env)
            self.assertEqual(branch.stdout.strip(), f"{slug}/task-07")

            calls = self._stub_calls(stub_dir)
            self.assertIn("stack view --json", calls)
            self.assertIn(f"stack init {slug}/task-07 --base main", calls)
            self.assertIn("stack submit --auto", calls)
            self.assertNotIn("stack add", calls)

            log = _git(root, "log", "-1", "--pretty=%B", env=env)
            self.assertIn("feat: implement backend tests #07", log.stdout)

    def test_commit_checkpoint_stacked_adds_layer_when_stack_active(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "repo"
            root.mkdir()
            slug = "stacked-next"
            tasks_root = self._setup_checkpoint_repo(root, slug, stacked=True)
            stub_dir, gh_env = self._gh_stub(Path(tmp))
            (stub_dir / "in-stack").touch()
            (tasks_root / slug / "task_02.md").write_text(
                "---\nstatus: pending\ntitle: second layer\n---\n",
                encoding="utf-8",
            )
            env = _git_env()
            _git(root, "add", "-A", env=env)
            _git(root, "commit", "-q", "-m", "task file", env=env)
            (root / "delta.txt").write_text("work", encoding="utf-8")

            result = self._run_checkpoint(
                root, tasks_root, slug, "--task", "task_02", extra_env=gh_env
            )
            self.assertEqual(result.returncode, 0, result.stderr)

            calls = self._stub_calls(stub_dir)
            self.assertIn(f"stack add {slug}/task-02", calls)
            self.assertNotIn("stack init", calls)
            self.assertIn("stack submit --auto", calls)

            branch = _git(root, "rev-parse", "--abbrev-ref", "HEAD", env=env)
            self.assertEqual(branch.stdout.strip(), f"{slug}/task-02")

    def test_commit_checkpoint_stacked_clean_tree_still_resubmits(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "repo"
            root.mkdir()
            slug = "stacked-clean"
            tasks_root = self._setup_checkpoint_repo(root, slug, stacked=True)
            stub_dir, gh_env = self._gh_stub(Path(tmp))
            (stub_dir / "in-stack").touch()

            result = self._run_checkpoint(
                root, tasks_root, slug, "--task", "task_01", extra_env=gh_env
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            lines = result.stdout.strip().splitlines()
            self.assertEqual(lines[0], "SKIP: no changes")
            self.assertTrue(lines[1].startswith("stack: submitted"))
            self.assertIn("stack submit --auto", self._stub_calls(stub_dir))

    def test_commit_checkpoint_stacked_rejects_slice(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "repo"
            root.mkdir()
            slug = "stacked-slice"
            tasks_root = self._setup_checkpoint_repo(root, slug, stacked=True)
            _, gh_env = self._gh_stub(Path(tmp))
            (root / "delta.txt").write_text("work", encoding="utf-8")

            result = self._run_checkpoint(
                root, tasks_root, slug, "--slice", "some slice", extra_env=gh_env
            )
            self.assertEqual(result.returncode, 2)
            self.assertIn("incompatible with stacked mode", result.stderr)

    def test_commit_checkpoint_stacked_guards_resume_drift(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "repo"
            root.mkdir()
            slug = "stacked-drift"
            tasks_root = self._setup_checkpoint_repo(root, slug, stacked=True)
            stub_dir, gh_env = self._gh_stub(Path(tmp))
            (tasks_root / slug / "task_02.md").write_text(
                "---\nstatus: pending\ntitle: second layer\n---\n",
                encoding="utf-8",
            )
            env = _git_env()
            _git(root, "add", "-A", env=env)
            _git(root, "commit", "-q", "-m", "task file", env=env)
            # A prior layer branch exists but the stub reports no active stack.
            _git(root, "branch", f"{slug}/task-01", env=env)
            (root / "delta.txt").write_text("work", encoding="utf-8")

            result = self._run_checkpoint(
                root, tasks_root, slug, "--task", "task_02", extra_env=gh_env
            )
            self.assertEqual(result.returncode, 2)
            self.assertIn("resume the stack first", result.stderr)
            self.assertIn(f"gh stack checkout {slug}/task-01", result.stderr)
            self.assertNotIn("stack init", self._stub_calls(stub_dir))

    def test_commit_checkpoint_stacked_review_round_requires_stack(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "repo"
            root.mkdir()
            slug = "stacked-review"
            tasks_root = self._setup_checkpoint_repo(root, slug, stacked=True)
            stub_dir, gh_env = self._gh_stub(Path(tmp))
            (root / "remediation.txt").write_text("fix", encoding="utf-8")

            outside = self._run_checkpoint(
                root, tasks_root, slug, "--review-round", "1", extra_env=gh_env
            )
            self.assertEqual(outside.returncode, 2)
            self.assertIn("belong to a stack", outside.stderr)
            self.assertIn("gh stack checkout <branch-or-pr>", outside.stderr)

            (stub_dir / "in-stack").touch()
            inside = self._run_checkpoint(
                root, tasks_root, slug, "--review-round", "1", extra_env=gh_env
            )
            self.assertEqual(inside.returncode, 0, inside.stderr)
            lines = inside.stdout.strip().splitlines()
            self.assertEqual(len(lines[0]), 40)
            self.assertTrue(lines[1].startswith("stack: submitted"))
            self.assertIn("stack submit --auto", self._stub_calls(stub_dir))


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Self-test for the cy-implement-spec helper scripts (read-only, tmpdir-scoped).

Run from anywhere:
    python3 .agents/skills/cy-implement-spec/scripts/test_scripts.py
"""
from __future__ import annotations

import importlib.util
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import ModuleType


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


def _criteria(*pairs: tuple[str, str]) -> list[dict]:
    return [
        {"text": text, "status": status, "iteration": 0}
        for text, status in pairs
    ]


class CyImplementSpecScriptTests(unittest.TestCase):
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
            "created_at": "2026-07-14T00:00:00Z",
            "last_updated": "2026-07-14T00:00:00Z",
            "iteration": 0,
            "goal_signature": "test goal",
            "progress": {
                "implementation_complete": True,
                "criteria": _criteria(("ship the feature", "met")),
            },
            "qa": {"report_done": True, "execution_done": True},
            "review": {"rounds": 1, "last_verdict": "SHIP", "ship": True},
            "verify": {"last_run": "2026-07-14T00:00:00Z", "last_status": "PASS"},
            "iterations": [],
        }
        state.update(overrides)
        state_path = slug_dir / "state.yaml"
        state_io.dump(state, state_path)
        return state_path

    # ---- skill behavior contract ----------------------------------------

    def test_skill_contract_repairs_failures_inside_current_action(self) -> None:
        skill = (SKILL_ROOT / "SKILL.md").read_text(encoding="utf-8")
        recovery = (SKILL_ROOT / "references" / "recovery-loop.md").read_text(
            encoding="utf-8"
        )

        self.assertIn("**self-healing continue** loop", skill)
        self.assertIn("Do not write final iteration state", skill)
        self.assertIn("A failure is **repairable by default**", recovery)
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
        self.assertNotIn("a second failure is a blocker", skill)

    def test_skill_contract_never_authors_a_task_graph(self) -> None:
        skill = (SKILL_ROOT / "SKILL.md").read_text(encoding="utf-8")
        self.assertIn("exit conditions, not work items", skill)
        self.assertNotIn("cy-create-tasks the loop", skill)

    # ---- init-state.py ---------------------------------------------------

    def _make_slug(self, tasks_root: Path, slug: str) -> Path:
        slug_dir = tasks_root / slug
        slug_dir.mkdir(parents=True)
        (slug_dir / "_techspec.md").write_text("# spec\n", encoding="utf-8")
        return slug_dir

    def test_init_state_requires_at_least_one_criterion(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            self._make_slug(tasks_root, "no-criteria")

            result = self.run_script(
                "init-state.py",
                "no-criteria",
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
            )

            self.assertEqual(result.returncode, 4)
            self.assertIn("at least one --criterion", result.stderr)
            self.assertFalse((tasks_root / "no-criteria" / "state.yaml").exists())

    def test_init_state_writes_pending_criteria_and_defaults(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug_dir = self._make_slug(tasks_root, "fresh")

            result = self.run_script(
                "init-state.py",
                "fresh",
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "ship it",
                "--criterion",
                "criterion A",
                "--criterion",
                "criterion B",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            state = state_io.load(slug_dir / "state.yaml")
            self.assertEqual(state["goal_signature"], "ship it")
            self.assertFalse(state["progress"]["implementation_complete"])
            self.assertEqual(
                [c["text"] for c in state["progress"]["criteria"]],
                ["criterion A", "criterion B"],
            )
            self.assertTrue(
                all(c["status"] == "pending" for c in state["progress"]["criteria"])
            )
            self.assertEqual(state["review"]["rounds"], 0)
            self.assertIsNone(state["review"]["last_verdict"])
            self.assertFalse(state["review"]["ship"])
            self.assertFalse(state["qa"]["report_done"])

    def test_init_state_rejects_duplicate_criteria(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            self._make_slug(tasks_root, "dup")

            result = self.run_script(
                "init-state.py",
                "dup",
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
                "--criterion",
                "same text",
                "--criterion",
                "same text",
            )

            self.assertEqual(result.returncode, 1)
            self.assertIn("duplicate", result.stderr)

    def test_init_state_refuses_slug_with_task_graph(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug_dir = self._make_slug(tasks_root, "graphed")
            (slug_dir / "_tasks.md").write_text("# tasks\n", encoding="utf-8")

            result = self.run_script(
                "init-state.py",
                "graphed",
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
                "--criterion",
                "criterion A",
            )

            self.assertEqual(result.returncode, 5)
            self.assertIn("cy-loop-tasks", result.stderr)

    def test_init_state_refuses_overwrite_and_missing_techspec(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug_dir = self._make_slug(tasks_root, "twice")
            common = [
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
                "--criterion",
                "criterion A",
            ]

            first = self.run_script("init-state.py", "twice", *common)
            self.assertEqual(first.returncode, 0, first.stderr)
            second = self.run_script("init-state.py", "twice", *common)
            self.assertEqual(second.returncode, 2)
            self.assertIn("refusing to overwrite", second.stderr)

            bare = tasks_root / "bare"
            bare.mkdir()
            no_spec = self.run_script(
                "init-state.py",
                "bare",
                "--tasks-root",
                str(tasks_root),
                "--goal",
                "test goal",
                "--criterion",
                "criterion A",
            )
            self.assertEqual(no_spec.returncode, 3)
            self.assertIn("_techspec.md missing", no_spec.stderr)
            self.assertIs(slug_dir.exists(), True)

    def test_init_state_preserves_multiline_goal_through_update(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug_dir = self._make_slug(tasks_root, "multiline-goal")
            goal = "ship the workflow\n- straight from the spec"

            initialized = self.run_script(
                "init-state.py",
                "multiline-goal",
                "--tasks-root",
                str(tasks_root),
                "--goal",
                goal,
                "--criterion",
                "criterion A",
            )

            self.assertEqual(initialized.returncode, 0, initialized.stderr)
            updated = self.run_script(
                "update-state.py",
                "multiline-goal",
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

    # ---- update-state.py -------------------------------------------------

    def test_criterion_met_requires_exact_existing_text(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "criteria"
            state_path = self.write_state(
                tasks_root,
                slug,
                progress={
                    "implementation_complete": False,
                    "criteria": _criteria(("criterion A", "pending")),
                },
            )

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--criterion-met",
                " criterion A ",
            )

            self.assertEqual(result.returncode, 1)
            self.assertIn("did not match", result.stderr)
            state = state_io.load(state_path)
            self.assertEqual(state["progress"]["criteria"][0]["status"], "pending")
            self.assertEqual(state["iteration"], 0)

            ok = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "milestone: criterion A proven",
                "--criterion-met",
                "criterion A",
            )

            self.assertEqual(ok.returncode, 0, ok.stderr)
            updated = state_io.load(state_path)
            self.assertEqual(updated["progress"]["criteria"][0]["status"], "met")
            self.assertEqual(updated["iterations"][0]["phase"], "B")

            again = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--criterion-met",
                "criterion A",
            )
            self.assertEqual(again.returncode, 1)
            self.assertIn("already met", again.stderr)

    def test_implementation_complete_is_guarded(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "gate"
            state_path = self.write_state(
                tasks_root,
                slug,
                progress={
                    "implementation_complete": False,
                    "criteria": _criteria(
                        ("criterion A", "met"), ("criterion B", "pending")
                    ),
                },
            )

            pending_left = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--implementation-complete",
                "--verify-pass",
            )
            self.assertEqual(pending_left.returncode, 1)
            self.assertIn("criteria still pending", pending_left.stderr)
            self.assertIn("criterion B", pending_left.stderr)

            no_verify = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--criterion-met",
                "criterion B",
                "--implementation-complete",
            )
            self.assertEqual(no_verify.returncode, 1)
            self.assertIn("requires --verify-pass", no_verify.stderr)
            state = state_io.load(state_path)
            self.assertEqual(state["progress"]["criteria"][1]["status"], "pending")

            ok = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "milestone: final",
                "--criterion-met",
                "criterion B",
                "--implementation-complete",
                "--verify-pass",
            )
            self.assertEqual(ok.returncode, 0, ok.stderr)
            updated = state_io.load(state_path)
            self.assertTrue(updated["progress"]["implementation_complete"])
            self.assertEqual(updated["verify"]["last_status"], "PASS")

    def test_add_criterion_appends_pending_entry(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "amend"
            state_path = self.write_state(
                tasks_root,
                slug,
                progress={
                    "implementation_complete": False,
                    "criteria": _criteria(("criterion A", "pending")),
                },
            )

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--phase",
                "B",
                "--action",
                "spec amendment",
                "--add-criterion",
                "criterion B",
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            state = state_io.load(state_path)
            self.assertEqual(len(state["progress"]["criteria"]), 2)
            self.assertEqual(state["progress"]["criteria"][1]["text"], "criterion B")
            self.assertEqual(state["progress"]["criteria"][1]["status"], "pending")

            duplicate = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--add-criterion",
                "criterion B",
            )
            self.assertEqual(duplicate.returncode, 1)
            self.assertIn("duplicates", duplicate.stderr)

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
            self.assertTrue(state["review"]["ship"])

    def test_update_state_rejects_verify_pass_and_fail_together(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "verify-conflict"
            self.write_state(tasks_root, slug)

            result = self.run_script(
                "update-state.py",
                slug,
                "--tasks-root",
                str(tasks_root),
                "--verify-pass",
                "--verify-fail",
            )
            self.assertEqual(result.returncode, 1)
            self.assertIn("mutually exclusive", result.stderr)

    # ---- detect-phase.py -------------------------------------------------

    def test_detect_phase_emits_bootstrap_when_state_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            (tasks_root / "unborn").mkdir(parents=True)

            result = self.run_script(
                "detect-phase.py", "unborn", "--tasks-root", str(tasks_root)
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), "phase=0 action=bootstrap")

    def test_detect_phase_emits_implement_with_criteria_counters(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "implementing"
            self.write_state(
                tasks_root,
                slug,
                progress={
                    "implementation_complete": False,
                    "criteria": _criteria(
                        ("criterion A", "met"),
                        ("criterion B", "pending"),
                        ("criterion C", "pending"),
                    ),
                },
                qa={"report_done": False, "execution_done": False},
            )

            result = self.run_script(
                "detect-phase.py", slug, "--tasks-root", str(tasks_root)
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                result.stdout.strip(),
                "phase=B action=implement pending=2 met=1",
            )

    def test_detect_phase_errors_on_empty_criteria(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "hollow"
            self.write_state(
                tasks_root,
                slug,
                progress={"implementation_complete": False, "criteria": []},
            )

            result = self.run_script(
                "detect-phase.py", slug, "--tasks-root", str(tasks_root)
            )
            self.assertEqual(result.returncode, 1)
            self.assertIn("empty progress.criteria", result.stderr)

    def test_detect_phase_walks_qa_then_review_then_done(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"

            self.write_state(
                tasks_root,
                "qa-first",
                qa={"report_done": False, "execution_done": False},
                review={"rounds": 0, "last_verdict": None, "ship": False},
            )
            report = self.run_script(
                "detect-phase.py", "qa-first", "--tasks-root", str(tasks_root)
            )
            self.assertEqual(report.stdout.strip(), "phase=C action=qa_report")

            self.write_state(
                tasks_root,
                "qa-second",
                qa={"report_done": True, "execution_done": False},
                review={"rounds": 0, "last_verdict": None, "ship": False},
            )
            execution = self.run_script(
                "detect-phase.py", "qa-second", "--tasks-root", str(tasks_root)
            )
            self.assertEqual(execution.stdout.strip(), "phase=C action=qa_execution")

            self.write_state(
                tasks_root,
                "needs-review",
                review={"rounds": 0, "last_verdict": None, "ship": False},
            )
            review = self.run_script(
                "detect-phase.py", "needs-review", "--tasks-root", str(tasks_root)
            )
            self.assertEqual(
                review.stdout.strip(), "phase=D action=peer_review round=1"
            )

            self.write_state(tasks_root, "done-ready")
            done = self.run_script(
                "detect-phase.py", "done-ready", "--tasks-root", str(tasks_root)
            )
            self.assertEqual(done.stdout.strip(), "phase=E action=done")

    def test_detect_phase_reenters_peer_review_when_ship_but_verify_fail(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tasks_root = Path(tmp) / "tasks"
            slug = "ship-but-broken"
            self.write_state(
                tasks_root,
                slug,
                verify={"last_run": "2026-07-14T00:00:00Z", "last_status": "FAIL"},
            )

            result = self.run_script(
                "detect-phase.py", slug, "--tasks-root", str(tasks_root)
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(
                result.stdout.strip(),
                "phase=D action=peer_review round=2",
            )

    # ---- commit-checkpoint.py --------------------------------------------

    def _init_git_repo(self, root: Path) -> None:
        env = _git_env()
        init = _git(root, "init", "-q", "-b", "main", env=env)
        self.assertEqual(init.returncode, 0, init.stderr)
        (root / ".gitkeep").write_text("", encoding="utf-8")
        add = _git(root, "add", ".gitkeep", env=env)
        self.assertEqual(add.returncode, 0, add.stderr)
        first = _git(root, "commit", "-q", "-m", "anchor", env=env)
        self.assertEqual(first.returncode, 0, first.stderr)

    def _setup_checkpoint_repo(self, root: Path, slug: str) -> Path:
        self._init_git_repo(root)
        tasks_root = root / ".compozy" / "tasks"
        self.write_state(tasks_root, slug, iteration=4)
        env = _git_env()
        _git(root, "add", "-A", env=env)
        _git(root, "commit", "-q", "-m", "state", env=env)
        return tasks_root

    def _run_checkpoint(self, root: Path, tasks_root: Path, slug: str, *flags: str) -> subprocess.CompletedProcess[str]:
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
            env=_git_env(),
            check=False,
            text=True,
            capture_output=True,
        )

    def test_commit_checkpoint_skips_when_no_changes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "noop"
            tasks_root = self._setup_checkpoint_repo(root, slug)

            result = self._run_checkpoint(
                root, tasks_root, slug, "--milestone", "anything"
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), "SKIP: no changes")

    def test_commit_checkpoint_milestone_builds_feat_header(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "milestone"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            (root / "feature.txt").write_text("change", encoding="utf-8")

            result = self._run_checkpoint(
                root, tasks_root, slug, "--milestone", "marketplace  trust\npipeline"
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(len(result.stdout.strip()), 40)

            log = _git(root, "log", "-1", "--pretty=%B", env=_git_env())
            self.assertEqual(log.returncode, 0, log.stderr)
            self.assertIn("feat: marketplace trust pipeline", log.stdout)
            self.assertIn(
                "Checkpoint via cy-implement-spec (iteration 4, phase B milestone).",
                log.stdout,
            )
            self.assertNotIn("Co-Authored-By", log.stdout)

    def test_commit_checkpoint_coauthor_flag_adds_trailer(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "coauthored"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            (root / "feature.txt").write_text("change", encoding="utf-8")

            result = self._run_checkpoint(
                root,
                tasks_root,
                slug,
                "--milestone",
                "wire the trust policy",
                "--coauthor",
                "Codex <noreply@openai.com>",
            )
            self.assertEqual(result.returncode, 0, result.stderr)

            log = _git(root, "log", "-1", "--pretty=%B", env=_git_env())
            self.assertIn(
                "Co-Authored-By: Codex <noreply@openai.com>", log.stdout
            )

    def test_commit_checkpoint_truncates_long_milestone(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            slug = "long-milestone"
            tasks_root = self._setup_checkpoint_repo(root, slug)
            (root / "change.txt").write_text("x", encoding="utf-8")

            long_text = "implement " + ("very " * 60) + "long milestone"

            result = self._run_checkpoint(
                root, tasks_root, slug, "--milestone", long_text
            )
            self.assertEqual(result.returncode, 0, result.stderr)

            header = _git(root, "log", "-1", "--pretty=%s", env=_git_env())
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

            log = _git(root, "log", "-1", "--pretty=%B", env=_git_env())
            self.assertIn("fix: peer-review round 2 remediation", log.stdout)
            self.assertIn(
                "Checkpoint via cy-implement-spec (iteration 4, phase D review round 2).",
                log.stdout,
            )

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
                root,
                tasks_root,
                slug,
                "--milestone",
                "text",
                "--review-round",
                "1",
            )
            self.assertEqual(two_given.returncode, 2)
            self.assertIn("exactly one", two_given.stderr)

            bad_round = self._run_checkpoint(
                root, tasks_root, slug, "--review-round", "0"
            )
            self.assertEqual(bad_round.returncode, 2)
            self.assertIn(">= 1", bad_round.stderr)

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
                    "--milestone",
                    "anything",
                ],
                cwd=root,
                env=_git_env(),
                check=False,
                text=True,
                capture_output=True,
            )
            self.assertEqual(result.returncode, 2)
            self.assertIn("state.yaml", result.stderr)


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Smoke tests for bootstrap-qa-env helpers that are not covered by repo gates.

Suite: QA bootstrap helper integration.
Invariant: bootstrap resolves bundled helpers; allocated isolation paths allow
targeted teardown to stop owned processes while preserving outside processes.
Boundary IN: bootstrap helper path resolution and real workspace initialization script.
Boundary OUT: daemon startup and behavioral QA, owned by downstream QA execution.
"""

from __future__ import annotations

import importlib.util
import json
import os
import shlex
import subprocess
import sys
import tempfile
import threading
import types
from pathlib import Path


def load_bootstrap_module():
    if importlib.util.find_spec("tomllib") is None:
        tomllib_stub = types.ModuleType("tomllib")

        def unavailable_tomllib(*args, **kwargs):
            raise RuntimeError("tomllib is not available in this smoke-test interpreter")

        tomllib_stub.load = unavailable_tomllib
        tomllib_stub.loads = unavailable_tomllib
        sys.modules["tomllib"] = tomllib_stub

    script_path = Path(__file__).with_name("bootstrap-qa-env.py")
    sys.path.insert(0, str(script_path.parent))
    spec = importlib.util.spec_from_file_location("bootstrap_qa_env", script_path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"failed to load module spec for {script_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load_script_module(path: Path, name: str):
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"failed to load module spec for {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def write_discovery_script(repo_root: Path, payload: dict) -> None:
    script_path = repo_root / ".agents" / "skills" / "qa-execution" / "scripts" / "discover-project-contract.py"
    script_path.parent.mkdir(parents=True)
    script_path.write_text(
        "#!/usr/bin/env python3\n"
        "import json\n"
        f"print(json.dumps({json.dumps(payload)}))\n",
        encoding="utf-8",
    )


def check_allocated_process_teardown(repo_root: Path) -> None:
    """Exercise allocation and real teardown through a symlinked temp root."""
    eng_root = repo_root / ".agents" / "skills" / "eng"
    allocator = eng_root / "eng-worktree-isolation" / "scripts" / "allocate-isolation.py"
    teardown = eng_root / "eng-qa-bootstrap" / "scripts" / "teardown-qa-env.py"
    with tempfile.TemporaryDirectory(prefix="compozy-isolation-check-") as raw_dir:
        base = Path(raw_dir).resolve()
        real_tmp = base / "real-tmp"
        real_tmp.mkdir()
        alias_tmp = base / "alias-tmp"
        alias_tmp.symlink_to(real_tmp, target_is_directory=True)
        allocation = subprocess.run(
            [sys.executable, str(allocator), "--slug", "teardown-check"],
            env={**os.environ, "TMPDIR": str(alias_tmp)},
            capture_output=True, text=True, check=True, timeout=10,
        )
        envelope = dict(
            shlex.split(line)[1].split("=", 1)
            for line in allocation.stdout.splitlines()
        )
        root = Path(envelope["COMPOZY_ISOLATION_ROOT"])
        program = "import time\nprint('ready', flush=True)\ntime.sleep(30)\n"
        processes = []
        reapers = []
        try:
            for script in (root / "owned.py", base / "outside.py"):
                script.write_text(program, encoding="utf-8")
                process = subprocess.Popen(
                    [sys.executable, str(script)], stdout=subprocess.PIPE,
                    text=True, start_new_session=True,
                )
                processes.append(process)
                if process.stdout.readline().strip() != "ready":
                    raise AssertionError("process fixture did not become ready")
                reaper = threading.Thread(target=process.wait, daemon=True)
                reaper.start()
                reapers.append(reaper)
            result = subprocess.run(
                [sys.executable, str(teardown), "--root", str(root), "--json", "--grace-sec", "1"],
                capture_output=True, text=True, timeout=15,
            )
            if result.returncode != 0 or not json.loads(result.stdout)["clean"]:
                raise AssertionError(f"teardown failed: {result.stdout} {result.stderr}")
            reapers[0].join(timeout=1)
            if processes[0].poll() is None:
                raise AssertionError("teardown reported clean with an allocated process still alive")
            if processes[1].poll() is not None:
                raise AssertionError("targeted teardown stopped an outside process")
        finally:
            for process in processes:
                if process.poll() is None:
                    process.terminate()
                process.wait(timeout=5)
                if process.stdout is not None:
                    process.stdout.close()
            for reaper in reapers:
                reaper.join(timeout=5)


def main() -> None:
    module = load_bootstrap_module()
    repo_root = Path(__file__).resolve().parents[5]
    check_allocated_process_teardown(repo_root)
    loader = module.real_scenario_script(repo_root, "playbook_loader.py")
    if not loader.is_file():
        raise AssertionError(f"real-scenario sibling path does not resolve: {loader}")

    with tempfile.TemporaryDirectory() as raw_dir:
        repo_root = Path(raw_dir)
        if module.discover_project_contract(repo_root) != {}:
            raise AssertionError("missing discovery script should return an empty contract")

        payload = {"schema_version": 1, "surfaces": ["loops"]}
        write_discovery_script(repo_root, payload)
        got = module.discover_project_contract(repo_root)
        if got != payload:
            raise AssertionError(f"discover_project_contract() = {got!r}, want {payload!r}")

    repo_root = Path(__file__).resolve().parents[5]
    playbook = module.load_playbook_via_helper(repo_root, "devtool-oss-launch")
    if playbook.get("playbook_ref") != "devtool-oss-launch":
        raise AssertionError(f"loaded playbook_ref = {playbook.get('playbook_ref')!r}")

    with tempfile.TemporaryDirectory() as raw_dir:
        workspace_info = module.run_init_workspace(repo_root, "path-resolution-smoke", raw_dir)
        workspace_path = Path(workspace_info["WORKSPACE_PATH"])
        if workspace_path.parent != Path(raw_dir):
            raise AssertionError(
                f"run_init_workspace() created {workspace_path}, want a lab directly under {raw_dir}"
            )
        if not (workspace_path / "qa-artifacts" / "qa" / "screenshots").is_dir():
            raise AssertionError("run_init_workspace() did not execute the bundled workspace initializer")

    playbook = module.load_playbook_via_helper(repo_root, "northstar-pay")
    if playbook.get("playbook_ref") != "northstar-pay":
        raise AssertionError("load_playbook_via_helper() returned the wrong playbook")

    targeted_contract = module.build_scenario_contract(
        repo_root,
        "vault-extension-targeted",
        requested_profile="targeted",
        required_surfaces=["cli", "runtime", "cli"],
    )
    if targeted_contract.get("release_grade") != "targeted":
        raise AssertionError("targeted profile did not retain its explicit release grade")
    targeted_minimums = targeted_contract.get("minimums", {})
    if targeted_minimums.get("surfaces_required") != ["cli", "runtime"]:
        raise AssertionError("targeted profile did not retain unique required surfaces in order")
    for key in (
        "agents",
        "differentiated_roles",
        "channels",
        "provider_backed_sessions",
        "cross_surface_objects",
        "disruption_probes",
        "artifacts_used_later",
    ):
        if targeted_minimums.get(key) != 0:
            raise AssertionError(f"targeted profile {key} minimum must be zero")

    with tempfile.TemporaryDirectory() as raw_dir:
        workspace_path = Path(raw_dir)
        summary = module.seed_playbook_workspace(repo_root, workspace_path, "northstar-pay")
        if not Path(summary["playbook_snapshot"]).is_file():
            raise AssertionError("seed_playbook_workspace() did not materialize the playbook")
        runtime_workspace_path = Path(summary["runtime_workspace_path"])
        if runtime_workspace_path.resolve() != (workspace_path / "project").resolve():
            raise AssertionError(
                f"runtime workspace = {runtime_workspace_path}, want isolated project root"
            )
        if (runtime_workspace_path / "qa-artifacts").exists():
            raise AssertionError("runtime workspace exposes QA artifacts to agents under test")
        global_knowledge = Path("knowledge/global/launch-week-brief.md")
        for workspace_name in (
            "launch-hq",
            "product-studio",
            "growth-studio",
            "platform-control",
            "finance-command",
            "merchant-success",
            "risk-ops",
        ):
            projected = runtime_workspace_path / "workspaces" / workspace_name / global_knowledge
            if not projected.is_file():
                raise AssertionError(f"global knowledge was not projected into {workspace_name}")
        risk_memo = Path("knowledge/workspace/executive-risk-memo.md")
        if not (runtime_workspace_path / "workspaces" / "launch-hq" / risk_memo).is_file():
            raise AssertionError("launch-hq is missing its declared scoped knowledge")
        if (runtime_workspace_path / "workspaces" / "product-studio" / risk_memo).exists():
            raise AssertionError("scoped launch-hq knowledge leaked into product-studio")

        open_tasks = json.loads((workspace_path / ".compozy" / "tasks" / "open-tasks.json").read_text())
        runtime_ids = [task.get("runtime_id") for task in open_tasks]
        expected_ids = [f"task-northstar-pay-{index:03d}" for index in range(1, 13)]
        if runtime_ids != expected_ids:
            raise AssertionError(f"runtime task ids = {runtime_ids!r}, want {expected_ids!r}")

        qa_root = workspace_path / "qa-artifacts" / "qa"
        qa_root.mkdir(parents=True)
        evidence_paths = module.seed_qa_evidence_contracts(
            repo_root, qa_root, "path-resolution-smoke", playbook
        )
        if not evidence_paths["AUDIT_COMMAND"].is_file():
            raise AssertionError("seed_qa_evidence_contracts() emitted a missing audit command")

        manifest_path = qa_root / "bootstrap-manifest.json"
        manifest = {
            "env": {
                "COMPOZY_HOME": str(workspace_path / ".compozy" / "runtime"),
                "WORKSPACE_PATH": str(workspace_path),
                "RUNTIME_WORKSPACE_PATH": str(runtime_workspace_path),
                "KICKOFF_POSTED": "false",
                "KICKOFF_TIMESTAMP": "",
            },
            "status": {"notes": []},
        }
        manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
        real_scenario_scripts = (
            repo_root / ".agents" / "skills" / "eng" / "eng-real-scenario-qa" / "scripts"
        )
        auditor = load_script_module(
            real_scenario_scripts / "audit-qa-evidence.py",
            "audit_qa_evidence",
        )
        lab_path, audited_runtime_path = auditor.workspace_paths_from_manifest(manifest_path)
        if lab_path != workspace_path.resolve():
            raise AssertionError(f"auditor lab root = {lab_path}, want {workspace_path.resolve()}")
        if audited_runtime_path != runtime_workspace_path.resolve():
            raise AssertionError(
                f"auditor runtime root = {audited_runtime_path}, "
                f"want {runtime_workspace_path.resolve()}"
            )
        if auditor.load_playbook_snapshot(lab_path) is None:
            raise AssertionError("auditor did not load the playbook snapshot from the lab root")
        if auditor.load_playbook_snapshot(audited_runtime_path) is not None:
            raise AssertionError("runtime workspace unexpectedly exposes the playbook snapshot")
        with tempfile.TemporaryDirectory() as deliverable_dir:
            ts_test_dir = Path(deliverable_dir)
            (ts_test_dir / "module.test.ts").write_text(
                'test("module", () => {});\n',
                encoding="utf-8",
            )
            (ts_test_dir / "component.test.tsx").write_text(
                'test("component", () => {});\n',
                encoding="utf-8",
            )
            deliverable_findings, deliverable_summary = auditor.check_required_deliverables(
                ts_test_dir,
                {"required_deliverables": {"ts_test": 2}},
            )
            valid_ts_tests = deliverable_summary["deliverable_counts"]["ts_test"]["valid"]
            ts_test_findings = [
                finding
                for finding in deliverable_findings
                if finding.message.startswith("deliverable ts_test")
            ]
            if ts_test_findings or valid_ts_tests != 2:
                raise AssertionError(
                    "auditor must accept both .test.ts and .test.tsx as TypeScript tests"
                )
        activation = load_script_module(
            real_scenario_scripts / "activate-playbook-tasks.py",
            "activate_playbook_tasks",
        )
        commands: list[list[str]] = []
        recorded: list[dict] = []
        paused = False

        def fake_runner(_compozy_bin: str, args: list[str], _env: dict[str, str]) -> dict:
            nonlocal paused
            commands.append(args)
            if args[:2] == ["scheduler", "status"]:
                return {"scheduler": {"paused": paused}}
            if args[:2] == ["scheduler", "pause"]:
                paused = True
                return {"scheduler": {"paused": True}}
            if args[:2] == ["scheduler", "resume"]:
                paused = False
                return {"scheduler": {"paused": False}}
            if args[:2] == ["task", "start"]:
                task_id = args[2]
                return {"task": {"id": task_id}, "run": {"id": f"run-{task_id}"}}
            raise AssertionError(f"unexpected fake Compozy command: {args!r}")

        def fake_recorder(_helper: Path, _log: Path, row: dict) -> None:
            recorded.append(row)

        prepared = activation.prepare_activation(
            workspace_path,
            workspace_path / "qa-artifacts",
            manifest_path,
            "compozy-test",
            runner=fake_runner,
            recorder=fake_recorder,
        )
        if prepared.get("status") != "prepared" or len(prepared.get("tasks", [])) != 12:
            raise AssertionError("task activation did not prepare all 12 playbook runs")
        if commands[1][:2] != ["scheduler", "pause"]:
            raise AssertionError(f"scheduler was not paused before task starts: {commands!r}")
        if any(command[:2] == ["scheduler", "resume"] for command in commands):
            raise AssertionError("prepare released the scheduler before kickoff confirmation")
        if len([row for row in recorded if row.get("task_kind") == "run"]) != 12:
            raise AssertionError("prepared runs were not recorded as real task-run evidence")

        kickoff_evidence = qa_root / "operator-kickoff.jsonl"
        kickoff_evidence.write_text('{"type":"result"}\n', encoding="utf-8")
        post_kickoff = load_script_module(
            real_scenario_scripts / "post-operator-kickoff.py",
            "post_operator_kickoff",
        )
        post_kickoff.confirm_posted(
            manifest_path,
            qa_root / "journey-log.jsonl",
            kickoff_evidence,
            "Sofia Mendes",
            "northstar-pay",
        )
        try:
            post_kickoff.confirm_posted(
                manifest_path,
                qa_root / "journey-log.jsonl",
                kickoff_evidence,
                "Sofia Mendes",
                "northstar-pay",
            )
        except post_kickoff.PlaybookError:
            pass
        else:
            raise AssertionError("duplicate kickoff confirmation was not rejected")

        released = activation.release_activation(
            workspace_path,
            workspace_path / "qa-artifacts",
            manifest_path,
            kickoff_evidence,
            "compozy-test",
            runner=fake_runner,
            recorder=fake_recorder,
        )
        if released.get("status") != "released" or paused:
            raise AssertionError("confirmed kickoff did not release the scheduler barrier")

    with tempfile.TemporaryDirectory() as raw_dir:
        browser_bin = Path(raw_dir) / "browser-use"
        browser_bin.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
        browser_bin.chmod(0o755)
        previous_path = os.environ.get("PATH")
        try:
            os.environ["PATH"] = raw_dir
            mode, blocker = module.detect_browser_mode(Path(raw_dir) / "empty-codex-home")
            if (mode, blocker) != ("browser-use", ""):
                raise AssertionError(
                    f"detect_browser_mode(browser-use CLI) = {(mode, blocker)!r}, "
                    "want ('browser-use', '')"
                )

            browser_bin.unlink()
            mode, blocker = module.detect_browser_mode(Path(raw_dir) / "empty-codex-home")
            if mode != "blocked" or "Neither browser-use" not in blocker:
                raise AssertionError(
                    f"detect_browser_mode(no browser) = {(mode, blocker)!r}, want blocked"
                )
        finally:
            if previous_path is None:
                os.environ.pop("PATH", None)
            else:
                os.environ["PATH"] = previous_path

if __name__ == "__main__":
    main()

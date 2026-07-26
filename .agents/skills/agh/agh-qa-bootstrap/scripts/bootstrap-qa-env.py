#!/usr/bin/env python3
"""Build or reuse a deterministic AGH local QA lab and emit a canonical manifest."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import tomllib
from pathlib import Path
from urllib.parse import urlparse

from qa_skill_paths import real_scenario_script

ALLOWED_PROVIDER_FILES = ("auth.json", "installation_id", "version.json")
MAX_UNIX_SOCKET_PATH_BYTES = 103


def slugify(value: str) -> str:
    cleaned = "".join(ch.lower() if ch.isalnum() else "-" for ch in value.strip())
    while "--" in cleaned:
        cleaned = cleaned.replace("--", "-")
    cleaned = cleaned.strip("-")
    return cleaned or "release-candidate"


def shquote(value: str) -> str:
    if not value:
        return "''"
    safe_chars = set("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_/.:")
    if all(ch in safe_chars for ch in value):
        return value
    return "'" + value.replace("'", "'\\''") + "'"


def pick_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def socket_path_length(path: Path) -> int:
    return len(str(path).encode("utf-8"))


def socket_limited_paths(agh_home: Path, provider_home: Path) -> dict[str, Path]:
    return {
        "AGH_UDS_PATH": agh_home / "aghd.sock",
        "TMUX_BRIDGE_SOCKET": agh_home / "tmux-bridge.sock",
        "PROVIDER_DEFAULT_UDS": provider_home / ".agh" / "daemon.sock",
    }


def socket_limit_violations(paths: dict[str, Path]) -> list[str]:
    violations: list[str] = []
    for label, path in paths.items():
        length = socket_path_length(path)
        if length > MAX_UNIX_SOCKET_PATH_BYTES:
            violations.append(f"{label} {path} is {length} bytes; max {MAX_UNIX_SOCKET_PATH_BYTES}")
    return violations


def short_runtime_root(scenario_slug: str) -> Path:
    digest = hashlib.sha256(scenario_slug.encode("utf-8")).hexdigest()[:12]
    return Path(tempfile.gettempdir()) / f"aghqa-{digest}"


def run_init_workspace(repo_root: Path, scenario: str, workspace_root: str) -> dict[str, str]:
    init_script = real_scenario_script(repo_root, "init-scenario-workspace.sh")
    proc = subprocess.run(
        [str(init_script), scenario, workspace_root],
        cwd=repo_root,
        check=True,
        capture_output=True,
        text=True,
    )
    result: dict[str, str] = {}
    for line in proc.stdout.splitlines():
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        result[key.strip()] = value.strip()
    required = {"SCENARIO_SLUG", "WORKSPACE_PATH", "QA_OUTPUT_PATH"}
    missing = sorted(required - set(result))
    if missing:
        raise RuntimeError(f"bootstrap workspace helper omitted required keys: {', '.join(missing)}")
    return result


def ensure_lab_scaffold(workspace_path: Path, qa_output_path: Path) -> None:
    dirs = [
        workspace_path / "company" / "leadership",
        workspace_path / "company" / "planning",
        workspace_path / "product" / "specs",
        workspace_path / "product" / "releases",
        workspace_path / "marketing" / "campaigns",
        workspace_path / "finance" / "reports",
        workspace_path / "ops" / "runbooks",
        workspace_path / "ops" / "logs",
        workspace_path / "reviews" / "findings",
        workspace_path / "knowledge",
        workspace_path / "hooks",
        workspace_path / "extensions",
        workspace_path / "skills",
        workspace_path / ".providers",
        qa_output_path / "qa" / "logs",
        qa_output_path / "qa" / "notes",
        qa_output_path / "qa" / "screenshots",
        qa_output_path / "qa" / "issues",
        qa_output_path / "qa" / "test-cases",
        qa_output_path / "qa" / "test-plans",
        qa_output_path / "qa" / "pids",
    ]
    for path in dirs:
        path.mkdir(parents=True, exist_ok=True)


def load_json(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as handle:
        data = json.load(handle)
    if not isinstance(data, dict):
        raise RuntimeError(f"expected JSON object in {path}")
    return data


def detect_browser_mode(codex_home: Path) -> tuple[str, str]:
    del codex_home
    if shutil.which("browser-use"):
        return "browser-use", ""
    if shutil.which("agent-browser"):
        return "agent-browser", "browser-use CLI is not available"
    return "blocked", "Neither browser-use nor agent-browser CLI is available"


def safe_symlink(target: Path, link: Path) -> None:
    link.parent.mkdir(parents=True, exist_ok=True)
    if link.exists() or link.is_symlink():
        if link.is_symlink() and link.resolve() == target.resolve():
            return
        if link.is_dir() and not link.is_symlink():
            shutil.rmtree(link)
        else:
            link.unlink()
    link.symlink_to(target)


def prepare_provider_home(global_codex_home: Path, provider_home: Path) -> Path:
    provider_home.mkdir(parents=True, exist_ok=True)
    provider_codex_home = provider_home / ".codex"
    provider_codex_home.mkdir(parents=True, exist_ok=True)

    for filename in ALLOWED_PROVIDER_FILES:
        source = global_codex_home / filename
        if source.is_file():
            shutil.copy2(source, provider_codex_home / filename)

    safe_symlink(global_codex_home / "skills", provider_codex_home / "skills")
    safe_symlink(global_codex_home / "plugins" / "cache", provider_codex_home / "plugins" / "cache")

    config_path = provider_codex_home / "config.toml"
    if not config_path.exists():
        config_path.write_text(
            "# Isolated QA provider config.\n"
            "# Keep this minimal so provider launches do not inherit user-global hooks or loop state.\n",
            encoding="utf-8",
        )
    return provider_codex_home


def write_runtime_config(agh_home: Path, http_port: str, uds_path: str) -> None:
    config_path = agh_home / "config.toml"
    config_path.write_text(
        "[http]\n"
        'host = "127.0.0.1"\n'
        f"port = {http_port}\n"
        "\n"
        "[daemon]\n"
        f"socket = {json.dumps(uds_path)}\n",
        encoding="utf-8",
    )


def validate_runtime_config(config_path: Path, http_port: str, uds_path: str) -> list[str]:
    if not config_path.is_file():
        return [f"runtime config missing: {config_path}"]
    try:
        with config_path.open("rb") as handle:
            data = tomllib.load(handle)
    except tomllib.TOMLDecodeError as err:
        return [f"runtime config invalid TOML: {err}"]
    notes: list[str] = []
    got_port = str(data.get("http", {}).get("port", ""))
    if got_port != str(http_port):
        notes.append(f"runtime config http.port={got_port}, manifest AGH_HTTP_PORT={http_port}")
    got_socket = str(data.get("daemon", {}).get("socket", ""))
    if got_socket != str(uds_path):
        notes.append(f"runtime config daemon.socket={got_socket}, manifest AGH_UDS_PATH={uds_path}")
    return notes


def manifest_health(manifest: dict) -> tuple[bool, list[str]]:
    notes: list[str] = []
    env = manifest.get("env")
    if not isinstance(env, dict):
        return False, ["manifest env block missing"]

    required_paths = {
        "WORKSPACE_PATH": Path(str(env.get("WORKSPACE_PATH", ""))),
        "QA_OUTPUT_PATH": Path(str(env.get("QA_OUTPUT_PATH", ""))),
        "AGH_HOME": Path(str(env.get("AGH_HOME", ""))),
        "PROVIDER_HOME": Path(str(env.get("PROVIDER_HOME", ""))),
        "PROVIDER_CODEX_HOME": Path(str(env.get("PROVIDER_CODEX_HOME", ""))),
    }
    healthy = True
    for label, path in required_paths.items():
        if not str(path):
            healthy = False
            notes.append(f"{label} missing from manifest")
            continue
        if not path.exists():
            healthy = False
            notes.append(f"{label} path missing: {path}")

    proxy_target = str(env.get("AGH_WEB_API_PROXY_TARGET", ""))
    parsed = urlparse(proxy_target)
    if not proxy_target or parsed.scheme not in {"http", "https"} or not parsed.netloc:
        healthy = False
        notes.append("AGH_WEB_API_PROXY_TARGET is missing or not an absolute URL")

    socket_paths = {
        "AGH_UDS_PATH": Path(str(env.get("AGH_UDS_PATH", ""))),
        "TMUX_BRIDGE_SOCKET": Path(str(env.get("TMUX_BRIDGE_SOCKET", ""))),
        "PROVIDER_DEFAULT_UDS": required_paths["PROVIDER_HOME"] / ".agh" / "daemon.sock",
    }
    for note in socket_limit_violations(socket_paths):
        healthy = False
        notes.append(note)
    config_notes = validate_runtime_config(
        required_paths["AGH_HOME"] / "config.toml",
        str(env.get("AGH_HTTP_PORT", "")),
        str(env.get("AGH_UDS_PATH", "")),
    )
    if config_notes:
        healthy = False
        notes.extend(config_notes)
    return healthy, notes


def discover_project_contract(repo_root: Path) -> dict:
    script_path = repo_root / ".agents" / "skills" / "qa-execution" / "scripts" / "discover-project-contract.py"
    if not script_path.exists():
        return {}
    proc = subprocess.run(
        [sys.executable, str(script_path), "--root", str(repo_root)],
        cwd=repo_root,
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(proc.stdout)


def scenario_profile(scenario_slug: str, playbook: dict | None = None) -> str:
    if playbook is not None:
        return "broad"
    broad_markers = ("release", "broad", "full", "final", "launch", "scenario")
    if any(marker in scenario_slug for marker in broad_markers):
        return "broad"
    return "feature"


def build_contract_minimums(scenario_slug: str, playbook: dict | None = None) -> dict:
    if playbook is None:
        profile = scenario_profile(scenario_slug)
        if profile == "feature":
            return {
                "agents": 4,
                "differentiated_roles": 3,
                "channels": 3,
                "tasks": {
                    "roots": 1,
                    "subtasks": 0,
                    "dependencies": 0,
                    "runs": 1,
                },
                "provider_backed_sessions": 1,
                "cross_surface_objects": 1,
                "disruption_probes": 1,
                "artifacts_used_later": 1,
                "surfaces_required": ["cli", "api", "web", "runtime", "provider"],
            }
        return {
            "agents": 8,
            "differentiated_roles": 6,
            "channels": 5,
            "tasks": {
                "roots": 2,
                "subtasks": 4,
                "dependencies": 2,
                "runs": 6,
            },
            "provider_backed_sessions": 1,
            "cross_surface_objects": 3,
            "disruption_probes": 3,
            "artifacts_used_later": 2,
            "surfaces_required": ["cli", "api", "web", "runtime", "provider"],
        }

    agents = [agent for agent in playbook.get("agents", []) if isinstance(agent, dict)]
    roles = {str(agent.get("role", "")).strip().lower() for agent in agents if agent.get("role")}
    channels = [channel for channel in playbook.get("channels", []) if isinstance(channel, dict)]
    open_tasks = [task for task in playbook.get("open_tasks", []) if isinstance(task, dict)]
    review_dependencies = [task for task in open_tasks if task.get("review_required_by")]
    disruption_seeds = [
        seed for seed in playbook.get("disruption_probe_seeds", []) if isinstance(seed, dict)
    ]
    return {
        "agents": len(agents),
        "differentiated_roles": len(roles),
        "channels": len(channels),
        "tasks": {
            "roots": len(open_tasks),
            "subtasks": 0,
            "dependencies": len(review_dependencies),
            "runs": len(open_tasks),
        },
        "provider_backed_sessions": 1,
        "cross_surface_objects": min(3, len(open_tasks)),
        "disruption_probes": len(disruption_seeds),
        "artifacts_used_later": min(2, len(review_dependencies)),
        "surfaces_required": ["cli", "api", "web", "runtime", "provider"],
        "required_deliverables": playbook.get("required_deliverables", {}),
        "required_collaboration": playbook.get("required_collaboration", {}),
    }


def build_scenario_contract(repo_root: Path, scenario_slug: str, playbook: dict | None = None) -> dict:
    profile = scenario_profile(scenario_slug, playbook)
    minimums = build_contract_minimums(scenario_slug, playbook)
    return {
        "schema_version": 1,
        "release_grade": profile,
        "scope_slug": scenario_slug,
        "minimums": minimums,
        "profile_overrides": {
            "feature": {
                "agents": 4,
                "differentiated_roles": 3,
                "channels": 3,
            },
        },
        "audit_command": str(real_scenario_script(repo_root, "audit-qa-evidence.py")),
        "enforcement": "block",
    }


def build_charter_skeleton(scenario_slug: str) -> dict:
    return {
        "schema_version": 1,
        "startup_situation": f"UNFILLED: realistic startup situation for {scenario_slug}",
        "operator_intent": "UNFILLED: what the operator needs to accomplish",
        "expected_business_outcome": "UNFILLED: user-visible outcome required for success",
        "agents": [],
        "channels": [],
        "task_tree": {
            "roots": [],
            "subtasks": [],
            "dependencies": [],
            "runs": [],
        },
        "provider_plan": {
            "required": True,
            "providers": [],
            "reachability_probe": "UNFILLED: exact provider-backed command to run",
            "fallback_boundary": None,
        },
        "cross_surface_targets": [],
        "disruption_probes": [],
        "artifacts": [],
    }


def build_charter_from_playbook(scenario_slug: str, playbook: dict) -> dict:
    company_name = playbook.get("company", {}).get("name", scenario_slug)
    tagline = playbook.get("company", {}).get("tagline", "")
    operator_role = playbook.get("operator_persona", {}).get("role", "operator")
    open_tasks = playbook.get("open_tasks", []) or []
    agents = [
        {
            "id": agent["id"],
            "role": agent["role"],
            "responsibility": agent.get("persona", agent["role"]),
        }
        for agent in playbook.get("agents", [])
        if isinstance(agent, dict)
    ]
    channels = [
        {"id": channel["id"], "purpose": channel["purpose"]}
        for channel in playbook.get("channels", [])
        if isinstance(channel, dict)
    ]
    artifacts = [
        {
            "path": task.get("deliverable_path_hint", ""),
            "producer_agent": task.get("owner_agent", ""),
            "deliverable_type": task.get("deliverable_type", ""),
            "used_later_by": [task.get("review_required_by")] if task.get("review_required_by") else [],
        }
        for task in open_tasks
    ]
    disruption_probes = [
        {
            "probe_id": f"{seed.get('type', 'probe')}-{seed.get('seed_at_minute', 0)}",
            "type": seed.get("type", ""),
            "delivery": seed.get("delivery", "knowledge_file"),
            "expected_recovery": seed.get("expected_recovery", ""),
        }
        for seed in playbook.get("disruption_probe_seeds", [])
    ]
    return {
        "schema_version": 1,
        "startup_situation": f"{company_name} — {tagline}".strip(" —"),
        "operator_intent": (
            f"{operator_role} drives the active work in {company_name} via a single in-persona kickoff; "
            "the AGH runtime then sustains autonomous collaboration without further QA prompts."
        ),
        "expected_business_outcome": (
            "All required deliverables exist as runnable artifacts (compile/parse/run) and the "
            "required collaboration loops complete (peer messages, reviews, disagreement resolved)."
        ),
        "playbook_ref": playbook.get("playbook_ref", ""),
        "agents": agents,
        "channels": channels,
        "task_tree": {
            "roots": [task["title"] for task in open_tasks if task.get("title")],
            "subtasks": [],
            "dependencies": [
                {
                    "task": task["title"],
                    "review_required_by": task.get("review_required_by", ""),
                }
                for task in open_tasks
                if task.get("review_required_by")
            ],
            "runs": [],
        },
        "provider_plan": {
            "required": True,
            "providers": [],
            "reachability_probe": (
                "Provider sessions are launched by the AGH runtime per the playbook agent registrations. "
                "Each agent system prompt names the persona and product context."
            ),
            "fallback_boundary": None,
        },
        "cross_surface_targets": [
            {"surface": "cli", "object": "task"},
            {"surface": "api", "object": "session"},
            {"surface": "web", "object": "channel"},
            {"surface": "runtime", "object": "task_run"},
        ],
        "disruption_probes": disruption_probes,
        "artifacts": artifacts,
        "required_deliverables": playbook.get("required_deliverables", {}),
        "required_collaboration": playbook.get("required_collaboration", {}),
    }


def load_playbook_via_helper(repo_root: Path, playbook_ref: str) -> dict:
    helper = real_scenario_script(repo_root, "playbook_loader.py")
    if not helper.is_file():
        raise RuntimeError(f"playbook loader not found at {helper}")
    proc = subprocess.run(
        [
            "python3",
            "-c",
            (
                "import json, sys; "
                f"sys.path.insert(0, {str(helper.parent)!r}); "
                "from playbook_loader import load_validated_playbook; "
                f"print(json.dumps(load_validated_playbook({str(repo_root)!r}, {playbook_ref!r})))"
            ),
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(proc.stdout)


def seed_playbook_workspace(repo_root: Path, workspace_path: Path, playbook_ref: str) -> dict:
    helper = real_scenario_script(repo_root, "seed-playbook-workspace.py")
    if not helper.is_file():
        raise RuntimeError(f"seed-playbook-workspace.py not found at {helper}")
    proc = subprocess.run(
        [
            "python3",
            str(helper),
            "--workspace",
            str(workspace_path),
            "--playbook",
            playbook_ref,
            "--repo-root",
            str(repo_root),
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as err:
        raise RuntimeError(
            f"seed-playbook-workspace.py emitted non-JSON output: {err}\n{proc.stdout}"
        ) from err


def build_provider_attempt_stub() -> dict:
    return {
        "schema_version": 1,
        "providers_probed": [],
        "live_proof_session_ids": [],
        "observed_agent_decisions": [],
        "boundary": None,
    }


def write_json_if_absent(path: Path, data: dict) -> None:
    if path.exists():
        return
    path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def seed_qa_evidence_contracts(
    repo_root: Path,
    qa_root: Path,
    scenario_slug: str,
    playbook: dict | None = None,
) -> dict[str, Path]:
    scenario_contract_path = qa_root / "scenario-contract.json"
    charter_path = qa_root / "behavioral-scenario-charter.yaml"
    journey_log_path = qa_root / "journey-log.jsonl"
    provider_attempt_path = qa_root / "provider-attempt.json"
    audit_command = real_scenario_script(repo_root, "audit-qa-evidence.py")
    scenario_contract = build_scenario_contract(repo_root, scenario_slug, playbook)
    if playbook is not None and scenario_contract_path.exists():
        scenario_contract_path.unlink()
    write_json_if_absent(scenario_contract_path, scenario_contract)
    if playbook is not None:
        if charter_path.exists():
            charter_path.unlink()
        charter_path.write_text(
            json.dumps(build_charter_from_playbook(scenario_slug, playbook), indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
    else:
        write_json_if_absent(charter_path, build_charter_skeleton(scenario_slug))
    if not journey_log_path.exists():
        journey_log_path.write_text("", encoding="utf-8")
    write_json_if_absent(provider_attempt_path, build_provider_attempt_stub())
    return {
        "SCENARIO_CONTRACT": scenario_contract_path,
        "BEHAVIORAL_CHARTER": charter_path,
        "JOURNEY_LOG": journey_log_path,
        "PROVIDER_ATTEMPT": provider_attempt_path,
        "AUDIT_COMMAND": audit_command,
    }


def write_env_file(env_path: Path, env_map: dict[str, str]) -> None:
    lines = [f"export {key}={shquote(value)}" for key, value in env_map.items()]
    env_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def fresh_lab_scope(requested_scenario: str) -> str:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S-%f")
    return f"{requested_scenario}-{stamp}"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scenario", default="release-candidate", help="QA scenario slug or context")
    parser.add_argument("--repo-root", default=".", help="Repository root")
    parser.add_argument(
        "--workspace-root",
        default=os.environ.get("AGH_QA_WORKSPACE_ROOT", str(Path.home() / "dev" / "qa-labs")),
        help="Parent directory for the QA lab workspace",
    )
    parser.add_argument(
        "--reuse-manifest",
        default="",
        help="Reuse an existing bootstrap manifest for the same active QA session or loop continuation",
    )
    parser.add_argument(
        "--playbook",
        default="",
        help="Real-scenario QA playbook ref (e.g., northstar-pay). When set, charter and workspace "
        "are materialized from .agents/skills/agh/real-scenario-qa/references/playbooks/<ref>.md.",
    )
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    global_codex_home = Path(os.environ.get("CODEX_HOME", str(Path.home() / ".codex"))).expanduser().resolve()
    scenario = slugify(args.scenario)
    playbook_ref = args.playbook.strip()
    playbook_data: dict | None = None
    if playbook_ref:
        try:
            playbook_data = load_playbook_via_helper(repo_root, playbook_ref)
        except (RuntimeError, subprocess.CalledProcessError, json.JSONDecodeError) as err:
            raise RuntimeError(f"failed to load playbook {playbook_ref!r}: {err}") from err

    reused_lab = False
    status_notes: list[str] = []
    existing_manifest: dict | None = None
    reuse_manifest_path = Path(args.reuse_manifest).expanduser().resolve() if args.reuse_manifest.strip() else None
    if reuse_manifest_path is not None and reuse_manifest_path.is_file():
        existing_manifest = load_json(reuse_manifest_path)
        healthy, health_notes = manifest_health(existing_manifest)
        if healthy:
            reused_lab = True
            status_notes.append("Reused existing healthy QA bootstrap manifest from the active QA session")
        else:
            status_notes.extend(health_notes)
            status_notes.append("Rebuilt QA bootstrap after requested manifest health check failed")

    if reused_lab and existing_manifest is not None:
        workspace_path = Path(str(existing_manifest["workspace_path"])).resolve()
        qa_output_path = Path(str(existing_manifest["qa_output_path"])).resolve()
        workspace_info = {
            "SCENARIO_SLUG": str(existing_manifest.get("scenario_slug") or scenario),
            "WORKSPACE_PATH": str(workspace_path),
            "QA_OUTPUT_PATH": str(qa_output_path),
        }
    else:
        workspace_scope = fresh_lab_scope(scenario)
        workspace_info = run_init_workspace(repo_root, workspace_scope, args.workspace_root)
        workspace_path = Path(workspace_info["WORKSPACE_PATH"]).resolve()
        qa_output_path = Path(workspace_info["QA_OUTPUT_PATH"]).resolve()

    qa_root = qa_output_path / "qa"
    ensure_lab_scaffold(workspace_path, qa_output_path)

    seed_summary: dict = {}
    if playbook_data is not None and not reused_lab:
        try:
            seed_summary = seed_playbook_workspace(repo_root, workspace_path, playbook_ref)
            status_notes.append(
                f"seeded playbook {playbook_ref!r} ({len(seed_summary.get('agents_registered', []))} agents, "
                f"{len(seed_summary.get('knowledge_files_written', []))} knowledge files)"
            )
        except (RuntimeError, subprocess.CalledProcessError) as err:
            raise RuntimeError(f"seed-playbook-workspace failed for {playbook_ref!r}: {err}") from err

    manifest_path = qa_root / "bootstrap-manifest.json"
    env_path = qa_root / "bootstrap.env"
    evidence_paths = seed_qa_evidence_contracts(
        repo_root,
        qa_root,
        workspace_info["SCENARIO_SLUG"],
        playbook=playbook_data,
    )

    if reused_lab and existing_manifest is not None:
        existing_env = existing_manifest.get("env", {})
        runtime_workspace_path = Path(
            str(existing_env.get("RUNTIME_WORKSPACE_PATH", workspace_path))
        ).resolve()
        provider_home = Path(str(existing_env.get("PROVIDER_HOME", workspace_path / ".provider-home"))).resolve()
        agh_home = Path(str(existing_env.get("AGH_HOME", workspace_path / ".agh" / "runtime"))).resolve()
    else:
        runtime_workspace_path = Path(
            str(seed_summary.get("runtime_workspace_path", workspace_path))
        ).resolve()
        provider_home = workspace_path / ".provider-home"
        agh_home = workspace_path / ".agh" / "runtime"
        violations = socket_limit_violations(socket_limited_paths(agh_home, provider_home))
        if violations:
            runtime_root = short_runtime_root(workspace_info["SCENARIO_SLUG"])
            provider_home = runtime_root / "provider"
            agh_home = runtime_root / "runtime"
            status_notes.append(
                "Allocated short runtime/provider homes for portable Unix socket paths: " + "; ".join(violations)
            )
            followup_violations = socket_limit_violations(socket_limited_paths(agh_home, provider_home))
            if followup_violations:
                raise RuntimeError(
                    "short QA runtime paths still exceed Unix socket limits: " + "; ".join(followup_violations)
                )

    provider_codex_home = prepare_provider_home(global_codex_home, provider_home)

    env_block: dict[str, str]
    if reused_lab and existing_manifest is not None:
        env_block = {key: str(value) for key, value in existing_manifest.get("env", {}).items()}
    else:
        agh_home.mkdir(parents=True, exist_ok=True)
        env_block = {
            "SCENARIO_SLUG": workspace_info["SCENARIO_SLUG"],
            "WORKSPACE_PATH": str(workspace_path),
            "RUNTIME_WORKSPACE_PATH": str(runtime_workspace_path),
            "QA_OUTPUT_PATH": str(qa_output_path),
            "AGH_HOME": str(agh_home),
            "AGH_HTTP_PORT": str(pick_free_port()),
            "AGH_UDS_PATH": str(agh_home / "aghd.sock"),
            "TMUX_BRIDGE_SOCKET": str(agh_home / "tmux-bridge.sock"),
            "AGH_WEB_API_PROXY_TARGET": "",
            "PROVIDER_HOME": str(provider_home),
            "PROVIDER_CODEX_HOME": str(provider_codex_home),
            "BROWSER_MODE": "",
            "BROWSER_BLOCKER": "",
            "SCENARIO_CONTRACT": str(evidence_paths["SCENARIO_CONTRACT"]),
            "BEHAVIORAL_CHARTER": str(evidence_paths["BEHAVIORAL_CHARTER"]),
            "JOURNEY_LOG": str(evidence_paths["JOURNEY_LOG"]),
            "PROVIDER_ATTEMPT": str(evidence_paths["PROVIDER_ATTEMPT"]),
            "AUDIT_COMMAND": str(evidence_paths["AUDIT_COMMAND"]),
            "PLAYBOOK_REF": playbook_ref,
            "KICKOFF_POSTED": "false",
            "KICKOFF_TIMESTAMP": "",
        }
        write_runtime_config(agh_home, env_block["AGH_HTTP_PORT"], env_block["AGH_UDS_PATH"])

    env_block["SCENARIO_SLUG"] = workspace_info["SCENARIO_SLUG"]
    env_block["WORKSPACE_PATH"] = str(workspace_path)
    env_block["RUNTIME_WORKSPACE_PATH"] = str(runtime_workspace_path)
    env_block["QA_OUTPUT_PATH"] = str(qa_output_path)
    env_block["PROVIDER_HOME"] = str(provider_home)
    env_block["PROVIDER_CODEX_HOME"] = str(provider_codex_home)
    env_block["AGH_WEB_API_PROXY_TARGET"] = f"http://127.0.0.1:{env_block['AGH_HTTP_PORT']}"
    env_block["SCENARIO_CONTRACT"] = str(evidence_paths["SCENARIO_CONTRACT"])
    env_block["BEHAVIORAL_CHARTER"] = str(evidence_paths["BEHAVIORAL_CHARTER"])
    env_block["JOURNEY_LOG"] = str(evidence_paths["JOURNEY_LOG"])
    env_block["PROVIDER_ATTEMPT"] = str(evidence_paths["PROVIDER_ATTEMPT"])
    env_block["AUDIT_COMMAND"] = str(evidence_paths["AUDIT_COMMAND"])
    teardown_script = repo_root / ".agents" / "skills" / "agh" / "agh-qa-bootstrap" / "scripts" / "teardown-qa-env.py"
    env_block["TEARDOWN_COMMAND"] = f"python3 {teardown_script} --manifest {manifest_path}"
    env_block.setdefault("PLAYBOOK_REF", playbook_ref)
    env_block.setdefault("KICKOFF_POSTED", "false")
    env_block.setdefault("KICKOFF_TIMESTAMP", "")
    if playbook_ref:
        env_block["PLAYBOOK_REF"] = playbook_ref

    browser_mode, browser_blocker = detect_browser_mode(global_codex_home)
    env_block["BROWSER_MODE"] = browser_mode
    env_block["BROWSER_BLOCKER"] = browser_blocker

    project_contract = discover_project_contract(repo_root)
    manifest = {
        "schema_version": 1,
        "scenario_slug": workspace_info["SCENARIO_SLUG"],
        "workspace_path": str(workspace_path),
        "qa_output_path": str(qa_output_path),
        "manifest_path": str(manifest_path),
        "bootstrap_env_path": str(env_path),
        "status": {
            "reused_lab": reused_lab,
            "health": "healthy" if reused_lab else "fresh",
            "notes": status_notes,
        },
        "env": env_block,
        "browser": {
            "mode": browser_mode,
            "blocker": browser_blocker,
        },
        "paths": {
            "project_root": str(repo_root),
            "runtime_workspace": str(runtime_workspace_path),
            "qa_root": str(qa_root),
            "provider_home": str(provider_home),
            "provider_codex_home": str(provider_codex_home),
            "scenario_contract": str(evidence_paths["SCENARIO_CONTRACT"]),
            "behavioral_charter": str(evidence_paths["BEHAVIORAL_CHARTER"]),
            "journey_log": str(evidence_paths["JOURNEY_LOG"]),
            "provider_attempt": str(evidence_paths["PROVIDER_ATTEMPT"]),
            "audit_command": str(evidence_paths["AUDIT_COMMAND"]),
            "teardown_command": env_block["TEARDOWN_COMMAND"],
        },
        "project_contract": project_contract,
    }

    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    write_env_file(env_path, env_block)

    outputs = {
        "SCENARIO_SLUG": workspace_info["SCENARIO_SLUG"],
        "WORKSPACE_PATH": str(workspace_path),
        "RUNTIME_WORKSPACE_PATH": env_block["RUNTIME_WORKSPACE_PATH"],
        "QA_OUTPUT_PATH": str(qa_output_path),
        "BOOTSTRAP_MANIFEST": str(manifest_path),
        "BOOTSTRAP_ENV": str(env_path),
        "AGH_HOME": env_block["AGH_HOME"],
        "AGH_HTTP_PORT": env_block["AGH_HTTP_PORT"],
        "AGH_UDS_PATH": env_block["AGH_UDS_PATH"],
        "TMUX_BRIDGE_SOCKET": env_block["TMUX_BRIDGE_SOCKET"],
        "AGH_WEB_API_PROXY_TARGET": env_block["AGH_WEB_API_PROXY_TARGET"],
        "PROVIDER_HOME": env_block["PROVIDER_HOME"],
        "PROVIDER_CODEX_HOME": env_block["PROVIDER_CODEX_HOME"],
        "BROWSER_MODE": env_block["BROWSER_MODE"],
        "BROWSER_BLOCKER": env_block["BROWSER_BLOCKER"],
        "SCENARIO_CONTRACT": env_block["SCENARIO_CONTRACT"],
        "BEHAVIORAL_CHARTER": env_block["BEHAVIORAL_CHARTER"],
        "JOURNEY_LOG": env_block["JOURNEY_LOG"],
        "PROVIDER_ATTEMPT": env_block["PROVIDER_ATTEMPT"],
        "AUDIT_COMMAND": env_block["AUDIT_COMMAND"],
        "TEARDOWN_COMMAND": env_block["TEARDOWN_COMMAND"],
        "REUSED_LAB": "true" if reused_lab else "false",
        "PLAYBOOK_REF": env_block.get("PLAYBOOK_REF", ""),
        "KICKOFF_POSTED": env_block.get("KICKOFF_POSTED", "false"),
    }
    for key, value in outputs.items():
        print(f"{key}={value}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

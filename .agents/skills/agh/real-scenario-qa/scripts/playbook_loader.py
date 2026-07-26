#!/usr/bin/env python3
"""Shared loader for real-scenario-qa playbooks.

Read-only. Imported by the real-scenario seeding, kickoff, validation, and audit helpers.
Each playbook is a markdown file ending with one ```json fenced block carrying the canonical spec.
"""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any


PLAYBOOKS_REL = ".agents/skills/agh/real-scenario-qa/references/playbooks"
SCHEMA_REL = ".agents/skills/agh/real-scenario-qa/references/playbook-schema.json"
FORBIDDEN_REL = ".agents/skills/agh/real-scenario-qa/references/forbidden-prompt-phrases.md"

_FENCE_RE = re.compile(r"```json\s*\n(.*?)\n```", re.DOTALL)
DELIVERABLE_EXTENSIONS = {
    "tsx_page": ".tsx",
    "tsx_component": ".tsx",
    "ts_module": ".ts",
    "ts_test": ".test.ts",
    "go_service_stub": ".go",
    "go_test": "_test.go",
    "python_script": ".py",
    "shell_script": ".sh",
    "sql_migration": ".sql",
    "runbook_md": ".md",
    "spec_md": ".md",
    "config_toml": ".toml",
    "html_static": ".html",
    "yaml_workflow": ".yaml",
}
MARKDOWN_DELIVERABLE_TYPES = {"runbook_md", "spec_md"}
REQUIRED_TOP_LEVEL = {
    "schema_version",
    "playbook_ref",
    "company",
    "operator_persona",
    "kickoff_brief",
    "workspaces",
    "agents",
    "channels",
    "knowledge_files",
    "open_tasks",
    "required_deliverables",
    "required_collaboration",
}


class PlaybookError(RuntimeError):
    """Raised when a playbook cannot be parsed or fails validation."""


def repo_root_from(path: Path) -> Path:
    """Walk up until a directory containing .agents is found."""
    current = path.resolve()
    for candidate in [current, *current.parents]:
        if (candidate / ".agents").is_dir():
            return candidate
    raise PlaybookError(f"could not locate repo root from {path}")


def _coerce_root(repo_root: Path | str) -> Path:
    return repo_root if isinstance(repo_root, Path) else Path(repo_root)


def playbook_path(repo_root: Path | str, ref: str) -> Path:
    root = _coerce_root(repo_root)
    if not re.fullmatch(r"[a-z0-9]+(-[a-z0-9]+)*", ref):
        raise PlaybookError(f"invalid playbook ref: {ref}")
    candidate = root / PLAYBOOKS_REL / f"{ref}.md"
    if not candidate.is_file():
        raise PlaybookError(f"playbook not found: {candidate}")
    return candidate


def load_playbook(repo_root: Path | str, ref: str) -> dict[str, Any]:
    path = playbook_path(repo_root, ref)
    text = path.read_text(encoding="utf-8")
    matches = _FENCE_RE.findall(text)
    if not matches:
        raise PlaybookError(f"playbook {path} has no ```json fenced block")
    if len(matches) > 1:
        raise PlaybookError(f"playbook {path} has multiple ```json blocks; expected exactly one")
    try:
        data = json.loads(matches[0])
    except json.JSONDecodeError as err:
        raise PlaybookError(f"playbook {path} JSON block is invalid: {err}") from err
    if not isinstance(data, dict):
        raise PlaybookError(f"playbook {path} JSON block must be an object")
    if data.get("playbook_ref") != ref:
        raise PlaybookError(
            f"playbook {path} declares playbook_ref={data.get('playbook_ref')!r}; expected {ref!r}"
        )
    return data


def _require_keys(data: dict[str, Any], keys: set[str], source: str, errors: list[str]) -> None:
    missing = sorted(keys - set(data))
    if missing:
        errors.append(f"{source} missing required keys: {', '.join(missing)}")


def _as_list(data: dict[str, Any], key: str, source: str, errors: list[str]) -> list[Any]:
    value = data.get(key)
    if not isinstance(value, list):
        errors.append(f"{source}.{key} must be an array")
        return []
    return value


def _as_dict(data: dict[str, Any], key: str, source: str, errors: list[str]) -> dict[str, Any]:
    value = data.get(key)
    if not isinstance(value, dict):
        errors.append(f"{source}.{key} must be an object")
        return {}
    return value


def _indexed_ids(items: list[Any], key: str, source: str, errors: list[str]) -> set[str]:
    ids: set[str] = set()
    seen: set[str] = set()
    for index, item in enumerate(items):
        if not isinstance(item, dict):
            errors.append(f"{source}[{index}] must be an object")
            continue
        value = item.get(key)
        if not isinstance(value, str) or not value.strip():
            errors.append(f"{source}[{index}].{key} must be a non-empty string")
            continue
        if value in seen:
            errors.append(f"{source}[{index}].{key} duplicates {value!r}")
        seen.add(value)
        ids.add(value)
    return ids


def _scan_forbidden(source: str, text: str, rules: dict[str, list[str]], errors: list[str]) -> None:
    lowered = text.lower()
    for literal in rules.get("literals", []):
        if literal.lower() in lowered:
            errors.append(f"{source} contains forbidden literal {literal!r}")
    for pattern in rules.get("patterns", []):
        try:
            matched = re.search(pattern, text)
        except re.error as err:
            errors.append(f"forbidden regex {pattern!r} is invalid: {err}")
            continue
        if matched:
            errors.append(f"{source} matches forbidden regex {pattern!r}")


def _validate_relative_path(source: str, value: str, errors: list[str]) -> None:
    path = Path(value)
    if path.is_absolute() or ".." in path.parts:
        errors.append(f"{source} must be a relative path without '..': {value!r}")


def validate_playbook_data(repo_root: Path | str, ref: str, data: dict[str, Any]) -> None:
    errors: list[str] = []
    _require_keys(data, REQUIRED_TOP_LEVEL, "playbook", errors)
    if data.get("schema_version") != 1:
        errors.append("playbook.schema_version must be 1")
    if data.get("playbook_ref") != ref:
        errors.append(f"playbook.playbook_ref must match {ref!r}")

    workspaces = _as_list(data, "workspaces", "playbook", errors)
    agents = _as_list(data, "agents", "playbook", errors)
    channels = _as_list(data, "channels", "playbook", errors)
    knowledge_files = _as_list(data, "knowledge_files", "playbook", errors)
    open_tasks = _as_list(data, "open_tasks", "playbook", errors)
    required_deliverables = _as_dict(data, "required_deliverables", "playbook", errors)
    required_collaboration = _as_dict(data, "required_collaboration", "playbook", errors)

    if len(workspaces) < 3:
        errors.append("playbook.workspaces must contain at least 3 entries")
    if len(agents) < 6:
        errors.append("playbook.agents must contain at least 6 entries")
    if len(channels) < 4:
        errors.append("playbook.channels must contain at least 4 entries")
    if len(knowledge_files) < 3:
        errors.append("playbook.knowledge_files must contain at least 3 entries")
    if len(open_tasks) < 4:
        errors.append("playbook.open_tasks must contain at least 4 entries")

    workspace_ids = _indexed_ids(workspaces, "id", "playbook.workspaces", errors)
    agent_ids = _indexed_ids(agents, "id", "playbook.agents", errors)
    channel_ids = _indexed_ids(channels, "id", "playbook.channels", errors)

    rules = load_forbidden_rules(repo_root)
    kickoff = data.get("kickoff_brief")
    if isinstance(kickoff, str):
        if len(kickoff.strip()) < 80:
            errors.append("playbook.kickoff_brief must be at least 80 characters")
        _scan_forbidden("playbook.kickoff_brief", kickoff, rules, errors)
    else:
        errors.append("playbook.kickoff_brief must be a string")

    company = _as_dict(data, "company", "playbook", errors)
    _require_keys(company, {"name", "tagline", "stage"}, "playbook.company", errors)
    persona = _as_dict(data, "operator_persona", "playbook", errors)
    _require_keys(persona, {"role", "voice_guidelines"}, "playbook.operator_persona", errors)

    for index, workspace in enumerate(workspaces):
        if not isinstance(workspace, dict):
            continue
        _require_keys(workspace, {"id", "name", "purpose"}, f"playbook.workspaces[{index}]", errors)

    for index, channel in enumerate(channels):
        if not isinstance(channel, dict):
            continue
        _require_keys(channel, {"id", "purpose"}, f"playbook.channels[{index}]", errors)
        primary_workspace = channel.get("primary_workspace")
        if isinstance(primary_workspace, str) and primary_workspace and primary_workspace not in workspace_ids:
            errors.append(
                f"playbook.channels[{index}].primary_workspace references unknown workspace {primary_workspace!r}"
            )

    for index, agent in enumerate(agents):
        if not isinstance(agent, dict):
            continue
        _require_keys(agent, {"id", "role", "persona", "system_prompt", "workspace"}, f"playbook.agents[{index}]", errors)
        workspace = agent.get("workspace")
        if isinstance(workspace, str) and workspace not in workspace_ids:
            errors.append(f"playbook.agents[{index}].workspace references unknown workspace {workspace!r}")
        system_prompt = agent.get("system_prompt")
        if isinstance(system_prompt, str):
            if len(system_prompt.strip()) < 60:
                errors.append(f"playbook.agents[{index}].system_prompt must be at least 60 characters")
            _scan_forbidden(f"playbook.agents[{index}].system_prompt", system_prompt, rules, errors)
        else:
            errors.append(f"playbook.agents[{index}].system_prompt must be a string")

    knowledge_paths: set[str] = set()
    for index, entry in enumerate(knowledge_files):
        if not isinstance(entry, dict):
            errors.append(f"playbook.knowledge_files[{index}] must be an object")
            continue
        _require_keys(entry, {"path", "content"}, f"playbook.knowledge_files[{index}]", errors)
        path = entry.get("path")
        if isinstance(path, str):
            _validate_relative_path(f"playbook.knowledge_files[{index}].path", path, errors)
            if path in knowledge_paths:
                errors.append(f"playbook.knowledge_files contains duplicate path {path!r}")
            knowledge_paths.add(path)

    assigned_scoped_paths: set[str] = set()
    for index, workspace in enumerate(workspaces):
        if not isinstance(workspace, dict):
            continue
        refs = workspace.get("knowledge_files", [])
        if refs is None:
            refs = []
        if not isinstance(refs, list):
            errors.append(f"playbook.workspaces[{index}].knowledge_files must be an array")
            continue
        for ref in refs:
            if not isinstance(ref, str) or not ref.strip():
                errors.append(
                    f"playbook.workspaces[{index}].knowledge_files entries must be non-empty strings"
                )
                continue
            if ref not in knowledge_paths:
                errors.append(
                    f"playbook.workspaces[{index}].knowledge_files references unknown path {ref!r}"
                )
            assigned_scoped_paths.add(ref)
    unassigned_scoped = sorted(
        path for path in knowledge_paths if not path.startswith("global/") and path not in assigned_scoped_paths
    )
    if unassigned_scoped:
        errors.append(
            "playbook workspace-scoped knowledge must be assigned: "
            + ", ".join(unassigned_scoped)
        )

    for index, task in enumerate(open_tasks):
        if not isinstance(task, dict):
            errors.append(f"playbook.open_tasks[{index}] must be an object")
            continue
        _require_keys(task, {"title", "owner_agent", "deliverable_type"}, f"playbook.open_tasks[{index}]", errors)
        owner = task.get("owner_agent")
        if isinstance(owner, str) and owner not in agent_ids:
            errors.append(f"playbook.open_tasks[{index}].owner_agent references unknown agent {owner!r}")
        reviewer = task.get("review_required_by")
        if isinstance(reviewer, str) and reviewer and reviewer not in agent_ids:
            errors.append(f"playbook.open_tasks[{index}].review_required_by references unknown agent {reviewer!r}")
        channel = task.get("channel")
        if isinstance(channel, str) and channel and channel not in channel_ids:
            errors.append(f"playbook.open_tasks[{index}].channel references unknown channel {channel!r}")
        deliverable_type = task.get("deliverable_type")
        if deliverable_type not in DELIVERABLE_EXTENSIONS:
            errors.append(f"playbook.open_tasks[{index}].deliverable_type is unknown: {deliverable_type!r}")
        path_hint = task.get("deliverable_path_hint")
        if isinstance(path_hint, str) and path_hint:
            _validate_relative_path(f"playbook.open_tasks[{index}].deliverable_path_hint", path_hint, errors)

    non_markdown_required = 0
    for deliverable_type, minimum in required_deliverables.items():
        if deliverable_type not in DELIVERABLE_EXTENSIONS:
            errors.append(f"playbook.required_deliverables contains unknown type {deliverable_type!r}")
            continue
        if not isinstance(minimum, int) or minimum < 0:
            errors.append(f"playbook.required_deliverables[{deliverable_type!r}] must be a non-negative integer")
            continue
        if deliverable_type not in MARKDOWN_DELIVERABLE_TYPES:
            non_markdown_required += minimum
    if non_markdown_required < 4:
        errors.append("playbook.required_deliverables must require at least 4 non-markdown deliverables")

    for key in ("peer_messages_min", "review_cycles_min", "disagreements_resolved_min"):
        value = required_collaboration.get(key)
        if not isinstance(value, int):
            errors.append(f"playbook.required_collaboration.{key} must be an integer")
    channels_active = required_collaboration.get("channels_active_min")
    if channels_active is not None and not isinstance(channels_active, int):
        errors.append("playbook.required_collaboration.channels_active_min must be an integer when set")

    for index, seed in enumerate(data.get("disruption_probe_seeds", []) or []):
        if not isinstance(seed, dict):
            errors.append(f"playbook.disruption_probe_seeds[{index}] must be an object")
            continue
        _require_keys(seed, {"type", "seed_at_minute", "expected_recovery"}, f"playbook.disruption_probe_seeds[{index}]", errors)
        delivery = seed.get("delivery", "knowledge_file")
        if delivery not in {"knowledge_file", "channel_message", "task_event", "config_change"}:
            errors.append(f"playbook.disruption_probe_seeds[{index}].delivery is invalid: {delivery!r}")

    if errors:
        raise PlaybookError("playbook validation failed:\n- " + "\n- ".join(errors))


def load_validated_playbook(repo_root: Path | str, ref: str) -> dict[str, Any]:
    data = load_playbook(repo_root, ref)
    validate_playbook_data(repo_root, ref, data)
    return data


def list_playbooks(repo_root: Path | str) -> list[str]:
    base = _coerce_root(repo_root) / PLAYBOOKS_REL
    return sorted(p.stem for p in base.glob("*.md") if p.stem != "README")


def load_forbidden_rules(repo_root: Path | str) -> dict[str, list[str]]:
    """Return {'literals': [...], 'patterns': [...]} parsed from forbidden-prompt-phrases.md."""
    text = (_coerce_root(repo_root) / FORBIDDEN_REL).read_text(encoding="utf-8")
    rules: dict[str, list[str]] = {"literals": [], "patterns": []}
    section: str | None = None
    for raw in text.splitlines():
        line = raw.strip()
        if line.startswith("## Hard-blocking literals"):
            section = "literals"
            continue
        if line.startswith("## Hard-blocking regex patterns"):
            section = "patterns"
            continue
        if line.startswith("## ") and section in {"literals", "patterns"}:
            section = None
            continue
        if section is None:
            continue
        if not line.startswith("- "):
            continue
        body = line[2:].strip()
        if body.startswith("`") and body.endswith("`"):
            rules[section].append(body[1:-1])
        else:
            backtick = re.search(r"`([^`]+)`", body)
            if backtick:
                rules[section].append(backtick.group(1))
    return rules


def deliverable_extension(deliverable_type: str) -> str:
    return DELIVERABLE_EXTENSIONS[deliverable_type]


def is_markdown_deliverable(deliverable_type: str) -> bool:
    return deliverable_type in MARKDOWN_DELIVERABLE_TYPES

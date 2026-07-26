#!/usr/bin/env python3
"""Materialize an AGH real-scenario QA playbook into a workspace.

Mutating helper. Called by the bootstrap when --playbook is provided.

Per playbook:
  - Creates an agent-visible project root under WORKSPACE_PATH/project/
  - Creates logical workspace directories under WORKSPACE_PATH/project/workspaces/<workspace.name>/
  - Writes canonical knowledge under WORKSPACE_PATH/knowledge/
  - Projects global and declared workspace knowledge into each readable workspace root
  - Writes a per-agent registration manifest at WORKSPACE_PATH/.agh/agents/<agent-id>.json
  - Writes the open task tree with deterministic runtime ids at WORKSPACE_PATH/.agh/tasks/open-tasks.json
  - Writes WORKSPACE_PATH/.agh/playbook.json with the resolved playbook spec for downstream tools

This script never starts the daemon, never calls `agh ...`, and never sends a prompt. The bootstrap
script and the real-scenario-qa skill drive registration and kickoff in subsequent steps.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
from playbook_loader import (  # noqa: E402  (import after sys.path mutation)
    PlaybookError,
    deliverable_extension,
    load_validated_playbook,
    repo_root_from,
)


def write_text(path: Path, body: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if not body.endswith("\n"):
        body += "\n"
    path.write_text(body, encoding="utf-8")


def write_json(path: Path, payload: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def materialize_workspaces(workspace_root: Path, playbook: dict) -> list[str]:
    notes: list[str] = []
    for ws in playbook["workspaces"]:
        ws_dir = workspace_root / "workspaces" / ws["name"]
        ws_dir.mkdir(parents=True, exist_ok=True)
        readme = ws_dir / "README.md"
        if not readme.exists():
            write_text(
                readme,
                f"# {ws['name']}\n\n{ws['purpose']}\n\nWorkspace id: `{ws['id']}`. "
                f"Owned channels and tasks for this workspace are seeded by the playbook "
                f"`{playbook['playbook_ref']}`.\n",
            )
        notes.append(str(ws_dir))
    return notes


def materialize_knowledge(
    workspace_root: Path,
    runtime_workspace_root: Path,
    playbook: dict,
) -> tuple[list[str], list[str]]:
    canonical_written: list[str] = []
    projected_written: list[str] = []
    knowledge_root = workspace_root / "knowledge"
    entries: dict[str, str] = {}
    for entry in playbook["knowledge_files"]:
        rel = Path(entry["path"])
        if rel.is_absolute() or ".." in rel.parts:
            raise PlaybookError(f"knowledge_files[{entry['path']!r}] must be a relative path")
        rel_text = rel.as_posix()
        if rel_text in entries:
            raise PlaybookError(f"knowledge_files contains duplicate path {rel_text!r}")
        entries[rel_text] = entry["content"]
        target = knowledge_root / rel
        write_text(target, entry["content"])
        canonical_written.append(str(target))

    global_paths = {path for path in entries if Path(path).parts[0] == "global"}
    declared_scoped_paths: set[str] = set()
    for workspace in playbook["workspaces"]:
        workspace_paths = set(workspace.get("knowledge_files", []))
        unknown = sorted(workspace_paths - set(entries))
        if unknown:
            raise PlaybookError(
                f"workspace {workspace['id']} references unknown knowledge file(s): "
                + ", ".join(unknown)
            )
        declared_scoped_paths.update(workspace_paths)
        readable_paths = sorted(global_paths | workspace_paths)
        workspace_knowledge_root = (
            runtime_workspace_root / "workspaces" / workspace["name"] / "knowledge"
        )
        for rel_text in readable_paths:
            target = workspace_knowledge_root / rel_text
            write_text(target, entries[rel_text])
            projected_written.append(str(target))

    unassigned = sorted(set(entries) - global_paths - declared_scoped_paths)
    if unassigned:
        raise PlaybookError(
            "workspace-scoped knowledge must be assigned to at least one workspace: "
            + ", ".join(unassigned)
        )
    return canonical_written, projected_written


def materialize_agents(
    workspace_root: Path,
    runtime_workspace_root: Path,
    playbook: dict,
) -> list[str]:
    base = workspace_root / ".agh" / "agents"
    written: list[str] = []
    workspace_paths = {
        ws["id"]: str(runtime_workspace_root / "workspaces" / ws["name"])
        for ws in playbook["workspaces"]
    }
    for agent in playbook["agents"]:
        ws_path = workspace_paths.get(agent["workspace"])
        if ws_path is None:
            raise PlaybookError(
                f"agent {agent['id']} references unknown workspace {agent['workspace']}"
            )
        payload = {
            "id": agent["id"],
            "role": agent["role"],
            "persona": agent["persona"],
            "system_prompt": agent["system_prompt"],
            "workspace_id": agent["workspace"],
            "workspace_path": ws_path,
            "skills": agent.get("skills", []),
            "playbook_ref": playbook["playbook_ref"],
        }
        target = base / f"{agent['id']}.json"
        write_json(target, payload)
        written.append(str(target))
    return written


def materialize_open_tasks(
    workspace_root: Path,
    runtime_workspace_root: Path,
    playbook: dict,
) -> str:
    workspace_lookup = {agent["id"]: agent["workspace"] for agent in playbook["agents"]}
    workspace_paths = {
        ws["id"]: str(runtime_workspace_root / "workspaces" / ws["name"])
        for ws in playbook["workspaces"]
    }
    payload: list[dict] = []
    playbook_slug = "".join(
        ch.lower() if ch.isalnum() else "-" for ch in playbook["playbook_ref"].strip()
    ).strip("-")
    for index, task in enumerate(playbook["open_tasks"], start=1):
        owner = task["owner_agent"]
        ws_id = workspace_lookup.get(owner)
        if ws_id is None:
            raise PlaybookError(f"open_tasks owner_agent {owner} not declared in agents")
        ws_path = workspace_paths.get(ws_id, "")
        deliverable_path = task.get("deliverable_path_hint", "")
        if not deliverable_path:
            extension = deliverable_extension(task["deliverable_type"])
            slug = task["title"].lower().replace(" ", "-")
            slug = "".join(ch if ch.isalnum() or ch == "-" else "" for ch in slug).strip("-")
            deliverable_path = f"{ws_id}/{slug}{extension}"
        payload.append(
            {
                "runtime_id": f"task-{playbook_slug}-{index:03d}",
                "title": task["title"],
                "description": task.get("description", ""),
                "owner_agent": owner,
                "owner_workspace_id": ws_id,
                "owner_workspace_path": ws_path,
                "deliverable_type": task["deliverable_type"],
                "deliverable_path": deliverable_path,
                "review_required_by": task.get("review_required_by", ""),
                "channel": task.get("channel", ""),
                "playbook_ref": playbook["playbook_ref"],
            }
        )
    target = workspace_root / ".agh" / "tasks" / "open-tasks.json"
    write_json(target, payload)
    return str(target)


def write_playbook_snapshot(workspace_root: Path, playbook: dict) -> str:
    target = workspace_root / ".agh" / "playbook.json"
    write_json(target, playbook)
    return str(target)


def write_disruption_seeds(workspace_root: Path, playbook: dict) -> str:
    seeds = playbook.get("disruption_probe_seeds", [])
    target = workspace_root / ".agh" / "disruption-seeds.json"
    write_json(
        target,
        {
            "playbook_ref": playbook["playbook_ref"],
            "seeds": seeds,
        },
    )
    return str(target)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--workspace", required=True, help="Absolute WORKSPACE_PATH")
    parser.add_argument("--playbook", required=True, help="Playbook ref (e.g., northstar-pay)")
    parser.add_argument("--repo-root", default=".", help="Repository root")
    args = parser.parse_args()

    workspace_root = Path(args.workspace).resolve()
    if not workspace_root.is_dir():
        print(f"workspace {workspace_root} does not exist", file=sys.stderr)
        return 2

    repo_root = Path(args.repo_root).resolve()
    if not (repo_root / ".agents").is_dir():
        try:
            repo_root = repo_root_from(repo_root)
        except PlaybookError as err:
            print(str(err), file=sys.stderr)
            return 2

    try:
        playbook = load_validated_playbook(repo_root, args.playbook)
    except PlaybookError as err:
        print(str(err), file=sys.stderr)
        return 2

    runtime_workspace_root = workspace_root / "project"
    runtime_workspace_root.mkdir(parents=True, exist_ok=True)
    workspaces_written = materialize_workspaces(runtime_workspace_root, playbook)
    knowledge_written, workspace_knowledge_written = materialize_knowledge(
        workspace_root,
        runtime_workspace_root,
        playbook,
    )
    agents_written = materialize_agents(workspace_root, runtime_workspace_root, playbook)
    open_tasks_path = materialize_open_tasks(workspace_root, runtime_workspace_root, playbook)
    snapshot_path = write_playbook_snapshot(workspace_root, playbook)
    seeds_path = write_disruption_seeds(workspace_root, playbook)

    summary = {
        "playbook_ref": playbook["playbook_ref"],
        "runtime_workspace_path": str(runtime_workspace_root),
        "workspaces_created": workspaces_written,
        "knowledge_files_written": knowledge_written,
        "workspace_knowledge_files_written": workspace_knowledge_written,
        "agents_registered": agents_written,
        "open_tasks_path": open_tasks_path,
        "playbook_snapshot": snapshot_path,
        "disruption_seeds_path": seeds_path,
    }
    print(json.dumps(summary, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

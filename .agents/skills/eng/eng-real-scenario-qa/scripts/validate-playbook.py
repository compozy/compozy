#!/usr/bin/env python3
"""Validate one or all real-scenario QA playbooks.

Read-only helper. It parses the markdown fenced JSON through playbook_loader.py,
validates cross-references and forbidden prompt phrases, and prints a JSON
summary that downstream agents can cite as deterministic evidence.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
from playbook_loader import (  # noqa: E402
    PlaybookError,
    list_playbooks,
    load_validated_playbook,
    repo_root_from,
)


def resolve_repo_root(value: str) -> Path:
    candidate = Path(value).resolve()
    if (candidate / ".agents").is_dir():
        return candidate
    return repo_root_from(candidate)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", default=".", help="Repository root")
    parser.add_argument("--playbook", default="", help="Single playbook ref to validate")
    parser.add_argument("--all", action="store_true", help="Validate every playbook in references/playbooks")
    args = parser.parse_args()

    try:
        repo_root = resolve_repo_root(args.repo_root)
        refs = list_playbooks(repo_root) if args.all else [args.playbook.strip()]
        refs = [ref for ref in refs if ref]
        if not refs:
            raise PlaybookError("pass --playbook <ref> or --all")
        summaries = []
        for ref in refs:
            playbook = load_validated_playbook(repo_root, ref)
            summaries.append(
                {
                    "playbook_ref": ref,
                    "agents": len(playbook.get("agents", [])),
                    "channels": len(playbook.get("channels", [])),
                    "knowledge_files": len(playbook.get("knowledge_files", [])),
                    "open_tasks": len(playbook.get("open_tasks", [])),
                    "required_deliverables": playbook.get("required_deliverables", {}),
                    "required_collaboration": playbook.get("required_collaboration", {}),
                }
            )
    except PlaybookError as err:
        print(str(err), file=sys.stderr)
        return 2

    print(json.dumps({"valid": True, "playbooks": summaries}, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

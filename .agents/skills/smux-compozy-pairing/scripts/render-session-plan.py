#!/usr/bin/env python3
"""Emit shell-safe launch variables for the smux-compozy-pairing skill."""

from __future__ import annotations

import argparse
import os
import re
import shlex
import sys

FEATURE_RE = re.compile(r"^[a-z][a-z0-9-]{0,63}$")


def shell_assign(name: str, value: str) -> str:
    return f"{name}={shlex.quote(value)}"


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Emit shell-safe variables for the smux-compozy-pairing skill."
    )
    parser.add_argument("--feature-name", required=True, help="Workflow slug under .compozy/tasks/.")
    parser.add_argument("--repo-root", required=True, help="Repository root to anchor pane commands.")
    parser.add_argument(
        "--session-prefix",
        default="smux-pair",
        help="Prefix for the tmux session name.",
    )
    parser.add_argument(
        "--claude-model",
        default="opus",
        help="Interactive Claude model alias to launch.",
    )
    args = parser.parse_args()

    feature_name = args.feature_name.strip()
    if not FEATURE_RE.fullmatch(feature_name):
        print(
            "feature-name must match ^[a-z][a-z0-9-]{0,63}$",
            file=sys.stderr,
        )
        return 1

    repo_root = os.path.abspath(args.repo_root)
    if not os.path.isdir(repo_root):
        print(f"repo root does not exist: {repo_root}", file=sys.stderr)
        return 1

    skill_root = ".agents/skills/smux-compozy-pairing"
    tasks_dir = f".compozy/tasks/{feature_name}"
    session_name = f"{args.session_prefix}-{feature_name}"
    orchestrator_label = f"{feature_name}-orchestrator"
    codex_label = f"{feature_name}-codex"
    claude_label = f"{feature_name}-claude"

    codex_launch = shlex.join(
        [
            "codex",
            "--cd",
            repo_root,
            "--no-alt-screen",
            "--model",
            "gpt-5.4",
            "-c",
            'reasoning_effort="xhigh"',
        ]
    )
    claude_launch = shlex.join(
        ["claude", "--model", args.claude_model, "--permission-mode", "bypassPermissions"]
    )
    validate_command = shlex.join(["compozy", "validate-tasks", "--name", feature_name])
    start_command = shlex.join(
        [
            "compozy",
            "start",
            "--name",
            feature_name,
            "--ide",
            "codex",
            "--model",
            "gpt-5.4",
            "--reasoning-effort",
            "xhigh",
        ]
    )

    values = {
        "REPO_ROOT": repo_root,
        "SESSION_NAME": session_name,
        "WINDOW_NAME": "pair",
        "TASKS_DIR": tasks_dir,
        "PRD_PATH": f"{tasks_dir}/_prd.md",
        "PRD_COMMAND": f"/cy-create-prd {feature_name}",
        "TECHSPEC_PATH": f"{tasks_dir}/_techspec.md",
        "ORCHESTRATOR_LABEL": orchestrator_label,
        "CODEX_LABEL": codex_label,
        "CLAUDE_LABEL": claude_label,
        "TECHSPEC_COMMAND": f"/cy-create-techspec {feature_name}",
        "TASKS_COMMAND": f"/cy-create-tasks {feature_name}",
        "VALIDATE_COMMAND": validate_command,
        "START_COMMAND": start_command,
        "SKILL_ROOT": skill_root,
        "BOOT_PROMPTS_PATH": f"{skill_root}/assets/boot-prompts.md",
        "RUNTIME_CONTRACT_PATH": f"{skill_root}/references/runtime-contract.md",
        "CODEX_LAUNCH": codex_launch,
        "CLAUDE_LAUNCH": claude_launch,
    }

    for key in sorted(values):
        print(shell_assign(key, values[key]))

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

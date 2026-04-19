#!/usr/bin/env python3
"""Emit shell-safe launch variables for the smux-compozy-pairing skill."""

from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import sys

FEATURE_RE = re.compile(r"^[a-z][a-z0-9-]{0,63}$")
SANITIZED_START_ENV_KEYS = (
    "CODEX_THREAD_ID",
    "TMUX",
    "TMUX_PANE",
)


def shell_assign(name: str, value: str) -> str:
    return f"{name}={shlex.quote(value)}"


def env_unset_prefix(keys: tuple[str, ...]) -> list[str]:
    command = ["env"]
    for key in keys:
        command.extend(["-u", key])
    return command


def read_text(path: str) -> str:
    try:
        with open(path, "r", encoding="utf-8") as handle:
            return handle.read().strip()
    except OSError as err:
        print(f"failed to read {path}: {err}", file=sys.stderr)
        raise SystemExit(1) from err


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
    skill_root_abs = os.path.join(repo_root, skill_root)
    codex_developer_instructions_path = os.path.join(
        skill_root_abs, "assets", "codex-developer-instructions.md"
    )
    claude_append_system_prompt_path = os.path.join(
        skill_root_abs, "assets", "claude-append-system-prompt.md"
    )
    codex_developer_instructions = read_text(codex_developer_instructions_path)
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
            "-c",
            f"developer_instructions={json.dumps(codex_developer_instructions)}",
        ]
    )
    claude_launch = shlex.join(
        [
            "claude",
            "--model",
            args.claude_model,
            "--permission-mode",
            "bypassPermissions",
            "--append-system-prompt-file",
            claude_append_system_prompt_path,
        ]
    )
    validate_command = shlex.join(["compozy", "validate-tasks", "--name", feature_name])
    start_command = shlex.join(
        env_unset_prefix(SANITIZED_START_ENV_KEYS)
        + [
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
        "FEATURE_NAME": feature_name,
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
        "START_ENV_SANITIZE_KEYS": " ".join(SANITIZED_START_ENV_KEYS),
        "SKILL_ROOT": skill_root,
        "BOOT_PROMPTS_PATH": f"{skill_root}/assets/boot-prompts.md",
        "RUNTIME_CONTRACT_PATH": f"{skill_root}/references/runtime-contract.md",
        "CODEX_DEVELOPER_INSTRUCTIONS_PATH": codex_developer_instructions_path,
        "CLAUDE_APPEND_SYSTEM_PROMPT_PATH": claude_append_system_prompt_path,
        "CODEX_LAUNCH": codex_launch,
        "CLAUDE_LAUNCH": claude_launch,
    }

    for key in sorted(values):
        print(shell_assign(key, values[key]))

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

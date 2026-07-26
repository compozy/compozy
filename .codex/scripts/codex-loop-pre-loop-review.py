#!/usr/bin/env python3
"""Project-local codex-loop pre-continuation review hook for AGH.

The script is intentionally read-only for repository state. It prepares a
review prompt, delegates the review to Claude through Compozy, and prints the
compact result to stdout so codex-loop can inject it into the next continuation
prompt.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any


sys.dont_write_bytecode = True

HEADER = "AGH_PRE_LOOP_REVIEW"
DEFAULT_COMPOZY_TIMEOUT = "8m"
SCRIPT_RELATIVE = Path(".codex/scripts/codex-loop-pre-loop-review.py")
STATE_IO_RELATIVE = Path(".agents/skills/cy-loop-tasks/scripts/_state_io.py")
CY_MARKERS = ("$cy-loop-tasks", "/cy-loop-tasks")
CY_ACTION_PATTERNS = (
    re.compile(r"^executed task_\d+$"),
    re.compile(r"^slice .+"),
    re.compile(r"^qa-report produced$"),
    re.compile(r"^qa-execution produced$"),
    re.compile(r"^peer-review round .+"),
    re.compile(r"^round .+ closed$"),
    re.compile(r"^coderabbit round .+"),
)
TASK_REF_RE = re.compile(r"\.compozy/tasks/([A-Za-z0-9._-]+)")
TASK_EXECUTED_RE = re.compile(r"^executed (task_\d+)$")


@dataclass(frozen=True)
class ReviewTarget:
    mode: str
    target: str
    slug: str | None
    task_stem: str | None
    prompt: str
    warning: str | None = None


def main() -> int:
    args = parse_args()
    try:
        payload = read_payload(args.input)
        workspace = resolve_workspace(payload)
        target = select_review_target(payload, workspace)

        if is_dry_run():
            print(dry_run_output(target))
            return 0

        output = run_compozy_review(target, workspace)
        print(format_review_output(target, output))
        return 0
    except Exception as exc:  # noqa: BLE001 - hook must not break continuation.
        print(f"{HEADER}\nMode: unavailable\nTarget: unknown\nVerdict: FIX\n")
        print("Blockers:")
        print(f"- pre_loop_continue review hook failed before Claude review: {exc}")
        print("\nSuggested next-round guidance:")
        print("- Continue the loop, but manually inspect the latest implementation before stopping again.")
        return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", help="codex-loop pre_loop_continue JSON input file")
    return parser.parse_args()


def read_payload(input_path: str | None) -> dict[str, Any]:
    if input_path:
        raw = Path(input_path).read_text(encoding="utf-8")
    else:
        raw = sys.stdin.read()
    if not raw.strip():
        raise ValueError("empty pre_loop_continue payload")
    payload = json.loads(raw)
    if not isinstance(payload, dict):
        raise ValueError("pre_loop_continue payload must be a JSON object")
    return payload


def resolve_workspace(payload: dict[str, Any]) -> Path:
    candidates = [
        payload.get("workspace_root"),
        os.environ.get("WORKSPACE_ROOT"),
        os.environ.get("CODEX_LOOP_PRE_LOOP_WORKSPACE_ROOT"),
        payload.get("cwd"),
        os.getcwd(),
    ]
    for item in candidates:
        text = str(item or "").strip()
        if not text:
            continue
        candidate = Path(text).expanduser().resolve()
        if candidate.exists():
            return candidate
    return Path.cwd().resolve()


def select_review_target(payload: dict[str, Any], workspace: Path) -> ReviewTarget:
    cy_target = build_cy_target(payload, workspace)
    if cy_target is not None:
        return cy_target
    return build_generic_target(payload, workspace, None)


def build_cy_target(payload: dict[str, Any], workspace: Path) -> ReviewTarget | None:
    loop = object_field(payload, "loop")
    prompt_text = joined_text(
        loop.get("task_prompt"),
        loop.get("activation_prompt"),
        payload.get("continuation_reason"),
        payload.get("latest_assistant_message"),
    )
    prompt_has_marker = any(marker in prompt_text for marker in CY_MARKERS)
    prompt_has_task_ref = TASK_REF_RE.search(prompt_text) is not None
    slug = find_slug(payload, workspace)
    state_path = workspace / ".compozy/tasks" / slug / "state.yaml" if slug else None
    if not prompt_has_marker and not prompt_has_task_ref:
        return None
    if not state_path or not state_path.exists():
        warning = "cy-loop-tasks task evidence was present, but no matching state.yaml was found."
        return build_generic_target(payload, workspace, warning)

    state = load_cy_state(workspace, state_path)
    latest = latest_cy_iteration(state)
    if latest is None:
        if prompt_has_marker:
            return build_generic_target(
                payload,
                workspace,
                f"cy-loop-tasks state exists at {relative(workspace, state_path)}, but no reviewable iteration was found.",
            )
        return None

    action = str(latest.get("action") or "").strip()
    task_match = TASK_EXECUTED_RE.match(action)
    if task_match:
        task_stem = task_match.group(1)
        task_path = workspace / ".compozy/tasks" / slug / f"{task_stem}.md"
        status = frontmatter_value(task_path, "status")
        if status != "completed":
            return build_generic_target(
                payload,
                workspace,
                f"cy-loop-tasks latest task target {task_stem} has status {status or 'missing'}, not completed.",
            )
        return build_cy_task_target(payload, workspace, slug, task_stem, state_path, latest)

    return build_cy_phase_target(payload, workspace, slug, state_path, latest)


def find_slug(payload: dict[str, Any], workspace: Path) -> str | None:
    loop = object_field(payload, "loop")
    prompt_text = joined_text(loop.get("task_prompt"), loop.get("activation_prompt"))
    for match in TASK_REF_RE.finditer(prompt_text):
        slug = match.group(1)
        if (workspace / ".compozy/tasks" / slug / "state.yaml").exists():
            return slug
    slug = str(loop.get("slug") or "").strip()
    if slug and (workspace / ".compozy/tasks" / slug / "state.yaml").exists():
        return slug
    return slug or None


def load_cy_state(workspace: Path, state_path: Path) -> dict[str, Any]:
    state_io_path = workspace / STATE_IO_RELATIVE
    if not state_io_path.exists():
        raise ValueError(f"missing cy-loop-tasks state parser: {relative(workspace, state_io_path)}")
    spec = importlib.util.spec_from_file_location("cy_state_io", state_io_path)
    if spec is None or spec.loader is None:
        raise ValueError(f"cannot load cy-loop-tasks state parser: {state_io_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    state = module.load(state_path)
    if not isinstance(state, dict):
        raise ValueError(f"cy-loop-tasks state is not a mapping: {relative(workspace, state_path)}")
    return state


def latest_cy_iteration(state: dict[str, Any]) -> dict[str, Any] | None:
    for item in reversed(list(state.get("iterations") or [])):
        if not isinstance(item, dict):
            continue
        action = str(item.get("action") or "").strip()
        outcome = str(item.get("outcome") or "").strip()
        if outcome != "completed":
            continue
        if any(pattern.match(action) for pattern in CY_ACTION_PATTERNS):
            return item
    return None


def build_cy_task_target(
    payload: dict[str, Any],
    workspace: Path,
    slug: str,
    task_stem: str,
    state_path: Path,
    latest: dict[str, Any],
) -> ReviewTarget:
    slug_dir = workspace / ".compozy/tasks" / slug
    task_path = slug_dir / f"{task_stem}.md"
    task_memory = slug_dir / "memory" / f"{task_stem}.md"
    shared_memory = slug_dir / "memory" / "MEMORY.md"
    prompt = "\n\n".join(
        [
            review_preamble("cy-loop-tasks-task", f"{slug}/{task_stem}"),
            payload_context(payload),
            "## cy-loop-tasks latest iteration\n\n" + json.dumps(latest, indent=2, sort_keys=True),
            file_section(workspace, task_path, 60000),
            file_section(workspace, task_memory, 30000),
            file_section(workspace, shared_memory, 45000),
            file_section(workspace, state_path, 25000),
            git_context(workspace, include_diff=False),
            output_contract(),
        ]
    )
    return ReviewTarget(
        mode="cy-loop-tasks-task",
        target=f"{slug}/{task_stem}",
        slug=slug,
        task_stem=task_stem,
        prompt=prompt,
    )


def build_cy_phase_target(
    payload: dict[str, Any],
    workspace: Path,
    slug: str,
    state_path: Path,
    latest: dict[str, Any],
) -> ReviewTarget:
    action = str(latest.get("action") or "phase").strip()
    mode = classify_cy_phase(action)
    slug_dir = workspace / ".compozy/tasks" / slug
    prompt = "\n\n".join(
        [
            review_preamble(mode, f"{slug}: {action}"),
            payload_context(payload),
            "## cy-loop-tasks latest iteration\n\n" + json.dumps(latest, indent=2, sort_keys=True),
            file_section(workspace, slug_dir / "memory" / "MEMORY.md", 45000),
            file_section(workspace, state_path, 30000),
            git_context(workspace, include_diff=False),
            output_contract(),
        ]
    )
    return ReviewTarget(mode=mode, target=f"{slug}: {action}", slug=slug, task_stem=None, prompt=prompt)


def classify_cy_phase(action: str) -> str:
    if action.startswith("slice "):
        return "cy-loop-tasks-slice"
    return "cy-loop-tasks-phase"


def build_generic_target(
    payload: dict[str, Any],
    workspace: Path,
    warning: str | None,
) -> ReviewTarget:
    target = generic_target_name(payload)
    prompt = "\n\n".join(
        [
            review_preamble("generic", target),
            warning_section(warning),
            payload_context(payload),
            git_context(workspace, include_diff=True),
            output_contract(),
        ]
    )
    return ReviewTarget(mode="generic", target=target, slug=None, task_stem=None, prompt=prompt, warning=warning)


def generic_target_name(payload: dict[str, Any]) -> str:
    loop = object_field(payload, "loop")
    name = str(loop.get("name") or "").strip()
    slug = str(loop.get("slug") or "").strip()
    if name and slug and name != slug:
        return f"{name} ({slug})"
    return name or slug or "active codex-loop implementation"


def review_preamble(mode: str, target: str) -> str:
    return f"""You are Claude Code running as an independent AGH pre-continuation reviewer.

Review mode: {mode}
Review target: {target}

This is a read-only review. Do not modify files, run formatters, run codegen, apply patches, commit, or execute destructive git commands. Inspect the workspace as needed and report only review findings.

Prioritize concrete correctness risks: real bugs, regressions, incomplete requirements, missing validation evidence, broken integration points, unsafe shortcuts, and inconsistencies between the claimed implementation and repository state. Avoid style nits unless they hide a functional issue."""


def payload_context(payload: dict[str, Any]) -> str:
    loop = object_field(payload, "loop")
    stop = object_field(payload, "stop")
    fields = {
        "session_id": payload.get("session_id"),
        "workspace_root": payload.get("workspace_root"),
        "cwd": payload.get("cwd"),
        "loop_name": loop.get("name"),
        "loop_slug": loop.get("slug"),
        "loop_limit_mode": loop.get("limit_mode"),
        "continuation_reason": payload.get("continuation_reason"),
        "task_prompt": loop.get("task_prompt"),
        "activation_prompt": loop.get("activation_prompt"),
        "loop_last_assistant_message": loop.get("last_assistant_message"),
        "stop_last_assistant_message": stop.get("last_assistant_message"),
    }
    lines = ["## codex-loop payload context"]
    for key, value in fields.items():
        text = str(value or "").strip()
        if not text:
            continue
        lines.append(f"\n### {key}\n\n{truncate(text, 20000)}")
    return "\n".join(lines)


def git_context(workspace: Path, include_diff: bool) -> str:
    sections = [
        "## Repository evidence",
        command_section(workspace, ["git", "status", "--short"], 12000),
        command_section(workspace, ["git", "diff", "--stat"], 12000),
        command_section(workspace, ["git", "diff", "--cached", "--stat"], 12000),
        command_section(workspace, ["git", "diff", "--name-only", "HEAD", "--"], 12000),
    ]
    if include_diff:
        sections.append(bounded_diff(workspace))
    return "\n\n".join(section for section in sections if section.strip())


def command_section(workspace: Path, command: list[str], limit: int) -> str:
    title = " ".join(command)
    result = run_quiet(command, workspace, timeout=20)
    if result is None:
        return f"### `{title}`\n\n(command unavailable)"
    output = result.strip() or "(no output)"
    return f"### `{title}`\n\n```text\n{truncate(output, limit)}\n```"


def bounded_diff(workspace: Path) -> str:
    name_result = run_quiet(["git", "diff", "--name-only", "HEAD", "--"], workspace, timeout=20)
    if not name_result:
        return "### Bounded tracked diff\n\n```text\n(no tracked diff against HEAD)\n```"
    files = [line.strip() for line in name_result.splitlines() if line.strip()]
    if not files:
        return "### Bounded tracked diff\n\n```text\n(no tracked diff against HEAD)\n```"
    command = ["git", "diff", "--no-ext-diff", "--", *files[:40]]
    result = run_quiet(command, workspace, timeout=30)
    if result is None:
        return "### Bounded tracked diff\n\n```text\n(diff unavailable)\n```"
    return f"### Bounded tracked diff\n\n```diff\n{truncate(result.strip() or '(empty diff)', 70000)}\n```"


def output_contract() -> str:
    return f"""## Required output contract

Return concise Markdown and start exactly with:

{HEADER}
Mode: <cy-loop-tasks-task|cy-loop-tasks-slice|cy-loop-tasks-phase|generic>
Target: <review target>
Verdict: <PASS|FIX>
Confidence: <0..1>

Then include these sections:

Blockers:
- Use "None" only when there is no concrete blocker.

Risks:
- List important non-blocking risks, or "None".

Suggested next-round guidance:
- If Verdict: FIX, give concrete instructions the next Codex turn must do before stopping again.
- If Verdict: PASS, say what to continue with next.

Evidence:
- Cite concrete files, symbols, commands, task artifacts, or absence of evidence.

Do not claim you changed files. Do not tell Codex to run destructive git commands."""


def warning_section(warning: str | None) -> str:
    if not warning:
        return ""
    return f"## Hook mode warning\n\n{warning}"


def run_compozy_review(target: ReviewTarget, workspace: Path) -> str:
    prompt_path: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            prefix="agh-pre-loop-review-",
            suffix=".md",
            delete=False,
        ) as prompt_file:
            prompt_file.write(target.prompt)
            prompt_path = Path(prompt_file.name)
        command = [
            "compozy",
            "exec",
            "--ide",
            "claude",
            "--model",
            "opus",
            "--access-mode",
            "default",
            "--timeout",
            DEFAULT_COMPOZY_TIMEOUT,
            "--prompt-file",
            str(prompt_path),
        ]
        completed = subprocess.run(
            command,
            cwd=workspace,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=540,
            check=False,
        )
        if completed.returncode != 0:
            return unavailable_output(
                target,
                f"compozy exec exited {completed.returncode}: {truncate(completed.stderr.strip(), 2000)}",
            )
        output = completed.stdout.strip()
        if not output:
            return unavailable_output(target, "compozy exec returned empty stdout")
        return output
    except subprocess.TimeoutExpired:
        return unavailable_output(target, f"compozy exec timed out after {DEFAULT_COMPOZY_TIMEOUT}")
    finally:
        if prompt_path is not None:
            try:
                prompt_path.unlink(missing_ok=True)
            except OSError:
                pass


def format_review_output(target: ReviewTarget, output: str) -> str:
    text = output.strip()
    if text.startswith(HEADER):
        return text
    return "\n".join(
        [
            HEADER,
            f"Mode: {target.mode}",
            f"Target: {target.target}",
            "Verdict: FIX",
            "Confidence: 0.50",
            "",
            "Blockers:",
            "- Claude review returned output that did not follow the required AGH_PRE_LOOP_REVIEW contract. Treat the raw output as advisory and inspect before stopping again.",
            "",
            "Risks:",
            "- Review output contract drift can hide actionable blockers from the continuation prompt.",
            "",
            "Suggested next-round guidance:",
            "- Read the raw Claude output below and either fix any concrete issues or rerun the review manually.",
            "",
            "Evidence:",
            "- The hook had to wrap Claude output because the required header was missing.",
            "",
            "Raw Claude output:",
            truncate(text, 9000),
        ]
    )


def unavailable_output(target: ReviewTarget, reason: str) -> str:
    return "\n".join(
        [
            HEADER,
            f"Mode: {target.mode}",
            f"Target: {target.target}",
            "Verdict: FIX",
            "Confidence: 0.40",
            "",
            "Blockers:",
            f"- Claude review was unavailable: {reason}",
            "",
            "Risks:",
            "- The next continuation lacks independent review evidence for the latest implementation.",
            "",
            "Suggested next-round guidance:",
            "- Continue the loop, but manually inspect the latest implementation and validation evidence before stopping again.",
            "",
            "Evidence:",
            "- The project pre_loop_continue hook reached the review step but did not obtain a valid Claude review.",
        ]
    )


def dry_run_output(target: ReviewTarget) -> str:
    return "\n".join(
        [
            HEADER,
            f"Mode: {target.mode}",
            f"Target: {target.target}",
            "Verdict: PASS",
            "Confidence: 1.00",
            "",
            "Blockers:",
            "- None. Dry-run mode did not call Claude.",
            "",
            "Risks:",
            "- Dry-run output proves selection logic only; it is not an implementation review.",
            "",
            "Suggested next-round guidance:",
            "- Disable CODEX_LOOP_REVIEW_DRY_RUN for live Claude review.",
            "",
            "Evidence:",
            f"- Would run: compozy exec --ide claude --model opus --access-mode default --timeout {DEFAULT_COMPOZY_TIMEOUT} --prompt-file <temp-prompt>",
            f"- Prompt bytes: {len(target.prompt.encode('utf-8'))}",
            f"- Script: {SCRIPT_RELATIVE}",
        ]
    )


def is_dry_run() -> bool:
    return os.environ.get("CODEX_LOOP_REVIEW_DRY_RUN") == "1"


def object_field(payload: dict[str, Any], key: str) -> dict[str, Any]:
    value = payload.get(key)
    if isinstance(value, dict):
        return value
    return {}


def frontmatter_value(path: Path, key: str) -> str | None:
    if not path.exists():
        return None
    text = path.read_text(encoding="utf-8", errors="replace")
    if not text.startswith("---\n"):
        return None
    end = text.find("\n---", 4)
    if end < 0:
        return None
    for raw_line in text[4:end].splitlines():
        if ":" not in raw_line:
            continue
        raw_key, _, raw_value = raw_line.partition(":")
        if raw_key.strip() == key:
            return raw_value.strip().strip("'\"")
    return None


def file_section(workspace: Path, path: Path, limit: int) -> str:
    label = relative(workspace, path)
    if not path.exists():
        return f"## File: `{label}`\n\n(file missing)"
    if path.is_dir():
        return f"## File: `{label}`\n\n(path is a directory)"
    text = path.read_text(encoding="utf-8", errors="replace")
    return f"## File: `{label}`\n\n```text\n{truncate(text, limit)}\n```"


def run_quiet(command: list[str], cwd: Path, timeout: int) -> str | None:
    try:
        completed = subprocess.run(
            command,
            cwd=cwd,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return None
    if completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip()
        return f"command exited {completed.returncode}: {detail}"
    return completed.stdout


def joined_text(*items: Any) -> str:
    return "\n".join(str(item or "") for item in items)


def truncate(text: str, limit: int) -> str:
    if len(text.encode("utf-8")) <= limit:
        return text
    encoded = text.encode("utf-8")[:limit]
    clipped = encoded.decode("utf-8", errors="ignore")
    return clipped + f"\n[truncated after {limit} bytes]"


def relative(root: Path, path: Path) -> str:
    try:
        return str(path.resolve().relative_to(root.resolve()))
    except ValueError:
        return str(path)


if __name__ == "__main__":
    raise SystemExit(main())

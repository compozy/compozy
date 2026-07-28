"""
Minimal block-YAML reader/writer for cy-implement-spec's state.yaml.

Both ends (init-state.py, update-state.py) use this module. The format
emitted by ``dump`` is the only format ``load`` is required to parse;
we own both sides, so the YAML subset is deliberately strict:

- 2-space indentation, never tabs
- strings always double-quoted; ``\\`` and ``"`` escaped
- empty list ``[]``; list-of-scalars in flow style ``[a, b, c]``
- list-of-dicts in block style with ``- key: value`` for the first key
- bool ``true``/``false``; null literal ``null``; integers bare

This is NOT a general YAML parser. It is sufficient for the schema in
``references/state-schema.md`` and nothing else.
"""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


# ---------- emit ----------------------------------------------------------


def _scalar(value: Any) -> str:
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, str):
        return json.dumps(value, ensure_ascii=False)
    raise TypeError(f"unsupported scalar type: {type(value).__name__}")


def _flow_list(items: list[Any]) -> str:
    if not items:
        return "[]"
    return "[" + ", ".join(_scalar(x) for x in items) + "]"


def dump(state: dict, path: Path) -> None:
    out: list[str] = []
    out.append(
        "# cy-implement-spec state.yaml -- managed by "
        ".agents/skills/cy-implement-spec/scripts/update-state.py. "
        "Do not hand-edit."
    )
    for key in (
        "slug",
        "created_at",
        "last_updated",
        "iteration",
        "goal_signature",
    ):
        out.append(f"{key}: {_scalar(state.get(key))}")

    progress = state.get("progress", {})
    out.append("")
    out.append("progress:")
    out.append(
        "  implementation_complete: "
        f"{_scalar(progress.get('implementation_complete', False))}"
    )
    criteria = progress.get("criteria", [])
    if not criteria:
        out.append("  criteria: []")
    else:
        out.append("  criteria:")
        for item in criteria:
            out.append(f"    - text: {_scalar(item.get('text', ''))}")
            out.append(f"      status: {_scalar(item.get('status', 'pending'))}")
            out.append(f"      iteration: {_scalar(item.get('iteration', 0))}")

    qa = state.get("qa", {})
    out.append("")
    out.append("qa:")
    out.append(f"  report_done: {_scalar(qa.get('report_done', False))}")
    out.append(f"  execution_done: {_scalar(qa.get('execution_done', False))}")

    review = state.get("review", {})
    out.append("")
    out.append("review:")
    out.append(f"  rounds: {_scalar(review.get('rounds', 0))}")
    out.append(f"  last_verdict: {_scalar(review.get('last_verdict'))}")
    out.append(f"  ship: {_scalar(review.get('ship', False))}")

    verify = state.get("verify", {})
    out.append("")
    out.append("verify:")
    out.append(f"  last_run: {_scalar(verify.get('last_run'))}")
    out.append(f"  last_status: {_scalar(verify.get('last_status'))}")

    iterations = state.get("iterations", [])
    out.append("")
    if not iterations:
        out.append("iterations: []")
    else:
        out.append("iterations:")
        for it in iterations:
            out.append(f"  - n: {_scalar(it.get('n', 0))}")
            out.append(f"    timestamp: {_scalar(it.get('timestamp', ''))}")
            out.append(f"    phase: {_scalar(it.get('phase', ''))}")
            out.append(f"    action: {_scalar(it.get('action', ''))}")
            out.append(f"    outcome: {_scalar(it.get('outcome', ''))}")
            out.append(f"    memory_written: {_flow_list(it.get('memory_written', []))}")
            out.append(f"    blockers: {_flow_list(it.get('blockers', []))}")

    Path(path).write_text("\n".join(out) + "\n", encoding="utf-8")


# ---------- parse ---------------------------------------------------------


def _parse_scalar(token: str) -> Any:
    token = token.strip()
    if token == "" or token == "null":
        return None
    if token == "true":
        return True
    if token == "false":
        return False
    if token.startswith('"') and token.endswith('"'):
        value = json.loads(token)
        if not isinstance(value, str):
            raise ValueError(f"expected string scalar, got: {token!r}")
        return value
    if token.lstrip("-").isdigit():
        return int(token)
    raise ValueError(f"cannot parse scalar: {token!r}")


def _parse_flow_list(token: str) -> list[Any]:
    token = token.strip()
    if token == "[]":
        return []
    if not (token.startswith("[") and token.endswith("]")):
        raise ValueError(f"expected flow list, got: {token!r}")
    inner = token[1:-1]
    items: list[Any] = []
    buf = ""
    in_str = False
    escape = False
    for ch in inner:
        if escape:
            buf += ch
            escape = False
            continue
        if in_str:
            if ch == "\\":
                buf += ch
                escape = True
            elif ch == '"':
                buf += ch
                in_str = False
            else:
                buf += ch
            continue
        if ch == '"':
            buf += ch
            in_str = True
        elif ch == ",":
            items.append(_parse_scalar(buf))
            buf = ""
        else:
            buf += ch
    if buf.strip():
        items.append(_parse_scalar(buf))
    return items


def _indent_of(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def _split_kv(stripped: str) -> tuple[str, str]:
    key, _, val = stripped.partition(":")
    return key.strip(), val.strip()


def load(path: Path) -> dict:
    text = Path(path).read_text(encoding="utf-8")
    raw_lines = text.splitlines()
    lines: list[tuple[int, str]] = []
    for raw in raw_lines:
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        lines.append((_indent_of(raw), raw.rstrip()))
    state: dict = {}
    i = 0
    while i < len(lines):
        indent, line = lines[i]
        if indent != 0:
            raise ValueError(f"unexpected indentation at top level: {line!r}")
        stripped = line.strip()
        if stripped.endswith(":"):
            key = stripped[:-1].strip()
            i, sub = _parse_block(lines, i + 1, base_indent=2)
            state[key] = sub
        else:
            key, val = _split_kv(stripped)
            if val.startswith("["):
                state[key] = _parse_flow_list(val)
            else:
                state[key] = _parse_scalar(val)
            i += 1
    return state


def _parse_block(
    lines: list[tuple[int, str]], start: int, base_indent: int
) -> tuple[int, Any]:
    """Parse the lines belonging to one block (mapping or list of mappings).

    Returns the index of the next unconsumed line and the parsed value.
    """
    if start >= len(lines):
        return start, {}
    first_indent, first_line = lines[start]
    if first_indent < base_indent:
        return start, {}
    stripped_first = first_line.strip()
    if stripped_first.startswith("- "):
        return _parse_list_of_maps(lines, start, base_indent)
    return _parse_map(lines, start, base_indent)


def _parse_map(
    lines: list[tuple[int, str]], start: int, base_indent: int
) -> tuple[int, dict]:
    result: dict = {}
    i = start
    while i < len(lines):
        indent, line = lines[i]
        if indent < base_indent:
            break
        if indent > base_indent:
            raise ValueError(f"unexpected deeper indent at line: {line!r}")
        stripped = line.strip()
        if stripped.endswith(":"):
            key = stripped[:-1].strip()
            i, sub = _parse_block(lines, i + 1, base_indent + 2)
            result[key] = sub
            continue
        key, val = _split_kv(stripped)
        if val.startswith("["):
            result[key] = _parse_flow_list(val)
        else:
            result[key] = _parse_scalar(val)
        i += 1
    return i, result


def _parse_list_of_maps(
    lines: list[tuple[int, str]], start: int, base_indent: int
) -> tuple[int, list[dict]]:
    items: list[dict] = []
    i = start
    while i < len(lines):
        indent, line = lines[i]
        if indent < base_indent:
            break
        if indent > base_indent:
            raise ValueError(f"unexpected deeper indent at list line: {line!r}")
        stripped = line.strip()
        if not stripped.startswith("- "):
            break
        first_kv = stripped[2:].strip()
        item: dict = {}
        key, val = _split_kv(first_kv)
        if val.startswith("["):
            item[key] = _parse_flow_list(val)
        else:
            item[key] = _parse_scalar(val)
        i += 1
        # subsequent fields are at indent + 2
        field_indent = base_indent + 2
        while i < len(lines):
            ni, nline = lines[i]
            if ni < field_indent:
                break
            if ni > field_indent:
                raise ValueError(
                    f"unexpected deeper indent under list item: {nline!r}"
                )
            ns = nline.strip()
            if ns.startswith("- "):
                break
            k, v = _split_kv(ns)
            if v.startswith("["):
                item[k] = _parse_flow_list(v)
            else:
                item[k] = _parse_scalar(v)
            i += 1
        items.append(item)
    return i, items

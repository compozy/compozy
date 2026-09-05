#!/usr/bin/env python3
"""Decode persisted Warp agent tasks without a protobuf runtime.

Warp stores each ``warp.multi_agent.v1.Task`` as a protobuf blob.  The skill
doctor only needs the task/message envelope, visible text, tool activity,
working-directory context, and skill references, so this module implements
that deliberately small wire-format subset.  Unknown fields and message types
are skipped, which keeps the decoder forward-compatible with additive schema
changes.
"""

import json
from datetime import datetime, timezone


class ProtobufDecodeError(ValueError):
    """Raised when a protobuf blob is malformed or uses an unsupported wire type."""


TOOL_CALL_NAMES = {
    2: "run_shell_command",
    3: "search_codebase",
    4: "server",
    5: "read_files",
    6: "apply_file_diffs",
    7: "suggest_plan",
    8: "suggest_create_plan",
    9: "grep",
    10: "file_glob",
    11: "read_mcp_resource",
    12: "call_mcp_tool",
    13: "write_to_long_running_shell_command",
    14: "suggest_new_conversation",
    15: "file_glob_v2",
    16: "suggest_prompt",
    17: "open_code_review",
    18: "init_project",
    19: "subagent",
    20: "read_documents",
    21: "edit_documents",
    22: "create_documents",
    23: "read_shell_command_output",
    24: "use_computer",
    25: "insert_review_comments",
    26: "read_skill",
    27: "request_computer_use",
    28: "fetch_conversation",
    30: "send_message_to_agent",
    31: "transfer_shell_command_control_to_user",
    32: "ask_user_question",
    34: "upload_file_artifact",
    35: "run_agents",
    36: "wait_for_events",
    37: "start_recording",
    38: "stop_recording",
}

TOOL_RESULT_NAMES = {
    2: "run_shell_command",
    3: "search_codebase",
    4: "server",
    5: "read_files",
    6: "apply_file_diffs",
    7: "suggest_plan",
    8: "suggest_create_plan",
    9: "grep",
    10: "file_glob",
    14: "cancel",
    15: "read_mcp_resource",
    16: "call_mcp_tool",
    17: "write_to_long_running_shell_command",
    18: "suggest_new_conversation",
    19: "file_glob_v2",
    20: "suggest_prompt",
    21: "open_code_review",
    22: "init_project",
    23: "subagent",
    24: "read_documents",
    25: "edit_documents",
    26: "create_documents",
    27: "read_shell_command_output",
    28: "use_computer",
    29: "insert_review_comments",
    30: "read_skill",
    31: "request_computer_use",
    32: "fetch_conversation",
    34: "send_message_to_agent",
    35: "transfer_shell_command_control_to_user",
    36: "ask_user_question",
    38: "upload_file_artifact",
    39: "run_agents",
    40: "wait_for_events",
    41: "start_recording",
    42: "stop_recording",
}

MESSAGE_TYPES = {
    2: "user_query",
    3: "agent_output",
    4: "tool_call",
    5: "tool_call_result",
    6: "server_event",
    9: "system_query",
    10: "update_todos",
    15: "agent_reasoning",
    16: "summarization",
    17: "code_review",
    18: "update_review_comments",
    19: "web_search",
    20: "web_fetch",
    21: "debug_output",
    22: "artifact_event",
    23: "invoke_skill",
    24: "messages_received_from_agents",
    25: "model_used",
    26: "events_from_agents",
    27: "passive_suggestion_result",
    28: "orchestration_config_snapshot",
}


def _read_varint(data, pos):
    value = 0
    shift = 0
    while pos < len(data) and shift < 70:
        byte = data[pos]
        pos += 1
        value |= (byte & 0x7F) << shift
        if not byte & 0x80:
            return value, pos
        shift += 7
    raise ProtobufDecodeError("unterminated protobuf varint")


def parse_fields(data):
    """Return ``{field_number: [(wire_type, value), ...]}`` for one message."""
    fields = {}
    pos = 0
    while pos < len(data):
        tag, pos = _read_varint(data, pos)
        field_number = tag >> 3
        wire_type = tag & 0x07
        if field_number == 0:
            raise ProtobufDecodeError("protobuf field number cannot be zero")

        if wire_type == 0:
            value, pos = _read_varint(data, pos)
        elif wire_type == 1:
            end = pos + 8
            if end > len(data):
                raise ProtobufDecodeError("truncated fixed64 field")
            value = data[pos:end]
            pos = end
        elif wire_type == 2:
            length, pos = _read_varint(data, pos)
            end = pos + length
            if end > len(data):
                raise ProtobufDecodeError("truncated length-delimited field")
            value = data[pos:end]
            pos = end
        elif wire_type == 5:
            end = pos + 4
            if end > len(data):
                raise ProtobufDecodeError("truncated fixed32 field")
            value = data[pos:end]
            pos = end
        else:
            raise ProtobufDecodeError(f"unsupported protobuf wire type {wire_type}")
        fields.setdefault(field_number, []).append((wire_type, value))
    return fields


def _bytes_values(fields, number):
    return [value for wire_type, value in fields.get(number, []) if wire_type == 2]


def _first_bytes(fields, number):
    values = _bytes_values(fields, number)
    return values[0] if values else None


def _first_varint(fields, number, default=0):
    for wire_type, value in fields.get(number, []):
        if wire_type == 0:
            return value
    return default


def _text(data):
    if data is None:
        return ""
    try:
        return data.decode("utf-8")
    except UnicodeDecodeError:
        return ""


def _first_text(fields, number):
    return _text(_first_bytes(fields, number))


def _is_readable_text(value):
    if not value:
        return False
    try:
        text = value.decode("utf-8")
    except UnicodeDecodeError:
        return False
    return all(char in "\n\r\t" or ord(char) >= 32 for char in text)


def _extract_payload_values(data, prefix="", depth=0, max_depth=5):
    """Extract readable leaves from an otherwise opaque tool payload."""
    if depth > max_depth:
        return []
    try:
        fields = parse_fields(data)
    except ProtobufDecodeError:
        return []

    values = []
    for number in sorted(fields):
        key = f"{prefix}{number}"
        for wire_type, value in fields[number]:
            if wire_type == 0:
                values.append((key, value))
            elif wire_type == 2:
                if _is_readable_text(value):
                    values.append((key, _text(value)))
                else:
                    values.extend(
                        _extract_payload_values(
                            value,
                            prefix=f"{key}.",
                            depth=depth + 1,
                            max_depth=max_depth,
                        )
                    )
    return values


def summarize_payload(data):
    """Render opaque tool payload fields into deterministic compact JSON."""
    values = _extract_payload_values(data)
    rendered = {}
    for key, value in values:
        if key in rendered:
            current = rendered[key]
            if not isinstance(current, list):
                current = [current]
            current.append(value)
            rendered[key] = current
        else:
            rendered[key] = value
    return json.dumps(rendered, ensure_ascii=False, sort_keys=True)


def _decode_timestamp(data):
    if not data:
        return None, None
    fields = parse_fields(data)
    seconds = _first_varint(fields, 1, 0)
    nanos = _first_varint(fields, 2, 0)
    try:
        timestamp = datetime.fromtimestamp(
            seconds + nanos / 1_000_000_000,
            tz=timezone.utc,
        ).isoformat()
    except (OSError, OverflowError, ValueError):
        timestamp = None
    return timestamp, (seconds, nanos)


def _decode_directory_from_context(data):
    if not data:
        return None
    context = parse_fields(data)
    directory_data = _first_bytes(context, 1)
    if not directory_data:
        return None
    directory = parse_fields(directory_data)
    return _first_text(directory, 1) or None


def _decode_skill(data):
    """Decode ``Skill`` into its stable name/path identifiers."""
    if not data:
        return {}
    skill = parse_fields(data)
    descriptor_data = _first_bytes(skill, 1)
    if not descriptor_data:
        return {}
    descriptor = parse_fields(descriptor_data)
    return {
        "path": _first_text(descriptor, 1) or None,
        "name": _first_text(descriptor, 2) or None,
        "bundled_skill_id": _first_text(descriptor, 4) or None,
    }


def _decode_read_skill(data):
    fields = parse_fields(data)
    return {
        "path": _first_text(fields, 1) or None,
        "bundled_skill_id": _first_text(fields, 2) or None,
        "name": _first_text(fields, 3) or None,
    }


def _decode_user_query(data):
    fields = parse_fields(data)
    return {
        "text": _first_text(fields, 1),
        "cwd": _decode_directory_from_context(_first_bytes(fields, 2)),
    }


def _decode_tool_call(data):
    fields = parse_fields(data)
    tool_number = next((number for number in TOOL_CALL_NAMES if number in fields), None)
    name = TOOL_CALL_NAMES.get(tool_number, "unknown")
    payload = _first_bytes(fields, tool_number) if tool_number else b""
    decoded = {
        "tool_call_id": _first_text(fields, 1),
        "name": name,
        "payload": summarize_payload(payload or b""),
        "skill": None,
    }
    if name == "read_skill" and payload:
        decoded["skill"] = _decode_read_skill(payload)
    return decoded


def _decode_tool_result(data):
    fields = parse_fields(data)
    result_number = next((number for number in TOOL_RESULT_NAMES if number in fields), None)
    payload = _first_bytes(fields, result_number) if result_number else b""
    return {
        "tool_call_id": _first_text(fields, 1),
        "name": TOOL_RESULT_NAMES.get(result_number, "unknown"),
        "payload": summarize_payload(payload or b""),
        "cwd": _decode_directory_from_context(_first_bytes(fields, 11)),
    }


def _decode_message(data):
    fields = parse_fields(data)
    message_number = next((number for number in MESSAGE_TYPES if number in fields), None)
    kind = MESSAGE_TYPES.get(message_number, "unknown")
    payload = _first_bytes(fields, message_number) if message_number else b""
    timestamp, order_key = _decode_timestamp(_first_bytes(fields, 14))
    decoded = {
        "id": _first_text(fields, 1),
        "kind": kind,
        "timestamp": timestamp,
        "order_key": order_key,
    }

    if kind == "user_query":
        decoded.update(_decode_user_query(payload))
    elif kind == "agent_output":
        decoded["text"] = _first_text(parse_fields(payload), 1)
    elif kind == "tool_call":
        decoded.update(_decode_tool_call(payload))
    elif kind == "tool_call_result":
        decoded.update(_decode_tool_result(payload))
    elif kind == "invoke_skill":
        invoke = parse_fields(payload)
        decoded["skill"] = _decode_skill(_first_bytes(invoke, 1))
        user_query_data = _first_bytes(invoke, 2)
        if user_query_data:
            decoded["user_query"] = _decode_user_query(user_query_data)
    return decoded


def decode_task(data):
    """Decode a persisted ``warp.multi_agent.v1.Task`` blob."""
    fields = parse_fields(data)
    dependencies_data = _first_bytes(fields, 3)
    parent_task_id = None
    if dependencies_data:
        parent_task_id = _first_text(parse_fields(dependencies_data), 1) or None
    return {
        "id": _first_text(fields, 1),
        "description": _first_text(fields, 2),
        "parent_task_id": parent_task_id,
        "messages": [_decode_message(value) for value in _bytes_values(fields, 5)],
    }

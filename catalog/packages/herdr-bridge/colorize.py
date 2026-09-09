#!/usr/bin/env python3
"""Render original CompozyOS session events and JSONL logs for herdr panes.

Preserve message whitespace while filtering infrastructure noise."""
import json
import re
import shutil
import sys

RESET = "\033[0m"

# Infrastructure noise does not explain agent activity.
SKIP_TYPES = {
    "usage",
    "session_stream_subscribed",
    "available_commands_update",
    "skill.shadowed",
    "skills.exposure.broken_detected",
}
SKIP_TYPE_PREFIXES = ("harness.",)

# Empty or redacted results add no useful output.
EMPTY_SUMMARIES = {"[REDACTED]", "", "null"}

# Tool labels arrive before the command itself.
# Combine both into one rendered line.
TOOL_LABEL = re.compile(r"^[A-Z][A-Za-z]{2,19}$")

TYPE_STYLE = {
    "agent_message": "1;97",
    "user_message": "33",
    "tool_call": "36",
    "tool_result": "2;36",
    "done": "1;32",
    "session_stopped": "35",
    "synthetic_reentry": "2;35",
    "coordinator.": "1;35",
    "hook.": "2;35",
    "memory.": "34",
    "transcript": "90",
    "runtime_progress": "2;37",
    "system": "2;37",
}

OUTCOME_STYLE = {
    "warning": "1;33",
    "failure": "1;31",
    "error": "1;31",
    "success": "1;32",
}


def terminal_safe(text):
    """Escape terminal controls while preserving printable text, tabs, and newlines."""
    return "".join(char if char.isprintable() or char in "\n\t"
                   else f"\\x{ord(char):02x}" for char in text)


def style_for(etype, outcome):
    """Select a trusted ANSI style for the event and outcome."""
    if outcome in OUTCOME_STYLE:
        return OUTCOME_STYLE[outcome]
    if etype in TYPE_STYLE:
        return TYPE_STYLE[etype]
    for prefix, code in TYPE_STYLE.items():
        if prefix.endswith(".") and etype.startswith(prefix):
            return code
    return "37"


def width():
    """Return the usable terminal width with a conservative fallback."""
    try:
        return max(60, shutil.get_terminal_size().columns) - 2
    except Exception:
        return 118


# Remove the working-directory prefix from command summaries.
CD_PREFIX = re.compile(r"^cd\s+\S+\s*(?:&&|;)\s*")


def flatten(summary):
    """Summarize a multiline command using its first nonempty line."""
    parts = [x.strip() for x in summary.replace("\r", "").split("\n") if x.strip()]
    if not parts:
        return ""
    head = CD_PREFIX.sub("", parts[0])
    return head + (" ⏎ …" if len(parts) > 1 else "")


def session_event(event):
    """Adapt original events without treating message text as log summaries."""
    content = event.get("content")
    if not isinstance(content, dict) or "sequence" not in event:
        return event
    etype = event.get("type")
    error = content.get("error") or content.get("tool_error")
    outcome = "error" if error or etype == "error" else ""
    if etype == "agent_message":
        summary = content.get("text", "")
    elif etype == "tool_result":
        # Original events include full tool results; preserve noise filtering.
        # Do not dump tool inputs or successful result bodies.
        summary = (content.get("error") or content.get("text") or
                   content.get("title") or "tool failed") if error else ""
    else:
        summary = next((content[field] for field in
                        ("text", "title", "error", "resource", "stop_reason", "tool_call_id")
                        if content.get(field)), "")
    return {**event, "summary": summary, "outcome": outcome}


class Renderer:
    """Render event fragments with per-session streaming and deduplication."""
    def __init__(self):
        """Initialize streaming, tool label, and repeated-line state."""
        self.last_text = ""         # Last rendered line for repeat updates.
        self.pending_label = None   # Tool label awaiting a command.
        self.streaming = False      # Inside an agent_message stream.
        self.stream_scope = None    # Keep sibling sessions and turns separate.
        self.last_key = None        # Last rendered event type and text.
        self.repeat = 1

    def out(self, text, key=None):
        """Emit trusted styled text and collapse repeated rendered lines."""
        if key is not None and key == self.last_key:
            # Rewrite the previous line with its repeat count.
            # Move up after the previous newline.
            # Clear and redraw the line with a counter.
            self.repeat += 1
            sys.stdout.write(f"\033[A\r\033[K{self.last_text} \033[2;90m×{self.repeat}{RESET}\n")
            sys.stdout.flush()
            return
        self.repeat = 1
        self.last_key = key
        self.last_text = text
        sys.stdout.write(text + "\n")
        sys.stdout.flush()

    def close_stream(self):
        """Finish the active message stream before a different event."""
        if self.streaming:
            sys.stdout.write("\n")
            sys.stdout.flush()
            self.streaming = False
            self.last_key = None

    def feed(self, d):
        """Render one event while preserving message fragment boundaries."""
        d = session_event(d)
        etype = terminal_safe(str(d.get("type") or "?"))
        if etype in SKIP_TYPES or etype.startswith(SKIP_TYPE_PREFIXES):
            return
        outcome = str(d.get("outcome") or "")
        summary = terminal_safe(str(d.get("summary") or ""))
        # Message deltas include spaces between words.
        # Command flattening would destroy message breaks and indentation.
        if etype != "agent_message":
            summary = flatten(summary)

        # Suppress empty successful tool results.
        if etype == "tool_result" and summary in EMPTY_SUMMARIES and outcome not in OUTCOME_STYLE:
            return

        # Hold the tool label for the following command.
        if etype == "tool_call" and TOOL_LABEL.match(summary):
            self.pending_label = summary
            return

        ts = terminal_safe(str(d.get("timestamp") or "")[11:19])
        code = style_for(etype, outcome)

        if etype == "agent_message":
            scope = (d.get("session_id"), d.get("turn_id"))
            if self.streaming and scope != self.stream_scope:
                self.close_stream()
            self.stream_scope = scope
            head = f"\033[2;37m{ts}{RESET} \033[{code}m{'msg':<14}{RESET} "
            if self.streaming:
                sys.stdout.write(summary)
            else:
                sys.stdout.write(head + summary)
                self.streaming = True
            sys.stdout.flush()
            return

        self.close_stream()

        label, label_len = "", 0
        if etype == "tool_call" and self.pending_label:
            label = f"\033[2;36m{self.pending_label}▸{RESET}"
            label_len = len(self.pending_label) + 1
        self.pending_label = None

        tag = etype if etype != "tool_call" else "tool"
        head = f"\033[2;37m{ts}{RESET} \033[{code}m{tag:<14}{RESET} {label}"
        room = width() - len(ts) - 17 - label_len
        body = summary if len(summary) <= room else summary[: room - 1] + "…"
        self.out(head + body, key=(etype, summary))


def main():
    """Render JSONL events and plain fallback lines from standard input."""
    r = Renderer()
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        if not line.startswith("{"):
            r.close_stream()
            r.out(f"\033[2;37m{terminal_safe(line)}{RESET}")
            continue
        try:
            r.feed(json.loads(line))
        except BrokenPipeError:
            raise
        except Exception as exc:
            diagnostic = terminal_safe(f"herdr-bridge: cannot render event: {type(exc).__name__}: {exc}")
            print(diagnostic, file=sys.stderr, flush=True)
    r.close_stream()


if __name__ == "__main__":
    try:
        main()
    except (KeyboardInterrupt, BrokenPipeError):
        pass

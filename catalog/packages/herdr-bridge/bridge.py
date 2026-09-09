#!/usr/bin/env python3
"""Reflect Compozy hook events into herdr rows; log failures without failing hooks."""
import calendar
import datetime
import json
import os
import re
import shlex
import sys
import time

from bridge_state import (AGENT_ID, SOURCE, SPOOL_DIR, Locked, close_row,
                          consolidate, drop_stale, herdr, load_map, log,
                          pane_alive, prune, row_key, save_map)
from bridge_loops import (LOOP_EVENTS, handle_loop, loop_tail_command,
                          reconcile_loops, watch_loops)

# Exclude internal spawned/dream sessions and synthetic reentry activity.
ALLOW_SESSION_TYPES = {"user", "system"}
SKIP_INPUT_CLASSES = {"synthetic_reentry"}
TERMINAL_EVENTS = {"session.post_stop", "agent.stopped", "agent.crashed"}
ATTENTION_CLASS_KEY = "class"
ATTENTION_BENIGN = {"none", "finished", ""}
EVENT_STATE = {
    "session.post_create": "idle",
    "turn.start": "working",
    "message.start": "working",
    "turn.end": "idle",
    "permission.request": "blocked",
    "permission.denied": "blocked",
    "permission.resolved": "working",
    "task.needs_attention": "blocked",
    "task.blocked": "blocked",
    "session.post_stop": "idle",
    "agent.stopped": "idle",
    "agent.crashed": "idle",
}


def tail_command(key):
    """Build the session event viewer command for one row."""
    here = os.path.dirname(os.path.abspath(__file__))
    reader = os.path.join(here, "tail.py")
    return f"python3 {shlex.quote(reader)} {shlex.quote(key)}\n"


def ensure_row(data, key, agent_name):
    """Return or create the agent row while holding the map lock."""
    entry = data.get(key)
    if entry and pane_alive(entry.get("pane_id", "")) is not False:
        return entry

    res = herdr("tab.create", {"label": f"cz:{agent_name}", "focus": False})
    if not res or "result" not in res:
        return None
    pane_id = res["result"]["root_pane"]["pane_id"]
    tab_id = res["result"]["tab"]["tab_id"]
    # Start a useful event viewer instead of leaving an empty shell.
    herdr("pane.send_text", {"pane_id": pane_id, "text": tail_command(key)})
    entry = {"pane_id": pane_id, "tab_id": tab_id, "agent": agent_name, "sessions": {}}
    data[key] = entry
    log(f"row created for {agent_name}: {pane_id}")
    return entry


def read_attention(payload):
    """Return attention state, or None for an unrecognized payload shape."""
    if ATTENTION_CLASS_KEY not in payload:
        return None
    cls = str(payload.get(ATTENTION_CLASS_KEY) or "").strip().lower()
    if cls in ATTENTION_BENIGN:
        return False
    # Unknown classes remain visible as attention.
    # Log them so the mapping can be updated from evidence.
    log(f"unknown attention class: {cls!r}")
    return True


def handle(payload):
    """Apply an eligible session or loop hook to its shared row."""
    event = payload.get("event") or ""
    if event in LOOP_EVENTS:
        return handle_loop(payload)
    state = EVENT_STATE.get(event)

    if event == "session.attention.changed":
        flag = read_attention(payload)
        if flag is None:
            # Unknown shape: log it without changing the row.
            log(f"unrecognized payload for {event}: {json.dumps(payload)[:2000]}")
            return
        state = "blocked" if flag else "idle"

    if not state:
        return
    if payload.get("session_type") not in ALLOW_SESSION_TYPES:
        return
    if payload.get("input_class") in SKIP_INPUT_CLASSES:
        return
    agent_name = payload.get("agent_name")
    if not agent_name:
        return
    session_id = payload.get("session_id") or "?"
    key = row_key(payload.get("workspace_id"), agent_name)

    with Locked():
        data = load_map()
        # Duplicate terminal hooks must not create or recreate a pane.
        # Both agent.stopped and session.post_stop can arrive.
        terminal = event in TERMINAL_EVENTS
        entry = data.get(key) if terminal else ensure_row(data, key, agent_name)
        if not entry:
            return
        sessions = entry.setdefault("sessions", {})
        if terminal:
            sessions.pop(session_id, None)
            if not sessions:
                close_row(data, key)
                save_map(data)
                return
        else:
            sessions[session_id] = {
                "state": state,
                "name": payload.get("session_name"),
                "type": payload.get("session_type"),
                "turn": payload.get("turn_id"),
                "ts": time.time(),
            }
        recent = drop_stale(sessions)
        row_state, active = consolidate(recent)
        if active and active[1].get("name"):
            entry["last_title"] = active[1]["name"]
        # Keep idle and quiet sessions until their terminal event.
        # Stopping one sibling must not close another sibling pane.
        live = sum(i.get("state") != "idle" for i in recent.values())
        last_title = entry.get("last_title")
        data[key] = entry
        save_map(data)
        pane_id = entry["pane_id"]

    seq = time.time_ns()
    active_id, active_info = active if active else (None, {})
    herdr("pane.report_agent", {
        "pane_id": pane_id, "source": SOURCE, "agent": AGENT_ID,
        "state": row_state, "seq": seq,
        "agent_session_id": active_id,
        "message": f"{event} · {payload.get('session_name') or ''}".strip(" ·"),
    })
    herdr("pane.report_metadata", {
        "pane_id": pane_id, "source": SOURCE, "seq": seq,
        "display_agent": AGENT_ID,
        "title": (active_info.get("name") if active_info else None) or last_title or agent_name,
        "tokens": {
            "cz_agent": agent_name,
            "cz_session": (active_id or "")[:24] or None,
            "cz_type": active_info.get("type") if active_info else None,
            "cz_live": str(live) if live else None,
        },
    })


def timestamp_order(value):
    """Sort RFC3339Nano timestamps exactly, including variable fractions/offsets."""
    match = re.fullmatch(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})", str(value or ""))
    if not match:
        return (0, 0)
    try:
        whole = datetime.datetime.fromisoformat(match[1] + match[3].replace("Z", "+00:00"))
        return (calendar.timegm(whole.utctimetuple()), int((match[2] or "").ljust(9, "0")))
    except ValueError:
        return (0, 0)


def drain_spool():
    """Drain complete spool files in timestamp order under a separate lock."""
    if not os.path.isdir(SPOOL_DIR):
        return 0
    handled = 0
    with Locked(".drain-lock"):
        while True:
            batch = []
            for name in os.listdir(SPOOL_DIR):
                if not name.endswith(".json"):
                    continue
                path = os.path.join(SPOOL_DIR, name)
                try:
                    with open(path) as fh:
                        payload = json.load(fh)
                except Exception:
                    try:
                        os.unlink(path)   # Discard a corrupt payload.
                    except Exception:
                        pass
                    continue
                batch.append((timestamp_order(payload.get("timestamp")), path, payload))
            if not batch:
                return handled
            batch.sort()
            for _, path, payload in batch:
                try:
                    handle(payload)
                except Exception as exc:
                    log(f"error in {payload.get('event')}: {exc}")
                finally:
                    try:
                        os.unlink(path)
                    except Exception:
                        pass
                handled += 1


def cmd_status():
    """Show rows after pruning missing panes and reconciling loops."""
    with Locked():
        data = load_map()
        dead = prune(data)
        if dead:
            save_map(data)
    for k in dead:
        print(f"  pruned (missing pane): {k}")
    for key, run_id, status in reconcile_loops():
        print(f"  reconciled: {key} run {run_id[-8:]} -> {status}")
    data = load_map()
    print(f"mapped rows: {len(data)}")
    for key, entry in sorted(data.items()):
        live = drop_stale(entry.get("sessions") or {})
        print(f"  {key:34} {entry['pane_id']:8} {entry['tab_id']:8} sessions={len(live)}")
        for sid, info in live.items():
            print(f"       {sid:26} {info.get('state')}")
    res = herdr("agent.list", {})
    if res and "result" in res:
        print("\nherdr agent list (Compozy rows):")
        for a in res["result"]["agents"]:
            if a.get("agent") == AGENT_ID:
                print(f"  {a['pane_id']:8} {a.get('agent_status'):8} {a.get('title') or ''}")


def cmd_reset():
    """Close bridge tabs and clear their map."""
    with Locked():
        data = load_map()
        for key, entry in data.items():
            herdr("pane.release_agent", {
                "pane_id": entry["pane_id"], "source": SOURCE,
                "agent": AGENT_ID, "seq": time.time_ns()})
            herdr("tab.close", {"tab_id": entry["tab_id"]})
            print(f"removed: {key} ({entry['tab_id']})")
        save_map({})


def cmd_refresh():
    """Restart event viewers in existing panes."""
    for key, entry in load_map().items():
        pane_id = entry["pane_id"]
        if entry.get("kind") == "loop":
            cmd = loop_tail_command(entry.get("run_id"), key.split("/", 2)[1])
        else:
            cmd = tail_command(key)
        if not pane_alive(pane_id):
            print(f"  {key}: pane unavailable, skipping")
            continue
        herdr("pane.send_keys", {"pane_id": pane_id, "keys": ["ctrl+c"]})
        time.sleep(0.3)
        herdr("pane.send_text", {"pane_id": pane_id, "text": cmd})
        print(f"  {key}: tail restarted in {pane_id}")


def main():
    """Dispatch a bridge command or consume one hook from standard input."""
    if len(sys.argv) > 1:
        if sys.argv[1] == "--drain":
            drain_spool()
            return watch_loops()
        if sys.argv[1] == "--watch-loops":
            return watch_loops()
        if sys.argv[1] == "--refresh":
            return cmd_refresh()
        if sys.argv[1] == "--status":
            return cmd_status()
        if sys.argv[1] == "--reset":
            return cmd_reset()
        print("usage: bridge.py [--drain|--watch-loops|--status|--refresh|--reset]   (no args: payload on stdin)")
        return
    raw = sys.stdin.read()
    if not raw.strip():
        return
    try:
        handle(json.loads(raw))
    except Exception as exc:
        log(f"error: {exc}")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        log(f"unexpected bridge failure: {exc}")
    sys.exit(0)

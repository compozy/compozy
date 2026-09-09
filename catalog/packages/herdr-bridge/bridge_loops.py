"""Loop row lifecycle and authoritative daemon reconciliation."""
import fcntl
import http.client
import json
import os
import re
import shlex
import socket
import time
from urllib.parse import quote

from bridge_state import (AGENT_ID, COMPOZY_SOCK, SOURCE, STATE_DIR, Locked,
                          close_row, consolidate, drop_stale, herdr, load_map,
                          log, normalize_workspace, pane_alive, save_map)

LOOP_EVENTS = {"loop.started", "loop.generation.pre", "loop.generation.post",
               "loop.gate.post", "loop.node.terminal", "loop.terminal",
               "coordinator.decision"}

LOOP_TASK_ID = re.compile(r"^loop\.(looprun-[a-z0-9]+)\.g(\d+)(?:\.node\.(.+))?$")

LOOP_BLOCKED_STATUSES = {"blocked"}

LOOP_CLOSED_STATUSES = {"done", "no-op", "failed", "exhausted", "stalled", "canceled"}

LOOP_STATUS_STATE = {"running": "working", "queued": "idle", "watching": "idle",
                     "needs-approval": "blocked", "paused": "blocked", "blocked": "blocked"}

LOOP_WATCH_SECONDS = 5


def loop_tail_command(loop_run_id, workspace_id=None):
    """Build the live loop timeline command."""
    args = ["compozy", "loop", "events", loop_run_id, "--follow"]
    workspace_id = normalize_workspace(workspace_id)
    if workspace_id:
        args += ["--workspace", workspace_id]
    return shlex.join(args) + "\n"


def loop_key(workspace_id, loop_name):
    """Separate loop rows by workspace and loop name."""
    return f"loop/{workspace_id or 'no-ws'}/{loop_name}"


def node_label(task_id):
    """Extract the node label from a loop task ID."""
    marker = ".node."
    return task_id.split(marker, 1)[1] if marker in (task_id or "") else None


def find_loop_entry(data, loop_run_id):
    """Find a run when a node event omits the loop name."""
    for key, entry in data.items():
        if entry.get("kind") == "loop" and (loop_run_id in (entry.get("sessions") or {})
                                            or entry.get("run_id") == loop_run_id):
            return key, entry
    return None, None


def ensure_loop_row(data, key, loop_name, loop_run_id):
    """Return or create the loop row while holding the map lock."""
    entry = data.get(key)
    workspace_id = key.split("/", 2)[1]
    if entry and pane_alive(entry.get("pane_id", "")) is not False:
        if entry.get("run_id") != loop_run_id:
            # A new run replaces the timeline followed by this pane.
            herdr("pane.send_keys", {"pane_id": entry["pane_id"], "keys": ["ctrl+c"]})
            time.sleep(0.3)
            herdr("pane.send_text", {"pane_id": entry["pane_id"], "text": loop_tail_command(loop_run_id, workspace_id)})
            entry["run_id"] = loop_run_id
        return entry
    res = herdr("tab.create", {"label": f"cz:loop:{loop_name}", "focus": False})
    if not res or "result" not in res:
        return None
    pane_id = res["result"]["root_pane"]["pane_id"]
    tab_id = res["result"]["tab"]["tab_id"]
    herdr("pane.send_text", {"pane_id": pane_id, "text": loop_tail_command(loop_run_id, workspace_id)})
    entry = {"kind": "loop", "pane_id": pane_id, "tab_id": tab_id,
             "loop": loop_name, "run_id": loop_run_id, "sessions": {}}
    data[key] = entry
    log(f"loop row created for {loop_name}: {pane_id}")
    return entry


def handle_loop(payload):
    """Apply a loop hook to its row without closing active sibling runs."""
    event = payload.get("event") or ""
    run_id = payload.get("loop_run_id")
    gen_from_task = node_from_task = None
    m = LOOP_TASK_ID.match(str(payload.get("task_id") or ""))
    if m:
        run_id = run_id or m.group(1)
        gen_from_task = int(m.group(2))
        node_from_task = m.group(3)
    if not run_id:
        return
    with Locked():
        data = load_map()
        loop_name = payload.get("loop_name")
        terminal = event == "loop.terminal"
        if loop_name:
            key = loop_key(payload.get("workspace_id"), loop_name)
            entry = data.get(key) if terminal else ensure_loop_row(data, key, loop_name, run_id)
        else:
            key, entry = find_loop_entry(data, run_id)
        if not entry:
            return
        runs = entry.setdefault("sessions", {})
        info = runs.get(run_id) or {"name": entry.get("loop"), "state": "working"}
        info["ts"] = time.time()

        if event == "loop.started":
            info["state"] = "working"
            info["status"] = payload.get("status")
        elif event in ("loop.generation.pre", "loop.generation.post"):
            info["state"] = "working"
            gen = payload.get("generation")
            info["gen"] = gen if gen is not None else gen_from_task
        elif event == "coordinator.decision":
            # Synchronous delivery survives even very short loops.
            info["state"] = "working"
            if gen_from_task is not None:
                info["gen"] = gen_from_task
            if node_from_task:
                info["node"] = f"{node_from_task}:{payload.get('decision') or 'running'}"
        elif event == "loop.gate.post":
            info["gate"] = payload.get("status") or payload.get("disposition")
        elif event == "loop.node.terminal":
            label = node_label(payload.get("task_id"))
            if label:
                info["node"] = f"{label}:{payload.get('disposition') or payload.get('run_status') or '?'}"
        elif event == "loop.terminal":
            status = str(payload.get("status") or "").lower()
            info["status"] = status
            if status in LOOP_BLOCKED_STATUSES:
                info["state"] = "blocked"
            elif status in LOOP_CLOSED_STATUSES:
                runs.pop(run_id, None)   # Finished runs leave the activity calculation.
                entry["last_status"] = status
                info = None
            else:
                # Unknown status does not prove termination.
                return
        if info is not None:
            runs[run_id] = info
        if terminal and not runs:
            close_row(data, key)
            save_map(data)
            return
        row_state, active = consolidate(drop_stale(runs))
        data[key] = entry
        save_map(data)
    report_loop_row(entry, row_state, active, event)


def report_loop_row(entry, row_state, active, event):
    """Publish loop activity and metadata to herdr."""
    seq = time.time_ns()
    loop_name = entry.get("loop")
    active_id, active_info = active if active else (None, {})
    herdr("pane.report_agent", {
        "pane_id": entry["pane_id"], "source": SOURCE, "agent": AGENT_ID,
        "state": row_state, "seq": seq,
        "agent_session_id": active_id,
        "message": f"{event} · {loop_name}",
    })
    herdr("pane.report_metadata", {
        "pane_id": entry["pane_id"], "source": SOURCE, "seq": seq,
        "display_agent": AGENT_ID,
        "title": f"loop {loop_name}",
        "tokens": {
            "cz_loop": loop_name,
            "cz_run": (active_id or "")[-8:] or None,
            "cz_gen": str(active_info["gen"]) if active_info and active_info.get("gen") is not None else None,
            "cz_node": (active_info or {}).get("node"),
            "cz_status": (active_info or {}).get("status") or entry.get("last_status"),
            "cz_live": str(len(entry["sessions"])) if entry.get("sessions") else None,
        },
    })


def query_loop_status(run_id, workspace_id):
    """Read the local briefing API without resolving the workspace again."""
    if not workspace_id or workspace_id == "no-ws":
        return None
    conn = http.client.HTTPConnection("localhost", timeout=5)
    try:
        conn.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        conn.sock.settimeout(5)
        conn.sock.connect(COMPOZY_SOCK)
        path = f"/api/workspaces/{quote(workspace_id, safe='')}/loop-runs/{quote(run_id, safe='')}/briefing"
        conn.request("GET", path)
        response = conn.getresponse()
        if response.status != 200:
            log(f"reconcile {run_id}: HTTP {response.status}")
            return None
        return str(json.loads(response.read()).get("status") or "").lower() or None
    except Exception as exc:
        log(f"reconcile {run_id}: {exc}")
        return None
    finally:
        conn.close()


def reconcile_loops():
    """Recover missing terminal hooks by reading authoritative loop status."""
    fixed = []
    # Query outside the map lock so a slow daemon cannot block hooks.
    with Locked():
        data = load_map()
        targets = []
        for key, entry in data.items():
            if entry.get("kind") != "loop":
                continue
            workspace_id = key.split("/")[1] if key.count("/") >= 2 else None
            runs = list(entry.get("sessions") or {})
            if not runs and entry.get("run_id"):
                runs = [entry["run_id"]]  # Versions through 0.3.1 retained empty rows.
            targets.extend((key, entry["pane_id"], run_id, workspace_id) for run_id in runs)
    for key, pane_id, run_id, workspace_id in targets:
        status = query_loop_status(run_id, workspace_id)
        if status not in LOOP_CLOSED_STATUSES and status not in LOOP_STATUS_STATE:
            continue
        with Locked():
            data = load_map()
            entry = data.get(key)
            if not entry or entry["pane_id"] != pane_id:
                continue
            runs = entry.setdefault("sessions", {})
            if run_id not in runs and entry.get("run_id") != run_id:
                continue
            if status in LOOP_CLOSED_STATUSES:
                runs.pop(run_id, None)
                entry["last_status"] = status
                fixed.append((key, run_id, status))
                if not runs:
                    close_row(data, key)
                    save_map(data)
                    continue
            else:
                info = runs.get(run_id) or {"name": entry.get("loop")}
                changed = info.get("state") != LOOP_STATUS_STATE[status] or info.get("status") != status
                info.update(state=LOOP_STATUS_STATE[status], status=status, ts=time.time())
                runs[run_id] = info
                if not changed:
                    save_map(data)
                    continue
                fixed.append((key, run_id, status))
            row_state, active = consolidate(runs)
            report_loop_row(entry, row_state, active, "reconcile")
            save_map(data)
    return fixed


def watch_loops():
    """Keep one detached monitor alive until all loop rows have closed."""
    os.makedirs(STATE_DIR, exist_ok=True)
    with open(os.path.join(STATE_DIR, ".watch-lock"), "w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        while True:
            reconcile_loops()
            with Locked():
                if not any(e.get("kind") == "loop" for e in load_map().values()):
                    # Release the watcher lock before allowing another hook to create a row.
                    # That drainer can then take over monitoring.
                    fcntl.flock(lock, fcntl.LOCK_UN)
                    return
            time.sleep(LOOP_WATCH_SECONDS)


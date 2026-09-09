"""Shared bridge persistence, row activity, and herdr transport."""
import fcntl
import json
import os
import socket
import time

HERDR_SOCK = os.path.expanduser("~/.config/herdr/herdr.sock")

COMPOZY_SOCK = os.path.join(os.environ.get("COMPOZY_HOME", os.path.expanduser("~/.compozy")), "daemon.sock")

STATE_DIR = os.path.join(os.environ.get("XDG_STATE_HOME", os.path.expanduser("~/.local/state")), "herdr-bridge")

MAP_PATH = os.path.join(STATE_DIR, "panes.json")

SPOOL_DIR = os.path.join(STATE_DIR, "spool")

LOG_PATH = os.path.join(STATE_DIR, "bridge.log")

SOURCE = "compozy-bridge"

AGENT_ID = "compozy"

STALE_SESSION_SECONDS = 1800

STATE_RANK = {"blocked": 3, "working": 2, "idle": 1}


def log(msg):
    """Write a diagnostic without failing a hook if logging is unavailable."""
    try:
        os.makedirs(STATE_DIR, mode=0o700, exist_ok=True)
        with open(LOG_PATH, "a") as fh:
            fh.write(f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())} {msg}\n")
    except Exception:
        pass


def herdr(method, params):
    """Call herdr JSON-RPC; return None when transport is unavailable."""
    if not os.path.exists(HERDR_SOCK):
        return None
    req = {"id": f"{SOURCE}:{time.time_ns()}", "method": method, "params": params}
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as cli:
            cli.settimeout(2.0)
            cli.connect(HERDR_SOCK)
            cli.sendall((json.dumps(req) + "\n").encode())
            buf = b""
            while b"\n" not in buf:
                chunk = cli.recv(65536)
                if not chunk:
                    break
                buf += chunk
        line = buf.split(b"\n")[0]
        return json.loads(line) if line else None
    except Exception as exc:
        log(f"herdr {method} failed: {exc}")
        return None


class Locked:
    """Serialize asynchronous hooks with an advisory file lock."""

    def __init__(self, name=".lock"):
        """Select the lock file."""
        self.name = name

    def __enter__(self):
        """Acquire the exclusive lock."""
        os.makedirs(STATE_DIR, exist_ok=True)
        self.fh = open(os.path.join(STATE_DIR, self.name), "w")
        fcntl.flock(self.fh, fcntl.LOCK_EX)
        return self

    def __exit__(self, *_):
        """Release the lock and file descriptor."""
        fcntl.flock(self.fh, fcntl.LOCK_UN)
        self.fh.close()


def load_map():
    """Read persisted rows, or return an empty map before the first hook."""
    try:
        with open(MAP_PATH) as fh:
            return json.load(fh)
    except FileNotFoundError:
        return {}


def save_map(data):
    """Atomically persist the row map."""
    os.makedirs(STATE_DIR, exist_ok=True)
    tmp = MAP_PATH + ".tmp"
    with open(tmp, "w") as fh:
        json.dump(data, fh, indent=1)
    os.replace(tmp, MAP_PATH)


def close_row(data, key):
    """Close only the bridge pane under the map lock; retain failures for retry."""
    entry = data[key]
    res = herdr("pane.close", {"pane_id": entry["pane_id"]})
    if res and ("result" in res or (res.get("error") or {}).get("code") == "pane_not_found"):
        data.pop(key, None)
        log(f"row closed for {key}: {entry['pane_id']}")
        return True
    log(f"pane.close failed for {entry['pane_id']}: {res}")
    return False


def pane_alive(pane_id):
    """Return True for a live pane, False for a missing pane, otherwise None."""
    res = herdr("pane.get", {"pane_id": pane_id})
    if res and "result" in res:
        return True
    if res and (res.get("error") or {}).get("code") == "pane_not_found":
        return False
    return None


def row_key(workspace_id, agent_name):
    """Separate rows by workspace and agent to prevent mixed session tails."""
    return f"{workspace_id or 'no-ws'}/{agent_name}"


def prune(data):
    """Remove only rows whose panes are confirmed missing."""
    dead = [k for k, e in data.items() if pane_alive(e.get("pane_id", "")) is False]
    for k in dead:
        data.pop(k, None)
    return dead


def drop_stale(sessions, now=None):
    """Filter recent activity; silence does not prove session termination."""
    now = now if now is not None else time.time()
    return {
        sid: info for sid, info in sessions.items()
        if info.get("state") == "blocked"  # Blocked sessions do not age out.
        or now - float(info.get("ts") or 0) < STALE_SESSION_SECONDS
    }


def consolidate(sessions):
    """Choose the highest activity across live sibling sessions."""
    if not sessions:
        return "idle", None
    best_id, best = None, None
    for sid, info in sessions.items():
        rank = STATE_RANK.get(info.get("state"), 0)
        if best is None or rank > STATE_RANK.get(best.get("state"), 0):
            best_id, best = sid, info
    return best.get("state", "idle"), (best_id, best)


def normalize_workspace(workspace_id):
    """Decode the persisted missing-workspace sentinel at command boundaries."""
    return None if workspace_id == "no-ws" else workspace_id

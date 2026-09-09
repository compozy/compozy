#!/usr/bin/env python3
"""Follow original session events for one bridge row.

Sequence cursors preserve message whitespace and resume after reconnects."""
import argparse
import http.client
import json
import socket
import time
from urllib.parse import quote, urlencode

import bridge_state as bridge
from colorize import Renderer

POLL_SECONDS = 1
INITIAL_EVENTS = 100


def get_json(path):
    """Read one JSON response from the local daemon socket."""
    connection = http.client.HTTPConnection("localhost", timeout=5)
    connection.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    connection.sock.settimeout(5)
    try:
        connection.sock.connect(bridge.COMPOZY_SOCK)
        connection.request("GET", path)
        response = connection.getresponse()
        if response.status != 200:
            raise OSError(f"CompozyOS HTTP {response.status}")
        return json.loads(response.read())
    finally:
        connection.close()


class SessionTail:
    """Track event cursors for the live sessions belonging to one row."""
    def __init__(self, key, renderer):
        """Initialize row scope and per-session cursors."""
        self.key = key
        self.workspace = key.split("/", 1)[0]
        self.renderer = renderer
        self.cursors = {}
        self.errors = {}

    def poll(self, data):
        """Read new events and drain removed sessions before dropping cursors."""
        sessions = data.get(self.key, {}).get("sessions", {})
        for sid in sessions:
            self.cursors.setdefault(sid, None)
        for sid, cursor in list(self.cursors.items()):
            query = {"archive": "all"}
            if cursor is None:
                query["limit"] = INITIAL_EVENTS
            else:
                # limit selects the LAST N even with after_sequence.
                # Do not truncate bursts or reconnect backlogs.
                query["after_sequence"] = cursor
            try:
                workspace = bridge.normalize_workspace(self.workspace)
                if not workspace:
                    owner = get_json(f"/api/sessions/{quote(sid, safe='')}/owner")
                    workspace = owner["workspace_id"]
                    if not workspace or owner["session_id"] != sid:
                        raise ValueError("session owner response does not match the requested session")
                path = (f"/api/workspaces/{quote(workspace, safe='')}/sessions/"
                        f"{quote(sid, safe='')}/events?{urlencode(query)}")
                events = get_json(path)["events"]
            except (OSError, http.client.HTTPException, ValueError, KeyError) as exc:
                # Failures neither advance the cursor nor interrupt sibling sessions.
                if self.errors.get(sid) != str(exc):
                    bridge.log(f"tail {sid}: {exc}")
                self.errors[sid] = str(exc)
                continue
            self.errors.pop(sid, None)
            for event in events:
                sequence = event["sequence"]
                if sequence > (self.cursors[sid] or 0):
                    self.renderer.feed(event)
                    self.cursors[sid] = sequence
            if self.cursors[sid] is None:
                self.cursors[sid] = 0
            # Read the final batch before forgetting a removed session.
            if sid not in sessions:
                self.cursors.pop(sid)


def main():
    """Poll the selected row and flush its renderer on exit."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("row", help="workspace_id/agent_name key in panes.json")
    args = parser.parse_args()
    renderer = Renderer()
    tail = SessionTail(args.row, renderer)
    try:
        while True:
            tail.poll(bridge.load_map())
            time.sleep(POLL_SECONDS)
    finally:
        renderer.close_stream()


if __name__ == "__main__":
    try:
        main()
    except (KeyboardInterrupt, BrokenPipeError):
        pass

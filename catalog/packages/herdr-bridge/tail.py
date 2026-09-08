#!/usr/bin/env python3
"""Segue os eventos originais das sessoes de uma linha do bridge.

Logs sao resumos aparados e truncados, nao deltas de texto. O cursor de
sequencia da API de sessoes permite acompanhar o conteudo sem perder espacos.
"""
import argparse
import http.client
import json
import socket
import time
from urllib.parse import quote, urlencode

import bridge
from colorize import Renderer

POLL_SECONDS = 1
INITIAL_EVENTS = 100


def get_json(path):
    connection = http.client.HTTPConnection("localhost", timeout=5)
    connection.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    connection.sock.settimeout(5)
    try:
        connection.sock.connect(bridge.COMPOZY_SOCK)
        connection.request("GET", path)
        response = connection.getresponse()
        if response.status != 200:
            raise OSError(f"Compozy HTTP {response.status}")
        return json.loads(response.read())
    finally:
        connection.close()


class SessionTail:
    def __init__(self, key, renderer):
        self.key = key
        self.workspace = key.split("/", 1)[0]
        self.renderer = renderer
        self.cursors = {}
        self.errors = {}

    def poll(self, data):
        sessions = data.get(self.key, {}).get("sessions", {})
        for sid in sessions:
            self.cursors.setdefault(sid, None)
        for sid, cursor in list(self.cursors.items()):
            query = {"archive": "all"}
            if cursor is None:
                query["limit"] = INITIAL_EVENTS
            else:
                # limit significa os ULTIMOS N, mesmo com after_sequence.
                # Limitar aqui descartaria o inicio de rajadas ou reconexoes.
                query["after_sequence"] = cursor
            path = (f"/api/workspaces/{quote(self.workspace, safe='')}/sessions/"
                    f"{quote(sid, safe='')}/events?{urlencode(query)}")
            try:
                events = get_json(path)["events"]
            except (OSError, http.client.HTTPException, ValueError, KeyError) as exc:
                # Uma falha nao avanca o cursor nem interrompe sessoes irmas.
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
            # Le a ultima leva de uma sessao removida antes de esquecer o cursor.
            if sid not in sessions:
                self.cursors.pop(sid)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("row", help="Chave workspace_id/agent_name em panes.json")
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

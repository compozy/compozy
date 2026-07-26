#!/usr/bin/env python3
"""Allocate an isolated AGH runtime envelope for a parallel-worktree scenario.

Outputs export statements (POSIX-shell syntax) to stdout so callers can:
    eval "$(python3 allocate-isolation.py --slug my-qa)"

Variables set:
    AGH_ISOLATION_ROOT  owned envelope root for targeted teardown
    AGH_HOME            unique directory under TMPDIR (or worktree-scoped)
    AGH_HTTP_PORT       free 127.0.0.1 TCP port
    AGH_UDS_PATH        unique UDS socket path under AGH_HOME
    TMUX_BRIDGE_SOCKET  unique tmux-bridge socket path under AGH_HOME

Exits 0 on success, 1 on failure.
"""

from __future__ import annotations

import argparse
import os
import random
import socket
import string
import sys
import tempfile
import time
from pathlib import Path


def random_suffix(n: int = 6) -> str:
    return "".join(random.choices(string.ascii_lowercase + string.digits, k=n))


def pick_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def resolve_home(slug: str, prefer_worktree: bool) -> Path:
    if prefer_worktree:
        cwd = Path.cwd()
        if "_worktrees" in cwd.parts:
            for i, part in enumerate(cwd.parts):
                if part == "_worktrees" and i + 1 < len(cwd.parts):
                    candidate = Path(*cwd.parts[: i + 2]) / ".agh"
                    candidate.mkdir(parents=True, exist_ok=True)
                    return candidate
    base = Path(tempfile.gettempdir()) / f"agh-iso-{slug}-{random_suffix()}"
    base.mkdir(parents=True, exist_ok=False)
    return base


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--slug", default=None, help="Scenario slug (default: agh-iso-<timestamp>)")
    parser.add_argument(
        "--prefer-worktree",
        action="store_true",
        help="Use Compozy/_worktrees/<slug>/.agh when invoked from a worktree",
    )
    args = parser.parse_args()
    slug = args.slug or f"agh-iso-{int(time.time())}"

    try:
        agh_home = resolve_home(slug, prefer_worktree=args.prefer_worktree)
    except Exception as exc:
        print(f"FAILED to allocate AGH_HOME: {exc}", file=sys.stderr)
        return 1

    try:
        http_port = pick_free_port()
    except Exception as exc:
        print(f"FAILED to pick free HTTP port: {exc}", file=sys.stderr)
        return 1

    uds_path = agh_home / f"daemon-{random_suffix(4)}.sock"
    tmux_socket = agh_home / f"tmux-bridge-{random_suffix(4)}.sock"

    sys.stderr.write(
        f"# Allocated isolation envelope for slug={slug}\n"
        f"#   AGH_HOME={agh_home}\n"
        f"#   AGH_HTTP_PORT={http_port}\n"
        f"#   AGH_UDS_PATH={uds_path}\n"
        f"#   TMUX_BRIDGE_SOCKET={tmux_socket}\n"
    )

    print(f"export AGH_ISOLATION_ROOT={shquote(str(agh_home))}")
    print(f"export AGH_HOME={shquote(str(agh_home))}")
    print(f"export AGH_HTTP_PORT={http_port}")
    print(f"export AGH_UDS_PATH={shquote(str(uds_path))}")
    print(f"export TMUX_BRIDGE_SOCKET={shquote(str(tmux_socket))}")
    return 0


def shquote(value: str) -> str:
    if not value:
        return "''"
    safe_chars = set(string.ascii_letters + string.digits + "-_./")
    if all(c in safe_chars for c in value):
        return value
    return "'" + value.replace("'", "'\\''") + "'"


if __name__ == "__main__":
    sys.exit(main())

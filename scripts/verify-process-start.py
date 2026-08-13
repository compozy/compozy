#!/usr/bin/env python3
"""Verify that a PID still identifies the process recorded by the daemon."""

import datetime
import re
import subprocess
import sys
import time


RFC3339_PATTERN = re.compile(
    r"^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d+))?([+-]\d{2}:\d{2})$"
)


def parse_recorded_start(value: str) -> float:
    normalized = value.replace("Z", "+00:00")
    match = RFC3339_PATTERN.fullmatch(normalized)
    if match is None:
        raise ValueError(f"invalid RFC3339 process start time: {value!r}")
    base, fraction, offset = match.groups()
    if fraction:
        normalized = f"{base}.{fraction[:6]}{offset}"
    else:
        normalized = f"{base}{offset}"
    return datetime.datetime.fromisoformat(normalized).timestamp()


def observed_process_start(pid: int) -> float:
    observed_text = subprocess.run(
        ["ps", "-p", str(pid), "-o", "lstart="],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    return time.mktime(time.strptime(observed_text, "%a %b %d %H:%M:%S %Y"))


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print("usage: verify-process-start.py <pid> <rfc3339-start-time>", file=sys.stderr)
        return 2
    try:
        pid = int(argv[1])
        recorded = parse_recorded_start(argv[2])
        observed = observed_process_start(pid)
    except (OSError, subprocess.SubprocessError, ValueError) as error:
        print(f"process identity verification failed: {error}", file=sys.stderr)
        return 2
    if abs(recorded - observed) > 2:
        print("process identity verification failed: recorded start time does not match", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))

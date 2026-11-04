"""Count successful write syscall bytes, including resumed calls, by SQLite file.

This is application file-write volume, not physical device traffic or WAL growth.
Shared-memory mmap dirtying and filesystem journal/cache amplification are excluded.
"""
import re
from datetime import datetime

HEADER = re.compile(r"^(?:\[pid\s+)?(\d+)\]?\s+(\d+\.\d+)\s+(.*)$")
FD = re.compile(r"^(pwrite64|write)\(\d+<([^>]+)>")
RESULT = re.compile(r"\)\s+=\s+(-?\d+)")


def write_bytes(path, result, core_path):
    start = datetime.fromisoformat(result["start"].replace("Z", "+00:00")).timestamp()
    end = datetime.fromisoformat(result["end"].replace("Z", "+00:00")).timestamp()
    pending = {}
    totals = {"core": 0, "telemetry": 0, "sessions": 0}
    calls = {"core": 0, "telemetry": 0, "sessions": 0}
    unmatched = []
    with open(path) as trace:
        for line in trace:
            header = HEADER.match(line)
            if not header:
                continue
            pid, stamp, body = header.groups()
            stamp = float(stamp)
            if body.startswith("<... "):
                original = pending.pop(pid, None)
                if original is None:
                    if "write resumed>" in body and start <= stamp <= end:
                        unmatched.append(line.rstrip())
                    continue
                target, began = original
            else:
                fd = FD.match(body)
                if not fd:
                    continue
                _, target = fd.groups()
                began = stamp
                if "<unfinished ...>" in body:
                    pending[pid] = (target, began)
                    continue
            returned = RESULT.search(body)
            if not returned or not start <= began <= end:
                continue
            count = int(returned.group(1))
            if count < 0:
                continue
            kind = None
            if target in (core_path, core_path + "-wal"):
                kind = "core"
            elif target in (core_path + ".telemetry.db", core_path + ".telemetry.db-wal"):
                kind = "telemetry"
            elif "/sessions/" in target and target.endswith(("/events.db", "/events.db-wal")):
                kind = "sessions"
            if kind:
                totals[kind] += count
                calls[kind] += 1
    if unmatched or any(start <= began <= end for _, began in pending.values()):
        raise RuntimeError(f"incomplete write accounting: {unmatched[:3]}, pending={pending}")
    if not totals["core"] or not totals["sessions"]:
        raise RuntimeError(f"missing SQLite write samples: {totals}")
    return {"bytes": totals, "write_calls": calls,
            "bytes_per_minute": {key: value * 60 / result["seconds"] for key, value in totals.items()}}

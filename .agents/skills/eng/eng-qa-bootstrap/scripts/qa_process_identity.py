"""Process identity and individually fenced signaling for one QA lab."""
from __future__ import annotations

import argparse
import json
import os
import signal
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class ProcessIdentity:
    pid: int
    started: str
    command: str


@dataclass(frozen=True)
class SignalTarget:
    identity: ProcessIdentity
    reason: str


def process_identity(pid: int) -> ProcessIdentity | None:
    proc = subprocess.run(
        ["ps", "-p", str(pid), "-o", "lstart=,command="],
        check=False, capture_output=True, text=True,
    )
    fields = proc.stdout.strip().split(None, 5)
    if proc.returncode != 0 or len(fields) != 6:
        return None
    return ProcessIdentity(pid, " ".join(fields[:5]), fields[5])


def matches_identity(identity: ProcessIdentity) -> bool:
    return process_identity(identity.pid) == identity


def registered_identity(path: Path) -> ProcessIdentity | None:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
        record = json.loads(lines[1])
        identity = ProcessIdentity(**record)
        return identity if str(identity.pid) == lines[0] else None
    except (OSError, ValueError, IndexError, TypeError):
        return None


def registered_process_matches(qa_root: Path | None, pid: int) -> bool:
    if qa_root is None:
        return True
    records = [registered_identity(path) for path in (qa_root / "pids").glob("*.pid")]
    return all(matches_identity(record) for record in records if record is not None and record.pid == pid)


def register_process(pid: int, path: Path) -> None:
    identity = process_identity(pid)
    if identity is None:
        raise RuntimeError(f"cannot identify process {pid}")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(f"{pid}\n" + json.dumps(identity.__dict__) + "\n", encoding="utf-8")


def pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True


def list_processes() -> list[tuple[int, int, str]]:
    """Return (pid, pgid, command) for every visible process."""
    proc = subprocess.run(
        ["ps", "-axo", "pid=,pgid=,command="],
        check=False,
        capture_output=True,
        text=True,
    )
    rows: list[tuple[int, int, str]] = []
    for line in proc.stdout.splitlines():
        parts = line.strip().split(None, 2)
        if len(parts) < 3:
            continue
        try:
            pid = int(parts[0])
            pgid = int(parts[1])
        except ValueError:
            continue
        rows.append((pid, pgid, parts[2]))
    return rows


def protected_pids() -> set[int]:
    protected = {os.getpid(), os.getppid(), 1}
    try:
        protected.add(os.getpgrp())
    except OSError:
        pass
    return protected


def lsof_pids(paths: list[Path]) -> set[int]:
    existing = [str(p) for p in paths if p.exists()]
    if not existing:
        return set()
    pids: set[int] = set()
    for path in existing:
        proc = subprocess.run(
            ["lsof", "-t", "+D", path],
            check=False,
            capture_output=True,
            text=True,
        )
        for line in proc.stdout.splitlines():
            try:
                pids.add(int(line.strip()))
            except ValueError:
                continue
    return pids


def port_listener_pids(port: int) -> set[int]:
    proc = subprocess.run(
        ["lsof", "-t", "-nP", f"-iTCP:{port}", "-sTCP:LISTEN"],
        check=False,
        capture_output=True,
        text=True,
    )
    pids: set[int] = set()
    for line in proc.stdout.splitlines():
        try:
            pids.add(int(line.strip()))
        except ValueError:
            continue
    return pids


def signal_pids(pids: dict[int, SignalTarget], grace_sec: float, result, dry_run: bool) -> None:
    protected = protected_pids()
    verified = {
        pid: target for pid, target in pids.items()
        if pid not in protected and matches_identity(target.identity)
    }
    for pid in pids.keys() - verified.keys() - protected:
        entry = {"pid": pid, "reason": "process identity changed before signaling"}
        result.stale.append(entry)
        result.skipped.append(entry)
    if dry_run:
        for pid, target in sorted(verified.items()):
            result.notes.append(f"dry-run: would kill pid {pid} ({target.reason})")
        return
    errors: dict[int, str] = {}
    for sig in (signal.SIGTERM, signal.SIGKILL):
        for pid, target in sorted(verified.items()):
            if not matches_identity(target.identity):
                if not any(entry["pid"] == pid for entry in result.stale):
                    entry = {"pid": pid, "reason": "process identity changed before signaling"}
                    result.stale.append(entry)
                    result.skipped.append(entry)
                continue
            if not pid_alive(pid):
                continue
            try:
                os.kill(pid, sig)
            except ProcessLookupError:
                continue
            except PermissionError:
                errors[pid] = f"EPERM on {sig.name}"
        if sig == signal.SIGTERM:
            deadline = time.monotonic() + grace_sec
            while time.monotonic() < deadline and any(
                matches_identity(target.identity) and pid_alive(pid)
                for pid, target in verified.items()
            ):
                time.sleep(0.25)
    if verified:
        time.sleep(0.25)
    for pid, target in sorted(verified.items()):
        if matches_identity(target.identity) and pid_alive(pid):
            result.survivors.append({"pid": pid, "reason": target.reason,
                                     "error": errors.get(pid, "still alive after SIGKILL")})
        else:
            result.killed.append({"pid": pid, "reason": target.reason})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Register an owned QA process identity immediately after spawn")
    parser.add_argument("--pid", type=int, required=True)
    parser.add_argument("--file", type=Path, required=True)
    args = parser.parse_args()
    register_process(args.pid, args.file)

#!/usr/bin/env python3
"""Tear down AGH QA labs: stop daemons, kill orphaned lab processes, optionally purge lab dirs.

This is the mandatory counterpart of bootstrap-qa-env.py. Every QA pass that
bootstraps a lab MUST end with a teardown invocation (success OR failure paths):

    python3 .agents/skills/agh/agh-qa-bootstrap/scripts/teardown-qa-env.py --manifest "<BOOTSTRAP_MANIFEST>"

Tear down one bare worktree-isolation envelope without touching other labs:

    python3 .agents/skills/agh/agh-qa-bootstrap/scripts/teardown-qa-env.py --root "<AGH_ISOLATION_ROOT>"

Reap every discoverable lab on the machine (safe default set of roots):

    python3 .agents/skills/agh/agh-qa-bootstrap/scripts/teardown-qa-env.py --all

Discovery roots for --all:
  - ~/dev/qa-labs/agh-*-lab/qa-artifacts/qa/bootstrap-manifest.json  (bootstrap labs)
  - $TMPDIR/aghqa-*                                                  (short socket-safe runtime roots)
  - $TMPDIR/agh-iso-* and /tmp/agh-iso-*                             (worktree-isolation envelopes)

Per lab the teardown:
  1. Stops the daemon gracefully (`agh daemon stop` against the lab AGH_HOME when a
     repo binary exists, else SIGTERM -> SIGKILL on the PID recorded in daemon.lock /
     daemon.json). Graceful stop lets the daemon shut down its ACP agent subprocesses,
     MCP sidecars, and tmux bridge sessions.
  2. Kills the tmux server bound to the lab tmux socket (`tmux -S <sock> kill-server`).
  3. Sweeps survivors: any process whose command line references a lab root, any
     PID recorded under <qa_root>/pids/*.pid, any listener on the lab HTTP port, and
     any process holding open files under the lab runtime/provider homes (lsof).
  4. Writes teardown evidence to <qa_root>/teardown.json and stamps the manifest with
     a `teardown` block so audits and continuations can verify process hygiene.

Exit codes: 0 = clean (no survivors), 1 = survivors remain or teardown error, 2 = usage.
"""

from __future__ import annotations

import argparse
import datetime as _dt
import glob
import json
import os
import signal
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass, field
from pathlib import Path

GRACE_DEFAULT_SEC = 10.0
DAEMON_STOP_TIMEOUT_SEC = 20.0
SAFE_ROOT_MARKERS = ("/qa-labs/", "/aghqa-", "/agh-iso-", "/_worktrees/")


@dataclass
class LabTarget:
    label: str
    manifest_path: Path | None
    roots: list[Path]
    agh_home: Path | None
    tmux_socket: Path | None
    uds_path: Path | None
    http_port: int | None
    qa_root: Path | None
    purgeable_roots: list[Path] = field(default_factory=list)


@dataclass
class TeardownResult:
    label: str
    stopped_daemon_pid: int | None = None
    killed: list[dict] = field(default_factory=list)
    survivors: list[dict] = field(default_factory=list)
    purged: list[str] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)

    @property
    def clean(self) -> bool:
        return not self.survivors


def utc_now() -> str:
    return _dt.datetime.now(_dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def load_json(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def is_safe_root(path: Path) -> bool:
    text = str(path)
    if len(text) < 8 or text == "/":
        return False
    return any(marker in text + "/" for marker in SAFE_ROOT_MARKERS)


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


def read_pid_file(path: Path) -> int:
    try:
        raw = path.read_text(encoding="utf-8").strip()
    except OSError:
        return 0
    if not raw:
        return 0
    try:
        pid = int(raw.splitlines()[0].strip())
    except ValueError:
        return 0
    return pid if pid > 0 else 0


def read_daemon_pid(agh_home: Path) -> int:
    info_path = agh_home / "daemon.json"
    if info_path.is_file():
        try:
            info = load_json(info_path)
            pid = int(info.get("pid", 0))
            if pid > 0:
                return pid
        except (OSError, ValueError, json.JSONDecodeError):
            pass
    return read_pid_file(agh_home / "daemon.lock")


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


def registered_pids(qa_root: Path | None) -> dict[int, str]:
    if qa_root is None:
        return {}
    pids_dir = qa_root / "pids"
    if not pids_dir.is_dir():
        return {}
    found: dict[int, str] = {}
    for pid_file in sorted(pids_dir.glob("*.pid")):
        pid = read_pid_file(pid_file)
        if pid > 0:
            found[pid] = pid_file.stem
    return found


def resolve_repo_agh_binary(repo_root: Path) -> Path | None:
    candidate = repo_root / "bin" / "agh"
    if candidate.is_file() and os.access(candidate, os.X_OK):
        return candidate
    return None


def stop_daemon_gracefully(
    agh_binary: Path | None,
    agh_home: Path,
    result: TeardownResult,
    dry_run: bool,
) -> None:
    pid = read_daemon_pid(agh_home)
    if pid <= 0 or not pid_alive(pid):
        return
    result.stopped_daemon_pid = pid
    if dry_run:
        result.notes.append(f"dry-run: would stop daemon pid {pid} (AGH_HOME={agh_home})")
        return
    if agh_binary is not None:
        env = dict(os.environ)
        env["AGH_HOME"] = str(agh_home)
        try:
            subprocess.run(
                [str(agh_binary), "daemon", "stop"],
                check=False,
                capture_output=True,
                text=True,
                timeout=DAEMON_STOP_TIMEOUT_SEC,
                env=env,
            )
        except subprocess.TimeoutExpired:
            result.notes.append(f"agh daemon stop timed out after {DAEMON_STOP_TIMEOUT_SEC}s for {agh_home}")
        deadline = time.monotonic() + DAEMON_STOP_TIMEOUT_SEC
        while pid_alive(pid) and time.monotonic() < deadline:
            time.sleep(0.25)
    if pid_alive(pid):
        result.notes.append(f"daemon pid {pid} survived graceful stop; escalating to signal sweep")


def kill_tmux_server(tmux_socket: Path | None, result: TeardownResult, dry_run: bool) -> None:
    if tmux_socket is None or not tmux_socket.exists():
        return
    if dry_run:
        result.notes.append(f"dry-run: would kill tmux server on {tmux_socket}")
        return
    subprocess.run(
        ["tmux", "-S", str(tmux_socket), "kill-server"],
        check=False,
        capture_output=True,
        text=True,
    )
    try:
        tmux_socket.unlink(missing_ok=True)
    except OSError as err:
        result.notes.append(f"could not remove tmux socket {tmux_socket}: {err}")


def collect_target_pids(target: LabTarget) -> dict[int, str]:
    """Map candidate PID -> reason string."""
    candidates: dict[int, str] = {}
    root_strings = [str(root) for root in target.roots if is_safe_root(root)]
    for pid, _pgid, command in list_processes():
        for root in root_strings:
            if root in command:
                candidates.setdefault(pid, f"cmdline references {root}")
                break
    for pid, name in registered_pids(target.qa_root).items():
        candidates.setdefault(pid, f"registered pid file {name}.pid")
    if target.http_port:
        for pid in port_listener_pids(target.http_port):
            candidates.setdefault(pid, f"listens on lab port {target.http_port}")
    lsof_scan_paths: list[Path] = []
    if target.agh_home is not None:
        lsof_scan_paths.append(target.agh_home)
    for root in target.roots:
        name = root.name
        if name.startswith("aghqa-") or name.startswith("agh-iso-") or name == ".agh":
            lsof_scan_paths.append(root)
    for pid in lsof_pids(lsof_scan_paths):
        candidates.setdefault(pid, "holds open files under lab runtime")
    for pid in protected_pids():
        candidates.pop(pid, None)
    return candidates


def signal_pids(pids: dict[int, str], grace_sec: float, result: TeardownResult, dry_run: bool) -> None:
    if not pids:
        return
    pgid_by_pid: dict[int, int] = {}
    for pid, pgid, _command in list_processes():
        pgid_by_pid[pid] = pgid
    own_pgid = os.getpgrp()
    if dry_run:
        for pid, reason in sorted(pids.items()):
            result.notes.append(f"dry-run: would kill pid {pid} ({reason})")
        return
    for pid, reason in sorted(pids.items()):
        pgid = pgid_by_pid.get(pid, 0)
        try:
            if pgid > 1 and pgid != own_pgid:
                os.killpg(pgid, signal.SIGTERM)
            else:
                os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            continue
        except PermissionError:
            result.survivors.append({"pid": pid, "reason": reason, "error": "EPERM on SIGTERM"})
            continue
    deadline = time.monotonic() + grace_sec
    remaining = {pid: reason for pid, reason in pids.items() if pid_alive(pid)}
    while remaining and time.monotonic() < deadline:
        time.sleep(0.25)
        remaining = {pid: reason for pid, reason in remaining.items() if pid_alive(pid)}
    for pid, reason in sorted(remaining.items()):
        pgid = pgid_by_pid.get(pid, 0)
        try:
            if pgid > 1 and pgid != own_pgid:
                os.killpg(pgid, signal.SIGKILL)
            else:
                os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            continue
        except PermissionError:
            result.survivors.append({"pid": pid, "reason": reason, "error": "EPERM on SIGKILL"})
            continue
    time.sleep(0.25)
    for pid, reason in sorted(pids.items()):
        if pid_alive(pid):
            if not any(entry.get("pid") == pid for entry in result.survivors):
                result.survivors.append({"pid": pid, "reason": reason, "error": "still alive after SIGKILL"})
        else:
            result.killed.append({"pid": pid, "reason": reason})


def purge_roots(target: LabTarget, result: TeardownResult, dry_run: bool) -> None:
    import shutil

    for root in target.purgeable_roots:
        if not root.exists():
            continue
        if not is_safe_root(root):
            result.notes.append(f"refusing to purge non-lab path {root}")
            continue
        if dry_run:
            result.notes.append(f"dry-run: would purge {root}")
            continue
        try:
            shutil.rmtree(root)
            result.purged.append(str(root))
        except OSError as err:
            result.notes.append(f"purge failed for {root}: {err}")


def write_evidence(target: LabTarget, result: TeardownResult, dry_run: bool) -> None:
    if dry_run:
        return
    payload = {
        "completed_at": utc_now(),
        "clean": result.clean,
        "stopped_daemon_pid": result.stopped_daemon_pid,
        "killed": result.killed,
        "survivors": result.survivors,
        "purged": result.purged,
        "notes": result.notes,
    }
    if target.qa_root is not None and target.qa_root.is_dir():
        evidence_path = target.qa_root / "teardown.json"
        try:
            evidence_path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        except OSError as err:
            result.notes.append(f"could not write teardown evidence {evidence_path}: {err}")
    if target.manifest_path is not None and target.manifest_path.is_file():
        try:
            manifest = load_json(target.manifest_path)
            manifest["teardown"] = payload
            target.manifest_path.write_text(
                json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
            )
        except (OSError, json.JSONDecodeError) as err:
            result.notes.append(f"could not stamp manifest {target.manifest_path}: {err}")


def target_from_manifest(manifest_path: Path) -> LabTarget:
    manifest = load_json(manifest_path)
    env = {str(k): str(v) for k, v in manifest.get("env", {}).items()}
    paths = {str(k): str(v) for k, v in manifest.get("paths", {}).items()}
    roots: list[Path] = []
    purgeable: list[Path] = []
    workspace = env.get("WORKSPACE_PATH", manifest.get("workspace_path", ""))
    qa_output = env.get("QA_OUTPUT_PATH", manifest.get("qa_output_path", ""))
    agh_home = Path(env["AGH_HOME"]).resolve() if env.get("AGH_HOME") else None
    provider_home = Path(paths["provider_home"]).resolve() if paths.get("provider_home") else None
    if workspace:
        roots.append(Path(workspace).resolve())
    if qa_output:
        roots.append(Path(qa_output).resolve())
    if agh_home is not None:
        roots.append(agh_home)
        # short runtime roots ($TMPDIR/aghqa-<digest>/runtime) purge at the aghqa root
        if agh_home.parent.name.startswith("aghqa-"):
            purgeable.append(agh_home.parent)
        elif agh_home.name == "runtime" and agh_home.parent.name == ".agh":
            purgeable.append(agh_home)
    if provider_home is not None:
        roots.append(provider_home)
    http_port: int | None = None
    if env.get("AGH_HTTP_PORT", "").strip().isdigit():
        http_port = int(env["AGH_HTTP_PORT"].strip())
    qa_root = Path(qa_output).resolve() / "qa" if qa_output else None
    return LabTarget(
        label=manifest.get("scenario_slug", manifest_path.parent.name),
        manifest_path=manifest_path,
        roots=roots,
        agh_home=agh_home,
        tmux_socket=Path(env["TMUX_BRIDGE_SOCKET"]).resolve() if env.get("TMUX_BRIDGE_SOCKET") else None,
        uds_path=Path(env["AGH_UDS_PATH"]).resolve() if env.get("AGH_UDS_PATH") else None,
        http_port=http_port,
        qa_root=qa_root,
        purgeable_roots=purgeable,
    )


def target_from_bare_root(root: Path) -> LabTarget:
    runtime = root / "runtime"
    agh_home = runtime if runtime.is_dir() else root
    tmux_socket: Path | None = None
    for sock in sorted(agh_home.glob("tmux-bridge*.sock")):
        tmux_socket = sock
        break
    worktree_scoped = root.name == ".agh" and "_worktrees" in root.parts
    return LabTarget(
        label=root.name,
        manifest_path=None,
        roots=[root],
        agh_home=agh_home,
        tmux_socket=tmux_socket,
        uds_path=None,
        http_port=None,
        qa_root=None,
        purgeable_roots=[] if worktree_scoped else [root],
    )


def discover_all_targets() -> list[LabTarget]:
    targets: list[LabTarget] = []
    seen_roots: set[str] = set()
    manifest_glob = str(Path.home() / "dev" / "qa-labs" / "agh-*-lab" / "qa-artifacts" / "qa" / "bootstrap-manifest.json")
    for manifest_file in sorted(glob.glob(manifest_glob)):
        try:
            target = target_from_manifest(Path(manifest_file))
        except (OSError, json.JSONDecodeError, KeyError):
            continue
        targets.append(target)
        for root in target.roots:
            seen_roots.add(str(root))
    bare_globs = [
        str(Path(tempfile.gettempdir()) / "aghqa-*"),
        str(Path(tempfile.gettempdir()) / "agh-iso-*"),
        "/tmp/aghqa-*",
        "/tmp/agh-iso-*",
    ]
    for pattern in bare_globs:
        for raw in sorted(glob.glob(pattern)):
            root = Path(raw).resolve()
            if not root.is_dir() or str(root) in seen_roots:
                continue
            already_covered = any(str(root).startswith(seen) or seen.startswith(str(root)) for seen in seen_roots)
            if already_covered:
                continue
            targets.append(target_from_bare_root(root))
            seen_roots.add(str(root))
    return targets


def teardown_target(
    target: LabTarget,
    repo_root: Path,
    grace_sec: float,
    purge: bool,
    dry_run: bool,
) -> TeardownResult:
    result = TeardownResult(label=target.label)
    agh_binary = resolve_repo_agh_binary(repo_root)
    if target.agh_home is not None and target.agh_home.is_dir():
        stop_daemon_gracefully(agh_binary, target.agh_home, result, dry_run)
    kill_tmux_server(target.tmux_socket, result, dry_run)
    pids = collect_target_pids(target)
    signal_pids(pids, grace_sec, result, dry_run)
    if purge and result.clean:
        purge_roots(target, result, dry_run)
    elif purge and not result.clean:
        result.notes.append("purge skipped: survivors remain")
    write_evidence(target, result, dry_run)
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--manifest", default="", help="Path to a bootstrap-manifest.json to tear down")
    parser.add_argument("--root", default="", help="Path to one bare isolation envelope to tear down")
    parser.add_argument("--all", action="store_true", help="Discover and tear down every known lab root")
    parser.add_argument("--purge", action="store_true", help="Remove lab runtime dirs after a clean sweep")
    parser.add_argument("--dry-run", action="store_true", help="Report actions without executing them")
    parser.add_argument("--repo-root", default=".", help="Repository root (for bin/agh graceful stop)")
    parser.add_argument("--grace-sec", type=float, default=GRACE_DEFAULT_SEC, help="SIGTERM grace before SIGKILL")
    parser.add_argument("--json", action="store_true", help="Emit machine-readable JSON summary")
    args = parser.parse_args()

    selected_modes = sum((bool(args.manifest.strip()), bool(args.root.strip()), args.all))
    if selected_modes != 1:
        parser.error("exactly one of --manifest, --root, or --all is required")

    repo_root = Path(args.repo_root).expanduser().resolve()
    targets: list[LabTarget]
    if args.all:
        targets = discover_all_targets()
    elif args.root.strip():
        root = Path(args.root).expanduser().resolve()
        if not root.is_dir() or not is_safe_root(root):
            print(f"TEARDOWN_ERROR=unsafe or missing isolation root: {root}", file=sys.stderr)
            return 1
        targets = [target_from_bare_root(root)]
    else:
        manifest_path = Path(args.manifest).expanduser().resolve()
        if not manifest_path.is_file():
            print(f"TEARDOWN_ERROR=manifest not found: {manifest_path}", file=sys.stderr)
            return 1
        targets = [target_from_manifest(manifest_path)]

    results = [
        teardown_target(target, repo_root, args.grace_sec, args.purge, args.dry_run)
        for target in targets
    ]
    all_clean = all(result.clean for result in results)

    if args.json:
        print(
            json.dumps(
                {
                    "completed_at": utc_now(),
                    "dry_run": args.dry_run,
                    "clean": all_clean,
                    "labs": [
                        {
                            "label": result.label,
                            "clean": result.clean,
                            "stopped_daemon_pid": result.stopped_daemon_pid,
                            "killed": result.killed,
                            "survivors": result.survivors,
                            "purged": result.purged,
                            "notes": result.notes,
                        }
                        for result in results
                    ],
                },
                indent=2,
                sort_keys=True,
            )
        )
    else:
        for result in results:
            print(f"TEARDOWN_LAB={result.label}")
            print(f"TEARDOWN_CLEAN={'true' if result.clean else 'false'}")
            if result.stopped_daemon_pid:
                print(f"TEARDOWN_DAEMON_PID={result.stopped_daemon_pid}")
            for entry in result.killed:
                print(f"TEARDOWN_KILLED={entry['pid']} ({entry['reason']})")
            for entry in result.survivors:
                print(f"TEARDOWN_SURVIVOR={entry['pid']} ({entry['reason']}; {entry.get('error', '')})")
            for path in result.purged:
                print(f"TEARDOWN_PURGED={path}")
            for note in result.notes:
                print(f"TEARDOWN_NOTE={note}")
        print(f"TEARDOWN_TOTAL_LABS={len(results)}")
        print(f"TEARDOWN_ALL_CLEAN={'true' if all_clean else 'false'}")

    return 0 if all_clean else 1


if __name__ == "__main__":
    sys.exit(main())

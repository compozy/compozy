#!/usr/bin/env python3
"""Stop one manifest-owned QA lab with verified process identities.

Use --manifest <path> for normal teardown; --root <path> for a bare isolation
root. --all is reserved for intentional stale-lab recovery. Evidence is written
to qa/teardown.json. Exit 0 means clean, 1 means survivors/errors, 2 means usage.
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

from qa_process_identity import (
    SignalTarget, list_processes, lsof_pids, matches_identity, pid_alive,
    port_listener_pids, process_identity, protected_pids, registered_identity, registered_process_matches, signal_pids,
)

GRACE_DEFAULT_SEC = 10.0
DAEMON_STOP_TIMEOUT_SEC = 20.0
SAFE_ROOT_MARKERS = ("/qa-labs/", "/compozyqa-", "/compozy-iso-", "/_worktrees/")


@dataclass
class LabTarget:
    label: str
    manifest_path: Path | None
    roots: list[Path]
    compozy_home: Path | None
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
    matched: list[dict] = field(default_factory=list)
    stale: list[dict] = field(default_factory=list)
    skipped: list[dict] = field(default_factory=list)
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


def read_daemon_pid(compozy_home: Path) -> int:
    info_path = compozy_home / "daemon.json"
    if info_path.is_file():
        try:
            info = load_json(info_path)
            pid = int(info.get("pid", 0))
            if pid > 0:
                return pid
        except (OSError, ValueError, json.JSONDecodeError):
            pass
    return read_pid_file(compozy_home / "daemon.lock")


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


def resolve_repo_compozy_binary(repo_root: Path) -> Path | None:
    candidate = repo_root / "bin" / "compozy"
    if candidate.is_file() and os.access(candidate, os.X_OK):
        return candidate
    return None


def stop_daemon_gracefully(
    compozy_binary: Path | None,
    compozy_home: Path,
    result: TeardownResult,
    dry_run: bool,
) -> None:
    pid = read_daemon_pid(compozy_home)
    if pid <= 0 or not pid_alive(pid):
        return
    identity = process_identity(pid)
    if identity is None or pid not in lsof_pids([compozy_home]):
        result.notes.append(f"refusing unverified daemon pid {pid}")
        return
    result.stopped_daemon_pid = pid
    if dry_run:
        result.notes.append(f"dry-run: would stop daemon pid {pid} (COMPOZY_HOME={compozy_home})")
        return
    if compozy_binary is not None and matches_identity(identity):
        env = dict(os.environ)
        env["COMPOZY_HOME"] = str(compozy_home)
        try:
            subprocess.run(
                [str(compozy_binary), "daemon", "stop"],
                check=False,
                capture_output=True,
                text=True,
                timeout=DAEMON_STOP_TIMEOUT_SEC,
                env=env,
            )
        except subprocess.TimeoutExpired:
            result.notes.append(f"compozy daemon stop timed out after {DAEMON_STOP_TIMEOUT_SEC}s for {compozy_home}")
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


def collect_target_pids(target: LabTarget, result: TeardownResult) -> dict[int, SignalTarget]:
    """Map candidate PID -> reason string."""
    candidates: dict[int, str] = {}
    stale_pids: set[int] = set()
    root_strings = [str(root) for root in target.roots if is_safe_root(root)]
    for pid, _pgid, command in list_processes():
        for root in root_strings:
            if root in command:
                candidates.setdefault(pid, f"cmdline references {root}")
                break
    for pid, name in registered_pids(target.qa_root).items():
        record = registered_identity(target.qa_root / "pids" / f"{name}.pid")
        if record is not None and matches_identity(record):
            candidates.setdefault(pid, f"verified registered process {name}.pid")
        else:
            if record is not None:
                stale_pids.add(pid)
            entry = {"pid": pid, "file": f"{name}.pid", "reason": "identity missing or changed"}
            result.stale.append(entry)
            result.skipped.append(entry)
            if not pid_alive(pid):
                (target.qa_root / "pids" / f"{name}.pid").unlink(missing_ok=True)
    lsof_scan_paths: list[Path] = [root for root in target.roots if is_safe_root(root)]
    if target.compozy_home is not None:
        lsof_scan_paths.append(target.compozy_home)
    for root in target.roots:
        name = root.name
        if name.startswith("compozyqa-") or name.startswith("compozy-iso-") or name == ".compozy":
            lsof_scan_paths.append(root)
    for pid in lsof_pids(lsof_scan_paths):
        candidates.setdefault(pid, "holds open files under lab runtime")
    for pid in protected_pids() | stale_pids:
        candidates.pop(pid, None)
    verified: dict[int, SignalTarget] = {}
    for pid, reason in candidates.items():
        identity = process_identity(pid)
        if identity is None:
            continue
        if reason.startswith("cmdline") and not any(root in identity.command for root in root_strings):
            continue
        if reason.startswith("holds open") and pid not in lsof_pids(lsof_scan_paths):
            continue
        if reason.startswith("verified registered"):
            name = registered_pids(target.qa_root).get(pid)
            record = registered_identity(target.qa_root / "pids" / f"{name}.pid")
            if identity != record:
                continue
        verified[pid] = SignalTarget(identity, reason)
        result.matched.append({"pid": pid, "reason": reason})
    if target.http_port:
        for pid in port_listener_pids(target.http_port):
            if pid not in verified and pid not in protected_pids():
                result.skipped.append({"pid": pid, "reason": "lab port alone does not prove process ownership"})
    return verified


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
        "matched": result.matched,
        "stale": result.stale,
        "skipped": result.skipped,
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
    compozy_home = Path(env["COMPOZY_HOME"]).resolve() if env.get("COMPOZY_HOME") else None
    provider_home = Path(paths["provider_home"]).resolve() if paths.get("provider_home") else None
    if workspace:
        roots.append(Path(workspace).resolve())
    if qa_output:
        roots.append(Path(qa_output).resolve())
    if compozy_home is not None:
        roots.append(compozy_home)
        # short runtime roots ($TMPDIR/compozyqa-<digest>/runtime) purge at the compozyqa root
        if compozy_home.parent.name.startswith("compozyqa-"):
            purgeable.append(compozy_home.parent)
        elif compozy_home.name == "runtime" and compozy_home.parent.name == ".compozy":
            purgeable.append(compozy_home)
    if provider_home is not None:
        roots.append(provider_home)
    http_port: int | None = None
    if env.get("COMPOZY_HTTP_PORT", "").strip().isdigit():
        http_port = int(env["COMPOZY_HTTP_PORT"].strip())
    qa_root = Path(qa_output).resolve() / "qa" if qa_output else None
    return LabTarget(
        label=manifest.get("scenario_slug", manifest_path.parent.name),
        manifest_path=manifest_path,
        roots=roots,
        compozy_home=compozy_home,
        tmux_socket=Path(env["TMUX_BRIDGE_SOCKET"]).resolve() if env.get("TMUX_BRIDGE_SOCKET") else None,
        uds_path=Path(env["COMPOZY_UDS_PATH"]).resolve() if env.get("COMPOZY_UDS_PATH") else None,
        http_port=http_port,
        qa_root=qa_root,
        purgeable_roots=purgeable,
    )


def target_from_bare_root(root: Path) -> LabTarget:
    runtime = root / "runtime"
    compozy_home = runtime if runtime.is_dir() else root
    tmux_socket: Path | None = None
    for sock in sorted(compozy_home.glob("tmux-bridge*.sock")):
        tmux_socket = sock
        break
    worktree_scoped = root.name == ".compozy" and "_worktrees" in root.parts
    return LabTarget(
        label=root.name,
        manifest_path=None,
        roots=[root],
        compozy_home=compozy_home,
        tmux_socket=tmux_socket,
        uds_path=None,
        http_port=None,
        qa_root=None,
        purgeable_roots=[] if worktree_scoped else [root],
    )


def discover_all_targets() -> list[LabTarget]:
    targets: list[LabTarget] = []
    seen_roots: set[str] = set()
    manifest_glob = str(Path.home() / "dev" / "qa-labs" / "compozy-*-lab" / "qa-artifacts" / "qa" / "bootstrap-manifest.json")
    for manifest_file in sorted(glob.glob(manifest_glob)):
        try:
            target = target_from_manifest(Path(manifest_file))
        except (OSError, json.JSONDecodeError, KeyError):
            continue
        targets.append(target)
        for root in target.roots:
            seen_roots.add(str(root))
    bare_globs = [
        str(Path(tempfile.gettempdir()) / "compozyqa-*"),
        str(Path(tempfile.gettempdir()) / "compozy-iso-*"),
        "/tmp/compozyqa-*",
        "/tmp/compozy-iso-*",
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
    compozy_binary = resolve_repo_compozy_binary(repo_root)
    if target.compozy_home is not None and target.compozy_home.is_dir() and registered_process_matches(target.qa_root, read_daemon_pid(target.compozy_home)):
        stop_daemon_gracefully(compozy_binary, target.compozy_home, result, dry_run)
    kill_tmux_server(target.tmux_socket, result, dry_run)
    pids = collect_target_pids(target, result)
    signal_pids(pids, grace_sec, result, dry_run)
    for pid_file in (target.qa_root / "pids").glob("*.pid") if target.qa_root else []:
        record = registered_identity(pid_file)
        if record is not None and not pid_alive(record.pid):
            pid_file.unlink(missing_ok=True)
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
    parser.add_argument("--repo-root", default=".", help="Repository root (for bin/compozy graceful stop)")
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
                            "matched": result.matched,
                            "stale": result.stale,
                            "skipped": result.skipped,
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

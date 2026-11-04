#!/usr/bin/env python3
"""Run paired baseline/head trials sequentially on one isolated CI machine."""
import argparse
from datetime import datetime
import json
import os
from pathlib import Path
import platform
import shutil
import statistics
import subprocess

from data import inspect_data, seed_large
from trace import write_bytes


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def copy_fixture(source, target):
    shutil.copytree(source, target)
    # A copied 1 GiB database is dirty kernel cache, unlike a cleanly stopped
    # production database. Flush fixture construction before timing SQLite's
    # first commit so that it does not pay for our unrelated copy operation.
    for path in target.rglob("*"):
        if path.is_file():
            with path.open("rb") as copied:
                os.fsync(copied.fileno())


def diagnose_failure(binary, upgraded, output):
    root = output / "diagnostic-data"
    diagnostic_error = None
    try:
        try:
            copy_fixture(upgraded, root)
            run(binary, root, "load", output, "head-diagnostic", runtime_trace=True)
        finally:
            if root.exists():
                shutil.rmtree(root)
    except (OSError, subprocess.SubprocessError, ValueError) as error:
        diagnostic_error = error
        save(output / "head-diagnostic-error.json", {"error": str(error), "type": type(error).__name__})
    raise RuntimeError("head acceptance failed; inspect results.json and per-trial logs") from diagnostic_error


def run(binary, root, mode, output, name, duration="30s", trace=False, runtime_trace=False):
    command = [str(binary), "-mode", mode, "-root", str(root), "-duration", duration]
    if runtime_trace:
        command += ["-runtime-trace", str(output / (name + ".trace"))]
    trace_file = output / (name + ".strace")
    if trace:
        command = ["strace", "--seccomp-bpf", "-f", "-yy", "-ttt", "-T", "-s", "0", "-e", "trace=write,pwrite64,fsync,fdatasync", "-o", str(trace_file)] + command
    print(name, flush=True)
    with (output / (name + ".log")).open("w") as logs, (output / (name + ".json")).open("w") as report:
        subprocess.run(command, check=True, stdout=report, stderr=logs, timeout=1800)
    result = json.loads((output / (name + ".json")).read_text())
    if mode == "load":
        end = datetime.fromisoformat(result["end"].replace("Z", "+00:00"))
        warmup_start = datetime.fromisoformat(result["warmup_start"].replace("Z", "+00:00"))
        warnings = []
        for line in (output / (name + ".log")).read_text().splitlines():
            try:
                record = json.loads(line)
                stamp = datetime.fromisoformat(record["time"].replace("Z", "+00:00"))
            except (ValueError, KeyError):
                continue
            failures = ("held for at least one second", "contention exhausted", "sqlite passive checkpoint failed",
                        "passive session wal checkpoint failed", "truncate session wal failed")
            if warmup_start <= stamp <= end and any(message in record.get("msg", "") for message in failures):
                warnings.append(record)
        result["contention_diagnostics"] = warnings
        result["transactions_per_second"] = {key: value / result["seconds"] for key, value in result["write_commit_hooks"].items()}
        if trace:
            result["file_writes"] = write_bytes(trace_file, result, str(root / "compozy.db"))
        save(output / (name + ".json"), result)
    return result


def operation_rates(result):
    return {name: sample["count"] / result["seconds"]
            for name, sample in result["latency"].items()}


def throughput_regressions(baseline, head):
    baseline_rates = [operation_rates(run) for run in baseline]
    head_rates = [operation_rates(run) for run in head]
    return {name: {"baseline": statistics.median(run.get(name, 0) for run in baseline_rates),
                   "head": statistics.median(run.get(name, 0) for run in head_rates)}
            for name in set().union(*baseline_rates)
            if statistics.median(run.get(name, 0) for run in head_rates)
            < statistics.median(run.get(name, 0) for run in baseline_rates)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline", type=Path, required=True)
    parser.add_argument("--head", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--baseline-sha", required=True)
    parser.add_argument("--head-sha", required=True)
    args = parser.parse_args()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=False)
    fixture = args.output / "baseline-data"
    upgraded = args.output / "head-data"
    run(args.baseline, fixture, "seed", args.output, "seed")
    seed = seed_large(fixture, 1024**3)
    save(args.output / "seed-size.json", seed)
    before = inspect_data(fixture)
    save(args.output / "upgrade-before.json", before)
    retention_rows = before["retention_eligible_rows"]
    if retention_rows < 20000:
        raise RuntimeError(f"fixture lacks representative retention work: {retention_rows} rows")
    baseline_view = run(args.baseline, fixture, "inspect", args.output, "baseline-observe")
    copy_fixture(fixture, upgraded)
    head_view = run(args.head, upgraded, "inspect", args.output, "head-upgrade")
    after = inspect_data(upgraded)
    save(args.output / "upgrade-after.json", after)
    for key in ("all_columns", "sha256", "rows", "max_seq", "high_water"):
        if before[key] != after[key]:
            raise RuntimeError(f"upgrade changed {key}: {before[key]} != {after[key]}")
    if baseline_view["observe"] != head_view["observe"]:
        raise RuntimeError("Observe API repository results changed")
    reopened = run(args.head, upgraded, "inspect", args.output, "head-reopen")
    if reopened["observe"] != head_view["observe"]:
        raise RuntimeError("Observe changed on second reopen")
    trials = {"baseline": [], "head": []}
    # Alternate order to expose rather than hide runner drift and cache effects.
    for trial in range(3):
        for label in (("baseline", "head") if trial % 2 == 0 else ("head", "baseline")):
            source = fixture if label == "baseline" else upgraded
            root = args.output / "trial-data"
            copy_fixture(source, root)
            try:
                trials[label].append(run(getattr(args, label), root, "load", args.output, f"{label}-{trial}"))
            finally:
                shutil.rmtree(root)
    traced = {}
    for label, source in (("baseline", fixture), ("head", upgraded)):
        root = args.output / "trial-data"
        copy_fixture(source, root)
        try:
            traced[label] = run(getattr(args, label), root, "load", args.output, f"{label}-writes", duration="15s", trace=True)
        finally:
            shutil.rmtree(root)
    environment = {"platform": platform.platform(), "cpu_count": os.cpu_count(), "baseline": args.baseline_sha,
                   "head": args.head_sha, "python_sqlite": __import__("sqlite3").sqlite_version,
                   "go": subprocess.check_output(["go", "version"], text=True).strip(),
                   "disk": subprocess.check_output(["df", "-h", str(args.output)], text=True),
                   "cpu": Path("/proc/cpuinfo").read_text(), "memory": Path("/proc/meminfo").read_text()}
    result = {"environment": environment, "seed": seed, "upgrade": {"rows": before["rows"], "sha256": before["sha256"],
              "baseline_open_ms": baseline_view["open_ms"], "upgrade_ms": head_view["open_ms"], "reopen_ms": reopened["open_ms"]},
              "trials": trials, "traced": traced}
    regressions = {"untraced": throughput_regressions(trials["baseline"], trials["head"]),
                   "traced": throughput_regressions([traced["baseline"]], [traced["head"]])}
    result["throughput_regressions"] = regressions
    save(args.output / "results.json", result)
    lines = ["# SQLite contention paired experiment", "", f"Baseline `{args.baseline_sha}`; head `{args.head_sha}`.", "",
             "30 sessions; 1,000 completed turns each; 20 Hz transcript chunks, 5 Hz transcript reads,",
             "4 Hz health and 2 Hz summaries/state per session; 20 Hz catalog reads; 1,500 model rows",
             "and 90 process observations refreshed every two seconds. Each measured phase also",
             f"sweeps {retention_rows:,} retained summaries older than the fixed six-hour boundary.",
             "Five-second warmup, three",
             "30-second untraced trials; separate 15-second strace trials for actual file-write bytes.", "",
             "| Metric | Baseline | Head |", "| --- | ---: | ---: |"]
    for metric in ("transcript_read", "catalog_read"):
        values = {label: [r["latency"][metric]["p99_ms"] for r in runs] for label, runs in trials.items()}
        lines.append(f"| {metric} p99 ms (median; three values) | {statistics.median(values['baseline']):.3f}; {values['baseline']} | {statistics.median(values['head']):.3f}; {values['head']} |")
    for name in sorted(set().union(*(operation_rates(run) for run in trials["baseline"]))):
        values = {label: statistics.median(operation_rates(run).get(name, 0) for run in runs)
                  for label, runs in trials.items()}
        lines.append(f"| {name} completed operations/s (median) | {values['baseline']:.2f} | {values['head']:.2f} |")
    for kind in ("core", "telemetry"):
        values = {label: statistics.median(r["transactions_per_second"][kind] for r in runs) for label, runs in trials.items()}
        lines.append(f"| {kind} write commit hooks/s | {values['baseline']:.2f} | {values['head']:.2f} |")
    for kind in ("core", "telemetry", "sessions"):
        lines.append(f"| {kind} file-write bytes/min (traced) | {traced['baseline']['file_writes']['bytes_per_minute'][kind]:.0f} | {traced['head']['file_writes']['bytes_per_minute'][kind]:.0f} |")
    lines += ["", "Throughput regressions (untraced medians and traced pair): " + json.dumps(regressions), ""]
    for label, runs in trials.items():
        lines += ["", f"{label}: {sum(sum(map(len,r['failures'].values())) for r in runs)} operation failures; {sum(len(r['contention_diagnostics']) for r in runs)} long-hold/contention warnings."]
        lines += [f"Retention rows removed: {[r['retention']['DeletedEventSummaries'] for r in runs]}."]
    lines += ["", f"Upgrade: {before['rows']:,} rows, {seed['logical_bytes']:,} core bytes; all-column SHA-256 `{before['sha256']}` unchanged.",
              f"Open/upgrade/reopen: {baseline_view['open_ms']:.1f}/{head_view['open_ms']:.1f}/{reopened['open_ms']:.1f} ms.",
              "Observe results, surviving sequence and retired high-water mark match. Index pages/plans are in upgrade-before.json and upgrade-after.json.", "",
              "Commit hooks execute before SQLite commits; rollback hooks and operation errors are reported separately. No failed transaction is silently counted as successful.",
              "Session commit counts are not instrumented: the guarded session connector owns a separate driver. Session file-write bytes are captured by the syscall tracer.",
              "Fixture copies are fsynced before launch. Errors and long-hold warnings include warmup; latency and file-write rates use the measured interval.",
              "File-write bytes sum successful write/pwrite64 return values for each .db and -wal, including clean drain/checkpoint. They exclude shm mmap dirtying and physical filesystem/device amplification.",
              "Traced timings are excluded from the latency comparison. These are synthetic store measurements, not HTTP/browser or live-provider latency guarantees."]
    (args.output / "report.md").write_text("\n".join(lines) + "\n")
    # Preserve receipts even if an acceptance condition fails.
    head_runs = trials["head"] + [traced["head"]]
    invalid = [r for r in head_runs if r["failures"] or r["contention_diagnostics"]]
    incomplete_retention = [r for r in head_runs
                            if r["retention"]["DeletedEventSummaries"] != retention_rows]
    if invalid or incomplete_retention or any(regressions.values()):
        # Keep failed acceptance unchanged. A separate diagnostic run attributes
        # time to Go scheduling, GC, locks and syscalls; it never replaces a trial.
        diagnose_failure(args.head, upgraded, args.output)
    shutil.rmtree(fixture)
    shutil.rmtree(upgraded)


if __name__ == "__main__":
    main()

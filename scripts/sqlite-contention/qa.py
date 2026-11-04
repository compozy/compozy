#!/usr/bin/env python3
"""Bootstrap, exercise and tear down one real-daemon load lab in CI."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shlex
import subprocess


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--driver", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[2]
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=False)
    bootstrap = repo / ".agents/skills/eng/eng-qa-bootstrap/scripts/bootstrap-qa-env.py"
    command = ["python3", str(bootstrap), "--scenario", "sqlite-contention", "--repo-root", str(repo),
               "--workspace-root", str(args.output / "labs"), "--profile", "targeted",
               "--required-surface", "api", "--required-surface", "cli", "--required-surface", "runtime"]
    initialized = subprocess.run(command, check=True, capture_output=True, text=True)
    (args.output / "bootstrap.log").write_text(initialized.stdout + initialized.stderr)
    values = dict(line.split("=", 1) for line in initialized.stdout.splitlines() if "=" in line)
    manifest_path = Path(values["BOOTSTRAP_MANIFEST"])
    manifest = json.loads(manifest_path.read_text())
    env = manifest["env"]
    scratch = Path(env["QA_OUTPUT_PATH"])
    qa = scratch / "qa"
    (args.output / "manifest-path.txt").write_text(str(manifest_path) + "\n")
    result = 1
    try:
        charter_path = Path(env["BEHAVIORAL_CHARTER"])
        charter = json.loads(charter_path.read_text())
        charter.update(startup_situation="Thirty sessions stream while the operator reads transcripts and the catalog.",
                       operator_intent="Keep accepted messages, renames and read routes responsive under background writes.",
                       expected_business_outcome="All thirty streams complete and accepted content survives clean daemon restart.",
                       cross_surface_targets=[{"surface": plane, "object": "session"} for plane in ("api", "cli", "runtime")],
                       disruption_probes=[{"name": "clean restart", "expected": "all completed transcript content retained"}],
                       artifacts=["daemon-result.json", "http-samples.json", "daemon-initial.log", "daemon-reopened.log"])
        charter["provider_plan"] = {"required": False, "providers": [],
                                    "reachability_probe": "Scripted ACP peer; bounded daemon transport and storage load only.",
                                    "fallback_boundary": "Live-provider latency and authentication are outside this experiment."}
        write_json(charter_path, charter)
        process_env = dict(os.environ, COMPOZY_QA_BOOTSTRAP_MANIFEST=str(manifest_path),
                           COMPOZY_TEST_DAEMON_BIN=str(args.binary.resolve()),
                           COMPOZY_TEST_ACPMOCK_DRIVER_BIN=str(args.driver.resolve()))
        verify_log = scratch / "daemon-verify.log"
        with verify_log.open("w") as log:
            completed = subprocess.run(["go", "test", "-p", "2", "-parallel", "1", "-tags", "contentionload",
                                        "./internal/testutil/e2e", "-run", "^TestSQLiteContentionDaemonLoad$",
                                        "-count=1", "-timeout=12m", "-v"], cwd=repo, env=process_env,
                                       stdout=log, stderr=subprocess.STDOUT, timeout=900)
        result = completed.returncode
        result_path = scratch / "daemon-result.json"
        session_ids = json.loads(result_path.read_text())["sessions"] if result_path.exists() else []
        with Path(env["JOURNEY_LOG"]).open("a") as journey:
            for surface, action, evidence in (
                ("api", "stream_and_read", "http-samples.json"),
                ("cli", "status_and_observe", "after-restart-status.json"),
                ("runtime", "restart_and_verify", "daemon-result.json"),
                ("runtime", "final_verify", "daemon-verify.log"),
            ):
                if (scratch / evidence).exists():
                    journey.write(json.dumps({"timestamp": datetime.now(timezone.utc).isoformat(), "surface": surface,
                                              "action": action, "actor": "operator", "ids": session_ids,
                                              "evidence_path": str(scratch / evidence),
                                              "outcome": "completed" if result == 0 else "failed"}) + "\n")
        verdict = "PASS" if result == 0 else "FAIL"
        (qa / "verification-report.md").write_text(
            f"# SQLite daemon transport load\n\n{verdict}: bounded HTTP/SSE, UDS and CLI exercise.\n\n"
            "Thirty scripted ACP streams; two-second interactive route deadlines; clean restart and retained transcripts/health.\n"
            "No live provider or browser UX claim. Operator credentials and runtime state were not used.\n\n"
            f"Final focused CI verification: {verify_log}\n"
            f"Behavioral evidence: {scratch / 'daemon-result.json'}\n"
        )
        audited = subprocess.run(["python3", env["AUDIT_COMMAND"], "--qa-output-path", str(scratch), "--strict"], cwd=repo)
        result = result or audited.returncode
    finally:
        torn_down = subprocess.run(shlex.split(env["TEARDOWN_COMMAND"]), cwd=repo)
        result = result or torn_down.returncode
        try:
            teardown = json.loads((qa / "teardown.json").read_text())
        except (OSError, ValueError):
            teardown = {"clean": False}
        if not isinstance(teardown, dict):
            teardown = {"clean": False}
        if not teardown.get("clean"):
            result = result or 1
        # Receipts stay in the lab; the workflow uploads only these files, never databases or homes.
        write_json(args.output / "result.json", {"exit_code": result, "manifest": str(manifest_path),
                                                "qa_output": str(scratch), "teardown_clean": teardown.get("clean")})
    if result:
        raise SystemExit(result)


if __name__ == "__main__":
    main()

---
id: MS-daemon-memory-reporting
area: MS
title: Configure and inspect daemon memory reports
persona: Dora
journey: J-drain-daemon-safely
expected: With daemon.memory_report_interval above zero, AGH emits baseline, periodic, and joined-shutdown process-memory snapshots and exposes the same latest runtime.memory evidence through HTTP, UDS, and agh doctor -o json. Setting the interval to 0s requires a daemon restart, starts no periodic worker, and produces an explicit disabled diagnostic.
entry_points: Web General Settings; config.toml; agh config; HTTP/UDS GET /api/doctor; agh doctor -o json; daemon logs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-002
---

Set the interval to a short positive duration, restart, and compare two `[memory]` log records with
the `runtime.memory` item from HTTP, UDS, and CLI. Confirm heap, goroutine, uptime, and resident
fields are populated; on macOS, confirm `resident_memory_kind=peak` is not presented as current
use. Set the value to `0s`, restart, confirm no periodic record appears, and verify doctor reports a
deterministic disabled state. Stop the daemon and confirm teardown joins cleanly.

QA impact 2026-07-15: new config, Web, logs, lifecycle, and doctor behavior. Planning flag only; no
QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Dora and linked to J-drain-daemon-safely;
companion to the §3.5 memory-observability probe. Forensic contract (SD-006): timestamped `[memory]`
log records compared with `runtime.memory` from HTTP, UDS, and `agh doctor -o json`; the `0s`
disabled diagnostic; and the clean teardown join.

src: .compozy/tasks/hermes-comparison/_techspec.md#35-reliability-adr-010-fixes-d5

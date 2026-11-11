---
id: RT-compozy-global-database
area: RT
title: Create the Compozy global database without changing session storage
persona: Bruno
journey: J-validate-compozy-hard-cut
expected: A fresh daemon creates compozy.db in the resolved home, keeps each session database named events.db, reports the global path consistently, and never creates or opens the retired global database.
entry_points: compozy daemon start; compozy status -o json; global and session database files on disk
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/bootstrap-manifest.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/api-status.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log;/Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-refuse-legacy-database; RT-compozy-home-layout
---

QA impact 2026-07-26: the global database filename moved to `compozy.db` while the
brand-neutral per-session `events.db` contract stayed unchanged. Planning flag only;
the next QA cycle owns fresh-home inspection and structured-path parity.

QA impact 2026-07-28: live read-only probes now participate in SQLite locking and WAL
visibility instead of asserting that an actively checkpointed global database is
immutable. Stale verdict reset to untested; planning flag only, with historical evidence
preserved and no QA replay in this change.

QA impact 2026-11-11 (#1465): telemetry WAL maintenance now reclaims allocation above
16 MiB with a nonwaiting TRUNCATE attempt after its periodic PASSIVE checkpoint.
This is a reclamation threshold, not a hard cap: active readers can retain required frames.
On temporary synthetic databases, hold an observation snapshot while appending events;
verify checkpoint progress stalls while ingestion continues. Release the reader, require
backfill and allocation reclamation on a later maintenance tick, then close and reopen
and verify every acknowledged event and `integrity_check`. Keep FULL ingestion sync.
The real SQLite walks in `TestStoreSQLiteHelpers` and `TestGlobalDBTelemetryDelivery`
passed; [the investigation](../reports/2026-11-11-telemetry-wal.md) records byte/frame
measurements and shutdown evidence. These checks do not claim a desktop or production-home walk.

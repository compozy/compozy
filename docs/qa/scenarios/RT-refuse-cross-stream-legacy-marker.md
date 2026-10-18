---
id: RT-refuse-cross-stream-legacy-marker
area: RT
title: Refuse either legacy marker in the shared database
persona: Bruno
journey: J-operate-daemon-schema
expected: Every opener of the shared compozy.db refuses it before mutation when either legacy migration marker exists, regardless of which marker is present.
entry_points: compozy daemon start; compozy daemon start --foreground; compozy extension list -o json; compozy provider auth status <bound-secret-provider> -o json
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-refuse-legacy-database; RT-refuse-legacy-cli-open
---

Implementation peer review found that each migration stream only recognized its own pre-Goose marker even
though both streams share `compozy.db`. The open preflight now treats `schema_migrations` and
`memory_schema_migrations` as database-wide legacy evidence. The next targeted cycle must walk both cross-stream
combinations and confirm byte-identical refusal through daemon startup and at least one local CLI opener.

QA impact 2026-10-07 (memory removal): the memory stream and its opener are gone; the global stream is the only owner of the shared database, and its legacy-table list still names both pre-Goose markers (`schema_migrations` and `memory_schema_migrations`, `internal/store/globaldb/migration_stream.go`), so the preflight keeps treating either one as database-wide legacy evidence. The cross-stream combinations reduce to each marker alone against the global opener: the refusal wraps `legacy_database`, logs `store.migrations.refused` with `reason: legacy_database`, and leaves the database bytes unchanged; the rest of the walk is unchanged.

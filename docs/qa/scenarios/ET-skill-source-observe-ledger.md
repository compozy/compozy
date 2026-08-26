---
id: ET-skill-source-observe-ledger
area: ET
title: Prove what a source or exposure change did from the durable ledger
persona: Dora
journey: J-operate-skill-sources-headless
expected: Each source, scan, and exposure lifecycle path leaves exactly its own durable event with correlation keys, a discarded generation is recorded as superseded and never as applied, and per-suppression decisions never enter the ledger
entry_points: compozy logs --type skills.sources.applied|skills.sources.superseded|skills.sources.apply_failed --component skill; compozy logs --type skills.scan.truncated|skills.scan.link_skipped --component skill; compozy logs --type skills.exposure.created|skills.exposure.removed|skills.exposure.operation_failed|skills.exposure.broken_detected|skills.exposure.cleanup_failed --component skill; GET /api/logs over HTTP or UDS; harness suppression diagnostics
qa_status: pass
bug_ids: BUG-20260825-skill-source-event-omits-custom-roots
fix_status: fixed
retest_status: pass
fix_commits: e7dffdb74
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/logs-skill-http.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/logs-skill-cli.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/live-apply-summary.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-skill-source-agent-parity; ET-live-skill-source-reload; ET-skill-exposure-lifecycle; ET-skill-source-diagnostics-cli
---

Drive each lifecycle path once, then read the operational ledger back and confirm it tells the same
story the surfaces told. A successful apply leaves `skills.sources.applied` carrying scope,
generation, and per-source root counts, emitted only on a successful commit. A validation or commit
failure leaves `skills.sources.apply_failed` with its error class and no `applied` record. A scanner
pass over an over-cap root leaves `skills.scan.truncated` with `root_id`, path, scanned count, and
cap; a skipped first-level link leaves `skills.scan.link_skipped` with `root_id`, path, and reason.
Expose and unexpose leave `skills.exposure.created` and `skills.exposure.removed` per target with
skill, target, and link path; a preflight or commit failure leaves
`skills.exposure.operation_failed`; reconcile that finds a damaged or foreign entry leaves
`skills.exposure.broken_detected` with its status; a rollback or reconcile that cannot finish leaves
`skills.exposure.cleanup_failed`.

Force the fence: land two source writes so the second wins while the first is still in flight.
The discarded generation must appear as `skills.sources.superseded` naming the discarded and winning
generations, and must never also appear as `skills.sources.applied`. Confirm every event carries the
base correlation keys — `config_generation`, `actor_kind`, `actor_id`, and `workspace_id` where the
change is workspace-scoped — and that the durable append happened before the live revision or SSE
broadcast derived from the same change, not after.

Then confirm the deliberate absence. Per-suppression decisions are high-cadence and session-scoped:
`skills.injection.suppressed` is a log record carrying `session_id`, `skill`, `source`, and
`provider`, readable in harness diagnostics. It must not appear in the durable ledger. A suppression
record found through the event query is a contract violation, not a bonus — the ledger would grow
without bound and the operator meter would stop being readable.

There is no `compozy observe` verb for these events on this branch; `compozy observe overview` is a
different read model. Read them through `compozy logs --type … --component skill -o json` and
`GET /api/logs`, and compare the CLI and the route for the same window.

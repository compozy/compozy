---
id: RT-observe-overview-cli
area: RT
title: compozy observe overview parity across output modes and transports
persona: Ada
journey: J-operate-daemon-schema
expected: `compozy observe overview -o json` prints the raw `overview` payload (schema_version observe-overview/v1) identical to `GET /api/observe/overview` over HTTP and UDS; `-o jsonl` emits one line per section (attention, today, outcomes, usage, pulse, system, freshness); human output renders Needs you / Today / Outcomes / Usage / Pulse / System sections; `--workspace` scopes aggregates and `--usage-window` accepts only 7|30|90 (422 otherwise); attention `actions` list only daemon-accepted verbs.
entry_points: `compozy observe overview`; `GET /api/observe/overview` (HTTP+UDS)
qa_status: pass
bug_ids: BUG-20260729-overview-json-parity; BUG-20261002-overview-zero-window; BUG-20261002-task-action-profile-scope
fix_status: fixed
retest_status: pass
fix_commits: 351f3535; 46d8b2f07; 8d1e73ab3
evidence: docs/qa/evidence/2026-10-02-untested/overview-approval-cli.json; docs/qa/evidence/2026-10-02-untested/overview-approval-http.json; docs/qa/evidence/2026-10-02-untested/overview-approval-uds.json; docs/qa/evidence/2026-10-02-untested/overview-zero-fixed.json; docs/qa/evidence/2026-10-02-untested/task-profile-fixed-approve.json; docs/qa/evidence/2026-10-02-untested/task-profile-fixed-reject.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps:
---

story: As an agent or operator I read the same home overview the web renders, from the CLI, in machine-readable form.

New verb shipped 2026-07-23 — the first `compozy observe` command; the spec registry exposes the operation on both transports and the generated CLI reference gained `observe/` pages.

2026-07-29 QA found that the CLI inserted `resolution_source` into the otherwise transport-identical
overview payload. The root fix and real three-surface replay are staged; the scenario remains failed
until the governed fix commit exists and the original persona retest is recorded.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

QA 2026-10-02: output modes and valid windows retain parity, but explicit CLI zero silently
selects 30 days while HTTP and UDS refuse it. See BUG-20261002-overview-zero-window.

Populated approval rows also retain JSON/JSONL/HTTP/UDS parity and workspace isolation, but CLI
approve/reject lose the selected profile. The live API accepts the same approval. Repair and replay
of BUG-20261002-task-action-profile-scope are required before this scenario is settled.

QA completion 2026-10-02: explicit-zero validation and owner-scoped CLI actions are repaired.
Fresh reads retain all output-mode and transport contracts; approve/reject succeed and disappear
from attention, while a foreign-profile mutation still refuses. The affected local gate passed.

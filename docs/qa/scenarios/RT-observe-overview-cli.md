---
id: RT-observe-overview-cli
area: RT
title: compozy observe overview parity across output modes and transports
persona: Ada
journey: J-operate-daemon-schema
expected: `compozy observe overview -o json` prints the raw `overview` payload (schema_version observe-overview/v1) identical to `GET /api/observe/overview` over HTTP and UDS; `-o jsonl` emits one line per section (attention, today, outcomes, usage, pulse, network, system, freshness); human output renders Needs you / Today / Outcomes / Usage / Pulse / System sections; `--workspace` scopes aggregates and `--usage-window` accepts only 7|30|90 (422 otherwise); attention `actions` list only daemon-accepted verbs.
entry_points: `compozy observe overview`; `GET /api/observe/overview` (HTTP+UDS)
qa_status: pass
bug_ids: BUG-20260729-overview-json-parity
fix_status: fixed
retest_status: pass
fix_commits: 351f3535
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/003-structured-read-contracts
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps:
---

story: As an agent or operator I read the same home overview the web renders, from the CLI, in machine-readable form.

New verb shipped 2026-07-23 — the first `compozy observe` command; the spec registry exposes the operation on both transports and the generated CLI reference gained `observe/` pages.

2026-07-29 QA found that the CLI inserted `resolution_source` into the otherwise transport-identical
overview payload. The root fix and real three-surface replay are staged; the scenario remains failed
until the governed fix commit exists and the original persona retest is recorded.

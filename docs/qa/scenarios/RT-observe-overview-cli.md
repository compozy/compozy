---
id: RT-observe-overview-cli
area: RT
title: agh observe overview parity across output modes and transports
persona: Operator/Agent
journey:
expected: `agh observe overview -o json` prints the raw `overview` payload (schema_version observe-overview/v1) identical to `GET /api/observe/overview` over HTTP and UDS; `-o jsonl` emits one line per section (attention, today, outcomes, usage, pulse, network, system, freshness); human output renders Needs you / Today / Outcomes / Usage / Pulse / System sections; `--workspace` scopes aggregates and `--usage-window` accepts only 7|30|90 (422 otherwise); attention `actions` list only daemon-accepted verbs.
entry_points: `agh observe overview`; `GET /api/observe/overview` (HTTP+UDS)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/cli/observe.go; internal/api/core/observe_overview_handlers.go; internal/api/spec/registry_observe_overview.go
last_report:
overlaps:
---

story: As an agent or operator I read the same home overview the web renders, from the CLI, in machine-readable form.

New verb shipped 2026-07-23 — the first `agh observe` command; the spec registry exposes the operation on both transports and the generated CLI reference gained `observe/` pages.

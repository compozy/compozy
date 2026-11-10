---
id: RT-session-cost-provenance
area: RT
title: Session usage reports truthful cost provenance
persona: Rafa
journey: J-14
expected: The session Context sidebar and structured usage response agree on actual provider cost, exact five-bucket model-catalog estimates, native-subscription inclusion, or unknown cost; every nonzero bucket requires its own rate, included and unknown never display a fabricated amount, and every state identifies its source.
entry_points: web session Context sidebar; `compozy session usage <session-id> -o json`; `GET /api/workspaces/:workspace_id/sessions/:session_id/usage`
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: unpublished-local-artifact/qa;docs/qa/reports/2026-09-12-session-context.md
last_report: docs/qa/reports/2026-09-12-session-context.md
overlaps:
---

Exercise one finished session for each available provenance path: `actual/agent_reported`,
`estimated/catalog_config|models_dev|builtin`, `included/none`, and `unknown/none`. Reload the
session and confirm the Context sidebar, CLI output, and fresh structured response remain consistent. Treat a
missing native account-usage probe as `included` only when the active auth mode proves a native
subscription; otherwise require `unknown` with no amount.

For estimated coverage, exercise nonzero input, output, cache-read, cache-write, and reasoning in one
update with five explicit compatible rates, then remove one active-bucket rate and require
`unknown/none` with no amount. No cache→input or reasoning→output substitution is allowed.

Phase C planning 2026-07-19: settles US-007 (U1, ADR-006) together with
TA-task-run-cost-provenance and the five-rate companion resets (ET-model-source-five-rate-pricing,
MS-042, MS-045, MS-055, MS-056, ET-053).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Silent-agent session showing estimated cost with its badge (screenshot) and the same via
  `compozy session usage <id> -o json` + HTTP parity.
- Subscription-auth fixture showing `included` with no `$` amount; missing-rate run showing
  `unknown` with no amount and no error.
- Reference to the recorded per-provider account-usage viability determination
  (`analysis/account-usage-token-reachability.md`, ADR-006 §5 — fetcher dropped).

Session-context integration: compare the cache-read/cache-write tiles with the aggregate and per-turn rows. Unknown and unavailable context must not change the independently reported cost provenance. Final execution is owned by session-context tasks 05/06.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.

## Cumulative cost accounting (2026-11-09)

Run three turns reporting cumulative USD amounts 1, 3 and 4. Confirm per-turn costs 1, 2 and 1
in usage/turns, transcript usage parts and the live prompt stream, with session/task/daily total 4.
Release and resume the runtime, then restart the daemon: a restored report 5 contributes 1;
a lower report 0.5 contributes zero, and the next report 2 contributes 1.5. Also verify a lagging
restore: `10, 12, 11.5, 12.5` produces `10, 2, 0, 1`, including across runtime restart.
An explicitly per-turn adapter keeps its reported amounts.
Upgrade a prior database with complete and missing session ledgers. Recomputed rows become actual;
missing/partial histories become unknown with no amount. Token counts, turn counts and timestamps
remain byte/value equivalent. Reopening must not repeat the repair.

Focused SQLite migration and prompt-contract tests are recorded in the harness efficiency report.
A fresh browser/HTTP/UDS scenario walk is not claimed by those tests.

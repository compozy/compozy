---
id: RT-session-cost-provenance
area: RT
title: Session usage reports truthful cost provenance
persona: Rafa
journey: J-14
expected: The session Usage surface and structured usage response agree on actual provider cost, exact five-bucket model-catalog estimates, native-subscription inclusion, or unknown cost; every nonzero bucket requires its own rate, included and unknown never display a fabricated amount, and every state identifies its source.
entry_points: web session inspector Usage tab; `agh session usage <session-id> -o json`; `GET /api/workspaces/:workspace_id/sessions/:session_id/usage`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Exercise one finished session for each available provenance path: `actual/agent_reported`,
`estimated/catalog_config|models_dev|builtin`, `included/none`, and `unknown/none`. Reload the
session and confirm the Web inspector, CLI output, and fresh structured response remain consistent. Treat a
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
  `agh session usage <id> -o json` + HTTP parity.
- Subscription-auth fixture showing `included` with no `$` amount; missing-rate run showing
  `unknown` with no amount and no error.
- Reference to the recorded per-provider account-usage viability determination
  (`analysis/account-usage-token-reachability.md`, ADR-006 §5 — fetcher dropped).

# BUG-20261005-loop-usage-counts-human-wait: Usage counts parked review time against the wall budget

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Marina
- **Journey Step:** J-03, inspect the finished review after human approval
- **Scenarios:** LP-009; LP-web-run-default-read-briefing
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction and evidence

Run `studio-onboarding-review` v2, request changes in round 1, then approve round 2 after
waiting. Run `looprun-b6d9838c4fb9f0b9` finishes Done. Its Usage rail says `6m 36s / 10m`,
although the daemon has excluded the human waits: `started_at=13:14:38.097145Z` and
`completed_at=13:14:38.200690Z`. Creation was `13:08:02.044561Z` on 2026-10-05.

`loops-human-review-reconnect.json`, `loops-human-review-settled-to-runs.json`, and
`loops-human-review-done-uds.json` preserve the UI and independent daemon proof under
`docs/qa/evidence/2026-10-02-untested/`. The later read confirms the usage value persists
after roster and timeline reconciliation; this is not a transient reconnect observation.

## Root cause and repair

The page reuses creation-to-completion duration for budget usage in non-running states.
The daemon shifts `started_at` to exclude parked intervals. Preserve the roster's elapsed
duration and use the budget clock for Usage and its approval fallback facts.
The existing `loop-run-usage.test.ts` suite owns the projection invariant.

Fix commit: pending delivery gate.
The five non-running budget-clock cases pass in the existing usage suite. On the fresh
finished review page, Usage is 0m00s / 10m, matching the independent 0.103545-second
active span; the roster's elapsed duration retains its separate meaning. Evidence:
loops-human-review-usage-fixed.png and loops-human-review-done-uds.json.

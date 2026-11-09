---
type: improvement
created: 2026-11-09 18:45
source: compozy/compozy · branch harness-token-efficiency · follows 20261109-1726_harness-token-efficiency.md and its report
---
# Round 2: unblock W10 (cost accounting) and W8 (idle compaction) with operator decisions, for every ACP adapter

## Problem

Round 1 left W6, W8 and W10 blocked, each on a decision. The operator has now decided W10 and W8.
Both decisions carry the same condition: the solution must work for every ACP adapter CompozyOS
supports, not only Claude. W6 is still undecided — do not start it. W9 stays as shipped. No pull
request is opened until every workstream is settled.

## Operator decisions

- **W10 — approved:** correct cost by storing per-turn increments, repair the history from the
  per-session ledgers, and mark as unknown whatever cannot be recomputed. A documented, rare residual
  error is accepted in exchange for removing the current ~200× inflation.
- **W8 — approved, on by default:** CompozyOS must let a provider adapter's own warm-cache idle
  compaction run instead of releasing the runtime before it fires. How it is done must follow from a
  survey of the other adapters.

## Current State

Everything in the round 1 brief and report still applies. Facts established since:

- ACP defines `Cost.amount` as cumulative (round 1 report, W10). `internal/observe/cost_estimation.go`
  passes the amount through and `internal/store/globaldb/queries/observe.sql` /
  `observe_overview.sql` add it to the running totals. The per-session `token_usage` ledger stores the
  raw reported amount per turn, so it holds the cumulative series (for one session: 1.34, 1.51, 2.46 …
  max 300.9, while `token_stats.total_cost` reads 59,163.13).
- Claude's counter can restart: resumes began at zero before CLI 2.1.277, newer versions restore the
  saved total, and a crash can report zeroed totals.
- `@agentclientprotocol/claude-agent-acp` 0.102.0 ships `dist/idle-compaction.js`: one instance per
  session, armed when the session goes idle, deadline = last main-thread request + 54 minutes
  (`CLAUDE_AGENT_ACP_IDLE_COMPACT_DELAY_MS` overrides it), requires a one-hour cache TTL, context at or
  above `CLAUDE_CODE_IDLE_COMPACT_MIN_TOKENS` (default 200000, floor 100000), not busy, rate limit
  allowed, and not disabled by `idleCompaction: false`, `autoCompactEnabled: false`, `DISABLE_COMPACT`
  or `DISABLE_AUTO_COMPACT`. It fires `/compact` through the adapter's own prompt path and skips when
  it is more than 60 seconds past its deadline. It is enabled by default.
- CompozyOS releases an idle runtime after `session.supervision.idle_release_after` (30m in the
  measured installation; `internal/session/manager_idle_release.go`, `work_supervision.go`), which
  kills the adapter process and its timer. In the measured data, sessions idle for more than 65
  minutes resumed with the whole context rewritten (190 occurrences, ~425k tokens each) and only 37
  compactions happened across 1,715 sessions.

## Requirements

**W10 — per-turn cost for every adapter.**
- Normalize where usage is observed, for every ACP adapter. The default semantics follow the ACP
  schema: the reported amount is cumulative for the session's counter, and a turn's cost is the
  reported amount minus the last amount recorded for that session.
- Counter restarts: a reported amount lower than the last recorded amount starts a new counter, and
  that turn's cost is the reported amount. A restored counter (amount not lower) continues. State the
  residual ambiguity in the docs and the report: a restart whose first amount exceeds the previous
  total is indistinguishable from a continuation.
- Survey every adapter CompozyOS ships a builtin provider for and that can report cost. Establish
  from the adapter source or documentation whether each reports a cumulative or a per-turn amount.
  An adapter that reports per-turn amounts is declared explicitly as such in provider/harness data and
  keeps working; an unknown adapter follows the ACP schema. Include the survey table, with sources, in
  the report.
- The baseline survives daemon restart, runtime release and resume.
- Every reader that presents a per-turn cost shows the turn's cost, not the cumulative amount: check
  the CLI usage views, transcript usage messages, task runtime views and the prompt-stream payloads.
- History: `token_stats.total_cost` and `token_usage_daily.total_cost` are recomputed with the same
  rule from the per-session ledgers where those exist; rows that cannot be recomputed become unknown
  cost. No inflated total may remain presented as `actual`. Every other column is preserved. Ship the
  migration and any backfill through the repository's persistence rules, and state the mechanism.
- Regression coverage: the sum of recorded turn costs equals the provider's final cumulative amount
  for a multi-turn session, across a restart with a restored counter and across a restart with a reset
  counter; a per-turn adapter is unaffected; the migration has fresh/reopen/equivalence coverage.

**W8 — idle release must not pre-empt an adapter's idle compaction.**
- Survey every supported ACP adapter: how its prompt cache expires, whether the adapter or its CLI
  performs idle or warm-cache compaction, under which conditions, and with which controls. Include the
  table, with sources, in the report.
- On by default: when an adapter would compact an idle session before its cache expires, CompozyOS
  does not release that session's runtime first. Release proceeds once the compaction finished or its
  deadline lapsed. The hold has a bounded maximum.
- A session the adapter would not compact (below the adapter's context threshold, wrong cache TTL,
  mechanism disabled, or an adapter without the mechanism) is released exactly as today.
- Express the adapter's mechanism as provider/harness capability data consumed by the release owner,
  not as a Claude-specific branch inside session management, so another adapter can declare the same
  capability later.
- CompozyOS issues no compaction of its own for this; the adapter stays the only owner of that
  compaction.
- An operator can turn the behavior off through config; document how it relates to
  `session.supervision.idle_release_after`.
- Provider compaction observed this way must keep driving the existing prompt-section invalidation, so
  the next prompt re-delivers full situation content.
- The deferral and the compaction outcome are visible in the session event stream.
- This changes default behavior for existing users (large idle Claude sessions get compacted after
  about 54 minutes): update the config reference, the `compozy` skill references, site docs and the
  affected `docs/qa/scenarios/` files, and write the release-note text in the report.

## Constraints & Non-goals

- Everything in the round 1 brief's "Constraints & Non-goals" still applies.
- Additionally: run generators (`make codegen`) but not `make codegen-check`, and do not run
  repository-wide or uncached typechecks; CI owns those. Focused tests and focused lint only.
- Do not touch W6. Do not change W1–W5, W7 or W9 except where W8 or W10 requires it.
- One commit per workstream, plus one `docs:` commit for this brief and the updated report. Do not
  push or open a pull request.
- If either workstream still cannot be completed safely under these decisions, stop it cleanly and
  report the exact remaining blocker.

## Verification

- W10: the observe and `globaldb` suites and the migration suites, as in round 1, plus the invariant
  tests above.
- W8: the idle-release and work-supervision suites prove the hold, the release after compaction or
  lapse, the bounded maximum, the unchanged path for ineligible sessions, and the off switch.
- A real provider-backed check of W8 is authorized for one small Claude session, using
  `CLAUDE_AGENT_ACP_IDLE_COMPACT_DELAY_MS` to shorten the delay. If you do not run it, say so plainly.

## Final report

Update `docs/prompts/20261109-1726_harness-token-efficiency-report.md` in place and reply with the
same structure as round 1 for W8 and W10: status, commit SHA, files changed, focused commands and
results, the two survey tables, compatibility and migration notes, the release-note text, open
questions, and a plain statement of anything not verified.

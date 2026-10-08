---
id: RT-session-compact-real-adapters
area: RT
title: Compact on real Claude and Codex adapters and in a Goal run
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: On real `claude` (claude-agent-acp) and `codex` (codex-acp) sessions, `compozy session compact <id>` returns `outcome: completed` and the session shows one Compaction item (with a summary on Claude, without one on Codex); a Goal run past `context_nudge_ratio` performs one compaction turn with `/compact` on its managed path and the next usage report is below the baseline.
entry_points: compozy session compact <session-id> -o json; compozy session history <session-id> -o json; compozy session usage <session-id> -o json; Goal controls and Run timeline with [goals] context_nudge_ratio set low
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-compact-now; RT-pressure-context-compaction; GL-018
---

Manual-with-evidence, real provider accounts, not part of CI. Record the exact commands, timestamps, and observed
output in the dated QA report; the walk stays `untested` until a human or an executor with live Claude and Codex
sign-ins performs it. Experimental: both adapters follow an unstable upstream ACP compaction contract, and the
adapters are launched at their latest versions, so record the versions used.

1. **Claude.** Start a `claude` session in an isolated lab, build enough context for a meaningful compaction, then
   run `compozy session compact <id> -o json`. Expect `outcome: completed`, one Compaction item with a summary in
   `compozy session history -o json`, one `session.compaction_fired` event with `trigger: "requested"`, and
   `context.used` unknown until the next usage report.
2. **Codex.** Repeat on a `codex` session. Expect `outcome: completed` and one Compaction item with no summary.
3. **Goal.** Run a Goal on a real adapter with `[goals] context_nudge_ratio` low enough to trip after a few turns.
   Expect exactly one compaction turn sent as `/compact` (or `/compress` where that is what the agent advertises),
   no reseed, work continuing afterwards, and the next usage report below the pre-compaction baseline. A Goal on
   an agent advertising neither command goes to the existing reseed path instead.
4. **Negative control.** An adapter or version that does not honor the compaction capability yields no Compaction
   item, event, marker, or hook call; its own "Compact conversation" tool row stays an ordinary tool row.

Teardown per L-029: run the bootstrap manifest `TEARDOWN_COMMAND` (or `make qa-reap`) on every terminal path and
keep `teardown.json` with `clean: true`.

QA impact 2026-10-07 (memory removal): new in this change; no prior verdict.

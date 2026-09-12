---
id: RT-acp-usage-cache-meta
area: RT
title: ACP cache counters and sanitized usage metadata survive replay
persona: Rafa
journey: J-14
expected: Canonical adapter cache counts reach done events, per-turn projections, transcript usage, and session totals; only sanitized object metadata reaches recorded and live events; invalid context observations are discarded.
entry_points: ACP provider prompt; session events; prompt SSE; session history; session usage
qa_status: pass
bug_ids: BUG-20260912-context-delivery-details
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-09-12-session-context.md
last_report: docs/qa/reports/2026-09-12-session-context.md
overlaps: RT-session-cost-provenance
---

On the final integrated build, complete a turn with OpenCode or Claude Code that reports
`cachedReadTokens` and `cachedWriteTokens`. Compare the done event, `token_usage`, transcript,
and `token_stats`; send another turn and verify accumulation without counting intermediate usage twice.
Unreported fields remain absent and a reported zero remains zero.

Use the ACP fixture's `usage_updates` and cache counters to exercise metadata with `_claude/origin`,
a nested `claim_token`, and a `compozy_claim_` value under another key. The claim key must be absent
and the token value redacted in canonical event content, prompt SSE, and transcript usage.
Metadata must not become database columns or counters. Replayed usage carries the owning event sequence.

Send an invalid `{used:-5,size:0}` update followed by `{used:80,size:100}`. Only the valid update
becomes a usage observation. Repeated invalid updates log one field-naming warning per session.
Send legacy cache names twice and verify one deprecation warning naming canonical replacements and
v0.6.0; canonical values win when both names are supplied.

Upgrade a previous-version database containing token totals. Existing totals and turns survive;
new cache columns initially read NULL. `token_usage_daily` retains its existing shape.

Execution owner: session-context tasks 05/06. Focused automated evidence is recorded in the task 01
workflow memory; this scenario remains untested until the integrated live walk.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.

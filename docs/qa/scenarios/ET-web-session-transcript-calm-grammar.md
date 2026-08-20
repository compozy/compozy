---
id: ET-web-session-transcript-calm-grammar
area: ET
title: Session transcript renders the calm-surface grammar
persona: Théo
journey: J-14
expected: Settled tool runs rest as one semantic summary line ("Ran N commands · Edited N files", distinct-file counts) that expands to plain 24px tool rows; the live tail shows the last 4 calls behind a "+N previous tool calls" toggle; failed calls stay individually visible and collapsed with the red × glyph and error-first-line preview (row text never turns red, success check is grey); settled turns fold behind "Worked for Ns" (the only border), interrupted turns never fold; normal queue, steer, interrupt, and dropped-input lifecycle markers do not render as warning noise; the active turn has one working row immediately above the composer; runtime events render as calm one-line markers, never tinted Alert cards; the user message, TodoWrite plan, and changed-file roll-up retain their compact semantic treatments.
entry_points: web session window transcript; session transcript REST + SSE
qa_status: skipped
bug_ids: BUG-20260803-prompt-cancel-warning-noise
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-03-session-input-coderabbit/01-working-queued-calm.png; /Users/pedronauck/dev/qa-labs/compozy-session-input-coderabbit-20260804-003939-559639-lab/qa-artifacts/qa/evidence/prompt-cancel-warning-before.txt; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: RT-session-message-reload, ET-tool-result-artifact-recovery, ET-web-session-thread-full-bleed
---

2026-08-20 retry: skipped by explicit user instruction. No real-provider transcript was created or inspected.

story: As a person supervising agent work I read a calm, text-first transcript where settled work collapses to semantic summaries and only failures and the live tail demand attention.

errors:

inventory: Needs QA — introduced by the session transcript redesign (calm-surface vocabulary, 2026-07-29).

QA impact 2026-08-03: reset for calm busy-input feedback and the single active-turn working row.

QA impact 2026-08-20: reset by the normie-friendly UI foundation pass. The calm one-line runtime
markers this file asserts changed rendering — `runtime-activity-notice.tsx` meta and the marker kind
string moved from `font-mono` to sans with `tabular-nums`, and the same de-mono pass hit
`chat-message-bubble.tsx`'s system role and `marker.tsx`. The thread's own empty and error copy also
changed: "Start a conversation. The assistant thread replays persisted history and continues live
over the daemon stream." → "Start the conversation. Everything you and the agent do here is saved.",
and "Transcript unavailable" → "Couldn't load this conversation".

The calm grammar itself — summary collapse, the last-4 live tail, failures staying individually
visible, "Worked for Ns" as the only border, interrupted turns never folding — is unchanged by the
pass. What needs the walk is whether the transcript still reads as calm now that the system lines are
sans: the pass's premise is that the front door stopped reading like syslog, and the failure mode to
watch for is the opposite one, where de-mono'd meta stops being distinguishable from real content.
The 24px tool rows should be intact — transcript geometry was explicitly excluded from the type lift.

QA pass 2026-08-03: lifecycle markers stayed durable but invisible as warning noise; expected cancellation no longer projected provider failure; active turns rendered exactly one working row above the composer.

QA impact 2026-08-03 (CodeRabbit remediation): reset after a live steer/interrupt walk exposed the singular `transcript_marker.prompt_cancel` as duplicate warning noise.

QA pass 2026-08-03 (CodeRabbit remediation): after the canonical lifecycle filter fix, a fresh load rendered no prompt-cancel warning while preserving the settled replacement conversation and one active-turn Working row.

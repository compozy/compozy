---
id: ET-web-session-transcript-calm-grammar
area: ET
title: Session transcript renders the calm-surface grammar
persona: Théo
journey: J-14
expected: One or multiple live tool rows appear above a calm completed-tools summary. Settled turns fold once; interrupted turns remain open with a truthful stop cause. Absorbed failures keep the group neutral and expose the individual failed row. Find searches the full retained projection, opens the exact field/fold, preserves focus through live updates and archive invalidation, and downloads complete payloads; the message trail supports previews and deliberate jumps.
entry_points: web session window transcript; session transcript REST + SSE
qa_status: pass
bug_ids: BUG-20260906-injected-guidance-missing-history
fix_status: pending
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-09-06-sessions-stability.md
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

QA blocked 2026-08-25 (ENG-135): the isolated web shell reached setup, but the transcript journey could not be seeded because `internal/demoseed/seed.go:79` references the unavailable `GlobalDB.ListPresets` method. Focused UI tests passed; real-user transcript verification remains pending until the daemon seed compiles.

QA impact 2026-09-06 (sessions-stability tasks07/08): final task10 owns the new
quiet transcript and long-history navigation walk. Find must search unloaded older
history, load its context, open the containing fold and preserve query focus while
new output streams. The full-history message trail must show one operator-message
tick (compressed beyond its threshold), preview the message and final reply, and
jump under user scroll ownership. Include no-matches, archive/compaction invalidation,
and return-to-bottom states. Backend search/outline parity and workspace denial pass
in memory/task_08.md of the owning spec; these checks do not verify browser behavior.
Historical evidence above remains scoped to the earlier grammar. New visual acceptance
is pending task10 and must follow the task07/task08 normative artboards.

QA 2026-09-06 integrated verdict: Selected new grammar and navigation branches passed: all task07/08 visual rows and substates, actual3,023-message history, unloaded Unicode input and all text/title/output/error/filename fields, exact nested landing, focus/trail/no-match/bottom ownership, full260-line browser download, and generation1→2 archive invalidation while Find remained focused. The report retains each repair/re-walk and the distinction between controlled runtime and deterministic visual fixtures.

QA impact 2026-09-10: the thinking indicator now re-arms an early timer callback until its existing flicker-guard deadline, preventing an indefinitely hidden pending state. The canonical thread-status timing tests and full conversation/runtime suites passed. Evidence and browser verification are owned by `docs/qa/reports/2026-09-10-release-integration-repair.md`.

## Bounded summaries (issue #598)

Use the sanitized `SessionsStability/Working/LongSummaries` story, then repeat the
relevant transcript flow against the isolated browser runtime. At 360, 860, and
1600 px, summaries stay on one line: tool/reasoning/live/activity/Goal text uses a
maximum 24 rem readable measure; headings use at most 20 rem. Smaller containers
shrink these bounds. Session sidebar and deck tabs retain their existing tighter
constraints. Full message bodies and intentionally expanded content remain readable.

- Inspect a multiline heredoc, an unbroken path, Unicode graphemes, a provider title
  separate from tool identity, concurrent tools, and running/settled/error states.
- Confirm the Working row remains 22 px high, elapsed time and stop/composer controls
  stay visible, and no horizontal page overflow appears. Repeat at 200% zoom and
  with reduced motion; opening a summary popup must not shift the transcript.
- Use Tab and Enter to inspect live tool/activity details, Escape to return focus,
  and the tool/reasoning/Goal disclosure to inspect complete originals. Window head
  titles reveal their full label on pointer activation or Enter from keyboard focus.
  The action is named `Show full title: <title>` while the heading retains the title
  itself; it must remain distinguishable from a same-named dock navigation action.
- Copy a full tool payload and compare its title/input/output with the fixture.
  Find a unique tail omitted from a summary; the matching title/name/input body
  must open in both running and settled tools. Closing the reveal remains reader-owned.

Verification record: [issue #598 report](../reports/2026-09-10-session-summary-truncation.md).
Post-merge action naming, clipboard coverage, and final CI evidence:
[main remediation](../reports/2026-09-10-merged-pr-ci-review-remediation.md).

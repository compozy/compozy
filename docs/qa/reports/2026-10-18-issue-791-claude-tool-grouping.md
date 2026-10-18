# QA Run Report — 2026-10-18 — Claude live tool grouping

- Scope: issue #791, the completed-work disclosure during a Claude Goal turn.
- Cadence: targeted engineering reproduction and real-provider re-walk.
- Build: branch `fix/issue-791-claude-tool-grouping`, based on `07aba5e6e`.
- Environment: worktree Vite frontend; isolated 0.3.0-beta.35 daemon and native
  Claude Code / Sonnet 5 using the operator's existing provider login. Dedicated
  home, HTTP port and socket; the operator's daemon was not changed.
- Status: in progress; final CI and reviewer evidence belongs to the PR.

## Session matrix

| Journey | Observed result | Status |
| --- | --- | --- |
| Ordinary Claude prompt, four consecutive Bash calls | Grouped both in the prompt stream replay and the real browser | Pass |
| Claude Goal, four consecutive Bash calls | Reproduced four separate rows before the fix; grouped after the fix | Fixed |
| Expand the live Goal's completed group while another call runs | Four original command rows appear; the fifth running call remains separate | Pass |
| Narrow viewport after settlement | Recorded at 860 × 900; full page does not overflow horizontally | Pass |

## Root cause and correction

The real Goal SSE frame attributes **every** tool receipt and usage event with
`goal`. `isProgressTick` treated the presence of that metadata as a visible
boundary, even though those events render nothing. The captured pre-reply frame
therefore produced 24 timeline rows instead of one work group. Tool IDs stayed
stable throughout `Terminal` → command → `Bash`; identity fragmentation was not
the cause.

The classifier now decides visibility from the event's kind. Goal attribution
does not turn hidden status into narrative. The Goal header still reads the
original metadata, and errors, warnings, markers, decisions, prose, and deliberate
terminal interactions retain their existing boundaries. No wire or storage change.

## Rendered evidence

Evidence is retained under `docs/qa/evidence/2026-10-18-issue-791/` (the repository's
sidecar-managed evidence directory):

- `before-goal-live.png`: live Goal with each of the four completed commands on
  its own row.
- `after-goal-live.png`: fresh real Goal after the fix, showing `Ran 4 commands`
  while a fifth native Bash call is still running.
- `after-goal-expanded.png`: the same live group opened by clicking its actual
  disclosure; all four original commands and the separate running call are visible.
- `after-narrow-settled.png`: 860 px viewport after settlement.

The source fixture is committed beside the canonical timeline suite, with its
capture provenance. Screenshots are local evidence until the evidence sidecar is
synced; they are not claimed as publicly uploaded PR attachments.

## Focused validation

- Canonical timeline suite: 70 tests passed, including the real captured frame,
  expansion, a running fourth call, and a Goal-attributed visible runtime warning.
- The captured-frame regression failed on the original classifier; the fix passed.
- Web typecheck passed after building its small `@compozy/client` dependency.
- Scoped oxlint: zero warnings or errors.
- Scoped React Doctor: no issues found; its score API was unreachable locally.
- No heavy local gate, broad Go test, desktop build, or full E2E suite was run.

## Limits and runtime observations

This is a targeted transcript verification, not a full calm-grammar/navigation
audit or a native desktop-shell test. An initial lab Goal explicitly requested
`compozy__goal_report`, which this installed runtime did not expose, and the lab's
judge returned malformed output. Those runs were cleared through the lab's public
Goal surface. A fresh bounded Goal verified the UI independently of Goal judging.
An explicit clear during a continuation also recorded `context canceled`; this
was teardown intent, not a grouping failure. The delivery Goal is separate.

The bootstrap's broad `feature` audit was also run and returned FAIL: its
four-actor/task/disruption/artifact-reuse contract and full local gate are not
satisfied by this bounded transcript reproduction. That audit is not reported as
passing, and no broad scenario or full release QA claim is made. The user forbade
heavy local gates; focused checks and the PR's CI own this delivery.

The lab teardown completed with `TEARDOWN_ALL_CLEAN=true`; its teardown record
reports `clean: true`. Only this lab's registered processes and runtime holders
were stopped. Final current-head CI and reviewer dispositions are recorded on
the PR so publishing those receipts does not create another unreviewed code head.

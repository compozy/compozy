# BUG-20260906-promoted-turn-missing-live: Promoting a queued input leaves the open transcript behind

- **Status:** fixed — final real browser re-walk passed
- **Impact:** Stale-UI
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Journey:** J-13 manage a session
- **Scenarios:** RT-019, RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

In the real 3,023+ entry Incident archive, queue two messages through the composer, edit the index, remove the glossary, then click the remaining row's Steer. The controlled ACP provider uses interrupt_fallback. The Web shows `Interrupted after 1s · replaced by your steer` and drops the row, but never appends the promoted authored message or its completed assistant reply. Public history confirms turn-b4842182116ccd47 at sequences4549–4552, with exactly one user message and `Queued recovery processed once.`; public search locates the edited text. The browser has no uncaught errors. The symptom persists after the turn has ended.

Evidence: integrated lab evidence/navigation-promoted-missing-live.json/.png, navigation-promoted-search.json, navigation-after-promote-history.json, navigation-queue-promoted.json. Reload comparison and remediation pending.

Additional observation before remediation: a subsequent retained 260-line Bash output is searchable at field=output with `payload needle ação λ café` on line251, but Enter gives “That message did not come into view.” The containing new turn is missing from the viewport while older promoted output becomes visible after another reload; this may share the host's stale range/append path, not the matched-field renderer. Evidence/navigation-large-output-match.json/.png records the failed landing; no pass claimed.

## Re-walk 12:30 UTC — still failing after cache reconciliation

Fresh Web queue → promote with `Queued recovery: verify live promotion after cache reconciliation.` accepted user `2cd6cf11-0f30-4278-9e90-a7215a0d73ca`, turn `turn-a8c57a3749295c38`. Public transcript retains user 4592 and assistant 4594..4595 (`Queued recovery processed once.`). Public SSE replay after4593 includes the assistant delta cursor4595. Web shows the promoted user and From the queue, but no assistant even after clicking Scroll to latest; actual chat-view scrollTop1838.5, height442, scrollHeight2281 (bottom). No browser errors. No reload was performed. Evidence: `navigation-promote-live-final-transcript.json`, `navigation-promoted-public-replay.sse`, `navigation-promote-after-latest.json/png` in the integrated lab. The earlier cache fix is insufficient for this live branch; keep open.

Following reload, stream opened with after_sequence=4595 yet the assistant was still not rendered; after sending the next normal Watch turn the preceding answer finally appeared. This implicates projection/rendering of the latest settled turn in addition to any stream race. An EventSource observer is installed at the browser network boundary for the next re-walk (no fabricated events).

Second reproduction after actual lost-ack retry and clicking Stop generation to drain two parked entries: the first queue answer appears, but the final settled answer turn-ad8e08fb9243df9a is absent at bottom. `navigation-queue-drain-browser-wire.json` now captures the REAL EventSource received by this same browser: transcript_delta4609 streaming and4610 done both contain that answer. Browser errors=[]; public `navigation-retry-dispatched-transcript.json` confirms exactly one authored user and answer. This rules out absent SSE delivery for the final reproduction; focus latest-turn rendering.

## Final browser re-walk — passed

The actual browser-layout reproduction identified zero-height marker-only rows being assigned positive estimated height, moving the last answer beyond the reachable virtual window. Fable repaired measured empty-row accounting (task_10-promoted-render.md; real headless red/green and six root Turbo suites, 265 tests). Root repeated Web Queue then the queued row's Steer against the normal daemon, without reload or another turn: authored message 012a95c6-7f0c-4401-8b4e-d74f6b8993ef and answer turn-e92f46785c8c9832 appeared immediately; the answer rectangle y514.26–592.76 fits the chat viewport ending at593. The view is at actual bottom (2973+442=3415), queued input drained, idle badge, errors[]. Public transcript confirms the same final answer. Evidence navigation-promote-measurement-fixed.json/png and -transcript.json; root inspected the PNG. Status: fixed, final delivery pending.

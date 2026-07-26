# BUG-20260713-cross-workspace-session-return-hangs: Returning to a second workspace can retain stale identity and hang the session route

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Théo
- **Journey Step:** J-11 return to a running session, steps 3–4
- **Scenarios:** RT-workspace-active-session-badge; RT-session-auto-title
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** AGH-84 residual found during controller acceptance

## Summary

Two live Cursor/Grok 4.5 user sessions were running simultaneously in the launch workspace and `bench-ops`. The first cross-workspace return link correctly selected the launch workspace and opened its exact latest session. Clicking the reciprocal return link for `bench-ops` changed the URL and active workspace, but the banner retained the launch-workspace session identity and the route stayed in `Loading` indefinitely. Reloading the same permalink and opening clean tabs subsequently blocked browser navigation and DOM access.

## Reproduction

- **Charter:** CH-background-session-switch · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US; isolated local daemon PID `57617`; live Cursor/Grok 4.5 (`Grok 4.5 (High, Fast)`) sessions.

1. In workspace `agh-automation-features-...-lab`, start session `sess-5ec18f5f2a13fe16` and send a meaningful prompt. Confirm the automatic title and exact inactive-workspace badge.
2. Switch to `bench-ops`, start session `sess-40e90687024bfb24`, and send another meaningful prompt.
3. Use the launch-workspace return link. Confirm its exact session ID, title, and reciprocal `bench-ops` badge.
4. Click the `bench-ops` return link.

**Expected:** The workspace selection, session permalink, banner identity, transcript, and loading lifecycle reconcile to `sess-40e90687024bfb24`; the page remains responsive.
**Actual:** The URL and selected workspace become `bench-ops` / `sess-40e90687024bfb24`, while the banner keeps the launch-workspace title from `sess-5ec18f5f2a13fe16`; `main` remains `Loading` for more than 12 seconds and subsequent reload/navigation commands hang.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-two-workspace-badge-title.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-hang.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-residual.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-second-fix-failed.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-second-fix-state.json`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-fixed.dom.txt`
- Source session: `sess-5ec18f5f2a13fe16`; destination session: `sess-40e90687024bfb24`; both created through the UI with real Cursor/Grok 4.5.

## Fix

- **Root cause:** Confirmed two-stage navigation race across the Return `Link`, Zustand workspace store, TanStack Router, and the session route guard. The original Return mutated the destination workspace before TanStack created pending navigation, so the source guard redirected. The typed-history correction removed that race, but incorrectly accepted the intent only for route `enter`. TanStack Router reuses the same file-route ID when only `$id` changes and therefore classifies the destination as `cause: "stay"`; the exact intent reached `history.state` but was discarded before loader selection. The globally unique `byId` lookup, workspace-scoped detail/transcript keys, and catalog/live-tail SSE lifecycles were audited and were not the cause.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** A canonical real-Router suite now exercises actual `Link` → history → `beforeLoad` → loader → Query → Zustand ordering. Its red state observed `cause: "stay"`, correct typed history, a primary loader result, and a still-bench store. Return now consumes the exact current history intent for committed `enter` or `stay` navigation, never preload, only after resolved ownership and live-workspace validation agree. A second assertion proves that a following navigation without state inherits no authority. Post-deslop evidence: 2 files / 37 focused tests, 7 files / 147 adjacent tests, full Web (396 files / 3,406 tests), Web typecheck, exact oxfmt, zero-warning oxlint, and diff-check all passed.

## Verification

- **Rejected first fix:** after the Browser surface was reopened, the exact primary → `bench-ops` reciprocal return still reproduced the defect. The destination URL and `bench-ops` workspace committed, but the primary title remained in the banner and `main` was still `Loading` after five seconds. That first pending-route-only fix is rejected by live evidence.
- **Current source:** the third correction is source-frozen with the real-Router red/green proof and full Web evidence above. A new exact primary ↔ `bench-ops` Browser replay remains mandatory before this bug can become `verified`.
- **Rejected second fix:** a clean Browser run opened `sess-40e90687024bfb24` directly and rendered its bench title/transcript correctly. Clicking the exact primary-workspace Return link then committed the primary permalink, but the selected workspace and banner remained on `bench-ops` and `main` stayed `Loading` beyond five seconds. The live Vite process was serving the current source tree, so the history-intent implementation is rejected by authoritative end-user evidence and requires another root-cause pass.
- **Second-fix state evidence:** the committed history entry contains the correct typed intent (`sess-5ec18f5f2a13fe16` / `ws_06366aad69887872`), while persisted `agh:active-workspace` still contains the bench workspace (`ws_74a58ac2bf973937`). This narrows the residual to intent consumption/route lifecycle rather than Link-state transport.
- **Browser handoff:** Vite was restarted with the same proxy to remove the contaminated renderer, and the new Web PID is registered in the QA envelope. A later health check found the previous daemon had independently exited; it was restored with the exact same `AGH_HOME`, port, database, and binary, and `/api/status` is healthy again. Crash recovery classified both formerly active test sessions as stopped/`agent_crashed`, while retaining their titles and transcripts, so the active-badge replay will reattach them or create fresh real Cursor/Grok sessions. The controlled tab became Browser Use's native `data:` connection-refused interstitial during the restart. Browser security policy blocks controlling or navigating from it and forbids alternate-surface workarounds, so the user must close/reload that interstitial and reopen localhost before the controller can perform the clean replay.
- **Accepted third fix:** a fresh post-fix lab created real Cursor/Grok 4.5 user sessions `sess-84c5282a292e7f0f` in `agh3` and `sess-0c73989a7e97390b` in `bench-ops`. Direct `bench-ops → agh3` and reciprocal `agh3 → bench-ops` Return clicks both converged to the exact target permalink, selected workspace, persisted automatic title, and original transcript in under five seconds, with no stale banner or Loading loop. One earlier click fell back to the destination agent overview without hanging, but immediate direct repetitions in both directions reached the exact sessions; the original blocking behavior is not reproducible. Stop/delete then removed the bench badge and session durably.

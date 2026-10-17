# QA Run Report — 2026-10-17 — Terminal background progress

- **Scope:** issue #774; workspace switches, hidden terminal views, stale stream recovery, and canceled attach requests.
- **Cadence tier:** targeted
- **Build:** branch `fix/issue-774-terminal-workspace-stall`, based on `b4636e3322ee306a36f955f0068745a1add51042`; original fix `4c7dd1b` plus the review-remediation candidate.
- **Environment:** isolated macOS daemon and real Web dev server; no provider required for this terminal-only journey.
- **Started:** 2026-10-17T20:10:07Z · **Status:** targeted re-walk passed

## Session Matrix & Results

| # | Charter | Scenario | Persona / device | Tour | Status |
|---|---|---|---|---|---|
| 1 | CH-terminal-shared-input-race (targeted stream slice) | ET-terminal-stream-resilience | Marina / desktop / en-US | Multi-Tab | Pass (changed slice) |
| 2 | CH-terminal-operator-shell (targeted lifecycle slice) | ET-terminal-browser-lifecycle | Bruno / desktop / en-US | Feature | Pass (changed slice) |

The scoped walk starts a noisy command in workspace A, switches to B for at least 30 seconds,
checks advancing output through public reads while A is hidden, then returns and confirms current
rendered output. It repeats with a minimized terminal and a background browser tab, and exercises
reconnect/snapshot recovery. A real ACK client that never acknowledges is an adjacent compatibility
probe. No host daemon or existing user workspace is changed.

## Reproduction and implementation evidence

- Before the fix, the existing flow suite confirmed that all high ACK subscribers pause the producer.
- New owning manager regression failed after 2 seconds: `process output stalled behind an attached writer without ACKs`.
- The same regression passes with `-race` after removing viewer-dependent PTY waits.
- Attach cancellation regression reproduced HTTP 500 in identity, ticket mint, and mapped errors;
  all three now return HTTP 499 with the existing `api_error` envelope.
- Protocol client suite: 43 tests passed via root Turborepo, including ordered parsing, gaps,
  snapshot recovery, reconnect, stale completions, and nonblocking writable attachments.
- The former producer-pause assertion intentionally contradicted the new issue contract and was
  retired with its internal Group implementation. Queue bounds/demotion/gap coverage remains.

## Session Debriefs

- Workspace A ran a Python process emitting 4 KiB progress records while the real Web UI showed B
  for 33 seconds. Final-candidate public read sequences advanced `1,356,606 → 3,216,134 → 5,013,952 →
  6,770,630`; the visible counter advanced from 324 to 1,640. Returning to A rendered current output.
- The first complete walk also covered 22 seconds minimized (`14,500,698 → 18,302,034`) and a
  genuinely hidden Chrome tab (`document.visibilityState = hidden`), advancing
  `23,530,928 → 24,691,879` and reaching `BACKGROUND_DONE`. Public `terminal_wait` HTTP equivalent
  returned `reason: match`; foregrounding showed the completion marker.
- Final-candidate repeats confirmed minimized progress `12,201,110 → 14,068,866` and hidden-tab
  progress `17,260,160 → 19,065,328` over 11 seconds each. The latter completed with a public wait
  match for `HIDDEN_DONE`, then rendered that marker after foregrounding.
- Real WebSocket ACK compatibility probes deliberately returned no credit. Three fresh attachments
  each completed a 2 MiB burst. The final candidate also passed three full-history reattachments in
  227/574/624 ms, with public wait matches and final sequence 6,291,751: no 10-second grace wait.
- The probe exposed a replay-demotion lock re-entry and loss of ATTACHED before GAP. Both were fixed
  in production, reproduced in the existing manager/wire suites, and re-walked against the rebuilt
  daemon. Large replay now preserves the stream handshake before bounded loss recovery.
- Screenshots: `docs/qa/evidence/2026-10-17-issue-774/workspace-b.png`,
  `docs/qa/evidence/2026-10-17-issue-774/workspace-a-return.png`, and
  `docs/qa/evidence/2026-10-17-issue-774/background-complete.png`.

## Review-remediation re-walk

- Greptile and CodeRabbit identified browser buffers that could grow after removing producer
  backpressure. Both the render backlog and the held snapshot tail now have limits of 1 MiB and 1,024 frames.
  Overflow discards stale work and requires a snapshot covering all discarded sequences.
- The owning protocol regression failed against the original PR head and passes after the fix.
  It covers overflow during normal output and initial replay, stale snapshots, input gating,
  and live output after recovery. A pending replay could reopen input mid-resync; production now
  leaves that transition to the recovery owner.
- A second CodeRabbit review identified one-byte frames accumulating too many promise operations
  below the byte budget. Both buffers now enforce the frame-count limit as well. The same
  protocol case covers 1-byte and 32 KiB frames; the two tiny-frame cases failed before this fix.
  A scoped real-browser repeat with 25,000 one-byte writes and 12x CPU throttling entered
  recovery and rendered `SMALL_DONE`; public read sequence advanced `16,271 → 38,995` and
  public wait returned `match`. Screenshot: `small-frames-recovered.png` in the evidence folder.
- Consecutive queue trims reproduced incomplete GAP metadata in both ACK and drop modes.
  The existing wire suite now checks the cumulative range/byte count, and healthy ACK delivery
  synchronizes with delivered credit before acknowledging it.
- Real Chrome was throttled to 1/12 CPU speed while a process emitted 20,000 records of 8 KiB.
  The UI visibly entered `Catching up…`. During 43 seconds in workspace B, public read sequence
  advanced `62,179,104 → 98,523,878 → 134,147,509`. Returning to A recovered the current screen;
  the public wait matched `BUFFER_DONE`, which was then visibly rendered with the shell prompt.
  Final sequence: `163,984,436`. This verifies recovery under an actually slow renderer; the
  deterministic suite owns exact byte-bound and stale-snapshot assertions.
- Rebuilt daemon passed three no-ACK full-history reattachments in 721/1487/1594 ms, each with
  a public wait match. Focused race suites, real PTY/WebSocket integration, affected Web typecheck,
  changed-file formatting/lint, and React Doctor 100/100 passed.
- Local captures: `bounded-buffer-catching-up.png` and `bounded-buffer-recovered.png` under
  `docs/qa/evidence/2026-10-17-issue-774/`; the latter shows the completion marker and prompt.

## Verification and limits

- Earlier-candidate `make gate` passed affected Go API/terminal race suites, Go lint,
  Web lint/typecheck and 6,854 tests in 677 files. The final heavy run was interrupted at the
  operator's direction to move heavy gates to CI; it is **not** a final-source pass.
- Final local validation uses only the focused manager/wire race suites, cancellation tests,
  existing real lifecycle integration and already-valid unchanged Web evidence. PR CI owns the
  complete current-head gate.
- PASS: real PTY/WebSocket lifecycle integration (`TestTerminalWireShouldCompleteRealLifecycle`)
  with `-race -tags=integration`; Windows terminal cross-build; React Doctor 100/100.
- PASS: deterministic cancellation tests at the identity-resolution, ticket-mint and mapped-error
  boundaries return HTTP 499. A client cancel is not reclassified as a server failure.
- Scope: macOS/Chrome at 1920×907; no real Windows runtime, desktop-native minimization, or exhaustive
  duration/platform matrix. Web window minimization and browser backgrounding were exercised.
- Native `terminal_read`/`terminal_wait` use the same service exercised through the isolated daemon's
  public HTTP routes; the host tool gateway was deliberately not used for lab terminals.
- Lint reports zero issues. Existing unrelated React act warnings and the macOS linker deprecation
  message remain in full gate output; no assertions or diagnostics were suppressed.
- Screenshots and raw receipts are local QA evidence. Skeeper 0.3.1 cannot load this managed worktree
  (`core.repositoryformatversion does not support extension: worktreeconfig`), so evidence-sidecar
  synchronization is unavailable. Shared Git configuration was not changed.

- Strict QA evidence audit: PASS, zero blockers and zero warnings.
- Isolated lab teardown: `TEARDOWN_CLEAN=true`; only the registered lab processes were stopped.

## Final Status

PASS for the changed terminal behavior and targeted real-interface journeys. Unchanged scenario
coverage remains historical; this is not a full-product QA verdict. Remote current-head CI and both
reviewers are delivery checkpoints tracked on the PR.

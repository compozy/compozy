# BUG-20260906-terminal-selection-resize-loop: Selection actions clear the selection they act on

- **Status:** fixed locally — real browser red/green and packaged desktop re-walk pass; new-head CI pending
- **Impact:** Correctness · Usability
- **Severity:** Major · **Priority:** P1
- **Scenario:** ET-terminal-journal-recording
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

PR #557 run `34062900278`, Web shard 4, failed terminal-agent E2E-008 because selection actions disappeared after a two-line drag. The trace shows the actions immediately after mouse-up and their subsequent removal. Pointer coordinates landed on the expected rendered quote rows.

The selection bar participated in the terminal pane's flex layout. Showing it reduced the emulator's height, which published a resize and cleared the emulator selection; clearing the selection removed the bar again. The actions now sit over the bottom-right of the existing terminal container. Showing or changing those actions does not change the PTY dimensions. The same selection-action component, callbacks, copy and keyboard controls are retained.

The canonical browser journey uses the rendered accessible quote rows to locate the real mouse gesture. Its old helper opened an extra watcher solely to reveal a size-vote label for coordinate calculation. Closing that watcher later removed the footer and independently invalidated the selection before the final no-session assertion. The extra viewer is no longer part of this quote journey. All quote source-ID/text, send/reply and no-active-session assertions remain unchanged; resize/watcher parity retains its dedicated Web and desktop journeys.

Invariant: selecting terminal output exposes stable actions, sends the sourced excerpt to the conversation, and retains the no-session fallback without the actions resizing away their own range. Owner: Web TerminalPane layout. Canonical suite: existing terminal-agent E2E-008; no CSS assertion or additional unit suite was added. The updated gesture still fails on the old production build at the original action-visibility assertion (26.5s), then passes on the corrected build (6.0s test, 18.2s total). The final screenshot was inspected and shows both selected rows and the floating no-session actions.

Evidence: `.cache/sessions-final-agent-quote-geometry-red.log`, `.cache/sessions-final-agent-quote-final-green.log`, `.cache/sessions-selector-e2e-root-agent-quote-final-results.json`, and the retained final screenshot beneath `.cache/sessions-selector-e2e-root-agent-quote-final-output/`. Packaged desktop E2E-013 passes again in 12.8s with the same corrected Web bundle, covering clipboard selection, accelerators, zoom and IME. Every wrapper restored web/dist byte-for-byte. The 264 canonical terminal component cases, full Web suite, root Turbo lint/typecheck and React Doctor 0.9.13 (100/100, zero diagnostics) pass.

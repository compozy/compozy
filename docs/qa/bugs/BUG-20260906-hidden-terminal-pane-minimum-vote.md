# BUG-20260906-hidden-terminal-pane-minimum-vote: A hidden terminal tab votes the protocol minimum and shrinks its PTY

- **Status:** fixed locally — all 12 real terminal journeys pass against the patched web build served as `web/dist`; new-head CI pending
- **Impact:** Correctness
- **Severity:** Major · **Priority:** P1
- **Scenarios:** ET-terminal-browser-lifecycle; ET-terminal-window-native-flow
- **Found:** 2026-09-06 · **Report:** .compozy/tasks/sessions-stability/memory/terminal-retention-ci.md

Web E2E shard 4 on head d897e90f5 failed `terminal.spec.ts` E2E-002: after two terminal windows, eight deck tab switches and a reload, the retained (unfocused) PTY's screen read held only a 20-column wrapped shell prompt instead of `first-screen-intact`. The Playwright trace shows the first terminal's re-attachment at the moment the second tab opened already carrying `cols=20&rows=5` in its upgrade query, and the pre-reload video shows the focused terminal rendering a 20-column prompt.

An instrumented local run gives the mechanism. When an OS window tab becomes inactive it is kept mounted under `hidden`. The terminal pane's resize observer still fires, and the xterm fit addon reads the host's `width: 100%; height: 100%` through `getComputedStyle`, which for an element without a layout box returns the literal percentages; `parseInt("100%")` is 100, so the addon proposes 12×4. The client clamps that to the wire minimum 20×5 and sends it as a write vote. The daemon sizes a PTY by the minimum over its write subscribers, so the hidden pane's vote shrinks the PTY to 20×5 and the emulator reflow pushes the marker into scrollback. Every tab switch repeats this for the pane being hidden. Whether the last pre-reload hide vote reaches the daemon before the reload decides the test: locally the reload wins, on the CI runner the vote lands first.

Repair in `@compozy/ui` `TerminalView`: a view without a rendered box proposes nothing, so a hidden pane keeps its last visible size. The guard uses real layout geometry and the visibility API when available; tests model layout only at the DOM boundary. The containing `TerminalPane` element is now a flex column, so the grid resolves its height from the window and can grow after a previous shrink.

The UI package’s 761 cases, 264 focused Web terminal cases and all 12 real terminal journeys pass. The fixed build was actually served from `web/dist` under the shared verification lock and restored byte-for-byte. A targeted probe measured the visible container at 417px and its host at 395px, while hidden containers produced no size vote; the retained E2E-002 screenshot was inspected. E2E-014 compares the size bar with the watcher’s current RESIZED frame, preserving live-view agreement when the viewers footer changes the available height. Deadlines and retained-screen assertions are unchanged.

Evidence: `.cache/sessions-terminal-e2e002-fixed2-all2-results.json`, `.cache/sessions-terminal-all-fixed2b.log`, `.cache/sessions-terminal-artifacts-probe2b/`, and the controller’s final integrated re-walk. Current-head CI remains required.

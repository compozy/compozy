---
id: ET-web-dock-default-window-size
area: ET
title: Dock apps open as a tab in the focused window, beside it, or at the default floating size
persona: Bruno
journey: J-operate-desktop-shell
expected: With the default `new_window_policy = tab`, opening an app from the dock onto an empty desktop fills the desk as one flat tiled pane, and opening it while a window on the same desktop is focused (tiled or floating) adds the app as a new tab of that window's frame and makes it the active, focused tab; with `new_window_policy = beside_focus` the app instead splits the focused tiled pane with the new window after it; when the focused window floats under `beside_focus`, or `new_window_policy = floating` is set explicitly, the app opens as a floating window (1px hairline + elevated shadow) at the one default floating rect every app shares (0.68 × 0.78 of the desk, inset 12% from its left and 8% from its top — ≈938×661 on a 1380×848 desk) clamped inside the desk; closing and reopening applies the same rule; an explicit config value always wins over the new default.
entry_points: web dock (left rail); [window_manager] new_window_policy; app-registry defaultRect
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-01-empty-desktop.png; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-window-routing-lifecycle; ET-web-desktop-shell-lifecycle; ET-web-catalog-navigation
---

Opening Agents, Loops, Jobs, Triggers, Knowledge, Vault, Marketplace, Dashboard, or Session from a fresh closed state lands a floating window at the enlarged registry defaultRect (≈920×640 list surfaces, ≈960×680 dashboards/marketplace, Session ≈860×680); Tasks and Settings keep their existing large defaults; clampRect still fits the window inside the desktop gutters on smaller viewports; closing and reopening applies the registry defaults again.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). The default policy changed from `floating` to `beside_focus` (SD-013 public default change; explicit values keep their behaviour). Walk both: a fresh config (tiles beside focus, no gaps) and `new_window_policy = "floating"` in `config.toml` (floating at defaultRect). The old dock band no longer exists, so floating clamps stop at the desk edges instead.

qa-impact: 2026-09-30 expectation corrected against the product: there are no per-app default sizes; every new floating window opens at the shared default rect (`DEFAULT_WINDOW_MANAGER_FLOATING_RECT`). Expected text only; status fields untouched.

qa-impact: 2026-09-30 shell-rail polish P6 — the default `new_window_policy` is now `tab` (additive enum value; `beside_focus` and `floating` keep their behaviour when set explicitly). Walk all three: a fresh config (empty desktop → one full pane, then a second app → a tab of the focused frame), `new_window_policy = "beside_focus"` (split), and `new_window_policy = "floating"` (default rect).

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

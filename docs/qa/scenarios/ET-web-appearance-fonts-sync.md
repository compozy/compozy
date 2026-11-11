---
id: ET-web-appearance-fonts-sync
area: ET
title: Choose the fonts, text size and letter spacing once and see every client follow
persona: Bruno
journey: J-operate-desktop-shell
expected: Settings › Appearance › Text offers Interface and Code pickers with Default, the catalog (Interface: Geist, Inter, Instrument Sans, JetBrains Mono, System; Code: JetBrains Mono, System) and, in the desktop app or Chromium, an Installed group (Code lists only monospace families); every option previews in its own face; choosing one repaints the whole client at once, including open terminals (no clipped cells, the PTY resizes), code blocks and diffs; a second connected client follows without picking again; a reload and a desktop relaunch paint the chosen font from the first frame; Use default resets every client; Text size (Small, Default, Large, Larger) scales every text and its line height across the client except the terminal, and Letter spacing (Tight, Default, Relaxed, Wide) changes the interface tracking, both following on every client and painting from the first frame after a reload or relaunch; `compozy config get shell.appearance.*` matches the UI.
entry_points: Settings > Appearance > Text; second browser tab or desktop app on the same daemon; browser reload; desktop relaunch; compozy config get|set|unset shell.appearance.ui_font|mono_font|text_size|letter_spacing
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-theme-toggle-persists; ET-web-font-assets-strict-csp; APP-theme-native-no-flash
---

Added 2026-10-27 for #984. Unlike the theme, fonts are daemon state: `[shell.appearance]` in global
`config.toml`, read through `GET/PATCH /api/settings/shell`. The web paints them as the
`--user-font-sans` / `--user-font-mono` overrides that head the token stacks, so a family missing on
one device falls back to Figtree / Geist Mono there. The pre-paint boot script (`/theme-boot.js`)
replays the last applied values from localStorage `compozy.fonts`.

Walk: open Appearance in the desktop app and in a browser tab on the same daemon. In the desktop,
open Code: confirm the Installed group lists only monospace fonts, then choose JetBrains Mono with a
terminal window open — glyphs and the prompt line up, and `tput cols` reports the refit width. The
browser tab repaints within a minute or on focus. Reload the tab and relaunch the desktop: no frame
in Geist Mono. Pick an installed Interface font in the desktop and confirm the browser on another
machine without it falls back to Figtree. Use default on both rows, confirm both clients revert, then
`compozy config get shell.appearance.ui_font` reads empty. Set an invalid value through the CLI
(`compozy config set shell.appearance.ui_font 'Inter; color: red'`) and confirm
`shell_config_invalid`.

Development evidence (not a QA run): `appearance-settings-pane.test.tsx` (picker writes, reset,
installed filtering, painted variable), `theme-runtime.test.ts` (boot-script parity, lazy bundled
face), `terminal-view.test.tsx` (live font change, refit, stale-change guard) and the Go shell config,
settings, API and CLI suites.

qa-impact: 2026-11-08 brand identity v1.2 (spec `.compozy/tasks/brand-site/`). The built-in Interface fallback is now Figtree Variable (Code stays Geist Mono); the catalog still offers JetBrains Mono and System, and a stored JetBrains Mono choice keeps loading. Expectation updated; walk status unchanged.

qa-impact: 2026-11-11 appearance text controls. The Interface catalog adds bundled Geist, Inter and Instrument Sans (lazy, like JetBrains Mono), and the group is now Text with two more rows: Text size (`shell.appearance.text_size`, the `--user-text-scale` multiplier on every rem `--text-*` step and its leading) and Letter spacing (`shell.appearance.letter_spacing`, `--user-tracking-body`). The boot script replays them from localStorage `compozy.type`. Walk addition: choose Large and Relaxed in the desktop, confirm the browser tab follows, reload and relaunch with no frame at the default size, confirm an open terminal keeps its size, then `compozy config unset shell.appearance.text_size` and confirm both clients return to Default. Development evidence: `appearance-settings-pane.test.tsx` (size/spacing writes and painted variables), `theme-runtime.test.ts` (boot-script parity for the type cache) and the Go shell config, settings, API and CLI suites. Status reset to untested.

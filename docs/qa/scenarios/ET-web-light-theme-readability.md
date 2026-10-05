---
id: ET-web-light-theme-readability
area: ET
title: Read and operate the whole desktop in the light theme
persona: Sol
journey: J-operate-desktop-shell
expected: In the light theme every shell surface (rail, topbar, window frames and deck tabs, menus, dialogs, toasts, settings) and the main app views keep text and icons at WCAG AA contrast against their surface, the keyboard focus ring stays visible on every control, the needs-you orange stays distinguishable from warnings, monochrome brand logos stay visible, and the terminal uses its light palette; nothing keeps a dark-only color.
entry_points: rail-foot theme toggle; every rail app in the light theme; keyboard Tab traversal
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-theme-toggle-persists; ET-web-theme-system-follows-os
---

Added 2026-09-30 for the shell-rail light theme (brand-spec light column). Contrast floors per theme live
in the generated `DESIGN.md`; light values in `packages/ui/src/tokens-light.css`.

Walk with the keyboard only: Tab through the topbar, rail (Up/Down), a floating window's controls, a menu,
a dialog and a settings form; the focus ring must be visible at every stop. Sample text/background pairs
on each surface with a contrast checker. Check OpenAI/Linear marks on marketplace cards and provider rows,
the loop editor canvas, and a terminal printing the 16 ANSI colors.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

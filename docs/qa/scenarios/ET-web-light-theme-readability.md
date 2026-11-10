---
id: ET-web-light-theme-readability
area: ET
title: Read and operate the whole desktop in the light theme
persona: Sol
journey: J-operate-desktop-shell
expected: In the light theme every shell surface (rail, topbar, window frames and deck tabs, menus, dialogs, toasts, settings) and the main app views keep text and icons at WCAG AA contrast against their surface, the keyboard focus ring stays visible on every control, the needs-you amber stays distinguishable from warnings and from the volt brand, monochrome provider logos and the blue-to-cyan Compozy symbol stay visible, and the terminal uses its light palette; nothing keeps a dark-only color.
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

## 2026-11-08 brand identity v1.2 slice

Spec `.compozy/tasks/brand-site/` (E2E-003). The light palette now uses volt `#2448e6` for brand fills,
`brand-strong` for brand-coloured text and amber `#9a5b00` for needs-you (5.43:1 on white). When you
sample pairs, include `brand-strong` on canvas, white `brand-ink` on a brand fill, and `attn` on canvas
and paper; the token contract suite (`packages/ui/src/__tests__/theme-token-contract.test.ts`) owns
the ratios.

Slice validation: spot check only. The light shell painted brand `#2448e6` and attn `#9a5b00`, and
the first keyboard stops showed a visible focus ring. The full keyboard, contrast and terminal walk
remains outstanding. Evidence: `/tmp/brand-s8-shots/` (lab `compozy-brand-s8-theme-20261108-222905-377921`, teardown clean).

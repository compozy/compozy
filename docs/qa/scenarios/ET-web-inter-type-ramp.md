---
id: ET-web-inter-type-ramp
area: ET
title: Figtree and Geist Mono type ramp with a 13.5px body
persona: Bruno
journey: J-operate-desktop-shell
expected: The web app and Storybook load Figtree Variable for UI and Geist Mono Variable for code, paths, and the terminal (Symbols Nerd Font still backfills private-use glyphs); no Inter face loads; the body computes Figtree 13.5px at weight 450, line-height 1.5, −0.01em, with no `cv11` feature setting; font-medium is 500 and font-semibold 600; the ramp is 11 (micro) · 12 (eyebrow, kbd) · 12.5 (meta) · 13 (small body) · 13.5 (body) · 14 (item and card titles) · 15 (headings) · 18 (page H1); key caps render 12px/450 from the system key font; muted, subtle, and faint text clear 4.5:1 on panes, sunken insets, chrome, and hover surfaces in both themes; the public site sets Figtree body text, Instrument Sans display titles and Geist Mono code.
entry_points: web SPA (`web/src/styles.css`); Storybook (`packages/ui/.storybook/preview.css`); tokens `--font-sans`, `--font-mono`, `--text-body`, `--font-weight-*`; site pages
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-font-assets-strict-csp; ET-web-shortcut-keycap-legibility; ET-web-light-theme-readability
---

Added 2026-09-30 for the shell rail type system; replaces `ET-web-geist-wght-medium-510`. Verify computed styles in the running app and Storybook in both themes, and on the site that body is Figtree 15px/400.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-08 brand identity v1.2 (spec `.compozy/tasks/brand-site/`). The UI face moved from Inter to Figtree Variable at a 450 regular, `cv11` was dropped, and the site moved to Figtree / Instrument Sans / Geist Mono. The scenario id is kept so reports and overlaps still resolve; expectation updated, status unchanged.

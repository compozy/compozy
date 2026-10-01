---
id: ET-web-inter-type-ramp
area: ET
title: Inter and Geist Mono type ramp with a 14.5px body
persona: Bruno
journey: J-operate-desktop-shell
expected: The web app and Storybook load Inter Variable for UI and Geist Mono Variable for code, paths, and the terminal (Symbols Nerd Font still backfills private-use glyphs); the body computes Inter 14.5px at weight 425, line-height 1.5, −0.01em, with `cv11`; font-medium is 500 and font-semibold 600; the ramp is 12 (eyebrow) · 13 (meta) · 13.5 (small body) · 14.5 (body) · 15 (titles) · 17 (headings); key caps render 12px from the system key font; muted, subtle, and faint text clear 4.5:1 on panes, sunken insets, chrome, and hover surfaces in both themes; the public site keeps its own Geist type unchanged.
entry_points: web SPA (`web/src/styles.css`); Storybook (`packages/ui/.storybook/preview.css`); tokens `--font-sans`, `--font-mono`, `--text-body`, `--font-weight-*`; site pages
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-font-assets-strict-csp; ET-web-shortcut-keycap-legibility; ET-web-light-theme-readability
---

Added 2026-09-30 for the shell rail type system; replaces `ET-web-geist-wght-medium-510`. Verify computed styles in the running app and Storybook in both themes, and on the site that body is still Geist 15px/400.

---
id: ET-site-docs-typography-opendesign
area: ET
title: Docs shell typography matches OpenDesign three-voice ramp
persona: Dora
journey: J-evaluate-compozy-beta
expected: On a runtime docs page (e.g. /docs/loops), Playfair Display appears only on the page title and h2 (retuned heading clamp, hairline top); h3 is Geist 600 at 1.25rem/1.25; lead is 18px muted at 64ch; body is Geist 16px/1.8 muted at 72ch with strong at 600 and underlined links; sidebar section labels use mono badge (10.5px accent-mix) and in-folder groups use mono group-label (10px subtle 72%) — no Geist uppercase, no weight 700, no Playfair below display size.
entry_points: compozy.com /docs/loops; docs/design/opendesign/_done/site/site-docs-sidebar.html; docs/design/opendesign/_done/site/site-example-page.html
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-geist-migration-20260809/qa-artifacts/qa/typography
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: ET-site-docs-masthead-opendesign; ET-site-docs-sidebar-opendesign
---

QA impact 2026-07-29: docs shell type ramp was aligned to the OpenDesign docs references
(three voices, retuned h2 clamp, h3 weight, lead/group-label tokens, prose
link underline). The next QA cycle owns visual parity against the canonical
specimen on desktop mid-width and mobile body retune.

QA impact 2026-08-09: the sans voice moved from Inter to Geist (`next/font/google` `Geist`
loader, `--font-geist`). Status was reset for re-walk — the ramp values are unchanged but the
rendered face is not. Re-walked the same day; verdict below.

## Walk 2026-08-09 — pass

Walked `/docs/loops/` on the production `next build` output served by `next start`, with
computed-style probes after `document.fonts.ready`.

- Body, `p`, `strong`, and doc-body prose compute
  `Geist, "Geist Fallback", Geist, -apple-system, system-ui, sans-serif`; loaded faces are
  `Geist 100 900`, `Geist Fallback`, `Playfair Display 400`, `Playfair Display Fallback`,
  `JetBrains Mono 100 800`, `JetBrains Mono Fallback`. No `Inter*` face is present.
- Playfair Display appears only on `h1` (54.4px/400) and `h2` (34.4px/400) — both well above the
  ~26px display floor. No Playfair below display size.
- No element in the article body computes `font-weight: 700`.
- Sidebar section labels and in-folder groups stay on the mono badge/group-label ramp; no sans
  uppercase in the doc body.
- Tabular figures align (identical advance for `1111111111` and `8888888888`).
- Zero failed `.woff2` requests.

Evidence: `computed-styles.json`, `computed-styles-2.json`, `site-docs-loops.png` under the
`evidence:` path.

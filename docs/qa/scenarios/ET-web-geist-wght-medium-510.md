---
id: ET-web-geist-wght-medium-510
area: ET
title: Geist Variable wght + UI medium 510
persona: Bruno
journey: J-operate-desktop-shell
expected: Runtime web and Storybook load Geist Variable on the wght axis only; the computed body `font-family` resolves to `Geist Variable`; no surface sets `font-optical-sizing` and no sans `font-feature-settings` character-variant block survives; every font-medium surface resolves to weight 510; the body baseline is 15px (0.9375rem) at line-height 1.55 and `--text-small-body` is 13.5px; the eyebrow computes Geist **sentence case** at 12px (`--text-eyebrow`) / 510 / `--tracking-eyebrow`, with uppercase reachable only through `<Eyebrow variant="caps">` and no other uppercase eyebrow source in the tree (L-022); UI titles and rows keep the DESIGN.md tracking ladder (detail-h1 / tight / body); tabular figures still align in Metric/KpiCard numeric columns.
entry_points: web SPA (`web/src/styles.css`); Storybook (`packages/ui/.storybook/preview.css`); site (`packages/site/app/layout.tsx` Geist loader); tokens `--font-weight-medium`, `--text-eyebrow`
qa_status: skipped
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-geist-migration-20260809/qa-artifacts/qa/typography; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps:
---

Replaces `ET-web-inter-opsz-medium-510` after the Inter → Geist migration (2026-08-09). Geist
exposes a `wght` axis only, so the old opsz and `cv01`/`ss03`/`cv11` assertions no longer describe
the runtime.

Verify against `docs/design/opendesign/_done/shell/compozy-refined-7.html` and `DESIGN.md` §3:
Geist Variable wght, UI medium 510, body tracking −0.006em, no optical sizing, no sans feature
block.

2026-08-20 retry: skipped by explicit user instruction. No current typography walk is claimed.

## Walk 2026-08-09 — pass

Walked the web SPA bundle (`vite build` output served statically), the `packages/ui` Storybook
(`preview.css` import site), and the `web` Storybook (`web/src/styles.css` import site) with
computed-style probes after `document.fonts.ready`.

- `--font-sans` resolves to `"Geist Variable", -apple-system, "BlinkMacSystemFont", sans-serif`;
  the only loaded sans face is `Geist Variable 100 900`. No `Inter*` face is present in
  `document.fonts` on any surface.
- `--font-weight-medium` is `510`; a `font-medium` element computes `font-weight: 510`.
  `--font-weight-display` computes `620` on `<Metric>` values.
- `--text-small-body` is `.78125rem` (12.5px). Body letter-spacing computes `-0.081px`
  (= −0.006em at 13.5px).
- Eyebrow computes Geist Variable / 11px / 600 / `-0.055px` (= −0.005em) / uppercase.
- Body `font-feature-settings` computes `normal`, and the built CSS contains zero occurrences of
  `font-optical-sizing`, `cv01`, `ss03`, or `cv11`.
- Tabular figures align: a `tabular-nums` span measures an identical `80.203125px` for
  `1111111111` and `8888888888`. MetricGrid columns render flush.
- Zero failed `.woff2` requests.

Payload note: the latin sans subset dropped from 72.9 KB (Inter opsz) to 28.7 KB (Geist wght),
and the emitted sans total from 340 KB / 7 subsets to 74.5 KB / 5 subsets.

Evidence: `computed-styles.json`, `computed-styles-2.json`, `web-storybook-probe.json`,
`metric-probe.json`, `storybook-eyebrow.png`, `metric-grid.png`, `web-dashboard.png`,
`web-design-system-typography.png` under the `evidence:` path.

The `web-spa.png` capture shows the route error state because the static server has no daemon
behind `/api`; the typography probe on that page is still valid and is corroborated by the two
Storybook surfaces, which render real components.

## 2026-08-20 qa-impact — reset, contract rewritten

The normie-friendly UI foundation pass changed the type contract this file owns, so the 2026-08-09
`pass` no longer describes the runtime. The walk above stays as the record of that walk; three of
its measurements are now retired:

- body baseline `0.84375rem` / 1.5 → **`0.9375rem` (15px) / 1.55** (`tokens-runtime.css:16`);
- `--text-small-body` 12.5px → **13.5px**;
- eyebrow `11px / 600 / uppercase` → **12px / 510 / sentence case**
  (`tokens-runtime.css` `@utility eyebrow` dropped `uppercase` and moved `font-semibold` to
  `font-medium`; `custom/eyebrow.tsx` added the opt-in `variant="caps"`).

The Geist wght-axis findings — one loaded sans face, `--font-weight-medium: 510`, no
`font-optical-sizing`, no `cv01`/`ss03`/`cv11`, tabular alignment, zero failed `.woff2` — are
untouched by this pass and are the parts most likely to still hold on re-walk.

L-022 is preserved, not weakened: there is still exactly one canonical eyebrow source. Only the
style that source emits changed, and uppercase became an explicit opt-in instead of the default.
Re-probe the site too (`packages/site/app/global.css`): its `--text-eyebrow` moved 11→12px and its
Fumadocs dark ramp now derives from the runtime tokens instead of a frozen hex block.

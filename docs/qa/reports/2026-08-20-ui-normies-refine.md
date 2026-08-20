# QA Run Report — 2026-08-20 — Normie-friendly UI refinement (de-sepia + label scale)

- **Scope:** Scoped re-walk after the owner-directed refinement of the normie-friendly foundation pass: surface ramp + text ladder neutralized (sepia/brown cast removed, lightness lift kept), eyebrow caps split into its own fixed rendition (11/600/+0.06em), sans pill / pill-group labels moved to `--text-eyebrow` (12px) with pill heights 18/20/24.
- **Cadence tier:** targeted (single scenario re-walk)
- **Build:** working tree on `ui-normies` over `origin/main`.
- **Environment:** `packages/ui` Storybook (scenario entry point) started and torn down by the walk (owned PID, confirmed dead; no daemon lab required — presentation-only diff with no runtime data path touched). Evidence: `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-refine-20260820/qa/typography/` (8 deterministic PNGs + `computed-style-probe.txt`).
- **Status:** Closed — walked scenario passed.
- **Automated precondition:** `make gate` escalated to `make verify` (token change); frontend lanes green in the run log (`@compozy/ui` 666 tests incl. the updated eyebrow contract test and the focus-ring contrast suite, `compozy-web` 5463, `@compozy/site` 322, zero-warning lint, web build). Final fingerprint recorded via `make gate-status`.

## Scenario verdicts

| Scenario | Verdict | Evidence |
|---|---|---|
| `ET-web-geist-wght-medium-510` | pass | Walk 2026-08-20 (refinement pass) in the scenario file: CDP computed-style probes after `document.fonts.ready` + PNG captures. Body 15px/1.55/−0.006em Geist Variable; eyebrow default 12/510 sentence case; caps 11/600/+0.06em uppercase via `.eyebrow-caps` only; pills 18/20/24 with 12px/510 sans labels; ramp/text tokens compute the neutralized values (chroma ≤0.006), contrast measured fg/canvas 15.45:1, muted/canvas 7.09:1. |

## Notes

- The durable charter `CH-plain-scale-legibility` stays re-runnable in later cycles; this walk covered its type-contract and ramp-contrast probes for the touched tokens. Full keyboard/screen-reader tour on the running shell remains owned by the charter's next full cycle.
- `DESIGN.md` §1 now pins the ramp warmth ceiling (OKLCH chroma ≤0.005, hue 60–90°) so the sepia drift class is doctrine-blocked, and §3 records the two fixed eyebrow renditions.

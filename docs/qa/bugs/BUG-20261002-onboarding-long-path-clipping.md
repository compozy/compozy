# BUG-20261002-onboarding-long-path-clipping: Long project paths push setup controls outside the panel

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-19, browse for a project folder
- **Scenarios:** RT-004; RT-onboarding-skip-to-global
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

- **Charter:** CH-onboarding-global-skip · **Tour:** Interrupt Tour
- **Environment:** Chrome, 1366×900, real isolated daemon and Web

1. Open first-run setup and choose a model with native sign-in.
2. Continue to Project and browse into a directory with a long absolute path.
3. Observe the selected-project column and Skip control.

**Expected:** Both columns stay within the panel; long paths truncate within their column.

**Actual:** The 960px panel body grows a 1030px scrollable width. The browser column becomes
806px wide and the second column extends past the panel. Skip and empty-state text are clipped;
scrolling the body to reach them then clips the heading on the opposite edge.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/onboarding-long-path-before.png`
- `docs/qa/evidence/2026-10-02-untested/onboarding-long-path-reproduced.json`
- `docs/qa/evidence/2026-10-02-untested/onboarding-commit-error.png`

## Fix

- **Root cause:** Both grid sections retain their intrinsic automatic minimum width. A long
  path therefore expands the grid track instead of letting the existing path truncation work.
- **Correction:** Allow both existing sections to shrink within their assigned grid columns.
- **Regression proof:** Browser replay owns this visual invariant. Do not add class-name or
  snapshot assertions that cannot establish rendered containment. A fresh model-to-project walk
  confirms a 960px body with exactly 960px scroll width, instead of 1030px. Both columns and
  controls fit at 1366, 1024, 768, and 390px viewports. The added project survives reload;
  removal and Skip return to persistent Global scope with zero projects and sessions.
- **Replay evidence:** `onboarding-long-path-fixed-desktop.json/png`,
  `onboarding-long-path-fixed-workspace.json`, `onboarding-fixed-layout-widths.json`,
  `onboarding-long-path-fixed-768.png`, and `onboarding-long-path-fixed-390.png`.
- **Limits:** Browser zoom and RTL were not verified; this replay uses the charter
  locale en-US. Commit and delivery checks remain pending.

# BUG-20260714-keyboard-focus-invisible: Keyboard focus is not visually distinguishable

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada; Bruno
- **Journey Step:** J-marketplace-acquisition, steps 1-5
- **Scenarios:** ET-web-marketplace-landing-browse; ET-web-marketplace-search-fanout; ET-web-marketplace-skill-install; ET-web-mcp-guided-install; ET-web-bundle-preview-activate; ET-web-ext-policy-block
- **Found:** 2026-07-14 · **Report:** `.compozy/tasks/marketplace/evidence/contrast/task-06.md`

## Summary

Keyboard users cannot reliably see which interactive control owns focus because the shared focus treatment is both too thin and too close in luminance to the surrounding surface.

## Reproduction

- **Charter:** UNCONFIRMED · **Tour:** Task 06 visual-contract audit
- **Environment:** Computed design-token audit on the production dark theme

1. Render a focusable card link or action that uses the shared focus token.
2. Move focus to it with the keyboard.
3. Compare the focus line with the adjacent surface.

**Expected:** A focus indicator at least 2 CSS pixels thick with at least 3:1 contrast against adjacent colors.
**Actual:** The shared one-pixel focus shadow computes to approximately `#2f2e2d` on `#131211`, about 1.38:1 contrast.

## Evidence

- `.compozy/tasks/marketplace/evidence/contrast/task-06.md`
- `packages/ui/src/tokens.css` shared focus token definition
- Resting and keyboard-focused site captures: `/Users/pedronauck/Dev/compozy/agh/.tmp/bug-20260714-focus/{resting,focused}.png`.
- Exact process teardown: `/Users/pedronauck/Dev/compozy/agh/.tmp/bug-20260714-focus/teardown.json` (`clean=true`).

## Fix

- **Root cause:** The shared focus token encodes a one-pixel, low-contrast shadow; this is a system design-token defect rather than a marketplace-local styling defect.
- **Correction:** The design-system owner now provides exclusive two-pixel outset and inset focus tokens at a verified contrast floor. Every low-contrast UI/Web consumer was migrated; accent focus rings use at least two pixels; the site walkthrough uses `focus-visible:ring-2`; and non-focus hairlines were hard-cut to neutral token names so they cannot be mistaken for focus indicators.
- **Fix commit:** pending Phase D checkpoint
- **Regression tests:** The `@agh/ui` token owner reads the generated CSS values and proves thickness plus at least 3:1 contrast across the complete surface cross-product. The design-system lint owner rejects one-pixel and low-contrast focus utilities across `packages/ui`, `web`, and `packages/site`.

## Verification

- **Retested:** 2026-07-16 on the real AGH site walkthrough plus fresh controller lanes.
- **Result:** Computed style is `rgb(232, 87, 42) 0 0 0 2px`, `:focus-visible` matches, and the focused capture shows a crisp accent ring on all four sides while the resting capture has no focus ring.
- Lint-plugin suite passed 32 tests; `make codegen-check` passed; UI passed 104 files / 532 tests; Web passed 420 files / 3,599 tests with zero lint warnings; site passed 50 files / 247 tests.

# BUG-20260730-docs-mobile-sidebar-offset: Docs content starts outside the mobile viewport

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-evaluate-compozy-beta, read docs on mobile
- **Scenarios:** ET-site-docs-sidebar-opendesign; ET-site-docs-first-session
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-29-site-improvs-deep-review.md

## Summary

At a 320 px viewport, docs pages inherited a 16 rem desktop sidebar width while the drawer was closed. The article began roughly 272 px to the right, leaving only a thin slice of the page visible.

## Reproduction

- **Charter:** CH-site-docs-marketplace-truth · **Tour:** Feature Tour
- **Environment:** mobile / wifi-fast / en-US, isolated `site-improvs-deep-review` lab

1. Open `/docs/loops` at 320 × 800.
2. Leave the mobile navigation drawer closed.
3. Inspect the article's left edge and horizontal overflow.

**Expected:** The closed mobile drawer consumes no layout width and the article fits the viewport.
**Actual:** The desktop sidebar width remained active and shifted the article mostly off-screen.

## Evidence

- Visual-contract implementation capture `VC07-sidebar-mobile` in the run QA output.

## Fix

- **Root cause:** Site CSS overrode Fumadocs' responsive `--fd-sidebar-width` with an unconditional 16 rem width on both the layout and sidebar.
- **Fix:** The sidebar now follows `--fd-sidebar-width`, and the 16 rem site override applies only from the desktop breakpoint.
- **Fix commit:** Working tree; this review task did not authorize a commit.
- **Regression gate:** Final responsive visual-contract capture plus the site lint, test, and build lanes.

## Verification

- **Retested:** 2026-07-30 at 320 × 800 in the isolated site server.
- The closed drawer consumes zero horizontal space; the article, masthead, and code content render inside the viewport without horizontal displacement.


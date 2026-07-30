# BUG-20260730-docs-index-invalid-hydration: Docs landing hydrates invalid nested paragraphs

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-evaluate-compozy-beta, enter the docs
- **Scenarios:** ET-site-docs-masthead-opendesign; ET-site-docs-single-tree-ia
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-29-site-improvs-deep-review.md

## Summary

The docs landing rendered group descriptions inside a paragraph nested within another paragraph. React reported a hydration error and could replace the affected subtree on the client.

## Reproduction

- **Charter:** CH-site-docs-marketplace-truth · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, isolated `site-improvs-deep-review` lab

1. Open `/docs` in a fresh browser session.
2. Inspect the browser console during hydration.
3. Inspect the group-index markup around a description.

**Expected:** Server and client markup hydrate without errors and use valid paragraph nesting.
**Actual:** A `GroupRow` paragraph contained the description component's paragraph.

## Evidence

- Fresh `site-docs-fixed` browser session before and after the correction; no console errors remain after retest.

## Fix

- **Root cause:** `GroupRow` used a paragraph as its outer text container even though MDX descriptions can render paragraph elements.
- **Fix:** The outer container is now a neutral block element, preserving typography without invalid nesting.
- **Fix commit:** Working tree; this review task did not authorize a commit.
- **Regression gate:** Real browser hydration plus the site test and build lanes.

## Verification

- **Retested:** 2026-07-30 in a fresh browser session.
- `/docs` hydrates without console errors, and the group-index structure remains visible and navigable.


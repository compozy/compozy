# BUG-20260730-sidebar-close-lost-reload: Active docs folder reopens after reload

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-evaluate-compozy-beta, navigate the docs tree
- **Scenarios:** ET-site-docs-sidebar-opendesign
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-29-site-improvs-deep-review.md

## Summary

Closing the active folder wrote the intended persisted value, but a reload expanded it again. The initial client effect overwrote the stored closed choice before restoring it.

## Reproduction

- **Charter:** CH-site-docs-marketplace-truth · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, isolated `site-improvs-deep-review` lab

1. Open `/docs/loops`.
2. Close the active Loops folder.
3. Reload the page.

**Expected:** The explicit closed choice remains authoritative; the Loops Overview link remains independently navigable.
**Actual:** The active-route server fallback reopened the folder during hydration.

## Evidence

- Fresh `site-docs-fixed` browser session: persisted value `0` before and after reload; trigger remains collapsed after the fix.

## Fix

- **Root cause:** The persistence writer stored the server-safe active fallback before reading the existing client choice.
- **Fix:** Hydration now reads and applies the persisted choice first, then writes subsequent user changes. The obsolete external-store hook and duplicate suite were removed.
- **Fix commit:** Working tree; this review task did not authorize a commit.
- **Regression test:** `packages/site/components/site/__tests__/sidebar-compact-tree.test.tsx` owns the SSR-to-hydration close-persistence invariant.

## Verification

- **Retested:** 2026-07-30 in a fresh browser session.
- Closing Loops writes `0`; a real reload leaves the folder collapsed and the current document visible. The canonical focused Turbo suite passed 5/5.


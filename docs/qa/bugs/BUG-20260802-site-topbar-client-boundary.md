# BUG-20260802-site-topbar-client-boundary: Shared topbar hook crashed every docs route

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-bundle-product-boundary, verify surviving and retired docs routes
- **Scenarios:** ET-site-docs-api-reference-ui
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-bundles-removal.md
- **Origin:** Task 06 real-user QA

## Summary

The shared topbar-slot hook called React `createContext` without declaring a client boundary. Next.js
therefore treated the module as a Server Component and returned HTTP 500 for every docs page.

## Reproduction

1. Start the real documentation site.
2. Open `/docs/resources/` or `/docs/api/extensions/`.
3. Observe `createContext only works in Client Components` and HTTP 500.

**Expected:** Surviving docs routes render normally and retired Bundle routes return the site's
not-found page.
**Actual:** The shared layout crashed before either route contract could render.

## Evidence

- Final surviving route capture: `qa/site-api-extensions.png`.
- Final retired route capture: `qa/site-api-bundles-not-found.png`.

## Fix

- **Root cause:** `packages/ui/src/components/custom/hooks/use-topbar-slot.ts` lacked the Next.js
  client-module directive required by its React context.
- **Correction:** The hook module is explicitly client-only.
- **Fix commit:** `a817e37`
- **Regression gate:** The real site production build owns the Server/Client module boundary.

## Verification

- `/docs/resources/` and `/docs/api/extensions/` returned 200.
- `/docs/api/bundles/` and `/docs/cli/bundle/` returned 404.
- The focused Turborepo site build passed all four tasks and generated 1,948 pages.


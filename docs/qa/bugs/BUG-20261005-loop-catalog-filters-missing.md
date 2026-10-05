# BUG-20261005-loop-catalog-filters-missing: Loop catalog omits kind and category filters

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-01, browse the Loop catalog
- **Scenarios:** LP-001
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The catalog offers only a Status filter, so an operator cannot narrow the list to
custom Loops or a category through the interface. The public API already supports
both filters and returns authoritative facets across the counted catalog.

## Reproduction

- **Charter:** CH-001 · **Tour:** Feature Tour
- **Environment:** laptop 1512×862, DPR 2, en-US, Wi-Fi; Studio Operations / resume-editorial

1. Open Loops with built-in Engineering definitions and custom Operations definitions.
2. Open Add filter.

**Expected:** Kind, Category and Status filters are available.
**Actual:** Only Status is offered, despite the server reporting both kinds and categories.

## Evidence

- docs/qa/evidence/2026-10-02-untested/loops-pagination-lea-missing-filters.png
- docs/qa/evidence/2026-10-02-untested/loops-pagination-independent-1.json
- docs/qa/evidence/2026-10-02-untested/loops-pagination-independent-2.json

## Fix

- **Root cause:** The Loop filter field configuration and toolbar expose only status;
  the existing route parser and server query already handle kind and category.
- **Scope:** Compose the existing filter primitive with both kind options and category
  values from server facets. Commit all chip changes in one route update, preserving
  search and view. No migration or public contract change.
- **Fix commit:** 60ddd98e12e10731a7f98d8dedca20ff88f2e459
- **Regression test:** Existing loop-list-filters.test.ts owns offered options and the
  typed chip/filter projection, including clearing and invalid values. The real catalog
  replay owns server filtering and counted pagination.

## Verification

- **Retested:** 2026-10-05, original-persona replay passed; see completed replay below.

## First repair replay

Kind, Category and all status values now filter through the server. Cards extends 50 to 53
definitions. The combined Custom + Operations + Canceled filter returns a truthful zero
through Web and independent UDS, and survives reload. That zero result exposes a missing
selected-category label: the primitive cannot label Operations once its facet vanishes.
The field configuration now includes the selected category alongside server facets, without
inventing counts. The existing projection suite owns this empty-facet case.

Evidence: loops-catalog-fixed-lea-{cards,kind,category-status,search-observation}.json and
loops-catalog-fixed-independent-empty.json. The earlier Status submenu observation is
resolved by its standard ArrowRight interaction; all twelve daemon statuses are offered.


## Completed replay — 2026-10-05

Lea selects Custom + Operations + Canceled, sees zero results with all three labels,
reloads, then removes only Status and pages all 51 matching copies. Built-in + Engineering
+ Done returns exactly implement-tasks in Cards and Rows after reload. Independent HTTP/UDS
reads agree on names, exact totals/facets and the completed run's 1/1 30-day aggregate.
The 35-frame loops-catalog-complete-lea recording is closed. Selected-category, catalog and
draft-owner suites pass 34 tests; root Turbo lint/typecheck/build pass. Fix commit: 60ddd98e12e10731a7f98d8dedca20ff88f2e459.

Evidence: loops-catalog-complete-lea-{filters,done}.json, labeled-empty/labeled-reload/
done-rows-reloaded PNGs, and loops-catalog-complete-{custom,done}-readback.json under
docs/qa/evidence/2026-10-02-untested/. The labeled-empty screenshot was visually inspected.


## Verified delivery — 2026-10-05

Fix commit: 60ddd98e12e10731a7f98d8dedca20ff88f2e459. Original-persona replay and the owning checks pass.
The warning-free affected gate passes, and the commit tree exactly matches
f1493b4c0a97be585cc4fe2e60b941df61a20ba4. Receipts: qa-catalog-delivery-gate-6.json
and qa-catalog-repairs-commit-identity.json in docs/qa/evidence/2026-10-02-untested/.

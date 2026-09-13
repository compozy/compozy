# BUG-20260913-marketplace-search-loading-visuals: Search emphasis and loading state differ from the approved board

- **Status:** verified
- **Impact (user-side):** Paper-Cut
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-marketplace-acquisition, search and initial loading
- **Found:** 2026-09-13 · **Report:** reports/2026-09-13-marketplace-catalog.md

## Reproduction

The Search story filters successfully but renders no query marks. The Loading story uses the default
shimmer from Skeleton despite the approved board requiring static blocks.
Reference: docs/design/opendesign/marketplace-catalog/marketplace-catalog-browse.html, br-search/br-loading.
Before-repair captures: task_01/VC-02 and VC-03 in the report's lab visual-contract root.

## Fix and Verification

Existing entry cards now mark literal query occurrences in title and description, preserving links,
text and server filtering. Existing Skeleton primitives receive static visual classes at their owning
grid. No parallel primitive or filtering implementation was introduced.
The canonical component suite proves literal punctuation, case-insensitive occurrence matching and
unchanged text/destination; all54tests pass. Web typecheck passes. Updated VC-02/03 captures inspected: literal query marks and static skeleton match the specified states. Live search found3review entries with3marks; slash focuses and Esc clears the field. Evidence search-live.png. Fix commit pending.

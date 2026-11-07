# BUG-20261107-catalog-entries-uncategorized-duplicated: Curated catalog entries all land in More and duplicate compozy-plugins apps

- Status: open
- Impact (user-side): Degraded browse
- Severity: Medium; Priority: P2
- Persona Affected: Bruno
- Journey Step: Browse categories with the default sources
- Scenarios: ET-web-marketplace-landing-browse
- Origin: spec marketplace-plugin-catalog S8 integrated walk (isolated lab, 2026-11-07)

## Reproduction

`catalog/v3/extensions.json` (the `compozy-catalog` feed) carries no category for its 18 entries, so
Notion, Linear, GitHub, Stripe and the others all resolve to **More**. With `compozy-plugins` enabled,
`compozy marketplace search notion` lists both `compozy-catalog/notion` (More) and
`compozy-plugins/notion` (Productivity, featured); Linear likewise. No `supersedes` covers the
`compozy-catalog` rows, contrary to F7 ("never lists the same app twice next to the existing default
sources").

Expected: curated feed entries carry Compozy categories, and each app is listed once.

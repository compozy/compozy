# BUG-20261107-catalog-compatibility-digest-stale: Hand-edited catalog feed no longer matches its compatibility digest

- Status: verified (fixed on feat/marketplace-plugin-catalog)
- Impact (user-side): Blocking once published (the whole CompozyOS catalog fails validation)
- Severity: High; Priority: P1
- Persona Affected: Bruno, Ada
- Journey Step: Refresh the CompozyOS catalog feed
- Scenarios: ET-web-marketplace-landing-browse
- Origin: spec marketplace-plugin-catalog S8 re-walk (isolated lab, 2026-11-07)

## Reproduction

Commit 30fcdfee8 changed `catalog/v3/extensions.json` (categories, featured, supersedes) without
regenerating `catalog/v3/compatibility.json`. Its `feed_digest_sha256` is still `cca095c2…`, while the file
hashes to `bde95da6…`. A daemon whose `marketplace.catalog.base_url` points at this catalog reports
`compozy-catalog` failed, `error_class: validation`, "compatibility metadata does not match the extension
feed" (`internal/marketplace/compatibility.go:26`), and lists none of its 18 entries.

`go run ./cmd/compozy-catalog publish ./catalog <out>` regenerates a byte-identical `extensions.json`
and a `compatibility.json` with the new digest; with that output the feed loads.

Expected: regenerate through the generator and commit `compatibility.json` together with the feed.

## Re-walk 2026-11-07 phase 4 (integration 92b36f639, fresh isolated lab, committed feed)

`catalog/v3/compatibility.json` records `bde95da6…`, matching `extensions.json`. A lab whose
`marketplace.catalog.base_url` points at the committed `catalog/` refreshes `compozy-catalog`
successfully (18 entries) without the temporary publish.

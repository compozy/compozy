# BUG-20261107-marketplace-empty-version-500: Marketplace list and sections return 500 when any listed plugin has no version

- Status: open
- Impact (user-side): Blocking
- Severity: Critical; Priority: P0
- Persona Affected: Bruno, Ada
- Journey Step: Open Marketplace with the default sources enabled after a successful refresh
- Scenarios: ET-web-marketplace-landing-browse, ET-cli-marketplace-search
- Origin: spec marketplace-plugin-catalog S8 integrated walk (isolated lab, 2026-11-07)

## Reproduction

Isolated lab on `feat/mpc-s8` (integration branch with S1–S7 and S9 merged), default sources on.
After `compozy marketplace refresh` succeeds for `claude-plugins-official` (316) and
`compozy-plugins` (333), about 30 catalog rows have an empty `version`. 29 come from
`claude-plugins-official`: `url` sources pinned with `sha` and no `version` (for example
`data-engineering`: `{"source":"url","url":"https://github.com/astronomer/agents.git","sha":"35b9…"}`).
One comes from `compozy-plugins` (`desktop-commander`).

- `GET /api/marketplace?limit=100` → `500 {"error":"Internal Server Error"}`
- `GET /api/marketplace/sections` → 500; Web Browse home shows "The marketplace is unreachable / No
  sources responded", which is also wrong.
- `GET /api/marketplace?category=` returns 500 for productivity, design, development, data and more,
  and 200 for the categories without such a row.
- `compozy marketplace search --category more --limit 100` and `compozy marketplace info data-engineering
  --source claude-plugins-official` → `extension: curated marketplace version is required for
  "data-engineering"`.

Cause: `internal/api/core/marketplace_list.go` (listing projection) calls
`Extensions.MarketplaceTrust` with `Version: entry.Version`; `MarketplaceTrustEvidence.validate`
(`internal/extension/marketplace_trust.go:187`) rejects an empty version, and the error fails the
whole page. The decoder also ignores the Claude `sha` pin on `url` sources.

Expected: every plugin row has a version (the pinned `sha`/`ref` or resolved commit when the
manifest has none), and one bad row never fails a page; it should list with an install blocker.

Evidence: /tmp/mpc-shots/s8/web-browse-home-dark-DEFECT-500.png, web-category-page-productivity-DEFECT-500.png (lab-local, not committed).

---
id: ET-site-marketplace-catalog
area: ET
title: Public /marketplace renders the real catalog feeds with install-command CTAs
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace shows one section per kind (Skills, Extensions, MCP servers — no bundles) with every entry from catalog/*.json, a Contribute card pointing at the catalog PR flow, and a line about compozy skill search / compozy extension search third-party registries. /marketplace/[kind] lists that kind; /marketplace/[kind]/[entryId] shows metadata plus a copyable install command (compozy skill install <slug> / compozy extension install <slug> / compozy mcp install <entry>) and kind-specific blocks: extension tier + digest + repository; MCP transport, env table with secrets flagged but never valued, and default scope. No ratings, downloads, featured flags, or other invented fields appear anywhere.
entry_points: compozy.com /marketplace; /marketplace/skills; /marketplace/extensions; /marketplace/mcp/context7; /marketplace/bridges; /marketplace/bundled/dev-cycle
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: 2026-07-29 walk on a local `next build` + `next start :4598`. All routes returned 200: /docs/, /docs/examples/ and its five wave-one pages, /marketplace/, /marketplace/{skills,mcp,extensions}/, /marketplace/bridges/, /marketplace/bundled/dev-cycle/, /marketplace/mcp/context7/, and the bridge setup guides. Visual-contract bundles VC-01..VC-05 under .compozy/tasks/site-docs-ia/evidence/visual/gap-closure/ validate PASS with 0 blocking divergences. VC-01/02/03 cover the hero with its feed-pipeline figure and counted stat strip, the kind list shell with tab strip and filter, and the extension detail with crumbs, meta strip, install CTA, and icon-headed Provenance. Rendered-HTML assertions confirm no Bundles kind and no invented trust field on any marketplace route.
last_report:
overlaps: ET-site-docs-single-tree-ia
---

Added 2026-07-29 with the site IA restructure (spec `.compozy/tasks/site-docs-ia/_spec.md`
Phase B): `/marketplace` is a build-time render of `catalog/skills.json`, `catalog/extensions.json`,
and `catalog/mcp.json` — the same feeds `internal/config/marketplace.go` points the daemon at.
Browse-only by design (the site has no daemon); the CTA is the CLI command. Re-walk when the
catalog population workstream (spec §9) lands new entries or when the feed schema changes.

QA impact 2026-07-29: `/marketplace` was rebuilt to the OpenDesign reference
(`docs/design/opendesign/site/site-marketplace*.html`). New surfaces: a two-column hero with a
feed-pipeline figure and a stat strip counted from the repository, a Bridge providers section plus
`/marketplace/bridges` derived from `extensions/bridges/*/extension.toml`, and a
`Ships with the runtime` section plus `/marketplace/bundled/dev-cycle` derived from
`extensions/dev-cycle/extension.json`. `/marketplace/[kind]` gained the reference list shell — a
kind tab strip with counts and a client-side filter with a no-match state — and
`/marketplace/[kind]/[entryId]` gained masthead crumbs, an identity meta strip, and icon-headed
sections. Reset to `untested` because every marketplace route changed. Deliberate deltas to verify as
absences: no Bundles section (three feed kinds only) and no Featured spotlight.

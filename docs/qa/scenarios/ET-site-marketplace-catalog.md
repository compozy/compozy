---
id: ET-site-marketplace-catalog
area: ET
title: Public /marketplace renders the checked-in catalog snapshot with daemon-search CTAs
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace identifies itself as a checked-in catalog snapshot, shows one section per kind (Skills, Extensions, MCP servers — no bundles) with every entry from catalog/*.json, and offers a Contribute card pointing at the catalog PR flow. /marketplace/[kind] lists that kind; /marketplace/[kind]/[entryId] shows metadata plus a copyable `compozy marketplace search <entry-id> --kind <kind>` command so the daemon resolves the entry against its configured active source before installation. Kind-specific blocks show extension tier + digest + repository, or MCP transport, env fields with secrets flagged but never valued, and default scope. No ratings, downloads, featured flags, or other invented fields appear anywhere.
entry_points: compozy.com /marketplace; /marketplace/skills; /marketplace/extensions; /marketplace/mcp/context7; /marketplace/bridges; /marketplace/bundled/dev-cycle
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc04-marketplace-root; /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc05-marketplace-list; /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc06-marketplace-detail
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: ET-site-docs-single-tree-ia
---

Added 2026-07-29 with the site IA restructure (spec `.compozy/tasks/site-docs-ia/_spec.md`
Phase B): `/marketplace` is a build-time render of `catalog/skills.json`, `catalog/extensions.json`,
and `catalog/mcp.json` — the same feeds `internal/config/marketplace.go` points the daemon at.
Browse-only by design (the site has no daemon); the CTA is the CLI command. Re-walk when the
catalog population workstream (spec §9) lands new entries or when the feed schema changes.

QA impact 2026-07-29 deep-review remediation: reset after the static snapshot was labeled explicitly,
entry actions changed from unverified install commands to active-daemon search commands, feed validation
was aligned with the daemon contract, and Marketplace typography/layout changed.

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

---
id: ET-site-marketplace-catalog
area: ET
title: Public /marketplace renders the checked-in catalog snapshot with daemon-search CTAs
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace renders one searchable extension catalog from v3, including the 17 packaged MCP servers and three existing extensions. Direct /marketplace/<entry_id> details show actual metadata, inputs, provenance and current extension commands. Retired kind paths are not found. Bundled resources and bridge-provider setup remain separate and usable. No old feed fallback, invented runtime state, popularity or secret values appear.
entry_points: compozy.com /marketplace; /marketplace/context7; /marketplace/herdr-bridge; /marketplace/bridges; /marketplace/bundled/spec-cycle
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/site-readback.json
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps: ET-site-docs-single-tree-ia
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.


Marketplace catalog task02 (2026-09-13): validate only v3 and the actual 20 package entries.
Verify search, direct detail links, icon fallback, typed input prompts, preserved existing
extension refs/digests, and distinct GitHub/Linear bridge setup. Retired skill/MCP/kind URLs
must return not-found without redirects. Documentation Writer has no standalone listing.
Final tasks09/10 own live rendering, keyboard/narrow-screen and visual evidence; focused
schema/index tests do not close this row.

## React Doctor follow-up — 2026-09-14

Raster feed icons use Next Image with fixed dimensions and responsive optimizer URLs.
The remote allowlist comes from the checked-in v3 feed. Verify successful image delivery
once the feed's `main/catalog/icons/*` assets are published; before merge those upstream
URLs return 404. The real development-server walk of `/marketplace/context7/` verified
that this failure retains the seeded logo fallback and readable page content. It did not
prove successful remote image optimization. The existing catalog suite separately verifies
optimizer markup with the real Next Image component and the error fallback.
See [the focused follow-up report](../reports/2026-09-14-marketplace-react-doctor.md).

## Historical evidence

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
`Ships with the runtime` section plus `/marketplace/bundled/spec-cycle` derived from
`extensions/spec-cycle/extension.json`. `/marketplace/[kind]` gained the reference list shell — a
kind tab strip with counts and a client-side filter with a no-match state — and
`/marketplace/[kind]/[entryId]` gained masthead crumbs, an identity meta strip, and icon-headed
sections. Reset to `untested` because every marketplace route changed. Deliberate deltas to verify as
absences: no Bundles section (three feed kinds only) and no Featured spotlight.

QA result 2026-07-30: the public MCP listing rendered the checked-in 17-entry manifest-v2 snapshot,
and GitHub detail rendered its launch metadata and secret input name without a value. The daemon API
returned the same 17 entry IDs.

QA impact 2026-07-30 deep-review remediation: reset after manifest-v2 site validation gained daemon
parity for launch identifiers, digests, public HTTPS remotes, argument safety, typed defaults, and
duplicate input destinations. Entry details must pair `--set`, `--secret`, and `--vault-ref` with the
owning `compozy mcp install` command and continue to expose no secret value.

QA impact 2026-08-20: `/marketplace/[kind]` filter field height now uses `--height-search` (28px)
instead of a raw `h-8`. Reset the kind-list chrome walk.

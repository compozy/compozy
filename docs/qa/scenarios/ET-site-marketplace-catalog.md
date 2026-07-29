---
id: ET-site-marketplace-catalog
area: ET
title: Public /marketplace renders the real catalog feeds with install-command CTAs
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace shows one section per kind (Skills, Extensions, MCP servers — no bundles) with every entry from catalog/*.json, a Contribute card pointing at the catalog PR flow, and a line about compozy skill search / compozy extension search third-party registries. /marketplace/[kind] lists that kind; /marketplace/[kind]/[entryId] shows metadata plus a copyable install command (compozy skill install <slug> / compozy extension install <slug> / compozy mcp install <entry>) and kind-specific blocks: extension tier + digest + repository; MCP transport, env table with secrets flagged but never valued, and default scope. No ratings, downloads, featured flags, or other invented fields appear anywhere.
entry_points: compozy.com /marketplace; /marketplace/skills; /marketplace/extensions; /marketplace/mcp/context7
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: 2026-07-29 walk on local `next build` + `next start :4123` — screenshots /tmp/site-shots/{marketplace,marketplace-mcp-detail}.png show the three kind sections with the real documentation-writer, repository-orientation (Official tier + sha256 digest), and context7 entries, install commands with copy affordances, the third-party-registry pointer, and the context7 detail with stdio transport, command line, global default scope, and the CONTEXT7_API_KEY row flagged Secret with no value; /marketplace/, /marketplace/extensions/, and /marketplace/extensions/repository-orientation/ all returned 200. Feed drift protection covered by lib/__tests__/marketplace-catalog.test.ts (strict zod parse fails the build).
last_report:
overlaps: ET-site-docs-single-tree-ia
---

Added 2026-07-29 with the site IA restructure (spec `.compozy/tasks/site-docs-ia/_spec.md`
Phase B): `/marketplace` is a build-time render of `catalog/skills.json`, `catalog/extensions.json`,
and `catalog/mcp.json` — the same feeds `internal/config/marketplace.go` points the daemon at.
Browse-only by design (the site has no daemon); the CTA is the CLI command. Re-walk when the
catalog population workstream (spec §9) lands new entries or when the feed schema changes.

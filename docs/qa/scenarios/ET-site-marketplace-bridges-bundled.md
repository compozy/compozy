---
id: ET-site-marketplace-bridges-bundled
area: ET
title: Marketplace browses bridge providers and runtime-bundled resources without false install claims
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace shows a Bridge providers section listing every in-tree provider from `extensions/bridges/*/extension.toml` with its platform, version, real secret-slot count, an Alpha chip, and a Setup guide link that resolves under /docs/bridges; /marketplace/bridges repeats the grid with the build-from-source commands. A `Ships with the runtime` section lists the bundled spec-cycle extension and the bundled `compozy` skill, and /marketplace/bundled/spec-cycle shows its real inventory (3 loops with use-when text, 9 skills, 3 agents, 3 tools with risk/visibility flags) plus provenance. No bundled or bridge surface offers an install command — bridges show a build path and spec-cycle shows `compozy extension status spec-cycle` — and `bridges` never appears as a catalog feed kind.
entry_points: compozy.com /marketplace; /marketplace/bridges; /marketplace/bundled/spec-cycle; /docs/bridges/setup-slack
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-14-review-handoff-spec-cycle/site-marketplace-spec-cycle.png
last_report: docs/qa/reports/2026-08-14-review-handoff-spec-cycle.md
overlaps: ET-site-marketplace-catalog; ET-spec-cycle-skill-bundle; ET-compozy-official-skill-discovery
---

Added 2026-07-29 with the site IA gap closure (spec `.compozy/tasks/site-docs-ia/_spec.md` §7). The
reference marketplace shows bridges and a featured first-party extension, but neither can be a catalog
entry today, and rendering them as one would have been a false claim:

- Every bridge manifest points `[subprocess]` at a locally built `./bin/<platform>`, so there is no
  cross-platform artifact to publish with a digest. Providers build from source, hence Alpha.
- `extensions/spec-cycle` is enrolled from the binary at first boot by `EnsureManagedInstall` with
  `extensionpkg.SourceBundled` (`internal/daemon/boot_automation_bundles.go`), so
  `compozy extension install compozy/spec-cycle` would be untrue and a feed entry would collide with
  that managed install. The nine `cy-*` skills live inside it, and `compozy skill install` resolves
  against ClawHub (`internal/cli/skill_marketplace.go`), so feeding them would print failing commands.

The walk owns the truthfulness edge: confirm no install command appears on either surface, that the
counts match the repository, and that the Setup guide links resolve. Feed-kind containment and count
parity are additionally gated by `lib/__tests__/marketplace-catalog.test.ts`.

QA impact 2026-07-29 deep-review remediation: reset after bridge tiles were recomposed from shared UI
primitives and Marketplace layout/token usage changed.

QA impact 2026-08-14: reset because the bundled inventory now contains three Loops and the detail
route is shared across page metadata, cards, sitemap, and public search.

QA result 2026-08-14: passed on the local production-shaped site. The bundled detail showed three
Loops, nine skills, three agents, three tools, the status command, and no install claim. Public search
resolved Spec Cycle to the shared detail route; reload and browser history preserved it, while an
unknown bundled slug returned the published not-found page.

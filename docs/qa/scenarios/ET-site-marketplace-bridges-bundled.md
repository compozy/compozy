---
id: ET-site-marketplace-bridges-bundled
area: ET
title: Marketplace browses bridge providers and runtime-bundled resources without false install claims
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace shows a Bridge providers section listing every in-tree provider from `extensions/bridges/*/extension.toml` with its platform, version, real secret-slot count, an Alpha chip, and a Setup guide link that resolves under /docs/bridges; /marketplace/bridges repeats the grid with the build-from-source commands. A `Ships with the runtime` section lists the bundled dev-cycle extension and the bundled `compozy` skill, and /marketplace/bundled/dev-cycle shows its real inventory (2 loops with use-when text, 9 skills, 3 agents, 3 tools with risk/visibility flags) plus provenance. No bundled or bridge surface offers an install command — bridges show a build path and dev-cycle shows `compozy extension status dev-cycle` — and `bridges` never appears as a catalog feed kind.
entry_points: compozy.com /marketplace; /marketplace/bridges; /marketplace/bundled/dev-cycle; /docs/bridges/setup-slack
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/raw/implementation/marketplace-bridges.png
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: ET-site-marketplace-catalog; ET-dev-cycle-skill-bundle; ET-compozy-official-skill-discovery
---

Added 2026-07-29 with the site IA gap closure (spec `.compozy/tasks/site-docs-ia/_spec.md` §7). The
reference marketplace shows bridges and a featured first-party extension, but neither can be a catalog
entry today, and rendering them as one would have been a false claim:

- Every bridge manifest points `[subprocess]` at a locally built `./bin/<platform>`, so there is no
  cross-platform artifact to publish with a digest. Providers build from source, hence Alpha.
- `extensions/dev-cycle` is enrolled from the binary at first boot by `EnsureManagedInstall` with
  `extensionpkg.SourceBundled` (`internal/daemon/boot_automation_bundles.go`), so
  `compozy extension install compozy/dev-cycle` would be untrue and a feed entry would collide with
  that managed install. The nine `cy-*` skills live inside it, and `compozy skill install` resolves
  against ClawHub (`internal/cli/skill_marketplace.go`), so feeding them would print failing commands.

The walk owns the truthfulness edge: confirm no install command appears on either surface, that the
counts match the repository, and that the Setup guide links resolve. Feed-kind containment and count
parity are additionally gated by `lib/__tests__/marketplace-catalog.test.ts`.

QA impact 2026-07-29 deep-review remediation: reset after bridge tiles were recomposed from shared UI
primitives and Marketplace layout/token usage changed.

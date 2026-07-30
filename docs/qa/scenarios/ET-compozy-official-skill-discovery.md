---
id: ET-compozy-official-skill-discovery
area: ET
title: Discover the official Compozy runtime skill
persona: Ada
journey: J-validate-compozy-hard-cut
expected: A fresh runtime discovers the bundled skill only as `compozy`; HTTP, UDS, CLI, native-tool, and Web reads agree on its declaration and activation state; metadata.compozy gates are enforced while foreign metadata is ignored; no retired skill alias or duplicate catalog entry exists.
entry_points: bundled skills/compozy/SKILL.md; GET /api/skills; compozy skill list|inspect|view -o json; compozy__skill_list|view; Web /skills
qa_status: skipped
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-web-final-2.log; /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-001; ET-002; ET-003; ET-skill-activation-gates
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: the bundled skill was read through CLI only; the scenario requires agreement across all listed surfaces.

QA impact 2026-07-26: the official bundled skill, loader namespace, and every structured
management surface received one hard cut. Planning flag only; the next QA cycle owns execution.

QA impact 2026-07-29 (ext-improvs Phase G): the bundled skill gained
`references/extension-authoring.md` and a new router row for writing extension code. Resource
discovery, the reference inventory, and router coverage changed; reset to untested for the next
cycle.

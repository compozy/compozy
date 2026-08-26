---
id: ET-compozy-official-skill-discovery
area: ET
title: Discover the official CompozyOS runtime skill
persona: Ada
journey: J-validate-compozy-hard-cut
expected: A fresh runtime discovers the bundled skill only as `compozy`; every read plane agrees, the router serves the desktop reference and teaches app commands, ownership, updates, diagnostics, and recovery without a duplicate catalog entry.
entry_points: bundled skills/compozy/SKILL.md; GET /api/skills; compozy skill list|inspect|view -o json; compozy__skill_list|view; Web /skills
qa_status: pass
bug_ids: BUG-20260825-skill-source-agent-write-doc-mismatch
fix_status: fixed
retest_status: pass
fix_commits: 2643f4aba
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/info-compozy.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/tool-skill-view.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-001; ET-002; ET-003; ET-skill-activation-gates
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: the bundled skill was read through CLI only; the scenario requires agreement across all listed surfaces.

QA impact 2026-07-26: the official bundled skill, loader namespace, and every structured
management surface received one hard cut. Planning flag only; the next QA cycle owns execution.

QA impact 2026-07-29 (ext-improvs Phase G): the bundled skill gained
`references/extension-authoring.md` and a new router row for writing extension code. Resource
discovery, the reference inventory, and router coverage changed; reset to untested for the next
cycle.

QA impact 2026-08-02: the official skill removed Bundle product teaching and gained kit resources,
secrets, inventory, preview, and digest confirmation guidance. Reset for a real agent read.

QA impact 2026-08-04: the official skill was cut to agent-facing usage only. Removed
`references/contributing-to-compozy.md`, `references/docs-design-and-copy.md`, and
`references/qa-and-verification.md`; renamed `references/capabilities.md` to
`references/extensions.md`; added `references/configuration.md`; rewrote the SKILL.md
description, router, and index. Verify no retired reference or router row survives on any read
plane and that the new references resolve.

QA walk 2026-08-04 (fresh isolated lab, home `compozy-skill-slim-20260804-111531-lab/home`,
port 42137): `compozy skill list -o json` reports exactly one bundled skill `compozy`; the served
router body contains 32 reference mentions and zero retired names; global-context
`skill view --file` serves `references/extensions.md` and `references/configuration.md` and
rejects `references/qa-and-verification.md` and `references/capabilities.md` with
"file does not exist"; `skill inspect -o json` lists exactly the twelve current references;
`GET /api/skills` carries the new description (native-tool and Web planes read the same registry
projection). Note: workspace-scoped `skill view` rejects `--file` by contract, so file reads were
verified from a global (non-workspace) cwd. Daemon teardown clean on both runs.

QA impact 2026-08-06: the official skill now routes connectivity-provider authoring and runtime
operations. Flag only; Tasks 08–09 own the re-walk.

QA walk 2026-08-07: a fresh isolated runtime served the new authoring and runtime-operation
references through `skill view`; both provider templates, the provider contract, and Gateway native
operations were present. Existing cross-plane discovery evidence remains valid for the unchanged registry.

QA impact 2026-08-10: the official skill gained `references/desktop.md` and desktop routing while
its public prose moved to CompozyOS. Reset to `untested`; Task 07 owns the cross-plane re-walk.

QA impact 2026-08-25 (skill sources): already `untested`, and this cycle adds a reason to walk it. Task_07 rewrote two of the bundled skill's references — `references/configuration.md` (the two source keys, their scopes, and the agent-write claim) and `references/tools-and-skills.md` (origins, precedence, suppression, exposure). The scenario's promise is that every read plane agrees on one `compozy` entry; this cycle's specific risk is that the served body now describes behavior the runtime does not implement. Read the configuration reference against the shipped tool surface, not just against itself — see `ET-skill-source-agent-parity` for the agent-write discrepancy this cycle found. Rides along in `CH-skill-sources-agent-plane`.

---
id: ET-compozy-official-skill-discovery
area: ET
title: Discover the official Compozy runtime skill
persona: Ada
journey: J-validate-compozy-hard-cut
expected: A fresh runtime discovers the bundled skill only as `compozy`; HTTP, UDS, CLI, native-tool, and Web reads agree on its declaration and activation state; metadata.compozy gates are enforced while legacy metadata.agh is ignored; no legacy `agh` skill alias or duplicate catalog entry exists.
entry_points: bundled skills/compozy/SKILL.md; GET /api/skills; compozy skill list|inspect|view -o json; compozy__skill_list|view; Web /skills
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-001; ET-002; ET-003; ET-skill-activation-gates
---

QA impact 2026-07-26: the official bundled skill, loader namespace, and every structured
management surface received one hard cut. Planning flag only; the next QA cycle owns execution.

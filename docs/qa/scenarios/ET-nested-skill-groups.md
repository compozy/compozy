---
id: ET-nested-skill-groups
area: ET
title: Discover grouped workspace skills without cross-workspace leakage
persona: Ada
journey: J-offer-runnable-capabilities
expected: An empty organizational group is ignored; nested `SKILL.md` leaves appear in the effective workspace catalog and management surfaces, a live nested add/edit/remove refreshes without daemon restart, and two workspaces expose only their own grouped skills. `compozy skill create <name> --group <relative/path>` scaffolds the nested leaf without changing its frontmatter identity or normal shadow precedence.
entry_points: `.compozy/skills/**/SKILL.md`; `compozy skill create --group`; `compozy skill list|view|where`; `GET /api/skills`; `compozy__skill_list|view`; workspace skill watcher
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-001; ET-003; ET-004; ET-skill-activation-gates
---

Create an empty `.compozy/skills/marketing/` group and nested leaves such as
`marketing/campaign-brief/SKILL.md` and `engineering/reviews/api-review/SKILL.md`. Confirm the
groups themselves never appear as skills and both leaves resolve by their frontmatter names across
CLI, HTTP/UDS, and native skill reads. Add, edit, and remove one nested leaf while the daemon stays
running; the next workspace resolution must show the current catalog.

Run the same fixture in two isolated workspaces with distinct grouped leaves. Each workspace may
read only its own leaf through list, view, prompt catalog, and resource publication. Include a
same-name declaration in different groups to prove ordinary source precedence and `skill where`
shadow diagnostics remain authoritative.

QA impact 2026-07-28: grouped skill discovery and `skill create --group` are new user-visible
runtime and CLI behavior. Planning flag only; no QA session ran in this implementation slice.

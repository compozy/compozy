---
id: ET-spec-cycle-skill-bundle
area: ET
title: Use the bundled spec-cycle skills in a managed session
persona: Ada
journey: J-offer-runnable-capabilities
expected: A managed session lists and prompts with exactly the nine spec-cycle workflow skills, each view returns its bundled body, and a workspace-local override wins only in its owning workspace while another workspace keeps the global bundled declaration.
entry_points: compozy extension list; compozy skill list|view; compozy__skill_list|view; managed session prompt
qa_status: skipped
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status: pass
fix_commits: e4df8634
evidence: /Users/pedronauck/dev/qa-labs/compozy-spec-unification-skill-bundle-20260814-022518-316742-lab/qa-artifacts/qa/skill-list.json; /Users/pedronauck/dev/qa-labs/compozy-spec-unification-skill-bundle-20260814-022518-316742-lab/qa-artifacts/qa/skill-view-cy-create-spec.txt; /Users/pedronauck/dev/qa-labs/compozy-spec-unification-skill-bundle-20260814-022518-316742-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-003; ET-004; ET-skill-activation-gates
---

QA impact 2026-07-27: the bundled spec-cycle extension now publishes its nine workflow skills
globally to Compozy-managed sessions while preserving workspace-local lookup, cache, and prompt
isolation. Planning flag only; the next QA cycle owns execution.

QA impact 2026-08-13: the spec-unification hard cut replaced cy-create-prd + cy-create-techspec
with the single cy-create-spec skill, shrinking the published bundle from nine skills to eight.
Walked 2026-08-13 in an isolated targeted lab (fresh binary, isolated COMPOZY_HOME): `compozy skill list -o json` returned exactly cy-create-spec, cy-create-tasks, cy-execute-task, cy-final-verify, cy-fix-reviews, cy-review-round, cy-workflow-memory, git-rebase (plus the separate official compozy skill); `skill view cy-create-spec` rendered the bundled body; `skill view cy-create-prd` failed deterministically with `skill not found`. Verdict: pass. Teardown clean.

QA impact 2026-08-14: the bundled `orchestrate-tasks` Loop adds `cy-orchestrate-tasks`, taking the
published bundle from eight skills to nine. Reset to untested — the next walk must see exactly
cy-create-spec, cy-create-tasks, cy-execute-task, cy-final-verify, cy-fix-reviews,
cy-orchestrate-tasks, cy-review-round, cy-workflow-memory, git-rebase. Planning flag only; execution
belongs to the QA phase of the owning workstream.

QA impact 2026-09-29 (#663): `cy-create-tasks` writes double-quoted, escaped YAML
titles matching the complete first H1 and validates generated task sets through
`ext__spec_cycle__import_tasks` before delivery. Check a title containing `: `,
embedded double quotes, backslashes, `#`, and YAML indicators: import must preserve
the title text. The original unquoted colon title must still fail with the task
filename in the validation error. Walked 2026-09-29: a task instantiated from the corrected bundled template, with
the metacharacter title above, imported through the actual `RunProvider` subprocess
RPC path and preserved its decoded title/H1. Focused importer/RPC evidence is recorded in
`/tmp/compozy-issues-20260929/task-title-report.md`; provider-driven skill authoring
remains a separate QA walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

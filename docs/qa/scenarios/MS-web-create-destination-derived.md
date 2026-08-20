---
id: MS-web-create-destination-derived
area: MS
title: Create and install surfaces inherit menubar destination
persona: Dora
journey: J-31
expected: Agent, session, task, job, trigger, bridge, MCP install, knowledge create, and task-bridge subscribe surfaces show a `workspace-scope-statement` footer note derived from the menubar Global switch. There are no destination pills, RadioCards, or `config_scope` search params. Global create omits `workspace` (sessions bind the hidden home id or `workspace_path` without flipping the menubar). Workspace create sends the project id. Knowledge list tabs stay filters; create follows the menubar unless the Agent tab is selected. Settings → Skills Global|Agent is a different axis and stays local.
entry_points: web New agent; New session; New task; New job/trigger; New bridge; Marketplace MCP install; Knowledge create
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/knowledge-global-clean.png; /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/mcp-global-install-clean.png
last_report: docs/qa/reports/2026-08-13-pr-368-coderabbit.md
overlaps: MS-web-agent-create-simple-advanced; MS-web-entity-modal-shell; ET-web-marketplace-mcp-authorize-installed
---

story: As a person running agent work I never pick Global vs a folder inside a create dialog — the menubar already decided, and the dialog only names that destination.

Introduced 2026-08-12. `ScopeSelector` and create/install destination pickers were deleted.

src: web/src/systems/workspace/components/workspace-scope-statement.tsx; web/src/systems/workspace/hooks/use-create-destination.ts; web/src/systems/session/lib/session-create-binding.ts

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13 re-walk: a project-scoped Knowledge draft named `draft-project` was abandoned. After switching Global, the reopened form was clean and stated "Creates in Global — visible to every workspace." The Global MCP install dialog likewise started clean and stated its Global destination. Remaining submission paths stay blocked-verify for a broader creation charter.

2026-08-20 qa-impact: job, trigger, and task destination chips left the body toolbar; every create/install surface now states destination in the footer hint. Reset to untested.

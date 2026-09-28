---
id: MS-web-agent-create-simple-advanced
area: MS
title: Create agent replaces the four-step wizard with one Simple/Advanced surface
persona: Dora
journey: J-31
expected: Opening Create agent shows one surface, never a stepper — no "Step N of 4", no Back/Continue. Simple carries the definition (agent name, instructions) and, side by side, the runtime selector with live catalog state and the category path — the two decisions a person makes while naming the agent. Advanced adds the permission policy cards, launch overrides (runtime command), and the tool/skill allowlists, without hiding any Simple field. Explanatory prose sits behind `(?)` help tips; catalog state ("Project runtime defaults will be used.", catalog errors) stays visible because it reports what the daemon will do. An agent name such as `audio designer` remains visible but fails Simple with an inline format error and a disabled Create button; no create request is sent. An invalid category path such as `operations//incident` now fails Simple and shows its error in place, without switching tiers; submitting with an invalid advanced-only field (a blank tool entry) still reveals Advanced instead of leaving a disabled primary with no visible cause. Submit errors render as a danger Alert at the top of the body. Leaving Advanced preserves every authored value. There is no MCP servers control anywhere in the dialog, and the created request never carries `mcp_servers`. The category path is sent as `category_path` segments split on `/`. Destination is derived from the menubar Global switch: the footer shows a `workspace-scope-statement` note ("Creates in Global — visible to every workspace." or "Creates in {workspace}."), never scope pills; Global omits `workspace` on the request.
entry_points: web desktop shell → New agent (menubar, command palette, agent catalog, agent detail duplicate)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits: 7e5d6b8
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-01; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-02; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-03; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-11; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/040-agent-create-authored-files; /Users/pedronauck/dev/qa-labs/compozy-pr-368-global-runtime-binding-20260813-091321-434791-lab/qa-artifacts/qa/notes/global-agent-session-e2e.md; docs/qa/reports/2026-08-13-pr-367-agent-name-validation-rewalk.md; /Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-agent-create-advanced-runtime.png
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: MS-web-entity-modal-shell
---

Open New agent and validate name, category, runtime and permission fields in both disclosure modes. A blank advanced tool entry reveals its field error without discarding other draft values. Names exceeding the supported length fail before dispatch. Create a valid agent, reload the catalog, and confirm category, destination, runtime and permission defaults round-trip through HTTP and UDS without mcp_servers.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

---
id: MS-web-agent-create-simple-advanced
area: MS
title: Create agent replaces the four-step wizard with one Simple/Advanced surface
persona: Dora
journey:
expected: Opening Create agent shows one surface, never a stepper — no "Step N of 4", no Back/Continue. Simple carries the definition (agent name, instructions) and, side by side, the runtime selector with live catalog state and the category path — the two decisions a person makes while naming the agent. Advanced adds the permission policy cards, launch overrides (runtime command), and the tool/skill allowlists, without hiding any Simple field. Explanatory prose sits behind `(?)` help tips; catalog state ("Project runtime defaults will be used.", catalog errors) stays visible because it reports what the daemon will do. An invalid category path such as `operations//incident` now fails Simple and shows its error in place, without switching tiers; submitting with an invalid advanced-only field (a blank tool entry) still reveals Advanced instead of leaving a disabled primary with no visible cause. Submit errors render as a danger Alert at the top of the body. Leaving Advanced preserves every authored value. There is no MCP servers control anywhere in the dialog, and the created request never carries `mcp_servers`. The category path is sent as `category_path` segments split on `/`. Scope sits in the toolbar as a borderless picker level with the scope pills, and reports the workspace the definition lands in; global scope omits `workspace`.
entry_points: web desktop shell → New agent (menubar, command palette, agent catalog, agent detail duplicate)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-01; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-02; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-03; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-11
last_report:
overlaps: MS-web-entity-modal-shell
---

story: As an operator I define an agent on one screen and only open Advanced when this definition needs to differ from the runtime defaults.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.1), task_03, implemented 2026-07-25. Before this change the dialog was a four-step wizard whose per-step gate could strand the operator on a step with no visible cause.

The MCP-servers control the artboard drew is cut permanently (T1/D5): `CreateAgentPayload` has no `mcp_servers` field and `AgentPayload.MCPServers` is populated on the response path only, so authoring it would need a contract change first.

The permission policy renders four cards, not the artboard's three, because `permissions` is optional on the create payload — "omit the field and follow the runtime" is a real, selectable outcome.

src: web/src/systems/agent/components/agent-create-dialog.tsx; web/src/systems/agent/components/agent-create-definition-section.tsx; web/src/systems/agent/components/agent-create-runtime-section.tsx; web/src/systems/agent/components/agent-create-permissions-section.tsx; web/src/systems/agent/components/agent-create-runtime-details-section.tsx; web/src/systems/agent/components/agent-create-tools-section.tsx; web/src/systems/agent/lib/agent-create-draft.ts

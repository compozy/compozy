---
id: MS-web-agent-create-simple-advanced
area: MS
title: New agent is one screen, with Advanced as one fold
persona: Dora
journey: J-31
expected: Opening New agent shows one screen on the `xl` dialog, never a stepper or Simple/Advanced tabs. The left column holds the avatar (Character, Icon or emoji with a Filled / No background choice that keeps the chosen color as the symbol's ink, Shuffle), Name, What it does, Instructions in `MarkdownEditor` with Watch issues, Review code and Fill with AI, then one Advanced fold. The side panel holds How it works (On-demand, Always-on), Where it works with search, Global and Decide later (Always-on only), Model with live catalog state, and a live preview. Typing a Name derives the Short name, editable under Advanced. A taken name shows before submit with a suggestion. A name with no valid characters opens Advanced. Closing with entered data asks to discard. Advanced holds the Short name, the Organize Group field, the permission cards, custom command and "What this agent is allowed to do"; an invalid value there opens the fold with the error in place, and a disabled primary never hides its cause. Submit errors render as a danger Alert. There is no MCP servers control, and the request never carries `mcp_servers`. An On-demand agent follows the menubar destination; an always-on agent is defined at the user level so any project can start it. Always-on with a place creates the agent, starts its first instance and returns to Sessions. Decide later, the instance limit and a provider that isn't signed in create it unstarted, each said before submitting. A failed start keeps the agent, reports it on the screen and offers Open agent.
entry_points: web desktop shell → New agent (menubar, command palette, agent catalog, agent detail duplicate)
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits: 7e5d6b8
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-01; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-02; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-03; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-11; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/040-agent-create-authored-files; /Users/pedronauck/dev/qa-labs/compozy-pr-368-global-runtime-binding-20260813-091321-434791-lab/qa-artifacts/qa/notes/global-agent-session-e2e.md; docs/qa/reports/2026-08-13-pr-367-agent-name-validation-rewalk.md; /Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-agent-create-advanced-runtime.png; docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/ (2026-11-03 always-on QA walk)
last_report: docs/qa/reports/2026-11-03-vocabulary-alignment-qa.md
overlaps: MS-web-entity-modal-shell
---

Open New agent and validate name, category, runtime and permission fields in both disclosure modes. A blank advanced tool entry reveals its field error without discarding other draft values. Names exceeding the supported length fail before dispatch. Create a valid agent, reload the catalog, and confirm category, destination, runtime and permission defaults round-trip through HTTP and UDS without mcp_servers.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

QA impact 2026-09-28 (ui-normie-pass): the Group (category path) field moved from Simple into an
Advanced "Organize" section, the header reads eyebrow "Agents" with a plain description, permission
cards lost their posture badges and share one vocabulary with agent settings, and the access lists use
plain labels with raw IDs only as help-tip examples. Not walked.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-10-27 (session responsiveness): create and duplicate actions now capture
their stable mutation functions, and model-catalog collections are retained until
their provider/catalog inputs change. Open the dialog after unrelated desktop
updates and verify current provider permissions, model/reasoning selection, creation,
duplication and persisted runtime values. The existing agent browser lifecycle in
`web/e2e/__tests__/agents.spec.ts` owns the real daemon/browser slice; its current-head
CI result is recorded in PR #983. Focused catalog/agent/shell-model suites pass 28
tests. This does not claim a re-walk of every Simple/Advanced control above.

QA impact 2026-11-01 (always-on F1): the Simple/Advanced tabs are gone; `expected` above describes the
one screen. Fill with AI and starting an instance need the daemon routes from the always-on backend
slices, so this slice verified them against typed mocks only. Focused suites own the screen's states;
a real walk is required after those slices land. Not walked.

### Missing default provider

With no configured default provider, leave Model unchanged and submit an otherwise valid agent.
The error explains that no default provider is set and asks you to choose a provider in Model.
Select a configured provider and retry; creation succeeds without re-entering the agent details.
The inherited-runtime help must describe a configured default conditionally.

B5 focused evidence: agent API adapter and create-screen suites passed (53 tests). The live
missing-default and recovery journey remains pending.

B5 isolated re-walk passed: absent defaults.provider produced the actionable copy, selecting Codex
GPT-6.1-Sol created the agent with the original name, description and instructions, and the public
API confirmed provider/model. Evidence: `reports/2026-11-01-stays-on-b5-remediation.md`.

QA impact 2026-11-03 (always-on hardening S5): Advanced's permission cards are one column of the five
shared levels. An On-demand agent adds "Follow Settings" (omits `permissions`); an always-on agent with
nothing chosen shows "Accept edits" marked Default, and the request still omits `permissions`.
The consequence Alert under the cards is gone; each card's description carries it. Not walked.

QA 2026-11-03 (always-on hardening, soh/qa 583415ef5): Not walked in this pass. The shared five-level cards were exercised on the Permissions page instead (see `MS-web-agent-permissions`).

QA impact 2026-11-03 (#1199 vocabulary, v0.5.1): the kind choice reads On-demand and Always-on, and Advanced's permission cards read Accept edits, Auto-review and Full access; an always-on agent with nothing chosen shows Accept edits marked Default and the request still omits `permissions`. Not walked; S4 walks it on the integration head.

Runtime scope regression: choose an explicit model on On-demand in a project, then switch to
Always-on and Decide later. When that provider exists globally, keep the model and reasoning
selection and create the global definition without starting an instance. If the provider is absent
globally, clear the runtime overrides and require a valid destination selection. The owning hook
suite covers both cases; `agents.spec.ts` always-on vocabulary journeys own the real browser walk.

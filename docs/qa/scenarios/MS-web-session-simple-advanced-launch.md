---
id: MS-web-session-simple-advanced-launch
area: MS
title: Start session separates launch details from the prompt composer
persona: Dora
journey: J-17
expected: Opening Start session shows agent selection, with workspace, optional name, and Network participation in Advanced; it contains neither a first-message composer nor a runtime selector. Launch creates one durable session at the selected workspace root, activates its returned owner workspace, and navigates to its composer. Choosing another workspace clears only workspace-scoped launch selections. The session composer owns the "Next prompt" RuntimeSelector and its catalog state; the header carries the only close control.
entry_points: web desktop shell → Start session (dock, command palette, agent catalog, agent detail, dashboard)
qa_status: untested
bug_ids: BUG-20260730-session-create-window-intent
fix_status: fixed
retest_status: pass
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-01; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-02; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-09;/Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-logs/qa;docs/qa/evidence/2026-07-30-session-runtime-selector/01-create-simple.png;docs/qa/evidence/2026-07-30-session-runtime-selector/02-create-advanced.png;docs/qa/evidence/2026-07-30-session-runtime-selector/04-session-open-after-create.png
last_report: docs/qa/reports/2026-07-30-session-runtime-selector.md
overlaps: MS-web-entity-modal-shell; NB-participation-controls-serialize
---

story: As a person running agent work I choose who runs and where, write the first message, and send one durable session-create request without losing access to runtime or advanced launch controls.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.2), task_02, implemented 2026-07-25. Before this change the dialog was runtime-first: it had no workspace picker and no session name, and it always sent the active workspace.

The workspace picker and session name are new writes against `CreateSessionRequest` fields the daemon already accepted (`internal/api/contract/session_runtime_payloads.go`). Network participation keeps its existing control (mode, channel id, strategy) rather than the single channel select in the artboard, because `network_participation` is the only channel-bearing field on the contract.

QA impact 2026-07-26: first-message creation is atomic and RuntimeSelector lives in the composer. The modal shell keeps Simple/Advanced disclosure, while Network participation remains hidden in Advanced. Scenario remains untested; flag only.

QA impact 2026-07-31: Working path removed from Start session Advanced; create always targets the selected workspace root. Status reset to untested.

src: web/src/systems/session/components/session-create-dialog.tsx; web/src/systems/session/components/session-create-simple-section.tsx; web/src/systems/session/components/session-create-advanced-section.tsx; web/src/systems/session/hooks/use-session-create-dialog.ts

inventory: Needs QA

QA impact 2026-07-26: opening the create flow without an explicit agent now resolves against the
live Query-backed agent catalog, including the first available agent after an initially empty read.
Status remains untested; no QA replay ran.

2026-08-20 qa-impact: density cleanup removed the Agent and Session name helper paragraphs. Status
lines while creating an environment or starting the session stay visible. Status remains untested.

2026-08-20 qa-impact: Simple/Advanced sits on a recessed `--color-canvas-tint` chrome strip against the `--color-canvas-soft` shell. Status remains untested.

2026-08-27 qa-impact: the destination composer Runtime Selector now includes catalog-backed Fast,
advanced ACP options, and provider-managed state. Status remains untested for the launch-to-composer
handoff.

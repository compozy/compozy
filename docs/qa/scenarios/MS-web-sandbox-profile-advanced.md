---
id: MS-web-sandbox-profile-advanced
area: MS
title: Sandbox profile editor exposes lifecycle, network, and Daytona behind Advanced
persona: Dora
journey:
expected: Opening a sandbox profile shows Simple only — the profile name, the execution backend as two cards (Local, Daytona), and the workspace sync mode. Advanced adds isolation and lifecycle (persistence, runtime root, environment, secret environment), network policy (allow outbound, allow public ingress, allow list, deny list), and — only while Daytona is the selected backend — the cloud workspace parameters. Switching Daytona to Local removes the Daytona block and drops those parameters from the replace body instead of persisting a cloud configuration for a profile that no longer runs there. On edit the profile name renders as readable locked identity. `secret_env` accepts only references — `env:NAME` or `vault:sandbox/<path>` — and a literal value is rejected with a field error that also blocks Save, including from Simple where the offending row is not visible. Every profile key this editor does not model is round-tripped untouched, because the sandbox PUT is a full replacement with no preservation channel. The inspect sheet never writes; it only launches this editor.
entry_points: web desktop shell → Sandbox → New sandbox profile / profile row → Edit, or profile sheet → Edit profile
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-09; .compozy/tasks/modals-redesign/evidence/visual/task_03/VC-10
last_report:
overlaps: MS-web-entity-modal-shell
---

story: As an operator I decide where agent sessions execute and what they may reach, without dropping to `agh config set` for the nested fields.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.13–4.14), task_03, implemented 2026-07-25. Before this change `env`, `secret_env`, `network`, and `daytona` were read-only: the dialog carried them through the PUT untouched and the inspect sheet told the operator to edit them on the CLI.

`secret_env` holds references, not secrets — `vault.ValidateSecretEnvMap` rejects anything that is not `env:NAME` or a namespaced `vault:sandbox/...` ref — so the references are editable and displayable, and the editor enforces the same rule before the request is built.

src: web/src/systems/sandbox/components/sandbox-editor.tsx; web/src/systems/sandbox/components/sandbox-editor-simple-section.tsx; web/src/systems/sandbox/components/sandbox-editor-advanced-section.tsx; web/src/systems/sandbox/lib/sandbox-profile-draft.ts; web/src/systems/sandbox/components/sandbox-profile-sheet.tsx

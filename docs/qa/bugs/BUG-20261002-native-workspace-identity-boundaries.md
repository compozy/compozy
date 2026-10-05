# BUG-20261002-native-workspace-identity-boundaries: Native aliases cross incompatible workspace identities

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, read the current project and recover its scheduled work
- **Scenarios:** ET-native-workspace-scope-isolation
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction and evidence

In a real hosted `approve-reads` session, read Studio memory using Studio's registered directory.
The runtime asks for access to the current project's durable identity even though the session
already belongs to that project. Rejecting this unexpected request produces `workspace_access_denied`.
The same agent can read an omitted-workspace automation list without a prompt.

An `approve-all` session creates disabled job `job-c586d38568678a3f` with omitted workspace and
reads it back successfully. An independent scoped operator invocation of `automation_jobs_get`
for that same job returns `tool_not_found`. The agent-created reference memory is independently
readable and Editorial's memory list remains empty.

Evidence: `native-hook-reviewer-attention.json`, `native-hook-late-history-*`,
`native-hook-summary-*`, `native-hook-job-readback.json`, `native-hook-memory-readback.json`,
`native-hook-editorial-memory-after.json`, and `native-hook-walk-ended.json` under
`docs/qa/evidence/2026-10-02-untested/`. Both sessions ended before investigation.

## Root cause and repair plan

Explicit native workspace references normalize to `ResolvedWorkspace.WorkspaceID`, the durable
metadata identity. Session authority and automation persistence use the registration key
`ResolvedWorkspace.ID`. Authorization compares these unequal representations, while automation
handlers directly filter by the normalized durable key. Omitted agent input keeps the trusted
registration key, explaining the inconsistent behavior.

Normalize native input to the registration key at the shared binder, before policy, approval and
handler execution. Both durable and registration IDs remain accepted resolver inputs. Memory and
configuration handlers still resolve their owning domain identity as before. The earlier task-claim
repair in BUG-20260910-task-claim-workspace-identity remains valid. No persisted shape or public
selector changes are needed; automation foreign keys already reference the registration table.

The old binder unit assertion expected a durable ID in the intermediate JSON, without exercising
policy or a persisted handler. That expectation contradicted the live session authority and the
automation foreign-key contract. It now expects the registered target, while new boot tests exercise
actual alias authorization and automation tests assert create/list/get behavior. Existing denial,
approval and task-claim assertions are retained. The approval digest fixture follows the corrected
canonical dispatch input; supplied raw-input digest validation remains covered.

Invariant: registered ID, durable ID, name, path and omitted own workspace must address the same
authorized project. The owning layer is daemon native dispatch. Existing suites:
`TestDaemonBootToolRegistry` and `TestDaemonNativeAutomationTools` (plus its integration lifecycle
suite for persistence). Fresh boot regressions for agent name/path fail before production edits
with the same cross-workspace denial: `native-identity-red.log`.
Automation selector regressions fail before the correction for all five selector forms in
`native-identity-automation-red.log`.

- **Fix commit:** 563990440
- **Retest:** passed for the identity repair; the separate hook-dispatch finding remains open.

## Verified replay

After rebuilding and restarting the isolated daemon, the previously created disabled job is
retrievable by registration ID, durable ID, name and path. The direct automation CLI independently
lists and reads that persisted job in Studio, still disabled. A new job created using the registered
name persists under Studio's registration key, is read through the direct CLI, then deleted and
confirmed absent.

Fresh hosted approve-reads session `sess-266c2ba8e427425d` reads Studio's workspace description,
reference memory, job list and job detail using the registered name with no own-workspace approval.
Its two Editorial read requests prompt with Editorial's registration ID, are rejected, and return
`workspace_access_denied`. The agent reports the incomplete handoff without writing; an independent
Editorial memory list stays empty. The session is stopped and confirmed through HTTP before closure.

Evidence: `native-identity-replay-*`, `native-identity-boundary-green.log` (race-enabled native,
boot and automation suites) and `native-identity-automation-integration.log` (real persistence).
The native test-shape checker is clean. The automation file retains its baseline heuristic finding
for the untouched `TestDaemonNativeAutomationToolsShouldForwardProfileScope`; the same output is
recorded for HEAD before these changes. The added cases use parallel `Should` subtests.

`make gate` passed all affected lanes with zero lint issues; receipt:
`native-identity-gate-retry.log`. The initial formatting-only gate failure was corrected before this pass.

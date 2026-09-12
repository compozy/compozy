# BUG-20260910-task-claim-workspace-identity: Native archive claims miss queued work and denied claims omit guidance

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-cross-workspace-access, claim one exact queued archive task
- **Scenarios:** ET-workspace-access-mode-matrix
- **Found:** 2026-09-10 (local time) · **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Real reproduction and chronology

Operator created task `task-c365b5cc279933d5` in field-archive registration `ws_6948d9a869661f6c` and queued `run-5cf0824217f60483` at00:11:10UTC. Approve-all Cursor session `sess-e4855b551031a4be` called native task_run_claim_next once at00:11:57 with that run ID and explicit foreign registration ID. Both tool/task authorization allowed the crossing, but the result was claimed=false/no claimable task runs. Public task inspect at00:13:11 still showed that exact run queued. A different scheduler worker claimed it only at00:14:28 and completed it at00:14:44. Therefore this failure precedes background ownership; it is not the earlier preparation race.

Independently, approve-reads session `sess-04e085f6d8b83faf` ran the agent CLI task-next path once through the managed terminal. It exited77 with `task: permission denied: claim workspace does not match trusted caller`, omitting the documented canonical workspace permission-mode hint. Other CLI spawn/coordination/peers denials include the hint.

Evidence in `docs/qa/evidence/2026-09-10-qa-execution-unblock/`: cross-claim-task-start.json, cross-claim-events.json, cross-claim-grants.json, cross-claim-inspect.json, cross-claim-task-current.json, cross-read-retry-events.json. Parent turns settled before source diagnosis; all caller stops were requested through the public CLI.

## Cause and bounded fix

The native workspace input binder normalizes explicit aliases to `ResolvedWorkspace.WorkspaceID` (stable workspace metadata identity). Task persistence and claim selection use the registration ID (`ResolvedWorkspace.ID`). autonomyClaimNext previously passed the normalized alias directly into claim criteria, silently querying a different workspace key. Resolve non-bound claim targets back to the registration ID before invoking the task manager; keep caller scope and both authorization phases intact. Implicit trusted caller IDs remain unchanged.

The task normalization denial branch also discarded the canonical workspace guidance. Append workspaceaccess.DenialHint while retaining ErrPermissionDenied and existing CLI/transport error classification.

Governor: two cohesive production files, no schema/data migration, no new API field, no permission change, and no product trade-off. Canonical native-boundary tests cover registration/stable/name/path targets and same-workspace stable aliases; the existing task claim normalization suite covers denial guidance. Both reproduced red before production edits. Live retest and the required gate passed; details below.

## Cross-surface impact

- Native tools: compozy__task_run_claim_next resolves the same task scope as CLI/HTTP/UDS. IDs, descriptor schemas, result shapes and permissions remain unchanged. Same-workspace and foreign aliases share the registration lookup boundary.
- Extensibility/hooks/config: no new setting, hook, SDK or provider requirement. Existing task authorization/audit and claim/start lifecycle remain authoritative.
- Isolation: target claim selection uses the registered workspace, while actor session/workspace/profile stay unchanged. No blanket operator authority or consent is introduced; denied claims still fail.
- Official skill: tasks-and-orchestration reference clarifies alias parity and actionable denied-claim guidance.
- Web/Docs: no Web component or route change; queued task state now reflects native access to the correct target. ET-workspace-access-mode-matrix owns real replay. Existing published task/permission contract is restored, with no new site API documentation required.
- Compatibility: no persisted shape or public schema changes. Existing error classification is retained with the already-promised guidance appended.

## Real retest and verification

With the rebuilt daemon, Cursor Grok4.6/high/fast session `sess-8618b40841ab2348` claimed exact `run-2f5d3acf89ca3289` using foreign name field-archive, wrote the requested one-line file, and completed the run through native task_run_complete. Public task inspect confirms completed state and the original caller binding; independent file read matches the exact content. Tool and task grant audits preserve the caller workspace and identify the intended target.

Fresh approve-reads session `sess-ce8de99a8a03aef3` executed the same CLI task-next command once. It exited77 with the complete canonical permission-mode hint. Independent journal command `cmd-1bbbb51270353af1bb8759c40c27a354` and workspace denial audit agree. Only ordinary MCP/tool allow-once approvals were granted; no workspace consent or permission mode changed. Both sessions were stopped through the public CLI. `claim-retest-proof.json` links the independent evidence.

Canonical native-boundary and task normalization regressions reproduced red, then passed with race detection; the full task race suite passed. Go build and make gate passed (all affected Go and Web lanes). The first gate caught formatter drift, corrected with the owning formatter before the successful retry. The standalone convention heuristic flags seven pre-existing untouched top-level tests in lease_test.go; the added subtest follows its owning suite and the native file check passed. No unrelated test restructuring occurred. Local fix commit is recorded in the run progress after commit.

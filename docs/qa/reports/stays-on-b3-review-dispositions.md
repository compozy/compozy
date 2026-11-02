# Stays-on B3 review dispositions

These decisions follow the remediation brief's conservative-reading rule for
ambiguous contracts. The track audit below maps every remediation finding to its
commit or justified disposition; final verification status is recorded at the end.

## N-38: retain daemon attribution for instance-owned main sessions

Disposition: no code change; the review identifies ambiguity, not a demonstrated
contract violation. The instance creator and the session initiator are distinct.

Evidence:

- `internal/agentinstance/instance.go` stores `CreatedByKind` and `CreatedByID` on
  the instance, preserving the requesting actor independently of session startup.
- `internal/daemon/agent_instance_main.go` starts the managed main with origin
  `daemon` and the instance ID. `agent_instance_rotation_start.go` uses the same
  origin for successors. This describes who launches the managed session.
- `internal/session/manager_create_prepare.go` derives root-session initiator from
  the authenticated creation origin; child sessions inherit the root initiator.
- `_spec.md` Initiator attribution defines daemon as a valid initiator and requires
  root attribution to be inherited. It does not require copying an instance's
  creator into its managed main session. Existing manager-start tests cover origin
  persistence and child inheritance.

Keep the daemon origin and the instance creator record. Replacing the origin with
an operator or agent would also affect workspace-access authority, not merely the
observability label. No authority change is justified by this ambiguous finding.
No new behavior test is needed for this documented no-change disposition.

## N-43: retain the relay's workspace and profile boundary

Disposition: retain the existing boundary under the conservative-reading rule.
The review explicitly says the exclusion may be intended. This is a documented
scope limitation: descendant relay currently covers the root's workspace/profile,
not every reachable workspace in the lineage.

Evidence:

- `internal/api/core/session_attention.go:authorizeSessionInteractionDiscovery`
  authorizes agent credentials against `workspaceRefFromRoute(c)` only.
- `ListSessionInteractions` checks the root with
  `RequireRouteSessionInWorkspace`, then returns the descendant reader's rows
  without authorizing each descendant workspace.
- `internal/store/globaldb/global_db_session_pending.go:listPendingInteractions`
  restricts recursive traversal to the root's workspace and profile. This is the
  boundary that prevents the authorized root read from disclosing another scope.
- The spec's relay sentence says root descendants, but does not define a broader
  authorization envelope or authorize exposing inaccessible child workspaces.
  Its authority/safety rules preserve workspace ceilings.

Do not remove the SQL restrictions in isolation. A broader relay would require
per-descendant workspace authorization across the shared API and native surfaces,
with filtering and traversal semantics defined together. Keep the current scoped
read for this ambiguity; answers continue to target original session/request IDs.
No production behavior changed and no additional behavior test was introduced.

## N-35: document the transcript-only rotation marker

Disposition: retain and document, as explicitly allowed by the B3 remediation
brief. The canonical `_dx.md` now defines the marker separately from persisted
report kinds. The relevant contract is retained here because the spec is ignored.

Rotation appends a `session.report` event with `kind: rotated`,
`report_id: rotation:<successor-session-id>` and a payload containing the
predecessor's `session_id`. This is a successor transcript marker, not a row in
`agent_instance_reports`. It has no report delivery state, is not injected into
prompts through the report outbox, and generates no operator notification.

`instanceMainSessions.StartSuccessor` appends the marker while preparing the
candidate, before the main-session pointer compare-and-set. A publication error
therefore follows the spec's pre-publication failure rule: stop the candidate,
keep the predecessor authoritative and report `rotation_failed`. The pointer,
not the marker, determines supersession. A marker in a discarded candidate does
not establish that rotation committed.

The existing `TestInstanceRotationFlow` covers candidate-start failure and pointer
preservation; `SessionReportPart` has an existing predecessor-link case for
`rotated`. No runtime behavior or enum changed in this documentation decision.

## Track B3 audit (2026-11-01)

Compared `git log feat/stays-on-agents..HEAD` on `feat/stays-on-fix-b3`
against the remediation track:

| Finding | Commit | Disposition and evidence |
| --- | --- | --- |
| D-17 | 0d705be22 | Fixed: native approval deadline emits timeout and enters expiry reporting. |
| D-19 | 5345e4042 | Fixed: indeterminate admissions no longer block rotation; retries have a cadence fence and predecessor report key. |
| D-20 | 6f99d6214 | Fixed: provider context refusal writes one failure report per instance turn. |
| D-21 | e9f9d4451 | Fixed: check supersession before resuming a stopped predecessor, returning the current main. |
| N-17 | 27499e20b | Fixed: publish catalog invalidation after pointer swap and before predecessor cleanup. |
| N-18 | 20f1e9b28 | Fixed: retain provider-versus-timeout expiry attribution in transactional reports. |
| N-34 | 1706de8ca | Fixed: bounded terminal-report retention preserves pending and reserved reports. |
| N-35 | f77b473d3 | Documented: transcript-only rotation marker, per the explicit remediation option and disposition above. |
| N-36 | 7500ca11f | Fixed: successor republication does not notify the operator twice. |
| N-38 | 2884ad0e8 | Retained: daemon launches the managed main; instance creator attribution remains separate, as traced above. |
| N-39 | bf6273b69 + c179b8953 | Fixed: isolate per-session failures; persist acknowledgement after the durable note and exclude acknowledged reports. |
| N-40 | c927d9877 | Fixed: expiry removes actionable approval before terminal emission and handles a missing reject option. |
| N-41 | dc0508026 | Fixed: distinguish boot interruption from resume interruption and retain tool identity. |
| N-42 | 7732f6f72 | Fixed: instance maintenance has an owned worker independent of session supervision. |
| N-43 | 1771b3d39 | Retained: relay preserves workspace/profile authorization boundary, as traced above. |

N-39's remaining historical rescan is implemented and committed in `c179b8953`. Recovery acknowledges each report only after its idempotent session
event append succeeds. The global catalog persists that acknowledgement and uses
a partial index to scan only unnoted indeterminate reports. Failures stay eligible
on the next pass; a failed acknowledgement retries the same durable event ID.
Migration 00171 preserves existing report rows and defaults them to unnoted.

Test invariants and owners: `TestInstanceReportsReachProvider` in session owns
retry after failed acknowledgement, once-only notes and exclusion on subsequent
recovery. `TestInstanceReports` in globaldb owns migration row preservation and
acknowledgement persistence across reopen. Existing store migration suites own
stream integrity, fresh/repeated apply and schema equivalence.

Impact follows the approved `_impact-entry.md`: no native tool, HTTP/CLI, hook,
extension, config or official skill contract changes. Reads remain internal to
the same report/session linkage. Web transcript semantics stay unchanged; the
existing reports QA scenario records the recovery change.

Validation so far: `make codegen` and `make codegen-check` passed. The focused
`TestInstanceReportsReachProvider` and `TestInstanceReports` race runs passed after
explicitly seeding the crash-before-acknowledgement state and using a cleanup
context that survives test cancellation. Test-shape heuristics passed.

Package lint reports 261 inherited diagnostics. No production change is made to
unrelated findings; diff-scoped lint passed with zero issues using
`golangci-lint v2.13.1 run --new-from-rev=HEAD` on session and globaldb.
`go build ./...` and the canonical migration race checks passed (exit 0).
The complete session race suite passed with `TMPDIR=/private/tmp`:
`go test -race ./internal/session -count=1` (395.699s, exit 0).
Canonical TMPDIR preserves the tests' path assertions on macOS, where `/var`
resolves to `/private/var`; no assertion or production behavior was changed.

The first full globaldb race run hit Go's default 10-minute package timeout.
The repository unit lane sets 30m and parallelism 4. The full globaldb suite is
rerunning with those limits and the exact repository safety flags
(`-race -parallel=4 -timeout=30m -gcflags=modernc.org/...=-d=checkptr=0`),
with COMPOZY_* unset and
`TMPDIR=/private/tmp`, under monitor `mon-1e223ac9d045` (terminal
`term-b8fb41689e4c`). This is the only remaining validation; no full globaldb
pass is claimed while the monitor is active.
Global gates and E2E remain excluded by the remediation brief. No runtime/browser
QA claim is made.

The intermediate globaldb run used the narrower modernc.org/libc checkptr
pattern and was interrupted before completion. The monitor above supersedes it;
no successful result is claimed for that interrupted invocation.

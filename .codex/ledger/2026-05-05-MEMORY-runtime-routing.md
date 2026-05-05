Goal (incl. success criteria):

- Complete network-threads task_06 in /Users/pedronauck/Dev/compozy/agh2: wire store-backed conversation state into runtime routing, delivery wrappers, structured prompt metadata, and network task ingress; add required tests; run clean verification; update tracking/memory; create one local commit.

Constraints/Assumptions:

- Follow system/developer/user instructions plus repo AGENTS/CLAUDE/internal guidance, task_06.md, \_techspec.md, \_tasks.md, ADRs, workflow memory, and required skills.
- No destructive git commands (`restore`, `checkout`, `reset`, `clean`, `rm`) without explicit user permission.
- Must use cy-workflow-memory before code edits and keep task memory current.
- Must use cy-execute-task end-to-end; task requires cy-final-verify before any completion/commit claim.
- Task explicitly requires nats, agh-code-guidelines, golang-pro, agh-test-conventions, testing-anti-patterns, and agh-cleanup-failure-paths.
- Automatic commit is enabled only after clean verification, self-review, tracking updates, and no unrelated staging.

Key decisions:

- Manager will be the durable commit boundary for runtime delivery: outbound prepares an envelope, writes the conversation repository, then publishes; inbound routes and writes accepted conversation rows before prompt delivery.
- Router direct-surface delivery will filter local delivery targets by deterministic direct-room membership, not only by NATS subject/`to`.
- Audit writer should remain an audit sink only; conversation timeline writes move out of audit side effects and into manager-owned conversation persistence.
- Task ingress metadata will be server-derived and merged into run metadata with `network_work_id` as correlation only; task run lifecycle remains under `task_runs`.

State:

- Implementation, self-review cleanup, workflow tracking updates, local commit, and post-commit verification are complete.

Done:

- Scanned existing ledgers and read relevant prior network task ledgers for task_04/task_05 context.
- Loaded workflow memory and current task memory.
- Loaded required skills: cy-workflow-memory, cy-execute-task, golang-pro, nats, agh-code-guidelines, agh-test-conventions, agh-cleanup-failure-paths, testing-anti-patterns, no-workarounds, systematic-debugging, and cy-final-verify.
- Read root/internal repo guidance, PRD docs, \_techspec.md, ADRs, tasks 02-05, and current runtime/store/task ingress code.
- Captured pre-change signal: targeted `go test ./internal/network -run 'TestFormatNetworkMessage|TestEnqueueRunFromPeer' -count=1` passes, while active gaps remain in manager persistence, audit timeline coupling, prompt metadata, and task ingress metadata.
- Patched manager/router/audit/task-ingress/runtime prompt metadata paths: manager writes `NetworkConversationStore` before outbound publish and inbound prompt delivery; router has prepare/publish split and direct-room membership filtering; audit writer is audit-only; task ingress attaches trusted `metadata_json.network_work_id` correlation; `PromptNetworkMeta` no longer exposes `InteractionID`.
- Added/updated tests for durable-before-side-effect ordering, direct routing, wrapper metadata, task ingress run metadata/claimability, and thread/direct/handoff/summarize-back integration.
- Passing focused checks: `go test ./internal/network ./internal/acp -count=1`; `go test -tags=integration ./internal/network -run 'TestManagerPersistsRuntimeConversationSurfacesAndHandoff|TestNetworkTaskIngressCreateAndEnqueueRun' -count=1`.
- Corrected `internal/testutil/acpmock/fixture.go` to match `surface`/`thread_id`/`direct_id`/`work_id` after `make verify` typecheck caught removed `PromptNetworkMeta.InteractionID`.
- Corrected Go lint findings from audit decoupling: split the inbound rejection audit call, removed an unused audit presence mutex, and deleted an unused audit test helper.
- Full `make verify` evidence before current rerun: frontend format/lint/typecheck/tests/build pass, Go lint reports 0 issues, touched packages (`internal/network`, `internal/acp`, `internal/testutil/acpmock`) pass under the full race test phase. The remaining failure was `internal/extension` TempDir cleanup; exact failing tests passed in isolation and package rerun reproduced a different cleanup failure.
- Full `make verify` rerun passed with `0 issues`, `DONE 8160 tests`, and `OK: all package boundaries respected`.
- Self-review removed the stale `WithAuditWriterPresenceWindow` no-op and manager option wiring after audit-side timeline writes were removed.
- Updated task_06.md status/checklists, compact \_tasks task 06 row, task memory, and shared workflow memory.
- Final full `make verify` after tracking/memory edits passed with `0 issues`, `DONE 8160 tests`, and `OK: all package boundaries respected`.
- Pre-commit full `make verify` after recording final memory evidence passed with `0 issues`, `DONE 8160 tests`, and `OK: all package boundaries respected`.
- Created local commit `3fae5a9b` (`feat: wire network conversation runtime routing`) with only task-scoped code/test files staged.
- Post-commit full `make verify` passed with frontend format/oxlint/typecheck/tests/build, Go lint `0 issues`, `DONE 8160 tests`, and `OK: all package boundaries respected`.

Now:

- Final report to user.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- Repo: /Users/pedronauck/Dev/compozy/agh2
- Task files: .compozy/tasks/network-threads/task_06.md, \_tasks.md, \_techspec.md, adrs/
- Workflow memory: .compozy/tasks/network-threads/memory/MEMORY.md, memory/task_06.md
- Code/tests touched: internal/network/manager.go, router.go, audit.go, tasks.go, manager_test.go, router_test.go, audit_test.go, tasks_test.go, manager_integration_test.go, tasks_integration_test.go, internal/acp/types.go, internal/acp/client_test.go, internal/testutil/acpmock/fixture.go.

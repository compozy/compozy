# Agent memory store

The daemon owns one `Store` and shares it across transports. Callers resolve the immutable profile ID, definition origin (`user`, `extension-<id>`, or `workspace-<id>`) and agent slug before calling the store. Stays-on eligibility and caller authorization belong to the transport-independent service boundary, not the file format.

`Read` returns the editable file and its digest. `Write` requires that digest; `Add`, `Replace`, `Remove`, and `Rollback` serialize with it per key. Writes redact secrets before persistence. `Load` returns a bounded startup snapshot, archiving dated overflow first. When undated content alone exceeds the budget, loading truncates without changing disk and entry mutations fail; an operator can repair the file with a digest-checked write.

The store writes `archive.md` before replacing `MEMORY.md`. A crash between these operations can duplicate archived notes but cannot lose the original notes. File revisions retain the latest 50 snapshots, including the content before an edit, and rollback uses the same redaction and budget path. Files use mode 0600 and directories use 0700. Retiring an agent renames its directory; removing an extension must not call retirement.

Search scans current and archived notes, ranks by the fraction of query terms matched, and preserves file/line order for ties. With `include_sessions`, the shared API service also pages the profile session catalog, authorizes each workspace before reading its transcript, and appends redacted matches with `source: session` and `session_id`. Denied workspaces are omitted without requesting broader access. Both HTTP and native tools use this service.

The owning contract is ADR-007 and the Memory sections of the `always-on-agent` spec. This package introduces no database or configuration changes and does not import legacy memory files. Its focused filesystem suite covers US-025/US-026 storage invariants; public transport, startup, and eligibility checks belong to their owning integration suites.

## Verification ownership

| Spec cases             | Canonical coverage                                                                                                                                                                                              |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| UT-050, UT-053, UT-085 | `agentmemory.TestStore/Should persist scoped entries and redact every write path`; shared API memory routes also reject stale edits.                                                                            |
| UT-051                 | `agentmemory.TestStore/Should archive oldest dates while preserving undated notes`                                                                                                                              |
| UT-052, UT-056         | `agentmemory.TestStore/Should rank lexical matches and support replacement and removal`                                                                                                                         |
| UT-054                 | `daemon.TestAgentMemoryNativeTools`; `core_test.TestAgentMemoryRoutes`                                                                                                                                          |
| UT-055, UT-102, UT-104 | `agentmemory.TestStore/Should preserve parallel adds and isolate definition origins`; deletion retirement also runs through `TestBaseHandlersAgentDefinitionMutations`.                                         |
| UT-101                 | `agentmemory.TestStore/Should bound oversized undated snapshots without modifying notes`                                                                                                                        |
| UT-103                 | `agentmemory.TestMemoryBoundaries/Should archive oversized loaded files and tolerate interrupted archival`                                                                                                      |
| IT-022                 | `daemon.TestPromptInputCompositeIntegrationAgentMemory` and `TestPromptInputCompositeIntegrationActiveAgentMemory`: cross-workspace fenced snapshots and active/new session behavior.                           |
| IT-023                 | `TestPromptInputCompositeIntegrationActiveAgentMemory`: SQLite session transcripts and the default workspace policy; `core.TestAgentMemorySessionSearch` owns pagination and exclusion before transcript reads. |
| IT-024                 | Store rollback/reopen and retirement cases, shared definition deletion regression, and startup after a profile-name change with the same immutable profile ID.                                                  |

Focused commands (from the repository root):

```sh
CGO_ENABLED=1 go test -race ./internal/agentmemory -count=1
CGO_ENABLED=1 go test -race ./internal/api/core -run 'TestAgentMemory|TestBaseHandlersAgentDefinitionMutations/Should_delete_only' -count=1
CGO_ENABLED=1 go test -race -tags integration ./internal/daemon -run 'TestAgentMemoryNativeTools|TestPromptInputCompositeIntegration.*AgentMemory' -count=1
go build ./...
```

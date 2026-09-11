# BUG-20260911-workspace-hooks-not-dispatched: Configured workspace hooks never run for managed sessions

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-26 draft admission under concurrent input
- **Scenarios:** GL-013 (blocks its public race fixture; no Goal race verdict)
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction and observed failure

Create a required synchronous input.pre_submit hook through compozy__hooks_create in disposable workspace ws_fc0dcf020b8034f1, matching field-writer. The hook is a bounded Python subprocess that reads its documented JSON input, records entry and waits for an owned release file when the unique draft marker appears; all other inputs return an empty patch. Restart the isolated daemon. hooks list reports the declaration, but rewrites its workspace matcher to durable identity 01M26N9AVTCXB8NYSTT4PTWKCJ. Submit a marked /goal draft in fresh managed Cursor/Grok4.6 High Fast session sess-89391f6df40779b5. The draft runs normally; no barrier entry file is produced and public hooks runs returns an empty list. The ordinary session uses workspace_id ws_fc0dcf020b8034f1.

After integrating main ace125a7e and rebuilding binary85c9d045c-dirty and Web, repeat with a user-scope hook restricted by the lab directory and field-writer. The workspace effective hook still receives the durable ULID matcher. Fresh sess-a7175fd376d1bec2 finishes its marked draft with HTTP200, finishReason stop and no hook execution. Neither attempt reached the intended race barrier; neither is a GL013 pass. No concurrent winner was submitted.

Evidence under docs/qa/evidence/2026-09-10-qa-execution-unblock/: goal-race-hooks-list.json, goal-race-observe-startup.json, goal-race-hook-observe.json, goal-race-user-hook-create.json, goal-race-user-hook-list.json, goal-race-user-draft-response.json, goal-race-user-hooks-observe.json. Both sessions stopped. Both hook scopes were deleted through native tools and the daemon restarted; goal-race-user-cleanup-config.json and goal-race-user-cleanup-workspace.json report empty hook config, with declaration absence confirmed by goal-race-user-cleanup-hooks.json.

## Initial diagnosis

The observed catalog/session mismatch agrees with scopeWorkspaceHookDecls injecting ResolvedWorkspace.WorkspaceID (durable directory ULID), while hookSessionContextFromInfo publishes the session registry ID. Workref only trims; dispatchRuntime forwards unchanged. Existing TestBootBuildsHooksFromWorkspaceConfigAgentAndSkills constructs a Session directly using the durable ID, masking the real session identity boundary. The agent-catalog mismatch repaired in BUG-20260803-agent-workspace-id-disagrees has the same two identities but a different observable; do not reopen that verified bug.

Next: establish a red case in the canonical daemon hook integration suite using the real registered session identity and preserve workspace isolation. Audit other hook payload families and authored matcher compatibility before selecting the smallest correction. No production fix has been applied.


## Repair under verification

The canonical daemon integration suite reproduced failures for configured/agent/skill session hooks, workspace task-run hooks and hot-reloaded skill hooks after using the registered ID carried by real runtime payloads. workspace-hooks-red.txt records all three failures. scopeWorkspaceHookDecls now injects ResolvedWorkspace.ID. The directory identity and matcher schema remain intact; a foreign-workspace task event is explicitly rejected before the matching event executes. See the owning cross-surface audit in docs/_memory/change-impact.md. Regression and public re-walk are pending; no fixed verdict yet.

The three canonical integration walks pass after the scoping correction (workspace-hooks-green.txt). Boundary follow-up found the shared HTTP/UDS hook catalog also filtering by directory identity; a red existing handler fixture proves the mismatch. That handler now matches the already-correct native tool's registered-ID query. The existing registration-refresh test's query was aligned; production dispatch assertions remain strict. The heuristic Go-test checker reports identical31 lines against baseline and current integration file, including a file-wide t.Setenv/t.Parallel false positive across separate cases; no new convention diagnostic was introduced.


## Verified replay

Fresh sess-49c5692469646aaf on the rebuilt binary enters the previously missing public input.pre_submit hook. CLI and native tool catalogs both expose the registered workspace matcher. The hook barrier enables the exact GL013 race: idle rewritten draft, ordinary prompt wins, draft409 without preemption/queue growth, ordinary native terminal task completes. After cleanup/restart the Goal and Run are absent, queue empty and both fixture config/catalog restored. Full evidence: docs/qa/evidence/2026-09-10-qa-execution-unblock/goal-race-fixed-proof.json.

Required make gate passed (workspace-hooks-gate.txt). The real-provider replay is separate from deterministic Go integration/boundary coverage. GL013 is verified using this race and the earlier complete prefill/busy evidence. Fix commit will be recorded after the authorized local commit; no push.

Final fix commit after main integration: `a172d49ff51da7a2f09dfb619a48ad041c5da36d`. The initial diagnosis and pending paragraphs above are chronological history; the verified replay owns the current verdict.

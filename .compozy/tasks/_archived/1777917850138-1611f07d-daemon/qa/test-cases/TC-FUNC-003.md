## TC-FUNC-003: Sync and Archive Daemon Control Plane

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'TestSyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory|TestArchiveCommandArchivesSyncedWorkflowIntoNewPathFormat' -count=1`
- Supporting seam: `go test ./internal/core -run 'Test(SyncTaskMetadataSyncsSingleWorkflowIntoGlobalDBWithoutMutatingArtifacts|SyncTaskMetadataRemovesLegacyGeneratedMetadataOnce|ArchiveTaskWorkflowRejectsPendingStateFromSyncedDBEvenWithStaleMeta|ArchiveTaskWorkflowRejectsActiveRunConflict|ArchiveTaskWorkflowsRootScanUsesDBStateAndSortsSkippedPaths)' -count=1`
**Automation Notes:** The public CLI lane proves operator behavior. The core integration lane locks the metadata-free sync/archive rules required by the daemon model.

### Objective

Verify that `compozy sync` and `compozy archive` are daemon-backed, operate from DB state instead of generated metadata, and keep archive eligibility deterministic.

### Preconditions

- [ ] Workspace fixtures include both archive-eligible and ineligible workflow state.
- [ ] The execution branch still treats `_tasks.md` and `_meta.md` as non-operational.

### Test Steps

1. Run the focused CLI sync/archive suite listed above.
   **Expected:** The package exits `0` and the public commands pass from both the workspace root and subdirectories.

2. Confirm the archive test validates the daemon-era archived path format.
   **Expected:** Archived workflows move into `.compozy/tasks/_archived/<timestamp>-<id>-<slug>` and skipped/conflict behavior is deterministic.

3. Run the supporting core sync/archive suite if deeper lifecycle validation is needed.
   **Expected:** Sync updates `global.db` without recreating `_meta.md`, legacy metadata cleanup stays one-time only, and archive rejects incomplete or active workflows.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Workspace subdirectory | Run `sync` or `archive` below workspace root | Correct workspace target is still resolved |
| Stale legacy metadata | Old `_meta.md` or generated `_tasks.md` present | Sync does not restore them as operational truth |
| Incomplete workflow | DB state not terminal | Archive is rejected/skipped deterministically |
| Active run conflict | Workflow still active | Archive is blocked with stable conflict behavior |

### Related Test Cases

- `TC-FUNC-001`
- `TC-FUNC-002`

### Traceability

- TechSpec Integration Tests: `sync` updates `global.db` but does not recreate `_meta.md`; `archive` is DB-state gated.
- ADR-002: operational truth moves to SQLite, not workspace runtime metadata.
- ADR-004: `sync` and `archive` remain top-level commands.
- Task references: `task_14`, `task_17`.

### Notes

- Any failure that suggests a return to metadata-file heuristics should be treated as a migration regression, not a documentation issue.

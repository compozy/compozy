# B2 test coverage

Worktree: stays-on-b2. Source: canonical always-on-agent `_tests.md` only. This is an incomplete delivery record.

| Cases | Evidence / remaining work |
| --- | --- |
| US-014 | Admission, idempotency, caps and workspace lifecycle covered below; wake isolation pending. |
| US-015 | Identity, permissions and depth covered; full native create/rotation integration pending. |
| US-016 | Pause/resume/archive/recreation covered; streak reset pending. |
| US-017 | Adoption rejection, rollback and history preservation covered. |
| UT-016–017 | globaldb TestInstanceServiceRecovery, TestInstanceServiceConcurrentCreate |
| UT-018, UT-020–023 | daemon TestInstanceCreateAdmission; agentinstance TestInstanceAuthority; globaldb TestInstanceCallerLineage |
| UT-019 | globaldb TestInstanceConcurrentLimit: 11 distinct calls, cap 10 |
| UT-024, UT-097 | globaldb TestInstanceLifecycle |
| UT-025 | Missing implementation: no instance streak state/reset found. |
| UT-026 | globaldb TestInstanceServiceRecovery: missing definition preserves paused row |
| UT-027 | globaldb TestInstanceServiceRecreation |
| UT-028–029, UT-031, UT-098 | daemon TestInstanceAdoptAdmission |
| UT-030 | globaldb TestInstanceAdoption |
| UT-084 | core TestAgentInstanceDefinitionDeletion, TestAgentInstanceDeletionPagination |
| UT-086 | core TestAgentInstanceErrorResponses covers eight sentinel codes; full documented mapping still requires completion. |
| IT-007 | globaldb TestInstanceWorkspaceIsolation covers lifecycle isolation; independent wake state missing. |
| IT-008 | globaldb TestInstanceWorkspaceArchive and TestInstanceWorkspaceDeletion |
| IT-009 | Native approval boundary, child policy lookup and session TestInstanceLaunchCeiling cover inherited launch restrictions; full create-to-rotation propagation pending. |
| IT-010 | globaldb TestInstanceAdoptionHistory: real global/session SQLite; event equality after adoption and reopen |
| IT-036 | Missing implementation: no agent.instance hook emission found. Documented payload unavailable in permitted test-only spec input. |
| IT-037 | globaldb TestInstanceAdmission, TestInstanceConcurrentLimit, TestInstanceServiceRecovery, TestInstanceServiceConcurrentCreate, TestInstanceServiceRetainedFailure, TestInstanceLifecycle, TestInstanceAdoption |
| IT-038 | globaldb TestInstanceLifecycle guards archive/delete; session TestInstanceMainPromptResume |
| IT-042 | agentinstance authority matrix and daemon TestNativeInstanceAuthority cover supported verbs; memory/checks/occurrences routes and full transport matrix pending. |

## Corrections exposed by tests

- Instance HTTP errors now retain canonical machine codes when wrapped by a repository error.
- Agent deletion now rejects non-archived instances before removing history or authored files, scanning every instance page in the applicable profile/workspace scope.

- Empty successful provider starts now release the unstarted instance row, preserving retry capacity. TestInstanceEmptyStartCleanup failed before the correction and passed afterward.

## Verification so far

The consolidated command `CGO_ENABLED=1 go test -race ./internal/agentinstance ./internal/store/globaldb ./internal/api/core ./internal/daemon ./internal/session -run 'TestInstance|TestAgentInstance|TestNativeInstance' -count=1` passed all five packages. `go build ./...` exited 0 after production fixes, with a module-cache metadata permission warning. Both used `GOCACHE=/private/tmp/compozy-b2-go-cache`. Pinned diff lint passed for agentinstance, API, globaldb and daemon; session lint is pending. Browser QA is not claimed. This commit is a verified partial checkpoint, not completion of the full case matrix.

## Unrelated generator output

`extensions/open-design/lint.gen.mjs` differs from HEAD only by local variable `css2` becoming `css` at declaration/use. The current generator's `--check` passes. Preserve this existing working change and exclude it from B2 commits.

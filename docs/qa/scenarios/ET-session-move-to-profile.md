---
id: ET-session-move-to-profile
area: ET
title: Move a session started in the wrong profile to another profile
persona: Ada
journey: J-scope-work-by-profile
expected: "A stopped top-level user session moves to another active profile from the row menu (Move ▸ Move to profile → profile name → dialog → Move to {profile}), the CLI (`compozy session move-to-profile <id> <profile>`), HTTP/UDS (`POST /api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile`), or the native tool `compozy__session_move_to_profile`. The result is a fork owned by the target profile with the same agent, title, and conversation, and the original archived in its own profile. A bound worktree is removed with its branch and commits kept, and the branch is checked out again in a new worktree owned by the target profile, which the copy binds. Archived, stopped descendants stay in the source profile with unchanged lineage; live descendants or unarchived descendants outside the original's subagent archive family refuse with a blocker count and instructions to stop and archive them first. A running session is refused before anything is sent (web) or with 409 `session_move_to_profile_running`; a worktree with uncommitted changes with 409 `session_move_to_profile_worktree_dirty`; refusals leave the original unarchived and its worktree in place."
entry_points: web session row menu and Move to profile dialog; compozy session move-to-profile; HTTP/UDS POST /api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile; native compozy__session_move_to_profile
qa_status: pending
bug_ids: [1298, 1320]
fix_status: fixed
retest_status: pending
fix_commits:
evidence: PR for #1185 (lab walk 2026-11-03, screenshots and transcripts in the PR body)
last_report: PR for #1185
overlaps: ET-web-session-fork-from-here; RT-session-worktree-fork; ET-profile-cli-lifecycle
---

Walk (isolated lab, two profiles `default` and `work`, acpmock agent, git workspace):

1. Create a worktree on a new branch in `default`, start a session on it, and run one turn.
2. While the session is live, move it with the CLI and the HTTP route: both refuse with 409
   `session_move_to_profile_running`; the web dialog says "Stop this session before moving it." and
   keeps Move to {profile} disabled.
3. Stop it, leave an uncommitted file in the worktree, and move it: 409
   `session_move_to_profile_worktree_dirty` naming the file count and branch; the original stays
   unarchived and its worktree ready.
4. Commit the file on the branch (no remote), then move it: the copy is a `fork` of the original in
   `work`, bound to a new ready worktree owned by `work` on the same branch; the original worktree
   reads `removed`, the branch still points at the new commit, and the original is archived in
   `default`. Prompt the copy under `--profile work`; it answers.
5. In the web session list, right-click a stopped worktree session → Move ▸ Move to profile → `work`: the
   dialog shows "Usage so far stays counted in default." and the worktree note; confirming shows
   "Moved to work" / "The original is archived here." and the row leaves the `default` list.

6. Interrupted move (#1211): start a move of a stopped worktree session with unpushed commits and
   kill the daemon (`SIGKILL`) while it runs, then start it again. The move settles at boot: either
   the copy exists in `work` and the original is archived, or the original is unarchived in
   `default`, bound to a ready worktree on its branch, with no copy and no leftover `work` worktree.
   The branch still points at its last commit, and no worktree row is left `removing`.

Covered by the owning suites, not walked: rollback after a failure past the archive (session suite
`TestManagerMoveToProfile`), recovery after a stop at each move step (`TestManagerRecoverProfileMoves`),
an interrupted keep-branch removal (`TestServiceRemoveAndRecover`), children and own-session
refusals, and the native tool's target-profile write authorization (`TestNativeSessionMoveToProfile`).

Native permission check: at `approve-edits` or `auto`, moving a session to another profile requires
destructive-tool approval because it archives the original and releases its worktree. Covered by
`TestBuiltinEditLevelAutoApprovals` and `TestBuiltinNativeDescriptors`.

7. Archived descendants (#1298): create a stopped root with a child and grandchild, stop and
   archive both descendants, then move the root. The target copy is created; the original and
   descendants remain archived in the source profile with unchanged descendant lineage.
8. Leave one descendant unarchived (repeat once stopped and once live), with the others archived.
   Moving refuses with 409 `session_move_to_profile_has_children`, counts only the one blocker,
   and says to stop and archive it first. The dialog displays that daemon message. No session
   changes profile, archive state, or lineage after refusal; no "without children" option is offered.

Automated verification for #1298 (2026-11-04): `TestManagerMoveToProfile` verifies steps 7–8
against real SQLite, including child/grandchild lineage and source-profile retention, and both
stopped and live unarchived blockers. The existing race-enabled lifecycle integration checks
`TestManagerIntegrationFullLifecycle` and `TestManagerIntegrationUserLifecycle` passed.
The fresh browser/daemon walk remains pending; these suite results do not claim that walk.

9. Concurrent restore: begin moving a stopped root with archived descendants. Unarchiving a
   descendant after move planning but before commit waits for the move; a canceled restore
   returns its context error and leaves the descendant archived. After the move completes,
   unarchive works normally. A restore completed before the move acquires descendant locks
   causes the move to refuse before archiving the root.

The profile-move suite also verifies unchanged descendant conversation history and that
canceling lock acquisition releases already-held descendant locks.

10. Stopped subagent family (#1320): create a stopped root with stopped subagent children and
    grandchildren, then move the root without archiving it first. The original and its subagent
    archive family are archived in the source profile, with history and lineage unchanged; only
    the fork belongs to the target. Repeat with a live subagent: the move refuses without changes.
    A subagent below an archived non-subagent remains a blocker until that separate family is archived.
11. Force target checkout creation to fail after archiving a stopped subagent family. The rollback
    restores the original and subagents to their prior unarchived state in the source profile,
    all bound to the replacement checkout. Resume and stop each family member; all succeed.

Automated verification for #1320 (2026-11-06): `TestManagerMoveToProfile` verifies steps 10–11
against real SQLite, including stopped subagent children and grandchildren sharing the original's
worktree, history/lineage retention, the individual subagent archive refusal, rollback after target
checkout failure, live subagent refusal, and a stopped subagent outside the original's archive
family. `TestManagerRecoverProfileMoves` and the existing integration suites
`TestManagerIntegrationFullLifecycle` / `TestManagerIntegrationUserLifecycle` also passed.
These results do not claim a fresh browser walk.

12. After a successful move of a family sharing one checkout, unarchive the original. The original
    and subagents return to the workspace directory without a binding to the removed checkout;
    subagents with their own checkout retain it. Resume and stop the family successfully.

The owning suite verifies family bindings and resumes after rollback, successful-move unarchive,
and interrupted rollback recovery. It uses the public Manager move, archive, unarchive and resume
methods with real SQLite; transport mapping remains covered separately by the existing handler,
CLI and native-tool suites.

13. Fail an inherited child binding update, then a source binding update, during rollback. While
    the failure persists, unarchive refuses with the open move unsettled. Remove the injected
    store failure and unarchive again: rollback completes first, both catalog bindings point to
    the restored checkout, and journal recovery is idempotent. No binding is cleared to bypass
    recovery. Repeat with padded IDs and reject a different workspace without changing state.
14. Ordinary archive/unarchive with a missing or damaged worktree and subagents keeps the existing
    binding and does not inspect Git. Only the recorded original of a completed profile move can
    clear inherited bindings to its released checkout. A stale restoration snapshot refuses before
    changing any child binding. Family conversation locks use the same sorted order as workspace
    removal, and restoration re-reads the root under those locks.

The owning profile-move suite covers child/source binding-store failures followed by unarchive
and journal retry, padded IDs, ordinary unarchive without Git, stale-root restoration, and workspace
removal preparation during restoration. The optional completed-move provenance in source metadata
is written before deleting the open journal; older metadata without it keeps ordinary unarchive
behavior. No SQLite schema change is required.

15. Move a source with no descendants, then unarchive and resume it: its released-checkout binding
    is cleared as for a family. Rename an archived original while completion provenance is being
    recorded: both the new name and the completed-move record survive. The owning move suite
    covers the childless workflow and coordinates the rename snapshot with the provenance write.

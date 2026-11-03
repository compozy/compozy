---
id: ET-session-move-to-profile
area: ET
title: Move a session started in the wrong profile to another profile
persona: Ada
journey: J-scope-work-by-profile
expected: "A stopped top-level user session moves to another active profile from the row menu (Move to profile → profile name → dialog → Move to {profile}), the CLI (`compozy session move-to-profile <id> <profile>`), HTTP/UDS (`POST /api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile`), or the native tool `compozy__session_move_to_profile`. The result is a fork owned by the target profile with the same agent, title, and conversation, and the original archived in its own profile. A bound worktree is removed with its branch and commits kept, and the branch is checked out again in a new worktree owned by the target profile, which the copy binds. A running session is refused before anything is sent (web) or with 409 `session_move_to_profile_running`; a worktree with uncommitted changes with 409 `session_move_to_profile_worktree_dirty`; refusals leave the original unarchived and its worktree in place."
entry_points: web session row menu and Move to profile dialog; compozy session move-to-profile; HTTP/UDS POST /api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile; native compozy__session_move_to_profile
qa_status: pass
bug_ids:
fix_status:
retest_status:
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
5. In the web session list, right-click a stopped worktree session → Move to profile → `work`: the
   dialog shows "Usage so far stays counted in default." and the worktree note; confirming shows
   "Moved to work" / "The original is archived here." and the row leaves the `default` list.

Covered by the owning suites, not walked: rollback after a failure past the archive (session suite
`TestManagerMoveToProfile`), children and own-session refusals, and the native tool's
target-profile write authorization (`TestNativeSessionMoveToProfile`).

Native permission check: at `approve-edits` or `auto`, moving a session to another profile requires
destructive-tool approval because it archives the original and releases its worktree. Covered by
`TestBuiltinEditLevelAutoApprovals` and `TestBuiltinNativeDescriptors`.

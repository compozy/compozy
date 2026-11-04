---
title: Move a session to another profile
type: feature
---

A session started in the wrong profile can now move to another one. **Move to profile** copies a stopped, top-level session into another active profile and archives the original. The copy keeps the agent, title and conversation, and uses the target profile's agents, credentials and config when it starts. The original keeps its history and past usage in its own profile.

- **Where:** the session row menu (**Move to profile**), `compozy session move-to-profile <id> <profile>`, and HTTP/UDS `POST /api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile`. Agents use `compozy__session_move_to_profile`, which needs the unrestricted workspace-access grant and can't move the agent's own session.
- **Worktrees move too.** A session bound to a worktree gets a new worktree on the same branch in the target profile. Every commit, including unpushed ones, stays on the branch.
- **What is refused:** running sessions (stop them first), sessions with child sessions, worktrees with uncommitted changes, and worktrees shared with another session. Each refusal returns a `session_move_to_profile_*` error that names the rule.
- **Interrupted moves recover.** If a move fails partway, CompozyOS rolls it back. If the daemon stops in the middle, it finishes or rolls back the move at the next start. The session and its branch are never deleted.

`compozy session move` still moves a session between folders.

### Migration notes

- The daemon applies a new database migration at startup. Existing worktrees keep their behavior.
- Deleting a session now returns 409 while a move of that session is still being recovered. It succeeds once the move settles at the next daemon start or the next move of that session.

---
title: See the worktrees, branches and pull requests behind each session
type: feature
---

A session now has **Linked work**: the worktrees, branches and pull requests that belong to it, separate from where its agent runs. A worktree a session creates with `compozy__worktree_create` shows up by itself, with no bind and no restart. Pull requests from child sessions also appear on the parent, read-only, as "From {child}".

- The session header shows the most urgent item, and `+3 PRs` or `+3 more` opens the list. List rows show a count, Kanban cards show the lead pull request plus `+N`, and the command palette finds a session by pull request number or branch.
- Paste a link or search by number, title or branch to link more by hand, or run `compozy session link <session-id> --pr-number 142`. `compozy session unlink` removes a link.
- Agents can link what they made outside the worktree tool with `compozy__session_link` and `compozy__session_unlink`, on their own session only.
- Unlinking removes the association and nothing else, and it offers **Undo**. An automatic link you unlink doesn't come back.
- The pull request details tooltip is now a checklist of what stands between the pull request and merging.

Limits: a pull request from another project is refused, and one from a fork can't be linked yet. A branch holds one pull request, so linking a second replaces the first. Pull request search needs a connected forge; without one you can still link branches.

Migration notes: one board behavior changes. A session that has settled with an open pull request linked directly to it now sits in **In review** instead of **Done**. A running session stays in **Working**, and so does one with queued input, so a session never enters **In review** while it works. Pull requests from child sessions never move the parent. With no open linked work, the column is the same as before. Existing `branch` and `pull_request` fields keep their meaning, session payloads gain an optional `linked_work`, and forge extensions need no change: without pull request search they still link branches.

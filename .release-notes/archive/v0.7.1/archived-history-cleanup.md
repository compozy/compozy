---
title: Clean up archived session history in bulk
type: feat
---

The Archived view in the sessions sidebar now has **Clean up history…** in its footer. Choose whether to delete whole archived sessions, or only their child sessions or subagents, and how old they must be: archived more than 7, 15 or 30 days ago, before a date, or at any time. The dialog counts every session that will be deleted before you confirm, and above 100 sessions you type that count to confirm.

Deleting runs in the background, so you can close the dialog while the footer shows `Deleting… X of N`, and Stop ends it early. Only archived sessions are deleted: child sessions still in your lists or in Later, always-on agents' main sessions, and worktrees all stay. If some sessions can't be deleted, the dialog lists them and Try again retries only those.

Scripts and agents get the same cleanup through `compozy session cleanup` (preview by default, `--yes` to delete, plus `status` and `stop`), `POST /api/sessions/history-cleanup`, and the `compozy__session_cleanup` native tool, which previews by default and asks for approval before deleting.

Migration notes: no action needed.

---
title: Mark a session's Needs you as seen
type: feature
---

A session in **Needs you** used to stay there until something changed, even after you had looked at it, and a failed session stayed until you archived it. Now you can mark it as seen from the session row menu, the Kanban card, or the button in the session header.

- **What it clears:** a turn that ended early with no pull request and no open work, a session back from Later, and a failed session. A failed session then reads as stopped and keeps its failure record. The next failure, early stop or Later return raises Needs you again.
- **What it keeps:** a pending permission, a question waiting for your answer, a missing provider sign-in, or a stop CompozyOS couldn't confirm still need you. Marking those as seen is refused, and the refusal names the answer still owed.
- Marking a child session as seen never wakes its parent.
- **From the terminal and agents:** `compozy session mark-seen <id>`, the native tool `compozy__session_mark_seen`, and HTTP/UDS `POST /api/workspaces/{workspace_id}/sessions/{session_id}/mark-seen`. A refusal returns `attention_requires_action`. Session payloads gain `attention_dismissible`, which is true only while the current Needs you can be marked as seen.

### Migration notes

The daemon applies a new database migration at startup. Existing sessions are unchanged.

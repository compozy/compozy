---
id: ET-session-mention-session
area: ET
title: Mention another session and the agent reads it with compozy__session_history
persona: Théo
journey:
expected: The picker's "Sessions" group lists up to 5 same-workspace sessions (current, archived, and other-workspace sessions excluded) with agent and relative time; picking one inserts a title chip and sends `@session:<id>`; the agent receives a `<compozy_session_references>` text block in reference form after the prompt text, never shown in the operator bubble, and calls compozy__session_history with that session_id before answering.
entry_points: web session window composer; compozy__session_history; compozy session events --json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-mention-session-inline; ET-session-mention-history-chips
---

E2E-002, US-003, US-006.AC-1/2/4. Create session A titled "Fix login bug" with a short exchange and
session B in the same workspace, once with real Claude and once with real Codex. Also create a session
in a second workspace and archive one session of the first. In B type `@`: Sessions shows the 5 most
recently updated sessions of this workspace without B itself, the archived one, or the other
workspace's session. Type `@login` and pick A: the chip reads "Fix login bug". With two sessions of the
same title, the secondary "agent · relative time" tells them apart.

Send. The bubble shows the text with the chip only. The delivered prompt has the text block, then a
text block opening `<compozy_session_references>` with `@session:<A> · title "Fix login bug" · agent
… · state … · updated <RFC 3339 UTC>` and the sentence telling the agent it MUST call
compozy__session_history; the delivery record shows `references_form: reference`. B's turn shows a
compozy__session_history call with A's id and an answer grounded in A's history. Type the archived
session's `@session:<id>` by hand and confirm it resolves; `@session:<B>` (self) and a deleted id stay
plain text.

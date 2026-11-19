---
id: ET-web-side-chat
area: ET
title: Ask side questions while the main session works
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: Side questions use frozen main context without interrupting main work or adding messages to its transcript; side history, attention, lifecycle, and profile ownership remain attached to the source.
entry_points: Web session Side chats; /side <question>; session.side.toggle; POST/GET /api/workspaces/{workspace_id}/sessions/{session_id}/sides
qa_status: planned
bug_ids:
fix_status:
retest_status: unrun
fix_commits:
evidence:
last_report:
overlaps: RT-session-derive-native-fork; RT-session-derive-retry; ET-cli-session-continue
---

Planned 2026-11-19 for the side-chat spec. No scenario walk was executed in the backend slice:
the authorized local validation is generation and `go build ./...`; integration owns the combined walk.

1. In an isolated QA lab, start a real Claude session with work that takes several minutes. Record
   the main transcript and pending interactions, then send `/side what is it doing right now?`.
2. Confirm the side panel opens, its transcript contains only the question and side answer, and its
   answer refers to the in-progress main work. The main status reads **Main working**. No `/side`
   message, queue entry, or interruption appears in the main session.
3. Ask a follow-up about a decision in the inherited plan. Confirm the side answers without resuming
   that plan or editing files. Ask a third question and verify it keeps acting as a side chat.
4. Let main work finish. Confirm its normal answer appears and its status changes to **Main finished**.
   A source history read from the side can observe new work, but its original context copy stays frozen.
5. Hide the side, retain a draft, and reopen with Command+Option+S or Control+Alt+S. Revisit another
   side through the switcher. Open the same main in a second window and verify side-list updates.
6. Trigger a permission or clarification in a side and verify its attention dot. The side stays absent
   from session lists, boards, attention summaries, subagent trees, and history-cleanup selection.
7. Close one side. Archive and restore the main: open sides return, while the side closed earlier stays
   archived. Explicitly stop the main with a running side and confirm both stop without losing history.
8. Move a source with one open and one closed side to a second profile. Both remain archived with the
   original; the new profile copy lists no sides. Restore the original and confirm only its formerly
   open side returns. Record source, copy, and side ids with profile ownership.
9. Delete a source with an archived side and verify all its sides disappear. Record API responses,
   screenshots, provider versions, and the integrated commit in the eventual QA report.

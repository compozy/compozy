---
id: RT-mcp-reload-failure-recovery
area: RT
title: A failed native load continues from history and a failed launch returns the message to the composer
persona: Théo
journey:
expected: When the native session/load fails the next message is still delivered and the transcript shows "conversation continued from history" (outcome replayed); when the launch fails the prompt is refused with 409 session_mcp_reload_failed (retryable, client message id), the Web composer gets the message back (not an unconfirmed send) and the transcript shows "Couldn't start … with the new MCP servers"; after removing the fault, sending again applies.
entry_points: session prompt (Web composer, compozy session prompt, compozy__session_prompt); session.mcp_reloaded; POST prompt 409 session_mcp_reload_failed
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-mcp-hot-activation-next-message
---

Covers E2E-010 (US-001.EC-3, Rule 9). acpmock-driven, not provider-real.

Lab: `eng-qa-bootstrap` isolated lab with an acpmock agent registration (`internal/testutil/acpmock`)
whose fixture file lives in the lab, plus the echo fixture.

Steps:

1. Create an acpmock session; send one message. Set `"load_missing": true` in the agent fixture
   (session/load fails with resource-not-found).
2. Add echo in Settings. Send "call echo". The message is delivered; transcript line "MCP servers
   updated · conversation continued from history"; event outcome `replayed`.
3. Add another server, then corrupt the acpmock fixture file (launch fails before initialize).
4. Send a message from the Web composer. Response `409 session_mcp_reload_failed` with
   `retryable: true` and the same `client_message_id`; the composer shows the message again (no
   "unconfirmed" state); transcript shows "Couldn't start <runtime> with the new MCP servers · your
   message wasn't sent"; event outcome `failed`; no user message persisted.
5. Repeat 4 with `compozy session prompt` and `compozy__session_prompt`: same code.
6. Restore the fixture; send again: outcome `applied` (or `replayed`), the tool works.

Pass: steps 2, 4, 6 as described; the stored fingerprint advances only in 2 and 6.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md

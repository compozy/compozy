---
id: RT-mcp-reload-failure-recovery
area: RT
title: A failed native load continues from history and a failed launch returns the message to the composer
persona: Théo
journey:
expected: When the native session/load fails, the next message is delivered and the transcript shows "conversation continued from history" (outcome replayed). When an MCP reload launch fails or hosted MCP cannot establish a first or released runtime bind, the prompt is refused with 409 session_mcp_reload_failed (retryable, client message id) and the Web composer restores the draft with "Message not sent". Sending again succeeds after the fault clears.
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
7. Create a logical session and leave it unbound. In the isolated lab, prevent the hosted MCP
   proxy from completing bind before its deadline, including a proxy that arrives after the
   deadline. Repeat with a fault in the initial tools projection during bind. Verify the response
   from step 4 and confirm the transcript has no user message or MCP reload event for the attempt.
   For failed bootstrap and stream projections, check the daemon log for `cause_class` and verify
   it omits upstream error text and binding tokens.
8. Clear each fault and resend with the same client message ID and idempotency key. Verify one
   persisted user message and one provider delivery. Repeat step 7 with a runtime that was bound
   successfully and then idle-released, keeping the MCP catalog unchanged.

Pass: steps 2, 4, 6, 7 and 8 as described. Failed binds leave the previous activation fingerprint
and user message count intact.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md

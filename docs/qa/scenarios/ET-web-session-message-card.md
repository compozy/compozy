---
id: ET-web-session-message-card
area: ET
title: See who sent a session message, and the reply it got back
persona: Bruno
journey: J-13-follow-a-live-run
expected: A message one agent session sends another with compozy__session_prompt renders in the receiver's transcript as a left-aligned framed card (the sender's provider mark with no status dot, "From {current title}" as a link, a Steered / Interrupted / Superseded chip only when it arrived that way, "Reply requested" when notify_on_complete was set, the time, and the body with the operator bubble's 176px clamp and Show more), never as a right-aligned operator bubble; a deleted sender reads "From a deleted session" with no link, a renamed one shows its current title, and one in another workspace adds its folder and workspace name; while queued it shows in the queue strip as "From {title}" with the provider mark and Remove only (no Steer or edit); in the sender's transcript the call renders as a "Sent to {title}" card outside the work group (Sending… → Waiting for reply with a pulsing success dot → Replied / Failed / Canceled / Dropped / Unknown, or no reply state when none was requested, or "Could not send to {title}" with the error) and the reply lands as a left-aligned "Reply from {title}" card with the target's mark, an outcome dot and word, and the answer, a fixed sentence for dropped / unknown, "No reply text." when the turn left none, and "Reply truncated · Read the full turn" when cut at 12,000 characters; every surface matches its VC in docs/design/opendesign/agent-collaboration/.
entry_points: web session window transcript (SessionMessageCard, SessionReplyCard, SessionSentCard); composer queue strip (SessionQueueEntryRow); docs/design/opendesign/agent-collaboration/agent-collaboration-transcript.html (VC-01…VC-04) and agent-collaboration-composer.html (VC-05)
qa_status: pass
bug_ids: compozy/compozy#742; compozy/compozy#1097
fix_status: fixed
retest_status: pass
fix_commits: branch fix/issue-742-session-message-cards
evidence:
last_report: docs/qa/reports/2026-10-17-issue-760-session-messaging.md
overlaps: RT-session-message-origin; RT-session-message-reply; ET-web-session-transcript-calm-grammar; ET-web-subagent-card
---

Spec: `.compozy/tasks/agent-collaboration/_uiux.md` S1–S4; copy in `COPY.md` §6 "Session Message
Terms". Automated owners: UT-060…UT-067, E2E-001 (session message card), E2E-002 (queued session
message), E2E-003 (reply round trip), and the `session-messages` and queue stories. This scenario
is the real-provider walk with visual parity against the boards.

1. **Receiver card (VC-01/VC-02).** From session A ("Refactor billing", Claude), have the agent call
   `compozy__session_prompt` on session B with `notify_on_complete: true`. Open B: the message is a
   left-aligned framed card with the Claude mark, `From Refactor billing`, `Reply requested`, and
   the time, between B's own right-aligned operator bubbles and plain assistant text. Click the
   link: A opens in this window; ⌘/Ctrl-click opens a new window. The article is named
   `Message from Refactor billing`.
2. **Card states.** Send a long message (clamped at 176 px; Show more / Show less); a steer into a
   live turn (`Steered`); an interrupt (`Interrupted`); a superseded steer (`Superseded`, body one ink
   step down); rename A (B shows the new title); delete A (`From a deleted session`, no link); send
   from a session in another workspace (folder glyph + workspace name); send with attachments. Each
   matches `agent-collaboration-transcript.html#tr-card`. Rename the other session to a title
   longer than the card (e.g. `Preparar BnUI para migração do AdvOS (tema AdvOS + base-ui 1.8 +
   ícones)`) and narrow the window: on the From, Sent to, and Reply from cards the title ellipsizes
   inside its column, hovering it shows the full title, the chips keep their size, and the reply
   state and time stay whole and never overlap the title (#1097).
3. **Queue row (VC-05).** While B is busy, send a queued message from A: the strip row shows the
   Claude mark and `From Refactor billing`, Steer (aria `Steer message from Refactor billing`) and
   Remove (aria `Remove message from Refactor billing`), never edit. Steer moves it into the live turn. Remove it: the row leaves, and A's sent card resolves to `Dropped`. A deleted sender
   reads `From a deleted session`. Operator rows keep Steer / edit / Remove.
4. **Sender side (VC-04/VC-03).** In A, the call reads `Sending to Billing reviewer` with `Sending…`,
   then `Sent to Billing reviewer` · `Waiting for reply` (pulsing dot), never a "Used session
   prompt" tool row. When B's turn ends, `Reply from Billing reviewer` lands with `Completed` and the
   answer, and the sent card reads `Replied`. Drive failed (danger dot and error body), canceled,
   dropped, unknown, an empty answer (`No reply text.`), and a truncated answer (`Reply truncated ·
   Read the full turn`). A call the daemon refuses (hop limit) reads `Could not send to Billing
   reviewer` with the error and no reply state. Other synthetic wakes still render nothing.
5. **Accessibility.** Cards are articles named `Message from …` / `Reply from …, {outcome}` /
   `Sent to …, {reply state}`; links are keyboard-operable (Enter); outcome and reply state are
   words, never colour alone.
6. **Visual parity.** Capture each state with `eng-ui-screenshot` and compare with the cited VC ids;
   record differences in the QA report.
7. **Provider matrix and reload.** Repeat steps 1, 3, and 4 with a Claude session and a Codex
   session as both sender and receiver: direct and queued, with and without `notify_on_complete`.
   Each transcript shows the cards above, live and after a cold reload, never a `Used session
   prompt` tool row or a `Data event data-compozy-session-message` row. A Claude send may show
   its tool row only until its streamed arguments name the target. A send that a queued prompt
   splits across two assistant messages still shows one card.
8. **After an update.** Update the runtime and app, reopen the app, and repeat step 1: the
   transcript renders with the new release's Web bundle, not a cached shell of the previous one.

QA impact 2026-10-16 (#742): the public walk found every session message rendered as an operator
bubble or a tool row plus a raw data row. Two causes: the desktop renderer reused a cached
`index.html` from before the update (served without `Cache-Control`), booting the pre-#722
bundle; and the daemon's `data-compozy-session-message` part read only the synthetic test shapes,
so Codex sends (MCP `{server, tool, arguments}` envelope, a result naming no tool, an MCP
CallToolResult) never named a target and Claude sends (string `raw_output`) never carried their
delivery or reply watch. The shell is now revalidated and the projection reads both providers;
existing transcripts are reprojected on open (projection version 3). Automated owners added:
`TestSessionMessageUIProjection` (recorded Claude and Codex shapes, live tracker parity),
`TestTranscriptProjectionUpgrade` (version 2 → 3), `TestStaticRoutesServeEmbeddedIndexForRootAndDeepLinks`,
and the Web timeline and thread suites. The real-provider walk in steps 7–8 remains to be run.

7. **Replies and groups (#760).** A message sent with `in_reply_to` shows the `In reply` chip; clicking it
   scrolls to the original when loaded. Three consecutive messages from one sender render as one group
   `Refactor billing · 3 messages` (latest shown, toggle expands). An expired watch reads `No reply` on
   the sent card, with the body `No reply arrived in time. The message may still be pending; check the
   target session.` A session marked Needs attention shows its reason in the list row.

QA impact 2026-10-09 (agent-collaboration tasks 03/05): new in this change; no prior verdict.

QA impact 2026-10-31 (#1097): a long title on an openable party link overflowed the From / Sent to /
Reply from header and rendered under the reply state and time. Step 2 now covers a long title at a
narrow width. Automated owners: E2E-004 in `web/e2e/__tests__/session-messages.spec.ts` (layout), the `long party titles (#1097)` cases in `session-messages.test.tsx` (name and tooltip),
and the `LongTitles` story (360px and the 560px card maximum).

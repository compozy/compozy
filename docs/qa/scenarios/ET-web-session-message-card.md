---
id: ET-web-session-message-card
area: ET
title: See who sent a session message, and the reply it got back
persona: Bruno
journey: J-13-follow-a-live-run
expected: A message one agent session sends another with compozy__session_prompt renders in the receiver's transcript as a left-aligned framed card (the sender's provider mark with no status dot, "From {current title}" as a link, a Steered / Interrupted / Superseded chip only when it arrived that way, "Reply requested" when notify_on_complete was set, the time, and the body with the operator bubble's 176px clamp and Show more), never as a right-aligned operator bubble; a deleted sender reads "From a deleted session" with no link, a renamed one shows its current title, and one in another workspace adds its folder and workspace name; while queued it shows in the queue strip as "From {title}" with the provider mark and Remove only (no Steer or edit); in the sender's transcript the call renders as a "Sent to {title}" card outside the work group (Sending… → Waiting for reply with a pulsing success dot → Replied / Failed / Canceled / Dropped / Unknown, or no reply state when none was requested, or "Could not send to {title}" with the error) and the reply lands as a left-aligned "Reply from {title}" card with the target's mark, an outcome dot and word, and the answer, a fixed sentence for dropped / unknown, "No reply text." when the turn left none, and "Reply truncated · Read the full turn" when cut at 12,000 characters; every surface matches its VC in docs/design/opendesign/agent-collaboration/.
entry_points: web session window transcript (SessionMessageCard, SessionReplyCard, SessionSentCard); composer queue strip (SessionQueueEntryRow); docs/design/opendesign/agent-collaboration/agent-collaboration-transcript.html (VC-01…VC-04) and agent-collaboration-composer.html (VC-05)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
   matches `agent-collaboration-transcript.html#tr-card`.
3. **Queue row (VC-05).** While B is busy, send a queued message from A: the strip row shows the
   Claude mark and `From Refactor billing`, Remove only (aria `Remove message from Refactor
   billing`). Remove it: the row leaves, and A's sent card resolves to `Dropped`. A deleted sender
   reads `From a deleted session`. Operator rows keep Steer / edit / Remove; other actors' rows keep
   none.
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

QA impact 2026-10-09 (agent-collaboration tasks 03/05): new in this change; no prior verdict.

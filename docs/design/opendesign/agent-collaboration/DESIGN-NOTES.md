# Agent collaboration — visual contract

Three boards for [`.compozy/tasks/agent-collaboration/_uiux.md`](../../../../.compozy/tasks/agent-collaboration/_uiux.md) (S1–S5), authored 2026-10-09 as the
spec's task 01 design pass. They gate tasks 03, 05 and 08. Implementation tasks cite them by the VC
ids below. Companion files: `index.html` (map + matrix), `agent-collaboration.css` (domain lane),
`agent-collaboration.js` (demo plumbing), `assets/` (provider marks copied from
`packages/ui/src/logos`).

Status: reference set for Pedro's review. Iterate on these files. Do not regenerate them.

Spec links (behavior authority; these boards own pixels, geometry, signal and copy):

- [`_spec.md`](../../../../.compozy/tasks/agent-collaboration/_spec.md): Part I behavior and Business Rules 6, 9, 17.
- [`_uiux.md`](../../../../.compozy/tasks/agent-collaboration/_uiux.md): surface map S1–S5, component plan, and open design decisions.
- [`_user_stories.md`](../../../../.compozy/tasks/agent-collaboration/_user_stories.md): the ACs and ECs every state above traces to.
- [`_dx.md`](../../../../.compozy/tasks/agent-collaboration/_dx.md): typed metadata the cards read (`origin`, `synthetic`,
  `data-compozy-session-message`, `worktree`).

## How the boards are written

Same grammar as `subagents/`, `automations/` and `session-context/`:

- **Contract block first** (`.sa-contract`): the surface in plain words · production components it
  adds, modifies or reuses, with repo paths · the states on the page with VC ids.
- **Numbered callouts + legend** on the anchor specimen (`.sa-n`, `.sa-legend`), then state grids
  and a tag strip per specimen (`.sa-tags`).
- **Reuse, not copy.** The boards link `../subagents/subagents.css` and `subagents.js` for the lab
  scaffold, `SubagentCard`, `SubagentAvatar`, `HoverCard`, the roster, `StateGlyph` drawing, the
  shared elapsed ticker and the toast. This set only adds what is new.
- **Classes named after production components**: `.session-message*` = `SessionMessageFrame`
  (+ `SessionMessageCard` / `SessionReplyCard`), `.session-sent-card*` = `SessionSentCard`,
  `.session-reply-state` = the sent card's reply slot, `.session-queue-strip*` / `.session-queue-row*`
  = `SessionQueueStrip` / `SessionQueueEntryRow`, `.subagent-branch` / `.subagent-pr-link` /
  `.subagent-hover__facts` = the S5 additions to `SubagentCard`, `SubagentRowTrail` and
  `SubagentHoverContent`. `ac-*` is lab-only.
- **Interactive where the spec defines behavior**: the 176 px clamp with Show more / Show less; the
  sender flow (Sending… → Waiting for reply → the reply card lands and the sent card resolves);
  queue Remove with the 180 ms fade, renumbering and the sender's `dropped` reply; hover cards
  (200 ms, focus-open, Escape); session links (⌘-click = new window) and PR links (external).

## Visual contracts

| VC | Board · section | What it locks |
| --- | --- | --- |
| VC-01 | transcript §02 | S1 card: default, reply requested, steered, interrupted, superseded, sender deleted, renamed, cross-workspace, long/clamped, attachments + skill |
| VC-02 | transcript §01 | S1 rhythm: left-aligned framed card between right-aligned operator bubbles and plain assistant text |
| VC-03 | transcript §03–04 | S2 reply card: completed, no text, truncated, failed, canceled, dropped, unknown, target deleted |
| VC-04 | transcript §03 · §05 | S3 sent card: sending, no reply requested, waiting, replied, steered, failed / canceled / dropped reply, call failed, target deleted |
| VC-05 | composer §01–03 | S4 queue row: session message, sender deleted, dispatching, long title, mixed queue, Remove-only, refusal table |
| VC-06 | subagents §01–02 | S5 card + trail: running isolated, PR open / draft / merged / closed, none, unknown, failed isolated, shared unchanged |
| VC-07 | subagents §01 · §03–04 | S5 hover facts (running, open clean, merged, dirty + none, unknown, git read failed) and the inspector roster |

## Component map (design → production)

| Verb | Component | Path | VC |
| --- | --- | --- | --- |
| new | `SessionMessageFrame` | `web/src/systems/session/components/session-messages/session-message-frame.tsx` | VC-01 · 03 · 04 |
| new | `SessionMessageCard` | `session-messages/session-message-card.tsx` | VC-01 · 02 |
| new | `SessionReplyCard` | `session-messages/session-reply-card.tsx` | VC-03 |
| new | `SessionSentCard` | `session-messages/session-sent-card.tsx` | VC-04 |
| new | `useSessionLabel(workspaceId, sessionId)` | `session-messages/use-session-label.ts` | VC-01 · 03 · 04 · 05 |
| new | clamp hook extracted from `UserMessageBubble` | `web/src/components/assistant-ui/` | VC-01 · 03 |
| modify | `SessionThreadMessage` (origin / synthetic routing) | `assistant-ui/session-thread-messages.tsx` | VC-01 · 03 |
| modify | `session-timeline.logic.ts` (`SessionRow` "session-message") · `tool-labels.ts` | `assistant-ui/` · `systems/session/lib/` | VC-04 |
| modify | `queued-prompt.ts` · `SessionQueueEntryRow` | `systems/session/` | VC-05 |
| modify | `subagent-payload.ts` · `types.ts` · `SubagentCard` · `SubagentHoverContent` · `subagent-format.ts` · `SubagentRowTrail` · `SessionInspectorSubagentsSection` | `systems/session/components/subagents/` | VC-06 · 07 |
| reuse | `Avatar` · `KindIcon` · `StatusDot` · `StateGlyph` · `Time` · `HoverCard` · `Tooltip` · `PropertyRow` · `Button` · `Icon` · `SessionAttachmentGallery` | `@compozy/ui` / session | all |

No new `@compozy/ui` primitive. `ChatMessageBubble` is not used (L-031).

## Geometry

- **Frame (S1, S2)**: grid `24px · 1fr`, column gap 10, padding 9/12/10/9, max-width 560 (`max-w-140`),
  `canvas-soft` + `line-soft`, radius-md, `align-self: flex-start`. Header min-height 24 (aligns with
  the avatar): "From" 12 `subtle` + title 12/510 `fg-strong` link · chips · time mono 10.5 `faint`
  pushed right. Body 13.5/1.6 `fg` (the operator bubble's reading type), indented under the title.
- **Clamp**: 176 px + 28 px bottom mask, 8 px slack, "Show more" 11 `subtle` → `fg` on hover — the
  `UserMessageBubble` rule, extracted and shared.
- **Chips**: 18 px, `badge-fill`, 10.5/500 `muted`, 11 px glyph. Mode glyphs reuse
  `steerMetaGlyph`: corner-down-right (Steered, Superseded), scissors (Interrupted). "Reply
  requested" uses `reply`.
- **Sent card (S3)**: grid `24px · 1fr · auto`, min-height 46 (the subagent card's), arrow-up-right in
  a 24 px round tile (`surface-2` + `line-strong`). Line 2 = first line of the message, 11.5 `muted`.
  Trail: reply state (7 px `StatusDot` + word 10.5) then time.
- **Queue row (S4)**: production row unchanged (min-h 32, pl 14, position, ListPlus). Owner slot =
  18 px provider avatar + "From" + title 11/510 `fg-2`, capped at 44% of the row so the preview keeps
  a share. Verbs: Remove only (24 px icon button).
- **Worktree facts (S5)**: branch mono 10.5 `fg-2` with an 11 px git-branch glyph, middle-truncated
  to ~26 ch, prepended to line 2 with a `·`. PR link 20 px pill, mono 10.5, 12 px state glyph, an
  arrow-up-right reveals on hover/focus to signal "external". Hover facts are a `62px · 1fr` dl under
  the status rows. Isolated roster rows grow to 40 px with a 10 px branch line.

## Signal

| State | Rendering |
| --- | --- |
| Sender identity (S1, S4) | provider mark only; **no status dot** on the sender avatar |
| Reply completed | `StatusDot` success, still · word "Completed" (reply card) / "Replied" (sent card) |
| Waiting for reply | `StatusDot` success, pulsing · "Waiting for reply" |
| Reply failed / call failed | `StatusDot` danger · word + body or line 2 in danger |
| Canceled · dropped · unknown | `StatusDot` faint · word in `subtle` · fixed sentence in `subtle` |
| Mode / Reply requested chips | neutral (`badge-fill`, `muted`) |
| Superseded | chip "Superseded" in `subtle` + body one ink step down (`subtle`) |
| PR open · draft · merged · closed | git-pull-request `info` · git-pull-request-draft `faint` · git-merge `success` · git-pull-request-closed `faint` |
| Dirty files | "{n} changed" in `warning` |

Accent is not used anywhere in this set. Danger appears only for failures.

## Copy (COPY.md register; add under §6 "Session Message Terms")

- S1: "From {title}" · "From a deleted session" · "Reply requested" · "Steered" · "Interrupted" ·
  "Superseded" · "Show more" / "Show less". Article name "Message from {title}".
- S2: "Reply from {title}" · "Reply from a deleted session" · Completed · Failed · Canceled · Dropped
  · Unknown · "No reply text." · "The message was removed from the queue before it ran." · "The
  message may not have been delivered; check the target session." · "Reply truncated · Read the full
  turn".
- S3: "Sending to {title}" · "Sent to {title}" · "Could not send to {title}" · "Sending…" · "Waiting
  for reply" · Replied · Failed · Canceled · Dropped · Unknown. Tool label: Send / Sending / Sent "a
  message".
- S4: "From {title}" · aria "Remove message from {title}". Refusal: "This message was sent by another
  session and can't be edited. Cancel it instead."
- S5: "#731" · aria "Pull request #731, open" · hover labels Worktree · Branch · Base · Commits · PR ·
  Observed · "{n} ahead" · "clean" · "{n} changed" · "#731 open|draft|merged|closed" · "No pull
  request" · "PR status unknown" · "{relative} ago".

## Gaps, interpretations and open questions

1. **PR glyphs are lucide git icons, not `StateGlyph`.** `_uiux.md` maps PR states to `StateGlyph`
   tones, but `StateGlyph` has no PR shapes; reusing running/done marks for PR state would collide
   with the subagent's own status on the same card. The boards keep the spec's tones (info · faint ·
   success · faint) on `Icon` with GitPullRequest / GitPullRequestDraft / GitMerge /
   GitPullRequestClosed. **Decision needed: accept, or force `StateGlyph`.**
2. **Card open control becomes a stretched button.** `SubagentCard` is a `<button>` today; a PR link
   inside it would nest interactive elements. The boards render the card as a container whose title
   button stretches over it (`::after`), with the PR link above it. Same rule for isolated roster
   rows (the subagents roster already flags row/Stop as siblings).
3. **Mode chip on both sides follows S1's rule**: only Steered / Interrupted / Superseded. The sent
   card does not say "Queued", because a `mode: "queue"` call to an idle target starts directly, and
   the part carries no delivery fact.
4. **Superseded** is drawn as a third chip value plus the subdued body. The operator meta line's copy
   ("Superseded by your next steer") is operator-voiced and does not fit an agent-sent message.
5. **Cross-workspace sender** adds a `folder` glyph + workspace name after the link. `_uiux.md` only
   requires the link to resolve; the label makes the cross-workspace reach visible (Open Question in
   `_spec.md`). Drop it if Pedro prefers the link alone.
6. **Renamed sender** shows only the current title; `title_at_send` is not surfaced in the UI.
7. **Reply card for "canceled"** shows "No reply text." when the turn left no assistant text; when it
   did, the body is that text like completed.
8. **Hover "Base"** shows the requested ref with the short sha when they differ (`origin/main ·
   4be1c9d`), and the sha alone when `base_ref` was omitted. `_uiux.md` wrote "branch ← base" on one
   row; two rows fit the 300 px card without truncating the branch.
9. **Queue row** used to render no verbs for other actors' rows (`isQueuedPromptMutable && owner ===
   null`). Session-message rows are the one owned kind that keeps Remove, as Business Rule 17 allows.
10. Window, composer and inspector chrome is drawn lean around the changed surfaces (SD-007). Titles,
    models, PR numbers and times are fixtures.

## Revision log

- 2026-10-09 · first pass: 3 boards, 12 specimen sections, VC-01..VC-07.

# Sessions stability — visual contract

Seven boards for `.compozy/tasks/sessions-stability/_uiux.md` (S1–S10), authored 2026-09-01 before task execution. Implementation tasks 01–08 cite these files as Visual Contracts. Companion boards: `index.html` (map + matrix), `sessions-stability.css` (domain lane, chapters 00–13).

## What changed about how boards are written

Feedback on the earlier sets (agent-comms, command-palette, integrated-terminal, profiles, skill-sources): looking at a finished board it was hard to tell what it represented and which production components it touched. Locked responses, applied to every board here:

- **Contract block first.** Every board opens with `.ss-contract`: (a) what the surface is, in plain words; (b) the production components it **modifies / adds / deletes / reuses unchanged**, each with its repo path; (c) the states on the page, linked, each with its Visual Contract id.
- **Numbered callouts + legend on every specimen.** Inverted `.ss-n` markers sit next to the element; the `.ss-legend` under the specimen names the component, the token values, and the authority (`production` · `spec <id>` · `delta`).
- **Tag strip per specimen.** `state · component · VC · tone/accent` in mono so a reviewer can grep a VC id straight to its specimen.
- **CSS classes named after production components.** `.ss-composer*` = `session-composer*.tsx`, `.ss-queue*` = `SessionQueueStrip`, `.ss-status` = `SessionThinkingRow`, `.ss-transport` = `SessionTransportChip`, `.ss-tool*` / `.ss-group*` / `.ss-fold*` = `SessionLiveToolRow` / `SessionToolGroupRow` / `SessionTurnFoldRow`, `.ss-find*` = `SessionFindBar`, `.ss-trail*` = `SessionMessageTrail`. Lab chrome is `lab-*` / `ss-contract` / `ss-n` / `ss-legend` / `ss-tags` and never product UI.
- **Index carries the surface → board → components → VC matrix.**

## Sources read before drawing

- Competitors (19 references from `_uiux.md`, read in full): t3code `ComposerPrimaryActions`, `ComposerBannerStack` (2s grace), `connection/presentation.ts`, `session-logic.ts` quiet derivation, `MessagesTimeline.logic.ts` folds + shimmer; orca `NativeChatComposerActions` (morph + `event.detail > 1` guard), `NativeChatStructuredSession` (unconfirmed + Retry), `NativeChatWorkingStatus`, `native-chat-live-status.ts` (working includes subagents), `NativeChatToolRun`; synara `ChatView` placeholders + Stop, `ComposerQueuedHeader` / `QueuedComposerActions`, `ThreadDetailHydrationState` (failed ≠ empty), `ToolCallGroupSummaryRow` (≥2 rule, category order), `useSmoothStreamedText` (160ms drain, 2000 ch/s, lerp 0.15, 40ms emit), `ThreadFindBar`, `messageTrail.logic.ts` (opacities .2/.52/.9/1, σ clamp), `_chat.settings.tsx`.
- `packages/ui/src/index.ts` + `tokens.css` (full inventory; see gaps below).
- Production session tree: `session-composer*.tsx`, `session-working-row.tsx`, `session-thread-messages.tsx`, `use-thread-scroll-controller.ts`, `runtime-activity-notice.tsx`, `session-chat-runtime-provider.tsx`, `use-session-page-controls.ts`, `session-live-tail-store-contract.ts`, `use-session-window-sidebar.ts`, `tool-call-card.tsx` + `packages/ui ToolCallRow`, `session-assistant-message.tsx`, `session-user-message.tsx`, `use-session-topbar-slot.tsx`, `session-status-line.tsx`, `session-badge.ts`, `session-timeline-render.tsx`, `session-thread-viewport.tsx`, `scroll-to-bottom-pill.tsx`, `tool-labels.ts`, `styles.css` (`.session-shimmer`).

## Component map (design → production)

| Verb | Component | Path | Board |
| --- | --- | --- | --- |
| modify | `SessionComposer` | `web/src/components/assistant-ui/session-composer.tsx` | composer, queue, connection |
| modify | `SessionComposerActionRow` | `web/src/components/assistant-ui/session-composer-action-row.tsx` | composer |
| modify | `sessionBusyInputLogic` | `web/src/components/assistant-ui/hooks/session-busy-input-store.ts` | composer |
| modify | `useSessionPageControls` | `web/src/hooks/routes/use-session-page-controls.ts` | composer, queue |
| delete | remount-on-cancel generation key | `web/src/systems/session/components/session-chat-runtime-provider.tsx:181` | composer |
| new | badge token `stopping` | `web/src/systems/session/lib/session-badge.ts` | composer |
| new | `SessionQueueStrip` | `web/src/systems/session/components/session-queue-strip.tsx` | queue |
| delete | `SessionComposerQueuedPrompts` | `web/src/components/assistant-ui/session-composer-queued-prompts.tsx` | queue |
| modify | `RuntimeActivityNotice` (queue traces) | `web/src/systems/session/components/runtime-activity-notice.tsx` | queue |
| new | `SessionThinkingRow` | `web/src/systems/session/components/session-thinking-row.tsx` | working |
| delete | `WorkingIndicator` / `SessionWorkingRowView` | `web/src/components/assistant-ui/session-working-row.tsx` | working |
| modify | `AssistantMessage` (no shell before content) | `web/src/components/assistant-ui/session-assistant-message.tsx` | working |
| modify | notice slot precedence (quiet warning) | `web/src/systems/os/apps/session/session-window-content.tsx:133-161` | working |
| new | `SessionTransportChip` | `web/src/systems/session/components/session-transport-chip.tsx` | connection |
| modify | `useSessionTopbarSlot` | `web/src/systems/session/hooks/use-session-topbar-slot.tsx` | connection |
| modify | `ThreadStatePane` (sync-failed) | `web/src/components/assistant-ui/session-thread-states.tsx` | connection |
| modify | `useSessionRuntimeExtensions` (SSE during POST, grace) | `web/src/systems/session/hooks/use-session-runtime-extensions.ts` | connection |
| modify | `useSessionWindowSidebar.disconnected` | `web/src/systems/os/apps/session/use-session-window-sidebar.ts:99` | connection |
| new | `SessionLiveToolRow` | `web/src/systems/session/components/session-live-tool-row.tsx` | timeline |
| new | `SessionToolGroupRow` | `web/src/systems/session/components/session-tool-group-row.tsx` | timeline |
| new | `SessionTurnFoldRow` | `web/src/systems/session/components/session-turn-fold-row.tsx` | timeline |
| modify | `AssistantMessageTimeline` row kinds | `web/src/components/assistant-ui/session-timeline-render.tsx` | timeline |
| modify | `ToolCallStatusIcon` (absorbed failure) | `packages/ui/src/components/custom/tool-call-status-icon.tsx` | timeline |
| modify | `useThreadScrollController` (40px band, composer height, fold suppression, prepend) | `web/src/components/assistant-ui/hooks/use-thread-scroll-controller.ts` | timeline, find-trail |
| modify | `MessageMarkdown` host (smooth reveal, throttled highlight) | `web/src/systems/session/components/message-markdown.tsx` | timeline |
| modify | `SessionUserMessage` meta line | `web/src/components/assistant-ui/session-user-message.tsx` | timeline, queue |
| new | `SessionFindBar` | `web/src/systems/session/components/session-find-bar.tsx` | find-trail |
| new | `SessionMessageTrail` | `web/src/systems/session/components/session-message-trail.tsx` | find-trail |
| new | `lib/transcript-find.ts`, `lib/message-trail.ts`, `lib/tool-group.ts`, `lib/turn-fold.ts`, `lib/scroll-mode.ts` | `web/src/systems/session/lib/` | timeline, find-trail |
| modify | `SessionThreadViewport` (bar slot, trail rail) | `web/src/components/assistant-ui/session-thread-viewport.tsx` | find-trail |
| new | `SessionsSettingsSection` | `web/src/systems/settings/components/sessions-settings-section.tsx` | settings |
| new | `useSmoothStreamingPreference` | `web/src/systems/session/hooks/use-smooth-streaming-preference.ts` | settings |
| reuse | `Button · Kbd · Pill · Spinner · TypingDots · Tooltip · Popover · SearchInput · PillGroup · Switch · Marker · ToolCallRow · CodeBlock · ScrollToBottomPill · ConnectionIndicator` | `@compozy/ui` + web — unchanged | all |

## `@compozy/ui` gap audit (the `_uiux.md` "none expected" claim)

| Need | Verdict | Where this set lands it |
| --- | --- | --- |
| Text shimmer (thinking, live tool row) | **gap in packages/ui** — `animate-shimmer` is a block sweep; the text shimmer exists only as `web/src/styles.css .session-shimmer` | reuse the web class now; promote to a `ShimmerText` primitive (story + test) when a second consumer outside sessions appears |
| Hover card (trail) | **absent** — no `HoverCard`; `Tooltip` is `pointer-events-none` | `SessionMessageTrail` wraps `Popover` with its `anchor` prop and a hover/focus trigger |
| Positioned tick list (trail) | **absent** | new domain component, layout by count only |
| Find bar | composable (`SearchInput` + `Kbd` + `Button icon-xs` + Command rows) | match glaze = `bar-fill`; active = `accent-tint-strong` + `accent-dim` (the `::selection` pair) — no new token |
| Chip / status dot / icon / popover | covered (`Pill`, `StatusDot` (warning/danger/accent/faint only) + `PillDot`, `Icon`, `Popover`) | `SessionTransportChip` = `Pill` info/neutral/danger |
| Badge | **`Badge` does not exist** — the primitive is `Pill` | all annotations say Pill |
| Session badge `stopping` | **missing token** — `session-status-line.ts` maps `stopping → running`, which US-009 forbids | new token: tone warning · ring · pulse · glyph `circle-stop` · attention none |

## State dictionary (copy, verbatim)

Composer primary: `Send` · `Stop` · `Stopping…`. Hint: `⏎ send` / `⏎ steer · ⌘⏎ queue` / `⏎ queue · ⌘⏎ steer` / `⏎ queue` (while stopping, or queue-only). Disposition notes: `Steering — delivered into the live turn` (`injected`) · `Steering — the agent sees it when the current tool finishes` (`pending_injection`) · `Interrupted and replaced — this agent can’t take guidance mid-turn` (`interrupt_fallback`) · `Queued #N — runs after the current turn` (`entry_id`) · `Interrupting — stopping the turn, then running your message` · `Turn ended — sent as your next message instead`. Refusals (`Not sent — …`): turn changed (`active_turn_mismatch`) · turn ended, send normally · session stopped (`session_not_promptable`) · files on steer (`steer_attachments_unsupported`) · queue full 10 of 10 (`queue_full`) · identity reused with other text (`send_conflict`) · disconnected (client, no code).

Status row: `Thinking…` · `Working for {elapsed}` · `· Running {tool}` · `· Running N tools` · `· Waiting for your decision` · `· N agents running` · `Stopping… · {elapsed} · escalates automatically after 10s` · `Stopped by you after {d}` · `Stopped after {d} · the agent didn’t answer the stop, so it was closed for you` · `Stopped after {d} · no work for {n} minutes` · `Failed after {d} · {cause}` · `Quiet for {n}m · stops in {m}m`. Completion race: `Completed · the stop arrived after the turn finished` (faint, 6s).

Queue: header `N queued` / `10 queued · full`; row states `Sending…` · `Not confirmed` + `Retry`; confirm `Remove all N queued follow-ups?` [Clear all] [Keep]; traces `You cleared the queue — N follow-ups removed` · `You removed a queued follow-up — “…”`; meta `From the queue · was #N`.

Transport chip: absent (live) · `Connecting` · `Reconnecting ·n` · `Catching up` · `Paused ·{elapsed}` · `Disconnected`. Panes/markers: `This conversation didn’t sync` + `Try again` · `Live updates stopped at {time} — couldn’t reconnect after N tries. What you see is saved up to then.` · `The conversation was rewound while you were away — showing the current history from turn N.`

Timeline: live row `Running shell — {preview}` / `Running N tools…`; group `Ran N commands, edited M files, read K files` (+ `· 1 failed`); fold `Worked for {d} · {counts}`; open-turn labels `You stopped after {d}` · `Interrupted after {d} · replaced by your steer` · `Failed after {d}`; steer metas `Steered — delivered into the live turn` · `Steered — the agent sees it when the current tool finishes` · `Superseded by your next steer` · `Steered — interrupted and replaced`; late output `The agent sent more output after you stopped it — kept in the log, not in the reply.`; truncation `Showing the first N of M lines · size` [Show all] [Download].

Find: placeholder `Find in conversation`; count `N of M` · `No matches` · `Loading older… · N of M` · `N of M · +1 new`; empty `No matches for “q” in this conversation.`; fold note `opened for a match`. Settings: `Follow-up behavior` (`Steer immediately` | `Queue until the turn ends`) · `Smooth streaming` · `Saving…` · `Couldn’t save Follow-up behavior.`

## Signal & motion rules

- Live = motion (text shimmer 2s linear subtle→fg-strong; TypingDots; ring pulse on the stopping dot). Settled = one step quieter (subtle/faint). Failure = danger **only** when it ended the turn or the live view (turn failure, sync exhausted). Stopped-by-you / interrupted = warning ink on label text only. Reconnecting / catching up = info chip; paused = neutral chip. Queued / unconfirmed = neutral; Retry is the only action-colored control while running.
- Accent budget per screen ≤2: Send disc (idle) or Retry (unconfirmed) + the active find match. Stop is neutral (production disc, danger tint on hover only). Switch-on is accent on the settings panel.
- Reduced motion: shimmer → plain subtle text; spinners static; dots at 70%; morph without transition; reveal off; disclosure without transition. Words carry every state the motion carried.
- Kind ≠ status: kind is the left glyph (production `tool-labels.ts` map); status is motion / ink / trailing glyph. No per-tool colors, chips, or labels under rows.

## Annotated deltas from production (authority)

| Delta | Was | Now | Authority |
| --- | --- | --- | --- |
| Interrupted turn label | `CircleStop` + `text-danger` | `circle-stop` + warning ink, stays open | spec ADR-009 |
| Absorbed tool failure glyph | `XIcon text-danger` | `x` subtle + mono `failed`; danger only for turn-ending failure | spec ADR-009 / US-026.AC-3 |
| Re-arm band | `AT_END_THRESHOLD = 96px` | 40px | spec ADR-007 |
| New-turn anchor | `composerOverlayHeight: 0` | measured composer height | spec ADR-007 / US-024.AC-3 |
| Send/Stop controls | two elements (Send disc, Stop disc + verbs) | one morphing disc + verbs; Stopping pill | spec S1 / ADR-004 |
| Empty assistant shell | mounts on send | mounts with first content; thinking row before | spec ADR-006 |
| Interrupt semantics | wipes the queue | keeps it; Clear all explicit + traced | spec ADR-003 |
| Badge `stopping` | falls back to `running` | own token (warning ring, pulse) | spec US-009 (new) |
| Quiet warning | notification only | notice slot on the session + status row | spec US-014.EC-2 |
| Trail rail inset | `px-4` (16px) | 36px while the trail is present (≥864px, ≥2 sent) | spec S9 (delta) |

## Visual Contract index

| Board | VC rows |
| --- | --- |
| `sessions-stability-composer.html` | task_01 VC-01 (§04 injected) · VC-02 (§04 pending) · VC-03 (§04 fallback) · VC-04 (§04 queued) · VC-05 (§05) · task_02 VC-01 (§06) · VC-02 (§07) |
| `sessions-stability-queue.html` | task_03 VC-01 (§01) · VC-02 (§02) · VC-03 (§03) · VC-04 (§04) · VC-05 (§05) · VC-06 (§06) |
| `sessions-stability-working.html` | task_05 VC-01 (§05) · task_07 VC-10 (§01) · VC-11 (§02–03) · VC-12 (§04) |
| `sessions-stability-connection.html` | task_06 VC-01 (§02) · VC-02 (§03) · VC-03 (§04) · VC-04 (§05) · VC-05 (§06) |
| `sessions-stability-timeline.html` | task_07 VC-01 (§02 single) · VC-02 (§02 parallel) · VC-03 (§03) · VC-04 (§04) · VC-05 (§05) · VC-06 (§06) · VC-07 (§07) · VC-08 (§09) · VC-09 (§12) |
| `sessions-stability-find-trail.html` | task_08 VC-01 (§01) · VC-02 (§02) · VC-03 (§03) · VC-04 (§05) · VC-05 (§06) |
| `sessions-stability-settings.html` | task_01 VC-06 (§01–02) |

Capture at 1440×900 per the task tables; the specimen windows are 860px wide inside the stage, so a capture crops to the `data-od-id` of the window (`*-win`) or the flush component.

## Fixture (one story across the set)

Session `dev-loop` · Claude Code (`claude-code`) · `~/Dev/compozy` · turn `t_9f2`. Turn 1 (settled): "Refactor the flaky manager tests" → Worked for 4m 12s · Ran 6 commands, edited 2 files → "Done — 14 tests updated…". Turn 2 (live): "Now make the same change in the store package" → shell `go test ./internal/store/... -run Retry` → steer "Only touch the lifecycle tests, skip the store package". Queue: `#1 Ship it with tests` (you, `inp_4d8`), `#2 Also update the changelog…` (agent `reviewer`, `inp_4e1`), `#3 Bump the version and tag the release`. Children: `reviewer`, `scout`. Identities: `msg_01k4 · idk_1f77 · inp_4d9`.

## Open items for the spec author

- The trail rail is omitted from the composer/queue/working/connection/timeline windows to keep each subject in focus; production renders it on every qualifying window — decide whether captures for those VC rows run with the rail on.
- `ShimmerText` promotion to `packages/ui` is deferred until a second consumer exists; the sessions surface keeps `web/src/styles.css .session-shimmer`.
- `StatusDot` lacks `success`/`info` tones; the transport chip uses `Pill` and needs nothing from `StatusDot`.

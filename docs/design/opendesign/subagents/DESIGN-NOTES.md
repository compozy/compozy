# Subagents — visual contract

Three boards for `.compozy/tasks/subagents/_uiux.md` (S1–S10) and `adrs/adr-005.md`, authored 2026-10-08 after the spec and before `cy-create-tasks`. Implementation tasks should cite these files as Visual Contracts by the VC ids below. Companion files: `index.html` (map + matrix), `subagents.css` (domain lane), `subagents.js` (demo plumbing), `assets/` (provider marks copied from `packages/ui/src/logos`).

Status: reference set for Pedro's review. Iterate on these files. Do not regenerate them.

## How the boards are written

Same grammar as `automations/` and `session-context/`:

- **Contract block first.** `.sa-contract`: what the surface is in plain words · the production components it adds / modifies / reuses, each with its repo path · the states on the page, linked, each with its VC id.
- **Numbered callouts + legend** on the anchor specimen of each board (`.sa-n`, `.sa-legend`), then state grids.
- **Tag strip per specimen** (`.sa-tags`): state · component · VC · tone/accent budget.
- **CSS classes named after production components**: `.subagent-card*` = `SubagentCard`, `.subagent-group*` = `SubagentGroup`, `.hover-card*` = the new `HoverCard` primitive, `.subagent-hover*` = `SubagentHoverContent`, `.subagent-waiting-banner*` = `SubagentWaitingBanner`, `.subagent-chip*` = `SubagentChip`, `.subagent-roster*` = `SessionInspectorSubagentsSection`, `.session-list-row*` / `.thread-toggle` = `SessionListRow` / `SessionListThread`, `.tool-call-row*` = `ToolCallRow`, `.session-continue-divider*` = `SessionContinueDivider`, `.state-glyph` = `StateGlyph` (drawn from `[data-sg]` with production geometry). Lab chrome is `lab-*` / `sa-*` and is never product UI.
- **Interactive where the spec defines behavior.** Hover card timing (200 ms open, focus-open, Escape), group disclosure, provider-native expansion, banner Stop (pending, success, failure), roster row Stop (pending, success, failure), paging, and one shared 1 s elapsed ticker that writes `textContent`.

## Sources read before drawing

- Spec: `_uiux.md`, `adr-005.md` (sidebar chip, inspector roster, lifecycle).
- Production: `packages/ui` `state-glyph.tsx`, `status-dot.tsx`, `tool-call-row(-parts).tsx`, `kind-icon-registry.ts`, `logos/claude.tsx`, `logos/openai.tsx`; `web/src/systems/session/components/session-list/session-list-row.tsx`, `session-list-thread.tsx`, `session-continue-divider.tsx`, `session-inspector-section.tsx`, `session-badge-mark.tsx`.
- Reference: `.resources/t3code/apps/web/src/components/chat/V2LifecycleRow.tsx` (`SubagentAvatar`, `SubagentTimelineLink`), plus the files `_uiux.md` cites per surface.

## Component map (design → production)

| Verb | Component | Path | Board | VC |
| --- | --- | --- | --- | --- |
| new | `HoverCard` · `HoverCardTrigger` · `HoverCardContent` | `packages/ui/src/components/ui/hover-card.tsx` (+ story, test) | transcript, nav | transcript VC-04 |
| new | `SubagentAvatar` · `SubagentElapsed` · `SubagentCard` | `web/src/systems/session/components/subagents/` | transcript | VC-01 · VC-02 · VC-05 |
| new | `SubagentGroup` | same | transcript | VC-03 |
| new | `SubagentHoverContent` | same | transcript, nav | VC-04 |
| new | `SubagentWaitingBanner` | same | composer | composer VC-01 · VC-02 |
| new | `SubagentChip` | same | nav | nav VC-01 · VC-02 |
| new | `SessionInspectorSubagentsSection` | same | nav | nav VC-01 · VC-03 |
| new | `useSubagentRoster(sessionId)` | `web/src/systems/session/hooks/` | all | — |
| modify | `session-timeline-work.ts` (fold exemption, `parentToolCallId` nesting) | `web/src/components/assistant-ui/` | transcript | VC-01 · VC-05 |
| modify | `tool-labels.ts` (subagent verb family; delegate row suppressed behind its card) | `web/src/systems/session/lib/` | transcript | VC-06 |
| modify | `SessionContinueDivider` · `session-origin.ts` | `web/src/systems/session/` | transcript | VC-07 |
| modify | `SessionComposer` (strip slot) | `web/src/components/assistant-ui/session-composer.tsx` | composer | VC-01 |
| modify | `session-working-status.ts` (count) | `web/src/systems/session/lib/` | composer | VC-03 |
| modify | `SessionListRow` · `SessionListThread` · `session-hierarchy.ts` · list query | `web/src/systems/session/` | nav | VC-01 · VC-02 |
| modify | `SessionInspector` (mount + deep link) | `session-inspector.tsx` | nav | VC-01 |
| backend | `subagents=include|exclude|only` on the session list (default `include`) | API · CLI · UDS | nav | — |
| reuse | `StateGlyph` · `StatusDot` · `KindIcon` · `AvatarGroup` · `AvatarGroupCount` · `Collapsible` · `ToolCallRow` · `PropertyRow` · `Button` · `Time` · `formatDuration` | `@compozy/ui` | all | — |

## Signal (resolves the spec's provisional table)

`_uiux.md` says "token names follow `tokens.css`; the design pass confirms exact names." The boards follow the shipped `StateGlyph` canon (`state-glyph.tsx` header), not the spec's draft, wherever the two differ:

| State | Glyph | Avatar dot (`StatusDot`) | Word |
| --- | --- | --- | --- |
| queued | `queued` (dashed ring, `--color-indicator`) | hollow ring, indicator | Queued |
| running | `running` (mint ring, `--color-success`) — **spec draft said info** | `success`, pulsing | Running |
| waiting for you | `attention` (accent dot) | `accent` | Waiting for you |
| completed | `done` (mint check) | `success`, still | Completed |
| failed | `failed` (danger ×) | `danger`; line 2 + word `danger` | Failed |
| canceled · interrupted | `stopped` (subtle square) | `faint` | Canceled · Interrupted |
| parent waiting on subagents | `delegated` (info ring + dot) | — | — |

Running and completed share mint. They differ by motion (pulse vs still), by the word, and by elapsed ticking vs frozen. State is never shown by colour alone. Accent appears only for "Waiting for you". Danger appears only for failures.

## Geometry

- **Card**: grid `24px · 1fr · auto · 14px`, gap 10, padding 7/10/7/9, min-height 46, max-width 560, `canvas-soft` + `line-soft` hairline, radius-md; hover `surface-2` + `line`. Title 12/510 `fg-strong`; word 10.5 `subtle`; line 2 11.5 `muted`; elapsed mono 10.5 tabular. Inside a group panel, cards lose their frame.
- **Avatar**: 24px round, `surface-2` + `line-strong`, 13px mark, 8px dot bottom-right on a 2px ring matching the host surface. 20px variant in the chip preview and the roster.
- **Group**: same trigger height as a card; avatars overlap by 7px; max 3 + `+N`; collapsed + all settled → opacity .62, restored on hover/focus.
- **HoverCard**: 300px, `elevated` + `line-strong` + `shadow-overlay`, radius-md, padding 10/12. Opens in 200 ms, closes in 120 ms.
- **Banner**: docks on the composer's top edge like `SessionQueueStrip` (10px inset, radius-lg top, no bottom border), two lines: title 12/510, names 11.5 `subtle` with `fg-2` underlined name buttons; ghost sm Stop.
- **Chip**: 20px pill, mono `text-micro` tabular, 12px glyph, sits before `ThreadToggle` in the row's trailing slot.
- **Roster**: rows 30px, 20px avatar, bounded at 316px; Stop is a 22px icon button that replaces elapsed on hover or focus.

## Copy (COPY.md register; nouns subagent · session · agent, never "thread")

- Card words: Queued · Running · Waiting for you · Completed · Failed · Canceled · Interrupted. Model fallback: Not reported.
- Group: "3 subagents" · "2 working · 1 done · 1 failed" (plus "N needs you" when one is waiting — derived, see open questions).
- Divider: "Subagent of" + parent name · "Open parent" · "Subagent of a deleted session".
- Banner: "Waiting on subagent <title>" · "Waiting on 3 subagents" · "and N more" · Stop · Stopping… · toast "Could not stop subagents."
- Tool rows: Check/Checking/Checked "subagent capabilities" · Delegate/Delegating/Delegated "a subagent" · Read/Reading/Read "subagent status" · Cancel/Canceling/Canceled "a subagent" · "… 2 times" · "Tried to check subagent capabilities".
- Chip aria: "3 of 10 subagents running" · "10 subagents, 2 failed". Hover preview foot: "+N more".
- Inspector: "Subagents" · "Subagents · N running" · "Previous subagents (N)" · "Show 12 more" · "Stop subagent" · toast "Could not stop subagent".

## Gaps, interpretations and open questions

Items 1–5 were decided by Pedro on 2026-10-09. The boards were updated to match.

1. **Failed rows pin above the live list** (S10). Order in the section: failed, then attention, then
   running, then the collapsed `Previous subagents (N)` group (completed, canceled, interrupted). The
   `N failed` accessory on that trigger is gone, because failed rows are no longer inside the group.
2. **Group summary with a waiting subagent: yes.** It adds `N needs you`. The tone stays `info`, and
   `danger` still wins when any subagent failed.
3. **Banner when only provider-native subagents are live: hidden.** The banner and its Stop cover
   only delegated subagents.
4. **Chip when nothing is live but a failure remains: total only** (`6` beside the failed glyph). A
   subagent waiting for you counts as live. `0/6` read as "0 of 6 running" and drew the eye to a zero. The aria
   label is unchanged (`6 subagents, 2 failed`). The chip still reads `live/total` while anything
   runs, and hides once everything has settled cleanly.
5. **Chip hover footer: `+N more` only.** "Click to see all" is dropped: the chip is a button that
   opens the inspector, COPY.md avoids helper prose, and the hint duplicated the click affordance.
6. **Roster row semantics.** The board draws the row as one `role="button"` holding the Stop button. Production must render the row button and Stop as siblings (as `SessionListRow` does with its actions) so interactive elements are never nested.
7. The window, composer and sidebar chrome are drawn lean around the changed surfaces. Production host chrome (runtime selector with `IntensityMeter`, topbar slots) is lossy by nature (SD-007).
8. Model names, titles, counts and times are fixtures. Runtime truth owns values.

## Revision log

- 2026-10-08 · first pass: 3 boards, 13 specimen sections, 7 + 3 + 3 VC rows.
- 2026-10-09 · Pedro's answers to gaps 1–5. Navigation board: failed roster rows moved above the live list in nav VC-01 and VC-03, and the `Previous subagents` counts updated; the chip with no live subagents shows the total only (nav VC-02); "Click to see all" removed from the chip hover.

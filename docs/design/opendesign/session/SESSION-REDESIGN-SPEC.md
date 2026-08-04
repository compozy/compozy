# Session Transcript Redesign — Implementation Spec

Status: prototype delivered, awaiting review
Prototypes: `docs/design/opendesign/session/session-design-system.html` (vocabulary + rules) · `docs/design/opendesign/session/session-chat.html` (applied) · `docs/design/opendesign/session/session.css` (canonical CSS)
References analyzed: `.resources/t3code` (MessagesTimeline lineage) · `.resources/synara` (MessagesTimeline + toolCallGroup lineage) — full annotated path index in §10; build order and porting contracts in §11
Token authority: `packages/ui/src/tokens.css` → `docs/design/opendesign/design-system/ds-core.css` (copied verbatim into `docs/design/opendesign/session/session.css`)

## 1. Thesis

The session transcript is the product's center of gravity and must disappear behind the
conversation. Both references converge on the same discipline, adopted here wholesale and
mapped onto Compozy tokens:

1. **One column (46rem)** shared by transcript and composer; the transcript owns no surface.
2. **Text first** — assistant is naked markdown; user is a borderless 4.5% ink block; no
   avatars, role labels, borders, or shadows between messages.
3. **One neutral ramp** (`--fg → --muted → --subtle → --faint`) carries all hierarchy.
4. **Status is a glyph, never a badge** — grey success check, red × on failure, grey spinner.
   Row text never changes color on failure.
5. **Collapse is the resting state** — settled runs collapse to a semantic summary line;
   settled turns fold behind "Worked for Ns" (the only divider); only the live tail is open.
6. **Decisions leave the transcript** — permission/clarification dock to the composer;
   the transcript keeps one-line receipts.
7. **Color budget (exhaustive):** danger × glyph · diff numerals · `--info` links · running
   accent dot (goal strip, todo active, window head) · composer focus. Tinted backgrounds
   (`*-tint` washes) are banned inside the transcript.

## 2. Component contracts (new vocabulary)

All classes live in `docs/design/opendesign/session/session.css`. Production equivalents go to `packages/ui`
(generic) or `web/src/systems/session` (domain), per the reuse-before-create rule.

| Class | Contract |
|---|---|
| `.chat-col` | 46rem centered column; used by transcript, banner, dock, composer |
| `.t-row` / `.t-row--turn-end` | 7px in-turn / 18px between-turn rhythm; no other margins |
| `.msg-user__bubble` | right-aligned, max-w 80%, `--chat-fill-user` (4.5% ink), radius-lg, **no border**; clamp at 176px with mask + "Show more" |
| `.msg-assistant.md` | naked markdown, full column, `--fg`, 13.5px/1.62 |
| `.msg-meta` | hover-revealed timestamp · copy; terminal message of settled turn only |
| `.trow` | 24px line: 18px icon well · verb (500, `--muted`) · mono preview (`--subtle`) · optional `.diffstat` · chevron · status glyph; hover brightens verb, `--row-hover` fill only when expandable |
| `.trow__detail` | expanded body: `margin-left: 25px`, 1px `--line` rail, mono 11px `--muted`, max-h 240px; diff/stderr colored **text**, no tinted blocks |
| `.trow__artifact` | retained-result affordance inside `.trow__detail` (`ToolResultArtifact`): the truncated preview stays in the mono pre; below it a quiet `--info` text action "Open full result" with faint mono byte meta; content loads paginated **in place** ("Load more" + byte progress); load failure = danger text line + "Retry" — never an Alert card |
| `.tgroup-sum` | collapsed run: first entry's icon + "Ran 2 commands · Edited 3 files" (middle-dot join, fixed order Ran/Edited/Read/Searched/Used; distinct-file counts) + chevron |
| `.tmore` | bare-text overflow toggle "+N previous tool calls", indented 25px |
| `.turnfold` | "Worked for {turnDuration}" 12px `--subtle` + chevron; **the only border-bottom in the transcript**; `--stopped` variant = danger text line, never folds |
| `.thinking-live` / `.thinking` | live: shimmer text label only; settled: `.trow` grammar with "Thought" verb + indented muted body |
| `.working` | three 4px dots (stepped 2s duty cycle) + "Working for Ns" 11.5px, ticker mutates text node |
| `.marker` | 12px tone glyph + one muted sentence + optional mono meta; replaces every in-transcript Alert |
| `.liferule` | centered mono caption between hairlines (resume, compaction) |
| `.banner` | session-level failure only, above transcript: 28% hairline + 4% wash + actions; budget: one |
| `.dock` | decision panel fused to composer top (shared border, top radius); eyebrow tracking .14em, mono pre on `--chat-fill-code`, actions: primary allow · always · spacer · danger-ghost reject with `.dock__menu` split ("Reject always"); keys 1–4 map to the four ACP decisions; optional `.dock__deadline` static mono hint ("times out 14:32"); submitting / retryable-error render as quiet dock status lines |
| `.choice__row` | clarification options, 30px rows with `.choice__key` shortcut chips |
| `.choice__free` | clarification free-text answer (production renders it when `choices` is empty): quiet textarea on the dock body + "Send answer"; Enter submits, Shift+Enter breaks line |
| `.receipt` | one-line decision record: tone glyph + sentence; allowed decisions also get one |
| `.goalstrip` | 24px line above transcript: state dot (accent pulse active · warning blocked/paused · success done · faint moved) + GOAL kicker + objective + `turn 3/20 · ctx 41%` mono facts + chevron → quiet key/value body: Contract · Run link · Context (tokens + nudge threshold) · Last verdict · Node/cause mono · Session link (moved goals) · Actions ("Draft goal command" / "Draft replacement" prefill) |
| `.changed` | "Edited N files +A −D" line → bare mono file lines behind the rail; cap 8 + "+N more" |
| `.todo` | TodoWrite renderer: caption "Plan · 2 of 4" + rows (done: grey check + line-through · active: accent pulse dot · pending: hollow dot) |
| `.composer` | redesigned send box: hairline `--line` border (hover `--line-strong`, focus `--accent-dim`), elevated fill, radius-lg, no zone top-border (scroll fade separates); bar = runtime selector (`.rtsel`, left) · quiet `kbd ⏎` hint · spacer · busy-phase `Queue`/`Interrupt` ghost buttons · ghost circular stop · circular accent send (28px). While a turn runs the hint reads `⏎ queue` and Enter stages the draft (SessionComposer busy contract). Decision shortcuts render as kbd chips on dock buttons, not in the composer hint |
| `.qstrip` / `.qrow` | queued follow-ups fused onto the composer top (`SessionComposerQueuedPrompts`): elevated fill, hairline border (squared under a `.dock`), 32px rows — queue glyph · single-line preview (`queuedPromptPreview` rules: first non-empty line, markdown markers stripped, fences → "Code block") · Steer (inject into running turn) · edit (back into field, never clobbers a live draft) · remove. No accent, no fills — bookkeeping, not an event |
| `.rtsel` / `.rtpop` | runtime selector, **variation A (flat palette)** from `runtime-selector-variations.html`: compact 28px `.pmr` trigger (provider glyph · model name · 7-bar `.im` meter · fast bolt · chevron) on the composer bar left; 320px popup opens above — search+refresh · provider chip line (32px) · grouped model rows (favorites on-row) · footer `.rzline` with quiet reasoning slider (`.rz`, 16px track, accent fill only) + ACP speed switch (`.spdsw`, PR #267, disabled when the adapter lacks the capability) |
| `.goal-zone` | goal strip mounts ABOVE the scroll viewport (pinned context, like `SessionGoalHeader`), never inside the scroller — the top scroll fade must not mask it |
| `.md` / `.codeblock` | compressed heading scale (17→13.5px), inline code on `--badge-fill`, fenced on `--chat-fill-code` with quiet header, `--info` links, row-separator tables |
| `.disclose` | the single disclosure motion: grid-rows 0fr→1fr + opacity, 220ms, reduced-motion safe |

## 3. Migration map (current → new)

Delete-targets are explicit per Zero Legacy Tolerance — no dual rendering paths.

| Current (file:line) | Action |
|---|---|
| `web/src/components/assistant-ui/session-thread.tsx:89` UserMessage `rounded-xl border bg-canvas-soft` | restyle → `.msg-user__bubble` (drop border, 4.5% fill, meta hover-only) |
| `packages/ui/src/components/custom/tool-call-row.tsx:90` + `web/src/systems/session/components/tool-call-card.tsx:78` | retone to `.trow`; **delete** `defaultExpanded \|\| status==="failed"` (`web/src/systems/session/components/tool-call-card.tsx:123`); delete danger icon-well tint; success check → `--subtle` |
| `web/src/components/assistant-ui/session-timeline-render.tsx:176` work eyebrow "N tool calls" | replace with `.tgroup-sum` semantic label; settled cluster = summary line (not "last row + toggle") |
| `web/src/components/assistant-ui/session-timeline.logic.ts:160` `SETTLED_WORK_VISIBLE_LIMIT` | settled runs render as collapsed `.tgroup-sum`; `ACTIVE_WORK_VISIBLE_LIMIT=4` keeps last-4 live tail + `.tmore` |
| `web/src/components/assistant-ui/session-timeline-render.tsx:115,214` toggle/fold buttons | keep logic; retone to `.tmore` / `.turnfold` |
| `web/src/systems/session/components/thinking-block.tsx:25` | drop icon well + "N updates" eyebrow; live = `.thinking-live` shimmer; settled = `.thinking` row; keep auto-open/auto-collapse behavior |
| `web/src/components/assistant-ui/session-working-row.tsx:43` | keep; retone to `.working` (4px dots, stepped pulse) |
| `web/src/systems/session/components/runtime-activity-notice.tsx:152–253` (5 Alert kinds) | replace with `.marker` lines; **delete** kind Pills; cluster consecutive same-kind markers with ×N count |
| `web/src/components/assistant-ui/session-message-parts.tsx:17` SessionDataEventCard | replace with `.marker` + mono meta; **delete** the `<pre>` preview card |
| `web/src/components/assistant-ui/session-thread.tsx:61` SessionMessageErrorNotice (`border-danger/30 bg-danger/8`) | replace with danger `.marker`; delete off-token alphas |
| `web/src/systems/session/components/session-resume-failure.tsx:41` | restyle to `.banner` (28%/4%); **delete** AlertMeta pills + MonoId pairs — path in body text |
| `web/src/systems/session/components/permission-prompt.tsx:69` sticky prompt | **delete sticky positioning**; move to `.dock` in `web/src/systems/os/apps/session/session-window-content.tsx` composer zone; 3 visible buttons + reject split menu — **all four ACP decisions stay reachable** (allow-once, allow-always, reject-once, reject-always; keys 1–4 fire directly, menu open or not); buttons render only for the `decisionOptions` the runtime offers |
| `web/src/systems/session/components/permission-prompt.tsx:242` PermissionRejectedNotice | replace with `.receipt`; **add** allowed-receipt (today allowed renders null) |
| `web/src/systems/session/components/clarification-card.tsx:23` / `web/src/systems/session/components/clarification-receipt.tsx:27` | card → `.dock` variant: `.choice__row` bounded choices **or** `.choice__free` free-text form (production shows free-text when `choices` is empty); deadline hint → `.dock__deadline` static mono; submitting/error → quiet dock status lines; receipt → `.receipt` |
| `web/src/systems/session/components/goal/goal-status-chip.tsx:93` GoalStatusChip | **delete** (icon tile, 2 pills, meter grid, moved strip, 5-button bar) → `.goalstrip`; meters → mono facts + body Context row (tokens + nudge); Pause/Resume/Approve/Clear → window-head **state-gated** actions (same visibility gates as `goal-status-controls.tsx:54–66`); Prefill draft/replacement → goalstrip body Actions row (stages the `/goal` command into the composer); moved strip + Active-session link → `data-state="moved"` variant with body Session link; Open-run link → body Run row |
| `web/src/systems/session/components/tool-result-artifact.tsx:21` ToolResultArtifact (preview + "Open full result" + pagination) | restyle → `.trow__artifact` inside `.trow__detail`; keep the `compozy://tool-artifacts/` fetch, byte progress, and Load-more pagination verbatim; **delete** the danger Alert on load failure → danger text line + Retry |
| `web/src/components/assistant-ui/session-timeline-render.tsx:286` GoalPromptNotice | replace with `.marker--goal` line |
| `web/src/systems/session/components/goal/session-goal-header.tsx:31` error bar (`bg-danger-tint` full-bleed) | replace with `.banner` |
| `web/src/components/assistant-ui/session-changed-files-row.tsx:54` card | replace with `.changed` line + file lines |
| `web/src/systems/session/components/tool-renderers/generic-content.tsx:24` TodoWrite JSON dump | **add** `.todo` renderer for TodoWrite/plan tools; generic JSON only inside `.trow__detail` |
| `web/src/systems/session/components/tool-renderers/edit-content.tsx:19` stacked tinted CodeBlocks | unified diff inside `.trow__detail`, colored text lines only |
| `web/src/systems/session/components/tool-renderers/bash-content.tsx:17` stderr `tone="danger"` block | mono detail; error lines in danger **text**; delete ring/tint |
| `web/src/systems/session/components/tool-renderers/search-content.tsx:11` bordered result list | bare mono lines in the detail rail |
| `packages/ui/src/components/custom/code-block.tsx:316` `tone` full-block recolor | **delete** tone text-recolor path (keep density/copy) |
| `packages/ui/src/components/alert-variants.ts:24-27` tinted variants | transcript stops importing them; audit remaining callers before deletion |
| `web/src/components/assistant-ui/session-composer.tsx:181` bar layout | keep queue/interrupt/steer logic verbatim; retone chrome to `.composer` grammar. **Authorized delta (truthful UI):** the composer bar ships WITHOUT the runtime selector — the daemon exposes no session-level runtime state or change surface (`SessionPayload` carries no model/reasoning/speed; no session-update operation; `speed` exists only on `createSession`), so a bar selector would render a control the runtime doesn't support. The variation-A selector lands on the surfaces whose contracts back it (session create dialog, agent settings); the bar mount returns when the daemon exposes session runtime state |
| `web/src/components/assistant-ui/session-composer-queued-prompts.tsx:35` strip | retone to `.qstrip`/`.qrow`; keep steer/edit/remove semantics and `queuedPromptPreview` |
| `web/src/systems/runtime/components/runtime-selector/*` popup | replace anatomy with variation A (`runtime-selector-variations.html`): 528×520 → 320×~400, provider rail → chip line, reasoning select → `.rz` slider + `.spdsw` speed switch |

## 4. Data truths (what a row may claim)

From `UIMessagePart` / `SessionTimelineToolPart` / `ToolUseResult`:

- Collapsed row: verb tense (`getToolLabel`), one input summary (`getToolCompactSummary`),
  5-state status, per-file `+a/−d` (aggregateChangedFiles), line counts (Read/Bash), truncation flag.
- **Per-tool duration does not exist** — never render it. `turnDurationMs` feeds the fold row only.
- Group labels count **distinct files** for Edited/Read (entryFileKeys semantics).
- Goal facts from the goal snapshot: `turns_used/turn_limit`, `context.ratio`, `last_verdict.outcome`.
- Markers consume `AgentEventPayload.marker/runtime/failure`; the raw kind string renders as
  mono meta, never as a pill.

## 5. Grouping rules (logic changes)

`web/src/components/assistant-ui/session-timeline.logic.ts` already clusters consecutive tool parts per turn — keep, plus:

1. **Semantic summary** — `summarizeToolGroup(entries)` → category counts, fixed order,
   middle-dot join (new, port of `.resources/synara/apps/web/src/components/chat/toolCallGroup.logic.ts`).
2. **Collapse on settle** — a run collapses the moment it settles, even mid-turn; only the
   live tail run stays expanded.
3. **Narration breaks runs** — reasoning/data rows flush the cluster (already true — keep).
4. **Marker clustering** — consecutive same-kind data rows merge into one marker with ×N.
5. **Receipts are persistent rows** — permission/clarification outcomes stay outside turn
   folds (`isPersistentTurnRow` already does this — keep).
6. Update `web/src/components/assistant-ui/timeline-row-estimates.ts` (trow ≈ 26px, marker ≈ 24px) and keep
   `isRowUnchanged` in sync with new row fields.

## 6. Behaviors

- **Failed rows stay collapsed**; failure is visible via glyph + error-first-line preview.
- **Interrupted turns never fold**; next turn re-folds nothing retroactively.
- **Meta on hover only**, terminal messages only; mid-turn commentary never shows meta.
- **Ticker discipline**: elapsed labels mutate text nodes (no per-second React commits).
- **Reduced motion**: shimmer → static subtle text; dots static; disclosures instant.
- **Keyboard**: 1–9 decide dock choices; Esc dismisses banner; fold/disclose are buttons
  with `aria-expanded`.
- **Busy composer**: while a turn runs, Enter queues the draft (never submits); Queue and
  Interrupt stay on the bar; Steer lives on queue rows only — steering injects the entry
  into the running turn as a user message. Dock digit shortcuts ignore focused inputs.
- **Runtime selector**: speed is sticky across model switches but collapses
  to `normal` when the next adapter doesn't advertise the capability; selecting a model
  resets `reasoning_effort` to `""` (provider default, hollow meter). Lives on the
  create-contract surfaces (session create dialog, agent settings) — see the §3
  composer-bar authorized delta.
- **Goal controls are state-gated window-head actions**: active → Pause · paused → Resume ·
  needs-approval → Approve (primary) · terminal snapshot → Clear (danger ghost). The head never
  shows more than one goal action at a time (production gates are mutually exclusive —
  `goal-status-controls.tsx:54–66`). Prefill actions live in the goal-strip body and stage the
  `/goal` command into the composer — never auto-send.
- **Permission keys**: 1 allow-once · 2 allow-always · 3 reject-once · 4 reject-always —
  key 4 fires even while the reject menu is closed.
- **Clarification**: bounded choices answer on keys 1–9; with no choices the dock renders the
  `.choice__free` form (Enter submits, Shift+Enter breaks). The deadline hint is static text —
  it never ticks.
- **Full tool results** load in place: "Open full result" streams pages under the same detail
  rail; byte progress is faint mono meta; failure is a danger text line with Retry.

## 7. Web/Docs impact

- `web/`: `web/src/systems/session/components/*` (major), `web/src/components/assistant-ui/*` (major),
  `packages/ui` (`tool-call-row`, new `marker`/`receipt`/`dock` primitives — story + test each),
  `web/src/systems/os/apps/session/session-window-content.tsx` (dock placement, goal strip, window-head goal controls).
- Docs: `packages/site` session screenshots will be stale after migration; refresh captures.
- QA: user-visible change → add `untested` scenarios for transcript rendering, permission
  dock flow, goal strip, and reset existing session-view scenarios to `untested`.

Compozy Impact Audit:

- Native tools: no impact — rendering only; tool IDs, schemas, descriptors, capability gates unchanged (checked: labels come from `web/src/systems/session/lib/tool-labels.ts` presentation layer).
- Extensibility and hooks: no impact — ACP/UIMessagePart wire shape untouched; MCP/generic tools render through the same `.trow` + detail path (checked: `toTimelineParts`, `session-data-renderers`).
- Workspace data isolation: no impact — no new data reads; all rows derive from the session's own transcript stream (checked: session-scoped queries only).
- Official Compozy skill: no impact — no CLI/HTTP surface or behavior change; visual layer only.

"Rendering only" is a wire claim, not a capability claim: §8 is the exhaustive proof that no
user-visible action is silently removed by this redesign.

## 8. Coverage matrix

Every user-visible session capability the redesign touches, with its explicit destiny.
Decisions: **restyle** (same capability, new grammar) · **relocate** (same capability, new
surface) · **keep** (unchanged semantics, retoned chrome at most) · **drop** (removed on
purpose, reason given). Prototype: `chat` = demonstrated in `session-chat.html` ·
`DS §NN` = demonstrated in `session-design-system.html` · `vocabulary-only` = specified +
styled but not required in the applied scenario.

### Goal autonomy

| Capability (production source) | Decision | Destination | Prototype |
|---|---|---|---|
| Pause goal (`goal-status-controls.tsx:88`) | relocate | window-head action, shown when `live && status === "active"` | chat (head) |
| Resume goal (`goal-status-controls.tsx:99`) | relocate | window-head action, `live && paused` | DS §09 head states |
| Approve goal (`goal-status-controls.tsx:110`) | relocate | window-head primary action, `run_status === "needs-approval"` | DS §09 head states |
| Clear goal (`goal-status-controls.tsx:121`) | relocate | window-head danger-ghost action, terminal snapshot | DS §09 head states |
| Prefill draft / Prefill replacement (`goal-status-controls.tsx:72`) | relocate | goalstrip body Actions row — stages `/goal …` / `/goal replace …` into the composer | chat (strip body) |
| "Moved to new session" strip + Active-session link (`goal-status-chip.tsx:157`) | restyle | `.goalstrip[data-state="moved"]`: faint dot, `moved` fact, body Session row with `--info` link | DS §09 |
| Open run link (`goal-status-chip.tsx:66`) | restyle | goalstrip body Run row link | chat |
| Turn meter (`goal-status-meters.tsx:71`) | restyle | `turn 3/20` mono fact; **progress bar dropped** — the number is the information | chat |
| Context meter + tokens + nudge (`goal-status-meters.tsx:13`) | restyle | `ctx 41%` fact + body Context row `84,120 / 200,000 tokens · nudge at 70%`; pending/unknown → `ctx —` + body sentence; **progress bar dropped** | chat |
| Last verdict (`goal-status-chip.tsx:196`) | restyle | goalstrip body row: outcome + blocking count + evidence mono | chat |
| Node / cause (`goal-status-chip.tsx:181`) | restyle | goalstrip body mono row | DS §09 |
| Goal states blocked / done / paused | restyle | `.goalstrip[data-state]` dot tones (warning / success / warning) | DS §09 variants |
| GoalPromptNotice (`session-timeline-render.tsx:286`) | restyle | `.marker--goal` line | chat |
| Goal error bar (`session-goal-header.tsx:31`) | restyle | `.banner` | DS §07 (vocabulary-only — a healthy live scenario can't host it) |

### Decisions

| Capability | Decision | Destination | Prototype |
|---|---|---|---|
| Allow once / Allow always / Reject once (`permission-prompt.tsx:149–173`) | relocate | `.dock` actions, keys 1–3 | chat |
| Reject always (`permission-prompt.tsx:176`) | relocate | `.dock__menu` split on Reject; key 4 fires directly | chat + DS §08 |
| `decisionOptions` gating | keep | buttons render only for offered options | spec §3 |
| Allowed receipt (today renders `null`) | **added** | `.receipt[data-tone="allowed"]` — mandatory | chat |
| Rejected receipt (`permission-prompt.tsx:242`) | restyle | `.receipt[data-tone="rejected"]` | chat (static) |
| Clarification choices (`clarification-card.tsx:92`) | relocate | `.dock` variant with `.choice__row`, keys 1–9 | chat (after permission resolves) + DS §08 |
| Clarification free-text (`clarification-card.tsx:122`) | keep | `.choice__free` textarea + Send answer | DS §08 interactive demo (chat scenario ships the choices variant) |
| Clarification deadline (`clarification-card.tsx:62`) | keep | `.dock__deadline` static mono hint | chat + DS §08 |
| Submitting / retryable-error states (`clarification-card.tsx:67–81`) | keep | quiet dock status lines (spinner sentence / danger sentence) | vocabulary-only |

### Tool outcomes

| Capability | Decision | Destination | Prototype |
|---|---|---|---|
| Result preview + Open full result + pagination (`tool-result-artifact.tsx:21`) | restyle | `.trow__artifact` in the detail rail; fetch/pagination semantics verbatim | chat (failed row detail) |
| Specialized renderers bash/edit/read/search/write | restyle | `.trow__detail` grammars per §3 | chat + DS §04 |
| TodoWrite plan | **added** | `.todo` renderer | chat + DS §10 |
| Generic JSON dump | restyle | detail-rail only, never a top-level card | DS §04 |

### Composer coexistence

| Capability | Decision | Destination | Prototype |
|---|---|---|---|
| Queue / Interrupt busy actions (`session-composer.tsx:211`) | keep | `.composer` bar ghost buttons; logic verbatim | chat |
| Enter-to-queue hint + busy Enter semantics (`session-composer.tsx:111`) | keep | `⏎ queue` hint; Enter stages the draft | chat |
| Queued strip: steer / edit / remove + `queuedPromptPreview` (`session-composer-queued-prompts.tsx:35`) | keep | `.qstrip` / `.qrow`; edit never clobbers a live draft | chat |
| Goal-command draft hint (`session-composer.tsx:183`) | keep | composer hint slot, `--info` quiet line "Goal command draft · ×" | vocabulary-only |
| SessionGoalCommandErrorNotice (`goal/session-goal-command-error-notice.tsx`) | keep | untouched — composer-zone notice outside the transcript; retone to marker grammar only | vocabulary-only |
| Draft persistence / stop / send | keep | `.composer` semantics unchanged | chat |

### Transcript states

| Capability | Decision | Destination | Prototype |
|---|---|---|---|
| Long user message clamp | restyle | `data-clamped` mask + `.msg-more` "Show more" | chat + DS §03 |
| Session-level failure banner (`session-resume-failure.tsx:41`) | restyle | `.banner` (the one allowed banner) | DS §07 (vocabulary-only in chat — contradicts a healthy resumed session) |
| Marker clustering ×N | restyle | one `.marker` + `.marker__meta` count | chat + DS §07 |
| Live thinking shimmer | restyle | `.thinking-live` | chat |
| Interrupted turn | restyle | `.turnfold--stopped` danger text line, never folds | chat |

## 9. Out of scope — must coexist

Untouched by this redesign; they keep their current contracts and only inherit tokens. No
dual rendering path may appear inside the transcript for any of them.

- **Session inspector** (`session-inspector.tsx` — Trace / Usage / Memory-ledger / Files / Vault).
- **Session create dialog** (`session-create-dialog.tsx`; prototype `_done/modals/start-session.html`).
- **Load older button** (`session-load-older-button.tsx`) and the scroll-to-bottom pill (`session-thread.tsx`).
- **Status line** (`session-status-line.tsx`) and topbar slot (`use-session-topbar-slot.tsx`).
- **Window chrome** (`session-window-content.tsx`) — hosts the goal zone, dock mount, and the
  new window-head goal actions; otherwise unchanged.
- **Repair / recap flows** — no dedicated UI today; none added.
- Wire contracts: no ACP/UIMessagePart change, no tool-ID change, no API change anywhere in
  this redesign.

## 10. Reference index — `.resources/*` (annotated)

Read the file before porting its pattern; line refs are from the checked-in snapshots. Both
repos are **visual-grammar references only** — Compozy's data layer (`UIMessagePart`,
`SessionTimelineToolPart`, goal snapshot) stays authoritative.

### 10.1 Timeline, turn fold, live tail

| Path | What it carries | Port? |
|---|---|---|
| `.resources/t3code/apps/web/src/components/chat/MessagesTimeline.logic.ts` | `MAX_VISIBLE_WORK_LOG_ENTRIES = 1` (:12) — t3code shows only the last live entry; `TIMELINE_CONTENT_MAX_WIDTH = 768` (:16) — the one-column premise; `computeMessageDurationStart` (:185); "Worked for ${duration}" derivation (:391); `deriveMessagesTimelineRows` (:405) with the hidden/visible slice for the "+N previous" overflow (:482–493); `computeStableMessagesTimelineRows` (:577) — the stable-row identity pattern our `isRowUnchanged` mirrors | pattern yes; our tail limit is `ACTIVE_WORK_VISIBLE_LIMIT = 4`, not 1 |
| `.resources/t3code/apps/web/src/components/chat/MessagesTimeline.tsx` | fold row anchoring, `WorkingTimer`/`LiveElapsed` ticker components (comment :121 — elapsed handled outside React commits) | ticker discipline yes; minimap no |
| `.resources/t3code/apps/web/src/components/chat/MessagesTimeline.logic.test.ts` + `MessagesTimeline.test.tsx` | test shapes for row derivation and folding | mirror the cases in `session-timeline.logic` suites |
| `.resources/synara/apps/web/src/components/chat/MessagesTimeline.logic.ts` | `MAX_VISIBLE_WORK_LOG_ENTRIES = 6` (:23); `chunkCollapsedTurnItems` (:43) + `chunkWorkEntries` (:77) — consecutive-kind chunking; `planWorkEntryRenderChunks` (:103) / `capOpenWorkEntryRenderChunks` (:128) — live-tail capping; `findLastLiveWorkGroupId` (:174) — only the last live run stays open; live-turn header mirroring the settled fold (:244); `collapseSettledTurns` post-pass (:634, contract comment :691) — **collapse on settle, even mid-turn** | yes — this is the §5 grouping engine |
| `.resources/synara/apps/web/src/components/chat/MessagesTimeline.tsx` | 46rem column render, receipt rows kept outside folds | grammar yes; MessageTrail/pin layers no |
| `.resources/synara/apps/web/src/components/chat/MessagesTimeline.toolGroupCollapse.browser.tsx` · `MessagesTimeline.toolDetails.browser.tsx` · `MessagesTimeline.markerScroll.browser.tsx` · `MessagesTimeline.messageEnter.browser.tsx` | browser-test contracts for collapse, inline detail, marker scroll anchoring, message enter motion | use as behavior checklists |

### 10.2 Tool-call grouping & summaries

| Path | What it carries | Port? |
|---|---|---|
| `.resources/synara/apps/web/src/components/chat/toolCallGroup.logic.ts` | `MIN_COLLAPSIBLE_TOOL_GROUP_SIZE = 2` (:15); `isSummarizableToolCallEntry` (:46); `classifyToolCallSummaryCategory` (:65); `entryFileKeys` (:99) — **distinct-file counting** for Edited/Read; `CATEGORY_ORDER` (:120) — fixed Ran → Edited → Read → Searched → agent tasks → Used; label templates "Ran N commands / Edited N files / …" (:137–150); `summarizeToolCallGroup` (:155) | yes — port as `summarizeToolGroup` in `session-timeline.logic.ts` |
| `.resources/synara/apps/web/src/components/chat/ToolCallGroupSummaryRow.tsx` | collapsed summary row anatomy (first entry's icon + label + chevron) | yes → `.tgroup-sum` |
| `.resources/synara/apps/web/src/components/chat/TimelineWorkEntryRow.tsx` | 24px entry row grammar, status glyph slot | yes → `.trow` |
| `.resources/synara/apps/web/src/components/chat/ToolCallDetailsDialog.tsx` | full result in a dialog | **no** — Compozy expands inline (`.trow__detail` + `.trow__artifact`) |
| `.resources/synara/apps/web/src/workLog.ts` | `WorkLogEntry` (:48) — the entry shape the summarizer consumes | map to `SessionTimelineToolPart`; do not copy the type |

### 10.3 Decisions (approval / user input)

| Path | What it carries | Port? |
|---|---|---|
| `.resources/synara/apps/web/src/components/chat/ComposerPendingApprovalPanel.tsx` | `APPROVAL_ACTIONS` accept (:47) / acceptForSession (:53) / decline (:59) / cancel (:65); capability filter when a decision is unavailable (:89); respond carries `lifecycleGeneration` + `requestKind` (:109) | action-order + gating grammar yes; Compozy keys stay 1–4 over ACP's four decisions |
| `.resources/synara/apps/web/src/components/chat/ComposerPendingUserInputPanel.tsx` | neutral (non-accent) choice rows (:31); multi-select "Select one or more." (:189) | row tone yes; **multi-select no** — Compozy clarification is single-choice or free-text |
| `.resources/synara/apps/web/src/components/chat/ComposerStackedPanel.tsx` + `ComposerStackedPanelContent.tsx` + `composerStackedPanelStyles.ts` | the stacked-above-composer panel frame (synara stacks; **Compozy fuses** — shared border, top radius) | geometry reference only |
| `.resources/t3code/apps/web/src/components/chat/ComposerPendingApprovalPanel.tsx` + `ComposerPendingApprovalActions.tsx` | minimal approval panel + action pair | secondary reference |
| `.resources/t3code/apps/web/src/components/chat/ComposerPendingUserInputPanel.tsx` + `.resources/t3code/apps/web/src/pendingUserInput.ts` | option rows ("Select one or more options.", panel :169) with a custom free-text answer path (`setPendingUserInputCustomAnswer`, pendingUserInput.ts:65) | free-text path yes → `.choice__free`; multi-select no — Compozy clarification is single-choice |
| `.resources/t3code/apps/web/src/components/chat/ComposerBannerStack.tsx` | banner stacking discipline above the composer | budget reference (Compozy budget: one) |

### 10.4 Composer busy phase & queue

| Path | What it carries | Port? |
|---|---|---|
| `.resources/synara/apps/web/src/components/chat/ComposerQueuedHeader.tsx` | `compactQueuedComposerPreviewMarkdown` (:40) — first non-empty line, fence → "Code block" (:45), markdown markers stripped | already mirrored by `queuedPromptPreview`; keep parity |
| `.resources/synara/apps/web/src/components/chat/QueuedComposerActions.tsx` | Steer / Edit / Remove cluster on queue rows (:19–28) | yes → `.qrow__acts` |
| `.resources/synara/apps/web/src/composerDraftStore.ts` + `composerDraftPersistence.ts` + `composerDraftActions.ts` | draft persistence across busy phases | Compozy already has `persistComposerText`; reference only |
| `.resources/synara/apps/web/src/components/ThreadRunningSpinner.tsx` + `.../chat/agentActivity.logic.ts` | running-state signal derivation | reference for `.working` gating |

### 10.5 Chrome, banners, meters, outcomes

| Path | What it carries | Port? |
|---|---|---|
| `.resources/synara/apps/web/src/components/chat/chatTypography.ts` | the compressed chat type scale | rationale for §11 heading scale (17→13.5px) |
| `.resources/synara/apps/web/src/components/chat/ThreadErrorBanner.tsx` · `RateLimitBanner.tsx` · `ChatColumnBannerFrame.tsx` | session-level banner anatomy in-column | yes → `.banner` (budget: one) |
| `.resources/t3code/apps/web/src/components/chat/ThreadErrorBanner.tsx` · `ProviderStatusBanner.tsx` | same, minimal variant | secondary |
| `.resources/synara/apps/web/src/components/chat/ContextWindowMeter.tsx` + `.resources/t3code/apps/web/src/components/chat/ContextWindowMeter.tsx` | context threshold semantics | semantics yes; **meter visual no** — Compozy renders mono facts (`ctx 41%` + tokens row) |
| `.resources/t3code/apps/web/src/components/chat/ChangedFilesTree.tsx` + `changedFilesPresentation.ts` | changed-files presentation, caps | yes → `.changed` (cap 8 + "+N more") |
| `.resources/synara/apps/web/src/components/chat/ComposerLiveChangesHeader.tsx` | live changes summary line | reference for `.changed` live state |
| `.resources/synara/apps/web/src/components/chat/ChatHeader.tsx` + `chatHeaderControls.tsx` · `.resources/t3code/apps/web/src/components/chat/ChatHeader.tsx` | header action placement | reference for window-head goal actions |

### 10.6 Explicitly NOT ported (seen and excluded)

`MessageTrail.tsx` + `messageTrail.logic.ts`, `ProposedPlanCard.tsx`/`ProposedPlanActions.tsx`
(both repos), `ComposerSubagentStrip.*`, `WorkflowRunCard.*`, `ComposerVoiceButton.tsx`/
`ComposerVoiceRecorderBar.tsx`, `ComposerStashMenu.tsx`/`ComposerStashBadge.tsx`,
`TraitsPicker.tsx`, `ToolCallDetailsDialog.tsx`, t3code timeline **minimap**
(`TIMELINE_MINIMAP_*`, `MessagesTimeline.logic.ts:13–99`), pin/undo/checkpoint layers.
These exist in the references, were evaluated, and are out of Compozy's scope — do not
reintroduce them as "reference parity".

## 11. Implementation playbook (build order)

Each phase lands green on its scoped lane; `make verify` once at the end. Geometry authority
is `session.css` (class contracts in §2); token authority stays `packages/ui/src/tokens.css`.

1. **Logic first — `web/src/components/assistant-ui/session-timeline.logic.ts`.**
   Port `summarizeToolGroup(entries)` from `toolCallGroup.logic.ts` (§10.2): fixed
   `CATEGORY_ORDER`, distinct-file counts via `entryFileKeys` semantics over
   `SessionTimelineToolPart`, `MIN_COLLAPSIBLE_TOOL_GROUP_SIZE = 2`. Add collapse-on-settle
   (synara `collapseSettledTurns` post-pass) and marker clustering (×N). Delete
   `SETTLED_WORK_VISIBLE_LIMIT`; keep `ACTIVE_WORK_VISIBLE_LIMIT = 4`. Update
   `timeline-row-estimates.ts` (trow ≈ 26px, marker ≈ 24px) and `isRowUnchanged`.
   Extend the existing logic test suite with the reference cases (§10.1 test files).
2. **Primitives — `packages/ui`** (story + test each, per reuse-before-create):
   retoned `tool-call-row` (`.trow` grammar), new `marker`, `receipt`, `dock` (incl.
   `.dock__menu` split, `.dock__deadline`, `.choice__row`/`.choice__free`). Delete the
   `code-block.tsx` tone-recolor path in the same change.
3. **Transcript components — `web/src/systems/session` + `assistant-ui`** per §3 rows:
   messages (clamp + hover meta), thinking, working, markers/liferule/banner, changed files,
   `.todo` renderer, `.trow__artifact` (keep `use-tool-artifact` fetch + pagination verbatim).
4. **Decisions — dock host in `session-window-content.tsx` composer zone.** Permission
   (4 decisions, keys 1–4, `decisionOptions` gating), clarification (choices | free-text,
   deadline, submitting/error lines), receipts incl. the **added allowed receipt**.
5. **Goal — strip + window head.** `.goalstrip` in the pinned goal zone (facts, body rows,
   prefill actions, moved variant); state-gated head actions wired to the existing
   `onPause/onResume/onApprove/onClear` callbacks; delete `goal-status-chip.tsx`,
   `goal-status-meters.tsx` and fold `goal-status-controls.tsx` into the head per §3.
6. **Composer + runtime selector.** Retone `.composer`/`.qstrip` (logic verbatim), mount the
   variation-A selector (`runtime-selector-variations.html`) on the bar left.
7. **Close-out.** QA scenario flags (§7), stale screenshot refresh, `make verify`, and the §8
   matrix re-checked row by row — any capability whose destination changed during
   implementation updates this spec in the same PR.

Acceptance for every phase: no `*-tint` wash inside the transcript, color budget §1 holds,
failed rows stay collapsed, receipts stay outside folds, and no dual rendering path survives
(Zero Legacy Tolerance).

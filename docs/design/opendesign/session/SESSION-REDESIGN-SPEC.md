# Session Transcript Redesign — Implementation Spec

Status: prototype delivered, awaiting review
Prototypes: `session/session-design-system.html` (vocabulary + rules) · `session/session-chat.html` (applied) · `session/session.css` (canonical CSS)
References analyzed: `.resources/t3code` (MessagesTimeline lineage) · `.resources/synara` (MessagesTimeline + toolCallGroup lineage)
Token authority: `packages/ui/src/tokens.css` → `design-system/ds-core.css` (copied verbatim into `session.css`)

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

All classes live in `session/session.css`. Production equivalents go to `packages/ui`
(generic) or `web/src/systems/session` (domain), per the reuse-before-create rule.

| Class | Contract |
|---|---|
| `.chat-col` | 46rem centered column; used by transcript, banner, dock, composer |
| `.t-row` / `.t-row--turn-end` | 7px in-turn / 18px between-turn rhythm; no other margins |
| `.msg-user__bubble` | right-aligned, max-w 80%, `--chat-fill-user` (4.5% ink), radius-lg, **no border**; clamp at 176px with mask + "Show more" |
| `.msg-assistant.md` | naked markdown, full column, `--fg`, 13.5px/1.62 |
| `.msg-meta` | hover-revealed timestamp · copy · use-as-goal; terminal message of settled turn only |
| `.trow` | 24px line: 18px icon well · verb (500, `--muted`) · mono preview (`--subtle`) · optional `.diffstat` · chevron · status glyph; hover brightens verb, `--row-hover` fill only when expandable |
| `.trow__detail` | expanded body: `margin-left: 25px`, 1px `--line` rail, mono 11px `--muted`, max-h 240px; diff/stderr colored **text**, no tinted blocks |
| `.tgroup-sum` | collapsed run: first entry's icon + "Ran 2 commands · Edited 3 files" (middle-dot join, fixed order Ran/Edited/Read/Searched/Used; distinct-file counts) + chevron |
| `.tmore` | bare-text overflow toggle "+N previous tool calls", indented 25px |
| `.turnfold` | "Worked for {turnDuration}" 12px `--subtle` + chevron; **the only border-bottom in the transcript**; `--stopped` variant = danger text line, never folds |
| `.thinking-live` / `.thinking` | live: shimmer text label only; settled: `.trow` grammar with "Thought" verb + indented muted body |
| `.working` | three 4px dots (stepped 2s duty cycle) + "Working for Ns" 11.5px, ticker mutates text node |
| `.marker` | 12px tone glyph + one muted sentence + optional mono meta; replaces every in-transcript Alert |
| `.liferule` | centered mono caption between hairlines (resume, compaction) |
| `.banner` | session-level failure only, above transcript: 28% hairline + 4% wash + actions; budget: one |
| `.dock` | decision panel fused to composer top (shared border, top radius); eyebrow tracking .14em, mono pre on `--chat-fill-code`, actions: primary allow · always · spacer · danger-ghost reject; keys 1–9 |
| `.choice__row` | clarification options, 30px rows with `.choice__key` shortcut chips |
| `.receipt` | one-line decision record: tone glyph + sentence; allowed decisions also get one |
| `.goalstrip` | 24px line above transcript: state dot (accent pulse/warning/success) + GOAL kicker + objective + `turn 3/20 · ctx 41%` mono facts + chevron → quiet key/value body |
| `.changed` | "Edited N files +A −D" line → bare mono file lines behind the rail; cap 8 + "+N more" |
| `.todo` | TodoWrite renderer: caption "Plan · 2 of 4" + rows (done: grey check + line-through · active: accent pulse dot · pending: hollow dot) |
| `.composer` | redesigned send box: hairline `--line` border (hover `--line-strong`, focus `--accent-dim`), elevated fill, radius-lg, no zone top-border (scroll fade separates); bar = quiet `kbd ⏎` hint · spacer · ghost circular stop · circular accent send (28px). Decision shortcuts render as kbd chips on dock buttons, not in the composer hint |
| `.goal-zone` | goal strip mounts ABOVE the scroll viewport (pinned context, like `SessionGoalHeader`), never inside the scroller — the top scroll fade must not mask it |
| `.md` / `.codeblock` | compressed heading scale (17→13.5px), inline code on `--badge-fill`, fenced on `--chat-fill-code` with quiet header, `--info` links, row-separator tables |
| `.disclose` | the single disclosure motion: grid-rows 0fr→1fr + opacity, 220ms, reduced-motion safe |

## 3. Migration map (current → new)

Delete-targets are explicit per Zero Legacy Tolerance — no dual rendering paths.

| Current (file:line) | Action |
|---|---|
| `session-thread.tsx:89` UserMessage `rounded-xl border bg-canvas-soft` | restyle → `.msg-user__bubble` (drop border, 4.5% fill, meta hover-only) |
| `packages/ui/…/tool-call-row.tsx:90` + `tool-call-card.tsx:78` | retone to `.trow`; **delete** `defaultExpanded \|\| status==="failed"` (`tool-call-card.tsx:123`); delete danger icon-well tint; success check → `--subtle` |
| `session-timeline-render.tsx:176` work eyebrow "N tool calls" | replace with `.tgroup-sum` semantic label; settled cluster = summary line (not "last row + toggle") |
| `session-timeline.logic.ts:160` `SETTLED_WORK_VISIBLE_LIMIT` | settled runs render as collapsed `.tgroup-sum`; `ACTIVE_WORK_VISIBLE_LIMIT=4` keeps last-4 live tail + `.tmore` |
| `session-timeline-render.tsx:115,214` toggle/fold buttons | keep logic; retone to `.tmore` / `.turnfold` |
| `thinking-block.tsx:25` | drop icon well + "N updates" eyebrow; live = `.thinking-live` shimmer; settled = `.thinking` row; keep auto-open/auto-collapse behavior |
| `session-working-row.tsx:43` | keep; retone to `.working` (4px dots, stepped pulse) |
| `runtime-activity-notice.tsx:152–253` (5 Alert kinds) | replace with `.marker` lines; **delete** kind Pills; cluster consecutive same-kind markers with ×N count |
| `session-message-parts.tsx:17` SessionDataEventCard | replace with `.marker` + mono meta; **delete** the `<pre>` preview card |
| `session-thread.tsx:61` SessionMessageErrorNotice (`border-danger/30 bg-danger/8`) | replace with danger `.marker`; delete off-token alphas |
| `session-resume-failure.tsx:41` | restyle to `.banner` (28%/4%); **delete** AlertMeta pills + MonoId pairs — path in body text |
| `permission-prompt.tsx:69` sticky prompt | **delete sticky positioning**; move to `.dock` in `session-window-content` composer zone; 3 visible actions (reject-always behind reject menu); numeric shortcuts |
| `permission-prompt.tsx:242` PermissionRejectedNotice | replace with `.receipt`; **add** allowed-receipt (today allowed renders null) |
| `clarification-card.tsx:23` / `clarification-receipt.tsx:27` | card → `.dock` variant with `.choice__row`; receipt → `.receipt` |
| `goal-status-chip.tsx:93` GoalStatusChip | **delete** (icon tile, 2 pills, meter grid, moved strip, 5-button bar) → `.goalstrip`; meters become mono facts; Approve/Clear → window-head actions |
| `session-timeline-render.tsx:286` GoalPromptNotice | replace with `.marker--goal` line |
| `session-goal-header.tsx:31` error bar (`bg-danger-tint` full-bleed) | replace with `.banner` |
| `session-changed-files-row.tsx:54` card | replace with `.changed` line + file lines |
| `generic-content.tsx:24` TodoWrite JSON dump | **add** `.todo` renderer for TodoWrite/plan tools; generic JSON only inside `.trow__detail` |
| `edit-content.tsx:19` stacked tinted CodeBlocks | unified diff inside `.trow__detail`, colored text lines only |
| `bash-content.tsx:17` stderr `tone="danger"` block | mono detail; error lines in danger **text**; delete ring/tint |
| `search-content.tsx:11` bordered result list | bare mono lines in the detail rail |
| `code-block.tsx:316` `tone` full-block recolor | **delete** tone text-recolor path (keep density/copy) |
| `alert-variants.ts:24-27` tinted variants | transcript stops importing them; audit remaining callers before deletion |

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

`session-timeline.logic.ts` already clusters consecutive tool parts per turn — keep, plus:

1. **Semantic summary** — `summarizeToolGroup(entries)` → category counts, fixed order,
   middle-dot join (new, port of synara `toolCallGroup.logic.ts`).
2. **Collapse on settle** — a run collapses the moment it settles, even mid-turn; only the
   live tail run stays expanded.
3. **Narration breaks runs** — reasoning/data rows flush the cluster (already true — keep).
4. **Marker clustering** — consecutive same-kind data rows merge into one marker with ×N.
5. **Receipts are persistent rows** — permission/clarification outcomes stay outside turn
   folds (`isPersistentTurnRow` already does this — keep).
6. Update `timeline-row-estimates.ts` (trow ≈ 26px, marker ≈ 24px) and keep
   `isRowUnchanged` in sync with new row fields.

## 6. Behaviors

- **Failed rows stay collapsed**; failure is visible via glyph + error-first-line preview.
- **Interrupted turns never fold**; next turn re-folds nothing retroactively.
- **Meta on hover only**, terminal messages only; mid-turn commentary never shows meta.
- **Ticker discipline**: elapsed labels mutate text nodes (no per-second React commits).
- **Reduced motion**: shimmer → static subtle text; dots static; disclosures instant.
- **Keyboard**: 1–9 decide dock choices; Esc dismisses banner; fold/disclose are buttons
  with `aria-expanded`.

## 7. Web/Docs impact

- `web/`: `systems/session/components/*` (major), `components/assistant-ui/*` (major),
  `packages/ui` (`tool-call-row`, new `marker`/`receipt`/`dock` primitives — story + test each),
  `session-window-content.tsx` (dock placement, goal strip, window-head goal controls).
- Docs: `packages/site` session screenshots will be stale after migration; refresh captures.
- QA: user-visible change → add `untested` scenarios for transcript rendering, permission
  dock flow, goal strip, and reset existing session-view scenarios to `untested`.

Compozy Impact Audit:

- Native tools: no impact — rendering only; tool IDs, schemas, descriptors, capability gates unchanged (checked: labels come from `tool-labels.ts` presentation layer).
- Extensibility and hooks: no impact — ACP/UIMessagePart wire shape untouched; MCP/generic tools render through the same `.trow` + detail path (checked: `toTimelineParts`, `session-data-renderers`).
- Workspace data isolation: no impact — no new data reads; all rows derive from the session's own transcript stream (checked: session-scoped queries only).
- Official Compozy skill: no impact — no CLI/HTTP surface or behavior change; visual layer only.

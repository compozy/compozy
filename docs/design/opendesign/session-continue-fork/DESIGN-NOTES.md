# Session continue & fork — visual contract

Three boards for `.compozy/tasks/session-continue-fork/_uiux.md` (S1–S10), authored 2026-09-11 after the spec and the task graph. Implementation tasks cite these files as Visual Contracts (the task tables are authoritative): task_04 owns VC-01..03, VC-06..10, VC-16, VC-18..22; task_06 owns VC-04, VC-05, VC-11..15, VC-17, VC-23; task_08 captures the bundles. Companion files: `index.html` (map + matrix), `session-continue-fork.css` (domain lane, chapters 00–10).

## How the boards are written

Same grammar as `session-context/`, `sessions-stability/` and `sessions-bulk-actions/`:

- **Contract block first.** Every board opens with `.ss-contract`: what the surface is in plain words · the production components it modifies / adds / deletes / reuses unchanged, each with its repo path · the states on the page, linked, each with its VC id.
- **Numbered callouts + legend on every specimen.** Inverted `.ss-n` markers next to the element; the `.ss-legend` under the specimen names the component, the token values, and the authority (`production` · `delta` · `truth`).
- **Tag strip per specimen.** `state · component · VC · tone/accent`. Every VC id is one specimen with `data-od-id="scf-vc-NN"`; sections are `id="scf-<board>-<nn>"`.
- **CSS classes named after production components.** `.session-row-actions*` = `SessionRowActions`, `.session-topbar-overflow*` = the `useSessionTopbarSlot` overflow, `.message-actions` / `.session-fork-message-action` / `.session-rewind-message-action` = the message action row, `.session-continue-dialog*` / `.session-fork-dialog*` = the two dialogs, `.session-derive-preview*` / `.session-derive-placement*` = the shared context line and Open-in radio, `.session-origin-pill` = `SessionOriginPill`, `.session-continue-divider*` = `SessionContinueDivider`, `.session-status-line*` = `SessionStatusLine`, `.session-resume-failure*` = `SessionResumeFailure`, `.runtime-activity-notice*` = `ProviderErrorNotice`, `.session-inspector-ledger` = `SessionLedgerMetaPanel`, `.session-sidebar` / `.session-list*` = the rail (copied from the bulk-actions board). Lab chrome is `lab-*` / `ss-contract` / `ss-n` / `ss-legend` / `ss-tags` / `scf-*` and never product UI.
- The window shell, thread, composer and message anatomy come from `../_done/sessions-stability/sessions-stability.css` unmodified (the sessions-stability set was archived into `_done/` on 2026-09-11 by another session while these boards were authored; the link follows the file); the dialog frame, menu, pill, buttons and fields from `../design-system/ds-core.css`; nothing here redraws production chrome.

## Sources read before drawing

- Spec set: `.compozy/tasks/session-continue-fork/{_spec.md,_uiux.md,_user_stories.md,_dx.md,_tasks.md}`; decisions D2–D7 in `.compozy/tasks/session-handoff/DECISIONS.md`.
- Research: `.compozy/tasks/session-handoff/analysis/01_analysis_superset.md` (handoff menu, dialog, token estimate, fork locks the agent), `03_analysis_synara.md` (`ForkSourceDivider`, "Fork from here" on settled turns, busy refusal, native-first fork with transcript fallback), `02_analysis_codex.md` (`lastTurnId` inclusive cut, in-progress turn refused, "Thread forked from" banner), `08_analysis_compozy-web-ux.md` (attachment map, copy collisions).
- Competitor references named in `_spec.md`: Superset `TerminalSessionHandoffMenu.tsx:33-38,94-117,259-278,327-336`; Synara `ForkSourceDivider.tsx:19-53`, `threadHandoff.ts:76-81,183-204`, `MessagesTimeline.tsx:1823-1826`; Codex `slash_command.rs:35,101`, `thread.rs` `ThreadForkParams`.
- Production: `session-row-actions.tsx:27-94`, `use-session-topbar-slot.tsx:94-238`, `message-actions.tsx:22-75`, `session-rewind-message-action.tsx`, `session-status-line.tsx:27-86`, `session-create-dialog.tsx:125-215`, `session-window-content.tsx:114-127`, `session-resume-failure.tsx`, `runtime-activity-notice.tsx:95-165`, `session-inspector-memory.tsx:84-119`, `session-list-thread.tsx:34-100`, `packages/ui` `pill`, `dialog`, `dropdown-menu`, `button`, `tokens.css`, `DESIGN.md` §1–2 and §10, `COPY.md` §5–6.

## Component map (design → production)

| Verb | Component | Path | Board | VC |
| --- | --- | --- | --- | --- |
| modify | `SessionRowActions` (two items after Rename) | `web/src/systems/session/components/session-row-actions.tsx` | menus §01 | task_04 VC-01, VC-02 (fork item: task_06) |
| modify | `useSessionTopbarOverflow` (same two items) | `web/src/systems/session/hooks/use-session-topbar-slot.tsx` | menus §02 | task_04 VC-03 |
| new | `SessionForkMessageAction` | `web/src/systems/session/components/session-fork-message-action.tsx` | menus §03 | task_06 VC-04, VC-05 |
| modify | `MessageActions` (mount before rewind) | `web/src/components/assistant-ui/message-actions.tsx` | menus §03 | VC-04, VC-05 |
| modify | `SessionWindowContent` · `SessionResumeFailure` (`retryLabel`, `lineage_kind: recovery`) | `web/src/systems/os/apps/session/session-window-content.tsx` · `session-resume-failure.tsx` | menus §04 | task_04 VC-21 |
| modify | `ProviderErrorNotice` · `provider-error.ts` (`handoff`) | `web/src/systems/session/components/runtime-activity-notice.tsx` · `lib/provider-error.ts` | menus §05 | task_04 VC-22 |
| new | `SessionContinueDialog` | `web/src/systems/session/components/session-continue-dialog.tsx` | dialogs §01 | task_04 VC-06..10 |
| new | `SessionForkDialog` | `web/src/systems/session/components/session-fork-dialog.tsx` | dialogs §02 | task_06 VC-11..15 |
| new | `useSessionDerive` · `useSessionDerivePreview` · `session-derive-api.ts` | `web/src/systems/session/hooks/use-session-derive.ts` · `adapters/session-derive-api.ts` | dialogs | — |
| new | `SessionOriginPill` | `web/src/systems/session/components/session-origin-pill.tsx` | lineage §01 | task_04 VC-16, VC-18 · task_06 VC-17 |
| modify | `SessionStatusLine` (pill after provider) | `web/src/systems/session/components/session-status-line.tsx` | lineage §01 | VC-16..18 |
| new | `SessionContinueDivider` | `web/src/systems/session/components/session-continue-divider.tsx` | lineage §02 | task_04 VC-19, VC-20 |
| new | `sessionOriginLabel` | `web/src/systems/session/lib/session-origin.ts` | lineage | — |
| modify | `SessionLedgerMetaPanel` (Origin row) | `web/src/systems/session/components/session-inspector-memory.tsx` | lineage §03 | task_06 VC-23 |
| reuse | `SessionListThread` · `session-hierarchy.ts` (nesting unchanged) | `web/src/systems/session/components/session-list/` · `lib/session-hierarchy.ts` | lineage | — |
| reuse | `DropdownMenu*` · `Dialog*` · `EntityDialogHeader/Body/Footer` · `Button` · `Pill` · `RadioGroup` · `Select` · `Textarea` · `FieldError` · `Spinner` · `Empty` | `@compozy/ui` | all | — |
| reuse | `AgentCommandSelect` · `RuntimeSelector` | `web/src/systems/session/components/agent-command-select.tsx` · `web/src/systems/runtime/components/runtime-selector/` | dialogs | — |

No new `@compozy/ui` primitive. `Badge` and `HoverCard` do not exist and are not added; the origin chip is `Pill`; the compact "Open in" choice is `RadioGroup`. `ConfirmDialog` stays Rewind's; neither new dialog confirms in danger tone.

## Signal and state dictionary

| Glyph / state | Primitive + token | Where |
| --- | --- | --- |
| Continue with another agent… · Fork session… | `DropdownMenuItem` default variant, icons `ArrowRightLeft` / `GitFork` 12px | row kebab, window overflow |
| menu items disabled | faint ink, same as the neighbouring items while a row action is pending / `controlsBusy` | menus |
| Fork from here (rest) | ghost `Button size="xs"`, `text-muted hover:text-fg`, `GitFork` 12px — the rewind button's twin | message row |
| Fork from here (busy) | `disabled`, faint ink, no tooltip | message row |
| Restart in a new session | production neutral `Button size="sm"` with `RefreshCw` — copy only | dead-runtime banner |
| handoff next step | sentence in the marker body + ghost `Button size="xs"` with `ArrowRightLeft` | provider-error marker |
| dialog frame | `Dialog` sm (448px) unframed, `EntityDialogHeader` eyebrow `Operate · Session`, icon `ArrowRightLeft` / `GitFork` in the neutral well | both dialogs |
| context line (ready / truncated) | `text-form-hint text-subtle`, `FileText` glyph `--faint`; truncated keeps the tone and adds the omitted-count sentence | both dialogs |
| context line (measuring) | same tone, `Measuring…` with `Spinner` | both dialogs |
| context line (error) | `FieldError` danger text with `CircleAlert` `--danger`; primary disabled | both dialogs |
| native clone line | `Uses the agent's own session clone.` in the same subtle tone, `Copy` glyph; present only when the preview reports `native_fork_possible` | fork dialog |
| Open in | `RadioGroup` two items, accent dot when checked; `New window` default | both dialogs |
| primary | `Button variant="primary"` — the one accent per dialog; disabled while measuring / preview error / no agent / cut turn in progress / after a fence conflict | both dialogs |
| pending | `Spinner` + `aria-live` sentence `Starting the new session…`; fields disabled; close hidden | both dialogs |
| refusal | `FieldError` text under the fields (`That turn hasn't settled yet.` · `Transcript changed — reopen to fork from the current state.` · daemon messages verbatim) | fork dialog, continue dialog |
| origin pill | `Pill` xs neutral: `badge-fill`, `--muted` text, agent/title in `--fg`, glyph `--subtle`; link variant hovers to `btn-hover`; plain variant has no hover | status line |
| divider | 1px `--line-soft` hairlines, link text `--accent-strong`, glyph `--subtle`; plain `--muted` when the source is gone | child transcript |
| empty child | compact `Empty`: form-weight title, micro description, no well | child transcript |
| ledger Origin row | `prow` label/value, kind word `--muted`, anchor mono | inspector ledger |

Signal colour marks state only. Nothing in this feature is danger-toned except production surfaces drawn as neighbours (Rewind's confirm, the dead-runtime banner, the provider-error glyph) and the refusal text inside the dialogs. Origin is information: neutral pill, hairline divider, no accent plate.

## Copy (COPY.md register; D6 binding)

- Menu items: **Continue with another agent…** · **Fork session…** (after "Rename session", before "Stop session").
- Message action: **Fork from here** (before "Rewind to here").
- Dead-runtime notice: **Restart in a new session** (replaces "Fork into a new session").
- Provider-error next step: **Continue this session with another agent or route.** + button **Continue with another agent…**
- Continue dialog: title **Continue with another agent**; description **Start a new session for another agent with this conversation carried over. This session stays unchanged.**; fields **Agent** · **Runtime** · **Route** (Default · `{provider} · {model}`) · **First message** (placeholder **Optional. Sent after the carried context.**) · **Open in** (**New window** default · **This window**); primary **Continue**; pending **Starting the new session…**; foot note **From {source title} · {agent}**.
- Fork dialog: title **Fork session**; description **Start a second session with the same agent and this conversation. This session stays unchanged.**; **Agent** read-only `{agent_name} · {provider}`; **Fork point**: **Whole session** / **Through "{first 60 chars}"** + **Change**; primary **Fork session**; foot note **From {source title}**.
- Context line: **Carries over {n} messages · {size}** · **Carries over {k} of {n} messages · {size}** + **{n−k} earlier messages omitted to fit the context budget.** · **Measuring…** · **Couldn't measure this session's context.** · **A turn is still in progress; it will not be carried over.** · **Uses the agent's own session clone.**
- Refusals: **That turn hasn't settled yet.** · **Transcript changed — reopen to fork from the current state.** · daemon messages verbatim for `agent_not_found`, `session_archived`, `route_not_found`, `new_work_admission_unavailable`.
- Origin: pill **Continued from {origin_agent_name}** / **Forked from {parent title}**; divider **Continued from {parent title}** / **Forked from {parent title}**; ledger **continue · from {agent}** / **fork · through {message_id}** / **fork**; empty child **Nothing said here yet** + **The conversation carried over is sent with your first message.** (continue) / **The conversation up to the fork point is carried into your first message.** (fork).
- Never in product copy: *handoff* (Network channel kind), *branch* (git, worktree, Loops), *chat*, *Handoff from X*, *Continued from chat*, a *Badge*.

## VC matrix

| VC | Board · section | State | Viewport | Owner |
| --- | --- | --- | --- | --- |
| VC-01 | menus §01 | row kebab open on a user row; both items present | 1440×900 | task_04 |
| VC-02 | menus §01 | row kebab open on an archived row; items absent | 1440×900 | task_04 |
| VC-03 | menus §02 | window overflow open on a running source; items present | 1440×900 | task_04 |
| VC-04 | menus §03 | Fork from here at rest beside Rewind to here | 1440×900 | task_06 |
| VC-05 | menus §03 | Fork from here disabled while the thread runs | 1440×900 | task_06 |
| VC-06 | dialogs §01 | Continue · default, other agent preselected, preview ready | 1440×900 | task_04 |
| VC-07 | dialogs §01 | Continue · Route row present | 1440×900 | task_04 |
| VC-08 | dialogs §01 | Continue · truncated preview with omitted count | 1440×900 | task_04 |
| VC-09 | dialogs §01 | Continue · preview error, primary disabled | 1440×900 | task_04 |
| VC-10 | dialogs §01 | Continue · pending | 1440×900 | task_04 |
| VC-11 | dialogs §02 | Fork · whole session | 1440×900 | task_06 |
| VC-12 | dialogs §02 | Fork · through a quoted message + Change | 1440×900 | task_06 |
| VC-13 | dialogs §02 | Fork · native clone sentence | 1440×900 | task_06 |
| VC-14 | dialogs §02 | Fork · cut turn in progress, primary disabled | 1440×900 | task_06 |
| VC-15 | dialogs §02 | Fork · transcript changed after opening | 1440×900 | task_06 |
| VC-16 | lineage §01 | pill "Continued from claude" as a link | 1440×900 | task_04 |
| VC-17 | lineage §01 | pill "Forked from {title}" as a link | 1440×900 | task_06 |
| VC-18 | lineage §01 | narrow head: title and agent truncate, pill keeps its verb | 1440×900, session window 640px wide | task_04 |
| VC-19 | lineage §02 | divider above the child's own turns | 1440×900 | task_04 |
| VC-20 | lineage §02 | divider alone above the compact empty state | 1440×900 | task_04 |
| VC-21 | menus §04 | dead-runtime banner with "Restart in a new session" | 1440×900 | task_04 |
| VC-22 | menus §05 | provider-error marker with the handoff sentence and button | 1440×900 | task_04 |
| VC-23 | lineage §03 | ledger Origin row (fork through message; continue from agent) | 1440×900 | task_06 |

Evidence for each row lands at `.compozy/tasks/session-continue-fork/evidence/visual/<task-id>/<VC>/` (task_08 owns the complete bundles; implementation tasks inspect a representative state early).

## Gaps and authorized deltas

- The window head status line is drawn with the sessions-stability meta (`word · agent · provider`); production `SessionStatusLine` renders the badge glyph plus the state word — host chrome, lossy by nature (SD-007).
- `RuntimeSelector` in the Continue dialog is drawn as two composer-style chips (model · reasoning, speed); production is the selector's dialog variant with `IntensityMeter`. Component identity stays with production.
- The rail rows and the toolbar are copied from the bulk-actions board and are not part of this contract.
- The plain (source deleted) pill and the "continue" ledger row are drawn as third specimens without a VC id: they share the VC-16 / VC-23 implementation and are checked by unit tests, not screenshots.
- Agent names, titles, ids, sizes and message counts are fixtures; runtime truth owns values.

## Open questions for the design pass

- Whether the Route row should also show the account fingerprint label once Spec A ships (today: `{provider} · {model}` only).
- Whether the empty-child copy needs an explicit "sent with your first message" for the native-fork path, where the child's transcript is replayed by the agent instead (ADR-003 of the spec).
- Palette / menubar `session.continue` / `session.fork` commands are not drawn; they are optional in `_uiux.md` and would reuse the same dialogs.

## Revision log

- 2026-09-11 · authored after the spec, the task graph and DECISIONS.md D6/D7.
- 2026-09-11 · peer review round 1 (N-004): VC owners aligned to the task tables (VC-04/05 → task_06); VC-18 viewport stated as 1440×900 with a 640px session window; sessions-stability css path follows its archive location.

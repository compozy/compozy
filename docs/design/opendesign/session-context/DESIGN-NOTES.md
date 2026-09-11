# Session context — visual contract

Two boards for `.compozy/tasks/session-context/_uiux.md` (S1–S5), authored 2026-09-11 after the spec and the task graph. Implementation tasks cite these files as Visual Contracts: task_03 owns VC-01..09, task_04 owns VC-10..11; task_06 captures the bundles. Companion files: `index.html` (map + matrix), `session-context.css` (domain lane, chapters 01–08).

## How the boards are written

Same grammar as `sessions-stability/` and `sessions-bulk-actions/`:

- **Contract block first.** Every board opens with `.ss-contract`: what the surface is in plain words · the production components it modifies / adds / deletes / reuses unchanged, each with its repo path · the states on the page, linked, each with its VC id.
- **Numbered callouts + legend on every specimen.** Inverted `.ss-n` markers next to the element; `.ss-legend` names the component, the token values, and the authority (`production` · `spec <id>` · `delta`).
- **Tag strip per specimen.** `state · component · VC · tone/accent`.
- **CSS classes named after production components.** `.session-context-control*` = `SessionContextControl`, `.session-context-tip*` = its `Tooltip` content, `.session-inspector*` = `SessionInspector` on `DetailInspector`, `.session-context-meter*` = `SessionContextMeterSection`, `.stacked-progress*` / `.status-breakdown*` = the `@compozy/ui` primitives, `.session-context-injected*` = `SessionContextInjectedSection`, `.session-inspector-usage*` = `SessionInspectorUsageSection`, `.session-context-turns*` = `SessionContextTurnsSection`, `.session-activity*` = `SessionActivitySection`. Lab chrome is `lab-*` / `ss-contract` / `ss-n` / `ss-legend` / `ss-tags` / `sc-*` and never product UI.
- The window shell and the composer anatomy come from `../sessions-stability/sessions-stability.css` unmodified; nothing here redraws production chrome.

## Sources read before drawing

- Research: `.compozy/tasks/session-context/analysis/summary.md` and slices 02 (web surface), 03–08 (Synara, T3 Code, Orca, Pi, Superset, Emdash), 09 (ACP, adapters, Claude SDK, Zed, Cursor).
- Competitor references named in `_spec.md`: emdash `context-usage-indicator.tsx` (donut geometry, `size > 0` gate, popover copy), `live-models.ts` (retain last-known usage); t3code `ContextWindowMeter.logic.ts` (reserve the slot while loading); Zed `thread_view.rs render_token_usage` (ring + tooltip rows, 0.85 warning); Synara `contextWindow.ts` (latest snapshot, null after compaction); Pi `agent-session.ts` (`?/200k` unknown after compaction); Claude Code `analyzeContext.ts` (categories exist only inside the agent).
- Production: `session-composer-action-row.tsx`, `session-composer.tsx`, `session-inspector*.tsx`, `use-session-inspector-state.ts`, `use-session-topbar-slot.tsx`, `session-panel-toggle.tsx`, `use-session-window-controller.tsx`, `packages/ui` `detail-inspector.tsx`, `stacked-progress.tsx`, `status-breakdown.tsx`, `metric.tsx`, `empty.tsx`, `pill-types.ts`, `lib/tone.ts`, `tokens.css`, `DESIGN.md` §1–2.

## Component map (design → production)

| Verb | Component | Path | Board | VC |
| --- | --- | --- | --- | --- |
| new | `SessionContextControl` | `web/src/systems/session/components/session-context-control.tsx` | composer | task_03 VC-01..05 |
| new | `useSessionContext` | `web/src/systems/session/hooks/use-session-context.ts` | composer, sidebar | — |
| modify | `SessionComposerActionRow` (mount after `environmentControl`) | `web/src/components/assistant-ui/session-composer-action-row.tsx` | composer | VC-01..04 |
| modify | `SessionComposer` · `SessionThread` · `SessionWindowContent` (plumbing) | `web/src/components/assistant-ui/session-composer.tsx` · `session-thread.tsx` · `web/src/systems/os/apps/session/session-window-content.tsx` | composer | — |
| modify | `useSessionWindowController` (usage query ungated; ledger/vault wiring removed) | `web/src/systems/os/apps/session/use-session-window-controller.tsx` | both | — |
| modify | `SessionPanelToggle` aria copy | `web/src/systems/session/components/session-panel-toggle.tsx` | — | none |
| modify | `SessionInspector` (title "Context", no tabs, five sections) | `web/src/systems/session/components/session-inspector.tsx` | sidebar | task_03 VC-06 |
| new | `SessionContextMeterSection` | `web/src/systems/session/components/session-context-meter-section.tsx` | sidebar | VC-06 · task_04 VC-10..11 |
| new | `SessionContextInjectedSection` | `web/src/systems/session/components/session-context-injected-section.tsx` | sidebar | task_04 VC-10..11 |
| modify | `SessionInspectorUsageSection` (cache tiles; Files section deleted) | `web/src/systems/session/components/session-inspector-sections.tsx` | sidebar | VC-07 |
| new | `SessionContextTurnsSection` | `web/src/systems/session/components/session-context-turns-section.tsx` | sidebar | VC-08 |
| new | `SessionActivitySection` | `web/src/systems/session/components/session-activity-section.tsx` | sidebar | VC-09 |
| new | `useSessionUsageTurns` | `web/src/systems/session/hooks/use-session-usage-turns.ts` | sidebar | — |
| delete | `SessionInspectorMemorySection` | `web/src/systems/session/components/session-inspector-memory.tsx` | sidebar | — |
| delete | `SessionInspectorFilesSection` · `deriveFileReads` · tab strip · `InspectorMemoryState` | `session-inspector-sections.tsx` · `session-inspector.logic.ts` · `session-inspector.tsx` · `session-inspector-types.ts` | sidebar | — |
| reuse | `DetailInspector` · `Metric` · `Empty` · `Eyebrow` · `Pill` · `StackedProgress` · `StatusBreakdown` · `Collapsible` · `Button` · `Tooltip` · `Kbd` | `@compozy/ui` | both | — |

No new `@compozy/ui` primitive. `HoverCard` is not added; the compact hover surface is `Tooltip` content on a `Button` trigger. `IntensityMeter` is not reused (it means reasoning effort).

## Signal and state dictionary

| Glyph / state | Primitive + token | Where |
| --- | --- | --- |
| ring fill (reported) | SVG arc `--fg` on track `--line-strong`, 2px, r 6.5 | control |
| ring fill (≥ threshold) | arc + glyph `--warning` | control |
| ring stale | dotted arc `--subtle` | control |
| ring unknown | dashed track `--faint`, no arc | control |
| ring used-only | solid track + centre dot `--subtle` | control |
| state chip `reported` | `Pill` xs neutral | tooltip, meter |
| state chip `stale` | `Pill` xs warning | tooltip, meter |
| state chip `unavailable` | `Pill` xs neutral hollow; values stay | meter |
| state chip `estimated size` | sentence "Window from model catalog." (no chip) | tooltip |
| state chip `near compaction` | `Pill` xs warning | meter |
| tier Compozy context ≈ | `StackedProgress` segment tone `accent` (`--color-chart-1` alias) | meter |
| tier Agent & conversation | segment tone `neutral` (`bg-muted`, `--color-neutral`) | meter |
| tier Free | track `bg-canvas-tint` | meter |
| threshold tick | 1px `--warning` at `pressure_threshold`; no label — the line below the bar names the number | meter |
| injected row bar | `--accent`; stale rows `--accent-dim` | injected |
| "may have been summarized" / "estimate exceeds reported" | `text-warning` suffix | injected, meter legend |
| compaction marker | `Eyebrow`-sized row on `canvas-soft`, `Minimize2` 11px | turns, meter line |
| runtime warning row | `TriangleAlert` `--warning` | activity |
| empties | `Empty` compacted: 32px well, form title, micro description | meter, usage, turns |

Signal colour marks state only: warning for near-compaction, stale rows, runtime warnings. The three tiers are magnitude, not status (DESIGN.md §2).

## Copy (COPY.md register)

- Rail title: **Context**. Topbar toggle: "Open context sidebar" / "Close context sidebar".
- Control labels: "Context 35% used" · "Context 35% used, stale" · "Context 89.7K used" · "Context usage unknown".
- Tooltip: "35% · 89.7K / 256K" · "reported · as of turn 12" · "Compaction runs at 85%" (only with an agent-reported window) · "Window from model catalog." · "This agent hasn't reported context usage." · "Usage unavailable" (keeps the last ring).
- Meter legend: "Compozy context ≈" · "Agent & conversation" · "Free" · "estimate exceeds reported".
- Injected rows: "System prompt" · "Agent prompt" · "Situation" · "Memory" · "Soul" · "Skills catalog" · "Tool manuals" · "Network" · "Workspace knowledge" · "Attachments" (name + bytes, no estimate for binary) · "sent on turn N" · "re-sent on turn N" · "unchanged since turn N · last seen turn M" · "may have been summarized" · "modified by a hook" · "included in the startup prompt" (startup-dedup row without an estimate) · foot "Estimate: bytes ÷ 4 over the text Compozy delivered."
- Tokens & cost: existing labels; "Cache read" · "Cache write"; "No usage yet" kept.
- Turns: "Compozy compaction · at 85% · 218K · replay span archived" ("· replay span not archived" otherwise) · "Show earlier turns" · "No turns yet". One row per turn that has a usage report or a delivery (usage-only, counter-only, delivery-only, both), ordered by ledger sequence. The marker never says the agent's window shrank and never claims the attempt completed; the Compozy rows go stale only when the agent's own `used` drops.
- Meter empty: "No context report yet" · "The meter fills once the agent reports its first turn."

## Gaps and authorized deltas

- The boards draw the composer with the sessions-stability lane (Claude Code chip with an avatar); production shows the runtime selector trigger with `IntensityMeter` — host chrome, lossy by nature (SD-007).
- Injected rows on the sidebar board depend on task_04; task_03 proves the section with fixtures and renders it empty against a daemon without attribution.
- The drawer specimen is drawn at 760px to fit the page; the breakpoint is 1440 per `DetailInspector`.
- Numbers, turn ids, and agent names are fixtures; runtime truth owns values.

## Revision log

- 2026-09-11 · peer review round 1 (B-005, B-009): `unavailable` state added; the compaction sentence appears only with an agent-reported window.
- 2026-09-11 · peer review round 2 (B-015, B-018, B-019, B-020, N-007): compaction marker copy → "Compozy compaction · at N% · replay span archived / not archived" (no completion claim); rows stale only on the agent's `used` drop (never on the daemon marker); Turns = union of usage and delivery rows ordered by sequence; live refresh from the stream's `session_usage_changed` event, not from transcript entries; display tier Compozy = `min(injected, used, size)`; new row states "included in the startup prompt" and "Agent prompt". Board annotations §03/§05/§07 updated in place; the specimens' numbers are unchanged.

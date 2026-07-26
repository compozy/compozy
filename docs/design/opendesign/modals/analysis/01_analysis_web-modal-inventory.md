# Analysis: web-modal-inventory

> Research snapshot. Living modal authority: `../MODAL-STANDARD.md` + 16 surfaces in `../`.

Read-only exploration of the slice `web-modal-inventory` (ordinal `01`) for the research prompt:

> Alinhar todos os modais de criação/edição das páginas principais ao padrão novo (cabeçalho + conteúdo + formulário) já aplicado a task, trigger e job; usar sub-agentes para inventariar os modais e suas integrações de dados de backend antes de desenhar; produzir designs (HTML) por modal, com padrão impecável e menor complexidade (ex.: abas Simples/Avançado e versão simplificada para usuário-final).

## Scope

- Slice question: Inventory every core user-facing create/edit/add/configure dialog or sheet in the production Web UI. For each flow: entity, route/action, create-vs-edit coverage, current fields, frontend schema/defaults, hooks/adapters invoked, supported states, complexity risks, and whether the right future container is modal, sheet, or inline. Distinguish core entity flows from destructive confirmations, marketplace trust/install prompts, and transient operational dialogs.
- Primary sources: `web/src/routes/**` and `web/src/systems/**` (excluding tests, mocks, stories, generated files).
- Sources read in full vs. sampled: Read in full — `task-editor-modal.tsx`, `automation-editor-dialog.tsx`, `automation-job-form.tsx`, `agent-create-dialog.tsx`, `bridge-create-dialog.tsx`, `knowledge-create-dialog.tsx`, `knowledge-edit-dialog.tsx`, `session-create-dialog.tsx`, `network-create-channel-dialog.tsx`, `new-direct-dialog.tsx`, `mcp-server-editor.tsx`, `settings-editor-dialog.tsx`, `provider-edit-form.tsx`, `provider-inspector-sheet.tsx`, `loop-configure-sheet.tsx`, `-vault-page.tsx` (VaultEditor). Sampled (headers/grep) — `bridge-edit-dialog.tsx`, `channel-policy-dialog.tsx`, `-sandbox-page.tsx`, `scheduler-controls-panel.tsx`, `workspace-setup.tsx`, `onboarding-wizard.tsx`, marketplace + delete dialogs.
- Total candidate sources surveyed: ~45 dialog/sheet/form files across 14 systems.

## Overview

The Web UI has three tiers of create/edit surfaces. **Tier 1 (the reference / target pattern)** is fully realized in exactly three flows — Task, Job, and Trigger — matching the `@*-create-redesign.html` artboards: an `unframed` `DialogContent` sized by `--width-modal-*`/`--height-modal-*` tokens, a `DialogHeader variant="ruled"` carrying a 36px accent icon tile + `<Eyebrow>` scope label ("Autonomy · Task", "Automation · Job") + `DialogTitle` + `DialogDescription`, a body of numbered/flow sections with a density switch (Task's `ModeToolbar` simple/advanced; Job/Trigger's live preview rail), and a `DialogFooter variant="ruled"` with a left hint sentence + Cancel + primary. These live at `web/src/systems/tasks/components/task-editor-modal.tsx` and `web/src/systems/automation/components/automation-editor-dialog.tsx` (+ `automation-job-form.tsx`, `automation-trigger-form.tsx`).

**Tier 2 (partially aligned)** flows adopt the shell (`unframed`, ruled header/footer, modal tokens) but miss the signature header treatment (icon tile + eyebrow + description), the density switch, and the footer hint. This is the bulk of the redesign target: Knowledge create/edit, Session create, the shared `SettingsEditorDialog` shell (MCP server, Vault secret, Sandbox profile), Bridge edit, and the two Sheets (Provider inspector, Loop configure — the latter is actually already reference-grade). **Tier 3 (legacy)** flows use plain `<header className="border-b">`/step-counter chrome and multi-step wizards instead of a simple/advanced axis: Agent create (4-step) and Bridge create (3-step). Network create-channel and New-direct also sit here with plain headers.

Data integration is real and must be respected: every flow is driven by a system hook and typed adapter (e.g. `useTasksCreateModalForm`, `useAutomationJobForm`, `useAgentCreateDialogViewState`, `use-mcp-page`, `useResolveNetworkDirectRoom`, `useLoopConfigure`). Several flows carry heavy runtime-catalog state (providers/models loading/stale/refresh/error) that any redesign must preserve — Session create and Agent create in particular. The operator will most likely act on: (a) unifying the ~9 Tier-2/Tier-3 create/edit dialogs onto the Tier-1 header/footer + simple/advanced pattern, and (b) collapsing the two step-wizards (Agent, Bridge create) into the simpler sectioned form the Task modal proves works for dense entities.

Overlap with adjacent slices: this slice is the container/UX inventory; a sibling slice covering `web/src/systems/*/hooks`, `adapters`, `lib/schemas` and the Go/OpenAPI contract owns the exact request/response field lists. I cite the hooks/draft types by name but do not enumerate backend field-by-field payloads — that is out of this slice's read scope.

## Mechanisms / Patterns

- **Reference modal shell (Tier 1):** `Dialog` → `DialogContent unframed className="w-(--width-modal-md) … grid-rows-[auto_minmax(0,1fr)] h-(--height-modal-md)"` with `showCloseButton={false}`. Header = accent icon-tile (`size-9 rounded-lg bg-accent-tint text-accent-strong ring-1 ring-accent-dim`) + `Eyebrow` + `DialogTitle` + `DialogDescription`. Body = scrollable `NumberedSection`/`FlowStep` blocks. Footer = `DialogFooter variant="ruled"` with a hint span + Cancel(outline) + primary(min-w-32, spinner). Canonical in `task-editor-modal.tsx:116-323` and `automation-editor-dialog.tsx:143-159`.
- **Density switch (Simple/Advanced):** Task modal uses `ModeToolbar` (`task-editor-modal.tsx:145-155`, `formMode: "simple"|"advanced"`); advanced reveals Placement / Queue & ownership / Ingress / Execution sections (`:196-243`). This is the exact "reduce complexity for end-users" mechanism the operator wants generalized.
- **Live-preview rail (Job/Trigger):** two-column grid `grid-cols-[1fr_var(--width-right-rail-default)]` with a `JobPreview`/`trigger-preview` pane that renders next-runs, payload, rendered prompt (`automation-job-form.tsx:165-314`). An alternative to Simple/Advanced for schedule-heavy entities.
- **Wizard/stepper (legacy):** Agent create and Bridge create use a `<nav>` stepper with numbered/`Check` badges + Back/Continue + "Step N of M" footer, plain `<header className="border-b border-line px-5 py-3.5">` (no icon tile, no eyebrow, no description) — `agent-create-dialog.tsx:124-296`, `bridge-create-dialog.tsx:79-242`.
- **Shared settings editor shell:** `SettingsEditorDialog` (`settings-editor-dialog.tsx`) is a reusable create/edit modal (ruled header/footer, `Alert` feedback, `canSave`/`isSaving`/`saveLabel`) consumed by MCP server (`mcp-server-editor.tsx:65`), Vault secret (`-vault-page.tsx:180`), and Sandbox profile (`-sandbox-page.tsx:342`). It has ruled header but no icon tile / eyebrow / description-icon and no simple/advanced — upgrading this one shell lifts three flows at once.
- **Sheet (side drawer) pattern:** `Sheet`/`SheetContent side="right" grid-rows-[auto_1fr_auto]` for Provider inspector (tri-mode inspect/edit/create, `provider-inspector-sheet.tsx:62-110`) and Loop configure (`loop-configure-sheet.tsx:44-178`). Loop configure already implements the reference header (icon well + `Eyebrow "Loops · Configure"` + title + subtitle) and grouped body + ruled footer — it is the proof the pattern maps cleanly onto Sheets.
- **Local-state vs. lifted-draft forms:** Knowledge create/edit hold field state in local `useState`/`useReducer` (`knowledge-create-dialog.tsx:80-83`); Task/Job/Agent/Bridge lift a typed `draft` + `onDraftChange` to the route/page. Redesign should standardize on the lifted-draft shape to keep validation/preview consistent.
- **Runtime-catalog status surface:** Session create renders a `CatalogStatusLine` for loading/refreshing/stale/error/empty provider-model states (`session-create-dialog.tsx:244-304`); Agent runtime step + create dialog thread the same catalog props. Any redesign must retain these states.
- **RadioCard / PillGroup selectors:** Knowledge type grid uses `RadioCard` (`knowledge-create-dialog.tsx:142-152`); Job target/schedule use `PillGroup` (`automation-job-form.tsx:204-253`); Agent scope uses `PillGroup` (`agent-create-dialog.tsx:328-344`). Consistent primitive vocabulary already exists in `@agh/ui`.

## Relevant Sources

- `web/src/systems/tasks/components/task-editor-modal.tsx:65-323` — Tier-1 reference: modal shell + icon header + `ModeToolbar` simple/advanced + numbered sections + ruled footer hint.
- `web/src/systems/automation/components/automation-editor-dialog.tsx:83-159` — Tier-1: `--width-modal-xl` shell, `EditorHeader` icon-tile factory shared by Job & Trigger.
- `web/src/systems/automation/components/automation-job-form.tsx:159-346` — Tier-1: FlowStep body + live `JobPreview` right rail + ruled footer hint.
- `web/src/systems/agent/components/agent-create-dialog.tsx:115-300` — Legacy 4-step wizard (Basics/Runtime/Instructions/Access); plain header; no icon/eyebrow/description; `--width-modal-lg`/`--height-modal-tall`. Create-only (no edit variant here).
- `web/src/systems/bridges/components/bridge-create-dialog.tsx:70-246` — Legacy 3-step wizard (Provider/Runtime/Delivery); plain header + step counter.
- `web/src/systems/bridges/components/bridge-edit-dialog.tsx:65-90,376` — Long single-scroll edit form; ruled header WITH description but no icon tile/eyebrow; `sm:max-w-3xl`.
- `web/src/systems/knowledge/components/knowledge-create-dialog.tsx:116-227` — Tier-2: ruled header (no icon/eyebrow), RadioCard type grid + name/description/content textarea; local `useState`; `onConfirm` → controller; `sm:max-w-2xl p-0`.
- `web/src/systems/knowledge/components/knowledge-edit-dialog.tsx:74-147` — Tier-2 edit twin: description + content only; `useReducer` local state.
- `web/src/systems/session/components/session-create-dialog.tsx:116-304` — Tier-2: ruled header (no icon/eyebrow), `AgentCommandSelect` + `RuntimeSelector` + `CatalogStatusLine`; `--width-modal-sm`; no footer hint.
- `web/src/systems/network/components/network-create-channel-dialog.tsx:59-157` — Tier-3-ish: plain `border-b` header (not ruled variant, no icon), name/purpose/`AgentCommandMultiSelect`; `sm:max-w-120`.
- `web/src/systems/network/components/directs/new-direct-dialog.tsx:137-170` — Plain header; peer picker `Command` list; `useResolveNetworkDirectRoom`; navigates to the resolved direct room.
- `web/src/systems/network/components/shell/channel-policy-dialog.tsx:240-271` — "Delivery policy" configure dialog (Dialog).
- `web/src/systems/settings/components/settings-editor-dialog.tsx:36-134` — Shared create/edit shell (ruled header/footer, Alert feedback); no icon tile/eyebrow, no density switch.
- `web/src/systems/settings/components/mcp-server-editor.tsx:65-364` — MCP add/edit via `SettingsEditorDialog`; name/command/target/args/env; `use-mcp-page` (`MCPEditorState`, `MCPDraft`).
- `web/src/systems/settings/components/provider-inspector-sheet.tsx:33-290` — Provider inspect/edit/create Sheet; icon-well header (not the accent icon-tile+eyebrow); `ProviderEditForm` (`provider-edit-form.tsx:13-20` → general + auth fields).
- `web/src/systems/loops/components/configure/loop-configure-sheet.tsx:44-203` — Reference-grade Sheet: icon-well + `Eyebrow "Loops · Configure"` + grouped body + ruled footer; `useLoopConfigure`.
- `web/src/routes/_app/-vault-page.tsx:160-246` — Vault "New vault secret" via `SettingsEditorDialog` (ref/kind/secretValue password).
- `web/src/routes/_app/-sandbox-page.tsx:322-382` — Sandbox profile create/edit via `SettingsEditorDialog`.
- `web/src/systems/workspace/components/workspace-setup.tsx:5-31` — "Add workspace" `Dialog` (entity create) wired at `-app-shell.tsx:108-110`.
- `packages/ui/src/tokens.css:329-350` — modal size tokens (`--height-modal-md 760`, `-tall 900`, `-wizard 960`, `-xl 840`; `--width-modal-sm 560`, `-md 720`, `-lg 880`, `-xl 1180`).
- Route wiring: `web/src/routes/_app/-app-shell.tsx:113-160` (Agent + Session hosts), `-tasks-new-route.tsx:19`, `-tasks-edit-route.tsx:36`, `bridges.tsx:127`, `bridges.$id.tsx:24`, `knowledge.tsx:183`, `settings/-providers-settings-page.tsx:182`, `loops.$name.configure.tsx:72`, `mcp.tsx:171`, `network.$workspaceId.$channel.directs.tsx:112`.

## Transferable Patterns

- **Tier-1 modal shell (icon tile + eyebrow + description header, ruled footer + hint)** → apply to Knowledge create/edit, Session create, Bridge edit, Network create-channel, Channel policy, and the `SettingsEditorDialog` shell. Replaces their plain/ruled-but-bare headers with the "Domain · Entity" eyebrow + description the artboards mandate.
- **`ModeToolbar` Simple/Advanced density switch** (`task-editor-modal.tsx:145`) → applies to Agent create, Bridge create/edit, MCP server editor, and Session create because those are the densest forms; a Simple view (name + one or two essentials) plus an Advanced reveal is exactly the "usuário-final" affordance the operator described, and it lets both step-wizards collapse into one sectioned form.
- **`EditorHeader` icon-tile factory** (`automation-editor-dialog.tsx:143-159`) → lift into a shared `@agh/ui`/domain header component so every create/edit modal renders the same header from `{icon, eyebrow, title, description}` instead of re-implementing chrome per dialog.
- **Shared `SettingsEditorDialog` upgrade** (`settings-editor-dialog.tsx`) → upgrading this single shell propagates the new header/footer to MCP server + Vault secret + Sandbox profile simultaneously (three flows, one change).
- **Loop-configure Sheet header** (`loop-configure-sheet.tsx:52-78`) → the canonical Sheet realization of the pattern; reuse it verbatim to bring `provider-inspector-sheet.tsx` (currently a plain icon-well header) to parity.
- **Live-preview rail** (`automation-job-form.tsx:165-314`) → transferable to any create flow with computable output (e.g. Trigger, and potentially a future scheduled-Job-like entity); an alternative density strategy to Simple/Advanced.
- **`RadioCard` type-selector grid** (`knowledge-create-dialog.tsx:136-153`) → a clean "pick a kind" step reusable for Bridge provider and Agent scope selection in a simplified single-screen form.

## Risks / Mismatches

- **Truthful-UI constraint (web/CLAUDE.md "Truthful UI > plausible UI"):** designs must not add fields/controls the backend doesn't accept. Field lists are owned by system hooks/draft types (`TaskEditorDraft`, `CreateAutomationJobRequest`, `AgentCreateDialogDraft`, `BridgeCreateDraft`, `MCPDraft`, `ProviderDraft`, `VaultDraft`, `NetworkCreateChannelDraft`) and the Go contract — a redesign must reuse the existing draft shape, not invent inputs. This slice deliberately does not enumerate backend payloads; that's a sibling adapter/contract slice.
- **Runtime-catalog states are load-bearing:** Session create's `CatalogStatusLine` (`session-create-dialog.tsx:244-304`) and Agent runtime step encode loading/refreshing/stale/error/empty/refresh-error. A "simplified" redesign that drops these degrades correctness — the Simple view must still surface provider/model availability truthfully.
- **Wizard→sectioned-form is not free:** Agent create (`:63-68`, 4 steps with per-step `canAdvance` gating) and Bridge create (`:54-59`, per-step `stepValidity`) encode step-scoped validation. Flattening to Simple/Advanced must preserve field-level validation and the "can't submit until X" gates, or it regresses.
- **Container choice conflicts:** Provider (`provider-inspector-sheet.tsx`) and Loop configure (`loop-configure-sheet.tsx`) are Sheets by design (inspect + edit + create in one surface / side-by-side with the list). Forcing them into centered modals would break their inspect/edit affordance. The redesign should keep Sheet where inspect-alongside-list matters and Modal where it's a focused create.
- **`SettingsEditorDialog` is multi-tenant:** its `max-h-[60vh]` body and `SettingsFieldRow`-based layout are shared by MCP/Vault/Sandbox; a header/footer redesign is safe but changing the body layout risks all three consumers at once — verify each (`mcp-server-editor.tsx`, `-vault-page.tsx`, `-sandbox-page.tsx`).
- **Scope-out: not core-entity flows.** Exclude from the redesign target (per the slice's distinction): marketplace trust/install prompts — `extension-trust-dialog.tsx:49` ("Install X?"), `bundle-activation-dialog.tsx:66` ("Activate X"), `mcp-install-dialog.tsx:74` ("Install X"); destructive confirmations — `knowledge-delete-dialog.tsx`, `mcp-server-delete-dialog.tsx`, `-vault-page.tsx` VaultDeleteDialog, `-sandbox-dialogs.tsx` (`ConfirmDialog`), `task-delete-action.tsx`, `use-agent-delete-flow.tsx`; transient/operational — `scheduler-controls-panel.tsx:277` ("Pause scheduler?"), `restart-daemon-button.tsx`; setup flows — `onboarding-wizard.tsx` (full-page, not a modal), `workspace-onboarding`. These share the shell but are not "create/edit an entity" forms and should not get the Simple/Advanced treatment.
- **No design-token invention:** DESIGN.md/tokens.css govern sizes and the accent tint header; reuse `--width-modal-*`/`--height-modal-*` and `bg-accent-tint`/`ring-accent-dim` — do not introduce new modal sizes or shadows.

## Open Questions

- Does the operator want the two step-wizards (Agent create, Bridge create) **converted to Simple/Advanced sectioned modals**, or kept as wizards but re-skinned with the Tier-1 header/footer? Both satisfy "impecável"; they differ in effort and end-user friction.
- Should **Provider** and **Loop configure** stay Sheets (recommended — inspect-alongside-list) or be normalized to modals for cross-flow consistency? Loop configure is already reference-grade; Provider needs only a header upgrade.
- Is **New direct room** (`new-direct-dialog.tsx`) in scope? It's an entity-create (opens a direct room) but is really a peer-picker; it may belong with transient dialogs rather than the redesign set.
- The exact backend field lists / defaults / validation per entity live in the system hooks, `lib/schemas`, adapters, and the Go/OpenAPI contract — out of this slice's read scope. A sibling slice must confirm them before any HTML proposes a field, so designs don't render inputs the runtime rejects.
- Is there a desired **Agent edit** modal? Today create is a modal (`agent-create-dialog.tsx`) but edit is a full route (`agents.$name.settings.tsx`), an asymmetry the redesign may want to reconcile.

## Evidence

- `web/src/systems/tasks/components/task-editor-modal.tsx`
- `web/src/systems/automation/components/automation-editor-dialog.tsx`
- `web/src/systems/automation/components/automation-job-form.tsx`
- `web/src/systems/automation/components/automation-trigger-form.tsx`
- `web/src/systems/automation/components/automation-operations-page.tsx`
- `web/src/systems/agent/components/agent-create-dialog.tsx`
- `web/src/systems/agent/components/agent-create-access-step.tsx`
- `web/src/systems/agent/components/agent-create-runtime-step.tsx`
- `web/src/systems/bridges/components/bridge-create-dialog.tsx`
- `web/src/systems/bridges/components/bridge-edit-dialog.tsx`
- `web/src/systems/knowledge/components/knowledge-create-dialog.tsx`
- `web/src/systems/knowledge/components/knowledge-edit-dialog.tsx`
- `web/src/systems/knowledge/components/knowledge-delete-dialog.tsx`
- `web/src/systems/session/components/session-create-dialog.tsx`
- `web/src/systems/network/components/network-create-channel-dialog.tsx`
- `web/src/systems/network/components/directs/new-direct-dialog.tsx`
- `web/src/systems/network/components/shell/channel-policy-dialog.tsx`
- `web/src/systems/settings/components/settings-editor-dialog.tsx`
- `web/src/systems/settings/components/mcp-server-editor.tsx`
- `web/src/systems/settings/components/mcp-server-delete-dialog.tsx`
- `web/src/systems/settings/components/provider-inspector-sheet.tsx`
- `web/src/systems/settings/components/provider-edit-form.tsx`
- `web/src/systems/settings/components/provider-edit-form-general-fields.tsx`
- `web/src/systems/settings/components/provider-edit-form-auth-fields.tsx`
- `web/src/systems/loops/components/configure/loop-configure-sheet.tsx`
- `web/src/systems/scheduler/components/scheduler-controls-panel.tsx`
- `web/src/systems/workspace/components/workspace-setup.tsx`
- `web/src/systems/onboarding/components/onboarding-wizard.tsx`
- `web/src/systems/marketplace/components/extension-trust-dialog.tsx`
- `web/src/systems/marketplace/components/bundle-activation-dialog.tsx`
- `web/src/systems/marketplace/components/mcp-install-dialog.tsx`
- `web/src/routes/_app/-app-shell.tsx`
- `web/src/routes/_app/-vault-page.tsx`
- `web/src/routes/_app/-sandbox-page.tsx`
- `web/src/routes/_app/-sandbox-dialogs.tsx`
- `web/src/routes/_app/-tasks-new-route.tsx`
- `web/src/routes/_app/-tasks-edit-route.tsx`
- `web/src/routes/_app/bridges.tsx`
- `web/src/routes/_app/bridges.$id.tsx`
- `web/src/routes/_app/knowledge.tsx`
- `web/src/routes/_app/mcp.tsx`
- `web/src/routes/_app/loops.$name.configure.tsx`
- `web/src/routes/_app/settings/-providers-settings-page.tsx`
- `web/src/routes/_app/network.$workspaceId.$channel.directs.tsx`
- `packages/ui/src/tokens.css`

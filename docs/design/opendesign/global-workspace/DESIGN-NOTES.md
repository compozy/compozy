# Global-workspace design set — shared contract

Problem: "Global" surfaces today as four disconnected encarnations — a pinned "Home workspace"
row in workspace lists, a "Use global workspace" card in setup/onboarding, a Global|Workspace
scope pill-group (re-implemented 3×) in create modals, and Workspace/Global radios in
marketplace install sheets. Selecting the home row inside a ScopeSelector silently flips scope
to global without firing `onChange` (`workspace-command-select.tsx:74-82`) — the canonical
confusion this set removes.

Redesign: **one control owns scope** — a small toggle in the OS menubar, next to the workspace
chip. ON → global scope (the user's `~`). OFF → the selected workspace. Every other surface
*derives* the target and states it; none re-selects it.

Every file links `../design-system/ds-core.css` → `ds-shell.css` → `global-workspace.css`
(this folder). Page-local styles stay inline and small. Lucide via CDN + `createIcons()`.

## Shared data story (same entities across every artboard)

User home `~` = `/Users/pedro`. Workspaces: **compozy** (`~/dev/compozy`, monogram CO,
selected), **branas-site** (`~/dev/branas/site`, B), **notes** (`~/notes`, N).
Remembered workspace across toggle flips: compozy. Agent story: "Fix payment retries" (claude).
Marketplace entry for install demos: **github** MCP server. Automation demos: job
"Nightly triage", webhook trigger "Deploy hook".

## Locked decisions (apply everywhere)

- **One owner.** Scope = `{workspace_id}` or `{global}` is a single runtime state owned by the
  menubar toggle (`.gw-toggle`, 28px, house icon, directly after `.ws-trigger`). No other
  surface renders a scope *control* — pill-groups, radios, selects and option cards for
  global-vs-workspace are deleted, not restyled.
- **State contract.** `active-workspace-store` gains `scope: "workspace" | "global"` beside
  `selectedWorkspaceId` (persisted, `compozy:active-workspace:v2` → `v3`). The workspace id is
  *remembered* while global is on — toggle-off returns to it. `taskScopeForActiveWorkspace` /
  `homeScopeForActiveWorkspace` read the store scope instead of home-dir detection.
- **Global is not a workspace row.** The `$HOME`-rooted workspace disappears from every list:
  no "Home workspace" badge, no pinned home row, no home card in the Workspaces overview, no
  `Home`-icon avatar branch. `splitHomeWorkspace` / `isHomeWorkspace` are delete targets, not
  render branches. Lists show project workspaces only.
- **Chip truth.** Toggle OFF → chip = monogram + workspace name (production verbatim). Toggle
  ON → chip = house glyph + **Global**. The chip never shows a scope that is not active; the
  remembered workspace resurfaces only on toggle-off. Accent lives on the pressed toggle only —
  the chip stays neutral (accent is a state indicator, and the toggle *is* the state).
- **Menu interplay.** The workspace menu never lists global. While global is on: no row is
  checked, one info-tone notice explains it ("Global scope is on — picking a workspace turns it
  off."), and selecting any workspace turns global off and switches in one gesture.
- **Derived-target statement.** Create/install surfaces state where the action lands via
  `.gw-scope` (chip) or `.gw-scope-note` (dialog foot): "Creates in **compozy**." /
  "Creates in **Global** — visible to every workspace." Read-only: no chevron, no popover,
  no pressed state. Scope changes happen at the menubar, period.
- **Copy register (locked).** Control name **Global scope**; chip label **Global**; global
  descriptor "visible to every workspace" (installs: "available in every workspace"). Retired
  forever: "Home workspace", "operator home", "Use global workspace", "Home" badge, the
  Globe icon (global's icon is the **house** — it ties the mode to `~`), and every
  Global|Workspace pill/radio/select.
- **Zero-workspace truth.** With no registered workspaces, global is on and the toggle is
  locked-on (tooltip names the reason: "Add a workspace to scope down"). The "No workspace"
  chip state and the arbitrary `workspaces[0]` fallback are deleted — the fallback is global.
- **Onboarding consequence.** Global exists out of the box, so onboarding registers *project*
  folders only and may finish with zero ("Skip — start in Global scope"). The
  `workspace-setup` global OptionCard and the old "Use home directory" card are deleted.
- **Exceptions stay honest.** Webhook triggers remain always-global with their existing alert
  (`automation-trigger-form.tsx:114-121`) — stated, not selectable. Entity scope is immutable
  after create: edit surfaces echo the entity's own scope and ignore the toggle. Read-side
  *filters* (jobs/triggers/bridges list Scope filter) survive — they describe queries, not
  targets. Settings→Skills Global|Agent is a different axis — untouched.
- **Keyboard.** `⇧⌘G` toggles global scope; the toggle is `role="button"` +
  `aria-pressed`; the ⌘K palette carries "Turn on/off Global scope" and workspace switches
  (switching notes "turns Global scope off" while it is on).
- **Gating comments:** every distinct control per artboard cites its production source or store
  field at least once (`gating:` prefix). Spec scaffolding (file paths, delete targets) lives
  in `.spec__note` columns and HTML comments only — never inside a mock's fidelity boundary.
- Provenance comment at the top of every file: production / redesign proposal / authorized delta.

## Change map (production, from the web/src audit)

| Surface | File | Fate |
| --- | --- | --- |
| Menubar chip | `os-menubar.tsx:134-144` | + `.gw-toggle` after chip; chip gains global identity state |
| Workspace menu | `os/components/menubar/workspace-menu.tsx:32-59` | home row removed; global-on notice + uncheck state |
| Workspace command-select | `workspace/components/workspace-command-select.tsx:74-82,186-196` | home branch, badge, scope-context handoff deleted |
| ScopeSelector + contexts | `workspace/components/scope-selector.tsx` (+2 context files) | **deleted** — call sites derive from store |
| Agent create scope pills | `agent/components/agent-create-dialog.tsx:134-181` | deleted → foot `.gw-scope-note` |
| Automation job/trigger scope | `automation-job-form.tsx:180-192`, `automation-trigger-form.tsx:87-121` | deleted → toolbar `.gw-scope`; webhook alert stays |
| Bridge create scope | `bridges/components/bridge-create-dialog.tsx:179-190` | deleted → foot note |
| Task editor scope | `tasks/components/task-editor-surface.tsx:137-148` | deleted → toolbar `.gw-scope` |
| MCP install radios | `marketplace/components/mcp-install-dialog.tsx:119-146` | deleted → foot note; vault ref derives (`use-mcp-install-dialog.ts:50-55`) |
| Marketplace config-scope pills | `marketplace-kind-page.tsx:153-172` | deleted → derived; `config_scope` search param dropped |
| Task bridge subscription scope | `task-bridge-subscription-create-dialog.tsx:123-154` | select + ws-id echo deleted → foot note |
| Session create workspace field | `session-create-advanced-section.tsx:41-60` | picker → derived statement (global session runs at `~`) |
| Workspace setup global card | `workspace-setup-location-pane.tsx:43-76` | **deleted**; copy in `workspace-setup-copy.ts` rewritten |
| Onboarding step | `onboarding/components/step-workspaces.tsx` | project folders only + skip-to-global |
| Workspaces overview | `os/components/os-workspaces-overview.tsx` | home card gone; counts = project workspaces |
| Store | `workspace/stores/active-workspace-store.ts` | + `scope` field, v3 key; `workspace-scope.ts` / `home-scope.ts` read it |
| Home-workspace lib | `workspace/lib/home-workspace.ts` | **deleted** (with `use-user-home-dir` call sites re-audited) |
| Layout profile scope | `settings/components/layouts/layout-profile-editor.tsx:17-28` | derives from toggle too (segmented deleted) — phase-2 candidate |

## Files

Finals: menubar-toggle, workspace-menu, modal-scope, onboarding.
Labs: current-audit (exhibits of everything removed), states (edge cases + matrix).
`index.html` maps finals × labs; finals cross-link each other through the shared story.

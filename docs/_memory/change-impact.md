# Compozy Change Impact

## Memory removal — 2026-10-07

Usage-stream follow-up (US-019 / BR11): the shared HTTP/UDS session stream emits the existing `session_usage_changed` payload for every persisted `compaction` snapshot and `session.compaction_fired` attribution, so Web invalidates the context reading and markers before prompt completion. Push and polling use the existing per-session cursor; no new DTO, native tool, hook/config, workspace scope, schema, or official-skill instruction is needed. Existing `TestWriteUsageChangedEvents` owns ordered mid-turn notification and replay deduplication; `ET-web-session-context-meter` now explicitly requires the mid-turn update. Browser scenario execution remains with the controller/Web QA owner.


Owning decision: `.compozy/tasks/memory-removal/_spec.md` with ADR-001 (hard cut that never blocks
the user), ADR-002 (CompozyOS-side compaction removed; every replay bounded), ADR-003 (workspace
knowledge augmenter removed), and ADR-004 (native ACP compaction observed and requested,
experimental). Release: the release after v0.3.0 (assumed v0.4.0); guide anchor `#memory-removal`.
This is the user's recorded exception to the SD-013 public-surface window for the memory family;
leftovers in user-owned files are retired without blocking the user.

- **Native tools / CLI / HTTP / UDS / MCP / SDK:** the `compozy memory` tree, 40 `/api/memory*`,
  settings, and workspace-ledger operations, the 35 `compozy__memory_*` tools with the
  `compozy__memory` and `compozy__memory_admin` toolsets, MCP `compozy_host__memory__*`, the Host API
  `memory/*`, `memory.backend`, consent `memory:read|write`, the `memory-backend-ts` scaffold, and the
  SDK members are deleted; tool IDs are retired permanently. Added, all `experimental`:
  `compozy session compact`, `compactSession`, and `compozy__session_compact` (toolset
  `compozy__sessions`, risk `mutating`; refusals carry the structural `session_busy` and
  `compaction_unsupported` codes on every surface, and the request is attributed `requested_by`
  `cli`, `http`, `tool`, `goal`, or `web`). Changed: `session.compaction_fired`, usage `compactions[]`
  markers (no `pressure_threshold`; `context_used`/`context_size` are omitted when unknown), the
  session `type` enum (no `dream`), the roles roster (`coordinator`, `auto_title`), and
  `StatusSchemaVersion` `2026-10-07` without `memory`. `GET …/history` and `compozy session history`
  keep returning raw grouped ledger rows (`compaction` snapshot rows plus `session.compaction_fired`);
  only the transcript projection folds one Compaction item per `compaction_id`. After a native terminal
  compaction, the session usage API reports occupancy `unknown` until a later usage update carries
  `used`; the Goal context readers stay unknown until an update carries both `used` and a positive
  `size`.
- **Extensibility / hooks / config:** `context.pre_compact` / `context.post_compact` become
  observation-only (`labels` patch only) with a `compaction_trigger` matcher replacing
  `compaction_reason` / `compaction_strategy`. `[memory]`, `[session.compaction]`, four roles, and the
  `memory.consolidated` trigger event are removed; `[session.derive]` bounds every replay.
  **v0.6.0 shim deletion list** (one shim generation, regime: public surface / user state):
  the `archiveRetiredMemorySettings` config archive (global, profile, workspace overlays), the SOUL
  `memory_policy` ignore rule, the retired tool-ID filter (`RetiredMemoryToolIDs`,
  `RetiredMemoryToolsetIDs`, `DropRetiredToolReferences`), and the extension manifest retired-entry
  filter (`dropRetiredMemoryManifestEntries`), plus AGENT.md and SKILL.md retired hook-matcher
  filters (`ignoreRetiredAgentHookMatchers`, `ignoreRetiredSkillHookMatchers`) for
  `compaction_reason` / `compaction_strategy`, warning once per owner. All of those boundary
  filters expire in v0.6.0. Session migration `00009` and global migration
  `00130` stay as migrations. Alongside the retired-ID tombstones, the session recovery list `retiredInternalSpawnRoles` records exactly
  `memory-extractor` and `checkpoint-summary`. Its recovery boundary is permanent and does not expire in
  v0.6.0 because retained session directories are never deleted.
- **Workspace data isolation:** no workspace or profile boundary changes. Migration `00130` drops the
  memory stream tables, `memory.consolidated` triggers and their dependents, and legacy `dream` /
  `memory-extractor` sessions; session migration `00009` restores events archived by the removed
  compaction (never inside a rewind receipt range). No file is deleted or rewritten: Markdown
  memory, `knowledge/` directories, and `ledger.jsonl` files stay on disk, unread. `<workspace>/knowledge/`
  is no longer injected into prompts. Disk recovery refuses non-empty unsupported session types or
  explicitly retired internal spawn roles before durable-owner verification or metadata normalization. It logs
  `observe.session_recovery_skipped` once per session per observer lifetime, and leaves those session directories unchanged. This is a permanent
  recovery boundary; ordinary user sessions and custom advisory roles such as `reviewer` remain recoverable
  without catalog rows.
- **Review round 1 boundary repairs:** session upgrade restores physically archived transcript entries
  and tool routes using persisted identities, preserves rewind exclusions and live projection state,
  and advances the projection generation once. Config retirement follows trigger array-element
  ownership across noncontiguous descendant tables, preserving retained filters. Installed extension
  automation resources ignore `memory.consolidated` before validation with
  `extension.retired_entries_ignored`; explicit API/CLI trigger creation remains rejected. Nested
  derive carries inherited omission evidence so the available history tool points to the immediate
  source session. Archive publication guards and permission failures warn with path/reason and leave
  retired settings inactive in the validated in-memory overlay; the next load retries publication.
  Existing explicit retired-key config writes remain refused. Migration `00130` also removes persisted
  `checkpoint-summary` roles while preserving advisory roles. The oldest-first suffix policy protects
  the last eight replay messages while framing and the first-user pin fit; its unused `KeepRecent` field
  is removed without changing the drop order. Layout reconciliation logs the profile, dropped app IDs and rewritten route count
  after persistence. These repairs add no native tools, hooks, config keys, or workspace/profile
  boundary changes; Web transcript reads/search/anchors retain restored history through the existing
  contracts, and the migration guide's full-history promise remains unchanged.
- **Official skill / Web / docs:** `skills/compozy/references/memory.md` is deleted and every memory
  or CompozyOS-compaction mention is removed; `compozy__session_compact` and Compact now are
  documented. Web drops the Knowledge app, Settings → Memory, the Home Memory tile, memory role
  panels, and the context-meter threshold warning, and gains the Compact now button (context rail meter
  section, no popover), the Compaction timeline item, agent/requested compaction markers, and the
  post-compaction meter sentence. Site: memory docs, the `cli/memory` subtree, landing Memory/Dream
  content, and the `defining-agent-sessions-compozyos` memory section are removed; new
  `sessions/compaction.mdx`; migration guide (root and site) and a breaking release note ship.
  COPY, README, PRODUCT, and the glossary drop memory vocabulary and gain a Compaction entry. Dated
  history (release notes, QA reports, `_done` boards, lessons) stays untouched.
- **QA / verification:** memory-only scenarios, journeys, and charters are deleted; compaction
  scenarios are rewritten; the retirement canary `ET-retired-product-surfaces-absent` covers memory,
  Dream, Knowledge, and CompozyOS-side compaction, including an upgrade leg. The upgraded-home lab
  walk, Compact now, and the real-adapter Goal compaction walk own the user-visible journeys.

## Automations — Jobs and Triggers merged in the Web UI — 2026-10-08

Owner: spec `.compozy/tasks/automations/` (ADR-001..004); one PR from branch `automations`.
The Web UI's Jobs and Triggers apps become one `automations` app (listing, detail, editor, dock,
palette, Settings › Automation row, Loop "Automate ▾"); the daemon keeps both entities and every
canonical noun (`job`, `trigger`). Native tools: ids unchanged;
`compozy__automation_{jobs,triggers}_{list,get}` results gain `last_run`, the two list tools gain an optional `target` input
(`agent|loop|task`) and one description clause naming the UI noun; `native-tool-catalog.json`
regenerates. Extensibility/hooks/config: hooks, trigger events, extension manifests and SDK types
unchanged; `window_layout` resources naming `jobs`/`triggers` are aliased to `automations` with
WARN `windowmanager.app_id_deprecated` until v0.5.0 (none in-repo); `config.toml` keys unchanged,
only Settings labels change. Public inputs on the one-release ladder (removal v0.5.0): web routes
`/jobs*`, `/triggers*` (redirect stubs), CLI `--app jobs|triggers` (stderr warning), window commands,
palette ids `app.open.{jobs,triggers}` / `palette.view.{jobs,triggers}` (WARN
`cmdpalette.command_id_deprecated`). User state: saved desktops migrate permanently through snapshot
v5 (no window closed); palette pins/recents/usage merge through Goose `00129`; `00128` adds the
latest-run indexes and the catalog `target` column. Workspace data isolation: `last_run` is computed
per owner column (`job_id` / `trigger_id`) inside the existing scoped list query; workspace/profile
filters and the other-project detail state are unchanged. Official skill: `SKILL.md` routing row
("Automations (jobs and triggers)"), `references/native-tools.md` (UI noun mapping, `last_run`,
`target`), `references/window-management.md` (retired app ids). Docs: site category
`automation/` retitled **Automations** with tutorials `run-an-agent-every-morning` and
`react-when-a-session-fails`; `jobs.mdx` → `schedules.mdx`, `triggers.mdx` → `events.mdx` (permanent
redirects in `packages/site/next.config.mjs`); inbound links, `loops/catalog.mdx` Automate menu and
`workspaces/window-management.mdx` deprecation note updated; generated CLI reference regenerates from
Cobra. COPY.md and glossary close the pending "Jobs / Triggers" alias row (Automations) and add a
glossary `Automation` entry. Release note `.release-notes/one-automations-window-*.md` carries the
migration block. QA: `TA-web-automations-listing`, `TA-web-automations-first-run`,
`TA-web-automation-detail`, `TA-web-automation-editor`, `TA-web-automations-shell-entry`,
`TA-web-automations-upgrade`, `TA-automation-last-run-agent` replace `ET-web-jobs-triggers-catalog`,
`ET-web-trigger-detail-rule-page`, `TA-web-jobs-zero-inventory-suggestions`,
`TA-web-triggers-zero-inventory-intro`, and `TA-web-automation-preview-toggle`; entry points of
`TA-automation-crud-loop-target`, `TA-scheduled-session-restart-recovery`, `ET-palette-domain-views`,
`ET-web-ui-resilience`, `ET-web-catalog-navigation`, and `TA-056` were flagged.

Fix round 1: native list descriptions preserve the canonical job/trigger nouns and UI mapping.
Shared retired-route vectors preserve explicit queries and leave nested detail paths unchanged.
No wire, config, hook, workspace isolation, or official skill contract changes.

## Modern Go adoption across module boundaries (#482) — 2026-10-06

Owner: this refactor PR (branch `modern-go-482`; issue #482). Behavior-preserving migration of
every Go module (root at Go 1.27.1, `sdk/go`, both SDK examples and both extension fixture modules
at Go 1.26.4) plus `magefiles`, with continuous enforcement: `make go-lint` now lints all modules,
`magefiles` (`mage` tag) and an `integration`-tag Modern Go pass; `forbidigo`/`depguard`/
`copyloopvar` cover the guideline gaps `modernize` does not; `scripts/gate.sh` and CI select nested
module lanes. Native tools: none — `compozy__*` descriptors, schemas and digests are byte-identical
(`make codegen` produced no generated diff). Extensibility/hooks/config: no interface, hook-order,
registry-order or `config.toml` key change; SDK and fixtures are now linted. Workspace isolation:
unchanged. JSON: 1,238 scalar/pointer tags moved to `omitzero` only where `encoding/json` output is
byte-identical; strings/collections, `IsZero` types, interfaces and generated or SDK-generator source
tags keep `omitempty`. Official skill and Web: unchanged. Docs: new developer runbook
`docs/runbooks/modern-go.md` (rule → enforcement map and retained exclusions). QA: no user-visible
behavior, no scenario change.

## Continue/fork runtime selection and top-level placement — 2026-10-06

Owner: this fix PR (branch `fix-continue-agent`; spec `.compozy/tasks/session-continue-fork/`).
A continued or forked child records its explicit runtime/route (fork: the source runtime) as
`runtime.selected` at revision 1, so the unbound child's composer and a runtime-less first prompt
bind the chosen runtime. Continue/fork lineage is provenance, not hierarchy: one predicate
(`store.HierarchyParentSessionID`, web `sessionTreeParentId`) keeps those sessions out of the
sidebar/catalog tree, the supervision active-child signal, and Goal descendant authorization.
Spawned, provenance, and recovery children keep nesting. No SQLite or wire shape changes:
`lineage.kind` (column + meta, backfilled by #684) already distinguishes the edge, and
`parent_session_id` keeps naming the source for the origin pill, the CLI, and the `parent=`/`root=`
filters (which match stored fields as-is). Native tools, hooks, config, and extensions keep their
shapes; workspace isolation is unchanged. Docs: `sessions/lifecycle.mdx`; official skill:
`runtime-operations.md`. QA: `ET-web-session-continue` (steps 10–11), `ET-web-session-fork-from-here`
(reset to untested), `ET-web-session-sidebar-threads` and `ET-web-sessions-catalog-modal` (flagged).

## Live model catalog freshness — 2026-10-06

Owner: `docs/qa/scenarios/MS-live-model-release-refresh.md`.
Claude live discovery now names version aliases after the release their provider label
advertises (`opus` + "Opus 5.5" -> `claude-opus-5-5`, transport binding stays `opus`) and curates
the advertised list, featuring aliases over pinned releases; user curation in `config.toml`
still outranks it. ACP model inspection keeps the advertised list when a per-model option probe
is rejected or runs out of budget (`acp.SessionModelInspection.ModelErrors`, internal); such a
model reports unknown reasoning with a row `last_error` rather than static seed levels. The Web
composer refresh sends `force: true`, matching the settings refresh, and reports failed sources
of signed-in providers that a 200 aggregate refresh returns. HTTP/UDS/CLI routes, DTOs,
`compozy__provider_models_*` tools, hooks, config keys, extension SDKs and SQLite shape are
unchanged; persisted selections of the bare alias (`opus`) still launch because unknown Claude
ids fall through to the transport value. Workspace/profile catalog contexts are untouched. No
official `skills/compozy/` or site documentation change is required.

Explicit caller cancellation aborts ACP discovery, including during its final option probe;
deadline exhaustion still preserves the advertised list. Only aliases resolving to Claude
release IDs receive live featured status; custom transport IDs remain ordinary curated rows.

## Dependency upgrades — 2026-10-05

Owner: `docs/qa/reports/2026-10-05-dependency-upgrades.md`.
Go/CEL/Bubble Tea/ULID and Bun library upgrades retain CLI, HTTP/UDS, native-tool IDs,
hooks, extensions and config contracts. Native TypeScript 7 remains the checker while the
OpenAPI generator uses its supported JavaScript compiler API. Profile input/storage decoders
translate six renamed Lucide slugs losslessly; no profile ID, ownership, workspace isolation,
database shape or stored payload is discarded. The same canonical slug reaches Web and native
profile reads. The compatibility decoder is scheduled for removal in v0.5.0 after persisted
names have been migrated. Official `skills/compozy/` instructions remain accurate.
Web composer hydration waits for the assistant-ui commit before persisting observations,
preserving per-session/project drafts. Catalog publication preserves existing compressed bytes
when decompressed package content is identical across Go toolchain changes. The profile identity
and composer text-entry scenarios plus existing CLI/profile/catalog suites own these journeys.
Desktop restoration commits window frames before mounting layout-heavy bodies, preserving
window identities, geometry, overlays and gestures. The existing 12-window performance E2E
and desktop-shell lifecycle scenario own this Web-only scheduling change; native tools,
data, public contracts, official skills and site documentation need no additional changes.
Release validation additionally repairs run-agent lane cleanup ordering: terminal binding
settlement records its cause before generic cancellation, while returned session IDs and
workspace/epoch ownership stay intact. `compozy__loop_node_cancel`, CLI/HTTP/UDS cancellation
and the Web control retain their shapes and semantics. No schema migration, historical data
rewrite, hook/config/extension change, or official skill/site update is required. The existing
binding integration suite and `LP-per-lane-node-control` own this internal settlement invariant.
HTTP/UDS request cancellation and notification-ledger fault injection are test-harness changes.
Web route entry now follows a live replacement of the remembered-profile read after silent
query cancellation during reconnect, rereading the workspace lens on retries and checking it
again after success. It still waits for authoritative profile identity before
loading scoped work and propagates real read failures. Native tools, HTTP/UDS/CLI contracts,
hooks, config, extension SDKs, persisted data and workspace/profile isolation are unchanged;
no migration or official `skills/compozy/`/site documentation change is required. The existing
route-preloading integration suite and terminal shell-config restart journey own verification.

## Untested QA sweep — 2026-10-02

Owner: `docs/qa/reports/2026-10-02-untested.md`; update this audit as further repairs land.

- **PR 691 CI remediation:** Failed extension removal preserves nullable Profile creation
  attribution with the rest of its rollback snapshot. Token-only programmable view opening
  resolves registered attachments independently of command-channel readiness, retaining
  foreign-client rejection. Web shows the fixed safe non-loopback refusal while keeping
  arbitrary runtime errors hidden; Storybook co-ships catalog WebSocket fixtures.
  No public DTO, route, native-tool ID, hook/config key, migration, SDK or workspace/Profile
  ownership changes. Site and official skills/compozy contracts remain accurate.
  [CI remediation evidence](../qa/reports/2026-10-05-pr-691-ci.md) records owning suites;
  broad deferred QA remains deferred.


- **Durable wait example:** not applicable — editorial only. The site's DSL example
  replaces an unknown event with catalog-backed task.run.completed and links the hook
  catalog. The exact YAML validates through the public CLI. Native tools, CLI/HTTP/UDS,
  Web runtime behavior, extension/hook/config contracts, workspace/profile isolation,
  persisted data and the official skills/compozy/ contract are unchanged. No migration
  or new test file is needed. LP-live-run-survives-extension-disable and
  BUG-20261005-loop-wait-example-unknown-event own the current-cycle evidence.

- **Orchestrated task results:** The bundled staging transform uses the existing namespace
  reference form for task IDs. Managed Goal normal/recovery readers concatenate ACP message
  fragments verbatim, so completed outputs retain the final summary and task identities.
  Action and verdict-only judge collectors also retain whitespace-only message chunks;
  the existing adapter suites own their separate answer/verdict boundaries.
  CLI/HTTP/UDS/native loop status and Web output inspection keep their existing schemas.
  Hook/config/extension SDK contracts and workspace/profile isolation are unchanged; no
  migration or historical output rewrite occurs. The site's copyable implement-tasks YAML
  co-ships the same mapping; the official skill's traceability contract stays accurate.
  LP-implement-tasks-orchestrated-mode and TA-080
  own the fresh replay; the embedded runtime suite and daemon Goal reader suite own the
  regressions. Bugs: BUG-20261005-loop-staged-task-id-literal and
  BUG-20261005-goal-result-loses-fields; BUG-20261005-loop-result-drops-whitespace.

- **Loop catalog profile scope:** Web passes the existing selected-profile or explicit
  aggregate params through the catalog hook, route preloads, filter normalization and
  adapter. The canonical cache key includes that scope; continuation keeps it. The
  cross-workspace command palette preserves its existing acting profile on every page.
  HTTP/UDS, CLI and native loop_list contracts and server filtering remain unchanged.
  Extension enablement, hooks/config and workspace data keep their existing owners;
  no schema migration or official skills/compozy/ contract change is needed. ET-052 and
  BUG-20261005-loop-catalog-ignores-profile own the real leave/return replay. The existing
  query-options suite owns paginated population isolation; route-preloading owns
  loader/hook cache reuse and the adapter suite owns query serialization.

- **Runs filter recovery:** Web carries origin/session filter presence into the existing
  empty-state model and clears all roster filters through their current setters. Server
  filtering, CLI/HTTP/UDS/native tools, extensions/hooks/config, workspace/profile isolation,
  storage and the official skill remain unchanged. LP-008 and the route-composition suite
  own recovery proof; the site Runs guide describes the clear-filter action. Bug:
  `BUG-20261005-loop-filter-hides-existing-runs`.
  Keep the toolbar mounted through query loading/failure so the session-id draft remains
  editable. The existing real-daemon Loops Web E2E owns this distinct lifetime invariant
  for `BUG-20261005-loop-session-filter-disappears`. The existing shared filter search stops
  forcing focus back on blur, allowing its input and menu focus lifecycle to complete.
  No new focus context, exported component prop or visual token changes; the same real-daemon
  E2E covers initial input focus, continuous typing, reload and Escape-to-trigger.
  The Runs view computes its optional filter fallback in the component body so React
  Compiler can compile it; its existing default/explicit-filter behavior is unchanged.

- **Breadcrumb destination:** The Web routing coordinator consumes a navigation-stack entry
  only when it matches the selected parent. A deep link can leave a different entry behind;
  that case uses the existing explicit navigation command. Matching history still pops once.
  Native `compozy__window_*` tools, CLI/HTTP/UDS `window.navigate` semantics, persisted stacks,
  layout migrations, hooks/extensions/config and workspace/profile ownership are unchanged.
  The official window-management reference remains accurate for explicit pop commands; the
  site Runs trail description remains valid. The coordinator suite owns the bridge invariant
  and LP-web-runs-breadcrumb owns the real deep-link/Back/crumb replay. Bug:
  `BUG-20261005-loop-breadcrumb-history-overrides-parent`.

- **Human gate decisions and context:** Accepted approval/request-changes decisions rearm only
  the matching wait epoch inside the existing decision transaction. Reevaluation settles the
  awaiting verdict projection once; the append-only event ledger retains both observations and
  final verdicts cannot be overwritten. HTTP/UDS, CLI and `compozy__loop_approve` keep their
  authorization, routes, flags and decision contract. Pending criteria add an optional rendered
  `prompt` to existing JSON diagnostics and detail DTOs; OpenAPI and generated TypeScript co-ship.
  Existing history remains readable and is not rewritten. The approval event carries the question
  and actual criterion count. Web projects the current generation/gate from durable detail and
  fences its live fallback by the same identity. No SQLite table shape, migration, hooks,
  extension/config changes, or workspace/profile ownership changes. The official skill's approval
  semantics remain correct; the site guide describes durable questions and correct CLI flags.
  The existing gate evaluator, store wait/history/coordinator, and Web run-page suites own the
  distinct boundaries. LP-009/010/011 own the real decision replay. Bugs:
  `BUG-20261005-loop-request-changes-finishes-done`, `BUG-20261005-loop-human-prompt-missing`.
  The Runs table also reserves readable name width inside the shared scrolling Table primitive;
  LP-008 owns mobile/desktop visual proof. No new component, token or public contract is introduced
  for `BUG-20261005-loop-mobile-run-names-truncated`.
  Usage and approval fallback facts now use the daemon's adjusted `started_at` budget clock;
  the elapsed duration shown in Runs keeps its existing semantics. No new wire field is needed.
  The existing usage projection suite and finished review replay own
  `BUG-20261005-loop-usage-counts-human-wait`; the site approval guide explains the distinction.

- **Managed Loop tool policy:** Carry the authored node's allowed_tools through managed
  session creation and reject explicit restrictions that differ from an active pinned
  profile. The existing Manager owns subset validation against the Agent ceiling.
  CLI/HTTP/UDS/native Loop calls retain their routes, fields and deterministic validation
  errors; Web consumes the existing node failure payload. No hook, extension, config key,
  workspace/profile ownership, storage shape or migration changes. Official Loop guidance
  and the DSL reference describe the enforced restriction and immutable reuse policy.
  The existing managed runtime integration suite owns this boundary; LP-046 owns the real
  provider-backed subset/refusal replay. Bug: BUG-20261005-loop-managed-allowed-tools-ignored.
  Session subset failures also retain a typed policy cause through the existing error chain.
  The Loop action_failure shape adds the specific allowed_tools_policy_violation code,
  rejected tool and safe recovery guidance. Existing ErrValidation matching and raw CLI
  error text remain stable; arbitrary errors still receive the safe generic projection.
  This extends the existing failure taxonomy without a new DTO or migration. The same
  integration case owns end-to-end refusal projection; existing session subset and daemon
  safe-failure suites remain canaries. Reopened bug: BUG-20260713-loop-failure-hidden.

- **Loop runtime field names:** Run and automation input wrappers pass their visible caption
  IDs through the existing typed control to RuntimeSelector's caption-plus-value accessible
  name. No shared selector behavior changes. CLI/HTTP/UDS/native tools, extension/hook/config
  contracts, workspace/profile isolation, storage and official skill commands are unchanged.
  LP-002 records the Web impact; no site contract changes. The existing LoopRunInputField
  runtime case owns field distinction and value selection; fresh run/automation form walks
  own real UI proof. Bug: `BUG-20261005-loop-runtime-labels-generic`.

- **Vault name correction:** the existing validator keeps the namespace/path grammar and
  `ErrUnsupportedSecretRef` identity, but reports a usable format correction without echoing the
  rejected ref. CLI/HTTP/UDS/native callers retain their current status and DTO contracts. Web
  New secret composes the existing field HelpTip; rejected writes retain their draft. No hook,
  extension, config, storage, workspace/profile isolation, migration, or generated shape changes.
  Official `skills/compozy/` commands remain valid; the site Vault guide explains supported names.
  `TestVaultHandlersRejectInvalidRequests` owns rejection before storage and safe diagnostics;
  Dora's real Vault recovery/overwrite walk owns UI evidence. Bug:
  `BUG-20261004-vault-name-recovery-missing`.
  The same editor now places its overwrite confirmation in the existing wrapping `AlertActions`
  row instead of the absolutely positioned compact-action slot, keeping the consequence readable.
  No shared primitive or consent behavior changes; browser layout replay owns
  `BUG-20261004-vault-warning-covered`.
  Settings rows also accept an explicit accessible help label for decorated field labels; Vault
  supplies About name without changing input names or the shared HelpTip interaction lifetime.
  The existing settings-field-row suite owns that association. Bug:
  `BUG-20261004-vault-help-name-generic`.

- **Attention policy after workspace deletion:** Channel-only Web writes omit the optional
  profile mute replacement, preserving the server's current list. The canonical policy query
  rereads after workspace-catalog removal; it never filters or rewrites daemon-owned mute rows.
  SQLite replacement retains its transaction and rollback while reporting the existing typed
  missing-workspace error, consumed by the shared HTTP/UDS 404 mapping. CLI/native config controls,
  route/DTO/tool IDs, hooks, config keys, storage shape and profile/workspace ownership are unchanged.
  Official configuration guidance and MS-attention-settings-roundtrip co-ship. The existing Web
  page and SQLite repository suites own their separate write/reconciliation and atomic-error
  invariants; the same public scenario owns the real replay.
- **Task session and start-response ownership:** Dedicated and automatic task workers carry
  the task's stable profile into session admission, and role reuse refuses a different profile.
  Nominal run transitions and task execution retries retain inherited profile identity; shared
  start/publish/approve handlers decorate the existing task/run fields. Task and run inspection
  label their existing task summary through the same owner map. CLI, HTTP/UDS, Web and
  hosted agent start consume that identity without changing routes, DTOs, tool IDs, hooks,
  extension methods, configuration, workspace placement or stored schema. Existing sessions are
  preserved; no history is reassigned. The official task guidance and TA-048 own public replay;
  daemon bridge/role, task service and handler suites own their separate boundary invariants.
- **Scheduler and run-page reads:** Active Dashboard scheduler status/backlog queries refresh at
  the existing dashboard cadence and stop while its window is inactive. The scheduler remains
  daemon-wide. Task/run detail, inspection, history, review lists, result pages and streams carry
  the current profile read scope; route preloads share the same scoped cache identities.
  Application entry hydrates the workspace lens and resolves its remembered profile before
  parallel loaders and shell consumers start; an explicit local view retains precedence.
  A failed identity read reaches the existing route retry boundary without default-profile work.
  Missing OpenAPI selectors are restored from the existing task actor contract, including
  single-profile writes and explicit aggregate reads; generated JSON/TypeScript co-ship.
  This is additive public documentation of accepted parameters, with no renamed surface,
  storage migration, config/hook change or native-tool ID change. Workspace authorization
  remains server-owned. Official task guidance and TA-048 record the visible behavior;
  handler, public-schema, query-controller, adapter and route suites own their distinct invariants.
- **Attached command client lifetime:** HTTP/UDS and CLI client discovery, plus native
  command invocation targeting, use the WindowManager's active command channels. Closed tabs no
  longer cause multiple_clients or remain targetable. Registered presentation/context and attachment
  authorization survive disconnection for reconnect; workspace scope and profile ownership remain
  unchanged. No DTO, route, native-tool ID, hook, extension API, configuration or storage change.
  The official native-tools reference explains live targeting. Existing daemon integration IT-031
  owns disconnect/reconnect behavior; Web E2E-027 and the structured targeting scenario own real replay.
- **Multi-client profile navigation:** the Web runtime refuses admission during a known conflict
  before publishing an optimistic route. Completion of an accepted open uses the window's current
  route, preserving an already consumed dialog intent. Existing conflict recovery and newer-intent
  fences remain authoritative. No public surface, persisted state, native tool, hook, extension,
  configuration or workspace/profile boundary change. Runtime/routing suites own the two races;
  the profile palette and remote-write scenarios retain original-flow and adjacent Back evidence.

- **Delegated profile selection result:** Web persists the canonical selection, returns the
  client-command result over its original connection, then activates the new profile. Direct
  UI selection keeps its optimistic behavior. The internal reply continuation is carried through
  every consumer together; public CLI/HTTP/UDS/native command shapes, WebSocket frames, extension
  contracts, hooks, configuration, storage and workspace/profile authority are unchanged. The
  existing channel/stream suites and E2E-027 own verification; official profile guidance describes
  the persisted result, and the same palette/remote-write scenarios own public replay.
- **Profile palette argument handoff:** Attached CLI/HTTP/UDS/native command invocations retain
  their existing action/args envelope; Web now reads the supplied profile and preserves scalar
  navigation arguments through the same route builder as direct palette dispatch. Settings
  carries create/rename suggestions into the canonical dialogs and consumes their route intent.
  The existing profile selection, lifecycle plan revisions, destructive confirmation and local
  authority remain unchanged. No native-tool ID, extension, hook, config, storage shape, migration
  or workspace boundary changes. Official profile guidance explains the handoff. Existing
  client-op, route and profile E2E suites own validation; the palette and remote-write scenarios
  retain the public replay and the separate unresolved navigation stall.
- **Profile owner response completeness:** Scheduler backlog runs inherit the owning task's
  stable profile ID, and shared handlers decorate backlog task/run owners and task-update owners.
  Existing HTTP/UDS fields now contain their promised identity; CLI and Web consume those same
  fields. No route, DTO, native-tool ID, Host API method, hook, configuration, workspace boundary
  or storage shape changes. Existing task-service and handler suites own the regressions;
  official task guidance and the profile lifecycle scenario carry the public replay.
- **Profile admission race diagnostics:** Session insertion translates only the two known SQLite
  profile availability guards into a typed store refusal, including identity-bound registration.
  Shared HTTP/UDS error handling returns the existing profile conflict payload and recovery action;
  failed automation history retains that guidance. Native session/automation tool IDs and extension
  methods are unchanged; their session admission guard remains authoritative. No new hook, config,
  DTO, schema, migration or workspace/profile ownership rule. Web uses the existing error/history
  fields. Store registration, transport error and automation history suites own their respective
  invariants; official profile guidance and the lifecycle race scenario own operational recovery.
- **Task run and operator action profile selection:** Run lifecycle and recovery, single/bulk
  force controls, fan-out, task update/delete, dependencies and review request/submit now use the
  existing CLI mutation selection boundary. HTTP/UDS routes and DTOs, native task tool IDs,
  extension methods, hooks, configuration and workspace storage are unchanged; no migration.
  Session-bound branches retain their authenticated identity. The existing CLI profile suite
  owns request selection; the profile lifecycle and task-control journeys own real replay.
  Official task guidance documents the affected commands. Web already sends the correct profile.
- **Empty task block lists:** The shared mapper returns an allocated empty array to honor the
  existing non-nullable response schema. HTTP/UDS and native `compozy__task_blocks` share this
  boundary; CLI reads preserve it. No new route, tool ID, schema, hooks, configuration or workspace
  ownership change. The existing HTTP task-block response suite owns regression coverage.
- **Task CLI control profile selection:** Pause/resume/cancel, block/unblock/recover and block reads
  use the existing single-profile boundary before transport. Task routes, native tools, extension
  methods, hooks, configuration, workspace storage and Web behavior do not change; no migration
  is needed. The existing CLI profile suite and the adjacent TA-010 walk own verification. The
  original overview approval repair remains verified. Approval-pending CLI copy is made neutral
  because source policy can require approval for read-only commands too.
- **Extension task response state:** Host API task payloads derive draft from the canonical status
  and copy the persisted creator notification flag, cursor, current run, pause inheritance, block
  reasons and attention metadata, including detail summaries. Free-text state stays redacted. Existing task
  methods, SDK/OpenAPI shapes, native-tool IDs, hooks, configuration, workspace/profile ownership
  and storage remain unchanged; no migration or generated-contract change is required. The task
  serialization suite and TA-001's adjacent extension-command replay own verification. Official
  extension guidance documents the fields; Web already displays the correctly persisted state.
- **Extension Host API workspace/profile binding:** The shared boundary recognizes the existing
  workspace_profile process scope and retains its workspace for domain calls and actor derivation.
  Own-workspace task creation keeps the bound profile; global and foreign workspace requests remain
  refused. Resource methods retain their full compound scope for kernel authorization. Host API
  method names, SDK shapes, native-tool IDs, hooks, configuration, database and workspace placements
  do not change. Existing resource and task handler suites own coverage; the profile approval
  scenario owns the real command replay. Official extension guidance and site permissions docs
  explain the binding. Web consumes the existing execution result without a new contract.
- **Pending palette decisions:** Add `approvals resolve <id> --decision approved|denied` and
  `POST /api/tools/approvals/{id}/resolve` on HTTP/UDS. Existing show/cancel behavior remains;
  OpenAPI now documents their existing profile selector as well. The shared profile resolver
  rechecks availability and session immutability before the coordinator enforces recorded ownership,
  expiry and the single-decision fence. HTTP uses the existing privileged loopback mutation guard.
  Deferred tools retain current policy checks and single-use authorization under the original
  workspace/profile. Web keeps a pending invocation open, exposes an explicit decision, and reads
  asynchronous execution state through a profile-keyed canonical query. Public contracts, generated
  Web types, CLI/site documentation and official native-tool guidance co-ship. No new native-tool
  ID, hook, extension contract, configuration key, database shape or workspace placement; existing
  `compozy__cmd_palette_invoke` callers receive the same approval ticket. No migration is required.
- **Settings restart truth:** Web reads the daemon's existing configuration status for current
  restart requirements, including writes from CLI/HTTP/UDS and other documents. The settings apply
  owner retains restart-required scoped writes outside the global hash until the next daemon boot;
  unrelated live writes cannot erase that requirement. Existing status payloads carry this truth
  across CLI/HTTP/UDS without a shape change or persistence migration. The latest apply
  record identifies the notice dismissed by the operator; it does not decide whether a restart is
  required. A previous successful operation cannot hide a later requirement. Settings mutations
  and terminal restart observations invalidate the canonical status/apply queries. The existing
  session-storage envelope upgrades losslessly from version 0 to 1, retaining operation identity
  and pending mutation while adding the dismissed apply-record ID. No daemon schema, public route,
  native-tool ID, hook, extension, configuration key or workspace/profile ownership changes.
  Official configuration guidance, MS-037 and its existing hook/presentation suites own verification.
- **Native permission cancellation:** ACP emits a terminal system cancellation when its caller or
  connection closes; session persistence marks the interaction canceled and clears derived pending
  attention. The existing native approval deadline reports `approval_timed_out` even when ACP
  returns its normal canceled outcome. Operator allow/reject choices, tool IDs, HTTP/UDS routes,
  authorization, hooks, configuration and workspace/profile ownership stay unchanged. The existing
  interaction status accepts cancellation, so no schema migration is needed. Web consumes the
  canonical projection; official runtime guidance and the profile/attention scenarios co-ship.
  Existing ACP, session transition and daemon bridge suites own the regressions.
- **Extension-declared profile provenance:** Global migration 00127 preserves every declaration
  marker and adds nullable creation provenance: existing rows remain unknown; new profile creation
  records true and binding records false. Creation claims also require the current profile ID, so
  deleting/recreating a name cannot transfer authorship. Existing create-once markers still prevent
  reseeding. Install/apply results consult persisted provenance before emitting the existing
  `extension.profile_created` event when creation races with the operator. CLI, HTTP/UDS and native
  extension detail retain the boolean `created_by_extension`; true confirms authorship, while false
  includes binding and unknown historical origin. No verb, route, native-tool ID, configuration,
  resource placement or authorization changes; workspace/profile ownership remains enforced by the
  existing extension and profile boundaries. Web already reads the shared payload and needs no new
  control. Official extension guidance and site installation docs explain the historical limit.
- **Retained Global history upgrades:** Catalog and health readers accept the workspace-free
  scope written by the existing home-to-Global migration. Creation profile versions 3–5 retain
  their original sandbox/Network fields solely as hash-bound provenance; current creation still
  writes version 6. The catalog's logical scope and the immutable events.db owner are resolved
  separately, with the original creation witness proving Global ownership. Metadata, database
  owner rows and historical hashes are not rewritten; no new schema or migration is introduced.
  HTTP/UDS add read-only `/api/sessions/{session_id}/transcript`, `/transcript/search`,
  `/transcript/outline`, `/status`, `/events`, `/history` and `/stream` routes for Global history,
  enforcing both Global ownership and profile read scope. Existing project routes and mutation
  boundaries stay intact. CLI transcript, search, outline, status, events (including follow) and
  history reads select the owning route; filters, archive selection, bounds and SSE cursors
  retain their existing semantics. Global status omits project-only Heartbeat wake enrichment;
  project Heartbeat failures still propagate. Native session tools keep their existing
  project/caller scope and IDs. No hook, extension SDK or configuration changes. Web links ask
  to enable Global without replacing the remembered project and render retained history read-only.
  Out-of-scope documents hide a session locally without retiring its shared window; only
  confirmed deletion or an explicit delete retires it. Existing presence and content guards remain.
  Generated OpenAPI/Web types, official runtime guidance and session control-plane docs co-ship.
  The retained-history bug and Global scope scenarios own the released-binary upgrade replay.
- **Native tools / CLI / HTTP / UDS:** Generic CLI tool invocation preserves public
  `credential_requirements` metadata and its discovery schema, matching the existing daemon
  contract. Validation patterns retain their exact syntax only inside discovery schemas; descriptions,
  defaults, and unrelated data keep normal secret redaction. Scoped session-list JSONL restores its documented leading `profile_resolution`
  frame for both empty and populated pages; aggregate frames and trailing page metadata remain.
  Profile selection reads retain the effective `profile` and add optional `note` for archived
  remembered fallback; CLI resolution preserves that provenance. No route, tool ID, or command changes.
- **Document-wide streams:** `/api/sessions/catalog-stream`, `/api/worktrees/catalog-stream`,
  and `/api/logs/stream` add WebSocket upgrades with one existing SSE frame per text message.
  HTTP/UDS SSE remains supported. Session/log upgrades resume through `last_event_id`, preserving
  header precedence and leaving existing numeric log query filters unchanged. Stream scopes,
  redaction, event names, payloads, and gateway tickets retain their owners. Upgraded connections
  join transport shutdown through the existing stream lifecycle primitive. Three Web consumers
  opt in so background notifications and profile lifecycle sweeps stay live across documents.
  Official runtime guidance, generated OpenAPI/Web types, and session/worktree docs co-ship.
- **Extensibility / hooks / config:** The shared redactor recognizes the public metadata field;
  nested secret fields and secret-shaped free text retain their existing protection. No new
  configuration, hook, or extension capability.
- **Overview CLI validation:** An explicitly supplied `--usage-window=0` now receives the same
  accepted-value validation as other invalid numbers. Omission retains the 30-day default; HTTP,
  UDS, native tools, and generated contracts already express 7, 30, or 90. No migration is needed.
- **Task execution CLI:** Publish, start, approve, and reject now resolve the selected profile
  through the existing command wrapper before transport. The daemon retains its existing
  ownership checks, including not-found responses for a foreign profile. No wire, native-tool,
  Web, hook, configuration, or persistence change; official task guidance co-ships.
- **Approval CLI ownership:** `approvals show` and `approvals cancel` resolve the selected profile
  through the existing command wrapper before transport, including flag, environment and remembered
  selection. The daemon's ownership checks remain authoritative. No command, wire, native-tool,
  Web, hook, configuration, extension or stored-state shape changes; no migration is required.
  Official native-tool guidance and the existing palette command suite co-ship; the QA report
  records the real pending-approval replay and profile lifecycle plan readback.
- **Automation cursor scope:** CLI transports opaque job/trigger cursors to the daemon. Shared
  HTTP/UDS and native-tool parsers receive the resolved profile scope before fingerprint validation.
  Canonical filter/order/count/cursor validation remains authoritative, including refusal after
  profile or filter changes. Extension Host already binds scope first and is unchanged. Web
  continuation consumes the repaired HTTP boundary. No DTO, cursor version, route, tool ID,
  hook, config, workspace storage or migration changes. Existing CLI cursor fixtures now carry
  real profile identity; public replays cover TA-052/TA-056. Official native-tool guidance co-ships.
- **Global automation details:** The Web detail boundary recognizes the resolved Global lens
  and retains refusal for a different concrete project. Project-owned edit forms resolve agent
  and Loop catalogs from the returned definition's workspace. Server profile authorization,
  mutation ownership and managed-source restrictions remain authoritative. No native-tool, CLI,
  HTTP/UDS, extension, hook, config or stored-state contract changes; no migration is needed.
  Official skill commands and site API guidance remain current. TA-052/TA-056 and the existing
  detail-hook suite own the repair evidence; fresh Chrome/CLI/UDS replay covers saved behavior.
- **Global scope persistence:** Daemon-derived window-scope cleanup no longer persists a stale
  document's navigation snapshot. Workspace readers skip observations for an already retained
  desktop. Explicit selection and first desktop resolution still persist with the same v4 key,
  version 1 envelope and lossless version 0 migration. Open documents retain their own view;
  reload uses the saved selection. No native tools, CLI/HTTP/UDS, hooks, configuration, extension,
  workspace-file or daemon-storage change. Official skill and site commands remain current.
  The workspace persistence suite and MS-web-menubar-global-scope-toggle own the repair evidence.
- **Task-backed job detail:** The Web job read surface reuses the form preview's pure run digest
  to display the persisted task title, description and owner, including the existing job-default
  fallback. Agent and Loop destinations retain their presentation. No native-tool, CLI/HTTP/UDS,
  extension, hook, configuration, profile/workspace isolation or stored-state contract changes;
  no migration is needed. Official skill and site interfaces remain current. TA-052 and the
  existing job detail/form suites own the regression and fresh browser replay.
- **Retained Loop window scope:** A foreground scoped Loop route adopts its known project;
  retained background windows and later shell scope choices no longer trigger that adoption.
  Route ownership validation and persisted scope formats stay unchanged. No native-tool,
  CLI/HTTP/UDS, extension, hook, config or workspace-storage contract changes; no migration.
  Official skill and site commands remain current. The existing Loops E2E suite owns the
  regression, with fresh Bruno replay shared by the Global toggle and automation catalog scenarios.
- **Offline automation pagination:** Jobs and Triggers carry Query's paused state to their
  shared pagination control. Loaded rows remain usable while the disabled control explains that
  continuation is waiting for connectivity. Existing reconnect, focus and invalidation policies
  remain authoritative. No native-tool, CLI/HTTP/UDS, extension, hook, config, profile/workspace
  isolation or stored-state changes; no migration. Official skill and site contracts remain
  current. The existing catalog component suite and both live catalogs own verification.
- **Trigger submission feedback:** The Web trigger editor keeps server submission errors beside
  its persistent actions in both form and preview views. Existing request validation, mutation
  ownership and draft recovery remain authoritative. No native-tool, CLI/HTTP/UDS, extension,
  hook, configuration, workspace isolation or stored-state contract changes; no migration.
  Official skill and site interfaces remain current. ET-web-jobs-triggers-catalog and the
  existing trigger form suite own the hidden-error regression and fresh recovery replay.
- **Automation preview directions:** Job schedule recovery and Trigger webhook/template guidance
  name the existing form/preview footer actions after the permanent preview rail was removed.
  This is a presentation-only correction; draft, validation and submission behavior are unchanged.
  No native tools, CLI/HTTP/UDS, extension, hook, config, workspace isolation or stored-state changes;
  no migration. Official skill and site interfaces remain current. TA-web-automation-preview-toggle
  owns the before/after real replay; existing form/editor suites retain behavior coverage.
- **Saved Loop target recovery copy:** Incompatible and unavailable automation targets give
  edit-mode remedies that preserve the immutable target; creation keeps its picker remedy.
  The existing mode and availability projection remain authoritative. No native-tool,
  CLI/HTTP/UDS, extension, hook, config, isolation, stored-state or migration changes. Official
  skill/site interfaces remain current; both real editors and existing form suites own proof.
- **Local Web bundle revalidation:** Static HTTP responses use the opened file's modification
  time, so COMPOZY_WEB_DIST_DIR rewrites invalidate prior HTML and asset validators without a
  daemon restart. Embedded zero-time assets retain the existing start-time fallback. Routes,
  CSP, range handling, native tools, CLI/UDS contracts, extensions, hooks, configuration keys,
  workspace data and persisted shapes stay unchanged; no migration. Official skill/site
  contracts remain current. The existing HTTP static suite and real entry/rewrite replay in
  TA-web-automation-preview-toggle own regression evidence.
- **Trigger authoring and error recovery:** The Loop mapping example follows the existing
  accepted template grammar; the preview serializes strings as valid sample JSON. Failed
  Trigger detail reads expose the existing catalog-return action alongside the daemon error.
  No native-tool, CLI/HTTP/UDS, extension, hook, configuration, workspace isolation or stored-state
  contract changes; no migration. Official skill and site contracts remain current. The existing
  trigger form/detail suites and fresh Bruno replays own verification, including mapping save
  and independent readback. The mapping copy uses replay evidence rather than a prose assertion.
- **Profile archive audit:** The daemon records `profile.archived` under the permanent operator
  owner, matching the existing delete audit convention. The payload retains the affected profile
  and operation identity. CLI, HTTP/UDS and palette lifecycle calls share the repair; Web receives
  the existing named event and sweeps the unavailable view. Ordinary writes under archived owners
  remain refused. No schema, wire, native-tool, hook, config or extension change, and no migration.
  The existing daemon recorder suite uses real SQLite; official profile guidance and QA replay
  document the event's ownership and restored live projection.
- **Profile operation recovery audit and reservation:** Failure/recovery audits use the same
  permanent operator owner as archive/delete, retaining the affected profile and operation in
  their existing payload. Archived identity audits follow the same rule using an internal subject
  state snapshot; active identity event ownership is unchanged. Pending operations block edits inside the
  canonical manager transaction. CLI, HTTP/UDS and delegated actions share this guard; ordinary
  unavailable-owner writes remain refused. No wire, tool ID, hook, config, extension or storage
  shape changes; no migration is needed. Web observes the existing event stream and refusal.
  Official profile guidance and the recovery QA scenario co-ship. Existing daemon-recorder and
  profile-availability suites own regression coverage.
- **Unavailable profile Web entry:** The window-manager HTTP adapter retains typed profile
  refusals emitted before its handler. The existing desktop projection carries its load error
  to the menubar, which exposes profile recovery and the server's remedy instead of a generic
  layout retry. An explicit profile switch provides access to Settings; remembered ownership
  and the unavailable-owner guard stay intact. No CLI, HTTP/UDS, native-tool, hook, config,
  extension or workspace-storage shape changes; no migration. The Profiles E2E suite and the
  existing recovery scenario cover cold entry and subsequent recovery. Official profile guidance
  names this Web recovery path; site documentation has no separate affected contract.
- **Profile view recovery:** The Web shell keeps its global profile lifecycle feed alive while
  desktop authority reconnects, so externally deleting the viewed profile can sweep the client
  to default. Other stream budgets remain unchanged. Public CLI, HTTP/UDS, native tools, hooks,
  configuration, extensions and persisted state retain their contracts; no migration is needed.
  The official profile workflow remains current. The existing profile E2E suite and fresh
  browser/CLI/UDS replay cover recovery, refresh persistence and retained neighboring profiles.
- **Profile dialog feedback:** Lifecycle mutations retain their failed state for their sole dialog
  consumer instead of also emitting a raw error toast. Inline validation and plan-retry callbacks
  remain active; existing success notices are preserved. Name refusals bind to the submitted value,
  while operation failures remain visible at form level and pending mutations retain their lifetime.
  This changes only Web feedback ownership and the internal create-dialog props/story.
  CLI, HTTP/UDS, native tools, hooks, configuration, extensions, workspace isolation and persisted
  shapes are unchanged; no migration or official skill command update is needed. The owning
  Settings E2E flow covers reserved/duplicate refusals and recovery with a valid name; the existing
  create-dialog suite covers current-value validity, operation errors and pending submissions.
- **Profile emoji keyboard navigation:** Create and edit-identity dialogs let arrow keys reach
  Frimousse's existing navigation listener through Base UI's event customization API. Escape,
  focus containment and all other modal consumers keep their existing behavior. The shared picker
  and dependencies are unchanged. CLI, HTTP/UDS, native tools, hooks, config, extensions, workspace
  isolation and persisted shapes retain their contracts; no migration or official skill change.
  E2E-014 covers keyboard identity selection and the saved public response; fresh keyboard replay
  covers both dialogs, canceled creation, focus return and reload persistence.
- **Profile archive automation pause:** CLI, HTTP/UDS, Web and native profile actions share a
  resource-aware archive plan and atomic pause transaction. Dynamic definitions retain their resource
  identity with a new version; configuration/extension definitions retain their content and receive
  existing enabled overrides. A journaled runtime synchronization finishes before success; failed
  synchronization remains unavailable until explicit operation retry. Other profiles remain unchanged.
  No wire, tool ID, hook, config key or schema shape changes. Official profile guidance co-ships.
  Web renders the same existing paused-list fields; both Web and CLI lifecycle scenarios track replay.
- **Profile deletion ownership:** Public counts and the transactional delete guard include canonical
  job/trigger resource owners and unshadowed legacy definitions, regardless of enabled state.
  Web uses the corrected count to withhold deletion of nonempty profiles. CLI, HTTP/UDS and native
  lifecycle actions share the existing profile_owns_work refusal. No tool ID, wire, hook, config,
  extension or persisted shape changes; no migration is required. Other owners and resource content
  remain unchanged. Official profile guidance and the two lifecycle scenarios co-ship.
- **Profile lifecycle navigation:** Browser Back/Forward uses the existing dialog close operation,
  including its transient drafts and success state. Palette route intents are consumed by the owning
  Settings window after raising a canonical dialog, so cancellation cannot be replayed on reload.
  The existing window-manager route replacement persists the consumed state. Public lifecycle
  commands, HTTP/UDS/native tool IDs, hooks, extensions, config, workspace content and storage shapes
  retain their contracts; no migration or official skill command change is needed. Web lifecycle
  QA and the existing profile E2E suite own the changed behavior.
- **Profile project restoration:** Web transfers the active view only when changing between
  workspace and Global breadth. Leaving a distinct project releases its ephemeral view; re-entry
  waits for the remembered selection query to settle before pinning the client view. External
  updates still leave an already-open client's profile unchanged. CLI, HTTP/UDS, native tools,
  hooks, extensions, config, workspace files and persisted shapes retain their contracts; no
  migration is needed. The official profile guidance and switcher QA scenario co-ship with the
  existing E2E-013, while aggregate breadth and two-client E2Es retain their separate invariants.
- **Profile rename repository offers:** Web derives accepted repository ids from the current
  daemon plan and retains only explicit declines in transient dialog state. Offers start selected,
  name edits preserve declines, and absent candidates cannot enter the request. E2E-016 uses real
  repositories to verify accepted-only movement and unchanged declined content. Public CLI,
  HTTP/UDS, native tools, hooks, configuration, extensions, workspace isolation and persisted
  shapes retain their contracts; no migration or official skill command change is needed.
- **Session stop ownership:** Web single, retry, and batch stops carry the selected session's
  workspace/profile through mutation and cache invalidation. OpenAPI now declares the profile
  selector already enforced by the shared HTTP/UDS handler; generated Web types co-ship. The
  existing native tool, CLI, hooks, config, and persistence contracts stay intact. Session lifecycle
  docs and the official runtime skill explain explicit owner selection. No migration is required.
- **Retained history isolation:** Boot logs typed session-database identity refusals and excludes
  those stores from subsequent boot history processing, allowing healthy sessions to remain
  available. CLI, HTTP/UDS, Web, and native history reads retain the existing ownership refusal;
  no store is adopted or rewritten. Migration failures and cancellation still stop boot. No wire,
  schema, config, hook, or extension change. The daemon operations guide and official runtime skill
  document complete-directory recovery; the existing boot suite and isolated runtime own evidence.
- **Extension manifest compatibility:** TOML and JSON static-resource string paths normalize into
  the current path/profile objects at decode. Current placements, strict unknown-field rejection,
  and canonical build output remain. This SD-013 regime-2 adapter is removed in v0.3.0-beta.31;
  authoring/manifest guides, official extension guidance, and a migration release note co-ship.
  CLI, HTTP/UDS, and native extension build/dev/install callers share the loader; there are no new
  routes, tools, hooks, config keys, Web controls, or stored-state changes.
- **Workspace data isolation:** No persistent state or workspace selection changes.
- **Native invocation approval:** HTTP/UDS approval issuance shares the native input binder with
  dispatch, so workspace names, paths and inherited operator workspace selection produce matching
  approval digests. Supplied input digests are checked before binding; digest-only approvals retain
  their existing bound-input contract. Final post-hook input, profile/session/workspace/agent scope,
  expiry and single-use enforcement remain at the approval store. No tool ID, schema, route, CLI
  flag, Web control, config key or stored-state change; no migration is required. Official tool
  guidance and the owning boot suite co-ship, with real operator replay in the QA report.
- **Native workspace identity:** the shared input binder emits the registration key used by
  session authority and automation/task persistence. Registered IDs, durable IDs, names and paths
  remain accepted selectors. CLI/HTTP/UDS and hosted native tools now address the same project;
  aliases for the caller's project no longer trigger foreign-access approval. Persisted memory
  and configuration identities retain their own resolver paths. This changes no stored shape,
  public field, permission mode, hook contract or Web control and requires no migration. Official
  native-tool guidance and the boot/automation owning suites co-ship; the dated report owns replay.
- **Task catalog workspace selection:** the task service preserves an explicit authorized target
  through the existing task-resource workspace policy instead of replacing it with the caller.
  Omitted targets retain caller defaults and the acting profile still fences the catalog. Native
  dispatch continues to canonicalize aliases and refuse agent global/all scope. CLI/HTTP/UDS
  operator behavior, Web reads, hooks/config, extensions, stored data and public shapes are
  unchanged; no migration is required. The existing task integration suite and official task
  reference co-ship, with a real hosted replay in the owning QA report.
- **Hook authoring reference:** not applicable — editorial only. Tool-event examples and the
  official extension skill use the existing `tool_id` matcher/payload field; permission-event
  `tool_name` remains documented separately. No runtime, wire, config, hook or workspace-state
  contract changes. Public create/restart/catalog/delete replay confirms authoring recovery.
- **Agent context runtime identity:** The shared situation projection prefers the session's
  effective model over its configured agent default. Authenticated HTTP/UDS context reads and
  the fresh prompt context share this correction; pending runtime selection remains intent.
  There is no DTO, tool ID, CLI, Web, hook/config, or persistence change. Existing startup
  snapshots remain historical. The owning situation suite, real provider replay, RT-031 and
  official runtime guidance cover the change; no migration or compatibility adapter is needed.
- **Directory-browser recovery:** The shared Web browser retains known parent/home/root
  destinations when a directory read fails. Both Add project and first-run onboarding consume
  that navigation state; the failed path and error remain visible, and no stale directory entries
  are substituted. Filesystem HTTP/UDS responses, CLI/native tools, hooks/config, stored workspace
  state, and official skill commands are unchanged. The directory-browser suite and the existing
  Add project scenario own regression/replay evidence; no compatibility migration is needed.
- **Onboarding Skip scope:** Explicit Skip enables the existing persisted Global scope only after
  completion succeeds, retaining the remembered project. Normal Finish and failed completion retain
  their previous scope. The change is internal Web orchestration: no API/CLI/native-tool, hook,
  configuration, workspace data, or schema changes. Official structured-surface commands remain
  unchanged; the owning Add project scenario records the populated-catalog handoff and its replay.
- **Official skill / Web / Docs:** Profile guidance documents the optional fallback note. Web
  consumers continue using the existing effective profile field; generated DTOs co-ship. Onboarding
  references now match the observed empty workspace catalog and optional Skip-to-Global flow.
  Onboarding grid sections allow long paths to truncate within their columns; existing controls,
  tokens, and state remain unchanged. Four-width browser replay confirms visible controls.
- **Compatibility:** The profile-selection note is additive under SD-013's public-surface regime.
  Existing profile values and persistence remain unchanged; no migration or deprecation is needed.
- **Verification:** Both regression cases failed before the change. The full redactor race suite
  passes (80.1% coverage); a rebuilt CLI reproduces complete structured HTTP/UDS parity. The two
  session-list regressions also failed before repair; the complete CLI race suite passes, and
  rebuilt scoped/aggregate JSONL output retains one leading frame and a usable trailing page.

## Release PR 680 — CI and generated extension packaging

- **Native tools / CLI / HTTP / UDS:** existing extension init, build, install, update,
  disable, remove, and tool invocation contracts are unchanged.
- **Extensibility / hooks / config:** TypeScript tool, memory, and connectivity templates
  bundle runtime dependencies with the existing Bun build convention after type checking.
  Newly generated packages remain executable outside the source tree. No SDK API, hook,
  permission, or configuration schema changes.
- **Workspace data isolation:** no persistent state changes. The nightly journey uses its
  isolated daemon home, registry server, and generated source directories.
- **Official skill / Web / Docs:** no operator command changes in `skills/compozy/` and no
  product UI changes. The extension quickstart documents the standalone build; the existing
  authoring QA scenario maps to the restored TypeScript/Go nightly lifecycle coverage.
- **Compatibility:** existing user sources are untouched; the template build-script correction
  applies to newly scaffolded extensions. No migration is required.
- **Verification:** the scheduler integration fixture explicitly claims its active run because
  equal queue timestamps are ordered by run ID. Its escalation assertions remain intact.
  Nightly coverage continues to reject an empty executable scenario set.

## Shell rail v2 — rail dock, flush tiling defaults, light and dark themes

Owning design: `docs/design/opendesign/shell-rail/` (`shell-rail-v2.html`, `DESIGN-NOTES.md`,
`brand-spec.md`, `IMPLEMENTATION-PLAN.md`); orchestration contract `.compozy/tasks/shell-rail/CONTRACT.md`
(decisions D1–D9).

- **Native tools / CLI / HTTP / UDS:** additive only. `compozy__layout_arrange` and
  `compozy layout arrange --arrangement` gain `main_stack` (the first listed window takes a 60% main
  column; the rest stack beside it); the OpenAPI enum and generated clients carry it. The command
  palette registry gains `layout.arrange.main-stack`, `layout.arrange.columns`, and
  `window.move_to_desktop.1` … `.9`, all unbound by default and bindable through
  `window_manager.shortcuts`. `layout.arrange` gains an optional `keep_frames` boolean (DTO,
  OpenAPI, generated Web types, `compozy__layout_arrange` schema, CLI `--keep-frames`): set, each
  named window stands for its whole tab frame and a deck is arranged whole; omitted, every named
  window is still its own participant, exactly as before. The shell's Window › Arrange sends it.
  No `compozy__*` ID, route, DTO field, or verb is renamed or removed.
- **Extensibility / hooks / config:** no new hook, event, or extension capability; arrange presets
  and move-to-desktop run through the existing layout/window commands and emit the existing
  window-manager events. Three public `[window_manager]` **defaults** change (SD-013 public-surface
  change, auto-migrating because only the default moves): `gaps.*` 8/8/10/8/10 → `0`,
  `new_window_policy` `floating` → `beside_focus`, `bindings.bottom_center` `reserved` → `zoom`.
  Explicit values in `config.toml` or layout documents keep winning; no key is renamed and no stored
  value is rewritten. The repo `config.toml` example follows the new defaults. The theme is not a
  config key: the preference lives in browser localStorage (`compozy.theme`: `light` | `dark` |
  `system`, default `dark`) and is applied before first paint by the same-origin `/theme-boot.js`.
- **Workspace data isolation:** unchanged. Window arrangements stay per (workspace, profile); new
  presets and moves act only on the addressed workspace's topology. The theme is per browser, never
  per workspace or per profile, and never crosses to the daemon.
- **Official CompozyOS skill:** `skills/compozy/references/window-management.md` documents
  `main_stack` and `--keep-frames` / `keep_frames`; its configuration section states no default values, so the default changes need no
  skill edit. `references/configuration.md` points to the same section; checked, no stale values.
- **Web / Docs impact:** the Web shell is redesigned — full-width 52px topbar with the desktop pager
  and All desktops in the tray, the dock as a 60px left rail with a profile / theme / Settings foot
  (bottom tab bar below 960px), gutterless flat tiling with 9px hairline seams, browser-style window
  tabs, quiet window controls, a flat default desk with an empty-desktop card, Inter/Geist Mono type,
  and light plus dark themes across every primitive. `packages/site` keeps its own theme and type
  (D9, pinned in `packages/site/app/global.css`). Public docs updated:
  `configuration/config-toml.mdx` (new defaults), `configuration/shortcuts.mdx` (new palette ids),
  `cli/layout/arrange.mdx`, `workspaces/window-management.mdx`. Internal design references updated:
  `DESIGN.md` (per-theme tables), `docs/design/opendesign/design-system/` (chapter 02 and the shared
  CSS), `COPY.md`, `docs/_memory/glossary.md`.
- **Compatibility:** user state is untouched (no migration; layout documents, profiles, and
  `config.toml` values survive as written). Users who relied on the old defaults restore them with
  explicit values; the release note carries the SD-013 migration block. Retired internal tokens
  (`--shell-glass*`, dock/window shadows and radii, traffic-light sizes) are internal-regime and are
  deleted with their last consumer, without aliases.
- **QA:** changed shell scenarios are reset to `untested` and new ones added (seam resize, arrange
  presets, move window to desktop, dock foot + compact tab bar, empty-desktop card, Inter type ramp,
  theme scenarios); `ET-web-dock-magnification` is deleted and `ET-web-geist-wght-medium-510`
  retired. The final QA pass walks them.
- **Polish round 2 (P6, window opening):** `new_window_policy` gains `tab` and it becomes the default
  in both config layers (was `beside_focus` on this unreleased branch, `floating` in the last
  release): a client open joins the client's focused window as a tab when that window is visible on
  the open's desktop, otherwise it falls back to beside-focus placement (an empty desktop gets one
  full pane). Clientless opens never join a frame by policy; a peer client keeps its visible tab when
  another client's open creates a new frame. `window.open` gains an optional `floating` flag
  (additive: contract DTO, OpenAPI/TS, `compozy__window_open`, `compozy window open --floating`);
  it conflicts with `insert_tiled`/`stack_target_window_id`. The Settings enum, Settings › Layouts
  picker (Tab in focused window), `config.toml` example, site docs (`config-toml.mdx`,
  `window-management.mdx`, generated `cli/window/open.mdx`), the release note's migration table, and
  `skills/compozy/references/{window-management,native-tools}.md` carry both. Web: rail ⌥-click
  splits, ⇧-click opens on a new desktop, and the rail menu (now on Sessions too) offers new tab /
  split / new window / new desktop / Go to tab. Workspace data isolation unchanged. QA: dock default
  size, Sessions launch, multi-instance, terminal native flow, and tab deck scenarios updated.
  E2E triage on the integrated tree added three product fixes with no contract change: snaps store
  exact zone fractions (odd-width desks drifted), a client's fallback focus after a close resolves to
  the frame's shown tab (daemon `focusForDesktop`), and Continue/Fork "New window" opens the child as
  a split so the source stays visible under the tab default.

## PR 685 — Wake creator on child turn completion

- **Native tools / CLI / HTTP / UDS:** no `compozy__*` ID, toolset, route, DTO, flag, or schema
  changed. `SpawnWakeReasonCompleted` ("completed") is an internal session wake reason consumed by
  the daemon bridge; no public surface gains a value.
- **Extensibility / hooks / config:** no new hook, extension capability, SDK shape, permission, or
  config key. The existing `NotifyCreator` lineage flag and notification opt-out still gate delivery.
- **Workspace data isolation:** unchanged. Wake dispatch resolves the creator from the child's
  recorded lineage parent and rejects self-wakes; no new table, migration, or cross-workspace read.
- **Official CompozyOS skill:** checked `skills/compozy/` spawn/wake/creator-notification guidance
  (`references/runtime-operations.md`, `references/native-tools.md`, `references/tasks-and-orchestration.md`);
  no reference needs updating — no public verb, tool, or config changed.
- **Web / Docs impact:** no `web/` route, component, or hook change; no `packages/site` doc change is
  required.
- **Compatibility:** additive internal behavior — `SpawnWakeReasonCompleted` is a new reason value and
  the `BadgeDone` mapping was previously a no-op for waking; no SD-013 public-surface break, no migration.

## Package cleanup — Network, Bridges, and managed Sandbox hard cut

Owning decision: `.compozy/tasks/pkgs-cleanup/adr-hardcut.md`; implementation and validation evidence:
`.compozy/tasks/pkgs-cleanup/implementation.md`. This is the user's explicitly authorized exception
to SD-013; no compatibility window or lossless translation of retired product data is required.

- **Native tools / CLI / HTTP / UDS / SDK:** remove retired product commands, tools, routes, schemas,
  flags, and DTO fields together. Tasks/autonomy, Goal, SOUL, HEARTBEAT, attention, status cursors,
  local session operation, MCP/ACP adapters, and transport networking remain.
- **Extensibility / hooks / config:** remove Bridge providers, notification presets/subscriptions,
  Network participation and managed Sandbox configuration. Retain tool extensions, hooks, authored
  context access, webhooks, and Gateway; `[gateway]` uses `gateway.private` / `gateway.public` and
  digest-confirmed requirements. Preserve Herdr hook IPC and native provider execution policy.
- **State / isolation:** migration 00121 deletes retired product tables, Network wake runs and their
  dependent records, and obsolete fields; clears retained references; rebuilds retained ownership
  constraints. Local sessions, Tasks and ordinary runs, workspaces, SOUL, HEARTBEAT, notification
  cursors, and recorded Gateway consent remain. Backup and destructive-state disposition are in the
  release-note migration block and public migration guide.
- **Web / docs / official skill:** remove product routes, panels, forms, search/navigation entries,
  landing sections, Marketplace Bridge catalog, screenshots and dedicated RFC/design guidance.
  Rewrite mixed Task/Loop/agent documentation around retained local functionality. The canonical
  `skills/compozy/` instructions describe only supported operations. Published release history stays
  historical; old blog descriptions carry a current-scope notice.
- **QA / verification:** owning suites verify retired surfaces are absent and retained local journeys
  remain, including SOUL/HEARTBEAT authoring and wake policy. Current QA scenarios drop obsolete journeys
  and retain historical run evidence. Site content generation, typecheck, tests, build, and generated
  CLI checks are tracked in the implementation checkpoint with their actual status.

PR 681 review/CI remediation: migration 00121 deletes triage owned by retired Network actors rather
than relabeling it into a retained actor's composite key; existing daemon triage flags survive
upgrade and reopen. Extension TOML/JSON loaders reject `network_participation` with Gateway rebuild
and digest-confirmation guidance; no compatibility conversion or implicit authorization is added.
Migration/release guidance distinguishes preservation of the recorded consent tuple from authority
for a new manifest. Public routes, native tools, hooks, workspace boundaries, and the official skill
remain as audited above. Web rendering sections are decomposed without changing controls or state.
Owning migration, manifest, runtime, and browser suites verify the corrected paths.

## Issue 669 — Preserve whitespace in session transcripts

- **Session transcript:** canonical, legacy, and raw agent text retain whitespace-only chunks between
  nonblank chunks through projection; raw nonblank text also retains surrounding spaces. The existing
  Web renderer keeps split Mermaid fences within their code block and renders subsequent Markdown
  headings and paragraphs normally.
- **Persisted sessions:** projection version 2 replays only assistant entries containing whitespace-only
  chunks or padded raw text when a version 1 session database opens. It leaves the authoritative event
  ledger, unrelated entries, message identity, sequence fences, active entry, and generation unchanged.
  The upgrade is transactional and idempotent; no SQL schema migration is needed.
- **Surfaces and isolation:** the session transcript REST and stream payloads inherit the corrected
  text. Routes, SDK shapes, configuration, native tools, hooks, and workspace ownership are unchanged.
  Inactive-session query opens upgrade version 1 through the owned writable path before returning a
  read-only reader; direct read-only opens still never mutate a database.
- **Verification:** transcript parser/projection regressions and the persisted version 1 upgrade
  regression cover exact Markdown bytes, unrelated data, identity, generation, and repeated open.

## Issue 671 — Awaited child output matches its declared shape

- **Native tools / CLI / HTTP / UDS:** Loop validate and publish reject unsupported `run-loop`
  `produces` fields, types, or constraints. Existing run/status/result routes and IDs stay unchanged. Awaited
  nodes with the exact declared `loop_run_id` and `status` strings persist those terminal values.
- **Extensibility / hooks / config:** no new hook, extension capability, or configuration key.
  The authored `produces` schema selects the structured result already allowed by the Loop DSL.
- **Compatibility / isolation:** awaited nodes without `produces` retain their scalar terminal
  output. Historical unsupported declarations retain that scalar too. Existing Run snapshots and
  workspaces are unchanged; child identity still comes from the same-workspace parent-owned Run.
- **Web / docs / official skill:** the shared validator supplies Web editor diagnostics without a
  UI change. Site authoring references document both result forms. The official Loop skill already
  directs operators to the persisted `child_loop_run_id`; no instruction change is needed.
- **QA / verification:** `LP-run-loop-await-child-ordering` adds declared-output and validation
  checks. Focused linter and coordinator tests cover rejection, child `done`/`no-op`, legacy scalar
  output, and downstream template rendering. A live restart walk remains separate evidence.

## PR 661 CI remediation — metadata-only session restart

- **Native tools / CLI / HTTP / UDS / extensions / hooks / config / official skill:** no contract or configuration change. The existing restart action can reach ready when a concurrently accepted, unbound session has metadata but no event database.
- **Workspace data isolation and compatibility:** the boot upgrade still verifies catalog ownership and migrates every existing retained event database. Only an owned session with no runtime transition or ACP identity and no database has nothing to upgrade; missing histories for previously bound sessions, even if now unbound, and incompatible existing databases still refuse boot. No state is deleted or migrated by this change.
- **Web / Docs / QA:** no UI or public documentation shape changes. `TA-scheduled-session-restart-recovery` and its existing `jobs-hardening.spec.ts` browser journey own the restart evidence; the manager query suite covers metadata-only boot and the bound-session refusal.

## Clarify keepalive — unbounded default plus ACP ping

Owning spec: `.compozy/tasks/clarify-keepalive/_spec.md` (Part II §Impact Analysis lists delete targets and regimes; ADR-001 records the ping-transport decision; reuses `.compozy/tasks/clarify-timeout` ADR-001 for the unbounded default).

- **Native tools / CLI / HTTP / UDS:** `compozy__clarify` keeps its ID, schemas, and risk; `session clarify pending|answer` keep their verbs and shapes except unbounded pending reports `deadline: null`. No route, DTO, or verb added or removed.
- **Extensibility / hooks / config:** extension `clarify/ask` and both SDK `AskClarification` paths inherit the wait and keepalive with no manifest, permission, or SDK change. Config `tools.clarify.timeout` keeps its key and duration shape; omitted/`0s` becomes unbounded (default flips from 5m), `1s`–`24h` stays finite — SD-013 regime 2 with release-note migration block, explicit finite values auto-keep working. The key is agent-mutable at user scope only: workspace and profile writes are rejected (`--scope user` guidance). New ACP agent-facing notification `_compozy/clarify_ping` (identity-only, ignorable by non-supporting agents). Follow-up (2026-09-18): the ACP ping alone does not keep real agents alive — opencode's MCP client aborts any hosted tool call after its ~60s default request window, and the ACP notification never reaches that timer. The hosted stdio proxy now emits standard MCP `notifications/progress` (every 30s while a call blocks, token echoed from the request) so `resetTimeoutOnProgress` clients reset instead of canceling; clients without a progress token receive nothing.
- **Workspace data isolation:** pings resolve session → own live agent process only; no cross-session or cross-workspace delivery. Pending waits stay broker-owned in memory; no stored-data change, no migration.
- **Official skill:** `skills/compozy/references/runtime-operations.md` clarify section stays valid (same verbs); provider contract note for `_compozy/clarify_ping` co-shipped in `packages/site/content/docs/agents/providers.mdx` ("Clarification keepalive") alongside the spec's `_dx.md`.
- **Web/Docs/QA:** no `web/` rendering change in this spec (zero-deadline display owned by `clarify-timeout`); the nullable wire deadline co-ships only the regenerated OpenAPI nullable flag, the generated TS type, and a matching `string | null` parameter widening. `packages/site` co-ships the config reference, the provider doc, and the stale-bounded-wording sweep (`tools/toolsets.mdx`, `sessions/permissions.mdx`, `extensions/develop.mdx`); the durable clarify transcript projects `deadline: null` for unbounded waits. QA: `J-answer-agent-requests` gains the unbounded branch, MS/RT scenarios and four charters carry the cycle. Verification owned by `_tests.md` (bridge/config/session/acp suites plus E2E-001/E2E-002).

## Issue 667 — Start session workspace default agent

- **Web:** unspecified Start session actions preselect the active workspace registration's
  `default_agent`; worktree actions use their owning workspace registration. Agent-specific
  actions preserve the explicitly selected agent. An absent default retains the existing
  `general` fallback and still requires a valid agent before submission.
- **Native tools / CLI / HTTP / UDS / extensibility / hooks / config / SDK / official skill:** no
  contract, configuration key, or behavior change. The Web reads the existing workspace catalog.
- **Workspace / profile isolation and compatibility:** the chosen agent comes from the exact
  target workspace row; no other workspace default, session profile binding, or persisted session
  data changes. Existing sessions retain their agent bindings.
- **Docs / QA:** workspace configuration guidance and the Start session scenario describe the
  preselection. The session-create hook suite covers ordinary and worktree launches; the
  isolated daemon-served browser replay confirms the registered default in the dialog.

## Issue 655 — Claude model identity and effort discovery

- **Native tools / CLI / HTTP / UDS:** routes and tool IDs are unchanged. Model payloads add
  `reasoning_known` and `reasoning_apply`; OpenAPI and generated Web/Go/TypeScript SDKs co-ship.
  Discovery retains each transport's inspected options and projects the same deterministic route
  used by Claude launch. Successful writes require matching provider readback.
- **Extensibility / hooks / config:** no new hooks or configuration keys. Explicit apply strategies
  and authoritative configuration matrices remain enforced. Future advertised effort identifiers
  remain provider-owned strings.
- **Compatibility / isolation:** saved selections, favorites and runtime configs keep their identities.
  Exact versions, account/profile/workspace keys and private transport mappings remain distinct.
  Global migration 00120 adds a validated per-binding option snapshot with an unknown default;
  existing rows and dependent selections survive unchanged. Failed discovery retains prior rows,
  last-success evidence and an explicit stale/error status. No manual cache repair.
- **Web:** unknown capability has its own footer state. Session options hydrate only the effective
  model when negotiation is enabled, including initially unknown effort; they do not replace pending
  or queued intent. Standard thought levels are dedicated controls, not advanced-option duplicates.
- **Session lifecycle:** unbound logical resume defers catalog validation until the next prompt's
  actual binding, matching initial creation. Bound sessions retain their existing resume behavior.
- **Docs / official skill / QA:** site and runtime-operation guidance explain exact discovery,
  negotiation policy and confirmed application. Existing catalog, selector and continuity scenarios
  own the real walkthrough; the issue 655 report tracks evidence and remaining limitations.

## Issue 654 — Bulk worktree removal and missing-record cleanup

- Web composes existing singular remove/dismiss operations in both workspace lists, using immutable workspace/profile/record identity, bounded selection, per-item receipts and failed-only reconciled retry. Successful cleanup clears only matching workspace UI scopes.
- Shared HTTP/UDS remove and dismiss handlers enforce explicit or authenticated session profiles against the retained record owner; operator calls without a selector infer the existing owner at the API boundary, preserving shipped clients (SD-013 lossless boundary translation); optional profile selectors now co-ship in OpenAPI and generated Web types. Existing routes, verbs and native tool IDs remain unchanged. The catalog stays cross-profile and is not mutation authority.
- Dismissal uses the domain usage fence and active-session check, is idempotent by retained ID, and never touches files or branches. Missing reconciliation uses compare-and-swap so stale list reads cannot overwrite removal/dismissal. Restore rejects a competing dismissal and a foreign owner.
- No SQLite/config/extension/SDK/hook shape changes or migrations. Existing removal hooks and Git safety checks remain authoritative; no force or session-stop batch operation is introduced. The official Compozy skill's ownership, cleanup and retained-history commands remain valid.
- Owning tests: worktree removal/recovery and real Git lifecycle; shared API core worktree suite; existing Web hook, scope-store and two list interaction suites; real-daemon worktree E2E. QA report: `docs/qa/reports/2026-09-16-worktree-bulk-delete.md`. Site removal/recovery docs and affected scenarios explain selection and recovery.

## Issue 653 — Rendered Markdown hierarchy

- **Web:** every `<Markdown>` surface gains a real heading ladder (22 / 18 / 16 / 15 / 13.5 px over the 15 px body), semibold emphasis, accent-strong underlined links, wider block rhythm, and framed tables with a tinted header that scroll inside their own frame at narrow widths. `compact` surfaces (tool panels, palette previews, and now the reasoning panel) keep their previous heading sizes, cell padding, and indents, and still gain the link, emphasis, and table-frame fixes. The reasoning panel uses `compact="relaxed"`, which keeps its previous paragraph rhythm.
- **Design system:** adds `--text-prose-h1..h3` and `--tracking-prose-h1..h3`; inline code now uses the existing `--text-inline-code`. `DESIGN.md` and the font-size class list are regenerated. Links are a deliberate, reviewed use of accent in prose; the underline keeps them recognisable without colour.
- **Native tools / CLI / HTTP / UDS / hooks / config / extensions / SDK / official skill:** none. The Markdown safe-mode contract (sanitisation, URL transform, image fallback) is unchanged.
- **Workspace / profile isolation and compatibility:** none. No stored data, routes, or public shapes change.
- **Docs / QA:** `ET-web-session-transcript-calm-grammar` (message bodies), `RT-055` (reasoning), `TA-107` (streamed width), `ET-web-agent-detail-tab-parity` (AGENT.md via `DescriptionCard`), `ET-palette-domain-views` (compact palette detail), `TA-web-task-detail-redesign` (task description card), and `TA-web-task-result-disclosure` (Markdown task result) carry dated impact entries; the `DescriptionCard` and task surfaces were not inspected with the Storybook verification and the live-runtime walk still owed. A token contract test, reading the same constants the components render with, keeps H1–H3 above the prose body and the link text and underline at their contrast floors on every prose surface; `MessageMarkdown` stories `LongAnswer` and `NarrowColumn` are the representative sample.

## Issue 651 — Session deletion profile scope

- Web single and bulk deletion send the selected session owner profile and workspace.
- Successful deletion rereads the owning workspace catalog, global catalogs, and attention counts even without catalog SSE; unrelated workspace caches remain untouched. Browser response matchers require the profile query and QA captures retain separate profile evidence.
- OpenAPI and generated consumers now declare the already-supported optional deletion profile selector. HTTP/UDS routes and authorization are unchanged; wrong-profile requests remain rejected.
- Native tools, CLI, hooks, configuration, extensions, SDK and official skill syntax are unchanged.
- No stored-data migration or profile boundary relaxation. Existing session QA covers non-default owners and neighbor preservation.


## Issue 644 — Marketplace acquisition recovery and production images

- **Native tools / CLI / HTTP / UDS:** existing Marketplace operations retain their IDs, routes and
  schemas. GitHub repository tarballs use the API JSON Accept type before following the archive
  redirect; binary release assets keep their binary media type. Source diagnostics retain safe HTTP,
  DNS, TLS and outbound-policy classifications without exposing upstream bodies or credentials.
  Git archive global PAX metadata is consumed without creating filesystem nodes; the decompressed
  byte limit still includes metadata and all actual entries retain path/type/count safeguards.
- **Extensibility / hooks / config:** no new keys, permissions, hooks or SDK shapes. Public plugin
  acquisition remains unauthenticated, commit-pinned, bounded and governed by outboundpolicy.
- **Workspace / profile isolation and compatibility:** existing source generations, cached entries,
  installation ownership and user data remain intact. No migrations or compatibility removals.
- **Web:** only degraded sources are named in refresh failures. Retry invalidates and rereads the
  canonical catalog as before. Shipped catalog icons reuse the repository assets in the web bundle;
  supported data images remain available and unsupported remote icons fall back before a request.
  CSP stays unchanged. No arbitrary image proxy or browser network allowance is introduced.
- **Official skill:** `compozy` tools-and-skills guidance remains correct: inspect source diagnostics,
  preserve cached entries and retry through the existing refresh operation; no command change.
- **Docs / QA:** Marketplace index and source docs explain recovery, diagnostics and image support.
  The existing landing scenario and Marketplace E2E suite cover partial failure, cached entries,
  Retry/Refresh recovery, production CSP image loading and viewport screenshots. Public acquisition
  is an opt-in CI integration probe; first CI reproduction fetched 52 plugins and then failed at the
  archive endpoint with HTTP 415. The next probe exposed rejection of Git global PAX metadata;
  canonical extraction tests now cover metadata acceptance and byte limits. All corrected-head
  gates and real browser evidence run in CI.

## Context rail quiet defaults (fix/context-rail-quiet)

- **Native tools / CLI / HTTP / UDS:** none. `/usage`, `/usage/turns`, and the session context
  read keep their DTOs; `reported_turn_id`, `reported_at`, and `pressure_threshold` still arrive and
  still feed the stale cue, the bar tick, and the tooltip compaction sentence.
- **Extensibility/hooks/config:** no contract, hook, permission, SDK, or config changes.
- **Workspace data isolation:** no schema, query, or authorization change.
- **Web / Docs / official skill:** Web only. The composer tooltip drops its provenance row
  (`reported` chip, `as of turn <id>`); the rail meter drops its `reported` chip and the
  clock/threshold line, keeping a chip only for loading, unavailable, stale, near compaction, and
  estimated size; Turns ships folded behind the same chevron head as Compozy context. The current
  row in the sessions sidebar drops its accent left bar and keeps the selected tint. QA scenarios
  `ET-web-session-context-meter`, `ET-web-session-context-sidebar`, and
  `ET-web-session-sidebar-threads` record the new reads. No skill or docs-site change.
- **Verification:** owning unit suite `session-inspector.test.tsx` and the focused browser E2E
  `session context E2E-001` against the acpmock fixture.

## Pending stop recovery after a Goal generation change

- **Native tools / CLI / HTTP / UDS:** existing Goal cancel/clear/replace and session-stop recovery
  select checkpoints from the Run's current generation. Historical checkpoints remain retained;
  an already-fenced prompt in an older generation cannot block daemon readiness.
- **Extensibility/hooks/config:** no contract, hook, permission, SDK, or config changes.
- **Workspace data isolation:** the existing scoped Run authorization and transactional ownership
  checks remain authoritative. No schema migration or data deletion is needed.
- **Web / Docs / official skill:** desktop startup and existing Goal controls inherit the fix.
  No UI or skill syntax changes. The affected QA scenario records the restart regression.
- **Verification:** `TestGoalTurnRuntimeLifecycleIntegration` covers current-generation cancellation
  and retained history with real SQLite. Time-travel and inline Goal replace/clear suites also pass
  with the race detector. Recovery passed on a database copy and through two real daemon boots.
  The full integration-tagged lint scan reports six pre-existing findings outside the changed lines.

## Issue 639 — Checkpoint role configuration status

- **CLI / HTTP / UDS / Web:** the shared role projection reports checkpoint availability from the
  daemon-level session-compaction configuration or the existing memory gate, subject to the effective
  role switch. List and detail keep their schemas, provenance, diagnostics and configured runtime.
  Web consumes that same projection; no component, layout or interaction changes are needed.
- **Invocation:** the resolver continues to require an actual compaction invocation when memory is
  disabled. Status reads neither invoke a summary nor prove that compaction has executed.
- **Native tools / extensibility / hooks / config:** no role-inspection native tool exists; existing
  CLI/API management surfaces remain authoritative. No tool ID, hook, SDK, key or default changes.
- **Workspace / profile isolation:** use the existing effective configuration resolver; workspace
  and profile role/memory overrides remain scoped. Both compaction availability and the pressure
  scheduler use the daemon-level compaction setting; scoped compaction overrides do not reconfigure
  that scheduler. The daemon memory master still suppresses workspace memory.
- **Compatibility / recovery:** no persistent shape or public schema changes; no migration or data
  repair is necessary. The corrected status becomes available with the upgraded daemon.
- **Official skill:** existing role inspection guidance in `skills/compozy/references/agent-definitions.md`
  remains accurate; no operation or skill change is needed.
- **Docs / QA / verification:** the memory guide explains availability versus invocation;
  `MS-inspect-background-role-routing` owns the regression walk. `TestRoleStatusProjection` covers
  switch combinations, invocation distinction and scoped configuration. `TestRoleResolverIntegration`
  uses the real CLI and isolated daemon over HTTP/UDS; the PR race shard explicitly runs this suite.
  All tests, lint, builds and QA execute in GitHub CI, with no local validation claimed.

## Issue 643 — Cancel unbound Goals before session deletion

- **Native tools / CLI / HTTP / UDS:** existing session stop/removal and Goal cancellation
  keep their IDs, routes and schemas. The canonical cancellation transaction projects the persisted
  session origin when its checkpoint has no session binding; a moved binding still wins.
- **Extensibility / hooks / config:** no new hooks, configuration, permission or SDK surface.
  The existing transactional outbox and relay remain the publication owners; clear still emits
  an unbound tombstone, and catalog-origin cancellation emits no session projection.
- **Workspace / profile isolation:** origin lookup and outbox validation keep the existing Run,
  workspace and session checks. The daemon retains its profile-scoped owned-Goal dispatch.
  The fallback does not create a binding, change a checkpoint's session or erase Goal audit history.
- **Compatibility / recovery:** no stored shape change or migration. After upgrading, retry the
  failed deletion through the existing UI/API. Other selected sessions already deleted stay deleted.
  Historical-generation selection is separate work; this change does not alter that query.
- **Official skill / Web / Docs:** `skills/compozy/references/loops.md` already documents cancellation
  before removal and retained Run history. No command change is needed. The existing bulk controller
  preserves individual errors and retries only failed IDs. Session deletion and bulk-action QA
  scenarios include the stopped/unbound case and distinguish automated evidence from a full manual walk.
- **Validation owner:** the store lifecycle suite covers cancellation projection and idempotency;
  the real Manager/SQLite integration covers stopped-session deletion; the existing bulk hook suite
  owns partial failure/retry and the browser suite covers active/stopped selection and preservation.
  All executable checks and rendered evidence run in GitHub CI under this issue's delivery contract.
  [PR #647](https://github.com/compozy/compozy/pull/647) owns the exact-head verification receipts,
  screenshots and dispositions of the first CodeRabbit/Greptile review findings.

## Issue 627 — Overlay model discovery and binding

- **Native tools:** existing provider model list/status/refresh/curate and session-create tools keep
  their IDs and schemas. CLI, HTTP, UDS and Extension Host consume the same corrected catalog rows.
- **Extensibility/hooks/config:** no new key, hook, SDK or permission. `runtime_provider` selects a
  registered adapter when the overlay ID has none. The overlay's command, auth mode, credential
  slots and home/environment policies remain authoritative. Settings reconciliation adds/removes
  live sources together with the durable catalog generation; no daemon restart is needed.
  Removing an overlay deletes only its derived live cache across all execution contexts in that
  transaction, so recreating it offline cannot revive the removed account observations.
- **Workspace data isolation:** account observations keep the overlay provider/source IDs and
  profile/workspace execution contexts. Command/runtime changes invalidate discovery identity.
  Session binding reads the overlay source, never the runtime family's account observations.
  Active and stopped runtime-selection admission resolve the configured runtime family while
  retaining the overlay ID for catalog membership.
- **Compatibility:** existing config, model IDs, database shapes and public DTOs remain unchanged.
  Built-in model identities are mapping hints for advertised Claude aliases only; they create no
  overlay rows and confer no account availability. Existing short curated aliases remain valid.
- **API follow-up:** provider inventory/auth and global agent creation, resolution and projection read
  the active Settings snapshot, so live additions are usable immediately. Snapshot errors propagate;
  workspace/profile loaders retain their existing ownership. No new routes or migration. See
  `docs/qa/bugs/BUG-20260912-live-provider-api-snapshot.md` and the September 12 integration report.
- **Official skill:** runtime operations now explain overlay-scoped discovery and binding.
- **Web/Docs/QA:** no Web component or layout changes; Settings and runtime selectors receive the
  shared corrected projection. Model catalog docs and `RT-model-catalog-cold-open` cover overlays.
  Owning checks: live-source and session suites, daemon reconciliation suite, and the SQLite/ACP
  subprocess catalog integration suite. Real provider authentication is not inferred from fixtures.
- **Related work:** PR #550 owns curated-merge semantics and broader picker/startability changes.
  This fix is based independently on main and changes only overlay discovery/binding plus necessary
  source lifecycle handling. Its overlapping Claude binding code must preserve overlay ownership
  when that PR is integrated. Issue #623 owns auth classification; it is excluded here.

## Issue 629 — Unicode terminal prompt redraw

- **Quote follow-up:** PTY `terminal_read` line ranges and CLI quotes now render the retained bytes
  with the existing VT emulator before selecting rows, so cursor edits do not leak into excerpts.
  Raw `tail`, pipe output, stored bytes, profile authorization, routes and DTOs keep their contracts.
  The projection respects buffer trimming and current terminal dimensions; no persistent migration.
  Existing hooks/config/extension IDs and official skill call syntax remain applicable. Web consumes
  the same corrected line projection; site quote docs and the shell-fidelity scenario record it.

- **Native tools / CLI / HTTP / UDS:** existing terminal input, stream, read, wait and quote paths
  preserve UTF-8 characters containing C1 byte values. No IDs, DTOs, routes or flags change.
- **Extensibility / hooks / config:** marker authentication and OSC/DCS security policies remain;
  scanning recognizes controls at character boundaries, including UTF-8 encoded C1 code points. No hook, SDK or config
  changes. The bug also occurs with shell integration disabled.
- **Workspace / profile isolation:** state is held by the existing per-session input/output filter.
  No database or file format changes, migrations, ownership changes or user-state loss. Existing
  sessions need the upgraded daemon/new terminal to use the corrected parser; previously discarded
  output cannot be recovered. Existing size bounds and ACK backpressure remain authoritative.
- **Official skill:** checked `skills/compozy/references/terminal.md`; existing untrusted-output,
  terminal-read and quote contracts remain accurate, with no new operation or guidance required.
- **Web / docs / QA:** browser rendering consumes corrected bytes without frontend production
  changes. The canonical terminal E2E suite owns separate zsh keystrokes, quote parity and reconnect.
  `ET-terminal-shell-config-fidelity` gains the controlled Unicode/ASCII walk; the
  [targeted report](../qa/reports/2026-09-12-terminal-unicode-prompt.md) records evidence and limits.
  Display-width tables, raw-mode visibility (#628), and ZDOTDIR behavior (#626) are outside this fix.

## Issue 616 — Supervised work recovery

- **Native tools:** existing session stop, task inspection/recovery and Loop status IDs and schemas
  stay unchanged. Only verified inactivity settlement queues a bounded linked task attempt.
  `task.run_recovered` carries `reason=supervised_silence`, stop identity and attempt counters;
  its existing `requeue` action contributes to the existing Requeued projection.
- **Extensibility/hooks/config:** no new configuration, hook family, SDK or permission. Existing
  `quiet_after`, `stop_grace` and `max_attempts` own policy. The task enqueue hook observes the
  committed successor. Zero grace remains warning-only. No general session auto-restart.
- **Workspace data isolation:** session/profile/workspace ownership and the exact task lease fence
  gate the atomic write. Success, cancellation, deletion, a pending terminal command or a new owner
  wins over stale recovery. The successor preserves worktree, participation and capability selectors;
  Loop recovery validates the node epoch and owned binding. Borrowed Goals retain their controls.
- **Compatibility:** no database shape change, migration, user-file rollback or public removal.
  The existing verified stop receipt remains until task settlement succeeds, including after restart.
  Multiple owned runs settle independently; a failed candidate does not roll back recovered work,
  and replay selects only the remaining active bindings.
  Prior lease recoveries remain charged; exhausted work is parked for attention. Committed outputs
  remain intact; external partial effects require application-specific reconciliation/idempotency.
- **Official skill:** runtime, tasks and Loop references distinguish Loop silence attention from
  configured session inactivity stop and document retry limits and side-effect boundaries.
- **Web/Docs/QA:** existing Tasks/Loop history and attention consume corrected durable state;
  no Web layout or controls change. Session health docs and `TA-action-run-liveness` are updated.
  Existing store/lifecycle suites plus the CI-only disposable ACP integration own verification.
  Local gates, builds and runtime labs are deferred under the explicit delivery instruction.

Use for changed runtime behavior, public contracts, config, or feature documentation. Record this analysis once in the owning spec/task/PR and link it from implementation slices; update only the affected entries. A small change needs only concise findings. Editorial changes with no runtime contract may state `not applicable — editorial only`.

- **Native tools:** changed `compozy__*` IDs, toolsets, descriptors, schemas/digests, risk flags, capability gates, diagnostics, and CLI/API fallbacks.
- **Extensibility and hooks:** extensions, hooks, skills/capabilities, resources, registries, bridge SDKs, MCP sidecars, and config lifecycle. Config changes co-ship defaults, loader/overlay behavior, validation, docs, and compatibility under SD-013.
- **Workspace data isolation:** classify changed data as global/workspace/session/agent-scoped. Follow `workspace_id` through affected CLI/HTTP/UDS/core/store/web/cache/SSE/event paths and verify the owning boundary prevents cross-workspace leakage.
- **Official CompozyOS skill:** update `skills/compozy/` when public behavior, tools, CLI paths, hooks, capabilities, resources, or memory/network/task semantics change.
- **Web/Docs impact:** name affected `web/` routes/components/hooks and `packages/site` docs, plus their verification owner. Backend changes carry this analysis with the feature.

For an unaffected entry, name the checked surfaces and why the change cannot affect them. Do not create separate artifacts or re-audit unchanged surfaces for every checkpoint. Breaking changes also name delete targets and the user-state/public/internal compatibility regime; apply SD-013 before deletion.

## Session continue and fork — derived sessions, lineage kind, `handoff`

Owning spec: `.compozy/tasks/session-continue-fork/_spec.md` (Part II §Impact Analysis lists delete targets and regimes; ADR-001…008 record the decisions; exploration in `.compozy/tasks/session-handoff/analysis/summary.md`, decisions D1–D9 in `../session-handoff/DECISIONS.md`; peer review round 1 incorporated 2026-09-11). Implementation slices link here and update only affected entries. Program order (D8): `fallback-account` ships first and owns route/account resolution, the accepted-binding commit, and resume affinity; this spec consumes that contract as a hard prerequisite.

- **Native tools:** new `compozy__session_continue` and `compozy__session_fork` in the sessions toolset (risk `mutating`, same permission gate as `compozy__session_create`); `compozy__session_create|rewind|describe` keep their IDs and schemas; session read payloads gain `lineage.kind`, `lineage.origin_message_id`, `lineage.origin_agent_name`, `derivation` (`kind`, `source_session_id`, `seed`, `native_state`, `native_fork_error`, `first_prompt`), and `runtime.acp_caps.supports_fork_session|supports_resume_session` (additive); the compact status read (`GET …/sessions/{id}/status`, `compozy session status`) gains optional `lineage` and `derivation` for continued/forked sessions so the CLI prints its `Origin`/`Derivation` lines (task_05). Tool catalog and digests regenerate.
- **Extensibility and hooks:** no hook, manifest, skill/capability, Host API, bridge SDK, or MCP sidecar change; hooks keep dispatching at their call sites (post-create for the child after its commit); hook payloads that embed lineage gain the optional fields only. ACP: `initialize.agentCapabilities.sessionCapabilities.fork|resume` captured; unstable `session/fork` called only on an idle, bound source that advertises it, under the source's exclusive prompt slot; every ACP notification and callback is routed by session id (bound / fork sink / foreign-dropped) so clone traffic never reaches the source (ADR-003). Failure vocabulary: `provider_error.next_action` gains `handoff` (additive enum value; decorated by the session owner from the typed session type; older clients render it as their neutral step). Config: new `[session.derive] max_replay_bytes` (default 131072, min 4096, hard bound) and `max_message_bytes` (default 16384, 1024…max_replay_bytes), workspace overlay; no other key changes. Event: new `session.derived` written to the daemon event ledger for the child (`compozy logs --session <child> --type session.derived`), emitted right after the committed creation transition. Derive response (task_03): `derived.child_session_id` is always set and `session` is omitted when the recorded child was deleted (`child_deleted: true`).
- **Workspace data isolation:** the child session belongs to the source's workspace/worktree and profile; the derive routes use the workspace route guard (a source in another workspace is 404). Global `sessions` gains `lineage_kind`, `origin_message_id`, `origin_agent_name` (migration `00122` — the spec text says `00110`, but `00110`–`00121` shipped first — lossless backfill from `session_type`/`spawn_role`/`parent_session_id`); per-session `meta.json` documents written before the feature are upgraded at the read boundary by the same rule (regime 1, no rewrite on read). New global side table `session_derivations` keyed `(workspace_id, idempotency_key)` with an immutable outcome; rows are tombstoned (`child_deleted_at`) by the session delete path, never removed. The carried context is one immutable payload in the child's own meta (`imported_context`), taken from one read-only snapshot of the source and never written to the source; attachment bytes are not copied. The optional first message is admitted through the existing `session_prompt_admissions` (key `<derive key>:first`). Nothing crosses workspaces or profiles.
- **Official CompozyOS skill:** `skills/compozy/references/runtime-operations.md` §sessions documents `session continue|fork`, `derive/preview` (`native_fork_possible`, `cut.turn_id`), `lineage.kind`, `derivation`, retry semantics (`replayed`, `child_deleted`), `handoff`, and the two tools.
- **Web/Docs:** `web/src/systems/session/components/session-row-actions.tsx` and `hooks/use-session-topbar-slot.tsx` (two menu items), `components/session-continue-dialog.tsx` / `session-fork-dialog.tsx` / `session-fork-message-action.tsx` / `session-origin-pill.tsx` / `session-continue-divider.tsx` (new), `components/session-status-line.tsx` (pill), `components/session-inspector-origin-section.tsx` (new — Origin + Seed rows in the Context sidebar, since the ledger meta panel was removed by #635; task_04), `components/runtime-activity-notice.tsx` + `lib/provider-error.ts` (`handoff`), `web/src/components/assistant-ui/message-actions.tsx` (fork action), `web/src/systems/os/apps/session/session-window-content.tsx` ("Restart in a new session", `lineage_kind: recovery`). `packages/site`: `sessions/lifecycle.mdx` (new section incl. durable carried context, native bootstrap states, retry semantics), `sessions/resume.mdx` (cross-link), `agents/providers.mdx` (`handoff`), `cli/session/continue.mdx` + `fork.mdx` + `new.mdx` (`--lineage-kind`) + `status.mdx`, `api/sessions.mdx` (three operations, additive fields), `configuration/config-toml.mdx` (`[session.derive]`), `sessions/events.mdx` (`session.derived`). `COPY.md` + `docs/_memory/glossary.md`: `fork` = conversation fork (this feature); recovery button renamed; `handoff` stays a Network term. Verification owners: `_tests.md` (UT/IT/E2E), `eng-ui-screenshot` bundles for `docs/design/opendesign/session-continue-fork/`, QA scenarios `RT-conversation-rewind` (rewind on provenance/derived children; carried context kept; "Fork from here" beside Rewind), `ET-web-session-sidebar-threads`, and `ET-web-sessions-catalog-modal` (row-menu Continue/Fork; task_06) updated and reset to `untested`; new `ET-web-session-continue`, `ET-web-session-fork-from-here`, `ET-cli-session-continue`, `RT-session-derive-native-fork`, `RT-session-derive-retry`, `RT-provider-error-handoff`, `RT-session-lineage-upgrade` (pre-feature home; task_07 plan `docs/qa/reports/2026-09-28-session-continue-fork-plan.md`).
- **Compatibility:** SD-013 regime 1 for the `sessions` migration and the `meta.json` read-boundary upgrade (both lossless); regime 2 additive for routes, CLI verbs, tools, DTO fields, config keys, event, and the `handoff` value; the rewind rule relaxation (`--parent` children become rewindable) is additive public behavior; the label "Fork into a new session" → "Restart in a new session" is a copy change with its QA scenario; `conversationHasParentLineage`, the `Status`-based parent lookup in `validateCreateLineageReferences`, and the id-agnostic `handleSessionUpdate` path are internal hard-cuts with no shim.

## Fallback account — route `command`, agent `fallback_chain`, `use_fallback`

Owning spec: `.compozy/tasks/fallback-account/_spec.md` (Part II §Impact Analysis; ADR-001…005 record the decisions; exploration in `.compozy/tasks/session-handoff/analysis/`, 2026-09-11; issue #617; peer review round 1 incorporated 2026-09-11). Implementation slices link here and update only affected entries. D8 sequences this spec before `session-continue-fork`, which consumes the route `command` and owns the post-acceptance `handoff` offer; this spec solves refusals **before** ACP acceptance only.

- **Public projections:** `RoleFallbackStatus.command_fingerprint` and `AgentPayload.fallback_chain[].command_fingerprint` (additive, `sha256:` over the resolved route command, present iff `command` is set) — consumed by `session-continue-fork` for its Route label.
- **Native tools:** `compozy__agent_create` gains an optional `fallback_chain` argument (array of `{provider, model, reasoning_effort?, speed?, acp_options?, command?}`); descriptor schema/digest regenerate; no ID, toolset, risk flag, or capability gate changes. `compozy__session_create` unchanged (no chain on create; sessions it creates run the agent chain as session-owned launches). Agents observe attempts through `compozy logs --type session.fallback.used|role.fallback.used` over UDS or `GET /api/logs`; the row for an attempt exists before that attempt starts and survives later cleanup.
- **Extensibility and hooks:** no hook, manifest, skill/capability, Host API, bridge SDK, or MCP sidecar change; the two events are observation surfaces, hooks keep dispatching at their call sites. Config: `roles.<role>.fallback_chain[].command` (string, default empty) and agent frontmatter `fallback_chain` (array, default empty), validated by the provider command parser only — **no route-count or command-length limit, so every previously valid `config.toml`/`AGENT.md` keeps loading**; values forwarded literally (no `~` expansion); overlays keep whole-chain replace; SD-013 regime 2 ladder (a): additive. Precedence: explicit route `command` wins on any provider; otherwise the existing provider-aware rule (agent command only on the agent's provider). Failure vocabulary gains `use_fallback` (pre-acceptance only; additive enum value). Events: `role.fallback.used` payload gains `provider_command_fingerprint`; new session-scoped `session.fallback.used` on the daemon ledger. Internal hard-cuts (regime 3, one change): `acp.AcceptedStartError` as the single acceptance fact through driver, session, role, and transient consumers; `ChainOwner` + `Command` on `CreateOpts`/`SpawnOpts`/`TransientModelCall` (one chain owner per launch; role-owned launches never run the agent chain); attempt-aware transcript marker input. The memory-controller chain is live through the write-controller tiebreaker unless `memory.controller.mode = "rules"`.
- **Workspace data isolation:** no new tables. Per-session `meta.json` gains `accepted_route` (`{attempt, provider, model, auth_mode, home_policy, command_fingerprint}`, opaque provenance, session-scoped, committed with `acp_session_id` and replaced on every accepted binding). Resume loads a native ACP id only on a route matching that record; otherwise the id is cleared and context replay runs (reason `accepted_route_missing`). Route commands resolve per workspace config and profile like agent/provider commands today; spawned children inherit a creator command only when neither the attempt nor the agent supplied one and the creator's route is compatible (`compatibleSpawnProviderRoute`). Raw commands never enter the ledger, logs, markers, notifications, or crash bundles (fingerprint only); each refused attempt keeps one `provider_failure` marker attributed to the refused route.
- **Official CompozyOS skill:** `skills/compozy/references/configuration.md` (fallback routes with `command`, no limits, literal values), `agent-definitions.md` (`fallback_chain`, `--fallback-route` with `command` last), `runtime-operations.md` (`use_fallback` in the `next_action` list; pre-acceptance advance vs. keep-available after acceptance until `session-continue-fork`; the two events; role-owned vs session-owned chains).
- **Web/Docs:** `web/src/systems/settings/components/role-fallback-editor.tsx` (Account command field), `web/src/systems/settings/lib/roles-config.ts` + `types.ts` (command on the route value), `web/src/systems/session/lib/provider-error.ts` + `components/runtime-activity-notice.tsx` (`use_fallback` copy). `packages/site`: `configuration/config-toml.mdx` (fallback table + example with absolute paths), `agents/definitions.mdx` (frontmatter table + example), `agents/providers.mdx` (recovery section: before vs after acceptance), `cli/roles/show.mdx`, generated `cli/agent/*` docs. Verification owners: `_tests.md` (UT/IT/E2E), QA scenarios `MS-background-role-fallback` (extended: route `command`, accepted-with-error fence) and new `RT-session-fallback-chain` (work-session advance, exhaustion, no reroute after acceptance, resume affinity) and `MS-settings-role-fallback-command` (settings field), reset to `untested`.

## Session context — composer meter, Context sidebar, usage contract

PR #635 CI follow-up: terminal input waits for an in-flight authenticated marker admission before
checking the journal audit state, including inputs queued behind another submission. This preserves
the existing fail-closed contract without changing native tool IDs, HTTP/UDS/CLI schemas, hooks,
configuration, stored data, workspace isolation, Web layout, or official skill/site instructions.
`ET-terminal-journal-fail-closed` and the existing terminal/API integration suites own verification.

Owning spec: `.compozy/tasks/session-context/_spec.md` (Part II §Impact Analysis lists delete targets and regimes; ADR-001…007 record the decisions; exploration in `analysis/summary.md`; peer review rounds 1 and 2 incorporated 2026-09-11, `qa/peer-review-incorporation-round{1,2}.md`). Implementation slices link here and update only affected entries.

- **Native tools:** no `compozy__*` tool added, renamed, or changed; `compozy__session_describe|events|history` keep their IDs, schemas, and digests (`session_events` now also lists `prompt_delivery` events through the existing passthrough). Agents read context through `compozy session usage -o json [--turns]` over UDS or `GET …/usage` / `GET …/usage/turns`.
- **Extensibility and hooks:** no hook, manifest, skill/capability, Host API, bridge SDK, or MCP sidecar change; `prompt.post_assemble` keeps its contract (a replaced startup prompt is measured as one opaque `system_prompt` span). New public session ledger event type `prompt_delivery` whose canonical content carries a typed `delivery` field (`turn_id`, `sent_at`, `estimate`, `spans[]` incl. `startup_dedup`), recorded after the transport confirmed a prompt; new named session-stream event `session_usage_changed` (`{sequence, turn_id, kind}`) for `usage` / `done` / `prompt_delivery` ledger events beyond the cursor (push and poll paths). ACP decoder: prompt-response cache counters read as `cachedReadTokens`/`cachedWriteTokens` (schema names); the former `cacheReadTokens`/`cacheWriteTokens` spelling stays accepted in v0.4.0 and v0.5.0 with a once-per-session deprecation warning naming the replacement and is deleted in v0.6.0 (boundary alias only); `usage_update._meta` decoded tolerantly (objects only), passed through `redact.ClaimTokensJSON` (keys named `claim_token` removed at any depth, token-shaped values replaced), kept on the recorded usage event content and forwarded as `meta` on live/prompt/transcript usage payloads (additive), never persisted into columns or counters; `used < 0`, `size ≤ 0`, negative counters dropped at the boundary with one warning per session. Config: no new key; `[session.compaction] enabled|pressure_threshold` populate `context.pressure_threshold` only when the current report carries an agent-reported size.
- **Workspace data isolation:** globaldb `token_stats` gains nullable `cache_read_tokens` / `cache_write_tokens` by Goose migration (accumulated per session/agent on done turns like the other counters; historical rows read NULL → absent); no session-DB schema change (the delivery manifest is a ledger event archived and wiped with the session ledger; usage payloads gain a wire-only `sequence`); `context` is derived per session from that session's ledger (observations, deliveries, compactions), projection, and effective model; the catalog window lookup is keyed by the session's provider/model; nothing crosses workspaces or sessions.
- **Official CompozyOS skill:** `skills/compozy/references/runtime-operations.md` §session usage documents cache totals, `context` (`state` incl. `unavailable`, `used`, `size`, `size_source`, `stale`, `sequence`, `pressure_threshold` eligibility, `injected` rows), `--turns` (union rows ordered by sequence, compaction markers with `span_archived` — a fact, never a completion claim), the `prompt_delivery` event, and the `session_usage_changed` stream event.
- **Web/Docs:** `web/src/components/assistant-ui/session-composer-action-row.tsx` (new context control), `web/src/systems/session/components/session-inspector*.tsx` (tab-less Context sidebar; Memory/Files/Vault sections deleted — Vault stays at `/vault`, file audit stays in the transcript roll-up, the ledger stays on `getMemorySessionLedger`), `web/src/systems/session/hooks/use-session-context.ts` + `use-session-usage-turns` (new), `hooks/session-stream-source.ts` + `hooks/session-live-tail-runtime.ts` (named `session_usage_changed` listener and invalidation), `web/src/systems/os/apps/session/use-session-window-controller.tsx` (usage query ungated; ledger/vault wiring removed). `packages/site`: `cli/session/usage.mdx` (cache lines, Context block, `--turns`), `api/sessions.mdx` (`context`, `getSessionUsageTurns`), `sessions/events.mdx` (`prompt_delivery`). Verification owners: `_tests.md` (UT/IT/E2E), `eng-ui-screenshot` bundles for `docs/design/opendesign/session-context/`, QA scenarios `RT-052`, `RT-session-cost-provenance`, `RT-024`, `ET-web-session-inspector-toggle` updated and reset to `untested`; new `ET-web-session-context-meter`, `ET-web-session-context-sidebar`, `ET-cli-session-usage-context`, `RT-acp-usage-cache-meta`.

PR #635 review follow-up: usage SSE keeps its own replay watermark and reads ordered ledger events.
Web teardown flushes pending aggregate and turn invalidations; an unavailable context snapshot no
longer hides newer token/cache totals. The window owns activity subscriptions and passes presentation
data into the Context panel. CLI TOON retains report timestamps, delivered rows, and separate raw
turn/usage/delivery/span/compaction arrays. HTTP/UDS schemas, native tool IDs, hook/config contracts,
and persisted workspace isolation are unchanged. The official runtime-operations skill and the
owning CLI/context-sidebar QA scenarios document these corrections.
The final review extends the terminal marker lock through journal reservation, preserving output
progress outside PTY writes. Text-only session errors now remain visible with their original detail;
empty errors and attributed stops retain their existing filtering. This changes Web presentation
only, with no new event fields, native tools, hook/config behavior, persistence, or skill contract.
Predicate coverage follows its library owner; the transcript-grammar scenario records the change.

## Marketplace catalog — one kind, plugin marketplaces as sources

PR636 CI repair: inventory OpenAPI now declares the existing HTTP/UDS workspace/profile selectors; generated clients, Web request/cache/hook ownership, CLI/site guidance and the official skill agree. Workspace-installed kit resources and skipped diagnostics remain visible instead of suppressing the section. Extension detail reads consume resource-qualified observed MCP health and passive auth state; real Settings probes publish observations without making extension reads launch processes. Dev candidates retain local-path provenance; published workspace installations read package network consent, while dev overlays retain their own consent. No migration, config, hook, extension SDK or credential-format change is introduced. Existing runtime/native/E2E and inventory/cache suites own verification; the current head still requires final gate and CI.

Final review refresh correction: the canonical HTTP/UDS/CLI/native catalog envelope gains optional `refreshing`, owned by the daemon flight lifecycle and independent of stale/error aggregation. Generated OpenAPI/Web types and API docs co-ship; Web follows pending work across mixed healthy/failed sources. No config, hooks, SDK extension contract, stored state, scope or secret changes. The existing service HTTP/SQLite integration and Web query suites own completion/backoff checks.

Task03 input recovery: the validated candidate manifest supplies `input_definitions` with missing IDs in `extension_inputs_required`. HTTP/UDS and native tools share acquisition error mapping; native partial updates retain `operation_error` with completed results. Generated OpenAPI/Web types, Web error adapters and the install guide co-ship. Clients retry the same scoped update; no second preview, source reconstruction, compatibility decoder, config change or persistence migration. Only absent declarations are exposed, never stored values or secret refs. Final input-dialog journeys remain with tasks09/10.


Owning spec: `.compozy/tasks/marketplace-catalog/_spec.md`; Pedro's explicit 2026-09-12 Marketplace-only hard-cut authorization is recorded in ADR-005/007. This amendment supersedes the previous one-release translations, dual feeds and v0.6.0 removal plan. It is an approved implementation contract, not a claim that branch cleanup is finished. `hardcut-removal-plan.md` records observed code and owning tasks.

- **Preserved boundary:** existing extension packages, IDs, versions, artifact bytes/digests, manifests, supported acquisition/update refs, lifecycle APIs, installed records, enablement, provenance and credentials. Manual MCP definitions/auth and installed local/ClawHub-origin skills continue loading. Pedro explicitly authorized disabling MCP declarations embedded in installed ClawHub/SourceMarketplace skills on 2026-09-13; their files and provenance remain. Final extension removal retains baseline instance-binding/exclusive-secret cleanup with shared/manual protection and rollback. User-state migrations remain lossless; only derived old-kind projection rows have ADR-001 deletion approval.
- **Native/CLI/API:** one Marketplace catalog/source contract. Remove old kind/grouped-search/MCP-install/remote-skills acquisition routes on HTTP and UDS, old CLI flags/arity/verbs and native kind arguments. Regenerate schemas/digests/OpenAPI/help. Preserve current extension and manual Settings operations; no translation or success stub. Installed-skill tools stay local.
- **Extensibility/config:** keep real input validation/readiness/rollback, manifest auth, digest consent, owner-qualified tokens/DCR/vault, scope isolation and plugin grammar adapters. Remove requested install runtime_name, manual-MCP-vault import and owner-less extension management aliases added for the old installer. Automatic sticky runtime names and internal runtime lookup remain. Remove obsolete Marketplace config consumers; the config owner archives removed values once without changing unrelated settings. No compat package or shim telemetry.
- **Feeds/Web/Docs:** only v3/extensions.json and v3/marketplaces.json; no root v2 publication, semantic adapter or reader fallback. Preserve existing extension distribution. Remove Web kind redirects/tab adapter; Browse/Installed/detail remain. Site uses one schema and current docs; release notes list removed Marketplace surfaces. Official skill updates follow the same contract. Historical design boards/review reports do not override the amended spec.
- **Workspace/data:** retain source/ref-qualified joins, scoped installation/input/override ownership, migration integrity, manual-owner backfill and isolation across profiles/workspaces. These are new-product safety requirements, not dispensable compatibility debt.
- **Evidence/delivery:** current UI checkpoint 220a59082 includes complete snapshot reads, cancellation-before-invalidation, consent, batch pending exclusion and truthful errors; its redirect contract is now superseded. Tasks01/02/05 own hard-cut removal; task05 no longer depends on task04 old-installer parity. Tasks09/10 own final QA/visual/state-preservation walks; make/heavy gates and review remain final-only under session instructions. No task is completed by this documentation amendment.

Task05 core hard-cut checkpoint (2026-09-13): HTTP/UDS registrations and core grouped/kind handlers, remote skill services/packages, generic MCP/skill detail DTOs and slug/name fallback joins are removed. Canonical extension joins use source/ref identity; exact installed detail uses installed_name only. Refresh has no kind selector and returns sources[]; CLI, Web, OpenAPI and generated site API co-ship. The daemon registers only the extension feed. Existing lifecycle suites now exercise extensions, and real local v3 feed/package HTTP/UDS/CLI parity passes. Config archival and remaining domain Kind/Store/decoder removal are still task05 work; task02 owns publisher/site-guide cleanup. Final QA remains09/10.

Task05 decoder checkpoint (2026-09-13): runtime decoding and projection now accept only the extension v3 schema. Retired MCP/skill launch parsers are deleted; the shared input grammar remains. File and HTTP source roots both address v3/extensions.json, with no root fallback; catalog validation also requires v3/marketplaces.json. Existing extension entries/package bytes are untouched. Focused source and real HTTP/SQLite failure-preservation suites pass. Remaining Kind service/store signatures are the next task05 cut. The copied testdata/v2decoder still has a production publisher import and remains a mandatory task02 deletion with its v2 publication paths, not a supported runtime decoder.

Round-3 incorporation: task01 → task05 → task02 → task03 → task04 is now explicit. Task05 owns all retired Kind/source/DTO/CLI/tool paths, including hand-written --runtime-name/schema strings and public core/daemon owner-less diagnostics; preserve discovered-resource execution. Task02 owns v3 publisher/site/fixtures after that cleanup. The actual schema is 00110 projection/provenance/inputs/overrides (task01), 00111 secret-binding identity/active state (task03), 00112 owner keys and 00113 installation attachments (task04). IT-016 owns the complete v109 upgrade; IT-021 owns attachment update/detach/rollback. Typed input/MCP persistence boundaries and digest-pinned pre-install inspection are retained with explicit owners. Canonical listing slugs are derived while existing feed acquisition refs and installed update provenance stay intact. Baseline CLI bare-ref selection is preserved; only its branch-added duplicate preflight loop is consolidated. Stored retired Marketplace locations keep their layout and render normal not-found with Back, without a redirect/hydration alias.

Task05 first implementation slice: install/preview reject the removed runtime_name field, CLI and native schemas no longer advertise it, and automatic allocation/rollback remains. Inputs reject manual MCP vault imports without altering stored credentials. Settings GET/auth and diagnostic lookup require an extension owner; manual defaults and discovered ResourceID execution remain. Focused HTTP/UDS Settings, CLI, daemon, secret isolation and real SQLite publication evidence is in marketplace-catalog/memory/task_05.md. Remaining Kind/source/config/acquisition removals are pending; final QA remains09/10.

Concrete documentation owners: task05 rewrites RELEASE_NOTES.md's pending translation/runtime-name promises and skills/compozy/references/tools-and-skills.md; task02 rewrites catalog/README.md and removes packages/site/content/docs/skills/marketplace.mdx. These are pending implementation co-ship targets, not claims that those runtime surfaces have already been removed. The incorporation record names all thirteen findings and the corrected baseline attribution for B-035.

PR #624 review follow-up: Marketplace mutation settlement cancels in-flight reads before canonical invalidation, so an initial search started before installation cannot suppress the authoritative installed-state reread. Server filters, workspace query keys, pagination envelopes and installed counts retain their owners. No native tool, CLI/HTTP/UDS schema, hook, extension, configuration or persisted-data change is needed; the official skill and site install instructions remain valid. ET-009 and the existing Marketplace acquisition E2E own this Web cache behavior. Evidence is recorded in `docs/qa/reports/2026-09-10-qa-execution-unblock/pr-review-resolution.md`.

## Issue 595 — Goal lifecycle and attention

Owning delivery: [PR #601](https://github.com/compozy/compozy/pull/601). Detailed regression and runtime evidence: [focused QA report](../qa/reports/2026-09-10-issue-595-goal-lifecycle.md).

- **Native tools:** Existing Goal control/read, session stop/removal, and Loop cancellation surfaces keep their IDs, schemas, and authorization. Run terminal state and quarantine govern Goal status. Stopping a session cancels its session-origin Goals; failed cancellation remains retryable in the existing stop settlement receipt.
- **Extensibility and hooks:** No hook, SDK, configuration, or registry shape changes. Cancellation uses the existing aggregate and durable cleanup outbox. Catalog Loop lineage and window dismissal retain their semantics.
- **Workspace data isolation:** Binding adoption validates workspace, task, control, phase, session, handle, and epoch. Session Goal cancellation reads immutable profile/workspace scope. Existing receipts preserve session/runtime identity across restart. No schema migration or historical record deletion.
- **Official CompozyOS skill:** `skills/compozy/references/loops.md` documents cancellation, orphan recovery, context ownership, and bounded supervision freshness.
- **Web/Docs:** Session badges, Goal strip, Tasks, and Loop detail consume corrected backend projections; no client-side badge clearing. The Goals guide and GL/LP/RT QA slices accompany the change. Canonical lifecycle suites and focused CLI/API/Web runtime walks own validation; remaining delivery gates run in CI.

## Merged PRs 596, 597, 599, 600, and 601

The [main-branch remediation report](../qa/reports/2026-09-10-merged-pr-ci-review-remediation.md)
owns the follow-up audit: captured terminal creation scope, stale-navigation
rejection, explicit title-disclosure action names, public session imports, and
review/CI disposition. Existing native, persisted, configuration, and official
skill contracts remain unchanged.

## Issue 602 — Embedded private endpoint verification

- **Native tools:** Gateway status/audit, CLI and HTTP/UDS keep their IDs, DTOs and authorization. Existing provider causes gain safe failure classification; only a core-verified route becomes advertised.
- **Extensibility and hooks:** The connectivity endpoint wire contract adds optional `verification_address` for a bounded loopback TCP relay. Go/TypeScript SDKs co-ship through codegen. Omission retains existing behavior; public proof rejects the field. No config, hook or manifest permission changes.
- **Workspace data isolation:** Provider transports remain global gateway runtime resources, owned per tier and torn down before the node closes. No persisted schema or data migration. Public addresses exclude the relay; endpoint identity comparisons include it.
- **Official CompozyOS skill:** Runtime Gateway guidance explains embedded private proof and safe diagnostic classes.
- **Web/Docs:** Existing Gateway views consume provider causes without new UI state. Tailscale and extension-authoring guides describe transport, proof and recovery. `RT-connectivity-provider-route` owns the affected scenario. TLS, nonce, redirects, tier binding and public outbound policy retain their owning gateway suites; provider tests cover relay lifecycle.

## Issue 603 — Effective Dream health reporting

Owning evidence: [focused QA report](../qa/reports/2026-09-10-issue-603-dream-health.md).

- **Native tools:** `compozy__memory_health` reports the same scoped Dream role state as CLI/HTTP/UDS health and role diagnostics. Existing IDs, descriptors, schemas, authorization, and error fields are retained.
- **Extensibility/hooks/config:** No new keys, hooks, SDK behavior, or background work. Role configuration and provenance stay owned by the existing resolver; Dream execution eligibility is unchanged.
- **Workspace data isolation:** Health passes its selected workspace and context to role status resolution. Unscoped API/Settings reads use the global role rather than aggregating unrelated workspaces. Memory catalog/profile ownership is unchanged; no persistence or migration changes.
- **Official skill:** `skills/compozy/references/memory.md` explains both opt-ins and diagnostic-only reads.
- **Web/Docs:** Memory health/Settings consumers receive corrected existing fields. No rendering or interaction change. The Memory System guide and MS-011 describe the truth table and same-scope comparison.
- **Compatibility:** Public wire shapes and user state are unchanged; this corrects a boolean projection with no deprecation or migration.

Issue #603 delivery also pins the site preview install command to the repository's Bun 1.4.0 with `--frozen-lockfile`: the preview image previously ignored the newer lockfile and installed unverified dependency versions. This affects dependency installation only; no runtime, wire, configuration, or user-data contract changes.

## Issue 605 — Mixed reasoning and tool work groups

- **Native tools:** No IDs, schemas, descriptors, CLI, HTTP or UDS changes. The Web projects existing reasoning and tool parts into one ordered work segment.
- **Extensibility and hooks:** Tool registration, MCP, extensions, hooks, bridge progress and config remain unchanged. Deliberate terminal tools retain their separate interactive rows.
- **Workspace data isolation:** Group anchors and disclosure state remain local to each message's timeline store. Persisted transcript, session/workspace keys and SSE contracts are unchanged; no migration or recovery action is needed.
- **Official CompozyOS skill:** No update required: public agent operations and protocol semantics are unchanged.
- **Web/Docs:** Timeline projection, work entries, group identity/equality, turn folds, and find reveal consume the widened internal entry union. The virtualizer already estimates work from entry count and measures actual expanded DOM. RT-048 and RT-055 own the updated behavior; canonical projection, thread/navigation and scroll suites plus rendered QA verify it. Public site documentation has no changed API or operational instructions.

## Issue 606 — Durable notification acknowledgement

- **Native tools:** no native IDs, tool descriptors or CLI verbs change. Additive operator HTTP/UDS
  `GET /notifications/attention` and `POST /notifications/attention/acknowledge` routes expose the
  notification read/acknowledgement contract. Existing task decisions, task triage, presence and
  session attention-summary surfaces keep their meanings.
- **Extensibility and hooks:** no hook, bridge preset, delivery cursor, SDK or configuration changes.
  Acknowledgement does not emit source completion or approval events.
- **Workspace data isolation:** exact occurrence receipts belong to a profile and actor. Home
  snapshots bind workspace/global scope plus profile lens; bell snapshots contain all workspaces and
  source profiles, with receipts belonging to the selected destination profile. Bulk writes only
  accept IDs captured in that snapshot, commit atomically and are idempotent. New occurrences race
  safely outside old snapshots. Task inbox triage filters the task candidates; raw escalations keep
  their existing lifecycle semantics.
- **Compatibility:** migration 109 adds receipt and snapshot tables without rewriting source data.
  Snapshots expire after 24 hours; receipts persist. Old databases migrate through the canonical
  schema generator. New overview fields are additive; the Home attention count now means unread
  occurrences. Existing runtime/session/task status and decision APIs are unchanged.
- **Official CompozyOS skill:** the native-tools reference documents the operator-only API, exact
  snapshot acknowledgement and separation from source actions.
- **Web/Docs:** bell, title count and Home use server receipts, show mutation errors, and invalidate
  both corresponding caches after settlement. Individual controls do not activate their row.
  Notification preset docs distinguish inbox acknowledgement from delivery configuration. Updated
  bell/Home/title QA scenarios record the new contract. SQLite, overview and existing API/component/
  browser suites own coverage; broad local gates and rendered labs are deferred to CI by explicit
  user instruction for this delivery.

## PRs 607–611 — Main integration remediation

The [integration report](../qa/reports/2026-09-10-pr-607-611-integration.md) records all five guarded squash results, first-round review dispositions, unpublished notification-worktree fixes and CI regressions. Approval receipts follow meaningful approval transitions; profile deletion removes owned receipts and snapshots. Continuous prose retains search indices, and window opening uses the same snapshot revision for identity lookup and command admission. Public wire/config/native-tool shapes and underlying source-state actions remain unchanged. Existing profile, observer, API, transcript and window-manager suites own verification, with delivery gates and rendered journeys in CI under the operator's explicit override.

## Issue 615 — Hide internal Goal queue rows

- **Native tools and CLI:** `compozy__session_inputs_list`, `compozy__session_inputs_clear`, and `session input list|clear` share the store's operator population with HTTP/UDS. `compozy__session_input_cancel` uses the protected remove path; `compozy__session_input_replace|promote` retain their ownership checks. Goal-owned rows are omitted from queue lists and `queue.entries`; direct remove rejects Goal IDs. Existing replace/promote ownership restrictions remain. Goal controls and dispatcher selection retain their own paths.
- **Extensibility/hooks/config:** Extension `sessions/inputs/list` receives the same filtered rows; mutation protections are shared. No DTO, tool ID, hook, config, or SDK shape changes.
- **Workspace data isolation:** The query remains session-scoped behind existing workspace authorization. Clear atomically cancels and traces only public queued inputs; dispatching rows and current-generation queued Goals carry forward to the new input generation so Goals remain eligible. Stale queued Goals are not revived. Goal fencing/terminal state is untouched; no schema migration or user-state loss. Attachment retention uses the same list; Goal prompts carry no attachments.
- **Official CompozyOS skill:** Runtime operations and extension-authoring references describe the internal Goal exclusion and dedicated lifecycle controls.
- **Web/Docs/compatibility:** Generic owner attribution remains for other actors; Goal-owned strip fixtures become agent-owned. This restores the pre-beta.23 public queue contract after #557 removed the owner filter, without a DTO change or compatibility shim. RT-019 owns the focused clear/list/dispatch QA; RT-059 receives the queue-strip expectation.

## Issue 613 — Stopped-session queue reads and deletion recovery

- **Native tools / public surfaces:** HTTP and UDS `GET .../sessions/:id/prompt/queue` now read persisted stopped sessions using the normal session identity lookup. No route, DTO, native tool ID, schema, CLI flag, or mutation authorization changes. Missing sessions and storage failures retain their errors.
- **Extensibility / hooks / config:** no hooks, extension contracts, provider configuration, or runtime startup behavior changes. Reading a queue does not bind or resume a provider.
- **Workspace / profile isolation:** route/profile guards and persisted session owner resolution remain authoritative; Web query keys retain workspace and session identity. Deletion cancels the whole session detail query prefix, suppresses queue observers while pending, removes it after success, and invalidates it after failure because stopping can precede rollback.
- **Compatibility / recovery:** additive read behavior; no migrations or data shape changes. Deletion now retries a pending stop receipt through the existing stop lifecycle instead of permanently rejecting each retry after the dependency recovers. Still-failing settlement preserves its error, stopped history and attachments. Existing pre-commit rollback and durable post-commit tombstone reconciliation remain intact. Failed deletion is logged with its session identity and underlying error; Web displays the returned error instead of discarding it.
- **Official skill / Web / docs:** runtime operations reference documents stopped queue reads and safe retry. Session controls and runtime observers use lifecycle-aware queue polling and stop interval retries for terminal responses, including older daemons returning `session_not_promptable`. Backend failures remain visible. QA owners: RT-014 and RT-020; existing session manager, query, mutation, route-control, daemon Goal E2E, and browser session-hardening suites.
- **Evidence boundary:** the reported host DELETE 500 lacks a response body and was not replayed on user data. Read-only status/history/Loop inspection does not establish its cause. Source tracing identified a reproducible recovery trap: `stageSessionDelete` rejected pending stop settlement without retrying it. Existing stop tests demonstrate the `ErrRecoveryPersistence` path; the deletion suite now exercises failure and retry with and without manager restart. CI owns disposable stopped-session/Goal deletion and rendered browser evidence. Local gates/builds/broad tests/labs are explicitly deferred by the user; scoped formatting and diff inspection precede commit. The issue is incomplete until the deletion failure is reproduced or the exact external reproduction limitation is documented with CI results.

## Session list bulk actions

- **Native tools:** None added or changed. Bulk actions are Web client fan-out over existing per-session stop, archive, unarchive, and delete operations; tool IDs, HTTP/UDS routes, and DTOs remain unchanged.
- **Extensibility/hooks:** No hook, extension, bridge, or configuration changes. Each mutation retains its existing cache and lifecycle behavior.
- **Workspace data isolation:** Selection is transient and local to each list instance. Every mutation reuses the host's workspace-scoped per-session routes. Selection is pruned on every catalog update and cleared on scope/Archived changes; all-workspaces groups offer no selection because row actions are unavailable there. No persisted data or migration changes.
- **Official skill:** Checked `skills/compozy/references/runtime-operations.md` and session delete/archive references. They document CLI/API lifecycle operations, not selection in the Web sessions list, so no bulk note or contract change is required.
- **Web/Docs:** The shared list, row, selection bar, lifecycle hook, and all existing delete-dialog hosts support selection and sequential batches. The Checkbox primitive renders its existing indeterminate state. `ET-web-session-list-bulk-actions` owns new QA coverage, with an impact flag in `ET-web-session-sidebar-threads`. Checked `packages/site/content/docs/sessions/lifecycle.mdx`: no sidebar-delete instructions require updating. `COPY.md` has no session-list label registry. Canonical unit/component suites, Storybook builds, and the daemon-served `session-bulk-actions.spec.ts` own automated verification; the controller owns visual parity against the unchanged design board.

PR #619 review remediation preserves the same surface and isolation contracts: editable controls retain text-editing shortcuts, a one-session bulk failure exposes its daemon error and retry, failed targets beyond the five-row preview remain named with their errors, and selection derives from current catalog membership before catalog/selection-store-driven pruning. The existing list/dialog/lifecycle suites own regression coverage; CI owns the final integration run.

## QA workspace hook dispatch identity — 2026-09-11

BUG-20260911-workspace-hooks-not-dispatched: workspace declaration scoping now uses the registered workspace ID, matching actual session, task-run and window hook payloads. The durable directory identity is not changed. The owning canonical daemon integration fixtures distinguish both IDs, dispatch real hook subprocesses and verify a foreign task workspace does not execute the hook.

Native compozy__hooks_* and CLI/HTTP/UDS hook management retain their schemas and restart-required lifecycle; effective matcher IDs now agree with workspace info and execution payloads. Config-backed, agent and skill declarations share the corrected scoping function. No database, config syntax, provider auth, public route or DTO changes. Resource projection rebuilds the config-owned binding snapshot on restart/reload using the registered scope; existing ownership and permission filtering are retained.

Web consumes the effective hook catalog unchanged. Official skills/compozy/references/extensions.md documents the existing registered-ID contract; site hook declarations already use public ws_ IDs. GL013 requires fresh public admission-race replay after repair; task-run, window and watched-skill integration cases are adjacent checks. No test fixture replaces the real Cursor acceptance walk.

The HTTP/UDS hook catalog handler now resolves alias/name/path input to ResolvedWorkspace.ID as the native hooks tool already does. The canonical core/hooks_test.go fixture keeps different registered/directory IDs and rejects the wrong query identity before the correction (workspace-hooks-catalog-red.txt). The existing registration-refresh unit fixture now queries by the same registered ID; its earlier durable-ID filter no longer represented the public catalog contract.

## Built-in open-design extension

- **Native tools:** adds extension-owned `ext__open_design__lint_artifact`; no `compozy__*` IDs or
  core CLI schemas change. Its typed paths input, original findings output, file digests, and
  read-only/read-risk descriptor publish through the existing provider and tool invocation surfaces.
- **Extensibility and hooks:** boot reconciles the new bundled extension and its declared profile;
  it uses the existing managed-install lifecycle unchanged. Two agents, three explicit skills,
  two curated design references, and one opt-in native Loop are added. An isolated, tool-capable
  critic returns a typed verdict; a second native lint confirms file digests before fail-closed
  completion. The review skill supplies explicit per-run limits through existing config_overrides;
  no loop-engine changes are needed.
  All guidance and lint code is maintained locally, with no upstream download or sync. No
  hook/config keys, sidecars, viewer, custom capture service, or artifact registry are introduced.
- **Workspace data isolation:** HTML belongs to the active workspace under `docs/design/`.
  The linter consumes daemon-authenticated workspace context, never a caller-supplied root, opens
  artifact components without following symlinks, rejects special files, and limits input/output
  and process lifetime. Designer instructions prohibit production-code changes as a design side effect.
- **Official CompozyOS skill:** `skills/compozy/references/extensions.md` documents profile resources,
  design/review behavior, Node/browser prerequisites, and locally maintained guidance.
- **Web/Docs:** existing profile, session, extension inventory, and Loop inspector surfaces expose
  the resources. Existing agent catalogs and session pickers bind their reads to the selected
  profile, allowing this profile to be used through the normal session workflow. Existing agent
  definition mutations, conflict refetches, and edit drafts preserve that profile identity; pending
  mutations update only their original profile cache. OpenAPI and the generated Web client now
  describe the seven optional profile query selectors already supported by these handlers.
  Definition mutation lookup reuses the effective catalog: extension AGENT.md files retain digest-
  checked customization at their effective source and can be duplicated into authored definitions.
  Package-owned agents cannot be deleted individually; the response directs operators to disable
  the extension. Internal package ownership is preserved at the catalog projection.
  Named-agent Soul/Heartbeat read, validate, write, delete, history, rollback, status, and wake
  routes accept 14 additive optional profile query selectors; CLI transport already forwards the
  global selector. The Web includes Profile in their queries, mutation variables, and editor identity.
  History and rollback filter by the existing persisted winning source path; session status/wake
  validates the persisted Profile and agent. No state shape or request body changes.
  The existing Marketplace bundled catalog, detail route, search, sitemap, and build inputs include
  Open Design alongside Spec Cycle. No new viewer or Web component is introduced. The new extension guide and ET-open-design scenario own
  user guidance and runtime/visual QA; RT-077 owns authored-context Profile editing. Package tests
  retain original lint cases plus local correctness regressions and resource compilation.
- **Compatibility:** additive built-in resources and tool namespace, no persisted user-data schema
  or public replacement. Profile query documentation is additive; existing handler semantics remain
  unchanged for default-profile authored definitions; non-default Soul/Heartbeat operations now
  resolve the selected source and history. Existing extension AGENT.md customization remains
  supported, and duplication leaves the original unchanged. Individual package-owned deletion is
  explicitly rejected. Existing bundled install policies preserve edits across unchanged-bundle
  restarts; a bundle upgrade can replace its managed files. Authored sidecar protections are unchanged.

### PR #625 remaining review findings and main rebase

The existing Open Design audit also covers body-level custom-property inheritance and exact slide
theme tokens in the linter; its tool ID, schemas, file isolation, and generated bundle owner are
unchanged. Missing or whitespace-only Heartbeat wake sessions now fail shared HTTP/UDS validation
with 400, matching the existing required `session_id` contract. Profile-bound session authorization,
source-specific history, and the authorized session identity from main remain intact. No config,
hook, database, official skill interface, or generated API shape changes are needed. Web provider
selection and editor validation were extracted without changing behavior. RT-077 and ET-open-design
record the public replay; the remediation report inventories every review source and its disposition.

The post-push continuation also normalizes equivalent theme selectors, excludes supported global
token declarations from raw-color counts, and scans nested structural emoji. These corrections
remain within the same native linter contract and workspace boundary; ET-open-design records the
public replay. No Web, hook, config, persistence, or official skill surface changes are required.

The following linter-only continuation preserves custom-property importance and recognizes icon
containers with delimiter-aware names across non-void tags. The same tool/schema/isolation audit
and ET-open-design scenario apply; no additional surface contract is introduced.

## Issue 623 — Structured native authentication probes

- **Native tools / CLI / HTTP / UDS:** existing provider status/probe and pre-start
  admission share the top-level JSON `loggedIn` verdict. False requests native
  login; invalid/missing/non-boolean/duplicate verdicts remain unknown. Exit zero
  and absence of a classified error are required for success. Unknown admission
  remains fail-closed. No native IDs, routes, or DTO shapes change.
- **Extensibility / hooks / config:** no new configuration or hook contracts.
  Existing text probes retain their vocabulary, with negative/error evidence
  preceding positive text. Structured output retains only its boolean verdict;
  unrecognized structured output is represented as an empty object, avoiding
  identity-derived substring classifications and disclosure. Local and prepared
  sandbox runners sanitize before bounding; API/CLI projection also sanitizes
  injected runner results. Native login execution is unchanged.
- **Workspace / profile isolation:** native credential ownership, operator-home
  policy, prepared executable identity, pre-start cache keys, and workspace/profile
  resolution remain unchanged. No schema, persisted-state migration, or host
  credential change. Previously collected support bundles are not rewritten.
- **Official skill / Web / Docs:** the agent definition reference and provider
  configuration guide document direct JSON support. Web consumes the same state
  and safe probe payload without UI changes. RT-026 owns the targeted replay;
  [the QA report](../qa/reports/2026-09-12-issue-623-claude-auth.md) records evidence
  and platform boundaries. Cursor/Codex output shapes and overlay discovery remain
  outside this fix.

PR #632 prefix remediation remains within the same projection boundary: bracket
log labels retain text compatibility; warning-prefixed JSON is reduced to its
verdict, and recognized prefix errors retain their safe recovery classification.
No additional public, config, persistence, or Web contracts change.

## Issue 626 — Zsh startup directory fidelity

- **Native tools:** terminal open/write/wait/read retain their IDs and schemas. Zsh user startup
  files see the original ZDOTDIR state and subsequent startup-file changes; authenticated markers
  still load after the interactive rc. No daemon authority or journal grammar change.
- **Extensibility/hooks/config:** existing `terminal.shell_integration` controls injection. No new
  key or hook. User `.zshenv`, `.zprofile`, `.zshrc`, `.zlogin` and `.zlogout` retain their ordering;
  unset versus set ZDOTDIR and its export attribute survive the temporary startup routing.
- **Workspace data isolation:** private per-process shim files remain ephemeral and cleanup-owned.
  User startup files are sourced without rewriting them. Child shells inherit the user's environment,
  not the marker nonce. An early child entering the shim from a global startup file restores the
  user directory/export state and bypasses parent marker injection. Temporary directory metadata
  is bound to the current private shim and removed before user files run; stale daemon metadata
  cannot select child routing. Profile/workspace ownership is unchanged.
- **Compatibility:** internal startup fix with no database, wire, CLI, or config shape change;
  no migration or recovery command. Bash, fish and disabled-integration paths are unchanged.
- **Official skill:** terminal operation guidance and structured interfaces remain valid; no skill
  resource edit is required for this transparent configuration fidelity fix.
- **Web/Docs/QA:** Web Terminal uses the corrected daemon PTY path without renderer changes.
  `ET-terminal-shell-config-fidelity` includes startup redirection, plugin loading and nested shells.
  The real-zsh PTY replay and platform limits are recorded in the linked QA report.

## Issue 628 — Nested raw terminal input

- **Native tools:** `compozy__terminal_write`, `compozy__terminal_request_input`, CLI respond,
  and HTTP/UDS/WebSocket input retain their schemas and authorization. Unix ordinary raw input
  no longer emits automatic hidden-input markers. Explicit redaction remains accepted for no-echo
  canonical and raw readers, and rejects echo-enabled input at admission and delivery.
- **Extensibility/hooks/config:** no new configuration, hook, SDK, or capability. Existing input
  events retain trusted redaction/length metadata. Process identity no longer guesses secrecy.
- **Workspace data isolation:** no database or layout migration; existing session/profile/workspace
  checks and journal reservation remain authoritative. Previously redacted recordings stay intact.
  Ordinary raw input is now journaled as ordinary input; raw secrets require explicit redaction.
- **Compatibility:** public shapes and error codes remain; the internal PTY interface adds an echo
  query to separate automatic classification from explicit hidden-input admission. Windows retains
  console echo behavior. No public removal or deprecation shim is needed.
- **Official skill:** terminal input-request guidance documents canonical detection, explicit raw
  secret entry, and the limit that application-rendered input cannot be hidden by termios.
- **Web/Docs/QA:** existing Terminal rendering consumes the corrected stream without component or
  layout changes. Terminal safety and recording docs explain the same limits. The affected
  `ET-terminal-redaction-boundaries` and `ET-terminal-agent-handoff-input` scenarios gain raw-mode
  cases. Canonical PTY, session input/recording, and real HTTP/WebSocket integration suites own
  regression evidence; changes do not affect Unicode counting or shell environment setup.

Task05 MCP installer removal: deleted mcp install and POST /api/settings/mcp-servers/install across transports, Settings, contracts and generated consumers. Current extension acquisition and manual MCP Settings/auth remain; the real manual-secret/executor integration passes. Official skill and MCP/vault docs are current, and retired MCP install QA scenarios link final replacement walks. Evidence and remaining removals are in marketplace-catalog/memory/task_05.md.

Task05 remote skill acquisition boundary removal: deleted CLI search/install/update/remove and HTTP/UDS lifecycle routes, DTOs and Web adapters/hooks/mocks. Local skill loading, provenance, creation/exposure and session attachment containment remain; filesystem containment moved to fileutil with its existing security cases. Generated contracts/CLI/site API and official skill co-ship. ET-web-marketplace-skill-install is retired. The old Kind remote-discovery service/packages and config migration remain pending in task05; final walks stay09/10. Focused evidence: marketplace-catalog/memory/task_05.md.

Task05 canonical discovery consumers: native marketplace_search and CLI search now read the one-catalog API; native kind and CLI --kind/two-argument info fail validation. Canonical browse/detail reject obsolete kind queries. Native boot no longer creates a remote skill-acquisition service. CLI pagination/scope/source fields, generated help, native descriptors, official skill and the CLI QA scenario co-ship. Old kind/grouped routes, refresh response and domain/config removal remain task05; evidence is in marketplace-catalog/memory/task_05.md.

Task06 client plugin loading: existing extension CLI/HTTP/UDS and native install surfaces now accept Claude/Codex/Cursor manifest directories through grammar adapters. Package bytes, trust gates, extension identity, manual MCPs and scoped credential ownership are preserved. Unsupported client commands/agents/hooks emit client_component_ignored; none become executable hooks. Nested manifest paths retain the package root for resource delivery, updates and removal. Provenance and the shared ExtensionPayload expose the detected layout; OpenAPI/TS consumers are regenerated. Official skill and extension docs describe the accepted grammar. No config or database shape changes. ET-agent-plugin-marketplace-install is untested for the new behavior; task10 owns the remaining live/visual walks. Focused archive/SQLite/HTTP lifecycle evidence is recorded in marketplace-catalog/memory/task_06.md.

Task05 documentation closeout: the configuration reference now removes active registry/base_url examples and root-feed fallback promises. Marketplace CLI examples use the canonical one-argument info and no kind flag. The migration guide and release notes list removed CLI/API/native acquisition inputs and preserved extension, manual MCP and installed-skill state. Retired QA rows stay skipped with replacement journeys linked; current namespace/search rows remain untested for final09/10. At this checkpoint, the installed-skill MCP authorization field remained active pending the task05 consent decision. The later Task05 embedded MCP retirement entry supersedes this status: the field is retired and archived, and embedded Marketplace-skill MCPs stay disabled. Task02 still owns v3 publisher/site conversion and the obsolete standalone skill-store guide.

Task02 publisher cut: compozy-catalog publish emits only v3 extension/preset feeds and verified package artifacts; the production v2 semantic adapter and copied decoder are deleted. Validation reads only v3 and always compares feed inputs with the packaged manifest. Existing extension entry metadata and artifact bytes are preserved by the canonical publisher suite. The 17 package definitions now compare launch/auth/input/default-scope behavior with an immutable pre-feature fixture, independent of root MCP feeds. No daemon config, native tool, credential, workspace or installed-state shape changes in this slice. Site kind consumers/root assets and the remaining task02 docs/schema obligations are still pending; existing final09/10 QA ownership remains.

Task02 site data and docs cut: the public catalog reads the single v3 extension/preset schema, uses direct /marketplace/<entry_id> routes, and removes Kind unions, root feed imports/outputs and the old standalone skill acquisition guide. Search and sitemap use current entry identities. Installed skill provenance stays documented in the local skills guide; its MCP consent allowlist was unchanged at this checkpoint pending the task05 decision. The later Task05 embedded MCP retirement entry supersedes that status; the hook allowlist remains active. Existing extension install refs/artifacts, bundled resources and bridge setup remain preserved. Official extension authoring guidance already covers inputs/auth/default scope. No additional native tool, workspace, credential or database change in this slice. Site UI integration and its focused checks are in progress; ET-site-marketplace-catalog and MS-marketplace-catalog-live-config retain final09/10 live QA ownership.

Task03 input ownership naming: requires_env bindings and optional expected_digest remain current supported extension behavior. Reader/helper comments now name those mechanisms directly rather than marking them legacy. No public DTO, config, persistence, runtime behavior or QA contract changes in this editorial/refactor slice; focused binder, lifecycle, readiness and SQLite input suites pass. Public scoped-install propagation remains pending in task03/04.

Task03 managed acquisition scope: MarketplaceInstallRequest carries the existing InstallationScope into registry persistence. Actual archive/SQLite install/update cases prove exact attachment retention (including created_at) and package/row cleanup for a nonexistent profile. Empty scope retains existing global/all-profiles installation. No public DTO/native tool, config, credential, generated client or Web change yet; daemon request/binder/status propagation remains pending. This reuses migration00113 attachment authority and creates no new storage shape.

Task03 public install scope: CLI install/preview, HTTP/UDS InstallExtensionRequest and the native
extensions_install schema now forward scope/workspace_id/profile to the existing attachment,
input binder and status owners. Explicit CLI profile overrides COMPOZY_PROFILE; omitted operator
profile preserves all-profile attachment, while agents stay in their trusted workspace/profile.
Input schemas and docs accept only owned vault:extensions references, removing the retired manual
MCP import promise. OpenAPI, Web DTOs, native catalog, CLI help and official skill co-ship. No
config, hook, SDK extension protocol or database shape changes. ET-extension-published-source-installs
is untested for final09/10. Manifest default_scope selection, scoped updates, workspace runtime and
UI remain task03/04 work; this slice proves explicit install selectors, not those remaining outcomes.

Task03 manifest defaults: explicit selectors and trusted agent scope retain priority. Operator installs
without selectors use the manifest servers' common default_scope; mixed defaults and missing workspace
context fail before managed writes. The staged installer accepts the final scope at Commit, avoiding
reacquisition or mutable setters. Local packages use the same resolver; updates retain attachments.
Owning daemon/archive/SQLite tests include default workspace, explicit override, mixed defaults and
failure cleanup. Generated scope descriptions and the existing final09/10 QA inventory co-ship.

Task03 scoped updates: shared HTTP/UDS DTOs, native extensions_update, CLI update and Web mutation
data carry scope/workspace/profile. The binder prepares/commits/rolls back the selected cell; package
and workspace locks remain held through publication/rollback. Existing attachments and other cells
are preserved; package bytes remain shared. Batch selection filters to installed scope/profile, and
named batches now process every distinct name. Cross-workspace agents cannot mutate global/inherited
attachments. No database, config, hooks or extension SDK shape change. Owning generated contracts,
CLI help, official skill and final09/10 QA scenario co-ship. The obsolete --runtime-name and marketplace
--kind examples are removed from the install guide. UI recovery and same-origin reinstall remain open.


Task03 runtime input publication: each profile loads its own durable input cell. Unconfigured
profile projections omit packaged MCP servers without aborting publication for configured profiles.
No input inheritance, credential copying, public DTO, hook, config, database or extension SDK change.
Real daemon CLI installation, HTTP/UDS readiness/settings and restart coverage lives in the existing
extension distribution integration suite. Web readiness keeps the current missing_inputs contract;
install guidance and ET-agent-plugin-marketplace-install cover isolation and restart for final task10.

Task03 update origin and rollback: removed the opportunistic curated trust lookup for direct
registry updates and stopped overwriting their recorded installation origin. HTTP/UDS, CLI and
native update use the shared lifecycle; no DTO, config, hook, extension SDK or database shape change.
Real daemon integration covers retained typed values, inactive/reactivated inputs and publication
failure compensation; the owning vault integration covers second-write install cleanup. The
IT020 storage-failure expectation uses the existing `500` contract, with `422` reserved for input
validation. Site/official skill and ET-extension-published-source-installs carry the final task10
walk. Workspace attachment publication remains task04; this evidence uses the global/default cell.

Task04 explicit workspace-profile startup: the manager now preserves both attachment selectors
when starting an installed profile runtime, using the existing startup/rollback and stop owners.
Real SQLite/subprocess tests cover boot, tool/log access, restart, archived-profile suppression,
and exclusion from global/foreign-workspace reads. No native-tool/HTTP/UDS DTO, hook or config
shape changed. The existing install guide describes the corrected scope behavior; official skill
selectors remain accurate. ET-extension-published-source-installs assigns the daemon walk to
final tasks09/10. All-profile workspace startup and package-wide MCP allocation rollback remain
task04 work; this focused fix does not claim those paths or final Web/QA delivery.

Task04 all-profile workspace runtime: scopedExtensions now owns published workspace and profile
instances while devExtensions retains development-overlay identity. Boot uses active attachments
and defers to persisted overlays; unlink restores published workspace instances through the same
startup owner. Restoration does not start processes while the manager is stopping. Profile grant
metadata is resolved using the actual instance ceiling instead of copying the base runtime's
broader grant. Existing SQLite/subprocess and development lifecycle suites cover startup, restart,
scope exclusion, default overlay restoration and shutdown interaction. Public DTOs, native tools,
hooks, configuration and stored data shapes are unchanged. Install docs and the final09/10 scenario
are updated; named-profile overlay transition concurrency and package-wide MCP rollback remain
task04 integration work before backend/data handoff or final delivery.

Task04 named-profile overlay transitions now retire affected workspace/profile processes within
the existing startup transaction. Activation failure restores their previous definitions;
unlink restoration failure reinstates the development link, base runtime and profile runtimes.
Workspace coordination serializes profile startup/invalidation with the transition. Supervisors
remain bound to their original runtime object, even if a replacement reuses a numeric generation.
Real SQLite/subprocess tests cover both transitions and injected source-activation failures;
existing concurrency, startup rollback, shutdown and generation-fencing suites remain the owners.
Install docs and the final09/10 scenario reflect the behavior. No new public DTO, native tool,
hook/config key, migration or compatibility path. Package-wide MCP allocation rollback is still
task04 work; these runtime results do not claim final UI, QA or CI delivery.

Task04 package allocation compensation: updates, batches and published reinstalls hold exclusive
package access across workspace operations. Instance-only operations retain workspace isolation.
Update rollback removes only newly created allocations for that package across every workspace
and profile after package/input restoration succeeds; existing allocations/overrides and other
packages survive. Install/dev snapshots remain workspace-scoped. This reuses the repository's
weighted semaphore pattern and adds no public surface, migration, hook, config or compatibility
adapter. HTTP/UDS/CLI/native updates share this lifecycle; official skill selectors stay accurate.
The install guide and ET-extension-published-source-installs describe final task10 acceptance.
Focused race evidence lives in marketplace-catalog/memory/task_04.md; full IT016/021 and UI remain open.

Task04 complete migration fixture now starts at v109 with3 extension/17 MCP/1 skill catalog
rows and installed package files, enablement, scoped env/header bindings, token/DCR rows and
vault bytes. The normal stream applies00110–00114; repeated opens preserve exact installed
state and authority. A subsequent HTTP refresh from a custom base path loads20 checked-in v3
entries without classifying unrelated installations. Existing owner/attachment suites retain
their detailed validation and cleanup invariants; the combined fixture reuses credential seeding.
The IT016 generation reference is corrected from1 to0, matching fresh source initialization,
existing store tests and immutable00110; generation fencing is unchanged. No SQL history,
public interface, hook, config, native tool, extension SDK or UI behavior changed in this slice.
Final task10 still owns the live upgrade/operator journey; focused receipts are in task04 memory.

Task04 public MCP addressing now rejects allocated runtime-name aliases even with an explicit
extension owner, at both Settings/auth resolution and native diagnostic lookup. Logical manifest
name plus owner/scope remains the public identity; internal discovered-resource execution is
unchanged. No DTO, generator, database, config or hook change. Existing Web queries already pass
published.name and explicit owner. Install docs, official tools-and-skills reference and final10
scenario co-ship. This removes a remaining alias prohibited by ADR008; full IT021 remains pending.

Task04 OAuth refresh now reuses the exact target's persisted client registration after validating
its definition, resource, issuer, scopes, auth method and secret expiry. The previous refresh path
required a login callback and could register another client; the executor lacked that callback,
so expired extension tools disappeared from discovery. Removed the unused internal redirect option.
No public route/DTO, config key, hook, SDK or database shape changed. HTTP/UDS Settings and discovered
tool execution share the existing owner and scope boundaries. The official skill and install guide
describe refresh without another login; final10 owns the corresponding UI journey. Focused real
daemon/SQLite/encrypted-vault evidence covers manual plus extension registration, exchange, refresh
and logout; full IT021 override/update/detach acceptance and task04 UI remain open.

Task04 GET detail follow-up removes the remaining explicit-owner runtime alias in
`internal/api/core/settings_mcp_collection.go`; the existing HTTP/UDS handler suite retains local,
inherited and sibling-scope checks using logical names. Real daemon IT021 now also verifies
override storage without package writes, owner-less manual edits, manual runtime-name refusal,
override/name preservation through package update, a second same-name extension, and sticky names
after manual removal. No wire/schema/config change; prior documentation already states this
contract. Scoped attachment update/restart/detach acceptance remains open.

Task04 published attachment lifecycle now adds a missing scope on same-origin reinstall without
rewriting unchanged package files. Scoped removal detaches only its selected installation; final
removal uses managed package retirement. Published and development workspace resource snapshots
share the existing runtime projection, so published workspace MCPs receive their own inputs and
allocations. Package locking covers selection, mutations and compensation. Allocation retirement
and the established removal event payload commit together; a failed event restores attachments,
enablement, inputs and allocations. Existing native/HTTP/UDS surfaces use this lifecycle; no new
DTO, route, config, hook, SDK or database shape. Other installations, manual credentials and dev
unlink remain protected. Install docs and the official tools-and-skills reference co-ship; final
QA09/10 owns the installed-management UI walk. Task04 UI implementation remains pending.

Task04 frontend data now exposes a Settings MCP controller and reusable manual/extension editors.
Manual definitions retain existing Settings serialization; extension writes contain only explicit
env/headers/url overrides. Selection keys include owner and exact scope. Authorization polls its
captured definition independently of page selection. Existing adapters and Query invalidation own
HTTP/UDS mutation envelopes; no new backend, wire, config, hook, schema or SDK changes. The Settings
route and Marketplace/Settings presentation remain the next UI slice; final09/10 owns visual QA.
Canonical editor-model, mutation, authorization and Marketplace-hook suites verify these boundaries.

Task04 presentation integrates Server details and Installed authorization with the prepared exact-owner
controllers, restores /settings/mcp and Settings window navigation, and limits extension edits to
overrides. Manual definitions keep their separate editor/removal path. Shared StatusDot gains its
existing semantic success tone for truthful runtime status. No additional native, HTTP/UDS, config,
hook, workspace storage or SDK contract changes. The install guide explains these entry points;
J-mcp-authorize-repair and ET-web-marketplace-mcp-authorize-installed now target the current routes
and remain untested for final09/10. UI integration and focused checks are still in progress.

Task07 package-cache foundation adds verified content-addressed files under a configured cache root.
Put stages and syncs approved bytes before publication, Open re-verifies the held file, and Sweep
preserves supplied projection/installation pins while evicting older unreferenced blobs. Existing
fileutil owns no-follow and bound publication/removal. Real-file race tests cover interruption,
corruption repair, cancellation, capacity, pins, symlink refusal and readers surviving eviction.
This is an internal foundation: source fetching/projection/install composition is still pending,
so there is no new callable CLI/HTTP/UDS/native surface, config, hook, SDK, Web or installed-state
change yet. Task07's existing audit will expand when those consumers are wired.

### Task07 source document reader foundation

The plugin-source owner now normalizes acquisition refs, decodes bounded marketplace documents with
per-plugin diagnostics, and reads root or `.claude-plugin/marketplace.json` through held directory
handles without following symlinks. Existing registry installers and extension behavior are unchanged.
The public-source/config/daemon composition remains assigned to task07/08; no new routes or native
tools are exposed by this checkpoint. Canonical coverage: pluginsource reader_test.go (origin identity,
metadata/source decoding, local document selection, size/cancellation and confinement).

### Task07 GitHub acquisition foundation

Plugin marketplace documents resolve to an exact GitHub commit and are fetched through the existing
registry client without ambient token/cookie use, within one 10-second deadline and existing bounded
retries. New repository read/archive methods retain the existing network and response ownership rules.
The shared archive finalizer also removes its spool when response cleanup fails. Public routes, config
composition, install/cache consumers and final QA remain owned by task07/08/09/10. Owning validation is
the registry/github and pluginsource race suites; this is not a final gate/QA delivery claim.

### Task07 owned Git checkout acquisition

The registry Git client now offers an owned checkout with verified commit identity and reuses its clone
acquisition for existing archive downloads. Failed acquisition releases the complete temporary tree;
archive callers still receive the same gzip format. New source composition will use the checkout parent
under the marketplace home. Focused registry/gitsrc race tests, lint and Windows compilation pass;
live Git source acquisition and public source integration remain pending in task07/final QA.

### Task07 canonical package tar

Raw package tar and existing gzip archives now share the fileutil serializer. All raw tar limit consumers
use the renamed internal errors directly. The serializer refuses a file replaced by a symlink before
reading, preventing external bytes from entering a marketplace package. Existing gzip bytes, limits
and cancellation remain covered by the same suite; no extension package/state migration is introduced.
Scoped race checks pass. Final gate still owns baseline formatting/coverage gaps and integrated QA.

### Task07 raw package installation boundary

Explicit application/x-tar downloads now pass through the same registry digest, extraction safety,
manifest/content validation and publication pipeline as gzip downloads. Canonical raw package bytes
are verified before extraction and retained as the archive digest in the result. Existing gzip behavior
is covered by unchanged suites. Public Marketplace install/source composition is still pending task07/08.

### Task07 checkout-to-cache capture

CapturePackage now creates the canonical digest-addressed package from a confined checkout path,
excludes Git metadata and publishes through the verified cache before releasing source bytes. The
acquisition suite proves cache reuse after checkout deletion and refuses traversal/symlink/over-budget
inputs. The source resolver, projection and install-lifecycle consumers remain pending task07.

### Task07 pinned GitHub repository snapshots

The existing registry extractor is now available to source acquisition without requiring an extension
manifest at the root of an entire marketplace repository. GitHubSource acquires one exact revision,
verifies its marketplace document matches the fetched document and exposes an owned snapshot for
relative package capture. Failure and Close remove the snapshot and download spool. Extraction safety
remains owned by registry; no second extractor or internal compatibility alias was introduced.

### Task07 Git and folder source snapshots

Git and folder sources now implement document acquisition and snapshot ownership alongside GitHub.
Git snapshots require a full resolved commit and use the existing isolated Git client; a focused test
executes real local Git operations with only the remote transport replaced by a fixture. Folder Close
preserves operator files and rejects document changes before package capture. Source/config aggregation
and lifecycle consumers remain pending; final live QA and delivery gates are not claimed.

Task05 embedded MCP retirement (2026-09-13): retired skills.allowed_marketplace_mcp from config/Settings/HTTP/UDS/CLI/Web and archived its values atomically through the existing config owner. SourceMarketplace skill MCP declarations are disabled; local skills, content/provenance, manual MCPs/credentials, extension execution and hook policy retain their owners. Wire artifacts, official skill, config guides, migration/release notes and ET013 co-ship. Real-file registry integration, config archival/reload, HTTP/UDS rejection, CLI validation and focused Web tests pass; memory/task_05.md owns exact receipts. Prior pending-consent statements are superseded. Final user journeys remain09/10.

Task07 source runtime composition: the daemon composes configured plugin readers, the existing manifest inspector, the package resolver and a home-owned cache. Feed refresh flights read v3 presets with bounded HTTP/file acquisition; operator enablement follows normalized origin, and an atomic derived preset cache restores registrations offline. Full refresh includes newly discovered enabled presets. Hot configuration retains the existing source generation fences and cancellation owner. Public wire contracts are unchanged in this checkpoint; plugin install routing, cache sweep/availability and source-management surfaces remain task07/08. The configuration guide documents this discovery behavior; final QA remains09/10.

### Marketplace plugin acquisition continuation (task07)

`POST /api/extensions` and install preview accept `source: marketplace` with the listed digest;
HTTP/UDS and native extensions_install use the same daemon/lifecycle owner. Update resolves persisted
origin and keeps publication, input, attachment, credential, and rollback behavior. Missing cache plus
unreachable source returns503 source_unreachable. Web action data selects the source union from origin;
OpenAPI, TS, native catalog, site sources docs and official extension skill co-ship. CLI source-index
selection and source-management UI remain task08-owned. No hook or manual MCP policy changes.
QA: ET-agent-plugin-marketplace-install and CH-agent-plugin-marketplace remain pending task10 live walk.

Task07 cache completion: service-owned refresh flights and daemon plugin install/preview/update/inspection
hold packages through publication. Sweep reads current projection and installed-provenance pins after
those holds release, retains every pin and reports capacity pressure. Browse/detail expose local blob
availability; blocked detail retains projected contents without reacquiring. No public DTO, schema or
manual-state change. Refresh, eviction, budget and acquisition mismatch logs use existing structured
logging, separate from canonical notifier events. Task10 owns the remaining live/visual QA.


Marketplace task08 sources: experimental HTTP/UDS source list/add/preview/toggle/remove/refresh,
CLI sources commands and aggregate failure semantics, read-only compozy__marketplace_sources.
Runtime owns global comment-preserving config edits, source-name retention, loader preview and
refresh diagnostics. Native tools and Web share source payload conversion; generated OpenAPI/TS
and tool catalog co-ship. Web adapters/query envelopes preserve global order and detailed errors.
Existing extension acquisition refs keep priority; explicit marketplace: CLI refs select plugins.
No hook/bridge SDK change; manual MCPs, installed skills, extension lifecycle/credentials/rollback
remain protected. Source UI and final QA scenarios are task08/task10; API docs use the existing
OpenAPI-generated marketplace page rather than introducing a duplicate hand-maintained API reference.

Task08 catalog Settings co-ships GET/PATCH /api/settings/marketplace, global catalog URL/TTL/timeout
editing through the existing Settings apply owner, and generated lifecycle metadata for live source
changes. CLI config set marketplace.plugin_sources.<name>.enabled uses the source mutation owner.
Source API errors retain suggested_name/retained_by/checked in CLI structured output; invalid source
input exits 2. Catalog section view data preserves authoritative source order/counts independently
of query matches. QA acquisition/parity journeys and source scenarios now follow the hard cut;
existing extension instance removal keeps baseline exclusive-secret cleanup and shared-state safety.

Marketplace final-review continuation: optional ExtensionPayload description and installation_profile
co-ship through the existing OpenAPI/TypeScript/SDK generator. Description comes from the manifest;
installation_profile identifies a persisted attachment, separately from the viewing profile. No SQLite
shape changes. Missing optional attachment metadata does not invalidate a runtime snapshot; genuine
read failures still propagate. Native install/update use the existing workspace selector and accept
the approved Marketplace source, mapping to HTTP/UDS workspace_id through the common authorization
boundary. Installed detail links preserve the captured profile/workspace; global and workspace axes
remain independent. Web, installed docs and the owning QA scenario include authorization feedback,
partial batch completion, local descriptions and scope. Existing extension lifecycle, manual MCPs,
installed skill provenance, hooks and credential ownership remain under their original owners.

Session-context implementation closure (2026-09-12): the turn query hook shares `use-session-context.ts`; confirmed receipt data uses AgentEvent's existing optional payload with isolated clones, preserving the public `delivery` JSON field. Lazy CreateAccepted retains startup ownership. All selected scenario verdicts pass after runtime and opaque-ID repairs; the feature QA report is `docs/qa/reports/2026-09-12-session-context.md`. Cache migration 00110 and owning generated contracts co-ship. Native tool IDs, hook contracts and workspace isolation remain as audited above.

Marketplace integration with main64b36b4cf: main's cache-token migration00110 is retained byte-for-byte. The unpublished Marketplace SQL payloads move unchanged from110–117 to111–118; projection/input/auth/attachment/discriminator ownership does not change. Prior receipts retain their historical numbering. Canonical v109 upgrade and main token-cache preservation tests both run against the combined stream; the generator owns the combined Atlas checksum and SQLC/OpenAPI outputs. No runtime fallback, data reset or edited migration from main is introduced.

Marketplace final repair addendum: installed-name detail uses optional exact-origin catalog enrichment for current trust/installability/update metadata while preserving installed contents and offline access. MCP override commit/rollback invalidates volatile observed readiness. Shared HTTP/UDS handlers and Settings publication own these behaviors; no new config, migration, native tool ID, or credential-retention rule is introduced. QA owners are passive update discovery and the existing real MCP publisher integration.

Catalog curation follow-up: the maintainer retired Repository Orientation from the official feed.
The canonical sources and generated v3 feed now contain19entries. Web, site, CLI, HTTP/UDS and
native Marketplace discovery consume that feed; their contracts, config, hooks, official skill and
workspace state are unchanged. Previously published package bytes and installed copies are retained.
The existing publisher/runtime-source suite verifies absence and preservation of the remaining entries.

Marketplace PR636 single-review remediation: the existing source synchronizer now applies the
retired Marketplace skill-MCP rule during dynamic resource publication and aborts boot on failed
reconciliation. Installed skill bytes/provenance, local declarations, manual MCPs and extension
resources remain protected. Auth config validates client-secret ownership before resolution;
manual user references from released config retain access to their own persisted keys, while
shared and environment references remain explicit. Profile lifecycle enumerates manual and
extension-owned profile/workspace-profile secret prefixes, re-encrypts renamed ciphertext with
its new identity in the existing transaction, and includes those credentials in deletion previews.
No public CLI/HTTP/UDS/native IDs or database shape changes accompany these corrections. Web
Settings and profile lifecycle actions consume the same runtime boundaries; official skill commands
remain current. Plugin inspection/classification/loading share one securely read manifest and
resolve package data paths before loading components. Owning runtime, Vault/SQLite lifecycle and
manifest suites supply focused evidence; affected scenario additions require final live re-walks
and heavy gates remain assigned to current-head GitHub CI.

Marketplace source mutations now serialize canonical overlay writes and restore rejected changes,
including original comments or prior file absence. Catalog source membership and its in-memory
publication commit under the same lifecycle lock. Failed add/disable/remove operations retain the
prior catalog snapshot; external overlay replacements are preserved with an explicit retry error.
Workspace extension hooks now use the effective profile/workspace projection. Persisted hook
bindings retain profile identity and workspace shadowing, including development packages without
hooks; reconstructed dispatch preserves these boundaries. Inventory hook IDs include the same
profile/workspace identity as published bindings. Existing CLI/HTTP/UDS/native operations and Web
controls consume these fixes; no new public route, config key or SQLite schema change is needed.
Focused real SQLite/reconciliation/subprocess suites passed. The existing source-management and
plugin-development QA scenarios include the affected public re-walks; current-head CI remains pending.

Marketplace review CI follow-up: acquisition and loading now share Agent Plugins unsupported-schema
error identity, preserving the existing HTTP/UDS 422 diagnostic and retained installed package on
rejection. Hook projection scope is cloned as one optional placement value; JSON/YAML keep the
top-level profile_id contract and the daemon codec retains private workspace shadowing. Owning
normalization, serialization, real SQLite publication and archive lifecycle suites verify these
boundaries. Empty optional Marketplace fixture fields follow the generated wire contract. Windows
config privacy is verified by the ACL suite; Unix alone asserts POSIX permission bits. No additional
public surface, config key, credential rule, migration, official skill or Web/docs contract changes.

PR636 public development re-walk follow-up: CLI dev/reload/watch now prepares supported Agent Plugins
source generations before calling the existing shared HTTP/UDS development handlers. The daemon
uses the same secure manifest selector for standard, Claude, Codex and Cursor layouts, retaining
authored identity and workspace containment. Native build behavior, tool IDs, DTOs, config,
credentials and storage shapes stay unchanged. Official skill commands remain valid without edits;
the site documents portable development preparation. The owning real SQLite lifecycle suite now
covers all four layouts through preparation and activation, including last-good reload behavior.

PR636 profile credential public re-walk follow-up: profile rename now updates exact owned Vault
references inside personal-profile config.toml and mcp.json files through the existing recoverable
folder finalizer. Preview counts include those configured occurrences; parser ranges preserve
comments, formatting, JSON keys, shared refs and other owners. Selected repository-folder renames
apply the same ref rewrite and report any remaining file repair in their existing per-folder result.
No credential value enters the journal, no schema/DTO/tool/config key changes, and Web/CLI/UDS keep
the shared profile lifecycle owner. Site profile lifecycle documentation owns the operator explanation;
official skill commands remain valid. Real lifecycle coverage loads both moved config formats,
resolves the resulting secret through Vault and repeats finalization without changing bytes.

PR636 Settings credential-replacement follow-up: reject foreign or daemon-managed OAuth client-secret
refs before definition commit and exclusive-secret cleanup. Reuse the existing Vault target policy;
HTTP/UDS/CLI now receive the existing validation error for the invalid mutation. Own/shared/env and
released user refs retain their established rules. No config/schema/DTO/tool IDs change. The canonical
Settings secret-write suite and public MCP authorization scenario own preservation evidence.

MCP repair follow-up: Settings determines pre-mutation existence from owner-qualified definitions without requiring successful auth/runtime probes. CLI/HTTP/UDS configuration application retains its existing validation, ownership and lifecycle contracts. Web can repair an invalid configured credential through the same Settings PUT; no schema, native tool ID, hook, official skill or workspace isolation change. The existing config-apply suite owns repair/addition classification and persisted definition evidence.

## Windows Gateway CLI private files — issue 532

- **Native tools and public surfaces:** existing CLI startup, `connect` commands and local
  Gateway credentials reuse platform-validated private files. No native tool ID, HTTP/UDS
  route, flag or wire schema changes. The fileutil opt-in private-file boundary retains
  exact Unix `0600`; Windows reads owner/DACL from the held no-follow handle. Other
  fileutil callers retain their existing permission policy.
- **Extensibility, hooks and config:** no extension, hook, bridge, provider or config-key
  changes. The existing OS keyring and encrypted credential format remain authoritative.
- **Workspace data isolation and compatibility:** no SQLite or persisted shape changes.
  Existing private files reopen; journals recover using existing commit/rollback logic.
  Scope-derived keyring identity stays unchanged. Unsafe Windows ACLs fail explicitly
  before reading/writing private bytes; no automatic ACL rewrite or state deletion.
- **Official skill:** `skills/compozy/` commands and runtime contracts remain valid; no new
  agent operation or skill procedure is introduced.
- **Web/Docs and QA:** no Web component or behavior changes. Gateway security documentation
  explains the platform policy. `RT-gateway-remote-cli-profile` owns the Windows persistence
  slice, verified by native CLI processes and real OS keyring use in GitHub CI. Existing
  owning transaction/credential suites and Windows fileutil ACL tests cover rejection and
  recovery. All gates/builds/tests/QA are assigned to current-head GitHub CI by user override.

First-review remediation preserves an unsafe existing destination's bytes and ACL instead of
replacing it. The same no-follow directory handle validates the destination before publication;
Windows ACL tests assert both the specific rejection and unchanged security descriptor. Unix
lock/journal tests assert the precise private-permission error. Newly created files retain their
explicit mode initialization under restrictive Unix umasks; existing files are never chmod-repaired.

Windows CI preparation: pin SQL and Atlas checksum files to LF in `.gitattributes` so
Git cannot convert embedded migration bytes on checkout. No released migration or checksum
bytes change, and runtime integrity validation remains enabled. The first native Windows
fileutil ACL suite passed; the CLI seed refused converted migration bytes before test execution.

Windows CI also exposed missing `FILE_READ_ATTRIBUTES` on the existing removal handle.
Gateway credential deletion and journal recovery now request that right for their existing
handle-based type/reparse checks. The canonical atomic-removal suite runs in the Windows
job alongside Gateway recovery; no safety check or test assertion was removed.

## Issue 673 — ACP provider full-access preference

- **Native tools / CLI / HTTP / UDS:** existing session and agent reads report the effective ACP
  mode; no tool IDs, routes, commands, or DTO fields change. A narrower session permission policy
  drops an inherited unrestricted agent mode at start and on later runtime changes; explicit
  unrestricted session selections are refused. A preference newly enabled after session start is
  also refused on a restricted prompt runtime change.
- **Extensibility / hooks / config:** `[permissions] provider_full_access` is an opt-in, operator-owned
  TOML setting. It maps to Codex `agent-full-access` and Claude Code `bypassPermissions` for new
  `approve-all` sessions. Existing defaults remain when it is false; unsupported ACP providers
  report a resolution error. Agent-authored ACP mode options retain precedence. Explicit `yolo`
  and `auto` modes are also treated as unrestricted at the session permission boundary.
- **Workspace isolation / compatibility:** no stored schema or migration changes. CompozyOS native
  tool policy, workspace sandbox selection, and operating-system permissions still apply. The
  provider-native command sandbox may be removed inside those boundaries when explicitly selected.
- **Web / docs / official skill:** no new Web control or skill command. Site configuration and
  permissions guides document the option and its limits. `RT-provider-full-access-mode` owns the
  live Codex/Claude walk; the previous Codex-only probe and focused Go suites are recorded there.

## Reported issue batch — 657, 659, 663, 665, 666, 675, 676, 677, 678, 679, 682

Owning delivery: branch `fix/reported-issues-657-682`; real-runtime evidence and remaining
verification are tracked in `docs/qa/reports/2026-09-29-reported-issues.md`.

PR #686 review remediation keeps the same audit owner. Managed delivery rechecks selected index
state before staging and records its own staged proof for recovery; deterministic safety refusals
are terminal, completion releases the session fence first, and unreadable journals remain intact
while unmatched interrupted SQL receipts fail without effects. The daemon preserves running
receipts on failed directory inventory and retries inventory failures or temporary SQLite
BUSY/LOCKED under its owned shutdown lifecycle; successful recovery stops polling and concurrent active controls remain intact.
Live applying installer identity survives its deadline, Linux discovery compares all versioned installations and retains an
unversioned fallback, and MIME registration has bounded subprocess cleanup. ACP forced stop
verifies PID/start identity; SQLite callbacks reject active same-database reentry and preserve
request diagnostic context. Facets documents its existing parent/root/resumable filters through
generated OpenAPI consumers. Web blocked-query retries retain only blocked hashes, unused facets
are disabled, search is debounced with scope-fenced previous rows and disabled stale actions, and
request client IDs reuse the existing browser cryptography boundary. No config, hooks, capability
IDs, SQLite shape, or permission boundary changes; journal additions preserve existing records.
The official worktree reference and generated CLI example require the actual reviewed scope.
The existing packaged Electron shell suite owns a real sixty-minute catalog request-rate
regression using public session/Loop activity and unchanged fixed budgets. Desktop CI retains
its current-head/runtime/client receipt on success or failure; no local runtime is launched.
CI exposed duplicate shell terminal-facet polling alongside live stream reconciliation. The
shell now uses the shared live clock for healthy reads, retains visible disconnected polling
fallback and retries failed facets reads at the existing thirty-second error interval even with a
live stream. Native public surfaces, configuration, persisted state and official skill contracts
remain unchanged.

- **Native tools / CLI / HTTP / UDS / SDK:** worktree exit plans and commits add explicit reviewed
  paths and a content fingerprint; managed delivery uses validated session identity, a durable
  intent, checkout fencing and exact draft-PR reconciliation. Existing whole-worktree commit
  behavior remains available. Session catalog reads add an opt-in count-free continuation while
  ordinary clients retain exact totals. A separate scoped metadata facets read owns exact chip
  and group counts, including pending terminal approvals. Web catalog rows use bounded count-free
  pages with explicit continuation, Unicode-aware title/agent search and preserved navigator ordering.
  Dock selection uses the existing creation order through a bounded one-row page; an explicit click
  resolves current server truth before selecting create or open and allows retry after a read failure.
  Deliberate new window opens use the serialized command's current revision; ordinary semantic
  identity lookups retain their revision fence against duplicate windows.
  The separate unread-notification title channel retains bounded background polling and error
  backoff while automatic session catalog and summary reads pause when hidden.
  Workspace detail reads explicitly request the operator's
  aggregate profile view; server workspace and agent authorization remain authoritative.
  Aborted presence requests retain the shared cancellation status. Forge contracts add optional
  branch, base, commit and draft evidence; old extensions remain usable for ordinary exit actions,
  while managed delivery requires proven identity. Generators co-ship the affected consumers.
- **Extensibility / hooks / config:** ACP control stages have bounded deadlines and session/new
  gets a bounded fresh-process retry after verified cleanup. Prompt and clarification lifetimes
  retain their existing owners. Task authoring emits escaped YAML title scalars, keeps H1 aligned,
  and validates through the existing importer. An unhandled permanent invalid-input action failure
  closes the Loop before automatic generation succession; authored correction routes and allow-fail
  retain their existing precedence. Skill and native-tool failures carry safe phase
  diagnostics that distinguish resource/registry failure from SQLite event-write contention.
  No credential, authored resource content or raw claim token is added to diagnostics.
  Local SQLite writer admission waits with context cancellation before reserving a pooled connection
  and before the unchanged external-BUSY retry budget. Durable delivery terminal receipts use the
  same owner; internal waiting cannot exhaust SQLite attempts before acquiring admission.
- **Workspace data isolation / compatibility:** session and worktree identities remain scoped to
  their registered owner; a delivery fence applies only to the exact checkout. Existing SQLite,
  config and session history are preserved. Appended catalog indexes upgrade the global database
  (migrations 00123 and 00124) without changing or deleting rows. Migration 00125 widens the
  persisted worktree exit-operation action constraint to include managed `deliver`, preserving
  existing operations and the active-operation uniqueness rule. Recovered child adoption requires the exact parent, workspace,
  node receipt and current authored child inputs, including the original optional typed
  `inputs.reviewed_worktree` JSON-string proof using the existing DSL input type.
  The registered worktree owner independently verifies its
  scoped HEAD, branch, content and index fingerprint; absent or changed proof still reruns.
  Expired updater operations settle through the journal owner; live executor and irreversible handoff protections
  remain. Linux installation detection follows the executable/running authenticated shell;
  desktop scheme registration launches that executable directly with URL arguments.
- **Web / docs / official skill:** catalog event wakes coalesce, preserve dirty work after in-flight
  reads, back off errors and defer hidden-window refetches. Global-null scope enables the canonical
  all-workspaces read while an unresolved project stays disabled. Requests carry a bounded client ID in
  daemon logs. Session navigation retains abort cleanup and authoritative scoped lookup. Official
  worktree guidance and affected site/CLI references explain selective scope and managed delivery;
  authoring references agree with the strict importer. Existing scenarios gain the changed public
  acceptance walks. Test, live-runtime, platform and current-head CI receipts remain separate
  claims in the owning report and PR; implementation alone is not delivery evidence.

## Issue 689 — Loop reconciliation and result persistence under SQLite contention

- **Native tools / CLI / HTTP / UDS:** existing Loop and task reads converge after an action
  returns even when SQLite temporarily rejects terminal writes. The daemon retries persistence
  while retaining the result; a transient SQLite persistence error does not become a transport failure that
  re-executes a side-effecting tool. Tool event summaries and action lease heartbeats use the
  same cancellable persistence retry boundary. No command, route, DTO or tool ID changes.
  Pending terminal writes reserve their existing ownership in the task service and retain the
  original command timestamp; automatic lease recovery excludes those reservations. Explicit
  ownership changes still invalidate the claim token. Shutdown allows a bounded settlement grace.
- **Extensibility / hooks / configuration:** extension tool invocation and post-call hooks run
  once. Only the subsequent event/result write is retried. Automation claim failures use
  capped exponential backoff, preserving the original fire identity. Existing scheduler clocks
  and daemon lifetimes own waiting and shutdown; no new operator setting is needed.
- **Workspace data / compatibility:** global migration 00126 adds reconciliation indexes
  without changing rows. Candidate discovery is read-only and starts from live execution records
  or unsettled tasks. Each orphan repair rechecks terminal status under its own transaction.
  Provenance repair runs after readiness until successful, with conditional per-record writes
  that preserve concurrent metadata edits and select the latest coordinator per task, breaking
  timestamp ties by run ID. Empty heartbeat retention avoids writer admission.
  Existing profile/workspace filters, claim tokens and ownership fences remain authoritative.
- **Web / docs / official skill:** existing Web views receive the corrected persisted task/Loop
  state; no Web component or public skill syntax changes. The official `skills/compozy/` commands
  remain valid. Acceptance and verification are recorded in
  `docs/qa/reports/2026-10-03-issue-689-sqlite-contention.md` and the affected terminal-settlement
  and automation scenarios. Related issue 678 is addressed only for shared contention paths.

## Native task filter error classification (2026-10-02 QA)

- Native `compozy__task_list` now preserves task-domain validation as `tool_invalid_input` / `schema_invalid` through its existing error mapper. HTTP/UDS native invocation and the CLI share this dispatch.
- Extensibility, hooks, configuration and workspace isolation are unchanged; no persisted shape or migration. The accepted task filters and official skill contract remain unchanged.
- Web task catalogs are unaffected. QA owner: `ET-native-workspace-scope-isolation`, bug `BUG-20261002-native-task-filter-error`; the dated report retains the hosted observation and fresh public replay.

## Native hook dispatch repair (2026-10-02 QA)

- Native tools now execute configured typed pre-call, post-call and post-error hooks at registry dispatch. Public CLI/HTTP/UDS invocation and hosted MCP share the repair; configuration keys and DTOs stay unchanged.
- Hook input patches retain the canonical workspace and immutable tool metadata, then pass the existing schema, workspace policy and approval gates. ACP observations do not repeat native lifecycle dispatch.
- Active session hook runs and lifecycle events use the existing session-owned recorder. No storage migration, workspace data movement, new worker or process owner is introduced.
- The internal HookRunner contract and its consumers change together. External MCP/extension registry backends retain their existing behavior; this adapter handles native tools only.
- Official skill impact is documented in `skills/compozy/references/native-tools.md`. Web hook/session views consume the same existing audit records. QA owner: `ET-native-workspace-scope-isolation`; bug `BUG-20261002-native-hook-dispatch-missing` and the dated report retain replay evidence.

## Settings search focus repair (2026-10-04 QA)

- Web Settings connects its advertised `/` search shortcut to the existing document keyboard
  boundary, scoped to the active visible Settings window. Fields, dialogs, modified chords and
  already-handled keys retain ownership. Shared listing-search consumers are adjacent canaries.
- Native tools, HTTP/UDS/CLI contracts, hooks, extensibility, config and workspace data isolation
  are unchanged. No persisted shape or migration; the official `skills/compozy/` contract is
  unaffected. Product wording and shortcut labels remain unchanged.
- QA/docs owner: `MS-web-settings-takeover-redesign`, bug
  `BUG-20261004-settings-search-shortcut-inactive`, and the 2026-10-02 untested report. Owning
  navigation-suite regression proof and fresh Chrome replay are required before verification.

## Help-tip dismissal preserves editor drafts (2026-10-04 QA)

- Shared Web UI Tooltip/Dialog composition gives an open descendant tooltip the first Escape,
  including hover while a sibling field retains focus. The nearest dialog defers its dismissal
  through Base UI's public event API; the tooltip keeps its existing dismissal lifecycle.
  Peer dialogs remain independent, and a subsequent Escape can close the enclosing dialog.
- HelpTip opts out of the trigger's click-to-close policy because its existing click handler opens
  explanatory content for touch. Touch activation retains the open lifetime across compatibility
  mouseleave events; outside press, blur and Escape still dismiss it. Generic Tooltip triggers
  retain their current click policy.
- Native tools, HTTP/UDS/CLI contracts, hooks, extensibility, config and workspace data isolation
  are unchanged. No persisted shape, migration or official `skills/compozy/` change is needed.
- QA/docs owners: `MS-web-modal-help-tips` and `MS-web-entity-modal-shell`, bug
  `BUG-20261004-help-tip-discards-draft` and `BUG-20261004-help-tip-vanishes-on-tap`.
  The existing shared HelpTip suite owns stable pointer activation, draft/focus and
  dialog-isolation coverage; real task and adjacent editor replays verify the production bundle.

## Compact Settings choice layout (2026-10-04 QA)

- SettingsChoiceGroup reuses the owning Settings window container breakpoint for its columns,
  so permission cards stack with compact navigation. No new token, primitive or selection state.
- Native tools, CLI/HTTP/UDS, hooks, extensions, config keys, workspace/profile data and official
  `skills/compozy/` contracts are unaffected. No persisted shape or migration.
- QA/docs owner: MS-web-settings-takeover-redesign and
  BUG-20261004-settings-choices-ignore-window. Matched reference/browser captures own reflow
  evidence; the existing choice-group suite owns keyboard and RTL selection.

## General idle-timeout save and display (2026-10-04 QA)

- Web General Settings parses canonical compound Go durations through the existing Settings
  duration helper. A successful save releases only its acknowledged draft after the mutation's
  canonical refetch; newer or differently scoped drafts survive. The save bar can settle cleanly
  without erasing a pending user edit, and reload reflects the persisted cutoff.
- Native tools, CLI/HTTP/UDS DTOs, hook and extension contracts, config keys, restart behavior and
  timeout enforcement are unchanged. User state, workspace/profile data and official
  skills/compozy/ contracts need no migration or update.
- QA/docs owners are MS-web-settings-takeover-redesign and adjacent MS-025, with
  BUG-20261004-settings-idle-timeout-display. The existing General route suite owns display;
  the existing General page-model suite owns save reconciliation and pending-edit preservation.
  A fresh real-browser save/reload/restore walk confirms the production bundle.

## Settings launcher availability (2026-10-04 QA)

- The shared Web rail foot reads settings.general through the existing palette projection,
  so floating and compact Settings controls reflect the same availability as their dispatch.
  Local theme changes remain independent. No timer, command bypass or second readiness owner.
- Native tools, CLI/HTTP/UDS contracts, hooks, extensibility, config, workspace/profile data,
  persisted layouts and official skills/compozy/ behavior are unchanged; no migration is needed.
- QA/docs owner: MS-web-settings-takeover-redesign and
  BUG-20261004-settings-startup-false-offline. The canonical os-dock component suite owns
  missing/unavailable/available command transitions; fresh Chrome entry verifies the actual
  startup ordering and neighboring theme/launcher behavior.


## Settings explicit saves without a connection (2026-10-04 QA)

- Web Settings form mutations attempt transport immediately and settle through the existing error
  path, preserving draft/discard/retry control. General, Memory, Automation, Diagnostics,
  Extensions, Roles, Persona, Skills and Notifications share this explicit-save policy.
  Attention retains its gesture-owned request. Global QueryClient and other domain mutation
  admission/retry policies remain unchanged; there is no new queue, timer or retry layer.
  Rejected fetches at these Settings write adapters become the existing SettingsApiError with
  status zero, actionable save guidance and the original error as cause. Cancellation and HTTP
  validation diagnostics retain their identity; no shared API-client behavior changes.
- Native tools, CLI/HTTP/UDS DTOs, hooks, extensibility and config keys are unchanged. Public
  writes retain their validation and scope ownership. No stored config/schema migration,
  workspace/profile data movement or official skills/compozy/ change is needed.
- QA/docs owner: MS-web-settings-takeover-redesign and BUG-20261004-settings-offline-save-stuck.
  The existing Settings mutation suite owns offline settlement and explicit retry after
  reconnect; fresh browser error/discard/retry/reload walks verify draft recovery and restoration.

## Relative CLI workspace registration (2026-10-04 QA)

- The workspace add positional root is resolved against the invoking CLI directory before the
  existing create request. Absolute roots retain their behavior. The daemon still owns symlink
  canonicalization, home refusal and registration; the CLI adds no lookup or home-policy copy.
- Native tools, HTTP/UDS DTOs and validation, Web registration, hooks, extensibility and config
  are unchanged. No migration, workspace/profile data movement or permission change is needed.
  The official skills/compozy/ native registration contract is unchanged; site resolver guidance
  documents the CLI-relative versus transport-absolute input boundary.
- QA owner: RT-home-workspace-not-registrable and BUG-20261004-workspace-add-relative-path.
  TestWorkspaceAddBuildsRequest owns relative-root conversion and directory-resolution failure;
  fresh CLI/HTTP/UDS replay owns canonical refusal, project persistence and baseline restoration.


## Task execution switch labels (2026-10-04 QA)

- Web task creation connects the existing execution switches to their visible labels. The shared
  Switch and FieldTitle primitives, copy, draft values and task execution policy are unchanged.
- Native tools, CLI/HTTP/UDS routes/DTOs, hooks, extensibility, config, profile/workspace isolation,
  persisted data and official skills/compozy/ behavior are unchanged; no migration is needed.
- QA owner: MS-global-scope-no-workspace-work and BUG-20261004-task-execution-switches-unnamed.
  The existing task editor component suite owns name-based operation; a fresh browser walk
  confirms the real accessible tree, saved Global draft and adjacent project draft behavior.


## Global agent fleet visibility (2026-10-04 QA)

- HTTP/UDS GET /api/agents/catalog accepts an omitted workspace for the existing Global
  definition population. Profile resolution remains authoritative; server filters, facets,
  ordering and cursor pagination apply before the page cut. Existing workspace requests and
  cursor fingerprints retain their shape and behavior; Web cache keys retain profile identity.
- Session Manager and the durable aggregate accept Global breadth with an explicit profile
  read scope. Exact metrics count visible sessions by agent name across projects and no-workspace
  work, preserving live overlays and existing internal/archive exclusions. No schema migration.
- Web catalog, route preload, detail metrics and session rows accept resolved Global scope;
  unresolved scope still gates reads. No fabricated workspace, client filtering or totals.
  Existing project-required session mutations retain their admission rules.
- Native tools, CLI agent list/info, extension contracts, hooks and config keys are unchanged.
  No workspace/profile data movement occurs. The owning OpenAPI source, generated types, site
  agent guide and official skills/compozy/references/agent-definitions.md co-ship the additive read.
- QA owner: MS-global-scope-no-workspace-work and BUG-20261004-global-agents-project-gate.
  Existing core fleet, Session Manager, SQLite aggregate and route preload suites own the
  changed boundaries. The global-agents-* receipts record the completed Global/project replay,
  independent HTTP/UDS counts and cursor boundaries, reload persistence and owned-object cleanup.

## QA delivery diagnostics (2026-10-04)

- The bounded SWR 2.5.1 package patch preserves deferred focus/reconnect revalidation while
  preventing a DOM event from becoming the timer delay. It changes no API, native tool,
  extension/hook/config contract or persisted workspace data. Its removal condition and owning
  visibility/focus checks are recorded in the 2026-10-02 untested QA report.
- Component fixtures preserve their behavioral checks while settling asynchronous work and
  unmounting before shared state resets. UIProvider still verifies both OS motion preferences.
  Official skill and site content need no additional behavior change for these test repairs.
- Dependency patch edits now select all JS workspace validation in make gate, with full PR CI
  still required. The existing gate integration classification suite owns this delivery contract.
- The React SDK test configuration uses an explicit .mjs extension and both invocation
  paths follow it. This fixes the ESM-as-CommonJS loader diagnostic while preserving the
  SDK's published dual-module package. The existing renderer suite validates the runner.


## Loop catalog controls (2026-10-05 QA)

- Web keeps the existing search toolbar mounted during catalog loading and exposes the
  existing kind/category filters through shared filter chips. One route update commits
  all selected facets; the daemon continues to filter, sort, count and page results.
- Search uses the existing debounced draft/commit owner so pending navigation cannot
  replace new keystrokes. A selected category remains labeled when other facets exclude
  it; the UI does not fabricate a count for that selection.
- Native tools, CLI/HTTP/UDS DTOs, hooks, extensions and config are unchanged. Query
  workspace/profile isolation and persisted data retain their existing owners. No
  migration or official skills/compozy/ behavior change is needed.
- QA/docs owner: LP-001, BUG-20261005-loop-catalog-filters-missing and
  BUG-20261005-loop-search-loses-focus. The existing chip projection suite owns filter
  values; the existing Loop E2E suite owns focus across real daemon responses. The
  persona replay verifies counted Rows/Cards continuation and filters from server facets.

## Loop prompt cancellation (2026-10-05 QA)

- The daemon action adapter preserves the ACP canceled outcome as the existing safe cancellation
  failure. Run-agent output validation cannot restart a canceled turn as a schema-repair prompt.
  Measured usage survives the error. The real Manager stop and binding owners remain unchanged.
- Native loop_cancel and loop_node_cancel, CLI/HTTP/UDS cancellation, Goal command judges and Web
  Cancel run retain their current contracts. No tool ID, DTO, extension/hook/config key, persisted
  enum or schema changes. Workspace/profile authorization and owned-session isolation are unchanged.
- The official skills/compozy/references/loops.md cancellation contract already requires fencing
  new work and stopping owned sessions; this repair restores that promise. Site cancellation
  guidance remains accurate. QA owner: LP-003 and BUG-20261005-loop-cancel-leaves-worker-running.
  The canonical prompt-adapter suite covers the canceled outcome; existing run-agent and judge
  suites are adjacent canaries. A fresh Bruno Web cancellation replay supplies real provider proof.

## Loop worker stop attribution (2026-10-05 QA)

- **Native tools and public surfaces:** Session status/history through CLI, HTTP, UDS and native
  tools gain the additive stop reason `owner_released`. The cleanup relay uses it for automatic
  terminal, reseed and revoked-binding retirement; explicit stop/cancel remains user-requested.
  Existing IDs, routes, request shapes and historical classifications remain valid. OpenAPI and
  generated Web types co-ship from the enum source.
- **Extensibility, hooks and configuration:** No hook, SDK or configuration shape changes. Task
  and Loop outcomes retain their authority; retiring a worker does not assert success. Actual
  provider/process failures continue to take precedence over neutral retirement.
- **Workspace data isolation:** Existing cleanup identities and scoped ownership determine the
  session to stop. Retry/acknowledgement and first-writer cleanup metadata remain unchanged.
  The existing unconstrained persisted stop-reason text needs no schema migration or history rewrite.
- **Web and documentation:** The session status fold attributes the new reason to automatic
  retirement from either the resource or durable transcript, without an operator marker or false
  failure. The lifecycle stop-reason table and official `skills/compozy/references/loops.md`
  explain the distinction. QA owner: LP-003 and BUG-20261005-loop-worker-cleanup-user-canceled;
  canonical relay, lifecycle, node-cancel and Web status suites own focused coverage, followed by
  real successful/exhausted owner replays and an explicit-cancel canary.

## Loop terminal failure navigation (2026-10-05 QA)

- Web offers failed-step navigation only for an actual failure blocker with a node or gate
  reference. Budget exhaustion and stalling retain generic Inspect without inventing a failure.
- Native tools, CLI/HTTP/UDS contracts, extensions, hooks, configuration and persisted data are
  unchanged. Existing scoped run reads remain authoritative; no migration or isolation change.
- No official skill or site contract changes are needed. QA owner: LP-003 and
  BUG-20261005-loop-budget-phantom-failure-action. The existing LoopRunBriefing component suite
  owns action availability/navigation; a real exhausted/failed pair supplies the persona replay.

## Loop gate succession and progress bounds (2026-10-05 QA)

- The coordinator's initial rejected control plan uses the existing idle-generation
  finisher, preserving gate verdicts and explicit routes before terminal/successor
  admission. Successful control-only completion keeps its existing behavior.
  Gate-driven revise/next_generation checks the established
  repeated-blocker and failure terminal guard before creating another generation.
- Native loop_run/status/why and their CLI/HTTP/UDS/Web consumers retain the same tool IDs,
  routes, DTOs, terminal enums and configuration keys. This restores authored behavior;
  no compatibility shim, schema migration or historical run rewrite is required.
- Existing generation/gate hook dispatch and scoped workspace/profile readers remain the
  owners. No extension SDK or config shape changes. Stored output payloads continue through
  the existing hydration path; no new persistence or cross-workspace lookup is introduced.
- Site guardrail guidance and official skills/compozy/references/loops.md retain the same
  contract. Clarify initial in-body routing and repeated-blocker precedence there. QA owner:
  LP-003, LP-revise-repair-context and the two gate-routing bug records in the owning report.
  Existing coordinator suites own the invariants; real command-gate runs own replay evidence.


## Desktop restoration readiness and shortcut verification (2026-10-06)

- Web waits for existing window-manager configuration before exposing restored desktops,
  preventing a transient empty composer from delaying restored frames. Internal projection
  only: native tools, CLI/HTTP/UDS contracts, extensibility/hooks/configuration, workspace
  isolation and persisted layouts are unchanged. No migration or compatibility shim.
- Electron changes only the packaged test's native-focus precondition; production security
  and shortcut behavior remain unchanged. Official skills and public site documentation
  require no changes.
- QA owners: ET-web-desktop-shell-lifecycle (E2E-023) and
  ET-electron-session-copy-debug (E2E-034). Evidence and verification limits are recorded
  in `docs/qa/reports/2026-10-05-dependency-upgrades.md`.


## Loop-record navigation verification (2026-10-06)

Verification only: the existing Tasks E2E now observes the rendered Loop detail
before returning. Native tools, extensibility/hooks/configuration, workspace data
isolation, public Web behavior, docs contracts and official skills are unchanged.
No migration or release note is needed. QA owner: TA-web-tasks-calm-default-reveal.

## Terminal fixture verification (2026-10-06)

Verification only: the existing interactive CLI test peer answers required DA1
queries, and the alternate-screen fixture builds before entering the isolated home.
Production native tools, public surfaces, extensibility/hooks/configuration, workspace
data isolation, Web behavior and official skills are unchanged. No migration or
release note is needed. Existing terminal E2Es own the invariants; QA slices are
ET-terminal-cli-public-contract and ET-terminal-stream-resilience. Evidence and
remaining verification are in `docs/qa/reports/2026-10-05-dependency-upgrades.md`.

## Profile recovery guidance during registration retries (2026-10-06)

The Web client retains its last registration failure while a retry is in flight,
clearing it after success or rebinding the workspace/profile. Native tools, CLI/HTTP/UDS
routes and DTOs, hooks/extensions/configuration, persisted data and workspace/profile
isolation are unchanged. No compatibility adapter or migration is required. Official
skills/compozy and site contracts remain accurate; the release note explains the visible
repair. Owner: ET-profile-operations-recovery and the existing registration hook suite;
evidence: `docs/qa/reports/2026-10-05-dependency-upgrades.md`.

## SQLite commit acknowledgement (2026-10-06)

The shared write boundary checks cancellation after the existing mutation authorization fence,
then finishes COMMIT without caller cancellation, matching driver transaction semantics.
Successful writes retain their success acknowledgement instead of being retried as deadline
failures. Native tools, CLI/HTTP/UDS and Web consumers benefit without DTO, tool ID, hook,
extension SDK or configuration changes. Workspace/profile scoping and persisted schemas stay
unchanged; no migration, compatibility adapter or official skill/site rewrite is needed.
The roster read E2E now owns a cancellation-driven retry fixture; the separate two-second
node-timeout E2E remains unchanged. Owners: TestExecuteWrite, TestDaemonToolEventSink,
LP-run-read-agent-journey and ET-skill-view-actionable-errors.
Evidence is recorded in docs/qa/reports/2026-10-05-dependency-upgrades.md.

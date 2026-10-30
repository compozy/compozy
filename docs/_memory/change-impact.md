# Compozy Change Impact

## Merge-train skip-link focus repair — 2026-10-29

Owner: merge train main CI run `40942387025`, web shard 2. Web-only focus styling
keeps Skip to content out of layout flow, preserving the desktop viewport and window
geometry. No native tool, CLI/HTTP/UDS, extension, hook, config, persisted-data,
workspace-isolation, or official `skills/compozy/` contract changes. The deep-link
isolation QA scenario records the failing journey and focused Chromium verification;
its existing E2E remains unchanged and full retest belongs to CI.

## Claude Keychain token refresh (#1023) — 2026-10-29

Owner: `fix/issue-1023-claude-keychain-access`; targeted evidence in
`docs/qa/reports/2026-10-29-issue-1023.md`.

- **Runtime / native tools / CLI / HTTP / UDS:** macOS Claude usage reads through the Apple
  `security` tool. Background intent checks ACL, partitions, unlock state, and confirmation flags
  without UI first. Explicit local intent may authorize directly. Existing fields and statuses
  remain; `keychain_read_timeout` is an additive error code in the existing unconstrained string
  field, mapped to `needs_consent` and Refresh. No DTO/schema/SDK shape changes or migrations.
- **Security / config / isolation:** `keychain_access` remains operator opt-in. Routed service and
  account, credential cache ownership, revocation, and profile boundaries remain authoritative.
  No shell, secret logging/persistence, or Keychain mutation. A short background deadline kills the
  process group; a check/read race can still show a brief prompt. No new hook or extension surface.
- **Web / docs / official skill:** the existing status model renders timeout recovery with its
  diagnostic. Provider usage docs, both affected QA scenarios, and `skills/compozy/` explain the
  metadata gate, stable reader, recovery, and race limit. No UI structure or copy framework changes.
- **Verification:** focused Go race tests and real macOS Keychain probes; isolated-daemon public
  API/CLI walk. Broad gates belong to CI per the issue's explicit instruction.

## TanStack data loading — 2026-10-28

Owner: branch `perf/tanstack-data-loading`; evidence and verification in
`docs/qa/reports/2026-10-28-tanstack-data-loading.md`.

- **Web:** settled pointer/keyboard intent warms dock app code and canonical data,
  session rows/cards warm only the selected transcript head, and caught-up session
  streams suppress redundant stale mount/focus rereads. Agents and Loop run loaders
  match their mounted consumers. Concurrent stream listener discovery shares one read.
- **Native tools / CLI / HTTP / UDS:** existing endpoints, DTOs and `compozy__*` IDs
  remain unchanged. Remote stream tickets remain individual; only listener discovery
  is shared. No public migration or compatibility shim is required.
- **Extensibility / hooks / config / workspace data:** no changes to hooks, extensions,
  config keys or persisted state. Profile/workspace authorization and query keys remain
  authoritative; speculative reads recheck scope after detail resolution. Cursor pages,
  stream fences, explicit invalidations and disconnected freshness remain intact.
- **Docs / official skill:** affected Web QA scenarios and the dated report co-ship.
  `skills/compozy/` needs no change because commands and operating contracts are unchanged.
  Local checks are affected suites, typecheck, lint and isolated real-daemon browser
  journeys. Global gates run in CI, as explicitly requested by the user.

## Subagent status review follow-ups (#993) — 2026-10-28

Owner: PR #993 review threads. Recovery settles a foreground native row from its committed tool
result before task summaries and reads each parent turn's transcript once. Finalization keeps a
known `child_activity` when the caller omits it. `subagent_summary.live` counts a resumed child, as
the rows and composer already do; the group clock hides while a resumed child works. **Public
surface (additive, SD-013):** `GET .../sessions/{session_id}/subagents` gains optional
`order=newest|urgency` (default unchanged); OpenAPI and TS/Go SDKs regenerated; the sidebar branch
pages most urgent first so a waiting or failed row is never behind "Show more". No migration,
config key, native tool, hook, extension SDK, or `skills/compozy/` change; isolation unchanged.

## Subagent child activity and native task lifecycle — 2026-10-27

Owner: branch `fix/subagent-status`. Delegated rows gain a persisted `child_activity`
(`''|working|idle|stopped`, migration `00154_schema.sql`) that mirrors the child session after
settlement without touching status, result, delivery, or wake. Claude sessions request four raw SDK
task subtypes (`emitRawSDKMessages`) and settle background provider-native rows per task.

- **Native tools / CLI / HTTP / UDS:** additive only. `SubagentPayload.child_activity` (omitempty)
  in OpenAPI/SDKs, `compozy__subagent_status`/list results, and `session subagents show`. Provider-native
  rows keep their status/progress/result fields; they now settle earlier and may gain a result from a
  later task summary. A new `native_task` event type is persisted in the session ledger (visible in
  raw event reads) but is not a prompt-stream or transcript part. No new routes, IDs, or cancel authority.
- **Extensibility / hooks / config:** `subagent.settled` fires once per row; result enrichment is a
  normal row update. Claude session `_meta.claudeCode` gains `emitRawSDKMessages` for the Claude bridge
  only; existing options, wake exclusions, and other providers are unchanged. No AIR capability,
  native child sessions, or `backgroundSubagents` are advertised.
- **Workspace data / isolation:** lossless table-rebuild migration (existing rows read `''`); typed
  task observations are redacted like other transcript payloads; enrichment and replay are guarded by
  workspace, parent, parent turn, tool call, terminal status, and NULL result. `_claude/sdkMessage` passes
  the bound-session guard; foreign/fork ids are refused.
- **Official skill / docs / QA:** `skills/compozy/references/native-tools.md` and
  `runtime-operations.md`, `packages/site/content/docs/sessions/subagents.mdx`, and the
  `ET-web-native-subagent`, `ET-web-subagent-card`, and `ET-web-session-sidebar-threads` scenarios
  co-ship. Real Claude walks of the per-task lifecycle remain pending (see the native scenario).

## Appearance fonts (#984) — 2026-10-27

Owner: branch `feat/font-catalog-984`; plan `.claude/plans/font-catalog.md`, research
`.audits/exploration/20261027-163010-font-touchpoints/analysis/summary.md`.

- **Config and contract:** additive public surface (SD-013), no delete targets or migration.
  New global, live `[shell.appearance] ui_font` / `mono_font` (one family name, letters/digits/
  space/`-_.'`, ≤128 chars, `""` = default; invalid → `shell_config_invalid`). `GET/PATCH
  /api/settings/shell` gains optional `config.appearance` (PATCH: omitted preserves, `""` resets
  and deletes the key); HTTP and UDS share the route; OpenAPI, TS and Go SDKs regenerated. CLI
  `compozy config set|unset shell.appearance.*`. Native `compozy__config_*` allow-map unchanged
  (follows `shell.sessions.group_by`).
- **Web and desktop:** `--font-sans` / `--font-mono` lead with `--user-font-*`, set inline on
  `<html>` by `FontAppearanceSync` and replayed pre-paint by `/theme-boot.js`; the desktop boot page
  strips the override and keeps built-in fonts. Terminals load the new face, refit and re-propose
  PTY size on a live change. JetBrains Mono is a lazy chunk; the desktop grants `local-fonts` for
  the installed-font list. Mermaid diagrams and `--font-keys` keep their own fonts.
- **Isolation:** daemon-wide, not workspace- or profile-scoped; workspace overlays still reject
  shell keys. **Extensibility/hooks:** none. **Official skill:** `skills/compozy/references/
  configuration.md`. **Docs:** site `config-toml.mdx`, `COPY.md`; QA
  `ET-web-appearance-fonts-sync` plus impact notes on `APP-theme-native-no-flash` and
  `ET-web-font-assets-strict-csp`.

## Desktop update session continuity — 2026-10-27

Owner: branch `fix/update-session-continuity`; evidence in
`docs/qa/reports/2026-10-27-update-session-continuity.md`.

- **Runtime/CLI:** app-owned bootstrap provisions the verified replacement before requesting
  the existing durable restart operation over the configured local UDS. It no longer sends the
  user-stop signal. The existing session checkpoint/admission and subagent recovery owners
  retain their safety gates, ordering and exactly-once continuation identities. The daemon
  captures its executable path before an atomic binary rename can redirect Linux relaunch.
  Linux bootstrap also preserves the original path when replacing a released daemon that
  resolves its executable only at restart time: link the recovery copy, then atomically
  publish the new binary without renaming the running executable to the backup path.
  Runtime backup removal follows coordinated restart so an executing Windows binary can exit.
  Failed provisioning keeps live work; failed restart retains its backup and reports the error.
  Successful replacement retains the public bootstrap `provision` resolution through readiness.
- **Native tools / HTTP / UDS:** no new routes, DTOs, CLI flags or `compozy__*` IDs. Existing
  restart and session read surfaces remain authoritative. Explicit user stop stays cancellation.
- **Extensibility / hooks / config / data isolation:** no schema migration, config key, provider
  credentials or hook changes. No new store or recovery scheduler. Bootstrap targets only its
  local runtime home and its custom socket, including when the operator has a remote daemon
  connection selected.
- **Web / Docs / official skill:** no presentation workaround. Web receives the repaired
  runtime state. The desktop operations documentation and four affected QA scenarios co-ship.
  `skills/compozy/references/desktop.md` already describes verified bundle provisioning and
  runtime-owned lifecycle controls; its commands and ownership guidance remain unchanged.
- **Verification:** extend the existing bootstrap suite, daemon restart suite and native
  E2E-016 update journey. Native installation uses isolated fixture apps/homes and real daemon,
  storage and ACP subprocesses; it does not certify the published signing/distribution chain.


## Session responsiveness — 2026-10-26

Owner: branch `perf/session-responsiveness`; measured evidence and remaining work in
`docs/qa/reports/2026-10-26-session-responsiveness.md`.

- **Runtime and database:** compute catalog sort keys once, resolve board PR/default
  branch facts by workspace JSON key, and hydrate only active family members for detail
  relations. Session scope, ordering, exact totals, cursor fences and active overlay
  remain authoritative. Reuse the existing one-minute repository identity window for
  missing/local origins; cancellation, deadlines and runner-start failures do not enter
  that cache. No schema or persisted-data change.
- **Web:** combine each session cache wake into one TanStack invalidation. Live, usage,
  mutation, inactive-cache and profile-lens reread semantics remain intact; stream
  debounce/detach behavior is unchanged. Catalog reconciliation retains its trailing
  read and stopped-transcript obligations with fewer cache traversals. A pinned core
  dependency patch lazily creates tool pipelines and disposes them with their runtime,
  including abandoned React renders and normal session closure. Authoritative geometry
  subscribes at the window layer; shell chrome reads window membership and attention
  indicators, and the closed desktops overview suspends its geometry projection. Opening
  the overview reads current state immediately. Model-catalog projections retain their
  collections until provider/catalog inputs change, and agent create/duplicate callbacks
  capture stable mutation functions. Provider order, auth gating, refresh/recovery, and
  draft behavior are unchanged. Profile-selection writes survive document reload using the
  existing settings transport's keepalive pattern, retaining optimistic switching and failure
  rollback. Window persistence and live-data ownership are unchanged.
- **Native tools / CLI / HTTP / UDS:** existing session list/status/board results retain
  their schemas and semantics. No generator, SDK contract or public tool ID changes.
- **Extensibility / hooks / config / isolation:** no new hooks, extension behavior,
  settings, credentials or cross-workspace reads. Repository identity errors use the
  existing per-root bounded freshness window; provider/forge response freshness is unchanged.
- **Official skill / public docs:** `skills/compozy/` and product instructions need no
  change because public workflows are unchanged. The performance report and existing
  streaming/board QA scenarios co-ship. Browser memory, API and board evidence are recorded
  in the owning report; the existing desktop-shell lifecycle scenario also covers the
  unchanged 11-window performance envelope. Model-catalog cold-open and agent-create
  scenarios record the projection follow-up; the profile-switcher scenario records immediate
  reload persistence. Complete delivery gates are tracked on PR #983.

## Hosted MCP proxy reconnect after a model switch — 2026-10-19

Owner: session `sess-dcba53bda9072a50` nonce-invalid incident. Releasing the authenticated current
proxy rearms its existing launch for one replacement proxy within `tools.hosted_mcp.bind_nonce_ttl_seconds`.
Concurrent binds still fail closed; a stale release cannot reopen or extend the current window, and
session stop or a new launch invalidates the previous credential. Peer/binary checks, workspace/profile
scope, active-run identity and tool authorization remain unchanged. No persisted shape, migration,
public DTO, CLI flag, native tool ID, hook, extension SDK or config key changes. Web and site need no
production changes; the official `skills/compozy/` operating contract is unchanged. Regression owner:
`TestHostedServiceBindNonceLifecycle`; QA owners: `RT-agy-acp-provider` model-switch slice and `ET-048`
proxy-reconnect slice. The isolated real Agy 1.4.1 walk exercises Flash → Argon → Flash with successful
workspace reads and an unchanged ACP session ID. The operator chose code-only delivery: the live
daemon and original session are not restarted or modified.

## Agy session reasoning selector — 2026-10-19

Owner: session model-picker reasoning regression. The built-in `agy` now enables the existing
`acp_option` strategy, so model-specific `thinking_level` values survive catalog projection and reach
the shared reasoning slider. Explicit `models.reasoning.apply = "none"` overrides still win; no model
IDs or effort levels are invented. Native provider/model/session tools and CLI/HTTP/UDS reuse their
existing contracts. Hooks, extension SDKs, config keys, persisted shapes, and workspace/profile
isolation are unchanged; no migration or compatibility adapter is needed. Web needs no production
change. The provider site guide co-ships; official `skills/compozy/` already documents thought-level
selection and requires no contract change. Regression owners: the daemon Agy catalog/context suite
and the existing AGY picker-to-prompt browser case; QA owner: `RT-agy-acp-provider` reasoning slice.

## Authored inline code whitespace review repair — 2026-10-24

Owner: post-merge audit of PR #968. `StreamMarkdown` authored mode preserves consecutive spaces
in inline code without applying whitespace preservation to list containers or fenced code.
The session transcript browser scenario and ACP fixture co-ship the send/reload regression.

- **Web/Docs:** user-message rendering only; owning QA is
  `docs/qa/scenarios/RT-session-transcript-whitespace.md`. Assistant rendering is unchanged.
- **Native tools, extensibility/hooks/config, workspace data, official skill:** no contract,
  persisted bytes, schema, authorization, or `skills/compozy/` changes.

## Interrupted worktree removal review repairs — 2026-10-24

Owner: post-merge audit of PR #964. Existing CLI/HTTP/UDS removal and
`compozy__worktree_remove` retain their schemas and force acknowledgement. Empty orphaned
checkouts now respect Git locks; retries and boot recovery clear the saved Git entry before
tombstoning an absent checkout. Identity mismatch or cleanup failure remains retryable.

- **Isolation and data:** validate the saved admin directory and backlink under the repository
  lock; remove only that entry, never prune unrelated worktrees. No schema or migration change.
- **Hooks/extensions/config:** existing hooks and configuration stay unchanged; the removed event
  occurs only after directory and Git registration cleanup succeeds.
- **Web and official skill:** existing removal/refusal flows and the documented force policy
  remain valid; no Web, tool descriptor, SDK, or `skills/compozy/` contract change is required.
- **Docs/QA:** `packages/site/content/docs/worktrees/removal-and-recovery.mdx` and
  `docs/qa/scenarios/RT-worktree-reconcile-branch-safety.md` cover lock acknowledgement, interrupted
  cleanup, and identity mismatch. Real-Git lifecycle regressions own these invariants.

## Sessions Kanban — session PR link, `waiting` badge, board columns — 2026-10-24

**Owning spec:** `.compozy/tasks/sessions-kanban/_spec.md`. Impact Analysis is in Part II, the
decisions are ADR-001 through ADR-004, and the exploration is in `analysis/` (2026-10-23, revised
after peer review round 1). Implementation slices link to this entry and update only the entries
they affect.

- **Native tools.**
  - `compozy__session_list` gains the `board_column` and `children` input filters.
  - `compozy__session_list` and `compozy__session_status` results gain these fields: `branch`,
    `pull_request`, `pull_request_lookup`, `board_column`, `parent_title` and `children`.
  - The badge enum adds `waiting`.
  - No new tools; the catalog and digests regenerate in the same slice.
- **CLI/HTTP/UDS.**
  - CLI: `compozy session list` gains `--column` and `--hide-children`, and there is a new
    `compozy session pr <id> [--refresh]`.
  - `GET /api/sessions` accepts `board_column` and `children=exclude`, applied before pagination.
  - `GET /api/sessions/facets` adds `board` inside every existing facets object.
  - New routes: `POST /api/sessions/{id}/pull-request/refresh` (app-token allowed, like
    `GET /api/sessions/{id}`) and `POST /api/workspaces/{workspace_id}/worktrees/{worktree_id}/exit/prepare`
    (app-token denied, like the exit plan; resolves the native PR capability with an 8 s bound so plain
    exit-plan reads stay provider-free).
  - Session payloads gain seven fields (including `waiting_on`), and the badge gains `waiting`; monitors become the `active_monitor` work signal.
  - All of this is additive public surface (SD-013 ladder (a)); nothing is retired. OpenAPI and
    `@compozy/client` regenerate in the slice that adds each field.
- **Extensibility and hooks.**
  - The `forge/status` result of `forge.provider` keeps every existing field. It gains optional
    `checks`, `review_decision`, `mergeable`, `updated_at`, `selection` and `retry_after` (the last
    only together with `cause`, carried through the extension and daemon error adapters). Older providers keep
    working; their missing facts render as unknown.
  - The GitHub provider fills the new fields through paginated GraphQL with aggregate check counts.
  - No hook event, Host API, manifest or MCP changes.
- **Config.** None.
- **Workspace data isolation.**
  - New table `forge_pull_requests`, keyed by `(workspace_id, repo_key, branch)`. It cascades on
    workspace delete and is read only within the session/workspace read scope.
  - `worktree_forge_status` rows with a branch and a parseable `pr_url` (which gives the repository
    identity) are copied into it; the table is renamed to `worktree_forge_status_legacy`. Every
    row is retained, and the legacy table is never read (migration `00153`). Dropping it requires a
    later signed-off ADR.
  - Managed delivery and cleanup keep their exact provider evidence local; they neither read nor write
    the display cache, and request a canonical refresh on completion.
  - New column: `sessions.branch`.
  - Child rollups and parent titles respect the read scope.
  - No credentials appear in payloads, logs or errors.
- **Official Compozy skill.**
  - `skills/compozy/references/runtime-operations.md`: `waiting`, board columns, `compozy session pr`.
  - `skills/compozy/references/native-tools.md`: the new fields and filters.
  - `skills/compozy/references/worktrees.md`: the PR display cache versus exact delivery evidence.
- **Web/Docs.**
  - `web/src/systems/session`: PR chip, `ToggleGroup` view toggle, board, cards, badge dictionary,
    60 s visible re-read.
  - `web/src/systems/os/lib/attention-delivery.ts`: toast copy.
  - `packages/site`: a "Sessions board" page with a step-by-step tutorial, the updated badges, and
    the generated CLI reference.
  - QA: `docs/qa/scenarios/` deltas ship per slice; the final QA pair walks the real GitHub PR.

## Stable release line v0.4.0 — 2026-10-23

Owner: plan `.claude/plans/stable-release-040.md`, branch `feat/stable-release-040`, with
`compozy/releasepr` v0.0.31 (stable predecessor includes prereleases; `Release-As` footer from the last stable tag).

- **Release surfaces (public, SD-013):** automatic releases follow the `RELEASE_AUTO_CHANNEL`
  variable (`stable` or `beta`) through `scripts/resolve-auto-release-identity.sh`; stable publishes
  the release PR version. The manual `legacy` dispatch option and its runbook are removed — v0.2 is
  end-of-life. The beta path stays available for explicit dispatch. GitHub/site release bodies cover
  only the range since the nearest tag, prereleases included. `cliff.toml` bumps the minor for
  feature releases. npm `latest` and Homebrew carry stable; CI cannot move the npm `beta` dist-tag
  (trusted publishing authorizes `publish` only).
- **Desktop:** builds accept `stable` (`COMPOZY_RELEASE_CHANNEL`, `release:config`/`release:collect
  --channel`). The `desktop_feed` dispatch input lets the bridge beta `0.3.0-beta.39` read
  `channel-stable`; `channel-beta` stays frozen on it. Stable publishes to `channel-stable` only;
  `compatibility-channels` plus `min-app-version` keep the bridge app served.
- **Native tools / extensibility / hooks / config:** no tool, hook, extension, or config-key change.
- **Update and CLI:** the beta track offers any newer stable release; desktop provenance accepts
  `stable`; npm guidance is `npm install -g @compozy/cli`. No persisted state shape changes.
- **Workspace data isolation:** not applicable — release tooling only.
- **Official skill / Web / Docs / QA:** `skills/compozy/references/runtime-operations.md`, README,
  site install/landing/migration/desktop-release pages, `MIGRATION_GUIDE.md`, and
  `REL-beta-channel-contract`, `REL-beta-install-paths`, `REL-beta-self-update` co-ship; the
  scenarios need a re-walk before the v0.4.0 publication.

## User scope work — `user-scope-work`, 2026-10-21

Owner: spec `.compozy/tasks/user-scope-work/` (prerequisite of `all-projects`), branch
`feat/all-projects`, slices U1–U5 + UW. Evidence: `docs/qa/reports/2026-10-21-all-projects-user-scope.md`.

- **Native tools:** every `workspace` input parses `"@user"` before registry lookup (User scope of the
  call's profile); WM/desktop/layout tools map it to the reserved `user` partition. User ↔ project and
  same-workspace/other-profile requests are cross-scope and reach `workspaceaccess` (typed `TargetRef`
  with kind `workspace|user|aggregate` and profile; no same-workspace shortcut across profiles).
  Sessions with effective `workspace_access: unrestricted` pass workspace-axis checks, may pass
  `profile`/`all_profiles`, and read aggregates (`requireOperatorOrGrantedAgent`); `global` and other
  profiles' partitions, presence leases, and notification receipts stay operator-only.
  `compozy__agent_create` and agent update tools reject setting/raising the grant
  (`workspace_access_operator_only`). Native loop request reads carry the authenticated profile.
- **Extensibility, hooks, config:** new `[user_scope] work_dir` (default `~`, profile overlay, `live`,
  validated at use → `user_scope_work_dir_invalid`). New strict AGENT.md `workspace_access`
  (`unrestricted`); marketplace/extension sources drop it (`workspace_access_untrusted_source`); older
  daemons reject files using it. Hook payloads for User work carry empty `workspace_id` and session
  `scope: "user"`; `tool.pre_call` still runs before the grant decision. No new hook events, extension
  permissions, or tool IDs. App tokens never carry the grant.
- **HTTP/UDS/CLI:** `@user` on route paths and selectors; `POST /api/sessions` `scope: "user"`
  (`session_create_scope_conflict`); session payloads add `scope` and `effective_workspace_access`;
  agent DTOs add `workspace_access`; errors `user_scope_unsupported`, `user_scope_worktree_unsupported`,
  `workspace_name_reserved`, `workspace_reserved`; `workspace_home_forbidden` extended to
  `COMPOZY_HOME`. CLI `--workspace @user` everywhere, `compozy session new --user`, `agent info` prints
  `workspace_access`. OpenAPI, TS/Go SDKs, and CLI reference pages regenerated.
- **Workspace data isolation and user state:** User is per profile and never a `workspaces` row; every
  User read filters `workspace_id ''` (or `NULL`) **and** `profile_id`; watch events compare exactly.
  Migrations: `00145` rebuilds `loop_config`, `loop_definition_snapshots`, `loop_ui_annotations`, and
  admission claims with `profile_id`; `00147` relaxes the session-folder/derivation owners for User;
  `00150` adds `sessions.effective_workspace_access` and `grant_ceiling` on tasks, automation
  jobs/triggers, and Loop runs (empty for existing rows); `00151` + `00152_user_scope_data.sql`
  (hand-written data migration) add `user_scope_migrations`, `user_scope_migration_archive`,
  `workspace_name_aliases`, rebuild `attention_workspace_mutes` and `tool_approval_pending` for User
  owners, re-key the `COMPOZY_HOME` pseudo-workspace into User per profile, merge
  usage/cursor/palette counters, archive collisions and workspace-only rows, and rename reserved
  workspace names; workspacedb `00005` adds the journal `legacy_import` ledger. Boot reconciliation
  copies journal rows and retained blobs (digest-verified, source kept as `.migrated`) and layouts
  (existing User layout wins) before User writers start, then deletes the registration only after a
  live schema reference scan. `keep_project` keeps worktree-bearing or Loop-target sources as
  projects. Session DB owner stamps and v1 creation profiles are never rewritten (User sessions use
  creation profile v2 with `@user`). `task_run_terminal_commands.workspace_id` stays `''` for User.
  Automation run attribution re-keys to `''` and keeps `NULL` unknown. Grant ceilings survive re-key.
- **Compatibility (SD-013):** reserved-name aliases resolve with a deprecation warning through v0.4.0,
  removed v0.5.0; `global` on data selectors resolves only a migration-created alias. Stored/emitted
  `"global"` values unchanged. Cross-workspace ADR-007 §1 superseded by USW ADR-003 for this one field
  (glossary + official references). Internal delete targets: every `globalWorkspacePath` use,
  `Request.TargetWorkspaceID`, the web `$HOME` binding and hidden-home partition.
- **Official skill (`skills/compozy/`):** `runtime-operations.md` (User scope and All projects),
  `native-tools.md` (Workspace Boundary `@user`, grant, WM partitions), `agent-definitions.md`
  (Unrestricted Workspace Access + authority matrix), `configuration.md` (`[user_scope]`, migration),
  `window-management.md` (Reserved Partitions); router row added in `SKILL.md`.
- **Web/Docs:** User switcher destination and desktop, New session in User, Unrestricted badge, agent
  editor Access field (UW). Site: `user-scope/` (overview, quickstart, unrestricted agents),
  `configuration/agent-md` and `config-toml`, `sessions/permissions`, `workspaces/resolver`,
  `workspaces/window-management`, `profiles/index`, migration guide; glossary entries User scope,
  Unrestricted grant, amended Cross-workspace access and Profile; release note
  `.release-notes/user-scope-work-*.md`.
- **Fixed during docs lab run:** User session create (reply-watch sender listing) and CLI/HTTP
  admitted prompts to User sessions failed; fixed in the docs slice with regression coverage.

## All projects — `all-projects`, 2026-10-21

Owner: spec `.compozy/tasks/all-projects/`, branch `feat/all-projects`, slices A1–A4 + AW1–AW4.
Evidence: `docs/qa/reports/2026-10-21-all-projects-user-scope.md`.

- **Native tools:** `compozy__window_*`, `compozy__desktop_*`, `compozy__layout_*` accept
  `workspace: "global"` for the operator only (binder, before registry normalization); every agent
  identity, including granted ones, is denied on `global`; other tools reject it. Window data binding
  is `route.search.workspace`. Native list tools keep workspace pinning and gain no aggregate params;
  aggregates are open only to operators and granted agents.
- **Extensibility, hooks, config:** no new hook events; workspace-unregistration retirement publishes
  on the existing workspace event channel. New `shell.sessions.group_by` (`none|project|profile`);
  `shell.sessions.scope` stays functional as legacy breadth intent through v0.4.0 (write warnings,
  `group_by` derived only while unset, `legacy_scope.revision` in `app_metadata`
  `shell.sessions.scope_revision`) and is dropped tolerantly in v0.5.0. `manual` sort accepted.
- **HTTP/UDS/CLI:** new app-denied operator aggregates `/api/loop-runs`, `/api/loop-requests`,
  `/api/loops`, `/api/worktrees`, `/api/terminals` (+ `/catalog-stream`, local only),
  `/api/automation/suggestions`, `/api/agents/aggregate`; `all_workspaces=true` + `workspace_ids` on
  sessions, tasks, task dashboard, overview, automation jobs/triggers/runs (existing omission
  populations, envelopes, and app access unchanged); errors `workspace_breadth_invalid`,
  `workspace_reserved`, `window_binding_invalid`. DTOs add `RunPayload.workspace_id`/`attribution`,
  `LoopRequestPayload.workspace_id`/`profile_id`, loop catalog `workspace_id` + `source.layer`, session
  facets `by_profile`, `SettingsShellResponse.warnings`/`legacy_scope`. Bell `workspace_label`
  `"Global"` → `"User"` (display string, changed without window). CLI `--all-workspaces` (exclusive
  with `--workspace`) on loop runs/requests/list, task list, automation jobs/triggers/runs/suggestions,
  worktree list, terminal list, agent list, with a `PROJECT` column.
- **Workspace data isolation and user state:** aggregates keep the profile lens and listener tiers;
  the needs-you bell stays operator-wide. Cursors embed breadth, filter, and profile. Migrations:
  `00144` (`idx_tasks_profile`), `00146` (loop run/request indexes), `00148` +
  `00149_automation_run_attribution.sql` (hand-written backfill from surviving definitions only;
  unknown stays `NULL`). Command-time window binding validation in `global`; `ValidateSnapshot` stays
  registry-free. Session deletion and workspace unregistration reconcile every partition of every
  profile, including navigation, closed entries, and undo/redo. Client store
  `compozy:active-workspace:v4` v1→v2 is owned by USW.
- **Official skill (`skills/compozy/`):** `native-tools.md` (All-Projects Aggregates),
  `runtime-operations.md`, `configuration.md` (`group_by`, `scope` deprecation),
  `window-management.md` (Reserved Partitions).
- **Web/Docs:** All projects mode, Group by, Project filter, composer pickers, project badges,
  open-in-place (AW1–AW4). Site: `all-projects/` (overview, Web quickstart, CLI/HTTP scripts),
  `config-toml` `[shell.sessions]`, generated CLI pages; glossary entries All projects and topbar;
  release note `.release-notes/all-projects-*.md`; migration guide section.

## Session monitor visibility — 2026-10-21

Owner: branch `feat/session-monitor-visibility`. Additive across every public surface (SD-013); no
delete targets. HTTP/UDS gain `listSessionMonitors` and `cancelSessionMonitor` under
`/api/workspaces/{workspace_id}/sessions/{session_id}/monitors`, and `SessionPayload` gains the
optional `monitor_summary` (OpenAPI, TS and Go SDKs regenerated). Every monitor start, cancel, or
settled delivery publishes a session catalog `upserted` event. Native monitor tools, hooks, extension
SDKs, config keys, and persisted user state are unchanged; monitors stay daemon-memory only.
Operator reads use the existing workspace-scoped session route guard, so workspace isolation is
unchanged. `@compozy/ui` `StateGlyph` gains the amber `monitoring` state; Web shows it on session rows
and adds the composer monitor popover. Site docs (`sessions/monitors.mdx`), the official skill
(`references/native-tools.md`), and QA scenario `RT-session-monitor-wake` step 8 are updated.


## Terminal journal storage under the daemon home — 2026-10-21

Owner: `internal/store/workspacedb/` and `internal/daemon/terminal_boot.go`.
The terminal database now uses `<COMPOZY_HOME>/workspaces/<registered-workspace-id>/compozy.db`,
matching the existing terminal artifact and recording directory. The daemon injects its resolved
home; storage never derives it from the project working directory or an ambient environment lookup.
Workspace removal uses the same path after restart, including when the project root is missing.

- Native `compozy__terminal_*`, CLI/HTTP/UDS, Web journal queries, hooks, extension SDKs and config
  keys keep their existing shapes. No SQLite schema or generated contract changes.
- Workspace registration IDs partition files; the existing durable identity check and profile-owned
  rows remain. A different daemon home has independent storage for the same workspace.
- Site journal documentation and `ET-terminal-journal-recording` step 6 describe the new location.
  The official `skills/compozy/references/terminal.md` still describes the same journal and profile
  operations and does not promise a project-local storage path, so it needs no change.
- Canonical verification: `workspacedb.TestOpen` owns storage, reopen, isolation and removal;
  terminal journal suites cover persistence consumers; existing HTTP/UDS terminal integration
  journeys exercise real processes. Test fixture constructors now pass an explicit isolated home.

### ADR: accepted storage hard cut

Decision: retire project-local database reads and writes without automatic import or fallback.
The user explicitly approved a hard cut on 2026-10-21 ("pode fazer hardcut mesmo"). This is a
specific exception to SD-013 regime 1's automatic upgrade rule, not a general policy change.
The conventional alternative is a lossless migration; the accepted tradeoff is that old command
history and its recording/artifact index are absent from the new journal. Existing project-local
SQLite files and retained artifact bytes are left intact. No files from a running older daemon
are deleted. The release note includes migration notes; internal constructors change together.

## Console lifecycle failures — issue #873

Owner: `fix/issue-873-console-lifecycle`. Web session detail polling and presence acquisition stop
after a scoped 404/410; profile boundaries remain enforced and the existing foreign-owner lookup
remains authoritative. Worktree catalog subscriptions use a canonical workspace set so list reorder
does not reconnect the socket. Transcript terminal previews attach only to a catalog-confirmed
running PTY. Pipe terminals retain recorded output and their terminal-window action. Attach-ticket
admission returns the existing `terminal_not_interactive` refusal for pipes before issuing a useless
ticket; the Web client treats that refusal as terminal rather than retrying.

Native tool IDs, CLI/UDS DTOs, hooks, extension contracts, config keys, persisted state and workspace
isolation are unchanged. HTTP retains the existing terminal error shape; no generator or migration
is needed. Session Markdown displays image references as text instead of interpreting host paths
as app-origin URLs. Validated attachment tiles retain their existing byte routes. `StreamMarkdown`
exposes the component override contract of its underlying primitive. Official `skills/compozy/`
operations are unchanged: pipe output remains available through read/wait. QA owners are
`ET-terminal-session-block-handoff`, `RT-session-done-presence`,
`ET-web-session-deep-link-isolation`, and `RT-worktree-web-missing-resolution`.
Historical HTTP 500s remain under investigation; these changes do not suppress them.

||||||| parent of e8a79c3ed (feat: remove discovered worktrees without adopting them)

## Remove discovered worktrees without adoption — 2026-10-20

Owner: `internal/worktree/remove_discovered.go` (`Service.RemoveDiscovered`). A Git-listed linked
worktree with no registry row is removed by its listed path: stale entries only lose Git metadata;
live checkouts reuse adoption identity checks and the dirty/unpushed refusals with a non-persisting
status read, and branches are never touched. Registered paths refuse with the new
`worktree_registered` code. Correction riding along: detached-HEAD removal risk compared
`HEAD --not --all`, which includes every linked worktree's HEAD and hid unique commits (this also
affected adopted detached worktrees); it now compares against branches, tags and remotes.

- **Native tools:** `compozy__worktree_remove` gains optional `path` (replaces `ref`; rejects
  `delete_branch`); `ref` is no longer schema-required but stays required in code unless `path`
  is set. Catalog golden regenerated.
- **CLI/HTTP/UDS:** `compozy worktree remove --path <path>`; `POST
  /api/workspaces/{workspace_id}/worktrees/discovered/remove` `{path, force?, expect_stale?}` → 204
  or the record removal 409 refusal payload; `expect_stale` refuses a checkout that reappeared with
  `worktree_discovered_changed` (Web sends it for stale targets). Removed/dismissed tombstones no
  longer claim their path in discovery or removal. OpenAPI and TS/Go clients regenerated.
- **Extensibility and hooks:** `worktree.pre_remove` payload gains additive `discovered: true` with
  an empty `worktree_id`/`origin`; stale entries skip the gate. No lifecycle event for discovered
  removals (no record ID). No config key.
- **Workspace data isolation:** the target must be listed by the workspace's own repository and
  must not belong to any registry row; no profile ownership exists to cross.
- **Official CompozyOS skill:** `references/worktrees.md` and `references/native-tools.md`.
- **Web/Docs:** discovered rows offer Delete worktree… / Remove stale entry… and join batch
  selection; single delete opens the batch dialog. Palette offers Remove for discovered rows. Site
  `worktrees/removal-and-recovery.mdx` and `agent-surface.mdx`; generated CLI page.
- **Compatibility:** additive public surface (SD-013); no shim, migration, or delete targets.
  QA owners: `RT-worktree-cli-lifecycle`, `RT-worktree-api-surface-parity`,
  `RT-worktree-web-removal-two-step`.
## Intel macOS release recovery — 2026-10-20

Owner: `.github/workflows/release.yml`, release run `39431274097`. The production
Intel runner uses TCP for Apple ticket delivery until actions/runner-images#14924
is resolved. Manual releases may reuse completed desktop artifacts from a prior
release run for the exact same source commit, then execute the existing smoke,
inventory, signing, and publication checks. The optional dispatch input is
additive. Native tools, CLI/HTTP/UDS, hooks, extension SDKs, config keys, workspace
isolation, persisted user data, Web, site docs, and the official `skills/compozy/`
contract are unaffected. Runbook: `docs/runbooks/release-recovery.md`.

## Session deletion lifecycle deadlock — 2026-10-21

Owner: `fix/session-lifecycle-freeze`, `internal/session/manager_delete.go`.
Deletion settles active/recovered stops, descendant cancellation, and session-owned Goals
before acquiring the global lifecycle lock for reversible history staging and catalog removal.
The per-session conversation operation remains held throughout, preserving serialization with
resume, another deletion, and workspace removal. This prevents subagent snapshot callbacks
from reacquiring a lifecycle lock already held by deletion and blocking all new session starts.

HTTP/UDS session DELETE and CLI session removal retain their existing contracts. Native session
creation/resume/spawn tools benefit from the released lifecycle lock; no native tool IDs, payloads,
hooks, extension contracts, config keys, persisted shapes, or workspace/profile boundaries change.
No migration, code generation, compatibility shim, Web implementation change, site documentation,
or official `skills/compozy/` update is required. Web deletion uses the repaired daemon path.
QA owner: `RT-session-delete-owned-history`, live delegated-tree deletion slice. The existing
daemon hosted-MCP integration suite exercises HTTP deletion followed by UDS creation/deletion;
`TestManagerDelete` retains ownership of rollback, history preservation, and crash recovery.

## Preserve configured commands across pre-start hooks — 2026-10-19

Owner: `internal/session/manager_start_launch.go` review correction. Pre-start hook command
rewrites stay in launch options; the runtime agent and inherited routing snapshot retain the
configured command and its built-in origin, while rewritten launches remain non-builtin. Compatible
spawned children apply their own hook once instead of inheriting the parent's rewritten command. Accepted route
fingerprints remain configuration-derived. `TestAgentPreStartHookProviderRouting` owns the
parent-first-prompt and child-launch regression; `TestAgentPreStartHookOpenCodeLaunchOrigin` also
proves that a child without its parent's hook retains npm fallback when native OpenCode is absent.
QA owner: `RT-session-spawn-wake` routing slice.
Native tools (including `compozy__session_spawn`), CLI/HTTP/UDS contracts, hook payloads and ordering,
extensions, config keys, workspace/profile isolation, persisted shapes, Web, site docs and the
official `skills/compozy/` contract are unchanged. No migration or compatibility shim is needed.

## Per-window session sidebar state — 2026-10-19

Owner: issue #843, branch `fix/session-sidebar-per-window-state`. Web-only: the session-window
sessions sidebar (open/closed, folder collapse, child-thread collapse) moves from one global client
store to per-window entries keyed by window scope (`byScope[windowId]`, shell entry = last-used state
new windows inherit). User state upgrades losslessly: the persisted `compozy:session:sidebar:v1`
blob (schema 2) migrates into the shell scope as schema 3; closed windows are pruned only by
the window-manager snapshot (workspace + profile partition) that listed them, so switching
workspace or profile keeps the other partition's windows. ⌘B, palette `sidebar.toggle`, and session cycling act on the focused window.
Native tools, CLI, HTTP/UDS DTOs, hooks, extension SDKs, config keys, the official skill, and
workspace/profile isolation are unchanged. Site `docs/sessions/index.mdx` and QA scenario
`ET-web-session-sidebar-per-window` record the behavior; sort order, list scope, and the All
profiles lens stay global.

## Composer mentions — 2026-10-19

Owner: spec `.compozy/tasks/composer-mentions/` (ADR-001..004); integration branch
`feat/composer-mentions`. `@path`, `@"path with spaces"`, `@folder/`, and `@session:<id>` tokens in
prompt text are resolved by the daemon on every admission path (send, queue, steer, replace,
promote) for every caller, independent of command ingress, against the receiving session's working
directory and workspace/profile. Files and folders reach the agent as literal text plus one ACP
`resource_link` each (no contents); sessions as a `<compozy_session_references>` block in reference,
inline (tools disabled + human ingress only), or unavailable form. Caps: 200 candidates, 50
files/folders, 16 sessions. Additive across every public surface (SD-013); no delete targets.

Native tools: no new IDs and no schema change — `compozy__session_prompt` accepts the tokens in its
`message` text; `compozy__session_history` is the tool the reference form points to;
`compozy__session_inputs_list` entries gain optional `mentions`. Extensibility/hooks/config:
`input.pre_submit` payload gains read-only `mentions` (`hooks.InputMentionMetadata`: `kind`, `token`,
`target`, `label`, `start`, `end`); a rewrite keeps only tokens still present and cannot add one;
extension SDK named types regenerate; `prompt.*` payloads unchanged; no `config.toml` keys (limits
fixed). Public wire: new `searchSessionFiles` route
(`GET /api/workspaces/{workspace_id}/sessions/{session_id}/files`, HTTP and UDS; 400
`invalid_request`, 404 `session_not_found`, 409 `session_directory_unavailable`), optional `mentions`
on `SessionInputPayload` and on transcript user-message metadata; CLI verbs, flags, and prompt DTOs
unchanged; OpenAPI and the generated Web client co-ship. User state: one additive Goose migration —
`mentions_json TEXT NOT NULL DEFAULT '[]'` on `session_prompt_admissions` and `session_input_queue`;
old event payloads simply lack `mentions`; the admission fingerprint version is unchanged. Workspace
data isolation: path mentions must stay inside the receiving session's working directory (real path,
at admission and again at dispatch); session mentions resolve only within the receiving session's
workspace and profile, never itself, through the same scoped lookup as `compozy__session_history`;
agent-origin prompts never receive inline history; persisted events, projections, hook payloads, and
API responses carry relative paths only. Official skill: `references/runtime-operations.md`
(session prompting: token syntax, resolution, delivery order, references forms, inspection).
Web/Docs: `_uiux.md` S1–S4 against `docs/design/opendesign/composer-mentions/` (VC-01..VC-22) — `@`
picker beside `/`, mention chip, placeholder, transcript chips; no new `@compozy/ui` primitive. Site:
new tutorial `sessions/mentions` ("Mention files, folders, and sessions"), the sessions index route
row, and the `input.pre_submit` note in `hooks/event-catalog`; regenerated API reference. Glossary
**Mention**; `COPY.md` §6 "Composer Mention Terms". QA: new `ET-session-mention-file`,
`ET-session-mention-folder-quoted`, `ET-session-mention-session`, `ET-session-mention-session-inline`,
`ET-session-mention-containment`, `ET-session-mention-typed-token`, `ET-session-mention-queued-input`,
`ET-session-mention-hook-rewrite`, `ET-session-mention-history-chips` (untested until the QA tail).
Data boundaries (`eng-data-boundaries` audit): `searchSessionFiles` scopes the session through
`requireSessionInWorkspace` and the profile read scope (foreign workspace or profile → 404 before the
working directory is resolved), HTTP and UDS share one handler with parity covering 404/409, and the
picker's session catalog query is scoped by workspace and profile on the server.

## Daemon idle-cost and renderer batching — 2026-10-18

Branch `perf/daemon-load-mitigations`. Hosted MCP projection streams wait on a `ProjectionEpoch`
change channel (3s fallback) instead of a 100ms poll, cache shared immutable snapshots, and Goal tool
availability uses a cheap per-session check invalidated on Goal/Loop transitions; the hosted MCP wire
(SSE names, digests) is unchanged. Model-catalog refresh reads owner IDs with `SELECT DISTINCT`, checks
TTL before ownership, honours TTL for providerless sources, skips row rewrites for unchanged content,
backs off failed probes, and bounds sweep parallelism; no schema change. Goal outbox and Loop effect
relays drain on post-commit store nudges; public config `goals.outbox_poll_interval` keeps its key but
now means the fallback sweep and its default moves 100ms → 2s (explicit values keep their cadence;
site config reference and official skill updated). Web live tail applies queued frames in one batch per
animation frame; smooth reveal pauses while hidden; desktop windows set `backgroundThrottling: true`.
Native tool IDs, hooks, extension SDKs, persisted state, and workspace/profile isolation are unchanged.

## Transcript survives a tool call crossing a Goal turn — 2026-10-18

Owner: issues #800, #801, #802. The transcript projector routes an identity-less `tool_call`
update (no tool name, title or input) to the entry that owns its tool lifecycle, the way results
already route. `dynamic-tool` parts always carry a `toolName`. The web client normalizes
`dynamic-tool` parts like `tool-*` parts, and it drops a part that still fails AI SDK validation
instead of failing the whole transcript. The live tail's transcript repair and query recovery back
off 5s → 60s. The Goal session outbox relay acknowledges a projection whose session was removed.
`ProjectionVersion` is unchanged: existing projections stay as stored, and the client salvages
their invalid parts.

Native tools, CLI/HTTP/UDS routes and DTOs, hooks, extension SDKs, config keys, workspace/profile
isolation and the persisted schema are unchanged. No migration is needed. The official skill and site
docs are unaffected. Web impact: a conversation that showed "Couldn't load this conversation" now
renders. QA owner: `ET-web-session-transcript-calm-grammar`.

## Role fallback after a provider refuses a title turn — 2026-10-18

Owner: PR #662. Automatic title generation runs each prompt inside its role-owned fallback
attempt. Typed authentication or rate-limit refusals before any output advance to the next route
only after the refused child stops. A turn that produced output remains accepted; failed child
cleanup retains ownership and ends the chain. Route account commands, ChainOwnerCaller, typed
start acceptance, cancellation deadlines and role.fallback.used events retain their current owners.
The retired checkpoint-summary and memory subsystems stay deleted. Existing auto-title and
role-fallback suites own the rejection, partial-output and cleanup boundaries.

Native tools, CLI/HTTP/UDS contracts, hooks, extension SDKs, config keys, workspace/profile data
isolation and persisted state are unchanged. No migration or compatibility shim is needed. The site
configuration reference describes the title-specific refusal boundary. Official skill commands and
Web behavior are unchanged; title generation uses the existing title QA journey.

## Release integration repairs — 2026-10-18

Pending-steer completion retains its provider outcome and retries persistence with bounded backoff
while preserving the dispatch lease. Logging happens outside the session lock, keeping status and
Stop responsive during a storage failure. Accepted steer events append by deterministic identity;
retries retain their complete event timestamp and correlation, and only newly inserted rows publish
post-record callbacks. The internal append result reports insertion without changing a wire shape.
`TestManagerAgentQueueDelivery` owns the regression and
`RT-session-message-origin` records the recovery walk. Native tool IDs, wire shapes, hooks,
configuration, official skill instructions, Web/Docs APIs, and workspace/profile isolation remain
unchanged; no schema migration is needed. Batch tool projection failures retain the existing safe failure-kind
logging once per batch. Event readers require metadata projection at compile time; the retired, unused
compaction-boundary test hook is removed in favor of complete context snapshots. Session fixture and
transcript-test imports follow the existing Web alias contract. Existing daemon integration assertions now match the
approved operator-first queue and restored sender-hop contract, while retaining exact delivery counts
and the independent forwarding-limit suite. The shared Go E2E seed disables host-load admission
for protocol journeys unless a scenario opts in, matching the existing daemon test harness. The seed
helper persists the complete load policy through the real config loader; production defaults,
deadlines and admission-policy coverage stay unchanged.

The App SDK rejects prompt EOF without a terminal marker and resumes transcript feeds with the
latest cursor, epoch and generation. The Go client preserves custom JSON wire values, caller deadlines
and wildcard-to-loopback discovery. The existing log-tail payload moves to the shared contract so
OpenAPI and both SDKs expose its SSE transport. App tokens cannot read operator status diagnostics or
register workspaces; identity and registered-workspace reads stay available. No native tool ID, hook,
configuration key or additional persisted shape changes; SDK site docs and stream QA co-ship.
Missing-session HTTP/UDS errors add the stable session_not_found code while preserving status and
message. Node-owned close waits for its launched child to exit within the stop deadline, including
when the CLI acknowledges shutdown early; replacement PID protection remains intact. The existing
SDK runtime journey, Node lifecycle suite, site guide and QA scenarios cover those boundaries.
The Web shell loads the remembered creation profile before an aggregate cold entry can read work;
the existing route and browser profile suites cover cache reuse and the reloaded destination. The
session_create description clarifies the resolved operator profile as well as inherited agent identity;
the generated native catalog co-ships, with no native ID, hook, config or workspace-isolation change.
Endpoint proof validates its request context after response-body cleanup, preserving timeout
diagnostics when cancellation produces EOF instead of a read error. Verification policy, nonce and
size checks, configured budgets, native surfaces and persisted endpoint state remain unchanged;
TestEndpointProbeDiagnostics and the connectivity-route QA scenario own this boundary. The pinned
Hey API generator patch handles its abort-cleanup Promise in the SSE template; patches/README.md
records the external reproduction and removal condition, and generated SDKs co-ship normally.

## Agent CPU priority and load-aware admission — 2026-10-17

Branch `perf/daemon-load-mitigations`. Agent ACP process groups now start at a background CPU
niceness (`setpriority(PRIO_PGRP)` after launch; Windows no-op) so tools they spawn inherit it, and a
daemon load sampler (1-minute load average / logical CPUs, darwin `vm.loadavg`, linux `sysinfo`)
makes the admission gate hold new sessions and prompts carrying the background-work context marker
(scheduled/trigger/extension automation dispatch, Loop action/judge sessions and prompts, scheduler
wakes) until pressure clears or a bounded wait elapses. User prompts, user-created sessions, subagent
spawns, and manual automation runs are never delayed. Public surfaces, all additive (SD-013): config
keys `daemon.load.agent_niceness` (10), `daemon.load.admission_threshold` (1.5, `0` disables),
`daemon.load.admission_max_wait` (2m), restart-required and agent-mutable through the config tools;
doctor item `runtime.load` over HTTP/UDS/CLI. Settings DTOs, OpenAPI, Web, native tool IDs, hooks,
extension SDKs, persisted state, and workspace/profile isolation are unchanged; no migration. Site
`configuration/config-toml.mdx`, the generated lifecycle matrix, and official skill
`references/configuration.md` record the keys.

## Session folders and manual order — 2026-10-17

Issue #781 adds daemon-owned session folders and manual session order. Persisted state: global
SQLite tables `session_folders` and `session_placements` (append-only migration 00139, additive, no
backfill; existing users see no folders and their current sort). Public surfaces, all additive
(SD-013): HTTP/UDS `GET|POST /api/workspaces/{workspace_id}/session-folders`,
`PATCH|DELETE …/session-folders/{folder_id}`, `PUT …/session-order/{folders,sessions}`,
`POST …/sessions/{session_id}/move`; `folder_id` on catalog rows and on `createSession`; catalog
sort and `shell.sessions.sort` gain `manual`; the catalog stream adds kind `layout` (no session id)
to `session_catalog_changed`. CLI: `compozy session folder list|create|rename|delete|reorder`,
`compozy session move`, `compozy session reorder`, `session list --sort manual`. Native tools:
`compozy__session_folder_{list,create,rename,delete,reorder}`, `compozy__session_move`,
`compozy__session_reorder`, and `manual` in `compozy__session_list`. Workspace/profile isolation:
folders and placements are keyed by profile and workspace; composite foreign keys prevent a session
from joining another workspace's folder; foreign sessions read as not found; subagents follow their
parent. Deleting a folder cascades only placements; session delete cascades its placement. Hooks and
extension SDKs are unchanged. Official skill `references/runtime-operations.md`, site
`docs/sessions/index.mdx`, generated CLI docs, and QA scenario
`ET-web-session-folders-manual-order` record the contract.

## Google Antigravity ACP provider — 2026-10-18

Owner: PR #773. The additive `agy` built-in launches the supplied `agy_acp_server`, uses
protocol-owned native login, and discovers exact model/configuration options through ACP. Provider
configuration and Settings gain the optional `auth_acp_method` field; source DTOs and generated
OpenAPI/client artifacts co-ship. Native tool IDs, existing CLI verbs, extension SDKs, hooks,
workspace/profile isolation and persisted database shapes are unchanged; no migration is needed.
Operator-owned login state follows the existing provider-home policy. Shared provider logos,
Settings fixtures, official skill provider/auth guidance and site provider/model-catalog docs are
updated. The long-tail provider removals and their compatibility window remain in effect.
Existing provider/auth, model-catalog, settings and KindIcon suites own the observable behavior;
the added provider QA scenarios retain their original journey steps. Authentication probes also verify
that the launch directory still exists before using it, falling back to the temporary directory
when the OS returns a stale working-directory path. The existing provider-handler subprocess
suite owns this regression.

## Long-tail built-in provider removal — 2026-10-17

Owner: branch `fix/issue-761-remove-longtail-providers`; issue #761.

The built-in registry drops `hermes`, `openclaw`, `blackbox`, `cline`, `goose`, `junie`, `kiro`,
`openhands`, `qoder`, `mistral`, `groq`, and `minimax` plus their 14 aliases. Kept: `claude`,
`codex`, `gemini`, `opencode`, `kimi-cli`, `qwen-code`, `copilot`, `cursor`, and the `pi_acp`
harness with `pi`, `openrouter`, `zai`, `moonshot`, `vercel-ai-gateway`, `xai`.

- **Native tools:** provider, model-catalog, and session tools lose the removed IDs from their
  projections; no tool ID, input, or output shape changes.
- **Extensibility/hooks/config:** provider IDs in `config.toml` (`defaults.provider`,
  `[providers.<id>]`), agent `provider:`, and Loop/automation runtime defaults are public surface.
  SD-013 deprecation window: a boundary shim in `internal/config/provider_retired.go` keeps
  removed IDs and aliases resolving with their former definition and logs one
  `provider.retired_builtin` warning per ID (`removed_in=v0.5.0`, `replacement` = exact
  `[providers.<id>]` TOML). Delete target in v0.5.0: the shim (public-surface regime); afterwards
  an undeclared reference fails as provider unavailable. A declared `[providers.<id>]` with its own
  `command` (and `harness = "pi_acp"` for pi IDs) is a plain custom provider. Hermes/OpenClaw
  special-casing (native `.agents/skills` readers, `HERMES_HOME`/`OPENCLAW_STATE_DIR` homes, Hermes
  live discovery, OpenClaw provider-managed strategy, auth-probe needle) is deleted (internal
  regime). Hooks and extension SDKs are unchanged.
- **Workspace data isolation:** no schema change or migration; persisted sessions and transcripts
  that name a removed provider stay readable. The Agents skill source now has no native readers,
  so `.agents/skills` is injected for every provider within the same workspace/profile scope.
- **Official skill:** `skills/compozy/references/{agent-definitions,extensions,native-tools,
  runtime-operations,tools-and-skills}.md` describe only kept built-ins, the custom-provider path,
  and the deprecation window.
- **Web/Docs:** Settings and pickers drop the removed providers and their logos; site docs
  (`agents/providers.mdx` gains "Run another ACP CLI" and "Retired built-in providers"), the bug
  template, `MIGRATION_GUIDE.md` (`kiro` → `unsupported_destination_value`), and the canonical copy
  trio (now Claude Code, Codex, and Cursor) are updated. QA: `RT-retired-builtin-provider-shim`
  added; `RT-hermes-live-model-readiness` and `RT-openclaw-provider-managed-runtime` retired.
  Release note: `.release-notes/twelve-long-tail-providers-leave-the-built-in-registry-1792269885.md`.

## Issue 764 — Idle UI polling and attention cost — 2026-10-17

`GET /api/notifications/attention` reads task attention once for every registered population
(`CatalogScopeAll`) instead of a global read plus one read per workspace; each inbox read
re-prepared the catalog CTE, so the cost grew with the workspace count. Notification identity
(`attention.Identity` over the task row), per-row workspace binding, `AllProfiles` read scope,
actor authorization, and the registered-workspace filter are unchanged; the needs-input escalation
read now maps the all population onto stored task scopes. In the web client, a session's
stream-backed reads (detail, by-id, usage, usage turns, prompt queue) drop to a 30 s safety read
while its transcript stream is live, background session windows stop polling their queue, and the
bell polls every 30 s while the document is hidden. Internal regime only: HTTP/UDS DTOs, CLI,
native tools, hooks, extensibility, config keys, persisted state, and official `skills/compozy/`
are unchanged; no migration or compatibility adapter is needed. Workspace/profile isolation is
unaffected. QA scenarios `RT-web-attention-title-count` and `RT-visible-session-streaming` record
the change. No site documentation changes are needed.

## Concurrent-session responsiveness — 2026-10-17

Owner: branch `perf/responsiveness-audit`; measured evidence, attempted experiments
and verification limits live in
`docs/qa/reports/2026-10-17-responsiveness.md`.

Database reads retain task activity/facets, pending terminal approval scope and
pagination. Workspace detail selects catalog IDs before hydrating their existing
metadata. Missing-origin Goal outbox rows remain durable and undelivered; they are
excluded before the delivery batch limit and become eligible if the origin is
restored. No schema or persisted-state shape changes are involved. Context reads consume
one ordered union of the five relevant ledger event types; usage notifications
read event metadata while retaining all-event cursor progression, snapshot
fences, cancellation and the existing store-owner retry behavior.

Web retains the same server-state keys, envelopes, workspace/profile partitions,
stream fences and visible-window ownership. Live catalog wakes coalesce canonical
detail rereads without canceling in-flight work or dropping the final wake. Weak
transcript-result retention releases obsolete arrays while held consumer views
remain valid. Exact query lookups preserve custom hashes and all by-ID profile
lenses when stream fallback timers are re-armed. Direct and permalink routes
start the canonical transcript prefetch after identity/scope validation without
blocking route entry on that optional data request.

MCP projection changes are internal: policy-invisible tools avoid backend probes,
unchanged stream projections avoid schema cloning, and extension availability is
batched within one projection. Calls retain their policy and backend revalidation;
no persistent availability cache or longer authorization staleness is introduced.
Native tool IDs, HTTP/UDS/CLI wire contracts, extension RPC contracts, hooks,
configuration and official skill behavior remain unchanged. Two paragraphs in
`skills/compozy/references/native-tools.md` are shortened without removing their
contracts so the startup manual fits its existing byte budget. No site guide,
compatibility adapter or migration is required. Generated SQL query code follows
the existing generator.

Affected QA legs are tracked in the owning workspace, live-session,
hidden-window, task catalog, attention catalog, usage/context, deep-link, Goal
lineage and tool-projection scenarios. Historical passes do
not prove this revision; the report records current checks and outstanding walks.

## Web runtime performance — 2026-10-17

Owner: branch `perf/web-performance`. Evidence and limits:
`docs/qa/reports/2026-10-17-web-performance.md`.

Web-only internal changes defer the empty-desktop composer and worktree dialog
modules, remove unconsumed attention queries, pause closed workspace-setup reads,
filter subagent cache subscriptions by their existing workspace/session key, and
reuse sparse facts from immutable transcript content. Message ordering, permission
and failure precedence, bounded paging, stream fences, stale handling, aggregate
counts, and existing Query keys remain authoritative. WeakMap entries follow the
lifetime of their source arrays; replacement/removal and workspace isolation stay
with the existing query/runtime owners.

Native tools and `compozy__*` IDs, CLI, HTTP/UDS DTOs and routes, hooks, extension
SDKs, configuration, user state, schemas, and official `skills/compozy/` are
unchanged. No compatibility adapter, migration, code generation, or site docs
change is needed. QA impact is flagged in `RT-047`,
`RT-visible-session-streaming`, `ET-hidden-window-live-resources`,
`RT-web-attention-title-count`, `LP-web-bell-loop-requests`,
`MS-web-workspace-add-directory-browser`, and `ET-web-catalog-navigation`.
Focused canonical suites and differential probes provide local slice evidence;
heavy final validation is CI-owned by explicit user instruction. Deferred-feature download failures are isolated by the existing window error boundary with local recovery surfaces; the worktree dialog remains dismissible and an explicit Reload retries module loading.

## Profile inheritance and global profile selection — 2026-10-18

Issue #792. `compozy__session_create` now binds the calling session's profile instead of falling
back to `default` (spawn, delegate and the in-session CLI already inherited it); no profile input was
added, so agent-native calls keep deriving their immutable session profile. Profile selection is one
global choice. User state (SD-013 regime 1): append-only migration 00140 seeds an empty Global slot
from the most recently remembered workspace slot and keeps every workspace row. Public surfaces
(regime 2, removal in v0.5.0): the CLI still resolves a workspace's legacy slot inside that workspace
with a stderr deprecation warning, and `PUT /api/profiles/selection` with `scope: "workspace"` returns
a `Warning` header; route and DTO shapes are unchanged. Writing the Global choice (`profile use`,
`profile create`, the Web switcher) clears the legacy slots, so an explicit new choice converges every
surface. Web (regime 3) drops the per-workspace lens: the switcher keeps its selection across
workspace and breadth changes, All profiles is kept by the browser across reloads, and creation under
All profiles lands in the remembered profile. Settings' "Where each profile is active" map is removed;
it only rendered `GET /api/profiles/selection`, which is unchanged. Hooks, extension SDKs and config
keys are unchanged. Scoped reads still return not found across profiles. Official skill
(`profiles.md`, `configuration.md`; `native-tools.md` is untouched to stay inside the startup budget), site profile pages, the release note, QA
scenarios `ET-profile-switcher-restore` and `ET-profile-selection-precedence`, and E2E-013 record the
contract. The skills-disabled startup-manual budget overflow is fixed by #793, which this PR depends
on.

## Profile-bound session actions — 2026-10-17

Owner: issue #783, branch `fix/issue-783-profile-session-404`.
Evidence and route-by-route audit: `docs/qa/reports/2026-10-17-issue-783-profile-sessions.md`.
Web lifecycle mutations carry the selected row's profile and workspace; rewind
and subagent cancellation refresh the current owner name through the canonical session detail query. Roster reads use the
existing authorized aggregate read scope, and terminal 404/410 control polls stop. Failed
cold-page walks require explicit retry. Existing workspace and profile authorization remains
unchanged. OpenAPI additively documents already-supported profile selectors for archive,
unarchive, rename, rewind, and subagent cancel; generated clients co-ship. HTTP/UDS paths and DTOs,
CLI verbs, native `compozy__session_*`/subagent tool IDs, extensions, hooks, configuration,
SQLite state, and workspace files retain their contracts; no migration or shim is needed.
Official `skills/compozy/` already describes explicit owner selection and needs no change. Site guides need no
behavioral rewrite. The session row, archive, profile-owner, and subagent QA scenarios cover
the affected interactions; other profile surfaces are outside this focused change.

## One-click product update — 2026-10-12

The menubar update indicator becomes an "Update" pill that applies every eligible
target through the existing `POST /api/settings/update/apply` in one click, holds a
spinner while the operation runs, and opens Settings only for a failed update or a
package-manager install. Settings → General → Updates folds the runtime and app
tracks into one "CompozyOS" row with one version and one Update action; the
runtime/app split no longer reaches the UI. A browser tab reloads itself once the
daemon settles on a new version; the desktop app keeps relaunching through the
existing installer handoff. Web-only (internal regime): HTTP/UDS DTOs, CLI
`compozy update`, native tools, hooks, extensibility, configuration keys, persisted
state, and official `skills/compozy/` are unchanged; no migration or compatibility
adapter is needed. Workspace/profile isolation is unaffected (host-global update
read). QA scenarios `APP-web-update-indicator` and `APP-web-update-two-track` record
the new contract. No site documentation changes are needed.

## Scheduled claim contention budget — 2026-10-11

Scheduled claims use a one-second default attempt budget, configurable through the
internal `WithSchedulerClaimTimeout` option, and observe scheduler cancellation.
Failed claims return control to the existing exponential backoff without advancing
the durable cursor. Durable finalization retains its cancellation-independent budget.
The existing real-SQLite `TestSchedulerIntegrationSQLiteContention` owns recovery
and exactly-once fire identity; QA scenario `TA-automation-crud-loop-target` records
the reproduction and verification. Native tools, HTTP/UDS/CLI, Web, extensibility,
hooks, configuration keys, and official `skills/compozy/` contracts are unchanged.
Workspace/profile isolation and persisted schemas are unchanged; no migration or
public compatibility adapter is needed. No site documentation changes are needed.

## Session startup, Windows caching, and attention polling — 2026-10-11

Owner: branch `fix/startup-cache-notifications`; issues #726–#730.
Evidence: `docs/qa/reports/2026-10-11-startup-cache-notifications.md`.

- ACP session creation uses a bounded 90-second default; other controls retain 30 seconds.
  Explicit `WithControlTimeout` overrides, caller cancellation, retry classification, and
  process cleanup remain authoritative. No new public configuration key is required.
- Session model binding reuses a provider-live row only within a separate ten-minute
  session binding window and the resolved provider/profile/workspace execution context.
  The global five-minute discovery interval is unchanged. This internal reuse policy
  adds no public configuration key.
  Missing, expired, stale, future-dated, or unusable bindings still force discovery.
  Claude and Cursor use their existing transport selection and validation rules.
- Detached raw stdout/stderr moves to `logs/daemon-stdio.log`; structured logs stay at
  `logs/compozy.log`, preserving rotation and all existing history. Raw startup failures
  still appear in process errors. No persisted state migration or relocation is needed.
- Skill discovery captures live filesystem metadata and ignores unstable NTFS directory
  allocation sizes. Content, membership, link containment, and invalidation checks remain.
- Attention reads omit unused inbox metadata through an internal query option. Public
  inbox totals/facets, actor scope, filters, ordering, cursors, and notification IDs remain
  unchanged. SQLite evaluates the shared facet population once through a materialized CTE.

Native tools, HTTP/UDS, CLI, Web, extensions, hooks, SDKs, and official `skills/compozy/`
retain their contracts. Web benefits from cheaper polling and startup; no UI change is
needed. No schema or generated contract changes. The affected QA scenarios link the
report. Heavy local gates are explicitly deferred to CI by the user; CI also runs the
native Windows filesystem and detached rotation regressions.

## App SDK — 2026-10-12

Owner: `.compozy/tasks/app-sdk/_spec.md`; implementation recorded 2026-10-17. This is the canonical
cross-surface audit for the spec and its delivery PR.

- **Public clients and contracts:** additive `@compozy/client`, Node-only `@compozy/client/node`, and
  `sdk/go/client` clients; resource methods, named request/response types, prompt streaming, typed SSE,
  daemon discovery, and version diagnostics. `GET /api/openapi.json` serves the release contract with
  ETag/gzip over local HTTP and UDS. OpenAPI componentization preserves wire JSON; prompt metadata now
  describes the existing UI message stream. One codegen path owns the document, TS/Go clients, and
  `openapi/sdk-samples.json`. Raw WebSocket upgrade clients remain outside the SDK scope.
- **App authorization and data isolation:** additive global app-token state stores secret hashes,
  metadata, origin sets, and revocation/last-use timestamps separately from paired devices and
  profiles. The schema migration preserves existing state. The local listener requires matching
  origin-scoped bearer authentication only for foreign browser requests and an explicit operation
  allowlist; management and privileged routes remain denied. Preflight does not authenticate the
  actual call. Opaque `null` origins are denied. Gateway rejects app tokens without last-use mutation.
  Existing local tokenless callers and the same-origin Web app retain their access. A token does not
  expand a session's workspace or tool permissions. Revocation closes token streams; disconnecting a
  client alone does not cancel an admitted turn.
- **Native tools:** no new `compozy__*` IDs or catalog shapes. Agents manage app tokens through
  structured `compozy app-token create|list|update|revoke -o json` or local HTTP/UDS. Existing prompt
  cancellation tools remain the explicit runtime cancellation path.
- **Extensibility, hooks, and configuration:** extension SDK packages remain separate from external
  app clients and keep their existing API. No token mutation hook, config key, or settings editor is
  added. SDK connection options and `COMPOZY_URL`, `COMPOZY_APP_TOKEN`, `COMPOZY_HOME`, and
  `COMPOZY_BIN` govern app connection/lifecycle; private Node homes isolate spawned daemons.
- **Web and Docs:** the Web app consumes `paths` from `@compozy/client`; its duplicate generated type
  file is removed as an internal hard cut. API reference samples consume the generated manifest with
  no method-name inference; browser documentation renders allowed operations from `app_access`.
  `packages/site/content/docs/apps/` covers Node, chat proxy, browser, Go, and token management with
  observable outcomes. The official skill routes to `references/app-sdk.md`; the glossary defines
  App token. SDK READMEs and pkg.go.dev link to the same tutorials and generated reference.
- **Compatibility and release:** SDK facade/resource layout, Node subpath, Go client, app-token
  CLI/HTTP, and browser-access error codes are experimental for the shipping minor release per
  ADR-004; all affected docs/help carry the explicit label. Runtime OpenAPI and generated operation
  types retain route stability. No legacy public surface is removed or shimmed. Release publication
  co-ships the npm client with the daemon and existing SDK; notes live in
  `.release-notes/app-sdk-and-browser-tokens-1792286400.md`.
- **Verification ownership:** SDK tests own transport/discovery/stream contracts; contract/gateway
  suites own authorization; codegen check owns full operation/sample agreement. The existing site
  `docs-api-reference.test.ts` owns SDK sample projection and browser allowlist rendering. Living QA
  journeys `J-build-external-app` and `J-authorize-browser-app` cover the end-to-end app and operator
  flows. Scenario status remains `untested` until a recorded real walk; authored plans and focused
  tests are not runtime evidence. Heavy gates and broad runtime/browser suites run in CI for this
  delivery, as requested.


The pending app-token schema migration is generated as 00141 after the profile-selection seed
00140. The existing device-row preservation suite verifies the upgrade without changing historical SQL.

## SQLite performance and contention audit — 2026-10-09

Owner: branch `perf/sqlite-contention`; evidence and verification are recorded in
`docs/qa/reports/2026-10-09-sqlite-performance.md`. This work excludes the incremental
transcript fold/cache/redaction already implemented in PR #715. Shared cancellation-aware
write admission, transaction cleanup, bounded queries, no-op maintenance, connection reuse,
and additive indexes reduce contention while retaining the existing public contracts.
Database shutdown uses passive checkpoints so retained readers do not prevent close;
committed WAL state survives reopen and explicit truncation still reports contention.

Native tools and CLI/HTTP/UDS/Web session, subagent, task, Loop, automation, extension,
and observation consumers keep their IDs, DTOs, ordering, scope, and mutation semantics.
No extension SDK, hook payload, or configuration key changes. Session health transitions
still persist before publishing hooks. Workspace/profile filters and existing authorization
remain authoritative; no cross-workspace cache or storage relocation is introduced.
User SQLite state upgrades through appended generated index migrations with reopen/data
preservation coverage. Official `skills/compozy/` and site guidance need no contract rewrite.
The owning QA scenario notes link the report and distinguish backend integration evidence
from historical browser/provider verdicts. Remaining profile callback lock duration and
unbounded inventory costs are explicit audit limits, not claimed as solved.

## Agent collaboration — 2026-10-09

Owner: spec `.compozy/tasks/agent-collaboration/` (ADR-001..004, §Compozy Cross-Surface Impact
Audit); branch `agent-collaboration`, integrated from the parallel task branches. Additive across
public surfaces (SD-013) except one recorded exception: `compozy__session_prompt` now rejects the
calling session as its target (`invalid_request`, "session_prompt cannot target the calling
session."), shipped without the one-release window per Business Rule 18 / ADR-002, with replacement
guidance in the release note. No delete targets. Native tools: `compozy__subagent_delegate` and
`compozy__subagent_status` gain `isolation`, `base_ref`, and the `worktree` facts object;
`compozy__session_prompt` stamps the sender from scope (no input override), adds
`notify_on_complete` and the `reply_watch` result, the self-target rejection, `message_hop_limit`
(8 hops), and `input_agent_authored` for edit/promote of a queued session message;
`compozy__session_status` gains `reply_watches`; no new tool IDs. Extensibility/hooks/config:
`spawn.pre_create` gains read-only `subagent.isolation` and `subagent.worktree_id` (a rebind patch is
`capability_denied`); `subagent.settled` gains typed `isolation` and optional `worktree` facts through
its daemon bridge,
with independent async snapshot copies and regenerated Go/TypeScript SDKs; worktree lifecycle
hooks fire for isolated worktrees; no config keys (reuses `[worktrees] run_branch_namespace`,
`setup_command`, `copy_list`) and no extension RPC. Public wire: CLI `compozy session subagents show`
(Isolation/Worktree/Branch/Pull request/Observed lines), `compozy session status` ("Waiting for
replies"), `compozy session input list` (`FROM` column); HTTP/UDS DTOs add `isolation`/`worktree`,
`origin` on session-message events, inputs and queue entries, typed `metadata.synthetic` on
`session_reply` wakes, the `data-compozy-session-message` transcript part, and `reply_watches`;
OpenAPI and the generated Web client co-ship. User state: three additive Goose migrations — admission
and queue `origin_json` plus `session_prompt_reply_watches`, and the `session_subagents` isolation,
worktree, git, and PR columns; lossless, existing rows default to `isolation = 'shared'`. Workspace
data isolation: isolated worktrees belong to the delegating session's workspace and are removed only
through the existing worktree access rules and refusals; origin records the sender workspace;
`session_prompt` reach is unchanged (the cross-workspace question stays open); reply watches are read
only through the sender session's read access. Official skill: `SKILL.md` routing,
`references/native-tools.md`, `references/tasks-and-orchestration.md`, `references/worktrees.md`,
`references/runtime-operations.md`.
Web/Docs: `_uiux.md` S1–S5 against the `docs/design/opendesign/agent-collaboration/` boards
(VC-01..VC-07) — session message card, reply card, sent card, queue row, and isolated-subagent branch,
PR link, hover facts and roster line; no new `@compozy/ui` primitive. Site reference pages
`sessions/subagents.mdx` (isolation and worktree facts) and `sessions/orchestration.mdx` (session
messages and replies), the new tutorial `sessions/isolated-subagents` ("Split work into pull requests
with isolated subagents"), regenerated CLI and API references; glossary **Isolated Subagent**,
**Session Message**, **Reply Wake**; `COPY.md` §6 "Session Message Terms" plus the isolated-subagent
line in "Subagent Terms". QA: new `RT-session-message-origin`, `RT-session-message-reply`,
`RT-subagent-isolated-worktree`, `ET-web-session-message-card`; reset to untested
`RT-subagent-delegate`, `ET-web-subagent-card`, `RT-session-spawn-wake`,
`ET-web-session-transcript-calm-grammar`.

Isolation details (tasks 06–07): shared remains the default; settlement facts stay absent until
observed. Git reads use `worktree.Service`, and forge PR states normalize case and whitespace.
Recovery stops unadmitted children before safe cleanup, retries pending cleanup, and preserves
admitted, dirty or ahead worktrees. Failed setup is the exception: force-remove its checkout and
branch, including files written by setup, because the agent has not started. Bound stopped children
retain a removed catalog tombstone; unbound failed materializations are deleted. Managed delivery
holds settlement facts until its worktree fence releases. CLI show follows the approved compact
`Branch … ← base · N ahead · clean/N changed`, numbered PR and `Observed` layout; missing facts are
omitted. CLI input lists show `FROM you` or the sender title and session ID. Native schemas describe
isolation/base_ref and enumerate all six PR states. The controller owns the integrated scenario walk;
this packet validates scoped race suites and real isolated-subagent daemon integration, without QA labs.

Review remediation (2026-10-10): reply admission retries re-arm only `send_failed` reservations;
returned watch state reflects persisted state. Event lookup uses the admitted input ID and then its
consuming turn; boot recovery tolerates individual row failures and warns for armed watches older
than 24 hours. Target deletion/archive reconciles outstanding watches immediately. Sent transcript
parts add actual `delivery` (`none`, `direct`, `after_turn`, `interrupt_then_prompt`); UI synthetic
metadata contains only the agreed typed fields, with `summary` only for `session_reply`. No new
hook, configuration, native tool ID, schema shape, or workspace access policy. The official native
tool reference and orchestration site document operator notify refusal and retry behavior. QA
ownership remains the scenarios above, linked by `J-session-collaboration`; the Web sent card gates
its Steered/Interrupted chip on `delivery`.

Long-running orchestration (2026-10-10, Pedro's report of an orchestrator stopped after 40 quiet
minutes): an armed reply watch whose target is live, or a fired watch not yet delivered, is the new
`awaiting_reply` work signal, so inactivity supervision never stops a sender while its answer can
still arrive. A reply resumes a sender CompozyOS stopped (timeout/inactivity, shutdown, agent crash)
and delivers once; operator stops, archive and delete are respected. Public surface: one additive
value in the closed `WorkSignalKind` vocabulary (OpenAPI, Web client) on session status/describe;
no new tool, route, config key or migration (new indexed query only). Docs: sessions lifecycle and
orchestration, the isolated-subagents tutorial, skill runtime-operations and tasks-and-orchestration;
QA `RT-session-message-reply` step 11. The broader durable multi-session watch is the next spec
(`.compozy/tasks/session-orchestration-watch/_brief.md`).

## Explicit release asset verification — 2026-10-09

Owner: branch `fix/explicit-release-web-assets`. Manual release planning builds
the Web output before comparing it with the pinned asset module. Run
`37984738188` failed on a fresh checkout because `webAssetsCheck` requires
`web/dist/index.html`, which the workflow had never generated. The existing
WebBuild target owns generation; stale-pin and main-branch mutation guards remain.
Native tools, hooks/config, workspace isolation, persisted user data, public Web
behavior, and official `skills/compozy/` are unaffected. Validation uses the
existing release workflow contract suite and a real Web build/pin comparison.

## Release smoke teardown verdict — 2026-10-09

Owner: branch `fix/release-smoke-teardown`. The QA teardown helper classifies
processes from final liveness after signal escalation, preserving failure for
live survivors while clearing stale signal-denial classifications for exited PIDs.
Release run `37975462651`, macOS x64 smoke job `113982345844`, recorded PIDs 5097
and 5108 in both `killed` and `survivors`; application provisioning had passed.
The earlier PR failure was a separate DMG detach error during packaging.

Native tools, extensibility/hooks/config, persisted user data, Web, and the
official `skills/compozy/` surface are unaffected. Lab process isolation and
signal scope remain unchanged. The bootstrap contract documents the verdict.
Coverage belongs to the existing QA bootstrap helper smoke suite, including
real owned/outside processes and signal-denial races at the OS I/O boundary.
Packaged release smoke remains mandatory for publication.

## Subagents — 2026-10-08

Owner: spec `.compozy/tasks/subagents/` (ADR-001..005, §Compozy Cross-Surface Impact Audit); one PR
from branch `subagents`. Additive across every public surface (SD-013); no delete targets. Native
tools: four new IDs in the `sessions` toolset — `compozy__subagent_capabilities`,
`compozy__subagent_delegate`, `compozy__subagent_status` (idempotent, not read-only: reading a terminal
result acknowledges delivery), `compozy__subagent_cancel`; all require an active caller turn
(`parent_not_active`). `compozy__session_spawn` and the creator wake keep their contracts; delegated
children set `notify_creator=false`. Extensibility/hooks/config: new observe-only `subagent.settled`
hook (family `subagent`); `spawn.pre_create` payloads gain `spawn_role: "subagent"` and a `subagent`
object, and a TTL patch for the subagent role is `capability_denied`; new `[subagents]
result_max_chars` (default 60000, 1000–1000000, global → workspace overlay); no new extension RPC; MCP
sidecars and marketplace unaffected. Public wire: CLI `compozy session subagents [show|cancel]` and
`compozy session list --subagents`; HTTP/UDS list/show/cancel routes, session-list `subagents` filter
(default `include`) and `subagent_summary`; SSE `subagents_snapshot`/`subagent_updated`; transcript
`data-compozy-subagent` part and optional `parentToolCallId`; synthetic `kind`/`subagent_ids`. User
state: new `session_subagents` and `session_subagent_wakes` tables and `session_input_queue.priority`
through one Goose migration. Workspace data isolation: records are workspace-scoped like sessions;
children run in the parent's workspace and worktree (no cross-workspace delegation); CLI/HTTP reads
use session read access and inaccessible sessions return `session_not_found`. Official skill:
`SKILL.md` routing row, `references/native-tools.md` (protocol), `references/tasks-and-orchestration.md`
(subagents vs spawned workers, shared worktree), `references/runtime-operations.md` (operator
surfaces, lifecycle, restart); the creator-wake text now includes the `completed` reason and the
canceled-turn suppression that `internal/session/spawn_wake.go` already implements (same drift fixed in
site `sessions/orchestration.mdx` and `autonomy/safe-spawn.mdx`). Web/Docs: `_uiux.md` S1–S10 (card,
group, hover, divider, waiting banner, tool phrases, status line, native nesting, sidebar chip,
inspector roster) with the `HoverCard` primitive in `@compozy/ui`; site tutorial
`sessions/subagents` ("Delegate to subagents"), `[subagents]` in `configuration/config-toml`,
`subagent.settled` in `hooks/event-catalog`; generated CLI and API references regenerate from Cobra and
OpenAPI. Glossary **Subagent** (delegated, provider-native, subagent wake) and COPY.md §6 "Subagent
Terms". QA: new `RT-subagent-delegate`, `RT-subagent-restart`, `ET-web-subagent-card`,
`ET-web-native-subagent`; `ET-web-session-sidebar-threads` reset to `untested` for the sidebar filter
and chip (ADR-005).

Follow-up 2026-10-09: acpmock `native_tool_call` executes injected hosted MCP tools from the
ACP agent process and emits correlated tool/result updates; the fixture and driver docs live in
`internal/testutil/acpmock/`. Hosted subagent bindings resolve an omitted turn ID only from a
matching authoritative active run/generation and the parent activity snapshot. No public wire,
hook, config, migration, or official skill shape changes. The daemon integration reads back the
parent transcript after a real delegate call, enabling the Web subagent-card E2E fixture without
operator-side delegation or synthetic tool results.


Store remediation 2026-10-09 (fix-store): scoped roster reads reject an empty workspace and
parent, cap pages at 200, and use direct ID lookup for internal cancellation/wait paths.
Wake lookup uses a read snapshot and parent/state index; progress and pending transitions
use the existing immediate transaction boundary. Recovery excludes stopped orphans and
provider-native reservations. The unreleased 00131 migration gains a parent/state wake
index and drops the redundant child index. M2 retry persistence and policy belong to fix-core. Parent catalog upserts follow summary changes while
progress remains on the roster stream. Native tool IDs, hooks, config, HTTP/UDS shapes,
workspace isolation, official skill and site contracts are unchanged. Existing owners:
UT-056, UT-060, IT-021, IT-031, IT-032; QA scenarios RT-subagent-restart and
ET-web-session-sidebar-threads retain their current fail verdict pending the controller's
integrated re-walk. Search ancestry (m19/S9) remains unresolved because the catalog service
combines active/durable rows and recuts pages after the store query; adding contextual
ancestors requires an explicit pagination contract beyond a store-only change.


Fix round 1 (sa-fix-core): wake steering freezes its batch on acceptance, channel-less pending
injection settles with its original turn, and failed wake attempts persist across successor batches
and restart (three failures dispose delivery and log `subagent.wake_abandoned`). The controller
authorized adding `attempts` to unreleased migration 00131 in place; Atlas and sqlc are regenerated.
Native work finalizes interrupted at turn/stop/recovery boundaries without a delivery wake. Recovery
isolates row failures, preserves sent input identity, and resumes delegated work after clean shutdown.
`subagent.settled` observers use the manager-owned asynchronous lifetime; denied concurrent replays
retain `capability_denied`; parent locks retire after their last waiter. Hosted MCP derives omitted
delegation keys from parent turn plus transport request identity when no provider call ID exists.
Public DTOs, tool IDs, configuration and workspace authorization stay unchanged. ROOT descendants
retain hosted native-tool access, verified over real MCP. Web consumers receive accurate native
terminal state, catalog stream and one root attention completion. Official skill/site wording and
real-provider scenario verdicts remain under sa-qa; automated evidence is in sa-fix-core-fix1.md.
The existing RT-subagent-delegate, RT-subagent-native and RT-subagent-restart scenarios own re-walks.

### API fix round 1

Subagent payloads add `created_at` for stable Web ordering. HTTP/UDS cancellation accepts an empty
body; tool request validation preserves `invalid_request` field messages through hosted MCP.
Descriptors own idempotency and validation-error metadata. A shared hosted-name normalizer feeds
transcript cards, native tool metadata, and hook classification. Later native task descriptions update
the stored roster and card; a provider is no longer guessed in the card payload. CLI show/cancel can
resolve an ID through authorized workspace routes without a registered cwd, and runtime display omits
empty fields. Delegated speed defaults to `normal`; elapsed CLI output supports hours. Native tool
IDs, config, hook shapes, workspace authorization, persisted schema, and official skill instructions
are unchanged. Generated OpenAPI, Web client, and SDK contracts co-ship. Existing subagent QA scenarios
remain failed pending the controller's re-walk; targeted API, daemon/SQLite/ACP, transcript, CLI, and
registry suites own regression coverage.


Tools fix round 2 (sa-tools): malformed hosted MCP `tools/call` params/metadata remain SDK-owned
request errors (`-32602`) without terminating the connection. Dispatcher `maxLength` validation
is opt-in through descriptor `InputErrorCode`, including descriptor registration; existing tool
bindings retain their prior behavior. No tool IDs, hook/config contracts, workspace isolation,
persisted data, Web or official skill/site changes. Owning automated evidence: hosted proxy
transport and tools dispatcher suites; existing scenario verdicts remain unchanged.

Fix round 2 (sa-fix-core): cancellation commits canceled/disposed before returning acceptance;
the manager-owned lifecycle task stops the descendant tree independently of client cancellation.
Interrupted pending steering remains deliverable and terminal wakes release transient tracking.
Failed wakes explicitly retry after 200 ms, preserving the existing three-attempt durable cap;
explicit turn cancellation takes precedence over a simultaneous error. Clean restart preserves an
already completed admitted answer instead of adding a continuation turn. Public tool/HTTP shapes,
workspace isolation, hooks/configuration and official skill contracts are unchanged. Web observes
the persisted terminal row before physical stop finishes, consistent with 202/cancel_requested.
Owning automated journeys and verification limits: sa-fix-core-fix2.md; real-provider verdicts remain
with the QA owner. Native-event ingestion must stay inline before turn-settled reconciliation.


Core fix round 3: async `subagent.settled` execution uses the service lifetime and the hook
executor's timeout, so returning from dispatch cannot cancel its subprocess. Subagent spawn denial
returns `capability_denied` with the hook reason alone. Wake retry backoff grows by 200 ms per failed
attempt (three-attempt cap unchanged); disposal derives steer tracking from row wake IDs and retains
partially claimed batches. Native tool behavior follows the existing contract; config/hook schemas,
workspace isolation, official skills and Web shapes are unchanged. Owning tests and limits are in
`sa-fix-core-fix3.md`; RT-subagent-delegate records automated coverage without changing its QA verdict.

Tools fix round 3 (sa-tools): `compozy__subagent_capabilities` previews at most 40 models per
delegable provider, current/default first, and omits model entries for unavailable providers.
Provider options add `models_total` and `models_truncated`; delegate validation still uses the full
catalog. Native descriptor/output schema and generated catalog co-ship with skill/site guidance.
No HTTP DTO, Web, hooks/config, persistence or workspace isolation changes. UT-017 and the native
binding suite own automation; RT-subagent-delegate retains its verdict pending the real-provider re-walk.

Subagents CI follow-up: the skills-disabled startup allowance tracks the two complete bundled
manuals (72,000 characters); the existing complete-manual assertion remains unchanged. No skill
content, public tool/schema, hook/config, isolation or Web contract changes. The memory-retirement
preservation test now isolates migration 00130, keeping its full data/schema/FK checks.

## Memory removal — 2026-10-07

CI Web layout-upgrade follow-up: E2E-008 now seeds the retired Knowledge window directly through the
clientstate store while its isolated daemon is stopped, and inspects the reread raw snapshot before
boot. The existing scenario then verifies registered Session visibility, Knowledge removal and stable
geometry after reload with no console errors. This repairs only the test boundary; production
reconciliation, public contracts, hooks/config, isolation and official skill behavior are unchanged.

CI runtime parity follow-up: the existing HTTP/UDS fixture now opens the registered `session` app
rather than the `sessions` palette view ID, and expects durable harness events for retained skills
and situation augmenters. Removed workspace-knowledge and memory augmenters cannot emit those events.
BR14 registered-app reconciliation and the existing no-op persistence/revision guards remain unchanged;
the exact race-enabled UDS transport CI selector passes. No production, wire, config/hook, isolation,
official skill or Web behavior changes are introduced by this fixture correction.

Review round 3: automatic prompt recovery retains the original prepared ordinary payload independently of the constructed wire request. Each replacement runtime receives one freshly bounded historical replay plus that payload; maintenance recovery retains its literal command and deferred context. The experimental compactSession operation now declares its existing 400 ErrorPayload refusal for inactive sessions (`session_not_promptable`), co-shipping generated OpenAPI/Web contracts. Existing prompt-lifetime, compact-handler and schema suites own coverage. Native tools, hooks/extensions/config, workspace isolation, SQL and official skill instructions are unchanged; existing automatic-recovery and Compact now QA scenarios record the checks. No handwritten Web changes or browser/live-provider walkthrough is claimed.

S14 / UT-W08 marker fields: experimental `compactions[].context_after` supplies the first event-level occupancy report after that ID's first terminal boundary and before the next ID's first terminal boundary; same-turn reports count, counter-only rows and later corrections do not. Experimental `context.cleared_by` identifies the compaction and first terminal sequence only while occupancy is unknown. Existing Go queries/contextusage/API conversions, CLI human/JSON/TOON and generated OpenAPI/Web types co-ship. Per-session ledger ownership, hooks/config and SQL are unchanged; the official runtime-operations skill and existing CLI usage scenario document the additive fields. The Web owner consumes the generated fields; browser scenario walks remain with that owner.

Review round 2: accepted ordinary replay delivery clears effective pending state even when its metadata acknowledgment fails; later lifecycle persistence retries the cleared snapshot without making history deliverable again. Cached replay is re-bounded from its original pre-maintenance message cut under the current workspace derive budget and effective history-tool framing. Native Compact domain codes are co-shipped through shared safe HTTP/UDS/hosted-MCP 409 mapping and the generated public ErrorCode enum. Existing replay/maintenance, tool-route parity and hosted-MCP suites own the regressions; session isolation, hooks/config shape and SQL are unchanged. Existing runtime and native-tools skill instructions remain accurate; affected QA cases are updated, with browser/live-provider walks owned by the controller.

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
  and advances the projection generation once. Review round 2 restores completion from the original
  ordered event context, including boundaries before late routed tool results, while preserving the
  final updated sequence and completed text across repeated opens. Review round 3 preserves routing
  context predating interleaved entries and completion boundaries after intervening foreign events;
  restored entries retain their original identities without disturbing surviving or reused-tool routes.
  Review round 4 includes updates after failed historical compaction attempts, preserving the final
  identity and public transcript sequence through the eventual archival and repeated opens.
  Config retirement follows trigger array-element
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

## Windows lifecycle and desktop text menus — 2026-10-10

Owner: branch `fix/windows-desktop-issues`; issues #717–#721, #698, #699.
Native tools and CLI/HTTP/UDS shapes are unchanged. Session list/status projections
read registered workspace roots without resolving runtime resources; create/resume
still use complete resolution. No timed cache or cross-workspace state is added.
Windows update locks keep process-owner metadata in `update.lock.owner`, following
the daemon PID sidecar convention; exclusive locking stays on `update.lock`.
Rooted writes request mutation access only to their target directory while keeping
handle-relative no-follow traversal. Session deletion retains the family lease and
directory capability, closes the descendant database handle for Windows renames,
and rebinds the same database identity and owner afterward. Tombstone cleanup
closes the database before directory removal. Bound tree removal acquires a write-capable
handle for the same directory identity and opens nested directories for mutation,
so Windows can flush each deletion without weakening durability checks. SQLite schemas and user data do not
change. The icon catalog accepts LF and CRLF source checkouts.

Desktop adds native editing/link menus, spelling suggestions, dictionary additions,
and multilingual spelling settings. Optional `spellcheck.enabled` and
`spellcheck.languages` fields extend `desktop-window.json`; its decoder accepts
existing files losslessly. Electron owns dictionary storage and macOS owns native
language selection. These are shell preferences, not daemon config or extension
contracts. Extensibility/hooks and official `skills/compozy/` are unchanged because
there are no new agent operations. Web custom menus remain renderer-owned; browser
behavior is unchanged. Operator docs cover the desktop preferences. QA owners are
APP-desktop-native-edit-shortcuts, ET-profile-identity-appearance-catalogs,
RT-session-delete-owned-history, and TA-daemon-lifecycle-command-guard; native
Windows evidence and packaged desktop evidence must be recorded before delivery.



## Bundled spec-cycle portability — issue #736 (2026-10-16)

Owner: `fix/issue-736-spec-cycle-skills`. The spec-cycle 0.5.1 instruction payload
replaces missing preflight/review dependencies with a language-neutral inline
self-check and optional available reviewer. Templates use the target project's
compatibility, design, vocabulary, and component authorities. Delivery/review
instructions and rebase helpers no longer require repository-only skills or paths.
All shipped skills, references, agents and loops in spec-cycle and open-design
were audited; connectivity/tailscale and forge/github ship no such resources.

Native tool IDs and schemas, CLI/HTTP/UDS, hooks, config, Web, workspace/profile
isolation and persisted user data are unchanged. SD-013: this repairs extension
instruction content without removing a public skill or changing authored task
shapes; no migration or compatibility shim. The nine-skill catalog stays stable.
The internal create-spec/create-tasks/final-verify variants remain separate under
`scripts/postinstall.sh`; the shared execute-task copy inherits the portable edit.
Official `skills/compozy/` and site API contracts stay accurate. The site review-and-fix example mirrors the corrected shipped Loop definition.

Owning checks: embedded spec-cycle product-contract suite, real materialization
and managed-install update, dependency closure, and installed shell helper
execution in a foreign Git repository. QA owner: `ET-spec-cycle-skill-bundle`.
Heavy daemon/provider-driven walks and full gates are CI-owned by maintainer
instruction; targeted checks do not establish provider-authored spec quality.






## Session attachment image previews — issue #738 (2026-10-16)

- Web composer and transcript images open an accessible, dismissible preview inside the
  session using the existing @compozy/ui Dialog. The transcript trigger no longer links
  the Electron/browser app window to the attachment bytes route. Close, Escape and
  backdrop dismissal return focus to the initiating control, including OS-window portals.
- Native tools, CLI, HTTP/UDS attachment routes/DTOs, configuration, extensions/hooks and
  the assistant-ui attachment runtime are unchanged. Existing workspace/session-scoped
  URLs remain authoritative; no new cache, persistence or cross-workspace read is added.
- SD-013 regime: internal Web composition only, with no persisted shape change, migration,
  public compatibility shim or removal window. Official skills/compozy/ and packages/site
  contracts need no update. QA owner: ET-session-attachment-paste-reload; the existing
  SessionThread suite owns preview behavior. Heavy delivery gates run only in CI by
  maintainer instruction; rendered evidence uses isolated Storybook, not a daemon lab.


Issue #738 follow-up: non-image attachment cards request a filename-preserving download,
with a separate browsing context as a fallback. The Electron navigation-policy owner now
rejects same-origin `/api` documents (including scoped attachment bytes and encoded paths)
while retaining SPA routes and the existing external-link policy. This is an internal
renderer/shell fix; native tool/HTTP/UDS/config/hook/skill contracts and user state do not
change. The existing desktop navigation-policy suite owns the additional guard; the
browser walk verifies TXT/PDF downloads without unloading the session.


## Issue #751 — subagent drill-in E2E readiness (2026-10-16)

Owner: `fix/issue-751-e2e-004-main`. Main CI run `38359736994` clicked the visible
queued card container before its roster supplied the child session to the UI;
the Playwright trace shows a static `role="group"` at `call@6196`, not an open
button. E2E-004 now targets the accessible `Open <title>` button for ordinary
and modifier clicks, waiting for the rendered navigation affordance. The
child-window, origin-divider, composer, return-to-parent and second-window
assertions remain intact. Owning suite: `web/e2e/__tests__/subagents.spec.ts`;
QA scenario: `ET-web-subagent-card` drill-in (E2E-004).

Product rule: a card without a confirmed child session does not navigate. The
existing UT-W16 wiring case now starts with a queued delegated card without a
child, confirms that clicking cannot navigate, then updates the live roster
cache and verifies ordinary/modifier navigation without reload or remount.
The existing live-tail suite owns `subagent_updated` delivery into that cache.

This changes test synchronization only. Native tools, CLI, HTTP/UDS, config,
extensions/hooks, workspace/profile isolation, persisted user state, Web
behavior, official `skills/compozy/` and site docs are unchanged. SD-013:
internal test code; no compatibility shim or migration. Heavy validation is
delegated to PR CI per maintainer instruction; no local daemon lab is started.

## Issue #747 — Goal origin identity after runtime binding — 2026-10-16

Owner: `fix/issue-747-goal-origin-profile`.

Goal admission now validates the immutable creation witness independently of the
session's mutable runtime provider. Logical creation records the configured
provider; the first prompt may bind another provider, and later prompts may replace
it. Those supported transitions must not invalidate the unchanged creation witness.
Creation-profile reference, policy digest, workspace, agent, active-session state,
caller lineage and store-transaction creation-digest checks remain enforced.

CLI `session goal`, HTTP/UDS Goal commands and native `compozy__goal_control` share
this correction. Their schemas, status codes and existing
`goal_origin_profile_unavailable` reason remain compatible. Failure logs add a
fixed `check` label plus workspace/session correlation IDs; they contain no profile
payload, provider command, prompt overlay, credentials or secrets. No SQLite shape,
config, user data, extension/hook contract or workspace ownership changes. No
migration or compatibility shim is needed under SD-013. Web benefits through the
same service without renderer changes. Official `skills/compozy/` already documents
immutable creation identity and per-run runtime selection; no skill or site API
reference update is needed. QA owner: `docs/qa/scenarios/GL-001.md`.

Validation uses the existing daemon Goal command lifecycle suite with real SQLite,
including cross-provider set/replace, invalid-provider rejection without Run
creation and immutable identity readback. The existing Session Manager lifecycle
suite also verifies logical creation with the agent default, initial binding to
another provider, process replacement, persisted witness retention and rejection
of an unresolved provider without changing the accepted process. Existing
managed-runtime borrowed-origin integration coverage, full gates and broad
end-to-end runs are delegated to PR CI per maintainer instruction.

## Issue #737 — Tailscale HTTPS browser origin forwarding (2026-10-16)

Owner: `fix/issue-737-tailscale-origin`, PR #743. The bundled provider uses an HTTP reverse
proxy after TLS termination, replaces client forwarding headers with its HTTPS scheme and
original Host, and rejects authorities other than its advertised endpoint. The private
verification relay remains TCP with the existing end-to-end TLS/challenge proof. Forwarded
schemes are trusted only by gateway-tier HTTP listeners from actual IP loopback peers;
foreign origins and mismatched ports remain forbidden. Device pairing/authentication remains
mandatory. No CLI verb/output, HTTP/UDS route/DTO, native tool ID, SDK/provider wire shape,
hook or config key changes. Workspace/profile ownership, stored data and credentials retain
their existing boundaries; no migration, compatibility alias or deprecation is required.

The existing Web pairing and module requests now pass same-origin checks; no React or visual
surface changes. Official `skills/compozy/` commands and gateway guidance remain accurate.
Site Tailscale docs explain the advertised URL and pairing response. QA owner:
`RT-connectivity-provider-route`, with focused socket/TLS/SQLite evidence and external-tailnet
limits in `docs/qa/reports/2026-10-16-issue-737-tailscale-origin.md`.

Provider health degrades only for listener/upstream dial failures; individual request errors
and client cancellation do not withdraw the route. Long-poll responses allow the 30-minute SessionWait maximum plus a one-minute header
settlement margin, while client writes retain renewed inactivity deadlines, including hijacked
WebSockets. Existing provider and HTTP middleware/pairing suites own these invariants.

## Issue #739 — Loop action claim ownership and recovery

The daemon retries SQLite claim persistence before executing an action. The task store applies
its existing daemon-owner settlement guard to lease release and block-and-release, preventing a
bound worker from clearing the claim while its provider continues. Force failure commits the
Loop node failure and canonical failure event with the task run, then publishes the terminal hook
that wakes its coordinator.
Terminal hooks recover profile identity from the owning task; sessionless mechanical/coordinator
runs have no terminal-session resource to release.

CLI and HTTP/UDS keep their verbs, routes and DTOs. Native `compozy__task_run_release` and the
session-bound block path now enforce the existing Loop ownership contract; ordinary agent-owned
leases still release normally. Extension terminal hooks retain their shapes and now receive the
owning profile. No config, schema, migration, generated contract, workspace boundary or Web
renderer changes. SD-013: lossless user state, stable public shapes and an authorization bug fix;
no compatibility shim or removal release is needed. Official skill:
`skills/compozy/references/loops.md`; public docs:
`packages/site/content/docs/autonomy/task-runs-and-leases.mdx`. QA owner:
`TA-loop-worker-exclusive-dispatch`; targeted real-SQLite evidence is recorded in the issue report.

## Issue #746 — Stable session-list sorting

- Owner: `fix/issue-746-session-sort-stable`. Shell sort adds `created` and `name`, defaulting only
  unset preferences to `created`. Explicit persisted activity/attention choices are retained; no
  state shape changes, migration, deletion, or compatibility shim. SD-013 regimes 1/2 preserved.
- HTTP/UDS `/api/sessions`, CLI `session list --sort`, and native `compozy__session_list` add `name`;
  existing `created` and all existing public defaults/orders remain unchanged. Name is visible-title
  Unicode lowercase ascending, creation time descending, ID descending. Opaque cursors retain version
  2 for existing sorts and add an optional name anchor bound to the query fingerprint.
- Settings shell DTO/OpenAPI/generated clients add both values; config validation/defaults and
  global-only persistence remain daemon-owned. No extensions, hooks, credentials, or isolation rules
  change. Filtering/profile/workspace scope still precedes ordering and the page cut.
- Web menu exposes four choices. The hierarchy preserves catalog order for nested children; no
  client sorting or activity-based fallback is added. All-workspace groups request the same sort.
- Official `skills/compozy/references/runtime-operations.md`, site config/CLI docs, and
  `RT-web-session-attention-sort` QA scenario co-ship. Targeted tests cover config overlays, real
  SQLite + active overlay pagination, Unicode/ties/search, menu selection and nested row stability.
- The first automatic title after a session's first turn is a rename boundary: Name may reposition
  the row once, whereas Created ignores title changes. Pointer overlays distinguish an absent sort
  from an explicit persisted value; serialized old defaults are preserved as explicit values.
- Heavy gates/whole-app QA are CI-owned by explicit maintainer instruction for this assignment.

## Provider catalog refresh failures — issue #740

The shared source-status projection adds optional `failure_reason` (`auth_required`,
`timeout`, or `refresh_failed`) for failed sources. CLI JSON/JSONL/TOON, HTTP/UDS,
native `compozy__provider_models_status`/`refresh`, and extension Host API reuse
this projection; generated OpenAPI and Go/TypeScript/Web contracts co-ship.
Existing `last_error` diagnostics and HTTP statuses remain compatible (SD-013
regime 2, additive). Human CLI and Web use calm guidance instead of subprocess
text. The session selector attributes discovery failures to their provider while
keeping other providers and cached/curated models usable. Settings reports the
same reason. No SQLite/config migration, hook, extension permission, or cache-scope
change; existing profile/workspace execution boundaries remain authoritative.
Official runtime skill references and the site model-catalog guide explain status
classification. QA: `RT-model-catalog-cold-open`; rendered Storybook evidence and
targeted suites are recorded in the issue PR. Heavy gates are CI-owned.
## Issue #750 — Queued inputs before prepared Goal prompts

- **Owner:** issue #750, branch `fix/issue-750-goal-queue-starvation`.
- **Cause and policy:** a Goal can prepare a durable prompt during an existing turn. Its older
  enqueue time allowed it to win FIFO over later admitted input at that turn's boundary. The unified
  selector now drains eligible public inputs before a prepared Goal, preserving interrupt,
  priority and FIFO ordering within a finite public batch. The first claim durably freezes its
  maximum admitted rowid; later arrivals follow one Goal prompt, preventing livelock. Queue mode
  never interrupts a long active turn; urgent guidance uses fenced steer or interrupt.
- **Native tools / CLI / HTTP / UDS:** queued `compozy__session_prompt` inputs, operator prompts and
  `prompt-reply:` wakes receive the same ordering. Existing queue/status positions and counts remain
  public-only. No IDs, routes, DTOs, command grammar, error codes or configuration keys change.
- **Goal / reply ownership:** the existing managed claim, budget checks, binding/control fences,
  work-turn accounting, reply-watch admission and atomic wake handoff remain authoritative. Public
  input turns do not consume Goal work turns. The selector reads persisted queue state on resume;
  the nullable batch boundary survives restart without an in-memory fairness cursor.
- **Extensibility / hooks / config:** existing synthetic wakes share the ordering; no extension SDK,
  hook payload, permission, provider or configuration changes.
- **Isolation / compatibility:** selection stays scoped to the exact session and current input
  generation. Profile/workspace admission and origin authority are unchanged. SD-013 public behavior
  bug fix with unchanged public shapes. Append-only migration 00137 adds one nullable integer
  boundary without backfill; Atlas/sqlc own the generated artifacts. Existing queue rows and user
  data survive migration and reopen; no public compatibility shim is needed.
- **Official skill / Web / site docs:** document ordering in `skills/compozy/references/loops.md`
  and the session messaging reference, plus the site's Goal guide. Web observes the corrected queue
  dispatch through existing session events; no React changes or visual evidence required.
- **QA / validation:** update RT-session-message-origin, RT-session-message-reply and GL-015.
  The existing managed-runtime integration suite owns deterministic real-SQLite admission, reply
  wake priority/deduplication and Goal state/accounting coverage. Heavy gates, full E2E and labs are
  CI-owned by explicit maintainer instruction; targeted race checks run locally. The existing
  global queue suite covers finite batches with concurrent admissions/reopen; the global migration
  suite covers retained queue state. The reported Needs-you badge comes from loop_run quarantine
  supervision, not queue backlog, and remains outside this scheduling fix.

## Issue 742 — Session message cards for real providers — 2026-10-16

Owner: branch `fix/issue-742-session-message-cards`; issue #742 (regression of #722).
`compozy__session_prompt` input, output, and errors are unchanged. Its transcript card part
(`data-compozy-session-message`, `transcript.UISessionMessagePayload`) keeps its shape and now
reads the recorded provider forms: Claude's streamed arguments and string `raw_output`, and
Codex's MCP `{server, tool, arguments}` envelope with a result that names no tool. A part is
projected only once the call names its target, so no part without `target_session_id` reaches
clients. The composer prompt stream folds each turn's events through
`transcript.SessionMessageCards`, matching the transcript entry.

User state (SD-013): the materialized transcript projection moves from version 2 to 3. Opening a
session database reprojects only the assistant entries that hold a `compozy__session_prompt` call,
following the #670 whitespace upgrade; the event ledger is unchanged and remains authoritative. The
upgrade runs once per session database, in one write transaction that records the version last, so
a rerun or an interrupted run reprojects the same entries from the ledger without duplicating or
dropping a card. As with #670, an older runtime treats a version 3 projection as incompatible
(ahead failure, explicit).

HTTP: every listener that serves the Web (local, private tier, public operator) sends
`Cache-Control: no-cache` for the SPA shell and unversioned public files, and
`public, max-age=31536000, immutable` for Vite-fingerprinted `assets/` files. Routes, DTOs, OpenAPI, CLI, config, hooks,
extension contracts, and native tool descriptors are unchanged, so the official
`skills/compozy/` and `packages/site` docs need no update. Desktop loads the shell with
`pragma: no-cache`, so a shell cached by an older runtime is skipped after an update. Web never
draws a raw data row for a session-message part it cannot name, and a call split across two
assistant messages by a queued prompt draws one card. Workspace isolation is unchanged: the
upgrade runs per session database. QA owners: ET-web-session-message-card and
APP-desktop-runtime-bundle-coherence.

## Issue #756 — daemon-owned verification session visibility

- Native tools, CLI, HTTP/UDS: public session catalogs omit `system` sessions whose immutable creation policy has `runtime_mode=verdict-only` when no type is selected; existing explicit `type=system` / `--type system` retains audit access. The filter applies to active overlays, durable pages, totals and facets before pagination. Direct ID/history reads remain available.
- Goals: judge creation carries the bound Goal session as informational provenance; ordinary Loop gates use their origin. No spawn governance is granted. The Goal disclosure provides an explicit, paginated verification-session audit with transcript links, coordinated by a session hook and refreshed through the existing catalog stream rather than fixed polling.
- Attention: verdict-only system sessions do not emit operator attention catalog events or contribute to default attention counts/bell candidates. Durable lifecycle events, hooks and owning work outcomes remain authoritative.
- Shutdown: completed/owner-released idle sessions ask the driver to close the process directly and verify exit; active turns and failed termination retain cancellation/escalation behavior.
- SD-013: no persisted shape change or destructive migration. Historical verdict-only system rows are filtered from their existing creation policy at read time. Public flags, routes, DTO shapes, hooks and tool IDs remain compatible; the intentional default population correction has an existing explicit audit selector. No config, extension SDK, permission or provider routing change. Profile/workspace predicates remain server-owned.
- Web/docs/official skill: Goal audit UI, Goal/session-list QA scenarios, site Goal guide and `skills/compozy/` runtime/Loop guidance co-ship. Heavy validation is delegated to exact-head PR CI per maintainer instruction; targeted local suites and rendered evidence are recorded in the PR.


Issue #756 system-session inventory: Loop/Goal judges (`loop_runtime_adapters.go`) force verdict-only/deny-all. Managed Loop/Goal workers (`loop_managed_binding_profile.go`), task workers (`task_session_bridge.go`), scheduler role workers (`task_role_sessions.go`), task reviewers (`review_router_create.go`), automations (`automation/dispatch_context.go`) and extension-created sessions (`extension/host_api_sessions.go`) also use system type and may require operator action; they remain visible. Heartbeat reuses its bound session; coordinator/spawned sessions use separate types; auto-title is already excluded by spawn role. Visibility therefore uses existing immutable `runtime_mode=verdict-only`, not type alone or a name heuristic. It is derived in bounded catalog projections; no stored shape changes. Judge lineage has no subagent spawn role, and the viewed-session path must not reveal it as a child/chip.

## Session row provider mark — 2026-10-16

Owner: branch `feat/issue-754-session-provider-logo`; issue #754.
Every session row in the shared Web list (window sidebar, sessions modal, nested
children, revealed subagents, all-workspaces groups) leads its sub line with the
provider mark of `runtime.effective.provider`, drawn from `providerKindIconRegistry`
in neutral ink. The row's accessible name and the mark's tooltip read `Provider · Model`
in the runtime selector's display names, from the same cached `/api/providers` and
`/api/model-catalog/models` reads (wire id where the catalog has none); the glyph is
not a tab stop and the tooltip also opens on the row's keyboard focus. A session that
has never bound a runtime renders no mark. The sub line now runs under the time
column, so the mark costs no attention word. Data comes from the existing list
payload and the catalog stream's `upserted` re-read, so continue, fork, and process
replacement repaint the mark with no per-row request. Web-only (internal regime):
HTTP/UDS DTOs, OpenAPI, CLI, native tools, hooks, extensibility, configuration keys,
persisted state, and official `skills/compozy/` are unchanged; no migration or
compatibility adapter is needed. Workspace/profile isolation is unaffected (the row
renders its own payload). No site documentation changes are needed. QA owner:
`ET-web-session-sidebar-threads`; automated owner: the `SessionList provider marks`
suite in `web/src/systems/session/components/__tests__/session-list.test.tsx`.


## Post-merge runtime selector repair — 2026-10-16

Owner: `fix/main-ci-after-merges`; main CI run 38936400681; originating PR #745.
The Web runtime selector keeps provider discovery guidance in a bounded scroll region
so multiple failures cannot collapse the model list or intercept model clicks. All
provider guidance and keyboard/model/reasoning behavior remain available. Sidebar
Storybook catalog fixtures move unchanged into a helper to satisfy the source-size
limit after #758/#759. Internal Web-only changes: no native tool, CLI, HTTP/UDS,
hook/extensibility/configuration, persisted-state, workspace/profile-isolation,
official skill, or site documentation contract changes. QA owner:
`RT-model-catalog-cold-open`; regression owner: the rendered `ProviderAuthRequired`
story plus the existing session provider override E2E journeys.


## Session-list connection warnings — issue #767 — 2026-10-16

Owner: branch `fix/issue-767-false-disconnected-banner`; issue #767.
The window sidebar used the composer transport guard, which immediately reports reconnecting
(`waiting-reconnect`, or `connecting` with a retained live timestamp) as disconnected. It now
reuses the existing 2-second transport-feedback grace. The sessions modal's catalog owner waits
10 seconds across reconnect attempts before reporting stale, clears on open/valid events, and
cancels its timer on teardown. Disabled/connecting/loading states are not connection failures.
Superseded catalog reads keep TanStack cancellation semantics; actual query failures stay visible.

- Web: sidebar and sessions modal only; no new polling, query, dependency, or retry path.
- Native tools, CLI, HTTP/UDS DTOs, hooks, extensions, config and official `skills/compozy/`: unchanged.
- Workspace/profile isolation: existing scoped keys, stream URLs and generation fences remain authoritative.
- User state/migrations: none. Public compatibility: unchanged. Site docs: no public contract change.
- QA: `ET-web-session-sidebar-threads` and `ET-web-sessions-catalog-modal`; targeted hook regressions,
  real local frontend walkthrough and rendered browser fault-injection evidence. Heavy gates run only in CI.

## Issue #766 — background native subagents and the composer subagents control

Owner: branch `fix/issue-766-subagent-status-indicator`; issue #766.

- Runtime: a Claude Agent/Task call that runs in the background returns a launch receipt at once.
  The ACP layer lifts claude-agent-acp's PostToolUse marker (`_meta.claudeCode.toolResponse`
  `status: "async_launched"` / `isAsync: true`) and stamps it on that tool result; the session
  layer also reads the launch input `run_in_background: true`. The receipt no longer settles the
  provider-native row: it stays `running`/`working`. claude-agent-acp holds the prompt response
  until every background subagent of the turn has settled and reports each outcome only to the
  model, so the turn's non-canceled `done` completes those rows with `result: null`. Native tool
  events are now ingested on the persistence path (when the event is recorded) instead of the
  prompt-delivery pump, so ingestion and the `done` handling run on the goroutine that settles the
  turn, in ledger order, whatever a slow prompt consumer does. Canceled turns, parent stops and boot
  recovery still settle them `interrupted`. Foreground calls keep settling on their result.
  Codex's own collab subagents are still not recorded as provider-native rows (unchanged).
- Native tools, CLI, HTTP/UDS: `compozy__subagent_status`, `compozy session subagents`, and the
  subagent routes keep their shapes; background provider-native rows now read `running` until the
  turn ends and then `completed` without a result. The `subagent.settled` hook fires at that later
  point with the same payload shape. Session events gain an optional `provider_tool_async` field on
  the launch result (additive, omitted otherwise).
- Persisted state (SD-013): no SQLite schema change or migration. Rows written before this change
  that settled on a launch receipt keep their recorded status and result; they are settled history
  and are not rewritten from prose. The event-ledger field is additive and older readers ignore it.
- Web: the composer toolbar adds `SessionComposerSubagents` after the context ring and before the
  attach button — a quiet icon while nothing runs, the subagent avatar's live dot plus the live
  count while subagents work — and a `@compozy/ui` Popover that hosts the inspector's roster rows
  (`SubagentRoster`, extracted from the inspector section and shared). The inspector section stays:
  the inspector is closed by default, and the sidebar chip and transcript group "and N more" land on
  it. The roster hook gains `coldRead` so an always-mounted host reads only the stream roster until
  the session reports subagents or the popover opens.
- Workspace/profile isolation: unchanged; rows stay scoped to their parent session and workspace.
- Official skill and docs: `skills/compozy/references/native-tools.md`, the site subagents guide,
  `COPY.md` Subagent Terms, and QA scenarios `ET-web-native-subagent` and `ET-web-subagent-card`
  co-ship. Automated owners: `TestClaudeAgentSubagentContract`, `TestNativeSubagentEventFromStored`,
  `TestSubagentNativeLifecycle`, IT-020 `TestSubagentNativeDaemonIntegration`, and the
  `SubagentComposerControl`, composer-placement, and composer-control wiring web tests.

## Daemon idle cost and Goal liveness — issues #762, #763, #764, #765 (2026-10-16)

Owner: branch `daemon-cpu-attention`.
- #763 Goal liveness: the Goal executor reports its bound session through the new liveness-only
  action port (`loop.ReportActionLivenessSession`). The action heartbeat uses that session's activity
  as evidence, so a working Goal no longer raises `silence` or a false session `needs-attention`. It
  binds no `task_runs.session_id` and adds no confirmed-death resume for Goal segments. Persisted
  false `silence` flags clear on the next evidence through the existing path. For every action
  node, evidence now requires session activity newer than the last observation or an in-flight
  tool, and a detected stall removes it. A session that stays alive without new activity therefore
  still reaches its silence window instead of renewing liveness from a stale timestamp.
- #762 hosted MCP: the projection stream (`/internal/hosted-mcp/projection/stream`, UDS) calls
  `HostedService.StreamProjection`. Without an authoritative generation (always today, because native
  availability carries none), it reuses the bind's last projection for at most 1s unless the daemon
  projection epoch advanced (dispatch, catalog, settings). One-shot `Projection` reads stay fresh. The
  wire payload, digest and SSE events are unchanged. Native tool availability flips reach agents
  within about 1s instead of 100ms.
- #764 usage reads: `session.Manager.Compactions` reads only compaction, compaction-fired, usage and
  done rows instead of the whole history, and returns early when a session has no compaction marker.
  This lowers the cost of `/usage` and `/usage/turns` for large sessions. Markers are read after the
  usage rows, so a compaction appended between reads cannot lend its usage to an earlier one. The attention bell's
  per-workspace inbox fan-out is unchanged and stays open on #764.
- #765 session WAL: the writer keeps its PASSIVE checkpoint every 128 writes. Once it has been idle
  for 1s after writes, it checkpoints and truncates a WAL above 16MB, letting readers delay that
  idle-only truncation by at most 200ms, and retries a deferred truncation with 1s–30s idle-gated
  backoff. Truncation never runs inside a write burst, so it adds no writer stall there.
  `store.CheckpointTruncateConnection` exposes the existing busy-aware TRUNCATE on an acquired
  connection.
- SD-013: no persisted shape, migration, config key, CLI, HTTP/UDS DTO, hook, extension SDK or
  native tool change. Workspace/profile isolation is unchanged. Official `skills/compozy/` and
  `packages/site` need no update; behavior only becomes accurate (no false attention) and cheaper.
  QA owner: `TA-action-run-liveness`.


## Terminal background progress — issue #774 — 2026-10-17

Owner: branch `fix/issue-774-terminal-workspace-stall`.

- Runtime: PTY output is always drained independently of viewer ACKs. Existing bounded ACK/drop
  queues, demotion, GAP, snapshot and replay remain. Removed internal Group wait coordination.
  Preserve ATTACHED before overflow GAPs, and emit replay demotion events without reacquiring
  the session lock.
- Web: writable and read-only browser attachments use the existing drop flow; hidden, suspended,
  minimized and stale renderers cannot pause the process. Input rights and resize votes remain
  controlled by attach mode. Obsolete browser ACK bookkeeping is removed with all consumers.
  Render and snapshot-tail buffers are bounded at 1 MiB or 1,024 frames; overflow requests a current snapshot
  without throttling the PTY. Input stays gated through recovery, including pending replay.
  Repeated server queue trims merge undelivered GAP ranges and byte counts.
- Native tools/CLI/HTTP/UDS: `terminal_read` and `terminal_wait` observe background progress. Existing
  `flow=ack` clients remain accepted with bounded queues; no request/frame/DTO shape changes.
  Canceled terminal requests are classified as 499 using the existing `api_error` envelope.
- Hooks/extensibility/config: no new hook, SDK shape, setting, permission, dependency or timer.
- Compatibility/user state: no persisted layout, workspace-file or SQLite changes, no migration.
  Profile/workspace authorization and retained history bounds remain authoritative.
- Official `skills/compozy/`: existing detach/closed-browser process-lifetime guidance remains true;
  no native invocation contract changes. Site documentation needs no changed command or shape.
- QA: `ET-terminal-stream-resilience` owns workspace-switch, hidden-view and recovery checks.
  Evidence is tracked in `docs/qa/reports/2026-10-17-issue-774-terminal-workspace-stall.md`.

## Issue #782 — hung-turn watchdog and silent idle runtime release (2026-10-17)

Owner: branch `fix/silent-idle-session-reclaim`; issue #782.

- Runtime: `session.supervision.quiet_after`/`stop_grace` become a hung-turn watchdog. Silence
  accrues only while a turn is in flight and waiting on the agent; an idle session (no turn in
  flight) or a turn blocked on a pending permission/clarify request never warns or stops for
  inactivity, and a quiet warning clears when the turn ends. The in-flight path is unchanged:
  `session.supervision_warning`, the stop ladder with cause `inactivity`,
  `session.supervision_stopped`, `stop_reason: "timeout"`/`stop_detail: "inactivity"`, task
  recovery, Loop death-resume and reply-wake. A new idle runtime release stops the agent process of
  an idle session with no work signal, no unknown source, no pending interaction, and ACP
  `session/load` support after `session.supervision.idle_release_after`. The session stays
  `active`, `runtime.status: "unbound"`, keeps transcript and ACP session id, and the next prompt
  from any surface rebinds through `session/load`, falling back to bounded transcript replay, or
  binds a newly selected route with the transcript. An unclean daemon exit while released boots the
  session `stopped`/`shutdown` with detail "daemon exited while the idle session runtime was
  released"; a later prompt resumes it.
- Native tools, CLI, HTTP/UDS: no new tool or route; session describe/status/resource fields keep
  their shapes. `runtime.status: "unbound"` on an active session now also means a released
  runtime, and `supervision.quiet_warning` appears only during in-flight turns.
- Extensibility/hooks/config (SD-013, additive): new key `session.supervision.idle_release_after`
  (duration, default `30m`, zero or positive, `0` disables), readable/writable through config
  get/set and `compozy__config_*`. A release dispatches no `session.post_stop` hook; the
  process-level `agent.stopped` hook fires as for any runtime replacement. Inactivity stops, and the hooks that follow them, now happen only for
  in-flight turns. No persisted schema change or migration; the released shape is the existing
  active/unbound metadata.
- Workspace/profile isolation: unchanged.
- Web: the window-level quiet warning notice ("Quiet for … This session stops in …", Stop now) is
  removed; a quiet in-flight turn adds a muted "quiet for …" segment to the working status line.
  Released sessions render as ordinary idle sessions. Historic inactivity stops keep "Stopped after
  … · no work for …".
- Official skill and docs: `skills/compozy/references/runtime-operations.md`, `native-tools.md`,
  `loops.md`, `tasks-and-orchestration.md`; site `sessions/lifecycle`, `sessions/health`,
  `sessions/events` (stale `runtime_warning`/legacy-key text corrected), `sessions/orchestration`,
  `agents/heartbeat`, and `configuration/config-toml`. QA scenarios `RT-session-native-stop`
  (supervision branch rewritten, idle-release branch added, re-walk required) and
  `RT-session-message-reply` (step 11).

## Settings "Pause idle agents after" drives idle runtime release (2026-10-17)

Owner: branch `fix/settings-idle-release-control`, stacked on #787; issue #782.

- Runtime: unchanged. The Settings → General control that wrote the unenforced
  `session.limits.timeout` now writes `session.supervision.idle_release_after` (restart-required
  under the existing `session.*` lifecycle rule).
- HTTP/UDS (SD-013 regime 2, additive + deprecate N→N+1): `SettingsGeneralConfigPayload` gains
  optional `idle_release_after` (Go duration, zero or positive; omitted on PATCH keeps the current
  value; always returned on GET). `session_timeout` keeps its required shape and its meaning
  (`session.limits.timeout`), is marked `deprecated` in OpenAPI, and a PATCH carrying a non-zero
  value returns `Warning: 299 compozy "... use idle_release_after; removed in v0.5.0"`. Delete
  target v0.5.0: the `session_timeout` DTO field, its decoder/encoder, and the Warning header.
- Config (SD-013 regime 2, deprecate N→N+1): `session.limits.timeout` stays readable/writable
  through `config get/set` and `compozy__config_*`; a non-zero value logs a load-time
  deprecation warning. Delete target v0.5.0: the key, `SessionLimitsConfig`, its overlay, CLI and
  tool-surface entries. No persisted schema change or migration.
- Native tools/CLI/hooks/extensions: no shape change. Workspace/profile isolation: unchanged.
- Web (regime 3): row label "Pause idle agents after", test id
  `settings-page-general-idle-release-input`, presets Never/15m/30m/1h/4h/1d plus the configured
  value as-is; E2E selectors and the settings transport spec follow the new field and key.
- Official skill and docs: `skills/compozy/references/configuration.md` (new Idle Session
  Settings section), `runtime-operations.md`; site `configuration/config-toml`,
  `sessions/lifecycle`. QA scenario `MS-025` (re-walk required for the renamed control).


## Issue #760 — Session messaging for long orchestrations

- **Owner:** issue #760, branch `fix/issue-760-session-messaging-delivery`.
- **Cause and policy:** a real 10-executor run showed queued agent messages never reaching long-turn
  targets, agents outranking the operator, one turn per message, a hop limit that measured chat length,
  and guidance that taught the failing paths. The queue is now operator-first with separate caps and a
  real `queue_position`; agent steer never interrupts; `supersede`, batched dispatch (up to 8) and
  promotion to steer after `queue_promote_after` apply; hops count forwarding only and a per sender→target
  rate budget replaces depth as the flood guard, and a reply wake restores the sender's own hop so
  sequential orchestration never accumulates; `in_reply_to` and a reply-watch TTL make replies explicit,
  and an explicit answer never releases the sender's reply watch (the completion wake still fires).
- **Native tools / CLI / HTTP / UDS:** `compozy__session_prompt` gains `supersede` and `in_reply_to`,
  and its descriptor text explains `mode` and `notify_on_complete` (catalog fixture updated). New typed
  error `message_rate_limited` (HTTP 429); the native tool returns the same detailed message as HTTP
  (limit, window, retry delay). `compozy session prompt` gains `--from-session`, `--in-reply-to`,
  `--supersede`; origin stamping is opt-in, not automatic (`--from-session auto` or an id; inside a
  managed session without the flag it prints a stderr hint and stays operator input) so existing
  scripts keep interrupt, slash commands and operator priority. `--in-reply-to` and `--supersede`
  without a sending session are refused (CLI error; HTTP 400 and native tool `invalid_request`). Identity is a local trusted
  session-ID header: no spoof protection beyond local operator trust. Operator promote of agent rows is
  allowed (edit still `input_agent_authored`). OpenAPI and generated web types co-ship. All additive.
- **Extensibility / hooks / config:** new `[session.messaging]` keys `rate_limit_messages`,
  `rate_limit_window`, `reply_watch_ttl` (default 24h, `0s` never expires) and
  `queue_promote_after`; no hook, extension SDK or permission change; origin gains `in_reply_to`.
- **Isolation / compatibility:** all reads and rate counts are scoped to the exact sender and target
  session; no cross-workspace change. Append-only migration 00138 (with the Atlas/sqlc
  artifacts) widens the reply-watch outcome check (adds `expired`) and the session-input queue schema
  (`origin_class`/`origin_session_id` with lossless backfill, per-sender steer index), copying `rowid`
  so Goal batch cutoffs keep their meaning; existing rows survive. Boot recovery also returns steers
  staged before a restart to delivery (agent steers to the queue, operator steers dispatchable). SD-013: public additions only, behavior fixes inside the
  experimental-free messaging contract; no shim.
- **Official skill / Web / site docs:** `native-tools.md`, `runtime-operations.md` and
  `tasks-and-orchestration.md` carry the queue-vs-steer guidance and supervisor/worker protocol; the
  `cy-orquestrate-issue` skill and executor prompt use the native tool with an orchestrator ID and a
  milestone-only reporting protocol. Web adds the In reply chip, No reply outcome, grouped inbound
  cards, Steer on agent queue rows and attention reasons; site orchestration, config and CLI prompt
  pages and COPY.md §Session Message Terms updated.
- **Review remediation:** promotion runs `input.pre_submit` and only injects messages whose runtime
  matches the live turn; the admission fingerprint includes `supersede` and `in_reply_to` (omitted when
  unset, so older receipts replay unchanged); reverting a reserved promotion reopens its admission;
  Web reply links are keyed by peer session plus message id.
- **QA / validation:** RT-session-message-origin (steps 9–16), RT-session-message-reply (12–13) and
  ET-web-session-message-card.md updated; deterministic regressions run on real SQLite in the owning suites. Heavy gates are CI-owned.


## 2026-10-18 merged-PR review follow-through

- Native tools, extensibility, and hooks: no new ID, DTO, permission, or hook.
  ACP terminal commands use the existing actor ownership to receive the configured
  agent niceness. Human and system terminals retain inherited process priority.
- Config: existing `daemon.load.agent_niceness` now reaches agent-owned terminal
  groups; managed Goal submissions use the existing background admission gate.
  A cancelled admission neither begins durable submission nor calls the driver;
  failed admission retains queued work and reselects with the existing retry path.
- Runtime state and isolation: no schema change. Goal event and cleanup relays
  drain healthy full batches immediately, stopping on failure until another wake.
  Workspace/profile ownership and durable acknowledgement rules are unchanged.
- CLI: ACP consent timeout no longer expires a post-login status probe. Provider
  tests retain exact platform-specific executable checks. The promoted-field
  literal flagged by Greptile compiles with the repository's current Go toolchain;
  no behavioral fix is needed for that language-level finding.
- Official skill and Web/Docs: configuration reference documents agent terminal
  priority, managed Goal admission, full-batch drain, and the 2s fallback sweep.
  The source provider-removal release note now lists `agy` and 15 current providers;
  generated release documents will be refreshed by the release workflow. No Web
  control or generated API contract changes in this follow-through.
- QA: `MS-daemon-host-load-governance`, `MS-background-role-fallback`, and
  `RT-agy-acp-provider` carry the added boundaries. Focused real-process and
  canonical lifecycle suites provide slice evidence; the broader host saturation
  journey is not claimed as complete.


CI follow-through 2026-10-18: the release full-integration runners install ripgrep
through the same Ubuntu-only setup used by preflight, so the existing product
language guard runs instead of exiting 127. Desktop E2E uses the Go harness's
20000–29999 port range and per-process reservations instead of released client
ephemeral ports. No production API, CLI, config, or timeout changes; the existing
cold-start/deep-link scenario owns end-to-end verification.

SDK review follow-through 2026-10-18: Go generation preserves all distinct successful response
shapes and exposes their HTTP status for variant selection. Registry transport metadata chooses
an explicit HTTP URL in HTTP-only Go examples. Node close signals its retained child process
handle and waits for exit, preventing a second home lookup from stopping a replacement.
The experimental compatibility notice names this release rather than an incorrect future version.
No daemon wire schema, permission, native tool ID, configuration, migration, or workspace boundary
changes; Go/Node clients, generated samples, package docs, site docs, and the source release note
co-ship. RT-sdk-go-golden-path and RT-sdk-node-lifecycle own the added QA boundaries.

SDK codegen boundary follow-through: the registry adapter supplies domain enum wire metadata
to the leaf Go generator. Generated clients and samples remain identical; no runtime behavior,
public contract, native tool, hook, config, data-isolation, or QA journey changes. The existing
boundary check and Go codec suites own verification of this internal dependency correction.

The terminal manager priority contract lives in its Unix-tagged suite, preserving every
real-process assertion while keeping POSIX-only introspection out of Windows test compilation.

Transport-parity follow-through: browser rate-limit probes use the app-access identity
endpoint. The existing real HTTP/UDS credential journey explicitly retains diagnostic-route
denial, local-process privileges, origin isolation, and revocation. No production policy is relaxed;
ET-app-token-browser-access carries the previously documented status/identity distinction.


## 2026-10-18 stop escalation persistence and termination share a phase deadline

- Native session stop, CLI, HTTP, and UDS retain their existing IDs, DTOs, caller scope, and
  cancellation policy. Slow history writes no longer create a false stop failure after exit proof.
- Escalation persistence retains the existing termination-phase context. Its initial one-second
  observation wait still reserves time for the physical stop action; the receipt settles under
  that same phase deadline before finalization. Cooperative, forced, and kill budgets are unchanged.
  Real persistence failures and exhausted phase deadlines remain visible.
- Hooks, extensibility, config, and workspace/profile isolation are unchanged. Event and metadata
  ownership stay scoped to the same session; no schema or migration is needed.
- Web controls and the official skill's phase timings remain unchanged. The source release note
  explains the corrected stop/delete behavior; RT-session-native-stop owns the delayed-history
  and blocked-persistence QA boundaries. Existing lifecycle and real ACP/SQLite suites verify them.

The prompt-event observer also snapshots the mutable session type under the session lock before
adding recovery guidance. Pre-stop hook context updates can now overlap a queued turn's final
events without a data race; recovery actions, event payloads, and public contracts are unchanged.
The existing origin-delivery and lifecycle suites own concurrent stop/prompt coverage. The
daemon receipt-fence test intercepts the current idempotent append boundary, the full integration
job builds the app SDK before running the published Node quickstart, and the browser shortcut
journey awaits the reopened sidebar before checking its toggle. These validation repairs preserve
the existing hop-limit, public-package, and keyboard behavior assertions.


## 2026-10-18 palette action selection during ranking refresh

- Web: opening an action panel commits the current row selection in the keyboard/pointer event.
  A simultaneous recents or pins update can reorder the catalog without dismissing that panel.
  Untouched highlights still follow the top result, and removing the selected target still closes
  its panel. Existing palette surface tests own both separate and simultaneous updates.
- Native tools, hooks/extensions/config, persisted data, workspace/profile scope, and official
  skills have no contract change. No migrations or generated clients are affected.
- ET-palette-action-panel and ET-editable-shell-shortcuts carry the focused QA impact. The real
  Herdr E2E-016 journey verifies editable activation, the sidebar chord, and the Escape ladder.


## 2026-10-18 background WAL maintenance yields to lifecycle writes

- SessionDB passive checkpoints use the existing 200 ms WAL maintenance budget while waiting
  for database family guards. An idle checkpoint cannot monopolize the recorder's writer and
  expire a queued terminal event. A deferred idle pass retains the existing retry/backoff path;
  foreground write failures and shutdown drain errors are still returned.
- Public stop/termination deadlines, native tools, HTTP/UDS/CLI shapes, hooks/extensions/config,
  official skills, and workspace/profile ownership are unchanged. No migration is needed.
- RT-session-native-stop records the held-family-guard boundary. The canonical passive-checkpoint
  suite reproduces a starved terminal write before the fix and verifies durable history afterward;
  the existing lifecycle/delete suites verify the consuming behavior. Windows CI remains the
  platform delivery check for the reported failure.


The terminal profile E2E now waits for the existing selection API's successful persistence
before reloading the concrete terminal deep link. An optimistic switcher label cannot prove
that write completed. ET-terminal-profile-segmentation records this validation-only correction;
profile selection, input authorization, MCP keepalive, and timeout contracts are unchanged.


## 2026-10-19 runtime selector model list issues mark — issue #827

- Web: the runtime selector no longer renders catalog status or per-provider model list guidance
  inline under the provider chips. A warning mark ("Model list issues") appears in the search
  header before refresh only when problems exist; its nested popover lists the catalog-wide
  message first, then each listed provider's message in provider order. Rail filters and search
  no longer narrow the guidance. `catalogStatus` narrows from `ReactNode` to `string | null`;
  every caller already passed text except the session composer, which now passes its message.
- Native tools, CLI/HTTP/UDS shapes, hooks/extensions/config, persisted data, workspace/profile
  isolation, official skills, and site docs are unchanged. No migration or codegen is needed.
- QA owner: `RT-model-catalog-cold-open` (issue #827 impact note). Regression owners:
  `runtime-selector.test.tsx` "model list issues" (content, ordering, nested Escape/outside
  press, focus return) and the rendered `ProviderAuthRequired` story (nested dismissal plus the
  healthy-model hit target from #745/#768).


## OpenCode v1/v2 model discovery — 2026-10-18

Owner: workflow `opencode-model-discovery` (Tasks 01–03 implement; Tasks 04–05 verify live legs). Spec `_spec.md` plus ADRs 001/002 in `.compozy/tasks/opencode-model-discovery/` own version dispatch, shared invocation, and origin handling.

- Discovery probes the selected OpenCode installation with `--version` once per actual refresh (shared 10s/configured deadline, no retry/fallback, no durable version) and runs exactly one catalog method: v1 `models --verbose` through the existing parser, v2 `api --standalone GET /api/model?location[directory]=<workdir>` through a minimal typed decoder (enabled-only rows, exact `providerID/id` identity, explicit-false vs unknown tools, validated limits/variants, ignored settings/costs/location; malformed/empty generations fail wholly, never echoing raw output). Explicit discovery commands/endpoints, disabled discovery, opaque-wrapper guidance, and overlay explicit-path requirements are unchanged.
- The hashed execution fingerprint embeds the resolved absolute executable, prefix arguments, command origin, and hashed assignments (no secrets, version, or install state); a changed selector partitions rows while prior rows stay stored. Expected-fingerprint mismatch fails the refresh before execution; previous-context/stale transfer is denied only across OpenCode selectors, preserving established stale behavior for other providers.
- Installation-switch remediation (2026-10-19): the OpenCode adapter supplies an explicit write-time retention policy, including custom provider IDs with an explicit discovery command. The same policy prevents cross-invocation stale transfer and survives provider-wide refresh snapshots and staged/batch publication. The store retains the current partition plus the seven most recently refreshed siblings per source/provider/scope/profile/workspace, including empty failure partitions, and removes unreferenced execution contexts. Only cached discovery history is evicted; configured models and curation are unchanged. The existing SQLite/executable-fixture integration suite verifies successful A-to-B discovery followed by failed discovery on A: A keeps its own stale rows and last-success timestamp while B stays fresh; a new invocation cannot borrow either generation. Other adapters retain existing pruning, and profile/workspace read isolation, native tools, public surfaces, hooks/extensions/config, Web, site, and official skill contracts are unchanged; no migration is needed. QA owner: `MS-live-model-release-refresh`.
- Installed-first default: the inherited OpenCode command becomes `opencode acp`; only genuine absence of a native executable selects the existing npm prefix (`npx -y opencode-ai@latest acp`) for discovery and ACP, explicit/npm-only commands keep their selector, and a present-but-non-runnable executable reports a permission error instead of not-found. Without an explicit status command, OpenCode readiness derives from that launch identity (source `provider_command`; executable presence remains unknown auth). No new config key, schema, or credential state: only the built-in default value and its derived readiness change.
- Review remediation (2026-10-19): the settings provider read payload gains the additive read-only `command_inherited` flag and the Web editor omits an untouched inherited command on save, so a round-trip save cannot author the builtin default and the npm fallback survives (an edited or authored command still writes through, keeping explicit pinning). Windows npm shims (`opencode.cmd`/`npx.cmd`) derive for discovery, a failed OpenCode refresh keeps the prior invocation partition rows stored, discovery runs under the provider command's private `VAR=value` assignments like the session launch, and a pre-start hook command rewrite no longer changes the accepted-route identity native resume matches. Review polish (2026-10-19): `command_available` resolves the same inherited/npx launch identity the start path uses, and the remaining advisories are comment, test-shape, and QA-evidence cleanups with no further behavior change. Generated OpenAPI/SDK artifacts co-ship the payload field; no schema, config, or layout change.
- Permission follow-up (2026-10-19): Settings command availability uses the existing subprocess executable resolver so a non-executable native OpenCode preserves its permission error; only `exec.ErrNotFound` permits the inherited npm fallback. The provider-list suite covers wrapped permission/relative-path errors and a real non-executable native file with executable npx. Web consumes the corrected `command_available` value; native tools, HTTP/UDS shapes, hooks/extensions/config, workspace isolation, persisted data, site docs, and the official skill are unchanged. QA owner: `MS-web-settings-providers-redesign`.
- SD-013: no schema/config/DTO/route/tool/key/migration change; the 2026-10-18 selector browsing follow-up and the OpenCode label/sort metadata below are the only Web changes and alter no persisted state (regime 1: persisted rows, curation, and intent survive transitions/reopen; regime 2: same provider/source IDs, commands, flags, routes, DTOs; regime 3: `parseOpenCodeModelRows` renamed to the explicit v1 name with all callers, independent default discovery-command choice removed for the automatic path).
- Web/site/tools/hooks/extensions/config lifecycle/workspace isolation: unchanged consumers read the same catalog result; no new operation or guidance ships here. Task 03 co-ships `providers.mdx` default-command references, official-skill guidance, and the `RT-model-catalog-cold-open` / `MS-live-model-release-refresh` scenario updates with real first/restarted-prompt proof. Fixture-based discovery/refresh evidence must not be claimed as live vendor compatibility; V-001 live legs (v1, v2, npm-only, private-service lifecycle) remain for Task 05.
- Task 03 (2026-10-18): IT-002 passes via `go test -tags=integration ./internal/session -run TestManagerIntegrationOpenCodeInvocation` (native v1/v2 first+restarted prompts, explicit selector/account, incompatible-model rejection with zero prompts, native startup failure with untouched npm canary, npm-only prompt). E2E-001 passes in the daemon-served browser lane (focused `--grep "OpenCode"` plus full spec file 3/3 on re-run; stop-mid-turn + re-prompt fresh bind, CLI/HTTP/UDS effective parity, `calls.log` marker agreement). Guidance: `providers.mdx` command row + versioned discovery/installed-first launch paragraph, `model-catalog.mdx` versioned discovery sentence, skill refs `native-tools.md`/`runtime-operations.md`. Evidence: `.compozy/tasks/opencode-model-discovery/evidence/task_03/`.
- Selector follow-up (2026-10-18): the development catalog already advertised 92 live OpenCode v2 models, but an explicit curated set containing only a deprecated model left every row `curated=false`. The shared Runtime selector now applies curated browsing only to All models; a provider chip exposes the full already-authorized catalog, like search. No config/curation mutation, discovery change, native tool/CLI/HTTP/UDS contract, hook/extension shape, persisted state, or workspace/profile/account isolation change. Site and official runtime-operations guidance co-ship. The existing selector component suite owns empty/mixed curation, provider isolation, selection and ranking; the OpenCode browser journey adds retired curation metadata for both native-major fixtures. `RT-model-catalog-cold-open` and `MS-live-model-release-refresh` own the replay.
- Selector verification: component suite 104/104 (91 before the metadata cases); provider-override daemon-served browser suite 4/4 (v1→v2 exact selection/first/restarted prompt included); live v2 picker 92/92 available models enabled with keyboard/focus intact. Race-enabled modelcatalog/providerexec/session plus IT-002 pass. Lint repairs retain behavior, extracting only existing replay preparation; vendor v1/npm/platform limits remain in the [QA report](../qa/reports/2026-10-18-opencode-model-discovery-task05.md#selector-follow-up--2026-10-18).
- OpenCode list metadata (2026-10-18): only `opencode` model rows show their native provider prefix in muted text after the model name; its catalog groups sort by native provider then display name ascending. Blank/null/missing provider identities omit the text without throwing or dropping the model. Exact selection IDs, pinned recents/favorites, all other CLI labels/ranking, catalog scope, and availability stay unchanged. Native tools, CLI/HTTP/UDS contracts, hooks/extensions/config, persisted data, workspace isolation, site, and official skill have no impact: presentation only. The existing selector suite and v1→v2 browser journey own the regression; `RT-model-catalog-cold-open` owns QA.

## Open-design library prototyping target — 2026-10-20

- **Native tools:** none added or changed. `ext__open_design__lint_artifact` keeps its ID, schemas, and `docs/design/` HTML boundary; library prototypes are linted through rendered evidence HTML under that directory.
- **Extensibility and hooks:** the bundled extension adds the `open-design-setup` skill and its recipes reference. The `open-design-review` Loop gains an optional `target` input (`auto` default, `html`, `library`); the designer output adds required `target` and optional `source_paths`/loopback-only `preview_url`; completion additionally requires the requested target and at least one source path for library results; critic blocking issues may cite prototype sources. No hook, config key, or loop-engine change.
- **Workspace data isolation:** prototypes live in the active workspace in a project-owned environment recorded at `docs/design/prototyping.md`. Setup changes the repository only after user approval; the Loop never changes the environment. Preview URLs are restricted to loopback in the Loop schema. Prototypes must not call product APIs.
- **Official CompozyOS skill:** `skills/compozy/references/extensions.md` documents the two targets, the setup skill, the record, and the Loop `target` input.
- **Web/Docs:** no Web change. The site guide `extensions/open-design.mdx` adds the library prototyping section and the updated resource count. `ET-open-design` step 7 owns the library-target QA walk (passed 2026-10-20 on three projects).
- **Compatibility:** additive. Existing HTML requests, artifact paths, lint calls, and Loop invocations without `target` behave as before (`auto` follows the skill, which defaults to HTML without a record).

## Issue #872 — Session message formatting and editing

- Web: preserve authored soft line breaks in user prose/list items; keep both message action rows visible; expose the existing conversation rewind as Edit message with an explicit draft/conversation consequence.
- Native tools and HTTP/UDS/CLI: reuse the existing fenced rewind and prompt submission contracts; no schema or tool ID changes.
- Extensibility, hooks, configuration: unchanged.
- Workspace/profile isolation and user data: unchanged; rewind remains daemon-authoritative and archives the discarded suffix without undoing file/tool effects. No migration.
- Official runtime skill: existing rewind contract remains accurate; no skill changes.
- Docs/QA: update RT-conversation-rewind and RT-session-transcript-whitespace; browser suites cover authored layout, persistent controls, and Stop/edit/resend.

## Provider usage — topbar usage mini-app, `[provider_usage]`, `compozy provider usage` — 2026-10-21

Owner: `.compozy/tasks/provider-usage/` (`_spec.md` Part II §Impact Analysis, ADR-001…005, exploration in
`analysis/`). Implementation slices cite this entry and update only the bullets they change. Every public
surface below is documented **experimental** (SD-013 regime 2 exception) until the history follow-up
promotes it.

- **Native tools:** additive `compozy__provider_usage_list` (read-only, cache only, no network) and
  `compozy__provider_usage_refresh` (mutating, always background intent, caller profile, waits up to 15 s),
  toolset `compozy__provider_usage` (sibling of `compozy__provider_models`); catalog and digests regenerate.
  No existing tool changes.
- **CLI/HTTP/UDS:** additive `compozy provider usage list|refresh` (refresh sends `explicit`); `GET
  /api/provider-usage`, `GET /api/provider-usage/providers/{provider_id}`, `POST /api/provider-usage/refresh`,
  `GET /api/provider-usage/stream` (SSE `ready`, `provider_usage_changed`, both with `boot_id`), and
  `GET/PATCH /api/settings/provider-usage`. `explicit` is honored only for local operators. Foreign-origin
  app tokens may use the GETs and are denied refresh; no new token scopes. Usage codes use the flat
  `ErrorPayload`, profile codes the profile envelope. Generated OpenAPI, Go/TypeScript SDKs, tool catalog, and
  CLI reference co-ship. A provider-filtered read or `refresh <provider>` returns only those rows with
  `tray: null`; the envelope and settings response carry read-only `keychain_supported` (macOS daemon only).
  Settings PATCH accepts `collection_profile` (default `default`) independently of user/machine config scope; a local consent
  apply starts exactly one explicit collection for that profile, while gateway/agent requests stay background.
  Reload and unrelated settings mutations never seed consent; after restart, saved consent offers explicit
  Refresh. Unsupported provider/auth-mode pairs use the existing `no_collector` code.
- **Extensibility and hooks:** no hook event, manifest, Host API, extension SDK, or MCP sidecar change; alert
  hooks are deferred with history.
- **Config:** `provider_usage.enabled` (true), `provider_usage.refresh_interval` ("5m", 1m–1h),
  `provider_usage.providers.<id>.enabled` (true), `provider_usage.providers.claude.keychain_access` (false,
  darwin only). User/machine scope; workspace config fails to load and profile overlays reject the root;
  live apply; additive (ladder (a)). `config set` accepts `keychain_access` only under `claude`; agents'
  native config tools may write the other three keys but are refused `keychain_access` (operator-only
  consent). Deleting a custom provider removes its companion usage override in the same write.
- **Workspace data isolation:** no SQLite migration, tables, or workspace files; snapshots are in-memory and
  keyed by `(profile, provider, route fingerprint)`, fenced by an in-memory credential identity; aggregate
  profile reads are rejected; no credential, digest, account ID, or raw payload in any output. CompozyOS reads
  provider credential files read-only; the Codex CLI may renew its own login (ADR-002), file store only
  (`keyring`/`auto` → `unsupported`). The macOS Keychain is read only on explicit local-operator intent; the
  token stays in memory, bound to Claude's account evidence, and is dropped on re-login, logout, consent
  revocation, or indeterminate evidence. A missing Keychain item reports `signed_out` (`auth_missing`);
  only a real OS denial reports `keychain_denied`, retained until another explicit action. Expired Claude
  credentials retain owner evidence and same-owner last-good values as stale; Keychain expiry offers refresh,
  while file expiry directs the operator to sign in through Claude. Bound-key
  replacements are checked against freshly resolved secrets without adding secrets to public fingerprints.
  File-backed reads recheck local ownership when file metadata changes and discard mismatched snapshots
  before returning; replacement collection runs in the background without a Keychain read.
  Removed targets cancel only their own flights; deleted profiles close subscriptions and clear credentials.
  Profile events reconcile through one coalescing worker with bounded resolution and joined shutdown.
  Replaced eligible routes collect promptly after invalidation. New targets added after the startup tick
  receive a first background collection immediately; targets discovered during startup share its jitter. Scheduled resolution uses the scheduler's
  owned cancellation context, so disable and shutdown cancel blocked profile/credential reads.
  A failed publication fence retains same-owner last-good values and retries only publication checks within
  the flight deadline, without repeating the upstream collection. If a concurrent resolution supersedes
  the captured epoch, the admitted flight recollects the eligible replacement instead of leaving it loading.
  Cancellation and removal still stop the flight. Reads cancel ineligible flights without
  waiting for cleanup; lifecycle reconciliation and shutdown own bounded joins. Explicit official ChatGPT
  origins normalize to the backend API before selecting the Codex usage path.
- **Official CompozyOS skill:** `skills/compozy/SKILL.md` router row; `references/native-tools.md` (two
  tools), `references/runtime-operations.md` §Provider Usage (verbs, routes, intent, statuses and recovery),
  `references/configuration.md` §Provider Usage (keys, scope, settings route).
- **Web/Docs:** `web/src/systems/provider-usage/` (domain), OS tray slot, popover, overlay `usage`, Usage app
  (`/usage`), Settings → Provider usage section; `unsupported`/`disabled` rows fold into one closed trailing
  group in the popover and Usage app; Allow and the Keychain setting render only when `keychain_supported`. Site: `agents/provider-usage.mdx` tutorial (tray, popover,
  Usage app, Keychain consent, Codex store, support table, statuses, CLI, config, tools, routes),
  `configuration/config-toml.mdx` `[provider_usage]`, `tools/toolsets.mdx` row, generated CLI pages.
  The daemon owns initial consent collection; the Web invalidates its query and reserves refresh for
  later Refresh/Try again actions. Failed explicit refreshes show one error toast even if the cache reread
  succeeds; Allow retains its settings-owned error toast. Opening also refreshes first-fetch rows without
  an active flight. Cursor partial auto/API percentages do not imply a monthly total.
- **QA:** `APP-provider-usage-tray` (operator walk, journey `J-provider-usage`) and
  `ET-provider-usage-surfaces` (transport, profiles, app tokens, SSE) — both `untested` until the trailing QA
  pair walks them with real Claude and Codex logins.

## Desktop update lifecycle and one-action recovery — 2026-10-21

Owner: `docs/research/desktop-update-flow/README.md`; branch `fix/desktop-update-flow`.

- **Desktop:** verify the app archive, await native installer readiness, then let the installer
  own shutdown. Executable deadlines cancel stalled work; transient acquisition failures retry
  through the existing watcher. Rosetta app selection follows hardware while runtime-only
  archive selection remains unchanged. A failed handoff after cleanup starts drains that same
  cleanup before quitting, avoiding a windowless process that cannot receive another update.
- **CLI/HTTP/UDS:** existing verbs, routes, target ordering, and DTOs remain accepted. After
  validating the original request, a joint update on a proven desktop-owned runtime becomes
  one app operation; bootstrap installs the bundled runtime. CLI completion reads the observed
  daemon version and compares PID/start time with the pre-update discovery identity for the
  existing `daemon_restarted` field. Missing or unreadable pre-update identity leaves that
  flag false without blocking installation; final readiness validation remains mandatory.
  Explicit runtime-only and standalone/managed behavior is preserved.
- **Native tools:** no new or changed `compozy__*` IDs; host update remains CLI/HTTP/UDS owned.
- **Extensibility/hooks/config:** no configuration keys, hooks, SDK contracts, or dependency
  changes. The one-action product flow uses the existing update consent.
- **Workspace/user state:** no database, config, or journal schema change. Existing provenance,
  digest checks, process identity, leases, and installer fence remain authoritative. Test apps,
  caches, homes, and ports are isolated from the operator's installed applications and home.
- **Web:** a settled failed app update offers Retry update in the menubar and Settings while
  retaining its error; live operations and managed-runtime restrictions remain enforced.
- **Official skill/Docs:** `skills/compozy/references/desktop.md`, getting-started and operations
  desktop docs explain one bundled update and distinguish update retry from startup retry.
- **QA:** APP-app-auto-update, APP-update-recovery-state, APP-runtime-update-app-owned,
  APP-single-command-multi-target-update, APP-web-update-indicator, and APP-web-update-two-track.
  Packaged E2E-016/017 must check both app replacement and the running bundled daemon version.
  E2E-017 also induces a real filesystem cleanup failure and requires archived failure plus shell exit.
  Signed release delivery and upgrades from already shipped broken updater code remain distinct
  from local fixture evidence; the research report records these limits.


## Beta release review corrections (2026-10-22, PRs #887 and #893)

Transcript previews retain recorded output until live bytes are painted, constrain server resize
frames to the inline display height, and count terminal cells for recorded output. Grouped session
search hides the unfiltered header total. Native tools, extensibility/hooks/config, persisted data,
workspace isolation, and the official skill are unchanged. Web impact is covered by the existing
terminal surface, tool-call card, and sessions modal suites; the affected QA scenarios are updated.
Go integration fixture changes are formatter-only and preserve their existing assertions.

The HTTP and UDS integration runtimes now inject the same real profile manager as daemon bootstrap
and use isolated operator homes. Their existing automation and webhook assertions remain unchanged; this
repairs fixture wiring for the accepted User scope contract, with no public behavior change.

## Provider-confirmed Fast and reasoning — issue #914, 2026-10-23

Owner: branch `fix/issue-914-fast-reasoning-apply`; evidence and research matrix:
`docs/qa/reports/2026-10-23-issue-914-fast-reasoning.md`.

- **Native tools:** additive `ultra` acceptance for session prompt/runtime selection, queued-input
  output, agent/fallback authoring, Goal runtime, Loop runtime rules, and provider default-effort
  curation. Subagent targets already accept provider effort identifiers. Descriptor catalog regenerated.
- **HTTP/UDS/CLI and extensions:** requested values remain persisted unchanged. `runtime.effective`
  and prompt events project the latest provider-confirmed effort/speed. Later ACP option updates
  revoke stale applied Fast; unsupported speed does not populate effective speed. Extension host
  reads share this projection. Additive `runtime.requested` exposes persisted intent separately;
  OpenAPI, app clients, and extension TS/Go contracts are regenerated. CLI effort help and generated
  reference include ultra; existing speed outcome enums suffice.
- **Hooks/config:** existing runtime events/fields only; no hook IDs, config keys, permissions,
  authentication policy. Equal-selection prompts recheck observed controls. A healthy provider refusal
  is attempted once per new prompt without same-intent rollback or process replacement.
- **Workspace data isolation/user state:** no schema migration, paths, scopes, or ownership changes.
  Persisted `speed` continues to mean requested speed and matches `speed_resolution.requested`.
  Public enum widening is additive; no compatibility shims or delete targets.
- **Official skill:** agent effort vocabulary and runtime guidance explain exact model membership,
  provider acknowledgement, later revocation, and requested versus effective values.
- **Web/Docs/QA:** Web includes ultra and consumes model-specific efforts. Composer defaults now prefer
  requested intent when selected is absent, preserving intent across observation refetches. Goal prompt
  queueing also uses requested rather than applied runtime. Provider docs explain this distinction. RT-061, RT-session-prompt-runtime-transitions,
  MS-057, and LP-runtime-provenance-observation receive scoped QA notes. Browser and live Loop/Goal
  checks are outside the authorized minimal lab; source/focused automated evidence is labeled.

## Harness dogfood corrections (2026-10-23)

Owner: `fix/harness-dogfood-2026-10-22`;
[findings and dispositions](../research/harness-dogfood/analysis/summary.md).

- **Native tools / public surfaces:** forked and continued user sessions materialize the existing
  delegation ceiling after lineage refresh, including legacy resume. Governed-child restrictions
  and runtime authorization remain authoritative. No tool IDs, routes, input/output shapes or
  stable error codes are removed. Validation uses existing optional operator-safe detail fields;
  submitted values, unknown keys and arbitrary backend errors remain masked. `session_history`
  answer guidance selects its existing `type=agent_message` filter; full history and artifact
  continuation are preserved. Native descriptors and their generated catalog co-ship.
- **Extensibility/hooks/config:** successful hook dispatch logs move to DEBUG and gain available
  correlation IDs; failures/denials, persisted hook runs, events and metrics retain their meaning.
  Successful read access and empty sweeps move to DEBUG; mutations/repairs remain INFO and errors
  visible. Existing `[log]` controls restore detailed tracing; no config key/default or hook
  payload changes. No extension or SDK protocol change.
- **Runtime process ownership:** idle release/replacement uses an optional internal driver stop
  budget, forwarded by the ACP adapter; verified exit and pending ownership remain authoritative.
  Stopped hooks get their own bounded context. This is an internal contract, with no public timeout
  knob or state schema change. Lifecycle docs and `RT-session-native-stop` co-ship.
- **Workspace/user state:** no SQL/config shape change, migration or historical transcript rewrite.
  Derived lineage repair occurs through the existing session lifecycle persistence owner, within
  the session's workspace/profile. Source sessions and governed-child budgets remain unchanged.
  Investigation reads of the operator home were read-only; tests use disposable homes.
- **Official skill:** `skills/compozy/SKILL.md` distinguishes ordinary delegated provider work from
  daemon Task authority and still forbids native-capability bypass. Native/task references describe
  filtered answer retrieval and exact continuation rather than pasting full manuals or raw history.
- **Web/Docs/QA:** no Web rendering change; existing safe error details remain consumable through
  current contracts. Site subagents and logging docs co-ship. Affected subagent, continuation,
  native-call, reply and observation scenarios link the focused automated verification record.
  No new live-provider/browser verdict or measured model/latency improvement is claimed. Final
  local gate and exact-head CI are PR delivery requirements.

## Merge train CI fixture and readiness repairs (2026-10-24)

Main CI exposed two test-support defects. The marketplace HTTP projection fixture treated
the optional `compatibility.json` request as another catalog fetch and panicked by closing
its start channel twice. It now serves only its declared catalog paths and returns the
contractual 404 for absent metadata. Existing source isolation and no-retry assertions remain.

The runtime harness considered any successful status response ready, including the daemon's
intentional `draining` state during boot. It now requires `daemon.status = running` from HTTP,
UDS, and CLI before running scenarios, using the same readiness timeout and polling cadence.
The owning lifecycle suite covers draining, missing, and unknown status on each surface.
The failed automation scenario and its task assertions are unchanged.

Native tools, public contracts, hooks, extensions, config, workspace data, Web, and the
official skill are unaffected. Fixtures use isolated HTTP servers and subprocesses. Focused
race tests verify these support contracts; the unchanged daemon scenario and complete
integration shards remain CI-owned under the user's local-validation override.

## In-product reduced motion for content transitions — 2026-10-24

Owner: post-merge review A-972-01 of PR #972. The desktop shell publishes its effective motion
preference (client-local Appearance setting OR system preference) to the shared document CSS.
View-transition snapshots and CSS animations honor either preference without changing navigation,
deferred content identity, window geometry, or palette subscriptions. Skeleton sweeps stop entirely
and delayed reveals retain their delay under either preference, avoiding fast-load flashes.

- **Web/Docs:** Settings, Session and all shared content transitions use the same suppression;
  `docs/qa/scenarios/ET-web-ui-resilience.md` tracks the focused real-browser verification.
- **Native tools, extensibility/hooks/config, workspace isolation, official skill:** no wire,
  schema, persisted preference, authorization, SDK or `skills/compozy/` change. The DOM attribute
  is an internal rendering boundary and is restored when the shell unmounts.

## Deferred loading skeletons during desktop restoration — 2026-10-25

Owner: v0.4.1 release CI remediation. The Web window host defers app-specific loading
skeletons with the window body, so react-rnd measures restored frames before their
layout-heavy descendants mount. The existing lightweight pending indicator covers
that first commit; suspended bodies retain their app-specific, route-aware fallback.
Authoritative geometry, focus, routing, and peer convergence remain unchanged.

- **Web/Docs:** the existing E2E-023 performance envelope and OsWindow loading suite own
  verification. `docs/qa/scenarios/ET-web-desktop-shell-lifecycle.md` records the affected
  journey and local measurements. Public site documentation does not change.
- **Native tools, extensibility/hooks/config, workspace isolation, official skill:** no
  CLI/HTTP/UDS or native tool contract, schema, persisted layout, authorization, extension,
  hook, config key, SDK, or `skills/compozy/` change. This is client render scheduling only.

## Merge train archive recovery remediation — 2026-10-29

Owner: PR #1010 follow-up on main (reviews 4407700798, 4407700810 and 6485005842).
Workspace registration reads isolate filesystem inspection failures and retain recovery
operations; execution resolution still fails closed. Managed automation replacements
refused for archived targets pause the previous definition through existing enabled
overlays. Source intent and history remain intact; after restore/sync, explicit enable
resumes the updated definition. The Go SDK example generator uses the parameterized
workspace-list method, preserving the legacy no-argument method.

- Native tools, CLI/HTTP/UDS and Web share the repaired registration and automation owners;
  no new DTOs, routes, native IDs, configuration keys, hooks or extension contracts.
- Workspace isolation and persisted histories are unchanged; no schema migration.
- Official skill contracts remain unchanged. QA coverage is recorded in RT-008;
  the generated site SDK samples co-ship with their generator.
- Focused SQLite/runtime suites and the existing all-example Go type check verify
  these changes. Complete current-head CI remains the release gate.

Follow-up entry-point fixes (reviews 4407737369, 4407737376, 4407792174): session
stop routes may resolve a missing-root registration just like retained reads; other
mutations continue through runtime resolution. Native archive/unarchive bind registry
identity via Get, then pass through the unchanged workspace authorization seam.
Shared core behavior covers HTTP and UDS; Web uses those same routes. Native IDs,
DTOs, official skill contracts, hooks/config, and storage shapes are unchanged.
Existing handler, native binding and SQLite archive suites own the regression checks.

CI also caught four native workspace descriptors requiring an explicit workspace
although the existing binder inherits the caller workspace. Their shared input schema
now permits omission, restoring the established native contract; operator calls without
any scope still fail the handler's required-input check. Descriptor consumers regenerate
through codegen. No new fields or IDs, and no persisted-data changes.

## Merge train overview focus and CI follow-up — 2026-10-29

The workspace overview scroll owner now enumerates listbox options, preserving
workspace order after project context menus replace the compositional data-slot.
A real pointer drag strengthens the existing mobile mixed-removal E2E and reproduces
the pre-fix loss of focus. The subagent journey waits for its existing settled-turn
disclosure after returning to the parent: route identity precedes deferred history,
so a one-time zero-row count was not a readiness signal. All navigation assertions
and timeouts remain unchanged. The SDK compatibility test retains the exact legacy
method signature through an interface while complying with staticcheck.

Web/QA: RT-workspace-overview-command-tab and the existing worktree/subagent journeys.
Native tools, CLI/HTTP/UDS, hooks/config/extensions, persisted data and workspace
isolation are unchanged. No generated contract or official skill changes.

## Detached message edit acknowledgment recovery — issue #1017

- **Web:** after navigation detaches a replacement send, a lost acknowledgment transfers
  ownership to the existing unconfirmed Retry row. The edit hook releases its submitted
  files and discards only the matching saved draft, preventing a fresh duplicate submission.
  Definite refusals and a different newer draft retain their existing behavior.
- **Native tools / public surfaces:** no CLI, HTTP/UDS, native ToolID, DTO, or admission
  contract changes. Retry uses the existing message identity and idempotency key.
- **Extensibility/hooks/config:** no extension, hook, SDK, or configuration changes.
- **Workspace/user data:** recovery remains scoped to the original workspace/session;
  no persisted schema changes or migrations. Browser verification uses disposable homes.
- **Official skill / Docs / QA:** no `skills/compozy/` or public site changes needed.
  `RT-conversation-rewind.md` records the lost-ack navigation journey, owned by the
  existing stop/edit/resend Playwright test in `session-hardening.spec.ts`.

## 2026-10-29 — SQLite Unicode function registration ordering

- **Owner:** `internal/store/sqlite_unicode.go` registers the existing Unicode
  function during package initialization, before probes, migration readers, or
  normal runtime connections open. Lazy first-open registration could mutate
  the driver's function map while another connection iterated it. Registration
  failures retain the existing error path from `openSQLiteHandle`.
- **Verification owner:** the existing SQLite connection/recovery suite checks
  Unicode SQL on a direct connection; the existing parallel migration suite
  exercises concurrent connection openings under the race detector.
- **Public surfaces, native tools, hooks/config, extensibility, Web/Docs and
  official skill:** unchanged. This is internal driver initialization ordering.
  Workspace isolation and persisted data/schema are unchanged; no migration.

## 2026-10-29 — Project overlay focus handoff (#1035)

- **Owner / Web:** overview creation handoffs skip obsolete trigger restoration;
  shared Dialog and ContextMenu popups transfer focus before modal isolation.
  Existing owning suites and interactive stories cover focus entry/restoration.
- **Native tools / CLI / HTTP / UDS:** unchanged; no routes, DTOs or tool IDs change.
- **Extensibility / hooks / config:** unchanged; no new setting or hook.
- **Workspace data / isolation:** no persisted state or schema changes. Runtime
  verification uses a disposable home, socket, ports and project.
- **Official skill / Docs:** no public operational contract or `skills/compozy/`
  change. Affected QA scenarios and the dated overlay focus report track evidence
  and limits; no public site change is needed.

## 2026-10-29 — Session processes

Owner: accepted session-processes plan; branch `feat/session-processes`.
The additive public contract exposes attributed OS processes and a session summary,
identity-fenced termination, and explicit keep/terminate lifecycle choices. Omitted
policy keeps processes. The operator aggregate includes retained and missing owners;
its separate termination route does not bypass session-scoped authorization.

- Native tools: session process list and approval-gated terminate; stop/archive gain
  the same optional policy. CLI and shared HTTP/UDS handlers own matching operations.
- Web: process popover, lifecycle choices and the `processes.left_running` palette
  command; the frontend executor owns consumers and browser evidence.
- Hooks/events: `session.processes.terminated` audits explicit cleanup. No automatic
  orphan cleanup, turn-cancel cleanup or shutdown policy change.
- Workspace data/isolation: environment markers plus durable lineage observations.
  Additive migration 00157 creates `session_process_observations`; observations have
  no session foreign key so deletion preserves cleanup ownership. Boot revalidates
  exact PID/start identities; scans prune dead rows. Group inference requires live
  identity continuity. Unknown owners require matching daemon-home and profile
  markers. Runtime/API scopes continue to enforce workspace and profile ownership.
  QA uses a separate daemon home. No existing schema history or user rows change.
- Official skill/site/QA: process inspection and cleanup guidance and a real-process
  lifecycle scenario co-ship with the implementation. No new configuration is needed;
  scan demand and shared snapshots bound background work.

Contract checkpoint: DTOs, OpenAPI and generated SDKs land first to unblock the
parallel frontend. Runtime, API, native tools and CLI passed focused validation. The isolated lab verified
terminate, keep/restart, deleted-owner cleanup and native-terminal shutdown; its teardown
is clean. See `docs/qa/reports/2026-10-29-session-processes.md`. Current-head CI remains
the delivery gate. Frozen dependency installation also reconciles generated linter
and emoji data with the pinned toolchain; no dependency versions change.

## Workspace folders and totals footer — 2026-10-29

Issue #1044 adds root workspace (project) folders and a manual workspace order for All projects, plus
a sessions-sidebar totals footer. Persisted state: global SQLite tables `workspace_folders` and
`workspace_placements` (append-only migration 00156, additive, no backfill; existing users see no
workspace folders and the current name order; session folders and placements are untouched). The
`workspaces` delete trigger also clears placements; `workspace_placements` joins the user-scope
migration inventory. Public surfaces, all additive (SD-013): HTTP/UDS `GET|POST
/api/workspace-folders`, `PATCH|DELETE /api/workspace-folders/{folder_id}`, `PUT
/api/workspace-order/{folders,workspaces}`, `POST /api/workspace-order/move` (operator or
unrestricted agent; app tokens denied; one profile, `all_profiles` rejected); the catalog stream
adds kind `workspace_layout` (profile only) to `session_catalog_changed`, delivered to
`all_workspaces` subscribers. CLI: `compozy workspace folder list|create|rename|delete|reorder`,
`compozy workspace move`, `compozy workspace reorder`. Native tools:
`compozy__workspace_folder_{list,create,rename,delete,reorder}`, `compozy__workspace_move`,
`compozy__workspace_reorder` in the workspace toolset; naming follows the canonical `workspace`
term while UI copy says "project". Isolation: folders and placements are keyed by profile;
workspaces are global, so any registered workspace (or `@user`) may be placed; foreign folders read
as not found. The totals footer reads the existing exact session facets (no contract change). Hooks
and extension SDKs are unchanged. Official skill `references/runtime-operations.md`, site
`docs/sessions/index.mdx`, generated CLI docs, and QA scenario
`ET-web-session-folders-manual-order` record the contract.

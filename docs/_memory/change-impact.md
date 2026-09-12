# Compozy Change Impact

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
- **Official skill:** runtime operations now explain overlay-scoped discovery and binding.
- **Web/Docs/QA:** no Web component or layout changes; Settings and runtime selectors receive the
  shared corrected projection. Model catalog docs and `RT-model-catalog-cold-open` cover overlays.
  Owning checks: live-source and session suites, daemon reconciliation suite, and the SQLite/ACP
  subprocess catalog integration suite. Real provider authentication is not inferred from fixtures.
- **Related work:** PR #550 owns curated-merge semantics and broader picker/startability changes.
  This fix is based independently on main and changes only overlay discovery/binding plus necessary
  source lifecycle handling. Its overlapping Claude binding code must preserve overlay ownership
  when that PR is integrated. Issue #623 owns auth classification; it is excluded here.

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

- **Native tools:** new `compozy__session_continue` and `compozy__session_fork` in the sessions toolset (risk `mutating`, same permission gate as `compozy__session_create`); `compozy__session_create|rewind|describe` keep their IDs and schemas; session read payloads gain `lineage.kind`, `lineage.origin_message_id`, `lineage.origin_agent_name`, `derivation` (`kind`, `source_session_id`, `seed`, `native_state`, `native_fork_error`, `first_prompt`), and `runtime.acp_caps.supports_fork_session|supports_resume_session` (additive). Tool catalog and digests regenerate.
- **Extensibility and hooks:** no hook, manifest, skill/capability, Host API, bridge SDK, or MCP sidecar change; hooks keep dispatching at their call sites (post-create for the child after its commit); hook payloads that embed lineage gain the optional fields only. ACP: `initialize.agentCapabilities.sessionCapabilities.fork|resume` captured; unstable `session/fork` called only on an idle, bound source that advertises it, under the source's exclusive prompt slot; every ACP notification and callback is routed by session id (bound / fork sink / foreign-dropped) so clone traffic never reaches the source (ADR-003). Failure vocabulary: `provider_error.next_action` gains `handoff` (additive enum value; decorated by the session owner from the typed session type; older clients render it as their neutral step). Config: new `[session.derive] max_replay_bytes` (default 131072, min 4096, hard bound) and `max_message_bytes` (default 16384, 1024…max_replay_bytes), workspace overlay; no other key changes. Event: new `session.derived` on the child ledger, emitted from the committed creation transition.
- **Workspace data isolation:** the child session belongs to the source's workspace/worktree and profile; the derive routes use the workspace route guard (a source in another workspace is 404). Global `sessions` gains `lineage_kind`, `origin_message_id`, `origin_agent_name` (migration `00110`, lossless backfill from `session_type`/`spawn_role`/`parent_session_id`); per-session `meta.json` documents written before the feature are upgraded at the read boundary by the same rule (regime 1, no rewrite on read). New global side table `session_derivations` keyed `(workspace_id, idempotency_key)` with an immutable outcome; rows are tombstoned (`child_deleted_at`) by the session delete path, never removed. The carried context is one immutable payload in the child's own meta (`imported_context`), taken from one read-only snapshot of the source and never written to the source; attachment bytes are not copied. The optional first message is admitted through the existing `session_prompt_admissions` (key `<derive key>:first`). Nothing crosses workspaces or profiles.
- **Official CompozyOS skill:** `skills/compozy/references/runtime-operations.md` §sessions documents `session continue|fork`, `derive/preview` (`native_fork_possible`, `cut.turn_id`), `lineage.kind`, `derivation`, retry semantics (`replayed`, `child_deleted`), `handoff`, and the two tools.
- **Web/Docs:** `web/src/systems/session/components/session-row-actions.tsx` and `hooks/use-session-topbar-slot.tsx` (two menu items), `components/session-continue-dialog.tsx` / `session-fork-dialog.tsx` / `session-fork-message-action.tsx` / `session-origin-pill.tsx` / `session-continue-divider.tsx` (new), `components/session-status-line.tsx` (pill), `components/session-inspector-memory.tsx` (Origin + Seed rows), `components/runtime-activity-notice.tsx` + `lib/provider-error.ts` (`handoff`), `web/src/components/assistant-ui/message-actions.tsx` (fork action), `web/src/systems/os/apps/session/session-window-content.tsx` ("Restart in a new session", `lineage_kind: recovery`). `packages/site`: `sessions/lifecycle.mdx` (new section incl. durable carried context, native bootstrap states, retry semantics), `sessions/resume.mdx` (cross-link), `agents/providers.mdx` (`handoff`), `cli/session/continue.mdx` + `fork.mdx` + `new.mdx` (`--lineage-kind`) + `status.mdx`, `api/sessions.mdx` (three operations, additive fields), `configuration/config-toml.mdx` (`[session.derive]`), `sessions/events.mdx` (`session.derived`). `COPY.md` + `docs/_memory/glossary.md`: `fork` = conversation fork (this feature); recovery button renamed; `handoff` stays a Network term. Verification owners: `_tests.md` (UT/IT/E2E), `eng-ui-screenshot` bundles for `docs/design/opendesign/session-continue-fork/`, QA scenarios `RT-conversation-rewind` (rewind on provenance/derived children; carried context kept) and `ET-web-session-sidebar-threads` updated and reset to `untested`; new `ET-web-session-continue`, `ET-web-session-fork-from-here`, `ET-cli-session-continue`, `RT-session-derive-native-fork`, `RT-session-derive-retry`, `RT-provider-error-handoff`.
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

Owning spec: `.compozy/tasks/session-context/_spec.md` (Part II §Impact Analysis lists delete targets and regimes; ADR-001…007 record the decisions; exploration in `analysis/summary.md`; peer review rounds 1 and 2 incorporated 2026-09-11, `qa/peer-review-incorporation-round{1,2}.md`). Implementation slices link here and update only affected entries.

- **Native tools:** no `compozy__*` tool added, renamed, or changed; `compozy__session_describe|events|history` keep their IDs, schemas, and digests (`session_events` now also lists `prompt_delivery` events through the existing passthrough). Agents read context through `compozy session usage -o json [--turns]` over UDS or `GET …/usage` / `GET …/usage/turns`.
- **Extensibility and hooks:** no hook, manifest, skill/capability, Host API, bridge SDK, or MCP sidecar change; `prompt.post_assemble` keeps its contract (a replaced startup prompt is measured as one opaque `system_prompt` span). New public session ledger event type `prompt_delivery` whose canonical content carries a typed `delivery` field (`turn_id`, `sent_at`, `estimate`, `spans[]` incl. `startup_dedup`), recorded after the transport confirmed a prompt; new named session-stream event `session_usage_changed` (`{sequence, turn_id, kind}`) for `usage` / `done` / `prompt_delivery` ledger events beyond the cursor (push and poll paths). ACP decoder: prompt-response cache counters read as `cachedReadTokens`/`cachedWriteTokens` (schema names); the former `cacheReadTokens`/`cacheWriteTokens` spelling stays accepted in v0.4.0 and v0.5.0 with a once-per-session deprecation warning naming the replacement and is deleted in v0.6.0 (boundary alias only); `usage_update._meta` decoded tolerantly (objects only), passed through `redact.ClaimTokensJSON` (keys named `claim_token` removed at any depth, token-shaped values replaced), kept on the recorded usage event content and forwarded as `meta` on live/prompt/transcript usage payloads (additive), never persisted into columns or counters; `used < 0`, `size ≤ 0`, negative counters dropped at the boundary with one warning per session. Config: no new key; `[session.compaction] enabled|pressure_threshold` populate `context.pressure_threshold` only when the current report carries an agent-reported size.
- **Workspace data isolation:** globaldb `token_stats` gains nullable `cache_read_tokens` / `cache_write_tokens` by Goose migration (accumulated per session/agent on done turns like the other counters; historical rows read NULL → absent); no session-DB schema change (the delivery manifest is a ledger event archived and wiped with the session ledger; usage payloads gain a wire-only `sequence`); `context` is derived per session from that session's ledger (observations, deliveries, compactions), projection, and effective model; the catalog window lookup is keyed by the session's provider/model; nothing crosses workspaces or sessions.
- **Official CompozyOS skill:** `skills/compozy/references/runtime-operations.md` §session usage documents cache totals, `context` (`state` incl. `unavailable`, `used`, `size`, `size_source`, `stale`, `sequence`, `pressure_threshold` eligibility, `injected` rows), `--turns` (union rows ordered by sequence, compaction markers with `span_archived` — a fact, never a completion claim), the `prompt_delivery` event, and the `session_usage_changed` stream event.
- **Web/Docs:** `web/src/components/assistant-ui/session-composer-action-row.tsx` (new context control), `web/src/systems/session/components/session-inspector*.tsx` (tab-less Context sidebar; Memory/Files/Vault sections deleted — Vault stays at `/vault`, file audit stays in the transcript roll-up, the ledger stays on `getMemorySessionLedger`), `web/src/systems/session/hooks/use-session-context.ts` + `use-session-usage-turns` (new), `hooks/session-stream-source.ts` + `hooks/session-live-tail-runtime.ts` (named `session_usage_changed` listener and invalidation), `web/src/systems/os/apps/session/use-session-window-controller.tsx` (usage query ungated; ledger/vault wiring removed). `packages/site`: `cli/session/usage.mdx` (cache lines, Context block, `--turns`), `api/sessions.mdx` (`context`, `getSessionUsageTurns`), `sessions/events.mdx` (`prompt_delivery`). Verification owners: `_tests.md` (UT/IT/E2E), `eng-ui-screenshot` bundles for `docs/design/opendesign/session-context/`, QA scenarios `RT-052`, `RT-session-cost-provenance`, `RT-024`, `ET-web-session-inspector-toggle` updated and reset to `untested`; new `ET-web-session-context-meter`, `ET-web-session-context-sidebar`, `ET-cli-session-usage-context`, `RT-acp-usage-cache-meta`.

## Marketplace catalog — one kind, plugin marketplaces as sources

Owning spec: `.compozy/tasks/marketplace-catalog/_spec.md` (Part II §Impact Analysis lists delete targets and regimes; ADR-001/005/007/008 record the SD-013 ladder outcomes; peer review rounds 1 and 2 incorporated 2026-09-10). Implementation slices link here and update only affected entries.

- **Native tools:** `compozy__marketplace_search` keeps its ID; its `kind` argument keeps working one release (`mcp` = extensions that provide servers, `skill` = the fenced skills listing; descriptor deprecation, removal v0.6.0) and the response gains `revision`/`stale`/`installable`/`trust.decision`. `compozy__extensions_install` accepts `source: marketplace`, `inputs`, and `expected_digest` (the approved acquisition). New read-only `compozy__marketplace_sources` (experimental one release). `compozy__extensions_*` and `compozy__mcp_status|mcp_auth_status` gain the optional `owner` argument. `compozy__skill_list|view|search` unchanged. Tool catalog and digests regenerate.
- **Extensibility and hooks:** extension manifest gains `[[inputs]]` (shared grammar with the feed) and `resources.mcp_servers.<name>.auth`; `env`/`secret_env` keep their map shape (binding name = input id). Agent Plugins loader locates `.claude-plugin/`, `.codex-plugin/`, `.cursor-plugin/` manifests and decodes client grammars through adapters (skills + `mcpServers`; commands/agents/hooks reported as ignored); `layout`, `source_name`, `source_ref`, `entry_id`, `resolved_ref` recorded in provenance. Extension-provided MCP servers carry `owner = extension:<name>` through publication, status, auth tokens and OAuth registrations, a disjoint vault namespace (`vault:mcp/ext/<name>/…`), and overrides (`extension_mcp_overrides`, which also persists the once-allocated runtime name); manual servers keep `owner = manual` and their vault refs; settings/auth routes resolve owner-less names by runtime name; additive `mcp_server_name_taken` validation. Input readiness is typed over stored input state (`missing_inputs`); dropped inputs stay inactive, never deleted. Hooks, bridge SDKs, MCP sidecars, capability registry unchanged. Config: new `[[marketplace.plugin_sources]]` (name · source · enabled) with preset overlay from `v3/marketplaces.json`; `skills.marketplace.registry`, `skills.marketplace.base_url`, `skills.allowed_marketplace_mcp` consumed-and-warned one release by the fenced skills marketplace path, removed v0.6.0. Feeds: new `v3/extensions.json` + `v3/marketplaces.json` (`manifest_version: 3`, `icon`, `inputs`); the root family (`extensions.json` v2, `mcp.json`, `skills.json`) stays published from the same generator until v0.6.0 for released daemons; upgraded daemons read `v3/` and fall back to the root with a warning.
- **Workspace data isolation:** catalog projection is global (per daemon home), keyed `(source, entry_id)` with per-source generation, plus a global content-addressed package cache under the daemon home (capped, swept at refresh commit); installed state is joined per request by acquisition origin (`source_ref`, `entry_id`) from the profile/workspace-scoped extension inventory; provenance origin is backfilled only where catalog evidence exists; input values (`extension_inputs`) and server overrides are scoped to the extension instance's profile/workspace; extension-provided MCP servers keep the extension instance's scope and their own owner-qualified auth tokens; nothing crosses workspaces. Migration `00109` re-keys the projection, drops rows for the retired kinds (derived cache; sign-off in ADR-001; release-note migration block), backfills provenance origin (catalog-evidenced rows only) and `owner = manual` on tokens and OAuth registrations, and creates the two new tables.
- **Official CompozyOS skill:** `skills/compozy/references/tools-and-skills.md` §Marketplace Discovery (no kinds, `marketplace sources` experimental, install with `--input`, retained verbs still working with warnings until v0.6.0, `extension_source_changed`/`extension_name_conflict`/`marketplace_cursor_stale`) and `references/extensions.md` (inputs block, client layouts, marketplace sources, owner-qualified servers).
- **Web/Docs:** `web/src/systems/marketplace/**` (one page, Installed page, entry card/logo/trail, add menu, marketplace dialog, brand registry, cursor restart), `web/src/systems/settings` (Marketplace page with the experimental label; Skills "Marketplace URL" row deleted), routes `/marketplace`, `/marketplace/installed`, `/marketplace/$entryId` with one-release redirects from kind paths. `packages/site`: `lib/marketplace-catalog.ts` validates `v3/` and the retained root family (build breaks otherwise); docs `marketplace/`, `extensions/install.mdx`, `cli/marketplace`, `api/marketplace.mdx`, `configuration/config-toml.mdx` rewritten with both shapes during the window; `skills/marketplace.mdx` marked retired (deleted v0.6.0). Verification owners: `_tests.md` (UT/IT/E2E), `eng-ui-screenshot` bundle for the OD boards, QA scenarios `ET-web-marketplace-*`, `ET-api-mcp-catalog-install`, `ET-cli-marketplace-search`, `ET-site-marketplace-catalog` reset to `untested`; charter `CH-marketplace-installed-default` retired.

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

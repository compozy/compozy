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

Task05 documentation closeout: the configuration reference now removes active registry/base_url examples and root-feed fallback promises. Marketplace CLI examples use the canonical one-argument info and no kind flag. The migration guide and release notes list removed CLI/API/native acquisition inputs and preserved extension, manual MCP and installed-skill state. Retired QA rows stay skipped with replacement journeys linked; current namespace/search rows remain untested for final09/10. The installed-skill MCP authorization field remains active pending the explicit task05 consent decision; docs do not claim that removal has shipped. Task02 still owns v3 publisher/site conversion and the obsolete standalone skill-store guide.

Task02 publisher cut: compozy-catalog publish emits only v3 extension/preset feeds and verified package artifacts; the production v2 semantic adapter and copied decoder are deleted. Validation reads only v3 and always compares feed inputs with the packaged manifest. Existing extension entry metadata and artifact bytes are preserved by the canonical publisher suite. The 17 package definitions now compare launch/auth/input/default-scope behavior with an immutable pre-feature fixture, independent of root MCP feeds. No daemon config, native tool, credential, workspace or installed-state shape changes in this slice. Site kind consumers/root assets and the remaining task02 docs/schema obligations are still pending; existing final09/10 QA ownership remains.

Task02 site data and docs cut: the public catalog reads the single v3 extension/preset schema, uses direct /marketplace/<entry_id> routes, and removes Kind unions, root feed imports/outputs and the old standalone skill acquisition guide. Search and sitemap use current entry identities. Installed skill provenance stays documented in the local skills guide; its MCP consent allowlist remains unchanged pending the separate task05 decision. Existing extension install refs/artifacts, bundled resources and bridge setup remain preserved. Official extension authoring guidance already covers inputs/auth/default scope. No additional native tool, workspace, credential or database change in this slice. Site UI integration and its focused checks are in progress; ET-site-marketplace-catalog and MS-marketplace-catalog-live-config retain final09/10 live QA ownership.

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

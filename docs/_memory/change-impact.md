# Compozy Change Impact

Use for changed runtime behavior, public contracts, config, or feature documentation. Record this analysis once in the owning spec/task/PR and link it from implementation slices; update only the affected entries. A small change needs only concise findings. Editorial changes with no runtime contract may state `not applicable — editorial only`.

- **Native tools:** changed `compozy__*` IDs, toolsets, descriptors, schemas/digests, risk flags, capability gates, diagnostics, and CLI/API fallbacks.
- **Extensibility and hooks:** extensions, hooks, skills/capabilities, resources, registries, bridge SDKs, MCP sidecars, and config lifecycle. Config changes co-ship defaults, loader/overlay behavior, validation, docs, and compatibility under SD-013.
- **Workspace data isolation:** classify changed data as global/workspace/session/agent-scoped. Follow `workspace_id` through affected CLI/HTTP/UDS/core/store/web/cache/SSE/event paths and verify the owning boundary prevents cross-workspace leakage.
- **Official Compozy skill:** update `skills/compozy/` when public behavior, tools, CLI paths, hooks, capabilities, resources, or memory/network/task semantics change.
- **Web/Docs impact:** name affected `web/` routes/components/hooks and `packages/site` docs, plus their verification owner. Backend changes carry this analysis with the feature.

For an unaffected entry, name the checked surfaces and why the change cannot affect them. Do not create separate artifacts or re-audit unchanged surfaces for every checkpoint. Breaking changes also name delete targets and the user-state/public/internal compatibility regime; apply SD-013 before deletion.

## Marketplace catalog — one kind, plugin marketplaces as sources

Owning spec: `.compozy/tasks/marketplace-catalog/_spec.md` (Part II §Impact Analysis lists delete targets and regimes; ADR-001/005/007/008 record the SD-013 ladder outcomes; peer review rounds 1 and 2 incorporated 2026-09-10). Implementation slices link here and update only affected entries.

- **Native tools:** `compozy__marketplace_search` keeps its ID; its `kind` argument keeps working one release (`mcp` = extensions that provide servers, `skill` = the fenced skills listing; descriptor deprecation, removal v0.6.0) and the response gains `revision`/`stale`/`installable`/`trust.decision`. `compozy__extensions_install` accepts `source: marketplace`, `inputs`, and `expected_digest` (the approved acquisition). New read-only `compozy__marketplace_sources` (experimental one release). `compozy__extensions_*` and `compozy__mcp_status|mcp_auth_status` gain the optional `owner` argument. `compozy__skill_list|view|search` unchanged. Tool catalog and digests regenerate.
- **Extensibility and hooks:** extension manifest gains `[[inputs]]` (shared grammar with the feed) and `resources.mcp_servers.<name>.auth`; `env`/`secret_env` keep their map shape (binding name = input id). Agent Plugins loader locates `.claude-plugin/`, `.codex-plugin/`, `.cursor-plugin/` manifests and decodes client grammars through adapters (skills + `mcpServers`; commands/agents/hooks reported as ignored); `layout`, `source_name`, `source_ref`, `entry_id`, `resolved_ref` recorded in provenance. Extension-provided MCP servers carry `owner = extension:<name>` through publication, status, auth tokens and OAuth registrations, a disjoint vault namespace (`vault:mcp/ext/<name>/…`), and overrides (`extension_mcp_overrides`, which also persists the once-allocated runtime name); manual servers keep `owner = manual` and their vault refs; settings/auth routes resolve owner-less names by runtime name; additive `mcp_server_name_taken` validation. Input readiness is typed over stored input state (`missing_inputs`); dropped inputs stay inactive, never deleted. Hooks, bridge SDKs, MCP sidecars, capability registry unchanged. Config: new `[[marketplace.plugin_sources]]` (name · source · enabled) with preset overlay from `v3/marketplaces.json`; `skills.marketplace.registry`, `skills.marketplace.base_url`, `skills.allowed_marketplace_mcp` consumed-and-warned one release by the fenced skills marketplace path, removed v0.6.0. Feeds: new `v3/extensions.json` + `v3/marketplaces.json` (`manifest_version: 3`, `icon`, `inputs`); the root family (`extensions.json` v2, `mcp.json`, `skills.json`) stays published from the same generator until v0.6.0 for released daemons; upgraded daemons read `v3/` and fall back to the root with a warning.
- **Workspace data isolation:** catalog projection is global (per daemon home), keyed `(source, entry_id)` with per-source generation, plus a global content-addressed package cache under the daemon home (capped, swept at refresh commit); installed state is joined per request by acquisition origin (`source_ref`, `entry_id`) from the profile/workspace-scoped extension inventory; provenance origin is backfilled only where catalog evidence exists; input values (`extension_inputs`) and server overrides are scoped to the extension instance's profile/workspace; extension-provided MCP servers keep the extension instance's scope and their own owner-qualified auth tokens; nothing crosses workspaces. Migration `00109` re-keys the projection, drops rows for the retired kinds (derived cache; sign-off in ADR-001; release-note migration block), backfills provenance origin (catalog-evidenced rows only) and `owner = manual` on tokens and OAuth registrations, and creates the two new tables.
- **Official Compozy skill:** `skills/compozy/references/tools-and-skills.md` §Marketplace Discovery (no kinds, `marketplace sources` experimental, install with `--input`, retained verbs still working with warnings until v0.6.0, `extension_source_changed`/`extension_name_conflict`/`marketplace_cursor_stale`) and `references/extensions.md` (inputs block, client layouts, marketplace sources, owner-qualified servers).
- **Web/Docs:** `web/src/systems/marketplace/**` (one page, Installed page, entry card/logo/trail, add menu, marketplace dialog, brand registry, cursor restart), `web/src/systems/settings` (Marketplace page with the experimental label; Skills "Marketplace URL" row deleted), routes `/marketplace`, `/marketplace/installed`, `/marketplace/$entryId` with one-release redirects from kind paths. `packages/site`: `lib/marketplace-catalog.ts` validates `v3/` and the retained root family (build breaks otherwise); docs `marketplace/`, `extensions/install.mdx`, `cli/marketplace`, `api/marketplace.mdx`, `configuration/config-toml.mdx` rewritten with both shapes during the window; `skills/marketplace.mdx` marked retired (deleted v0.6.0). Verification owners: `_tests.md` (UT/IT/E2E), `eng-ui-screenshot` bundle for the OD boards, QA scenarios `ET-web-marketplace-*`, `ET-api-mcp-catalog-install`, `ET-cli-marketplace-search`, `ET-site-marketplace-catalog` reset to `untested`; charter `CH-marketplace-installed-default` retired.

## Issue 595 — Goal lifecycle and attention

Owning delivery: [PR #601](https://github.com/compozy/compozy/pull/601). Detailed regression and runtime evidence: [focused QA report](../qa/reports/2026-09-10-issue-595-goal-lifecycle.md).

- **Native tools:** Existing Goal control/read, session stop/removal, and Loop cancellation surfaces keep their IDs, schemas, and authorization. Run terminal state and quarantine govern Goal status. Stopping a session cancels its session-origin Goals; failed cancellation remains retryable in the existing stop settlement receipt.
- **Extensibility and hooks:** No hook, SDK, configuration, or registry shape changes. Cancellation uses the existing aggregate and durable cleanup outbox. Catalog Loop lineage and window dismissal retain their semantics.
- **Workspace data isolation:** Binding adoption validates workspace, task, control, phase, session, handle, and epoch. Session Goal cancellation reads immutable profile/workspace scope. Existing receipts preserve session/runtime identity across restart. No schema migration or historical record deletion.
- **Official Compozy skill:** `skills/compozy/references/loops.md` documents cancellation, orphan recovery, context ownership, and bounded supervision freshness.
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
- **Official Compozy skill:** Runtime Gateway guidance explains embedded private proof and safe diagnostic classes.
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

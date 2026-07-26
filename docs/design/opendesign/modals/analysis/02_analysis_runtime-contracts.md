# Analysis: runtime-contracts

> Research snapshot for field/API truth. Living modal UI authority: `../MODAL-STANDARD.md` + 16 surfaces in `../`. Re-check `internal/api/contract` before production binding.

Read-only exploration of the slice `runtime-contracts` (ordinal `02`) for the research prompt:

> Alinhar o padrão de modal de criação/edição (como os já feitos para task, trigger e job) para as demais páginas/módulos principais que têm ação de adicionar/editar. Explorar o projeto com sub-agentes para identificar quais modais existem, quais dados/integrações de backend eles têm (para não implementar algo que não existe), e depois criar os designs. Diminuir a complexidade; onde o modal for muito denso, criar visualização simples/avançada e habilitar o "usuário final" a cadastrar.

## Scope

- Slice question: Map the real daemon-backed create/update contracts for core entities the Web UI can manage. For each entity: request fields, required/optional semantics, enums/defaults, validation, related lookup data/integrations, route + CLI/UDS parity, create/edit asymmetry. Flag secrets/unsupported concepts designs must not expose. Classify scope global/workspace/session/agent.
- Primary sources: `internal/api/contract/*`, `internal/api/core/*`, `internal/api/httpapi/*routes*.go`, domain packages under `internal/{automation,bridges,task,vault,resources,network,notifications}`, CLI parity files under `internal/cli/*`.
- Sources read in full vs. sampled: Read in full — `contract/automation.go`, `automation/model/types.go`, `contract/bridges.go`, `contract/bundles.go`, `contract/notifications.go`, `contract/extensions.go`, `contract/vault.go`, `contract/loops.go`, `contract/agent_definitions.go`, `contract/settings_mutations.go`, `contract/settings_collection_payloads.go`, `contract/settings_config_payloads.go`, `contract/mcp_catalog_install.go`, `contract/resources.go`, `httpapi/routes.go`, `httpapi/loops_routes.go`, `httpapi/agent_routes.go`. Sampled — `contract/contract.go` (session/agent/network/workspace request regions), `contract/tasks.go` (create/update region), `bridges/types.go`, `task/types.go`, `task/validate.go`, `store/types_network_channel.go`, CLI file inventory in `internal/cli/`.
- Total candidate sources surveyed: ~45 create/update request structs (enumerated below) across the contract package plus route registration + domain enum/validation sources.

## Overview

AGH exposes create/update for its manageable entities as **shared handler contracts** in `internal/api/contract` — the same request struct feeds both HTTP (web UI, `internal/api/httpapi`) and UDS (CLI, `internal/api/udsapi`), so every entity that has a modal already has CLI parity by construction (`internal/CLAUDE.md`: "`internal/api/core` is the canonical handler home … HTTP and UDS only choose registration and authentication"). The three reference redesigns (task/trigger/job) map to `POST/PATCH /api/tasks`, `POST/PATCH /api/automation/triggers`, `POST/PATCH /api/automation/jobs`. Their contracts (`contract/tasks.go:609-672`, `contract/automation.go:178-276`) show exactly why a **simple/advanced split** was introduced: the create struct carries a small required core (title/name, agent, prompt, schedule/event) plus a large tail of scheduling, retry, fire-limit, scope, owner, and metadata fields that only a power operator needs.

The operator's target — "align other main pages to this level" — has a concrete, finite backend surface. The full inventory of daemon-backed create/edit entities the Web UI can manage is: **agent definition**, **bridge instance** (+ secret bindings), **notification preset**, **network channel**, **loop** (definition + config override), **bundle activation**, **extension install**, **workspace**, **session start**, and the settings-collection entities **provider**, **MCP server**, **sandbox profile**, **hook**, plus **vault secret** and the generic **resource** upsert. Each has a stable request struct, explicit required/optional semantics (pointer fields = optional patch), and validation in its domain package.

The single most load-bearing thing to act on from this slice is the **secret and scope discipline**: several of these entities carry write-only secret fields that the response deliberately redacts (`webhook_secret_value`→`webhook_secret_present`/`webhook_secret_hash`; vault `secret_value`→`present`; provider/MCP/bridge secrets→`*_ref` + presence). A redesigned modal MUST render a write-only "set/rotate secret" affordance and a "•••• present" read state — never a value field pre-filled from the API, because the API never returns the value. Equally load-bearing: **provider auth-mode** (`native_cli` vs `bound_secret`) gates whether credential-slot inputs may appear at all (`internal/CLAUDE.md` Provider auth boundary).

A second actionable finding: not every "add/edit" is a free-form form. **Bundle activation** and **MCP catalog install** are *catalog-pick + fill-required-values* flows (with a preview/next-step step), and **loop** authoring is a graph document with server-side lint/compile (422 diagnostics), not a flat form. Those need a different modal archetype than task/trigger/job.

This slice overlaps with slice 01 (web modal inventory — which UI modals exist today) and any slice covering the design tokens/HTML output. It is the authority for *what fields are real*; it does not decide *which entities get modals first* (a product call) or *what the modal looks like* (design).

## Mechanisms / Patterns

- **Shared contract → dual transport (parity by construction):** every request struct in `internal/api/contract` is consumed by both `httpapi` and `udsapi` route registrations; `internal/cli/*` has a command file per entity (`agent_mutate.go` `create`, `notifications.go` `create`, `bridge.go`, `automation.go`, `loop.go`, `vault.go`, `mcp_install.go`, `provider.go`, `bundle.go`, `extension.go`). Designs can assume any modal field is also settable via CLI/UDS. See `httpapi/routes.go`, `httpapi/loops_routes.go`, `httpapi/agent_routes.go`.
- **Create vs Patch asymmetry via pointer fields:** create structs use value fields with a required core; update structs use `*T` pointers + a `HasChanges()` guard so an empty patch is rejected (`contract/automation.go:209-221,261-276`; `contract/tasks.go:661-672`). Edit modals must send only changed fields.
- **Immutable-on-edit identity fields:** several updates deliberately omit fields that create requires — bridge update cannot change `platform`/`extension_name`/`scope` (`contract/bridges.go:136-145`); network channel update only mutates `purpose`/`fanout_policy`/`coordinator_peer_id`, not name or `agent_names` (`contract/contract.go:716-720`); workspace update omits `root_dir` (`contract/contract.go:1146-1151`). Edit modals must lock those fields.
- **Optimistic concurrency tokens:** agent update requires `expected_digest` (`contract/agent_definitions.go:14-18`), loop patch takes `expected_version` and returns `LoopVersionConflictResponse` on CAS miss (`contract/loops.go:100-103,124-128`), resource put takes `expected_version` (`contract/resources.go:11-15`). Edit modals must round-trip the version/digest and surface a 409 conflict.
- **Write-only secret + redacted read (repeat pattern):** trigger `webhook_secret_value` (write) → `webhook_secret_present`+`webhook_secret_hash` (read) `contract/automation.go:86-89,132-139,239,257`; vault `secret_value` (write) → `present` (read) `contract/vault.go:12-19,31-55`; bridge secret binding `secret_value` write-only `contract/bridges.go:148-155`; provider `SettingsProviderSecretWritePayload.Value` (write) → credential slot `secret_ref`+`present` (read) `contract/settings_collection_payloads.go:25-41,64-69`; MCP `secret_env`/`oauth_client_secret` write-only via `SettingsMCPSecretValuesPayload` / `SettingsMCPSecretInputPayload` (`contract/settings_mutations.go:39-42`, `contract/mcp_catalog_install.go:11-21`).
- **Secret-rejecting free-JSON payloads:** bridge `provider_config` is raw JSON but `validateBridgeProviderConfigPayload` rejects any embedded token/secret (`ErrUnsafeBridgeProviderConfigPayload`, `contract/bridges.go:14-17,506-522`); `delivery_defaults` is whitelisted to `peer_id/thread_id/group_id/mode` only (`contract/bridges.go:524-578`). Secrets must go through secret *slots/bindings*, never the config blob.
- **Provider-declared dynamic form drivers:** bridge create is parameterized by the selected provider — `GET /api/bridges/providers` returns `BridgeProviderPayload{ SecretSlots[], ConfigSchema }` (`contract/bridges.go:399-411`; `bridges/types.go:246-270`). The provider's `secret_slots` enumerate which secret bindings the modal must collect; `config_schema` is only a schema-hint reference/version (not field descriptors).
- **Catalog-pick install flows (not free forms):** bundle activation = `{extension_name, bundle_name, profile_name, scope, workspace}` selected from `GET /api/bundles/catalog`, with `POST /api/bundles/preview` returning the would-create inventory (`contract/bundles.go:114-121,16-94`); MCP install = `{entry_id, name, scope, values}` from a catalog feed with a `next_step: authorize` OAuth follow-up (`contract/mcp_catalog_install.go:23-36`); extension install = path/checksum/slug/source + `allow_unverified` trust gate (`contract/extensions.go:6-21`).
- **Server-side validate-before-save:** loop has dedicated `POST …/loops/:name/validate` returning `LoopValidationResponse{valid, errors[]LoopLintErrorPayload}` (per-node code/severity) and a dry-run `POST …/run` preview (`contract/loops.go:105-159`; `httpapi/loops_routes.go:37-38`); agent has `POST /api/agents/:name/soul/validate`; bridge has `POST /api/bridges/:id/test-delivery`. Modals should offer a "validate/preview" action distinct from save.
- **Scope discriminator + workspace binding rule:** `scope ∈ {global, workspace}` with the invariant "workspace_id required iff scope=workspace, forbidden iff scope=global" recurs across task (`task/validate.go:1223-1240`), automation (`automation/model/validate.go:168-174`), bridges (`bridges/catalog.go:128-134`). The modal's scope toggle must show/hide the workspace picker accordingly.

## Relevant Sources

- `internal/api/httpapi/routes.go:176-240` — task/scheduler/task-run routes (reference).
- `internal/api/httpapi/loops_routes.go:5-53` — automation job/trigger routes (`POST/PATCH /api/automation/{jobs,triggers}`) + loop routes (`POST/PATCH /api/workspaces/:id/loops`).
- `internal/api/httpapi/agent_routes.go:25-46` — agent CRUD (`POST /api/agents`, `PUT /api/agents/:name`, `POST /:name/duplicate`, soul/heartbeat sub-resources).
- `internal/api/httpapi/routes.go:71-89` — bridge routes (`POST /api/bridges`, `PATCH /:id`, `PUT /:id/secret-bindings/:name`).
- `internal/api/httpapi/routes.go:91-99,101-109,319-344,346-369,371-431` — notification presets, workspaces, network channels, bundles/extensions, settings (providers/mcp/sandboxes/hooks/vault).
- `internal/api/contract/automation.go:48-92,178-276` — Job/Trigger payloads + Create/Update requests + webhook-secret redaction (reference pattern).
- `internal/automation/model/types.go:9-13,18-67,116-196` — automation enums/defaults (`Scope`, `TargetKind`, `ScheduleMode`, `RetryStrategy`, `RetryConfig`, `FireLimitConfig`, `ScheduleSpec`, `LoopTarget`, `JobTaskConfig`), `DefaultTimezone=UTC`, `DefaultMaxConcurrentJobs=5`.
- `internal/api/contract/tasks.go:600-672` — CreateTaskRequest / CreateTaskChildRequest / UpdateTaskRequest (reference).
- `internal/task/types.go:12-65` + `internal/task/validate.go:20-119,1223-1335` — task `Scope{global,workspace}`, `Priority{low,medium,high,urgent}`, `ApprovalPolicy{none,manual}`, scope/workspace validation.
- `internal/api/contract/contract.go:21-30,321-366,706-720,1136-1168` — CreateSessionRequest, CreateAgentRequest/CreateAgentPayload (+ `AgentMCPServerJSON.secret_env`), Create/UpdateNetworkChannelRequest, Create/UpdateWorkspaceRequest.
- `internal/api/contract/agent_definitions.go:13-48` — UpdateAgentRequest (`expected_digest`), DuplicateAgentRequest/Overrides, DeleteAgentResponse.
- `internal/api/contract/bridges.go:73-155,211-237,399-432,506-578` — Create/UpdateBridgeRequest, PutBridgeSecretBindingRequest, provider payload w/ secret_slots+config_schema, secret-rejecting validators.
- `internal/bridges/types.go:51-53,167-171,246-293,295-311` — bridge `Scope`, `BridgeDMPolicy{open,allowlist,pairing}`, `BridgeSecretSlot`, `BridgeProviderConfigSchema`, `RoutingPolicy{include_peer,include_thread,include_group}`, `DeliveryMode`.
- `internal/api/contract/notifications.go:11-62` — NotificationTargetPayload (bridge_id + delivery_mode), Create/UpdateNotificationPresetRequest, built-in/user-modified metadata.
- `internal/api/contract/loops.go:9-135,167-211,319-456` — LoopSource/Status/enums, CreateLoopRequest (definition|fork), PatchLoopRequest (expected_version), PutLoopConfigRequest/LoopConfig, LoopDefinitionDocument (graph).
- `internal/api/contract/bundles.go:16-125` — bundle catalog/preview payloads, ActivateBundleRequest, UpdateBundleActivationRequest.
- `internal/api/contract/extensions.go:6-21` — InstallExtensionRequest / UpdateExtensionRequest (`allow_unverified`).
- `internal/api/contract/vault.go:12-55` — VaultSecretPayload (redacted), PutVaultSecretRequest (write-only `secret_value`) + Validate.
- `internal/api/contract/settings_mutations.go:33-50` + `settings_collection_payloads.go:9-218` — PutSettingsProvider/MCPServer/Sandbox/Hook requests + provider/MCP/sandbox/hook payloads (auth_mode, credential_slots, secret_env, env_policy/home_policy, hook event/matcher).
- `internal/api/contract/mcp_catalog_install.go:11-36` — MCP catalog install (entry_id + write-only secret inputs, `next_step`).
- `internal/api/contract/resources.go:10-33` — PutResourceRequest (`expected_version`, spec JSON).
- `internal/api/contract/settings.go:126-128` — `SettingsPermissionMode{deny-all,approve-reads,approve-all}`; `internal/codegen/sdkts/generate_test.go:54` — ReasoningEffort `{none,minimal,low,medium,high,xhigh,max}`.
- `internal/store/types_network_channel.go:28-32` — network `FanoutPolicy{capability_match,coordinator,all_members}`.

## Transferable Patterns

- **Simple/Advanced split, formalized by the contract** → applies to every dense entity because the contract already separates a required core from optional tails. Concrete "simple vs advanced" partitions the design can adopt verbatim:
  - **Task** (reference): simple = `title`, `description`, `priority`, `scope`(+workspace); advanced = `network_channel`, `max_attempts`, `approval_policy`, `owner`, `auto_enqueue_on_ready`, `draft`, `metadata` (`contract/tasks.go:609-625`).
  - **Trigger**: simple = `name`, `agent_name`, `prompt`, `event`, `scope`; advanced = `filter`, `target_kind`/`loop_target`, `retry`, `fire_limit`, webhook (`webhook_id`, `endpoint_slug`, `webhook_secret_value`), `enabled` (`contract/automation.go:224-240`).
  - **Job**: simple = `name`, `agent_name`, `prompt`, `schedule{mode,expr|interval|time}`; advanced = `task`, `loop_target`, `retry`, `fire_limit`, `enabled` (`contract/automation.go:178-191`).
  - **Agent**: simple = `name`, `provider`, `model`, `prompt`; advanced = `command`, `reasoning_effort`, `tools`/`toolsets`/`deny_tools`, `permissions`, `category_path`, `skills.disabled`, `mcp_servers` (`contract/contract.go:329-347`). This is the densest form and the clearest win for an "end-user simple" mode.
  - **Bridge**: simple = provider pick + `display_name` + required `secret_slots`; advanced = `routing_policy`, `dm_policy`, `provider_config`, `delivery_defaults`, `notification_suppress` (`contract/bridges.go:74-86`).
- **Write-only secret affordance** → applies to trigger (webhook), bridge (secret bindings), provider (credential slots), MCP (secret_env + OAuth), sandbox/hook (secret_env), vault. Reuse one component: "•••• present · [Rotate]" for existing, "[Set value]" for new; never a pre-filled password box. Backed by `contract/vault.go:12-19`, `contract/automation.go:86-89`, `contract/settings_collection_payloads.go:25-41`.
- **Scope toggle + conditional workspace picker** → applies to task, trigger, job, bridge, provider, and any global/workspace entity; the invariant is identical everywhere (`task/validate.go:1223-1240`, `automation/model/validate.go:168-174`). One reusable `ScopeField` composite.
- **Provider/catalog-driven dynamic subform** → bridge create renders required secret slots from the chosen provider (`GET /api/bridges/providers`); bundle/MCP/extension install render from catalog entries. Reuse a "select source → fill required values → preview" archetype distinct from the flat-form archetype.
- **Validate/preview action separate from Save** → loop (`/validate`, dry-run `/run`), agent soul (`/soul/validate`), bridge (`/test-delivery`), bundle (`/preview`). Modals for these should show inline server diagnostics (e.g. `LoopLintErrorPayload{node_id,code,severity}`) before commit.
- **Lookup selectors already have endpoints** (so designs won't invent data): agents `GET /api/agents` + `/catalog`; providers/models `GET /api/providers`, `/api/model-catalog`, `/api/settings/providers`; bridges `GET /api/bridges` (+ `/providers`); channels `GET /api/workspaces/:id/network/channels`; sandboxes `GET /api/settings/sandboxes`; loops `GET /api/workspaces/:id/loops`; tools/toolsets `GET /api/tools`,`/api/toolsets`; skills `GET /api/skills`; vault refs `GET /api/vault/secrets/metadata`; hook events `GET /api/hooks/events`; bundle/MCP catalogs `GET /api/bundles/catalog`, MCP catalog feed.

## Risks / Mismatches

- **Never render a returned secret value.** Every secret is write-only; the read side returns presence/hash/ref only. A modal that shows a value field populated from GET will always be empty and will imply the daemon stores retrievable plaintext — false. Constraint: `internal/CLAUDE.md` "claim_token redaction is non-negotiable … secret bindings MUST NEVER appear in … web UI". Sources: `contract/vault.go:12-19`, `contract/automation.go:86-89`.
- **Provider credential slots are auth-mode-gated.** For `auth_mode = native_cli`, the modal MUST NOT show AGH credential-slot/secret inputs (provider owns login via `auth_login_command`); only `bound_secret` exposes `credential_slots` + secret writes. Rendering slots for native_cli violates the Provider auth boundary (`internal/CLAUDE.md`; `contract/settings_collection_payloads.go:9-41,51-62`).
- **Bridge secrets must not be typed into `provider_config`.** The contract actively rejects secret-shaped JSON there (`ErrUnsafeBridgeProviderConfigPayload`, `contract/bridges.go:506-522`). A generic "paste provider JSON" advanced field invites a rejected payload; route secrets to the provider's declared `secret_slots` → secret-binding PUT instead.
- **Loop is not a flat modal.** `CreateLoopRequest` is a whole `LoopDefinitionDocument` (graph nodes/edges/contract) or a `fork_from_name`, with server lint/compile producing per-node 422s and CAS on patch (`contract/loops.go:94-135,319-456`). Forcing loop authoring into a task-style form would misrepresent the runtime; the modal-scale surface for loops is `PutLoopConfigRequest`/`LoopConfig` (gate/iteration/budget/model overrides, `contract/loops.go:167-211`) — a settings-style form, not full authoring.
- **Bundle "edit" is essentially non-editable.** `UpdateBundleActivationRequest` only toggles `bind_primary_channel_as_default` (`contract/bundles.go:123-125`); there is no field-level edit of an activation. An "edit bundle" modal promising more than that would render controls the runtime doesn't support (`CLAUDE.md` "Truthful UI > plausible UI").
- **Immutable identity on edit.** Bridge platform/extension/scope, network channel name/members, workspace root_dir, task scope are create-only. Edit modals reusing the create layout must disable those inputs, or they imply mutability the PATCH contract rejects (`contract/bridges.go:136-145`, `contract/contract.go:716-720,1146-1151`).
- **Optimistic-concurrency conflicts need UI.** Agent (`expected_digest`), loop (`expected_version`→409 `LoopVersionConflictResponse`), resource (`expected_version`) can reject a stale edit. A modal without a conflict/refresh path will silently fail or clobber (`contract/agent_definitions.go:14-18`, `contract/loops.go:100-103,124-128`).
- **"End-user simple mode" for agent/provider/bridge still touches operator-only concepts.** `permissions{deny-all,approve-reads,approve-all}`, `env_policy`/`home_policy`, `reasoning_effort`, `deny_tools` are security/operator surfaces; a simplified end-user path should choose safe defaults and hide these, not relabel them (`contract/settings.go:126-128`, `contract/settings_collection_payloads.go:9-23`).
- **Session "create" is a start action, not a persisted entity edit.** `CreateSessionRequest` (`contract/contract.go:21-30`) spawns a run; there is no session "edit" modal — treat it as a launcher, not a CRUD form.

## Open Questions

- Which entities are in-scope for *this* redesign wave vs. deferred? The backend supports ~16 create/edit surfaces; prioritization (task/trigger/job are done) is a product decision this slice can't make. (Likely first tier by user-facing frequency: agent, bridge, notification preset, network channel, workspace.)
- Do "settings-collection" entities (provider/MCP/sandbox/hook) render as modals or as inline settings-page forms today? The contract supports both; slice 01 (web modal inventory) should confirm current UI surface before this slice's field maps are bound to a modal.
- Is there an existing web endpoint powering the bridge provider-config *field* rendering, or is `provider_config` today a raw JSON textarea? `BridgeProviderConfigSchema` is only a schema-hint (`schema`/`version` strings), not field descriptors (`bridges/types.go:266-270`) — designing a structured provider-config form may require a schema resolver that this slice did not find.
- Exact HTTP method/path for session creation and memory-note create/edit were not confirmed in the sampled route files (session routes live in `httpapi/session_routes.go`, not read in full); confirm before treating them as modal entities.
- UDS parity was inferred from `internal/cli/*` command-file presence and the shared-contract architecture rule, not by reading `udsapi/routes.go` in full (490 lines, sampled by size only). A parity spot-check per entity is advisable if a design asserts CLI equivalence.

## Evidence

- `internal/api/httpapi/routes.go`
- `internal/api/httpapi/loops_routes.go`
- `internal/api/httpapi/agent_routes.go`
- `internal/api/udsapi/routes.go`
- `internal/api/contract/automation.go`
- `internal/automation/model/types.go`
- `internal/automation/model/validate.go`
- `internal/api/contract/tasks.go`
- `internal/task/types.go`
- `internal/task/validate.go`
- `internal/api/contract/contract.go`
- `internal/api/contract/agent_definitions.go`
- `internal/api/contract/bridges.go`
- `internal/bridges/types.go`
- `internal/bridges/catalog.go`
- `internal/api/contract/notifications.go`
- `internal/api/contract/loops.go`
- `internal/api/contract/bundles.go`
- `internal/api/contract/extensions.go`
- `internal/api/contract/vault.go`
- `internal/api/contract/settings_mutations.go`
- `internal/api/contract/settings_collection_payloads.go`
- `internal/api/contract/settings_config_payloads.go`
- `internal/api/contract/settings.go`
- `internal/api/contract/mcp_catalog_install.go`
- `internal/api/contract/resources.go`
- `internal/store/types_network_channel.go`
- `internal/codegen/sdkts/generate_test.go`
- `internal/cli/` (agent_mutate.go, automation.go, bridge.go, notifications.go, loop.go, vault.go, mcp_install.go, provider.go, bundle.go, extension.go)
- `internal/CLAUDE.md` (Security Invariants — claim_token/secret redaction; Provider auth boundary; "internal/api/core is the canonical handler home")

---
name: 02-config-settings
description: QA child report — TOML config parsing, hot-apply vs restart-required, settings overlay, workspace resolution, agent-local config, secret/vault redaction, multi-workspace isolation.
type: qa-child
module: config-settings
sources:
  - internal/config
  - internal/settings
  - internal/workspace
  - internal/vault
  - internal/frontmatter
---

# Config + Settings + Workspace + Vault — Final QA Plan

## 1. Module Surface

This module owns the daemon configuration plane: how `~/.agh/config.toml` (plus optional workspace overlays, MCP JSON sidecars, and `AGENT.md` definitions) becomes a validated runtime `Config`, how mutations are persisted (CLI + HTTP/UDS settings service), how secrets are stored encrypted, how workspaces resolve their effective view of agents/skills/config, and how all of this is redacted before reaching any operator-visible surface.

### 1.1 Filesystem layout (the home root)

`internal/config/home.go:11-37` declares the on-disk vocabulary used everywhere downstream:

- Home root: `~/.agh/` (env override `AGH_HOME`) — `internal/config/home.go:58-75`.
- `config.toml` (the canonical TOML file) — `internal/config/home.go:25,100`.
- `agents/<name>/AGENT.md` — `internal/config/home.go:13,36`, `internal/config/agent.go:78`.
- `skills/<name>/SKILL.md` — `internal/workspace/scanner.go:18`.
- `memory/`, `sessions/`, `restarts/`, `logs/` — `internal/config/home.go:15-23`.
- `agh.db` (catalog), `daemon.sock`, `daemon.lock`, `daemon.json` — `internal/config/home.go:25-31`.
- `logs/agh.log`, `logs/network.audit` — `internal/config/home.go:33-35`.
- `vault.key` (0600 daemon-local AES-256 key) — `internal/vault/crypto.go:38`.

Workspace overlay layout: `<workspace>/.agh/config.toml`, `<workspace>/.agh/agents/<n>/AGENT.md`, `<workspace>/.agh/skills/<n>/SKILL.md`, `<workspace>/.agh/mcp.json`, `<workspace>/.env` — driven by `WorkspaceDiscoveryRoots()` in `internal/config/agent.go:117-146` and the workspace scanner (`internal/workspace/scanner.go:48-83`).

### 1.2 Top-level config sections (`internal/config/config.go:247-269`)

`Config` is a flat composition of typed sections. Every section is mergeable from at least two layers (default → global → workspace). Validation lives next to the type:

| Section | Type / file | Validation | Notable defaults |
|---|---|---|---|
| `daemon` | `DaemonConfig` `config.go:33-36` | `Validate()` rejects empty socket — `config.go:774-781` | `socket = ~/.agh/daemon.sock` `config.go:418-421` |
| `http` | `HTTPConfig` `config.go:38-42` | host non-empty, port 1..65535 — `config.go:783-793` | `host=localhost port=2123` `config.go:423-426` |
| `defaults` | `DefaultsConfig` `config.go:44-49` | agent required — `config.go:795-802` | `agent="general"` `config.go:427-429,111` |
| `agents.soul` | `SoulConfig` `config.go:57-62` | bytes positive; projection ≤ body — `config.go:840-858` | `enabled=true; max=32768; projection=2048` `config.go:805-811` |
| `agents.heartbeat` | `HeartbeatConfig` `config.go:64-78` | interval bounds + ≤1MiB body cap — `config.go:861-919` | `min=5m default=30m cooldown=1m wakes=25` `config.go:813-829` |
| `limits` | `LimitsConfig` `config.go:80-84` | both positive — `config.go:921-931` | `max_sessions=10; max_concurrent_agents=20` `config.go:434-437` |
| `session.limits` | `SessionLimitsConfig` `config.go:92-95` | `timeout >= 0` — `config.go:942-947` | unset = no timeout |
| `session.supervision` | `SessionSupervisionConfig` `config.go:97-104` | warning ≤ timeout, others positive — `config.go:961-995` | `heartbeat=30s, progress=10m, warn=15m, timeout=30m, grace=30s` `config.go:950-958` |
| `permissions.mode` | `PermissionMode` `config.go:107-117` | one of `deny-all`/`approve-reads`/`approve-all` — `config.go:1003-1017` | `approve-all` `config.go:443` |
| `mcp_servers` | `[]MCPServer` `config.go:256` (config) + `mcp.json` sidecar | per-server `Validate()` — `config.go:542-546` | none |
| `providers` | `map[string]ProviderConfig` `config.go:257`, see `provider.go:34-46` | per-provider — `config.go:587-600`, `provider.go:864-...` | empty (built-ins resolved) |
| `sandboxes` | `map[string]SandboxProfile` `config.go:215-225` | backend/sync/persistence enums + secret/env grammar — `config.go:602-619, 654-702` | empty |
| `observability` | `ObservabilityConfig` `config.go:125-130` | retention/bytes positive, nested transcripts — `config.go:1019-1052` | `enabled=true, retention=7d, max=1GiB, probe=2s, segment=1MiB, max_per_session=256MiB` `config.go:447-457,116` |
| `log.level` | `LogConfig` `config.go:141-143` | one of `debug,info,warn,error` — `config.go:1054-1062` | `info` `config.go:458-460` |
| `memory` + `memory.dream` | `MemoryConfig` `config.go:146-159` | nested dream Validate — `config.go:1064-1067` | `enabled=true, dream.enabled=true, agent="general", min_hours=24, min_sessions=3, check=30m` `config.go:461-471` |
| `skills` | `SkillsConfig` `config.go:174-181` | poll positive when enabled, marketplace shape — `config.go:1069-1082` | `enabled=true, poll=3s` `config.go:472-475` |
| `extensions` | `ExtensionsConfig` `config.go:184-187` | resources kinds/scope/rate-limits — `config.go:1084-1126` | empty |
| `tools` | `ToolsConfig` (`tool_surface.go`/`tools.go`) | `Validate(MCPServers, Providers)` — `config.go:569-571` | `DefaultToolsConfig()` |
| `automation` | `AutomationConfig` (`automation.go`) | `validateWithEnv(lookup)` — `config.go:572-574` | `enabled=true, default tz, fire-limit defaults` `config.go:478-483` |
| `hooks` | `HooksConfig` (`hooks.go`) | `Validate()` — `config.go:575-577` | empty |
| `network` | `NetworkConfig` `config.go:204-213` | channel pattern `^[a-z0-9][a-z0-9_-]{0,63}$`, port -1 or 1..65535 — `config.go:1128-1175` | `enabled=true, default_channel="default", port=-1, max_payload=1MiB, greet=30s, replay=300s, queue=100` `config.go:484-492` |
| `autonomy` | `AutonomyConfig` (`autonomy.go`) | `Validate(c)` — `config.go:581-583` | coordinator defaults `config.go:493-495` |

### 1.3 Load + merge pipeline

- Entry points: `config.Load(opts...)` (`config.go:322-350`), `config.LoadForHome(homePaths, opts...)` (`config.go:354-377`). Both expand `WithWorkspaceRoot(root)` and an internal `withoutDotEnv()` / `withoutValidation()` switch.
- Stage order inside `loadWithHome` (`config.go:379-406`):
  1. `DefaultWithHome(homePaths)` (`config.go:418-497`).
  2. `ApplyConfigOverlayFile(homePaths.ConfigFile, &cfg)` — global TOML overlay using pointer-typed overlay structs (`merge.go:16-200+`).
  3. `applyConfigMCPSidecarFile(globalMCPJSONFile(...), &cfg)` — global `mcp.json`.
  4. If a workspace root resolved, repeat the overlay pair against `<root>/.agh/config.toml` and `<root>/.agh/mcp.json`.
  5. `normalizeConfigPaths(&cfg)` — expands `~/`, makes paths absolute (uses `ResolvePath`).
  6. `cfg.validateWithEnv(lookup)` (`config.go:504-521`) walks every section.
- Env / `.env` precedence is a layered lookup: `layeredEnvLookup(processEnvLookup, dotenvLookup)` — process env wins over workspace `.env` (`config.go:283-295, 335-342, 367-374`).

### 1.4 Persistence pipeline (CLI + settings writes)

- `WriteScope` enum: `global` / `workspace` — `persistence.go:24-42`.
- `WriteTargetKind`: `global-config`, `workspace-config`, `global-mcp-sidecar`, `workspace-mcp-sidecar` — `persistence.go:44-57`.
- `ResolveConfigWriteTarget(homePaths, workspaceRoot, scope)` and `ResolveMCPSidecarWriteTarget(...)` are the only path-handing helpers exposed (`persistence.go:91-152`). Higher layers never touch raw filesystem paths.
- `OverlayEditor` is a comment-preserving TOML editor: `SetValue`, `SetTable`, `UpsertArrayTableItem`, `Delete`, `DeleteArrayTableItem`, `HasPath`, `Bytes` — `persistence.go:154-295`.
- `EditConfigOverlay(homePaths, workspaceRoot, target, mutate)` (`persistence.go:296-338`) is the validated writer: parses → mutates → re-validates the *effective* merged config (global+workspace+mcp), and only then writes the new bytes back. Validation failures abort the write, preserving the on-disk file.
- `validateEffectiveConfigWrite(...)` (`persistence.go:340-394`) reapplies `DefaultWithHome → global overlay → global MCP → workspace overlay → workspace MCP`, then `validateWithEnv(lookup)` with `.env` injected.

### 1.5 CLI surface (`internal/cli/config.go`, `vault.go`, `workspace.go`)

- `agh config show [--workspace ROOT]` — full effective config (redacted) — `config.go:182-206`.
- `agh config list [--workspace ROOT]` — flat key/value entries (redacted) — `config.go:208-230`.
- `agh config get <path> [--workspace ROOT]` — one redacted value — `config.go:232-254`.
- `agh config set <path> <value> [--scope global|workspace] [--workspace ROOT]` — validated mutation — `config.go:256-312`. Scalar paths whitelisted in `configScalarMutationKinds` (`config.go:91-163`). Provider scalar paths (`providers.<n>.command|default_model`) — `config.go:1042-1050`. Sandbox + sandbox.network + sandbox.daytona scalar paths — `config.go:1052-1090`.
- `agh config path [--scope ...] [--workspace ROOT]` — resolved paths and selected target — `config.go:314-383`.
- `agh config validate [--workspace ROOT] [--repair-env]` (alias `agh config check`) — re-runs full validate; `--repair-env` runs `RepairDotEnvFile(WorkspaceDotEnvFile(workspace))` first — `config.go:385-445`.
- `agh config edit [--scope ...] [--workspace ROOT]` — opens the selected file in `$VISUAL`/`$EDITOR`, then re-loads to validate — `config.go:447-491`. `requireUnmanagedForMutation` blocks edits when AGH was installed via a managed installer.
- `agh vault list [--prefix vault:NS/...] [--namespace NS]` — `cli/vault.go:29-61`.
- `agh vault get <ref>` — redacted metadata only — `cli/vault.go:63-81`.
- `agh vault put <ref> --kind <kind> --value-stdin` — write-only (`--value-stdin` mandatory) — `cli/vault.go:83-122`.
- `agh vault delete <ref>` — `cli/vault.go:125-142`.
- `agh workspace add <path> [--name N --add-dir D --default-agent A --sandbox S]` — `cli/workspace.go:26-71`.
- `agh workspace list|info <id-or-name>|edit|remove` — `cli/workspace.go:73-235`.

### 1.6 HTTP / UDS settings service

`internal/settings/service.go`, `models.go`, `sections.go`, `collections.go` define a typed orchestration layer that the API layer consumes through one `Service` interface (`models.go:114-121`).

- Sections: `general`, `memory`, `skills`, `automation`, `network`, `observability`, `hooks-extensions` — `models.go:43-58`.
- Collections: `providers`, `mcp-servers`, `sandboxes`, `hooks` — `models.go:61-72`.
- HTTP routes (mounted under `/api/settings`): `/api/settings/<section>` GET+PATCH, plus collection routes — see `internal/api/core/settings_test.go:203,733,756-761,800-930`.
- HTTP vault routes: `GET /api/vault/secrets`, `GET /api/vault/secrets/metadata`, `PUT /api/vault/secrets`, `DELETE /api/vault/secrets` — `internal/api/core/vault_test.go:130-262, 321-324`.
- HTTP log-tail SSE endpoint: `internal/api/core/settings.go:290-340` — emits `SettingsLogTailEventPayload` over SSE with `transport=sse` (`SettingsStreamTransportSSE`).
- `MutationResult.Behavior` ∈ `applied_now` | `restart_required` | `action_trigger` — `models.go:86-96`.
- `MutationResult.RestartRequired bool` JSON-tagged `restart_required` — `models.go:204` and `internal/api/contract/settings.go:60,652`.

### 1.7 Hot-apply vs restart matrix (`internal/settings/classify.go:80-129`)

- `general`, `memory`, `automation`, `network`, `observability` field changes → `restart_required` + `RestartScope=daemon` — `classify.go:87-88, 123-128`.
- `skills.disabled_skills` → `applied_now` (special case — only field that hot-applies) — `classify.go:90-95`.
- All other `skills.*` fields → `restart_required` — `classify.go:96-97`.
- `extensions.*`, `hooks.*` → `restart_required` — `classify.go:99-102`.
- `providers.*`, `mcp-servers.*`, `sandboxes.*`, `hooks.*` collections → `restart_required` — `classify.go:106-113`.
- Section actions: `general.restart`, `memory.consolidate`, `hooks-extensions.{extension-install,extension-enable,extension-disable}` → `action_trigger` — `classify.go:51-77`.

### 1.8 Vault grammar (`internal/vault/types.go`, `service.go`, `crypto.go`)

- Two ref schemes: `env:VAR` (`types.go:95-111`) and `vault:<namespace>/<path>` (`types.go:25-33, 113-148`).
- Eight namespaces: `automation`, `bridges`, `extensions`, `hooks`, `mcp`, `providers`, `sandbox`, `sessions` — `types.go:25-44`.
- Key scheme: AES-256-GCM with random nonce, `aes-gcm:` prefix, base64 payload — `crypto.go:18-19, 95-119`.
- Key source: `AGH_VAULT_KEY` env (base64 / hex / 32-byte raw) overrides `~/.agh/vault.key` (auto-generated 0600) — `crypto.go:43-77`.
- Service: `PutSecret`, `ResolveRef`, `GetMetadata`, `ListMetadata`, `DeleteSecret` — `service.go:69-194`. `ResolveRef` is daemon-internal-only; never returned to operator surfaces.
- `Metadata` struct (the only operator-visible shape) — `types.go:67-74`. Carries `Ref, Kind, Present, CreatedAt, UpdatedAt`. **No plaintext fields.**
- `SecretLikeEnvName` heuristic forbids plain-`env` declarations of `*SECRET*`, `*TOKEN*`, `*PASSWORD*`, `*PASSWD*`, `*API_KEY*`, `*APIKEY*`, `*AUTHORIZATION*`, `*BEARER*`, `*CREDENTIAL*` (`types.go:46-56, 244-269`). Forces them through `secret_env`.
- `ValidateSecretEnvMap(path, namespace, secretEnv)` (`types.go:271-286`) checks `secret_ref` references match the provider/sandbox/etc. namespace.
- Provider credential slot: `name, target_env, secret_ref, kind, required` — `provider.go:25-32`. `secret_ref` validation at `provider.go:760-764`.

### 1.9 Workspace resolver (`internal/workspace/`)

- Resolver caches workspace snapshots keyed by ID, TTL-evicted (`resolver.go:36-83, 333-344`). Cache reuse is content-hashed via `filesnap.Snapshot` of every relevant file (`resolver.go:310-331`).
- Lookup: by ID `ws_*`/`ws-*`, by name, by absolute path; canonicalized via `EvalSymlinks + Abs` (`resolver_crud.go:234-310, 312-354`).
- `ResolveOrRegister(ctx, path)` auto-registers a workspace at `path` if none matches (`resolver.go:165-235`). Rolls back on resolve / change-hook failure with a 2s timeout context (`resolver.go:237-242, rollbackDeleteTimeout`).
- `WorkspaceDiscoveryRoots(rootDir, additionalDirs, homePaths)` returns left-to-right precedence: workspace root → additional dirs → global home — `config/agent.go:117-146`.
- Discovery roots produce per-source `AgentsDir()` and `SkillsDir()` paths. The global root reads `<home>/agents` (no `.agh/` prefix), workspace/additional roots read `<root>/.agh/agents` — `config/agent.go:148-164`.
- Per-agent overrides: `AgentDef.Provider`, `Model`, `Tools`, `Toolsets`, `DenyTools`, `Permissions`, `MCPServers`, `Hooks`, `Capabilities` — `config/agent.go:17-31, 211-295`. Loaded via `LoadAgentDefFile` (`config/agent.go:91-115`) which: reads file → `ParseAgentDef` → `mergeAgentMCPSidecar` (per-agent `mcp.json`) → `LoadAgentCapabilities` (per-agent capability catalog, single-file or directory).
- `AGENT.md` parsing accepts either YAML or TOML frontmatter (`config/agent.go:314-336`); `goccy/go-yaml` strict mode for YAML; `BurntSushi/toml` Undecoded() rejection of unknown keys for TOML.
- `ResolvedWorkspace` is the per-workspace value the daemon hands to sessions / settings / autonomy — `workspace.go:43-51`. Ships a snapshot of `Config`, the merged agents list, the merged skills list, and the resolved sandbox.
- Workspace inference for HTTP requests: `X-AGH-Workspace-ID` header (`agentidentity/identity.go:29`) constrains agent operations to the caller's workspace.

### 1.10 Frontmatter (`internal/frontmatter/`)

`Split([]byte)` and `Decode([]byte, callback)` — `frontmatter.go:28-75`. Normalizes `\r\n` → `\n` (`frontmatter.go:77-83`). Errors: `ErrMissing` (no opening `---`) and `ErrUnterminated` (no closing `---`). The lone parser used by AGENT.md, SOUL.md, HEARTBEAT.md, SKILL.md, capability catalogs, and any other markdown-with-YAML/TOML head.

### 1.11 Env vars touching this module

- `AGH_HOME` — overrides home root (`config/home.go:64`).
- `AGH_VAULT_KEY` — overrides vault key file (`vault/crypto.go:44`).
- `AGH_SESSION_ID`, `AGH_AGENT` — caller identity inside agent sessions (`agentidentity/identity.go:20-22`).
- `HOME` — implicit fallback for both home root and `.agents/skills` (`config/home.go:69, 170-192`).
- `<workspace>/.env` — secondary env source (`config/dotenv.go:75-78`).
- `VISUAL`, `EDITOR` — used by `agh config edit` (`cli/config.go:467`).

## 2. Existing Test Coverage Map

### 2.1 `internal/config/` tests

- `config_test.go` (1700+ LoC): covers cold parse with all sections (`TestLoadValidTOMLConfigWithAllSections:18`), default-with-home (`TestDefaultWithHomeIncludesSoulDefaults:488`), section validation (Soul, Heartbeat, Sandbox, SessionLimits, SessionSupervision, Observability), workspace overrides (`TestLoadWorkspaceOverridesGlobalValues:813`, `…AgentsSoulConfig:923`, `…AgentsHeartbeatConfig:962`), MCP sidecar merge (`TestLoadMergesTopLevelMCPServersAcrossConfigAndJSONSidecars:1043`), unknown-key rejection (`TestLoadRejectsUnknownConfigKeys:1495`, `TestLoadRejectsUnknownSkillsConfigKeys:1519`), `.env` integration (`TestLoadUsesDotEnvForAGHHome:1996`, `…WithoutMutatingProcessEnv:2024`, `TestLoadWithoutDotEnvOptionIgnoresDotEnv:2105`), workspace-precedence (`TestLoadWorkspaceAddsValuesWithoutClobberingGlobal:1313`, `TestLoadWithoutWorkspaceRootIgnoresCurrentDirectoryWorkspaceFiles:1380`).
- `agent_test.go`: AGENT.md parsing (`TestLoadAgentDefFromHomePath:207`), missing/mismatched names (`TestLoadAgentDefRejectsBlankAndMismatchedNames:390`), MCP sidecar merge (`TestLoadAgentDefFileMergesMCPSidecar:339`), workspace precedence with agents (`TestLoadWorkspaceAgentDefsAppliesDocumentedPrecedence:468`), discovery root order (`TestWorkspaceDiscoveryRootsReturnsWorkspaceAdditionalGlobalOrder:418`).
- `agent_capabilities_test.go`: capability TOML/JSON normalization, directory mode discovery, precedence with capabilities.
- `automation_test.go`, `automation_integration_test.go`: workspace overlay merging without clobber, fail-fast on invalid trigger templates.
- `autonomy_test.go`: autonomy overlay precedence, unknown-key rejection (`TestLoadRejectsUnknownAutonomyConfigKeys:210`), no ambient-workspace mutation (`TestLoadAutonomyDoesNotUseAmbientWorkspaceOrMutateEnv:456`).
- `bootstrap_test.go`: home-layout creation.
- `capabilities_test.go`: capability catalog single-file and directory loading.
- `dotenv_test.go`: `.env` parse / repair / sanitize.
- `home_test.go`: `ResolveHomeDir`, `ResolvePath`, `ResolveUserAgentsSkillsDir`.
- `hooks_test.go`, `mcp_resource_test.go`, `mcpjson_test.go`: hooks declarations + MCP JSON sidecar.
- `merge_test.go`: overlay pointer semantics — preserves global value when workspace omits the field; replaces when present.
- `perf_bench_test.go`: micro-benchmark for `Load`.
- `persistence_test.go`, `persistence_integration_test.go`: comment-preserving overlay editor, `EditConfigOverlay` validate-before-write contract, write-target resolution.
- `provider_test.go`: provider validation, alias resolution, credential-slot validation, `secret_ref` rejection of bad shapes.
- `release_config_test.go`: round-trip of release-managed config.
- `tool_grammar.go`/`tool_surface_test.go`/`tools_test.go`: tool config grammar.

**Gap flag — most existing tests are pure Go unit tests. They prove parser/merger/validator correctness on synthesized inputs but never prove that a real provider-backed agent picks up the config the way an operator running `agh config set` expects.** The runtime side of "did the daemon actually re-resolve the agent's effective config after a hot-apply?" is not exercised here.

### 2.2 `internal/settings/` tests

- `service_test.go` (~1700 LoC): full mutation matrix, `restart_required` classification (`:447, 451, 495, 554, 651, 801, 913, 1320, 1452`), action triggers (`:560`), workspace + global scope behavior, vault-secret integration through `ProviderSecrets` mock.
- `service_integration_test.go`: end-to-end through real `aghconfig.Load*` and a real workspace resolver.

**Gap flag — `service_test.go` uses an in-memory `ProviderSecretStore` mock — no real vault encryption path. The daemon-resident store is not exercised in the same test.**

### 2.3 `internal/workspace/` tests

- `resolver_test.go` (1500+ LoC): cache TTL eviction, stale-cache invalidation on file edit, lookup-by-ID/name/path/symlink, root-not-found error path (`ErrWorkspaceRootMissing`), cross-FS path canonicalization.
- `resolver_integration_test.go`: register → resolve → re-resolve flow, change-hook firing on register/unregister/update.
- `workspace_test.go`: workspace registration, name auto-generation collision handling (`UniqueWorkspaceName`).
- `perf_bench_test.go`: resolver throughput.

**Gap flag — no scenario proves that running an actual ACP agent in workspace A cannot read AGENT.md or skills from workspace B.** The cache is keyed by ID, but cross-workspace negative tests are missing.

### 2.4 `internal/vault/` tests

- `service_test.go`: AES-GCM round-trip, env-ref resolution, namespace validation, key-source precedence (`AGH_VAULT_KEY` vs file), 0600-permission assertion (`:353`), supported encodings (raw/hex/base64) (`:302-320`).

**Gap flag — vault never tests "raw secret leaks into a log line / SSE event / HTTP response / CLI output" in a real running daemon. Redaction is asserted in unit tests on the data-shape level only.**

### 2.5 `internal/frontmatter/` tests

- `frontmatter_test.go`: LF vs CRLF normalization, missing/unterminated errors, decode callback, nil-callback rejection, malformed YAML pass-through.

**Gap flag — no fuzz / property tests for: BOM, embedded tabs in keys, mixed delimiter (e.g., `---\t\n`), 4-byte UTF-8 inside metadata, or extremely long frontmatter blocks (DoS protection). Body normalization for trailing whitespace is also not asserted.**

### 2.6 `internal/cli/` tests

- `config_test.go`: scalar-mutation kind matching, redacted fields hidden in stdout, `--scope` parsing, validate-before-write semantics, write-target resolution under `AGH_HOME`.
- `vault_test.go`: PUT does not echo plaintext (`:111`), DELETE returns redacted record, JSON output shape preserved.
- `workspace_test.go`: workspace registration command shapes, error mapping.

**Gap flag — CLI tests do not cover concurrent invocations against the same home, nor do they verify atomic-write semantics (partial-write recovery on power-cut simulation, lock contention with the daemon).**

### 2.7 `internal/api/core/` tests

- `settings_test.go` (~2000 LoC): full HTTP routing for `/api/settings/<section>` GET+PATCH, `restart_required` propagation through to JSON response, `applied_now` for `disabled_skills`, log-tail SSE path.
- `vault_test.go`: list/metadata/put/delete HTTP shapes, namespace filtering, refusing requests without `?ref=` etc.

**Gap flag — HTTP tests run against a Gin engine but the underlying service is a stub. No test exercises the full daemon pipe (HTTP → settings.Service → config.EditConfigOverlay → fs).**

## 3. Coverage Gaps (claims that are not behaviorally proved)

The bullets below are assertions made in code or docs that today have **no end-to-end real-LLM-or-real-daemon test**.

The prior per-agent model-override gap is now covered by
`internal/daemon/daemon_mock_agents_integration_test.go`, which starts a real daemon against the ACP mock and proves the
resolved `AGENT.md` model and reasoning effort reach ACP in protocol order before the first prompt.

1. **Hot-apply for `skills.disabled_skills`** — `classify.go:90-95` declares `applied_now`, but no scenario verifies that an active skill becomes invisible to a running ACP agent inside the same session without a restart.
2. **Restart-required for everything else under `skills.*`** — no scenario forces the API to flag `restart_required: true` and then verifies that a process-level restart picks up the new value while a non-restart leaves the running agent on the old value.
3. **Workspace overlay precedence inside a live session** — no test runs the same `defaults.provider` declared in global → workspace and proves the per-workspace session uses the workspace value.
4. **Multi-workspace memory + skill isolation** — `workspace/resolver.go` keys cache by ID, but no test starts session A in workspace A and session B in workspace B simultaneously and proves no cross-talk in agents/skills/memory/provider catalog.
5. **Secret never appears in any operator-visible surface** — partially proven in unit tests (CLI vault PUT, settings projection), but no scenario walks every surface (`agh config list`, `agh config show`, `/api/settings/...`, `/api/vault/secrets`, SSE log tail, daemon log file, `agh sessions logs`) and asserts a known fake secret string is absent everywhere.
6. **Vault unification flow** — vault namespaces (`providers`, `bridges`, `automation`, `mcp`, `hooks`, `extensions`, `sandbox`, `sessions`) all share one store, but the audit story is not end-to-end proved: a `GET /api/vault/secrets/metadata` for one namespace must not leak presence of refs in another namespace beyond what the prefix filter allows.
7. **Default-value drift** — there is no test that fails when `config.toml` (operator-shipped reference) declares a default that disagrees with `DefaultWithHome(...)`. The example and builtin currently agree on `claude-sonnet-5`; a guard must keep them aligned as model defaults change.
8. **Concurrent `agh config set` against the same key** — no scenario runs two concurrent CLI invocations against the same `AGH_HOME` and verifies a defined ordering or a clean rejection. Standing directive forbids parallelizing config writes against one isolated QA home; this should be an enforcement test, not a documentation rule.
9. **TOML invariant: comment-preserving editor never silently drops user comments** — `OverlayEditor` claims comment preservation; no test runs a config.toml with header / inline / trailing comments through `SetValue`/`Delete` and asserts byte-for-byte preservation outside the targeted region.
10. **`.env` repair never widens scope** — `RepairDotEnvFile` (`dotenv.go:92-160`) refuses symlinks, dirs, and unsupported syntax. No scenario verifies the temp-file fallback survives a crash mid-rename (the os.CreateTemp + os.Rename pattern at `dotenv.go:480-540`).
11. **Workspace path with spaces / unicode / symlinks pointing outside an approved root** — root canonicalization is `EvalSymlinks + Abs` (`resolver_crud.go:329-339`) but no scenario proves a symlink workspace pointing to `/private/var/folders/...` (the macOS canonicalization quirk called out in `internal/CLAUDE.md:57`) round-trips identically.
12. **Frontmatter with BOM, mixed CRLF/LF in body, embedded tabs in key** — `frontmatter_test.go` covers LF/CRLF normalization, but no test injects a BOM (UTF-8 0xEF,0xBB,0xBF) at byte 0; this is a real failure mode for files saved by Windows editors.
13. **Provider config secret resolves through real vault** — the provider credential slot's `secret_ref = "vault:providers/..."` path is documented; no scenario writes a real vault secret, starts a real provider session, and proves the env var is set in the spawned subprocess.

## 4. Real-LLM / Real-Agent Scenarios (CFG-01 .. CFG-16)

Each scenario uses the `qa-scenario` + `qa-flow` anatomy from openclaw. Scenarios mark `live: true` when they require a real ACP agent under a real `agh sessions start`. They mark `live: false` when they exercise the runtime-side daemon API/CLI/persistence layer end-to-end without a real provider call (still a higher bar than the existing unit tests).

Mandatory boilerplate for every scenario (omitted in the per-scenario blocks for brevity):

- All scenarios run inside an isolated `AGH_HOME`, an isolated daemon port (no `:2123` fallback), and an isolated `tmux-bridge` socket per the worktree-isolation directive (`docs/_memory/standing_directives.md`).
- Live scenarios also set `PROVIDER_HOME` / `PROVIDER_CODEX_HOME` from the `bootstrap-manifest.json` (`agh-qa-bootstrap`), per the provider-home isolation directive.
- Every scenario `cleanup` step calls `agh daemon stop` and removes the bootstrap home tree.

````markdown
```yaml qa-scenario
id: cfg-01-cold-parse-canonical-defaults
title: Cold parse of canonical config.toml against built-in defaults (golden snapshot)
theme: config
coverage:
  primary:
    - config.cold-parse
  secondary:
    - config.defaults-drift
    - config.validate
live: false
provider: none
preconditions:
  - Fresh AGH_HOME (no prior config.toml).
  - Repo root config.toml copied verbatim to <AGH_HOME>/config.toml.
steps:
  - "agh daemon start --foreground=false (managed by tmux-bridge)"
  - "agh config show -o json > /tmp/show.json"
  - "agh config validate -o json"
expected:
  - "config show JSON parses without 'value: <nil>' anywhere"
  - "Every key documented in config.toml.example resolves to a non-zero typed value or the explicit zero (e.g. session.limits.timeout=0s)"
  - "Compare /tmp/show.json against fixtures/config-defaults.golden.json — diff must be empty (golden snapshot)"
  - "config validate exits 0 with status='valid'"
evidence:
  - "/tmp/show.json byte-equal to golden"
  - "agh.log shows config.load.completed for global path only"
failure_signatures:
  - "Field added/removed in DefaultWithHome but golden not updated"
  - "config.toml example drifted from defaults (covered by CFG-09)"
cleanup:
  - "agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-02-invalid-toml-stable-error
title: Daemon refuses boot on syntactically invalid TOML with file:line citation
theme: config
coverage:
  primary:
    - config.boot-refusal
  secondary:
    - config.error-shape
live: false
provider: none
preconditions:
  - Fresh AGH_HOME.
  - <AGH_HOME>/config.toml contains "[[mcp_servers\nname = \"oops\"" (unterminated array of tables).
steps:
  - "agh daemon start --foreground (capture stdout+stderr+exit code)"
  - "agh config validate -o json (separate run)"
expected:
  - "Daemon exits non-zero (not 0)"
  - "stderr contains 'load global config:' and the original parse error including file path AGH_HOME/config.toml and a line number"
  - "agh config validate exits non-zero with stable JSON shape: {error: {code: 'config.parse', file, line}}"
  - "Daemon does NOT auto-repair, does NOT silently fall back to defaults"
evidence:
  - "exit code != 0"
  - "Error JSON parses and includes file + line"
failure_signatures:
  - "Daemon boots with a partial config (silent fallback)"
  - "Error message lacks file:line"
cleanup:
  - "agh daemon stop || true"
```
````

````markdown
```yaml qa-scenario
id: cfg-03-missing-required-key-stable-exit
title: Missing required key (e.g. defaults.agent) — agent-parseable error + stable exit code
theme: config
coverage:
  primary:
    - config.required-key
  secondary:
    - config.error-shape
live: false
provider: none
preconditions:
  - <AGH_HOME>/config.toml present but with `[defaults]\nagent = ""`.
steps:
  - "agh config validate -o json"
expected:
  - "exit code 1 (or stable code matching CLI policy)"
  - "JSON output: {status:'invalid', errors:[{path:'defaults.agent', message:'defaults.agent is required'}]}"
  - "internal/config/config.go:797-801 produces 'defaults.agent is required'"
evidence:
  - "stderr stable across runs"
failure_signatures:
  - "Error references unrelated section"
  - "Validation succeeds (regression of validateCore wiring)"
cleanup:
  - "agh daemon stop || true"
```
````

````markdown
```yaml qa-scenario
id: cfg-04-hot-apply-disabled-skills
title: agh config set skills.disabled_skills hot-applies without restart
theme: config
coverage:
  primary:
    - settings.applied-now
  secondary:
    - skills.runtime-toggle
    - settings.sse-projection
live: true
provider: claude
preconditions:
  - Daemon running on isolated port.
  - Fixture skill named 'qa-marker-skill' installed under <AGH_HOME>/skills/.
  - Active session: agh sessions start --agent claude-code --workspace <ws>
steps:
  - "agh skills list -o json --session-id <sid> > /tmp/before.json"
  - "subscribe to /api/settings via SSE (background) -> /tmp/sse.log"
  - "agh config set skills.disabled_skills '[\"qa-marker-skill\"]' --scope global -o json -> capture {behavior, applied, restart_required}"
  - "agh skills list -o json --session-id <sid> > /tmp/after.json"
  - "Send to running Claude session: 'List available skills now and report whether qa-marker-skill appears.'"
expected:
  - "config set response.behavior == 'applied_now'"
  - "config set response.restart_required == false"
  - "before.json shows qa-marker-skill enabled"
  - "after.json shows qa-marker-skill missing or eligible=false"
  - "SSE stream emits a `settings.changed` payload with section='skills' and a list snapshot"
  - "Claude reply does NOT include qa-marker-skill in the listed skills (real LLM evidence)"
evidence:
  - "/tmp/before.json + /tmp/after.json diff"
  - "SSE payload archived in /tmp/sse.log"
  - "Claude transcript contains the absence assertion"
failure_signatures:
  - "behavior == 'restart_required' (regression of classify.go:90-95)"
  - "Active session still sees the skill (skill registry didn't reload)"
cleanup:
  - "agh sessions stop <sid>"
  - "agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-05-restart-required-flag
title: agh config set log.level surfaces restart_required and requires daemon restart
theme: config
coverage:
  primary:
    - settings.restart-required
  secondary:
    - daemon.restart
live: false
provider: none
preconditions:
  - Daemon running, log.level=info initially.
  - tail -f <AGH_HOME>/logs/agh.log (background)
steps:
  - "agh config set log.level debug --scope global -o json -> capture {behavior, restart_required}"
  - "Generate noisy activity (agh sessions list 5x) -> verify log file has NO debug-level lines"
  - "agh daemon stop && agh daemon start"
  - "Generate noisy activity again -> verify log file NOW contains debug-level lines"
expected:
  - "First config set: behavior=='restart_required', restart_required==true, applied==false"
  - "Mid-run log lines remain at level 'info' (no debug)"
  - "Post-restart log lines include 'level=debug'"
  - "config get log.level returns 'debug' (persisted regardless of runtime apply)"
evidence:
  - "Captured response JSON"
  - "Log file diff before/after restart"
failure_signatures:
  - "Mid-run log starts emitting debug (silent hot-apply where there shouldn't be one)"
  - "Restart fails to honor the persisted value"
cleanup:
  - "agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-06-overlay-precedence-five-layers
title: Five-layer skill+agent precedence — workspace overrides everything; agent-local overrides workspace
theme: config
coverage:
  primary:
    - workspace.overlay-precedence
  secondary:
    - skills.precedence
    - settings.shadow-audit
live: true
provider: claude
preconditions:
  - Workspace registered at /tmp/ws-cfg06 with name=cfg06.
  - Same skill 'overlap-skill' installed at <AGH_HOME>/skills/overlap-skill (bundled-equivalent shadow), at /tmp/ws-cfg06/.agh/skills/overlap-skill (workspace).
  - Same skill packaged inside <AGH_HOME>/agents/general/.agh/skills/overlap-skill (agent-local, highest precedence).
  - Each tier's SKILL.md prompt declares 'I am from <tier>' so the LLM can self-report.
steps:
  - "agh sessions start --agent general --workspace cfg06 --message 'Run overlap-skill and quote the first line of its prompt.'"
  - "Subscribe to /api/observability events filtered by event_kind=skills.shadow"
expected:
  - "Claude reply quotes 'I am from agent-local' (highest tier wins)"
  - "Observability emits one shadow-audit event per shadowed skill listing the suppressed sources"
  - "agh skills list -o json --session-id <sid> shows the agent-local source for overlap-skill"
evidence:
  - "Claude transcript"
  - "Shadow-audit event list"
failure_signatures:
  - "Claude quotes the workspace or global tier (precedence regression)"
  - "No shadow-audit event emitted (silent collapse)"
cleanup:
  - "agh sessions stop <sid>; agh workspace remove cfg06; agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-07-workspace-resolver-modes
title: Workspace resolution by --workspace, AGH_WORKSPACE env, cwd, config-discovery
theme: workspace
coverage:
  primary:
    - workspace.resolver-modes
live: false
provider: none
preconditions:
  - Two workspaces registered: ws-alpha at /tmp/alpha, ws-beta at /tmp/beta.
steps:
  - "from /tmp/alpha: agh workspace info -o json (no flags)  -> expect ws-alpha resolved by cwd"
  - "AGH_WORKSPACE=ws-beta agh workspace info -o json (still cwd=/tmp/alpha)  -> expect ws-beta (env wins over cwd)"
  - "agh workspace info ws-alpha -o json --workspace /tmp/beta  -> expect explicit positional overrides --workspace flag (or stable rule, captured here)"
  - "from /tmp/elsewhere: agh workspace info -o json  -> expect daemon-side error 'workspace not found' (cwd matches nothing)"
expected:
  - "Resolution order documented and stable: explicit positional > env > cwd-discovery > config-default"
  - "Each result records resolution_source in JSON output for agent-debuggability"
evidence:
  - "Captured JSON for each invocation"
failure_signatures:
  - "env override silently ignored"
  - "cwd-discovery picks a sibling workspace whose path begins with the cwd path"
cleanup:
  - "agh workspace remove ws-alpha ws-beta; agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-08-multi-workspace-isolation
title: Two concurrent workspaces — separate sessions, separate memory, no cross-talk
theme: workspace
coverage:
  primary:
    - workspace.isolation
  secondary:
    - memory.scope
    - skills.scope
live: true
provider: claude
preconditions:
  - workspace-a at /tmp/wsA with /tmp/wsA/.agh/skills/secret-a-skill (prompt: 'I am the alpha-only skill, marker=ALPHA-7').
  - workspace-b at /tmp/wsB with /tmp/wsB/.agh/skills/secret-b-skill (prompt: 'I am the beta-only skill, marker=BETA-7').
  - Daemon launched once with both registered.
steps:
  - "agh sessions start --agent claude-code --workspace wsA --message 'Use secret-a-skill and quote its marker.' (sid_a)"
  - "agh sessions start --agent claude-code --workspace wsB --message 'Use secret-b-skill and quote its marker.' (sid_b)"
  - "agh sessions start --agent claude-code --workspace wsA --message 'Try to use secret-b-skill.' (sid_a2)"
  - "agh memory list --workspace wsA -o json > /tmp/mem_a.json"
  - "agh memory list --workspace wsB -o json > /tmp/mem_b.json"
expected:
  - "sid_a transcript contains 'ALPHA-7'"
  - "sid_b transcript contains 'BETA-7'"
  - "sid_a2 transcript reports skill not found (or equivalent absence claim)"
  - "mem_a.json and mem_b.json have disjoint workspace_id sets — no entries belong to the other workspace"
  - "Per-workspace databases (events.db) live under separate workspace projections"
evidence:
  - "Two transcripts + two memory dumps"
failure_signatures:
  - "sid_a2 successfully invokes secret-b-skill"
  - "memory dump for ws A contains ws B markers"
cleanup:
  - "agh sessions stop sid_a sid_b sid_a2; agh workspace remove wsA wsB; agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-09-default-drift-guard
title: Built-in DefaultWithHome must not drift from repo-rooted config.toml example
theme: config
coverage:
  primary:
    - config.defaults-drift
live: false
provider: none
preconditions:
  - Repo at HEAD. Test runs `go test ./internal/config -run TestExampleConfigMatchesDefaults` (this scenario specifies the test that MUST exist).
steps:
  - "Construct cfg := DefaultWithHome(testHomePaths)"
  - "Parse repo-rooted config.toml via aghconfig.ApplyConfigOverlayFile(repo/config.toml, &empty)"
  - "Diff the two — every overlapping field must agree"
expected:
  - "Test passes with the current claude-sonnet-5 default and fails on any future example-vs-builtin disagreement"
evidence:
  - "Diff output captured"
failure_signatures:
  - "Test does not exist (codify drift guard)"
  - "Drift accepted (test passes when values disagree)"
cleanup:
  - "n/a"
```
````

````markdown
```yaml qa-scenario
id: cfg-10-secret-redaction-everywhere
title: Vault-backed secret never leaks across CLI / HTTP / SSE / log surfaces
theme: vault
coverage:
  primary:
    - vault.redaction
  secondary:
    - vault.audit
live: true
provider: claude
preconditions:
  - Daemon running.
  - Known fake secret string: 'AGHQA-FAKE-SECRET-9c4e1a'.
  - Provider 'claude' configured with credential_slot using secret_ref=vault:providers/claude/api_key, target_env=ANTHROPIC_API_KEY.
steps:
  - "printf 'AGHQA-FAKE-SECRET-9c4e1a' | agh vault put vault:providers/claude/api_key --kind api_key --value-stdin -o json"
  - "agh vault list --namespace providers -o json"
  - "agh vault get vault:providers/claude/api_key -o json"
  - "agh config show -o json"
  - "agh config list -o json"
  - "curl -s --unix-socket <socket> http:/agh/api/settings/general | jq ."
  - "curl -s --unix-socket <socket> http:/agh/api/vault/secrets/metadata?ref=vault:providers/claude/api_key | jq ."
  - "Subscribe to SSE: curl -s -N --unix-socket <socket> http:/agh/api/settings/observability/log-tail (10s window)"
  - "agh sessions start --agent claude-code --workspace cfg10 --message 'Read your environment and report any unusual long base64-looking strings.' (real Claude run)"
  - "grep -R 'AGHQA-FAKE-SECRET-9c4e1a' <AGH_HOME>"
expected:
  - "vault put response includes only Metadata fields (no plaintext)"
  - "vault list / get returns Present=true plus dates; never the value"
  - "config show/list never includes the value (env, secret_env redacted via RedactStringMap, see config/redaction.go:13-23)"
  - "HTTP responses never include the value"
  - "SSE log-tail does not stream the value"
  - "Claude reply does not echo the value (provider's own redaction OR our env-mask)"
  - "grep returns zero hits anywhere under <AGH_HOME>"
  - "Audit event 'vault.put' fires with ref + namespace + actor, but no value field"
evidence:
  - "All captured JSON archived"
  - "grep output"
failure_signatures:
  - "Any surface returns the literal AGHQA-FAKE-SECRET-9c4e1a"
  - "agh.log contains the value"
cleanup:
  - "agh vault delete vault:providers/claude/api_key; agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-11-vault-unification-flow
title: Real provider session fetches a secret via vault.ResolveRef, value reaches subprocess env, never logged
theme: vault
coverage:
  primary:
    - vault.resolve-ref
  secondary:
    - vault.audit
    - providers.credential-slot
live: true
provider: claude
preconditions:
  - vault:providers/claude/api_key seeded with a *real* throwaway ANTHROPIC_API_KEY (live-frontier lab key) per the bootstrap manifest.
  - Provider config: secret_ref=vault:providers/claude/api_key, target_env=ANTHROPIC_API_KEY, required=true.
steps:
  - "agh sessions start --agent claude-code --workspace cfg11 --message 'Echo \"alive\".'"
  - "Inspect ACP subprocess via daemon child-monitor (no direct ps): /api/sessions/<sid>/processes | jq ."
  - "Tail agh.log throughout"
expected:
  - "Claude responds 'alive' (real provider call succeeds, proving the env var was passed)"
  - "agh.log contains 'vault.resolve_ref ref=vault:providers/claude/api_key namespace=providers' but NEVER the resolved value"
  - "Audit event 'vault.read' fires with actor=daemon, scope=session/<sid>"
  - "Subprocess inspection (when supported) does not expose the value back through the API"
evidence:
  - "Successful Claude response"
  - "agh.log lines for vault.resolve_ref"
failure_signatures:
  - "agh.log contains the raw key value"
  - "Provider call fails with 401 (the env var did not propagate)"
  - "vault.read audit event missing"
cleanup:
  - "agh sessions stop <sid>; agh vault delete vault:providers/claude/api_key; agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-12-frontmatter-malformed
title: Frontmatter parser rejects malformed AGENT.md cleanly
theme: frontmatter
coverage:
  primary:
    - frontmatter.malformed
live: false
provider: none
preconditions:
  - Workspace cfg12 with three malformed agents:
    * /tmp/cfg12/.agh/agents/no-fence/AGENT.md       (no opening '---')
    * /tmp/cfg12/.agh/agents/unterminated/AGENT.md  (opening '---' but no closing one)
    * /tmp/cfg12/.agh/agents/bom/AGENT.md           (file begins with UTF-8 BOM 0xEF 0xBB 0xBF then '---')
    * /tmp/cfg12/.agh/agents/embedded-tab/AGENT.md  (key contains an embedded tab byte)
steps:
  - "agh workspace add /tmp/cfg12 --name cfg12 -o json (auto-discovers agents)"
  - "agh agent list --workspace cfg12 -o json"
expected:
  - "Workspace registers without error"
  - "agent list returns a per-agent diagnostic for each malformed file: {path, error_kind: 'frontmatter.missing'|'frontmatter.unterminated'|'frontmatter.bom'|'frontmatter.invalid_key', message}"
  - "Daemon does NOT exit; healthy agents (if any) load normally"
  - "internal/frontmatter/frontmatter.go:30-47 maps to ErrMissing / ErrUnterminated; BOM and embedded tab cases need an explicit code path (CFG-12 codifies the requirement)"
evidence:
  - "JSON list of diagnostics"
failure_signatures:
  - "Daemon crashes / refuses to register the workspace"
  - "BOM file silently parses (the BOM was misread as part of the metadata)"
cleanup:
  - "agh workspace remove cfg12; agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-13-overlay-editor-comment-preservation
title: agh config set preserves user comments and unrelated structure byte-for-byte
theme: config
coverage:
  primary:
    - config.persistence.comments
live: false
provider: none
preconditions:
  - <AGH_HOME>/config.toml hand-authored with header comments, inline comments after values, blank-line separators, and unrelated sections.
  - Snapshot original to /tmp/orig.toml.
steps:
  - "agh config set log.level debug --scope global -o json"
  - "diff -u /tmp/orig.toml <AGH_HOME>/config.toml > /tmp/diff.txt"
expected:
  - "diff is bounded to the single targeted line: only `level = \"info\"` -> `level = \"debug\"`"
  - "All comments, blank lines, and other sections preserved byte-for-byte"
  - "TOML still parses (LoadForHome succeeds afterward)"
evidence:
  - "/tmp/diff.txt"
failure_signatures:
  - "Diff includes whitespace-only changes elsewhere"
  - "Header comment lost"
  - "Reformatted to canonical TOML output"
cleanup:
  - "n/a (file is the artifact under test)"
```
````

````markdown
```yaml qa-scenario
id: cfg-14-concurrent-config-set
title: Two concurrent agh config set against the same key produce a defined outcome
theme: config
coverage:
  primary:
    - config.concurrent-write
live: false
provider: none
preconditions:
  - Daemon running, isolated home.
  - Standing directive: never parallelize config writes against one isolated QA home — but enforce that the daemon NEVER produces a corrupt file even if a misbehaving caller violates the rule.
steps:
  - "Run in parallel (& backgrounded):
       (agh config set log.level debug --scope global -o json) &
       (agh config set log.level error --scope global -o json) &
       wait"
  - "agh config get log.level -o json"
  - "agh config validate -o json"
expected:
  - "Either: (a) both succeed and the last writer wins (final value is one of {debug, error}, file is parseable), OR (b) one succeeds and the other fails with a stable 'config: locked' error"
  - "Validate succeeds — never a partial-write that breaks reload"
  - "If lock-based: the loser surfaces error_code='config.locked' with retry-after"
evidence:
  - "Captured JSON for both writes"
  - "Final config.toml byte snapshot"
failure_signatures:
  - "config.toml has interleaved bytes from both writers"
  - "Validate fails post-run"
cleanup:
  - "agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-15-agent-local-model-override
title: Per-agent AGENT.md `model:` value reaches the spawned ACP subprocess
theme: config
coverage:
  primary:
    - agents.local-model
  secondary:
    - workspace.agent-precedence
live: true
provider: claude,codex
preconditions:
  - Two AGENT.md files under workspace cfg15:
    * /tmp/cfg15/.agh/agents/op-x/AGENT.md  with provider: claude, model: claude-sonnet-5
    * /tmp/cfg15/.agh/agents/op-y/AGENT.md  with provider: codex,  model: gpt-5.6-luna
steps:
  - "agh sessions start --agent op-x --workspace cfg15 --message 'What model identifier are you running under? Reply with only that string.'"
  - "agh sessions start --agent op-y --workspace cfg15 --message 'What model identifier are you running under? Reply with only that string.'"
expected:
  - "op-x reply matches 'claude-sonnet-5' (or the official model self-id) — proves AGENT.md model field reached the runtime"
  - "op-y reply matches the gpt-5.6-luna family"
  - "Daemon log shows ResolvedAgent.Model = AGENT.md value (not the provider default)"
evidence:
  - "Two transcripts"
  - "Daemon log lines"
failure_signatures:
  - "Both replies cite the provider default model"
  - "ResolvedAgent constructed with provider default while AGENT.md is ignored"
cleanup:
  - "agh sessions stop ...; agh workspace remove cfg15; agh daemon stop"
```
````

````markdown
```yaml qa-scenario
id: cfg-16-dotenv-and-aghhome-precedence
title: Workspace .env feeds env-ref provider lookup; process env wins over .env
theme: config
coverage:
  primary:
    - config.dotenv-precedence
  secondary:
    - vault.env-ref
live: false
provider: none
preconditions:
  - Workspace cfg16 with /tmp/cfg16/.env containing 'AGH_HOME=/tmp/cfg16-home'.
  - Provider config: secret_ref='env:CFG16_TEST_TOKEN'.
  - Vault key file present in the new home.
steps:
  - "From inside /tmp/cfg16:
       agh config validate -o json (without process AGH_HOME)
     -> expect home resolves to /tmp/cfg16-home (dotenv wins when no process env)"
  - "AGH_HOME=/tmp/cfg16-other-home agh config validate -o json
     -> expect home resolves to /tmp/cfg16-other-home (process env beats .env)"
  - "Echo CFG16_TEST_TOKEN into /tmp/cfg16/.env, do NOT export it.
     agh sessions start --agent ... attempts ResolveRef('env:CFG16_TEST_TOKEN').
     Expected: ResolveRef returns the dotenv value when ResolveRef's lookupEnv is layered with dotenv (today only Load uses the layered lookup; vault.Service uses os.LookupEnv unless WithLookupEnv is wired)"
expected:
  - "Process env always wins over .env (config/config.go:283-295)"
  - "Without process env, .env is consulted via layeredEnvLookup (config/config.go:335-342)"
  - "If vault.Service is constructed without WithLookupEnv, env-refs do NOT see .env values — this scenario codifies the wiring requirement"
evidence:
  - "Captured JSON for both validate runs"
  - "Vault.ResolveRef behavior captured"
failure_signatures:
  - ".env value silently overrides exported process env (precedence inverted)"
  - "vault.ResolveRef finds env values that Load did not see, or vice versa"
cleanup:
  - "rm /tmp/cfg16/.env; agh daemon stop"
```
````

## 5. Edge Cases

The QA pass MUST also drive the following edges, each as a one-shot assertion (smaller than a full scenario but ledger-tracked):

- **Tab-vs-space indentation in TOML**: `pelletier/go-toml` accepts both, but a hand-mixed `[section]\n\tkey = "v"` must round-trip through `OverlayEditor.SetValue` without spurious whitespace mutation.
- **UTF-8 BOM at the head of `config.toml`**: today `loadConfigOverlayFile` reads raw bytes; verify it survives the BOM or fails with a clear "BOM not supported" message — not a generic parse error. Today's parser is `BurntSushi/toml.Decode`, which rejects BOM silently producing odd errors; verify and codify behavior.
- **Trailing-newline missing in `config.toml`**: file like `[daemon]\nsocket = "/tmp/x"` (no final `\n`) — both load and `OverlayEditor.SetValue` MUST work and the persisted result MUST end with exactly one trailing newline.
- **Comment-only file**: a file with only `# header\n` and no sections must load as `DefaultWithHome` (no error) and a subsequent `agh config set log.level debug` MUST append the new section cleanly.
- **Time-zone normalization**: `automation.timezone` defaults are exercised in `automation_test.go` but no scenario verifies that a workspace overlay with `automation.timezone = "America/Sao_Paulo"` survives daemon restart and emits cron events at the right wall-clock time.
- **Default-value drift**: the existing repo `config.toml` (used as marketing fixture) currently disagrees with `DefaultWithHome` on `claude.default_model`. CFG-09 codifies the test. Other potential drifts: `Limits.MaxSessions = 10` (matches), Network Live defaults/limits (must match the typed config contract), and `permissions.mode = "approve-all"` (matches).
- **Env-var override precedence with quoted values containing equals**: `KEY="a=b=c"` in `.env`. `godotenv.Unmarshal` handles this; CFG-12-style edge ensures the daemon does too on round-trip.
- **Secret with embedded newlines**: `agh vault put` reading from stdin uses `strings.TrimRight(content, "\r\n")` (`cli/vault.go:144-154`). A multi-line value (e.g. PEM-formatted secret) — the QA must clarify whether multi-line is supported or rejected.
- **Workspace path with spaces / unicode**: `/tmp/Pasta com espaços e ümlauts/`. Scanner walks correctly (`os.ReadDir` is byte-safe), but the JSON output for `agh workspace list` and `agh config path` MUST escape correctly.
- **Symlink workspace pointing outside an approved root**: `EvalSymlinks` resolves through the symlink. The `internal/CLAUDE.md` symlink-escape directive applies to skill paths and bundle install paths, not workspace roots — but any future hardening MUST avoid breaking this case. Codify: workspace at `/tmp/link → /private/var/folders/.../real` round-trips and stable on macOS.
- **macOS `/private/var/folders` canonicalization quirk** (`internal/CLAUDE.md:57`): scanner snapshots use both raw and canonical paths; verify cache-key stability across canonicalization.

## 6. Integration Surfaces (this module ↔ others)

- **`internal/daemon` (composition root)**: boot calls `aghconfig.LoadForHome(homePaths)`. Boot fails fast on validation. Bootstrap creates the home layout via `EnsureHomeLayout` (`config/home.go:117-136`). Restart-required mutations require the daemon to consume the persisted overlay on next start.
- **`internal/session`**: each session captures a `ResolvedWorkspace` + `ResolvedAgent` snapshot at start. Per-session config snapshot is what determines hot-apply boundaries.
- **`internal/skills`**: `SkillsConfig.DisabledSkills` consumed by skill registry (only field with hot-apply pathway). Workspace skills loaded via `WorkspaceDiscoveryRoots` (`config/agent.go:117-146`) and `scanWorkspace` (`workspace/scanner.go:48-83`). Five-tier precedence (Bundled → Marketplace → User → Additional → Workspace, agent-local override) per `internal/CLAUDE.md:125-127`.
- **`internal/api/core` (`BaseHandlers`)**: settings + vault HTTP/UDS handlers all use the shared `service` types. `MutationResult.RestartRequired` propagates as the `restart_required` JSON field (`internal/api/contract/settings.go:60,652`).
- **`internal/api/httpapi` + `udsapi`**: thin transport selection. Tests at `httpapi/helpers_test.go:130, 158, 175` and `udsapi/helpers_test.go:71, 99, 116` cover the same restart-required propagation.
- **`internal/sse`**: settings projection emitted over SSE for live UI feedback. Log-tail SSE endpoint at `internal/api/core/settings.go:290-340`.
- **`internal/agentidentity`**: workspace inference via `X-AGH-Workspace-ID` header (`identity.go:29`). Caller-identity validation before agent-scoped settings/vault ops.
- **`internal/automation`**: workspace-overlayed automation triggers; templates validate against `lookup` (workspace `.env` aware).
- **`internal/sandbox`**: sandbox-profile env/secret_env subject to `vault.ValidateNonSecretEnvMap` and `ValidateSecretEnvMap`.
- **`internal/extension`**: `ExtensionsConfig.Resources.AllowedKinds`, `MaxScope`, rate limits — config-driven and re-validated on every overlay mutation.
- **`internal/observe`**: emits `vault.put`, `vault.read`, `settings.changed`, `config.load`, `config.write`, `workspace.register`, `workspace.update` events.
- **`internal/store/globaldb`**: workspace registrations live in `agh.db`. Vault records also live here (encrypted blobs).

## 7. DX Cliffs (high-friction holes likely to bite real users)

1. **"Who wins when an agent has set a value vs an operator?"** — today, agent-scoped overrides via AGENT.md (model, tools, permissions, MCP) override config defaults but do NOT override workspace-scoped overlay values for the same field where they collide. `applyDefaultAgentOverride` in `workspace/resolver.go:279` overrides only `cfg.Defaults.Agent` from `Workspace.DefaultAgent`. Document the precise collision rules in CLI help and `agh config show` output.
2. **`agh config set` succeeding silently when the key is unknown** — today we error with `"cli: config path %q is not supported by config set"` (`cli/config.go:1039`). But there is no analogous protection for `agh config edit` (free-form file edit) — a typo there is caught only by the post-edit `LoadForHome` re-validation. Verify the error surface emits the exact failing key (CFG-03 covers required keys; this is for "unknown key" specifically).
3. **Vault key not redacted in error responses** — if `decryptValue` fails (`vault/crypto.go:121-150`), the error uses `fmt.Errorf("vault: decrypt value: %w", err)`. Confirm that `err` from `gcm.Open` does NOT include any plaintext / nonce / payload bytes in its String form. (Today it shouldn't; codify in CFG-10's failure_signatures.)
4. **`MutationResult.Warnings: ["no changes"]`** (`internal/settings/sections.go:204`) — what does an agent reading the SSE stream do with this? The shape is technically `applied_now=true`; a naive consumer assumes a mutation happened. Document and verify the warning is surfaced in `agh config set` JSON output.
5. **Workspace not registered yet but path is given** — `ResolveOrRegister` auto-registers (`workspace/resolver.go:165-235`); `Resolve` on a path string fails. Confirm that `agh config set --workspace /not/registered/path ...` produces a clear error rather than auto-registering the workspace as a side-effect of a config write.
6. **`agh config validate --workspace` without `--repair-env`**: workspace `.env` is parsed fresh on every load. If it has unsupported syntax, validate fails — but the message points at the `.env` file via `dotEnvUnsupportedError`, not at the TOML. Operators may misread this as a TOML problem.
7. **`AGH_VAULT_KEY` env vs file precedence**: env wins. If an operator sets the env in one shell but launches the daemon from another, vault decryption fails with a generic error. Surface a clear "key source mismatch" diagnostic in `agh config path` or `agh diagnostics`.
8. **Provider credential `secret_ref = "env:VAR"` when `VAR` is unset**: `vault.ResolveRef` returns `ErrMissingSecret` (`vault/service.go:128-130`). Verify the daemon surfaces this as an actionable error in `agh sessions start` (e.g., "provider 'claude' requires env:ANTHROPIC_API_KEY which is unset; set it via agh vault put or process env").

## 8. Failure Modes QA Must Catch

- **No raw secret in any output**: covered by CFG-10. Failure signature: any of the surfaces enumerated returns the literal known-fake string. Run an automated `grep` over `<AGH_HOME>` plus all captured HTTP / SSE / CLI outputs.
- **Two-workspace memory or skill leak**: covered by CFG-08. Failure signature: workspace A's session can recall workspace B's marker, or `agh memory list --workspace wsA` returns workspace B entries.
- **Missing `restart_required` flag on a restart-required key**: covered by CFG-05. Failure signature: `behavior=='applied_now'` for a key listed in `classify.go:87-88` field set.
- **TOML mis-merge across overlay layers**: covered by CFG-06 + CFG-13. Failure signature: workspace overlay silently fails to override; or comment preservation breaks; or unrelated section accidentally rewritten.
- **Config write that fails validation still gets persisted**: `EditConfigOverlay` validates *before* `writePersistedFile` (`persistence.go:330-336`). Failure signature: a write that violates `validateWithEnv` mutates the on-disk file. Test by attempting `agh config set http.port 99999` and asserting file unchanged.
- **`vault put` accepts a value but fails to redact in error case**: failure signature: an error path that includes the plaintext.
- **Workspace registration succeeds, agent files refuse to parse, daemon never reports the failures**: failure signature: `agh agent list` returns empty without diagnostics. CFG-12 codifies the requirement that agent-load diagnostics must be collected and surfaced.

## 9. Fixtures / Bootstrap Requirements

Per `agh-qa-bootstrap` and the worktree-isolation directive:

- **Two AGH_HOMEs** — `bootstrap-manifest.json` lab A (`AGH_HOME_A=/tmp/aghqa-cfg-A`) and lab B (`AGH_HOME_B=/tmp/aghqa-cfg-B`). Used by CFG-08.
- **Distinct daemon ports** — port allocation via `worktree.allocatePort()` (e.g. 21230 / 21231). NEVER `2123`.
- **Distinct UDS sockets** — `<AGH_HOME>/daemon.sock` is implicit per home; verify no shared lock.
- **Vault seeded with a known fake secret** — `AGHQA-FAKE-SECRET-9c4e1a` (CFG-10) and a real throwaway provider key (CFG-11) under `vault:providers/claude/api_key` of lab A only.
- **Baseline `config.toml` fixture** — `_fixtures/config-default-snapshot.golden.json` that CFG-01 validates against. Regenerate via `go test ./internal/config -run TestExampleConfigMatchesDefaults -update`.
- **Fixture skills**:
  - `qa-marker-skill` (CFG-04) — single-file SKILL.md with prompt `"I am qa-marker-skill, marker=QA-MARKER-1."`.
  - `overlap-skill` (CFG-06) — three copies, each with a tier-self-identifying prompt.
  - `secret-a-skill`, `secret-b-skill` (CFG-08) — workspace-only.
- **Fixture agents**:
  - `op-x` (claude / claude-sonnet-5) and `op-y` (codex / gpt-5.6-luna) — CFG-15.
  - Malformed AGENT.md set (no fence / unterminated / BOM / embedded tab) — CFG-12.
- **Real Claude Code subagent** — for CFG-04, CFG-06, CFG-08, CFG-10,
  CFG-11, CFG-15. ACP runtime:
  `npx -y @agentclientprotocol/claude-agent-acp@latest` (per
  `provider.go:166`). Direct `claude` auth comes from the effective Claude
  home for the lane: operator `HOME` by default, or isolated `PROVIDER_HOME`
  only for explicit isolated-home scenarios.
- **Real Codex subagent** — for CFG-15. ACP runtime: `npx -y @agentclientprotocol/codex-acp@latest` (per the current builtin provider config). Lab-key sourced from `PROVIDER_CODEX_HOME`.
- **A workspace `.env`** — `/tmp/cfg16/.env` for CFG-16.
- **Per-lane artifact directory** — `.artifacts/qa/cfg-<run-id>/` containing one `<scenario>-{report,summary,observed-events,output}.{md,json,json,log}` quartet per the openclaw four-artifact contract.

## 10. Citations

Backend implementation citations (file:line) used in this plan:

- Config loader & defaults: `internal/config/config.go:1-200`, `200-600`, `600-1180`, especially `Load` 322-350, `LoadForHome` 354-377, `loadWithHome` 379-406, `DefaultWithHome` 418-497, `validateWithEnv` 504-521, `validateCore` 523-548, `validateFeatures` 550-585.
- Section types & defaults: `DaemonConfig` 33-36, `HTTPConfig` 38-42, `DefaultsConfig` 44-49, `AgentsConfig`/`SoulConfig`/`HeartbeatConfig` 52-78 (validation 832-919), `LimitsConfig` 80-84, `SessionConfig` 86-104, `PermissionsConfig` 119-122, `ObservabilityConfig` 124-138, `LogConfig` 140-143, `MemoryConfig`/`DreamConfig` 146-159, `SkillsConfig` 173-181, `ExtensionsConfig` 183-202, `NetworkConfig` 204-213 (validation 1128-1185), `Config` 247-269, `SandboxProfile` 215-225 (validation 654-702).
- Defaults (`DefaultWithHome`): `config.go:418-497`. `DefaultSoulConfig` 805-811, `DefaultHeartbeatConfig` 813-829, `DefaultSessionSupervisionConfig` 950-958.
- Home/path layout: `internal/config/home.go:1-212`. `ResolveHomeDir` 58-75, `ResolveHomePathsFrom` 92-114, `EnsureHomeLayout` 117-136, `ResolvePath` 150-167, `ResolveUserAgentsSkillsDir` 170-192.
- Persistence: `internal/config/persistence.go:1-622`. `WriteScope` 24-42, `WriteTargetKind`/`WriteTarget` 44-90, `ResolveConfigWriteTarget` 91-152, `OverlayEditor` 154-295, `EditConfigOverlay` 296-338, `validateEffectiveConfigWrite` 340-394, helpers 396-622.
- Merge overlay structs: `internal/config/merge.go:1-200`. Section overlays 16-200, especially `extensionsOverlay` 197-200 and pointer-typed leaf fields throughout.
- Provider config: `internal/config/provider.go:1-200`. `ProviderHarness` 16-23, `ProviderCredentialSlot` 25-32, `ProviderConfig` 34-46, `MCPServer` 84-93, `ResolvedAgent` 96-113, `builtinProviderAliases` 124-162, `builtinProviders` 164-200+, secret_ref validation 760-764, 853, 876.
- Agent definition: `internal/config/agent.go:1-367`. `AgentDef` 17-31, `WorkspaceDiscoveryRoots` 117-146, `LoadAgentDef`/`LoadAgentDefFile` 71-115, `LoadWorkspaceAgentDefs` 167-209, `ParseAgentDef` 211-250, `Validate` 252-295, `decodeAgentFrontmatter` 314-336.
- AgentDef clone: `internal/config/agent_clone.go:1-22`.
- Redaction: `internal/config/redaction.go:1-32`. `redactedConfigValue` 3, `RedactStringMap` 13-23, `RedactedMCPServer` 27-32.
- DotEnv: `internal/config/dotenv.go:1-200`, `200-461`, `462-540`. `WorkspaceDotEnvFile` 75-78, `InspectDotEnvFile` 80-89, `RepairDotEnvFile` 92-160, line parser 178-263, `parseDotEnvAssignment` 265-318, `secretLikeDotEnvKey` 435-441, `replaceDotEnvFile` 480-540.
- Frontmatter: `internal/frontmatter/frontmatter.go:1-112`. `Split` 28-58, `Decode` 61-75, `normalizeLineEndings` 77-83, `findClosingDelimiter` 97-111. Tests `frontmatter_test.go:1-125`.
- Settings service: `internal/settings/service.go:1-219`. Dependencies struct 77-94, NewService 119-152, `loadConfig` 192-214.
- Settings models: `internal/settings/models.go:1-220`. `ScopeKind` aliases 16-24, `WriteTargetKind` aliases 26-38, sections 40-58, collections 60-72, `MutationBehavior` 86-96, `Service` interface 114-121, `MutationResult` 196-220.
- Settings classify: `internal/settings/classify.go:1-129`. `ClassifyMutation` 9-49, `classifyAction` 51-78, `classifyField` 80-121, `restartRequiredClassification` 123-129.
- Settings sections file (writes + applied_now): `internal/settings/sections.go:180-271`.
- Vault types: `internal/vault/types.go:1-287`. Errors 13-20, `EnvNamePattern` 22-23, `vaultRefPattern`/`vaultRefPrefixPattern` 25-33, namespaces 35-44, `secretLikeEnvNeedles` 46-56, `Record` 58-65, `Metadata` 67-74, `Store` 76-82, `IsSecretRef`/`IsEnvRef` 89-97, `ValidateSecretRef` 113-120, `SecretLikeEnvName` 244-252, `ValidateNonSecretEnvMap` 254-269, `ValidateSecretEnvMap` 271-286.
- Vault service: `internal/vault/service.go:1-205`. `Service` 13-18, `NewService` 38-67, `PutSecret` 70-113, `ResolveRef` 116-155, `GetMetadata` 158-168, `ListMetadata` 171-185, `DeleteSecret` 188-194, `metadataForRecord` 196-204.
- Vault crypto: `internal/vault/crypto.go:1-151`. `KeyProvider` 22-25, `fileKeyProvider` 27-77 (env override 44, key file 53-76), `decodeKey` 79-93, `encryptValue` 95-119, `decryptValue` 121-150.
- Workspace types: `internal/workspace/workspace.go:1-64`. Errors 14-29, `Workspace` 31-41, `ResolvedWorkspace` 43-51, `RuntimeResolver` 60-63.
- Workspace resolver: `internal/workspace/resolver.go:1-345`. `Resolver` 36-47, `cachedEntry` 52-58, `NewResolver` 62-83, `Resolve` 86-162, `ResolveOrRegister` 165-235, `Invalidate` 245-254, `buildResolvedWorkspace` 266-300, `canReuse` 310-331, eviction 333-344.
- Workspace CRUD: `internal/workspace/resolver_crud.go:1-355`. `Register` 13-52, `Unregister` 55-74, `Update` 77-125, `lookupWorkspace` 234-281, `lookupWorkspaceBySameRoot` 283-310, `refreshRootDir` 312-354.
- Workspace scanner: `internal/workspace/scanner.go:1-245`. `scanWorkspace` 37-83, `scanAgentSource` 86-127, `scanSkillSource` 129-175, `loadAgents` 177-204, `mergeSkillPaths` 206-227.
- CLI config commands: `internal/cli/config.go:1-200`. `configScalarMutationKinds` 91-163, `newConfigCommand` 166-180, `newConfigShowCommand` 182-206, `newConfigListCommand` 208-230, `newConfigGetCommand` 232-254, `newConfigSetCommand` 256-312, `newConfigPathCommand` 314-383, `newConfigValidateCommand` 385-445, `newConfigEditCommand` 447-491. Mutation classification 998-1090. Flatten + redaction 731-767.
- CLI vault commands: `internal/cli/vault.go:1-267`. `newVaultCommand` 17-27, list 29-61, get 63-81, put 83-122, delete 125-142, `readVaultSecretStdin` 144-154.
- CLI workspace commands: `internal/cli/workspace.go:1-473`. `newWorkspaceCommand` 12-24, add 26-71, list 73-95, info 97-120, edit 122-209, remove 211-235.
- AgentIdentity: `internal/agentidentity/identity.go:1-100`. `EnvSessionID`/`EnvAgent` 20-22, `HeaderWorkspaceID` 29, exit codes 35-45, errors 47-58, `Credentials` 60-65, `SessionSnapshot` 67-86.
- API contracts: `internal/api/contract/settings.go:55-65, 644-660`. Restart-required wire-format.
- Repo config example: `/Users/pedronauck/Dev/compozy/agh/config.toml:1-71` (drift-target for CFG-09).
- Repo CLAUDE.md (workflow rules): `/Users/pedronauck/Dev/compozy/agh/CLAUDE.md:43-47` (worktree isolation, provider-home isolation, AGH_WEB_API_PROXY_TARGET, never-parallelize-config-writes).
- Internal CLAUDE.md (security invariants and architecture): `/Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md:54-62, 95, 100-101, 121, 125-127`.

Reference QA patterns:

- openclaw config theme: `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md:279, 853-858`.
- openclaw scenario anatomy: `openclaw-qa-patterns.md:90-262`.
- hermes hermetic shield + secret-shaped env redaction: `hermes-qa-patterns.md:74-99` (autouse fixtures, secret needles).

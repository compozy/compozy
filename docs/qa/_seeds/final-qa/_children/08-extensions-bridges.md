---
name: 08-extensions-bridges
description: AGH pre-release QA — extensions + bridges + bundles + bridge SDK module. Real-LLM scenarios required. Read-only research deliverable.
type: qa-child
module: extensions-bridges
owner: pre-release-qa
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
---

# 08 — Extensions, Bridges, Bundles, and Bridge SDK QA

## 1. Module scope

This child stresses every load-bearing surface in the AGH extension stack. An
extension is the canonical AGH unit of "code that the daemon supervises but
does not own": tool providers, hook subprocesses, memory backends, and
bridge adapters all flow through one manifest, one registry, and one
subprocess supervisor. Bundles are the activation projector that turns one
extension's declared profile (skills + tools + hooks + bridges + agents +
jobs) into a coherent runtime overlay; deactivation must reverse the overlay
atomically. The bridge SDK (Go and TS) is the contract every adapter binds
to.

Packages and SDKs in scope (file:line citations are repo-absolute):

| Surface              | Path                                                                           | Authoritative API                                                                                                                                                                                                                                                                                                  |
| -------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Manifest + registry  | `/Users/pedronauck/Dev/compozy/agh/internal/extension/`                        | `LoadManifest` (`internal/extension/manifest.go:239`), `Manifest.Validate` (`:269`), `Registry.Install` (`internal/extension/registry.go:162`), `Uninstall` (`:171`), `Enable`/`Disable` (`:194`/`:199`), `installWithConfig` (lower in `registry.go`)                                                              |
| Manager + supervisor | `/Users/pedronauck/Dev/compozy/agh/internal/extension/manager.go`              | `Manager.recoverExtension` (`:1093`), `launchRuntime` (`:1152`), `ExtensionPhaseRecover` (`:125`)                                                                                                                                                                                                                  |
| Host API             | `/Users/pedronauck/Dev/compozy/agh/internal/extension/host_api.go`             | `HostAPIRateLimitedCode = -32002` (`:36-37`), `HostAPIInvalidParamsCode` (`:42-43`), default rate-limit / dedup TTL constants (`:47-52`)                                                                                                                                                                           |
| Managed install      | `/Users/pedronauck/Dev/compozy/agh/internal/extension/install_managed.go`      | Symlink hardening for runtime dependency copy (`:355-572`); cycle detection (`:536`); escape rejection (`:562`)                                                                                                                                                                                                    |
| Bundles spec         | `/Users/pedronauck/Dev/compozy/agh/internal/extension/bundle.go`               | `BundleSpec` (`:27`), `BundleProfile` (`:35`), `BundleAgent` (`:46`), `BundleChannel` (`:66`), `BundleJob` (`:71`), `BundleTrigger` (`:84`), `BundleBridgePreset` (`:96-104`); bundle root resolve (`:741-750`)                                                                                                     |
| Bundle service       | `/Users/pedronauck/Dev/compozy/agh/internal/bundles/service.go`                | `Activate` (`:220`), `Deactivate` (`:401`), `PreviewActivation` (`:203`), `reconcileLocked` rollback (`:268-287`, `:388-396`, `:416-425`), Network requirement confirmation, `joinRollbackFailure`                                                                                                              |
| Bridge registry      | `/Users/pedronauck/Dev/compozy/agh/internal/bridges/registry.go`               | `Registry.CreateInstance`, `UpdateInstance`, `UpdateInstanceState`, `BuildRoutingKey`, `ResolveOrCreateRoute`                                                                                                                                                                                                      |
| Bridge lifecycle     | `/Users/pedronauck/Dev/compozy/agh/internal/bridges/lifecycle.go`              | `ValidateInstanceStateTransition` (`:9`), `validateInstanceLifecycle` (`:43`), `transitionFromStarting/Ready/Degraded/AuthRequired/Error` (`:101+`)                                                                                                                                                                |
| Bridge delivery      | `/Users/pedronauck/Dev/compozy/agh/internal/bridges/delivery_broker.go`        | `Broker.Deliver` (`:301`), `enqueueEventLocked` (`:334`/`:424`), bounded per-route queue (`:33,95,118-137`); defaults `defaultDeliveryQueueCapacity=4`, `defaultDeliveryRetryDelay=25ms`, `defaultDeliveryRequestTimeout=5s` (`internal/bridges/delivery_types.go:38-40`)                                          |
| Bridge SDK (Go)      | `/Users/pedronauck/Dev/compozy/agh/internal/bridgesdk/`                        | `Runtime.Serve` (`internal/bridgesdk/runtime.go:82-100`), `RuntimeConfig` (`:33`), `Session` (`:54`), batching (`batching.go`), dedup (`dedup.go`), webhook helper (`webhook.go`), peer (`peer.go`), HostAPI client (`hostapi.go`)                                                                                  |
| Bridge SDK (TS)      | `/Users/pedronauck/Dev/compozy/agh/sdk/typescript/src/`                        | `Extension` class (`sdk/typescript/src/extension.ts:109`), `REQUIRED_PROVIDES_METHODS` (`extension.ts:45-49`), tool registration `RegisteredTool` (`:103-107`), `HostAPI` (`host-api.ts`), `StdioTransport` (`transport.ts`)                                                                                       |
| create-extension     | `/Users/pedronauck/Dev/compozy/agh/sdk/create-extension/`                      | `parseArgs` (`src/index.ts:30`), templates (`templates/{hook-subprocess,memory-backend,tool-provider,go-tool-provider}`), `DEFAULT_SDK_SPEC = "^0.1.0"`                                                                                                                                                            |
| Bridge providers     | `/Users/pedronauck/Dev/compozy/agh/extensions/bridges/{slack,telegram,discord,gchat,github,linear,teams,whatsapp}` | Each `extension.toml` declares `bridge.platform`, `secret_slots` (e.g. `slack/extension.toml:14-22` for `bot_token`/`signing_secret`), and subprocess command (`./bin/<name>`) wired through bridge SDK runtime.                                                              |
| Test harness         | `/Users/pedronauck/Dev/compozy/agh/internal/extensiontest/`                    | `bridge_adapter_harness.go`, `bridge_conformance_matrix.go`                                                                                                                                                                                                                                                        |
| CLI                  | `/Users/pedronauck/Dev/compozy/agh/internal/cli/{extension,bundle,bridge}.go`  | `agh extension {search,list,install,remove,update,enable,disable,status}` (`extension.go:34-233`); `agh bundle {catalog,preview,activate,list,get,update}` (`bundle.go:20-148`); `agh bridge {list,get,create,update,enable,disable,...}` (`bridge.go:17-300`); restart guidance constants `extensionInstallRestartMessage`, `extensionUpdateRestartMessage` (`internal/cli/extension_marketplace.go:22-25`) |

Out of scope (covered by other children): full ACP transport correctness
(module 03), autonomy kernel internals (module 04), AGH Network channel
correctness (module 06), web UI (module 07).

## 2. Authoritative invariants under test

Every scenario below maps back to one or more of these IDs. Coverage IDs
follow the openclaw lowercase dotted/dashed convention.

| Coverage ID                                 | Invariant                                                                                                                                                | Source                                                                                                                                                                                                                                                                                |
| ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `extension.manifest.validate`               | `LoadManifest` rejects missing/invalid name, version, or `min_agh_version`; manifests requesting capabilities they cannot provide are rejected typed.    | `internal/extension/manifest.go:239,269,332`                                                                                                                                                                                                                                          |
| `extension.manifest.compat`                 | Manifests requiring a higher daemon than the SUT version fail with `ErrManifestIncompatible`.                                                            | `internal/extension/manifest.go:39,213-214,387-395`                                                                                                                                                                                                                                   |
| `extension.manifest.namespaced-metadata`    | Bridge `config_schema` and other namespaced extension metadata (e.g. `agh.bridge.slack`) round-trip through manifest+registry without loss.              | `internal/extension/manifest.go:99-105,332-355`; `internal/extension/manifest_test.go:295,409,426`                                                                                                                                                                                    |
| `extension.install.symlink-escape`          | Install runtime tree-copy rejects symlinks whose canonical target escapes the source root or forms a cycle.                                              | `internal/extension/install_managed.go:355-572`; `internal/extension/install_managed_test.go:298-499`                                                                                                                                                                                 |
| `extension.install.checksum`                | Install verifies the on-disk artifact checksum and refuses with `ErrExtensionChecksumMismatch` on mismatch.                                              | `internal/extension/registry.go:31,160-169`                                                                                                                                                                                                                                           |
| `extension.install.restart-guidance`        | After `agh extension install` (or update), CLI emits restart guidance to stderr; new tools/skills become available after a daemon restart.               | `internal/cli/extension_marketplace.go:22-25`; `internal/cli/extension_marketplace_test.go:415,764,845`                                                                                                                                                                               |
| `extension.lifecycle.recover`               | A subprocess panic / unexpected exit triggers `recoverExtension` with backoff; the daemon never crashes; the extension is marked unhealthy then recovered or disabled. | `internal/extension/manager.go:1035-1149`                                                                                                                                                                                                                                             |
| `extension.host-api.rate-limit`             | Per-extension Host API call rate-limit enforced at `HostAPIRateLimitedCode = -32002`; bursts allowed up to configured `defaultHostAPIBurst = 20`.        | `internal/extension/host_api.go:36-50`                                                                                                                                                                                                                                                |
| `extension.skill.verify-content`            | Every non-bundled skill (including those packaged inside extensions/bundles) is scanned by `internal/skills.VerifyContent` on every load; critical findings block. | `internal/CLAUDE.md` "Load-time security scan"; `internal/skills/registry.go:504`; `internal/skills/verify.go:101-102`                                                                                                                                                                |
| `extension.lifecycle-hook.session-postcreate` | Extension-declared `session.post_create` hooks fire in hierarchy precedence + alphabetical order; legacy `on_session_created` event names are rejected with the documented translation message. | `internal/skills/loader.go:517-526`; `internal/skills/loader_test.go:650-660`                                                                                                                                                                                                          |
| `extension.workspace-scope`                 | Workspace-scoped extension/bundle activation only exposes its tools/skills inside that workspace.                                                        | `internal/bundles/service.go:163-290`; `internal/bundles/model/model.go` (Scope kinds)                                                                                                                                                                                                |
| `bundle.activate.atomic`                    | Bundle `Activate` writes activation row + reconciles overlays atomically; if reconcile fails, the activation is fully rolled back via `joinRollbackFailure`. | `internal/bundles/service.go:220-290`                                                                                                                                                                                                                                                 |
| `bundle.deactivate.clean`                   | Bundle `Deactivate` reverses every overlay (skills/tools/hooks/agents/triggers/bridges); on reconcile failure the activation is restored.                | `internal/bundles/service.go:401-427`                                                                                                                                                                                                                                                 |
| `bundle.activate.no-leak`                   | Two bundles' activations never share state — uninstalling one cannot reach into the other's overlays.                                                    | `internal/bundles/service.go:200-330`                                                                                                                                                                                                                                                 |
| `bundle.network-requirement-confirmation`   | An extension-declared Live requirement blocks activation until the operator confirms the current requirement digest; a changed digest clears confirmation. | `internal/bundles/network_requirement.go`                                                                                                                                                                                                                                             |
| `bridge.lifecycle.transitions`              | Only the documented status transitions in `lifecycle.go` are accepted; e.g. `disabled → starting` only with `enabled=true`; nothing reaches `ready` without going through `starting`. | `internal/bridges/lifecycle.go:9-160`                                                                                                                                                                                                                                                 |
| `bridge.delivery.bounded-queue`             | Per-route delivery queue is bounded (`defaultDeliveryQueueCapacity = 4`); overflow is reported via the broker's `DeliveryBacklog` metrics, not unbounded growth. | `internal/bridges/delivery_broker.go:33,95,221`; `internal/bridges/delivery_types.go:38`                                                                                                                                                                                              |
| `bridge.delivery.ordering`                  | A delivery's start → delta(s) → terminal sequence is preserved per route under load (no out-of-order or token-delta partials leak past the terminal).    | `internal/bridges/delivery_broker.go:24-78`; matches openclaw `streaming-final-integrity` shape                                                                                                                                                                                       |
| `bridge.auth.surface`                       | Provider authentication failure transitions the instance to `auth_required` and surfaces a clean user-visible reason; no flooded retry loop.             | `internal/bridges/lifecycle.go:86-96`; `internal/bridges/types.go:123-124`                                                                                                                                                                                                            |
| `bridge.secret-redaction`                   | Bridge secret slot values (`bot_token`, `signing_secret`, `driverToken`) NEVER appear in logs/SSE/responses. Standing token-redaction discipline applies. | `internal/CLAUDE.md` Security Invariants; `extensions/bridges/slack/provider.go:837,915`                                                                                                                                                                                              |
| `bridge.signature.verify`                   | Slack bridge rejects webhooks with invalid signing-secret HMAC and missing/expired timestamps; Discord verifies ed25519; Telegram verifies bot-token-derived secret-token header. | `extensions/bridges/slack/provider.go:1094`; `internal/extension/discord_provider_integration_test.go:34-170`                                                                                                                                                                          |
| `bridge.sdk.contract`                       | Any extension authored against `@agh/extension-sdk` claiming `tool.provider` MUST register `provide_tools` + `tools/call`. Capability declared but methods missing → initialize fails. | `sdk/typescript/src/extension.ts:42-49`; `sdk/typescript/src/integration.test.ts`                                                                                                                                                                                                     |
| `bridge.sdk.scaffold`                       | `npx @agh/create-extension <name> -t <template>` produces a buildable extension whose smoke run boots through `Extension.start()`.                       | `sdk/create-extension/src/index.ts:30-80`; templates `tool-provider/`, `go-tool-provider/`, `hook-subprocess/`, `memory-backend/`                                                                                                                                                     |
| `cli.extension.machine-readable`            | `agh extension list -o json` and `agh extension status <name> -o json` parity between local-registry path and HTTP/UDS path; payloads identical for the same instance. | `internal/cli/extension.go:73-85,220-233,235-260`                                                                                                                                                                                                                                     |

## 3. Operating model

QA mode is **real-scenario** (per the standing directive on real-scenario
QA). Every scenario:

- Runs against an isolated `AGH_HOME` with unique daemon ports + tmux-bridge
  socket (per `agh-worktree-isolation` skill).
- Resolves provider auth from the bootstrap manifest according to each
  provider contract: bound-secret, brokered, and explicitly isolated-home
  lanes use `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, while `native_cli`
  lanes with `home_policy=operator` preserve the operator `HOME` unless the
  scenario explicitly validates isolated provider-home behavior.
- Uses real Claude Code (`claude-opus-4-8` for parent coordinator;
  `claude-sonnet-5` for spawned children where indicated) as the
  subprocess agent. Bridges call out to real Slack / Telegram only on the
  `live: conditional` lanes guarded by a credential broker (openclaw
  pattern, see §4).
- Emits four artifacts under `.artifacts/qa/<run-id>/ext-XX/`:
  - `ext-XX-report.md` (Worked / Failed / Blocked / Follow-up)
  - `ext-XX-summary.json` (machine-readable)
  - `ext-XX-events.json` (EventStore + bridge delivery rows scoped to the
    scenario window)
  - `ext-XX-output.log` (combined stdout/stderr)
- Asserts against EventStore rows + `extensions` table state +
  `bundle_activations` rows + bridge `delivery_broker` metrics + structured
  log output, never just process exit codes.

Scenarios are numbered `EXT-01..EXT-NN`; each is a fenced `qa-scenario`
block plus a flow narrative.

## 4. Provider matrix

| Mode                         | When                                                                                              | Driver                                                                                                                                                  |
| ---------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `real-claude-code`           | Default for all scenarios that exercise real subagent behavior, tool round-trip, transcript flow. | `claude-opus-4-8` for parent coordinator; `claude-sonnet-5` for spawned children where indicated.                                                  |
| `mock-acp`                   | Determinism gate for race-sensitive scenarios (only EXT-08 and EXT-15 backpressure ordering).     | `internal/e2elane` mock ACP server; the surrounding daemon, extension manager, bridge SDK, and SQLite are all real code paths.                          |
| `live: conditional` Slack    | EXT-06 (real Slack ↔ Claude Code round-trip).                                                     | Real Slack workspace pair leased through a credential broker (openclaw `convex-credential-broker` shape, but local SQLite-backed; see §11).             |
| `live: conditional` Telegram | EXT-07 (real Telegram bot bidi flow).                                                             | Real Telegram bot pair (driver bot + SUT bot) leased through the same credential broker; pool kind = `telegram`.                                        |

`live: conditional` means: **default skip when no broker lease is available;
fail-fast (not silently pass) when a lease is partially configured**. CI
release dry-run runs the live lanes; per-developer local QA may skip them
explicitly with `EXT_SKIP_LIVE=1` in scenario env.

## 5. Preconditions (apply to every scenario)

- Fresh QA bootstrap via the `agh-qa-bootstrap` skill. `bootstrap-manifest.json`
  exported into shell as `bootstrap.env` before any `agh` command.
- Unique `AGH_HOME` per worktree.
- Bound-secret, brokered, and explicitly isolated-home auth staged into
  `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`; `native_cli` providers with
  `home_policy=operator` intentionally use the operator `HOME` / native login
  state unless the scenario explicitly validates isolated provider-home
  behavior.
- Daemon started in background. HTTP / UDS listeners reachable.
- `make verify` is green on the SUT branch before QA runs.
- For bridge live lanes only: credential broker reachable; `EXT_BRIDGE_BROKER_URL`,
  `EXT_BRIDGE_BROKER_SECRET_MAINTAINER` (or `_CI`) set.

Provider-specific config:

```text
AGH_HOME=$HOME/.qa/ext-08/<scenario>/agh-home
AGH_DAEMON_HTTP=127.0.0.1:<unique-port>
AGH_DAEMON_UDS=$AGH_HOME/sock/uds.sock
PROVIDER_HOME=$AGH_HOME/provider-home
PROVIDER_CODEX_HOME=$AGH_HOME/provider-codex-home
AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:<unique-port>
EXT_BRIDGE_BROKER_URL=https://broker.qa.example/    # only for live lanes
EXT_BRIDGE_BROKER_SECRET_MAINTAINER=<...>           # only for live lanes
```

## 6. Cleanup (applies to every scenario)

- `agh daemon stop` (or kill PID from manifest).
- For live lanes: release the broker lease (`/release` endpoint) before
  archiving evidence.
- Inspect `extensions`, `bundle_activations`, `bridge_instances`, and
  `bridge_routes` for stuck rows; if found, attach to scenario report and
  do NOT clean — it is evidence.
- Archive `agh.db`, `events.db` snapshots before tearing down the
  AGH_HOME.

## 7. Mandatory scenarios

### EXT-01 — Local extension install at runtime, manifest validation, host API exposure

```yaml qa-scenario
id: ext-01-runtime-install
title: agh extension install <local-path> validates manifest, persists registry row, exposes host API after restart, returns deterministic JSON
theme: extensions.install
coverage:
  primary:
    - extension.manifest.validate
    - extension.install.checksum
    - extension.install.restart-guidance
    - cli.extension.machine-readable
  secondary:
    - extension.lifecycle.recover
risk: high
live: false
provider: real-claude-code
preconditions:
  - One local fixture extension on disk with a valid extension.toml
    (`name`, `version`, `min_agh_version=0.5.0`, `capabilities.provides=["tool.provider"]`,
    `subprocess.command=./bin/ext-fixture`, single tool declared under
    `[resources.tools.echo]`).
  - Daemon running.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest.go:239
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest.go:269
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/registry.go:160
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/extension.go:87
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/extension_marketplace.go:22
steps:
  - Run `agh extension install ./fixtures/ext-fixture -o json`. Capture
    stdout + stderr.
  - Run `agh extension list -o json`. Capture.
  - Run `agh extension status ext-fixture -o json`. Capture.
  - Restart daemon. Wait for daemon ready.
  - Prompt a real Claude Code session to call the fixture's tool ("Run
    the `echo` tool with input `hello-ext-01`"). Capture transcript +
    EventStore.
expected:
  - `agh extension install` returns success JSON with `name=ext-fixture`,
    `version=…`, non-empty `checksum`, `enabled=true`.
  - Stderr contains the `extensionInstallRestartMessage` line.
  - `extensions` table row exists with `name=ext-fixture`,
    `source=user`, `manifest_path` set.
  - `agh extension list -o json` and `agh extension status` payloads
    match the row content (parity check).
  - After restart, the agent's tool call returns `hello-ext-01` echoed
    back through the extension subprocess; transcript shows the tool id
    in canonical form.
evidence:
  - `ext-01-install.json`, `ext-01-list.json`, `ext-01-status.json`
    captured payloads.
  - `extensions` table dump.
  - Transcript fragment showing tool roundtrip.
failure_signatures:
  - `extension install` succeeds without manifest validation: schema
    bypass.
  - Restart guidance missing on stderr: install UX regression.
  - `extension list` and `extension status` payloads diverge for the
    same record: machine-readable parity violated.
  - Tool call fails after restart: host API not exposed end-to-end.
cleanup:
  - `agh extension remove ext-fixture`. Confirm the row is gone from
    SQLite and the install dir is removed.
```

### EXT-02 — Bundle activation: skills + tools + hooks atomic; rollback on partial failure

```yaml qa-scenario
id: ext-02-bundle-activate-atomic
title: A bundle profile activates skills + tools + hooks + bridges atomically; injected reconcile failure rolls back the activation row
theme: extensions.bundle.activate
coverage:
  primary:
    - bundle.activate.atomic
  secondary:
    - extension.skill.verify-content
    - extension.workspace-scope
risk: critical
live: false
provider: real-claude-code
preconditions:
  - Installed extension `bundle-fixture` declaring one bundle with one
    profile that overlays:
    - 2 skills (one with a `session.post_create` hook).
    - 1 tool registered through the bridge SDK.
    - 1 bridge preset.
    - 1 agent definition.
  - A test-only fault flag (`AGH_TEST_BUNDLE_RECONCILE_FAULT=true`)
    that causes `s.store.ApplyBundleActivationResources` to return an
    error AFTER the activation row has been created, exercising the
    rollback path.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/service.go:220
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/service.go:268
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/service.go:401
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/bundle.go:27
steps:
  - Variant A (happy path): `agh bundle activate bundle-fixture --profile default --workspace wsp-ext02`. Capture state.
  - Variant B (rollback): set fault flag; repeat activate; capture errors and final state.
expected:
  - Variant A:
    - `bundle_activations` row created with profile=default,
      scope=workspace.
    - All four overlays observable via `agh bundle get …` /
      `agh skills list` / `agh tools list` / `agh bridge list`.
    - `session.post_create` hook from the bundle's skill is registered
      and visible in hook taxonomy.
  - Variant B:
    - The `Activate` call returns a `joinRollbackFailure`-shaped error
      that wraps both the original reconcile error AND the rollback
      outcome.
    - `bundle_activations` row does NOT exist post-failure (created
      activation fully deleted by `s.store.DeleteBundleActivation`).
    - No partial overlay visible — neither skill nor tool nor hook
      appears in their respective listings.
evidence:
  - `bundle_activations` snapshot in both variants.
  - List outputs for skills/tools/hooks/bridges in both variants.
  - Error payload for variant B.
failure_signatures:
  - Activation row exists post-rollback: atomic violated.
  - Partial overlay visible after rollback: leak; partial-surface
    completion violated.
  - Reconcile error swallowed: trust broken.
cleanup:
  - `agh bundle deactivate <activation-id>` if variant A row exists;
    confirm clean removal.
```

### EXT-03 — Bundle deactivation removes every overlay; nothing leaks across bundles

```yaml qa-scenario
id: ext-03-bundle-deactivate-clean
title: Two bundles activate side-by-side; deactivating bundle A leaves bundle B intact and removes only A's overlays
theme: extensions.bundle.deactivate
coverage:
  primary:
    - bundle.deactivate.clean
    - bundle.activate.no-leak
  secondary:
    - extension.workspace-scope
risk: high
live: false
provider: real-claude-code
preconditions:
  - Two installed extensions, each declaring one bundle:
    - bundle-A: skill `qa-aaa`, tool `aaa.echo`, hook `pre.aaa`.
    - bundle-B: skill `qa-bbb`, tool `bbb.echo`, hook `pre.bbb`.
  - Both activated against the same workspace.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/service.go:401
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/service.go:220
steps:
  - `agh bundle list -o json`; expect two rows.
  - `agh bundle deactivate <bundle-A-activation-id>`.
  - Re-list bundles, skills, tools, hooks. Capture all four.
  - Prompt a real Claude Code session to enumerate available tools.
expected:
  - After deactivate: `bundle_activations` has only bundle-B's row.
  - Skills list lacks `qa-aaa`, still contains `qa-bbb`.
  - Tools list lacks `aaa.echo`, still contains `bbb.echo`.
  - Hooks taxonomy lacks `pre.aaa`, still contains `pre.bbb`.
  - Agent's enumerated tools include `bbb.echo` only (no stale
    `aaa.echo`).
evidence:
  - Pre/post listings for the four resource families.
  - Transcript line of agent's tool enumeration.
failure_signatures:
  - Any of bundle-A's overlays still visible: leak.
  - Bundle-B affected: cross-contamination; serious bug.
cleanup:
  - Deactivate bundle-B and uninstall both extensions.
```

### EXT-04 — Malicious manifest: traversal in path / symlink escape rejected

```yaml qa-scenario
id: ext-04-malicious-manifest
title: A manifest with traversal in resources paths and a symlink-escape source tree is rejected at install time with a critical finding
theme: extensions.security
coverage:
  primary:
    - extension.install.symlink-escape
    - extension.manifest.validate
  secondary:
    - extension.skill.verify-content
risk: critical
live: false
provider: mock-acp
preconditions:
  - Two malicious fixtures:
    - Fixture-A: extension.toml with
      `resources.skills = ["../../../etc/passwd"]` and a placeholder
      tool. The traversal target is outside the source root.
    - Fixture-B: extension.toml referencing a directory whose contents
      include a symlink whose canonical path resolves outside the source
      root (`/tmp/agh-evil-target/`), per
      `internal/extension/install_managed_test.go:298-499` shape.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/install_managed.go:355
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/install_managed.go:483
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/install_managed.go:562
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest.go:269
steps:
  - `agh extension install ./fixtures/malicious-A`. Capture exit code +
    stderr.
  - `agh extension install ./fixtures/malicious-B`. Capture.
  - Audit `extensions` directory for any partially-extracted artifact.
expected:
  - Fixture-A install fails with `ErrManifestInvalid` or
    `extension: invalid manifest` field message; exit code non-zero.
  - Fixture-B install fails with `extension: reject runtime dependency
    symlink` / `symlink target … escapes source root` error; exit code
    non-zero.
  - No partial extension directory was left under `$AGH_HOME/extensions/`.
  - `extensions` table is empty (no row created for either fixture).
  - Stderr does NOT print the absolute traversal target — error
    references the field name, not the resolved path of `/etc/passwd`.
evidence:
  - Captured stderr/error payloads.
  - `ls $AGH_HOME/extensions/` showing no debris.
failure_signatures:
  - Either fixture installs successfully: critical security violation.
  - Partial directory left behind: rollback failure.
  - Resolved sensitive path printed in error: information leak.
cleanup:
  - Remove fixture sources from disk.
```

### EXT-05 — Manifest namespaced metadata round-trip

```yaml qa-scenario
id: ext-05-namespaced-metadata
title: Bridge config_schema using namespaced agh.* metadata round-trips through manifest -> registry -> describe without loss
theme: extensions.manifest
coverage:
  primary:
    - extension.manifest.namespaced-metadata
  secondary:
    - cli.extension.machine-readable
risk: medium
live: false
provider: real-claude-code
preconditions:
  - Fixture extension declaring a bridge adapter with
    `[bridge.config_schema]` `schema = "agh.bridge.fixture-platform"`,
    `version = "1"`, plus two `bridge.secret_slots` entries — same shape
    as `extensions/bridges/slack/extension.toml:24-26`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest.go:99-105
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest.go:332-355
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest_test.go:295,409,426
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/describe.go
steps:
  - Install the fixture.
  - Read the persisted manifest blob from `extensions` table or via
    `agh extension status -o json`.
  - Compare to the source `extension.toml`.
expected:
  - `bridge.platform`, `bridge.display_name`, `bridge.config_schema.schema`,
    `bridge.config_schema.version`, all secret-slot entries, AND the
    namespaced `agh.bridge.fixture-platform` schema id are preserved
    byte-for-byte (after canonical whitespace trimming) in the round-tripped
    payload.
  - No silent drop of any namespaced metadata field.
evidence:
  - Source `extension.toml` and `agh extension status -o json` diff with
    only acceptable whitespace differences.
failure_signatures:
  - Any namespaced metadata field is missing or mutated: format-extension
    invariant violated (per the namespaced-metadata default rule in CLAUDE.md).
cleanup:
  - Uninstall the fixture.
```

### EXT-06 — Real Slack bridge: bidi message round-trip with real Claude Code

```yaml qa-scenario
id: ext-06-slack-real-roundtrip
title: A user message in a Slack workspace channel is delivered to a real Claude Code subagent; agent reply lands back in the same Slack thread
theme: bridges.slack
coverage:
  primary:
    - bridge.lifecycle.transitions
    - bridge.delivery.ordering
    - bridge.signature.verify
    - bridge.secret-redaction
  secondary:
    - extension.lifecycle.recover
    - cli.extension.machine-readable
risk: high
live: conditional
provider: real-claude-code
preconditions:
  - Credential broker pool kind=`slack` reachable; lease acquired with
    `kind=slack`, role=`maintainer` (local CI uses role=`ci`).
  - Lease provides: `bot_token` (xoxb-…), `signing_secret`,
    `team_id`, `channel_id` for an isolated QA Slack workspace.
  - Slack `slack` extension installed from `extensions/bridges/slack/`
    (`extensions/bridges/slack/extension.toml`).
  - Real Claude Code session created in workspace `wsp-ext06`,
    coordinator running, `agh bridge create --extension slack
    --workspace wsp-ext06 --secret-binding ...` issued so the bridge
    instance reaches `ready` status.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/extensions/bridges/slack/provider.go:602,837,915,1094
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/lifecycle.go:9
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/delivery_broker.go:301
steps:
  - From the QA driver Slack account, post a message in the channel:
    "Summarize today's plan as 3 bullets — marker EXT-06-{uuid}".
  - Wait for the Slack bot account (SUT) to post a reply in the same
    thread.
  - Capture: Slack inbound webhook log lines, `bridge_routes` row,
    EventStore deliveries (`bridge.delivery.{start,delta,terminal}`),
    Claude Code transcript.
expected:
  - Slack signing-secret check passes for the legitimate inbound (one
    successful HMAC verification log line).
  - Bridge instance transitions `disabled → starting → ready` and
    remains `ready`.
  - The agent receives the message via the bridge SDK
    `bridges/deliver` host call; transcript shows the EXT-06 marker.
  - The agent's reply produces a `start → delta(s) → terminal`
    sequence on the route; only ONE outbound posted to Slack (no
    duplicate, no token-delta partials leaked through).
  - The reply lands in the same Slack thread (thread_ts respected).
  - In every log/SSE/event payload, `bot_token` and `signing_secret`
    raw values NEVER appear (`grep -E '(xoxb-[A-Za-z0-9-]+)' …`
    returns zero).
evidence:
  - Slack inbound webhook log fragment.
  - Claude Code transcript with marker.
  - EventStore window dump filtered to `bridge.delivery.*`.
  - Redaction grep output (must be empty).
failure_signatures:
  - Bridge stuck in `starting` or oscillating: lifecycle bug.
  - Reply duplicated in Slack: ordering / dedup violated.
  - Signing-secret check passes for a tampered signature: critical
    security; see EXT-13.
  - Token in any sink: critical redaction violation.
cleanup:
  - Disable + delete the bridge instance via CLI.
  - Release the broker lease.
```

### EXT-07 — Real Telegram bridge: bidi flow with bot token honored, secret redacted

```yaml qa-scenario
id: ext-07-telegram-real-roundtrip
title: A Telegram user message routes to AGH; agent replies; bot tokens never appear in logs/SSE
theme: bridges.telegram
coverage:
  primary:
    - bridge.lifecycle.transitions
    - bridge.delivery.ordering
    - bridge.secret-redaction
  secondary:
    - extension.lifecycle.recover
risk: high
live: conditional
provider: real-claude-code
preconditions:
  - Credential broker pool kind=`telegram` reachable; lease acquired.
    Lease payload: `{groupId, driverToken, sutToken}` per openclaw
    pattern (`docs/concepts/qa-e2e-automation.md` line 253).
  - Telegram extension installed from `extensions/bridges/telegram/`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/extensions/bridges/telegram/provider.go
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/delivery_broker.go:301
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/telegram_provider_integration_test.go
steps:
  - Driver bot sends a message in the QA group.
  - SUT bot replies via Claude Code.
  - Capture full lifecycle similar to EXT-06.
expected:
  - SUT replies once; Telegram message_id round-trips in
    `bridge_routes`.
  - Bot tokens absent from every sink (regex: `bot[0-9]+:[A-Za-z0-9_-]+`
    must not match anywhere).
  - Bridge instance reaches `ready` and stays there.
evidence:
  - Telegram getUpdates / webhook log fragment.
  - Bridge delivery EventStore dump.
  - Bot-token redaction grep output (must be empty).
failure_signatures:
  - Bot token in any sink: critical redaction violation.
  - Multiple replies for one inbound: ordering / dedup violated.
cleanup:
  - Stop bridge, release broker lease.
```

### EXT-08 — Bridge backpressure: bounded queue + ordering under load

```yaml qa-scenario
id: ext-08-bridge-backpressure
title: 1000 inbound messages/min from a synthetic adapter; per-route queue stays bounded; ordering preserved; no message lost
theme: bridges.delivery
coverage:
  primary:
    - bridge.delivery.bounded-queue
    - bridge.delivery.ordering
  secondary:
    - extension.host-api.rate-limit
risk: high
live: false
provider: mock-acp
preconditions:
  - Synthetic bridge adapter (under `internal/extensiontest/bridge_adapter_harness.go`)
    pushing 1000 messages over 60 seconds across 4 routes (250 msgs/route).
  - Mock ACP for deterministic ordering.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/delivery_broker.go:33-78,95-143,221,301-450
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/delivery_types.go:38-40
  - /Users/pedronauck/Dev/compozy/agh/internal/extensiontest/bridge_adapter_harness.go
steps:
  - Drive load.
  - Tail `instanceDeliveryMetrics` and `routeWorker.queue` length per
    route via diagnostics endpoint.
  - Capture the sequence of `EventType` values per delivery id.
expected:
  - Per-route queue length never exceeds `defaultDeliveryQueueCapacity`
    + small overshoot (assert `<= 8`).
  - For every delivery id, the captured EventType sequence matches one
    of the legal patterns: `start → delta* → terminal` or
    `start → resume → delta* → terminal`.
  - Total delivered count equals total injected (1000); no drop, no
    duplicate.
  - `instanceDeliveryMetrics.deliveryFailuresTotal == 0`.
evidence:
  - Per-route metrics dump.
  - Per-delivery EventType trace (sampled and full counts).
failure_signatures:
  - Queue length unbounded (>>8): bounded-queue invariant violated.
  - Out-of-order or duplicated event: ordering invariant violated.
  - `delivered != injected`: message loss.
cleanup:
  - Stop synthetic adapter.
```

### EXT-09 — Bridge auth failure: revoked token produces clean error, retries with backoff

```yaml qa-scenario
id: ext-09-bridge-auth-failure
title: Slack bot token revoked mid-run; bridge transitions to auth_required; user-visible reason surfaced; no flooded log retries
theme: bridges.auth
coverage:
  primary:
    - bridge.auth.surface
    - bridge.lifecycle.transitions
    - bridge.secret-redaction
  secondary:
    - extension.lifecycle.recover
risk: high
live: conditional
provider: real-claude-code
preconditions:
  - Slack bridge from EXT-06 in `ready` state.
  - Broker provides a "revocable" Slack pair where the maintainer
    can flip the bot token to invalid mid-run (or use a fixture-only
    Slack mock that simulates `auth_revoked`).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/lifecycle.go:86-96
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/types.go:123-124
  - /Users/pedronauck/Dev/compozy/agh/extensions/bridges/slack/provider.go:837,915
steps:
  - With the bridge `ready`, revoke the bot token (or simulate via mock).
  - Send 5 inbound messages over 60s.
  - Capture: `bridge_instances` status transitions, retry log lines,
    user-visible error surfaced via CLI / web surface.
expected:
  - Bridge transitions `ready → auth_required` (NOT `error` —
    `auth_required` is the documented surface for reauth-needed).
  - CLI `agh bridge get <id> -o json` returns
    `status: "auth_required"`, `degradation` populated with a
    clean human-readable reason (no stack trace, no token).
  - Retry log shows exponential / capped backoff — fewer than 10
    retry attempts in 60s, not "flooded" (>=100).
  - No raw bot token appears in any retry log entry.
  - In-flight inbound messages either drain successfully (if pre-revoke)
    or surface a typed `auth_required` reason to the channel client; no
    silent drops.
evidence:
  - `bridge_instances` snapshots T0/T1/T2.
  - Retry log fragment with timestamp deltas.
  - User-visible error payload from CLI + web.
failure_signatures:
  - Status went to `error` instead of `auth_required`: lifecycle wrong.
  - >= 100 retries in 60s: backoff missing / log flood.
  - Token in retry log: critical redaction violation.
cleanup:
  - Restore valid token (or replay the broker lease) and stop bridge.
```

### EXT-10 — Bridge SDK contract: TS extension scaffold builds and round-trips a tool

```yaml qa-scenario
id: ext-10-sdk-tool-roundtrip
title: A TS extension authored via @agh/create-extension scaffold registers a tool; daemon picks it up; agent invokes it; output round-trips
theme: extensions.sdk
coverage:
  primary:
    - bridge.sdk.contract
    - bridge.sdk.scaffold
    - cli.extension.machine-readable
  secondary:
    - extension.skill.verify-content
risk: high
live: false
provider: real-claude-code
preconditions:
  - `bun` (or `npm`) installed.
  - SDK packages (`@agh/extension-sdk`, `@agh/create-extension`) built
    locally via `make bun-typecheck` or sourced from `dist/`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/sdk/create-extension/src/index.ts:30-80
  - /Users/pedronauck/Dev/compozy/agh/sdk/create-extension/templates/tool-provider/
  - /Users/pedronauck/Dev/compozy/agh/sdk/typescript/src/extension.ts:42-49,109
  - /Users/pedronauck/Dev/compozy/agh/sdk/typescript/src/integration.test.ts
steps:
  - Run `bun create @agh/extension qa-tool-fixture --template tool-provider --dir /tmp/ext-10/`.
  - Inspect generated tree: must contain `extension.json`, `package.json`,
    `src/`, `tsconfig.json`.
  - `cd /tmp/ext-10/qa-tool-fixture && bun install && bun run build`.
  - `agh extension install /tmp/ext-10/qa-tool-fixture` and restart daemon.
  - Real Claude Code session: prompt agent to invoke the scaffolded
    tool with a unique marker.
expected:
  - Scaffold completes without prompt; tree matches template fixtures.
  - Build succeeds; output bundle present where extension.json points.
  - `agh extension install` succeeds; status payload shows
    `capabilities.provides=["tool.provider"]`.
  - On agent invocation: tool's input/output round-trips; transcript
    shows the canonical tool id; output contains the marker.
  - SDK initialize handshake honors `REQUIRED_PROVIDES_METHODS`: the
    scaffold's stub registers both `provide_tools` and `tools/call`.
evidence:
  - Generated tree listing.
  - Build output log.
  - Transcript fragment.
failure_signatures:
  - Scaffold produces a tree missing `tsconfig.json` / `extension.json`:
    template drift.
  - Initialize fails with `provider claims tool.provider but missing
    methods`: contract regression.
  - Tool round-trip fails: SDK<->daemon contract broken.
cleanup:
  - Remove `/tmp/ext-10/`. Uninstall the extension.
```

### EXT-11 — `agh extension list` and `extension status` machine-readable parity (HTTP + UDS)

```yaml qa-scenario
id: ext-11-cli-machine-readable-parity
title: agh extension list -o json and agh extension status -o json return identical payloads via HTTP, UDS, and offline (local registry) paths
theme: extensions.cli
coverage:
  primary:
    - cli.extension.machine-readable
  secondary:
    - extension.manifest.validate
risk: medium
live: false
provider: real-claude-code
preconditions:
  - Two extensions installed and one disabled.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/extension.go:73,220,235
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/extension_test.go:155,260
steps:
  - Run `agh extension list -o json` against the running daemon (HTTP).
  - Run `agh extension list -o json` with `AGH_TRANSPORT=uds`.
  - Stop daemon. Run `agh extension list -o json` (forces local
    registry path via `withLocalExtensionRegistry`).
  - Same triple for `agh extension status <name>` for each extension.
expected:
  - All three outputs for `extension list` are identical sets (after
    sorting by `name`); fields `enabled`, `version`, `manifest_path`,
    `installed_at`, `capabilities`, `actions`, `checksum`,
    `registry_slug`, `registry_name`, `remote_version` agree.
  - All three `extension status` outputs are identical for the SAME
    extension.
  - Disabled extension still appears in `list` with `enabled=false`.
  - No `requires_env` value appears in human-mode output (per
    `extension_test.go:312-337` — env values must NOT be leaked).
evidence:
  - Three captured payloads per command.
  - Diff output (must be empty).
failure_signatures:
  - Any field diverges across transports: parity violated.
  - Env values leak in human output: information leak.
cleanup:
  - None.
```

### EXT-12 — Hot install + workspace scoping (workspace W vs W')

```yaml qa-scenario
id: ext-12-hot-install-workspace-scope
title: After install + restart, the new tool/skill is available in workspace W where the bundle is activated; agent in workspace W' does NOT see it
theme: extensions.workspace-scope
coverage:
  primary:
    - extension.workspace-scope
    - bundle.activate.no-leak
  secondary:
    - cli.extension.machine-readable
risk: high
live: false
provider: real-claude-code
preconditions:
  - Two workspaces `wsp-W` and `wsp-W2`, each with one active
    Claude Code session.
  - Bundle `qa-scope-fixture` installed (extension supplies one bundle
    that overlays one tool `scope.echo` and one skill `qa-scope`).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/service.go:163-330
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/model/model.go
steps:
  - `agh bundle activate qa-scope-fixture --profile default --workspace wsp-W`.
  - Restart daemon.
  - Ask agent in `wsp-W` to enumerate tools and call `scope.echo`.
  - Ask agent in `wsp-W2` to enumerate tools and try the same.
expected:
  - Agent in `wsp-W` sees `scope.echo` in its tool list and the
    invocation succeeds.
  - Agent in `wsp-W2` does NOT see `scope.echo` in its tool list; if
    it attempts to call by name, daemon returns a typed
    `tool not available in workspace` error.
  - `agh bundle list -o json --workspace wsp-W2` shows the bundle
    activation only when filter respects the right scope.
evidence:
  - Two transcripts (one per workspace).
  - `bundle_activations` row showing `scope=workspace`,
    `workspace_id=wsp-W`.
failure_signatures:
  - `wsp-W2` agent sees the tool: workspace scoping leak; serious bug.
  - `wsp-W` agent does NOT see the tool: activation failed.
cleanup:
  - Deactivate; uninstall extension.
```

### EXT-13 — Bridge signature verification rejects forgeries (Slack + Discord + Telegram)

```yaml qa-scenario
id: ext-13-bridge-signature-verify
title: Tampered or unsigned webhooks are rejected at the bridge boundary across providers; no host API call attempted
theme: bridges.security
coverage:
  primary:
    - bridge.signature.verify
    - bridge.secret-redaction
  secondary:
    - bridge.lifecycle.transitions
risk: critical
live: false
provider: mock-acp
preconditions:
  - All three providers installed.
  - Each bridge instance in `ready` state with seed secrets.
  - Local HTTP driver that POSTs forged webhooks to each provider's
    listen address.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/extensions/bridges/slack/provider.go:1094
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/discord_provider_integration_test.go:34-170,417
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/telegram_provider_integration_test.go
steps:
  - Slack: POST a webhook with mutated body byte; signature mismatches.
  - Slack: POST with stale (>5min old) `X-Slack-Request-Timestamp`.
  - Discord: POST with bad ed25519 signature.
  - Telegram: POST without the bot-token-derived secret-token header.
  - For each: capture HTTP response, daemon log, and verify NO
    `IngestBridgeMessage` host call was made.
expected:
  - Each provider returns 4xx (typed `invalid signature` /
    `stale timestamp` / `unauthorized`) within 50ms of the attempt.
  - No host API ingress event for any of the four forgeries.
  - No `IngestBridgeMessage` log line correlated to the forged inbound.
  - No raw secret value (signing_secret / discord public key /
    bot_token) in the rejection log line.
evidence:
  - HTTP responses (4 attempts).
  - EventStore filter on `bridge.ingest.*` (must be empty for the
    scenario window).
  - Daemon log fragment showing rejection.
failure_signatures:
  - Any forgery accepted (host call attempted): critical security.
  - Secret value in any error path: critical redaction.
cleanup:
  - None (read-only attack).
```

### EXT-14 — Extension lifecycle hook: session.post_create runs in hierarchy + alphabetical order

```yaml qa-scenario
id: ext-14-lifecycle-hook-order
title: Extension declares a session.post_create hook; it runs in hierarchy precedence and alphabetical order alongside other extensions and skills
theme: extensions.hooks
coverage:
  primary:
    - extension.lifecycle-hook.session-postcreate
  secondary:
    - extension.skill.verify-content
risk: medium
live: false
provider: real-claude-code
preconditions:
  - Three extensions installed:
    - ext-bbb (declares session.post_create hook with handler that
      writes a marker to a side-channel file).
    - ext-aaa (same shape; alphabetically first).
    - ext-ccc (legacy fixture using `on_session_created` event).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/skills/loader.go:517-526
  - /Users/pedronauck/Dev/compozy/agh/internal/skills/loader_test.go:650-660
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest.go:108-130
steps:
  - Variant A: install ext-aaa + ext-bbb (no legacy). Create a real
    session in workspace; capture marker-file ordering and hook
    event names.
  - Variant B: install ext-ccc that uses the legacy
    `on_session_created` name; verify load fails with the documented
    translation message.
expected:
  - Variant A: marker file shows hooks fired with event
    `session.post_create` (canonical), executed in alphabetical
    order across extensions of the same precedence (`aaa` before
    `bbb`); five-layer precedence (bundled → marketplace → user →
    additional → workspace) honored if extensions are at different
    layers.
  - Variant B: install / load fails with error
    `hook event "on_session_created" was removed; use "session.post_create"`
    (verbatim, per `loader.go:519` translation logic and
    `loader_test.go:660` test).
evidence:
  - Marker file contents (Variant A).
  - Captured load error (Variant B).
failure_signatures:
  - Variant A: ordering wrong or events not fired.
  - Variant B: legacy event accepted silently → vocabulary regression.
cleanup:
  - Uninstall extensions.
```

### EXT-15 — Extension subprocess panic: contained, marked unhealthy, agent receives typed error, daemon NOT crashed

```yaml qa-scenario
id: ext-15-subprocess-panic-contained
title: An extension subprocess panics in a host API call; daemon catches, logs, marks the extension unhealthy; daemon stays alive
theme: extensions.lifecycle
coverage:
  primary:
    - extension.lifecycle.recover
  secondary:
    - extension.host-api.rate-limit
    - cli.extension.machine-readable
risk: critical
live: false
provider: real-claude-code
preconditions:
  - Fixture extension `crashy-fixture` whose tool handler panics on a
    specific input marker (`__CRASH__`).
  - `AGH_BRIDGE_ADAPTER_CRASH_ONCE_PATH` style fault flag is supported
    by the SDK (used by Slack provider — see
    `extensions/bridges/slack/extension.toml:48`).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manager.go:1035-1149
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manager.go:125
steps:
  - Real Claude Code session calls the fixture tool with `__CRASH__`.
  - Capture: daemon log, `extensions` status, EventStore.
  - Wait for backoff window; observe recovery attempt.
  - Have the agent retry without the crash marker.
expected:
  - Daemon does NOT crash (PID identical pre/post).
  - First call returns a typed error to the agent (mapped from
    JSON-RPC error code, e.g. `HostAPIUnavailableCode = -32005`).
  - `agh extension status crashy-fixture` reports the runtime as
    unhealthy (with `lastError` set, `phase=recover`).
  - Manager's `recoverExtension` runs after backoff; on success the
    extension transitions back to active.
  - Subsequent (non-crash) tool call succeeds.
evidence:
  - PID before/after.
  - Daemon log showing `extension.lifecycle.shutdown_failed` /
    `extension.lifecycle.loaded` recovery line (per
    `manager.go:1146`).
  - EventStore window dump.
failure_signatures:
  - Daemon crash: containment failed; critical.
  - Agent gets opaque 500 instead of typed JSON-RPC error: contract
    regression.
  - Recovery never attempted: lifecycle regression.
cleanup:
  - Uninstall fixture.
```

### EXT-16 — Real-LLM scenario: install bridge extension, agent uses two of its tools in one multi-turn conversation

```yaml qa-scenario
id: ext-16-real-llm-multi-tool
title: Install a bridge extension (slack) plus a tool-provider extension; agent uses two distinct tools across a multi-turn conversation; transcript shows tool ids + results
theme: extensions.end-to-end
coverage:
  primary:
    - extension.skill.verify-content
    - bridge.sdk.contract
    - bundle.activate.atomic
  secondary:
    - cli.extension.machine-readable
    - bridge.delivery.ordering
risk: high
live: false
provider: real-claude-code
preconditions:
  - `slack` extension installed in mock-Slack mode (uses
    `AGH_BRIDGE_SLACK_API_BASE_URL` to point at the test harness;
    `extensions/bridges/slack/extension.toml:50`).
  - Companion tool-provider extension `qa-multitool` installed
    (registers two tools: `qa.lookup` and `qa.summarize`).
  - Real Claude Code session in workspace `wsp-ext16`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/extensions/bridges/slack/extension.toml
  - /Users/pedronauck/Dev/compozy/agh/sdk/typescript/src/extension.ts:42-49
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/delivery_broker.go:301
steps:
  - Drive a 6-turn conversation with the agent that requires:
    Turn 2: call `qa.lookup` for marker E16-A.
    Turn 4: call `qa.summarize` with the lookup output.
    Turn 6: post the summary to the Slack QA channel via the bridge.
  - Capture transcript + EventStore + Slack mock outbound buffer.
expected:
  - Transcript shows both tool calls with canonical tool ids and
    their results inline.
  - `tool_calls` ledger contains the two calls in order with
    distinct ids.
  - Slack mock outbound buffer contains exactly one message with
    the summary text.
  - Bridge delivery EventStore: one start/delta(s)/terminal sequence.
evidence:
  - Transcript fragment with both tool calls.
  - `tool_calls` table snapshot for the session window.
  - Slack mock outbound buffer.
failure_signatures:
  - Either tool not invoked: tool selection failure.
  - Bridge delivery does not produce a terminal: streaming failure.
  - Multiple Slack outbounds: dedup violated.
cleanup:
  - Uninstall both extensions.
```

### EXT-17 — Managed-extension dependency copy: resolved targets stay inside approved roots

```yaml qa-scenario
id: ext-17-managed-dependency-copy
title: install_managed copy verifies every resolved dependency target remains inside source root; symlink-escape rejected even with a directory cycle
theme: extensions.security
coverage:
  primary:
    - extension.install.symlink-escape
  secondary:
    - extension.manifest.validate
risk: critical
live: false
provider: mock-acp
preconditions:
  - Three malicious source trees:
    - Tree-A: a symlink whose canonical target resolves to
      `/tmp/agh-evil-target/` (outside the source root).
    - Tree-B: a directory cycle (A → B → A) created via symlinks
      under the source root.
    - Tree-C: a legitimate symlink whose target stays inside the
      source root (control / positive path).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/install_managed.go:355,483,536,562
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/install_managed_test.go:298-499
steps:
  - Drive `copyInstallTree` (or trigger via marketplace install with a
    fixture) for each tree.
  - Verify error / success outcomes.
expected:
  - Tree-A: error wraps `extension: reject runtime dependency symlink
    %q` and references the source-root containment violation.
  - Tree-B: error wraps `extension: symlink directory cycle detected
    from %q to %q` (per `install_managed.go:536`).
  - Tree-C: copy succeeds; final checksum stable across reruns.
evidence:
  - Captured error strings for A and B.
  - Successful checksum diff for C.
failure_signatures:
  - Either A or B succeeds: symlink hardening regressed; critical.
  - C fails: false positive.
cleanup:
  - Remove fixtures.
```

### EXT-18 — Skill VerifyContent applied to every non-bundled extension skill

```yaml qa-scenario
id: ext-18-skill-verify-content-extension
title: An extension shipping a skill with prompt-injection content is rejected at load via VerifyContent; bundled-extension skills are exempt
theme: extensions.security
coverage:
  primary:
    - extension.skill.verify-content
  secondary:
    - extension.manifest.validate
risk: high
live: false
provider: mock-acp
preconditions:
  - Extension fixture `qa-injected` shipping a skill markdown that
    matches a critical pattern from `internal/skills/verify.go`
    (e.g. an explicit prompt-injection lure).
  - Bundled-skill counterexample: a skill with similar content is
    embedded via `go:embed` under `internal/skills/bundled/`. Bundled
    skills should NOT be scanned (per the security invariant).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/skills/registry.go:504
  - /Users/pedronauck/Dev/compozy/agh/internal/skills/verify.go:101-102
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Security Invariants — Load-time security scan)
  - /Users/pedronauck/Dev/compozy/agh/internal/skills/registry_test.go:532,571
steps:
  - Install `qa-injected`. Restart daemon. Capture daemon log + skill
    catalog state.
  - Confirm the bundled counterexample loads without rejection.
expected:
  - `qa-injected`'s skill is rejected at load with a critical-severity
    `VerifyContent` warning; the skill is NOT registered in the
    catalog.
  - Daemon log includes a critical-finding line referencing the
    skill's name + finding pattern (no raw injection content quoted).
  - Bundled skill loads; its row appears in the catalog.
  - `qa-injected` extension itself remains installed, but no skill is
    activated from it (so the agent cannot pick up the malicious
    content).
evidence:
  - Catalog state for both fixtures.
  - Daemon log fragment.
failure_signatures:
  - Malicious skill loaded: critical security violation.
  - Bundled skill rejected: false positive; immutability assumption
    broken.
cleanup:
  - Uninstall `qa-injected`.
```

## 8. Optional / nice-to-have scenarios (run if time)

### EXT-19 — Bridge config-schema validation rejects unknown fields

```yaml qa-scenario
id: ext-19-bridge-config-schema-strict
title: Bridge create with provider_config containing fields outside the declared bridge.config_schema is rejected with a typed error
theme: bridges.config
coverage:
  primary:
    - extension.manifest.namespaced-metadata
  secondary:
    - bridge.lifecycle.transitions
risk: medium
live: false
provider: mock-acp
preconditions:
  - Slack bridge installed; provider_config schema declares fixed set
    of fields.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/extension/manifest.go:99-105
  - /Users/pedronauck/Dev/compozy/agh/internal/bridges/registry.go:42-110
steps:
  - `agh bridge create --extension slack --workspace wsp-ext19
    --provider-config '{"bot_token_alias":"foo","unknown_field":"x"}'`.
expected:
  - Returns 4xx with typed message naming the unknown field.
evidence:
  - Captured error.
failure_signatures:
  - Bridge created with garbage config: schema not enforced.
cleanup:
  - None.
```

### EXT-20 — Bundle Live requirement needs explicit confirmation

```yaml qa-scenario
id: ext-20-bundle-network-requirement
title: Activating a bundle with an unconfirmed Live requirement fails without partial state
theme: bundles.network
coverage:
  primary:
    - bundle.network-requirement-confirmation
  secondary:
    - bundle.activate.atomic
risk: medium
live: false
provider: mock-acp
preconditions:
  - An extension manifest declares a Live Network participation requirement.
  - No activation has confirmed the current requirement digest.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/bundles/network_requirement.go
steps:
  - Activate without confirmation. Capture the error and activation inventory.
  - Activate again with `confirm_network_requirement=true`.
expected:
  - The first call returns `ErrNetworkRequirementConfirmationRequired`; no activation row or
    projected resource is created.
  - The confirmed call succeeds and records the exact digest, confirmer, and timestamp.
  - Declared channels remain inventory only and do not enroll an execution.
evidence:
  - Captured error + activation and projected-resource snapshots.
failure_signatures:
  - Unconfirmed activation succeeds or leaves partial resources: invariant violated.
cleanup:
  - Deactivate the confirmed activation.
```

## 9. Coverage matrix (this child)

| Coverage ID                                   | Scenarios                                                                |
| --------------------------------------------- | ------------------------------------------------------------------------ |
| `extension.manifest.validate`                 | EXT-01, EXT-04, EXT-11, EXT-17, EXT-18                                   |
| `extension.manifest.compat`                   | (covered indirectly by EXT-01 fixture; explicit static check possible)   |
| `extension.manifest.namespaced-metadata`      | EXT-05, EXT-19                                                           |
| `extension.install.symlink-escape`            | EXT-04, EXT-17                                                           |
| `extension.install.checksum`                  | EXT-01                                                                   |
| `extension.install.restart-guidance`          | EXT-01                                                                   |
| `extension.lifecycle.recover`                 | EXT-06, EXT-07, EXT-09, EXT-15                                           |
| `extension.host-api.rate-limit`               | EXT-08, EXT-15                                                           |
| `extension.skill.verify-content`              | EXT-02, EXT-10, EXT-16, EXT-18                                           |
| `extension.lifecycle-hook.session-postcreate` | EXT-14                                                                   |
| `extension.workspace-scope`                   | EXT-02, EXT-03, EXT-12                                                   |
| `bundle.activate.atomic`                      | EXT-02, EXT-16, EXT-20                                                   |
| `bundle.deactivate.clean`                     | EXT-03                                                                   |
| `bundle.activate.no-leak`                     | EXT-03, EXT-12                                                           |
| `bundle.network-requirement-confirmation`     | EXT-20                                                                   |
| `bridge.lifecycle.transitions`                | EXT-06, EXT-07, EXT-09, EXT-13, EXT-19                                   |
| `bridge.delivery.bounded-queue`               | EXT-08                                                                   |
| `bridge.delivery.ordering`                    | EXT-06, EXT-07, EXT-08, EXT-16                                           |
| `bridge.auth.surface`                         | EXT-09                                                                   |
| `bridge.secret-redaction`                     | EXT-06, EXT-07, EXT-09, EXT-13                                           |
| `bridge.signature.verify`                     | EXT-06, EXT-13                                                           |
| `bridge.sdk.contract`                         | EXT-10, EXT-16                                                           |
| `bridge.sdk.scaffold`                         | EXT-10                                                                   |
| `cli.extension.machine-readable`              | EXT-01, EXT-06, EXT-09, EXT-10, EXT-11, EXT-12, EXT-15, EXT-16           |

Total: 18 mandatory + 2 optional = 20 scenarios. Every coverage ID is
exercised by at least one scenario; security-critical IDs
(`extension.install.symlink-escape`, `bridge.signature.verify`,
`bridge.secret-redaction`, `extension.skill.verify-content`,
`bundle.activate.atomic`) are exercised by at least two each.

## 10. Forbidden-needle list (transcript and event payloads)

Per the openclaw `forbiddenNeedles` pattern. None of the following may
appear in any outbound message, transcript, SSE event, or audit log
across any EXT scenario:

- Slack bot token shape: `xoxb-[A-Za-z0-9-]+`.
- Slack signing-secret shape: 32-hex character secret literal.
- Telegram bot-token shape: `bot[0-9]+:[A-Za-z0-9_-]+`.
- Discord bot-token shape: `[A-Za-z0-9_-]{24}\.[A-Za-z0-9_-]{6}\.[A-Za-z0-9_-]{27,}`.
- Generic provider keys: `sk-`, `xoxb-`, `AKIA`, `ya29.`.
- Raw `agh_claim_*` (per the autonomy kernel forbidden-needle list —
  applies in cross-cutting EXT scenarios that reach into autonomy).
- Resolved sensitive paths leaking into error messages
  (`/etc/passwd`, `~/.ssh/`, `/private/var/folders/.../auth`).
- The deleted legacy hook event name `on_session_created` in any new
  artifact (only acceptable in EXT-14 Variant B as the rejected input,
  not in any happy-path output).

A single scenario test failure on this list is shippability-critical and
must be triaged immediately.

## 11. Credential broker (live lanes)

Live Slack and Telegram lanes (EXT-06, EXT-07, EXT-09 conditional)
require pooled credentials per the openclaw `convex-credential-broker`
pattern — adapted for AGH per the §2 directive in the openclaw
reference: substrate is local-first SQLite (or a thin HTTP service),
not Convex.

Required broker contract (mirrors openclaw verbatim except for
substrate):

- Endpoints: `/acquire`, `/heartbeat`, `/release`,
  `/admin/{add,remove,list}`.
- Pool kinds: `slack`, `telegram` (extendable: `discord`, `gchat`,
  `linear`, `teams`, `whatsapp`, `github`).
- Selection: least-recently-leased.
- Heartbeat: required for the duration of the run; release on
  shutdown.
- Role split: `EXT_BRIDGE_BROKER_SECRET_MAINTAINER` vs `_CI`.
- Payload shape per kind:
  - `slack`: `{teamId, botToken, signingSecret, botUserId, channelId,
    driverUserToken}`.
  - `telegram`: `{groupId, driverToken, sutToken}` (numeric chat-id
    string).
- Failure mode: callers fail fast when broker unreachable; never
  silently fall back to no-credential.
- Local QA opt-out: `EXT_SKIP_LIVE=1` in env skips live-lane scenarios
  with `outcome=blocked`, never `outcome=worked`.

This child's reporting MUST distinguish `worked` (live executed),
`blocked` (broker unreachable, no lease), and `failed` (lease acquired
but scenario assertions failed).

## 12. Reporting contract

Each scenario writes the four-artifact set required by the openclaw
operator-flow pattern (markdown report + JSON summary + observed events
+ combined log). The aggregate `ext-summary.json` for this child carries
the coverage matrix from §9 alongside per-scenario `outcome ∈ {worked,
failed, blocked, follow-up}` and machine-readable timing.

The scenario operator runs in-character (per the `real-scenario-qa`
skill); every run ends with a Worked / Failed / Blocked / Follow-up
section covering all 18 mandatory scenarios. A child run is shippable
only when:

- Every mandatory scenario is `worked` or has an explicit accepted
  follow-up. (Live conditional lanes EXT-06, EXT-07, EXT-09 may be
  `blocked` if the broker is unreachable, but not `failed`.)
- EXT-04, EXT-13, EXT-17, EXT-18 are all clean (security floor —
  symlink hardening, signature verification, dependency containment,
  skill verify-content).
- No forbidden-needle hit anywhere.
- `make verify` passed on the SUT branch before this child ran (cite
  commit SHA in `ext-summary.json`).

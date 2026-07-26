---
name: 11-api-cli-parity
title: HTTP/SSE + UDS + CLI Parity + OpenAPI Codegen — Real-LLM QA Plan
description: Behavior-first QA scenarios for the AGH transport surfaces — `internal/api/contract`, `internal/api/core` (BaseHandlers), `internal/api/httpapi`, `internal/api/udsapi`, `internal/sse`, `internal/cli`, `internal/codegen`, `openapi/agh.json`, `web/src/generated/agh-openapi.d.ts`, the `mage Boundaries` rule, and the codegen+cli-docs Make targets. Closes the parity loop end-to-end so any state-transition operation is reachable from HTTP, UDS, and CLI with identical envelope and identical authority.
type: final-qa-child
module: api-cli-parity
parent: ../_parent.md
provider_lanes: [claude-code]
authoritative_runtime_truth:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
---

# 11 — HTTP/SSE + UDS + CLI Parity + OpenAPI Codegen

Sibling of `03-acp-sessions.md` (which proves real ACP behavior end-to-end through the prompt path) and `04-autonomy-kernel.md` (which proves the autonomy-kernel state machine). This module proves **the transport surfaces themselves** — that every agent-manageable runtime capability shows up identically over HTTP, UDS, and CLI; that BaseHandlers is the single canonical handler home; that the codegen pipeline (`make codegen` / `make codegen-check`) refuses drift; that SSE replay/backpressure are durable; that the `mage Boundaries` rule keeps low-level packages clean of API/daemon/CLI imports; and that the CLI's `-o json` / `-o jsonl` envelopes are stable enough for agents to script against.

The CLAUDE.md invariants this child encodes:

- "**`internal/api/core` is the canonical handler home.** REST/UDS endpoints exist as shared `BaseHandlers` methods; HTTP and UDS only choose registration and authentication. No transport-duplicated parsing/validation." (`internal/CLAUDE.md:23`).
- "**Agent-manageable by default.** … CLI verbs with `-o json` / `-o jsonl` where relevant, HTTP/UDS parity when state crosses the daemon boundary…" (`internal/CLAUDE.md:26`).
- "**No partial-surface completions.** Any change touching a public surface closes the loop end-to-end in one pass: contract → HTTP handler → UDS handler → CLI client → CLI command → extension/config/docs surfaces → tests → docs." (`internal/CLAUDE.md:27`).
- "**Codegen drift fails CI** (`make codegen-check`)." (root `CLAUDE.md` Build Commands).
- "**CI-enforceable boundaries** — `mage Boundaries` rules prevent import cycles. Update `magefile.go` Boundaries() in the same commit that introduces a new `internal/api/*` subpackage." (`internal/CLAUDE.md:22`).
- "**Live broadcasters publish only after durable append; reconnect/replay uses `after_seq`.**" (`internal/CLAUDE.md:52`).

Every scenario below is written for the **real-claude-code** lane unless explicitly marked otherwise. Mocks are only used where the test target is a control-plane invariant (codegen drift, route-coverage matrix, parity matrix), since those are independent of any LLM token stream.

## 1. Module surface — Parity Matrix (HTTP × UDS × CLI)

The OpenAPI spec is the source of truth. **`internal/api/spec/Operations()` returns 202 distinct `OperationSpec` entries** (`grep -hE "OperationID:" internal/api/spec/*.go | sort -u | wc -l` → 202; assertion in `internal/api/spec/spec_test.go:1219-1232` enforces uniqueness). Every operation declares a `Method`, `Path`, `OperationID`, and `Transports []Transport` field with values `TransportHTTP` and/or `TransportUDS` (`internal/api/spec/spec.go:121-122,144-157`).

The full per-operation table is embedded in code: see `internal/api/spec/spec.go` (`var operationRegistry = []OperationSpec{...}`, lines 210-3547), `internal/api/spec/authored_context.go` (every `OperationSpec`), `internal/api/spec/settings.go`, `internal/api/spec/vault.go`, `internal/api/spec/resources_test.go` (the resource subset). The QA runner MUST read those files at scenario start and assert the matrix by group rather than by hand-curated copy. The grouped breakdown below is what the report renders to operators.

> Real-Claude-Code parity proof for the prompt path lives in scenarios `API-02` and `API-03`; the typed-envelope state machine that those rely on is at `internal/api/httpapi/prompt.go:251-580` and emits `start`, `text-start`, `text-delta`, `text-end`, `tool-input-start`, `tool-input-available`, `tool-output-available`, `data-agh-event`, `data-agh-permission`, `error`, `finish`, `[DONE]` (per `prompt.go:251-580`).

### 1.a Group-level parity (HTTP + UDS + CLI)

| Group              | Operations | HTTP route reg                              | UDS route reg                              | CLI verb root                                  | All three? | Notes                                                                                                                                          |
| ------------------ | ---------- | ------------------------------------------- | ------------------------------------------ | ---------------------------------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `agent` (kernel)   | 14         | `internal/api/httpapi/routes.go:89-112`     | `internal/api/udsapi/routes.go:111-133`    | `agh me`, `agh spawn`, `agh ch`                | partial    | `agh me`, `agh ch`, `agh spawn` cover `getAgentMe` / `listAgentChannels` / `receiveAgentChannelMessages` / `sendAgentChannelMessage` / `replyAgentChannelMessage` / `spawnAgentSession`. `getAgentContext` exposed via `agh me context`. Coordinator config and channels are UDS-also; HTTP exposes a subset (no `agent kernel` group registered on HTTP). Verify gap: HTTP `/api/agent/coordinator/config` is registered only on UDS (`udsapi/routes.go:118`); HTTP returns 404 — this is intentional: agent-kernel routes are UDS-first. Document this explicitly in the QA report. |
| `agents`           | 17         | `httpapi/routes.go:95-112`                  | `udsapi/routes.go:90-109`                  | `agh agent`                                    | yes        | Soul + heartbeat full CRUD + history + rollback + wake.                                                                                          |
| `automation`       | 15         | `httpapi/routes.go:168-191`                 | `udsapi/routes.go:193-217`                 | `agh automation`                               | yes        | Jobs + triggers + runs + history + delete.                                                                                                      |
| `bridges`          | 14         | `httpapi/routes.go:38-54`                   | `udsapi/routes.go:32-50`                   | `agh bridge`                                   | yes        | Includes `POST /api/bridges/:id/test-delivery`.                                                                                                  |
| `bundles`          | 8          | `httpapi/routes.go:268-278`                 | `udsapi/routes.go:308-320`                 | `agh bundle`                                   | yes        |                                                                                                                                                  |
| `daemon`           | 1          | `httpapi/routes.go:249-252`                 | `udsapi/routes.go:285-290`                 | `agh status`                            | yes        | Single `getDaemonStatus`. CLI lifecycle (`start`/`stop`/`relaunch`) is local-only and not an HTTP/UDS operation.                                |
| `extensions`       | 5          | `httpapi/routes.go:280-288`                 | `udsapi/routes.go:322-331`                 | `agh extension`                                | yes        | HTTP `POST /api/extensions/...` requires `privilegedMutationGuard` (loopback only); CLI default uses UDS so it bypasses the loopback check.      |
| `hooks`            | 3          | `httpapi/routes.go:125-130`                 | `udsapi/routes.go:150-157`                 | `agh hooks`                                    | yes        |                                                                                                                                                  |
| `memory`           | 9          | `httpapi/routes.go:236-247`                 | `udsapi/routes.go:270-283`                 | `agh memory`                                   | yes        |                                                                                                                                                  |
| `network`          | 11         | `httpapi/routes.go:254-266`                 | `udsapi/routes.go:292-306`                 | `agh network`                                  | yes        | `POST /api/network/send` plus channel/peer reads.                                                                                                |
| `observe`          | 4          | `httpapi/routes.go:114-123`                 | `udsapi/routes.go:135-148`                 | `agh observe`                                  | yes        | `GET /api/observe/events/stream` is SSE only — see §1.b.                                                                                         |
| `resources`        | 5          | `httpapi/routes.go:132-148`                 | `udsapi/routes.go:159-168`                 | (none today)                                  | partial    | **GAP**: `agh resource` verb does not exist; HTTP requires `resourceAuthMiddleware`; UDS-only is the agent path. Either CLI `agh resource` or explicit no-CLI doc note required. CLI gap noted in `cy-web-docs-impact` follow-up. |
| `sessions`         | 17         | `httpapi/routes.go:66-87`                   | `udsapi/routes.go:64-87`                   | `agh session`                                  | yes        | Includes `:id/prompt`, `:id/prompt/cancel`, `:id/stream` (SSE).                                                                                  |
| `settings`         | 25         | `httpapi/routes.go:290-334`                 | `udsapi/routes.go:333-376`                 | `agh config`                                   | partial    | HTTP mutations are loopback-only (`privilegedMutationGuard`). CLI uses local config edit + UDS reload. **GAP**: only `general/memory/skills/...` GET surfaces are UDS-mirrored; some PUTs go via UDS-no-auth. Per scenario API-09 below, prove that CLI flow can read every settings group. |
| `skills`           | 5          | `httpapi/routes.go:227-234`                 | `udsapi/routes.go:259-268`                 | `agh skill`                                    | yes        |                                                                                                                                                  |
| `tasks`            | 26         | `httpapi/routes.go:193-225`                 | `udsapi/routes.go:219-244`                 | `agh task`                                     | yes        | Includes triage `read/archive/dismiss`, dependencies `add/remove`, `enqueue`, `runs list`, `tree`, `timeline`, `stream`.                          |
| `task-runs`        | 7          | `httpapi/routes.go:217-225`                 | `udsapi/routes.go:246-257`                 | `agh task run …`                                | yes        | Covered by `agh task run claim/start/attach-session/complete/fail/cancel/get`.                                                                   |
| `tools`            | 10         | `httpapi/routes.go:150-166`                 | `udsapi/routes.go:170-191`                 | `agh tool`, `agh toolsets`                      | yes        | `POST /api/tools/:id/invoke` requires `privilegedMutationGuard` on HTTP.                                                                          |
| `vault`            | 4          | `httpapi/routes.go:336-343`                 | `udsapi/routes.go:378-386`                 | `agh vault`                                    | yes        | HTTP requires `privilegedMutationGuard`; all four verbs are loopback-only on HTTP and unrestricted on UDS.                                       |
| `webhooks`         | 2          | `httpapi/routes.go:345-349`                 | (no UDS)                                   | (no CLI)                                       | http-only  | **EXPECTED gap**: webhook delivery is inherently external (signed timestamp + signature); no UDS or CLI verb. Spec marks `Transports: [TransportHTTP]` (`spec.go:896-918`). |
| `workspaces`       | 6          | `httpapi/routes.go:56-64`                   | `udsapi/routes.go:52-62`                   | `agh workspace`                                | yes        |                                                                                                                                                  |
| `agent-kernel-tasks` (UDS-only) | 5 | (none)                                  | `udsapi/routes.go:124-131`                 | `agh task next/heartbeat/complete/fail/release` | uds+cli   | **EXPECTED gap**: peer-claim flow uses `X-AGH-Session-ID`; HTTP refuses the agent identity check (no peer-cred). CLI passes the header transparently via `agentidentity.HeaderSessionID` (`internal/agentidentity/identity.go:25-29`). Validate as scenario API-12. |
| `hosted-mcp` (UDS-only) | 1+ | (none)                                  | `udsapi/routes.go:29` (`registerHostedMCPRoutes`) | (no CLI)                                | uds-only   | Hosted MCP TCP bridge for sidecar; not exposed on HTTP, not exposed via CLI. Document as expected.                                              |

**Total operations across the registry**: 202 (asserted at `internal/api/spec/spec_test.go:1219-1232`).

**Real parity invariants to prove in CI**:

- For every `OperationSpec` that lists `TransportHTTP`, the HTTP router MUST register a Gin handler at the exact `Method` + `Path` (`/api/...`).
- For every `OperationSpec` that lists `TransportUDS`, the UDS router MUST register a handler at the exact same path.
- For every `OperationSpec` whose `OperationID` ends in a state transition verb (`create*`, `update*`, `delete*`, `enable*`, `disable*`, `start*`, `stop*`, `cancel*`, `claim*`, `complete*`, `fail*`, `release*`, `approve*`, `reject*`, `wake*`, `restart*`, `triggerSettingsRestart`), there MUST be a corresponding `cobra.Command` reachable from `NewRootCommand`. State-transition exceptions are: `webhooks/*` (external by design), `:id/test-delivery` (operator-only mutation; CLI is `agh bridge test-delivery`), and the loopback-only HTTP guards.

### 1.b SSE / streaming endpoints (BaseHandlers shared)

| Operation                                         | HTTP                                                 | UDS                                                  | CLI client                                                | Replay key                                  | Source                                                     |
| ------------------------------------------------- | ---------------------------------------------------- | ---------------------------------------------------- | --------------------------------------------------------- | ------------------------------------------- | ---------------------------------------------------------- |
| Session prompt (POST returns SSE)                 | `httpapi/routes.go:80`                               | `udsapi/routes.go:79`                                | `agh session prompt <id> <msg> -o jsonl`                  | turn ids; finish frame                      | `httpapi/prompt.go:90-156`, `udsapi/prompt.go:22-74`       |
| Session prompt cancel                             | `httpapi/routes.go:81`                               | `udsapi/routes.go:80`                                | `agh session prompt --cancel <id>` (or via SDK)           | n/a                                         | `httpapi/sessions.go:43-50`                                |
| Session events stream (SSE replay)                | `httpapi/routes.go:85`                               | `udsapi/routes.go:84`                                | `agh session events --follow <id>`                        | `Last-Event-ID` header → `after_sequence`   | `core/handlers.go:508-545`, `core/session_stream.go:69-100` |
| Observe events stream (SSE)                       | `httpapi/routes.go:117`                              | `udsapi/routes.go:139`                               | `agh observe events --follow`                             | `Last-Event-ID` w/ ObserveCursor            | `core/handlers.go:813`, `core/parsers.go:149-163`           |
| Bridge health stream                              | `httpapi/routes.go:43`                               | `udsapi/routes.go:38`                                | `agh bridge health` (TBD)                                 | n/a                                         | BaseHandlers `StreamBridgeHealth`                          |
| Task stream                                       | `httpapi/routes.go:207`                              | `udsapi/routes.go:234`                               | `agh task watch` (TBD)                                    | `after_sequence` query                      | `BaseHandlers.StreamTask`                                  |
| Settings observability log tail                   | `httpapi/routes.go:308`                              | `udsapi/routes.go:350`                               | (`agh observe events --tail` indirectly; explicit verb TBD) | n/a                                       | `streamSettingsObservabilityLogTail`                       |

The SSE decode helper used by every CLI client is at `internal/sse/decode.go:33-95` (`Decode(ctx, body, handler)` with `Event{ID, Event, Data}`; max line 1 MiB; `ErrStop` to short-circuit).

`Last-Event-ID` reading lives in `internal/api/core/handlers.go:521` (`parseLastEventID(c.GetHeader("Last-Event-ID"), h.transportName())`) and `internal/api/core/parsers.go:149-163` (`ParseObserveCursor`). Both are tested in `internal/api/core/error_paths_test.go:188-208` with bad inputs.

### 1.c CLI command tree (from `internal/cli/root.go:65-123`)

```
agh
├── version
├── install / update / uninstall
├── config / config show / list / get / set / path / edit
├── daemon (start | stop | status | relaunch)
├── network (status | peers | channels | send | inbox)
├── me (root) / me context
├── spawn
├── ch (list | recv | send | reply)
├── session (new | list | stop | status | inspect | resume | repair | wait | prompt | events | history)
├── bridge (list | get | create | update | enable | disable | restart | routes | test-delivery)
├── bundle (catalog | preview | activate | list | get | update | deactivate | network-settings)
├── workspace (add | list | info | edit | remove)
├── agent (list | info)
├── extension (search | list | install | remove | update | enable | disable | status)
├── hooks (list | info | events | runs)
├── automation (jobs | triggers | runs)
├── task (list | create | get | update | cancel | next | heartbeat | complete | fail | release | child | dependency | run)
├── skill (list | view | info | create | search | install | remove | update)
├── memory (health | history | list | read | search | write | delete | reindex | consolidate)
├── vault (list | get | put | delete)
├── tool (list | search | info | invoke)
├── toolsets (list | info)
├── mcp (auth login / status / logout)
├── observe (events | health)
├── whoami
└── doc
```

Cobra's flat dispatch + persistent flag `--output / -o {human|json|jsonl|toon}` (`internal/cli/root.go:89-90`) and legacy `--json` (`:91`) wire every subcommand. The `OutputFormat` constants are at `internal/cli/format.go:20-27`.

## 2. Existing coverage — do NOT duplicate

Tests already in tree that this child must NOT replicate:

- `internal/api/spec/spec_test.go` — duplicate-operation-id detector (`:1219-1232`); response-status invariants; tag inventory.
- `internal/api/spec/resources_test.go`, `vault_test.go`, `settings_test.go`, `authored_context_test.go` — group-by-group transport coverage assertion (`assertOperationTransports(t, op, TransportHTTP, TransportUDS)`).
- `internal/api/httpapi/handlers_test.go` — full route-registration coverage including `POST /api/sessions/:id/prompt/cancel` (`:193`).
- `internal/api/udsapi/handlers_test.go` — UDS route coverage; SSE replay via `Last-Event-ID` (`:1517`); workspace context on synthetic stop event (`:1568`).
- `internal/api/httpapi/transport_parity_integration_test.go` and `internal/api/udsapi/transport_parity_integration_test.go` — already prove HTTP↔UDS payload parity for read endpoints under one test binary.
- `internal/api/core/handlers_test.go`, `parsers_test.go`, `more_coverage_test.go`, `error_paths_test.go` — BaseHandlers shared logic; `parseLastEventID` invalid input (`error_paths_test.go:188`); `ParseObserveCursor` round-trip; `EnqueueTaskRun` dispatcher; coverage helpers.
- `internal/codegen/openapits/generate_test.go`, `internal/codegen/sdkts/generate_test.go`, `cmd/agh-codegen/main_test.go` — codegen drift unit tests.
- `internal/sse/decode_test.go` — SSE framing parser including malformed inputs and 1 MiB ceiling.
- `internal/cli/cli_integration_test.go`, `cli_historical_*_integration_test.go`, `tool_integration_test.go`, `extension_marketplace_integration_test.go`, `skill_marketplace_integration_test.go` — CLI end-to-end integration with stub UDS server.

The gap real-LLM and behavior scenarios MUST close: every handler-side test stubs the daemon. **None spawn a real Claude Code subprocess and prove the SSE event sequence is identical when the same prompt is delivered through HTTP and through UDS to a real agent**. None physically violate the codegen drift gate. None run a real worktree-isolated lab through the CLI.

## 3. Gaps the real-scenario lane must close

1. **Real prompt parity HTTP↔UDS↔CLI**: identical SSE envelope sequence and identical persisted `events.db` rows (API-02, API-03, API-04).
2. **SSE reconnect after-seq durability**: a 30 s disconnect must not duplicate any event with sequence ≤ N (API-05).
3. **SSE backpressure**: slow consumer must not OOM the daemon and must not lose events that were durably appended (API-06).
4. **Codegen drift**: hand-edit `openapi/agh.json`, run `make codegen-check`, fail with explicit diff. Same for hand-edit of a `OperationSpec.Description` or a contract type field (API-07, API-08).
5. **Web TS compile after codegen**: `agh-openapi.d.ts` regeneration leaves the web SPA TypeScript build green (API-09).
6. **BaseHandlers single source of truth**: induce the same not-found / validation error class via HTTP and UDS, prove identical `ErrorPayload` shape and identical mapping (API-10).
7. **HTTP origin/loopback guard vs UDS peer access**: HTTP `privilegedMutationGuard` denies non-loopback; UDS only requires the socket file mode `0o600`; CLI defaults to UDS (API-11).
8. **CLI `-o json` envelope is stable**: golden snapshot test for `agh status -o json`, `agh session list -o json`, `agh task list -o json`. CLI `-o jsonl` emits one valid object per line for streaming endpoints (API-13, API-14).
9. **`mage Boundaries` is real**: introduce a fixture file that imports `internal/api/httpapi` from `internal/session` (forbidden) and prove `mage Boundaries` exits non-zero with the expected violation message (API-15).
10. **Agent-manageable parity via CLI**: every state-transition operation present in `internal/api/spec` must have a CLI verb reachable from `NewRootCommand`. Derive the matrix at runtime; assert the diff is empty modulo the documented exceptions (API-16).
11. **SSE event coverage**: every required correlation key (`session_id`, `parent_session_id`, `root_session_id`, `agent_name`, `task_id`, `run_id`, `claim_token_hash`, `lease_until`, `workflow_id`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `spawn_depth`, `actor_kind`, `actor_id`, `release_reason`) appears at least once across the SSE captures from API-02..API-06 (API-17).
12. **Compose flow (CLI + HTTP SSE replay + CLI tail)**: switch transports mid-conversation; final reconstructed transcript is identical (API-18).

## 4. Operating model — provider matrix and bootstrap

Same template as the sibling children (`03-acp-sessions`, `04-autonomy-kernel`):

- **`real-claude-code`** (default): real Claude Code ACP subprocess for any scenario that drives a prompt. Every prompt scenario asserts ledger and SSE. Driver: `internal/config/provider.go:165-173` (`npx -y @agentclientprotocol/claude-agent-acp@latest`).
- **`mock-acp`** (rare, only for non-LLM control-plane scenarios): the `internal/e2elane` deterministic driver. Used here for API-15 (boundaries fixture test) and API-07/API-08 (codegen drift) since neither involves an LLM.
- **`live-frontier`** is **not** required for this child since the parity surfaces are network-protocol invariants; the prompt scenarios cover the real-LLM lane through the existing `real-claude-code` provider.

Bootstrap and isolation discipline (mandatory):

- One isolated `AGH_HOME`, daemon HTTP port, UDS socket path, `tmux-bridge` socket, and `PROVIDER_HOME`/`PROVIDER_CODEX_HOME` per scenario (per `agh-worktree-isolation` skill and `agh-qa-bootstrap`).
- `AGH_WEB_API_PROXY_TARGET` exported when web QA accompanies a parity scenario (per CLAUDE.md "Isolated Web QA must export `AGH_WEB_API_PROXY_TARGET`").
- Bound-secret, brokered, and explicitly isolated-home auth staged into
  `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`; `native_cli` providers with
  `home_policy=operator` intentionally use the operator `HOME` / native login
  state unless the scenario explicitly validates isolated provider-home
  behavior.
- Sequential config writes only (no parallel `agh config set` against the same provider — per Workflow Rules).

## 5. Preconditions (apply to every scenario)

- Fresh QA bootstrap; `bootstrap-manifest.json` saved and `bootstrap.env` exported.
- `make verify` is green on the SUT branch (per the Critical Rules).
- Daemon running: `agh status -o json` reports `status="running"`.
- For HTTP scenarios, the bound HTTP host is reachable on `127.0.0.1:<port>` from the same shell.
- For UDS scenarios, the UDS path resolved via `cli/root.go:267-286` (`Daemon.Socket` → `HomePaths.DaemonSocket`) is mode `0o600`.
- For real-LLM scenarios, the direct `claude` provider is authenticated in the
  effective Claude home for the lane: operator `HOME` by default, or isolated
  `PROVIDER_HOME` only when the scenario explicitly validates isolated native
  auth. `agh provider show claude` reports the expected ACP command.

Per-scenario evidence layout under `.artifacts/qa/<run-id>/api-XX/`:

- `api-XX-report.md` (Worked / Failed / Blocked / Follow-up)
- `api-XX-summary.json` (machine-readable)
- `api-XX-events.json` (events.db rows scoped to the scenario window)
- `api-XX-output.log` (combined stdout/stderr)
- Per-scenario raw SSE logs and parity diffs as named below.

## 6. Cleanup (applies to every scenario)

- `agh daemon stop` (or kill PID from manifest).
- Inspect for leftover SSE goroutines; `goleak.VerifyNone(t)` on the in-process integration runner.
- Archive any SSE logs and parity diffs alongside the report bundle.
- Tear down the worktree only after evidence artifacts are written.

## 7. Mandatory scenarios

### API-01 — Parity matrix coverage from the OpenAPI spec

```yaml qa-scenario
id: api-01-parity-matrix
title: Every Operation in the registry has the routes its Transports field declares; CLI verb exists for every state-transition operation
theme: api.parity
coverage:
  primary:
    - api.parity.http
    - api.parity.uds
    - api.parity.cli
  secondary:
    - codegen.coverage
risk: high
live: false
provider: mock-acp
preconditions:
  - SUT branch checked out; daemon not necessarily running
  - Go available; `mage` or `make` available
docs_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/spec/spec.go:144-208
  - /Users/pedronauck/Dev/compozy/agh/internal/api/spec/spec_test.go:1219-1232
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/routes.go:6-36
  - /Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/routes.go:6-30
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/root.go:65-123
steps:
  - Iterate `internal/api/spec.Operations()` programmatically (e.g. write a one-shot Go binary under `internal/api/spec/parity_check_main.go` for the duration of the run, then delete).
  - For each `OperationSpec`, check (a) HTTP router has a handler at `Method` + `Path` whenever `TransportHTTP ∈ Transports`; (b) UDS router has a handler at `Method` + `Path` whenever `TransportUDS ∈ Transports`. Use `http.NewServeMux` introspection on the in-process Gin engines from `internal/api/httpapi/server.go` and `internal/api/udsapi/server.go`.
  - Build the CLI tree from `cli.NewRootCommand()` and walk every leaf `*cobra.Command`. For every state-transition `OperationID` (regex `^(create|update|delete|enable|disable|start|stop|cancel|claim|complete|fail|release|approve|reject|wake|restart|trigger)`), assert at least one CLI verb covers it. Use the documented exceptions list (webhooks, hosted-mcp, agent-kernel-tasks UDS-only).
expected:
  - Every operation listed in `Operations()` has its declared `Transports` honored on HTTP and UDS.
  - For state-transition operations: every one is reachable from a CLI verb. Document the exceptions inline in the report.
  - Output the rendered parity matrix as `api-01-parity.json` (operation_id, http_route_present, uds_route_present, cli_verb_present, transports_declared).
evidence:
  - `api-01-parity.json` (machine-readable matrix)
  - `api-01-parity.md` (human-readable matrix grouped by tag)
  - Diff line count == 0 for the matrix-vs-declared comparison.
failure_signatures:
  - HTTP route absent for an operation that lists `TransportHTTP`: missing `httpapi.Register*Routes` entry.
  - UDS route absent: missing `udsapi.Register*Routes` entry.
  - State-transition `OperationID` without a CLI verb that is not on the documented exceptions list: violates "Agent-manageable by default" (`internal/CLAUDE.md:26`).
cleanup:
  - Delete the one-shot parity-check binary if it was generated.
```

### API-02 — Real Claude Code prompt via HTTP and via CLI: identical SSE envelope sequence

```yaml qa-scenario
id: api-02-prompt-http-vs-cli
title: Real Claude Code prompt produces identical SSE event sequence when delivered through HTTP and through CLI (UDS)
theme: api.parity.prompt
coverage:
  primary:
    - api.parity.http
    - api.parity.cli
    - sse.typed_envelope
  secondary:
    - transcript.canonical
    - event.lineage-correlation
risk: high
live: true
provider: real-claude-code
preconditions:
  - ACP-01 preconditions from `03-acp-sessions.md` (Claude Code subprocess
    reachable; direct `claude` auth resolved from the effective Claude home
    for the lane)
  - Daemon HTTP and UDS both bound; CLI default UDS socket points at the daemon
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:90-156
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:251-580
  - /Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/prompt.go:22-74
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/session.go:287-315
steps:
  - Create session S1 via CLI: `agh session new --agent claude -o json` → S1.
  - Send the same prompt P over HTTP: `curl -N -X POST http://127.0.0.1:$PORT/api/sessions/$S1/prompt -H 'Content-Type: application/json' -d '{"message":"Read README.md and tell me the title in one sentence"}'` → capture all SSE frames into `api-02-http.sse`.
  - Wait for `[DONE]`.
  - Create a second session S2 with the exact same agent/cwd/workspace.
  - Send the identical prompt P over CLI: `agh session prompt $S2 "Read README.md and tell me the title in one sentence" -o jsonl` → capture stdout into `api-02-cli.jsonl`.
  - Normalize both captures by stripping volatile fields (`id` per AI-SDK uuids, timestamps, `tool_call_id` strings, monotonic sequence numbers) and ordering by event type sequence.
  - Diff the normalized event-type sequences.
expected:
  - Both captures contain the same ordered event-type sequence: `start` → `text-start` → ≥1 `text-delta` → `text-end` → optional `tool-input-start`+`tool-input-available`+`tool-output-available` blocks for any tools used → `data-agh-event` for ACP lifecycle events → `finish` (with `finishReason: "stop"`) → `[DONE]`.
  - The diff of the normalized event-type sequence is empty.
  - Both `events.db` (S1 and S2) contain `agent_message` rows with token text concatenated to the same final string (modulo provider nondeterminism — assert byte length within ±10%).
  - Both responses emit `Content-Type: text/event-stream; charset=utf-8` (HTTP) and the UDS prompt streams the same shape. CLI tail is JSONL by `--output jsonl`.
evidence:
  - `api-02-http.sse`, `api-02-cli.jsonl`, `api-02-event-sequence-diff.txt`
  - `events_S1.json`, `events_S2.json` from `agh session events <id> -o jsonl`
failure_signatures:
  - Different ordered event-type sequence between transports → BaseHandlers/HTTP/UDS divergence.
  - HTTP missing `Last-Event-ID` exposure header (CORS) — `httpapi/middleware.go:46` is the only place this is set.
cleanup:
  - `agh session stop $S1; agh session stop $S2`.
```

### API-03 — Real Claude Code prompt via UDS direct: identical to HTTP and CLI

```yaml qa-scenario
id: api-03-prompt-uds-direct
title: Posting directly to the UDS socket with curl --unix-socket produces the same SSE envelope as HTTP and CLI
theme: api.parity.prompt
coverage:
  primary:
    - api.parity.uds
    - sse.typed_envelope
risk: high
live: true
provider: real-claude-code
preconditions:
  - API-02 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/prompt.go:22-74
  - /Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/server.go:631-684
steps:
  - Create session S3.
  - Send the prompt over UDS using `curl --unix-socket "$AGH_HOME/sock/uds.sock" -N -X POST http://localhost/api/sessions/$S3/prompt -H 'Content-Type: application/json' -d '{"message":"<same prompt>"}'`.
  - Capture into `api-03-uds.sse`.
  - Diff against `api-02-cli.jsonl` (CLI also goes through UDS — they should match byte-for-byte modulo volatile fields).
  - Diff against `api-02-http.sse` (HTTP path) for ordered envelope sequence.
expected:
  - UDS direct response payload is byte-identical to CLI capture after the same volatile-field stripping.
  - UDS payload event-type sequence equals HTTP event-type sequence.
  - UDS HTTP server enforces `0o600` on the socket file (assert `stat -c '%a' $AGH_HOME/sock/uds.sock` == `600`).
evidence:
  - `api-03-uds.sse`, `api-03-uds-vs-cli-diff.txt`, `api-03-uds-vs-http-diff.txt`, `api-03-socket-mode.txt`
failure_signatures:
  - UDS socket file mode != 0o600 → security regression in `udsapi/server.go:636`.
  - UDS payload differs from CLI: BaseHandlers code path is forking transport-side parsing (violates "no transport-duplicated parsing").
cleanup: `agh session stop $S3`.
```

### API-04 — Cross-transport reads return identical contracts for the same session

```yaml qa-scenario
id: api-04-read-parity
title: GET reads (sessions, tasks, hooks, observe, settings) return byte-equal JSON for HTTP and UDS
theme: api.parity.read
coverage:
  primary:
    - api.parity.http
    - api.parity.uds
    - basehandlers.shared
risk: medium
live: true
provider: real-claude-code
preconditions:
  - One session S4 created with one completed prompt for content variety
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go (BaseHandlers methods)
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/transport_parity_integration_test.go (existing partial coverage)
steps:
  - For each of `GET /api/sessions`, `GET /api/sessions/$S4`, `GET /api/sessions/$S4/transcript`, `GET /api/sessions/$S4/events`, `GET /api/sessions/$S4/history`, `GET /api/tasks`, `GET /api/hooks/catalog`, `GET /api/hooks/runs`, `GET /api/hooks/events`, `GET /api/observe/health`, `GET /api/observe/events`, `GET /api/settings/general`, `GET /api/settings/memory`, `GET /api/settings/skills`, `GET /api/settings/network`, `GET /api/agents`, `GET /api/skills`, `GET /api/memory`, `GET /api/network/status`, `GET /api/bundles/catalog`:
    - Fetch over HTTP into `api-04-http-<op>.json`.
    - Fetch over UDS (curl --unix-socket OR `agh <verb> -o json`) into `api-04-uds-<op>.json`.
    - Compute `jq -S .` canonicalized diff.
expected:
  - For every operation in the list, canonicalized JSON is byte-identical between HTTP and UDS — modulo:
    - Server-set timestamps (mask `generated_at`, `last_seen_at`, `updated_at` fields before diff).
    - Trace IDs in 5xx (none expected at 200).
  - HTTP responses include `Content-Type: application/json` charset; UDS the same.
  - HTTP response includes loopback origin permissive headers; UDS does not (verify CORS-related headers absent on UDS).
evidence:
  - `api-04-parity-table.json` listing per-op diff size; all diffs zero.
  - One representative pair of canonical JSONs per group.
failure_signatures:
  - Per-op diff != 0 → BaseHandlers payload assembly drift; this violates "BaseHandlers is the canonical handler home" (`internal/CLAUDE.md:23`).
cleanup: `agh session stop $S4`.
```

### API-05 — SSE reconnect with `after_seq=N` after 30 s disconnect

```yaml qa-scenario
id: api-05-sse-reconnect-after-seq
title: After a 30-second SSE disconnect, reconnecting with Last-Event-ID delivers only events with sequence > N and never duplicates
theme: sse.replay
coverage:
  primary:
    - sse.last_event_id
    - sse.replay.no_duplicate
  secondary:
    - persistence.events_db
risk: high
live: true
provider: real-claude-code
preconditions:
  - Session S5 with a long-running prompt (≥45 s of token output)
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go:508-545
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-100
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/parsers.go:28-50
  - /Users/pedronauck/Dev/compozy/agh/internal/sse/decode.go:33-95
steps:
  - Connect to `GET /api/sessions/$S5/stream` over HTTP. Record every received `id:` field. Capture into `api-05-phase1.sse`.
  - At wall-clock T+10s, kill the TCP socket but do NOT cancel the prompt.
  - Wait 30 s (per spec). Note `last_event_id = N`.
  - Reconnect with `Last-Event-ID: $N` header. Capture into `api-05-phase2.sse`.
  - Wait until prompt finishes; capture into `api-05-phase3.sse` if any remaining frames after the second reconnect window.
  - Cross-reference the persisted event sequence: `agh session events $S5 -o jsonl --since N > api-05-events-after-N.jsonl`.
expected:
  - Phase 2 emits only events with sequence > N. Assert via `awk -F'"sequence":' '{print $2}' | sort -n | head -1` ≥ N+1.
  - No event ID appears in both phase1 and phase2 (Python `set()` intersection of `id:` fields equals ∅).
  - The persisted events list (`api-05-events-after-N.jsonl`) is a superset of the phase2 SSE frames.
  - The reconstructed transcript (`agh session transcript $S5 -o json`) is byte-identical to the transcript reconstructed from continuous streaming (run the same prompt on a sibling session S5b with no disconnect, both with `--cwd` matched workspace, then compare normalized transcripts).
evidence:
  - `api-05-phase1.sse`, `api-05-phase2.sse`, `api-05-phase3.sse`
  - `api-05-events-after-N.jsonl`
  - `api-05-id-intersection.txt` (must report empty intersection)
  - `api-05-transcript-diff.txt`
failure_signatures:
  - Phase 2 contains an event with sequence ≤ N: replay regression in `pollAndStreamSessionEvents` (`session_stream.go:69-100`).
  - Phase 2 misses an event with sequence > N: poll loop not re-entering after reconnect.
  - SSE `id:` field is empty (server not setting `id`): replay-key broken at the BaseHandlers SSE writer.
cleanup: `agh session stop $S5`.
```

### API-06 — SSE backpressure: slow consumer; durable append before broadcast; no OOM

```yaml qa-scenario
id: api-06-sse-backpressure
title: Slow SSE consumer cannot OOM the daemon and never loses an event that was durably appended
theme: sse.backpressure
coverage:
  primary:
    - sse.backpressure
    - persistence.events_db
risk: high
live: true
provider: real-claude-code
preconditions:
  - Session S6 with a prompt that produces ≥2 MB of token output (e.g. "Print every line of generated_long_file.txt verbatim", file pre-seeded ~2 MB)
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md "Live broadcasters publish only after durable append; reconnect/replay uses after_seq"
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-100
steps:
  - Connect to `GET /api/sessions/$S6/stream` and read at 64 KiB/s with `dd bs=1024 count=64` style throttling (or `pv -L 64k`).
  - Sample daemon RSS once per second via `ps -o rss= -p $DAEMON_PID` for the duration; write to `api-06-rss.csv`.
  - After the stream completes, fetch `agh session events $S6 -o jsonl > api-06-events-final.jsonl` and `agh session transcript $S6 -o json > api-06-transcript.json`.
expected:
  - Daemon RSS does NOT exceed 4× steady-state baseline; no monotonic growth past stream completion.
  - `api-06-events-final.jsonl` is non-empty and contains the entire run; sequence is monotonic with no gaps.
  - The slow-consumed SSE log contains every `id:` from `api-06-events-final.jsonl` (no events lost — durable append happens **before** broadcast).
  - Daemon log records no `runtime_warning` for stalled writer aborts.
evidence:
  - `api-06-rss.csv`, `api-06-events-final.jsonl`, `api-06-transcript.json`, `api-06-slow-sse.log`
failure_signatures:
  - RSS grows unboundedly: in-memory broadcast buffer (violates "Live broadcasters publish only after durable append").
  - Slow-consumed SSE missing event ids that exist in `api-06-events-final.jsonl`: broadcaster shed events before durable append (the inverse — daemon dropped a durable event in flight).
cleanup: `agh session stop $S6`.
```

### API-07 — Codegen drift fail: hand-edit `openapi/agh.json`

```yaml qa-scenario
id: api-07-codegen-drift-openapi
title: Hand-editing openapi/agh.json then running `make codegen-check` fails with a clear stale-generated-file diff
theme: codegen.drift
coverage:
  primary:
    - codegen.drift.openapi
risk: high
live: false
provider: mock-acp
preconditions:
  - Worktree clean; `make codegen-check` is currently green (`make verify` passes).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/cmd/agh-codegen/main.go:85-160
  - /Users/pedronauck/Dev/compozy/agh/magefile.go:148-178
steps:
  - Save a backup of `openapi/agh.json` to `openapi/agh.json.bak`.
  - Hand-edit `openapi/agh.json`: change one operation's `summary` text or remove a field from a schema. Save.
  - Run `make codegen-check`. Capture stdout and stderr.
  - Restore `openapi/agh.json` from backup.
  - Run `make codegen-check` again to prove green.
expected:
  - Edited run exits non-zero with an error chain that includes `cmd/agh-codegen` `ErrStaleGeneratedFile` AND the path `openapi/agh.json` AND the suggestion `run codegen` (per `main.go:134, 156`).
  - Restored run exits 0.
evidence:
  - `api-07-edited-output.log`, `api-07-restored-output.log`, `api-07-edit.diff` (the hand edit applied)
failure_signatures:
  - Edited run exits 0: codegen-check is regenerating instead of comparing (regression of `checkJSONFile`).
  - Edited run fails for a different reason (e.g. JSON parse): `canonicalJSON` (`main.go:162-168`) is not normalizing; OK to flag but the test still passes — record in follow-up.
cleanup: ensure `openapi/agh.json` matches HEAD (use `git diff openapi/agh.json` → empty).
```

### API-08 — Codegen drift fail: edit a contract field, run codegen-check BEFORE running codegen

```yaml qa-scenario
id: api-08-codegen-drift-contract
title: Editing internal/api/contract/* without rerunning codegen produces a clear stale-generated-file failure
theme: codegen.drift
coverage:
  primary:
    - codegen.drift.contract
risk: high
live: false
provider: mock-acp
preconditions:
  - `make codegen-check` is currently green
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/contract/responses.go
  - /Users/pedronauck/Dev/compozy/agh/cmd/agh-codegen/main.go:101-160
  - /Users/pedronauck/Dev/compozy/agh/internal/codegen/openapits/generate.go (Check function)
steps:
  - Save backups of `internal/api/contract/responses.go` and `openapi/agh.json` and `web/src/generated/agh-openapi.d.ts`.
  - Edit one field in `internal/api/contract/responses.go` (e.g. add a new field `Foo string \`json:"foo"\`` to `DaemonStatusPayload`). Save.
  - Run `make codegen-check`. Expect non-zero exit.
  - Capture output; assert the failure points to the OpenAPI artifact OR the SDK contracts artifact (both are downstream of contract types).
  - Now run `make codegen` and assert exit 0.
  - Run `make codegen-check` and assert exit 0.
  - Restore the original contract file and run `make codegen` to revert generated artifacts; assert `git diff` is empty.
expected:
  - Step 3 fails with `ErrStaleGeneratedFile` referencing either `openapi/agh.json` or `web/src/generated/agh-openapi.d.ts` or `sdk/typescript/src/generated/contracts.ts`.
  - Step 5 succeeds.
  - Step 6 reverts cleanly.
evidence:
  - `api-08-edited-codegen-check.log`, `api-08-edited-codegen.log`, `api-08-final-codegen-check.log`, `api-08-final-git-diff.txt`
failure_signatures:
  - Step 3 succeeds: codegen-check is not rerunning the schema reflection from contract types.
  - Step 5 fails on the second run: codegen is not idempotent; bug in `cmd/agh-codegen/main.go` `writeOpenAPI` or `writeSDKContracts`.
cleanup: assert `git diff internal/api/contract/ openapi/ web/src/generated/ sdk/typescript/` is empty before report write.
```

### API-09 — `make codegen` produces lockstep TS + JSON; web TypeScript build still green

```yaml qa-scenario
id: api-09-codegen-lockstep-and-tsbuild
title: After `make codegen`, openapi/agh.json and web/src/generated/agh-openapi.d.ts are in lockstep and `make web-build` (or `make bun-typecheck`) succeeds
theme: codegen.lockstep
coverage:
  primary:
    - codegen.lockstep
    - web.tsbuild
risk: medium
live: false
provider: mock-acp
preconditions:
  - Worktree clean
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/codegen/openapits/generate.go:24-43
  - /Users/pedronauck/Dev/compozy/agh/Makefile:38-42 (`make codegen`, `make codegen-check`)
  - /Users/pedronauck/Dev/compozy/agh/Makefile (`make web-build`, `make bun-typecheck`)
steps:
  - Add a new contract field to `internal/api/contract/responses.go` (e.g. `DaemonStatusPayload.SchemaTestField string`).
  - Run `make codegen`. Capture exit + diff.
  - Run `make bun-typecheck` (or scope to `make web-typecheck` if faster).
  - Run `make web-build`.
  - Revert the test contract change.
expected:
  - `make codegen` regenerates both `openapi/agh.json` and `web/src/generated/agh-openapi.d.ts`. Both files appear in the resulting diff.
  - `make bun-typecheck` exits 0 because the new field is reflected in the generated `.d.ts` and the web SPA does not reference it (or compiles cleanly if it is referenced).
  - `make web-build` exits 0.
evidence:
  - `api-09-codegen-diff.txt` (file list + summary), `api-09-bun-typecheck.log`, `api-09-web-build.log`
failure_signatures:
  - Only one of the two artifacts updated: `openapits.Generate` regression or wrapped artifact list missing the new web file.
  - `make web-build` fails: codegen produced TS that references types not exported (regression in oxfmt formatting or in `openapi-typescript`).
cleanup: assert `git diff internal/api/contract/ openapi/ web/src/generated/` is empty.
```

### API-10 — BaseHandlers shared error mapping: HTTP and UDS map identical errors

```yaml qa-scenario
id: api-10-error-mapping
title: A 404 / 400 / 422 / 503 induced via HTTP and via UDS produces identical ErrorPayload bodies and identical status codes
theme: api.error_mapping
coverage:
  primary:
    - basehandlers.shared
    - api.error_mapping
risk: medium
live: false
provider: mock-acp
preconditions:
  - Daemon running; one session S10 not present in DB ("not found"); one tool that requires permission not granted ("forbidden")
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/errors.go
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/middleware.go (errorMiddleware)
  - /Users/pedronauck/Dev/compozy/agh/internal/api/contract (ErrorPayload)
steps:
  - For each of: (a) `GET /api/sessions/sess-not-found` (expect 404), (b) `POST /api/sessions/sess-not-found/prompt` body `{}` (expect 400), (c) `POST /api/tasks` malformed body `{}` (expect 400 or 422), (d) `POST /api/sessions/sess-not-found/clear` (expect 404):
    - Hit HTTP and capture status+body into `api-10-http-<case>.json`.
    - Hit UDS via curl --unix-socket and capture into `api-10-uds-<case>.json`.
  - Compare canonicalized bodies (`jq -S .`) and status codes.
expected:
  - Status code identical between HTTP and UDS for every case.
  - `ErrorPayload` body identical (same `error` field text; same code if present). Note: HTTP may add CORS headers; that's fine — only the body and status are compared.
  - The handler logic comes from `internal/api/core` `BaseHandlers` (assert by code-search that there is no transport-side custom error formatter).
  - CLI exit code matches `cliExitCodeForError` mapping (e.g. 64 for identity_required, 65 for identity_invalid, 77 for unauthorized — `internal/agentidentity/identity.go:34-45`).
evidence:
  - Per-case `api-10-{http,uds}-<case>.json`, `api-10-error-diff.txt`
failure_signatures:
  - Body or status differs between transports: BaseHandlers error mapping bypassed (transport-side override).
cleanup: nothing to clean up — error paths only.
```

### API-11 — Auth boundary: HTTP origin / loopback guard vs UDS socket-mode-only

```yaml qa-scenario
id: api-11-auth-boundary
title: HTTP refuses non-loopback origins and refuses privileged mutations from non-loopback hosts; UDS only requires socket file mode 0o600
theme: api.auth_boundary
coverage:
  primary:
    - http.cors
    - http.privileged_mutation_guard
    - uds.socket_mode
risk: high
live: false
provider: mock-acp
preconditions:
  - Daemon listening on a loopback host (default config)
  - A spare bind alias or simulated remote host (use `0.0.0.0` bind for the test daemon, then connect from `127.0.0.2` to simulate a non-loopback path; or use an HTTP proxy with `Origin: https://attacker.example`)
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/middleware.go (corsMiddleware, loopbackMutationGuard, errLoopbackMutationRequired)
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/handlers.go:141 (resourceAuthMiddleware)
  - /Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/server.go:632-640 (socket chmod 0o600)
steps:
  - Connect HTTP with `Origin: https://attacker.example` to `GET /api/sessions`: assert response 403 with body `{"error":"origin not allowed"}` (per `middleware.go:51-53`).
  - Connect HTTP with `Origin: http://127.0.0.1:$PORT`: assert response 200 (loopback case).
  - Bind daemon to a non-loopback host (test config) and try `POST /api/extensions` (privileged mutation): assert 403 with body referencing `errLoopbackMutationRequired` ("remote HTTP settings and extension mutations are disabled in v1 unless the daemon is bound to a loopback host", `middleware.go:18-19`). Restore loopback bind after.
  - For UDS: assert `stat -c '%a' $AGH_HOME/sock/uds.sock` == `600` (per `udsapi/server.go:636`).
  - Try to connect to the UDS socket from a different uid (if the test runner has root, drop to a sibling user; else skip with `t.Skip("requires multi-user setup")` and document).
expected:
  - HTTP origin denial returns 403 with the documented body.
  - HTTP loopback-only mutation guard returns 403 with the documented error message.
  - UDS file mode is exactly `0o600`.
  - CLI commands work without setting any auth header (because UDS uses the socket-mode boundary).
evidence:
  - `api-11-http-origin-denied.json`, `api-11-http-loopback-allowed.json`, `api-11-http-non-loopback-mutation-denied.json`, `api-11-uds-socket-mode.txt`
failure_signatures:
  - Origin denial does not return 403: regression in `corsMiddleware`.
  - Privileged HTTP mutation succeeds on a non-loopback bind: `loopbackMutationGuard` regression.
  - UDS socket mode != `0o600`: security regression (`udsapi/server.go:636`).
cleanup: restore daemon bind to loopback; restore worktree default config.
```

### API-12 — Agent-kernel UDS-only routes carry identity headers; HTTP rejects them

```yaml qa-scenario
id: api-12-agent-kernel-headers
title: /api/agent/* routes require X-AGH-Session-ID + X-AGH-Agent and are reachable via UDS+CLI but not via HTTP
theme: api.identity
coverage:
  primary:
    - identity.headers
    - api.parity.uds_only
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Session S12 created; subprocess is alive; `X-AGH-Session-ID` and `X-AGH-Agent` env values can be read from `agh session inspect S12 -o json`
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/agentidentity/identity.go:18-247
  - /Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/routes.go:111-133
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/agent_identity.go
steps:
  - Subtest A — UDS+identity OK: `curl --unix-socket $UDS http://localhost/api/agent/me -H "X-AGH-Session-ID: $S12" -H "X-AGH-Agent: claude"` → 200 with the agent payload.
  - Subtest B — UDS without identity: same call without headers → 401 with `code:"identity_required"`.
  - Subtest C — UDS with stale identity: header `X-AGH-Session-ID: sess-stale-NA` → 401 with `code:"identity_stale"`.
  - Subtest D — HTTP attempt: `curl http://127.0.0.1:$PORT/api/agent/me -H "X-AGH-Session-ID: $S12" -H "X-AGH-Agent: claude"` → 404 (HTTP does not register the agent-kernel routes; per `httpapi/routes.go:89-112`, only the agent root + agents subtree).
  - Subtest E — CLI: `agh me -o json` (CLI sets headers automatically via env vars, see `internal/agentidentity/identity.go:20-29`).
expected:
  - Subtest A: 200, body matches `AgentMePayload`.
  - Subtest B: 401, body has `code:"identity_required"`, exit code 64 if shelled via CLI.
  - Subtest C: 401, body has `code:"identity_stale"`, exit code 65 via CLI.
  - Subtest D: 404 (no HTTP route), confirming agent-kernel is UDS-only.
  - Subtest E: 200 with same payload as A.
evidence:
  - Per-subtest `api-12-<subtest>.json`, `api-12-cli.log`
failure_signatures:
  - HTTP returns 200 to /api/agent/me — agent-kernel surface accidentally HTTP-registered (security/architectural regression).
  - UDS without headers returns 200: identity gate bypass (regression of `requireAgentCaller`).
cleanup: `agh session stop $S12`.
```

### API-13 — CLI `-o json` envelope is stable: golden snapshot test

```yaml qa-scenario
id: api-13-cli-json-stability
title: CLI -o json output for status verbs is byte-stable against a golden snapshot (modulo masked timestamps)
theme: cli.format.json
coverage:
  primary:
    - cli.json_stability
risk: medium
live: false
provider: mock-acp
preconditions:
  - Daemon running with one workspace, one agent, no session
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/format.go:20-27
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/daemon.go:95 (`agh status`)
steps:
  - Run `agh status -o json | jq -S '.daemon | del(.uptime_seconds, .started_at, .last_seen_at)'` → write to `api-13-daemon-status.json`.
  - Compare against committed golden `internal/cli/testdata/golden/daemon_status.json` (create on first run; CI updates on opt-in).
  - Repeat for `agh session list -o json`, `agh task list -o json`, `agh memory list -o json`, `agh skill list -o json`, `agh tool list -o json`.
  - Mask volatile fields (`generated_at`, `*_id` if random per run, `created_at`).
expected:
  - Each output matches the golden modulo the explicit mask list.
  - Schema check: every JSON has the same top-level keys it had in the previous run.
evidence:
  - `api-13-<verb>.json` per command; `api-13-diff-<verb>.txt` per command (must be empty).
failure_signatures:
  - Schema drift: a key was renamed or removed without updating the golden — fix the golden in the same commit (per "Greenfield Alpha — Zero Legacy Tolerance", since AGH allows breaking changes, the golden must move with the code).
  - JSON unparseable: CLI emitting non-JSON to stdout when `-o json` is set (bug in `format.go`).
cleanup: nothing.
```

### API-14 — CLI `-o jsonl` streams typed objects, one valid JSON per line

```yaml qa-scenario
id: api-14-cli-jsonl-stream
title: CLI streaming verbs emit one valid JSON object per line under -o jsonl
theme: cli.format.jsonl
coverage:
  primary:
    - cli.jsonl_stream
risk: medium
live: true
provider: real-claude-code
preconditions:
  - Session S14 with a long-running prompt
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/format.go:24-26
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/session.go (events / prompt subcommand `-o jsonl` writers)
steps:
  - Run `agh session prompt $S14 "Print fibonacci(20) and name it" -o jsonl > api-14-prompt.jsonl`.
  - Validate each line: `while read -r line; do echo "$line" | jq . > /dev/null; done < api-14-prompt.jsonl` — exit 0 means every line is a valid JSON.
  - Run `agh session events $S14 --follow -o jsonl > api-14-events.jsonl` for ~5 s after the prompt, then SIGINT.
  - Repeat the per-line jq validation.
expected:
  - Every line of both files parses with `jq .`.
  - First line of `api-14-prompt.jsonl` is a `{"type":"start", ...}` object.
  - Last non-empty line is `{"type":"finish", ...}` for the prompt JSONL.
evidence:
  - `api-14-prompt.jsonl`, `api-14-events.jsonl`, `api-14-jq-validation.log`
failure_signatures:
  - Any line fails jq parse: CLI is buffering and printing partial JSON or interleaving stderr.
  - `--follow` does not exit cleanly on SIGINT: signal-handling regression in CLI streaming reader.
cleanup: `agh session stop $S14`.
```

### API-15 — `mage Boundaries` rule fires on a planted violation

```yaml qa-scenario
id: api-15-boundaries-rule
title: Adding an import of internal/api/httpapi from internal/session causes `mage Boundaries` (and `make boundaries`) to fail with a clear diagnostic
theme: api.boundaries
coverage:
  primary:
    - boundaries.rule
risk: high
live: false
provider: mock-acp
preconditions:
  - Worktree clean; `make boundaries` is currently green
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/magefile.go:231-300
steps:
  - Create `internal/session/boundaries_qa_violation.go` with the body:
    ```go
    //go:build qa_boundary_violation
    package session
    import _ "github.com/pedronauck/agh/internal/api/httpapi"
    ```
  - Run `make boundaries`. Capture stdout/stderr.
  - Note: build tag prevents `go build` from picking it up; the boundaries rule uses `grep -r --include=*.go` so it WILL flag the file regardless of build tags.
  - Remove the file. Run `make boundaries` again; assert green.
  - Repeat for one more pair: `internal/store/boundaries_qa_violation.go` importing `internal/api/udsapi`.
expected:
  - First `make boundaries` exits non-zero with output starting `VIOLATION: internal/session imports internal/api/httpapi` (per `magefile.go:287`) and lists the planted file path.
  - Final summary: `found N boundary violations` with N ≥ 1.
  - Removal restores green.
evidence:
  - `api-15-violation-output.log`, `api-15-clean-output.log`, `api-15-files-list.txt`
failure_signatures:
  - `make boundaries` exits 0 with the file present: `magefile.go:281` grep mistargeted; rule is unreliable.
cleanup: assert `git status` shows no leftover boundaries_qa_violation files.
```

### API-16 — Every state-transition operation has a CLI verb (programmatic enforcement)

```yaml qa-scenario
id: api-16-cli-coverage
title: For every state-transition Operation in spec, a Cobra leaf command exists; document exceptions explicitly
theme: cli.coverage
coverage:
  primary:
    - api.parity.cli
    - cli.coverage
risk: medium
live: false
provider: mock-acp
preconditions:
  - Worktree builds; `cli.NewRootCommand()` is callable
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/spec/spec.go:144-208 (OperationSpec)
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/root.go:65-123 (NewRootCommand)
steps:
  - Walk `spec.Operations()`; for each entry whose `OperationID` matches the state-transition regex, record `(OperationID, Method, Path)`.
  - Walk `cli.NewRootCommand()` recursively: for each leaf cobra.Command, record its full path (e.g. `agh task complete`) and its declared use string.
  - Cross-reference using a hand-curated mapping table embedded in this scenario report under `mappings:` (e.g. `claimTaskRun → agh task run claim`, `enableSkill → agh skill enable`, etc.). The expected shape is one line per OperationID.
  - Emit `api-16-coverage.json` listing missing CLI verbs and unmapped operations.
  - Document the documented exceptions: `webhooks/*`, `hosted-mcp/*`, agent-kernel-tasks UDS-only paths surfaced by `agh task` peer-claim verbs (`agh task next/heartbeat/release/complete/fail`), `streamSettingsObservabilityLogTail`, `streamBridgeHealth`, `streamTask`, `listObserveEvents` stream variant.
expected:
  - `api-16-coverage.json` reports zero non-exception missing CLI verbs.
  - Document any operation that becomes a CLI gap (and propose its verb name) under "Follow-up".
evidence:
  - `api-16-coverage.json`, `api-16-coverage.md`
failure_signatures:
  - Non-exception missing CLI verb: violates "Agent-manageable by default" (`internal/CLAUDE.md:26`).
cleanup: nothing.
```

### API-17 — SSE event coverage: every required correlation key present

```yaml qa-scenario
id: api-17-sse-correlation-keys
title: Across the SSE captures from API-02..API-06, every observability correlation key listed in CLAUDE.md appears at least once
theme: sse.coverage
coverage:
  primary:
    - sse.correlation_keys
    - event.lineage-correlation
risk: medium
live: true
provider: real-claude-code
preconditions:
  - At least the artifacts from API-02, API-04, API-05, API-06 already produced
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Observability bullet listing keys)
  - /Users/pedronauck/Dev/compozy/agh/internal/transcript/transcript.go (canonical agh.session.event.v1 schema)
steps:
  - For each of the keys: `session_id`, `parent_session_id`, `root_session_id`, `agent_name`, `task_id`, `run_id`, `claim_token_hash`, `lease_until`, `workflow_id`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `spawn_depth`, `actor_kind`, `actor_id`, `release_reason`:
    - Grep the union of SSE captures and `events.db` JSON dumps from previous scenarios.
    - For every key, record the first 3 file:line occurrences.
  - Generate scenarios that exercise the missing keys: spawn (parent_session_id, root_session_id, spawn_depth — covered by ACP-08), task claim (claim_token_hash, lease_until, actor_kind, actor_id — covered by AUT-01..AUT-02), hooks dispatch (hook_event, hook_name — covered by AUT-03 in `04-autonomy-kernel.md`), automation/coordinator (workflow_id, coordinator_session_id, scheduler_reason).
  - Also assert every event row has a `schema = "agh.session.event.v1"` marker.
  - Final report: per key, "found N times" plus the originating scenario id.
expected:
  - Every key found at least once across the unified evidence corpus.
  - Schema marker present in every event content.
  - Raw `agh_claim_*` token never appears anywhere (per the security invariant).
evidence:
  - `api-17-coverage.json` (per-key locations), `api-17-keys-missing.txt` (must be empty)
failure_signatures:
  - A key missing entirely: observability gap; the canonical event for the corresponding lifecycle is incomplete.
cleanup: nothing.
```

### API-18 — Compose: CLI prompt → switch to HTTP SSE replay → switch back to CLI tail

```yaml qa-scenario
id: api-18-compose-transports
title: A real Claude Code conversation kicked off via CLI is observable mid-stream via HTTP SSE replay AND tailable via CLI again — all three views are consistent
theme: api.compose
coverage:
  primary:
    - api.parity.compose
    - sse.replay
risk: high
live: true
provider: real-claude-code
preconditions:
  - ACP-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/session.go:287-378
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go:508-545
steps:
  - Phase 1 (CLI): `agh session new --agent claude -o json` → S; `agh session prompt $S "Plan a 5-step refactor of src/ then start step 1" -o jsonl &` (background); record start wall-clock.
  - Phase 2 (HTTP SSE replay): after 3 s, while the prompt is still running, in a sibling shell connect to `GET http://127.0.0.1:$PORT/api/sessions/$S/stream` with `Last-Event-ID: 0` (replay full). Read until you see at least one `text-delta`. Capture into `api-18-http-replay.sse`.
  - Phase 3 (CLI tail): in a third shell, `agh session events $S --follow -o jsonl > api-18-cli-tail.jsonl`. Let it run until prompt completes.
  - Wait for prompt completion (Phase 1's job).
  - Compare: every `id:` from `api-18-http-replay.sse` must appear in `api-18-cli-tail.jsonl` and the canonical events.db (`agh session events $S -o jsonl > api-18-events.jsonl`).
  - Final transcript: `agh session transcript $S -o json > api-18-transcript.json`. Reconstruct from each capture; assert all three reconstructions are identical post-normalization.
expected:
  - Three transports give three consistent views of the same session.
  - The HTTP replay, the CLI follow tail, and the persisted events.db all agree on every event id and every event payload (modulo volatile fields).
  - The reconstructed transcript is identical from all three.
evidence:
  - `api-18-http-replay.sse`, `api-18-cli-tail.jsonl`, `api-18-events.jsonl`, `api-18-transcript.json`, `api-18-three-way-diff.txt`
failure_signatures:
  - HTTP replay missing events that CLI tail saw: durable-append vs broadcast race; events broadcast before durable append (forbidden).
  - CLI tail diverges from events.db: CLI client filters or transforms before stdout (not allowed for `-o jsonl`).
cleanup: `agh session stop $S`; kill any leftover background CLI processes.
```

## 8. Edge cases (must be hit at least once)

- **Empty prompt body** — `POST /api/sessions/:id/prompt` with `{}` returns 400 "message is required" (`internal/api/httpapi/prompt.go:213-244`); UDS does the same; CLI exits non-zero with a typed error.
- **`Last-Event-ID` non-numeric** — replay stream returns 400 with the wrapped error from `parseLastEventID` (`internal/api/core/session_stream.go:16-27`); ObserveCursor variant returns the same shape (`internal/api/core/parsers.go:149-163`).
- **Slow consumer disconnect** — TCP close during SSE: prompt continues (per detached-lifetime invariant; covered in ACP-05). The /stream endpoint, however, must NOT keep the prompt alive on its own — `/stream` is a passive observer.
- **`--output` flag interplay with `--json`** — `cli/format.go:89-102` prefers `--output` if set; `--json` is the legacy alias. Assert both end up at `OutputFormat == OutputJSON`.
- **Duplicate `OperationID`** — caught by `internal/api/spec/spec_test.go:1219-1232`; this child must NOT emit a duplicate.
- **HTTP CORS preflight (OPTIONS)** — `corsMiddleware` returns 204 without body for `OPTIONS` (`middleware.go:58-61`); the rest of the chain is short-circuited.
- **UDS socket path missing** — daemon must fail to start with a clear "udsapi: listen on …: …" error (`udsapi/server.go:631-635`); `agh daemon start` exits non-zero.
- **Connection hijack on UDS** — peer-info wiring uses `mcppkg.PeerInfoFromConn` (`udsapi/server.go:648`). When `PeerInfoFromConn` fails (e.g. on macOS without `getsockopt(SO_PEERCRED)`), the context still carries the error so handlers can refuse if they choose. CLI must surface the typed error rather than 5xx.
- **CodegenCheck partial failure** — `cmd/agh-codegen/main.go:45-58` runs OpenAPI then SDK; if the first fails, the second is not attempted. Assert the failing path's diagnostics include the artifact path.
- **`make verify` interaction** — `magefile.go:302-322` orders `CodegenCheck → InstallerCheck → BunLint → BunTypecheck → BunTest → WebBuild → Fmt → Lint → Test → buildGo → Boundaries`. A failure in any stage stops the chain (`magefile.go:317-320`); assert error message names the failing stage.

## 9. Integration surfaces

| Surface                                      | Kind          | Source                                                                                                            |
| --------------------------------------------- | ------------- | ----------------------------------------------------------------------------------------------------------------- |
| `POST /api/sessions/:id/prompt`               | HTTP SSE      | `internal/api/httpapi/prompt.go:90-156` (entry); `:251-580` (typed envelope state machine)                        |
| `POST /api/sessions/:id/prompt`               | UDS SSE       | `internal/api/udsapi/prompt.go:22-74`                                                                              |
| `POST /api/sessions/:id/prompt/cancel`        | HTTP + UDS    | `internal/api/httpapi/sessions.go:43-50`, `internal/api/udsapi/sessions.go:20-…`                                  |
| `GET /api/sessions/:id/stream` (SSE)          | HTTP + UDS    | `internal/api/core/handlers.go:508-545`, `internal/api/core/session_stream.go:69-100`                              |
| `GET /api/observe/events/stream` (SSE)        | HTTP + UDS    | `internal/api/core/handlers.go:813` (`ObserveCursor` from `Last-Event-ID`)                                         |
| `GET /api/bridges/health/stream` (SSE)        | HTTP + UDS    | BaseHandlers `StreamBridgeHealth`                                                                                  |
| `GET /api/tasks/:id/stream` (SSE)             | HTTP + UDS    | `BaseHandlers.StreamTask`, `core/parsers.go:249-254`                                                              |
| `GET /api/settings/observability/log-tail` (SSE) | HTTP + UDS | `streamSettingsObservabilityLogTail`                                                                              |
| OpenAPI generation                            | Build         | `cmd/agh-codegen/main.go`, `internal/api/spec/spec.go:159-208`                                                     |
| TS type generation                            | Build         | `internal/codegen/openapits/generate.go`, runs `bunx openapi-typescript` + `bunx oxfmt`                            |
| SDK contracts generation                      | Build         | `internal/codegen/sdkts/generate.go`                                                                              |
| Boundaries enforcement                        | Static        | `magefile.go:231-300`                                                                                              |
| HTTP middleware                               | HTTP          | `internal/api/httpapi/middleware.go` (CORS, errorMiddleware, loopbackMutationGuard)                                |
| UDS server                                    | UDS           | `internal/api/udsapi/server.go:625-740` (Unix listener, `0o600` chmod, peer info)                                  |
| CLI root                                      | CLI           | `internal/cli/root.go:65-123`                                                                                      |
| CLI format                                    | CLI           | `internal/cli/format.go:20-27`                                                                                     |
| CLI exit codes                                | CLI           | `internal/agentidentity/identity.go:34-45` and `internal/cli/format.go` (cliExitCodeForError)                      |

## 10. Failure modes

| Mode                                                         | Surface              | Detection                                                                                       | Scenario     |
| ------------------------------------------------------------ | -------------------- | ----------------------------------------------------------------------------------------------- | ------------ |
| Operation declared `TransportHTTP` but missing HTTP route    | `httpapi/routes.go`  | API-01 matrix run; route inventory ≠ spec inventory                                             | API-01       |
| Operation declared `TransportUDS` but missing UDS route      | `udsapi/routes.go`   | Same                                                                                             | API-01       |
| State-transition op has no CLI verb                          | `internal/cli/`      | API-16 coverage check                                                                            | API-01, API-16 |
| HTTP and CLI prompt diverge in event-type sequence           | BaseHandlers prompt  | API-02 normalized diff                                                                           | API-02, API-03 |
| HTTP + UDS reads diverge                                     | BaseHandlers reads   | API-04 canonical diff                                                                            | API-04       |
| SSE reconnect replays events ≤ N                             | session_stream.go    | API-05 id-intersection check                                                                     | API-05       |
| Slow consumer OOMs daemon                                    | SSE writers          | API-06 RSS sampling                                                                              | API-06       |
| Slow consumer loses durable events                           | broadcaster          | API-06 events.db vs slow SSE log union                                                           | API-06       |
| Codegen drift undetected                                     | codegen-check        | API-07/API-08 deliberate edits                                                                   | API-07, API-08 |
| OpenAPI ↔ TS not in lockstep                                 | codegen + openapits  | API-09 web-build after codegen                                                                   | API-09       |
| Error mapping diverges between transports                    | core/errors.go       | API-10 canonical body diff                                                                       | API-10       |
| HTTP origin denial regression                                | corsMiddleware       | API-11 forged Origin header                                                                      | API-11       |
| HTTP loopback-only mutation guard regression                 | loopbackMutationGuard| API-11 non-loopback bind test                                                                    | API-11       |
| UDS socket mode wrong                                        | udsapi/server.go     | API-11 stat                                                                                      | API-11       |
| Agent-kernel HTTP exposure                                   | routes registration  | API-12 HTTP /api/agent/me probe                                                                  | API-12       |
| CLI JSON envelope drift                                      | cli/format.go        | API-13 golden snapshot                                                                           | API-13       |
| CLI JSONL line is non-JSON                                   | cli streaming        | API-14 jq per-line validation                                                                    | API-14       |
| Boundaries rule false-positive/negative                      | magefile.go:231      | API-15 planted violation file                                                                    | API-15       |
| Correlation key missing across SSE/events                    | observability        | API-17 keyword sweep                                                                             | API-17       |
| Three-transport view diverges                                | full stack           | API-18 compose                                                                                   | API-18       |

## 11. Fixtures

- **Bootstrap manifest**: produced by `agh-qa-bootstrap`. Includes unique `AGH_HOME`, daemon HTTP port, daemon UDS path, tmux-bridge socket, `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, `AGH_WEB_API_PROXY_TARGET`.
- **Workspace seed**: `$LAB/workspace/` with `README.md` (≥3 paragraphs), `src/file_a.go`, `src/file_b.go`, `generated_long_file.txt` (~2 MB) for API-06.
- **Provider auth**: direct `claude` uses native Claude CLI auth from the
  effective Claude home for the lane (operator `HOME` by default; isolated
  `PROVIDER_HOME` only for explicit isolated-home scenarios). Bound-secret,
  brokered, and Codex-specific lanes stage auth into `PROVIDER_HOME` /
  `PROVIDER_CODEX_HOME`.
- **Golden snapshots** (API-13): seeded under `internal/cli/testdata/golden/` on first run; updated only via explicit opt-in flag `AGH_GOLDEN_UPDATE=1` (similar to vitest snapshot policy).
- **Boundaries violation fixture** (API-15): planted file `internal/session/boundaries_qa_violation.go` with `//go:build qa_boundary_violation` tag; removed at scenario cleanup.
- **Forbidden-needle list** (covered by ACP-18 in `03-acp-sessions.md`): `["agh_claim_FAKE_QA_", "agh_claim_TESTONLY_"]` swept across SSE, events.db dumps, daemon log, web SSE, transcripts.
- **goleak build tag** for the in-process integration runner: `//go:build goleak_check`.

## Citations

- Repo-wide rules: `/Users/pedronauck/Dev/compozy/agh/CLAUDE.md` (Critical Rules; Workflow; Build Commands; Skill Dispatch; CI/Release; Cross-References).
- Backend invariants: `/Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md` — Architecture (lines 9-49), Concurrency (29-37), Observability (47-52), Security Invariants (55-62).
- Contract:
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/contract/` — `bridges.go`, `bundles.go`, `agents.go`, `automation.go`, `responses.go`, `tasks.go`, `tools.go`, `vault.go`, `settings.go`, `resources.go`, `authored_context.go`. ≥411 declared types per grep.
- Spec / OpenAPI:
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/spec/spec.go:121-208` (`Transport`, `OperationSpec`, `Document`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/spec/spec_test.go:1219-1232` (duplicate-id detector).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/spec/authored_context.go`, `settings.go`, `vault.go`, `resources_test.go` — per-group transport assertions.
- BaseHandlers:
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go:31-241` (config, ctor, port shadow); `:508-545` (StreamSession); `:813` (ObserveCursor parse); `:895-1043` (DaemonStatus + HTTPPortValue).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:16-27` (parseLastEventID), `:69-100` (pollAndStreamSessionEvents).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/core/parsers.go:28-50, 149-163, 232-254` (after_sequence, ObserveCursor, TaskStreamQuery parsers).
- HTTP transport:
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/routes.go:1-350` (full route map).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/middleware.go:1-260` (CORS, errorMiddleware, loopbackMutationGuard, errLoopbackMutationRequired).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/handlers.go:141` (resourceAuthMiddleware).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:90-580` (HTTP SSE entry + typed envelope state machine).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/sessions.go:43-50` (cancelSessionPrompt).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/transport_parity_integration_test.go` (existing parity coverage to extend).
- UDS transport:
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/routes.go:1-387` (full route map; agent-kernel UDS-only at `:111-133`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/server.go:625-740` (listener with `0o600` chmod; peer-info ConnContext).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/prompt.go:22-74`.
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/sessions.go:20-…`.
- Identity:
  - `/Users/pedronauck/Dev/compozy/agh/internal/agentidentity/identity.go:18-247` (env vars `AGH_SESSION_ID`/`AGH_AGENT`; UDS headers `X-AGH-Session-ID`/`X-AGH-Agent`/`X-AGH-Workspace-ID`; exit codes 64/65/77/69).
- CLI:
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/root.go:21-303` (root construction, format flags, deps wiring).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/format.go:20-27` (output formats).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/session.go:16-378` (session subtree).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/task.go:48-1100` (task tree, including run subverbs).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/daemon.go:30-119` (daemon lifecycle).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/agent_kernel.go:19-231` (`me`, `ch list/recv/send/reply`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/spawn.go:31-…` (spawn).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/network.go:17-178` (network).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/observe.go:14-100` (observe).
- SSE shared helper:
  - `/Users/pedronauck/Dev/compozy/agh/internal/sse/decode.go:1-153`.
- Codegen:
  - `/Users/pedronauck/Dev/compozy/agh/cmd/agh-codegen/main.go:1-211` (entry; `writeOpenAPI`, `writeSDKContracts`, `checkOpenAPI`, `checkSDKContracts`, `canonicalJSON`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/codegen/openapits/generate.go:1-100+` (Generate / Check via `bunx openapi-typescript` + `bunx oxfmt`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/codegen/sdkts/generate.go` (SDK contracts emitter).
- Boundaries / build:
  - `/Users/pedronauck/Dev/compozy/agh/magefile.go:130-300` (Codegen, CodegenCheck, BunLint, BunTypecheck, BunTest, Boundaries).
  - `/Users/pedronauck/Dev/compozy/agh/Makefile:9-100` (`make codegen`, `make codegen-check`, `make boundaries`, `make verify`, `make cli-docs`).
- QA framework references:
  - `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md` (provider-mode tri-state, evidence-as-pass-criterion, four-artifact contract).
  - `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md` (hermetic-env shield, async/cancel ≤2 s, source-text invariants — adapted to Go boundaries-rule + codegen-drift fixtures).
- Sibling final-qa children for cross-reference:
  - `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/03-acp-sessions.md` (real Claude Code prompt path; ACP-04 SSE replay; ACP-18 claim-token redaction).
  - `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/04-autonomy-kernel.md` (AUT-01 claim-race correlation keys).

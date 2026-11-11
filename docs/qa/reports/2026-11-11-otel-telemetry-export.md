# QA Run Report — 2026-11-11 — otel-telemetry-export

- **Scope:** OTLP trace and metric export (#1361, spec `otel-telemetry-export`) — the six `OBS-otel-export` walks
  against the `_dx.md` Collector → Jaeger + Prometheus stack, plus the `RT-001` `otel` leg.
- **Cadence tier:** targeted
- **Build:** branch `run/otel-task-14-real-user-qa-execution-3c3d82c8` from `09ba702a3` (feat/otel-telemetry-export,
  including `e434ff7fd`), daemon `v0.5.4-77-g09ba702a3`; web from the same branch through its Vite dev server
  (`--port 43117`, proxied to the lab daemon); fix `47b40a7b4` re-walked on the same lab.
- **Environment:** isolated `eng-qa-bootstrap` lab (`/tmp/compozyqa-5ea098baf91a/runtime`, HTTP `127.0.0.1:51131`, own
  UDS and tmux socket); every lab call ran through a wrapper that unsets inherited `COMPOZY_*` variables. Compose stack
  verbatim from `_dx.md` (Collector contrib 0.138.0, Jaeger 2.11.0, Prometheus 3.7.3) on the default host ports, no
  remap. Provider `claude` (`native_cli`, operator home) on `claude-sonnet-5-5`; `metric_interval = 5s` for lab speed.
- **Started:** 2026-11-11T05:27Z · **Status:** closed

Parity notes: the daemon was linked to `/dev/shm` because the root filesystem was full (about 1 GB free at the start,
350 MB at the end); `make build` itself failed with `no space left on device`. The daemon's embedded web assets come
from the pinned `compozy-web-assets` module, not this branch, so the web walk used the branch's dev server.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | runtime administrator | desktop / wifi-fast / en-US | CH-otel-export-collector-walk |
| Ada | agent operator (walk 6) | desktop / wifi-fast / en-US | CH-otel-export-collector-walk |

## Flows in Scope

- `J-export-telemetry-to-collector` — export daemon traces and metrics to an OTLP collector
  (`../journeys/J-export-telemetry-to-collector.md`)

## Session Matrix & Results

Evidence paths are relative to `docs/qa/evidence/2026-11-11-otel-telemetry-export/`.

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-otel-export-collector-walk | J-export-telemetry-to-collector / OBS-otel-export §1 golden path | Dora | Feature Tour | Pass | BUG-20261111-otel-model-exports-other; BUG-20261111-otel-model-probe-spawn-provider-other | |
| 2 | CH-otel-export-collector-walk | J-export-telemetry-to-collector / OBS-otel-export §2 status + RT-001 | Dora | Feature Tour | Blocked (human decision) | BUG-20261111-otel-header-refs-redacted | |
| 3a | CH-otel-export-collector-walk | J-export-telemetry-to-collector / OBS-otel-export §3 web | Dora | Feature Tour | Fixed | BUG-20261111-web-adapter-requests-skip-traceparent | 47b40a7b4 |
| 3b | CH-otel-export-collector-walk | J-export-telemetry-to-collector / OBS-otel-export §3 desktop | Dora | Feature Tour | Blocked (needs human verify) | | |
| 4 | CH-otel-export-collector-walk | J-export-telemetry-to-collector / OBS-otel-export §4 provider forwarding | Dora | Feature Tour | Blocked (needs human verify) | | |
| 5 | CH-otel-export-collector-walk | J-export-telemetry-to-collector / OBS-otel-export §5 background traces | Dora | Feature Tour | Pass | | |
| 6 | CH-otel-export-collector-walk | J-export-telemetry-to-collector / OBS-otel-export §6 agent refusal | Ada | Feature Tour | Blocked (human decision) | BUG-20261111-otel-header-refs-redacted | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

Scenario verdicts: `OBS-otel-export` → `blocked-decision`; `RT-001` → `pass` (the `otel` object matched across CLI,
HTTP, and UDS in every state).

### Per-walk results

1. **Golden path — Pass.** `OTel: disabled` → `config set` shows `Lifecycle: restart-required`, `Next Action:
   restart-daemon`, `Restart Required: true` → after restart `ok · http/protobuf → localhost:4318 · traces pending,
   metrics pending · sample 1.0`, then `traces ok, metrics ok`. The CLI prompt produced one trace
   `42112074…`: `POST /api/workspaces/:workspace_id/sessions/:session_id/prompt` → `session.turn`
   (`compozy.session_id`, `turn_id`, `run_id`, `workspace_id`, `profile_id`, `agent = general`, `gen_ai.system =
   claude`, `outcome = ok`, `db.flushes = 28`, `db.write_ms_total = 48`) → `acp.process.spawn` (`claude`) with
   `acp initialize`/`session/new`, `acp session/prompt` with a `first_token` event, and `mcp.hosted.call` →
   `tool.execute` (`compozy.tool = compozy__session_list`, same `run_id` as the turn, `risk = read`), plus
   `hook.dispatch` and `db.write` children. p95 query: `NaN` after one turn (the series' first sample already held the
   turn, so `rate()` saw no increase), `35.75` s after the second turn. `compozy_tokens_total`,
   `compozy_tool_calls_total`, `compozy_cost_total`, and `http_server_request_duration_seconds_bucket` exist. Canary
   `QA-CANARY-otel-20261111`: 0 hits in its trace and 0 in every daemon trace of the last hour, which also had no
   `/home/…` path. Every exported span name, attribute key, metric, and resource attribute is listed in
   `telemetry-export.mdx` "What is exported". Evidence: `w1-*`.
2. **Status over four surfaces — Blocked (human decision).** For ok, metrics off, degraded unreachable
   (`127.0.0.1:1`: `traces unavailable (3 failed), metrics unavailable (3 failed)`), degraded `header_ref_unresolved`
   (`failed_exports: 0`), recovered, and forwarding (`… · forwarding to providers`, `forward_to_providers: true`), the
   human line matched `_dx.md` and the `otel` object was identical across `status --json`, HTTP, and UDS. The only
   difference was `last_export_at` in the forwarding read, where a 5 s export landed between the CLI and HTTP reads. A
   prompt completed normally in each degraded state, and `status` exited 0. One expectation failed: the daemon log
   masks the unresolved ref (`"ref":"[REDACTED]"`), which needs a product decision (below). The optional 401 leg was
   not walked; E2E-002 owns it. Evidence: `w2-*`.
3. **Web — Fixed; desktop — Blocked (needs human verify).** The composer's `POST …/prompt` carried
   `00-fc6c6d88…-…-00`, and Jaeger's trace for that id is the HTTP server span → `session.turn` → ACP spans, so
   `e434ff7fd` works. 232 of 243 daemon requests carried a fresh `-00` header (232 unique trace ids). Nine did not:
   agent-instance, terminal, and window-manager requests → BUG-20261111-web-adapter-requests-skip-traceparent, fixed
   in `47b40a7b4`. After the fix, every `/api/` request carried one except the EventSource `…/stream`, which cannot
   set headers; Jaeger shows the `GET /api/terminals` trace under the client's id. No request left for a non-daemon
   origin. At `sample_ratio 0`, a web prompt worked and Jaeger returned `trace not found` for its id. Desktop: no
   packaged desktop build exists on this machine, and packaging (`desktop build` = Go runtime + Electron) did not fit
   on the full disk; `desktop/src/bootstrap/__tests__/runtime-health-monitor.test.ts` passes 13/13 (identity probe
   sends `-00`, update feed gets none). The desktop window embeds the same web client, so the web fix covers its
   adapters. Evidence: `w3-*`.
4. **Provider forwarding — Blocked (needs human verify).** With forwarding on, each process in the Claude chain
   (`npm exec` → `claude-agent-acp` → `claude`) received `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`,
   `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`, `OTEL_RESOURCE_ATTRIBUTES=compozy.session_id=<sess>,compozy.agent=otelprobe`,
   and `TRACEPARENT` equal to the turn's `acp.process.spawn` span (`60f9c4f9e00a45ee`), and no
   `OTEL_EXPORTER_OTLP_HEADERS` (none configured). An agent `session_env` endpoint (`http://127.0.0.1:4999`) won over
   the forwarded one. With forwarding off, only the agent's own variables remained. The provider-side check could not
   pass: Claude Code 2.1.321 opened no connection to the Collector and Prometheus had no `claude_code_*` series, even
   when run directly (`claude -p` with `CLAUDE_CODE_ENABLE_TELEMETRY=1`, `OTEL_METRICS_EXPORTER=otlp`, the same
   endpoint), so the gap is in the provider/account environment, not in forwarding. Docs check (US-008.AC-4):
   `telemetry-export.mdx` states in bold that forwarding is an operator policy not covered by the daemon privacy
   guarantee, that the provider receives resolved header values, and that nothing is forwarded while export is
   disabled, inactive, or degraded at start. Evidence: `w4-*`.
5. **Background traces — Pass.** `scheduler.tick` roots every 15.0 s (longer gaps only across daemon restarts).
   `automation.run` root with `compozy.automation.id = job-b5792d09897ce012`; the job's `acp.process.spawn`,
   `session.turn`, and `hook.dispatch` (`session.post_create`, `session.message_persisted`, `session.post_stop`) descend
   from it. `loop.run` root with `compozy.loop.id = looprun-295092b387aac6d7`; both node turns descend from it. Hook
   spans sit under the turn that fired them. No prompt text appears in any background trace. Evidence: `w5-*`.
6. **Agent refusal — Blocked (human decision).** A live Claude turn called `compozy__config_set` for
   `observability.otel.enabled` and `observability.otel.endpoint`; both returned `config_trust_root_forbidden`, and
   `config.toml` stayed byte-identical (sha256). The operator-side `compozy tool invoke compozy__config_set` was refused
   the same way (`tool_denied`, `reason_codes: [config_trust_root_forbidden]`), which matches `_dx.md` ("refuses every
   `observability.otel.*` key"). `compozy__config_get` for a vault header returned `"value": "[REDACTED]",
   "redacted": false` instead of the promised ref; the operator CLI returned `vault:observability/qa-otlp`. No secret
   leaked. Status was unchanged afterwards. Evidence: `w6-*`.

## Session Debriefs

### CH-otel-export-collector-walk — Dora (Ada for §6)

- **Ran:** 2026-11-11T05:29Z → 05:56Z (box respected: yes; the build and the disk-full workaround happened before the box).
- **Findings:**
  - Some web daemon requests did not join a client trace (Friction): agent-instance, terminal, and window-manager
    adapters used a bare fetch. Fixed.
  - Every turn's model exports `other` (Trust-Damage: the docs promise the catalog name for dashboards grouped by model).
  - A turn's model-option inspection spawn exports `gen_ai.system = other` (Friction).
  - Header refs are masked in the daemon log and in `compozy__config_get` (Friction; needs a decision).
- **Bugs filed/updated:** BUG-20261111-web-adapter-requests-skip-traceparent (fixed),
  BUG-20261111-otel-model-exports-other, BUG-20261111-otel-model-probe-spawn-provider-other,
  BUG-20261111-otel-header-refs-redacted.
- **Scenarios settled:** OBS-otel-export → blocked-decision; RT-001 → pass.
- **Paper cuts:** see Paper Cuts.
- **Surprises:** the first prompt of a fresh daemon spent about 14 s in a synchronous model-option inspection (15
  `session/set_config_option` probes) before the real provider spawn. The export made this visible; it predates this
  spec. Claude Code did not export its own telemetry in this environment.
- **Suggested next charter:** desktop app attached to a lab daemon (needs a packaged build and disk space); provider
  forwarding on a machine where Claude Code's own OTLP export works.

## What Was Fixed

### BUG-20261111-web-adapter-requests-skip-traceparent: Some web requests to the daemon start their own trace
- **Symptom:** agent-instance, terminal, and window-manager requests from the web UI carried no `traceparent`.
- **Root cause:** five adapters called the daemon through `runtimeFetch`, a bare `globalThis.fetch` delegate; only
  `apiClient`'s middleware and `daemonFetch` applied the traceparent rule.
- **Fix:** `47b40a7b4` — `runtimeFetch` removed; `apiClient` and every adapter use `daemonFetch`, the single place
  that applies the daemon-origin rule (the middleware's duplicate call was dropped).
- **Regression test:** `web/src/systems/terminal/adapters/__tests__/terminal-api.test.ts` "Should join each daemon
  request to a fresh unsampled trace" — failed before (no header), passes after.
- **Retested:** web walk re-run on the same lab: every `/api/` request except the EventSource stream carried a `-00`
  header; the `GET /api/terminals` trace id resolved in Jaeger. Focused checks: `bunx vitest run` on `api-client`,
  terminal, settings adapters, os, agent adapters, support, and the goal chat transport — 168 files, 1976 tests passed;
  `bunx turbo run typecheck --filter=./web` passed; `oxlint` and `oxfmt --check` on the changed files clean.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Dora | J-export-telemetry-to-collector, Prometheus p95 | "The p95 query from the docs returns NaN after my first prompt." Needs a second turn inside the 5 m window. | dull | watching (docs could mention it) |
| Dora | J-export-telemetry-to-collector, `config set` | The published sample shows `Scope: global`, `Status: saved`, `Winning Layer: global`; the CLI prints `Scope: user`, `Status: --`, `Winning Layer: --`. | dull | deferred (docs sample drift) |
| Dora | J-export-telemetry-to-collector, Jaeger `db.write` | Some `compozy.db.operation` values are Go symbol names (`github.com/compozy/compozy/internal/store/sessiondb.(*SessionDB).writeTokenUsage`, `globaldb.UpsertToolProcessRecord`) next to snake_case names. | dull | deferred |
| Dora | J-export-telemetry-to-collector, restart | Once, right after web onboarding, `compozy daemon stop && compozy daemon start` failed with `daemon: already running with pid …`; a retry worked and five later restarts were clean. | dull | watching (not reproduced) |
| Ada | Loop start | `compozy loop run` from the CLI needs `start: - kind: uds`; declaring `kind: cli` alone is refused with `start_kind_not_allowed`. | dull | deferred (outside this spec) |

## Runtime Errors Observed

- `failed to handle notification` `auth/status_update` (`Method not found`) from the Claude ACP adapter on every spawn —
  provider adapter notification the daemon does not handle; unrelated to export, not filed.
- `daemon.model_catalog.refresh_failed` for providers without a CLI or credential in the lab — expected in a fresh lab.
- `daemon: background update check failed` (GitHub rate limit) — environmental.

## Human Verifications Needed

- [ ] Desktop (row 3b): on a machine with disk space, `bun run --cwd desktop build` (or use a packaged CompozyOS
  build from this branch), attach the app to a daemon with export `ok` and `sample_ratio 1.0`, and capture one
  `GET /api/status/identity` from the desktop logs or a proxy. Its `traceparent` must be `00-<id>-<span>-00`, and
  `curl -s http://localhost:16686/api/traces/<id>` must return the HTTP server span. Send one prompt from the desktop
  window and look up its trace the same way: it must continue into `session.turn`. The update feed request must carry
  no `traceparent`.
- [ ] Provider export (row 4): on an account and machine where `claude -p` with `CLAUDE_CODE_ENABLE_TELEMETRY=1`,
  `OTEL_METRICS_EXPORTER=otlp`, and `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` produces `claude_code_*`
  series in the `_dx.md` Prometheus, rerun OBS-otel-export §4 steps 1–3 with the Collector's `prometheus` exporter set
  to `resource_to_telemetry_conversion: { enabled: true }`; the provider series must carry
  `compozy_session_id = <session id>`.

## Decisions for a Human

### Header refs are masked where the contract shows them (BUG-20261111-otel-header-refs-redacted)
- What's broken: the daemon log line for an unresolved header and `compozy__config_get` print `[REDACTED]` instead
  of `env:NAME` / `vault:observability/<path>`; the published troubleshooting table and `_dx.md` promise the ref.
  Evidence: `w2-degraded-header-daemon-log.txt`, `w6-tool-invoke-config-get-header.json`.
- Why not auto-fixed: both outputs pass the daemon-wide redaction engine, which deliberately masks every
  `env:`/`vault:` reference (`internal/redact`). Exempting them is a security-policy change outside this QA task.
- Options: 1. Correct `_dx.md` and `telemetry-export.mdx` ("the log names the header"; agents read refs through the
  operator CLI only) — no security change, but agents lose the documented read. 2. Exempt telemetry header refs from
  ref redaction on these two surfaces (a protected log key and a config-tool allow rule for
  `observability.otel.headers.*`) — matches the contract, but changes a daemon-wide rule and needs its own tests.
- Recommendation: option 1 now. Refs are low-value for agents, and the header name already locates the entry. Revisit
  option 2 only if operators report friction.

## Learnings

- The daemon's embedded web assets come from the pinned `compozy-web-assets` module. A web fix on a branch is not in
  the lab daemon's UI; run the branch's Vite dev server against the lab daemon.
- `rate()`-based p95 needs two samples with an increase; a single-turn smoke shows `NaN`.
- Tool results and logs pass the global redaction engine, which masks `env:`/`vault:` refs. A contract that promises
  to show a ref on those surfaces needs an end-to-end test through that engine.
- Claude Code's own OTLP export did not work in this environment, even run directly; check it before planning a
  provider-forwarding walk.

## Final Status

- **Exit gate:** delivery gates (`make gate`, CI) are owned by the enclosing cy-loop-tasks workflow and were not run
  here. Focused checks for the fix: vitest 168 files / 1976 tests passed; web typecheck passed; oxlint/oxfmt clean.
  Strict lab audit (`audit-qa-evidence.py --strict`): exit 2 with one blocker, C14 "final local gate evidence is
  missing", because it requires a `make verify` log and repository-wide gates belong to the enclosing workflow; C7
  (surfaces) and C12 (verification report) passed after the journey log and lab verification report were written.
  Lab teardown: `teardown.json` `clean: true`, no survivors; Compose stack removed with `docker compose down -v`.
  QA tree checks: `scripts/check-product-language.sh` passed; `materialize_state.py docs/qa` parsed 1096 scenarios.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 (model `other`, open) · Friction 3
  (web traceparent fixed; header refs masked, decision; probe provider `other`, open) · Cosmetic 0.
- **Coverage:** 6/6 walks attempted; walks 1 and 5 pass, walk 3 web fixed and re-walked; walks 2 and 6 wait on the
  header-ref decision; desktop (3b) and the provider's own export (4, step 3) need a human-verified environment. The
  optional 401 leg was not walked (E2E-002 owns it).
- **Verdict:** QA round complete with blocked items: export works end to end against a real Collector, Jaeger, and
  Prometheus. One defect was fixed (`47b40a7b4`); one product decision and two environment-bound verifications remain,
  and two low-to-medium attribute-labeling bugs are open.

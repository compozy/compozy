# BUG-20261111-otel-model-probe-spawn-provider-other: A turn's model-discovery spawn shows provider `other`

- **Status:** fixed
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Dora
- **Journey Step:** J-export-telemetry-to-collector, step "Jaeger: POST prompt then session.turn with ACP and hosted tool spans"
- **Scenarios:** OBS-otel-export
- **Found:** 2026-11-11 · **Report:** docs/qa/reports/2026-11-11-otel-telemetry-export.md

## Summary

The first prompt of a fresh daemon shows two `acp.process.spawn` children under `session.turn`. The first one (about
14 s: `acp initialize`, `acp session/new`, 15 × `acp session/set_config_option`) is the model catalog's ACP model-option
inspection, and it carries `gen_ai.system = other`; only the second spawn (the real provider process) says `claude`.
Dora sees most of her turn's latency in a spawn she cannot attribute to a provider.

## Reproduction

- **Charter:** CH-otel-export-collector-walk · **Tour:** Feature Tour
- **Environment:** isolated lab, fresh `COMPOZY_HOME`, provider `claude`

1. Enable export and restart the daemon.
2. `compozy session new --agent general`, then `compozy session prompt <id> "Call the compozy__session_list tool …"`.
3. Open the turn's trace in Jaeger.

**Expected:** every `acp.process.spawn` carries the provider it launched (`gen_ai.system = claude`).
**Actual:** the model-option inspection spawn carries `gen_ai.system = other`.

## Evidence

- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w1-jaeger-session-turn.json` (span `541d27df3c5c064c`)

## Root cause

`internal/modelcatalog/live_sources_acp.go` `SessionACPModelProbe.InspectModels` builds an
`acp.SessionInspectionRequest` without a provider, so `internal/acp/start.go` `startSpawnSpan(ctx,
normalized.ProviderName)` records an empty provider, which the sanitizer exports as `other`. Passing `ProviderName`
is not a telemetry-only change: a non-empty `ProviderName` also enables the provider auth pre-start probe
(`runProviderPreStart`), so the fix needs a telemetry label separate from `ProviderName`.

Related observation (outside this spec): the inspection runs inside the first turn and delayed the provider spawn by
about 14 s.

## Fix

`acp.SessionInspectionRequest.TelemetryProvider` labels the inspection's `acp.process.spawn` span without setting
`ProviderName`, so the provider pre-start stays off. `SessionACPModelProbe.InspectModels` passes the probed provider
id; `internal/acp/start.go` labels the spawn with `ProviderName`, else that label.

## Retest

`internal/acp` `TestDriverTurnSpans` "Should label a model inspection spawn with the probed provider" exports an
inspection spawn through the real telemetry runtime to the in-process OTLP collector and asserts
`gen_ai.system = claude`; it fails with `other` without the fix. A live Jaeger re-walk of OBS-otel-export §1 is still
pending.

# BUG-20261111-otel-model-exports-other: Every turn's model exports as `other` instead of the catalog model name

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-export-telemetry-to-collector, step "Prometheus: p95 of compozy_session_turn_duration_seconds"
- **Scenarios:** OBS-otel-export
- **Found:** 2026-11-11 · **Report:** docs/qa/reports/2026-11-11-otel-telemetry-export.md
- **Fix commit:** 82347190f
- **Regression test:** `internal/daemon/telemetry_test.go` `TestTelemetryModelClassesFollowTheCatalog` (catalog
  names, `custom`, `other`, snapshot expiry), plus the lab replay in Retest

## Summary

Dora runs a Claude session on `claude-sonnet-5-5` (a built-in catalog model) and groups latency, tokens, and cost by
model in Prometheus. Every series carries `gen_ai_request_model="other"`, so she cannot split by model, although the
published docs ("Value rules", `telemetry-export.mdx`) promise the catalog name for catalog models and `custom` for
config-only ones.

## Reproduction

- **Charter:** CH-otel-export-collector-walk · **Tour:** Feature Tour
- **Environment:** isolated lab, `_dx.md` Compose stack, provider `claude` with `[providers.claude.models] default =
  "claude-sonnet-5-5"`

1. Enable export (`compozy config set observability.otel.enabled true`, endpoint `http://localhost:4318`), restart.
2. `compozy session prompt <id> "…"` twice.
3. `curl -s http://localhost:9090/api/v1/query --data-urlencode query=compozy_session_turn_duration_seconds_count`.
4. Jaeger trace of the turn: `session.turn` tag `gen_ai.request.model`.

**Expected:** `gen_ai.request.model = claude-sonnet-5-5` (catalog name).
**Actual:** `gen_ai.request.model = other` on `session.turn`, `compozy_session_turn_duration_seconds`,
`compozy_tokens_total`, and `compozy_cost_total`.

## Evidence

- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w1-jaeger-session-turn.json` (`session.turn` tags)
- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w1-prom-series.txt`

## Root cause

`internal/daemon/telemetry.go` builds `telemetry.Registries` without `Models`; the sanitizer
(`internal/telemetry/sanitizer.go`, `s.reg.Models == nil`) classifies every model as unknown → `other`. No spec task owns
the model-catalog classifier (builtin/models.dev rows → catalog name, config-only → `custom`). Fix belongs to the
composition root plus a classifier backed by `internal/modelcatalog`; not trivial, so it was not fixed in the QA task.

## Fix

`82347190f` wires `Registries.Models` to a classifier backed by the model catalog
(`internal/daemon/telemetry_models.go`): built-in or models.dev rows export the name, operator-configured or curated
rows export `custom`, anything else `other`. Lookups read an atomically swapped snapshot built at boot and rebuilt in
the background once it is older than 5 minutes.

## Retest

Re-walk of OBS-otel-export §1 on a fresh isolated lab, daemon built from `82347190f`, `_dx.md` Compose stack,
provider `claude` with `[providers.claude.models] default = "claude-opus-5-5"`. The session reported provider
`claude`, model `claude-opus-5-5` (requested, effective, and last known), and the catalog held the row
`(claude, claude-opus-5-5)` with sources `config`, `provider_live`, and `models_dev`.

| Turn | When | `gen_ai.request.model` |
|---|---|---|
| 1 | fresh home, 17 s after boot | `other` |
| 2 | 67 s after boot | `other` |
| 3, 4 | after the 5-minute snapshot TTL | `claude-opus-5-5` |
| 5 | 12 s after a restart, catalog already persisted | `claude-opus-5-5` |

Prometheus `compozy_session_turn_duration_seconds_bucket` and `compozy_tokens_total` carry
`gen_ai_request_model="claude-opus-5-5"` from turn 3 on; the only values in any exported trace or series are
`claude-opus-5-5` and `other`, so no raw user-authored model string was exported.

**Residual (not a regression):** on a fresh catalog, the boot snapshot (telemetry init 14:39:32Z) predates the
`models_dev` refresh for `claude` (14:39:48Z, at session creation). Because the snapshot is rebuilt only on its
5-minute TTL, not when the catalog refreshes, every turn until the TTL exports `other`. That covers the first five
minutes, not only the first turn as the fix commit describes. Once the catalog is persisted, a restarted daemon exports
the name from its first turn. A first-run dashboard therefore shows a short `other` series before the model series
starts. Options are in the report addendum.

Evidence (`docs/qa/evidence/2026-11-11-otel-telemetry-export/`): `w1-model-turn-timeline.txt`,
`w1-model-jaeger-turn2-before-ttl.json`, `w1-model-jaeger-turn4-after-ttl.json`, `w1-model-jaeger-restart-turn.json`,
`w1-model-prom-turn-duration.json`, `w1-model-prom-tokens.json`, `w1-model-distinct-values.txt`,
`w1-model-catalog-sources.txt`, `w1-model-session-runtime.json`.

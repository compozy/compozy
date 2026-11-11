# BUG-20261111-otel-model-exports-other: Every turn's model exports as `other` instead of the catalog model name

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-export-telemetry-to-collector, step "Prometheus: p95 of compozy_session_turn_duration_seconds"
- **Scenarios:** OBS-otel-export
- **Found:** 2026-11-11 · **Report:** docs/qa/reports/2026-11-11-otel-telemetry-export.md

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

## Root cause (from code, not fixed)

`internal/daemon/telemetry.go` builds `telemetry.Registries` without `Models`; the sanitizer
(`internal/telemetry/sanitizer.go`, `s.reg.Models == nil`) classifies every model as unknown → `other`. No spec task owns
the model-catalog classifier (builtin/models.dev rows → catalog name, config-only → `custom`). Fix belongs to the
composition root plus a classifier backed by `internal/modelcatalog`; not trivial, so it was not fixed in the QA task.

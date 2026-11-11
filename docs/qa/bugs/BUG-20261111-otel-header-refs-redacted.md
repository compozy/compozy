# BUG-20261111-otel-header-refs-redacted: Agents and the daemon log cannot see which header ref is configured

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Dora (operator), Ada (agent)
- **Journey Step:** J-export-telemetry-to-collector, "degraded: header_ref_unresolved → fix the vault ref, restart";
  "Agent: compozy__config_get observability.otel.*"
- **Scenarios:** OBS-otel-export
- **Found:** 2026-11-11 · **Report:** docs/qa/reports/2026-11-11-otel-telemetry-export.md

## Summary

Header refs (`env:NAME`, `vault:observability/<path>`) are not secrets, and the contract shows them in two places
where the user sees `[REDACTED]` instead:

1. **Daemon log.** Status reports `header_ref_unresolved`, and the published troubleshooting table
   (`telemetry-export.mdx`, "Troubleshoot export") says "the daemon log names the ref (never its value)". The log line
   names only the header: `{"msg":"telemetry: header ref unresolved","header":"api-key","ref":"[REDACTED]"}`. Dora has
   to open `config.toml` to learn which variable or vault path is missing.
2. **`compozy__config_get`.** `_dx.md` (Native Tools) promises `{"path": "observability.otel.headers.api-key"}` →
   `"vault:observability/newrelic-license"` ("header refs stay refs"). An agent gets
   `{"value": "[REDACTED]", "redacted": false}`; `redacted: false` contradicts the masked value. The operator CLI
   (`compozy config get`) returns the ref correctly.

No resolved secret leaked on either surface.

## Reproduction

- **Charter:** CH-otel-export-collector-walk · **Tour:** Feature Tour

Log:

1. With export ok, `compozy config edit` adds `[observability.otel.headers] "api-key" = "env:QA_OTLP_MISSING"`;
   leave `QA_OTLP_MISSING` unset.
2. `compozy daemon stop && compozy daemon start`; `compozy status` → both signals `header_ref_unresolved`.
3. `grep 'header ref unresolved' $COMPOZY_HOME/logs/compozy.log` → `"ref":"[REDACTED]"`.

Native tool:

1. `printf '%s' '<dummy>' | compozy vault put vault:observability/qa-otlp --kind token --value-stdin`;
   `compozy config edit` adds `"api-key" = "vault:observability/qa-otlp"`.
2. `compozy tool invoke compozy__config_get --input '{"path":"observability.otel.headers.api-key"}' -o json` →
   `"value": "[REDACTED]", "redacted": false`.
3. `compozy config get observability.otel.headers.api-key` → `vault:observability/qa-otlp`.

**Expected:** both surfaces name the ref, never the value.
**Actual:** both surfaces print `[REDACTED]` in place of the ref.

## Evidence

- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w2-degraded-header-daemon-log.txt`
- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w6-tool-invoke-config-get-header.json`
- `docs/qa/evidence/2026-11-11-otel-telemetry-export/w6-config-get-after.txt`

## Root cause (from code, not fixed)

`internal/telemetry/runtime.go` `resolveHeaders` logs `"ref", ref`, and the config tool returns the ref, but both
outputs pass the daemon-wide redaction engine: `redact.exactRedactString` replaces every match of
`secretReferencePattern` (`env:NAME` / `vault:path`, `internal/redact/patterns.go`) with `[REDACTED]`. The telemetry
and tool-policy tests use their own logger or read the structured value before that pass, so none sees it. Which side
changes is a product decision (report, Decisions for a Human): treat refs as displayable on these two surfaces (a
narrow exemption that touches the daemon-wide redaction rule), or correct `_dx.md` and the published docs to say the
log names the header and agents read refs only through the operator CLI.

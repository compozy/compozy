# 2026-09-04 — Model picker startability parity (#549)

Scope: `RT-model-picker-startability-parity`. Isolated lab
`compozy-model-picker-startability-20260904-144612-742622-lab`, `COMPOZY_HOME=/tmp/compozyqa-a0732a662e55/runtime`,
HTTP `127.0.0.1:45365`, binary built from `d4428ab8`. Provider `claude` runs `native_cli` with
`home_policy = operator`, so the operator login was preserved per the provider contract.

## Verdict

`RT-model-picker-startability-parity` — **pass**.

## What the walk showed

**Degraded discovery is now stated, not hidden.** On the cold lab home, before any live refresh, all five
curated Claude rows returned `"startable": false` with `"start_blocked_reason": "live_discovery_unavailable"`.
Before this change the same rows carried `availability_state: "unknown"` and the picker rendered them as
confirmed-live, which is what let it offer models session start then refused.

**A successful refresh binds the logical ids.** After `POST /api/model-catalog/providers/claude/models/refresh`,
`provider_live:claude` reported `row_count: 4`, and exactly the four models Claude Code advertises came back
startable under their CompozyOS ids — `claude-opus-5`, `claude-fable-5`, `claude-sonnet-5` (two configurations:
its own transport value plus `default`), and `claude-haiku-4-5-20251001`. Every `models_dev`-only row
(`claude-sonnet-4-5`, `claude-opus-4-6`, and eight others) stayed `startable: false`. Those are precisely the
rows the old picker showed as selectable.

**Curating one model no longer erases the rest.** `POST /api/model-catalog/providers/claude/models/curate`
with `{"model_id":"claude-opus-4-8","hidden":true}` wrote the same one-entry array the bug report shows:

```toml
[[providers.claude.models.curated]]
hidden = true
id = "claude-opus-4-8"
```

After a forced refresh the other four curated models kept their display names, context windows, and
`startable: true`. Under the previous wholesale replace this write collapsed the effective curated set to that
single model, which destroyed the transport-to-logical id map and made the live list return raw `sonnet` and
`haiku`.

**The refused combination now starts.** A session created against an agent pinned to `provider: claude`,
`model: claude-sonnet-5` reached `active`, and its persisted metadata records
`{"provider": "claude", "model": "claude-sonnet-5"}`. The lab log contains zero occurrences of
`not advertised by the live ACP catalog` — the error the issue reports.

## Evidence

- `qa-artifacts/qa/evidence/catalog-after-curation.json`
- `qa-artifacts/qa/evidence/config-after-curation.toml`
- `qa-artifacts/qa/evidence/session-meta.json`

Evidence lives in the lab root above; it is scratch, not synced.

## Not covered

Browser-side verification of the disabled row and its tooltip was not run: the lab reported
`BROWSER_MODE=blocked` (neither `browser-use` nor `agent-browser` is available on this machine). The
presentation mapping is covered instead by the `to-runtime-selector-options` unit suite, which asserts each
`start_blocked_reason` maps to its badge and tooltip and that sign-in still takes precedence.

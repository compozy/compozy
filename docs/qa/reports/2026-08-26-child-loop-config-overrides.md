# QA Run Report — 2026-08-26 — Child Loop config overrides

- **Scope:** Ephemeral `run-loop.params.config_overrides` for await and detach child runs, typed template materialization, closed-key validation, default compatibility, and stored-config isolation.
- **Cadence tier:** Targeted CLI and runtime.
- **Build:** Uncommitted `feat/run-loop-config-overrides` worktree at base `8753ad9e9`; direct Go binary built from the feature worktree.
- **Environment:** Fresh isolated lab `/home/francisross/tmp-builds/compozy-child-loop-config-overrides-20260826-220440-305142-lab`; manifest `qa-artifacts/qa/bootstrap-manifest.json`; workspace `ws_c16f60623ed4d02c`.
- **Started:** 2026-08-26T22:04:40-03:00 · **Status:** closed, pass.

## Results

| Journey | Parent run | Child run | Status | Evidence |
|---|---|---|---|---|
| Await with typed dynamic overrides | `looprun-8c127e3412b62072` | `looprun-de6aba4f607ad492` | Pass | Child effective config was iteration cap 4, token budget 250000, wall budget 120, `halt`, and one runtime rule; every requested source was `per_run`. |
| Await without overrides | `looprun-9d3971551b3da5a6` | `looprun-8d1bdc727ed0f52d` | Pass | Child retained defaults: iteration cap 50, zero configured budgets, and `failed_only`. |
| Detach with literal overrides | `looprun-27cc6dbfcb141eb3` | `looprun-e229b1452ec86c8d` | Pass | Parent completed without awaiting; child received iteration cap 5, token budget 125000, wall budget 60, and `halt`. |
| Unknown config key | n/a | n/a | Pass | `compozy loop validate` returned `valid:false` and identified unknown field `iteration_caps`. |

## Boundary Evidence

- The dynamic token budget and runtime-rule array came from a preceding transform output, exercising the persisted generation-output path rather than an in-memory fixture.
- Both parent runs kept their own effective defaults. `compozy loop inspect --name qa-child` still reported the stored/default effective config after every child run.
- The configured child reported the requested runtime rule under `run_runtime_rules`; no global or stored Loop configuration write was performed.
- The lab was provider-free: the runtime rule was carried as configuration evidence but did not require an agent session.

## Runtime Errors Observed

None in the final fresh-lab walk. A pre-fix diagnostic run exposed `json.Number` becoming a string through the generic YAML parameter decoder; the canonical coordinator regression test now covers that boundary and the narrow run-loop decoder preserves the materialized JSON value.

## Compozy Impact Audit

- **Persistence:** No schema, migration, Atlas, SQLC, or stored-config contract changed.
- **Public surface:** One optional DSL field was added to the existing reserved `run-loop` action. Omitting it preserves historical behavior.
- **Runtime:** The existing `Inputs.ConfigOverrides` and effective-config precedence are reused. Overrides apply only to the newly created child run.
- **Native tools and API:** No tool ID, HTTP route, UDS method, or request/response schema changed.
- **Official skill and public docs:** Both now document typed templates, strict validation, child-only scope, and no implicit parent-rule inheritance.

## Teardown

The canonical teardown reported `TEARDOWN_ALL_CLEAN=true`. Daemon PID `4046286` required the script's bounded signal sweep; no survivor remained.

## Final Status

**PASS.** Await, detach, default compatibility, typed materialization, validation, provenance, and isolation were observed on a fresh isolated daemon.

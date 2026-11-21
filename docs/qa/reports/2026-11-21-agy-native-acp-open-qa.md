# QA Run Report — 2026-11-21 — Antigravity native ACP open QA follow-up

- **Scope:** open AGY, runtime-policy, and Web UI QA legs after deep-review fixes and `origin/main` (`8b16a4882`) rebase on `feat/agy-native-acp` — `RT-agy-acp-provider` (steps 6–8, `#1889` native file tools, `#1883` login readiness refresh, structured refusal codes), `RT-provider-full-access-mode` (`agy` `auto` precedence + `#1618` retired `provider_full_access` key upgrade), and `ET-web-runtime-selector-minimal-slider` / `MS-web-provider-auth-gate` (AGY Playwright E2E + live Chromium walkthrough).
- **Cadence tier:** targeted.
- **Build:** `3bf59ba3a` (`feat/agy-native-acp`, rebased onto `origin/main` `8b16a4882`) + uncommitted test fix in `internal/api/core/providers_agy_test.go`; `bin/compozy` built with `-ldflags "-X github.com/compozy/compozy/internal/version.Version=v0.7.2 -X github.com/compozy/compozy/internal/version.Commit=3bf59ba3a"` and `web/dist` built via `bun run --cwd web build:raw`.
- **Environment:** Linux x86_64 (`agy` CLI 1.5.13); isolated QA lab `agy-native-open-qa-20261121-045056-325891` (`http://127.0.0.1:37835`, workspace `ws_76bb79e444dc0d19` at `/home/vscode/dev/qa-labs/compozy-agy-native-open-qa-20261121-045056-325891-lab`).
- **Started:** 2026-11-21T04:50:56Z · **Completed:** 2026-11-21T05:10:12Z · **Status:** **PASS (targeted); clean teardown (`clean: true`)**.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Existing native CLI operator | Linux desktop / available network / en-US | `CH-agy-native-cli-operation` |
| Sol | Keyboard-reliant operator | Desktop (1440x900) and 375x820 viewport / available network / en-US | `CH-agy-native-runtime-picker` |

## Flows in Scope

- `J-15` — Operate a session via CLI / API (`../journeys/J-15-operate-session-via-cli-api.md`)
- `J-17` — Launch a session, then choose its next-prompt runtime (`../journeys/J-17-session-create-unified-selector.md`)
- `J-22` — Configure providers and credentials in Settings (`../journeys/J-22-settings-providers.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue / Evidence Summary | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | `CH-agy-native-cli-operation` | J-15 / `RT-agy-acp-provider` step 6 (`Hydrogen -> Argon -> Hydrogen` hosted MCP + `#1889` native file tools + structured error codes) | Ada | Feature Tour | **Pass** | `sess-6f0d7f96cc0d11a4` (ACP `ed918dae-e878-482d-95c2-46e6142b2ca6`): default `gemini-4-neon`/`high`/`auto`, 3x `compozy__workspace_info` across Hydrogen->Argon->Hydrogen with 0 `hosted_mcp_nonce_invalid`; `sess-31d991eea6e2afc8`: 6 individual mid-turn native file/terminal `tool_call`/`tool_result` pairs + `QA_TERMINAL` (`client_calls_count: 0`); `sess-93b407e3d3bd1da6` & flag-bearing `agy`: `native_tool_policy_unsupported`; Hydrogen `max`: `reasoning_effort_unsupported` (`rc=71`) preserving `xhigh` | |
| 2 | `CH-agy-native-cli-operation` | J-15 / `RT-agy-acp-provider` steps 7–8 (`missing_cli`, no `.par` fallback, standalone `needs_login` isolation, `#1883` login refresh / repair) | Ada | Feature Tour | **Pass** | `missing_cli` / `provider_cli_missing` on `--local` & `--remote` with `agy_acp_server.par` never executed (`par_never_executed=true`); clean HOME `needs_login` / `provider_not_authenticated` & dual token stores untouched (`sha256` identical); `#1883` `provider auth login agy --timeout 30s` reused operator login with immediate `authenticated` + `last_probe_at`. (Clean-HOME interactive Google consent was previously closed in `2026-11-17` Round 2) | |
| 3 | `CH-agy-native-cli-operation` | J-15 / `RT-provider-full-access-mode` (`agy` `auto` precedence + `#1618` retired `permissions.provider_full_access` upgrade) | Ada | Feature Tour | **Pass** | Under `approve-all`, `agy` selected `mode=auto` (never `yolo`); `agy-mode-narrow` (`sess-6d6879957cfbf2a4`) kept `accept-edits`; explicit `auto`/`yolo` under `approve-reads` refused. `#1618`: upgraded global/workspace configs (`false` with comments & `true`), exact `.retired-keys.bak`, single `WARN config.retired_keys_removed`, restart idempotent; RO pre-registered workspace `.compozy/config.toml` started safely (`1618-ro-ws-resolve.json`; `chmod 0555` on `$COMPOZY_HOME` nuance noted in Learnings) | |
| 4 | `CH-agy-native-runtime-picker` | J-17 & J-22 / `ET-web-runtime-selector-minimal-slider` & `MS-web-provider-auth-gate` (Playwright `settings` & `session-provider-override`, real-daemon Web Settings & RuntimeSelector check) | Sol | Feature Tour | **Pass** | Playwright E2E `settings.spec.ts -g "Antigravity"` (4.3s) & `session-provider-override.spec.ts -g "exact AGY catalog model"` (7.6s) passed. Live Chromium vs `:37835`: Settings `Google Antigravity` (`Ready · 9 models`), `oauth-personal`, `agy Available`, 0 credential slots, 375px `overflow<=1px`; RuntimeSelector Hydrogen (4 stops, `End -> Extra high`) vs Argon (3 stops, `End -> High`, 0 `Max`); live UI session `sess-e363ed9db99c6ee7` submitted Hydrogen/xhigh, persisted across Web/API/CLI (`LIVE_WEB_QA_OK`, `data-kind="agy"`) | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-agy-native-cli-operation — Ada — model catalog, hosted MCP switching, `#1889`, and refusals (Row 1)

- Baseline discovery against native `agy` 1.5.13 (`provider inspect agy -o json`, `provider auth status agy --remote -o json`, `provider models list agy --refresh --all -o json`) returned `source=auth_acp_method`, `executable=agy`, `presence=present`, `state=authenticated`, and 9 logical models (`claude-opus-5-5`, `claude-sonnet-5-5`, `gemini-3.8-flash`, `gemini-4-argon` `[low,medium,high]`, `gemini-4-hydrogen` `[low,medium,high,xhigh]`, `gemini-4-neon` `[low,medium,high,xhigh]`, `gemini-pro-orchestrator-g1-pro`, `gemini-pro-orchestrator-neon`, `gpt-oss-120b-medium`), all `available_live` with `reasoning_apply=acp_option` and zero effort-suffixed duplicates.
- Session `sess-6f0d7f96cc0d11a4` (`agy-example`, no initial `--model`) started at provider default `gemini-4-neon` (`high`) and ACP `mode=auto` under ACP session `ed918dae-e878-482d-95c2-46e6142b2ca6`. Sequential prompts switching `gemini-4-hydrogen` (`xhigh`) → `gemini-4-argon` (`high`) → `gemini-4-hydrogen` (`xhigh`) all executed `compozy__workspace_info` (`ws_76bb79e444dc0d19`) under the **same** logical and ACP session IDs with zero `hosted_mcp_nonce_invalid` entries in `daemon.log`.
- `#1889` (`sess-31d991eea6e2afc8`): read `qa-file-tools/{alpha.txt,beta.txt}`, edited both on disk (`ALPHA_EDITED_101`, `BETA_EDITED_202`), created `gamma.txt` (`GAMMA_CREATED_303`), and ran `printf QA_TERMINAL` (`exit_code: 0`, output `QA_TERMINAL`). `s2-events.json` captured 6 individual mid-turn `tool_call`/`tool_result` pairs (`04:58:03Z–04:58:22Z`) before the final `agent_message` (`04:58:33Z`), with `client_calls_count: 0`.
- Refusals: `sess-93b407e3d3bd1da6` (`agy-narrow`, `approve-reads`) and a flag-bearing custom command (`agy --disable-telemetry acp` at `approve-reads`) both refused pre-launch with `code=native_tool_policy_unsupported` (`rc=78`) and left `should_not_exist.txt` absent. Prompting `sess-6f0d7f96cc0d11a4` with `gemini-4-hydrogen --reasoning-effort max` returned `code=reasoning_effort_unsupported` (`rc=71`, `valid_choices=["high","low","medium","xhigh"]`) while retaining `xhigh`.

### CH-agy-native-cli-operation — Ada — missing CLI, logged-out isolation, and `#1883` reuse (Row 2)

- With `agy` removed from `PATH` and an instrumented `agy_acp_server.par` present, both `--local` and `--remote` probes returned `state=missing_cli`, `code=provider_cli_missing`, `executable=agy`, `recommended_action=install_cli`, and `par_never_executed=true`.
- With a clean `HOME` seeded only with dummy sentinels in `.gemini/antigravity/sentinel.txt` and `.antigravity/sentinel.txt`, `--local` and `--remote` returned `state=needs_login`, `code=provider_not_authenticated`, `recommended_action=login`; session prompt start failed with `provider_not_authenticated`; both sentinels matched their original `sha256` hashes afterward.
- `#1883`: `compozy provider auth login agy --timeout 30s -o json` reused the operator login without a second consent flow, and `provider list -o json` immediately reported `authenticated` with a fresh `last_probe_at`.

### CH-agy-native-cli-operation — Ada — full access precedence & `#1618` config upgrade (Row 3)

- `approve-all` negotiated `agy` `mode=auto` (no `yolo` injected). Agent `agy-mode-narrow` (`--acp-option mode=accept-edits`, `sess-6d6879957cfbf2a4`) preserved `accept-edits` over workspace `approve-all`; explicit `mode=auto` or `mode=yolo` under `approve-reads` was rejected pre-launch.
- `#1618`: across disposable homes (`provider_full_access = false` with comments, `provider_full_access = true`, workspace `.compozy/config.toml`), daemon startup preserved comments, stripped only the retired line after creating exact `.retired-keys.bak` backups, emitted a single `WARN config.retired_keys_removed` per file, and made zero modifications or log noise on restart. Pre-registered workspace `.compozy` (`0555`) + `config.toml` (`0444`) started safely and remained untouched (`1618-ro-ws-resolve.json`).

### CH-agy-native-runtime-picker — Sol — deterministic E2E & live Chromium browser walk (Row 4)

- Deterministic Playwright specs passed after symlinking local Playwright 1.64.1 browser revisions in temporary `/tmp/pw-browsers` (`chromium-1247 -> chromium-1248`):
  - `web/e2e/__tests__/settings.spec.ts -g "Antigravity"` (4.3s)
  - `web/e2e/__tests__/session-provider-override.spec.ts -g "exact AGY catalog model"` (7.6s)
- Live Chromium against `http://127.0.0.1:37835` verified:
  - Settings → Providers: `Google Antigravity` (`Ready · 9 models`), keyboard `Enter` opens inspector, `Configure` displays `oauth-personal` and `agy Available` with `0` credential slots / `0` secret inputs, and `375px` viewport passes `scrollWidth - clientWidth <= 1` (`live-agy-settings-card.png`, `live-agy-settings-editor.png`, `live-agy-settings-375px.png`).
  - RuntimeSelector & submission (`sess-e363ed9db99c6ee7`): `gemini-4-hydrogen` exposes `aria-valuemax="3"` (`End -> Extra high`) with `0` duplicate `thought_level` controls and `0` advanced toggles (`live-agy-selector-hydrogen.png`); switching to `gemini-4-argon` narrows slider to `aria-valuemax="2"` (`End -> High`, zero `Max`/`Extra high`, `live-agy-selector-argon.png`); submitting Hydrogen/`xhigh` returned HTTP 200, bound `agy`/`gemini-4-hydrogen`/`xhigh`, rendered `data-kind="agy"`, persisted `LIVE_WEB_QA_OK` in transcript, and matched across Web, `/api/...` (`live-web-api-session.json`), and CLI `session status` (`live-web-cli-session.json`).

## What Was Fixed

- Pre-QA test alignment carried into this run: `internal/api/core/providers_agy_test.go` (`ProviderID` assertion updated for `agy_acp_server.par` / `agy` native command resolution after rebase).

## Paper Cuts / Environment Findings

- **`#1618` read-only `$COMPOZY_HOME` caveat:** setting `chmod 0555` on `$COMPOZY_HOME` itself causes daemon startup to fail at `mkdir gateway` because `$COMPOZY_HOME` also holds mutable daemon runtime directories (`gateway/`, `compozyd.sock`, `data/`). Placing a read-only `.compozy/config.toml` (`chmod 0555 .compozy && chmod 0444 .compozy/config.toml`) inside a registered workspace starts cleanly and leaves `config.toml` untouched without `.bak`.
- **`make web-build` worktree sync:** initial build hit missing `tsrs` because `node_modules` had not yet been synced after rebasing onto `origin/main`; after installing workspace deps (`tsrs 0.25.0-ts7.1.0-beta`), `make web-build` (`5/5` Turbo tasks), `packages/ui` `tsrs --noEmit`, and `web` `typecheck:raw` all pass.
- **Playwright revision mismatch:** container caches Chromium `1247` / `ffmpeg-1011` in `/ms-playwright`, while `@playwright/test@1.64.1` requests `1248` / `1013` unless pointed via temporary symlinked `PLAYWRIGHT_BROWSERS_PATH`.

## Runtime Errors Observed

- Initial UI automation selector checks inside `scratch/live_web_qa.mjs` hit test-script mismatches (`[data-testid="session-window-..."]` instead of `[data-slot="os-window-surface"][data-app="session"][data-instance-key="..."]`, and `waitForResponse` vs `waitForRequest` method lookup) before converging; no product errors occurred on the corrected walkthrough.

## Human Verifications Needed

- None for the targeted AGY / Web UI / `#1618` scope. *(Clean-HOME interactive Google OAuth consent was previously closed in [`2026-11-17-agy-native-acp-consented.md`](./2026-11-17-agy-native-acp-consented.md) Round 2.)*

## Decisions for a Human

- Decide whether to accept the `chmod 0555` workspace `.compozy/config.toml` proof as sufficient for `#1618`'s read-only config invariant (given `$COMPOZY_HOME` requires write access for runtime state) or isolate a dedicated `$COMPOZY_CONFIG_DIR` test case.

## Verification and Evidence Closure

- **Post-rebase Go targeted verification (`umask 0022`):** `go test -count=1 ./internal/api/core ./internal/session ./internal/acp ./internal/api/httpapi ./internal/modelcatalog ./internal/config` — **all 6 packages PASS**.
- **Post-rebase Web unit verification:** `provider-detail-dialog.test.tsx` (20 tests), `use-provider-detail-body.test.tsx` (4 tests), `to-runtime-selector-options.test.ts` (20 tests), `runtime-selector.test.tsx` (128 tests) — **4 files, 172 tests PASS**.
- **Strict QA audit (`audit-qa-evidence.py --strict --explain`):** **PASS** (`Blockers: 0`, `Warnings: 0`).
- **Teardown & hygiene:** `teardown-qa-env.py` reported `clean: true` (`stopped_daemon_pid: 1510197`, `survivors: []`), disposable `#1618` daemon PID `1520135` and `/tmp/pw-browsers` cleaned up, and operator `compozy provider auth status agy --local -o json` remained `authenticated`.
- **Retained captures in repo:** `docs/qa/evidence/2026-11-21-agy-native-acp-open-qa/live-agy-{settings-card,settings-editor,settings-375px,selector-hydrogen,selector-argon,session-submitted}.png` (lab receipts also retained under `/home/vscode/dev/qa-labs/compozy-agy-native-open-qa-20261121-045056-325891-lab/qa-artifacts/qa/`).

## Learnings

- Inside a Compozy-managed agent session, inherited `COMPOZY_*` env vars override sourced lab configs unless explicitly unset (`for v in $(env | awk -F= '/^COMPOZY_/{print $1}'); do unset "$v"; done`) before every CLI invocation against an isolated lab daemon.
- Always grep `ps -ef` for stray disposable test daemons started outside `bootstrap-manifest.json` (`daemon.pid`) before declaring teardown complete.

## Final Status

**PASS (targeted):** all four targeted rows (`RT-agy-acp-provider` steps 6–8 / `#1889` / `#1883` / refusal codes, `RT-provider-full-access-mode` `agy` `auto` & `#1618`, `ET-web-runtime-selector-minimal-slider`, and `MS-web-provider-auth-gate` AGY Settings / Playwright E2E) passed with independent CLI, HTTP API, filesystem, Playwright E2E, and live Chromium evidence (`Blockers: 0`, `Warnings: 0`, `clean: true`).

---
id: RT-agy-acp-provider
area: RT
title: Google Antigravity explicit login, remote probe, and first prompt
persona: Ada
journey: J-15
expected: With only the official agy_acp_server on PATH and no [providers.agy] block, explicit local login authenticates, a remote probe reports readiness, and the first prompt persists under agy with the server-default model. Logged-out or missing-server states give actionable guidance and never trigger automatic login.
entry_points: compozy provider auth login agy; compozy provider auth status agy --remote; compozy session new --agent agy-example
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/agy_acp_entry_integration_test.go
last_report: docs/qa/reports/2026-10-19-agy-reasoning-selector.md
overlaps: MS-web-provider-auth-gate; RT-provider-full-access-mode; ET-048
---

story: As an operator with a Google Antigravity account I sign in once explicitly, verify readiness remotely, and run ordinary sessions without surrendering secrets or re-consenting.

## Explicit login to first prompt

1. Install Antigravity's official ACP server for the OS (`agy_acp_server.par` on Linux/macOS, `agy_acp_server.exe` on Windows) on PATH. Do not install or substitute the interactive `agy` CLI. Confirm no `[providers.agy]` block exists.
2. Run `compozy provider inspect agy -o json`: `source` is `auth_acp_method`, `presence` is `present`, and the executable is the server basename only.
3. With the provider logged out, run `compozy provider auth status agy -o json`: `state` is `needs_login`. No browser or terminal prompt appears.
4. Run `compozy provider auth login agy --timeout 5m -o json`: exit 0, `state` is `authenticated`. Repeat the login: no second consent request appears.
5. Run `compozy provider auth status agy --remote -o json`: `state` is `authenticated`.
6. Create a session with an `agy-example` agent (`provider: agy`, no model) and send one prompt. Session reads identify provider `agy` and the exact accepted model; the server-default model is used, never a fabricated default.
7. Remove or rename the server binary and repeat step 3: `state` is `missing_cli` with an `install_cli` recommendation naming only the basename.

## Repair without restart

1. With the provider logged out, attempt session creation and observe the authentication-required diagnostic.
2. Complete explicit login, run the remote probe, and immediately retry session creation without restarting the daemon. The session starts; a failed or canceled probe must not clear the failure.

## Removed daemon launch directory

1. Start the daemon from a disposable directory, then remove that directory.
2. Read `/providers` and a non-ACP provider detail: both remain available without a working-directory 500.
3. Probe native-command and ACP providers: both use the temporary directory when the daemon working directory cannot be resolved; ACP still receives a non-empty `Cwd`.
4. With an intact launch directory, probes continue to use that directory instead of the fallback.

Automated coverage: `TestProviderHandlersWithoutWorkingDirectory` in
`internal/api/core/providers_agy_test.go` exercises all four HTTP handler calls
after actually removing the working directory in an isolated subprocess, with
provider command/ACP boundaries stubbed. `TestProviderProbeEnvCarriesWorkingDirectory`
pins the successful-resolution path. This is not a new live-provider walkthrough.

## Model discovery and exact selection

1. Run `compozy provider models list agy --refresh --all -o json`: the result contains exactly the IDs the effective server/account advertises (verified 1.4.0 shape: the 2 bare IDs `gemini-4-argon`, `gemini-3.8-flash` plus a `thinking_level` option; the superseded 1.3.0 suffixed IDs such as `gemini-3.8-flash-high` are no longer advertised and prompt as `model_unavailable`), published under `provider_live:agy` with `reasoning_apply: acp_option`, the advertised `reasoning_efforts`, and their `default_reasoning_effort`. Unobserved pricing, context, and speed stay unknown. No Gemini CLI/API model is substituted.
2. Run the same list with the provider logged out: the source reports `auth_required`, keeps only that context's prior rows as stale, and never opens a browser. A different workspace, profile, or home cannot reuse those rows.
3. Run `compozy provider models list agy` with `models.discovery.enabled = false`: no peer call is made and no synthetic rows are published.
4. Prompt with one currently advertised exact ID and confirm the persisted runtime/event model matches exactly; prompt with an unavailable ID and confirm `model_unavailable` instead of a silent default.
5. In the session model picker, select an Agy model and choose Low on the reasoning slider. Submit a prompt; its runtime snapshot and effective runtime both retain `reasoning_effort: low`, and the acknowledged ACP `thinking_level` reads `low`. Choosing another supported level keeps the same exact model ID. An explicit `providers.agy.models.reasoning.apply = "none"` still disables adjustable effort.
6. With hosted MCP connected, switch Flash → Argon → Flash through subsequent prompts and invoke a read-only native tool after each switch. All three calls succeed under the same logical and ACP session IDs; a replaced proxy must not fail with `hosted_mcp_nonce_invalid`. A concurrent bind or a reconnect outside the configured post-release window remains rejected.

The exact-ID, stale/context-isolation, disabled-discovery, and unavailable-ID checks above passed against the real 1.4.0 server in the consented 2026-10-17 run (see the 1.4.0 consented note); the `acp_option` projection and steps 5–6 were verified separately on 2026-10-19 as described below. The 1.4.0 shape is 2 bare IDs plus `thinking_level`. Unit source coverage (`internal/modelcatalog/live_sources_test.go`) and daemon context coverage (`internal/daemon/model_catalog_test.go`) continue to pin the exact-ID, stale/context-isolation, disabled-discovery, and no-forbidden-message boundaries with the external ACP peer faked.

## Browser management and selection (E2E-003, deterministic peer)

2026-10-16 Task 04 browser check (no Google account; the `agy` overlay points
at the deterministic `acpmock` peer with one curated exact ID,
`qa-browser-model-alt`):
`web/e2e/__tests__/session-provider-override.spec.ts` → "operator selects an
exact AGY catalog model and keeps agy session attribution" passes. Observed:
after one daemon restart (builtin provider overlays apply to catalog/launch
only after restart), the catalog carries the exact peer-advertised ID under
`agy`; the existing runtime picker shows the `agy` rail and the exact model
row with the shared mark and no special filtering; the prompt binds
`{provider: agy, model: qa-browser-model-alt}`; the live session status reads
`agy` with the session provider mark; HTTP/UDS/CLI session reads agree.
Capture:
`.compozy/tasks/agy-acp/evidence/task_04/captures/session-provider-agy-attribution.png`.
The Settings half of E2E-003 (method round-trip, unverified state, keyboard,
narrow viewport) is in `MS-web-provider-auth-gate.md`. This receipt is
reusable by Task 05 unless later changes invalidate it.

## Task 05 live-probe note (2026-10-16, blocked at the time; superseded by the 1.4.0 consented run)

Implementation build (`compozy dev`), official server 1.3.0 on Linux x86_64
(registry advertises 1.4.0, not executed). Isolated lab, operator HOME
preserved, teardown clean. `provider inspect agy` matches the contract
(`auth_acp_method`, basename-only, `present`). Local and `--remote`
`provider auth status agy` both return `unknown` with
`Provider ACP probe failed: acp: cwd is required` — the server is never
launched (defect D-001, routed to Task 02/IT-005 wiring; IT-005 stubs the
probe). `provider models list agy --refresh --all` against real 1.3.0
returned exactly the six historical IDs, all `available_live`, with no
browser. Login, prompts, tools, `yolo`, cancellation, and restoration were
not attempted (no consenting account/browser in this environment).
`session/new` auth-gating remains unverified. Full receipts:
`.compozy/tasks/agy-acp/evidence/task_05/`; report:
`docs/qa/reports/2026-10-16-agy-acp.md`.

## Post-05 repair note (2026-10-16, blocked at the time; superseded by the 1.4.0 consented run)

D-001 is repaired in-tree (uncommitted): `providerAuthProbeEnv`
(`internal/cli/provider_credentials.go`), `runACPProviderLogin` /
`providerACPLoginTarget` (`internal/cli/provider.go`), and
`probeProviderACPAuth` / `providerProbeEnv` (`internal/api/core/providers.go`)
now carry the working directory, and `toSDKMCPServers(nil)` returns `[]`
(not null) so the probe `session/new` passes strict servers
(`internal/acp/authentication.go`, `client_protocol.go`). IT-005 regression
pins: `internal/cli/provider_agy_test.go` (CommandDir + real
`ProbeAuthentication` past cwd validation for probe and login) and
`internal/api/core/providers_agy_test.go` (non-empty existing-dir Cwd).
Post-fix live check on the operator home: local
`provider auth status agy` now returns `authenticated` (operator creds
present) — the `unknown`/`cwd is required` observation above is superseded
for the authenticated home only. Still never observed live: `needs_login` on
a clean logged-out home, explicit login, repair-without-restart, prompts,
tools, `yolo`, cancellation, restoration. Task 07 walks W-1..W-5 per the
Task 06 handoff in `.compozy/tasks/agy-acp/memory/MEMORY.md`.

## Task 07 W-1 residual note (2026-10-16, blocked at the time; superseded by the 1.4.0 consented run)

Fresh implementation-build binary + isolated lab daemon (own COMPOZY_HOME,
port, socket; operator HOME preserved, creds never copied; teardown clean —
`.compozy/tasks/agy-acp/evidence/task_07/`): local probe → `authenticated`
(`ACP session creation succeeded`); `--remote` → `authenticated` with
`last_probe_at` (CLI/HTTP parity on the fixed contract); `models list agy
--refresh --all` → the same six exact IDs, all `available_live`, fresh, no
browser. No new captures; E2E-003 receipts stand. W-2..W-5 not executed —
no clean provider home, no consenting account/browser/callback (B-01);
recorded as explicit blockers, not coverage. The live walk above remains
blocked-verify until run with a consenting account in an authorized clean
provider home.

## 1.4.0 re-baseline note (2026-10-17, blocked at the time; superseded by the 1.4.0 consented run)

Real 1.4.0 server (registry `antigravity-acp`, Linux x86_64; provenance +
sha256 in
`.compozy/tasks/agy-acp/evidence/task_07/observations-1.4.0.md`) with the
same implementation build in an isolated lab (operator HOME/daemon
untouched; teardown clean). Logged-out legs now observed live for the
first time: clean-home local and `--remote` probes → `needs_login` /
`provider_not_authenticated`, no browser; logged-out model refresh fails
closed with the source failure and no browser. Operator-home local and
`--remote` probes on 1.4.0 → `authenticated` with `last_probe_at`.
Contract change vs 1.3.0: model refresh yields **2 bare IDs**
(`gemini-4-argon`, `gemini-3.8-flash`) plus a `thinking_level`
(`low`/`medium`/`high`) option — the suffixed six-ID shape is gone on this
server, so exact-model steps written against 1.3.0 IDs need re-basing.
Install note: the 1.4.0 archive's `localharness_external` must accompany
the `.par` on PATH. Login→repair, first prompt, tools/`yolo`,
cancellation, and restoration remain unexecuted (no consenting
account/browser/callback, no approve-all consent). Full receipts:
`.compozy/tasks/agy-acp/evidence/task_07/` (`observations-1.4.0.md`).
Report: `docs/qa/reports/2026-10-16-agy-acp.md` (§1.4.0 re-baseline + W-2 logged-out legs).

## 1.4.0 consented run (2026-10-17, exercised verdicts only)

Same implementation build + real 1.4.0 server in a consented isolated lab
(operator HOME/daemon untouched; teardown clean). Full receipts:
`.compozy/tasks/agy-acp/evidence/task_07/`
(`observations-1.4.0-consented.md`); report:
`docs/qa/reports/2026-10-16-agy-acp.md` (§1.4.0 consented run).

- Explicit login → repeat login: no second consent; `--remote` probe →
  `authenticated` with `last_probe_at`; session creation retry with no
  daemon restart starts. **Repair leg passes.**
- First prompt persists under `agy` with exact bare IDs (`AGY_READY` on
  `gemini-3.8-flash`, `EXACT_MODEL_OK` persisted exactly as
  `gemini-4-argon`); unavailable suffixed ID → `model_unavailable` with
  the valid choices, never a silent default. **E2E-001 passes.**
- Allowed write succeeds only via operator `allow-once` (deny-all
  auto-rejects, never pending — `session approve` cannot allow it);
  denied writes stay rejected with the target absent and no workaround;
  read-only hosted-MCP tools complete after operator authorization;
  `permissions: approve-all` (+ `mode=yolo`) runs with zero permission
  events and selects the provider yolo mode. **E2E-004 passes.**
- A truly-executing host `sleep 60` cancels (`canceled`, process gone)
  and the same session answers the next prompt; a memory token survives
  an isolated daemon restart and is recalled exactly with matching
  runtime IDs. **E2E-005 passes.**
- Owner dispositions (2026-10-17, recorded): F-140-1 adopted the 1.4.0 shape — spec, _dx, _tests, site docs, and skill references re-based to 2 bare IDs + `thinking_level`, with 1.3.0 six-ID material labeled superseded where retained. O-140-C accepted as an intermittent `compozy-hosted-tools` bind flake (reads succeed on retry); no product change, no bug filed, host tooling untouched by owner direction.

## Peer-backed integration evidence

`TestAgyACPBuiltInEntry` (`internal/daemon/agy_acp_entry_integration_test.go`, `integration` lane) walks built-in discovery, real `acp.Authenticate`, real `acp.ProbeAuthentication`, and a real manager Create/Prompt against a raw-JSON-RPC peer through a `agy_acp_server.par` PATH shim. The peer replaces only Google's external server: it records exactly one `oauth-personal` authenticate, no prompt during login or probe, and a persisted prompt reply. It is not Google-account evidence; the live walk above has since passed against the real server in the consented 1.4.0 run.

Provider probe recovery: remove the daemon launch directory after startup, then list providers
and probe native and ACP authentication. The endpoints remain available and probes run from the
temporary directory, including on systems that return a stale path from `Getwd()`. The owning
`TestProviderHandlersWithoutWorkingDirectory` subprocess regression verifies this boundary.


Review remediation 2026-10-18: a successful ACP login remains authenticated when
the consent timeout expires before the subsequent status probe. The probe uses
the caller context; the timeout still bounds the consent phase. Verify the built-in
server executable as `agy_acp_server.exe` on Windows and `agy_acp_server.par` on
Unix, including CLI missing-command guidance, provider inventory, and Settings.

## Reasoning selector regression — 2026-10-19

The live catalog exposed both bare model IDs and their `thinking_level` values, but reported
`reasoning_apply: none` and omitted `reasoning_efforts`, preventing the session slider from selecting
them. The built-in now enables `acp_option`; explicit `none` overrides remain authoritative.
`TestDaemonModelCatalogWiring/Should publish exact Agy models with context isolation` reproduced the
failure before the fix and passes with model-specific Low/Medium/High, the live High default, and
the opt-out preserved. The complete configuration/model-catalog packages and daemon catalog wiring
suite pass with `-race`. The daemon-served browser case passes after selecting Low and checking the
submitted and effective runtime with a deterministic peer. A separate isolated daemon running the
rebuilt binary also preserves the real server's Low/Medium/High and High default in matching CLI and
HTTP catalog projections. No account-backed prompt was submitted; the previous consented
Google-account walkthrough remains historical, not evidence of a new live-account effort switch.

## Hosted MCP model-switch regression — 2026-10-19

Agy 1.4.1 rebuilds its agent and stdio MCP proxy at the next turn after a model change. The consumed
launch nonce previously rejected that legitimate replacement. The authenticated current proxy's
release now opens one bounded reconnect window; concurrent binds, stale releases, invalid peers,
expired windows and credentials from a replaced/stopped launch remain refused. An isolated
account-backed CLI/API walk completes Flash → Argon → Flash with `compozy__workspace_info` after
each bind and the same ACP session ID. See `docs/qa/reports/2026-10-19-hosted-mcp-reconnect.md`.
This closes only the reconnect slice, not every historical scenario leg. The original operator
session is left untouched at the operator's request.

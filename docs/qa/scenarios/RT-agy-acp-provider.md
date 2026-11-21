---
id: RT-agy-acp-provider
area: RT
title: Google Antigravity login, prompts, and file tools
persona: Ada
journey: J-15
expected: With the official Antigravity CLI on PATH (command agy, argument acp), no [providers.agy] block, and operator-selected approve-all for execution, explicit local login reuses the CLI's shared sign-in and updates the daemon's sign-in verdict, a remote probe reports readiness, and the first prompt persists under agy with an advertised logical model and its own thought_level effort (Hydrogen xhigh on the verified fixture). Narrower host policies refuse native execution before launch without widening permissions. A standalone-only installation is missing_cli with no fallback, logged-out or missing-CLI states give actionable guidance, and nothing triggers automatic login. Under approve-all, provider-native file tools complete individually during the turn; gateway-routed edits in Ask before changes wait for operator approval.
entry_points: compozy provider auth login agy; compozy provider list; compozy provider auth status agy --remote; compozy session new --agent agy-example
qa_status: untested
bug_ids: "#1881"
fix_status: fixed
retest_status: pending
fix_commits:
evidence: internal/daemon/agy_acp_entry_integration_test.go
last_report: docs/qa/reports/2026-11-17-agy-native-acp-consented.md
overlaps: MS-web-provider-auth-gate; RT-provider-full-access-mode; ET-048
---

story: As an operator with a Google Antigravity account I sign in once explicitly, verify readiness in the provider list and picker, and run ordinary sessions without re-consenting.

## Explicit login to first prompt

1. Install Antigravity CLI using Google's installation instructions and confirm `agy acp --help` succeeds. Do not install or substitute the standalone `agy_acp_server` archive. Confirm no `[providers.agy]` block exists.
2. Run `compozy provider inspect agy -o json`: `source` is `auth_acp_method`, `presence` is `present`, and the executable basename is `agy` only.
3. With the provider logged out, run `compozy provider auth status agy --remote -o json` (and `compozy provider auth status agy -o json`): `state` is `needs_login`. No browser or terminal prompt appears. Run `compozy provider models list agy --refresh --all -o json` and confirm that the failed live source names `agy` in its login hint.
4. Run `compozy provider auth login agy --timeout 5m -o json`: exit 0, `state` is `authenticated`. Repeat the login: no second consent request appears. For headless/SSH setup, Google's `agy acp --login` is the operator alternative; it is never a background probe.
5. Run `compozy provider list -o json` directly after login (`auth_status.state` is `authenticated`; once live catalog refresh completes, the composer picker shows the advertised models and Settings shows the provider as ready) and verify `compozy provider auth status agy --remote -o json` reports `state: authenticated`.
6. In the operator-owned lab, explicitly select `approve-all` and create a session with an `agy-example` agent (`provider: agy`, no model). Send one prompt. Session reads identify provider `agy` and the exact accepted logical model; the provider-default model is used, never a fabricated default. A narrower host policy is refused before launch, not changed automatically.
7. Remove or rename the `agy` executable and repeat step 3: `state` is `missing_cli` with an `install_cli` recommendation naming only `agy`. Leaving only a standalone `.par` on PATH keeps `missing_cli`; it is never launched as a fallback.
8. Keep the standalone server's login but remove the CLI sign-in: `state` is `needs_login`, both token stores remain untouched, and only explicit CLI login repairs it.

## File tool lifecycle and edit approval

1. In a throwaway workspace with Antigravity signed in, prepare two text files with different contents. Start an `agy` session in Full access (`approve-all`, ACP mode `yolo`). Ask it to read and edit both files and create a third using file tools. Ask it to run `printf QA_TERMINAL` with its terminal tool after the file operations.
2. Capture `compozy session prompt <id> <message> -o jsonl` and read back `compozy session events <id> -o json`. Match each provider-native file `tool_call` to its own successful `tool_result` by tool call ID. Results should arrive during the turn, before subsequent tool activity or the final reply, without a turn-end batch or `client_*` file calls. Confirm the edits and creation on disk. Verify the terminal call returns `QA_TERMINAL` with its own result.
3. Note on Ask before changes (`approve-reads`): built-in native `agy acp` rejects `approve-reads` pre-launch (`native_tool_policy_unsupported`); an explicitly gateway-routed agent with ACP client FS enabled preserves pending edit approval (`compozy session approve <id> --request-id <request-id> --decision allow-once`).

## Repair without restart

1. With the provider logged out, attempt session creation and observe the authentication-required diagnostic.
2. Complete explicit login and immediately retry session creation on the same daemon. The session starts. A failed or canceled probe leaves the cached failure in place.

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

1. Run `compozy provider models list agy --refresh --all -o json`: the result contains exactly the logical IDs the effective CLI/account advertises (verified 1.5.9 shape: 7 logical IDs including `gemini-4-argon` and `gemini-4-hydrogen`, with per-model `thought_level` options; the legacy `availableModels` list also carries 18 effort-suffixed transport aliases that must not become duplicate rows), published under `provider_live:agy` with `reasoning_apply: acp_option`, the advertised `reasoning_efforts`, and their `default_reasoning_effort`. Argon offers `low`/`medium`/`high`; selecting Hydrogen re-announces four levels including `xhigh`; `max` is rejected as unsupported. Unobserved pricing, context, and speed stay unknown. No Gemini CLI/API model is substituted.
2. Run the same list with the provider logged out: the source reports `auth_required`, keeps only that context's prior rows as stale, and never opens a browser. A different workspace, profile, or home cannot reuse those rows.
3. Run `compozy provider models list agy` with `models.discovery.enabled = false`: no peer call is made and no synthetic rows are published.
4. Prompt with one currently advertised exact ID and confirm the persisted runtime/event model matches exactly; prompt with an unavailable ID and confirm `model_unavailable` instead of a silent default.
5. In the session model picker, select an Agy model and choose Low on the reasoning slider. Submit a prompt; its runtime snapshot and effective runtime both retain `reasoning_effort: low`, and the acknowledged ACP `thought_level` reads `low`. Choosing another supported level keeps the same exact model ID. Requesting Hydrogen `max` fails as `reasoning_effort_unsupported` before any prompt runs. An explicit `providers.agy.models.reasoning.apply = "none"` still disables adjustable effort.
6. With hosted MCP connected, switch Hydrogen → Argon → Hydrogen through subsequent prompts and invoke a read-only native tool after each switch. All three calls succeed under the same logical and ACP session IDs; a replaced proxy must not fail with `hosted_mcp_nonce_invalid`. A concurrent bind or a reconnect outside the configured post-release window remains rejected.

The exact-ID, stale/context-isolation, disabled-discovery, and unavailable-ID checks above passed against the real 1.4.0 server in the consented 2026-10-17 run (see the 1.4.0 consented note); the `acp_option` projection and steps 5–6 were verified separately on 2026-10-19 as described below. The 1.4.0 shape is 2 bare IDs plus `thinking_level`. Unit source coverage (`internal/modelcatalog/live_sources_test.go`) and daemon context coverage (`internal/daemon/model_catalog_test.go`) continue to pin the exact-ID, stale/context-isolation, disabled-discovery, and no-forbidden-message boundaries with the external ACP peer faked.

## Narrow-policy enforcement — 2026-11-16 (live rewalk failed; repaired only by a final rejection rewalk)

The first live narrow-policy rewalk of the native CLI **failed**. With `effective_permissions:
approve-reads`, session `sess-f7e19ce5a1b6858e` acknowledged `mode=plan`,
`model=gemini-4-hydrogen`, and `thought_level=xhigh`, then the provider's own `write_file` created
`agy-denied-plan-7431.txt` without a host `session/request_permission`. Evidence in the 2026-11-16
lab: `native-readonly-verdict.json`, `native-readonly-response.json`, and
`native-readonly-bound.json`. The CLI does not route that write through the host, so an advertised
`plan` mode is not an enforcement boundary, and the earlier read-only-mode claim was wrong.

Replacement contract (final rejection rewalk observed on `8d501bba2`):

1. Start a native `agy acp` session at `deny-all`, `approve-reads`, `approve-edits`, or `auto`:
   startup refuses before any prompt, no prompt text is sent, no tool call runs, and the session and
   transcript stay intact.
2. `approve-all` still starts and negotiates the advertised `auto` mode. The picker/auth browser
   journey uses that policy and is not a permission-enforcement test.
3. An authored standalone or custom command keeps its route and its own approval behavior.
4. The refusal is never a silent widening, and an agent is never advised to raise its own policy;
   the operator chooses a trusted runtime and policy for constrained work.

The final live rewalk returned HTTP 422 with `approve-reads` unchanged, runtime `unbound`, no
processes, and fresh target `agy-denied-before-launch-7434.txt` absent. The earlier failed targets
remained present. This is safe refusal, not native read-only support. The original failure record
is retained; its lab receipts and the final refusal receipt were lost in the subsequent environment
restart, so that historical run alone remains unauditable. The 2026-11-17 rewalk recorded a fresh
refusal receipt in the linked report.

## Native CLI re-baseline — 2026-11-16 (scoped walks observed; final audit blocked)

Consented re-walk 2026-11-17: native CLI 1.5.9 reused the existing sign-in, acknowledged
Hydrogen/xhigh, persisted the reply, and refused unsupported `max` without reducing effort.
The hosted workspace-info call and authorized write passed independent public-history/filesystem
checks. A fresh `approve-reads` session refused before launch, stayed unbound with no processes,
and left its forbidden target absent. Actual running work was observed before cancellation;
verified quiescence, empty explicit cleanup, and a usable follow-up were recorded.
Round 2 closed the remaining authentication leg: a separate clean HOME completed real
provider-owned Google consent through the native CLI's documented headless login, the product
probe reported `authenticated`, and `compozy provider auth login` reused the store without a
second consent. A second pending login was interrupted with `SIGINT`, stayed `needs_login`, and
left no process. Persistent receipts and the complete acceptance boundary are in the linked
2026-11-17 report. Scenario-level `qa_status` is reset to `untested` because steps 7–8
(`missing_cli`/no-fallback/dual-token-store) and step 6's Hydrogen → Argon → Hydrogen
hosted-MCP switch sequence remain unwalked on native CLI 1.5.9.

The built-in now launches the official Antigravity CLI as `agy acp`, so the 1.4.0 standalone-server
receipts above are historical evidence for a superseded runtime, not native-CLI acceptance. The
active steps above were re-based to CLI 1.5.9 (`analysis/01-evidence.md`): shared CLI login,
`oauth-personal`, logical model IDs with per-model `thought_level` (Hydrogen `xhigh`, unsupported
`max`), and `missing_cli` for a standalone-only installation. On upgrade, a session whose executable
route changed continues from its retained transcript through the existing replay path; that is not
native-history import, and neither Google token store is copied or deleted. An explicit command or
provider-private option override keeps its bytes and fails validation rather than being converted.

The owning lab observed native login reuse, Hydrogen/xhigh acknowledgement, unsupported-max
rejection, hosted `compozy__workspace_info`, an allowed write, cancellation with a usable follow-up,
explicit detached-task cleanup, headed Settings and keyboard picker operation, and clean-HOME
`needs_login` without consent. That historical run left a separately consented login/cancellation
unverified and lost its receipts in the environment restart; the 2026-11-17 round-2 addendum
closed the consent/cancellation gap with persistent receipts. Non-Linux execution remains an
operator exclusion. Peer tests still fake only the external CLI and never establish
Google-account acceptance.

## Preserved ambiguous mode on upgrade — 2026-11-16 (Q2; observed before evidence loss)

Released first-prompt and `CreateOpts` records can lack a selected runtime while the effective ACP
mode lives in both runtime meta and the immutable creation profile, so an automatic mode and an
explicit one are indistinguishable. The operator chose to preserve that ambiguous mode rather than
drop it:

1. From a released-build session that holds the old injected `mode=yolo` effective option and no
   selected-runtime intent, upgrade only the runtime.
2. Prompt the same logical session. If the native CLI does not offer that mode, the start fails with
   the existing unsupported-option repair guidance; the transcript, creation profile, and logical
   session stay unchanged, and no prompt runs with a remapped or dropped mode.
3. Confirm current intent through the existing runtime-set surface without a raw mode:
   `compozy session runtime set <id> --provider agy --model <advertised> --reasoning-effort <offered>`.
   Prompt again: the same logical session replays on a fresh native ACP session ID. That replay is not
   provider-private history import.
4. `compozy session runtime clear <id>` returns to the effective/agent default; it is not the step-3
   confirmation.
5. A session whose accepted route still matches an authored standalone command resumes on that route
   instead of replaying.

The genuine old-runtime session retained its options/binding/creation identity through HTTP 422
refusal and runtime-clear refusal. Complete runtime-set then permitted transcript replay under
the same logical ID into a new native ID, with the original continuity marker recovered.
Subsequent stop/resume retained that new native ID and logged successful `session_load`.
The full-history read-back comparison remained unfinished; raw receipts were lost on restart.
Owning cases remain IT-002 and E2E-002. No E2E pass, private-history import, or automatic deletion
of unknown-provenance mode is claimed.

Consented re-walk 2026-11-17: the genuine baseline `1141d864` conversation passed refusal,
runtime-clear refusal, explicit runtime confirmation, new-ID transcript replay, and subsequent
same-ID native restoration on `48aee1ed`. Structured comparisons kept all prior agent-message
events unchanged through refusal and present after replay, together with the logical identity
and creation time. The original marker was recovered after both replay and restoration.
Durable source receipts and `native-continuity-verdict.json` are linked from the current report.
This closes the targeted real-continuity leg; the separate round-2 consent addendum closes the
authentication leg.

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

Follow-up 2026-11-16 (peer fixture extended; E2E stays CI-owned and unexecuted here): the
AGY browser overlay now points at the fixture agent `browser-agy-native-agent`, whose `model`
option reannounces per-model `thought_level` sets through the acpmock declarative
`when_selected` contract. The same journey selects `gemini-4-hydrogen` with
`low`/`medium`/`high`/`xhigh` (keyboard End → `Extra high`, then submits and reads an
effective runtime of `xhigh`), switches to `gemini-4-argon` with three levels and no
`xhigh`/`max` or duplicate raw `thought_level` control, then returns to Hydrogen. The
2026-10-16 receipt above remains historical for the `qa-browser-model-alt` fixture and is
superseded for the native model leg; it was not re-run and is not claimed as new evidence.

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

`TestAgyACPBuiltInEntry` (`internal/daemon/agy_acp_entry_integration_test.go`, `integration` lane) walks built-in discovery, real `acp.Authenticate`, real `acp.ProbeAuthentication`, and a real manager Create/Prompt against a raw-JSON-RPC peer through an `agy` PATH shim whose recorded argv must be `acp`; no standalone `.par` is present. The peer replaces only Google's external runtime: it records exactly one `oauth-personal` authenticate, no prompt during login or probe, and a persisted prompt reply after the model/effort acknowledgement. It is not Google-account evidence. The 2026-10-17 live walk above passed against the real standalone server and is historical for this native re-baseline, not inherited CLI acceptance.

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
`TestDaemonModelCatalogWiring/Should publish native Agy models inside the effective command context` reproduced the
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

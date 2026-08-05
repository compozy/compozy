# QA Run Report — 2026-08-02 — Provider auth hard cut

- **Scope:** Safe provider inventory, prepared-runtime auth probing, write-only login configuration, CLI-only login, plus one-kickoff autonomous release collaboration as the adjacent runtime canary.
- **Cadence tier:** targeted
- **Build:** `v0.3.0-beta.3-6-g741d3563-dirty`; fresh `make build-go`, focused race suites, and affected full race suites passed · **Environment:** fresh isolated `devtool-oss-launch` lab with live daemon, HTTP, UDS, provider fixture, and Web.
- **Started:** 2026-08-02T20:20:02-03:00 · **Status:** Dora rewalk complete; Bruno one-kickoff canary pending
- **Invalidated preflight manifest:** `/home/pedronauck/dev/qa-labs/compozy-provider-auth-hard-cut-20260802-230742-932864-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Preflight teardown:** `/home/pedronauck/dev/qa-labs/compozy-provider-auth-hard-cut-20260802-230742-932864-lab/qa-artifacts/qa/teardown.json` (`"clean": true`; zero survivors)
- **Execution manifest:** `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/bootstrap-manifest.json` (`reused_lab: false`, `PLAYBOOK_REF=devtool-oss-launch`).
- **Execution isolation:** `COMPOZY_HOME=/tmp/compozyqa-5c85c0f2b17e/runtime`, HTTP `127.0.0.1:40279`, UDS `/tmp/compozyqa-5c85c0f2b17e/runtime/compozyd.sock`, provider home `/tmp/compozyqa-5c85c0f2b17e/provider`.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User / Runtime Administrator | desktop / wifi-fast / en-US | CH-provider-auth-surfaces-dora |
| Bruno | Operational Expert | desktop / wifi-fast / en-US | CH-helix-one-kickoff-bruno |

## Flows in Scope

- `J-administer-provider-auth` — configure, inspect, probe, and run provider authentication without exposing private command material (`../journeys/J-administer-provider-auth.md`).
- `J-one-kickoff-collaboration` — complete reviewed and disruption-aware project work after exactly one operator kickoff (`../journeys/J-one-kickoff-collaboration.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-provider-auth-surfaces-dora | J-administer-provider-auth / RT-025 | Dora | Feature Tour | Fixed | BUG-20260802-provider-login-secret-disclosure verified fixed across CLI/UDS, HTTP, Settings, and Web | working-tree |
| 2 | CH-provider-auth-surfaces-dora | J-administer-provider-auth / RT-026 | Dora | Feature Tour | Blocked (needs human verify) | Local prepared-runtime, HTTP/UDS, no-auth, missing-command, and restart branches pass; real Daytona backend unavailable | working-tree |
| 3 | CH-provider-auth-surfaces-dora | J-administer-provider-auth / RT-027 | Dora | Feature Tour | Fixed | CLI and native-tool writes are redacted; CLI-only login executes privately; no HTTP/UDS login route | working-tree |
| 4 | CH-helix-one-kickoff-bruno | J-one-kickoff-collaboration / RT-073 | Bruno | Feature Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Planning Coverage

- **Journeys:** both changed provider behavior and the adjacent autonomous runtime canary have mapped entry, actions, branches, side effects, abandonment, and true end states.
- **Functional:** public CLI/HTTP/UDS/Web/native read and mutation paths, exact terminal classifications, and one-kickoff runtime progression are in scope.
- **Experiential:** truthful safe-descriptor copy, actionable failures, refresh survival, and Web status clarity are in scope.
- **Edge/error/empty:** unknown provider, auth mode none, missing status command, private command with distinctive arguments/env/path, empty or stale inputs, absent HTTP/UDS login route, provider unreachability, and runtime stall are in scope.
- **Cross-cutting:** CLI/HTTP/UDS/Web parity and the provider/task/network/runtime canary are covered. Mobile, non-English locale, and full accessibility conformance are deliberately skipped because this change does not alter layout or localized copy; keyboard, focus, and 200% zoom remain lightweight Web checks.

## Session Debriefs

### CH-provider-auth-surfaces-dora — first walk stopped on disclosure; governed rewalk complete

- **Entry:** Dora registered the isolated Helix project, wrote the custom native-provider configuration through `compozy config set`, followed the returned restart instruction, and returned through the documented CLI and HTTP reads.
- **Observed:** the login mutation response and a fresh `config get` both returned `[redacted]`. The first provider detail read and the independent auth-probe read then exposed the test environment assignment as `login.executable` instead of returning the executable basename.
- **Goal reached:** no. The public confidentiality contract failed before the login action.
- **True end state:** not confirmed; `BUG-20260802-provider-login-secret-disclosure` was filed and RT-025/026/027 were set to `fail`.
- **Fidelity:** no source, database, or private config read was used to decide the verdict. Evidence came from the mutation response, a fresh CLI read, HTTP provider detail, and HTTP probe.
- **Rewalk entry:** after the root-cause fix and rebuild, Dora returned through CLI config, native `compozy__config_set`, provider inventory/detail, HTTP and UDS-backed probes, internal CLI login, Settings/Web, Doctor, and two daemon restarts.
- **Rewalk observed:** every public read exposed only `qa-login-runner`; the controlled login environment and arguments still executed and produced `authenticated`; `auth_mode=none`, unknown-provider, missing-status-command, and absent-login-route branches returned their exact public diagnostics. Web retained the safe projection after route reload.
- **Rewalk true end state:** RT-025 and RT-027 pass and `BUG-20260802-provider-login-secret-disclosure` is verified fixed. RT-026 is `blocked-verify` only for its explicit Daytona clause because the isolated lab has no configured Daytona backend or Daytona CLI; the local final-environment contract passed.
- **Rewalk fidelity:** the verdict uses only public CLI/tool, HTTP, UDS-backed CLI, Settings/Web, and controlled subprocess outputs. No private config or database read was used.

The fresh manifest, eight agents, eleven deterministic tasks, five channels, five knowledge files, deliverable matrix, collaboration minimums, and populated Helix charter agree with the validated `devtool-oss-launch` playbook. Bruno's canary has not started.

## What Was Fixed

- Leading `NAME=value` assignments are now parsed as private child environment, not as the executable. Executable resolution, argv construction, live execution, and the public descriptor all share the same parsed command identity.
- Canonical provider, runner, and CLI regressions prove execution semantics and confidentiality. The complete affected packages pass under `-race`; vet, zero-warning lint, Windows build, test-shape checks, formatting, and diff checks are green.
- Public proof covers CLI config, `compozy__config_set`, provider CLI/UDS, HTTP, Settings, Doctor, Web, internal login execution, restart survival, no-auth behavior, and error recovery.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- `BUG-20260802-provider-login-secret-disclosure`: provider inventory, detail, and probe exposed a login environment assignment through `login.executable`; verified fixed in the working tree after a full Dora replay.

## Human Verifications Needed

- RT-026 still needs a real configured Daytona backend to prove the auth-status command resolves and executes inside Daytona's final environment and working directory. The local prepared-runtime proof is not substituted for that backend-specific walk.

## Decisions for a Human

None identified yet.

## Learnings

- RT-025 and RT-027 had no journey, while RT-026 referenced a provider model-catalog canary that did not cover provider authentication. This run repairs that planning gap before execution.
- A same-reviewer Bridge contract finding arrived after bootstrap but before the first charter. The lab was torn down immediately and its evidence excluded; the provider walk requires a fresh binary, home, port, socket, and manifest after source freeze.

## Final Status

- **Exit gate (full automated suite):** pending; the workstream-wide final `make gate-full` remains deferred until the final mutation.
- **Issues by user impact:** pending
- **Coverage:** 1 / 2 journeys walked; two provider scenarios fixed/pass, one provider scenario blocked only on Daytona verification, and Bruno's canary pending.
- **Verdict:** pending — the disclosure is fixed and Dora's available public surfaces are complete, but no release-readiness claim is made before Bruno's one-kickoff canary, strict audit, final gate, and mandatory teardown complete.

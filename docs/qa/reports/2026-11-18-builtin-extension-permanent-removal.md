# QA Run Report — 2026-11-18 — built-in extension permanent removal and restore

- **Scope:** operator journey for `compozy extension remove --permanent`, `restore`, and `removed` on the bundled `spec-cycle` built-in across CLI, UDS/HTTP, the agent native-tool surface, and the web UI, plus the restart leg of `ET-bundled-extension-trust`.
- **Cadence tier:** targeted (E2E-001 to E2E-004 from the task graph; no charter file, a single lab pass).
- **Build:** branch `feat/builtin-ext-docs-qa` on base `a03609681`, with uncommitted task_11 and task_12 changes. The lab binary was built from this worktree. **Environment:** isolated lab `compozy-builtin-permanent-removal-20261118-190753-611849-lab`, `COMPOZY_HOME` under `/tmp/compozyqa-6d838056c456/runtime`, HTTP `127.0.0.1:59939`, UDS under the same runtime. The lab HOME is `/tmp/compozyqa-6d838056c456/home`. The operator daemon and `~/.compozy` were not touched.
- **Started:** 2026-11-18T19:08Z · **Status:** closed 2026-11-19. E2E-001 to E2E-004 pass. E2E-003 was walked in lab 2 after the operator's manual login in the lab HOME (D1, Session 3). Lab 2 was torn down with `clean: true`.
- **Verdict:** PASS for E2E-001 to E2E-004. The QA audit still reports blocker C14 (no local gate log). It needs an operator decision (D3).

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Vera (Policy Administrator) | seed `docs/qa/personas.md` | desktop CLI and browser, local network, en | E2E-001, E2E-002, E2E-003 (agent, Session 3), E2E-004 |

## Flows in Scope

- `J-extension-policy-admin` — the administrator removes a bundled built-in for good, lists it, and restores it (`../journeys/J-extension-policy-admin.md`).
- `ET-bundled-extension-trust` restart leg — a tombstoned built-in is not re-enrolled after a daemon restart.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | — (targeted lab run) | J-extension-policy-admin / ET-builtin-extension-permanent-removal (E2E-001) | Vera | CLI remove → restart → list → removed → restore | Pass | — | — |
| 2 | — | ET-builtin-extension-permanent-removal (E2E-002) | Vera | UDS delete → HTTP removed-builtins → HTTP restore | Pass (F1 doc drift) | F1 | — |
| 3 | — | ET-builtin-extension-permanent-removal (E2E-003) | Vera (through agent `qa-restore-agent`) | agent native tools + permanent-tool absence | Pass (Session 3) | D1 (resolved) | — |
| 4 | — | ET-builtin-extension-permanent-removal (E2E-004) | Vera | web: Remove… → permanent → Removed built-ins → restore → installed | Pass | — | — |
| 5 | — | ET-bundled-extension-trust (restart reconciliation, tombstone leg) | Vera | restart after permanent removal | Pass (tombstone leg only; trust walk not run) | — | — |

Status legend: `Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### E2E-001 — Vera (CLI)

- **Ran:** 2026-11-18, same lab session that started the daemon at 19:08:38Z (box respected: yes).
- **Steps and results:** `extension remove spec-cycle --permanent --global` printed "Removed spec-cycle permanently. It will not return on restart or update." (exit 0). `daemon stop` exited 0, `daemon start` came back on a new PID. `extension list` had no `spec-cycle` row (0 hits). `extension removed -o json` listed it. `extension restore spec-cycle --global` printed "Restored spec-cycle 0.6.0 (bundled, enabled)." (exit 0). The final `extension list -o json` showed `source: bundled`, `enabled: true`, `state: active`.
- **Findings:** F2 (the tombstone's `removed_by` is `human:local-user`, not the `operator` shown in the DX example). F3 (the second permanent removal error text differs from the DX example; the JSON error has no code field).
- **Scenarios settled:** E2E-001 → pass.

### E2E-002 — Vera (UDS and HTTP)

- **Ran:** 2026-11-18, same lab session (box respected: yes).
- **Steps and results:** UDS `DELETE /api/extensions/spec-cycle?permanent=true` → 200 with `{"extension":{"name":"spec-cycle","path":"…","status":"removed","permanent":true,"tombstoned_at":"2026-11-18T19:09:29Z"}}`. HTTP `GET /api/extensions/removed-builtins` → 200 `{"removed":[{"name":"spec-cycle",…}]}`. HTTP `POST /api/extensions/spec-cycle/restore` → 200 `{"name":"spec-cycle","status":"restored","source":"bundled","version":"0.6.0","enabled":true,…}`.
- **Findings:** F1. The DX example shows a flat object. The daemon keeps the pre-existing `{"extension": …}` envelope (`ExtensionRemoveResponse`), and it omits empty `data_path`, `quarantine_path`, and `warnings`, as `omitempty` requires. Behavior matches the public contract; the DX example is wrong.
- **Scenarios settled:** E2E-002 → pass (F1 recorded).

### E2E-003 — Vera (agent journey)

- **Ran:** 2026-11-18, same lab session (box respected: yes).
- **Attempted:** tombstone `spec-cycle` (CLI, exit 0, baseline `removed` captured); create a session with the lab-scoped agent `qa-restore-agent` (`permissions: approve-all`, provider `claude`, model `claude-haiku-5-5`); send the prompt that calls `compozy__extensions_removed_list`, `compozy__extensions_restore`, then `compozy__extensions_removed_list`, then `compozy__tool_search` for "permanent".
- **Result:** session creation passed (`effective_permissions: approve-all`). The prompt failed twice with `provider_not_authenticated` (exit 70). The lab daemon runs with the lab HOME, and `claude auth status` under that HOME reports `loggedIn: false`. The CLI probe `compozy provider auth status claude` reports `authenticated` (`probe_source: daemon`, `home_policy: operator`), but the live pre-start check still fails.
- **Substitute attempted:** `compozy mcp serve` with `--workspace` set to the lab path, over stdio. It disconnects before `initialize` with `session_id=""` and "initialized before initialize", so it needs an agent session identity. No tool-level evidence was obtained through it.
- **Not verified:** a live agent calling `compozy__extensions_restore` or `compozy__extensions_removed_list`; the result comparison with `extension removed -o json`; and the observation that no permanent-removal tool is offered to a session. The catalog-level evidence exists in the native tool catalog and policy tests (`UT-016`), not in this walk.
- **Scenarios settled:** E2E-003 → blocked (D1).

### E2E-004 — Vera (web)

- **Ran:** 2026-11-18 (box respected: yes).
- **Command:** `bunx turbo run build --filter=compozy-web` (exit 0, produced `web/dist`), then `COMPOZY_TEST_DAEMON_BIN=$LAB_BIN/compozy COMPOZY_E2E_CLI_BIN=$LAB_BIN/compozy bun run --cwd web test:e2e:daemon-served:raw -- e2e/__tests__/builtin-extension-removal.spec.ts` → `1 passed (30.8s)`. The spec ran against Playwright's own daemon with a temporary home, not against the lab daemon.
- **Steps covered:** open `/marketplace/installed` → spec-cycle detail → "Actions for spec-cycle" → "Remove…" → "Remove permanently" → type-to-confirm → navigation to `/marketplace/installed` → CLI `extension removed` contains spec-cycle, CLI `extension list` omits it → "Removed built-ins" section → "Restore spec-cycle" → row gone → installed list shows spec-cycle, CLI shows `bundled` and `enabled: true`.
- **Scenarios settled:** E2E-004 → pass.

### ET-bundled-extension-trust — restart leg

- **Ran:** as part of E2E-001 (daemon stop and start, with spec-cycle tombstoned).
- **Result:** after the restart, `spec-cycle` stayed absent from the list, and `removed` still listed it. The tombstone gates boot enrollment. The trust scenario's other legs (provenance, checksum evidence, policy) were not walked in this run and keep their current status.

### Session 2 — lab re-run for E2E-003 and the D2 examples (2026-11-18, 21:00Z)

- **Lab:** `/tmp/compozy-qa-labs/compozy-builtin-permanent-removal-e2e003-20261118-210036-216767-lab`. Manifest: `qa-artifacts/qa/bootstrap-manifest.json`. Binary built from this worktree at `e8afa77ce`. `COMPOZY_HOME=/tmp/compozyqa-3d96cbf2fc47/runtime`; isolated HOME `/tmp/compozyqa-3d96cbf2fc47/provider`; HTTP `127.0.0.1:55935`. Every command goes through `bin/qa-compozy`, which clears inherited `COMPOZY_*` markers. The daemon PID is registered in `qa-artifacts/qa/pids/daemon.pid`. The operator daemon and `~/.compozy` were not touched.
- **Provider check (manifest step 2):** no provider authenticates under the lab HOME without a manual login. No provider key or token is in the environment, and the wrapper filters the environment anyway. `claude` and `agy` are `needs_login`. `codex`, `copilot`, `cursor`, `gemini`, and `kimi-cli` have no CLI. `moonshot`, `openrouter`, `vercel-ai-gateway`, `xai`, and `zai` are bound-secret with a missing credential. `opencode`, `pi`, and `qwen-code` are `unknown`, and the lab HOME holds no credential files. Only `claude` fits the agent journey, and it needs the login. Full list: `qa-artifacts/qa/logs/provider-list.json`.
- **E2E-003:** not walked. No agent session was created, so no live tool call was observed. Catalog evidence from the lab CLI (operator view, not a session): `compozy__extensions_remove` is destructive with input `{name, owner}` and `additionalProperties: false`, and has no `permanent` input. `compozy__extensions_restore` is mutating with `{name}`. `compozy__extensions_removed_list` is read-only with `{}`. `tool search permanent` returns no permanent-removal tool. Files: `qa-artifacts/qa/logs/catalog-*.json`.
- **Golden path re-captured (input to D2):** `qa-artifacts/qa/logs/dx-*.out`. UDS `DELETE ?permanent=true` returned 200 with the `{"extension": …}` envelope. A second DELETE returned 404 with `{"error":"extension: extension not found: spec-cycle"}`. HTTP `removed-builtins` showed `removed_by: human:local-user`. UDS restore POST returned 200 with the flat payload. The lab was left with `spec-cycle` tombstoned, which is the E2E-003 baseline.
- **Side effect:** a probe with `opencode auth list` tried to start an opencode managed service on port 49374. A process that was already running, PID 3207894 (`opencode serve --service`, not started by this lane), holds that port. The probe failed, and nothing was stopped. Do not run opencode CLI commands that may start services.
- **Teardown:** not run, on purpose. E2E-003 waits on the manual login, and the lab stays up for it.

### Session 3 — E2E-003 agent walk (2026-11-19, lab 2)

- **Lab:** the same lab as Session 2 (`$LAB` below is its root, and `$LAB_BIN` in Session 1 is its `bin/` directory; manifest `qa-artifacts/qa/bootstrap-manifest.json`, binary from `e8afa77ce`, HTTP `127.0.0.1:55935`). Every lab CLI call used `bin/qa-compozy`. The operator daemon and `~/.compozy` were not used, and the operator's `opencode` process (PID 3207894) was not signalled.
- **Login (D1, option b):** the operator ran `provider auth login claude` in the lab HOME. `provider auth status claude -o json` then reported `state: authenticated` and the probe `{"loggedIn":true}` (exit 0). The credential lives in the lab HOME only (`.claude/.credentials.json`, `.claude.json`). The operator's `~/.claude/.credentials.json` was not modified.
- **Setup:** `workspace add "$LAB" --name e2e003-lab` (`ws_0027069a19ca2ec0`); `agent create qa-restore-agent --workspace e2e003-lab --provider claude --model claude-haiku-5-5 --permissions approve-all`; `session new --workspace e2e003-lab --agent qa-restore-agent` (`sess-d5c7ed31703200a3`, `effective_permissions: approve-all`).
- **Prompt and run:** five steps through the session's native tools (prompt text in `logs/e2e003-05-prompt.txt`). The turn ended `done` and the session returned to `idle`.
  1. `compozy__extensions_removed_list` `{}` → `{"removed":[{"name":"spec-cycle","removed_at":"2026-11-18T21:03:38Z","removed_by":"human:local-user"}]}`.
  2. `compozy__extensions_restore` `{"name":"spec-cycle"}` → `{"enabled":true,"name":"spec-cycle","source":"bundled","status":"restored","version":"0.6.0"}`.
  3. `compozy__extensions_removed_list` `{}` → `{"removed":[]}`.
  4. `compozy__tool_search` `{"query":"permanent"}` → 4 tools: `compozy__extensions_removed_list`, `compozy__extensions_restore`, `compozy__profile_current`, `compozy__profile_list`. The two profile tools match on "permanent default outside a bound session" in their descriptions. No removal tool.
  5. `compozy__tool_info` `{"tool_id":"compozy__extensions_remove"}` → risk `destructive`; input `name` (required) and `owner`; `additionalProperties: false`; no `permanent`.
- **Removed list, CLI vs tool:** `logs/e2e003-13-removed-list-comparison.txt` checks structural JSON equality. The agent's pre-restore list equals the CLI baseline `logs/dx-17-removed-baseline.out`. The agent's post-restore list equals `extension removed -o json` run after the restore (`{"removed": []}`).
- **Permanent-tool absence:** the session's permission policy (`logs/e2e003-14-session-policy-removal-tools.txt`) has 297 tools. Its removal-related entries are `compozy__agent_memory_remove`, `compozy__extensions_remove`, `compozy__extensions_removed_list`, `compozy__extensions_restore`, and `compozy__worktree_remove`. No tool name contains "permanent". The plain `compozy__extensions_remove` has no `permanent` input, so an agent cannot request permanent removal through it. This matches Q5 a and `_spec.md`.
- **Post state:** `extension list -o json` shows `spec-cycle` `0.6.0`, `source: bundled`, `enabled: true`, `state: active`.
- **Teardown (two attempts, one valid):**
  - Attempt 1 (`logs/e2e003-16-teardown-attempt1.{out,json}`) ran with `env -i LANG=C.UTF-8`. It printed `clean: true`, but it skipped the registered lab daemon (PID 3316888) as "identity missing or changed", so the daemon and its new extension providers stayed alive. The cause is F5. This attempt is not valid evidence of teardown.
  - Attempt 2 (`logs/e2e003-17-teardown-dry-run-locale.out`, then `logs/e2e003-18-teardown.out`) ran the manifest `TEARDOWN_COMMAND` from the lab directory, with `env -i` and `LANG=pt_BR.UTF-8`, the locale that wrote `daemon.pid`. It stopped the daemon (`stopped_daemon_pid: 3316888`). `qa-artifacts/qa/teardown.json` reports `clean: true`, `survivors: []`, `completed_at: 2026-11-19T11:04:04Z`. A process check afterwards showed no lab process left, and the operator's `opencode` (PID 3207894) still running.
  - The lab was not purged, because the teardown runs without `--purge`. The lab HOME still holds the lab's own Claude login copy under `/tmp/compozyqa-3d96cbf2fc47/provider/.claude/`. Purge it when the lab is no longer needed.
- **Scenarios settled:** E2E-003 → pass.
- **Risk note (F6):** the agent did not read the Compozy reference manuals before its native calls. The session guidance asks for that, but the walk prompt limited the agent to the five calls. The tool results are unaffected. A future agent walk should allow the router reads.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Vera | E2E-001 `extension removed` | the lab `extension list` table has more columns than the DX example | low | recorded under F4 |
| Vera | E2E-003 session start | an agent session cannot start in a lab whose HOME is not the operator's, with no clear hint about the HOME dependency | medium | Decisions for a Human (D1) |

## Findings

- **F1 — DX example drift (DELETE envelope):** the `_dx.md` DELETE example is flat. The real contract keeps the pre-existing `{"extension": …}` envelope and omits empty optional fields. Impact: documentation only; the behavior is correct. Recommendation in D2.
- **F2 — `removed_by` value:** CLI, HTTP, and UDS operator calls store `human:local-user`. The DX examples show `operator`. Impact: documentation only, unless a consumer filters on the string. Recommendation in D2.
- **F3 — error text drift:** a second permanent removal returns `{"error":"extension: extension not found: spec-cycle"}` with exit 1. The DX example reads `error: extension_not_found: spec-cycle is not installed`, and the JSON has no code field. Impact: low; the exit code matches. Recommendation in D2.
- **F4 — `extension list` table:** the human output columns (`Name Version Update Type State Source Missing Env Capabilities`) differ from the DX example (`NAME VERSION SOURCE ENABLED`). Impact: documentation only.
- **F5 — teardown reports `clean: true` while a registered daemon survives (QA tooling, not changed here):** `teardown-qa-env.py` skips a registered process whose identity check fails, and it does not record that process as a survivor, so `clean` stays `true` with `survivors: []`. The identity check compares `ps -o lstart`, which depends on `LANG`. Trigger: a run with `LANG=C.UTF-8` against a daemon whose `daemon.pid` was written under `pt_BR.UTF-8`. Impact: a false clean on the teardown evidence. Recommendation: count identity-skipped live daemons as survivors and set `clean: false`. Owner: `eng-qa-bootstrap`.
- **F6 — walk prompt skipped the router reads:** see the Session 3 risk note. Impact: none on tool results; a harness deviation in the walk. Recommendation: allow the router reads in agent walks.

## Decisions for a Human

- **D1 - a:** how E2E-003 should be verified. *Why it matters:* the agent journey needs a provider session, and the lab's isolated HOME hides the operator's native Claude login. Options:
  - a) Run the lab daemon with the operator HOME for the native login lane (`home_policy=operator`). This reads the operator's `~/.claude` and the user-tier `~/.compozy` extensions, and it breaks the isolation the lab was built for. *Recommended only if you accept that boundary.*
  - b) Log in to Claude inside a lab-scoped HOME from a local terminal, then rerun E2E-003. This keeps isolation and needs one manual step from you.
  - c) Accept E2E-003 as blocked for this cycle and rely on the native tool catalog and policy tests (`UT-016`) for the tool surface, with no live agent walk.
  - **Status (D1):** resolved on 2026-11-19. Option b was used: the operator logged in to Claude in the lab HOME, and E2E-003 was walked in Session 3.
- **D2 - a:** how to settle F1 to F4. *Why it matters:* the spec's golden path is the contract reviewers read first, and it currently disagrees with the daemon. Options:
  - a) Update `_dx.md` examples to the verified shapes (envelope, `human:local-user`, error text, list columns). *Recommended: the behavior already follows the public contract.*
  - b) Change the daemon to match the flat DX example. This breaks the existing `{"extension": …}` envelope that SD-013 regime 2 protects.
  - c) Change the stored `removed_by` to `operator` for operator-scoped callers. This changes stored data for a value nothing filters on today.
  - **Status (D2):** option a applied in `_dx.md` (see its Contract Notes), on the operator's approval. F1–F4 are settled in the spec. The daemon was not changed.
- **D3 - a:** how to satisfy the QA audit's local gate check (C14). *Why it matters:* the audit wants a `verify*.log` from a local `make verify` in the final report. That target runs `mage verify` for the whole repo, and the operator's rule keeps `make gate` out of local runs. This change touches only two QA docs. Options:
  - a) Accept CI as the gate for this docs-only QA change, and record the waiver in the report. *Recommended: CI owns the delivery gates, and no code changed.*
  - b) Run `make verify` locally once and cite its log. This is heavy, and it shares the machine lock.
  - c) Change the audit contract so a CI gate reference counts for docs-only QA runs. This changes the skill, not this run.
  - **Status (D3):** pending the operator's choice.

## Evidence

- Lab logs (scratch, lab-side): `qa-artifacts/qa/logs/e2e001-*`, `e2e002-*`, `e2e004-playwright.log`. E2E-003 (Session 3): `qa-artifacts/qa/logs/e2e003-*`, indexed in `qa-artifacts/qa/logs/e2e003-00-index.md`.
- Lab manifest: `qa-artifacts/qa/bootstrap-manifest.json`. Teardown evidence: `qa-artifacts/qa/teardown.json` (`clean: true`, Session 3).
- QA audit: `qa-artifacts/qa/qa-audit-report.md`. The first run (`logs/e2e003-19-audit-qa-evidence.out`) had four blockers: C7 (empty journey log, fixed with the backfilled `cli`, `api`, and `runtime` entries, each marked `backfilled: true`), C12 (template placeholders and no verdict, fixed in this report), and C14 (no local gate log). After the fixes (`logs/e2e003-20-audit-qa-evidence-after-fix.out`), one blocker remains, C14. It is decision D3.

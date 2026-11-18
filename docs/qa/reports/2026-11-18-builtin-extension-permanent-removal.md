# QA Run Report — 2026-11-18 — built-in extension permanent removal and restore

- **Scope:** operator journey for `compozy extension remove --permanent`, `restore`, and `removed` on the bundled `spec-cycle` built-in across CLI, UDS/HTTP, the agent native-tool surface, and the web UI, plus the restart leg of `ET-bundled-extension-trust`.
- **Cadence tier:** targeted (E2E-001 to E2E-004 from the task graph; no charter file, a single lab pass).
- **Build:** branch `feat/builtin-ext-docs-qa` on base `a03609681`, with uncommitted task_11 and task_12 changes. The lab binary was built from this worktree. **Environment:** isolated lab `compozy-builtin-permanent-removal-20261118-190753-611849-lab`, `COMPOZY_HOME` under `/tmp/compozyqa-6d838056c456/runtime`, HTTP `127.0.0.1:59939`, UDS under the same runtime. The lab HOME is `/tmp/compozyqa-6d838056c456/home`. The operator daemon and `~/.compozy` were not touched.
- **Started:** 2026-11-18T19:08Z · **Status:** closed (E2E-003 blocked by an external provider login boundary; see Decisions for a Human).

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Vera (Policy Administrator) | seed `docs/qa/personas.md` | desktop CLI and browser, local network, en | E2E-001, E2E-002, E2E-003 (blocked), E2E-004 |

## Flows in Scope

- `J-extension-policy-admin` — the administrator removes a bundled built-in for good, lists it, and restores it (`../journeys/J-extension-policy-admin.md`).
- `ET-bundled-extension-trust` restart leg — a tombstoned built-in is not re-enrolled after a daemon restart.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | — (targeted lab run) | J-extension-policy-admin / ET-builtin-extension-permanent-removal (E2E-001) | Vera | CLI remove → restart → list → removed → restore | Pass | — | — |
| 2 | — | ET-builtin-extension-permanent-removal (E2E-002) | Vera | UDS delete → HTTP removed-builtins → HTTP restore | Pass (F1 doc drift) | F1 | — |
| 3 | — | ET-builtin-extension-permanent-removal (E2E-003) | Vera (through agent `qa-restore-agent`) | agent native tools + permanent-tool absence | Blocked (needs human verify) | D1 | — |
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
- **Substitute attempted:** `compozy mcp serve --workspace <lab>` over stdio. It disconnects before `initialize` with `session_id=""` and "initialized before initialize", so it needs an agent session identity. No tool-level evidence was obtained through it.
- **Not verified:** a live agent calling `compozy__extensions_restore` or `compozy__extensions_removed_list`; the result comparison with `extension removed -o json`; and the observation that no permanent-removal tool is offered to a session. The catalog-level evidence exists in the native tool catalog and policy tests (`UT-016`), not in this walk.
- **Scenarios settled:** E2E-003 → blocked (D1).

### E2E-004 — Vera (web)

- **Ran:** 2026-11-18 (box respected: yes).
- **Command:** `bunx turbo run build --filter=compozy-web` (exit 0, produced `web/dist`), then `COMPOZY_TEST_DAEMON_BIN=<lab bin>/compozy COMPOZY_E2E_CLI_BIN=<lab bin>/compozy bun run --cwd web test:e2e:daemon-served:raw -- e2e/__tests__/builtin-extension-removal.spec.ts` → `1 passed (30.8s)`. The spec ran against Playwright's own daemon with a temporary home, not against the lab daemon.
- **Steps covered:** open `/marketplace/installed` → spec-cycle detail → "Actions for spec-cycle" → "Remove…" → "Remove permanently" → type-to-confirm → navigation to `/marketplace/installed` → CLI `extension removed` contains spec-cycle, CLI `extension list` omits it → "Removed built-ins" section → "Restore spec-cycle" → row gone → installed list shows spec-cycle, CLI shows `bundled` and `enabled: true`.
- **Scenarios settled:** E2E-004 → pass.

### ET-bundled-extension-trust — restart leg

- **Ran:** as part of E2E-001 (daemon stop and start, with spec-cycle tombstoned).
- **Result:** after the restart, `spec-cycle` stayed absent from the list, and `removed` still listed it. The tombstone gates boot enrollment. The trust scenario's other legs (provenance, checksum evidence, policy) were not walked in this run and keep their current status.

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

## Decisions for a Human

- **D1 - a:** how E2E-003 should be verified. *Why it matters:* the agent journey needs a provider session, and the lab's isolated HOME hides the operator's native Claude login. Options:
  - a) Run the lab daemon with the operator HOME for the native login lane (`home_policy=operator`). This reads the operator's `~/.claude` and the user-tier `~/.compozy` extensions, and it breaks the isolation the lab was built for. *Recommended only if you accept that boundary.*
  - b) Log in to Claude inside a lab-scoped HOME from a local terminal, then rerun E2E-003. This keeps isolation and needs one manual step from you.
  - c) Accept E2E-003 as blocked for this cycle and rely on the native tool catalog and policy tests (`UT-016`) for the tool surface, with no live agent walk.
- **D2 - a:** how to settle F1 to F4. *Why it matters:* the spec's golden path is the contract reviewers read first, and it currently disagrees with the daemon. Options:
  - a) Update `_dx.md` examples to the verified shapes (envelope, `human:local-user`, error text, list columns). *Recommended: the behavior already follows the public contract.*
  - b) Change the daemon to match the flat DX example. This breaks the existing `{"extension": …}` envelope that SD-013 regime 2 protects.
  - c) Change the stored `removed_by` to `operator` for operator-scoped callers. This changes stored data for a value nothing filters on today.

## Evidence

- Lab logs (scratch, lab-side): `qa-artifacts/qa/logs/e2e001-*`, `e2e002-*`, `e2e003-*`, `e2e004-playwright.log`.
- Lab manifest: `qa-artifacts/qa/bootstrap-manifest.json`. Teardown evidence: `qa-artifacts/qa/teardown.json`.

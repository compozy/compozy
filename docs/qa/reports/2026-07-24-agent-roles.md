# QA Run Report — 2026-07-24 — Agent Roles

- **Scope:** Targeted release-grade execution of the six-role routing, fallback, reserved-name, projection, Settings, and default dream-pipeline contracts on `feat/agent-roles`.
- **Cadence tier:** targeted
- **Build:** `a9a8fcad63f4354505e4c9a0701a6d0f559cc991` plus the uncommitted Task 05/06 QA artifacts · **Environment:** isolated `devtool-oss-launch` lab `agent-roles-devtool-oss-launch-20260724-094737-758561`, HTTP `127.0.0.1:51624`, dedicated UDS/runtime/provider homes.
- **Started:** 2026-07-24T11:15:48Z · **Status:** blocked — full Web E2E precondition is red outside Agent Roles

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | `devtool-oss-launch` | desktop / wifi-fast / en-US | CH-background-role-routing-scopes, CH-settings-roles-live-truth, CH-dream-pipeline-canary |
| Ada | `devtool-oss-launch` | desktop / wifi-fast / en-US | CH-role-fallback-boundary, CH-reserved-builtin-name-sweep, CH-roles-projection-truthfulness |

## Flows in Scope

- `J-route-background-work` — route background work without restart or scope bleed (`../journeys/J-route-background-work.md`).
- `J-digest-sessions-into-memory` — preserve the default extraction and dream pipeline after role rewiring (`../journeys/J-digest-sessions-into-memory.md`).
- `J-32` — retain the existing agent-definition lifecycle while fencing reserved builtin identities.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-background-role-routing-scopes | J-route-background-work / MS-background-role-routing | Dora | Feature Tour | Fixed | BUG-20260724-coordinator-config-list-path; BUG-20260724-inherited-role-provider-resolution | 69b2099f3; a9a8fcad |
| 2 | CH-role-fallback-boundary | J-route-background-work / MS-background-role-fallback | Ada | Network Tour | Fixed | BUG-20260724-inherited-role-provider-resolution | a9a8fcad |
| 3 | CH-reserved-builtin-name-sweep | J-32 / RT-reserved-builtin-agent-names | Ada | Garbage Tour | Fixed | BUG-20260724-bundle-agent-snapshot-loss; BUG-20260724-reserved-bundle-error-mapping | c841d7e06; a1c966c01 |
| 4 | CH-roles-projection-truthfulness | J-route-background-work / MS-inspect-background-role-routing | Ada | Feature Tour | Pass | | |
| 5 | CH-settings-roles-live-truth | J-route-background-work / MS-settings-roles-panel, MS-026 | Dora | Back-Button Tour | Fixed | BUG-20260724-inherited-role-provider-resolution | a9a8fcad |
| 6 | CH-dream-pipeline-canary | J-digest-sessions-into-memory / MS-011, MS-016, MS-017 | Dora | Feature Tour | Fixed | BUG-20260724-memory-extractor-agent-tier | b6f740843 |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Automated Preconditions

| Gate | Result | Evidence |
|---|---|---|
| `make test-e2e-runtime` | Pass after two invalid new assertions were corrected in their canonical daemon suite | Focused `-race` replay passed 4 cases; the complete rerun ended without a reported failure. Initial log: `/Users/pedronauck/Library/Application Support/rtk/tee/1784886632_make_test-e2e-runtime.log`. |
| `make test-e2e-web` | Fail — 51 passed, 62 failed in 1.2h | `/Users/pedronauck/Library/Application Support/rtk/tee/1784891469_make_test-e2e-web.log`; the feature-owned `settings.spec.ts:402` Roles journey passed in 5.7s. The 62-case matrix exactly reproduces the broad pre-existing OS-shell selector/fixture drift already observed during Task 03. |
| `make verify` | Pass | Final source-frozen run on 2026-07-24 exited 0: codegen/installer checks, Bun lint/typecheck/test, Web build, Go fmt/lint/race tests/build, and boundaries all passed with zero reported errors. |
| QA teardown | Pass | `qa-artifacts/qa/teardown.json`: `clean: true`, killed only registered Web PID 89757 and daemon PID 93662, `survivors: []`. |

The Web gate failure is inherited automated-suite debt rather than a reproduced Agent Roles product defect. It remains a release blocker: this report cannot conclude `ready`, and Task 06 cannot satisfy its all-green exit contract while the full lane is red.

## Session Debriefs

### CH-background-role-routing-scopes — Fixed

- Global and workspace `dream` fields retained independent provenance, including a workspace override equal to the global value; the workspace agent route repaired cleanly from `ghost` to `qa-curator`.
- Exact CLI config writes rejected the max-children bound, four deleted paths, invalid `roles.dream.timeout`, and an old `[memory.dream] agent` key while preserving the last good configuration.
- A real Codex session resolved and called `agh__config_list|get|path|set|unset`; the temporary auto-title model was live immediately and restored afterward.
- Two defects were fixed and retested: the leaked `roleconfig` path and the inherited provider chain. The latter's final hidden auto-title child ran on `general/codex/gpt-5.6-luna` with no restart and no fallback.

### CH-role-fallback-boundary — Fixed

- A real auto-title primary failed before acceptance and advanced exactly once to `codex/gpt-5.6-sol`; CLI and HTTP returned the same durable event id `sum-60ee72216e65824f`, parent session, workspace, role, attempt, provider, and model.
- After the inherited-provider fix, the same public workflow ran the primary `codex/gpt-5.6-luna`, generated the title, and emitted zero fallback events.
- Ordered multi-entry exhaustion, single-try behavior, zero failed-attempt residue, empty-chain behavior, and the no-fallback-after-acceptance fence passed in the real-daemon integration lane.
- Deliberate skip: the public CLI/HTTP/UDS contract has no operation that kills only an accepted hidden ACP child. `agh session stop` is a normal lifecycle action and would not prove the fence, so no synthetic public evidence was claimed. The `memory_controller` branch was also skipped because it has no live LLM invocation.

### CH-reserved-builtin-name-sweep — Fixed

- CLI, HTTP, UDS, and real native `agh__agent_create` rejected reserved creates; rename and duplicate rejected without mutating the source; case/whitespace variants rejected; `coordinator-helper` succeeded and was cleaned up.
- The bundle probe found and fixed both snapshot loss and error-code collapse. After rebuild, CLI/HTTP/UDS/native activation returned `agent_name_reserved`, HTTP/UDS used 422, and activation/agent catalogs remained unchanged.
- A pre-existing `$AGH_HOME/agents/coordinator/AGENT.md` was diagnosed and skipped at boot while the virtual coordinator still resolved. CLI, API, UDS, native, and Fleet catalogs stayed builtin-free.

### CH-roles-projection-truthfulness — Pass

- Normalized six-role list and single-role `dream` responses matched field-for-field across CLI, HTTP, and UDS.
- Provenance reported global/workspace ownership per field, including an equal-value workspace override. Inherited projection fields stayed null, timeout appeared only on memory-controller, and no field gained invented provenance.
- A ghost dream agent returned a 200/exit-0 projection with `role_agent_not_found`; an unknown role returned nonzero/404 with exact `role_unknown` envelopes on all three surfaces.

### CH-settings-roles-live-truth — Fixed

- Browser QA rendered all six roles in product order with truthful BUILTIN/INHERIT/OFF states, per-field provenance, only the memory-controller timeout, no prompt editor, and no horizontal overflow at 900×700.
- Auto-title model/fallback saved live and survived hard reload. A dirty model draft disappeared after navigating away/back; a blank fallback retained its draft and focused `auto_title.fallback.0.provider`; the repaired entry saved cleanly.
- A `ghost` dream agent rendered an inline diagnostic with the mono agent name, which cleared after repair. Fleet showed five authored agents and neither builtin.
- The adjacent Memory page retained its persistence, recall, dreaming-policy, ledger, daily-log, and workspace controls while exposing none of the removed role-owned controls. Deep screen-reader/keyboard re-sweep was deliberately skipped under the charter's existing Settings-primitives coverage.
- The live save exposed BUG-20260724-inherited-role-provider-resolution; the rebuilt-daemon retest generated the title on the primary route and no fallback event.

### CH-dream-pipeline-canary — Fixed

- The first sparse-lab dream trigger truthfully skipped with no candidate run. Live disabling returned `dream consolidation is disabled`; re-enabling changed the next trigger to the truthful gate-not-satisfied reason without restart.
- A real provider-backed turn exposed invalid extractor `agent_tier` metadata outside agent scope. After the fix, a fresh turn produced two workspace memories, drain reached zero pending, no new DLQ appeared, and health returned `ok` with two indexed workspace files and zero orphans.
- Dream status/list remained truthfully empty because no run met the gates; no fake run was created. Hidden-session and agent-catalog checks remained clean.

## What Was Fixed

### BUG-20260724-coordinator-config-list-path: Coordinator enabled state is published under a path operators cannot use

- **Symptom:** `agh config list` exposed `roles.coordinator.roleconfig.enabled` instead of the writable public path `roles.coordinator.enabled`.
- **Root cause:** the redacted reflection projector named an anonymous embedded TOML struct after its Go type instead of flattening its fields.
- **Fix:** `69b2099f3cada66395ced4c8ae862b21b5ebc996` merges anonymous untagged TOML struct fields into their parent projection.
- **Regression test:** `internal/cli/config_test.go` — the canonical config rendering suite failed before and passes after the fix.
- **Retested:** J-route-background-work baseline from a rebuilt/restarted daemon; list and single-value reads expose only the canonical path. Package `-race` and repository lint gates pass.

### BUG-20260724-memory-extractor-agent-tier: Extractor emits agent tier metadata outside agent scope

- **Symptom:** global/workspace candidates could carry `agent_tier`, causing controller rejection and a DLQ entry.
- **Root cause:** the prompt presented the field universally and the untrusted-output adapter preserved it for every scope.
- **Fix:** `b6f7408439a68b9e5225b1b086770b4e37347e58` makes the prompt conditional and strips the field outside agent scope.
- **Retested:** a real session produced two valid workspace memories; pending and new-DLQ counts were zero and Memory health was `ok`.

### BUG-20260724-bundle-agent-snapshot-loss: Installed bundle profiles silently lose packaged agents

- **Symptom:** a profile containing `agents/coordinator` previewed/activated as empty, bypassing materialization validation.
- **Root cause:** `cloneBundleSpecs` omitted `BundleProfile.Agents`.
- **Fix:** `c841d7e06428c28e4e1b4ba8c17bccb4a103eea1` deep-clones packaged agents and their Soul/Heartbeat sidecars.
- **Retested:** the same installed fixture reached reserved-name validation with no activation or agent residue.

### BUG-20260724-reserved-bundle-error-mapping: Reserved bundle agents return internal errors

- **Symptom:** the domain sentinel became HTTP/UDS 500 and native `tool_backend_failed`.
- **Root cause:** bundle status mapping omitted `ErrAgentNameReserved`, and the native adapter relied on the generic status.
- **Fix:** `a1c966c01b40ae37372e4431704703acd92e679a` maps 422 and preserves `agent_name_reserved` natively.
- **Retested:** CLI/HTTP/UDS/native all returned the exact code and left catalogs unchanged.

### BUG-20260724-inherited-role-provider-resolution: Model-only inherited roles omit the invoking provider

- **Symptom:** valid model-only inherited auto-title/extractor routes reached Spawn with an empty provider and failed before acceptance.
- **Root cause:** the resolver returned early for `Inherit` before reading invocation correlation and the invoking-agent/default provider chain.
- **Fix:** `a9a8fcad63f4354505e4c9a0701a6d0f559cc991` resolves the correlated agent only in invocation context while projection reads remain honestly unresolved.
- **Retested:** hidden child `sess-afc891322e1060b8` ran `general/codex/gpt-5.6-luna`, completed, generated the parent title, stayed out of the fleet, and emitted zero fallback events. The daemon package passed 1,399 `-race` tests and lint passed.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Dora | J-route-background-work / inherited config inspection | `agh config get` on an unset inherited leaf returns path-not-found while the Roles projection truthfully returns null. | Low | Accepted contract boundary: config reads persisted leaves; role projection owns effective/inherited truth. |

## Runtime Errors Observed

- The complete daemon-served Playwright matrix reproduced 62 pre-existing failures across Agents, Automation, Bridges, Dashboard, Jobs, Knowledge, Marketplace, Network, OS shell, Sandbox, Sessions, Settings, Tasks, Triggers, and Workspace Setup. Representative failures wait for retired `nav-agents`/sidebar selectors while the current product uses the OS dock; several catalog/detail fixtures also expect stale UI text or state. The Agent Roles case passed. Evidence: `/Users/pedronauck/Library/Application Support/rtk/tee/1784891469_make_test-e2e-web.log`.

## Human Verifications Needed

None. The two deliberate charter skips are bounded contract limitations, not hidden requests for manual confirmation: full screen-reader/keyboard depth is owned by the existing Settings-primitives lineage, and no public surface can fault only an accepted hidden child.

## Decisions for a Human

### Restore the full daemon-served Web E2E precondition

- **What's broken:** `make test-e2e-web` is red with 62 broad failures even though the feature-owned Roles journey is green.
- **Why not auto-fixed:** the backlog spans more than twenty spec files and many product surfaces, has multiple root-cause clusters, and fails the QA fix-loop bounds for small size, demonstrated single cause, and contained blast radius.
- **Options:**
  1. Run a dedicated owner-by-owner E2E stabilization round, align each stale journey with the current OS-shell/product contract, and rerun the full matrix — larger scope, but restores the required release signal.
  2. Explicitly accept the inherited gate exception for this feature after preserving the green Roles-specific case — faster, but leaves Task 06's written all-green success criterion unsatisfied.
- **Recommendation:** option 1 through a dedicated `agent-output-audit`/frontend stabilization workstream, then rerun Task 06's complete Web lane.

## Learnings

- A feature-specific green Playwright journey does not replace the repository's complete daemon-served browser precondition.

## Final Status

Blocked only by the full daemon-served Web E2E decision. All six charters are executed, every reproduced Agent Roles defect is fixed and retested, the feature-owned browser E2E is green, final `make verify` passed, and mandatory teardown is clean with zero survivors.

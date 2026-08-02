# QA Run Report — 2026-08-03 — extension-host-api-grant-continuity

- **Scope:** Focused verification that installed Go and TypeScript extensions retain their own Host API grants across manager reloads.
- **Cadence tier:** sanity
- **Build:** `v0.3.0-beta.3-6-g741d3563-dirty` · **Environment:** fresh isolated lab `compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab`, CLI/UDS/HTTP/runtime surfaces, no AI provider or Web server
- **Started:** 2026-08-03T00:49:03-03:00 · **Status:** closed

## Persona and Journey

Ada installed two local subprocess extensions and checked their live authorization through public
lifecycle and status surfaces. This scenario does not create an AI-agent session: its actors are the
daemon plus the installed Go and TypeScript extension processes.

| Scenario | Journey | Persona | Focused status | Issue |
|---|---|---|---|---|
| `ET-extension-host-api-grant-continuity` | `J-extension-distribution` | Ada | Pass | `BUG-20260803-extension-session-grant-denied` verified |

## Results

- Go `secret-guard` received `sessions/list` in its initialize handshake and completed the Host API
  call under PID `1077871`. Installing the TypeScript extension reloaded it; later public lifecycle
  changes produced PIDs `1078119`, `1078355`, and `1078409`. The final process completed the same
  authorized call and remained active and healthy.
- TypeScript `prompt-enhancer` received the same grant and completed `sessions/list` under PID
  `1078107`. Disable plus enable replaced it with PID `1078397`, which completed the authorized call
  and remained active and healthy.
- The TypeScript runtime attempted ungranted `sessions/create` after replacement and received the
  expected fail-closed JSON-RPC `-32001` response. The diagnostic listed only the granted
  `sessions/list` capability.
- UDS-backed CLI status and HTTP `GET /api/extensions/{name}` agreed on enabled state, active state,
  healthy status, process identity, and the exact `sessions/list` permission for both extensions.
- Mandatory teardown stopped daemon PID `1077650`; `teardown.json` records `clean=true` and an empty
  survivor list. Both child processes were absent after teardown.

Behavioral evidence:
`/home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/evidence/extension-host-api/verification.json`.

## Strict Evidence Audit

The generic feature-profile audit returned **FAIL** with nine disclosed blockers. The narrow charter
has three runtime actors instead of four, no differentiated AI-agent roles or product channels, no
Task root/run, no Web/provider surface, no object spanning CLI/API/Web/runtime, no provider-backed
session, and no final `make gate-full` evidence. Artifact reuse and the disruption probe passed.

These are wider release-profile requirements, not failures of the named extension authorization
behavior. The contract was not weakened and no synthetic provider, Task, channel, or Web evidence
was invented. The workstream-wide gate remains intentionally deferred until the last mutation.

Strict audit:
`/home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/qa-audit-report.json`.

## Compozy Impact Audit

- **Native tools:** No tool ID, toolset, descriptor, schema digest, risk flag, availability diagnostic,
  or capability gate changed. Checked extension list/info/install native projections; they consume the
  unchanged public extension contract.
- **Extensibility and hooks:** The live subprocess authorization boundary changed internally. Manager
  launch now carries a private session grant identity separately from the stable public extension
  name. Hook ownership, resources, bridge authorization, rate limits, registries, bundles, SDK wire
  schemas, and `config.toml` keys/defaults remain unchanged.
- **Workspace data isolation:** The private grant remains scoped to one live extension instance and
  session. Stable extension/workspace identity still owns downstream operations; no store, cache,
  SSE, event, list, or read path changed, and the focused tests prove a replacement session cannot
  rely on the public name as an authority alias.
- **Official Compozy skill:** No impact. Checked `skills/compozy/` extension commands and native-tool
  references; public commands, tool IDs, hook events, capability names, and configuration semantics
  are unchanged.

## Web and Docs Impact

No runtime Web component or public product contract changed. The repair corrects an internal grant
lookup while preserving the existing CLI/HTTP/UDS response shape. Durable QA scenario, bug, and run
report documentation are the only documentation changes for this focused fix.

## Final Status

- **Named behavior:** PASS — both reference runtimes retained valid session-bound grants across
  replacement and continued to deny an ungranted capability.
- **Bug:** VERIFIED — `BUG-20260803-extension-session-grant-denied`.
- **Strict generic audit:** FAIL — nine wider feature-profile/final-gate blockers, all disclosed above.
- **Lab teardown:** PASS — `/home/pedronauck/dev/qa-labs/compozy-extension-host-api-grant-continuity-20260803-034903-293855-lab/qa-artifacts/qa/teardown.json` has `clean=true` and no survivors.
- **Overall release-grade verdict:** BLOCKED until the modernization workstream's final gate and wider
  release-grade evidence are complete.

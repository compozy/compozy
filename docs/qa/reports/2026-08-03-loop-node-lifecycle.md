# QA Run Report — 2026-08-03 — Loop node lifecycle

- **Scope:** Real-user validation of Loop node lifecycle behavior across CLI, HTTP/UDS, native tools, Web, daemon restart, approval, repair, quarantine, and authored failure handling.
- **Cadence tier:** full
- **Environment:** isolated lab `compozy-loop-node-lifecycle-20260803-191237-281307`; unique `COMPOZY_HOME`; daemon `60062`; Web `60063`; proxy target derived from the bootstrap manifest.
- **Behavior verdict:** **PASS WITH BLOCKED VERIFICATIONS**
- **Coverage:** 23 scenarios walked — 13 pass/fixed, 10 `blocked-verify`, 0 untested, 0 fail.

`blocked-verify` means the product has no safe public control for the required clock, event replay,
provider death, or synthetic breaker state. Those cases retain automated coverage, but this report
does not present that as real-user proof.

## Personas and live collaboration

| Persona | Role | Device / network | Result |
|---|---|---|---|
| Bruno | operator | desktop / wifi-fast | Recovery, pause, quarantine, requeue, cancel, kill, and restart paths walked |
| Ada | managing agent | desktop / wifi-fast | Eight native lifecycle tools, neighbor isolation, and structured reads walked |
| Lea | Loop author | laptop / wifi-fast | Catalog, run form, detail, editor deep links, error route, and effect context walked |
| Marina | approver | phone-large / 4g plus offline transition | Approve, request changes, reject, and duplicate-decision behavior walked |

The live scenario used seven Claude agents, four channels, eleven completed tasks, seventeen real
messages, and three public review approvals. The assignment and activation deliverables passed
their own executable checks. Evidence:

- `/Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/provider-attempt.json`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/evidence/task-catalog-live.json`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/evidence/task13/network-collaboration-live.json`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/journey-log.jsonl`

The disruption entries are intentionally narrow. Silent-drop behavior was observed through the
task hold. Assignment skew and lifecycle-notification suppression were evaluated through the
produced executable artifacts; no claim is made that stopped providers reacted after the probe.

## Scenario results

| # | Scenario | Status | Evidence or reason |
|---|---|---|---|
| 1 | LP-sick-target-degrades-one-lane | Blocked (needs human verify) | No public breaker/half-open injector |
| 2 | LP-quarantine-diagnose-requeue | Pass | `looprun-9552c0109f7a382c`; repaired generation 10 shown in Web |
| 3 | LP-crash-death-resume | Blocked (needs human verify) | Boot lease bug fixed; no managed provider-death injector |
| 4 | LP-cancel-vs-kill | Pass | Run and node cancel/kill paths reached their distinct terminal states |
| 5 | LP-016 | Pass | Approve, request changes, reject, and duplicate decisions |
| 6 | TA-084 | Pass | Public cancel control remained cooperative and truthful |
| 7 | LP-days-long-node-no-clock | Blocked (needs human verify) | `resume_at` survived restarts; no far-forward controllable clock |
| 8 | LP-live-pause-repair-resume | Fixed | CLI, HTTP, native, and restart resume modes; cross-origin reservation fixed |
| 9 | LP-operator-lifecycle-ui | Pass | Final Web spot checks plus Task 08 lifecycle captures |
| 10 | LP-agent-operates-lifecycle-via-native-tools | Fixed | Eight native tool IDs, neighbor isolation, foreign mutation denied |
| 11 | TA-070 | Pass | CLI, HTTP/UDS, and native status/control parity |
| 12 | TA-076 | Fixed | Native lifecycle operations and normalized workspace identity |
| 13 | LP-durable-wait-restart | Blocked (needs human verify) | Timer durability passed; no public event-cursor injector |
| 14 | LP-waiting-inventory-escalation | Blocked (needs human verify) | Timer/approval inventory passed; no escalation clock/admission injector |
| 15 | LP-duplicate-event-suppressed | Blocked (needs human verify) | No public replay injector or dedupe-horizon clock |
| 16 | LP-editor-authoring-walk | Blocked (needs human verify) | Final deep link/envelope read passed; complete edit/publish walk not repeated |
| 17 | LP-catalog-runform-walk | Pass | Catalog, required input, run form, detail, and hard navigation |
| 18 | LP-transient-blip-heals | Blocked (needs human verify) | Automated E2E passes; no equivalent public transport-failure injector |
| 19 | LP-error-route-fallback | Fixed | `looprun-ce061b65a4695127`; fallback ran once, success branch skipped |
| 20 | LP-unannotated-escalation | Blocked (needs human verify) | Automated E2E passes; no public classified-failure injector |
| 21 | LP-on-error-notification-with-context | Fixed | Committed failure context and isolated effect result agreed across surfaces |
| 22 | LP-terminal-outcome-notification | Blocked (needs human verify) | Four outcomes observed; no public fixture for all seven distinct effects |
| 23 | LP-approval-link-journey | Pass | Phone/4g/offline transition plus atomic duplicate rejection |

## Bugs found, fixed, and re-walked

- `BUG-20260803-loop-boot-active-coordinator-lease` — daemon boot treated an active Loop
  coordinator lease as fatal. It now treats the lease as backpressure and preserves the claimed
  and queued work. The canonical scheduler regression and same-lab restart pass.
- `BUG-20260803-error-route-fallback-blocked` — task materialization gave an authored error-route
  target a success dependency on the failed source. The route target now owns only its error edge;
  the repaired run completed with fallback `recovered` and the success-only branch skipped.
- `BUG-20260803-cross-origin-coordinator-duplicate` — boot and hook origins could collide on one
  deterministic coordinator run. Reservation now coalesces the same semantic run across origins
  while rejecting a conflicting identity.
- `BUG-20260803-agent-workspace-id-disagrees` — agent create/list/get and
  `compozy__agent_create` exposed a durable internal identity instead of the registered public
  workspace ID. All public reads now agree on the `ws_…` ID; separate fresh-lab retests pass.
- GitHub issue `#285` — deferred generation output payload resolution is now scoped by workspace,
  Loop run, generation, node, item, and reference before returning a value. The fix landed before
  this behavior pass so the final QA was not duplicated.

All failures were fixed in production code and strengthened in the owning canonical suites; no
test expectation was weakened.

## Browser evidence

- Catalog: `qa/evidence/task13/06-loop-catalog.png`
- Required run input: `qa/evidence/task13/09-run-form-required-input.png`
- Error-route recovery: `qa/evidence/task13/13-route-fallback-done.png`
- Detail hard navigation: `qa/evidence/task13/14-loop-detail-hard-navigation.png`
- Editor hard navigation: `qa/evidence/task13/15-loop-editor-hard-navigation.png`
- Quarantine/requeue completion: `qa/evidence/task13/16-quarantine-requeue-done.png`
- Recording: `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/loop-node-lifecycle-task13` (48 frames, stopped cleanly)

All relative browser paths are under
`/Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/`.
The first editor hard-navigation observation was captured before the route finished loading; an
element wait proved the editor was healthy, and two repeated hard navigations loaded it within
1.8 seconds. No Web fix was needed.

## Runtime and harness observations

- A stale review-and-fix E2E expectation still described pre-quarantine behavior. The canonical
  suite now asserts the approved quarantine contract and passes in the full runtime E2E lane.
- Shared hosted-MCP diagnostics could choose a background session. The mock helper now binds the
  requested Compozy session, removing the suite-order race.
- Concurrent node progress could be normalized away while a coordinator was open. The durable
  completion path now preserves a wake newer than the in-flight snapshot; the regression and ten
  race-enabled repetitions pass.
- The lab observer falsely reported stale task completions after all eleven public task reads were
  complete. This was recorded as lab-only evidence at
  `qa/issues/BUG-observer-stale-task-completions.md`; no product state was changed for it.

## Human verifications still needed

1. Inject a real target breaker transition through open, half-open, and recovered states.
2. Kill a managed provider session three times and observe repair/quarantine progression.
3. Advance a controllable clock far enough to prove a days-long node has no hidden wall clock.
4. Restart with event cursors both behind and ahead of the committed durable wait.
5. Advance an authored escalation clock after three real admission failures.
6. Replay one delivery inside and outside the dedupe horizon.
7. Repeat the complete editor edit/publish/fork/wait-exclusivity walk on the final build.
8. Inject one retryable transport failure through the public path and restart during backoff.
9. Inject one non-retryable unannotated failure and inspect the real repair successor.
10. Seed all seven terminal outcomes with distinct effects in one public fixture.

These are `blocked-verify`, not product failures. Each tracker names the missing control and the
available automated evidence.

## Compozy Impact Audit

- **Native tools:** no tool ID, descriptor, schema digest, risk flag, or capability gate changed;
  the eight Loop lifecycle native IDs and unknown retired `stop` behavior were checked.
- **Extensibility and hooks:** existing error-route/effect and coordinator-hook behavior changed
  internally; extension IDs, hook names, bundles, bridge SDKs, MCP sidecars, and config keys were
  checked and are unchanged.
- **Workspace data isolation:** output payload data is workspace/run/generation/node/item/reference
  scoped after `#285`; neighbor reads and foreign mutations were checked through CLI, HTTP/UDS,
  native tools, stores, and Web state.
- **Official Compozy skill:** `skills/compozy/` was checked; no public tool ID, CLI path, hook event,
  capability, bundle, resource, memory, network, or task contract changed, so no update is needed.

## Closeout

- Fresh full runtime E2E precondition: PASS.
- Strict real-scenario audit: all behavior and collaboration contracts pass; the final gate
  evidence check is intentionally deferred.
- Full verification gate: intentionally deferred until after the one requested deep-review round
  and its remediation, so `make gate-full` runs once against the final tree.
- Lab teardown evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/teardown.json` records `"clean": true` with no survivors.
- User-impact issues remaining: Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0.
- **Final QA verdict:** **PASS WITH BLOCKED VERIFICATIONS**. The QA work is complete; only the
  workstream-level deep review, remediation, final gate, and delivery remain.

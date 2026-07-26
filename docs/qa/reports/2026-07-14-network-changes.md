# QA Run Report — 2026-07-14 — network-changes

- **Scope:** Opt-in Agent Network participation release cycle: Local defaults, bounded Live admission and usage, future-run coordination, administration/extensibility, discoverability, and an adjacent ordinary-session canary.
- **Cadence tier:** targeted + adjacent canary + release-grade real-scenario companion
- **Build:** rebased checkpoint `567f9bac1` plus the current Task 08 QA correction · **Environment:** fresh isolated manifest per charter; final post-QA `make verify` pending
- **Started:** 2026-07-14T20:40:07-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
| --- | --- | --- | --- |
| Ada | Agent operator | desktop / flaky or wifi-fast / en-US | CH-live-bounds-agent-path, CH-network-local-session-canary |
| Bruno | Autonomy and Network operator | desktop / wifi-fast / en-US | CH-network-admin-lifecycle, CH-coordination-future-runs |
| Nia | Solo builder discovering AGH | laptop / wifi-fast / en-US | CH-network-local-default |
| Mateo Rivera | Helix CLI founder/operator | desktop / production-like provider path / en-US | devtool-oss-launch companion inside CH-live-bounds-agent-path |

## Flows in Scope

- `J-run-bounded-live-collaboration` — explicitly Live work admits only eligible, bounded, workspace-scoped wake sources (`../journeys/J-run-bounded-live-collaboration.md`).
- `J-administer-network-live` — an administrator changes availability and Live policy without enrolling executions (`../journeys/J-administer-network-live.md`).
- `J-enable-coordinated-conversations` — an invitation affects future coordinated runs while current task state stays authoritative (`../journeys/J-enable-coordinated-conversations.md`).
- `J-network-local-default` — ordinary owners complete Local work with no hidden Network artifact or usage (`../journeys/J-network-local-default.md`).
- `J-15` — the ordinary session CLI/API lifecycle remains independent of Network (`../journeys/J-15-operate-session-via-cli-api.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-live-bounds-agent-path | J-run-bounded-live-collaboration / NB-run-bounded-live-collaboration; NB-agent-manages-participation; NB-020 | Ada + Mateo | Interrupt Tour | Blocked (needs human verify) | `BUG-0028`; three verified QA-found defects | pending final whole-diff commit |
| 2 | CH-network-admin-lifecycle | J-administer-network-live / NB-network-live-config-lifecycle; NB-network-availability-toggle; NB-001; NB-002; MS-037; ET-025; ET-026; ET-027; ET-028; ET-030; ET-network-participation-hooks; NB-023 | Bruno | Multi-Tab Tour | Blocked (needs human verify) | three verified QA-found defects; `ET-028` public changed-extension fixture unavailable | pending final whole-diff commit |
| 3 | CH-coordination-future-runs | J-enable-coordinated-conversations / NB-coordination-invitation-future-runs; NB-run-conversation-bounds-usage | Bruno | Back-Button Tour | Fixed | four verified QA-found defects | pending final whole-diff commit |
| 4 | CH-network-local-default | J-network-local-default / NB-execution-participation-defaults; NB-network-empties-onboarding-settings; NB-participation-controls-serialize; RT-010; RT-031; TA-001; TA-004; TA-007; TA-049 | Nia | Feature Tour | Fixed | five verified QA-found defects | pending final whole-diff commit |
| 5 | CH-network-local-session-canary | J-15 / RT-023; RT-042 | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Preconditions and Execution Register

- **Automated precondition:** the pre-rebase source freeze passed `make verify` at 20:17 BRT. After the semantic rebase onto `f230872fc`, fresh runtime and Web E2E lanes pass; the one final full `make verify` remains the completion gate after QA fixes freeze.
- **Playbook rotation:** previous real-scenario run used `consumer-saas-growth` and recorded clean teardown; this run selects validated `devtool-oss-launch`.
- **Open-registry awareness:** the existing autonomous one-kickoff completion stall remains open. A recurrence is appended to that bug as re-found; the observer never sends a second prompt.
- **Runtime E2E lane:** PASS — the first post-rebase `rtk make test-e2e-runtime` exposed two Network fixtures still using the removed occurrence-based ACP matcher. The existing daemon E2E fixtures now select causal message metadata; both failing tests passed under focused `-race`, and the complete gate then exited 0. The earlier `running` → `active` assertion correction remains covered by the same complete gate.
- **Web E2E lane:** PASS — `rtk make test-e2e-web` exited 0 after fresh codegen check, Web build, and the complete daemon-served Playwright lane.
- **Manifest/teardown register:** `CH-live-bounds-agent-path` used companion manifest `/Users/pedronauck/dev/qa-labs/agh-devtool-oss-launch-20260715-054443-670777-lab/qa-artifacts/qa/bootstrap-manifest.json` and deterministic manifest `/Users/pedronauck/dev/qa-labs/agh-network-live-bounds-20260715-061317-610983-lab/qa-artifacts/qa/bootstrap-manifest.json`. Both `qa/teardown.json` files report `clean: true` and zero survivors; the deterministic teardown reaped registered daemon PID 96663. `CH-network-admin-lifecycle` used manifest `/Users/pedronauck/dev/qa-labs/agh-network-admin-lifecycle-20260715-065421-388927-lab/qa-artifacts/qa/bootstrap-manifest.json`; its `qa/teardown.json` reports `clean: true`, zero survivors, and reaped registered Web PID 17038 plus daemon PID 42469. `CH-coordination-future-runs` used manifest `/Users/pedronauck/dev/qa-labs/agh-network-coordination-future-runs-20260715-081644-086405-lab/qa-artifacts/qa/bootstrap-manifest.json`; its `qa/teardown.json` reports `clean: true`, zero survivors, and reaped registered daemon PID 108. `CH-network-local-default` used manifest `/Users/pedronauck/dev/qa-labs/agh-network-local-default-20260715-095346-610866-lab/qa-artifacts/qa/bootstrap-manifest.json`; its `qa/teardown.json` reports `clean: true`, zero survivors, and reaped registered Web PID 47301 plus daemon PID 56091. `CH-network-local-session-canary` used manifest `/Users/pedronauck/dev/qa-labs/agh-network-local-session-canary-20260715-114820-089792-lab/qa-artifacts/qa/bootstrap-manifest.json`; its `qa/teardown.json` reports `clean: true`, zero survivors, and reaped the registered follow clients plus daemon PID 89467 and runtime holder PID 89479.
- **Production-parity deviations:** The real-provider companion did not progress from one kickoff into Task runs and remains blocked by `BUG-0028`. The deterministic fixture proved exact Network mechanics with ACP mock agents, but had no installable extension-host fixture or browser journey. Its strict auditor therefore failed eight generic real-scenario minimums. Automated E2E evidence is not relabeled as a manual provider pass.

## Session Debriefs

### CH-live-bounds-agent-path — release-grade companion

- **Persona / tour:** Mateo Rivera + Ada · Interrupt Tour companion.
- **Setup:** fresh manifest `devtool-oss-launch-20260715-054443-670777`; five unique named channels across five isolated workspaces, nine real Claude Live sessions, 11 explicit-Live ready tasks, no auto-enqueue, one operator kickoff only.
- **Observed:** the Engineering Lead applied the 15% cold-start policy and opened one release-room checklist. One eligible same-workspace wake settled with actual usage (`3m59.334s`, 44 input, 22,194 output tokens); the Release Pipeline Engineer produced a shell release script that passes `sh -n` and a Go control stub that passes `go test ./...`.
- **Blocked outcome:** all 11 seeded tasks remained `ready` with zero runs; no other workspace domain activated; no review, reuse, disagreement, or disruption cycle completed. The 5-minute observer threshold fired and the strict auditor reported 15 blockers. This partially improves but does not close `BUG-0028`; no second prompt was sent.
- **Web deviation:** bootstrap selected `browser-use`, whose local daemon had no active connection; declared `agent-browser` fallback succeeded. The app served the requested Network URL but the fresh home truthfully remained behind first-run setup, captured at `qa/screenshots/live-initial-first-run-1280.png`; this capture does not settle the Network page itself.
- **Evidence:** `qa/{operator-kickoff.jsonl,observation-summary.json,qa-audit-report.json,provider-attempt.json}`, `qa/notes/cli-release-eng-usage-after-stall-by-id.json`, and the two produced artifacts under `workspaces/release-eng/` in the lab.
- **Teardown:** PASS — `qa/teardown.json` has `clean: true`; no lab-owned process survived.

### CH-live-bounds-agent-path — deterministic bounded-Live walk

- **Persona / tour:** Ada · Interrupt Tour.
- **Setup:** fresh manifest `network-live-bounds-20260715-061317-610983`; registered workspace `bounds-lab` (`ws_b94c43d66a542d79`); Local control plus explicit Live owners; finite `max_wakes=2`, `max_wake_depth=2`, and 5-second coalescing.
- **Observed:** eligible direct and mention roots plus a same-root burst created exactly three initial prompts and actual usage of 31 input / 9 output tokens over 2.377 seconds. All 14 messages remained durable; depth- and budget-capped messages did not wake. Disabling Network canceled the active wake with `usage_unavailable`; exact replay was idempotent.
- **Restart:** a queued wake behind an in-flight blocker survived daemon restart. After the correction, persisted target `sess-f6e4d5140f5dc947` resumed, one prompt ran, one source message remained, and `wake-aca546f93f266528` settled with actual usage (19 input / 6 output tokens, 201 milliseconds).
- **Structured surfaces:** CLI, HTTP, UDS, native tools, and agent context agreed on canonical workspace ID, immutable participation, Local tool denial, Live availability, bounds, usage, and typed invalid requests. Native/HTTP/UDS identity and raw-token probes produced no spoofed or secret-bearing persistence.
- **QA-found fixes:** canonical workspace-name resolution, restart-time target resume, and taskless wake-run detail were fixed in their owning layers and live-retested. Bugs: `BUG-20260715-network-usage-workspace-name-empty`, `BUG-20260715-network-wake-restart-target-stopped`, and `BUG-20260715-taskless-network-wake-run-unreadable`.
- **Evidence:** `docs/qa/evidence/2026-07-14-network-changes/ch-live-bounds-agent-path.md`; lab `qa/{journey-log.jsonl,qa-audit-report.json,mock-recovery-worker-v2.jsonl}`.
- **Blocked verification:** the strict generic auditor failed eight minimums: differentiated playbook roles, root Task, Web action, cross-surface object linkage, real provider, reused artifact, final verification report, and final `make verify`. The companion supplies provider/Web attempts but remains blocked by `BUG-0028`; the two labs cannot be combined into an artificial PASS. `NB-run-bounded-live-collaboration` itself passed, while `NB-agent-manages-participation` and `NB-020` remain `blocked-verify` for the unwalked delegated-authority/unsupported/Loop and agent-channel/extension-host paths.
- **Teardown:** PASS — `/Users/pedronauck/dev/qa-labs/agh-network-live-bounds-20260715-061317-610983-lab/qa-artifacts/qa/teardown.json` has `clean: true`, registered PID 96663 reaped, and zero survivors.

### CH-network-admin-lifecycle — configuration, bundles, hooks, and Web

- **Persona / tour:** Bruno · Multi-Tab Tour.
- **Setup:** fresh manifest `network-admin-lifecycle-20260715-065421-388927`; isolated daemon on port 54282; current worktree Web build served by the daemon. `browser-use` had no connection, so the declared `agent-browser` fallback owned the browser walk.
- **Configuration:** supported Live defaults/limits survived reload and restart. Over-ceiling values and removed keys were strictly rejected. Disable canceled active Live work, preserved durable state and Local continuity, and rejected new Live requests; re-enable and replay were idempotent. A controlled two-tab write proved the specified whole-envelope last-write contract and the lab restored max wakes 5/depth 3.
- **Bundles:** preview, explicit activation, list/get, requirement evidence, channel inventory, and no-enrollment semantics passed. Missing confirmation initially mapped to 400 instead of 409 and was fixed. Activation reads now expose optimistic versions; version 1 advanced to 2, then stale CLI and HTTP writes at version 1 returned resource conflict/409 without overwriting current evidence.
- **Changed requirement constraint:** the public local-extension installer correctly rejects reinstalling an already installed extension, so this fixture could not change its manifest digest in place. The canonical service suite proves digest change advances the version, clears confirmation, rejects stale confirmation, and accepts the current version. `ET-028` remains `blocked-verify` for that manual branch rather than being synthesized as a pass.
- **Hooks:** a real installed participation hook initially appeared in the catalog but never ran because its shared resolver captured a nil hook runtime before hook boot. After late runtime attachment, real allow, deny, narrow, and forbidden-widen paths passed.
- **Web:** daemon-served 375/768/1280 captures prove responsive layout, visible keyboard focus, dirty/save/discard state, restart guidance, truthful zero-participant runtime state, and explicit no-enrollment copy. AGH exposes no Web bundle-activation route; CLI/HTTP/UDS/native tools are its specified management surfaces.
- **QA-found fixes:** `BUG-20260715-bundle-confirmation-status-bad-request`, `BUG-20260715-participation-hooks-inert-after-boot`, and `BUG-20260715-bundle-activation-version-hidden` are fixed and retested in their owning surfaces.
- **Evidence:** `docs/qa/evidence/2026-07-14-network-changes/ch-network-admin-lifecycle.md`; lab `qa/{journey-log.jsonl,screenshots/}`.
- **Verdict:** 11 scenarios pass; `ET-028` is `blocked-verify` only for the unavailable public changed-extension fixture.
- **Strict audit:** FAIL with 10 generic release-playbook blockers: actor/role/channel minimums, Task root/run, provider surface/session, cross-surface object, reused artifact, and the still-pending cycle-final `make verify`. These are not administrative-charter claims and were not synthesized.
- **Teardown:** PASS — `/Users/pedronauck/dev/qa-labs/agh-network-admin-lifecycle-20260715-065421-388927-lab/qa-artifacts/qa/teardown.json` has `clean: true`, both registered long-lived processes reaped, and zero survivors.

### CH-coordination-future-runs — invitation, shared run conversation, and Web

- **Persona / tour:** Bruno · Back-Button Tour.
- **Setup:** fresh manifest `network-coordination-future-runs-20260715-081644-086405`; isolated daemon on port 65162; deterministic `coord-worker` ACP mock; current worktree Web build served by the daemon.
- **Invitation:** the first coordinator route hid its stored designation and therefore suppressed the invitation. After correction, the invitation rendered with future-run-only copy. Dismissal persisted through the daemon, and task-scoped acceptance advanced once even under repeated activation. Already-created runs remained Local, the later selected fan-out resolved Live, and an unrelated workspace remained Local.
- **Eligibility:** daemon-served direct routes for a single-agent run and a terminal run showed no invitation. Dismissed scope and unrelated workspace remained ineligible. The existing Web eligibility suite owns the Network-disabled branch because no otherwise-active coordinator remained after the source-fix cycle.
- **Conversation:** three real siblings in designation group `tdg-701c2ad27c0c4afc` now share channel `coord-tdg-701c2ad27c0c4afc-0a305f94c62e`; an independent group resolves a distinct channel. The run route first showed truthful silence, then received `msg-ch3-live-001` through SSE without reload. The 120-message first page loaded five older messages without duplicates.
- **Authority and usage:** bounds and run-fenced actual usage stayed truthful at zero wakes for evidence-only messages. Fresh reads retained `needs_attention`; conversation text did not mutate Task status, claims, or reviews.
- **Web:** direct URL, refresh, Back, and Forward preserved persisted state. Daemon-served 375/768/1280 captures remained responsive. A real Tab pass exposed the shared one-pixel low-alpha focus ring as imperceptible; the system token now renders a contrast-safe two-pixel accent edge at all three widths.
- **QA-found fixes:** `BUG-20260715-task-run-designation-hidden`, `BUG-20260715-run-conversation-hardcoded-empty`, `BUG-20260715-designated-fanout-conversation-split`, and `BUG-20260715-shared-button-focus-invisible` are fixed and retested in their owning surfaces.
- **Evidence:** `docs/qa/evidence/2026-07-14-network-changes/ch-coordination-future-runs.md`; lab `qa/{journey-log.jsonl,qa-audit-report.json,screenshots/}`.
- **Verdict:** both scenarios pass after fixes.
- **Strict audit:** FAIL with 10 generic release-playbook blockers: actor/role/channel minimums, root Task marker, CLI/provider/runtime log rows, cross-surface object, real provider session, reused artifact, disruption probe, and cycle-final `make verify`. These are not claims of this focused charter and were not synthesized.
- **Teardown:** PASS — `/Users/pedronauck/dev/qa-labs/agh-network-coordination-future-runs-20260715-081644-086405-lab/qa-artifacts/qa/teardown.json` has `clean: true`, registered daemon PID 108 reaped, and zero survivors.

### CH-network-local-default — ordinary Local owners and discoverability

- **Persona / tour:** Nia · Feature Tour.
- **Setup:** fresh manifest `network-local-default-20260715-095346-610866`; isolated daemon on port 56385; deterministic `local-default` ACP mock; current worktree Web build served by the daemon.
- **Owner matrix:** omitted participation resolved Local for ordinary session, spawned and detached children, Task run/review, CLI and HTTP/Web Loop runs, Automation Job, and Trigger-backed execution. The Task completed through claim, start, handoff, completion, exact idempotent replay, and daemon restart without a Network channel, wake, prompt fragment, tool capability, or usage row.
- **Context and boundaries:** Local agent context exposed no coordination fiction; every `agh__network*` descriptor remained non-executable with `not_participating`. Legacy owner fields returned named `400 unknown_field` errors before mutation. Empty `task next` returned no claim token, while `--wait` remained blocked until controlled cancellation.
- **Web and docs:** onboarding, Network ready/disabled empties, Settings, the public guide, and the official skill agreed on Local-by-default and explicit future Live participation. Session, Task, Loop, and Automation controls rendered Local/Live truthfully at 375/768/1280 px without submitting the probes.
- **QA-found fixes:** `BUG-20260715-network-ready-empty-unoriented`, `BUG-20260715-loop-participation-contract-dropped`, `BUG-20260715-automation-task-participation-control-missing`, `BUG-20260715-automation-editor-compact-layout-clipped`, and `BUG-20260715-loop-run-compact-layout-collapsed` are fixed and retested in their owning contract, component, and rendered surfaces.
- **Evidence:** `docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md`; lab `qa/{journey-log.jsonl,qa-audit-report.json,screenshots/}`; visual-contract envelope `/tmp/agh-ui-screenshot.9xvTyy` with `teardown.json` `clean: true`.
- **Verdict:** all nine scenarios pass after fixes.
- **Strict audit:** FAIL with six generic release-playbook blockers: actor/role/channel minimums, one real Live provider-backed session, one later-reused artifact, and cycle-final `make verify`. Those minimums contradict or exceed this focused zero-Network Local charter and were not synthesized; its Task root/run, provider surface, disruption probe, and cross-surface object linkage passed.
- **Teardown:** PASS — `/Users/pedronauck/dev/qa-labs/agh-network-local-default-20260715-095346-610866-lab/qa-artifacts/qa/teardown.json` has `clean: true`, registered Web PID 47301 and daemon PID 56091 reaped, and zero survivors.

### CH-network-local-session-canary — adjacent ordinary session lifecycle

- **Persona / tour:** Ada · Feature Tour.
- **Setup:** fresh manifest `network-local-session-canary-20260715-114820-089792`; isolated daemon on port 59646 and UDS `aghd.sock`; deterministic `local-canary` ACP mock; Network enabled/ready with no channel or usage.
- **Lifecycle:** omitted participation created `sess-dac16c82d9215e16` as immutable Local. A raw CLI follow reconstructed one user/assistant/done turn while the session continued idle in background. A fenced transcript subscriber remained open through stop, received the terminal delta through cursor 7, and closed without a recorder-unavailable error.
- **Transcript:** one-entry REST pages walked stable start cursors 4, 3, 2, and 1 with constant epoch/generation/max sequence. Matching fences yielded one bounded delta; missing fences yielded `reset=true, reason=fence_missing`. HTTP and UDS bodies matched for each page and SSE payload.
- **Parity and restart:** stopped list, detail, and transcript were byte-identical across HTTP/UDS. Status and recap matched after masking only the contract's server-set timestamps. The same comparisons passed after a controlled daemon restart, with the Local snapshot and complete transcript unchanged.
- **Network isolation:** channels remained `[]`, usage remained all-zero, and deterministic provider diagnostics contained no Live coordination context, channel/wake identity, or wake prompt.
- **Evidence:** `docs/qa/evidence/2026-07-14-network-changes/ch-network-local-session-canary.md`; lab `qa/{journey-log.jsonl,qa-audit-report.json,logs/}`.
- **Verdict:** both scenarios pass; no new defect or source change.
- **Strict audit:** FAIL with 10 generic release-playbook blockers: actor/role/channel minimums, Task root/run, Web, an object spanning CLI/API/Web/runtime, a real provider, final verification report, and cycle-final `make verify`. The charter explicitly forbids Web and Live and owns no Task or multi-agent collaboration, so those minimums were not synthesized.
- **Teardown:** PASS — `/Users/pedronauck/dev/qa-labs/agh-network-local-session-canary-20260715-114820-089792-lab/qa-artifacts/qa/teardown.json` has `clean: true`; registered raw/transcript follow clients, daemon PID 89467, and runtime holder PID 89479 were reaped with zero survivors.

## Experiential Lens Pass

`J-administer-network-live` completed its widest responsive and keyboard pass: the Settings route remained legible at 375/768/1280, keyboard focus was visible, dirty/discard recovery was clear, and status/no-enrollment copy matched runtime truth. `J-enable-coordinated-conversations` completed responsive, direct-navigation, Back/Forward, refresh, pagination, SSE, and keyboard passes; its focus defect was corrected at the shared token owner and recaptured at all widths. `J-network-local-default` completed responsive Network-empty, owner-control, compact-layout, strict-input, restart, and cross-surface passes; all five defects were corrected and recaptured. `J-15` completed its headless bounded-transcript, fenced-reconnect, stop/finalize, transport-parity, and restart canary without a Network artifact. The bundle changed-digest branch remains blocked on a public fixture, and the real-provider companion remains blocked by `BUG-0028`; all planned experiential walks are otherwise terminal.

## What Was Fixed

### Runtime E2E contract drift: Network status expected a removed transport state

- **Symptom:** two real Network collaboration E2E scenarios failed after receiving `enabled=true, status=active` with two local peers because their assertions still required `status=running`.
- **Root cause:** the E2E suite did not co-ship the Task 06 status hard cut from broker-oriented `running` to Local/Live `disabled|ready|active`; production and the lower canonical status suite already matched the TechSpec.
- **Fix:** current Task 08 local diff; two existing assertions now require the public `active` state. No production path or assertion strength changed.
- **Regression test:** existing `TestDaemonE2ENetworkDirectReplyLifecycleWithMockAgents` and `TestDaemonE2ENetworkWhoisAndCapabilityExchange`; focused `-race` run passed 4 tests and full `make test-e2e-runtime` then exited 0.
- **Retested:** automated owning suite only; the in-persona Live charter remains Pending.

### ACP mock fixture drift: Network wakes selected turns by occurrence

- **Symptom:** the first post-rebase runtime E2E gate rejected `network_bounds_fixture.json` and `network_cancel_fixture.json` because `occurrence` is not part of the current strict fixture schema.
- **Root cause:** the Network fixtures predated main's hard cut from order-sensitive occurrence matching to stable prompt metadata. Reintroducing the removed matcher would make retries and scheduling order part of the contract.
- **Fix:** current Task 08 local diff; the bounds fixtures select the mention/direct wake by `message_id` and the coalesced burst by its stable causal `reply_to`; the cancellation fixture already had a complete message identity and simply drops `occurrence`.
- **Regression test:** existing `TestDaemonE2ENetworkLiveBoundsCoalesceDepthAndBudget` and `TestDaemonE2ENetworkWakeCancellationWithMockAgent` in the canonical daemon E2E suite.
- **Retested:** focused `-tags=integration -race` run exited 0, followed by a complete `make test-e2e-runtime` pass.

### Named workspace Network usage queried the wrong store key

- **Symptom:** `agh network usage --workspace bounds-lab -o json` returned an empty report despite settled wakes in that registered workspace.
- **Root cause:** the shared API route resolver validated the name but returned the raw route token rather than the canonical resolved workspace ID.
- **Fix:** `requireRouteWorkspaceID` now returns the resolved ID; no store sees a workspace name as an ownership key.
- **Regression test:** `internal/api/core/network_usage_public_test.go` routes through name `alpha`, requires query ID `ws-alpha`, and asserts the canonical response ID.
- **Retested:** focused API core `-race`, then live CLI and UDS reads; both returned all eight durable wake rows under `ws_b94c43d66a542d79`.

### Restart recovery prompted a stopped Network target

- **Symptom:** a durable queued wake survived restart but settled in 1 millisecond as `network wake prompt failed`.
- **Root cause:** the wake runner recovered the taskless run and called `PromptNetwork` without first resuming the persisted target session; session prompting correctly accepts only active sessions.
- **Fix:** claimed Network wakes resume their target before prompt execution; active resume remains idempotent.
- **Regression test:** the canonical wake-runner startup recovery test requires exactly one `Resume` before the recovered prompt.
- **Retested:** focused daemon `-race`, then a live daemon restart; `wake-aca546f93f266528` prompted once and settled with actual usage.

### Taskless Network wake run could not be read by its public ID

- **Symptom:** Network usage exposed `task_run_id=run-1e1b1175e599c620`, but `agh task run show` failed with `task id is required`.
- **Root cause:** task-run detail unconditionally used the task-backed load path even though `network_wake` runs intentionally require an empty Task ID.
- **Fix:** run detail now reads every persisted run and includes a Task reference only when one exists. OpenAPI, TypeScript, extension host, CLI human/JSON/TOON, Web task-route guard, official AGH skill, and generated contracts co-ship the optional reference.
- **Regression test:** the canonical Task live-view suite proves a taskless Network wake returns without a fabricated Task; API, CLI, extension, and Web contract lanes pass.
- **Retested:** CLI, HTTP, and UDS all returned the corrected live run; `make codegen-check`, four owning Go race packages (363 tests), and the full Web Turbo lane pass.

### Missing bundle confirmation reported malformed input

- **Symptom:** a valid Live-requiring bundle activation without explicit confirmation returned HTTP 400 even though the public contract requires a resolvable 409 conflict.
- **Root cause:** the bundle error mapper grouped missing requirement confirmation with malformed requests.
- **Fix:** missing confirmation maps to Conflict; invalid request handling remains unchanged.
- **Regression test:** the canonical API core bundle error-mapping suite.
- **Retested:** real HTTP returned 409 and created no activation; explicit confirmation then activated successfully.

### Participation policy hooks were inert after boot

- **Symptom:** a healthy installed `network.participation.pre_resolve` hook appeared in the catalog but did not receive real session resolutions.
- **Root cause:** the shared participation resolver was constructed before hook boot and permanently retained a nil runtime.
- **Fix:** the resolver owns atomic late attachment and `bootHooks` attaches the booted runtime.
- **Regression test:** the canonical daemon boot-hooks suite requires the shared resolver to receive the runtime.
- **Retested:** real allow, deny, Live-to-Local narrowing, and forbidden Local-to-Live widening all produced the expected structured outcomes.

### Bundle activation hid its optimistic version

- **Symptom:** two stale operators could not participate in the required activation CAS because reads omitted `version`, updates omitted `expected_version`, and the store silently reread current state.
- **Root cause:** activation conversion dropped the resource version and update storage used a fresh internal read as the expected version.
- **Fix:** version/expected-version now travel through model, store, service, API, CLI, generated contracts, site docs, and the official skill; requirement reconciliation advances changed digests and clears prior confirmation.
- **Regression test:** canonical resource-store, Network requirement, API core, and CLI suites.
- **Retested:** live version 1 -> 2 succeeded; stale CLI exited 65 and stale HTTP returned 409 without overwriting current confirmation.

### Task-run detail hid stored coordinator designation

- **Symptom:** an otherwise eligible active coordinator run showed no contextual future-coordination invitation.
- **Root cause:** public run conversion retained the designation group and raw metadata but dropped the parsed designation summary, so Web could not recognize index zero.
- **Fix:** `TaskRunPayloadFromRun` projects `DesignationFromRun(...).Summary()` into the public run payload.
- **Regression test:** the canonical API payload/surface suite requires designation group, index, and brief.
- **Retested:** the real coordinator route rendered the invitation; daemon-backed dismissal and task-scoped acceptance then passed.

### Run detail hardcoded an empty conversation and zero usage

- **Symptom:** a durable run message existed, but Web reported silence and had no pagination or live update path.
- **Root cause:** task-run detail exposed no run Network projection and the Web route constructed placeholder conversation/usage values.
- **Fix:** run detail now includes the deterministic workspace/channel/thread reference plus run-bound budget/usage; a typed run-scoped SSE endpoint and paginated hook/panel consume the durable store.
- **Regression test:** canonical GlobalDB budget projection, API run-detail/SSE, generated contract, Web route, hook, and panel suites.
- **Retested:** empty silence rendered first, `msg-ch3-live-001` arrived without reload, and the 120-message page loaded five older rows without duplicates.

### Designated fan-out siblings resolved different conversations

- **Symptom:** coordinator and worker snapshots in one fan-out group carried distinct derived channels.
- **Root cause:** the run-channel resolver received each individual run ID even when `designation_group_id` already defined the conversation group.
- **Fix:** participation resolution derives the conversation key from the designation group while retaining individual run ownership and budgets.
- **Regression test:** the canonical Task integration suite requires three siblings to share one channel and an independent group to use another.
- **Retested:** all three real siblings project `coord-tdg-701c2ad27c0c4afc-0a305f94c62e`; the independent group remains isolated.

### Shared keyboard focus ring was imperceptible

- **Symptom:** Tab reached `Load older messages` and `:focus-visible` matched, but the control looked unfocused on the dark canvas.
- **Root cause:** the shared outward focus token used only a one-pixel, 9%-alpha hairline, which is an active-control border treatment rather than a perceivable keyboard indicator.
- **Fix:** the canonical token now paints a one-pixel canvas separator plus a two-pixel `accent-strong` outer edge; individual buttons were not patched.
- **Regression owner:** generated design-token contract plus real keyboard capture; a CSS-literal unit test was deliberately not added because it would not prove rendered perception.
- **Retested:** settled computed style and screenshots at 375/768/1280 show the edge; its contrast against the dark surface ramp is at least 6.43:1. `packages/ui` Turbo (103 files / 517 tests), `make web-build`, and `make codegen` pass.

### Ready Network empty state omitted Local/Live orientation

- **Symptom:** a ready Network with zero channels rendered only `No channels yet`, without explaining that ordinary work stays Local or that availability never enrolls an execution.
- **Root cause:** the real route bypassed the state-aware `NetworkEmpty` composite and mounted the generic empty primitive.
- **Fix:** ready and disabled routes now share truthful Local-continuity and explicit-Live guidance with one neutral Settings action.
- **Regression owner:** the existing Network empty-state component suite and daemon-served Network route.
- **Retested:** ready/disabled captures at 375/768/1280 px show visible focus and no horizontal overflow.

### Loop definition dropped typed Network participation

- **Symptom:** a Loop could accept participation at creation but its public definition document discarded it, so the run form could not faithfully expose or preview Local/Live selection.
- **Root cause:** `LoopDefinitionDocument` omitted the typed participation request during contract conversion.
- **Fix:** the Go/OpenAPI/TypeScript document retains `network_participation`; the run form uses the shared control and previews the value resolved when the run starts.
- **Regression owner:** the canonical Loop contract round-trip suite plus the existing Web run-form suite.
- **Retested:** API contract `-race` passed 202 tests; the complete Web Turbo lane passed 406 files / 3,529 tests; Local/Live captures passed at all three widths.

### Automation Task target omitted Network participation

- **Symptom:** Session, Task, and Loop surfaces exposed Local/Live selection, but an Automation Task target offered no equivalent control.
- **Root cause:** the shared participation control was not wired into the Task-target editor or its typed serialization path.
- **Fix:** Task targets expose Local/Live before owner selection and serialize only `network_participation`.
- **Regression owner:** the existing Automation target editor and serialization suites.
- **Retested:** Local/Live controls and payload removal on return to Local passed at 375/768/1280 px without submitting the probe.

### Automation compact editors clipped primary controls

- **Symptom:** the Automation dialog and Job/Trigger editors created competing scroll owners, clipping target, schedule, participation, or owner controls on compact viewports.
- **Root cause:** nested constrained grids and overflow regions fought the dialog's height contract.
- **Fix:** the modal owns the viewport constraint; compact editors use block flow, one scroll owner, and wrapping pill groups while desktop retains the split preview.
- **Regression owner:** rendered Automation stories and the existing component behavior suites.
- **Retested:** the 375 px dialog fits at 343/343 px, 768 px remains inside the viewport, and desktop preserves the split rail; the Storybook envelope teardown is clean.

### Loop run form collapsed below desktop

- **Symptom:** at 375 px the Loop run form had a 40 px client height for 590 px of content, hiding the participation control and most fields.
- **Root cause:** responsive grid rows and nested overflow owners collapsed the form inside the page shell.
- **Fix:** compact layouts use one page-level vertical scroll owner and visible form/preview overflow; desktop keeps its split layout.
- **Regression owner:** the existing Loop run-form component suite plus rendered compact and desktop captures.
- **Retested:** at 375 px the form is 741/741 px inside a 764/1490 px scroll container with a 375/375 px document; 768/1280 captures remain correct.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
| --- | --- | --- | --- | --- |
| Ada | J-run-bounded-live-collaboration / usage reconciliation | A valid workspace name made real usage look absent. | Misleading | Fixed and verified. |
| Ada | J-run-bounded-live-collaboration / wake reconciliation | The public wake run ID could not be followed with `task run show`. | Workflow break | Fixed and verified. |
| Bruno | J-administer-network-live / bundle activation | A required operator decision looked like malformed input. | Misleading | Fixed and verified. |
| Bruno | J-administer-network-live / stale confirmation | Activation reads hid the version needed to reject stale decisions. | Trust break | Fixed and verified. |
| Bruno | J-administer-network-live / participation policy | A catalogued policy hook silently did not run. | Trust break | Fixed and verified. |
| Bruno | J-enable-coordinated-conversations / invitation | The eligible coordinator looked like an ordinary run because its designation disappeared at the API boundary. | Workflow break | Fixed and verified. |
| Bruno | J-enable-coordinated-conversations / run conversation | Durable coordination evidence looked permanently empty. | Trust break | Fixed and verified. |
| Bruno | J-enable-coordinated-conversations / fan-out | Siblings assigned to one coordination group could not see one shared conversation. | Workflow break | Fixed and verified. |
| Bruno | J-enable-coordinated-conversations / pagination | Keyboard focus technically moved but remained visually imperceptible. | Accessibility friction | Fixed systemically and verified. |
| Nia | J-network-local-default / Network discovery | The ready empty state did not explain that ordinary work remains Local. | Orientation friction | Fixed and verified. |
| Nia | J-network-local-default / Loop owner | A typed participation choice disappeared at the public definition boundary. | Workflow break | Fixed and verified. |
| Nia | J-network-local-default / Automation Task target | The visible owner surface could not choose explicit Live participation. | Workflow break | Fixed and verified. |
| Nia | J-network-local-default / Automation compact editor | Core target and scheduling controls were clipped on small screens. | Workflow break | Fixed and verified. |
| Nia | J-network-local-default / Loop compact editor | The run form collapsed to a thin strip below desktop width. | Workflow break | Fixed and verified. |

## Runtime Errors Observed

- The automated ACP fixture schema failure above was corrected before live execution.
- The real provider path itself stayed healthy. The release-grade companion stalled at autonomous activation: 11 ready tasks had zero runs and only one same-workspace Network wake completed. This is linked to open `BUG-0028` rather than filed as a duplicate.
- The admin lab exposed three contract defects, all fixed and live-retested: missing bundle confirmation status, late participation-hook attachment, and public activation optimistic concurrency.
- The coordination lab exposed four contract defects, all fixed and live-retested: hidden designation, placeholder run conversation/usage, per-sibling conversation derivation, and the shared keyboard focus treatment.
- The Local-default lab exposed five product defects, all fixed and retested: unoriented Network empty state, dropped Loop participation, missing Automation Task participation, clipped Automation compact editors, and collapsed Loop compact layout.

## Human Verifications Needed

- `ET-028` needs a public fixture capable of advancing an installed extension's Network requirement digest; the isolated local installer correctly rejects in-place reinstall. Current-version/stale-version confirmation is manually proven and the changed-digest production suite passes.

## Decisions for a Human

- Decide whether the generic release-grade auditor should support a focused deterministic feature charter without requiring unrelated playbook Tasks, Web, and reusable artifacts. This run preserves the current enforced FAIL and does not weaken the auditor.

## Learnings

- Focused deterministic charters and the generic release-playbook auditor answer different questions. Keep the enforced FAIL when minimums contradict the charter, but preserve the focused scenario verdict and exact missing minimums instead of fabricating unrelated actors, Tasks, Web actions, or Live providers.
- HTTP/UDS parity must normalize only the documented server-set timestamps. Exact bodies matched for session list, detail, transcript pages, and SSE; status/recap raw diffs contained only request-time fields and normalized to zero.
- Local-by-default requires both runtime isolation and visible discoverability. The owner matrix alone would not have exposed the dropped Loop document field or the two compact-layout failures.

## Final Status

- **Exit gate (full automated suite):** Pending.
- **Issues by user impact:** Blocks-Completion 8 (1 open, 7 fixed) · Data-Loss 0 · Trust-Damage 4 fixed · Friction 4 fixed · Cosmetic 0
- **Coverage:** 5/5 charters walked; 2 Fixed, 1 Pass, 2 Blocked (needs human verify). Twenty-five of 28 scenarios pass; three remain `blocked-verify`.
- **Verdict:** pending — no release-readiness claim exists before the matrix, strict audit, automated lanes, and teardown register are terminal.

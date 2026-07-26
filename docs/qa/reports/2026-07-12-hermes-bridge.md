# QA Run Report — 2026-07-12 — Hermes bridge

- **Scope:** Hermes bridge Tasks 01–08: setup and diagnostics, provider-native progress, long replies, durable restart recovery, inbound edit/reply intent, Web setup, and eight-provider documentation.
- **Cadence tier:** Targeted, with required runtime/Web E2E lanes and a `northstar-pay` network-channel scenario.
- **Build:** `62ab3bc` · **Environment:** fresh isolated lab at `http://127.0.0.1:40645`; provider behavior uses deterministic fake/sandbox endpoints unless stated otherwise.
- **Started:** 2026-07-13T02:22:26Z · **Lab closed:** 2026-07-13T05:29:36Z · **Status:** charters settled and teardown clean; final global gate pending
- **Plan:** [`2026-07-12-hermes-bridge-plan.md`](2026-07-12-hermes-bridge-plan.md)
- **Bootstrap manifest:** `/home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Charter result ledger:** `/home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json`

## Result at a Glance

Seven of nine bridge charters reached `Pass`. CH-guided-setup-credentials and CH-structured-telegram-setup share the same `Blocked (human decision)` cause because guided Telegram setup cannot represent the provider's alternative direct-message, ordinary-group, and forum-topic route shapes (`BUG-20260713-telegram-route-shapes`). The isolated Northstar scenario independently reproduced open autonomy failure `BUG-0028`. Provider-fake, browser, CLI, HTTP, UDS, race-enabled owner, integration, and exact daemon-E2E evidence otherwise agreed.

This is not release-ready yet: the strict Northstar auditor is red, the source-fixed `BUG-20260712-reasoning-evidence-attribution` still needs one fresh full runtime lane, the stale-bundle Web result must be replaced by one fresh final lane, and the workflow-wide `make verify` has not run by explicit scheduling policy.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Tessa — First-time Bridge Operator | installed provider extension, no bridge | laptop / wifi-fast / en-US | CH-first-slack-response, CH-guided-setup-credentials, CH-web-bridge-setup |
| Ada — Automation Agent | documented structured interfaces | CLI/HTTP/UDS / isolated runtime / machine-readable output | CH-structured-telegram-setup |
| Maya — Channel Teammate | routed provider conversation | laptop / wifi-fast / en-US | CH-bridge-progress-stress, CH-edit-reply-context |
| Omar — Bridge Fleet Operator | configured multi-provider fleet | laptop / wifi-fast / en-US | CH-long-provider-replies, CH-mid-turn-bridge-restart, CH-bridge-verification-secrets |
| Sofia Mendes — Northstar operator | materialized `northstar-pay` launch room | isolated runtime / 10 channels / one kickoff | real-scenario collaboration observation |

## Flows in Scope

- [`J-connect-bridge-provider Connect a bridge provider and receive the first response`](../journeys/J-connect-bridge-provider.md) — finish on the provider's real chat, issue/comment, or Agent Session surface.
- [`J-watch-agent-work-channel Watch agent work in a channel`](../journeys/J-watch-agent-work-channel.md) — useful provider-native progress without spam, secrets, or transcript chrome.
- [`J-diagnose-repair-bridge Diagnose and repair a bridge`](../journeys/J-diagnose-repair-bridge.md) — structured checks identify a repair without falsifying lifecycle state.
- [`J-deliver-long-formatted-reply Deliver a long formatted reply`](../journeys/J-deliver-long-formatted-reply.md) — every wire body stays under its provider limit and reconstructs losslessly.
- [`J-recover-mid-turn-restart Recover after a mid-turn restart`](../journeys/J-recover-mid-turn-restart.md) — universal visible fail-open, durable metrics, no text replay, exact ownership.
- [`J-complete-web-bridge-setup Complete bridge setup in the Web`](../journeys/J-complete-web-bridge-setup.md) — daemon-derived setup and remediation survive reload/back navigation.
- [`J-edit-reply-context Edit and reply in context`](../journeys/J-edit-reply-context.md) — edits remain distinct and reply context stays bounded and isolated.
- Adjacent canary: `J-23` catalog/health continuity through `NB-024` in CH-bridge-verification-secrets.

## Preconditions

| Precondition | Status | Evidence |
|---|---|---|
| Fresh isolated bootstrap and populated `northstar-pay` charter | Pass | bootstrap manifest and validated playbook under the lab QA output path |
| `make test-e2e-runtime` | Historical fail; source-fixed | `BUG-20260712-goal-judge-fixture-model` fixed the stale Goal fixture. The separately approved `BUG-20260712-reasoning-evidence-attribution` harness fix now stamps and selects the daemon-provided AGH session owner; its concurrent shared-file regression and exact reasoning E2E pass under `-race`, including ten stress runs. The fresh complete lane remains reserved for the final source freeze. |
| `make test-e2e-web` | Invalidated | The first run passed 62/70 but `BUG-0037` proved it served obsolete Web assets. `BUG-20260712-bridge-e2e-retired-route` updated the Bridge owner for the current catalog/detail routes; focused current-bundle Bridge scenarios pass 2/2. The fresh full-lane rerun is reserved for the final workflow gate. |
| Browser driver | Pass with fallback | `browser-use` required a human Chrome remote-debugging approval. The charter-prescribed `agent-browser` fallback ran headless with a registered PID and captured the complete CH-web-bridge-setup journey. |
| Deterministic provider boundary | Pass, qualified | Slack, Telegram, Discord, and WhatsApp used local fake APIs; provider extension binaries were rebuilt from current source before evidence. No result claims a live vendor account. |

## Session Matrix & Results

| # | Charter | Journey / Scenarios | Persona | Tour | Status | Issue | Evidence |
|---|---|---|---|---|---|---|---|
| 1 | CH-first-slack-response | J-connect-bridge-provider / NB-025, NB-029, NB-036, NB-037, NB-038, NB-bridge-provider-setup | Tessa | Feature Tour | Pass | — | fake Slack request log; manifest/verify/send payloads; routed acpmock fixture |
| 2 | CH-guided-setup-credentials | J-connect-bridge-provider / NB-bridge-provider-setup | Tessa | Paste Tour | Blocked (human decision) | [BUG-20260713-telegram-route-shapes](../bugs/BUG-20260713-telegram-route-shapes.md) | validator outputs; masked bindings; fake Telegram registration/send log |
| 3 | CH-structured-telegram-setup | J-connect-bridge-provider / NB-024, NB-025, NB-026, NB-036, NB-037, NB-038, NB-039, NB-bridge-provider-setup | Ada | Feature Tour | Blocked (human decision) | [BUG-20260713-telegram-route-shapes](../bugs/BUG-20260713-telegram-route-shapes.md) | strict CLI JSON; HTTP/UDS parity; two group+topic delivery IDs |
| 4 | CH-bridge-progress-stress | J-watch-agent-work-channel / NB-028, NB-bridge-tool-progress, NB-provider-progress-rendering | Maya | Garbage Tour | Pass | — | Slack visible provider log plus exact progress/redaction/transcript owners |
| 5 | CH-long-provider-replies | J-deliver-long-formatted-reply / NB-long-bridge-replies | Omar | Paste Tour | Pass | — | shared chunker plus all six provider wire owners under `-race` |
| 6 | CH-mid-turn-bridge-restart | J-recover-mid-turn-restart / NB-031, NB-bridge-restart-recovery | Omar | Interrupt Tour | Pass | — | broker/GlobalDB/boot owners, fresh-broker integration, exact daemon restart E2E |
| 7 | CH-web-bridge-setup | J-complete-web-bridge-setup / NB-026, NB-028, NB-039, NB-web-bridge-setup | Tessa | Back-Button Tour | Pass | — | six screenshots; API/current-source CLI/provider-log readback |
| 8 | CH-edit-reply-context | J-edit-reply-context / NB-bridge-edit-reply | Maya | Interrupt Tour | Pass | — | Slack `message_changed` route reuse plus Slack/Telegram/GChat/cache/Host API/E2E owners |
| 9 | CH-bridge-verification-secrets | J-diagnose-repair-bridge / NB-024, NB-029, NB-bridge-provider-setup | Omar | Garbage Tour | Pass | — | malformed-secret and URL matrix owners; structured provider checks; doctor aggregation |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`.

## Time to First Message

One action is one deliberate operator input or provider-console submission. Reading output, waiting, and daemon-owned `setWebhook` calls count as zero.

| Provider / lane | Hermes baseline | AGH actions | Wall time | Delta / note | Evidence |
|---|---:|---:|---:|---|---|
| Slack manifest | ≈7 | 8 | 1m57s | +1; dashboard import/install simulated; first fake-provider message at 04:46:06Z | `bridge-charter-results.json`; fake Slack log |
| Telegram guided / structured CLI | ≈4 | 5 | 2m19s | +1; intended four-action path required a group+topic retry because of BUG-20260713-telegram-route-shapes. Three deliberate invalid-input probes preceded the timed valid path. | setup JSON and fake Telegram log |
| Telegram HTTP/UDS | ≈4 | 7 | 3m48s | +3; create, two writes, register, verify, enable, send are distinct requests. Wall time includes rebuilding stale local provider binaries before evidence resumed. | HTTP/UDS corpus and fake Telegram log |
| WhatsApp guided setup | unavailable | 5 | not claimed | Four deliberate invalid submissions plus one successful disabled setup; no first-message journey was fabricated. | structured setup output |
| Discord guided setup | unavailable | 2 | not claimed | One malformed-key rejection plus one successful disabled setup; no first-message journey was fabricated. | structured setup output |
| Teams generic | unavailable | not measured | not measured | Deterministic progress/chunk owners passed; no TTFM persona walk was assigned. | bridge-progress-transcript-soak follow-up |
| Google Chat generic | unavailable | not measured | not measured | Deterministic progress/chunk/reply owners passed; no TTFM persona walk was assigned. | bridge-progress-transcript-soak / bridge-restart-fail-open follow-up |
| GitHub issue/comment | unavailable | not measured | not measured | Explicit no-side-effect progress and skipped identity checks passed; no TTFM persona walk was assigned. | bridge-progress-transcript-soak follow-up |
| Linear comment/Agent Session | unavailable | not measured | not measured | Explicit no-side-effect progress and skipped identity checks passed; no TTFM persona walk was assigned. | bridge-progress-transcript-soak follow-up |

The missing four-provider TTFM measurements are an explicit coverage limit, not a green result. `bridge-progress-transcript-soak` owns the all-provider progress soak and `bridge-restart-fail-open` owns the full edit/reply/restart matrix.

## Experiential Lens Pass

| Journey | Usability | Accessibility | Trust | Performance | Resilience | Polish |
|---|---|---|---|---|---|---|
| J-connect-bridge-provider | Provider-specific validation and handoffs were clear; Telegram route choice is structurally blocked. | Strict JSON, HTTP, and UDS paths were fully operable without a browser. | Secrets stayed masked; provider logs recorded no secret-shaped progress body. | Slack and Telegram fake-provider paths reached a real send in under four minutes. | Disabled setup resumed with existing bindings; current-source CLI/API readback agreed. | Slack manifest, Discord invite, and Meta/Telegram instructions named the next external step. |
| J-complete-web-bridge-setup | One create led directly to the persisted Slack manifest and detail checklist. | Controls exposed names and keyboard focus; the headless walk completed through focus/Enter activation. | Dry-run and real-send were distinct; failed reachability showed an actionable remediation. | Create, copy, refresh, resolve, and send responded without observable stalls. | Refresh preserved bindings and enabled/runtime state; transient verification evidence correctly required a fresh check. | Copy feedback, progress count, status pills, delivery ID, target, and remote ID were explicit. |

## Focused Verification Evidence

All commands below were scoped to the owners named by the charters. No monorepo-wide test target or `make verify` ran during task iteration.

| Charter | Focused evidence | Result |
|---|---|---|
| CH-first-slack-response/050/051/057 | CLI setup/manifest/control owners; core secret/control/doctor/webhook owners; Slack/Telegram/Discord/WhatsApp/GitHub/Linear control owners; exact CLI setup+manifest integration | Pass under `-race` |
| CH-bridge-progress-stress | bridges/bridgesdk/redact/toolmeta projection and dispatcher owners; eight provider progress owners; exact subprocess delivery integration | Pass under `-race` |
| CH-long-provider-replies | `ChunkMessage`/`UTF16Len`; Slack/Telegram/Discord/Teams/GChat/WhatsApp formatting and chunking owners | Pass under `-race` |
| CH-mid-turn-bridge-restart | broker checkpoint/metrics/reconcile owners; GlobalDB bridge-delivery store; boot admission; fresh-broker integration; `TestDaemonE2EBridgeDeliveryReconcilesAfterRestart` | Pass under `-race` |
| CH-edit-reply-context | edit family, parent cache, Host API contract; Slack/Telegram/GChat mappings; `TestDaemonE2EBridgeIngressCreatesAndReusesRouteThroughOptedInLowTierContractMock` | Pass under `-race` |
| Documentation | bridge docs conformance 4/4, test-shape checker, Oxfmt on 18 docs, Turbo site MDX/typecheck, `git diff --check` | Pass |

## What Was Fixed

### BUG-20260712-goal-judge-fixture-model: Goal runtime E2E cannot start its configured judge model

- **Symptom:** five Goal lifecycle cases entered `goal_judge_broken` before exercising rejection, pause, approval, or restart behavior.
- **Root cause:** the fixture configured `goal-e2e-judge` but did not advertise it through the mock agent's ACP model option.
- **Fix:** co-shipped the configured judge value in all four Goal fixture agents; production fail-loud negotiation and assertions are unchanged.
- **Regression test:** existing Goal E2E passed 6/6 under `-race`; combined Goal/reasoning owners passed 14/14.

### BUG-20260712-reasoning-evidence-attribution: Runtime reasoning evidence had no stable invocation owner

- **Symptom:** daemon-owned background ACP processes and the intended user session wrote into one per-agent diagnostics JSONL while their process-local ACP session IDs collided.
- **Root cause:** production already supplied distinct `AGH_SESSION_ID` values, but the acpmock writer discarded that owner and the reasoning assertion projected the entire shared file.
- **Fix:** the central writer stamps the process owner and rejects caller-forged ownership; readers select the API-returned AGH session ID exactly before semantic projection. No production launch, API, config, or memory behavior changed.
- **Regression tests:** writer anti-forgery, real subprocess propagation, exact owner/order/fail-closed unit cases, and a concurrent two-process shared-JSONL daemon E2E all pass under `-race`; the exact reasoning owner also passes ten consecutive runs. The fresh full runtime lane remains the final completion proof.

### BUG-0037: Web E2E lane can serve a stale frontend bundle

- **Symptom:** the 70-case browser lane reported eight failures while serving JavaScript older than the source tree.
- **Root cause:** the lane rebuilt only when `web/dist/index.html` was absent, and reduced per-spec environments could discard `AGH_WEB_DIST_DIR`.
- **Fix:** always build current Web sources before the lane and preserve only the machine-controlled E2E variables in reduced environments.
- **Regression tests:** focused Mage 6/6 and Web runtime fixture 17/17 through Turbo.

### BUG-20260712-bridge-e2e-retired-route: Bridge browser scenario still targets the retired two-pane route

- **Symptom:** current-bundle runs searched for removed scope pills and expected catalog and detail simultaneously after navigation.
- **Root cause:** the release scenario had not co-shipped with the current catalog/detail route split and humanized status labels.
- **Fix:** follow public catalog → detail navigation while retaining exact API/UDS/CLI enum assertions.
- **Regression test:** both Bridge Playwright cases passed against the rebuilt current bundle.

## Open Product Issues

### BUG-20260713-telegram-route-shapes: Telegram guided setup rejects valid route shapes

Guided setup always persists `include_group=true` plus `include_thread=true`, while core routing requires every selected dimension on every event. The public direct-message command supplies only a peer and ordinary groups may have no forum-topic thread. Both are rejected before a provider call; group+topic succeeds. Replacing the wizard default with another conjunction would only move the failure. The required decision is a structural route contract that supports alternative Telegram identity shapes while preserving isolation.

### BUG-0028: One kickoff does not activate the declared collaborator graph

The fresh Northstar replay reproduced the prior failure exactly enough to keep the existing ID: six new duplicate unowned tasks, twelve seeded tasks still `ready`, zero runs, nine idle collaborator sessions, one launch-room message, no replies, no artifacts, and no review/disruption cycles. No second evaluator prompt was sent.

## Paper Cuts and Lab Deviations

- The checked-in/ignored local provider executables were older than current source. Telegram health initially failed until the four channel binaries were rebuilt in both source and installed extension directories. Evidence after that point uses current binaries; this was classified as lab state, not a runtime bug.
- The existing `./bin/agh` client was also stale and could not decode current `delivery_defaults.progress`. Current-source `go run ./cmd/agh bridge list --json` read back all six instances correctly; the stale artifact is not cited as product evidence.
- Disabled verification intentionally skips external reachability; enabled HTTP verification returns structured fail records with HTTP 200, while the CLI maps a fail record to exit 1. The Web surfaced the record rather than equating transport success with setup success.
- Generic secret binding still accepts `--secret-value` as a process argument. The revised public docs disclose this and prefer guided hidden input; a safer stdin/file source remains separate work.

## Human Verification and Production-Parity Qualification

No live Slack, Telegram, Discord, WhatsApp, Teams, Google Chat, GitHub, or Linear credential was available. The deterministic evidence exercises AGH's public surfaces and provider wire contracts, but a release candidate should still spot-check one real Slack manifest/import, one Telegram BotFather registration, one Discord Interactions Endpoint, and one Meta webhook verification from a public callback. These checks must confirm vendor-console state and public Internet reachability; they must not replace the automated contract evidence.

## Real-Scenario Evidence

- **Playbook:** `northstar-pay`
- **Operator kickoff:** one in-persona kickoff, session `sess-30ac4158fe73bbb4`, turn `turn-ec30505844bb3ea3`; no follow-up prompt
- **Runtime observation:** stalled after the PM turn; BUG-0028 reproduction recorded in `qa/notes/bug-0028-retest.json`
- **Strict auditor:** `fail`; `qa/qa-audit-report.json`
- **Auditor verdict:** FAIL — the one-kickoff collaboration contract did not complete.
- **Cleared evidence checks:** provider live evidence, required surfaces, provider attempt, and issue recording are present.
- **Remaining blockers:** C6 zero task runs; C8 no playbook object crossed three surfaces; C10 no reused artifacts; C11 no completed disruption probes; C16 no declared deliverables; C17 no collaboration/review/disagreement graph; C14 final `make verify` evidence pending.
- **Production-parity qualification:** Northstar used a live Claude provider turn; external bridge platform calls used deterministic local provider APIs.

## Documentation Outcome

The user-requested Hermes comparison materially changed the docs rather than polishing prose. `setup.mdx` is now the shared operator hub; Slack, Telegram, Discord, WhatsApp, Teams, Google Chat, GitHub, and Linear each have a dedicated how-to; `operations.mdx` owns rotation/recovery/restart behavior; the overview includes a capability matrix; provider READMEs and the public/internal bridge-author guides now reach an install→create→bind→verify→route→send outcome. The docs retain AGH runtime truth, including fake/live boundaries, masking, disabled-first setup, and known CLI secret-input limits.

## Learnings

- Nominal provider coverage is not documentation parity. Hermes is useful because it gives one clear entry point, provider-specific checkpoints, and recovery guidance; AGH now matches that information architecture while keeping stronger typed verification.
- A fixed conjunction of route dimensions cannot model providers with alternative identity shapes. Wizard defaults must be validated against every provider event family, not only the most specific happy path.
- Fake-provider logs become much stronger evidence when paired with public API/CLI/Web readback and explicit secret-shaped-field detection.
- Build freshness is part of E2E correctness. A green or red browser result against stale assets is not product evidence.
- The strict scenario auditor correctly resists green-by-omission: a healthy PM turn is not autonomous collaboration when owned work, peer exchanges, artifacts, reviews, and disruptions never happen.

## AGH Impact Audit

- **Native tools:** no Task 10 tool ID, toolset, descriptor schema, digest, risk flag, or capability gate changed. Checked the generated native catalog and used CLI/HTTP/UDS public bridge controls; current-source decoding matches the daemon. Earlier bridge progress metadata remains covered by the focused projection owners.
- **Extensibility and hooks:** all eight bundled bridge providers' setup/control/delivery contracts were checked through manifests, fake APIs, or exact owners. Public and internal bridge-author docs were expanded. No hook, bundle, MCP sidecar, or config key changed in Task 10. BUG-20260713-telegram-route-shapes identifies a future core routing-contract change rather than an adapter-local patch.
- **Workspace data isolation:** every manual bridge instance was workspace-scoped to `ws_20b83d86cad594f0`; CLI, HTTP, UDS, Web, route/cache, GlobalDB, and daemon-E2E evidence kept instance/workspace identity intact. Focused GlobalDB and parent-cache owners prove cross-workspace isolation; no new global datum was introduced.
- **Official AGH skill:** the bundled `skills/agh/` bridge operations reference was updated in the implementation tasks for progress, limits, and setup behavior. Task 10 introduced no new public command/tool/event requiring another skill change; the revised site docs and official skill were checked for contract consistency.

## Final Status

- **Exit gate:** pending. The single workflow-wide `make verify`, fresh full runtime E2E, and fresh full Web E2E are deliberately reserved for the final tasks/review tail. `BUG-20260712-reasoning-evidence-attribution` is source-fixed with focused concurrent evidence, but no fresh complete runtime result is claimed yet; the first full Web result remains invalidated by BUG-0037.
- **Strict scenario audit:** fail on BUG-0028 collaboration/deliverable requirements plus pending C14.
- **Issues by user impact:** Blocks-Completion 2 open (`BUG-0028`, `BUG-20260713-telegram-route-shapes`) · Friction 0 open / 4 fixed (`BUG-20260712-goal-judge-fixture-model`, `BUG-20260712-reasoning-evidence-attribution`, `BUG-0037`, `BUG-20260712-bridge-e2e-retired-route`) · Data-Loss 0 · Trust-Damage 0 · Cosmetic 0.
- **Coverage:** 9/9 charters terminal — 7 Pass, 2 Blocked (human decision), 0 Pending. Four non-Slack/Telegram provider TTFM measurements remain explicitly unmeasured.
- **Teardown:** pass — `qa/teardown.json` records `"clean": true`, no survivors, and all registered daemon/Web/browser/provider/observer PIDs stopped.
- **Verdict:** **not ready** — bridge contracts are broadly green under qualified fake-provider evidence, but Telegram routing, Northstar autonomy, the runtime gate, and the final global gate remain unresolved.

[QA_BOOTSTRAP]
manifest_path=/home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab
runtime_home=/tmp/aghqa-e192b01b8545/runtime
base_url=http://127.0.0.1:40645
verification_report=/home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/verification-report.md
strict_audit=/home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/qa-audit-report.json
teardown_report=/home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/teardown.json
health_status=teardown-clean
[/QA_BOOTSTRAP]

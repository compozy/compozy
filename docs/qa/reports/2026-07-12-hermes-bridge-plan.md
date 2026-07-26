# Hermes bridge QA plan — 2026-07-12

Planning-only report for Task 09. No QA session ran and no behavior received a passing verdict. This report maps the Hermes bridge implementation into the conflict-resistant living QA tree and defines Task 10's execution contract.

## Scope and tracker impact

Existing scenario history remains in the scenario files. Changed observable behavior is reset to `untested`; historical bugs, fixes, retests, evidence, and reports are preserved.

| Scenario | Planning disposition | Persona / journey | Primary charter |
|---|---|---|---|
| `NB-024` | Catalog and health remain `untested`; verification diagnostics are an adjacent probe | Omar / `J-23` | `CH-bridge-verification-secrets` |
| `NB-025` | Reset for eight-provider setup-capability discovery | Tessa / `J-connect-bridge-provider` | `CH-first-slack-response`, `CH-structured-telegram-setup` |
| `NB-026` | Typed progress and setup orchestration changed creation | Tessa / `J-complete-web-bridge-setup` | `CH-web-bridge-setup`, `CH-structured-telegram-setup` |
| `NB-028` | Progress fields and lossless Web editing changed updates | Tessa / `J-complete-web-bridge-setup` | `CH-bridge-progress-stress`, `CH-web-bridge-setup` |
| `NB-029` | Provider initialization changed operational error ownership | Omar / `J-diagnose-repair-bridge` | `CH-first-slack-response`, `CH-bridge-verification-secrets` |
| `NB-031` | Shared listener shutdown and recovery changed | Omar / `J-recover-mid-turn-restart` | `CH-mid-turn-bridge-restart` |
| `NB-036` | Reset for guided setup and masked read-back | Tessa / `J-connect-bridge-provider` | `CH-first-slack-response`, `CH-structured-telegram-setup` |
| `NB-037` | Reset for strict wizard and structured secret setup | Tessa / `J-connect-bridge-provider` | `CH-first-slack-response`, `CH-structured-telegram-setup` |
| `NB-038` | Reset for setup cleanup and resume | Tessa / `J-connect-bridge-provider` | `CH-first-slack-response`, `CH-structured-telegram-setup` |
| `NB-039` | Dry-run target resolution must remain distinct from real send-test | Tessa / `J-complete-web-bridge-setup` | `CH-web-bridge-setup`, `CH-structured-telegram-setup` |

Task 09 completes ownership for the content-addressed Hermes scenarios introduced by Tasks 01–06:

| Scenario | Canonical invariant | Persona / journey | Primary charter |
|---|---|---|---|
| `NB-bridge-tool-progress` | Ordered, redacted, coalesced progress with transcript purity | Maya / `J-watch-agent-work-channel` | `CH-bridge-progress-stress` |
| `NB-long-bridge-replies` | Lossless long replies across six chat-provider wire limits and markup dialects | Omar / `J-deliver-long-formatted-reply` | `CH-long-provider-replies` |
| `NB-provider-progress-rendering` | Provider-native progress policy across all eight providers | Maya / `J-watch-agent-work-channel` | `CH-bridge-progress-stress` |
| `NB-bridge-provider-setup` | Setup, manifest, verification, webhook registration, and real send through public surfaces | Tessa / `J-connect-bridge-provider` | setup charter trio plus security probe |
| `NB-web-bridge-setup` | Truthful Web setup checklist and remediation | Tessa / `J-complete-web-bridge-setup` | `CH-web-bridge-setup` |
| `NB-bridge-edit-reply` | Inbound edit intent and bounded reply-to context without isolation bleed | Maya / `J-edit-reply-context` | `CH-edit-reply-context` |
| `NB-bridge-restart-recovery` | Visible fail-open recovery after restart, durable metrics, and no text replay | Omar / `J-recover-mid-turn-restart` | `CH-mid-turn-bridge-restart` |

All remain `untested` until Task 10 records public-surface evidence.

## Provisional seed consolidation

The numbered `NB-047..NB-073` names in `.compozy/tasks/hermes-bridge/_qa.md` were provisional seeds written before the living-docs migration. They do not allocate or overwrite tracker IDs. Each seed is folded into the content-addressed scenario that owns its invariant:

| Provisional seed | Intended behavior | Living scenario owner |
|---|---|---|
| `NB-047` | Generate Slack app manifest | `NB-bridge-provider-setup` |
| `NB-048` | WhatsApp wizard validation/remediation | `NB-bridge-provider-setup` |
| `NB-049` | Telegram wizard and daemon `setWebhook` | `NB-bridge-provider-setup` |
| `NB-050` | Discord wizard and invite URL | `NB-bridge-provider-setup` |
| `NB-051` | Agent-only, non-TTY setup | `NB-bridge-provider-setup` |
| `NB-052` | HTTP/UDS webhook registration | `NB-bridge-provider-setup` |
| `NB-053` | Structured verification records | `NB-bridge-provider-setup` |
| `NB-054` | Bridge doctor category | `NB-bridge-provider-setup`, with `NB-024` as catalog diagnostic |
| `NB-055` | Real send-test versus dry-run | `NB-bridge-provider-setup`; dry-run remains `NB-039` |
| `NB-056` | Slack progress rendering | `NB-bridge-tool-progress`, `NB-provider-progress-rendering` |
| `NB-057` | Telegram progress rendering | `NB-bridge-tool-progress`, `NB-provider-progress-rendering` |
| `NB-058` | Discord progress rendering | `NB-bridge-tool-progress`, `NB-provider-progress-rendering` |
| `NB-059` | Teams, Google Chat, and WhatsApp opt-in | `NB-provider-progress-rendering`; config lifecycle also uses `NB-026`/`NB-028` |
| `NB-060` | Typed progress block lifecycle | `NB-provider-progress-rendering`; create/update truth uses `NB-026`/`NB-028` |
| `NB-061` | Progress transcript purity | `NB-bridge-tool-progress` |
| `NB-062` | Progress storm throttling/coalescing | `NB-bridge-tool-progress` |
| `NB-063` | Long-reply channel limits | `NB-long-bridge-replies` |
| `NB-064` | Slack mrkdwn fidelity | `NB-long-bridge-replies` |
| `NB-065` | Telegram MarkdownV2 fidelity | `NB-long-bridge-replies` |
| `NB-066` | Inbound edit routing | `NB-bridge-edit-reply` |
| `NB-067` | Threaded reply context | `NB-bridge-edit-reply` |
| `NB-068` | Restart-mid-turn recovery | `NB-bridge-restart-recovery` |
| `NB-069` | Durable delivery metrics | `NB-bridge-restart-recovery` |
| `NB-070` | Web setup orchestrator | `NB-web-bridge-setup` |
| `NB-071` | Web progress fields | `NB-web-bridge-setup`; persistence truth uses `NB-026`/`NB-028` |
| `NB-072` | Eight-provider documentation parity | `NB-bridge-provider-setup` |
| `NB-073` | Secret redaction in progress previews | `NB-bridge-tool-progress` |

This preserves one living verdict per invariant while the charters retain provider-specific examples.

## Journey inventory

| Journey | Value | Persona | Abandonment and resume |
|---|---|---|---|
| [`J-connect-bridge-provider`](../journeys/J-connect-bridge-provider.md) | Connect any of eight providers and receive one real provider-visible response | Tessa, Ada | Stop after credential-shape failure; reopen the same disabled instance with bindings masked |
| [`J-watch-agent-work-channel`](../journeys/J-watch-agent-work-channel.md) | Observe bounded provider-native progress without transcript pollution | Maya | Leave during a tool storm; return to a terminal provider outcome and clean transcript |
| [`J-diagnose-repair-bridge`](../journeys/J-diagnose-repair-bridge.md) | Diagnose a degraded bridge through structured actionable checks | Omar | Resume from persisted health and the named unresolved check |
| [`J-deliver-long-formatted-reply`](../journeys/J-deliver-long-formatted-reply.md) | Receive a complete formatted answer across provider limits | Omar, Maya | Return to the ordered complete answer or a truthful terminal failure |
| [`J-recover-mid-turn-restart`](../journeys/J-recover-mid-turn-restart.md) | Recover visibly after restart without replaying persisted answer text | Omar, Maya | Restart is the interruption; recovery emits one scoped terminal error |
| [`J-complete-web-bridge-setup`](../journeys/J-complete-web-bridge-setup.md) | Complete browser setup from daemon-owned facts and remediation | Tessa | Return after failed verification to the same instance and current checklist |
| [`J-edit-reply-context`](../journeys/J-edit-reply-context.md) | Correct a message and reply with bounded already-observed context | Maya | A cold cache stays empty and never triggers provider history fetches |

GitHub and Linear end the setup and observation journeys in their supported issue, comment, or Agent Session surfaces; they are not modeled as chat providers.

## Immutable charter roster

| Order | Charter | Journey | Persona | Tour | Time box | Primary risk |
|---|---|---|---|---|---|---|
| 1 | [`CH-first-slack-response`](../charters/CH-first-slack-response.md) | `J-connect-bridge-provider` | Tessa | Feature | 90m | Slack manifest handoff and first response |
| 2 | [`CH-guided-setup-credentials`](../charters/CH-guided-setup-credentials.md) | `J-connect-bridge-provider` | Tessa | Paste | 90m | wrong-product credential remediation |
| 3 | [`CH-structured-telegram-setup`](../charters/CH-structured-telegram-setup.md) | `J-connect-bridge-provider` | Ada | Feature | 90m | strict JSON plus HTTP/UDS parity |
| 4 | [`CH-bridge-progress-stress`](../charters/CH-bridge-progress-stress.md) | `J-watch-agent-work-channel` | Maya | Garbage | 90m | pressure, redaction, policy, transcript purity |
| 5 | [`CH-long-provider-replies`](../charters/CH-long-provider-replies.md) | `J-deliver-long-formatted-reply` | Omar | Paste | 90m | wire units, fences, markup, reconstruction |
| 6 | [`CH-mid-turn-bridge-restart`](../charters/CH-mid-turn-bridge-restart.md) | `J-recover-mid-turn-restart` | Omar | Interrupt | 90m | fail-open recovery, stale anchors, isolation, metrics |
| 7 | [`CH-web-bridge-setup`](../charters/CH-web-bridge-setup.md) | `J-complete-web-bridge-setup` | Tessa | Back button | 90m | Web truth after failure, reload, and navigation |
| 8 | [`CH-edit-reply-context`](../charters/CH-edit-reply-context.md) | `J-edit-reply-context` | Maya | Interrupt | 60m | edits, replies, cold cache, bounded context |
| 9 | [`CH-bridge-verification-secrets`](../charters/CH-bridge-verification-secrets.md) | `J-diagnose-repair-bridge` | Omar | Garbage | 90m | hostile verification targets and secret masking |

Charters are immutable once execution begins. Task 10 writes every observation, verdict, evidence link, and debrief into its dated report, never back into these files.

## Time-to-first-message protocol

Task 10 measures from an installed provider extension and no bridge instance to the first externally visible real AGH response. One operator action is one deliberate operator input or one external provider-console submission/paste. Reading output, waiting, and automatic provider calls do not count.

- Slack compares against the Hermes baseline of approximately seven operator actions.
- Telegram compares guided and structured paths against approximately four operator actions; daemon-owned `setWebhook` counts as zero.
- WhatsApp, Discord, Teams, Google Chat, GitHub, and Linear record actions and wall time without inventing a threshold.
- A dry-run `test-delivery` result never ends the measurement; the end state is a real provider message, issue/comment response, or Agent Session result.

Every measurement records actions, timestamps, public observables, remediation loops, total actions, elapsed time, and any production-parity deviation.

## Risk-directed hunts

| Risk | Charter owner | Required observable |
|---|---|---|
| progress spam, throttling, deduplication, terminal loss | `CH-bridge-progress-stress` | bounded calls, ordered terminal state, no per-tool storm |
| secret leakage in previews, setup, or verification | progress and security charters | raw values absent; only redacted or masked metadata |
| transcript pollution | `CH-bridge-progress-stress` | provider chrome visible externally and absent from persisted ACP history |
| provider-policy mismatch | `CH-bridge-progress-stress` | editable defaults, explicit opt-ins, and issue-provider no-side-effect behavior |
| split-unit or markup rejection | `CH-long-provider-replies` | every wire body under cap and losslessly reconstructable |
| silent half-answer or text replay after restart | `CH-mid-turn-bridge-restart` | one visible terminal error and no answer replay |
| scope/workspace/conversation bleed | restart and edit/reply charters | exact ownership before any side effect |
| wizard false acceptance or partial secret writes | setup charters | named remediation before provider calls; safely resumable state |
| stale optimistic Web checklist | `CH-web-bridge-setup` | every check traces to current daemon facts after reload |
| SSRF or unsafe redirect probing | `CH-bridge-verification-secrets` | unsafe destinations refused with structured output |
| documentation drift | `CH-bridge-verification-secrets` | all eight guides match real slots, commands, callbacks, and troubleshooting |

## Automation candidates

- [`bridge-web-setup-remediation`](../automation-backlog/bridge-web-setup-remediation.md): real-daemon Web setup and remediation.
- [`bridge-progress-transcript-soak`](../automation-backlog/bridge-progress-transcript-soak.md): eight-provider progress-storm soak.
- [`bridge-restart-fail-open`](../automation-backlog/bridge-restart-fail-open.md): serialized recovery matrix.
- [`bridge-time-first-message`](../automation-backlog/bridge-time-first-message.md): action-count replay and baseline reporting.

## Task 10 environment and evidence contract

1. Bootstrap a fresh isolated lab with `agh-qa-bootstrap`; do not reuse another cycle's manifest.
2. Honor unique `AGH_HOME`, ports, provider homes, and tmux-bridge sockets. Export `AGH_WEB_API_PROXY_TARGET` from the manifest.
3. Register every long-lived daemon, Web server, browser, watcher, and tmux process under `<QA_OUTPUT_PATH>/qa/pids/`.
4. Create one dated execution report with every charter initially `Pending`; append observations immediately after each journey checkpoint.
5. Keep bulk logs and captures under `QA_OUTPUT_PATH`; cite only evidence that supports a verdict or failure.
6. Drive the structured Telegram charter entirely through CLI/HTTP/UDS output, without Web or prose parsing.
7. Capture browser evidence for the Web setup charter and cite it from the execution report.
8. Deduplicate failures against `docs/qa/bugs/`; new bugs use `BUG-<YYYYMMDD>-<slug>.md` and include reproduction, observed/expected behavior, severity, and evidence.
9. Update each living scenario file from session evidence; never edit or commit the generated `state.csv` view.
10. Run the required serialized `make test-e2e-runtime` and `make test-e2e-web` lanes as supporting evidence, not substitutes for persona verdicts.
11. On every terminal path, execute the manifest `TEARDOWN_COMMAND` or `make qa-reap`. Completion requires `teardown.json` with `"clean": true` and no surviving lab processes.

## AGH Impact Audit

- **Native tools:** no new tool ID is introduced by this planning task. The structured charter verifies CLI/HTTP/UDS bridge behavior and checks any exposed native descriptors and capability diagnostics for parity.
- **Extensibility and hooks:** setup begins at installed bridge extensions and covers all eight provider capabilities, secret slots, manifests, routing, and lifecycle behavior. Unrelated bundles, hooks, and sidecars are unchanged.
- **Workspace data isolation:** bridge instances and durable delivery state retain their global/workspace ownership. Restart and edit/reply charters probe workspace, scope, instance, route, conversation, cache, and event boundaries.
- **Official AGH skill:** the security/verification charter compares bundled `skills/agh/` bridge guidance with public commands and provider guides. This task changes QA planning artifacts only.

## Planning completeness

- Every new journey has a Mermaid flow, concrete observables, a true end state, and abandonment/resume behavior.
- Every provisional seed maps to one living scenario without allocating a duplicate verdict.
- Every changed scenario has a persona, journey, and bounded charter owner.
- Every charter uses one tour and declares inputs, observables, avoidance rules, evidence expectations, and a time box.
- Slack and Telegram TTFM comparisons are explicit; other provider measurements do not invent baselines.
- Functional, experiential, edge/error, cross-cutting, and journey dimensions are covered or deliberately bounded.
- No QA execution or pass/fail claim occurred during planning.

# QA Run Report — 2026-09-06 — sessions-stability

- **Scope:** Integrated sessions-stability tasks03–08 and invalidated task01/02 branches; selected scope and evidence reuse in `.compozy/tasks/sessions-stability/memory/qa.md`.
- **Cadence tier:** targeted.
- **Build:** ca00890294bc99efa76f7c5eea71ac0ded321766 plus current working changes; fresh binary built for this run. Final commit and gates remain with the delivery workflow.
- **Environment:** `/Users/pedronauck/dev/qa-labs/compozy-sessions-stability-integrated-20260906-102255-367122-lab/qa-artifacts/qa/bootstrap-manifest.json`; HTTP `http://127.0.0.1:61085`. Native CLI providers retain the operator home; no operator logout or credential mutation is planned.
- **Started:** 2026-09-06T10:22:21.895237+00:00 · **Status:** completed with the external provider branch blocked.
- **Parity:** Live runtime/browser evidence and deterministic visual fixtures are recorded separately. Earlier real Claude steering evidence is reused only for its unchanged capability/delivery contract.

## Personas and flows

Desktop, Wi-Fi, en-US. Théo follows live work and returns to long transcripts (J-11, J-13, J-14); Ada controls and recovers sessions through CLI/APIs (J-15); Bruno recovers scheduled work (J-24); Dora checks provider settings (J-22). Existing journey maps and immutable charters own the full missions; this run selects the changed branches in the QA handoff.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix delivery |
|---|---|---|---|---|---|---|---|
| 1 | CH-managed-session-intervention | J-13 / RT-019, RT-session-live-steer | Théo | Feature Tour | Pass | BUG-20260906-injected-guidance-missing-history; queue/live promotion repairs | Included in this change |
| 2 | CH-herdr-session-orchestration | J-15 / RT-session-prompt-cancel, RT-session-native-stop | Ada | Interrupt Tour | Pass — selected local scope | Native stop, self/foreign denial and repeated caller passed | Included in this change |
| 3 | CH-session-prompt-identity | J-11 / prompt identity and reload | Théo | Interrupt Tour | Pass | BUG-20260906-promoted-turn-missing-live; lost-ack retry passed | Included in this change |
| 4 | CH-visible-session-window-streams | J-13 / RT-visible-session-streaming | Théo | Multi-Tab Tour | Pass | BUG-20260906-session-header-transport; current-watermark recovery | Included in this change |
| 5 | CH-acp-stream-disconnect-recovery | J-15 / provider disconnect | Ada | Network Tour | Pass — transport recovery | Exhausted retries, draft guard and actual Try again passed | Included in this change |
| 6 | CH-schedule-recovery-guard | J-24 / TA-schedule-catchup-overlap | Bruno | Interrupt Tour | Pass — selected capacity/restart scope | BUG-20260906-deferred-schedule-drain | Included in this change |
| 7 | CH-crash-resume-compaction | J-11 / RT-session-context-rebuild | Théo | Interrupt Tour | Pass — selected compaction/recovery scope | Profile resume, startup hooks and repeated boundaries repaired | Included in this change |
| 8 | CH-session-calm-transcript | J-14 / ET-web-session-transcript-calm-grammar | Théo | Feature Tour | Pass | 29 integrated visual rows and exact field/fold re-walks | Included in this change |
| 9 | CH-session-history-navigation | J-14 / full history navigation | Théo | Feature Tour | Pass | Full history, archive invalidation and complete download | Included in this change |
| 10 | CH-session-permission-dock | J-answer-agent-requests / decision canary | Théo | Feature Tour | Pass | BUG-20260906-native-profile-agent fixed | Included in this change |
| 11 | CH-untested-020-22-dora | J-22 / provider auth canary; RT-018 auth/rate error | Dora | Feature Tour | Blocked — real auth lapse/rate limit | Isolated real account and controllable provider failure required |  |

## Session Debriefs

### CH-managed-session-intervention — Théo — initial CLI/API leg (historical; repaired below)

- **Ran:** 10:24–10:28 UTC; stopped this leg after the persisted-guidance defect.
- **Findings:** interrupt preserved both parked entries; independent outline shows replacement then index then glossary in order. Real Claude injection returned the same turn, but cold transcript/search/outline lost the authored guidance.
- **Bugs filed:** BUG-20260906-injected-guidance-missing-history (Data-Loss).
- **Scenarios settled:** RT-session-live-steer fails this changed persistence branch; the full browser/queue leg remains Pending for repair and re-walk.
- **Paper cuts:** CLI prompt examples advertise `--expected-turn-id` while help exposes `--expected-turn`; retain for correction in this scope.
- **Next:** preserve the delivered authored event and verify exact identity through canonical manager coverage, then repeat with a fresh provider session.


## Visual contract evidence

All 29 integrated rows pass with their repaired substates and row-specific review. Task01 VC-01..05 and task02 VC-01..02 were recaptured because later composer/timeline changes invalidated their geometry. Task01 VC-06 is reused: its settings control layout and styling remain unchanged, including after the later event-handling-only Settings navigation repair. All 37 bundles carry the required artifacts; visual diagnostics do not establish parity by themselves. The new full-thread callback fixtures are visual evidence, separate from the real provider/runtime walks. Pending-stop status was corrected to match the guarded composer intent.

## What Was Fixed

Integrated findings repaired and re-walked: injected guidance missing from cold history; stopped history migration and pooled navigation capabilities; profile-bound session resume; startup-hook compaction barriers; specialized-field navigation and nested scroll landing; lost-acknowledgment feedback; stale REST transcript replacement and zero-height virtual rows hiding the latest promoted answer; native policy resolution for profile-only agents. Individual bug records retain the owning red/green and public re-walk evidence. Header transport context and current-watermark reconnect confirmation were also repaired and re-walked.

## Paper Cuts

Recorded in the repair checkpoints and bug records below.

## Runtime Errors Observed

Expected injected network failures, controlled-driver unmatched-input/setup failures and model-catalog auth/timeouts are retained in lab logs. Repaired user-facing cases were re-walked; capture records state browser errors explicitly. No fixture failure is counted as successful provider execution.

## Human Verifications Needed

Real-provider auth-loss/rate-limit and successful reauthentication within the cache TTL remain unverified. The operator account is logged in and real Claude prompts succeeded; the Compozy native auth probe truthfully returns unknown because no auth_status_command is configured. Reproduction requires a separate real-provider account/session whose authentication or rate limit can be controlled. No operator logout or credential mutation was performed. Owning protocol/cache suites remain valid; they do not replace this branch.

## Decisions for a Human

None established.

## Learnings

The bootstrap feature profile imposes unrelated release-wide agent/channel minimums. This bounded branch run uses an explicit targeted profile with all five selected surfaces, preserving the task09 scope; provider-backed proof remains a requirement of the selected journeys even though targeted scaffolding does not impose it globally.

## Final Status

Selected integrated walks and all 37 visual rows are complete, with the real auth-loss/rate-limit branch explicitly blocked as described above. Manifest teardown completed with `clean: true`; evidence is retained at the manifest’s sibling `teardown.json`. Final diff review, local gate, commit and current-head PR CI remain owned by the enclosing delivery workflow; this QA closeout is not a release-readiness claim.

## Repair and re-walk checkpoints

These entries are chronological. Earlier pending/failing observations are superseded by the final result table and subsequent repair evidence; they are retained as the investigation record.

- Real Claude handbook steering: injected message msg_handbook_guidance_02 retains its authored text/turn in the public transcript after settlement and daemon restart. Web shows one bubble and Steered — delivered into the live turn. Search/outline independently find the sequence/turn after the stopped-reader repair. BUG-20260906-injected-guidance-missing-history is fixed; remaining queue/browser branches of charter1 are still pending.
- Upgrade/restart exposed BUG-20260906-stopped-history-schema-upgrade (old stopped histories bypassed writable migration) and BUG-20260906-stopped-session-navigation (pooled reader omitted new navigation capabilities). Both were repaired in their owners and re-walked against the same retained histories. The SQL migration preserves authoritative event bytes and stable identities; canonical migration/open, boot and real-pool integration evidence is recorded in task_10.md.
- Long-history setup completed through 1,510 public CLI prompts to a real controlled ACP subprocess (not a real-provider capability claim). An independent paginated HTTP read counted3,023 distinct messages, with incident0000 and incident1509 both retained; outline returns1,510 messages, and literal search finds incident0000 outside the latest page. Evidence: navigation-prompts.jsonl, navigation-progress.json, navigation-history-audit.json, navigation-outline-reloaded.json and navigation-search-reloaded.json. Browser navigation/focus/scroll checks are still running.
- Provider canary: actual Claude CLI auth status reports loggedIn=true, native claude.ai/firstParty/Max. Compozy provider controls truthfully return classification unknown because no auth_status_command is configured; remote probe explains that prerequisite. Successful real Claude prompts already establish usable authentication. No operator logout, forced auth-loss or rate-limit recovery was performed; those branches remain unverified.

- Lost acknowledgment after real daemon acceptance: browser I/O fault discards only the first successful202 response for message34b3eea6-a485-4f3a-ab80-363b64f55eac/key590b5171-1034-4d99-b715-fa88d7ccda66. Web displays Not confirmed with Retry. Actual Retry sent the byte-identical request, server returned the same entryinq-b9ec31d2fdc2808a with replayed=true. Stop generation then drained both parked entries; independent transcript retains exactly one authored user and one assistant for the retried identity. Evidence navigation-lost-ack-accepted.json/png, navigation-lost-ack-retried.json/png, navigation-retry-dispatched-transcript.json. Delivery/idempotency passes. The later zero-height virtual-row correction and fresh browser promotion closed the latest-answer issue; see the checkpoint below.

- Full-payload search re-walk passed: line251 of260 is highlighted inside all nested clipping boxes at viewport y279.49, find retains focus,1of1, errors[]. Earlier specialized input Unicode proof remains valid. BUG-20260906-find-specialized-tool-field fixed; navigation-large-output-final.json/png inspected.

- Fresh promotion after virtual-row repair passed without a reload or another turn: user012a95c6-7f0c-4401-8b4e-d74f6b8993ef and assistantturn-e92f46785c8c9832 appear at the actual bottom; answer y514.26–592.76 stays inside the viewport, errors[]. Public transcript agrees. Evidence navigation-promote-measurement-fixed.json/png and -transcript.json; PNG inspected. BUG-20260906-promoted-turn-missing-live fixed.
- Native clarification canary passed after profile policy resolution was repaired: real Question dock / waiting-for-input → coordinate Staging choice → CLI tool completed choice0/fallbackfalse → answered marker and running state. Evidence clarification-profile-answer-result.json, clarification-dock-pending.json/png and -answered.json/png. Earlier unanswered attempts canceled and are not passing answers.
- Permission canary passed with isolated decision-operator using the supported approve-reads mode and a controlled ACP edit permission request. Session sess-ee056d23de777045, turn-e54da459325e89e1, interaction int_01M1VDFM96F2ZEKPYPM7QY5JC2. Pending interaction survived a page reload and appeared in the composer dock beside Waiting for your decision. Coordinate Allow once yielded the canonical permission receipt and resumed/finished the same prompt; driver observed allow-once, Web idle, errors[]. Evidence decision-agent.json, decision-session.json, decision-pending-status.json, decision-driver.json, logs/decision-prompt.log, permission-dock-pending.json/png, permission-dock-answered.json/png and permission-final-transcript.json. Both PNGs inspected. The fixture emits its selected decision before its final prose; that fixture concatenation is not product copy.
- Transport leg: real browser URL blocking caused six actual SSE errors, retained the visible transcript and draft, and showed the terminal retry marker. Unblock + actual Try again restored missing public messages without auto-sending the draft. The header did not show the matching chip, tracked in BUG-20260906-session-header-transport; remaining two-window/pause and send-guard branches stay pending. Full browser offline mode alone did not sever an existing SSE socket, so that attempt is not treated as a reconnect failure.

- Final transport recovery passed after both fixes: hidden/restore starts a quiet grace with no
  chip; after 3s the actual header shows Reconnecting ·4; six failures show Disconnected and
  Try again. Coordinate Queue immediately shows Not sent with the exact offline draft retained.
  Unblock + coordinate Try again returns live at the unchanged cursor4651 within the500ms
  observation, without another transcript event or auto-send; errors[]. Evidence
  transport-fixed-grace.json/png, -retry-count.json/png, -draft-guard.json/png and
  -recovered.json/png; the latter three PNGs inspected. Native and localhost-proxied SSE both
  confirm the head with the same empty transcript_delta. Two simultaneous live windows and
  quiet/capacity branches remain pending; this is not a blanket charter pass.
- Queue Clear all passed: Keep preserved both rows and the exact offline draft; confirmed
  Clear all removed both rows. Independent queue-cleared-public.json returns inputs=[],
  queue.entries=0/cap10; queue-cleared-web.png inspected. Durable clear trace audit remains
  pending separately.

- Final queue trace re-walk passed at14:41 UTC: three public submissions, actual browser
  removal of the first (3→2), then actual Clear all confirmation (2→0). Web retains one
  neutral single-removal receipt and one clear cluster ×2 while later assistant checkpoints
  continue. Reload preserves both receipts; the independent public queue is empty and the
  transcript retains all per-entry identities. Evidence queue-trace-{before,after-remove},
  queue-trace-final-{public,transcript,web,reloaded}.json and queue-trace-final-web.png
  (root inspected). The editor now reserves enough height for the canonical Textarea and
  both Save/Cancel actions; implementation-editing.png inspected with both fully visible.
- Final inactivity re-walk passed: fresh public turn4b5522421e45fd48 shows Stopped after1m9s,
  no work41s, with no Session failed notice; daemon stop_cause=inactivity, verified/escalated.
  Evidence quiet-final-verified-{public,transcript,web}.json and -web.png (root inspected).
- Scheduled deferral repair passed a real daemon restart: original unstarted one-shot and
  recurring fire IDs run_a8075bbe7a38adcb199f2adc / run_c92696f85965e7f0ac31b77a completed at
  attempt1 after the occupied daemon drained and restarted. See schedule-fixed-* and the
  fixed BUG-20260906-deferred-schedule-drain record; the recurring job was then disabled.
- Mechanical capacity walk passed: owner-bound task-f96b40d84205ae2e waited on the prompting
  sess-b36e351dbe08b6f6. With isolated config thresholds1/2/3/4 and min age1s, its run
  run-6fa9978364170747 emitted exactly one scheduler.capacity_waiting_escalated at37.5s,
  then needs_attention at53s with reason prompting after4 cycles. Once the holder settled,
  public task run recover created linked attempt2 run-40e8deb3e101f15d and public session-bound
  task next claimed it. Recovery intentionally creates a successor; it does not reuse the
  parked run. Evidence capacity-bound-{detail,scheduler-status,run-recovered,recovery-claimed,
  final-detail}.json. The initial unbound task is setup evidence only. Spawned controlled
  fixtures rejected unmatched wake text; those provider replies are not a successful task
  execution claim. Owning task05 integration covers completion/exhaustive escalation.

## Final navigation and visual reconciliation

- Title, error and filename searches highlight the exact materialized field and retain Find focus: navigation-{title,error,filename}-field-web.json/.png and navigation-field-search-*.json; root inspected all three. Specialized input and line251 output proofs remain valid.
- Actual browser Download wrote Bash-output.txt: 6,774bytes, all260lines, first/last lines and payload needle ação λ café retained. SHA256 f611bf90114e46efb8acd152c895030f5a15c75dd13260c0cf397f4e654bae5b; navigation-download-verified.json owns the audit.
- Repeated compaction initially exposed stopped/interleaved turn boundaries. After the repair, the same open Find query changed from1match at generation1 to No matches at generation2 without losing focus or the unsent draft. The new active authored/assistant identity remains visible;200previous active event contents compare exactly after archival. See BUG-20260906-compaction-repeat-boundaries and find-invalidation-audit.json, navigation-find-invalidation-{before,after}.json/.png.
- Two visible sessions independently received checkpoints through focus changes. Hiding the neighbor removed only its source; restoring reopened its own cursor source and caught up. two-windows-final-{start,focus-neighbor,focus-history,hidden,restored}.json/.png record geometry, sources and text. Root inspected focus-history and restored PNGs; errors[] in the retained records. Earlier scroll ownership, trail, no-match and long-history landing evidence remains current.
- Latest canonical thread check passes129tests including pending-stop status. Compaction race selection passes3.656s. These are repair evidence, not the final delivery gate.

- Provider controls canary passed in the real Settings editor: Claude native_cli reports unknown with its available login CLI and empty status command. Native CLI shows no credential slot; switching the unsaved draft to Bound secret exposes the slot fields, and returning to Native CLI removes them. Cancel discarded the draft; no provider save or credential action was sent. provider-canary-{overview,native-editor,bound-editor,restored-editor}.json/.png retain this evidence. Initial route loading recovered after a normal page reload; it is not a provider-auth failure claim. The separate real auth-loss/rate-limit/cache-recovery branch remains externally unverified.

- Final operator-stop re-walk passed: the actual Stop generation click on turn `turn-46ff547edf100939` immediately rendered Stopping in both status row and guarded composer, with Queue available and the draft retained. Public transcript records prompt_cancel followed by forced escalation and `session.turn_quiesced` with verified=true, escalated=true, elapsed_ms=10002 and stop_cause=user_requested. Runtime automatically recovered at generation2; the later inactivity stop belongs to the recovered idle session. `stop-status-final-{web,status,transcript}.json`, the pending PNG and `stop-status-settled-web.json/.png` distinguish both episodes. Both PNGs were inspected; no Stopping label remained after settlement.

## Final controller review repairs

- BUG-20260906-raw-stream-resumed-stop: the existing real HTTP/SQLite reconnect suite reproduces a historical terminal frame closing an active resumed raw stream, then passes after current-status termination replaces historical-event termination. New prompt and replay frame IDs exactly match durable events. Focused HTTP selection15.011s; core unit and tagged integration pass.
- BUG-20260906-queue-edit-concurrent-draft: the composer reads the draft at rejection time and applies explicit recovered text to both persistence and runtime. Existing canonical interaction suite129/129 passes; core payload test0.751s proves dispatching and already-sent responses. The actual browser reproduced a real sent409 without its recovery code, then passed with daemon77460: hold original PUT, type concurrently, publicly cancel/drain, release the same PUT, observe actionable409 and both complete text blocks in order. Page reload retains both. Evidence review-queue-fixed-{accepted,pending,cancel,dispatched,browser,reloaded}.json and browser.png. The controller inspected the image. DOM paragraphs own the text check because innerText adds browser layout newlines; no expected authored text was weakened.
- The lab was reopened only for the invalidated queue-edit journey; earlier37 visual rows and other actual walks remain valid. The second manifest teardown completed at 2026-09-06T16:12:33Z with clean=true and survivors=[]; the previous clean record remains in evidence/teardown-before-final-review.json.

- BUG-20260906-empty-log-head-cursor: an empty live-only log stream no longer synthesizes a wall-clock cursor that drops newly persisted older-timestamp events. The owning real observer/SQLite/HTTP integration waits for the actual empty head, writes a dated event and verifies its exact timestamp and positive SSE sequence. Final HTTP/UDS selections pass12.803s/4.784s in `.cache/sessions-final-gate-stream-integration-green.log`.


## PR #557 diagnostic and CI repairs

- React Doctor 0.9.13 reproduced the PR's 2 errors and 14 warnings (84/100). The final changed-file scan reports 0 errors, 0 warnings and 100/100, with no suppression or rule/config change. Component and view-model extraction preserves existing interaction ownership; Find exposes its empty result as status outside the listbox. Reveal progress now has a per-instance clock, including cancellation fencing for stale animation callbacks. Root Turbo passed the 72 focused navigation, topbar, Find, tool, transport, thinking and reveal tests.
- The verifier browser failure had two Web visibility owners: settled folds could absorb the warning, and the empty-message filter discarded a standalone marker message. Both now preserve unresolved file-mutation evidence. The existing timeline suite proved typed and persisted marker visibility; the existing thinking suite proved standalone warning visibility without reply actions. Their combined root Turbo run passed 50 tests.
- The unchanged daemon-served `session-hardening.spec.ts` title/verifier journey passed in 19.3s against the rebuilt current source. It asserts the generated title, warning tone, marker kind and failure summary. The controller inspected `session-auto-title-verifier-marker.png`: the warning remains below the completed answer with work collapsed. Evidence is retained under `.cache/sessions-marker-passed-browser/`, with the run in `.cache/sessions-ci-browser-green.log`. The fixture disposed its daemon and the wrapper released the shared verification lock.
- Real daemon transport-storm (50,000 events) and blocked-cancel regressions passed in 50.115s. Raw-stream callers now supply the API's required bounded limit. Sandbox cancellation now retains the terminal creation acknowledgement so cleanup can release the resource; the unchanged real cancellation journey and owning cleanup suite passed 20 repetitions. Native tool metadata includes session search, outline and inputs-clear, including the public `q` preview argument. The owning metadata/harness race suites and Windows mock-driver build passed.
- These repairs do not change the separate external provider auth-loss/rate-limit/cache-recovery validation limit documented above. Required CI for the next commit remains pending until GitHub reports its result.


### Follow-up: constrained transport storm

The current-head runtime CI exposed a remaining timeout in the unchanged 50,000-event storm.
A one-core local run reproduced the same 60s request deadline (76.24s including cleanup);
the ordinary two-core run had passed. A three-second CPU sample attributed the active work to
canonical redaction regex scans. The assignment expression requires `:` or `=`, but it scanned
long unstructured text even when neither character existed. `exactRedactString` now checks that
necessary condition before invoking the same expression; rule order and replacements are unchanged.

The rebuilt daemon passed the same one-core storm in 54.72s, preserving all 50,000 chunks, terminal
completion and the single slow-watcher degradation record. The request timeout and fixture were
unchanged. Evidence: `.cache/sessions-storm-profile.log` (failure),
`.cache/sessions-storm-daemon-sample.txt` (profile), `.cache/sessions-storm-fixed.log` (pass), and
retained Go artifacts under `.cache/sessions-storm-profile-artifacts/`. The fixture disposed its
processes and the wrapper released the machine verification lock.

The canonical redaction race suite passes in 3.059s, including a secret assignment after 64KiB of
streamed text. A before/after SHA256 comparison of 800 fixed combinations (canonical/heuristic
shapes, registered literal secrets, prefixes, suffixes and both heuristic settings) is identical.
The existing benchmark, measured sequentially with one core and 10 samples per case, improves
64KiB plaintext median from 42.21ms to 29.46ms (30.2% less time). JSON envelopes retain their
mandatory scan and remain approximately 52ms; short provider messages improve from 41.29µs to 29.97µs.
Earlier concurrent exploratory timings are not the reported benchmark. See
`.cache/sessions-redact-{golden-before,golden-after,benchmark-comparison}.json` and
`.cache/sessions-redact-isolated-{before,after}.log`. Tracked defect:
`BUG-20260906-stream-redaction-storm-timeout`. This repair needs its own final gate and CI head;
the external provider validation limit remains unchanged.


### Follow-up: remaining provider-token scan cost

Runtime CI on `500a93afe` still exceeded the unchanged storm deadline after the assignment optimization. A fresh one-core run passed in 51.92s, but a 12-second CPU sample showed the additive provider-token expressions dominating the remaining active work. Each expression now retains a required literal prefix derived by `regexp.LiteralPrefix` before its unchanged word-boundary wrapper. The redactor skips an expression only when its prefix is absent from the current intermediate string. Empty prefixes always scan. Ordering, tie-breaking and replacements are identical; no randomness or floating-point behavior is involved. The measured hotspot's impact/confidence/effort score is 5×5/2 = 12.5.

The existing canonical suite's provider taxonomy case now also covers a literal Google prefix and the Stripe alternative with no common literal prefix, using low-entropy values so entropy redaction cannot hide an omitted provider rule. The invariant remains protection of canonical provider token shapes, owned by `internal/redact` and `TestStringRedactsCanonicalSecretTaxonomy`; no standalone suite was added. Race verification and the test-convention check pass. The expanded fixed corpus has 11,840 identical before/after SHA256 outputs across every provider prefix, word-boundary variants, exact rules, registered literals, and enabled/disabled heuristics.

Ten sequential benchmark samples per case show median plaintext cost falling from 28.244ms to 7.420ms (73.7%), JSON envelope cost from 49.319ms to 28.404ms (42.4%), and short provider messages from 29.614µs to 9.633µs (67.5%). Plaintext allocation falls from approximately 5.61MB to 1.26MB per 64KiB input. The rebuilt daemon passes the unchanged one-core 50,000-chunk stress journey in 16.83s, including the whole durable byte window, terminal completion and one canonical slow-consumer degradation record. The fixture disposed its processes and released the machine verification lock. An initial wrapper attempt failed before the journey because its new artifact parent directory did not exist; creating that directory repaired the harness without changing any test.

Evidence: `.cache/sessions-storm-profile-after-redact.log` and `.cache/sessions-storm-after-redact-sample.txt` (baseline/profile); `.cache/sessions-redact-prefix-{golden-before,golden-after,benchmark-comparison}.json`; `.cache/sessions-redact-prefix-{before,after}-bench.log`; `.cache/sessions-redact-prefix-race.log`; `.cache/sessions-prefix-storm-green.log` and `.cache/sessions-prefix-storm-artifacts/`. Current-head CI must be rerun after publication; the external provider validation limit remains unchanged. This continues the existing `BUG-20260906-stream-redaction-storm-timeout` record.


### Follow-up: Settings section navigation during a pending open

Web shard 3 on `500a93afe` exposed a lost Profiles click after archiving the selected profile and reloading. The retained CI trace shows an optimistic Settings launcher transition to General with a 321ms daemon round trip; the URL still read `/settings/profiles` when the operator clicked Profiles. That same-location router link produced no route report, and the pending launcher completion later pushed General. Profile selection was already restored correctly and was not the cause.

Claude Fable repaired the owning routing coordinator so explicit same-location navigation still reaches the window manager and a newer navigation supersedes an older open's deferred history write. Settings section links hand plain clicks to that owner; modified clicks retain native link behavior. The existing coordinator suite owns deferred command ordering and no extra history writes; a new nav-component suite owns the previously uncovered plain/modified-click handoff. No timeout, retry, selector, visual value or user copy changed. The controller reviewed the diff and tightened the modified-click test's native-I/O boundary: it records that React did not prevent the click before preventing jsdom's unsupported document navigation. All 34 focused tests pass without the prior jsdom error.

Three unchanged pre-fix local repetitions pass, so they do not reproduce CI's timing; the original failure trace and deterministic coordinator cases are retained as causal evidence. The five local E2E-014 repetitions and seven related journeys originally claimed as patched-build evidence actually served `web/dist`: the launch fixture overrides the wrapper’s alternative directory. That claim is withdrawn. Profiles E2E-014 and the related Settings journeys passed CI on d897e90f5 and 201591b96 with the corrected source. The full root Turbo Web suite passes 7,101 tests across 771 files; lint/typecheck pass. The controller's pinned React Doctor 0.9.13 rescan reports 100/100 with zero errors or warnings, without suppression. Fixtures disposed their processes and released the machine lock. Existing visual captures remain applicable because the patch changes routing handlers only.

Evidence: `.compozy/tasks/sessions-stability/memory/profile-navigation-ci.md`, `.cache/sessions-profiles-e2e014-{prefix,postfix,settingsnav}.log`, `.cache/sessions-profiles-root-focused.log`, and `.cache/sessions-profiles-react-doctor.json`. Tracked defect: `BUG-20260906-settings-nav-stale-open-history`; affected scenario: `ET-profile-web-settings-lifecycle-dialogs` (selected E2E-014 branch). Current-head CI is required after publication; the separate provider validation limit remains unchanged.


### Follow-up: negotiate the storm watcher's receive window before connecting

Runtime CI on `d897e90f5` completed the producer and verified every durable byte, then failed at the slow-watcher marker wait (28.38s total, including that 10s wait). This separated the resolved production redaction cost from an invalid pressure condition in the TCP test setup. The helper had set its small receive buffer only after dialing. An isolated Linux container running a static Go socket probe admitted 2,559,994 bytes in that case, versus 1,028,090 bytes when the same `SO_RCVBUF=1024` option was set before connection negotiation; both reported a 2,626,560-byte sender buffer. The old setup could hold the whole 1.6MB response without filling the application subscriber queue, so a degradation event was not evidence the test could require from that setup.

The invariant remains durable completion of all 50,000 chunks while a genuinely backpressured HTTP watcher is shed once, owned by the real daemon transport layer and the existing `TestDaemonE2EACPmockTransportStorm` suite. Its helper now applies the receive-buffer option using `net.Dialer.Control` before connecting. No production behavior, assertion, event count, expected bytes or deadline changed. The same one-core actual daemon journey passes in 16.05s, including exact durable bytes, terminal completion, and one correlated degradation event. The owning test-convention check passes.

Evidence: `.cache/sessions-pr-557-prefix-runtime-failure.log`, `.cache/sessions-tcp-buffer-linux-result.jsonl` (probe before/after), `.cache/sessions-tcp-buffer-probe.go` (probe source), and `.cache/sessions-preconnect-storm-green.log` with artifacts under `.cache/sessions-prefix-storm-artifacts/`. The probe container used `--rm --network none` and has no surviving container; the daemon fixture disposed its processes and released the machine lock. This follows the same recorded storm incident. New-head CI remains required after the test setup correction.


## Final CI repairs: terminal sizing, detach, and selector focus

The Linux runtime lane on 201591b96 passed all 268 tests, including 182 daemon cases and the transport storm. The unchanged small receive buffer is now negotiated before TCP connection establishment, so the storm actually exceeds the watcher’s receive capacity. The final current-head CI result is recorded separately after the remaining frontend/CLI repairs ship.

A hidden terminal tab proposed a 12×4 fit from unresolved `100%` CSS dimensions; the client clamped it to 20×5 and shrank the PTY before reload. The shared TerminalView now requires a rendered box before proposing dimensions, including a geometry check when the visibility API is absent. Its containing Web element is a flex column, allowing the grid to fill the window instead of inheriting its current row count. The canonical UI suite has 761 passing cases, and 264 Web terminal cases pass. All 12 terminal journeys passed against the actual fixed build served as `web/dist`; the wrapper restored the original build byte-for-byte. The visible-height probe measured a 417px container and a 395px host, and its retained screenshot was inspected. E2E-014 now compares the viewer with the watcher’s latest RESIZED frame: the viewers footer changes available height, so the initial ATTACHED frame is not a stable expected size. The invariant remains agreement between live views.

The CLI could discard a completed detach when the server closed its WebSocket before the input writer reported completion, causing the attach loop to reconnect. A deterministic network I/O case in the existing CLI stream suite fails before the repair and passes afterward; all TestTerminalClient cases pass with the race detector. The unchanged E2E-001 golden path passes three times in 18.7s with Bash, matching CI. The local default Fish environment separately delays the start marker by about 10 seconds and loses the approximate journal actor; the same journal assertion fails on the old binary. That pre-existing environment-specific result is retained as a limit, not attributed to this CLI repair or silently counted as passing.

The runtime selector supplied a forced final-focus callback, bypassing Base UI’s guard that preserves focus after the operator moves outside the closing popup. A real browser probe observed focus leaving the composer after about 170ms. Removing that callback restores the library’s normal focus lifecycle. The existing provider/model-override journey now asserts that the composer remains focused after the popup closes, before Enter; this stronger assertion fails on the old served build. All 98 selector component cases, Web lint/typecheck and the production build pass. The combined real-browser re-walk and final gates follow below.

Tracked defects: BUG-20260906-hidden-terminal-pane-minimum-vote, BUG-20260906-terminal-detach-close-race, BUG-20260906-runtime-selector-closing-focus. No public DTO, protocol, schema or configuration changes accompany these repairs. Existing workspace/profile isolation and official skill contracts remain applicable.

Combined controller re-walk: six real journeys passed in about one minute with the repaired CLI/daemon and Web build: provider/model override with focus retention, advertised model/reasoning persistence, CLI open/run/detach/journal, two browser terminals across reload, two CLI writers with single/double Ctrl-\ behavior, and alternate-screen watcher reflow. Evidence: `.cache/sessions-selector-root-integrated-green.log` and `.cache/sessions-selector-e2e-root-integrated-green-results.json`; `web/dist` restored byte-for-byte.

After the search-header extraction, both existing provider/model browser journeys passed again in 13.2s on the rebuilt Web bundle with the repaired daemon. The wrapper restored `web/dist` byte-for-byte. All 98 canonical selector tests still pass. Pinned React Doctor 0.9.13 reports 100/100 with zero errors or warnings across changed Web sources, including the new component. Evidence: `.cache/sessions-selector-root-final-e2e.log`, `.cache/sessions-selector-e2e-root-final-selector-results.json`, `.cache/sessions-selector-root-search-tests.log`, and `.cache/sessions-final-repairs-react-doctor.json`.

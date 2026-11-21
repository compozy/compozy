# QA Run Report — 2026-11-17 — Antigravity native ACP, consented rerun

- **Scope:** native Antigravity operation, safe policy refusal, session continuity, provider-owned authentication, and existing browser management/selection.
- **Cadence tier:** targeted.
- **Build:** `48aee1ed92637d746fe6ae2ce40afff4584ac263`, branch `feat/agy-native-acp`.
- **Environment:** Linux x86_64; isolated lab at `http://127.0.0.1:44379`; existing operator-owned native login is used only if available.
- **Started:** 2026-11-17T04:15:53Z · **Status:** PASS (targeted); lab closed with `clean: true`. A round-2 addendum completed the previously blocked separate-home consent and pending-login cancellation.
- **Authorization:** the operator authorized the real acceptance rerun and separate-home OAuth/cancellation, requested persistent evidence, and excluded provisioning another Google account and non-Linux execution. No push, PR, merge, or operator-credential modification is authorized.
- **Persistent evidence root:** workspace-root `.compozy/tasks/agy-native-acp/evidence/qa-20261117-consented/`.
- **Manifest:** `compozy-agy-native-acp-20261117-consented-20261117-041553-936719-lab/qa-artifacts/qa/bootstrap-manifest.json` beneath that root.
- **Isolation:** manifest-owned short runtime/provider homes avoid Unix-socket length limits; the manifest, report, journey log, and retained evidence live outside temporary storage. Provider usage collection is disabled.
- **Historical evidence:** the 2026-11-16 report remains unchanged. This is a separately authorized run, not reconstruction of its lost receipts.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
| --- | --- | --- | --- |
| Ada | Existing native CLI operator | Linux desktop / available network / en-US | CH-agy-native-cli-operation, CH-agy-native-session-continuity |
| Sol | Keyboard-reliant operator | Desktop and 375px viewport / available network / en-US | CH-agy-native-runtime-picker |

## Flows in Scope

- [J-15](../journeys/J-15-operate-session-via-cli-api.md): authenticated provider operation, permission refusal, cancellation, and durable recovery.
- [J-17](../journeys/J-17-session-create-unified-selector.md): choose an advertised model and effort, submit, and independently inspect persistence.
- Adjacent canary: Settings provider-owned authentication, without CompozyOS credential entry.

## Session Matrix & Results

All in-scope rows were recorded `Pending` before the first product journey.

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-agy-native-cli-operation | J-15 / native presence, readiness, Hydrogen/xhigh, persisted reply, unsupported max | Ada | Feature Tour | Pass | `native-selection-verdict.json`; native 1.5.9, acknowledged `thought_level=xhigh`; max refused without lowering effort | |
| 2 | CH-agy-native-cli-operation | J-15 / allowed write, native MCP read, constrained-start refusal, active cancellation and cleanup | Ada | Feature Tour | Pass | `native-authority-cancellation-verdict.json`; actual running job, canceled turn, verified quiescence, usable follow-up | |
| 3 | CH-agy-native-session-continuity | J-15 / real prior-runtime conversation, route-change refusal/reselection/replay, native restoration | Ada | Interrupt Tour | Pass | `native-continuity-verdict.json`; actual baseline conversation, unchanged prior reply events through refusal/replay, new native ID then same-ID native restore | |
| 4 | CH-agy-native-cli-operation | J-15 / separate clean-home probe, explicitly initiated OAuth and login cancellation | Ada | Feature Tour | Pass | `r2-native-consent-post-status.json` and `r2-product-login-reuse.json`; real provider consent via the native CLI's documented headless login, product reuse without a second consent; `r2-cancel-*` records a genuinely pending login interrupted with `SIGINT`, re-probe `needs_login`, no process left | |
| 5 | CH-agy-native-runtime-picker | J-17 / visible Settings, keyboard model/effort selection, narrow pane, submitted reply | Sol | Feature Tour | Pass | `browser-native-verdict.json`; Settings screenshots; public history confirms browser reply and effective Hydrogen/xhigh | |
| 6 | CH-agy-native-runtime-picker | J-17 / actual unknown/stale catalog-source states | Sol | Feature Tour | Pass | Built-in AGY stale/recovery; fresh lab-only native AGY alias unknown/unconfirmed; public receipts plus browser captures, no mocked service | |
| 7 | — | Provision a new Google account | — | — | Skipped | Explicitly removed by the operator; existing access remains a prerequisite for live prompts | |
| 8 | — | Native execution outside Linux | — | — | Skipped | Explicitly removed by the operator; no cross-platform acceptance claim | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`.

## Session Debriefs

### CH-agy-native-cli-operation — Ada — selection checkpoint

- Existing native sign-in passed the public remote probe without new consent. CLI/API independently agree on `authenticated`, basename `agy`, and `auth_acp_method`.
- Live catalog advertises logical Hydrogen with four effort levels and Argon with three; no model entitlement is inferred from those rows.
- Session `sess-872038a9b5206964` persisted `Saved MAPLE-8421`, independently read through public history. Public session details acknowledge Hydrogen, `thought_level=xhigh`, and advertised `mode=auto`.
- An explicit Hydrogen `max` request returned `reasoning_effort_unsupported` before producing prompt events. A separate session read retained effective `xhigh`.
- Evidence: `native-selection-verdict.json`, `native-bound-session.json`, `native-first-note-history.json`, and `native-reject-max.json` under the manifest's `QA_OUTPUT_PATH/qa`.
- The first inspection driver inherited the supervising daemon's session ID and received `session_not_found` from the isolated daemon. The driver was corrected to use its separate lab identity; that original receipt remains recorded and is not a product regression.

### CH-agy-native-cli-operation — Ada — authority and interruption

- The hosted native `compozy__workspace_info` call completed through normal MCP. The provider wrote the exact requested `project-note.txt` contents; an independent filesystem read confirmed them.
- A separate `approve-reads` session refused before launch, retained `effective_permissions=approve-reads`, stayed `unbound`, exposed no owned processes, and did not create the fresh forbidden target.
- An inotify close-write event and the public process inventory independently proved that the requested Python note-processing job was actually running before cancellation. The public cancellation command returned `canceled`, not `nothing-in-flight`.
- A persisted `session.turn_quiesced` event reports `verified=true`, cooperative settlement, and no escalation. The observed task had already exited by the follow-up process read; explicit cleanup was separately requested and returned no remaining targets. No implicit process-tree-reaping guarantee is inferred.
- The same conversation subsequently returned the original `MAPLE-8421` label, independently read from public history.
- Evidence: `native-authority-cancellation-verdict.json`, its named source receipts, and `native-quiescence-events.json`.
- A separate clean HOME probe returned `needs_login` / `provider_not_authenticated` without consent. This is not successful fresh OAuth evidence.

### CH-agy-native-cli-operation — Ada — explicit authentication boundary (round 1 and round 2)

- Round 1: explicit login on the signed-in operator home returned `authenticated` without another consent flow. The separate clean HOME returned `needs_login` before the explicit login attempt and again afterwards.
- Round 1: the authorized clean-home login failed before a consent window could open because the native CLI reported `xdg-open` absent from PATH. This was an environment prerequisite, not a successful cancellation or a failed Google grant.
- Round 2: `xdg-open` is now installed, but this container has no browser handler or display. A headless Chromium opener was registered for the product login process only. The product `provider auth login agy` reached a real Google consent URL and a real loopback listener (`/auth/callback`), proving the product initiates provider-owned consent without CompozyOS credential entry.
- Round 2: the first completion attempt used that loopback route. The provider's ACP login window is about five minutes; the operator's browser host could not deliver the callback to the container, so the attempt ended with the provider's `Sign-in timed out` and wrote no token. The expired authorization URL is not retained.
- Round 2: completion used the documented headless alternative — the native CLI's `agy acp --login` in the same clean HOME. The operator approved Google consent and pasted the one-time code into that CLI's own terminal prompt; the code never passed through the agent chat. The product probe then returned `authenticated`, and `compozy provider auth login agy --timeout 5m` returned exit 0 with `authenticated` and no second consent, proving the product reads the shared native store.
- Round 2 cancellation: a second clean HOME started the product login, waited until the real consent URL had been emitted, and was interrupted with `SIGINT`. The result was `context canceled` / `interrupt signal received`, the re-probe stayed `needs_login`, and no `agy` process remained.
- The operator home remained `authenticated` throughout (`r2-operator-login-preserved.json`). The native CLI refreshed the operator token's expiry in place during a readiness probe; the agent did not read or copy token bytes. The clean HOME and its token store were removed after the checks, and no credential store is archived.
- Evidence: `r2-native-consent-pre-status.json`, `r2-native-consent-post-status.json`, `r2-product-login-reuse.json`, `r2-cancel-pre-status.json`, `r2-cancel-consent-opened.json`, `r2-cancel-exit.json`, `r2-cancel-post-status.json`, `r2-cancel-process-sweep.json`, `r2-native-token-store.json`, and `r2-operator-login-preserved.json`.

### CH-agy-native-runtime-picker — Sol — Settings checkpoint

- Completed the real onboarding using the existing native sign-in and the lab project; no onboarding flags or browser services were mocked.
- Opened Settings → Providers, entered the Antigravity inspector with Enter, and inspected its unsaved Configure view.
- The method remains `oauth-personal`, the app descriptor names `agy` as available, and Native CLI mounts no credential-slot or credential-value controls.
- At 375 × 820, the method input remained reachable and the inspector body had no horizontal overflow. The draft was not saved.
- Evidence: `settings-native-overview.{json,png}`, `settings-native-editor.{json,png}`, and `settings-native.{json,png}`.
- The old Python Playwright driver needed selector escaping and semantic waits compatible with the installed Chromium/CSP. Those automation errors are not product failures; failed-attempt captures remain separate.

### CH-agy-native-runtime-picker — Sol — model and submission

- Keyboard search, ArrowDown, and Enter selected the advertised native Hydrogen row rather than a free-form model ID. End selected Extra high; the accessible slider had four stops.
- Argon re-announced three stops and End selected High. No raw duplicate `thought_level` control was mounted. Returning to Hydrogen restored its own fourth stop.
- Captured the real selector at 375 px. The browser POST carried `agy`, `gemini-4-hydrogen`, and `reasoning_effort=xhigh`; independent public session/history reads confirmed the effective selection and `Browser saved WILLOW-5138`.
- The provider omitted the requested terminal period. The driver's exact-punctuation wait therefore timed out despite the already-rendered and persisted reply; the error-state screenshot records that reply. A separate assert-based evidence check passed without another paid prompt.
- Evidence: `browser-native-verdict.json` and its named request, session, history, and screenshot source receipts.

### CH-agy-native-session-continuity — Ada — actual upgrade

- Built and ran baseline `1141d864060678bfec1b75990973fba1b5a87bf1` in the same isolated lab. Removed only the lab's newly created onboarding provider overlay so the old build actually resolved its standalone default.
- Created a genuine old-runtime conversation storing `Saved CEDAR-9267` and its old effective `yolo` mode. The current build then refused the same logical session before any prompt events, with explicit reselect-runtime guidance.
- Both refusal and runtime-clear refusal preserved the prior agent-message history byte-for-byte through structured JSON comparison, the logical ID, creation time, and old native binding. Clear was not treated as confirmation.
- Explicit runtime-set confirmed Hydrogen/xhigh. The same logical session replayed into a new native ID and recovered `CEDAR-9267`; every previously persisted reply event remained intact. This is transcript replay, not private provider-history import.
- Stopping and prompting that new native session retained its new ACP ID and recovered the label again, establishing native restoration independently from upgrade replay.
- Evidence: `prior-build.json`, `prior-runtime-status.json`, `prior-bound-conversation.json`, refusal/confirmation/replay/restoration receipts, and `native-continuity-verdict.json`.

### CH-agy-native-runtime-picker — Sol — real catalog failure states

- Changed only the lab's supported discovery setting to a lab-owned link to the installed native CLI. A successful real discovery returned seven logical rows; removing only that link caused an actual executable-not-found failure.
- The public source retained that generation's `last_success`; all seven rows became `available_stale`, and the browser visibly labelled AGY `Stale`. Restoring the link recovered a non-stale successful source and seven `available_live` rows. The original installed executable and credentials were untouched.
- The first exploratory attempt, despite its `catalog-unknown-*` filename, returned the existing AGY generation as stale, not unknown. It is not used as the unknown-state oracle.
- For unknown, created a fresh lab-only native AGY alias (`agy-fresh`) using the real `agy acp` execution route and operator-home policy. Its curated Hydrogen ID came from the already observed native catalog; no reasoning metadata or provider responses were fabricated. Its separate missing discovery path had never succeeded: public status had zero live rows and no `last_success`, and the curated row had `available=null`, `availability_state=unknown`, and no known efforts.
- Selecting that exact alias row rendered “Reasoning options haven't been confirmed.” without a slider. This is explicitly the fresh-source alias leg, not a claim that the cached built-in source had become unknown.
- The alias's rail freshness badge reads `Live` for its fresh configured row even while live availability is unknown; that badge is not authentication or successful native discovery evidence. The footer and public availability remain the oracles.
- Evidence: `catalog-native-verdict.json`, `catalog-{live,stale,restore}-{status,rows}.json`, `catalog-stale.{json,png}`, and `catalog-fresh-unknown{,-status,-rows}.json` plus its screenshot.

## What Was Fixed

No implementation changes in this round.

## Paper Cuts

The selector's freshness-only `Live` badge can be read as readiness even for a configured row with unknown live availability. The unknown footer correctly withholds reasoning controls; no auth/entitlement claim is inferred from the badge.

The provider's ACP `oauth-personal` login uses a loopback callback with a window of about five minutes. It needs a browser on the same host as the CLI and a reachable callback port; on a remote or headless host the browser cannot deliver the code, and the provider reports `Sign-in timed out`. The native CLI's documented `agy acp --login` paste prompt is the working headless route. This is an operator-path limitation, not a CompozyOS defect.

## Runtime Errors Observed

Bootstrap's preferred browser CLIs were absent, but headed Chromium ran through installed Python Playwright and Xvfb. Driver-only failures (old regex serialization, early captures, multiple mounted windows, and exact-punctuation waiting) were corrected or independently verified from retained receipts; no production regression is established by those failures.

In the first round-2 completion attempt, a chat copy of the consent URL arrived truncated (Google returned `400`), while the unmodified URL returned `302` to Google sign-in. The second attempt used the product login, which reached the real Google URL but ended at the provider's five-minute loopback timeout because the browser host could not call the container's `localhost` port. Neither event is a product regression; the third attempt used the CLI's own headless prompt and succeeded.

## Human Verifications Needed

None for the executable scope. Row 4 is closed by the round-2 receipts above. The two operator-excluded rows remain `Skipped`: no new Google account was provisioned, and no non-Linux execution was attempted. The operator completed the Google consent and pasted the one-time code into the native CLI's terminal prompt; no credential or token store was shared with the agent.

## Decisions for a Human

The operator's scope exclusions are recorded above; accepted production behavior and security boundaries are unchanged.

## Verification and Evidence Closure

- Native operation, authority/cancellation, actual upgrade/native restore, browser management/submission, real stale/unknown-source rows, and the separate-home consent/cancellation addendum passed their scoped independent checks.
- The final public session and history snapshots for all four lab sessions are retained. `evidence-sanitization.json` records the text scan and the driver omits private reasoning; Settings, narrow-selector, and unknown-state screenshots were visually inspected without visible credentials.
- `focused-final-verification.json`: `git diff --check`, QA-driver syntax compilation, and evidence-redaction/path-boundary self-checks passed. `focused-round2-verification.json` repeats the same checks for the round-2 scripts. No production source changed. Prior focused engineering checks are reused; repository-wide lint, full gates/tests, and CI E2E were not run locally or claimed.
- The first strict audit failed only C12/C14 because the final report and focused verification receipt had not yet been written. Those original reports/command receipts are preserved with `-initial` suffixes.
- After writing `verification-report.md` and recording the actual `final_verify` receipt, the manifest's unchanged `AUDIT_COMMAND --qa-output-path … --strict` passed with zero blockers and warnings (`qa-audit-report.{json,md}`, `strict-audit-command.json`). The round-2 addendum was re-audited with the same command; `round2-strict-audit-command.json` records that result.
- The exact manifest `TEARDOWN_COMMAND` completed at 2026-11-17T04:52:18Z. `teardown.json` reports `clean: true`, no survivors, and no provider-resource errors. Graceful stop needed an ownership-verified signal sweep; that escalation and removed copied-resource links are recorded. Round 2 used the retained lab binary and provider commands only; the lab daemon stayed closed.
- The lab is closed, not a live continuation. The original operator sign-in remained authenticated immediately before teardown and again after round 2 (`final-operator-auth-preserved.json`, `r2-operator-login-preserved.json`). The native CLI performed its normal in-place token refresh during a readiness probe (expiry extended at 10:33:01); no operator credential was read by the agent, copied, or re-consented. The round-2 clean HOME and its token store were deleted after the checks.
- `evidence-index.json` contains sizes and SHA256 hashes for 188 retained evidence files. Evidence remains at the persistent workspace-root path above; runtime/provider homes are not the evidence store.
- Portable archive: `qa-evidence-20261117-consented.tar.gz` in that persistent evidence root, with adjacent `.sha256` receipt (`e29fda727d4da10a29f90f362d6d0b5923e07bcbee8736fedf3a5013df66f78e`). Round-2 receipts are additionally sealed in `qa-evidence-20261117-consented-round2.tar.gz` with its own adjacent `.sha256` receipt. Both contain QA receipts/captures, drivers, the dated report and four scenario snapshots, but no runtime/provider home, credential store, or browser profile.

## Learnings

Use semantic waits and exact source/provider identities with the installed browser driver. Distinguish fresh configured metadata from successful live discovery, stale generations from unknown availability, and upgrade replay from later same-ID native restoration. Preserve the first failed audit and genuine cleanup receipt rather than reconstructing either. For provider-owned browser OAuth on a headless or remote host, use the native CLI's own paste prompt instead of assuming the loopback callback can reach the daemon host.

## Final Status

**PASS (targeted):** all six executable behavior charters passed, including separate-home provider-owned Google consent and cancellation of a genuinely pending login. The two rows the operator explicitly excluded — provisioning a new Google account and native execution outside Linux — remain `Skipped`; no new-account or cross-platform acceptance claim is made. Evidence audit passed and the lab teardown is clean. No push, PR, merge, or release-readiness claim is made.

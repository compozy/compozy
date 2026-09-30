# QA Run Report — 2026-09-29 — reported issues

## Authoritative runtime verdict

The native runtime verdict below belongs to the recorded pre-remediation artifacts. PR #686 review
remediation adds new index/recovery, installer/process identity, browser/catalog and contract
regressions in the existing canonical suites. Their validation runs in CI on the new published
head, per the user's instruction; historical runtime and CI receipts are not relabeled as proof
of those changed bytes. The PR's current check rollup owns subsequent delivery status.

**Runtime QA: PASS for the eleven reported issue journeys, with the evidence boundaries below.** Native managed delivery created the single authorized draft [PR #686](https://github.com/compozy/compozy/pull/686). Exact-head CI and owned-lab teardown are separate delivery checks and remain pending at this report checkpoint; successful publication is no longer pending.

- Scope: #657, #659, #663, #665, #666, #675, #676, #677, #678, #679, #682.
- Environment: isolated macOS Apple Silicon daemon, HTTP `http://127.0.0.1:65298`, unique home/socket; Linux KDE evidence came from the owning isolated native Linux container.
- Canonical lab: `/Users/pedronauck/Dev/qa-labs/compozy-reported-issues-20260929-20260930-025651-581547-lab`.
- Manifest and receipts: the lab's `qa-artifacts/qa/bootstrap-manifest.json` and `qa-artifacts/qa/` directory. Receipt filenames below resolve under that directory unless an external path is given.
- Current repaired-flow build: Git source `347f3c885b8b3036845e05486a1c8a1386249544`, Web index SHA-256 `71ea3236e585a54471d994843a07ea687a1797a34badc28b6af7086f13d18b57`, Go executable SHA-256 `3b964b0f62628df3e75c45f98cf8d5e7f84e0871b2d7ffa41afa35b4a812c2e9`, schema125. Native PID83550 launched 08:59:23Z; actual daemon PID83728 was born 08:59:25Z. Hidden-title proof used the same assets on preceding owned PID32540/daemon32709. The version0.3.0 is a development bootstrap label, not a release claim.
- Automatic catalog window: the complete literal native sixty minutes ran **2026-09-30T07:39:10.868302Z–08:39:10.868302Z** on native PID57041/daemon57528, source `c2929d1230a98d485f52f84023b00d6a92c9a6b5`, Web index `e6bc4e764b56a49213be2cc80a298d4fd0ec684f4a09d45a902b945a1fef25bf`, and the same Go executable. Later changes repaired background notification-title polling and explicit dock/window commands; the automatic catalog query/coalescer/paging invariant was unchanged. This is evidence of that exact original build, followed by separate final-asset native walks; it is not a sixty-minute soak of subsequently rebuilt bytes.

## Session matrix

| Issue | Journey | Verdict | Actual evidence |
|---|---|---|---|
| #657 | Delete stopped run-owned/system sessions in default and nondefault profiles | Pass | Genuine native row Delete and confirmation; removed transcript/row, independently scoped404 in both profiles. `web-delete-final-scoped-reads.json` |
| #659 | Rapid child/session and parent Goal Details navigation | Pass | Actual Working child → parent Goal → Open run/Details, checklist reply and terminal Goal independently confirmed. `goal-navigation-receipt.json` |
| #663 | Import failure terminality, quoted title and concurrency release | Pass | Malformed import failed generation1 in151ms; quoted input completed with exact title and next start accepted. `import-malformed-fixed-{start,status}.json`, `import-quoted-fixed-start.json` |
| #665 | Dead update holder, cancel and deadline recovery | Pass | Real public macOS CLI dead applying-holder cancel and expired status/check. `/tmp/compozy-issues-20260929/mac-update-public-proof.log`; owning update report |
| #666 | Linux desktop discovery and URI dispatch | Pass within the stated platform boundary | Actual KDE6/KIO registration, dispatch argv, shortcut deletion and public built CLI discovery/open/settings. `/tmp/compozy-issues-20260929/linux-desktop-final-public-proof.log` |
| #675 | Same-generation recovered-child adoption and changed candidate refusal | Pass | Unchanged original proof adopts exact original recovered child; changed selected content retains original proof and creates fresh child. `675-public-replay-summary.json` |
| #676 | Genuine managed native delivery, caller drain, commit/push/draft PR | Pass | Exact bound caller stopped by hand-off, durable completed operation, identical local/remote/PR SHA, single draft PR686, fence subsequently released. `managed-delivery-v3-publication-proof.json` |
| #677 | Selective delivery preserves outside staged/unstaged/private files | Pass | Real disposable Git selective commit and independent before/after index/worktree proof. `selective-exit-plan.json`, `selective-commit.json`, `selective-independent-git-proof.json` |
| #678 | Managed skill resource under SQLite contention and recovery | Pass | Real hosted Codex skill call failed safely at tool_event_write under external writer, then succeeded on same daemon; actual local-writer admission/terminal receipt recovery. `skill-resource-{real,recovered}.json`, `schema125-admission-boot-proof.json` |
| #679 | Literal native sixty-minute bounded catalog, active Loop and growing history | Pass with exact-build split above | History374→554,180 public create/stop cycles,1964HTTP200, rolling peak44≤52; final assets separately passed title/dock/cursor/search. `final-native60-result.json`, `final-repaired-catalog-native.json` |
| #682 | ACP startup deadline/retry and process cleanup | Pass within the stated provider boundary | Real ACP subprocess stall/retry/reaping integration plus actual installed Cursor READY/public Stop. `cursor-live-{prompt,stop}.json`; owning ACP report |

## Native sixty-minute evidence

The Global/default, empty-query Sessions palette packet mounted two distinct100-row pages (`last_activity` and `navigator`), one shared population facets aggregate and separate attention summary. The frozen ceilings per rolling60 seconds were **26 session-list,13 facets,13 attention-summary; combined52**, established before acceptance. Startup and manual prewalks were outside the window; sustained ordinary catalog activity was included.

The exact window recorded **1964 responses, all HTTP200**:982 session-list,491 facets and491 summary. The rolling peaks were **22/11/11, combined44**, below every ceiling. History grew from374 to554 through **180 actual public CLI session creation/stopping cycles**, all successful. Three additional cycles occurred while collecting/stopping endpoint receipts and are explicitly excluded from the180-window count. No SQL-seeded history or timer acceleration substituted for this activity.

Real Loop `looprun-4b6d5e6b11d64c70` ran a managed Codex preparation node in12.14s, using35,800 tokens and session `sess_155631c5fc9538556d3bcbf1d1e4c8bb`, then stayed in its authored65-minute durable wait. Public status at the endpoint remained Running; only this owned run was subsequently canceled. Effective iteration cap was50, despite authored cap1; no additional iteration was taken during the waiting window.

There were119 native renderer samples at approximately30-second intervals: all reported actual `visibilityState=visible` and the100-row page. One natural unfocused sample is retained; the window is not described as continuously focused. No synthetic visibility, throttling override or operator-Space manipulation was used. Metadata grew across the500-history boundary without additional page consumers.

Two external measurement mistakes were corrected transparently without changing the clock or runtime. Public navigation before the clock reloaded the renderer UUID; the actual frozen client was `web-e35b9924-388e-4e50-b0ad-e3571e8e74d3`, replacing the earlier pre-navigation UUID. A monitor initially showing zero was not accepted; all requests were recomputed from the original start. At07:53:50 the daemon rotated its10MB log; the monitor was corrected to scan the preserved rotated and current logs, again from the original cutoff. The lower current-file-only count was not accepted. Original configuration, actual native query keys, corrected monitor receipts and log hashes are retained.

Evidence: `completed-native60-runtime-build.json`, `final-native60-result.json`, `final-soak-start-original-client.json`, `final-soak-start.json`, `final-native60-real-client-keys.json`, `final-soak-http-requests.json`, `final-soak-native-samples.jsonl`, `catalog-activity.jsonl`, `final-native60-loop-{nodes,end-status,owned-cancel}.json`.

## Final repaired-flow native walks

The final canonical root Turbo Web artifact build passed; no local tests, lint, typecheck or delivery gates were run. The current71ea index was loaded by actual isolated Electron/daemon processes, with executable SHA verified before spawn and actual daemon birth/command recorded. Go bytes were unchanged. `final-repaired-flows-runtime-boot-proof.json` and `final-title-runtime-boot-proof.json` distinguish both final-asset process instances.

- **Background title:** cleared the existing unread projection through public snapshot acknowledgement204, yielding base title `CompozyOS`. Genuine CUA Cmd-H made the actual renderer hidden/unfocused. A provider-free public Loop ask produced `needs_you=1`, `finished=0`; while still hidden the title became `(1) CompozyOS`. Public acknowledgement204 of that exact snapshot restored `CompozyOS` while still hidden. The pending ask was then answered through its inspected public generation/node/item identity. `final-title-hidden-{growth,attention,ack,cleared}.json`, `final-title-loop-{requests,respond}.json`.
- **Sessions dock:** selected the freshly registered empty project `ws_027d3a73af87c426`. The actual Sessions dock opened the cold Start session form; Cancel preserved the empty catalog. Public CLI created `sess-fc21088417872871`; the next dock click opened that exact session, not another create form. Actual Minimize window then dock click restored the same title/session with native visibility/focus true. The cache was empty before external creation; no unobserved cache state at click is claimed. Remote unchanged E2E-136 and the owning query boundary cover the causal stale-cache regression. `final-dock-cold-workspace.json`, `final-dock-external-new.json`, `final-dock-restored.json`.
- **Terminal capacity:** public CLI opened eight named interactive terminals in that owned project. Actual Terminal dock adopted Capacity QA8; actual New terminal opened the unchanged workspace-cap dialog naming all eight IDs and `8 of 8 terminals are open`. Independent public list retained eight terminals. The manual click latency did not reproduce the66ms CI race; the canonical serialized-queue regression and unchanged E2E own that causal boundary. Only those eight disposable terminals were subsequently killed through public CLI, all successful. `final-terminal-cap-{public-seed,after-list,native,network}.json`, `final-terminal-owned-fixture-cleanup.json`.
- **Catalog smoke:** final assets rendered All558, a100-row page, actual Next/Previous cursor controls, contextual Greek query `ος` finding `ΟΣ İ contextual search`, and successful clear back to the full population. `final-repaired-catalog-{native,network}.json`.

After hiding, the host's CUA/native fullscreen reactivation failed to focus the product reliably. Only the owned Electron/daemon was restarted using the identical final artifacts; no additional build, source change, synthetic visibility or operator workspace manipulation occurred. The valid hidden-title proof is retained on the preceding process; the fresh same-byte instance completed dock/cap/catalog controls. The temporary QA-only fullscreen window closed with that owned instance.

Earlier final prewalks also exercised genuine page continuation, origin-scoped network failure with90.2s and zero automatic retries, real Retry recovery, hidden catalog zero-request behavior and no focus storm. These are separate manual/error walks outside the sixty-minute visible window. Receipts `native-pagination-events.json`, `native-error-{start,events-first,backoff,recovery-events}.json`, `final-native-unicode-prewalk.json`. No queued hidden-wake flush is claimed where none was observed.

## Native delivery and preserved failures

The successful v3 intent was invoked once by exact bound Codex session `sess-b40b8effd975f574` through its hosted native terminal actor. Operation `op_d4f4ba9c59660917` admitted at07:05:00.526249Z, stopped that caller through the manager, and completed durably at07:05:42.880654Z. Actual required precommit tasks all ran under the process-only safe hook overlay, with `--no-stash --no-hide-partially-staged --no-revert`; no task or diagnostic was suppressed. Commit, remote branch and draft PR686 all initially pointed to `268933c20e844f42355feb06af5b06ae4f4dff42`. Public same-worktree session creation after settlement proved fence release, then that proof session was stopped. Subsequent controller-owned public selective commits updated this same PR; no second PR was created.

Evidence: `managed-delivery-v3-{native-prompt,terminal-events,caller-after,publication-proof,pr-independent}.json`, `managed-delivery-v3-hook-stderr.log`, `managed-delivery-v3-fence-release-{new,stop}.json`. The current daemon retains only the safe process Git configuration for the hook, recorded without raw environment/secrets in `final-daemon-safe-hook-env.json`; original/local/global Git configuration was not changed.

Historical failed attempts remain failures:

1. The original native intent exposed missing `deliver` in the persisted SQL action CHECK. A prepared journal existed even though no SQL operation was admitted. Lossless migration125 preserved all existing columns/constraints/index while adding the action. No journal/identity was edited or deleted.
2. Initial recovery then exposed local writer admission consuming the SQL attempt budget; terminalization remained Running. Production admission was corrected to wait cancelably before pool acquisition/BEGIN attempt1, without increasing SQL retries or busy timeout. Subsequent durable recovery failed explicitly with `Delivery inputs changed after authorization`; HEAD was unchanged and no Git publication occurred. `schema125-old-intent-terminalization-failure.json`, `schema125-admission-old-intent-terminal-events.json`.
3. Native v2 admitted and stopped its caller but the real hook rejected an unsafe optional-chain test fixture and oversized dialog composition. It settled Failed with no commit/push/PR. The diagnostics were repaired rather than skipped. `managed-delivery-v2-{caller-after,hook-stderr,terminal-events,failure-proof}`.
4. The first native sixty-minute attempt ran only11m17s before a required UI repair. It was interrupted and **did not pass**. Its receipts remain under the `interrupted-064335-` prefix; the complete window above is distinct.
5. Early eager multi-page/browser and Global-empty catalog walks were diagnostic only, not literal native acceptance. The first recovered-child replay also failed before the authoritative targetless receipt repair. These failures led to production fixes and real re-walks; none is counted as a pass.

## Platform and provider qualifications

#666 used actual isolated Linux KDE6/KIO with the production registration script and built Linux CLI, including outside-Applications AppImage symlink discovery and deleted-shortcut recovery. Its executable fixture sits at the app process boundary; a physical KDE panel and packaged Electron installer/AppImage update are not claimed. The container was stopped/removed. See `docs/qa/reports/2026-09-29-update-deadline-linux-uri.md` and the final public Linux receipts.

#682 bounded startup stall/retry and child-process reaping were exercised by the canonical real ACP subprocess suite. Actual installed Cursor answered READY and public Stop completed; no upstream vendor stall was observed. #665 unexpected packaged app exit was also not observed; its accepted public cancellation/deadline transitions were real. These boundaries are explicit rather than inferred from unit mocks.

## Historical delivery checkpoint before CI closeout

Runtime journeys are complete with the exact build/platform/provider qualifications above. Publication #676 is Pass; [PR686](https://github.com/compozy/compozy/pull/686) remains a draft until the controller verifies the final head. The user explicitly directed delivery gates to CI; no additional broad local gate/test/typecheck was used as a substitute.

The controller owns the final report/scenario/impact/test-fixture public commit, exact-head CI and terminal release. External `/tmp/compozy-issues-20260929/ci/` receipts own remote CI status. The final verification/strict QA evidence audit and owned-lab teardown receipts will be retained under the canonical lab; their completion must be read independently from runtime Pass. Operator browsers, Spaces, profiles, unrelated Git entries and unrelated live/queued processes remain outside teardown ownership. No human action or permission is currently required.

## PR review remediation

All eighteen inline findings and the request-context nit were confirmed against source. Coverage
stays in the existing real Git/SQLite/subprocess, API contract, provider, query/cache, and caller
UI suites. The changes preserve reviewed index versions, terminalize deterministic delivery
refusals, release fences before completion, retire orphan receipts without effects, preserve
unknown journals, protect live installers and process identities, compare Linux installation
versions, bound MIME registration, reject write reentry, publish the honored facet filters, and
retain browser/query/search contracts without suppressing errors or increasing existing budgets.

The general Linked Issues check exposed documentation omissions: official managed handoff now
requires include/scope, its JSON shape names delivery identity fields, and the owning Cobra output
example supplies every required flag. Both excluded task templates already require quoted YAML
titles with quote/backslash escaping; the importer retains indicator round trips and malformed
input rejection. The bot's partial docstring metric names no missing function contract and
conflicts with the explicit one-line WHY comment policy. Named safety invariants are documented
at their owners; no review configuration or threshold was lowered. The historical sixty-minute
catalog run remains qualified by its exact original source and assets. New blocked-sibling,
facets and debounced-search regressions must pass the new-head CI and do not reuse that soak as
new-byte evidence. The canonical packaged Electron shell suite now also adds a literal sixty-minute
regression with 600 publicly created/stopped sessions, a running real Loop and continuous public
lifecycle activity. Actual renderer request headers identify the client; every rolling sixty-second
window retains the frozen 26/13/13 endpoint budgets and 52 combined limit. Native focus/visibility,
monotonic elapsed time, current head, runtime digest and detailed rate samples are retained as a
CI artifact on success or failure. The existing desktop lane owns this run; its result is pending.

The subsequent Greptile review found that a failed directory listing was incorrectly treated as
an absent inventory. Recovery now preserves running receipts on non-absence listing errors and
logs the failure without execution effects. The existing exit-action suite uses a real filesystem
listing failure, verifies preserved state and bytes, and restores a matching journal to recover
the same receipt. The targeted real-Git integration suites now run in the existing PR CI lane.

Inventory restoration after successful daemon boot is owned by a cancellable recovery worker
with backoff capped at thirty seconds, ending after successful recovery. Existing daemon
boot-worktree tests use real SQLite and filesystem failure/restoration without a second manual
recovery call; shutdown and failed-boot paths cancel and join it. A further Greptile finding
extends the same retry policy to wrapped SQLite BUSY/LOCKED using the existing store classifier.
The canonical daemon suite locks the actual receipt database through an independent SQLite
connection and observes the production receipt writer exhausting its bounded attempts,
then verifies recovery of the original receipt with one terminal event after releasing the lock.
These added regressions require current-head CI evidence. The existing managed-delivery
real-Git suite also protects a concurrent live receipt from delayed recovery. The subsequent ACP
timestamp review is a source-backed false positive: the single production constructor records
the timestamp before publication and session wrappers preserve it.

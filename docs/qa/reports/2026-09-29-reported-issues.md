# QA Run Report — 2026-09-29 — reported issues

- **Scope:** GitHub issues #657, #659, #663, #665, #666, #675, #676, #677, #678, #679, #682.
- **Cadence tier:** targeted
- **Build:** frozen native development binary SHA-256 `3b964b0f62628df3e75c45f98cf8d5e7f84e0871b2d7ffa41afa35b4a812c2e9`; schema125; exact runtime provenance below.
- **Environment:** isolated daemon at http://127.0.0.1:65298; macOS Apple Silicon; native KDE/X11 is unavailable on this host.
- **Started:** 2026-09-30T02:56:51Z · **Status:** in-progress
- **Canonical manifest:** `/Users/pedronauck/dev/qa-labs/compozy-reported-issues-20260929-20260930-025651-581547-lab/qa-artifacts/qa/bootstrap-manifest.json`

## Personas

Nia operates sessions and Loop navigation in the Web UI. Bruno manages workspace tasks, worktrees and delivery through public CLI/API. Ada uses the managed native tools to load skills and deliver work. All use laptop/desktop, fast local network, en-US.

## Flows in Scope

Existing session catalog, worktree lifecycle, spec-cycle tasks, managed skill-loading and desktop update journeys are reused. Workers own affected scenario updates; this report records runtime evidence and coordinates verdicts.

## Session Matrix & Results

| # | Journey / Scenario | Persona | Tour | Status | Issue | Evidence |
|---|---|---|---|---|---|---|
| 1 | Stopped Loop session deletion | Nia | Feature Tour | Pass | #657 | Actual native renderer row Delete and confirmation in default and planning-review; row/transcript removed and independent scoped404 |
| 2 | Rapid child session / parent Goal navigation | Nia | Feature Tour | Pass | #659 | Actual native child Working → parent Goal → Open run/Details; checklist reply and terminal Goal independently confirmed |
| 3 | Quoted task import and load-failure terminality | Bruno | Feature Tour | Pass | #663 | Real CLI malformed failure, quoted success and next-start concurrency release; receipts below |
| 4 | Dead-holder update cancel and deadline recovery | Bruno | Interruption Tour | Pass | #665 | Owning public macOS CLI dead-holder cancel/deadline/check re-walk; linked report |
| 5 | KDE desktop startup and visibility | Nia | Platform Tour | Pass | #666 | Real Linux KDE6 KIO registration/discovery/URI dispatch; executable boundary fixture, packaged Electron not claimed |
| 6 | Existing worktree adoption | Bruno | Feature Tour | Pass | #675 | Public same-generation paused-parent replay adopts exact recovered child; original proof plus changed selected content forces fresh child |
| 7 | Active managed-session native worktree delivery | Ada | Feature Tour | Pending | #676 | Actual native submission exposed missing SQLite action migration; schema125 and recovery repaired, old intent durably refused without Git effects; publication remains pending |
| 8 | Selective worktree delivery preserves private/pre-staged files | Bruno | Feature Tour | Pass | #677 | Real selective exit/commit; independent Git proves only selected file committed, outside index/worktree and private task preserved |
| 9 | SQLite contention and managed skill-resource recovery | Ada | Interruption Tour | Pass | #678 | Actual Codex hosted skill fails safely under external writer, then succeeds after release on same daemon |
| 10 | 60-minute catalog soak, 300+ sessions, active Loop, client identity, visibility/error backoff | Nia | Time Tour | Pending | #679 | First final native window interrupted at11m17s for real hook UI correction, NOTPASS; definitive60-minute window starts after native publication on final assets |
| 11 | Cursor ACP startup deadline/retry and provider process reaping on stop | Bruno | Interruption Tour | Pass | #682 | Real ACP subprocess stall/retry/reaping suite plus actual Cursor READY/public stop; vendor stall not observed |

## Session Debriefs

### Infrastructure and live-provider preflight

- `make worktree-bootstrap` completed (mise pins and bun dependencies).
- Root Turbo production asset build `bunx turbo run build --filter=./web --only` succeeded. This infrastructure build bypassed the currently unstable transitive codegen-check and is not a delivery gate. Existing CSS pseudo-element, ineffective dynamic-import and chunk-size warnings were observed.
- Initial isolated daemon started at 2026-09-30T03:02:02Z, PID 27093 registered under the manifest PID directory. Initial CLI binary is a development build; final fixed build is still required for acceptance.
- Public CLI registered workspace `ws_6c0c4dd0c4de226d`, created the real `planner` agent using `codex/gpt-6.1-sol`, and its managed prompt completed with `end_turn` and response `READY` at 2026-09-30T03:06:55Z. This establishes live provider availability, not #682 stall/reaping acceptance.
- Public `session new` / `session stop` is populating 320 history records. Exact IDs are retained at the manifest lab's `qa-artifacts/qa/persisted-history-ids.json` when complete.
- Browser-use could not attach to operator Chrome without an Allow remote-debugging prompt. A dedicated Chrome for Testing profile was launched at CDP port 65299, PID 32396 registered. Its renderer navigation/evaluation currently times out; native CUA confirms a blank, loading page. HTTP `/` independently returns production index with 200 in ~7ms. Browser acceptance clock has not started.


### Fixed-build public runtime preflight

- Rebuilt CLI and production Web assets; restarted only the isolated daemon at 2026-09-30T03:16:06Z, PID 50918. Authoritative daemon status reports schema 123 and 322 persisted sessions. Binary SHA-256 `4cdcbf251c7758e34f788b6960707e2b75c831196f8ae11b4037abc1d94c6ffd`; production index SHA-256 `8d042e742bb686c238e8759ba571b6a1dd7c6942ed9175bd503524658beac85e`.
- Operator Chrome was controlled through native CUA to complete the real onboarding flow with Codex GPT-6.1 Sol and the already registered isolated project, then bound through browser extension CUA tab 1938874199. Operator Chrome is not owned by this lab; teardown must close only the added QA tab, never its process.
- `session new` / `session stop` completed 320 public history mutations. Independent HTTP scoped page returns 321 stopped sessions across profiles/workspaces, with 100 returned and a continuation cursor. Exact created IDs retained in `qa-artifacts/qa/persisted-history-ids.json`.
- Public Loop `planning-cycle` (`looprun-476823d52282e2bd`) executed a live Codex planning node, then remained running in its authored 90-minute wait. This is an active waiting Loop; it does not by itself reproduce sustained provider/session event activity.
- #679 clock remains unstarted: real Sessions modal shows `All 0` / `No sessions in this project yet` while the public list independently confirms the records. Browser request telemetry contains catalog-stream and attention-summary but no `/api/sessions` list request. Visibility is `visible` and focus is true. The issue owners are investigating; an empty catalog cannot yield a meaningful soak pass.
- #663 default-path reproduction remains unsettled on this build: import-only Loop `looprun-a9fea33bf85de058` consumed malformed unquoted task title containing a colon; public Loop status reports `running`, `completion_state: complete`, generation 2, zero tokens after the importer failure. Default effective configuration is `failed_only` and cap 50. The Loop worker is repairing this separately from its already passing halt-only check. Evidence `qa-artifacts/qa/import-malformed-{start,status}.json`.

### Current production-parity soak and importer re-walk

- Rebuilt fixed CLI and Web assets; fresh isolated daemon PID 65974 started at 2026-09-30T03:33:40Z. The intermediate blank reload was traced to the old daemon serving its launch-time index while the asset rebuild removed the referenced script; the fresh startup restores asset parity.
- Global Sessions now renders real persisted history (`Showing 150 of 323`). The 60-minute browser window began at 2026-09-30T03:35:04.938199Z, earliest completion 04:35:05Z. Client identity is `web-37d1889f-7bb8-469f-ac89-4b21bca32167`. Binary SHA-256 `18ea9b31ad3a7bce5f1f92633e8c43695ce811b110b2ca985f15c5d74c86150d`; index SHA-256 `819a65abe175d5704777abd05d19c8ecd4ea30d9791a2bb16e5ec3fe67ba1b9d`.
- Frozen passive request threshold: 91 session HTTP requests per rolling minute, derived from 13 wake windows and seven mounted requests (four catalog pages, one attention page, one done page, one attention summary). Explicit user navigation must be identified separately. Owned PID 66911 continuously retains per-client request receipts and rolling counts; visibility, errors and UI responsiveness still require the remaining walks.
- #663 re-walk on the fixed daemon: malformed import run `looprun-075ca0d29ea1d92f` reached `failed`, generation 1, `completion_state: complete`, zero tokens in 151ms. Node output contains `tool_invalid_input`, cause and recovery guidance. A subsequent quoted-title run was accepted immediately, proving concurrency release; quoted run `looprun-0703f13051de736d` completed `done`, generation 1, preserving the exact title `Task 1: Log summary: core and CLI`. Receipts `import-malformed-fixed-{start,status}.json` and `import-quoted-fixed-start.json`.
- #678 actual managed Codex native `compozy__skill_view` read of `compozy/references/terminal.md` overlapped an external 20-second SQLite writer lock. Two tool calls returned safe `tool_backend_failed` / `backend_unhealthy` with phase `tool_event_write`; no filesystem-load success was inferred. After the lock released, the same managed session and daemon retried successfully and reported the first heading `# Terminal`. Receipts `skill-resource-{real,recovered}.json`, `writer-transient.jsonl`.
- #657 default/project slice: actual stopped system Loop child opened from the run Graph's `Open session` and rendered its retained transcript. Public Loop cancel followed by native `session remove` succeeded; independent workspace-scoped GET returned 404 only after deletion. The non-default `planning-review` slice also rendered its actual transcript while the browser remained on default profile, then canceled, inspected stopped, removed through native CLI and returned independent scoped404. Rapid Goal navigation remains pending (#659).
- #679 current window is diagnostic only: review found eager full-list page refetch still scales with larger history. Controller is implementing real pagination plus exact aggregate metadata and server search; final acceptance must restart on that code. The fixed 91 combined-prefix bound here consists of 78 exact session-list requests plus 13 attention-summary requests per minute and applies only to the frozen 323-row configuration.
- #659 actual session-origin Goal spawned native child `sess-6401932af4bae32c` (`Planning live child`). The real browser rendered it Working, then the visible Sessions sidebar switched to parent `Planning goal review`; the parent Goal strip opened its current run and Details rendered `{"status":"complete"}` without an error pane or artificial delay. Independent public Goal status is complete/done and the child outline retains a real planning checklist reply. TTL120 later stopped the child as authored. Receipts `goal-navigation-receipt.json`, `live-child-{goal-status,outline,inspect}.json`. Canceled request classification is additionally covered by the canonical core suite; no suppression or delayed navigation was introduced.
- #677 real disposable linked Git worktree: `worktree exit --include selected.txt` returned an exact scope fingerprint; native `worktree commit --include selected.txt --expected-scope ...` accepted operation `op_1d7c336ce7e8fcf7`. After asynchronous settlement, independent Git reports HEAD changed only `selected.txt`; `outsider.txt` retains its original staged bytes and distinct unstaged bytes, and unignored `.compozy/tasks/private.md` remains untracked and unchanged. Receipts `selective-{exit-plan,commit,independent-git-proof}.json`.
- #676 authorized reversible preparation registered the primary Git workspace and adopted the existing implementation linked worktree without creating a branch. Managed session `sess-07d475818180f7e6` is bound to `wt_2fd65e6b65e94a20`; GitHub forge credentials were bound through the write-only extension secret boundary from existing authenticated `gh`, with no value output. Commit/push/PR effects await the controller's final reviewed packet and are not claimed here.
- Final #679 acceptance preparation is separate from the diagnostic run: the final monitor records fixed per-endpoint and combined request limits, statuses, and rolling 60-second peaks. A public CLI activity fixture will create and stop archive sessions every 20 seconds while an actual Codex planning Loop remains active, allowing the population to grow beyond 500. Neither fixture nor the final acceptance clock has started; final assets and a fresh owned daemon build are required first.
- #665 owner public macOS CLI receipts independently reviewed: dead `applying` holder cancellation returns `canceled`, public app status returns idle, expired applying records settle failed, and `update --check` reconciles without projecting applying. See [update/Linux QA report](2026-09-29-update-deadline-linux-uri.md) and `/tmp/compozy-issues-20260929/mac-update-public-proof.log`. Original unexpected app-process exit remains unobserved.
- #666 owner isolated Linux arm64 Docker proof uses actual KDE6 KIO, `xdg-mime`, `desktop-file-validate`, `kbuildsycoca6` and production registration module. KIO dispatch succeeds; after shortcut deletion, built Linux public CLI detects versioned AppImage and launches with exact URI. This is native KDE registration/dispatch proof using an executable fixture at the application boundary. Packaged Electron AppImage install/update and a physical KDE panel remain release-artifact QA, not falsely claimed. Owner container was stopped/removed.
- #682 canonical real ACP wrapper/child subprocess integration covers silent startup, exactly two fresh attempts, prior process exit before retry, both-stall terminality and explicit Manager Stop cleanup. Actual installed Cursor `gpt-5.4-nano` also answered `READY` (`end_turn` at 03:53:59Z) and native public Stop completed in about three seconds. No upstream Cursor stall was observed in that live attempt; bounded stall/retry/reaping evidence belongs to the real subprocess suite, not a claimed vendor-stall reproduction. Receipts `cursor-live-{prompt,stop}.json`; owning `/tmp/compozy-issues-20260929/acp-session-integration-race.log`.

## What Was Fixed

Implementation workers record their own focused verification. Runtime verdicts remain Pending until the fixed public surfaces are walked.

## Paper Cuts

Not run yet.

## Runtime Errors Observed

Not run yet.

## Human Verifications Needed

None determined yet. KDE availability is a known host limitation and must be checked against the existing native Linux path.

## Decisions for a Human

None determined yet.

## Learnings

The 60-minute catalog clock begins only once fixed code, production-parity browser, persisted history and active Loop are available. Short tests cannot replace this acceptance window.

## Final Status

In progress. Completed runtime journeys are marked individually above. Managed draft PR publication and literal native60-minute completion remain pending; strict audit and owned-lab teardown are required at terminal outcome.

- Final native preparation: actual isolated Electron process 25125 launched at 05:19:18Z and spawned daemon 25204 at 05:19:20Z. Process command and `lsof` executable mapping confirm the isolated runtime binary copied before launch; SHA-256 `25be1c0ce757b6072677fdf44bac08076e5eb982fc43f91136b50c93e41191c2`. No binary copy occurred afterward. The semantic version 0.3.0 is a development bootstrap label, not a release claim. Native CDP 53152 inspects the actual product renderer at the lab origin.
- Final native contextual search re-walk passed: public session creation/stopping produced title `ΟΣ İ contextual search`; actual palette queries `ος` and `i̇` both render that stopped session. Receipt `final-native-unicode-prewalk.json`. Current native client is `web-6f63a7b5-5392-4435-a0c6-4d66241e661d`. The empty Global palette packet has two unique 100-row page queries, one shared facets aggregate, and attention-summary; fixed rolling-60-second limits are 26/13/13 and combined 52. The literal native 60-minute acceptance clock remains unstarted.
- Earlier #657 CLI removal is explicitly a server-path/workaround receipt, not completion of the reported Web Delete journey. Fresh stopped Loop records are being prepared for real native confirmation and independent post-delete404.


## Current final runtime and acceptance window

The earlier native preparation entries are historical. The current owned Electron process is PID57677, launched at 2026-09-30T06:38:42Z; it spawned daemon PID57700 at 06:38:44Z. Before spawn, the canonical development runtime was copied into the isolated bundle and home. Process birth, command and `lsof` executable mapping establish the loaded binary SHA-256 `3b964b0f62628df3e75c45f98cf8d5e7f84e0871b2d7ffa41afa35b4a812c2e9`. Web index SHA-256 is `7c6308b2cc24f953287594ef33dca1f4d8ef4de06eac20051a8788e9e2ed9db1`; shell SHA-256 is `d2eb82e57c22906cba6579600d5333869a403d12de882979f6560ca2040c67c6`. Version0.3.0 labels the development bootstrap artifact; it is not a release claim. No binary copy/restart occurs during the acceptance window.

### Literal Web Delete completion

On the actual Electron renderer, documented CUA accessibility actions selected the stopped run-owned system row, opened More actions → Delete session, and activated the real confirmation for default session `sess-09ef5877b6ca49ca`. The row and transcript disappeared and the renderer navigated to `/sessions` with No session selected. For `sess_30066c53f2b9ebd54ad86f88ed74f6d5`, the actual profile affordance switched to planning-review; the sole stopped system row was deleted through the same real controls. Its profile list became empty and the selected transcript disappeared. Independently scoped GETs to each owning workspace/profile returned structured session-not-found404s. No CLI removal substituted for either final walk. Evidence: `web-delete-literal-cua-summary.json`, `web-delete-default-scoped-read.json`, `web-delete-nondefault-scoped-read.json`.

### Exact recovered-child public replay

The first public replay exposed a missing-target settled-output shape and did not pass. After the production repair, unchanged proof replay paused parent `looprun-48d13041f6717442` with failed review in generation1, recovered original child `looprun-368790b1ee7fa072` in place, then resumed the parent. It completed generation1 and its receipt referenced that exact original child. In the negative replay, parent `looprun-bd7632a02bd2d7fa` retained the original proof while only selected file content changed; HEAD and index were unchanged. Recovered child `looprun-afdeb8f6d3164dce` was denied adoption and fresh child `looprun-2d7b1a85f30e2375` was created, with the receipt still pending. Only the negative waiting parent/child were canceled; exact selected bytes and original scope were restored and unrelated index/worktree/private files were preserved. Evidence: `675-public-replay-summary.json` and its referenced public receipts.

### Managed native delivery failure, migration and recovery

The approved first packet was invoked once through the real bound agent's hosted native terminal tool. CLI exit69 exposed the upgraded `worktree_exit_ops` CHECK that lacked `deliver`. No SQL operation was admitted at that point, but a prepared durable journal persisted. Migration125 adds the action losslessly while preserving all seven columns, foreign key, state constraint and active-operation index. The first boot admitted operation `op_ca096d9d0ca0c53c`, but terminalization failed under local SQLite writer contention and its durable state remained running. This historical failure is preserved rather than reported as success.

The owning admission repair waits cancelably for the local writer before acquiring a pool connection or consuming the first SQLite attempt; the existing SQL retry budget and busy timeout are unchanged. On the current fixed runtime, the interrupted operation settled failed at 06:39:05.473Z. Recovery operation `op_a04be1d98768df2e` settled failed at 06:39:07.223Z. Actual durable SSE event4469 reports `worktree_safety_check_failed: Delivery inputs changed after authorization.` HEAD remains `a15f541f93e1da6d5d1678b22e2312101a1cda4f`; no commit, push or PR occurred. Original journal and identity were not edited or deleted. A new reviewed identity/packet is required for publication. Evidence: `managed-delivery-terminal-journal-failed.json`, `managed-delivery-original-journal.json`, `schema125-old-intent-terminalization-failure.json`, `schema125-admission-boot-proof.json`, `schema125-admission-old-intent-terminal-events.json`. This proves failure recovery, not successful native publication.

### First final native60-minute window — interrupted, not passed

Documented CUA Raise and actual window controls restored the native window on the visible screen without synthetic visibility, throttling changes or non-CUA OS gestures. Standard main-process getters report the actual visible, focused, nonminimized product window; the actual renderer reports `visibilityState=visible` and `hasFocus=true`. The earlier macOS Space blockage is resolved.

The acceptance clock started at **2026-09-30T06:43:35.136325Z** and cannot finish before **07:43:35.136325Z**. Client identity is `web-00d97949-2ded-4504-aed3-2883540a9994`. Empty-query Global/default Sessions palette and shell expose exactly two canonical100-row page keys (`last_activity` and `navigator`), one shared population facets key and separate attention-summary. The frozen rolling60-second ceilings are **26 session-list,13 facets,13 attention-summary; combined52**. Startup/manual prewalk is excluded; ordinary background activity remains included. The bound is independent of total history because each consumer reads one page.

Initial history is336 persisted sessions. Real Loop `looprun-28f25ce2acc2c3dc` completed its Codex preparation node at 06:40:55.908Z and remains running in its authored65-minute wait. Public CLI session creation/stopping every20 seconds supplies sustained catalog activity and history growth. Registered activity PID85488 and monitor PID85489 retain receipts. Early visible samples show growing exact metadata while the page remains100 rows. No60-minute pass is claimed before the complete wall-clock window, threshold/status review, final Loop/visibility checks and evidence audit. Evidence: `final-soak-start.json`, `final-native-catalog-preclock.json`, `final-soak-loop-preclock-nodes.json`, `final-soak-current.json`, `final-soak-native-samples.jsonl`, `catalog-activity.jsonl`.

The user directed delivery gates to CI to avoid local machine resource use. No additional broad local gates/tests/typechecks were run; indispensable native compilation and actual public runtime QA continued. CI and final publication are separate pending proof boundaries.


### Second native hand-off and real hook refusal

The second exact reviewed packet was invoked once through bound session `sess-4fbccb0e8acc7342` and its genuine hosted native terminal tool. Operation `op_01805eade15ac3b0` was durably admitted at06:50:44.474Z. The manager stopped the caller as part of hand-off; independent public health confirms lifecycle/state stopped, active prompt false and attachable false. No operator stop substituted for that behavior. Guardian/native Exec and one terminal-wait receipt are retained.

The actual precommit hook ran every existing lint-staged task under the process-only safe overlay: `--no-stash --no-hide-partially-staged --no-revert`. It rejected the API test's unsafe optional chain and an oversized desktop shell component; concurrent `make fmt` was killed by lint-staged after that failure. Neither diagnostic was suppressed or skipped. Operation failed durably at06:51:00.376Z, HEAD remained `a15f541f93e1da6d5d1678b22e2312101a1cda4f`, and no commit, push or PR was created. The v2 journal remains preserved in committing phase. Evidence: `managed-delivery-v2-native-prompt.json`, `managed-delivery-v2-caller-after.json`, `managed-delivery-v2-hook-stderr.log`, `managed-delivery-v2-terminal-events.json`, `managed-delivery-v2-failure-proof.json`. Successful native caller drain/admission is established; completed publication remains Pending.

The controller directed the real diagnostics to be repaired: the unsafe optional chain changes only test readiness code, and existing desktop dialog composition moves into a static host in the existing sessions modal module without changing query/lifecycle owners or adding a DOM wrapper. No broad local gate/test/lint was run; the mandatory real hook is retained for the next reviewed submission.

### Interrupted window and remaining definitive acceptance

The native60-minute window above was interrupted at **2026-09-30T06:54:52.486Z**, after **677.350 seconds (11m17s)**. It is explicitly **not a60-minute pass** because the final UI source changes for the real hook repair. Original request/visibility/history receipts are preserved as `interrupted-064335-*` with `final-soak-interrupted.json`; observed peaks stayed under the fixed52 ceiling and responses were200, but this partial run cannot close #679. Only owned activity PID85488, monitor PID85489 and native sampler PID34784 were stopped. Superseded Loop `looprun-28f25ce2acc2c3dc` was canceled through public CLI at06:56:15.861Z; runtime and user processes were preserved.

The new definitive native60-minute clock remains **Pending** until the final renderer asset build, actual native draft PR hand-off and final visible/focused prewalk. A new reviewed delivery identity is required; prior identities/journals are not rewritten or retried automatically. Subsequent evidence stays outside the tracked report until the hand-off finishes. CI runs on the published exact head; no local delivery gate substitutes for the user's CI-directed validation.

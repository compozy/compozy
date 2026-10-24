# Compozy desktop update causes and repair evidence

Investigation baseline: `b1008af26`, isolated worktree `desktop-update-flow`, 2026-10-21. Source references in the diagnosis below describe that baseline unless explicitly marked as the repaired implementation. No installed app or user home was modified. The native probes use the cached Electron 44.5.1 arm64 distribution under `.cache/desktop-update-flow-native-probe/`, with separate Electron user-data and session-data directories.

## Findings

### 1. The shell can quit before Squirrel starts its installation

**Confirmed current defect, repaired in this worktree.** The normal product-window path is sufficient; no network failure is required.

- `desktop/src/update/electron-installer.ts:36-37` disables both automatic download and automatic install on quit.
- The pinned `electron-updater@6.8.9` implementation, `out/MacUpdater.js:219-227`, dispatches the JavaScript download event and resolves the download without asking native Squirrel to fetch/stage the ZIP when `autoInstallOnAppQuit` is false. `MacUpdater.js:240-254` starts that native check inside `quitAndInstall()`.
- Our installer called `prepareForRestart()` before that method (`electron-installer.ts:78-80`). `main.ts:395-398` ran `cleanup()` during preparation.
- Cleanup destroys the product window (`main.ts:165-166`, `window/product-window.ts:182-186`); the boot window is normally already closed (`main.ts:348`). Closing the final window triggers the unconditional `window-all-closed` → `quitCleanly()` handler (`main.ts:459`). That handler awaits the same cleanup promise and calls ordinary `app.quit()` (`main.ts:181-187`).

The recent change `a36fa4108` added product-window disposal to cleanup. This independently useful teardown repair introduced a conflict with the update handoff because the ordinary quit path and installer path shared cleanup without a quit owner.

A native Electron probe retaining the actual event/await ordering produced:

```text
product.destroy
window-all-closed
cleanup.complete
release-lock
ordinary.app.quit
before-quit
will-quit
quit
updater.quitAndInstall/native-check
```

The process exited successfully before a 100 ms simulated native-ready callback ran. This proves the shutdown ordering with a real BrowserWindow and Electron lifecycle. The updater itself is an I/O fake in this focused probe; the signed replacement journey remains an integration requirement.

**Repair:** separate native staging from final quit. Verify the planned artifact first, await native `update-downloaded`/`error` with cancellation while the UI remains open, then drain lease work, fence, prepare shutdown, and invoke `quitAndInstall`. An ephemeral `installerOwnsQuit` flag prevents ordinary quit paths from racing cleanup. The fixed native probe records native readiness before window disposal and updater invocation before `before-quit`, `will-quit`, and `quit`.

### 2. The applying watchdog does not run while its owner is waiting for I/O

**Confirmed current defect, repaired in this worktree.**

`AppUpdateConsumer.handle()` marks the operation active and awaits the complete installation (`app-update-consumer.ts:46-73`). `OperationWatcher` awaits that handler before reading again (`operation-watcher.ts:61-88`). The consumer writes a ten-minute watchdog to the journal but does not schedule its local timeout during download; the only active timer is lease renewal. Its `#scheduleDeadline()` call occurs after the installer returns (`app-update-consumer.ts:83-148`). Even an external call to `handle()` with an expired applying operation is ignored while that ID is active (`:47`).

The daemon does not take over app applying/handoff operations (`internal/daemon/update_background.go:229-238`). Generic journal reconciliation intentionally leaves an applying operation alone while the shell process identity is alive (`internal/update/operation_reconcile.go:26-31`). That safety fence is reasonable, but it means the live shell must implement its own deadline.

A probe imported the actual consumer, blocked installer I/O, advanced its injected clock eleven minutes, and called `handle()` again. The pure policy selected `fail-silent-install`; the running consumer still had `phase=applying`, no scheduled timeout, no failure, and a 30-second renewal interval. Slow/trickling I/O or a missing native completion can therefore keep the channel occupied.

**Repair:** executable deadlines bound checking/downloading (60 seconds of inactivity, with a ten-minute absolute ceiling), native staging (five minutes), and final shutdown handoff (30 seconds). Only an increasing download percentage refreshes the inactivity timer; repeating a percentage does not keep a stalled transfer alive. The persisted applying watchdog retains the ten-minute recovery ceiling, while the active executor enforces the shorter inactivity limit without adding journal phase writes. Known timeout reasons are fixed, safe messages in `last_error`; arbitrary internal exception details remain in the logger.

`AbortSignal` reaches the adapter; the adapter cancels the real electron-updater `CancellationToken`, removes native event listeners, and checks cancellation before every later installer action. A late download or native-ready event cannot trigger installation after a deadline. Lease renewal remains active while native staging is pending and stops before the final fence. The upstream check/native-check APIs do not expose cancellation, so timeout settles our executor and fences their later results rather than claiming those platform requests were canceled.

### 3. Released older shells reject the runtime phases needed to reach their own app update

**Confirmed historical compatibility defect, already fixed in current source; strongly corroborates the reported incident but does not uniquely identify the removed journal.**

At tag `v0.3.0-beta.35`, `desktop/src/update/operation-contract.ts:14-23` accepts runtime phases `installing`, `ready-check`, and `completed`. The Go producer emits `swapping`, `health-checking`, and `finalized`. The app watcher validates the runtime object before looking at its own staged app target (`operation-contract.ts:108`). A combined runtime/app operation reaches `runtime.phase=finalized` before the shell should download its app, and the older shell instead repeatedly logs `The runtime update operation is invalid.`

Commit `d6084fdc9` (#808) fixed these parser names as well as discovery cadence. Both beta.36 and beta.37 contain the parser fix. The old shell is still the program that must consume the operation before it can be replaced, so fixing only the new release's parser does not repair the already running beta.35 updater.

A read-only source probe compiled the actual beta.35 parser obtained through `git show` and compared it with current source:

| Journal | beta.35 parser | Current parser |
| --- | --- | --- |
| runtime `finalized` plus app `staged` | Rejects with the exact runtime-operation error | Accepts; app policy selects install |
| app-only `staged` | Accepts; app policy selects install | Accepts |

The parent investigation separately observed repeated identical runtime-operation errors in the user's desktop log on 2026-10-20. The canceled journal and executing old app version are no longer available. Invalid runtime fields can produce the same message; therefore the log is corroboration, not proof of every historical field/version.

**Delivery implication:** upgrade tests must run the updater code from a previously released app, not compile both baseline and candidate from the fixed tree. A desktop-owned app-only update avoids requiring the old shell to understand runtime phases. The existing packaged test builds both fixture versions from current source and seeds a staged operation instead of invoking the full apply planner (`desktop/e2e/_electron/__tests__/updates.spec.ts:348-352`); that cannot expose this compatibility trap.

### 4. A transient acquire failure is acknowledged without a retry trigger

**Reproduced, repaired in this worktree.**

`operation-watcher.ts:84-88` records `#lastRevision` before invoking the consumer. The consumer catches transition-client errors, logs them, and returns without scheduling a retry (`app-update-consumer.ts:69-73`). For an unchanged waiting-for-app journal, subsequent polls skip that revision. A transient CLI execution failure before acquisition therefore leaves the staged operation dormant until another journal change, a shell relaunch, or the 30-minute operation reconciliation deadline.

A probe using the actual watcher and consumer against a temporary journal, with a 20 ms poll interval and one injected acquisition error, observed only one callback/acquire attempt across 180 ms. The journal remained at its original revision. This differs from revision conflicts that produce a new journal revision and can wake the watcher.

**Repair:** the consumer returns whether the revision was accepted. The watcher acknowledges it only after that return value is true. Errors before acceptance or errors whose outcome could not be journaled return false; the existing polling interval retries the same revision. A successfully journaled terminal failure returns true and is not reinstalled. The watcher still serializes callbacks and coalesces events, and the consumer retains its executor fence. The existing consumer suite now runs the real watcher against a temporary journal, proves the second acquire succeeds after one transient failure, and proves only one download attempt occurs when that attempt records failure.

### 5. Rosetta can download a different architecture from the signed plan

**Reproduced at the pinned provider selection boundary; differential guard repaired in this slice, plan architecture repair owned by the parallel Go slice.**

`internal/update/manager.go:63-64` defaults the planning architecture to `runtime.GOARCH`; `internal/update/plan.go:201-213` records the x64 ZIP when that runtime is amd64. The release manifest legitimately contains both mac architectures. `recordedUpdateAsset()` verifies that x64 exists in the feed, but does not constrain which file electron-updater selects. Pinned `MacUpdater.js:55-81` detects Rosetta/hardware arm64 and selects the arm64 ZIP. Our adapter then rejects the returned arm64 basename because the durable operation requires x64 (`electron-installer.ts:71-74` at baseline). The later SHA-256 check must also remain bound to the planned file.

Calling the real pinned `MacUpdater.filterFilesForArch()` with a dual-architecture feed and the Rosetta/arm64 flag returned only `CompozyOS-1.1.0-mac-arm64.zip`; the recorded amd64 plan requests `CompozyOS-1.1.0-mac-x64.zip`. The differential-download guard also receives `os.arch()`, which describes the running Node architecture rather than a Rosetta hardware probe, so it does not resolve this mismatch.

**Repair:** the adapter now uses Electron's supported `app.runningUnderARM64Translation` property (present in the pinned Electron 44.5.1 type definitions) to disable cross-architecture differential download. Its boundary regression failed with `false` before the repair and passes for both translated and native execution. The obsolete helper comparing two process-architecture readings and its pure helper test were removed. The parallel Go slice aligns the app asset plan with Darwin hardware architecture while preserving the runtime's own architecture. Basename and digest verification stay exact; no provider override or mutation of protected updater state was added. Full cross-architecture installation on an actual Rosetta app remains unverified.

## One-click ownership model

For a desktop-owned runtime, the app bundle can be the single downloaded product update. After relaunch, existing bootstrap already checks the signed runtime bundle, stops the older desktop-owned daemon, promotes the bundle, and starts the new daemon:

- `desktop/src/main.ts:311-317` verifies the bundle before starting the consumer/bootstrap runner.
- `internal/cli/daemon_bootstrap.go:146-201` probes the running daemon, replaces outdated desktop-owned runtime state, provisions the bundle, and starts it.
- `internal/cli/daemon_bootstrap_desktop_runtime.go:19-49,94-117` preserves a compatible newer runtime, refuses to claim operator-owned runtime state, and otherwise stops the old daemon by recorded process identity.
- `internal/cli/daemon_bootstrap_provenance.go:59-90` uses the existing binary replacement and provenance path. These paths do not acquire the update-operation journal lock, so an app `installer-handoff` journal does not itself block bootstrap promotion.

Retain runtime-only updates for standalone/operator installations and public CLI compatibility. The default desktop action should select the app, display download progress, await native readiness while the product remains usable, restart once, then show observed installed version truth. A failure should expose Retry directly; canceling a dormant journal should be recovery machinery rather than a required normal user step. The parent owns target selection and UI retry changes.

This recommendation still needs a released-old-app → candidate app integration run proving the new bundled daemon PID/version, preserved user state, and correct final journal outcome. The separate active runtime-skew worktree is outside this task and was not modified.

## Regression ownership and verification

| Invariant | Owning layer / canonical suite | Evidence |
| --- | --- | --- |
| Blocked installer I/O becomes terminal; real progress refreshes only the idle limit; late completion never installs | Desktop consumer, `app-update-consumer.test.ts` | Download/handoff regressions failed before repair; pass with 60-second inactivity, progress renewal, repeated-percent stall, and the absolute ten-minute ceiling |
| Native readiness precedes shutdown; native error/abort preserves the shell; download cancellation reaches the updater | Electron adapter, existing `electron-installer-policy.test.ts` | Four new regressions failed before repair; pass after repair, including late update-downloaded after abort |
| An unaccepted staged revision retries while a journaled terminal failure does not reinstall | Consumer and actual filesystem watcher, existing `app-update-consumer.test.ts` | Regression failed with a permanently staged operation before repair; passes with two acquisitions and one download |
| Old runtime phase vocabulary cannot strand the staged app | Shell contract/policy, `consumer-policy.test.ts`; released-version integration | Existing current-source phase tests pass; actual beta.35 parser probe reproduces historical rejection |
| No ordinary quit before native handoff | Main lifecycle, existing packaged E2E-016 | Native BrowserWindow red/green probe completed; signed packaged E2E delegated to parent |
| App-only relaunch promotes its owned bundled daemon | Bootstrap, `internal/cli/daemon_bootstrap_test.go`; `APP-desktop-runtime-bundle-coherence` | Source path and existing suite identified; actual old-to-new run required |
| Rosetta does not attempt an x64 differential for the selected native app | Electron adapter, existing `electron-installer-policy.test.ts`; plan selection in Go | Adapter regression red/green; Go hardware-selection repair tracked separately; native Rosetta walk still required |

Commands completed for this slice:

```text
bunx turbo run test --filter=./desktop --only -- --coverage.enabled=false src/update/__tests__
  red: 6 failed, 12 passed
  initial green: 18 passed
  acquisition/inactivity red: 6 failed, 2 passed in consumer suite
  Rosetta red: 1 failed, 7 passed in adapter suite
  final updater suites: 24 passed
bunx turbo run test typecheck lint --filter=./desktop --only
  3 tasks passed; 191 tests in 22 files; zero lint warnings/errors
```

The first dependency-inclusive Turbo run stopped before tests because baseline `extensions/open-design/lint.gen.mjs` did not match the local generator under Bun 1.4.3. It was not repaired or counted green by this slice. Repository-pinned Bun 1.4.0 passes that generator check. The [integrated verification report](../../qa/reports/2026-10-21-desktop-update-flow.md) records the final gate and eight passing native packaged tests, including observed app/runtime 0.3.0 → 0.3.1. Those locally signed fixture apps both contain the repaired source; previously released app code and the production signature chain remain separate release acceptance.

Residual lifecycle limit: failure during the final destructive cleanup can leave the shell partially drained even though the operation records failure. The repair keeps all network/native-preparation failure paths before cleanup and bounds cleanup to avoid an indefinitely live executor, but it does not attempt to reconstruct an aborted bootstrap inside the same process.

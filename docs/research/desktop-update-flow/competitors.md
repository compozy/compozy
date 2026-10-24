# Desktop auto-update comparison

Research date: 2026-10-21. Scope: the three local source checkouts requested by the user. Sources were read without executing their apps, update installers, provider integrations, or release workflows. No installed app, release, or checkout was changed. This is source-level evidence, not proof of production reliability.

## Main result

MonoCode provides the closest literal match to the requested interaction: its sidebar action downloads, installs, and relaunches without a second confirmation. Synara automatically prepares the payload first, then offers a single restart/install action. T3 Code's local-only flow still separates download and install and confirms the restart; its broader “Update Everything” operation performs the steps after one confirmation and a postponable five-second countdown. The three implementations therefore do not establish that every competitor has the same one-click flow.

The most relevant Electron lifecycle detail is that Synara and T3 Code keep their main windows alive while stopping backends before `quitAndInstall`. T3 destroys windows only when Electron emits native `before-quit-for-update`; both avoid a plain `app.quit()` in the macOS `window-all-closed` handler. Neither treats a successful call to `quitAndInstall` as proof that the new version is running.

## Source snapshots

The SHAs, descriptions, and clean working-tree state below were obtained from `git rev-parse HEAD`, `git describe --tags --always --dirty`, and `git status --short` in each checkout. These are the inspected snapshots, not claims about the latest published releases.

| Checkout | SHA | Git description | Relevant declared dependencies |
| --- | --- | --- | --- |
| `.resources/synara` | `299fd573b1d6520c323a87c78d30be74e87e53e1` | `v1.0.7-4-g299fd573b` | Desktop package `1.0.7`, Electron `43.4.1`, `electron-updater ^6.6.2`. [Package](/Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/package.json:1) |
| `.resources/t3code` | `5b2ee393c3d6576bcf44c5d2b74565770943e189` | `v0.1.1-nightly.20261020.3438-39-g5b2ee393c` | Desktop package `0.1.0` is not the release identifier; Electron `44.4.5`, `electron-updater ^6.8.9`. [Package](/Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/package.json:1) |
| `.resources/monocode` | `115f0e97c4f869188b5868cc4e9969d6f1a3f972` | `v0.16.0-12-g115f0e9` | App/package `0.16.0`; Rust lockfile pins `tauri-plugin-updater 2.10.1`. [Config][M-config], [lockfile][M-lock] |

Source roots are `/Users/pedronauck/Dev/compozy/compozy/.resources/{synara,t3code,monocode}`. Citations below point to those actual files and starting lines. Dependency ranges are reported as ranges; they are not represented as installed versions.

## Observable interaction and ownership

| Phase | Synara | T3 Code | MonoCode |
| --- | --- | --- | --- |
| Discovery | Startup after 15 seconds, four-hour polling, foreground recheck with time/background thresholds; manual check also exists. | Startup after 15 seconds, four-minute polling, manual check. | Sidebar probes on mount; Settings/menu can check manually. No periodic timer exists in the inspected sidebar/updater modules. |
| Download start | Own handler invokes background download immediately after an acceptable update is discovered. | Local-only flow requires download action. “Update Everything” also downloads the local app as part of its operation. | Sidebar “Update to …” starts download and install together. |
| Staged/ready UI | Disabled preparing/progress state, then actionable Update. | Downloaded state offers install/restart. | No separate downloaded/staged state in the application model. |
| User action after download | Click Update once to restart/install; download retry can require a later install click. | Local-only: click install and confirm. Update Everything: automatic restart after five-second countdown, with Later. | None; `downloadAndInstall` completion is followed by `relaunch`. |
| Failed install | Retryable state; restarts backend if remaining in old process; durable marker detects old version on next launch. | Retryable state and backend recovery on reported errors; durable record detects update not applied on next launch. | Error snapshot and dialog. Sidebar hides error state; Settings/menu remain recovery entry points. |

Evidence: Synara [timers][S-timers], [background preparation][S-background], [button policy][S-button], [click handler][S-click]; T3 [timers][T-timers], [local actions][T-actions], [combined operation][T-everything]; MonoCode [sidebar][M-sidebar], [update flow][M-updater].

## Synara: predownload, validate, explicit handoff

### Discovery, transfer, and staging

- `configureAutoUpdater` sets `autoDownload=false` and `autoInstallOnAppQuit=false`, but its own `update-available` handler invokes `prepareAvailableUpdateInBackground`. The boolean alone is therefore misleading: downloads are automatic at the application layer. The packaged `app-update.yml` owns the GitHub feed; a flavor-specific channel and prerelease policy are set explicitly. Candidate versions must belong to the app flavor and be newer than the running version. [Configuration/events][S-config], [candidate guard][S-candidate], [background preparation][S-background]
- A successful download is not enough to mark the payload ready. `update-downloaded` starts fingerprinting the downloaded artifact; the action waits for that identity task, and install rechecks path/size/SHA-512 before and after backend shutdown. The identity utility rejects symlink and non-regular payload paths and detects replacement during hashing. [Download action][S-download], [install handoff][S-install], [artifact identity][S-identity]
- The application replaces the updater's transfer function with a resumable implementation: HTTP Range, bounded retries, working inactivity timers, size/hash verification, and one clean redownload after failed verification. It discards existing temporary bytes at the start rather than trusting them as a prior resume. [Transfer implementation][S-transfer]
- The source comment attributes this patch to an Electron request/socket-timeout mismatch. That upstream explanation was not independently reproduced here; the local override and its retry/hash behavior are directly visible in code. This is useful evidence that adopting `electron-updater` alone did not remove all operational problems for Synara. [Transfer rationale][S-transfer-rationale]
- Stale/unacceptable candidates and changed artifacts clear the pending cache internally. Deletion is deferred while a download is in flight, avoiding deletion underneath the transfer. The normal user flow contains no “cancel staged update” requirement. [Pending cache handling][S-cache], [deferred deletion queue][S-cache-queue]

### Window and quit lifecycle

1. Installation admits one preparation attempt, keeps the existing main window, marks quitting, pauses update polling, and stops the backend. `stopBackendAndWaitForExit` suspends its CUA host and terminates the backend; it does not close the main BrowserWindow. [Preparation/handoff][S-install], [backend stop][S-backend-stop]
2. It revalidates the artifact, writes an exact attempt/version/artifact marker, marks the handoff active, calls `electron-updater.quitAndInstall()`, and arms the watchdog. It returns `accepted=true, completed=false`. [Install handoff][S-install]
3. General app shutdown is a different path: `shutdownDesktopRuntime` hides the window and disposes browser services. Install preparation does not call that general shutdown routine. [General shutdown][S-shutdown]
4. `before-quit` permits an updater-owned quit after validating its marker. While install preparation is still underway, ordinary quit is prevented and remembered for later replay. The code explicitly warns that canceling an updater-owned quit converts the operation into a plain app quit. [Quit ownership][S-quit]
5. The macOS close guard is disabled during update preparation/handoff, and `window-all-closed` calls `app.quit()` only on non-macOS platforms. [Window close][S-window], [all windows closed][S-all-closed]

The app imports `autoUpdater` from `electron-updater`, and its visible `update-downloaded` listener belongs to that library. No application-level listener waiting for Electron/Squirrel's native `update-downloaded` was found in the inspected flow. Native readiness/install behavior is delegated to the library; the dependency's `MacUpdater.js` was absent from both local root and desktop `node_modules` candidates, so this report does not assert the library's exact internal readiness ordering. [Import][S-updater-import], [download event][S-downloaded]

### Failure recovery and release integrity

- Timers bound check at 45 seconds, stalled download at 60 seconds, cancellation settlement at 20 seconds, and a process that remains alive after install handoff at 15 seconds. The install watchdog clears the attempt state, reports failure, and restores the backend/update polling unless a deferred normal quit should proceed. [Timer constants][S-timers], [watchdog][S-watchdog], [recovery][S-recovery]
- A durable marker records from/to versions and attempt identity. On launch, success clears the marker and legacy cache; running the old version records failure and suppresses automatic activity until explicit retry. The renderer recommends manual download after at least two install failures, but this is fallback, not successful update flow. [Startup verification][S-marker], [manual fallback threshold][S-fallback]
- The macOS release finalizer creates the updater ZIP with native `ditto`, verifies Electron framework symlinks, extracts the ZIP again, and runs strict code-signature verification on the extracted app, including expected bundle identity. It then calculates SHA-512/size from the final ZIP and rewrites manifest metadata. Stale ZIP blockmaps are deleted, and macOS differential download is disabled. [ZIP finalizer][S-finalizer], [signature verifier][S-signature], [runtime differential policy][S-config]
- This protects the exact update payload, not merely the DMG used for initial installation. The release documentation says ZIP finalization occurs after notarization. The HEAD-only artifact smoke is explicitly a metadata/addressability check; it does not establish that an installed app successfully upgrades and relaunches. [Release contract][S-release], [smoke scope][S-smoke]

## T3 Code: separate local actions, native-owned quit

### Discovery, actions, and installation

- The updater has `autoDownload=false` and `autoInstallOnAppQuit=false`. Update discovery updates state; it does not itself start transfer. A single active-action reservation coordinates checks/downloads/install/channel changes. Download temporarily disables differential transfer when an Intel build runs on an Apple Silicon host. [Configuration][T-config], [action reservation/download][T-download]
- The local-only sidebar starts `downloadUpdate`, then offers `installUpdate` with an interruption confirmation. Its different “Update Everything” flow is selected when servers/providers also have updates; it downloads the app as part of the operation, handles other targets, then calls install automatically after a five-second countdown. This distinction matters when comparing click counts. [Local actions][T-actions], [selection criterion][T-everything-selection], [combined operation][T-everything], [countdown][T-countdown]
- Install records from/to versions and a restart marker, snapshots running backends, stops every backend with a five-second bound, then calls `quitAndInstall({isSilent:true,isForceRunAfter:true})`. It does not close windows before this call. The API intentionally returns accepted but not completed, because the current process cannot prove the next version launched. [Install][T-install], [record][T-record]
- The exact native ownership boundary is explicit: the Electron adapter subscribes to `Electron.autoUpdater`'s `before-quit-for-update`; the lifecycle sets `updaterQuitAllowed=true` and synchronously destroys windows at that point. Its normal `before-quit` handler respects this ownership. `window-all-closed` never initiates quit on macOS and checks the quitting state on other platforms. [Native adapter][T-native], [lifecycle][T-lifecycle]

### Recovery and packaging

- Synchronous install failure, thrown defects, and updater `error` events while install is active invoke recovery: clear quitting/install markers, restart only previously running backend instances, and expose a retryable state. Startup reads the durable install record and reports `notAppliedVersion` when the attempted update did not take effect. [Recovery][T-recovery], [record][T-record], [startup/renderer][T-startup], [retry presentation][T-retry]
- No same-process silent-install watchdog equivalent to Synara's was found in `DesktopUpdates.ts`; the observed timers there are startup/polling, backend stop, and a 90-second wait for an interfering check during prepared install. This is a limitation of the inspected application flow, not a demonstrated production hang. [Timers][T-timers], [install admission][T-install]
- Release packaging emits a macOS DMG plus update ZIP, selects a GitHub stable/nightly feed, and merges platform manifests for multiple architectures. The workflow enables signing/notarization when configured credentials are present, but has an unsigned branch when required secrets are missing. Source configuration therefore does not prove every published artifact is signed. [Packaging][T-package], [feed][T-feed], [manifest merge][T-manifest], [signing workflow][T-signing]

## MonoCode: one action delegates install to Tauri

- The Rust application registers Tauri's updater and process plugins; frontend capabilities expose them. Release builds inject a public update-verification key and HTTPS feed URL. The checked-in base config has empty updater settings and ad-hoc macOS signing, while the release workflow rejects missing/ad-hoc release signing identity and supplies the real configuration. [Plugin registration][M-plugins], [base config][M-config], [release configuration][M-release-config]
- The sidebar probes on mount, displays `Update to <version>`, and immediately guards duplicate clicks with a ref. `installPendingUpdate` calls `downloadAndInstall(progress)`, records a “what changed” notice only after that promise succeeds, clears the pending object, and awaits `relaunch()`. The frontend has no separately persisted staged-update transaction or second install action. [Sidebar][M-sidebar], [install flow][M-install]
- Manual check is slightly different: Settings/menu call `runUpdateFlow(true)`, which asks whether to install when a candidate is discovered. If Settings already has an available candidate, its action invokes install directly. [Manual flow][M-manual], [Settings action][M-settings]
- Download/install errors become an error snapshot and a dialog; the pending candidate is retained if installation throws before success. The sidebar renders only available/downloading states and therefore hides after error. Retrying through manual check is available, but an always-visible in-place Retry affordance is not established by this source. Missing feed/target errors are quiet for automatic checks and explain manual download for explicit checks. [Error behavior][M-install], [check fallbacks][M-manual], [sidebar visibility][M-sidebar]
- Linux `.deb`/`.rpm` are explicitly package-managed and never self-installed. The feed includes a specific AppImage target rather than a generic Linux target. This installer distinction is intentional and should not be flattened into one platform rule. [Package manager policy][M-updater], [feed targets][M-feed]
- The release workflow builds and signs/notarizes separate arm64/x86_64 macOS apps, validates architecture/signatures, publishes `.app.tar.gz` payloads with detached signatures and a per-platform JSON feed, and uploads immutable versioned payloads before publishing the uncached feed. Stable and prerelease feeds are separate. [macOS release][M-release-config], [feed][M-feed], [publish order][M-publish], [channel selection][M-channel]
- `rememberInstalledUpdate` is a UI notice in local storage. It is not equivalent to Synara/T3's old/new-version reconciliation on the next startup. No custom install watchdog, durable install-failure marker, or application-level rollback was found in the inspected updater modules. Actual extraction/replacement/permission handling is delegated to `tauri-plugin-updater`; that dependency's implementation and a real macOS upgrade were not executed in this scope. [Notice implementation][M-notice], [install flow][M-install], [pinned plugin][M-lock]

## Design implications for Compozy — recommendations, not diagnosed causes

1. Present one user intent: Update. If no verified payload is ready, the same accepted action should continue through download, verification, install, and relaunch. MonoCode proves this can be a small frontend flow; Synara shows that background preparation can shorten the time after clicking without exposing a separate staging task.
2. Keep updater handoff distinct from ordinary shutdown. Stop owned backend work as necessary, but keep the process/windows needed by the native updater alive until it owns quit. T3's native `before-quit-for-update` boundary is the strongest direct comparison for when to destroy windows. Whether Compozy currently violates this boundary must be established against its own pinned updater and lifecycle.
3. Treat progress, handoff, and observed success as different evidence. A resolved download RPC or returned `quitAndInstall()` cannot prove relaunch. Preserve retry/error state in the same user control; check from/to version on next startup and use bounded recovery if the old process remains alive.
4. Repair stale pending state internally when safe. Clearing a failed cache belongs to the updater's retry path; users should not need to discover a Settings cancellation action to make another attempt possible.
5. Validate the exact signed update payload after archive extraction and publish its metadata only after its final bytes are available. A successful manual DMG install is not evidence that the update ZIP/tar, feed, architecture selection, and native install handoff work.
6. Borrow the minimum proven mechanism, not a whole competitor subsystem. Synara's transport patch and durable markers demonstrate real failure classes, but are not automatic justification for copying its substantial custom implementation. First reproduce Compozy's failing sequence and use its existing updater facilities where they cover that behavior.

Validation required beyond this source research: a real isolated macOS upgrade from an installed older signed build to the candidate release, with progress, app replacement, process exit, relaunch, and observed new version; plus interrupted transfer and failed-handoff recovery. This report contains no claim that those runtime scenarios passed for any competitor.

[S-timers]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:507
[S-updater-import]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:64
[S-config]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:3687
[S-candidate]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:2948
[S-background]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:3418
[S-download]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:3319
[S-downloaded]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:3843
[S-install]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:3524
[S-identity]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/updateArtifactIdentity.ts:39
[S-button]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/web/src/components/desktopUpdate.logic.ts:47
[S-click]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/web/src/components/Sidebar.tsx:6594
[S-fallback]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/web/src/components/desktopUpdate.logic.ts:188
[S-transfer-rationale]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/resumableUpdateDownload.ts:1
[S-transfer]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/resumableUpdateDownload.ts:508
[S-cache]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:3076
[S-cache-queue]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/updatePendingCache.ts:68
[S-backend-stop]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:4763
[S-shutdown]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:4829
[S-quit]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:6195
[S-window]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:5803
[S-all-closed]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:6377
[S-watchdog]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:1194
[S-recovery]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:1070
[S-marker]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/apps/desktop/src/main.ts:3009
[S-finalizer]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/scripts/lib/mac-update-zip-finalize.ts:243
[S-signature]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/scripts/lib/mac-update-zip-finalize.ts:134
[S-release]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/docs/release.md:81
[S-smoke]: /Users/pedronauck/Dev/compozy/compozy/.resources/synara/scripts/smoke-mac-update-artifact.ts:259
[T-timers]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/updates/DesktopUpdates.ts:52
[T-config]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/updates/DesktopUpdates.ts:1066
[T-download]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/updates/DesktopUpdates.ts:433
[T-actions]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/web/src/components/sidebar/SidebarUpdatePill.tsx:274
[T-everything-selection]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/web/src/components/sidebar/useUpdateEverything.ts:94
[T-everything]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/web/src/components/sidebar/useUpdateEverything.ts:140
[T-countdown]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/web/src/components/sidebar/updateEverythingRun.tsx:137
[T-install]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/updates/DesktopUpdates.ts:690
[T-record]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/updates/DesktopUpdates.ts:586
[T-native]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/electron/ElectronApp.ts:196
[T-lifecycle]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/app/DesktopLifecycle.ts:206
[T-recovery]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/updates/DesktopUpdates.ts:645
[T-startup]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/desktop/src/updates/DesktopUpdates.ts:1042
[T-retry]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/apps/web/src/components/desktopUpdate.logic.ts:93
[T-package]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/scripts/build-desktop-artifact.ts:2861
[T-feed]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/scripts/build-desktop-artifact.ts:2738
[T-manifest]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/scripts/merge-update-manifests.ts:51
[T-signing]: /Users/pedronauck/Dev/compozy/compozy/.resources/t3code/.github/workflows/release-desktop.yml:480
[M-config]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src-tauri/tauri.conf.json:1
[M-lock]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/Cargo.lock:5235
[M-sidebar]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src/app/shell/SidebarUpdate.tsx:12
[M-updater]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src/app/model/updater.ts:26
[M-manual]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src/app/model/updater.ts:86
[M-install]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src/app/model/updater.ts:183
[M-settings]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src/features/settings/ui/SettingsView.tsx:1881
[M-plugins]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src-tauri/src/lib.rs:265
[M-release-config]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/.github/workflows/release.yml:70
[M-feed]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/.github/workflows/release.yml:235
[M-publish]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/.github/workflows/release.yml:292
[M-channel]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/scripts/release-channel.cjs:3
[M-notice]: /Users/pedronauck/Dev/compozy/compozy/.resources/monocode/src/app/model/updateNotice.ts:12

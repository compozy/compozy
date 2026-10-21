# App-owned update planning

Scope: `internal/update/plan.go`, its existing `TestManagerCheck` suite, and the CLI update completion projection. This is source and isolated-test evidence. The parent task owns packaged-app QA and release validation.

## One bundle owns the desktop update

After validating the original request and both tracks' eligibility, `PlanOperation` normalizes `[runtime, app]` to `[app]` only when the runtime is an unmanaged `desktop-app` installation. Explicit runtime-only requests, independently installed runtimes, and managed-install refusals retain their behavior. The caller's target slice is unchanged. The acquired durable operation contains only the verified app identity; planning downloads the checksum catalog, its verification bundle, and compatibility metadata, with no runtime archive. See [plan.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/update/plan.go:19) and [canonical regression](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/update/manager_check_test.go:859).

This relies on existing bootstrap ownership, not a new runtime installer:

- Install detection requires the canonical home binary path and a matching provenance digest; the managed override wins first. [detect.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/update/detect.go:88)
- On launch, bootstrap replaces an outdated desktop-owned running daemon, then compares the installed binary and bundled release identity before provisioning. It preserves a newer compatible runtime. [daemon_bootstrap_desktop_runtime.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/cli/daemon_bootstrap_desktop_runtime.go:18), [daemon_bootstrap.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/cli/daemon_bootstrap.go:147)
- Provisioning refuses to replace an independently owned runtime, uses the existing binary replacement operation, writes provenance, and rolls back replacement if provenance cannot be recorded. [daemon_bootstrap_provenance.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/cli/daemon_bootstrap_provenance.go:28)

The inference is that the desktop bundle can satisfy the joint intent without first swapping the daemon underneath the old shell. This also removes the intermediate runtime journal phases from that path. The tests establish the new plan and artifact behavior; successful native bundle replacement still requires the packaged-app QA owned by the parent task.

## CLI completion observes the replacement

The old CLI terminal projection marked a verified app updated while retaining the original runtime's `available` status and old version whenever the normalized request had no runtime plan. App verification alone is insufficient: the shell starts its update consumer before the bootstrap runner finishes. [main.ts](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/desktop/src/main.ts:310)

The CLI now obtains the runtime projection from the local daemon's existing settings endpoint, bounded by the update deadline. It waits for bootstrap identity/readiness checks, a runtime version at least equal to the requested release, and a minimum-app-version constraint that accepts the updated shell. The daemon status and settings snapshot must agree on the runtime version. It copies the actual runtime snapshot, including a newer compatible version, status, and `daemon_restarted`; it does not infer those fields from `App.ToVersion`. A closed app remains staged. Failure to confirm the runtime produces an explicit runtime failure while preserving the app's verified outcome. [update_projection.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/cli/update_projection.go:47), [update_runtime.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/cli/update_runtime.go:167)

The readiness check reuses the existing bootstrap probe: the daemon must report running and match its published local discovery record's PID, process start, port, and socket. This is the existing bootstrap readiness contract; the settings endpoint by itself does not guarantee it. The daemon publishes discovery after `bootComponents`; its HTTP status otherwise reports running unless draining. [daemon_bootstrap.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/cli/daemon_bootstrap.go:355), [boot.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/daemon/boot.go:165), [schema_stream_status.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/api/core/schema_stream_status.go:55)

## Rosetta app identity

The pinned `electron-updater` is 6.8.9. `MacUpdater` prefers arm64 files on Apple Silicon, including when the process is translated by Rosetta; it probes translation and hardware before filtering release files. A Go runtime reporting `amd64` therefore cannot by itself determine the app ZIP identity. [package.json](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/desktop/node_modules/electron-updater/package.json:3), [MacUpdater.js](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/desktop/node_modules/electron-updater/out/MacUpdater.js:28)

For Darwin/amd64 app planning only, the planner uses the existing command I/O boundary to read `/usr/sbin/sysctl -n hw.optional.arm64` with a one-second timeout. A positive capability selects the arm64 app ZIP. Intel hardware or an unavailable probe retains x64 selection. Parent cancellation remains an error. Runtime archive selection continues to use the runtime's own architecture, and other platforms retain their behavior. [plan.go](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/update/plan.go:200), [asset-selection regression](/Users/pedronauck/.compozy/worktrees/compozy/desktop-update-flow/internal/update/manager_check_test.go:737)

## Regression evidence

All regressions reuse the owning suites and their existing fixture utilities. Release planning uses temporary home files, real provenance and local HTTP artifact delivery; only GitHub metadata transport and the Sigstore verifier's I/O boundary are substituted. CLI completion uses the real daemon HTTP client against a local fixture server and a real temporary discovery record. No installed application or operator home is changed.

| Invariant | RED observation before the fix | Owning suite |
| --- | --- | --- |
| Desktop joint intent is fulfilled by the app bundle | Planned/acquired `[runtime, app]`, active target `runtime`, one runtime archive downloaded | `TestManagerCheck/Should plan desktop-owned joint updates through the app bundle` |
| Rosetta planner agrees with native app payload selection | Selected `CompozyOS-1.2.0-mac-x64.zip` on the simulated Apple Silicon host | `TestManagerCheck/Should select only the platform-supported desktop updater asset` |
| CLI reports the observed replacement runtime | Returned `updated` with old runtime `available/v1.0.0`, making zero daemon reads; bootstrap failure was also reported as success | `TestUpdateTerminalProjection/Should refresh the app-owned runtime from the local daemon after verified app install` |
| A matching version alone does not establish readiness | Completed without querying daemon status while the new daemon was still `starting` | Same CLI projection suite, readiness case |
| A newer runtime must accept the updated shell | Reported stable `v0.3.0` with minimum app `v0.3.0` as success for app `v0.3.0-beta.38` before bootstrap replaced that incompatible daemon | Same CLI projection suite, stable/beta compatibility case |

Final scoped verification:

- `CGO_ENABLED=1 go test -race ./internal/update` — PASS.
- `CGO_ENABLED=1 go test -race ./internal/cli -run 'TestUpdateCommandContract|TestUpdateTerminalProjection|TestSettingsRestartTimeout|TestLocalRestartStatusDuringBoot' -count=1` — PASS, including readiness and stable/beta compatibility cases.
- `COMPOZY_GO_LINT_SCOPES='./internal/update ./internal/cli' make go-lint` — PASS, zero issues for default and integration build tags.
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./internal/update` — PASS.
- Existing test-convention checker on both changed canonical test files — PASS. `git diff --check` on the owned changes — PASS.

The [integrated verification report](../../qa/reports/2026-10-21-desktop-update-flow.md) records the root gate and native installer journey. Eight packaged macOS checks pass, including app/runtime 0.3.0 → 0.3.1 and failed-handoff recovery. Both fixture versions contain repaired source; this is not a published-old-app upgrade receipt.

## Contract and scope

No public DTO, route, command, flag, journal schema, persisted user-data shape, config key, native tool, hook, or extension contract is added or renamed. The daemon apply response can retain the original requested targets because they express intent, while the internal durable operation uses the app target. CLI terminal fields retain their existing shape and are refreshed from actual daemon truth. Runtime-only CLI behavior is preserved. The parent task owns the cross-surface impact audit, Web/Docs updates, and final QA scenario evidence.

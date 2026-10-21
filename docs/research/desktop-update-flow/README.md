# Desktop update investigation and repair

Date: 2026-10-21. Baseline: `b1008af26`. Branch: `fix/desktop-update-flow` in the
isolated `desktop-update-flow` worktree. Subsequent review and delivery are tracked
in [PR #886](https://github.com/compozy/compozy/pull/886). No release publication
or change to the operator's installed app is part of this work.

## Outcome

The principal current macOS defect was a shutdown race: Compozy destroyed its
last window before Squirrel had staged the update, allowing ordinary `app.quit()`
to terminate the updater. Separate defects allowed stalled I/O and a transient
acquisition failure to leave an operation occupying the update channel.

The repaired desktop-owned flow is:

1. Activate Update in the menubar or Settings. The existing daemon planner checks
   eligibility and verifies release metadata.
2. Download and verify one app bundle. Native staging finishes while the product
   window remains available; actual deadlines bound stalled work.
3. Let the native installer own the one restart. Existing bootstrap provisions
   the runtime bundled with the app only when provenance establishes ownership.
4. Observe the new app and ready daemon. On a download/staging failure, archive
   the attempt and offer Retry update directly, without a cancellation detour.

No new service, dependency, public protocol, journal schema, or parallel updater
was introduced. Explicit runtime-only requests and independent/managed installs
keep their existing behavior. A compatible newer runtime remains installed.

## Confirmed causes and changes

| Cause | Evidence and repair |
| --- | --- |
| Ordinary quit races native installation | Actual Electron BrowserWindow reproduction; native readiness now precedes cleanup and the installer owns quit |
| Persisted watchdog never interrupts the live waiter | Blocked-I/O reproduction; executable inactivity/absolute/staging/handoff deadlines, cancellation and late-result fences |
| Watcher acknowledges a failed acquisition | Actual filesystem watcher reproduction; acknowledge only accepted revisions and reuse the existing polling retry |
| beta.35 rejects newer runtime journal phases | Exact released parser reproduction; already repaired in beta.36/37, but an old running updater cannot acquire that parser fix before installation |
| Rosetta plan and downloader choose different ZIPs | Pinned MacUpdater selection plus Go boundary regression; app planning follows Apple Silicon hardware and disables cross-architecture differential download |
| Joint desktop update swaps runtime before replacing its owner | Verified planner regression; normalize eligible desktop-owned joint intent to the app bundle without downloading another runtime archive |
| A handoff error after cleanup can leave a windowless shell | Native filesystem-failure regression; finish existing cleanup and quit after failure is journaled |
| CLI copies a restart flag that omits successful app-owned restarts | Real discovery/HTTP regression; compare daemon PID and start time before and after the update |
| App success can precede daemon readiness | Real local HTTP client regression; CLI waits for published daemon identity, compatible versions and a matching runtime snapshot |
| Settled failure offers diagnostics but no direct retry | Existing Web suite regressions; menubar and Settings expose Retry update when a release is eligible and the channel is free |

The user's desktop log contained 22 instances of `The runtime update operation
is invalid.` on 2026-10-20. That corroborates the historical parser failure;
the canceled journal and executing app version are unavailable, so it cannot
uniquely reconstruct that incident. The current shutdown and liveness defects
were reproduced independently.

## Competitor evidence

- [Grok Bot artifact analysis](grok-bot.md): REA analysis of the installed 0.75.0
  artifact, hashed identity, packaged JavaScript lifecycle, timers, native staging
  and recovery. [Machine-readable REA evidence](grok-rea-evidence.json) preserves
  the partial semantic-graph coverage and nine untrusted unpacked native entries.
- [Synara, T3 Code and MonoCode comparison](competitors.md): exact local checkout
  revisions and source citations. Grok and Synara stage before a restart action;
  MonoCode's sidebar downloads/installs/relaunches in one action. T3 has a separate
  local confirmation flow and a combined countdown flow, so the competitors do
  not all expose identical interaction sequences.
- [Compozy causal reproductions](compozy-causes.md): shutdown race, deadlines,
  released parser compatibility, transient watcher failure and Rosetta selection.
- [App-owned planner and CLI evidence](compozy-app-owned-plan.md): eligibility,
  ownership, exact artifact selection and observed runtime readiness.

The useful shared pattern is one authority for each phase: release eligibility,
verified download, native staging, native quit, then observed installed truth.
Competitor source inspection does not establish their runtime reliability.

## Verification and delivery boundary

The [targeted verification report](../../qa/reports/2026-10-21-desktop-update-flow.md)
records commands, native rehearsal results and remaining release acceptance.
The existing suites own the regressions; the packaged E2E fixture now builds
different bundled runtime versions and checks the actual daemon after relaunch.
The [cross-surface impact audit](../../_memory/change-impact.md) records native
tools, CLI/HTTP/UDS, extensibility, user state, Web, Docs and the official skill.

Local fixture signatures are ad hoc. Both fixture app versions are compiled from
this repaired tree and use a seeded verified operation; this proves the native
replacement/bootstrap path, not the published release's signature/metadata chain
or an already shipped old updater. Release acceptance still needs the actual
previously released app to consume the signed candidate, including a Rosetta
installation and Linux. Those results must not be inferred from the local run.

An already running broken updater cannot be repaired by code it has not installed.
Affected historical versions may require one recovery installation or a separately
verified bridge. Network/native-staging failures are recoverable before cleanup;
an exceptional failure during final destructive cleanup is journaled and exits the
drained shell so it can be relaunched.

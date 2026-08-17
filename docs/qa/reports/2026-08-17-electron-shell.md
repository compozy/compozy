# QA Run Report — 2026-08-17 — electron-shell

- **Scope:** End-to-end closeout of the Electron shell cutover, runtime and app update authority, agent-facing update surfaces, web parity, release contracts, packaged desktop smoke, and release documentation.
- **Cadence tier:** full
- **Build:** `electron` at `560cd179` plus the QA ledger changes in this report
- **Environments:** isolated macOS and Linux QA labs; browser parity against an isolated daemon and web proxy; hosted signed Electron artifacts on GitHub Actions
- **Started:** 2026-08-16T10:37:38-03:00 · **Finished:** 2026-08-17T11:03:56-03:00
- **Status:** ready for code review with an explicit release-decision block

## Executive Summary

The Electron implementation and every locally or hosted-walkable product surface passed. The run records 27 passing scenarios and eight `blocked-decision` scenarios. The blocked group is not a software failure: the operator explicitly required deletion of every beta tag and GitHub release created during QA and required that none be recreated. That decision removes the public beta channel state needed to prove a real N→N+1 install and self-update.

The retained GitHub Actions receipt proves that the beta.20 release plan, signed Linux build, signed macOS x64 and arm64 builds, and all three packaged-desktop provisioning smokes passed before the workflow was cancelled. The release publication, desktop channel publication, package-manager publication, and public N→N+1 update were not completed. The associated tags and releases were then deleted.

This branch is suitable for code review. It must not be described as release-ready until a future, explicitly authorized run publishes two consecutive beta versions and completes the installed-app update walk.

## Personas

| Persona | Goal | Primary surfaces |
|---|---|---|
| Ada | Operate CompozyOS without relying on the desktop UI | CLI, HTTP, UDS |
| Dora | Publish a coherent release from one authority | release workflow, package metadata, docs |
| Bruno | Understand and apply app/runtime updates safely | Settings, menubar, durable update operation |
| Lea | Install and start from an empty machine state | signed packages, first-run provisioning |
| Théo | Use desktop-native navigation safely | deep links, single instance, zoom, quit |
| Cora | Find accurate installation and migration guidance | website, release notes, migration docs |

## Journeys and Charters

- `J-desktop-first-run` through `CH-electron-offline-first-run-macos` and `CH-electron-offline-first-run-linux`.
- `J-desktop-attach-daily` and desktop shell safety through the macOS and Linux shell charters.
- `J-desktop-link-driven` through deep-link, single-instance, navigation, and zoom walks.
- `J-desktop-update-moment` through browser parity, Settings, menubar, CLI, HTTP, and UDS update operations.
- `J-publish-compozy-beta` through explicit release planning, signed artifact production, packaged smoke, channel validation, and documentation checks.
- The public beta update charters terminated as `blocked-decision` after the operator-directed release and tag deletion.

## Scenario Matrix

| Area | Scenario | Verdict | Evidence summary |
|---|---|---|---|
| APP | `APP-abandoned-install-update-polling` | Pass | Durable-operation polling recovers after the initiating client leaves. |
| APP | `APP-agent-cli-app-verbs` | Pass | Structured app verbs preserve their deterministic success and error contracts. |
| APP | `APP-app-auto-update` | Blocked — decision | Requires a public N→N+1 app channel; the required releases were explicitly deleted. |
| APP | `APP-attach-running-daemon` | Pass | The shell attaches without taking ownership of an operator-started daemon. |
| APP | `APP-brand-channel-visibility` | Pass | Packaged smokes report the release identity and channel truth. |
| APP | `APP-cancel-dormant-update` | Pass | CLI, HTTP, and UDS cancellation agree when no operation is active. |
| APP | `APP-deep-link-cold-start` | Pass | Cold-start navigation remains inside the product boundary. |
| APP | `APP-deep-link-running` | Pass | Running-app deep links focus and route one application instance. |
| APP | `APP-desktop-page-zoom` | Pass | Desktop zoom commands preserve the supported range and current route. |
| APP | `APP-install-first-run-provision` | Pass | Hosted Linux and both macOS packaged smokes provision from empty isolated homes. |
| APP | `APP-quit-contract` | Pass | Quit stops owned runtime state without stopping an attached operator runtime. |
| APP | `APP-runtime-update-app-owned` | Blocked — decision | The public app-owned update handoff needs the deleted beta channel state. |
| APP | `APP-runtime-update-managed` | Pass | Runtime update ownership, status, apply, cancellation, and restart truth agree across surfaces. |
| APP | `APP-single-command-multi-target-update` | Blocked — decision | The app leg needs a public successor release; runtime-only behavior passed. |
| APP | `APP-single-instance-focus` | Pass | Repeated launches focus the existing window and do not duplicate runtime ownership. |
| APP | `APP-start-installed-daemon` | Pass | The installed shell provisions and starts its bundled daemon from an empty home. |
| APP | `APP-update-config-cadence` | Pass | Update cadence and channel settings persist and are reflected by the daemon. |
| APP | `APP-update-recovery-state` | Pass | Interrupted and recoverable operation states remain visible and actionable. |
| APP | `APP-web-update-indicator` | Pass | Browser-driven menubar state reflects the daemon update authority. |
| APP | `APP-web-update-two-track` | Pass | Settings separates app and runtime tracks without inventing availability. |
| APP | `APP-window-geometry-recovery` | Pass | Invalid or off-screen geometry recovers to a usable window. |
| REL | `REL-beta-channel-contract` | Blocked — decision | Publication and final channel validation were cancelled; the beta releases were deleted. |
| REL | `REL-beta-install-paths` | Blocked — decision | Public installer links no longer exist after the requested cleanup. |
| REL | `REL-beta-installer-provenance` | Blocked — decision | Packaged smoke passed, but durable public-release provenance cannot be linked after deletion. |
| REL | `REL-beta-self-update` | Blocked — decision | A real installed N→N+1 update cannot run without two public beta releases. |
| REL | `REL-channel-repair-known-good` | Blocked — decision | No public beta channel remains on which to exercise repair. |
| REL | `REL-electron-cutover-announcement` | Pass | Public copy and release framing describe Electron and remove Tauri claims. |
| REL | `REL-migration-guide-parity` | Pass | Migration guidance matches current commands, storage, ownership, and update behavior. |
| REL | `REL-os-landing-proof` | Pass | OS-aware installation guidance maps to the produced artifact families. |
| REL | `REL-published-npm-channel-readiness` | Pass | Package metadata and the non-publishing preparation path are coherent. |
| REL | `REL-release-archive-update-contract` | Pass | Immutable artifacts and updater archive contracts passed the hosted build jobs. |
| REL | `REL-release-candidate-plan` | Pass | Explicit release planning and exact web-asset pinning passed. |
| REL | `REL-release-note-signal` | Pass | Release notes describe user-visible behavior and operational limits. |
| REL | `REL-stable-changelog-hard-cut` | Pass | Stable changelog and migration language contain no compatibility bridge. |
| Settings | `MS-settings-update-mutations` | Pass | Apply and cancel mutations remain consistent across browser, HTTP, UDS, and persistence. |

Status legend: `Pass | Blocked — decision`. There are no `untested`, `fail`, or `blocked-verify` verdicts in this scope.

## Agent-Facing Surface Walk

The update operation was exercised through the structured CLI and both daemon transports. Reads, apply requests, cancellation, dormant cancellation, blocked state, staged state, and persisted operation identity agreed across CLI, HTTP, and UDS. The browser Settings surface and menubar indicator read the same daemon-owned state. No UI-only update authority was introduced.

The preserved `compozy app` verbs were also exercised for status, start/attach, navigation, diagnostics, update check, and deterministic not-running behavior. Browser parity was recorded through the isolated web proxy rather than a hard-coded daemon address.

## Hosted Package Evidence

The retained [GitHub Actions run](https://github.com/compozy/compozy/actions/runs/32017263255) records these successful jobs before cancellation:

- Explicit Release Plan, including synchronization and pinning of `compozy-web-assets v0.0.139`.
- Desktop Linux x86_64 build.
- Desktop macOS x64 and macOS arm64 signed builds.
- Packaged Linux AppImage smoke from an empty isolated home.
- Packaged macOS x64 and arm64 DMG smokes from empty isolated homes.
- macOS provenance, updater archive, and signed bundled-runtime integrity checks.

The Stage Production Release job reached draft staging, but publication jobs were cancelled. The resulting beta.17 through beta.20 GitHub releases and corresponding root and Go SDK tags were deleted at the operator's direction. No release or tag was recreated.

## Defects and Fix Loops

Seven completion-blocking defects were registered during release-grade QA:

| Bug | Result | Proof |
|---|---|---|
| `BUG-20260817-desktop-release-channel-provenance` | Verified | Hosted beta.20 packaged smokes reported release identity correctly. |
| `BUG-20260817-desktop-smoke-local-isolation` | Verified | Isolated local packaged smoke passed after socket, port, and runtime path separation. |
| `BUG-20260817-explicit-release-web-assets-sync` | Verified | Explicit Release Plan synchronized and pinned the exact web asset version. |
| `BUG-20260817-signed-macos-x64-digest-drift` | Verified | Signed macOS x64 build and packaged smoke passed digest verification. |
| `BUG-20260817-macos-finalized-dmg-integrity-stale` | Fixed; public rewalk blocked | Local finalized-DMG integrity passed; final public channel publication was cancelled. |
| `BUG-20260817-macos-channel-manifest-includes-dmg` | Fixed; public rewalk blocked | DMGs are excluded from updater entries; no public channel remains to rewalk. |
| `BUG-20260817-linux-channel-manifest-includes-deb` | Fixed; public rewalk blocked | DEBs are excluded from updater entries; no public channel remains to rewalk. |

No defect was dismissed by weakening a scenario or test. The two channel manifest fixes and finalized DMG fix remain honestly pending on a future authorized public-channel rewalk.

## Browser and Visual Evidence

- Browser parity recording: `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/electron-shell-task07` (33 frames plus event metadata).
- Visual-contract bundle: `.compozy/tasks/electron-shell/evidence/visual/task_02/VC-01` through `VC-15`; every review records zero blocking structural divergences.
- The PR embeds representative, durable screenshots uploaded to the existing Compozy R2 evidence bucket. This upload stores QA evidence only; it does not deploy the product.

## Automated Evidence

- Runtime E2E lane: passed in the isolated QA cycle.
- Web E2E lane: 160 passed, 3 skipped by documented platform capability.
- Desktop E2E lane: passed for the Electron shell contract.
- Focused Go, TypeScript, Electron, release-script, artifact-contract, and packaged-smoke checks passed through the task and fix loops.
- Close gate: the local `make gate-full` was interrupted at the operator's direction; the complete gate is deferred to the PR checks.

## Paper Cuts and Runtime Observations

- Apple notarization made one hosted macOS leg wait for several hours; a targeted rerun completed successfully.
- A local packaged smoke initially collided with long Unix socket paths and existing runtime ports. The smoke now owns isolated paths and passed.
- No unexpected daemon, web, desktop, browser, or watcher process survived teardown.

## Teardown Evidence

All three active QA envelopes ended with `clean: true` and no survivors:

- Main lab: `/Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260816-103738-647344-lab/qa-artifacts/qa/teardown.json`.
- Final macOS update lab: `/Users/pedronauck/dev/qa-labs/compozy-electron-beta-update-macos-final-20260817-111452-787802-lab/qa-artifacts/qa/teardown.json`.
- Final Linux update lab: `/Users/pedronauck/dev/qa-labs/compozy-electron-beta-update-linux-final-20260817-111501-157775-lab/qa-artifacts/qa/teardown.json`.

## Decision Record

The operator decided to remove all beta tags and releases created during QA and not recreate them. That choice supersedes the planned public release gate for this closeout. A later release owner may authorize a fresh beta N and N+1 pair; until then, the eight public-channel scenarios remain `blocked-decision` and the PR must not claim release readiness.

## Compozy Impact Audit

- **Native tools:** Update-operation behavior is exposed through the existing agent-manageable CLI, HTTP, and UDS surfaces; no new `compozy__*` native tool ID or toolset was added. Checked tool descriptors, schemas, digests, risk flags, capability gates, availability diagnostics, and CLI/API fallbacks for update and app operations.
- **Extensibility and hooks:** The Electron shell replaces Tauri at the host boundary while preserving the daemon as authority. Checked extensions, hooks, capabilities, tools/resources, registries, bridge SDKs, MCP sidecars, desktop release authority, updater resources, and the update-related `config.toml` lifecycle. Obsolete Tauri wiring and settings were removed rather than bridged.
- **Workspace data isolation:** App installation and update metadata are global to the resolved Compozy home; update operations are daemon-global, while session and agent data remain workspace-scoped. CLI, HTTP, UDS, core/store, web cache, SSE, and event paths were checked so the shell does not add a cross-workspace data path or omit existing `workspace_id` boundaries.
- **Official Compozy skill:** `skills/compozy/` was updated for the Electron desktop, app verbs, update operation, release/install paths, and agent-manageable behavior. Checked the desktop, runtime-operation, native-tool, installation, and release guidance against the shipped commands.

## Final Status

- **Scenario verdicts:** 27 pass · 8 blocked by explicit operator decision · 0 untested · 0 fail · 0 blocked-verify.
- **Issues by user impact:** Blocks-Completion 7 found · 4 verified · 3 fixed with public-channel rewalk blocked · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0.
- **Automated close gate:** deferred to PR CI by explicit operator direction; the interrupted local run is not claimed as passing evidence.
- **Release proof:** signed builds and packaged first-run smokes passed; public N→N+1 auto-update is not proven because the required releases were deleted.
- **Verdict:** ready for code review with blocked release items; not release-ready until an explicitly authorized public N→N+1 cycle passes.

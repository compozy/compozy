# QA Run Report — 2026-08-10 — desktop-app-release

- **Scope:** CompozyOS desktop release candidate from Tasks 01–06: install/provisioning, runtime attach/start/quit, native links and window behavior, app/runtime updates, agent CLI, release integrity, and the three-webview SSE gate.
- **Cadence tier:** full
- **Build:** `648d9fd9973561edc15cefe7f40da231ac866166` plus QA fix commits through `f081a1e` · **Environment:** isolated targeted lab `desktop-app-release-20260810-110811-513872`; macOS 26.5.1 arm64 hardware.
- **Started:** 2026-08-10T11:08:28Z · **Status:** terminal, no-ship

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | first-run per platform |
| Dora | Power User | desktop / wifi-fast / en-US | attach/quit and release rehearsal |
| Théo | Power User | desktop / wifi-fast / en-US | links and SSE gate |
| Bruno | Power User | desktop / wifi-fast / en-US | update rehearsal per platform |
| Ada | Autonomous Agent | desktop / wifi-fast / en-US | structured CLI lifecycle |

## Flows in Scope

- `J-desktop-first-run` — install and reach a working product without a terminal (`../journeys/J-desktop-first-run.md`)
- `J-desktop-attach-daily` — attach/start/quit without owning daemon lifetime (`../journeys/J-desktop-attach-daily.md`)
- `J-desktop-link-driven` — return through native links into one focused window (`../journeys/J-desktop-link-driven.md`)
- `J-desktop-update-moment` — apply app/runtime updates with consent and recover honestly (`../journeys/J-desktop-update-moment.md`)
- `J-desktop-agent-headless` — operate the app through structured CLI verbs (`../journeys/J-desktop-agent-headless.md`)
- `J-publish-compozy-beta` — keep the release draft until desktop feed verification (`../journeys/J-publish-compozy-beta.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-desktop-first-run-macos | J-desktop-first-run / install, brand | Lea | Feature Tour | Blocked (needs human verify) | Unsigned build and `cgWindowNotFound`; runtime/product legs passed after three fixes | `01a45c49`, `b415f24b`–`02b55a46`, `f081a1e` |
| 2 | CH-desktop-attach-quit-macos | J-desktop-attach-daily / attach, start, quit | Dora | Interrupt Tour | Blocked (needs human verify) | Runtime reached product and survived quit; visible progress/window proof unavailable | `b415f24b`–`02b55a46`, `f081a1e` |
| 3 | CH-desktop-links-instance-macos | J-desktop-link-driven / links, instance focus | Théo | Garbage Tour | Blocked (needs human verify) | Running-app navigation returned the target path; cold-start focus and visual routing were not capturable | `0805f649` |
| 4 | CH-desktop-update-rehearsal-macos | J-desktop-update-moment / app/runtime update and recovery | Bruno | Interrupt Tour | Blocked (needs human verify) | No signed staging feed or `minisign`; development check returned no offer | |
| 5 | CH-desktop-sse-gate-macos | J-desktop-attach-daily / E2E-021 WKWebView | Théo | Network Tour | Blocked (needs human verify) | The native WKWebView window could not be captured; the required three-window 10-minute profile was not measured | |
| 6 | CH-desktop-first-run-windows | J-desktop-first-run / install, brand | Lea | Feature Tour | Blocked (needs human verify) | Windows/WebView2 runner unavailable | |
| 7 | CH-desktop-attach-quit-windows | J-desktop-attach-daily / attach, start, quit | Dora | Interrupt Tour | Blocked (needs human verify) | Windows/WebView2 runner unavailable | |
| 8 | CH-desktop-links-instance-windows | J-desktop-link-driven / links, instance focus | Théo | Garbage Tour | Blocked (needs human verify) | Windows/WebView2 runner unavailable | |
| 9 | CH-desktop-update-rehearsal-windows | J-desktop-update-moment / app/runtime update and recovery | Bruno | Interrupt Tour | Blocked (needs human verify) | Windows/WebView2 runner and signed staging feed unavailable | |
| 10 | CH-desktop-sse-gate-windows | J-desktop-attach-daily / E2E-021 WebView2 | Théo | Network Tour | Blocked (needs human verify) | Windows/WebView2 runner unavailable | |
| 11 | CH-desktop-first-run-linux | J-desktop-first-run / install, brand | Lea | Feature Tour | Blocked (needs human verify) | Linux/WebKitGTK runner unavailable | |
| 12 | CH-desktop-attach-quit-linux | J-desktop-attach-daily / attach, start, quit | Dora | Interrupt Tour | Blocked (needs human verify) | Linux/WebKitGTK runner unavailable | |
| 13 | CH-desktop-links-instance-linux | J-desktop-link-driven / links, instance focus | Théo | Garbage Tour | Blocked (needs human verify) | Linux/WebKitGTK runner unavailable | |
| 14 | CH-desktop-update-rehearsal-linux | J-desktop-update-moment / app/runtime update and recovery | Bruno | Interrupt Tour | Blocked (needs human verify) | Linux/WebKitGTK runner and signed staging feed unavailable | |
| 15 | CH-desktop-sse-gate-linux | J-desktop-attach-daily / E2E-021 WebKitGTK | Théo | Network Tour | Blocked (needs human verify) | Linux/WebKitGTK runner unavailable | |
| 16 | CH-desktop-agent-headless-cli | J-desktop-agent-headless / APP-agent-cli-app-verbs | Ada | Feature Tour | Fixed | Newline framing timed out; healthy retry later corrupted state | `0805f649`, `f081a1e` |
| 17 | CH-desktop-release-rehearsal | J-publish-compozy-beta / E2E-019 | Dora | Garbage Tour | Blocked (human decision) | The production workflow would publish npm/Homebrew and no safe draft/staging release target was authorized | |

Status legend: `Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **macOS first run:** The unsigned bundle initially crashed in updater setup. After the base updater configuration was added, the app remained running. The runtime path then exposed four separate daemon-contract mismatches; after their fixes, status reached `product` with an attached app-owned runtime. The run did not claim trusted-installer or visual completion.
- **macOS attach and quit:** A real detached daemon ran under the isolated home on port `63494`. After the desktop PID exited, the daemon stayed parented to PID 1 and `GET /api/status` returned HTTP 200.
- **macOS links and instance:** The live control socket accepted `/sessions/qa-desktop-link` and returned the same path. Native cold-start routing, focus, and single-instance visibility remain unverified because no CGWindow was available to the automation.
- **macOS updates:** The development updater initialized and `update --check` completed, but returned no offer. No signed apply, mid-install failure, rollback, permission check, or pending-on-quit claim was made.
- **macOS SSE:** The daemon web surface loaded its real onboarding and model catalog with no browser console errors. This is not a substitute for the required three-WKWebView connection profile.
- **Windows and Linux:** No compatible runners existed in the environment. Each platform charter is terminal as `blocked-verify`, never inferred from macOS.
- **Agent CLI:** Status, diagnose, update check, and navigation succeeded against the installed app. Retry in a healthy state now returns `retry_unavailable` and preserves `product`.
- **Release rehearsal:** Static release gates were implemented in Task 05. The publish-capable workflow was not invoked because this run had no authorized draft or staging destination.

## What Was Fixed

| Bug | User impact | Status | Fix |
|---|---|---|---|
| `BUG-20260810-desktop-dev-shell-crashes` | App exited before setup | verified | `01a45c49` |
| `BUG-20260810-desktop-runtime-stalls` | App never reached product | verified | `b415f24b`, `b3aa3d27`, `bd610cfa`, `02b55a46` |
| `BUG-20260810-app-control-timeout` | Agent control verbs timed out | verified | `0805f649` |
| `BUG-20260810-healthy-retry-corrupts-state` | Healthy retry replaced product with an error | verified | `f081a1e` |
| `BUG-20260810-initial-boot-window-absent` | Startup had no boot-window owner | fixed; visual verify blocked | `f081a1e` |

The canonical Rust suite passed after the fixes: 90 unit tests, 5 contract tests, 1 real-process fake-daemon test, 3 runtime-resolution tests, and 8 update-system tests.

## Paper Cuts

None recorded. Every observed issue affected completion or trust and was registered as a bug.

## Runtime Errors Observed

- Development updater configuration was absent and terminated the shell during plugin setup.
- The supervisor called the removed `daemon run` contract.
- A package launcher PID was treated as if it had to equal the final daemon PID.
- `daemon.user_home_dir` was misread as `COMPOZY_HOME` instead of operator-home metadata.
- `0.3.0-beta.8` was rejected by the pre-release version floor.
- The control server waited for EOF instead of one newline-framed request.
- Retry was accepted in `product` and attempted to create a second `main` webview.
- Computer Use repeatedly returned `cgWindowNotFound` for the running installed app; this is recorded as a verification blocker, not proof that an end user sees no window.

## Human Verifications Needed

- macOS: install a signed/notarized artifact and visually walk first run, transitional states, product reveal, cold/running links, single-instance focus, geometry restore, and brand/channel display.
- macOS: run E2E-021 with three real WKWebViews for 10 minutes and retain measured connection profiles.
- Windows: run all five WebView2 charters, including the updater exit/restart behavior.
- Linux: run all five WebKitGTK charters, including `libfuse2`/AppImage paths.
- Signed staging feed: walk app-owned and managed runtime updates, forced mid-install failure, post-update permissions, pending-on-quit, rollback/roll-forward, and recovery state.

## Decisions for a Human

- Provide authorized Windows, Linux, and signed staging-release runners before reconsidering ship.
- Keep the production release job uninvoked until E2E-019 can use a non-publishing rehearsal target or an explicitly approved draft destination.

## Learnings

- A schema-valid payload is insufficient when process lineage, command verbs, and field meaning have drifted; real-process QA found all four layers.
- Local stream protocols need an explicit frame boundary. Waiting for EOF deadlocks clients that correctly wait for a response on the same connection.
- Recovery controls must be state-gated. A retry callback that exists is not evidence that the current state is retryable.
- Platform webview verdicts are not portable. Browser success on the daemon URL does not satisfy WKWebView, WebView2, or WebKitGTK release evidence.

## Compozy Impact Audit

- **Native tools:** No impact. Checked `compozy__*` tool IDs, toolsets, descriptors, schema digests, risk flags, capability gates, and CLI fallbacks; this fix loop changed the existing `compozy app` control path only.
- **Extensibility and hooks:** No impact. Checked extensions, hooks, skills/capabilities, registries, bridge SDKs, MCP sidecars, and `config.toml`; no extension contract, hook event, resource, registry, or config key changed.
- **Workspace data isolation:** No impact. Desktop `app.json`, sockets, provenance, and update state remain global to the selected `COMPOZY_HOME`; no workspace/session/agent datum or `workspace_id` path changed across CLI, HTTP, UDS, store, web, SSE, cache, or events.
- **Official Compozy skill:** No additional change. Task 04 already documented the shipped `compozy app` surface; the QA fixes restore that documented behavior without changing commands or semantics.

## Final Status

- **Exit gate (full automated suite):** pending final post-review `make gate-full`; Task 07's complete Rust suite is green.
- **Issues by user impact:** 4 verified fixes; 1 fixed visual issue awaiting human verification; no open automated defect.
- **Coverage:** 17/17 charters terminal; 1 fixed, 15 blocked-verify, 1 blocked-decision.
- **Scenario tracker:** 13/13 APP scenarios terminal; 2 pass, 11 blocked-verify, 0 untested/fail.
- **QA teardown:** `/Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/teardown.json` records `"clean": true` with zero survivors. The QA app was moved from `/Applications` to the operator Trash and remains recoverable.
- **Verdict:** **BLOCKED — NO SHIP.** E2E-021 lacks all three required webview profiles, the signed update rehearsal is incomplete, and Windows/Linux platform charters were not executed.

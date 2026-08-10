# QA Run Report — 2026-08-10 — desktop-coderabbit-remediation

- **Scope:** Targeted re-walk of user-visible desktop behavior changed while resolving the CodeRabbit review: structured CLI state validation, app registration truth, health recovery, deep-link boundaries, and concurrent boot actions.
- **Cadence tier:** targeted
- **Build:** working tree on the current branch · **Environment:** current macOS development app with a fresh isolated `COMPOZY_HOME`; signed/installable desktop artifacts were not produced by this branch
- **Started:** 2026-08-10T12:38:24-03:00 · **Finished:** 2026-08-10T12:53:17-03:00 · **Status:** closed with release-grade verification blocked

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-desktop-agent-headless-cli |
| Lea | New User | laptop / wifi-fast / en-US | CH-desktop-first-run-macos |
| Théo | Power User | desktop / wifi-fast / en-US | CH-desktop-links-instance-macos |
| Dora | Power User | desktop / wifi-fast / en-US | CH-desktop-attach-quit-macos |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-desktop-update-rehearsal-macos |

## Flows in Scope

- `J-desktop-agent-headless` — operate the desktop app through structured CLI (`../journeys/J-desktop-agent-headless.md`)
- `J-desktop-first-run` — install CompozyOS and reach a working product (`../journeys/J-desktop-first-run.md`)
- `J-desktop-link-driven` — follow a CompozyOS link into one app window (`../journeys/J-desktop-link-driven.md`)
- `J-desktop-attach-daily` — attach to an existing runtime without disturbing it (`../journeys/J-desktop-attach-daily.md`)
- `J-desktop-update-moment` — recover from app and runtime updates without being stranded (`../journeys/J-desktop-update-moment.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-desktop-agent-headless-cli | J-desktop-agent-headless / APP-agent-cli-app-verbs | Ada | Feature Tour | Blocked (needs human verify) | Signed install/update and cross-OS legs unavailable | working tree |
| 2 | CH-desktop-first-run-macos | J-desktop-first-run / APP-install-first-run-provision | Lea | Feature Tour | Blocked (needs human verify) | BUG-20260810-boot-controls-unavailable fixed; signed clean-machine legs unavailable | working tree |
| 3 | CH-desktop-links-instance-macos | J-desktop-link-driven / APP-deep-link-running | Théo | Garbage Tour | Blocked (needs human verify) | Windows/Linux scheme activation and configured deleted-session leg unavailable | working tree |
| 4 | CH-desktop-links-instance-macos | J-desktop-link-driven / APP-deep-link-cold-start | Théo | Garbage Tour | Blocked (needs human verify) | Real provision/start wait and Windows/Linux legs unavailable | working tree |
| 5 | CH-desktop-attach-quit-macos | J-desktop-attach-daily / APP-attach-running-daemon | Dora | Interrupt Tour | Blocked (needs human verify) | Provider-backed session, browser parity, and Windows/Linux legs unavailable | working tree |
| 6 | CH-desktop-update-rehearsal-macos | J-desktop-update-moment / APP-update-recovery-state | Bruno | Interrupt Tour | Blocked (needs human verify) | Signed N/N+1 recovery rehearsal unavailable | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Ada:** Confirmed schema-valid status before launch and while attached, `/settings` navigation, diagnostics, update check, and deterministic `app_not_running`. Signed apply/recovery and clean-OS installation remain blocked.
- **Lea:** Reproduced the initial boot-control failure, fixed it, then confirmed diagnostics rendered both isolated log paths and retry reached the designed starting/error states without stale busy UI.
- **Théo:** Confirmed running and cold-start delivery to `/settings`; an encoded-backslash payload stayed inside the product boundary. Cross-OS activation and a real deleted entity remain blocked.
- **Dora:** Attached the app to the CLI-owned isolated daemon and confirmed app quit did not stop it. Provider-backed browser/app parity remains blocked.
- **Bruno:** Confirmed update-check and structured control readouts. A signed forced-failure and recovery rehearsal needs release-grade N/N+1 artifacts.

## What Was Fixed

- `BUG-20260810-boot-controls-unavailable`: the initially configured Tauri boot webview did not receive the minimal invoke bridge. The bridge is now installed after every boot-page load, including the configured initial window; the same lab replay passed after the fix.
- Of the 35 CodeRabbit findings, 30 produced focused production or canonical-suite hardening. Five were disproven against current code or dependency behavior: the separate app-state schemas, Satori eyebrow styling, relative `COMPOZY_HOME`, fake-daemon `ECHILD`, and Tauri's prehashed updater verification mode.

## Paper Cuts

None observed.

## Runtime Errors Observed

The stopped-runtime fixture intentionally produced `runtime_start_failed`. No unexpected runtime error remained after the boot-control fix.

## Human Verifications Needed

The terminal `blocked-verify` rows require signed per-OS installers and runtime feeds, a clean-machine provision run, Windows/Linux scheme activation, a provider-backed session for browser/app parity, and signed N/N+1 update-recovery fixtures. Exact observables are recorded in each scenario and in the lab's `platform-capability-blockers.txt`.

## Decisions for a Human

None.

## Learnings

- A Tauri window declared in configuration does not share the JavaScript initialization path of a programmatically recreated window; both paths need one idempotent bridge installer.
- The current Rust webview mock cannot execute configured-page JavaScript, so the initial-window bridge remains tracked for desktop E2E automation.

## Automated Evidence

- `make desktop-test` — PASS, 96 Rust unit/contract tests.
- `make desktop-lint` — PASS, formatting and strict Rust lint.
- `CGO_ENABLED=1 go test -race ./internal/desktoprelease ./internal/config ./internal/cli ./internal/daemon ./internal/api/core` — PASS.
- Changed release scripts parse under `bash -n`; `git diff --check` is clean.
- Final full-gate evidence: `/Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/final-make-verify.log` (written only after the final tree passes).
- The QA manifest teardown completed with `clean: true`, no survivors, in `/Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/teardown.json`.

## Compozy Impact Audit

- **Native tools:** No impact. Checked the diff for `compozy__*` IDs, toolsets, descriptors, schemas, digests, risk flags, diagnostics, and capability gates; none changed. Existing CLI/API drain and desktop commands only received correctness fixes.
- **Extensibility and hooks:** No impact. Checked extensions, hooks, capabilities, tools/resources, registries, bridge SDKs, MCP sidecars, and `config.toml`; no public extension surface or config key/default changed.
- **Workspace data isolation:** Desktop app state and locks remain global to the resolved `COMPOZY_HOME`; drain remains daemon-global admission state. No `workspace_id`, HTTP/UDS routing, SSE, cache, event, session, or agent ownership path changed.
- **Official Compozy skill:** No impact. Checked `skills/compozy/references/desktop.md`, `runtime-operations.md`, and `native-tools.md`; the documented commands and behavior remain accurate.

## Final Status

- **Exit gate (full automated suite):** Completion is valid only with a current passing `make gate-full` record reported by `make gate-status` for this final tree.
- **Issues by user impact:** Blocks-Completion 1 found, fixed, and verified · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 5 of 5 journeys walked on the available macOS development surface; all 6 scenario rows are terminal `blocked-verify` for release-grade or cross-OS legs.
- **Verdict:** ready with blocked items — no open defect remains in the walkable macOS scope; signed/cross-OS evidence remains explicitly blocked.

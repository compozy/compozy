# QA Run Report — 2026-09-28 — session-continue-fork, part B2 (web walks + visual bundles)

- **Scope:** branch `continue-fork` (head `79c7d4b57` + uncommitted task_08 repairs): Web Continue/Fork (task_04/task_06). Web legs of `ET-web-session-continue`, `ET-web-session-fork-from-here`, `ET-web-sessions-catalog-modal`, `ET-web-session-sidebar-threads`, `RT-provider-error-handoff`. Visual Contract bundles VC-01…VC-23 (E2E-005). Plan: `docs/qa/reports/2026-09-28-session-continue-fork-plan.md`.
- **Cadence tier:** targeted
- **Build:** lab daemon built from the working tree (includes B1's concurrent derive fixes) serving `web/dist` rebuilt after each B2 web fix · **Environment:** isolated lab `compozy-session-continue-fork-web-b2-20260929-010831-000020-lab` (HTTP 64166, acpmock only, headless Chromium 1440×900 via Playwright)
- **Started:** 2026-09-29T01:08Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | desktop builder | desktop / wifi-fast / en-US | CH-session-derive-web |

## Flows in Scope

- `J-14` — continue and fork a session from every web entry point without touching the source (`../journeys/J-14-read-a-finished-transcript.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-derive-web | J-14 / ET-web-session-continue | Bruno | Feature Tour | Fixed | BUG-20260928-agent-resource-drops-fallback-chain; BUG-20260928-derive-budget-ignores-workspace-overlay; BUG-20260928-derive-retry-creates-second-child; BUG-20260928-derived-child-empty-state-hidden | uncommitted |
| 2 | CH-session-derive-web | J-14 / ET-web-session-fork-from-here | Bruno | Feature Tour | Fixed | BUG-20260928-message-actions-enabled-during-remote-turn | uncommitted |
| 3 | CH-session-derive-web | J-14 / ET-web-sessions-catalog-modal | Bruno | Feature Tour | Fixed | row menu width (VC-01/02) | uncommitted |
| 4 | CH-session-derive-web | J-14 / ET-web-session-sidebar-threads | Bruno | Feature Tour | Pass | | |
| 5 | CH-session-derive-web | J-14 / RT-provider-error-handoff (web leg) | Bruno | Feature Tour | Pass | BUG-20260928-provider-error-notice-live-duplicate (open, cosmetic, pre-existing) | |
| 6 | E2E-005 | VC-01…VC-23 bundles | — | — | Pass (23/23 validator exit 0) | menus width, VC-05, VC-07, VC-08, VC-20 repaired | uncommitted |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-derive-web — Bruno

- **Ran:** 01:10Z → 02:00Z (box respected: yes)
- **Findings:**
  - Continue with a declared route was impossible (Blocks-Completion): the resource catalog dropped `fallback_chain`. Fixed backend-side, then re-walked.
  - The workspace `[session.derive]` overlay was ignored, so VC-08 truncation couldn't be reached (Trust-Damage). Fixed; preview and commit share the per-workspace budget.
  - A 422 after the commit (first message refused at bind) rotated the idempotency key, so the retry created a duplicate child (Data-Loss class). Fixed in web.
  - The derived child's empty state never rendered because status-only hook messages counted as messages. Fixed.
  - Fork from here / Rewind to here stayed enabled while a turn started elsewhere was running. Fixed: the gate now reads the daemon's running state.
  - Row and overflow menus rendered at 128px, so every item wrapped onto 2–3 lines (board: one line). Fixed with `min-w-56` on both `DropdownMenuContent`s.
- **Bugs filed/updated:** BUG-20260928-agent-resource-drops-fallback-chain, BUG-20260928-derive-budget-ignores-workspace-overlay, BUG-20260928-derive-retry-creates-second-child, BUG-20260928-derived-child-empty-state-hidden, BUG-20260928-message-actions-enabled-during-remote-turn, BUG-20260928-provider-error-notice-live-duplicate (open). Re-found (B1-owned, fixed during this run): BUG-20260928-derive-stopped-source-turn-in-progress (the preview on stopped sources read "A turn is still in progress"; it's gone after B1's fix in the rebuilt daemon).
- **Scenarios settled:** ET-web-session-continue → pass (after fixes), ET-web-session-fork-from-here → pass (after fix), ET-web-sessions-catalog-modal → pass (after fix), ET-web-session-sidebar-threads → pass, RT-provider-error-handoff web leg → pass (CLI steps owned by B1).
- **Paper cuts:** see table.
- **Surprises:** a daemon restart stops every active session, so all later derives in the lab ran on stopped sources. That is what exposed the B1 stopped-source bug.
- **Suggested next charter:** spawned-row absence of Continue/Fork on a live spawned child (unit-covered by `sessions-modal.test.tsx`, not walked); `RT-conversation-rewind` web leg after the shared-gate change.

## What Was Fixed

### BUG-20260928-agent-resource-drops-fallback-chain: Continue with a declared route fails
- **Symptom:** "agent route-agent declares 0 route(s); route 2 does not exist".
- **Root cause:** `validateAgentResourceSpec` omitted `FallbackChain`.
- **Fix:** `internal/config/agent_resource.go` (uncommitted).
- **Regression test:** `internal/config/agent_resource_test.go` `TestAgentResourceCodecCanonicalizesTypedRecordSpec`.
- **Retested:** Continue with Route 2 → child `sess-b946f53737c279ea` answered on the route fixture; CLI `--route 1` created a child.

### BUG-20260928-derive-budget-ignores-workspace-overlay
- **Symptom:** 10.8 KiB carried despite `max_replay_bytes = 4096` in the workspace overlay.
- **Root cause:** `deriveBudget()` read the global config only.
- **Fix:** `internal/session/derive_prepare.go`, `derive_preview.go` (uncommitted).
- **Regression test:** `internal/session/manager_derive_test.go` `TestDerivePreview/Should_bound_the_preview_and_the_continue_by_the_source_workspace_overlay`.
- **Retested:** VC-08 "Carries over 4 of 12 messages · 3.7 KiB".

### BUG-20260928-derive-retry-creates-second-child
- **Symptom:** a second click on Continue after a post-commit 422 created a second child.
- **Root cause:** idempotency key rotated on every 4xx.
- **Fix:** `web/src/systems/session/hooks/use-session-derive.ts`, `use-session-continue-dialog.ts`, `use-session-fork-dialog.ts` (uncommitted).
- **Regression test:** `session-continue-dialog.test.tsx` "Should retry with the same idempotency key after a failure".
- **Retested:** two submits → one child (`sess-3540e32ceffb49f3`).

### BUG-20260928-derived-child-empty-state-hidden
- **Symptom:** blank child thread under the divider.
- **Root cause:** status-only `hook.dispatch.*` messages counted as messages.
- **Fix:** `web/src/components/assistant-ui/session-thread-messages.tsx` (uncommitted).
- **Regression test:** `session-thread.test.tsx` "Should keep the empty state of a derived child whose transcript holds only status events" (failed before the fix).
- **Retested:** VC-20, continue and fork children.

### BUG-20260928-message-actions-enabled-during-remote-turn
- **Symptom:** Fork from here / Rewind enabled while a CLI/API-started turn ran.
- **Root cause:** the gate read only local `thread.isRunning`.
- **Fix:** `use-session-message-action-gate.ts`, `use-session-runtime-extensions.ts`, `session-chat-runtime-provider.tsx`, `lib/session-runtime-render-context*.ts(x)` (uncommitted).
- **Regression test:** `session-thread.test.tsx` "Should disable Fork from here together with Rewind while the daemon reports a running turn". Test harness idle preconditions made explicit in `session-thread.test.tsx` and `session-chat-runtime-provider.test.tsx`.
- **Retested:** VC-05.

### Menu width divergence (VC-01/02/03, no bug id: visual contract)
- **Symptom:** 128px menus, wrapped items.
- **Fix:** `className="min-w-56"` on `SessionRowActions` and the window overflow `DropdownMenuContent` (`session-row-actions.tsx`, `use-session-topbar-slot.tsx`); composes the existing primitive per repo convention.
- **Retested:** VC-01/02/03 recaptured.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Bruno | J-14 Continue with first message, bind refused after commit | "It says it failed, but the session exists and I can't reach it from here; the error is raw internal text (`session: admit first message of derived session …`)" | sharp | deferred: Decisions for a Human |
| Bruno | J-14 Continue/Fork dialog in the default 604px window | "Open in and the pending line are below the fold" | dull | watching (host constraint, authorized) |
| Bruno | Stop on an unbound child | "Stopping a fresh child says 'Session failed to start' / 'Failed after … cancellation'" | dull | pre-existing (same for any unbound session), reported only |
| Bruno | Workspace overlay config | "Setting only `max_replay_bytes = 4096` made the workspace fail to resolve with a bare 500" | dull | noted in BUG-20260928-derive-budget-ignores-workspace-overlay |

## Runtime Errors Observed

- 404s for old chunk hashes in the console right after `make web-build` while an old page was loaded. Stale browser cache; cleared, not a product issue.
- Daemon restarts (to load fixes) left sessions reading "Stopped after … the agent didn't answer the stop, so it was closed for you". Expected for the lab restart; not filed.

## Human Verifications Needed

- None.

## Decisions for a Human

### Post-commit first-message failure leaves the child unreachable from the dialog
- What's broken: when the child is committed but its first message's admission fails (422), the dialog shows the daemon's raw internal error, and the new child isn't opened. After the B2 fix, a retry reuses the key, completes on the same child, and no longer duplicates it.
- Why not auto-fixed: changes the daemon response contract (return the child with a failed `first_prompt` state, or a structured code) or the dialog's landing rule. That's a design decision beyond a minimal fix.
- Options: 1. daemon returns 201 with `first_prompt: failed` + the child, and the web lands it with an error toast (spec change, ADR-007 wording). 2. daemon adds `child_session_id` to the 422 body, and the web offers "Open the new session". 3. keep as is (retry completes on the same child).
- Recommendation: 2. It's additive and keeps the idempotent retry.

### Generic empty state hidden for any new session
- What's broken: `hook.dispatch.*` status messages suppress `ThreadStatePane`'s empty state for every fresh session (outside this feature, whose branch is fixed).
- Options: 1. apply the same narrative check to the generic empty branch. 2. project status-only lifecycle messages out of the transcript.
- Recommendation: 1, in a follow-up owned by the sessions surface.

## Learnings

- Restarting a lab daemon stops every active session. Derive walks then run on stopped sources, which is useful coverage for stopped-source paths.
- `make web-build` while a page is open serves stale chunk references until the browser cache is cleared.
- Build lab binaries before exporting `HOME=$PROVIDER_HOME`, or `go build` downloads the module cache into the provider home.

## Visual Contract bundles (E2E-005)

Root: `.compozy/tasks/session-continue-fork/evidence/visual/<task_04|task_06>/<VC>/`. Each has `reference.png` (board viewport with the specimen at top), `reference-crop.png`, `implementation.png` (1440×900), `implementation-crop.png`, `side-by-side.png`, `diff.png`, `comparison.json` (verdict + authorized differences) and `review.md`. `validate-visual-contract.mjs` exits 0 for all 23.

| VC | Owner | Verdict | Note |
|---|---|---|---|
| VC-01 | task_04 | PASS | fixed menu width |
| VC-02 | task_04 | PASS | fixed menu width |
| VC-03 | task_04 | PASS | fixed menu width; running source |
| VC-04 | task_06 | PASS | |
| VC-05 | task_06 | PASS | fixed busy gate |
| VC-06 | task_04 | PASS | |
| VC-07 | task_04 | PASS | fixed backend route resolution |
| VC-08 | task_04 | PASS | fixed overlay budget |
| VC-09 | task_04 | PASS | network abort |
| VC-10 | task_04 | PASS | POST held; pending line below fold (host) |
| VC-11 | task_06 | PASS | |
| VC-12 | task_06 | PASS | "4 of 6 messages" |
| VC-13 | task_06 | PASS | native clone line on idle bound source |
| VC-14 | task_06 | PASS | fulfilled `turn_settled: false` |
| VC-15 | task_06 | PASS | live CLI prompt → fence conflict |
| VC-16 | task_04 | PASS | "Continued from alpha" |
| VC-17 | task_06 | PASS | |
| VC-18 | task_04 | PASS | edge-dragged to 640px |
| VC-19 | task_04 | PASS | |
| VC-20 | task_04 | PASS | fixed empty state; fork + continue variants |
| VC-21 | task_04 | PASS | recovery child created |
| VC-22 | task_04 | PASS | durable state; live duplicate filed |
| VC-23 | task_06 | PASS | Context-sidebar Origin/Seed |

Authorized differences cited in every row's `review.md`: dialog width 560 (`--width-modal-sm`), window-scoped dialog height, accent header well, outline Cancel + hint glyph, `RuntimeSelector` trigger, `AgentCommandSelect` anatomy, route label copy (ADR-008), neutral `Pill`, `Empty` compact well, `OwnerAvatar sm`, status-line host chrome (SD-007), Origin/Seed Context section (#635), production `MessageActions` / `ProviderErrorNotice` / `SessionResumeFailure` anatomy, lab fixture data.

## Final Status

- **Exit gate (full automated suite):** not run by B2 (`make gate` belongs to task_08 closure on the final head). Focused evidence: `go test ./internal/config -run TestAgentResourceCodec` ok; `go test ./internal/session -run 'Derive|Fork|Continue|Handoff'` ok; `bunx turbo run test --filter=compozy-web -- src/components/assistant-ui src/systems/session src/systems/os` → 220 files / 2232 tests passed; `bun run typecheck:raw` clean; oxlint/oxfmt clean on changed web files; focused Playwright `e2e/__tests__/session-derive.spec.ts` E2E-001/002/004 → 3 passed. Lab strict audit PASS.
- **Issues by user impact:** Blocks-Completion 1 (fixed) · Data-Loss 1 (fixed) · Trust-Damage 2 (fixed) · Friction 1 (fixed) · Cosmetic 1 (open, pre-existing)
- **Coverage:** 5/5 in-scope web scenarios walked; 23/23 bundles. Not walked: live spawned-row absence (unit-covered), `RT-conversation-rewind` web leg (not in B2 scope).
- **Verdict:** ready with blocked items. The web derive flows pass after repairs; the two Decisions for a Human and the cosmetic live duplicate stay open.

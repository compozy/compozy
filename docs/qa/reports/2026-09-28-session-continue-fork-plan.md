# QA Plan — 2026-09-28 — Session continue and fork (+ fallback-account prerequisite)

- **Scope:** branch `continue-fork` (merge base `67b86a9b9`, head `668676ea2`): session-continue-fork tasks 01–06 (lineage kind + migration `00122`, ACP fork/resume caps and routing, derive primitive + Continue, Web Continue, Fork backend + native clone, Web Fork, preview `source_message_count`) and the D8 prerequisite fallback-account tasks 01+02 (route `command`, agent `fallback_chain`, acceptance contract, work-session pre-acceptance chain, `session.fallback.used`, resume affinity).
- **Cadence tier:** targeted — every user-visible change in the diff plus the compatibility leg (pre-feature home). Unchanged journeys are not re-walked.
- **Status:** planning only. No runtime was launched, no persona session ran, no verdict is recorded here. `make gate-status` at planning time: only a stale `codegen-check` record (fingerprint `2b7ba6b5…`), so no gate evidence exists for the head.
- **Execution owner:** session-continue-fork task_08. It creates `docs/qa/reports/<YYYY-MM-DD>-session-continue-fork.md` from `docs/qa/templates/report.md` before the first session, with every matrix row `Pending`.

## Evidence reconciled

Task completion notes (gitignored workflow memory, read in place):

| Task | Memory | Evidence it recorded (green) | Deferred to QA |
|---|---|---|---|
| continue-fork 01 | `.compozy/tasks/session-continue-fork/memory/task_01.md` | store/globaldb/session/api/cli focused suites (`TestGlobalDBSessionLineageMigration` IT-017, `TestResumeUpgradesPreFeatureLineageMetadata` IT-019, `TestConversationRewindLineageRule`, `TestCreateSessionLineageKind`), `make codegen` idempotent | pre-feature home walk; `RT-conversation-rewind` provenance leg |
| continue-fork 02 | `…/memory/task_02.md` | `-race` acp routing/fork suites (`TestAgentProcessRouteSessionUpdate`, `TestAgentProcessForkSession`, `TestStartCapturesSessionLifecycleCapabilities`) | OpenCode `supports_fork_session: true` live check |
| continue-fork 03 | `…/memory/task_03.md` | derive manager suites, `TestDeriveCommitBoundaries`, `TestDeriveSnapshotConcurrency`, `TestBootSessionRepair`, E2E-003 `TestDaemonE2ESessionContinueCLI` (integration tag) | CLI/retry/handoff walks; E2E-003 not re-run after `17dc03551` |
| continue-fork 04 | `…/memory/task_04.md` | 32 web files / 440 tests, typecheck, oxlint | E2E-001, E2E-004 (`fixme`), VC bundles, web walks |
| continue-fork 05 | `…/memory/task_05.md` | fork manager/http/cli suites, `TestDaemonE2ESessionForkCLI` (acpmock native pending→loaded, load-missing, replay-only) | OpenCode native fork, Claude/Codex replay on real binaries |
| continue-fork 06 | `…/memory/task_06.md` | 13 web files / 285 tests + follow-up 8 files / 95 tests, typecheck | E2E-002, VC bundles, Storybook render of new stories |
| fallback-account 01 | `.compozy/tasks/fallback-account/memory/task_01.md` | config/acp/session/daemon/cli suites, `TestAutoTitleRoleIntegration` (account-route + `reject_set` fence) | `MS-background-role-fallback` walk (route `command`) |
| fallback-account 02 | `.compozy/tasks/fallback-account/memory/task_02.md` | bind/eager/spawn/resume suites, `TestDaemonE2EAgentFallbackChain` | `RT-session-fallback-chain` walk (fallback-account task_04/05 are not on this branch, so task_08 owns it) |

## Scenario ledger

Every scenario the diff touched, with its reused automated evidence and the planned walk. All start `untested`.

| Scenario | Charter | Automated evidence (reused, not a verdict) | Planned walk (task_08) |
|---|---|---|---|
| `ET-web-session-continue` | `CH-session-derive-web` | `session-continue-dialog.test.tsx`, `use-session-derive.test.ts`, `runtime-activity-notice.test.tsx`, `session-status-line.test.tsx`, `session-thread.test.tsx`, `session-inspector.test.tsx`; E2E-001 (not run) | scenario body steps 1–9 on acpmock alpha/beta + route agent; handoff marker via the new prompt-error fixture; dead-runtime banner via persistent crash |
| `ET-web-session-fork-from-here` | `CH-session-derive-web` | `session-fork-dialog.test.tsx` (UT-067/069/074), `session-thread.test.tsx` (UT-068); E2E-002 (not run) | steps 1–8 on `fork-web-agent` and `fork-native-agent` (native sentence); real OpenCode for the native line when CH-session-derive-real-agents runs first |
| `ET-web-sessions-catalog-modal` | `CH-session-derive-web` | `sessions-modal.test.tsx` | focused re-walk: Continue/Fork in the row menu, dialog over the still-open catalog, child in its own window |
| `ET-web-session-sidebar-threads` | `CH-session-derive-web` | `session-status-line.test.tsx` (pill) | continued and forked children nest under the source in rail and modal; in-place select shows the pill |
| `RT-conversation-rewind` | `CH-session-derive-web` (web leg), `CH-session-derive-interrupt` (provenance/upgrade leg) | `TestConversationRewindLineageRule`, `TestDeriveCarriedContextSurvivesRewind` | Fork from here and Rewind disable together; rewind a `--parent` child (new and pre-feature); spawned child still `409` |
| `ET-cli-session-continue` | `CH-session-derive-real-agents` | `TestSessionContinueCommand`, `TestContinueSessionHandler`, `TestPreviewSessionDeriveHandler`, E2E-003 | steps 1–8 with a real Codex source continued into a real Claude agent |
| `RT-session-derive-native-fork` | `CH-session-derive-real-agents` | `TestForkSession`, `TestForkNativeGate`, `TestForkAccountInheritance`, `TestForkSessionHandler`, `TestSessionForkCommand`, `TestDaemonE2ESessionForkCLI` | steps 1–4 and 8 on real `opencode`; step 5 on real `claude` and a Codex source; steps 6–7 on any agent; acpmock `fork-load-missing-agent` fallback for step 8 |
| `RT-session-derive-retry` | `CH-session-derive-interrupt` | derive receipt/idempotency manager cases, `TestContinueSessionHandler` replay cases, `TestDeriveCommitBoundaries` | steps 1–6 incl. daemon restart between return and first turn |
| `RT-provider-error-handoff` | `CH-session-derive-interrupt` | session-owner decoration unit cases, prompt contract case; E2E-004 `fixme` | steps 1–5 on the new acpmock prompt-error fixture (`rate_limited` and `not_authenticated`); spawned child keeps `retry`/`login` |
| `RT-session-lineage-upgrade` (new) | `CH-session-derive-interrupt` | IT-017, IT-019, UT-002 | pre-feature binary writes the home; branch build boots it; steps 1–6 |
| `RT-session-fallback-chain` | `CH-session-fallback-seat` | `TestPromptBindFallbackChain`, `TestCreateFallbackChain`, `TestManagerSpawnRunsAgentFallbackChain`, `TestResumeAcceptedRouteAffinity`, `TestDaemonE2EAgentFallbackChain` | steps 1–7 with `seat-reviewer` (the bundled `reviewer` name collides) |
| `MS-background-role-fallback` | `CH-role-fallback-boundary` (reused) | `TestAutoTitleRoleIntegration` (account route, `reject_set` fence) | the 2026-09-28 route-`command` leg in the scenario body: `roles show` command column, fingerprint without command text, accepted-then-failed start stops the chain |

Journeys reused and extended in place: `J-14` (derive branch + Cancel/Change abandonment), `J-15-operate-session-via-cli-api` (derive branch + lost-response retry abandonment), `J-route-background-work`, `J-rewind-conversation`, `J-operate-desktop-shell` (unchanged).

## Session matrix, ordered by risk

| Order | Charter | Persona | Journey | Scenarios | Tour | Box |
|---:|---|---|---|---|---|---|
| 1 | [`CH-session-derive-real-agents`](../charters/CH-session-derive-real-agents.md) | Rafa | J-15 | `ET-cli-session-continue`, `RT-session-derive-native-fork` | Money Tour | 90 |
| 2 | [`CH-session-derive-web`](../charters/CH-session-derive-web.md) | Bruno | J-14 | `ET-web-session-continue`, `ET-web-session-fork-from-here`, `ET-web-sessions-catalog-modal`, `ET-web-session-sidebar-threads`, `RT-conversation-rewind` (web leg) | Feature Tour | 90 |
| 3 | [`CH-session-derive-interrupt`](../charters/CH-session-derive-interrupt.md) | Théo | J-15 | `RT-session-derive-retry`, `RT-provider-error-handoff`, `RT-session-lineage-upgrade`, `RT-conversation-rewind` (provenance leg) | Interrupt Tour | 90 |
| 4 | [`CH-session-fallback-seat`](../charters/CH-session-fallback-seat.md) | Rafa | J-route-background-work | `RT-session-fallback-chain` | Network Tour | 60 |
| 5 | [`CH-role-fallback-boundary`](../charters/CH-role-fallback-boundary.md) | Ada | J-route-background-work | `MS-background-role-fallback` | Network Tour | 60 |

## Visual Contract bundles (E2E-005)

Authority: `docs/design/opendesign/session-continue-fork/DESIGN-NOTES.md` §VC matrix. Procedure: `eng-ui-screenshot` + `.agents/skills/eng/eng-ui-screenshot/references/visual-contract.md` (reference.png, implementation.png, side-by-side.png, diff.png, comparison.json, review.md; validator exit 0 per row). Viewport 1440×900 except VC-18 (session window 640px wide inside 1440×900). Reference = the board section; implementation = the running lab app.

Bundle root: `.compozy/tasks/session-continue-fork/evidence/visual/<task-id>/<VC>/`. Nothing is bundled yet (the directory does not exist); representative read-only inspections by task_04/06 are not evidence.

| VC | Board · section | Bundle | Implementation state setup |
|---|---|---|---|
| VC-01 | menus §01 | `…/task_04/VC-01/` | sessions modal, kebab open on a user row (alpha session) |
| VC-02 | menus §01 | `…/task_04/VC-02/` | kebab open on an archived row |
| VC-03 | menus §02 | `…/task_04/VC-03/` | window overflow open while a `block_until_cancel` turn runs |
| VC-04 | menus §03 | `…/task_06/VC-04/` | hover a settled user message of `fork-web-agent` |
| VC-05 | menus §03 | `…/task_06/VC-05/` | same, while a later turn runs |
| VC-06 | dialogs §01 | `…/task_04/VC-06/` | Continue from alpha, beta preselected, preview measured |
| VC-07 | dialogs §01 | `…/task_04/VC-07/` | Continue to the route agent (two same-model routes, different `command` → suffix) |
| VC-08 | dialogs §01 | `…/task_04/VC-08/` | source in the workspace whose overlay sets `[session.derive] max_replay_bytes = 4096` |
| VC-09 | dialogs §01 | `…/task_04/VC-09/` | preview request aborted at the browser network boundary (Playwright route); cite as the reachable way to render a client-side failure |
| VC-10 | dialogs §01 | `…/task_04/VC-10/` | Continue POST held by a Playwright route delay |
| VC-11 | dialogs §02 | `…/task_06/VC-11/` | window overflow → Fork session… (whole) |
| VC-12 | dialogs §02 | `…/task_06/VC-12/` | Fork from here on message 2 of 3 (board copy "Carries over 18 of 42 messages" now reachable via `source_message_count`) |
| VC-13 | dialogs §02 | `…/task_06/VC-13/` | whole fork of an idle bound `fork-native-agent` (or real OpenCode) source |
| VC-14 | dialogs §02 | `…/task_06/VC-14/` | preview response with `cut.turn_settled: false` (Playwright route fulfil) — the message gate disables Fork while the thread runs, so the live race is not reproducible on demand; record as authorized source |
| VC-15 | dialogs §02 | `…/task_06/VC-15/` | open the fork dialog, prompt the source from the CLI, submit → fence conflict |
| VC-16 | lineage §01 | `…/task_04/VC-16/` | continued child whose origin agent is named `claude` |
| VC-17 | lineage §01 | `…/task_06/VC-17/` | forked child status line |
| VC-18 | lineage §01 | `…/task_04/VC-18/` | continued child window resized to 640px wide; long title and agent truncate, pill keeps its verb (task_04 flagged: provider truncates instead of dropping) |
| VC-19 | lineage §02 | `…/task_04/VC-19/` | continued child after its first turn |
| VC-20 | lineage §02 | `…/task_04/VC-20/` | continued child without a message (divider + compact empty) |
| VC-21 | menus §04 | `…/task_04/VC-21/` | dead runtime via the `persistent-crash-mid-stream` pattern of `driver_fault_fixture.json` |
| VC-22 | menus §05 | `…/task_04/VC-22/` | rate-limited turn from the new acpmock prompt-error fixture |
| VC-23 | lineage §03 | `…/task_06/VC-23/` | Context sidebar Origin/Seed on a stopped fork-through-message child |

Known authorized deltas to cite in `review.md` (from task_04/task_06 memory and DESIGN-NOTES §Gaps): accent icon well in the dialog header, outline Cancel, neutral `Pill` tokens, `Empty size="compact"` icon well, `RuntimeSelector` default trigger, `OwnerAvatar sm` 20px, Origin/Seed as a Context-sidebar section (ledger panel removed by #635), status-line host chrome (SD-007). Anything else is a blocking divergence.

## Automated items owned by task_08

1. **acpmock prompt-error step (new capability).** acpmock has no way to fail `session/prompt` today (`StepKind` and `DriverControlAction` in `internal/testutil/acpmock/fixture_types.go` only disconnect, write raw JSON-RPC, block, delay, hold). Add a `driver_control` action (e.g. `fail_prompt` with `error_message` and optional JSON-RPC `code`) that answers the matched `session/prompt` with a JSON-RPC error, implemented in `internal/testutil/acpmock/cmd/acpmock-driver/`, with a fixture test beside the existing acpmock fixture tests. The text must classify through `acp.ProviderFailureDiagnosticFromError` → `internal/providers/classify.go` needles (`"429"`, `"rate limit"` → `rate_limited`; an auth phrase → `not_authenticated`). Add the fixture agent the spec expects (second prompt `rate limit this turn`) to a testdata fixture, un-`fixme` E2E-004 in `web/e2e/__tests__/session-derive.spec.ts`, and reuse the same fixture for `RT-provider-error-handoff` and VC-22.
2. **Web E2E run:** E2E-001, E2E-002, E2E-004 in `web/e2e/__tests__/session-derive.spec.ts` through the repo web E2E entry (`make test-e2e-web`; focused Playwright runs are iteration only).
3. **Integration re-run on the final head:** `go test -tags integration ./internal/daemon/ -run 'TestDaemonE2ESessionContinueCLI|TestDaemonE2ESessionForkCLI|TestDaemonE2EAgentFallbackChain|TestAutoTitleRoleIntegration' -count=1` (E2E-003 was last run before `17dc03551`).
4. **Storybook render** of `session-continue-dialog.stories.tsx`, `session-origin.stories.tsx`, `session-fork-dialog.stories.tsx` (never executed).
5. **E2E-005** bundles above.
6. **Dev-daemon checks deferred by tasks 01/02/05:** pre-feature home upgrade (`RT-session-lineage-upgrade`); OpenCode `runtime.acp_caps.supports_fork_session: true` (and `supports_resume_session` as advertised); native fork on OpenCode (`RT-session-derive-native-fork` 1–4); replay path on Claude and Codex (step 5).
7. **`make gate`** on the final head after the last mutation (only a stale `codegen-check` record exists).

## Lab manifest requirements

Two isolated labs; planning allocated none.

**Lab A — main walks and bundles**

```bash
python3 .agents/skills/eng/eng-qa-bootstrap/scripts/bootstrap-qa-env.py \
  --scenario "session-continue-fork" --repo-root . \
  --profile targeted --required-surface cli --required-surface api \
  --required-surface web --required-surface runtime --required-surface provider
```

- Manifest must provide unique non-default `COMPOZY_HOME`, `COMPOZY_HTTP_PORT`, `COMPOZY_UDS_PATH`, `TMUX_BRIDGE_SOCKET`, plus `COMPOZY_WEB_API_PROXY_TARGET`, `QA_OUTPUT_PATH`, `AUDIT_COMMAND`, `TEARDOWN_COMMAND`. Web runs with the manifest proxy target.
- **Real providers (operator home, `native_cli`, `home_policy=operator`):** `opencode` (`/Users/pedronauck/.opencode/bin/opencode`, ACP via `opencode acp`), `claude` (`~/.local/bin/claude`), `codex`. Agents: one OpenCode agent, one Claude agent, one Codex agent. Short prompts only.
- **acpmock provider** registered as in `web/e2e/fixtures/runtime-helpers.ts` (`[providers.acpmock]`, driver built from `./internal/testutil/acpmock/cmd/acpmock-driver`, per-agent `--fixture/--agent/--diagnostics`). Agents: `alpha`, `beta` (`multi_agent_fixture.json`); `fork-web-agent` (`browser_session_fork_fixture.json`); `fork-native-agent`, `fork-load-missing-agent` (`session_fork_fixture.json`); the new prompt-error agent; a lab-local fixture agent with a `block_until_cancel` turn and a `persistent-crash-mid-stream` turn (lab file under `QA_OUTPUT_PATH`, not repo testdata); a route agent whose `fallback_chain` has two routes on the same provider/model with different `command`; `seat-reviewer` whose primary refuses `session/new` (acpmock model outside the fixture list, per task_02 learnings) with a healthy fallback route carrying `command=`.
- **Roles:** `roles.auto_title.fallback_chain` with a `command` route and a refusing primary (`rejected-title-model` from `auto_title_fixture.json`) for `MS-background-role-fallback`.
- **Config:** a second workspace whose overlay sets `[session.derive] max_replay_bytes = 4096` (VC-08 truncation) so the first workspace keeps defaults.
- Browser driver: `browser-use` (Playwright-backed) with route interception available for VC-09/10/14.

**Lab B — pre-feature home upgrade** (unique home/ports/sockets; same bootstrap command with `--scenario "session-continue-fork-upgrade" --profile targeted --required-surface cli --required-surface api --required-surface runtime`)

- Pre-feature binary: `git archive 67b86a9b9 | tar -x -C "$QA_OUTPUT_PATH/prefeature-src"`, `go build -o "$QA_OUTPUT_PATH/bin/compozy-prefeature" ./cmd/compozy` there (no worktree, no checkout of the main tree).
- acpmock provider only; the pre-feature daemon writes the home (root, `--parent` child, spawned or role child, stopped session with an ACP id); snapshot `session list/status -o json` and every `meta.json` before switching binaries.
- Register every daemon, Web server, and driver PID under `<QA_OUTPUT_PATH>/qa/pids/`; append actions to `journey-log.jsonl`.

**Teardown:** each lab runs its exact `eval "$TEARDOWN_COMMAND"` on every terminal path and cites `<QA_OUTPUT_PATH>/qa/teardown.json` with `"clean": true`; finish with `make qa-reap` to confirm no stale lab process survives. Run `python3 "$AUDIT_COMMAND" --qa-output-path "$QA_OUTPUT_PATH" --strict` before each behavior verdict.

## Taxonomy decisions

- **Journeys:** J-14 and J-15 gained a derive branch with a true end state (source unchanged, one child) and an abandonment path (Cancel/Change; lost response → same-key retry).
- **Functional:** every scenario has an executable walk plus an independent structured read (`session status -o json`, `GET /api/sessions`, `compozy logs`).
- **Experiential:** the web charter records menu discoverability, dialog copy against `COPY.md` §6, and focus/keyboard in both dialogs; the VC bundles own pixels.
- **Edge/error/empty:** preview error, truncated context, unsettled cut, fence conflict, archived source, unknown agent, `child_deleted`, native load failure, rate limit/auth lapse, empty child, dead runtime.
- **Cross-cutting:** CLI/HTTP/native-tool parity (`compozy__session_continue`, `compozy__session_fork`), workspace overlay config, restart continuity, pre-feature upgrade (SD-013), real-provider compatibility. Workspace isolation of receipts is covered by `TestDeriveReceiptScope` (UT-078); not re-walked.

## Human-review decisions carried (not QA blockers)

From task memory, flagged for Pedro rather than walked as defects: Origin/Seed in the Context sidebar; `route_not_found` as a Route row; client sentence for `child_deleted`; Open in hidden outside a session window; idempotency-key rotation on 4xx; fork fences skipped for a running source; fork-point quote cut at exactly 60 characters; failed route bind leaves `runtime.status: unbound` (spec) where `_tests.md` IT-026 says `failed`; fallback-account task_01 shipped inside this branch (D8 Q1).

## Close conditions for task_08

- Tracker materializes (`python3 .agents/skills/qa-report/scripts/materialize_state.py docs/qa`) and all 12 scenarios above end `pass`, `blocked-verify`, `blocked-decision`, or reasoned `skipped`; any `fail` links a deduplicated bug and a same-persona re-walk after the fix.
- All 23 VC bundles exist with validator exit 0; E2E-001/002/004 pass; E2E-004 no longer `fixme`.
- Both labs `teardown.json` `clean: true`; `make gate` green on the final head.

## Compozy Impact Audit

Owned by `docs/_memory/change-impact.md` §"Session continue and fork — derived sessions, lineage kind, `handoff`" and its fallback-account entry; this plan adds no contract change. QA line there already lists the scenarios; `RT-session-lineage-upgrade` is the one addition.

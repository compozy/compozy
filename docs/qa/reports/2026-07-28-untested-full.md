# QA Run Report — 2026-07-28 — all current untested scenarios

- **Scope:** Every scenario whose planning snapshot resolved to qa_status=untested (452 rows across ET, GL, LP, MS, NB, REL, RT, SITE, and TA)
- **Cadence tier:** full
- **Build:** 5d8ed82b plus current QA planning repairs · **Environment:** fresh isolated northstar-pay lab, http://127.0.0.1:60565; CLI/HTTP/UDS/Web/provider lanes use the bootstrap manifest
- **Started:** 2026-07-29T02:38:46Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Autonomous Agent | desktop / wifi-fast / en-US | CH-004, CH-018, CH-024, CH-027, CH-031, CH-043, CH-agent-marketplace-parity, CH-compozy-platform-hard-cut, CH-daemon-schema-parity, CH-live-bounds-agent-path, CH-mcp-client-operates-compozy, CH-memory-batch-integrity, CH-reserved-builtin-name-sweep, CH-role-fallback-boundary, CH-runaway-work-bounded, CH-runnable-capabilities-truth, CH-subprocess-health-recovery, CH-untested-001-01-ada, CH-untested-008-07-ada, CH-untested-010-09-ada, CH-untested-035-29-ada, CH-untested-036-30-ada, CH-untested-039-32-ada, CH-untested-042-administer-window-manager-ada, CH-untested-046-agent-marketplace-parity-ada, CH-untested-050-bound-runaway-work-ada, CH-untested-058-extension-policy-admin-ada, CH-untested-066-operate-daemon-schema-ada, CH-untested-070-operate-workspace-context-ada, CH-untested-valid-004-14-ada, CH-untested-valid-006-20-ada, CH-untested-valid-007-23-ada, CH-untested-valid-009-24-ada, CH-untested-valid-015-32-ada, CH-untested-valid-016-administer-network-live-ada, CH-untested-valid-020-extension-policy-admin-ada, CH-untested-valid-021-mcp-authorize-repair-ada, CH-untested-valid-024-operate-daemon-schema-ada, CH-untested-valid-026-validate-compozy-hard-cut-ada, CH-wake-dedup-stress, CH-workspace-run-capacity |
| Bruno | Delivery Builder | desktop / wifi-fast / en-US | CH-003, CH-005, CH-006, CH-007, CH-010, CH-012, CH-022, CH-023, CH-025, CH-026, CH-028, CH-032, CH-038, CH-automation-crud-loop-target, CH-coordination-future-runs, CH-cursor-agent-mode, CH-database-refusal-recovery, CH-loop-goal-delete, CH-marketplace-under-a-minute, CH-mcp-authorize-repair-truth, CH-network-admin-lifecycle, CH-new-session-latency-title, CH-schedule-recovery-guard, CH-suggestions-consent, CH-task-template-draft, CH-task-tree-loop-rollup, CH-untested-002-01-bruno, CH-untested-005-05-bruno, CH-untested-007-06-bruno, CH-untested-009-07-bruno, CH-untested-012-12-bruno, CH-untested-013-13-bruno, CH-untested-014-14-bruno, CH-untested-021-23-bruno, CH-untested-024-24-bruno-part-1, CH-untested-025-24-bruno-part-2, CH-untested-029-26-bruno, CH-untested-031-27-bruno, CH-untested-034-28-bruno, CH-untested-037-31-bruno, CH-untested-040-32-bruno, CH-untested-043-administer-window-manager-bruno-part-1, CH-untested-044-administer-window-manager-bruno-part-2, CH-untested-047-agent-marketplace-parity-bruno, CH-untested-051-complete-task-tree-bruno, CH-untested-056-evaluate-compozy-beta-bruno, CH-untested-059-extension-policy-admin-bruno, CH-untested-063-marketplace-acquisition-bruno, CH-untested-064-mcp-authorize-repair-bruno, CH-untested-067-operate-daemon-schema-bruno, CH-untested-068-operate-desktop-shell-bruno, CH-untested-072-retire-workspace-bruno, CH-untested-valid-001-01-bruno, CH-untested-valid-002-11-bruno, CH-untested-valid-010-24-bruno, CH-untested-valid-013-30-bruno, CH-untested-valid-014-31-bruno, CH-untested-valid-017-agent-marketplace-parity-bruno, CH-untested-valid-022-network-local-default-bruno, CH-untested-valid-027-validate-compozy-hard-cut-bruno |
| Cora | Non-technical Founder | laptop / wifi-fast / en-US | CH-compozy-landing-canary, CH-untested-069-operate-home-dashboard-cora |
| Dora | Runtime Administrator | desktop / wifi-fast / en-US | CH-background-role-routing-scopes, CH-compozy-beta-candidate, CH-drain-without-loss, CH-dream-pipeline-canary, CH-secret-redaction-sweep, CH-settings-roles-live-truth, CH-untested-006-05-dora, CH-untested-017-17-dora, CH-untested-020-22-dora, CH-untested-022-23-dora, CH-untested-026-24-dora, CH-untested-028-25-dora, CH-untested-038-31-dora, CH-untested-041-administer-runtime-settings-dora, CH-untested-049-approve-compozy-beta-candidate-dora, CH-untested-052-complete-task-tree-dora, CH-untested-053-complete-web-bridge-setup-dora, CH-untested-055-drain-daemon-safely-dora, CH-untested-057-evaluate-compozy-beta-dora, CH-untested-061-keep-secrets-contained-dora, CH-untested-062-manage-sandbox-profiles-dora, CH-untested-065-mcp-authorize-repair-dora, CH-untested-071-operate-workspace-context-dora, CH-untested-valid-012-25-dora, CH-untested-valid-023-offer-runnable-capabilities-dora, CH-untested-valid-025-operate-daemon-schema-dora |
| Iris | Remote Operator | laptop / wifi-slow / en-US | CH-remote-operator-manual-auth |
| Lea | First-time Adopter | laptop / wifi-fast / en-US | CH-001, CH-008, CH-030, CH-046, CH-untested-003-01-lea, CH-untested-004-04-lea, CH-untested-019-19-lea, CH-untested-030-26-lea |
| Marina | Reviewer / Evaluator | phone-large / 4g / en-US | CH-002, CH-009, CH-033, CH-untested-015-14-marina, CH-untested-027-24-marina, CH-untested-032-27-marina-part-1, CH-untested-033-27-marina-part-2, CH-untested-048-agent-marketplace-parity-marina, CH-untested-valid-011-24-marina |
| Maya | Channel Teammate | laptop / wifi-slow / en-US | CH-bridge-progress-stress, CH-edit-reply-context |
| Nia | First-time Session Viewer | laptop / wifi-fast / en-US | CH-015, CH-network-local-default |
| Omar | Bridge Fleet Operator | desktop / wifi-fast / en-US | CH-bridge-overload-taxonomy, CH-long-provider-replies, CH-mid-turn-bridge-restart, CH-untested-054-connect-bridge-provider-omar, CH-untested-valid-008-23-omar |
| Rafa | Transcript Reviewer | desktop / wifi-fast / en-US | CH-017, CH-021, CH-039, CH-artifact-recovery-paging, CH-truthful-cost-provenance, CH-untested-valid-019-digest-sessions-into-memory-rafa |
| Sol | Accessibility-Reliant User | desktop / wifi-fast / en-US | CH-013, CH-034, CH-untested-018-17-sol |
| Tessa | First-time Bridge Operator | laptop / wifi-fast / en-US | CH-first-slack-response, CH-web-bridge-setup |
| Théo | Returning Session User | desktop / wifi-fast / en-US | CH-014, CH-016, CH-037, CH-approval-grant-memory, CH-background-session-switch, CH-clarify-answer-roundtrip, CH-crash-resume-compaction, CH-session-affordances-truth, CH-untested-011-11-theo, CH-untested-016-15-theo, CH-untested-023-23-theo, CH-untested-045-administer-window-manager-theo, CH-untested-valid-003-12-theo, CH-untested-valid-005-14-theo, CH-untested-valid-018-answer-agent-requests-theo |
| Vera | Policy Administrator | desktop / wifi-fast / en-US | CH-extension-policy-admin-gates, CH-untested-060-extension-policy-admin-vera |

## Flows in Scope

- `J-01` — A user runs software-delivery with a few inputs and it drives their tasks to a truthful, verified terminal outcome without babysitting. (`../journeys/J-01-arrive-and-use-run.md`)
- `J-02` — A user sees the first generation's plan and confirms their inputs are valid before spending any budget or creating a run. (`../journeys/J-02-dry-run-preview.md`)
- `J-03` — An evaluator trusts a run truly completed and was verified, and can approve/decline the merge gate — from the global Runs queue, on desktop or phone. (`../journeys/J-03-observe-and-approve.md`)
- `J-04` — An operator can safely suspend a running Loop at a generation boundary and resume or stop it, with the status always telling the truth. (`../journeys/J-04-operator-pause-resume.md`)
- `J-05` — An operator adjusts checks, the human gate, re-attempt strategy, and limits for a Loop without rebuilding it — and the next run honors those tweaks. (`../journeys/J-05-configure-no-fork.md`)
- `J-06` — An author adapts a proven Loop's structure on a canvas — gated by the runtime's own validation — and publishes a runnable new version without rebuilding orchestration. (`../journeys/J-06-fork-and-edit.md`)
- `J-07` — An agent runs and monitors a Loop through structured, non-UI surfaces with deterministic output and enforced capability gates — proving web-UI-only control is never required. (`../journeys/J-07-agent-operated-run.md`)
- `J-08` — An agent-authored review becomes inspectable workspace evidence and bounded remediation without depending on a pull-request provider. (`../journeys/J-08-watch-and-maintain.md`)
- `J-09` — An operator makes a Loop run hands-free by attaching an existing automation pointed at it — bounded by the Loop's declared start surfaces. (`../journeys/J-09-automation-start-bindings.md`)
- `J-10` — A user watches agents converse to a decision inside the run and sees the harvested result drive the next step — the multi-agent capability no plain orchestrator has. (`../journeys/J-10-converse-and-decide.md`)
- `J-11` — My running work is still there, current, and truthful when I come back — never a blank thread, never a false status. (`../journeys/J-11-return-to-running-session.md`)
- `J-12` — Opening a session feels instant, even a huge one — one loading phase, full history reachable, never a double spinner. (`../journeys/J-12-open-session-fast.md`)
- `J-13` — I can watch, steer, and trust a live run — the stream is smooth, the composer honors queue semantics, and the settled turn tells the truth. (`../journeys/J-13-follow-a-live-run.md`)
- `J-14` — I can audit exactly what the agent did — every tool call inspectable, grouping legible, usage truthful — without wading through card bulk. (`../journeys/J-14-read-a-finished-transcript.md`)
- `J-15` — An agent can drive and read sessions deterministically over CLI, HTTP, or UDS — bounded REST history, fenced transcript deltas, explicit resets, keep-alive, gap-free reconnect, and identical lifecycle state everywhere. (`../journeys/J-15-operate-session-via-cli-api.md`)
- `J-16` — An operator or agent arms a loop to sleep at zero cost until a specific daemon event happens, wakes it deterministically on a match, and trusts it survives a restart without missing or double-firing. (`../journeys/J-16-watch-events-wake.md`)
- `J-17` — One fast control shows my effective provider, model, and reasoning truthfully — untouched values keep project inheritance, while explicit choices become one session-scoped override. (`../journeys/J-17-session-create-unified-selector.md`)
- `J-19` — On my first run I pick a provider, model, and reasoning depth with the same fast control I'll use everywhere, and it becomes my default — no throwaway grid I never see again. (`../journeys/J-19-onboarding-default-model.md`)
- `J-20` — I can list, curate, refresh, and inspect provider models entirely through structured tool output — every surface agrees on the same state, and bad targets fail with the same stable code. (`../journeys/J-20-catalog-curation-agent-surfaces.md`)
- `J-21` — When I pick max reasoning for a Claude model, the runtime actually applies it before the first prompt — and if it can't, it tells me loudly instead of silently ignoring me. (`../journeys/J-21-claude-reasoning-end-to-end.md`)
- `J-22` — The parts of the app I didn't change — settings inspector, edit form, status card, session/task summaries — still show the same truthful provider and model data after the catalog refactor. (`../journeys/J-22-provider-settings-canary.md`)
- `J-23` — I can resume workspace conversations and act immediately without losing history, seeing partial totals as complete, or sending work to another workspace. (`../journeys/J-23-return-to-network-work.md`)
- `J-24` — I can find, triage, and automate work in catalogs larger than one page without partial counts, client-side reordering, or lost mutations. (`../journeys/J-24-triage-work-at-scale.md`)
- `J-25` — I can find every durable memory in my selected identity and trust the catalog to recover after interruption without losing, ghosting, or leaking knowledge. (`../journeys/J-25-browse-recover-knowledge.md`)
- `J-26` — An operator can state an objective once, see real judge feedback converge it, and safely pause, approve, replace, draft, or clear without losing the durable audit. (`../journeys/J-26-converge-and-control-goal.md`)
- `J-27` — An evaluator can trust every Goal read surface, while a builder can author and run the same closed contract without the UI inventing state. (`../journeys/J-27-observe-and-author-goal.md`)
- `J-28` — A long-running Goal survives context and budget pressure without replaying work, judging partial output, or starting an ungranted effect. (`../journeys/J-28-recover-context-and-budget.md`)
- `J-29` — An agent can manage and audit Goal entirely through structured surfaces, survive crashes/races, and trust that uncertain effects are never silently replayed. (`../journeys/J-29-operate-and-recover-goal.md`)
- `J-30` — An operator can find the right workspace-visible agent without trusting invented status. (`../journeys/J-30-scan-agent-fleet.md`)
- `J-31` — An operator can inspect and safely edit definition-owned state without losing concurrent work. (`../journeys/J-31-steward-agent-definition.md`)
- `J-32` — Operators and agents get one durable lifecycle contract through every public surface. (`../journeys/J-32-manage-agent-lifecycle.md`)
- `J-administer-network-live` — An administrator can govern whether Live exists, bound its defaults, and confirm extension requirements without any setting or activation silently enrolling work. (`../journeys/J-administer-network-live.md`)
- `J-administer-runtime-settings` — I can change runtime policy without guessing which value is active, whether it applied, or whether cancel wrote anything. (`../journeys/J-administer-runtime-settings.md`)
- `J-administer-window-manager` — An operator can customize snapping, layout, focus, gaps, shortcuts, and edge bindings while preserving one validated runtime authority. (`../journeys/J-administer-window-manager.md`)
- `J-agent-marketplace-parity` — As an agent I discover and acquire exactly what a human can, in one structured call per step, with deterministic errors and no hidden web-only state. (`../journeys/J-agent-marketplace-parity.md`)
- `J-answer-agent-requests` — A decision I already made — approval or answer — is remembered, revocable, and never re-asked; an agent question blocks until I answer instead of dead-ending. (`../journeys/J-answer-agent-requests.md`)
- `J-approve-compozy-beta-candidate` — A release administrator can prove which commit, version, channel, and policy would ship while leaving every irreversible action untouched. (`../journeys/J-approve-compozy-beta-candidate.md`)
- `J-bound-runaway-work` — Failure is bounded by budgets, breakers, and liveness — the kernel never loops forever, never double-owns a run, and never kills healthy long work. (`../journeys/J-bound-runaway-work.md`)
- `J-complete-task-tree` — I can finish delegated child work and trust the parent plus its automation to settle without manual cleanup. (`../journeys/J-complete-task-tree.md`)
- `J-complete-web-bridge-setup` — I can finish provider setup from one truthful Web orchestration path without losing the created bridge or mistaking a dry run for a real send. (`../journeys/J-complete-web-bridge-setup.md`)
- `J-connect-bridge-provider` — I can follow the setup path that actually belongs to my provider and prove the bridge with a visible response. (`../journeys/J-connect-bridge-provider.md`)
- `J-deliver-long-formatted-reply` — I receive the complete agent answer in order, readable in my platform's dialect, without invalid Unicode or silent truncation. (`../journeys/J-deliver-long-formatted-reply.md`)
- `J-diagnose-task-session-health` — I can distinguish an unhealthy ACP subprocess from task state, repair the cause, and resume work without hidden restarts or duplicate transitions. (`../journeys/J-diagnose-task-session-health.md`)
- `J-digest-sessions-into-memory` — My everyday sessions become durable, recallable knowledge automatically, and every stage of that background pipeline is inspectable and truthful. (`../journeys/J-digest-sessions-into-memory.md`)
- `J-drain-daemon-safely` — I can quiesce the daemon deliberately: nothing new is admitted, nothing in flight is lost, and every surface tells me the same truthful state. (`../journeys/J-drain-daemon-safely.md`)
- `J-edit-reply-context` — I can correct or contextualize my instruction without the agent treating stale quoted text as a new request. (`../journeys/J-edit-reply-context.md`)
- `J-enable-coordinated-conversations` — An operator can adopt Network collaboration at the moment it is useful without mutating an in-flight run or weakening task authority. (`../journeys/J-enable-coordinated-conversations.md`)
- `J-evaluate-compozy-beta` — A first-time reader can decide whether Compozy is an integrated agent OS, understand the beta boundary, and choose a truthful install or migration path. (`../journeys/J-evaluate-compozy-beta.md`)
- `J-extension-policy-admin` — I decide what this runtime may acquire: unverified stays blocked until I say otherwise, curated means digest-verified, and a bad catalog entry disappears the moment curation pulls it. (`../journeys/J-extension-policy-admin.md`)
- `J-keep-secrets-contained` — Even when an agent echoes a live key, nothing durable or streamed ever holds the raw value — and I can prove it with greps, not promises. (`../journeys/J-keep-secrets-contained.md`)
- `J-manage-sandbox-profiles` — I can govern execution isolation through public surfaces and prove the chosen policy applied only where intended. (`../journeys/J-manage-sandbox-profiles.md`)
- `J-marketplace-acquisition` — I can evaluate and acquire a capability of any kind from one truthful marketplace in under a minute, without losing control of scope, secrets, or trust policy. (`../journeys/J-marketplace-acquisition.md`)
- `J-mcp-authorize-repair` — I can make a remote MCP server actually usable — and see the truth when it is not — from any machine, without ever losing a working credential to a failed attempt. (`../journeys/J-mcp-authorize-repair.md`)
- `J-network-local-default` — A builder can learn that the Agent Network exists and complete ordinary work without hidden enrollment, context cost, model activation, or orchestration dependency. (`../journeys/J-network-local-default.md`)
- `J-offer-runnable-capabilities` — An agent is never offered a skill it cannot run, an operator can always see why something is inactive, and a dead sidecar heals itself instead of being hammered. (`../journeys/J-offer-runnable-capabilities.md`)
- `J-operate-bounded-task-capacity` — Operators can bound concurrent workspace execution while agents keep excess work durable and make progress as capacity opens. (`../journeys/J-operate-bounded-task-capacity.md`)
- `J-operate-compozy-from-mcp-client` — Any MCP-speaking tool becomes a Compozy operator for exactly one workspace — with real effects, real isolation, and no bespoke integration. (`../journeys/J-operate-compozy-from-mcp-client.md`)
- `J-operate-daemon-schema` — An operator can start Compozy without silently rewriting incompatible or corrupt state and can inspect the exact daemon-global schema versions through agent-manageable surfaces. (`../journeys/J-operate-daemon-schema.md`)
- `J-operate-desktop-shell` — I can reach and arrange my work from any shell entry point without duplicate windows, lost context, or invented state. (`../journeys/J-operate-desktop-shell.md`)
- `J-operate-home-dashboard` — I can understand and unblock agent work from Home without reading logs or trusting invented metrics. (`../journeys/J-operate-home-dashboard.md`)
- `J-operate-workspace-context` — People and agents can run workspace-scoped operations from project context while retaining observable, leak-free workspace ownership. (`../journeys/J-operate-workspace-context.md`)
- `J-recover-mid-turn-restart` — I am never left wondering whether a half-answer is still running; Compozy makes the interruption visible and accepts new work only after recovery. (`../journeys/J-recover-mid-turn-restart.md`)
- `J-retire-workspace` — I can remove a project cleanly without deleting active work or credentials owned by another scope. (`../journeys/J-retire-workspace.md`)
- `J-route-background-work` — An operator can control the runtime identity and model used by daemon-owned work without changing its policy or leaking configuration across workspaces. (`../journeys/J-route-background-work.md`)
- `J-run-bounded-live-collaboration` — An agent can collaborate intentionally without hidden activation, runaway ping-pong, lost messages, or unaccounted provider work. (`../journeys/J-run-bounded-live-collaboration.md`)
- `J-validate-compozy-hard-cut` — An operator and an autonomous agent can trust that every runtime and public identity is Compozy, with no hidden legacy alias or state merge. (`../journeys/J-validate-compozy-hard-cut.md`)
- `J-watch-agent-work-channel` — I can tell that the agent is working and what finished without being flooded or leaving the conversation that started the work. (`../journeys/J-watch-agent-work-channel.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---:|---|---|---|---|---|---|---|
| 1 | CH-untested-valid-017-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-001 | Bruno | Feature Tour | Pending | BUG-20260729-skill-workspace-error-mapping | Browser scope passed; staged workspace-status fix awaits its governed commit. |
| 2 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-002 | Ada | Feature Tour | Pass | 2026-07-29 | Effective detail matched across HTTP/UDS/CLI/native tool and remained isolated between two workspaces. |
| 3 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-003 | Bruno | Feature Tour | Pass | 2026-07-29 | Exact verified body rendered in web; critical-content exclusion remained green across structured reads. |
| 4 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-006 | Bruno | Feature Tour | Pass | 2026-07-29 | Web tombstone cleanup passed; malformed isolated agent returned 422 and left no residue. |
| 5 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-007 | Ada | Feature Tour | Pass | 2026-07-29 | Curated/remote search and invalid limits passed across HTTP/UDS/CLI/native discovery. |
| 6 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-008 | Ada | Feature Tour | Pass | 2026-07-29 | Stable entry detail matched HTTP/UDS and both CLI namespaces; negative IDs/kinds classified correctly. |
| 7 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-009 | Bruno | Feature Tour | Pass | 2026-07-29 | Live web install moved 11→12, detail was immediately visible, and typed removal restored 11 with clean HTTP/filesystem state. |
| 8 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-010 | Bruno | Money Tour | Pass | 2026-07-29 | Real CLI/HTTP/UDS check/apply branches stayed visible and up to date; the web truthfully omitted Update for a current entry. |
| 9 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-011 | Bruno | Feature Tour | Pass | 2026-07-29 | HTTP/UDS blank-name validation, typed web removal, daemon-backed CLI removal, and no-residue checks passed. |
| 10 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-012 | Bruno | Feature Tour | Pending | BUG-20260729-skill-agent-default-selection | Staged browser retest selected `general` and applied a tombstone live; governed fix commit remains. |
| 11 | CH-untested-060-extension-policy-admin-vera | J-extension-policy-admin / ET-013 | Vera | Feature Tour | Pending | BUG-20260729-skill-policy-normalized-dirty | Policy persistence/restart behavior is green; staged dirty-state fix awaits its governed commit. |
| 12 | CH-untested-060-extension-policy-admin-vera | J-extension-policy-admin / ET-015 | Vera | Feature Tour | Pass | 2026-07-29 | Live HTTP/UDS/CLI/native/browser state agreed; the owning API suite proved the boot-time defensive 503 branch. |
| 13 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-016 | Ada | Feature Tour | Pass | 2026-07-29 | Positive discovery and invalid-limit paths passed; a fresh unavailable-catalog lab returned 503 and tore down cleanly. |
| 14 | CH-untested-valid-020-extension-policy-admin-ada | J-extension-policy-admin / ET-017 | Ada | Feature Tour | Pass | | Local path validation, policy/consent gates, checksum mismatch, and native/HTTP managed install passed. |
| 15 | CH-untested-valid-020-extension-policy-admin-ada | J-extension-policy-admin / ET-018 | Ada | Feature Tour | Pending | | |
| 16 | CH-untested-059-extension-policy-admin-bruno | J-extension-policy-admin / ET-019 | Bruno | Feature Tour | Pending | BUG-20260729-extension-update-partial-error | Repaired native partial-error replay is green; governed fix commit pending. |
| 17 | CH-untested-059-extension-policy-admin-bruno | J-extension-policy-admin / ET-020 | Bruno | Feature Tour | Pass | | Active-bundle 409, confirmed web/native removal, and post-commit cleanup warning passed. |
| 18 | CH-untested-059-extension-policy-admin-bruno | J-extension-policy-admin / ET-021 | Bruno | Feature Tour | Pass | | HTTP/native/CLI/web toggles applied immediately without restart. |
| 19 | CH-untested-058-extension-policy-admin-ada | J-extension-policy-admin / ET-022 | Ada | Feature Tour | Pass | | |
| 20 | CH-untested-059-extension-policy-admin-bruno | J-extension-policy-admin / ET-024 | Bruno | Feature Tour | Pass | 2026-07-29 | Catalog parity and defensive service-unavailable ownership passed. |
| 21 | CH-network-admin-lifecycle | J-administer-network-live / ET-025 | Bruno | Multi-Tab Tour | Pass | 2026-07-29 | HTTP/UDS/CLI preview agreed on projected resources and the Live requirement digest without persistence. |
| 22 | CH-network-admin-lifecycle | J-administer-network-live / ET-026 | Bruno | Multi-Tab Tour | Pass | BUG-20260715-bundle-confirmation-status-bad-request | Unconfirmed 409, confirmed 201, agent conflict 409, and missing-agent 422 passed across public surfaces. |
| 23 | CH-network-admin-lifecycle | J-administer-network-live / ET-027 | Bruno | Multi-Tab Tour | Pass | BUG-20260715-bundle-activation-version-hidden | HTTP/UDS/CLI/native reads agreed on version, confirmation, inventory, and drift; unknown id returned 404. |
| 24 | CH-network-admin-lifecycle | J-administer-network-live / ET-028 | Bruno | Multi-Tab Tour | Pass | BUG-20260715-bundle-activation-version-hidden | Live reapply, stale conflict/no mutation, repeat confirmation, and canonical changed-digest invalidation passed. |
| 25 | CH-untested-059-extension-policy-admin-bruno | J-extension-policy-admin / ET-029 | Bruno | Feature Tour | Pass | 2026-07-29 | HTTP/CLI/native deactivation cycles removed activation-owned resources and channels cleanly. |
| 26 | CH-untested-valid-016-administer-network-live-ada | J-administer-network-live / ET-030 | Ada | Back-Button Tour | Pass | 2026-07-29 | Network settings exposed declared channels only and removed them immediately on deactivation. |
| 27 | CH-untested-046-agent-marketplace-parity-ada | J-agent-marketplace-parity / ET-031 | Ada | Feature Tour | Pass | | |
| 28 | CH-untested-046-agent-marketplace-parity-ada | J-agent-marketplace-parity / ET-032 | Ada | Feature Tour | Pass | | |
| 29 | CH-untested-046-agent-marketplace-parity-ada | J-agent-marketplace-parity / ET-033 | Ada | Feature Tour | Pass | BUG-0009 | 8eeb8a38 |
| 30 | CH-untested-046-agent-marketplace-parity-ada | J-agent-marketplace-parity / ET-035 | Ada | Feature Tour | Pass | | |
| 31 | CH-untested-046-agent-marketplace-parity-ada | J-agent-marketplace-parity / ET-036 | Ada | Feature Tour | Pass | | |
| 32 | CH-untested-048-agent-marketplace-parity-marina | J-agent-marketplace-parity / ET-037 | Marina | Feature Tour | Pass | 2026-07-29 | HTTP/CLI digests matched, scope mismatch was typed, and raw approval tokens never entered durable evidence or logs. |
| 33 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-038 | Bruno | Feature Tour | Pass | 2026-07-29 | Completed invoke contract plus stable invalid and unavailable errors over HTTP, UDS, and CLI. |
| 34 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-040 | Bruno | Feature Tour | Pass | 2026-07-29 | HTTP/UDS parity for 27 valid toolsets; CLI list/info and negative ID branches passed. |
| 35 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-041 | Bruno | Feature Tour | Pass | 2026-07-29 | Five workspace-clean hooks matched across HTTP/UDS; CLI/native list/info passed. |
| 36 | CH-untested-046-agent-marketplace-parity-ada | J-agent-marketplace-parity / ET-042 | Ada | Feature Tour | Pass | | |
| 37 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-043 | Bruno | Feature Tour | Pass | Fresh workspace-scoped acpmock session produced applied post-create and post-stop audit records with exact HTTP/UDS parity, CLI/native parity, missing-session 400, and wrong-workspace 404 isolation. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/019-hook-session-audit |
| 38 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-044 | Vera | Garbage Tour | Pending | | |
| 39 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-045 | Vera | Garbage Tour | Pending | | |
| 40 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-046 | Bruno | Feature Tour | Pending | | |
| 41 | CH-untested-valid-021-mcp-authorize-repair-ada | J-mcp-authorize-repair / ET-047 | Ada | Network Tour | Pass | 2026-07-29 | Fresh automatic/manual S256 flows, scoped status, alias, timeout, logout isolation, redaction, and cleanup passed. |
| 42 | CH-031 | J-20 / ET-049 | Ada | Feature Tour | Pass | 2026-07-29 | All 37 required native IDs were registered/callable and invoked; cross-surface parity, no-write validation, MCP registry recovery, and complete one-time startup guidance passed. |
| 43 | CH-untested-060-extension-policy-admin-vera | J-extension-policy-admin / ET-050 | Vera | Feature Tour | Pending | | |
| 44 | CH-untested-valid-001-01-bruno | J-01 / ET-052 | Bruno | Feature Tour | Pass | The default-enrolled package left and restored its two Loops, three agents, and three tools exactly; unknown and repeated lifecycle commands did not corrupt state, and no watch source appeared. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/026-dev-cycle-lifecycle |
| 45 | CH-031 | J-20 / ET-053 | Ada | Feature Tour | Pending | | |
| 46 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-api-marketplace-namespace | Ada | Feature Tour | Pending | BUG-20260729-marketplace-file-cursor-fence | Rebuilt pagination replay is green; the root fix awaits its governed commit. |
| 47 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-api-mcp-catalog-install | Ada | Feature Tour | Pending | | Public install, validation, replacement, redaction, and cleanup passed; deterministic rollback/event fault branches remain. |
| 48 | CH-untested-valid-021-mcp-authorize-repair-ada | J-mcp-authorize-repair / ET-api-mcp-oauth-endpoints | Ada | Network Tour | Pending | | Executable OAuth endpoint, race, expiry, replacement, non-loopback, and cleanup branches are green; the defensive callback-503 state lacks a public fault owner. |
| 49 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-cli-marketplace-info | Ada | Feature Tour | Pending | BUG-20260729-marketplace-json-parity | Rebuilt CLI/HTTP/UDS parity is green; the root fix awaits its governed commit. |
| 50 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-cli-marketplace-search | Ada | Feature Tour | Pending | BUG-20260729-marketplace-json-parity; BUG-20260729-marketplace-file-cursor-fence | Rebuilt JSON and pagination replay is green; both root fixes await governed commits. |
| 51 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / ET-cli-mcp-auth-manual-exchange | Iris | Paste Tour | Pending | BUG-20260729-mcp-manual-exchange-timeout | Paste, redirect, cancellation, and both timeout phases are green on the rebuilt CLI; the root fix awaits its governed commit. |
| 52 | CH-untested-valid-021-mcp-authorize-repair-ada | J-mcp-authorize-repair / ET-cli-mcp-authorize | Ada | Network Tour | Pending | BUG-20260729-mcp-cli-json-parity | OAuth lifecycle is green; the structural JSON writer fix awaits its TechSpec. |
| 53 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-cli-mcp-install | Ada | Feature Tour | Pending | BUG-20260729-mcp-cli-json-parity | Install lifecycle is green; the structural JSON writer fix awaits its TechSpec. |
| 54 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / ET-compozy-extension-contract-identity | Ada | Garbage Tour | Pending | | |
| 55 | CH-untested-valid-026-validate-compozy-hard-cut-ada | J-validate-compozy-hard-cut / ET-compozy-public-brand-navigation | Ada | Feature Tour | Pending | | |
| 56 | CH-untested-valid-020-extension-policy-admin-ada | J-extension-policy-admin / ET-ext-curated-digest-verify | Ada | Feature Tour | Pending | | |
| 57 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-drag-rebalance | Bruno | Back-Button Tour | Pending | | |
| 58 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-gaps-follow-canvas | Bruno | Back-Button Tour | Pending | | |
| 59 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-group-overlap-refused | Bruno | Back-Button Tour | Pending | | |
| 60 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-load-saved-layout | Bruno | Back-Button Tour | Pending | | |
| 61 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-shortcut-recorder | Bruno | Back-Button Tour | Pending | | |
| 62 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-split-orientation | Bruno | Back-Button Tour | Pending | | |
| 63 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-split-weights | Bruno | Back-Button Tour | Pending | | |
| 64 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-marketplace-kill-switch | Vera | Garbage Tour | Pending | | |
| 65 | CH-untested-valid-006-20-ada | J-20 / ET-model-source-five-rate-pricing | Ada | Feature Tour | Pending | | |
| 66 | CH-approval-grant-memory | J-answer-agent-requests / ET-native-tool-approval-grants | Théo | Interrupt Tour | Pending | | |
| 67 | CH-untested-070-operate-workspace-context-ada | J-operate-workspace-context / ET-native-workspace-scope-isolation | Ada | Back-Button Tour | Pending | | |
| 68 | CH-runnable-capabilities-truth | J-offer-runnable-capabilities / ET-skill-activation-gates | Ada | Feature Tour | Pending | | |
| 69 | CH-artifact-recovery-paging | J-14 / ET-tool-result-artifact-recovery | Rafa | Garbage Tour | Pending | | |
| 70 | CH-untested-037-31-bruno | J-31 / ET-web-agent-detail-tab-parity | Bruno | Back-Button Tour | Pending | | |
| 71 | CH-untested-037-31-bruno | J-31 / ET-web-agent-fleet-listing-rows | Bruno | Back-Button Tour | Pass | Live Rows and Cards matched the ten-agent workspace catalog; shared row/card grammar, origins, monospace metadata, trail/actions, and route round-trip all passed without console or page errors. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/043-agent-fleet-listing |
| 72 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-bundle-activation-detail | Bruno | Money Tour | Pending | | |
| 73 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-bundle-preview-activate | Bruno | Money Tour | Pending | | |
| 74 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-catalog-navigation | Bruno | Money Tour | Pending | | |
| 75 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-command-palette-shortcuts | Bruno | Feature Tour | Pending | | |
| 76 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-desktop-shell-lifecycle | Bruno | Feature Tour | Pending | | |
| 77 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-dock-default-window-size | Bruno | Feature Tour | Pending | | |
| 78 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-dock-magnification | Bruno | Feature Tour | Pending | | |
| 79 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-ext-policy-block | Bruno | Money Tour | Pending | | |
| 80 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-extension-detail | Bruno | Money Tour | Pending | | |
| 81 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-extensions-manage | Bruno | Money Tour | Pending | | |
| 82 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-inter-opsz-medium-510 | Bruno | Feature Tour | Pending | | |
| 83 | CH-untested-024-24-bruno-part-1 | J-24 / ET-web-jobs-triggers-catalog | Bruno | Garbage Tour | Pending | | |
| 84 | CH-untested-007-06-bruno | J-06 / ET-web-loop-editor-node-truncate | Bruno | Back-Button Tour | Pending | | |
| 85 | CH-untested-007-06-bruno | J-06 / ET-web-loop-editor-sidebar-tabs | Bruno | Back-Button Tour | Pending | | |
| 86 | CH-untested-007-06-bruno | J-06 / ET-web-loop-editor-topbar | Bruno | Back-Button Tour | Pending | | |
| 87 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-marketplace-installed-management | Bruno | Feature Tour | Pending | | |
| 88 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-marketplace-kind-navigation | Bruno | Feature Tour | Pending | | |
| 89 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-marketplace-landing-browse | Bruno | Money Tour | Pending | | |
| 90 | CH-untested-064-mcp-authorize-repair-bruno | J-mcp-authorize-repair / ET-web-marketplace-mcp-authorize-installed | Bruno | Network Tour | Pending | | |
| 91 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-marketplace-remove-scope-return | Bruno | Feature Tour | Pending | | |
| 92 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-marketplace-search-fanout | Bruno | Money Tour | Pending | | |
| 93 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-marketplace-skill-install | Bruno | Money Tour | Pending | | |
| 94 | CH-mcp-authorize-repair-truth | J-mcp-authorize-repair / ET-web-mcp-authorize | Bruno | Interrupt Tour | Pending | | |
| 95 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / ET-web-mcp-authorize-manual | Iris | Paste Tour | Pending | | |
| 96 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-mcp-guided-install | Bruno | Money Tour | Pending | | |
| 97 | CH-mcp-authorize-repair-truth | J-mcp-authorize-repair / ET-web-mcp-remote-editor | Bruno | Interrupt Tour | Pending | | |
| 98 | CH-mcp-authorize-repair-truth | J-mcp-authorize-repair / ET-web-mcp-status-matrix | Bruno | Interrupt Tour | Pending | | |
| 99 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-menubar-menu-set | Bruno | Feature Tour | Pending | | |
| 100 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-page-content-gutter | Bruno | Feature Tour | Pending | | |
| 101 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-route-chrome-topbar | Bruno | Feature Tour | Pending | | |
| 102 | CH-untested-018-17-sol | J-17 / ET-web-runtime-selector-minimal-slider | Sol | Feature Tour | Pending | | |
| 103 | CH-untested-070-operate-workspace-context-ada | J-operate-workspace-context / ET-web-session-deep-link-isolation | Ada | Back-Button Tour | Pending | | |
| 104 | CH-untested-014-14-bruno | J-14 / ET-web-session-inspector-toggle | Bruno | Feature Tour | Pending | | |
| 105 | CH-untested-012-12-bruno | J-12 / ET-web-session-thread-full-bleed | Bruno | Feature Tour | Pending | | |
| 106 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-sessions-catalog-modal | Bruno | Feature Tour | Pending | | |
| 107 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-web-settings-extensions-policy | Vera | Garbage Tour | Pending | | |
| 108 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-web-settings-hooks | Vera | Garbage Tour | Pending | | |
| 109 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-shell-shortcuts-about-dialogs | Bruno | Feature Tour | Pass | Keyboard-only menubar navigation opened both capped dialogs; live shortcut and daemon-status truth, degraded polling, Escape, and focus return all passed. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/041-shell-shortcuts-about-dialogs |
| 110 | CH-untested-024-24-bruno-part-1 | J-24 / ET-web-tasks-mode-url | Bruno | Garbage Tour | Pending | | |
| 111 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-ui-resilience | Bruno | Feature Tour | Pending | | |
| 112 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-vault-opendesign-listing | Bruno | Feature Tour | Pending | | |
| 113 | CH-untested-061-keep-secrets-contained-dora | J-keep-secrets-contained / ET-web-vault-overwrite-confirmation | Dora | Garbage Tour | Pending | | |
| 114 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-window-routing-lifecycle | Bruno | Feature Tour | Pending | | |
| 115 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-window-manager-drop-swap | Bruno | Back-Button Tour | Pending | | |
| 116 | CH-untested-042-administer-window-manager-ada | J-administer-window-manager / ET-window-manager-hooks-resources | Ada | Back-Button Tour | Pending | | |
| 117 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-window-manager-layout-gestures | Bruno | Back-Button Tour | Pending | | |
| 118 | CH-untested-042-administer-window-manager-ada | J-administer-window-manager / ET-window-manager-layout-recovery | Ada | Back-Button Tour | Pending | | |
| 119 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-window-manager-multi-client | Bruno | Back-Button Tour | Pending | | |
| 120 | CH-untested-042-administer-window-manager-ada | J-administer-window-manager / ET-window-manager-public-parity | Ada | Back-Button Tour | Pending | | |
| 121 | CH-mcp-client-operates-compozy | J-operate-compozy-from-mcp-client / ET-workspace-host-api-mcp | Ada | Feature Tour | Pending | | |
| 122 | CH-046 | J-26 / GL-003 | Lea | Feature Tour | Pending | | |
| 123 | CH-043 | J-29 / GL-028 | Ada | Feature Tour | Pending | | |
| 124 | CH-043 | J-29 / GL-036 | Ada | Feature Tour | Pending | | |
| 125 | CH-001 | J-01 / LP-001 | Lea | Feature Tour | Pending | | |
| 126 | CH-001 | J-01 / LP-002 | Lea | Feature Tour | Pending | | |
| 127 | CH-012 | J-01 / LP-003 | Bruno | Feature Tour | Pending | | |
| 128 | CH-012 | J-01 / LP-005 | Bruno | Feature Tour | Pending | | |
| 129 | CH-008 | J-02 / LP-006 | Lea | Garbage Tour | Pending | | |
| 130 | CH-002 | J-03 / LP-008 | Marina | Interrupt Tour | Pending | | |
| 131 | CH-002 | J-03 / LP-009 | Marina | Interrupt Tour | Pending | | |
| 132 | CH-003 | J-04 / LP-014 | Bruno | Interrupt Tour | Pending | | |
| 133 | CH-003 | J-04 / LP-016 | Bruno | Interrupt Tour | Pending | | |
| 134 | CH-006 | J-05 / LP-017 | Bruno | Back-Button Tour | Pending | | |
| 135 | CH-006 | J-05 / LP-018 | Bruno | Back-Button Tour | Pending | | |
| 136 | CH-006 | J-05 / LP-019 | Bruno | Back-Button Tour | Pending | | |
| 137 | CH-013 | J-05 / LP-020 | Sol | Back-Button Tour | Pending | | |
| 138 | CH-007 | J-06 / LP-021 | Bruno | Multi-Tab Tour | Pending | | |
| 139 | CH-007 | J-06 / LP-022 | Bruno | Multi-Tab Tour | Pending | | |
| 140 | CH-007 | J-06 / LP-023 | Bruno | Multi-Tab Tour | Pending | | |
| 141 | CH-007 | J-06 / LP-024 | Bruno | Multi-Tab Tour | Pending | | |
| 142 | CH-004 | J-07 / LP-025 | Ada | Feature Tour | Pending | | |
| 143 | CH-004 | J-07 / LP-026 | Ada | Feature Tour | Pending | | |
| 144 | CH-004 | J-07 / LP-027 | Ada | Feature Tour | Pending | | |
| 145 | CH-004 | J-07 / LP-028 | Ada | Feature Tour | Pending | | |
| 146 | CH-005 | J-08 / LP-029 | Bruno | Interrupt Tour | Pending | | |
| 147 | CH-005 | J-08 / LP-030 | Bruno | Interrupt Tour | Pending | | |
| 148 | CH-009 | J-09 / LP-033 | Marina | Back-Button Tour | Pending | | |
| 149 | CH-009 | J-09 / LP-034 | Marina | Back-Button Tour | Pending | | |
| 150 | CH-009 | J-09 / LP-035 | Marina | Back-Button Tour | Pending | | |
| 151 | CH-010 | J-10 / LP-036 | Bruno | Feature Tour | Pending | | |
| 152 | CH-022 | J-16 / LP-040 | Bruno | Feature Tour | Pending | | |
| 153 | CH-023 | J-16 / LP-041 | Bruno | Interrupt Tour | Pending | | |
| 154 | CH-024 | J-16 / LP-042 | Ada | Feature Tour | Pending | | |
| 155 | CH-022 | J-16 / LP-043 | Bruno | Feature Tour | Pending | | |
| 156 | CH-022 | J-16 / LP-044 | Bruno | Feature Tour | Pending | | |
| 157 | CH-027 | J-07 / LP-045 | Ada | Feature Tour | Pending | | |
| 158 | CH-026 | J-01 / LP-046 | Bruno | Feature Tour | Pending | | |
| 159 | CH-025 | J-16 / LP-047 | Bruno | Feature Tour | Pending | | |
| 160 | CH-025 | J-16 / LP-048 | Bruno | Feature Tour | Pending | | |
| 161 | CH-025 | J-16 / LP-049 | Bruno | Feature Tour | Pending | | |
| 162 | CH-025 | J-16 / LP-050 | Bruno | Feature Tour | Pending | | |
| 163 | CH-untested-003-01-lea | J-01 / LP-action-failure-detail | Lea | Feature Tour | Pending | | |
| 164 | CH-loop-goal-delete | J-06 / LP-delete-custom-loop | Bruno | Feature Tour | Pending | | |
| 165 | CH-untested-004-04-lea | J-04 / LP-run-detail-story-redesign | Lea | Feature Tour | Pending | | |
| 166 | CH-task-tree-loop-rollup | J-complete-task-tree / LP-task-rollup-wakes-loop | Bruno | Feature Tour | Pending | | |
| 167 | CH-loop-goal-delete | J-06 / LP-toggle-loop-goal | Bruno | Feature Tour | Pending | | |
| 168 | CH-untested-006-05-dora | J-05 / LP-web-loop-configure-modal | Dora | Back-Button Tour | Pending | | |
| 169 | CH-039 | J-25 / MS-001 | Rafa | Interrupt Tour | Pending | | |
| 170 | CH-039 | J-25 / MS-006 | Rafa | Interrupt Tour | Pending | | |
| 171 | CH-untested-valid-012-25-dora | J-25 / MS-008 | Dora | Back-Button Tour | Pending | | |
| 172 | CH-untested-valid-012-25-dora | J-25 / MS-009 | Dora | Back-Button Tour | Pending | | |
| 173 | CH-untested-valid-019-digest-sessions-into-memory-rafa | J-digest-sessions-into-memory / MS-011 | Rafa | Feature Tour | Pending | | |
| 174 | CH-039 | J-25 / MS-015 | Rafa | Interrupt Tour | Pending | | |
| 175 | CH-dream-pipeline-canary | J-digest-sessions-into-memory / MS-016 | Dora | Feature Tour | Pending | | |
| 176 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-025 | Dora | Back-Button Tour | Pending | | |
| 177 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-027 | Dora | Back-Button Tour | Pending | | |
| 178 | CH-033 | J-22 / MS-028 | Marina | Back-Button Tour | Pending | | |
| 179 | CH-mcp-authorize-repair-truth | J-mcp-authorize-repair / MS-029 | Bruno | Interrupt Tour | Pending | | |
| 180 | CH-untested-062-manage-sandbox-profiles-dora | J-manage-sandbox-profiles / MS-030 | Dora | Feature Tour | Pending | | |
| 181 | CH-untested-055-drain-daemon-safely-dora | J-drain-daemon-safely / MS-035 | Dora | Network Tour | Pending | | |
| 182 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-036 | Dora | Back-Button Tour | Pending | | |
| 183 | CH-network-admin-lifecycle | J-administer-network-live / MS-037 | Bruno | Multi-Tab Tour | Pending | | |
| 184 | CH-untested-061-keep-secrets-contained-dora | J-keep-secrets-contained / MS-038 | Dora | Garbage Tour | Pending | | |
| 185 | CH-untested-061-keep-secrets-contained-dora | J-keep-secrets-contained / MS-040 | Dora | Garbage Tour | Pending | | |
| 186 | CH-untested-061-keep-secrets-contained-dora | J-keep-secrets-contained / MS-041 | Dora | Garbage Tour | Pending | | |
| 187 | CH-031 | J-20 / MS-042 | Ada | Feature Tour | Pass | Curated default, five-rate nullability, and the live Runtime selector's browse/search projection passed against the shared catalog. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/044-model-catalog-read-parity |
| 188 | CH-031 | J-20 / MS-043 | Ada | Feature Tour | Pass | Provider/global refresh retained successful sources beside typed, redacted failures across CLI, HTTP, UDS, and native tools. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/045-model-catalog-lifecycle |
| 189 | CH-031 | J-20 / MS-044 | Ada | Feature Tour | Pass | Provider and global source-status payloads had one exact normalized hash per projection across all four structured surfaces. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/045-model-catalog-lifecycle |
| 190 | CH-031 | J-20 / MS-045 | Ada | Feature Tour | Pass | The HTTP OpenAI list matched curated identities and sampled cost buckets exactly; invalid provider syntax returned a typed OpenAI-style 400 envelope. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/044-model-catalog-read-parity |
| 191 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-049 | Dora | Back-Button Tour | Pending | | |
| 192 | CH-031 | J-20 / MS-053 | Ada | Feature Tour | Pass | All was a strict 498-row superset of the 473-row curated default, adding 25 non-curated deprecated OpenCode rows; the Web selector revealed one only through search. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/044-model-catalog-read-parity |
| 193 | CH-031 | J-20 / MS-054 | Ada | Feature Tour | Pass | Four serialized live curation mutations converged across CLI, HTTP, UDS, and native readback; missing targets retained model_not_found. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/045-model-catalog-lifecycle |
| 194 | CH-031 | J-20 / MS-055 | Ada | Feature Tour | Pass | Complete curated and all payloads had identical normalized hashes across CLI, HTTP, UDS, and native structured output, including independent cost fields. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/044-model-catalog-read-parity |
| 195 | CH-031 | J-20 / MS-056 | Ada | Feature Tour | Pending | BUG-20260729-provider-model-pricing-roundtrip; BUG-20260729-provider-model-validation-status | Repaired five-rate/reasoning/restart and validation replay is green; governed fix commits remain. |
| 196 | CH-033 | J-22 / MS-058 | Marina | Back-Button Tour | Pending | | |
| 197 | CH-039 | J-25 / MS-059 | Rafa | Interrupt Tour | Pending | | |
| 198 | CH-memory-batch-integrity | J-11 / MS-atomic-memory-batch | Ada | Garbage Tour | Pending | | |
| 199 | CH-role-fallback-boundary | J-route-background-work / MS-background-role-fallback | Ada | Network Tour | Pending | | |
| 200 | CH-background-role-routing-scopes | J-route-background-work / MS-background-role-routing | Dora | Feature Tour | Pending | | |
| 201 | CH-untested-044-administer-window-manager-bruno-part-2 | J-administer-window-manager / MS-configure-window-manager | Bruno | Back-Button Tour | Pending | | |
| 202 | CH-drain-without-loss | J-drain-daemon-safely / MS-daemon-memory-reporting | Dora | Interrupt Tour | Pending | | |
| 203 | CH-untested-044-administer-window-manager-bruno-part-2 | J-administer-window-manager / MS-layout-editor-clear-selection | Bruno | Back-Button Tour | Pending | | |
| 204 | CH-untested-044-administer-window-manager-bruno-part-2 | J-administer-window-manager / MS-layout-profile-cli-roundtrip | Bruno | Back-Button Tour | Pending | | |
| 205 | CH-extension-policy-admin-gates | J-extension-policy-admin / MS-marketplace-catalog-live-config | Vera | Garbage Tour | Pending | | |
| 206 | CH-untested-020-22-dora | J-22 / MS-provider-detail-modal | Dora | Back-Button Tour | Pending | | |
| 207 | CH-settings-roles-live-truth | J-route-background-work / MS-settings-roles-panel | Dora | Back-Button Tour | Pending | | |
| 208 | CH-untested-038-31-dora | J-31 / MS-web-agent-create-simple-advanced | Dora | Back-Button Tour | Pass | One Simple/Advanced surface preserved values across both validation branches, omitted MCP authoring, persisted the exact category, matched HTTP/UDS, and cleaned up fully. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/040-agent-create-authored-files |
| 209 | CH-untested-053-complete-web-bridge-setup-dora | J-complete-web-bridge-setup / MS-web-bridge-create-secret-slots | Dora | Network Tour | Pending | | |
| 210 | CH-untested-053-complete-web-bridge-setup-dora | J-complete-web-bridge-setup / MS-web-bridge-edit-delivery-fold | Dora | Network Tour | Pending | | |
| 211 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-web-entity-modal-shell | Dora | Back-Button Tour | Pending | | |
| 212 | CH-untested-028-25-dora | J-25 / MS-web-knowledge-edit-immutable-identity | Dora | Back-Button Tour | Pending | | |
| 213 | CH-untested-065-mcp-authorize-repair-dora | J-mcp-authorize-repair / MS-web-mcp-editor-simple-advanced | Dora | Network Tour | Pending | | |
| 214 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-web-modal-help-tips | Dora | Back-Button Tour | Pending | | |
| 215 | CH-untested-020-22-dora | J-22 / MS-web-provider-auth-gate | Dora | Back-Button Tour | Pending | | |
| 216 | CH-untested-062-manage-sandbox-profiles-dora | J-manage-sandbox-profiles / MS-web-sandbox-profile-advanced | Dora | Feature Tour | Pending | | |
| 217 | CH-untested-017-17-dora | J-17 / MS-web-session-simple-advanced-launch | Dora | Feature Tour | Pending | | |
| 218 | CH-untested-020-22-dora | J-22 / MS-web-settings-providers-redesign | Dora | Back-Button Tour | Pending | | |
| 219 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-web-settings-takeover-redesign | Dora | Back-Button Tour | Pending | | |
| 220 | CH-untested-052-complete-task-tree-dora | J-complete-task-tree / MS-web-task-editor-window-modal | Dora | Feature Tour | Pending | | |
| 221 | CH-untested-071-operate-workspace-context-dora | J-operate-workspace-context / MS-web-workspace-add-directory-browser | Dora | Back-Button Tour | Pending | | |
| 222 | CH-crash-resume-compaction | J-11 / MS-workspace-checkpoint-continuity | Théo | Interrupt Tour | Pending | | |
| 223 | CH-untested-070-operate-workspace-context-ada | J-operate-workspace-context / MS-workspace-resolution-chain | Ada | Back-Button Tour | Pending | | |
| 224 | CH-untested-070-operate-workspace-context-ada | J-operate-workspace-context / MS-workspace-resolution-provenance | Ada | Back-Button Tour | Pending | | |
| 225 | CH-untested-valid-016-administer-network-live-ada | J-administer-network-live / NB-001 | Ada | Back-Button Tour | Pending | | |
| 226 | CH-network-admin-lifecycle | J-administer-network-live / NB-002 | Bruno | Multi-Tab Tour | Pending | | |
| 227 | CH-037 | J-23 / NB-003 | Théo | Interrupt Tour | Pending | | |
| 228 | CH-037 | J-23 / NB-004 | Théo | Interrupt Tour | Pending | | |
| 229 | CH-037 | J-23 / NB-005 | Théo | Interrupt Tour | Pending | | |
| 230 | CH-untested-023-23-theo | J-23 / NB-006 | Théo | Network Tour | Pending | | |
| 231 | CH-037 | J-23 / NB-007 | Théo | Interrupt Tour | Pending | | |
| 232 | CH-037 | J-23 / NB-008 | Théo | Interrupt Tour | Pending | | |
| 233 | CH-037 | J-23 / NB-009 | Théo | Interrupt Tour | Pending | | |
| 234 | CH-037 | J-23 / NB-010 | Théo | Interrupt Tour | Pending | | |
| 235 | CH-untested-023-23-theo | J-23 / NB-011 | Théo | Network Tour | Pending | | |
| 236 | CH-037 | J-23 / NB-012 | Théo | Interrupt Tour | Pending | | |
| 237 | CH-037 | J-23 / NB-013 | Théo | Interrupt Tour | Pending | | |
| 238 | CH-untested-023-23-theo | J-23 / NB-014 | Théo | Network Tour | Pending | | |
| 239 | CH-037 | J-23 / NB-015 | Théo | Interrupt Tour | Pending | | |
| 240 | CH-untested-023-23-theo | J-23 / NB-016 | Théo | Network Tour | Pending | | |
| 241 | CH-untested-023-23-theo | J-23 / NB-017 | Théo | Network Tour | Pending | | |
| 242 | CH-037 | J-23 / NB-019 | Théo | Interrupt Tour | Pending | | |
| 243 | CH-untested-valid-007-23-ada | J-23 / NB-020 | Ada | Network Tour | Pending | | |
| 244 | CH-untested-valid-016-administer-network-live-ada | J-administer-network-live / NB-023 | Ada | Back-Button Tour | Pass | 2026-07-29 | Shared owning journey proved declaration-only HTTP/UDS/CLI/native parity with no implicit enrollment. |
| 245 | CH-untested-valid-008-23-omar | J-23 / NB-027 | Omar | Network Tour | Pending | | |
| 246 | CH-mid-turn-bridge-restart | J-recover-mid-turn-restart / NB-031 | Omar | Interrupt Tour | Pending | | |
| 247 | CH-untested-valid-008-23-omar | J-23 / NB-032 | Omar | Network Tour | Pending | | |
| 248 | CH-first-slack-response | J-connect-bridge-provider / NB-037 | Tessa | Feature Tour | Pending | | |
| 249 | CH-web-bridge-setup | J-complete-web-bridge-setup / NB-039 | Tessa | Back-Button Tour | Pending | | |
| 250 | CH-untested-021-23-bruno | J-23 / NB-045 | Bruno | Network Tour | Pending | | |
| 251 | CH-037 | J-23 / NB-047 | Théo | Interrupt Tour | Pending | | |
| 252 | CH-live-bounds-agent-path | J-run-bounded-live-collaboration / NB-agent-manages-participation | Ada | Interrupt Tour | Pending | | |
| 253 | CH-edit-reply-context | J-edit-reply-context / NB-bridge-edit-reply | Maya | Interrupt Tour | Pending | | |
| 254 | CH-bridge-overload-taxonomy | J-connect-bridge-provider / NB-bridge-overload-recovery | Omar | Network Tour | Pending | | |
| 255 | CH-first-slack-response | J-connect-bridge-provider / NB-bridge-provider-setup | Tessa | Feature Tour | Pending | | |
| 256 | CH-mid-turn-bridge-restart | J-recover-mid-turn-restart / NB-bridge-restart-recovery | Omar | Interrupt Tour | Pending | | |
| 257 | CH-bridge-progress-stress | J-watch-agent-work-channel / NB-bridge-tool-progress | Maya | Garbage Tour | Pending | | |
| 258 | CH-coordination-future-runs | J-enable-coordinated-conversations / NB-coordination-invitation-future-runs | Bruno | Back-Button Tour | Pending | | |
| 259 | CH-untested-054-connect-bridge-provider-omar | J-connect-bridge-provider / NB-indeterminate-bridge-delivery | Omar | Network Tour | Pending | | |
| 260 | CH-long-provider-replies | J-deliver-long-formatted-reply / NB-long-bridge-replies | Omar | Paste Tour | Pending | | |
| 261 | CH-network-admin-lifecycle | J-administer-network-live / NB-network-availability-toggle | Bruno | Multi-Tab Tour | Pending | | |
| 262 | CH-network-local-default | J-network-local-default / NB-network-empties-onboarding-settings | Nia | Feature Tour | Pending | | |
| 263 | CH-network-admin-lifecycle | J-administer-network-live / NB-network-live-config-lifecycle | Bruno | Multi-Tab Tour | Pending | | |
| 264 | CH-untested-valid-022-network-local-default-bruno | J-network-local-default / NB-participation-controls-serialize | Bruno | Network Tour | Pending | | |
| 265 | CH-bridge-progress-stress | J-watch-agent-work-channel / NB-provider-progress-rendering | Maya | Garbage Tour | Pending | | |
| 266 | CH-live-bounds-agent-path | J-run-bounded-live-collaboration / NB-run-bounded-live-collaboration | Ada | Interrupt Tour | Pending | | |
| 267 | CH-coordination-future-runs | J-enable-coordinated-conversations / NB-run-conversation-bounds-usage | Bruno | Back-Button Tour | Pending | | |
| 268 | CH-web-bridge-setup | J-complete-web-bridge-setup / NB-web-bridge-setup | Tessa | Back-Button Tour | Pending | | |
| 269 | CH-untested-022-23-dora | J-23 / NB-web-channel-fanout-policy | Dora | Network Tour | Pending | | |
| 270 | CH-untested-023-23-theo | J-23 / NB-web-network-head-trail | Théo | Network Tour | Pending | | |
| 271 | CH-untested-057-evaluate-compozy-beta-dora | J-evaluate-compozy-beta / REL-beta-install-paths | Dora | Feature Tour | Pending | | |
| 272 | CH-untested-057-evaluate-compozy-beta-dora | J-evaluate-compozy-beta / REL-beta-installer-provenance | Dora | Feature Tour | Pending | | |
| 273 | CH-untested-057-evaluate-compozy-beta-dora | J-evaluate-compozy-beta / REL-beta-self-update | Dora | Feature Tour | Pending | | |
| 274 | CH-compozy-landing-canary | J-evaluate-compozy-beta / REL-os-landing-proof | Cora | Feature Tour | Pending | | |
| 275 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-release-candidate-plan | Dora | Garbage Tour | Pending | | |
| 276 | CH-untested-049-approve-compozy-beta-candidate-dora | J-approve-compozy-beta-candidate / REL-stable-changelog-hard-cut | Dora | Feature Tour | Pending | | |
| 277 | CH-daemon-schema-parity | J-operate-daemon-schema / RT-001 | Ada | Feature Tour | Pass | | |
| 278 | CH-untested-valid-025-operate-daemon-schema-dora | J-operate-daemon-schema / RT-002 | Dora | Garbage Tour | Pending | The Doctor filter and parity matrix passed, but the available log-tail item omitted structured evidence before the staged root fix. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/025-runtime-doctor |
| 279 | CH-030 | J-19 / RT-004 | Lea | Feature Tour | Pending | | |
| 280 | CH-untested-072-retire-workspace-bruno | J-retire-workspace / RT-008 | Bruno | Back-Button Tour | Pending | | |
| 281 | CH-028 | J-17 / RT-010 | Bruno | Feature Tour | Pending | | |
| 282 | CH-untested-011-11-theo | J-11 / RT-011 | Théo | Network Tour | Pass | Backend cursor/filter/health/type/error coverage passed; Home and Agent metrics matched daemon totals, and the real 50→51 cursor continuation removed its Load more control without browser errors. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/030-session-catalog; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/036-agent-catalog-ui |
| 283 | CH-untested-valid-003-12-theo | J-12 / RT-012 | Théo | Network Tour | Pass | Scoped/direct snapshots, optional health, invalid boolean, and missing IDs matched across HTTP/UDS after normalizing only the health freshness timestamp; the canonical web route opened without errors. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/038-agent-detail-deep-links; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/039-session-snapshot |
| 284 | CH-016 | J-13 / RT-013 | Théo | Multi-Tab Tour | Pending | BUG-20260729-session-window-cross-tab-focus | Stop/history passed; cross-tab session focus failed and awaits root fix. |
| 285 | CH-untested-016-15-theo | J-15 / RT-014 | Théo | Feature Tour | Pass | Cancel preserved the stopped session; confirmed delete removed detail, history, catalog membership, and stale Web routes. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/047-session-delete-attach |
| 286 | CH-014 | J-11 / RT-015 | Théo | Interrupt Tour | Pending | BUG-20260729-session-window-cross-tab-focus; BUG-20260729-session-attach-openapi-ttl | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/047-session-delete-attach |
| 287 | CH-untested-valid-005-14-theo | J-14 / RT-017 | Théo | Feature Tour | Pending | | |
| 288 | CH-016 | J-13 / RT-018 | Théo | Multi-Tab Tour | Pending | | |
| 289 | CH-016 | J-13 / RT-019 | Théo | Multi-Tab Tour | Pending | | |
| 290 | CH-untested-valid-018-answer-agent-requests-theo | J-answer-agent-requests / RT-021 | Théo | Feature Tour | Pending | | |
| 291 | CH-untested-valid-004-14-ada | J-14 / RT-022 | Ada | Feature Tour | Pass | Fresh real-session events, grouped history, transcript, and recap reads were bounded across HTTP/UDS; transcript paging was gap-free via before_sequence, unsupported cursors/limits returned 400, and cross-workspace reads returned 404. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/020-session-contracts |
| 292 | CH-014 | J-11 / RT-024 | Théo | Interrupt Tour | Pass | HTTP/UDS/CLI health, status, inspect, digest, and 400/404 branches passed; the web inspector rendered truthful Trace/Usage empties plus exact Memory lineage and eight persisted events. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/032-session-health-inspect; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/034-session-inspector |
| 293 | CH-untested-020-22-dora | J-22 / RT-026 | Dora | Back-Button Tour | Pass | Public config lifecycle installed an isolated native auth probe; HTTP/UDS/CLI classified it authenticated without leaking the daemon sentinel, while auth-none returned 200/no subprocess and missing command returned typed 422. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/021-provider-auth-probe |
| 294 | CH-untested-valid-014-31-bruno | J-31 / RT-028 | Bruno | Back-Button Tour | Pending | The bundled explorer installer ignored COMPOZY_HOME and its legacy frontmatter failed strict discovery; the staged root fix is green across HTTP, UDS, CLI, and web. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/033-agent-catalog |
| 295 | CH-untested-valid-015-32-ada | J-32 / RT-029 | Ada | Back-Button Tour | Pass | Five global/workspace AGENT.md definitions persisted the canonical runtime fields across HTTP, UDS, CLI, and native create; all strict negatives returned 400 without residue, and cleanup removed every fixture. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/027-agent-authoring |
| 296 | CH-untested-039-32-ada | J-32 / RT-030 | Ada | Back-Button Tour | Pending | | |
| 297 | CH-untested-039-32-ada | J-32 / RT-032 | Ada | Back-Button Tour | Pending | | |
| 298 | CH-untested-040-32-bruno | J-32 / RT-036 | Bruno | Back-Button Tour | Pass | HTTP/UDS validation, CAS writes, stale rejection, history, byte-exact rollback, semantic status parity, dry-run wake without an extra prompt, and complete cleanup passed. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/029-heartbeat-lifecycle |
| 299 | CH-untested-062-manage-sandbox-profiles-dora | J-manage-sandbox-profiles / RT-037 | Dora | Feature Tour | Pending | | |
| 300 | CH-untested-011-11-theo | J-11 / RT-039 | Théo | Network Tour | Pending | | |
| 301 | CH-015 | J-12 / RT-040 | Nia | Network Tour | Pass | The public `/session/$id` permalink replaced to the canonical agent route; missing ID showed a clear 404 toast and the desktop settled without an indefinite spinner. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/042-session-permalink |
| 302 | CH-014 | J-11 / RT-041 | Théo | Interrupt Tour | Pending | | |
| 303 | CH-014 | J-11 / RT-043 | Théo | Interrupt Tour | Pending | | |
| 304 | CH-untested-valid-003-12-theo | J-12 / RT-044 | Théo | Network Tour | Pending | | |
| 305 | CH-014 | J-11 / RT-045 | Théo | Interrupt Tour | Pending | | |
| 306 | CH-015 | J-12 / RT-046 | Nia | Network Tour | Pending | | |
| 307 | CH-021 | J-14 / RT-047 | Rafa | Garbage Tour | Pending | | |
| 308 | CH-017 | J-14 / RT-048 | Rafa | Feature Tour | Pending | | |
| 309 | CH-018 | J-15 / RT-050 | Ada | Feature Tour | Pass | A controlled slow ACP turn kept detail/events/history/transcript readable while active, in observed stopping state, and after stopped finalization across HTTP/UDS; no recorder-unavailable branch appeared. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/020-session-contracts |
| 310 | CH-018 | J-15 / RT-051 | Ada | Feature Tour | Pending | | |
| 311 | CH-021 | J-14 / RT-052 | Rafa | Garbage Tour | Pending | | |
| 312 | CH-017 | J-14 / RT-055 | Rafa | Feature Tour | Pending | | |
| 313 | CH-017 | J-14 / RT-056 | Rafa | Feature Tour | Pending | | |
| 314 | CH-016 | J-13 / RT-058 | Théo | Multi-Tab Tour | Pending | | |
| 315 | CH-016 | J-13 / RT-059 | Théo | Multi-Tab Tour | Pending | | |
| 316 | CH-032 | J-21 / RT-061 | Bruno | Feature Tour | Pending | The active acpmock can record ordered config/prompt RPCs but advertises neither Claude `max` nor Codex explicit `none`; no substitute profile was treated as proof. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/028-reasoning-preflight |
| 317 | CH-028 | J-17 / RT-063 | Bruno | Feature Tour | Pending | | |
| 318 | CH-028 | J-17 / RT-064 | Bruno | Feature Tour | Pending | | |
| 319 | CH-028 | J-17 / RT-066 | Bruno | Feature Tour | Pending | | |
| 320 | CH-028 | J-17 / RT-067 | Bruno | Feature Tour | Pending | | |
| 321 | CH-034 | J-17 / RT-068 | Sol | Feature Tour | Pending | | |
| 322 | CH-untested-valid-014-31-bruno | J-31 / RT-069 | Bruno | Back-Button Tour | Pending | | |
| 323 | CH-030 | J-19 / RT-071 | Lea | Feature Tour | Pending | | |
| 324 | CH-032 | J-21 / RT-072 | Bruno | Feature Tour | Pending | | |
| 325 | CH-untested-valid-013-30-bruno | J-30 / RT-074 | Bruno | Feature Tour | Pending | | |
| 326 | CH-untested-valid-013-30-bruno | J-30 / RT-075 | Bruno | Feature Tour | Pending | | |
| 327 | CH-untested-valid-014-31-bruno | J-31 / RT-076 | Bruno | Back-Button Tour | Pass | Valid direct tab/file/filter restoration, default normalization, Settings overlay, close-to-detail, and the active session child route passed without writes or browser errors. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/038-agent-detail-deep-links |
| 328 | CH-untested-valid-014-31-bruno | J-31 / RT-077 | Bruno | Back-Button Tour | Pending | BUG-20260729-heartbeat-status-stale-eligibility; BUG-20260729-heartbeat-wake-rollback-stale-policy | Authored lifecycle and repaired live replays are green; both root fixes await governed commits. |
| 329 | CH-untested-valid-014-31-bruno | J-31 / RT-078 | Bruno | Back-Button Tour | Pending | | |
| 330 | CH-untested-valid-015-32-ada | J-32 / RT-079 | Ada | Back-Button Tour | Pending | | |
| 331 | CH-untested-valid-015-32-ada | J-32 / RT-080 | Ada | Back-Button Tour | Pending | | |
| 332 | CH-untested-036-30-ada | J-30 / RT-083 | Ada | Feature Tour | Pending | | |
| 333 | CH-untested-037-31-bruno | J-31 / RT-agent-detail-runtime-live-edit | Bruno | Back-Button Tour | Pending | | |
| 334 | CH-untested-037-31-bruno | J-31 / RT-agent-overview-canonical-metrics | Bruno | Back-Button Tour | Pass | Overview matched the daemon aggregate exactly: 0 active of 8, 27m 9s runtime, 5 failed, and canonical last activity; Failed rendered five matching rows. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/038-agent-detail-deep-links |
| 335 | CH-untested-valid-027-validate-compozy-hard-cut-bruno | J-validate-compozy-hard-cut / RT-compozy-global-database | Bruno | Feature Tour | Pending | | |
| 336 | CH-cursor-agent-mode | J-17 / RT-cursor-agent-mode | Bruno | Feature Tour | Pending | | |
| 337 | CH-drain-without-loss | J-drain-daemon-safely / RT-daemon-drain-admission | Dora | Interrupt Tour | Pending | | |
| 338 | CH-untested-045-administer-window-manager-theo | J-administer-window-manager / RT-desktop-pager-overview | Théo | Back-Button Tour | Pending | | |
| 339 | CH-untested-066-operate-daemon-schema-ada | J-operate-daemon-schema / RT-dev-bootstrap-ready | Ada | Garbage Tour | Pending | | |
| 340 | CH-untested-069-operate-home-dashboard-cora | J-operate-home-dashboard / RT-home-approve-from-dashboard | Cora | Feature Tour | Pending | | |
| 341 | CH-untested-069-operate-home-dashboard-cora | J-operate-home-dashboard / RT-home-dashboard-zones | Cora | Feature Tour | Pass | Seven ordered zones, truthful empty/degraded states, daemon-owned usage, head action, and body heading contract passed against normalized HTTP/UDS overview truth. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/037-home-dashboard |
| 342 | CH-untested-069-operate-home-dashboard-cora | J-operate-home-dashboard / RT-home-usage-window-persistence | Cora | Feature Tour | Pass | 7/30/90 requests, retention footnotes, unknown-cost omission, and 90d plus expanded-System reload persistence passed; browser-local state was restored. | /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/037-home-dashboard |
| 343 | CH-untested-valid-023-offer-runnable-capabilities-dora | J-offer-runnable-capabilities / RT-mcp-dead-recovery | Dora | Feature Tour | Pending | | |
| 344 | CH-untested-066-operate-daemon-schema-ada | J-operate-daemon-schema / RT-migrate-memory-stream-when-disabled | Ada | Garbage Tour | Pending | | |
| 345 | CH-new-session-latency-title | J-17 / RT-new-session-fast-feedback | Bruno | Network Tour | Pending | | |
| 346 | CH-untested-066-operate-daemon-schema-ada | J-operate-daemon-schema / RT-observe-overview-cli | Ada | Garbage Tour | Pending | BUG-20260729-overview-json-parity — fix staged, governed commit pending | |
| 347 | CH-untested-019-19-lea | J-19 / RT-onboarding-setup-panel-over-shell | Lea | Feature Tour | Pending | | |
| 348 | CH-untested-067-operate-daemon-schema-bruno | J-operate-daemon-schema / RT-preserve-corrupt-database-family | Bruno | Garbage Tour | Pending | | |
| 349 | CH-crash-resume-compaction | J-11 / RT-pressure-context-compaction | Théo | Interrupt Tour | Pending | | |
| 350 | CH-database-refusal-recovery | J-operate-daemon-schema / RT-refuse-ahead-database | Bruno | Garbage Tour | Pending | | |
| 351 | CH-untested-067-operate-daemon-schema-bruno | J-operate-daemon-schema / RT-refuse-cross-stream-legacy-marker | Bruno | Garbage Tour | Pending | | |
| 352 | CH-untested-valid-024-operate-daemon-schema-ada | J-operate-daemon-schema / RT-refuse-legacy-cli-open | Ada | Garbage Tour | Pending | | |
| 353 | CH-database-refusal-recovery | J-operate-daemon-schema / RT-refuse-legacy-database | Bruno | Garbage Tour | Pending | | |
| 354 | CH-untested-067-operate-daemon-schema-bruno | J-operate-daemon-schema / RT-refuse-legacy-session-database | Bruno | Garbage Tour | Pending | | |
| 355 | CH-reserved-builtin-name-sweep | J-32 / RT-reserved-builtin-agent-names | Ada | Garbage Tour | Pending | | |
| 356 | CH-secret-redaction-sweep | J-keep-secrets-contained / RT-secret-redaction-boundary | Dora | Garbage Tour | Pending | | |
| 357 | CH-clarify-answer-roundtrip | J-answer-agent-requests / RT-session-clarification-roundtrip | Théo | Feature Tour | Pending | | |
| 358 | CH-crash-resume-compaction | J-11 / RT-session-context-rebuild | Théo | Interrupt Tour | Pending | | |
| 359 | CH-truthful-cost-provenance | J-14 / RT-session-cost-provenance | Rafa | Money Tour | Pending | | |
| 360 | CH-session-affordances-truth | J-11 / RT-session-cwd-resume | Théo | Feature Tour | Pending | | |
| 361 | CH-untested-valid-002-11-bruno | J-11 / RT-session-delete-owned-history | Bruno | Network Tour | Pending | | |
| 362 | CH-session-affordances-truth | J-11 / RT-session-lifecycle-affordances | Théo | Feature Tour | Pending | | |
| 363 | CH-subprocess-health-recovery | J-diagnose-task-session-health / RT-subprocess-health-escalation | Ada | Feature Tour | Pending | | |
| 364 | CH-background-session-switch | J-11 / RT-workspace-active-session-badge | Théo | Interrupt Tour | Pending | | |
| 365 | CH-untested-056-evaluate-compozy-beta-bruno | J-evaluate-compozy-beta / SITE-changelog-release-receipts | Bruno | Feature Tour | Pending | | |
| 366 | CH-untested-valid-022-network-local-default-bruno | J-network-local-default / TA-001 | Bruno | Network Tour | Pending | | |
| 367 | CH-038 | J-24 / TA-002 | Bruno | Feature Tour | Pending | | |
| 368 | CH-untested-024-24-bruno-part-1 | J-24 / TA-003 | Bruno | Garbage Tour | Pending | | |
| 369 | CH-untested-valid-022-network-local-default-bruno | J-network-local-default / TA-004 | Bruno | Network Tour | Pending | | |
| 370 | CH-untested-024-24-bruno-part-1 | J-24 / TA-017 | Bruno | Garbage Tour | Pending | | |
| 371 | CH-untested-024-24-bruno-part-1 | J-24 / TA-018 | Bruno | Garbage Tour | Pending | | |
| 372 | CH-untested-024-24-bruno-part-1 | J-24 / TA-019 | Bruno | Garbage Tour | Pending | | |
| 373 | CH-untested-024-24-bruno-part-1 | J-24 / TA-022 | Bruno | Garbage Tour | Pending | | |
| 374 | CH-untested-024-24-bruno-part-1 | J-24 / TA-023 | Bruno | Garbage Tour | Pending | | |
| 375 | CH-untested-050-bound-runaway-work-ada | J-bound-runaway-work / TA-024 | Ada | Feature Tour | Pending | | |
| 376 | CH-untested-024-24-bruno-part-1 | J-24 / TA-027 | Bruno | Garbage Tour | Pending | | |
| 377 | CH-untested-024-24-bruno-part-1 | J-24 / TA-033 | Bruno | Garbage Tour | Pending | | |
| 378 | CH-untested-027-24-marina | J-24 / TA-039 | Marina | Garbage Tour | Pending | | |
| 379 | CH-untested-valid-011-24-marina | J-24 / TA-040 | Marina | Garbage Tour | Pending | | |
| 380 | CH-untested-025-24-bruno-part-2 | J-24 / TA-044 | Bruno | Garbage Tour | Pending | | |
| 381 | CH-untested-025-24-bruno-part-2 | J-24 / TA-047 | Bruno | Garbage Tour | Pending | | |
| 382 | CH-untested-050-bound-runaway-work-ada | J-bound-runaway-work / TA-050 | Ada | Feature Tour | Pending | | |
| 383 | CH-038 | J-24 / TA-052 | Bruno | Feature Tour | Pending | | |
| 384 | CH-038 | J-24 / TA-054 | Bruno | Feature Tour | Pending | | |
| 385 | CH-schedule-recovery-guard | J-24 / TA-055 | Bruno | Interrupt Tour | Pending | | |
| 386 | CH-038 | J-24 / TA-056 | Bruno | Feature Tour | Pending | | |
| 387 | CH-untested-026-24-dora | J-24 / TA-061 | Dora | Garbage Tour | Pending | | |
| 388 | CH-untested-026-24-dora | J-24 / TA-062 | Dora | Garbage Tour | Pending | | |
| 389 | CH-untested-010-09-ada | J-09 / TA-063 | Ada | Feature Tour | Pending | | |
| 390 | CH-untested-010-09-ada | J-09 / TA-064 | Ada | Feature Tour | Pending | | |
| 391 | CH-untested-valid-009-24-ada | J-24 / TA-065 | Ada | Garbage Tour | Pending | | |
| 392 | CH-untested-010-09-ada | J-09 / TA-066 | Ada | Feature Tour | Pending | | |
| 393 | CH-untested-008-07-ada | J-07 / TA-067 | Ada | Feature Tour | Pass | 2026-07-29 | HTTP/UDS catalog CRUD, filters, pagination, errors, CAS, lint, and two-workspace isolation passed. |
| 394 | CH-untested-008-07-ada | J-07 / TA-068 | Ada | Feature Tour | Pending | | |
| 395 | CH-untested-008-07-ada | J-07 / TA-069 | Ada | Feature Tour | Pending | BUG-20260729-loop-sidecar-lifecycle | Fresh rebuilt-candidate HTTP/UDS replay passed; governed fix commit is still pending. |
| 396 | CH-untested-008-07-ada | J-07 / TA-070 | Ada | Feature Tour | Pending | | |
| 397 | CH-untested-013-13-bruno | J-13 / TA-071 | Bruno | Network Tour | Pending | | |
| 398 | CH-untested-009-07-bruno | J-07 / TA-072 | Bruno | Feature Tour | Pass | 2026-07-29 | All 24 Loop OpenAPI/TS operations, statuses, enums, defaults, envelopes, events, and automation additions passed fresh drift/spec/typecheck gates. |
| 399 | CH-untested-010-09-ada | J-09 / TA-073 | Ada | Feature Tour | Pass | 2026-07-29 | HTTP/UDS create/update/list/error parity and delegated Loop-run linkage passed with exact no-residue cleanup. |
| 400 | CH-untested-010-09-ada | J-09 / TA-074 | Ada | Feature Tour | Pass | 2026-07-29 | Exact native toolset/availability, filtered counted pages, rich run receipts, scope denial, parity, and cleanup passed. |
| 401 | CH-untested-008-07-ada | J-07 / TA-075 | Ada | Feature Tour | Pass | BUG-20260729-loop-native-error-semantics | Governed fix `103192e4`; rebuilt native/HTTP/UDS CAS and read-only-source replay passed without mutation. |
| 402 | CH-untested-008-07-ada | J-07 / TA-076 | Ada | Feature Tour | Pass | BUG-20260729-loop-native-error-semantics; BUG-20260729-loop-resume-restart-stuck | Governed fix `103192e4`; native run/dry-run/status/runs/Pause/Resume/Stop, restart recovery, and isolation passed after repair. |
| 403 | CH-untested-008-07-ada | J-07 / TA-077 | Ada | Feature Tour | Pending | | |
| 404 | CH-untested-008-07-ada | J-07 / TA-078 | Ada | Feature Tour | Pending | | |
| 405 | CH-untested-008-07-ada | J-07 / TA-079 | Ada | Feature Tour | Pending | | |
| 406 | CH-untested-001-01-ada | J-01 / TA-080 | Ada | Feature Tour | Pending | | |
| 407 | CH-untested-001-01-ada | J-01 / TA-081 | Ada | Feature Tour | Pending | | |
| 408 | CH-untested-002-01-bruno | J-01 / TA-082 | Bruno | Feature Tour | Pending | | |
| 409 | CH-untested-002-01-bruno | J-01 / TA-083 | Bruno | Feature Tour | Pending | | |
| 410 | CH-untested-013-13-bruno | J-13 / TA-084 | Bruno | Network Tour | Pending | | |
| 411 | CH-untested-005-05-bruno | J-05 / TA-085 | Bruno | Back-Button Tour | Pending | | |
| 412 | CH-untested-007-06-bruno | J-06 / TA-086 | Bruno | Back-Button Tour | Pending | | |
| 413 | CH-untested-032-27-marina-part-1 | J-27 / TA-087 | Marina | Garbage Tour | Pending | | |
| 414 | CH-untested-032-27-marina-part-1 | J-27 / TA-088 | Marina | Garbage Tour | Pending | | |
| 415 | CH-untested-032-27-marina-part-1 | J-27 / TA-089 | Marina | Garbage Tour | Pending | | |
| 416 | CH-untested-032-27-marina-part-1 | J-27 / TA-090 | Marina | Garbage Tour | Pending | | |
| 417 | CH-untested-032-27-marina-part-1 | J-27 / TA-091 | Marina | Garbage Tour | Pending | | |
| 418 | CH-untested-034-28-bruno | J-28 / TA-092 | Bruno | Feature Tour | Pending | | |
| 419 | CH-untested-029-26-bruno | J-26 / TA-093 | Bruno | Feature Tour | Pending | | |
| 420 | CH-untested-030-26-lea | J-26 / TA-094 | Lea | Feature Tour | Pending | | |
| 421 | CH-untested-030-26-lea | J-26 / TA-095 | Lea | Feature Tour | Pending | | |
| 422 | CH-untested-035-29-ada | J-29 / TA-096 | Ada | Feature Tour | Pending | | |
| 423 | CH-untested-035-29-ada | J-29 / TA-097 | Ada | Feature Tour | Pending | | |
| 424 | CH-untested-035-29-ada | J-29 / TA-098 | Ada | Feature Tour | Pending | | |
| 425 | CH-untested-032-27-marina-part-1 | J-27 / TA-099 | Marina | Garbage Tour | Pending | | |
| 426 | CH-untested-032-27-marina-part-1 | J-27 / TA-100 | Marina | Garbage Tour | Pending | | |
| 427 | CH-untested-032-27-marina-part-1 | J-27 / TA-101 | Marina | Garbage Tour | Pending | | |
| 428 | CH-untested-031-27-bruno | J-27 / TA-102 | Bruno | Garbage Tour | Pending | | |
| 429 | CH-untested-032-27-marina-part-1 | J-27 / TA-103 | Marina | Garbage Tour | Pending | | |
| 430 | CH-untested-030-26-lea | J-26 / TA-104 | Lea | Feature Tour | Pending | | |
| 431 | CH-untested-032-27-marina-part-1 | J-27 / TA-105 | Marina | Garbage Tour | Pending | | |
| 432 | CH-untested-033-27-marina-part-2 | J-27 / TA-106 | Marina | Garbage Tour | Pending | | |
| 433 | CH-untested-015-14-marina | J-14 / TA-107 | Marina | Feature Tour | Pending | | |
| 434 | CH-runaway-work-bounded | J-bound-runaway-work / TA-action-run-liveness | Ada | Garbage Tour | Pending | | |
| 435 | CH-automation-crud-loop-target | J-24 / TA-automation-crud-loop-target | Bruno | Garbage Tour | Pending | | |
| 436 | CH-suggestions-consent | J-24 / TA-automation-suggestions | Bruno | Feature Tour | Pending | | |
| 437 | CH-schedule-recovery-guard | J-24 / TA-daemon-lifecycle-command-guard | Bruno | Interrupt Tour | Pending | | |
| 438 | CH-runaway-work-bounded | J-bound-runaway-work / TA-exact-claim-single-owner | Ada | Garbage Tour | Pending | | |
| 439 | CH-runaway-work-bounded | J-bound-runaway-work / TA-lease-recovery-attempt-budget | Ada | Garbage Tour | Pending | | |
| 440 | CH-runaway-work-bounded | J-bound-runaway-work / TA-loop-failure-breaker | Ada | Garbage Tour | Pending | | |
| 441 | CH-task-tree-loop-rollup | J-complete-task-tree / TA-parent-rollup-completion | Bruno | Feature Tour | Pending | | |
| 442 | CH-schedule-recovery-guard | J-24 / TA-schedule-catchup-overlap | Bruno | Interrupt Tour | Pending | | |
| 443 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-task-create-async-activation | Bruno | Feature Tour | Pending | | |
| 444 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-task-role-session-activation | Bruno | Feature Tour | Pending | | |
| 445 | CH-untested-valid-010-24-bruno | J-24 / TA-task-run-cost-provenance | Bruno | Garbage Tour | Pending | | |
| 446 | CH-task-template-draft | J-complete-task-tree / TA-task-template-preserves-draft | Bruno | Back-Button Tour | Pending | | |
| 447 | CH-wake-dedup-stress | J-operate-bounded-task-capacity / TA-task-wake-dedup | Ada | Garbage Tour | Pending | | |
| 448 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-terminal-run-inspect | Bruno | Feature Tour | Pending | | |
| 449 | CH-untested-026-24-dora | J-24 / TA-web-automation-preview-toggle | Dora | Garbage Tour | Pending | | |
| 450 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-web-task-detail-redesign | Bruno | Feature Tour | Pending | | |
| 451 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-web-task-run-detail-redesign | Bruno | Feature Tour | Pending | | |
| 452 | CH-workspace-run-capacity | J-operate-bounded-task-capacity / TA-workspace-run-capacity | Ada | Feature Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-daemon-schema-parity — Ada

- **Ran:** 2026-07-29T02:50:44Z → 2026-07-29T02:50:44Z (box respected: yes)
- **Findings:** Fresh CLI, HTTP, and UDS status reads agreed on daemon identity and ordered global/memory schema streams; current Network truth was enabled/ready and the payload exposed no credential values.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-001 → pass
- **Paper cuts:** none
- **Surprises:** The historical charter expected both stream versions to be 1; the current declarative schema correctly reports global v27 and memory v1, so the scenario's parity invariant—not the obsolete literal version—owned the verdict.
- **Suggested next charter:** CH-untested-066-operate-daemon-schema-ada for overview and negative schema controls.

### CH-untested-066-operate-daemon-schema-ada — Ada — overview slice

- **Ran:** 2026-07-29T02:58:09Z → 2026-07-29T03:03:18Z (box respected: yes)
- **Findings:** JSONL emitted all nine required lines, human output rendered every promised section, 7/30/90-day windows succeeded, and invalid 14-day input returned 422 over HTTP and UDS. The JSON payload differed only on the CLI-only `resolution_source` field.
- **Bugs filed/updated:** BUG-20260729-overview-json-parity
- **Scenarios settled:** none — the staged fix replay is green, but the governed commit and original-persona verification remain pending
- **Paper cuts:** none
- **Surprises:** HTTP and UDS were byte-identical across every structured read endpoint sampled in the same batch.
- **Suggested next charter:** continue CH-untested-066-operate-daemon-schema-ada with disabled-memory migration and development-readiness cases.

### CH-untested-058-extension-policy-admin-ada — Ada

- **Ran:** 2026-07-29T03:07:25Z → 2026-07-29T03:07:26Z (box respected: yes)
- **Findings:** CLI, native tool, HTTP, and UDS all exposed the active `dev-cycle` extension as healthy and running. HTTP and UDS were byte-identical.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-022 → pass
- **Paper cuts:** none
- **Surprises:** The nonexistent-extension branch returned stable not-found diagnostics through all public reads and left the healthy projection byte-identical before/after.
- **Suggested next charter:** CH-untested-059-extension-policy-admin-bruno for bundle and lifecycle controls.

### CH-untested-046-agent-marketplace-parity-ada — Ada

- **Ran:** 2026-07-29T03:09:41Z → 2026-07-29T03:17:30Z (box respected: yes)
- **Findings:** Filtered resource list/get worked through the operator UDS, CLI, and native tools; the unconfigured HTTP operator route remained intentionally absent. A registered `automation.job` proved canonical create/update, 409 conflict, 422 validation, live reconciliation, deletion, and clean native snapshot. HTTP and UDS exposed the same 215 complete tool projections and 90 hook events where both transports are public.
- **Bugs filed/updated:** none; BUG-0009's historical codec-validation fix passed its fresh replay.
- **Scenarios settled:** ET-031, ET-032, ET-033, ET-035, ET-036, ET-042 → pass
- **Paper cuts:** none
- **Surprises:** The historical resource charter used the protected bundle kind and an obsolete singular native-tool spelling; current public inventory exposes `compozy__resources_list`.
- **Suggested next charter:** CH-untested-047-agent-marketplace-parity-bruno for skill authoring and hook CRUD.

### CH-untested-047-agent-marketplace-parity-bruno — Bruno — structured registry slice

- **Ran:** 2026-07-29T03:29:09Z → 2026-07-29T03:29:10Z (box respected: yes)
- **Findings:** Tool invocation completed over HTTP, UDS, and CLI with stable invalid/unavailable errors. HTTP and UDS agreed exactly on all 27 toolsets and five workspace-clean native hooks; CLI and native list/info entry points returned the same current catalog identities.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-038, ET-040, ET-041 → pass
- **Paper cuts:** none
- **Surprises:** The current daemon exposes 27 toolsets and five catalog hooks, replacing obsolete historical counts without weakening the status and isolation invariants.
- **Suggested next charter:** continue CH-untested-047-agent-marketplace-parity-bruno with skill authoring, trust, and activation gates.

### CH-untested-047-agent-marketplace-parity-bruno — Bruno — skill scope/content backend slice

- **Ran:** 2026-07-29T03:39:55Z → 2026-07-29T03:39:56Z (box respected: yes)
- **Findings:** Two same-named workspace skills stayed isolated across HTTP, UDS, CLI, and native reads; a critical-content fixture was blocked from both catalog and content; agent disable/enable stayed scoped and cleanup restored the initial state. The missing-workspace list branch initially returned 500 and passed as 404 after the root fix was rebuilt.
- **Bugs filed/updated:** BUG-20260729-skill-workspace-error-mapping
- **Scenarios settled:** none — ET-001, ET-003, and ET-006 retain browser and/or explicit 422 work
- **Paper cuts:** none
- **Surprises:** The runtime already preserves the workspace sentinel; only the skill transport mapper discarded its status classification.
- **Suggested next charter:** continue this charter in browser mode, then add a valid malformed agent-local fixture for ET-006's 422 branch.

### CH-agent-marketplace-parity — Ada — skill detail slice

- **Ran:** 2026-07-29T03:39:55Z → 2026-07-29T03:39:56Z (box respected: yes)
- **Findings:** Effective skill detail and activation metadata matched over HTTP and UDS, CLI inspect, and native view. Two workspaces resolved distinct winners; blank and unknown identities returned 400/404.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-002 → pass
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** continue CH-agent-marketplace-parity with deterministic Marketplace search/detail.

### CH-agent-marketplace-parity — Ada — Marketplace read slice

- **Ran:** 2026-07-29T03:46:54Z → 2026-07-29T03:47:13Z (box respected: yes)
- **Findings:** Curated idle results and the remote `review` query agreed on stable order across HTTP/UDS; specialized/shared CLI namespaces and the native search resolved the same install identities. Detail for `skill_cmV2aWV3` matched over HTTP/UDS and both CLIs, with correct 400/404 negative classifications.
- **Bugs filed/updated:** none; BUG-0007's hard-cut fix passed its fresh replay.
- **Scenarios settled:** ET-007, ET-008 → pass
- **Paper cuts:** none
- **Surprises:** The specialized CLI intentionally emits `slug`, while the shared marketplace CLI emits stable `entry_id`; comparing `slug` to `install_slug` preserves the public contracts without inventing parity.
- **Suggested next charter:** CH-untested-047-agent-marketplace-parity-bruno for install visibility and cleanup.

### CH-untested-047-agent-marketplace-parity-bruno — Bruno — Marketplace install backend slice

- **Ran:** 2026-07-29T03:51:57Z → 2026-07-29T03:52:42Z (box respected: yes)
- **Findings:** A real remote skill installed through privileged HTTP and then through the specialized CLI; both paths refreshed the registry and became immediately visible over HTTP, UDS, CLI list, and CLI inspect. UDS and CLI removal each restored the clean baseline.
- **Bugs filed/updated:** none; BUG-0007's install visibility fix passed its fresh replay.
- **Scenarios settled:** none — ET-009 retains the browser install, visibility, and removal lifecycle.
- **Paper cuts:** none
- **Surprises:** The current native registry intentionally exposes skill discovery and reads but no mutation tools; structured daemon-backed CLI/UDS owns agent-manageable install and removal.
- **Suggested next charter:** continue the same charter in browser mode on the Marketplace skills route.

### CH-untested-047-agent-marketplace-parity-bruno — Bruno — Skills browser completion slice

- **Ran:** 2026-07-29T04:18:00Z → 2026-07-29T04:36:46Z (box respected: yes)
- **Findings:** The rebuilt local SPA rendered the workspace-scoped installed catalog and exact verified skill body. Agent settings applied and cleared a real tombstone immediately. A live Marketplace install became visible in the installed catalog and detail, then the typed removal flow restored the clean baseline. The explicit malformed agent fixture returned 422 and was removed without residue.
- **Bugs filed/updated:** BUG-20260729-skill-agent-default-selection; BUG-20260729-skill-workspace-error-mapping retained its staged green browser replay.
- **Scenarios settled:** ET-003, ET-006, ET-009 → pass; ET-001 and ET-012 remain pending only for governed fix commits.
- **Paper cuts:** none
- **Surprises:** The Go candidate intentionally serves the pinned `compozy-web-assets` module, so the local TypeScript repair required the manifest-targeted Vite candidate for honest browser evidence. The deterministic route capture and full web Turbo lane both passed.
- **Suggested next charter:** continue the same charter with ET-010, ET-011, and the remaining settings/policy cases.

### Skills marketplace mutation and policy slice — Bruno / Vera

- **Ran:** 2026-07-29T04:42:00Z → 2026-07-29T05:08:00Z (box respected: yes)
- **Findings:** A real installed skill passed CLI/HTTP/UDS update checks and apply, current catalog truth suppressed an inapplicable web Update action, and HTTP/UDS/web/CLI removal paths all left a clean baseline. Invalid policy values returned 400. Rapid web policy edits persisted all fields and required restart, but exposed a false dirty-state regression after duration normalization; the staged correction passed its live replay.
- **Bugs filed/updated:** BUG-20260729-skill-policy-normalized-dirty.
- **Scenarios settled:** ET-010, ET-011 → pass; ET-013 remains pending only for the governed fix commit.
- **Paper cuts:** none
- **Surprises:** Go duration normalization and an omitted optional tombstone field combined into a structural draft mismatch even though the daemon had already persisted the policy.
- **Suggested next charter:** continue J-extension-policy-admin with ET-015 through ET-023.

### Extension lifecycle and partial-update recovery slice — Ada / Bruno / Vera

- **Ran:** 2026-07-29T05:10:00Z → 2026-07-29T05:49:00Z (box respected: yes)
- **Findings:** Local and marketplace install paths, trust gates, bundle-protected removal, confirmed web removal, immediate toggles, cleanup warnings, list parity, and extension discovery ran against one isolated daemon. A two-extension native all-update committed the first target and stopped at the invalid second target while preserving its event and filesystem state.
- **Bugs filed/updated:** BUG-20260729-extension-update-partial-error. The rebuilt candidate now returns typed validation plus the committed update and cleanup warning in `partial_result`.
- **Scenarios settled:** ET-015, ET-016, ET-017, ET-020, ET-021 → pass; ET-019 remains pending only for the governed fix commit. ET-018 retains the browser Install action.
- **Paper cuts:** none
- **Surprises:** The service and event store already preserved partial progress; only the native adapter discarded it. The archive identity mismatch was also emitted as untyped prose, so the repair classified it as manifest validation at the domain boundary.
- **Suggested next charter:** complete the catalog-unavailable branches in a fresh secondary runtime, then resume the browser marketplace detail action.

### Marketplace unavailable recovery slice — Ada / Vera

- **Ran:** 2026-07-29T05:59:28Z → 2026-07-29T06:03:00Z (box respected: yes)
- **Findings:** A fresh secondary runtime with an unreachable loopback catalog returned 503 over HTTP and UDS; the daemon-backed CLI exited 69 with the same unavailable classification. The secondary lab teardown reported `clean: true`. The positive extension list and discovery paths had already agreed across public surfaces in the primary lab.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-015, ET-016 → pass.
- **Paper cuts:** none
- **Surprises:** External HTTP correctly masked the internal fetch chain while the local UDS and CLI retained it; the status contract remained identical.
- **Suggested next charter:** resume the extension detail browser action, then continue the remaining extension policy cases.

### Bundle lifecycle and Live requirement slice — Bruno / Ada

- **Ran:** 2026-07-29T06:11:00Z → 2026-07-29T06:18:00Z (box respected: yes)
- **Findings:** A managed three-profile extension fixture proved catalog/preview parity, non-persisting dry run, explicit Live confirmation, agent-conflict and missing-agent failures, versioned reads, stale-write protection, reapply, confirmation refresh, declaration-only network settings, and clean deactivation. HTTP/UDS payloads matched exactly wherever both transports exposed the same operation; CLI and native tools completed independent activation/deactivation cycles.
- **Bugs filed/updated:** historical bundle fixes now point to their reachable `8eeb8a38` commit; no new bug.
- **Scenarios settled:** ET-024 through ET-030 and overlapping NB-023 → pass.
- **Paper cuts:** none
- **Surprises:** Changed-manifest digest invalidation requires an extension update boundary, so its canonical owning race suite supplied that branch while the full stale/current update lifecycle ran against the daemon.
- **Suggested next charter:** exercise tool approval redaction and hook/session audit, then continue MCP lifecycle cases.

### Tool approval boundary slice — Marina

- **Ran:** 2026-07-29T06:24:00Z → 2026-07-29T06:27:00Z (box respected: yes)
- **Findings:** HTTP minted a scoped one-shot approval with deterministic input digest and the daemon-backed CLI produced the same binding. Conflicting scope returned the expected typed mismatch. Temporary raw responses were trashed after redaction; exact-token scans found no durable/log leak.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-037 → pass.
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** generate a session hook audit or continue settings/MCP cases that do not require a provider.

### Per-session hook audit slice — Bruno

- **Ran:** 2026-07-29T06:30:30Z → 2026-07-29T06:33:30Z (box respected: yes)
- **Findings:** A public-configured `acpmock` provider created a real active session and public CLI stop settled it. HTTP and UDS returned byte-identical audit records; CLI and `compozy__hooks_runs` exposed the same post-create and post-stop history. Missing-session input returned 400, while the same session queried through another workspace returned 404 without leaking records.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-043 → pass.
- **Paper cuts:** none
- **Surprises:** The stop lifecycle correctly emitted both observe and dream hook records for the same event.
- **Suggested next charter:** reuse the healthy isolated provider for session lifecycle, transcript, reconnect, and provider-runtime cases.

### Session transport and stopping-read slice — Ada / Théo

- **Ran:** 2026-07-29T06:36:23Z → 2026-07-29T06:49:00Z (box respected: yes)
- **Findings:** Real HTTP and UDS creates reached the configured ACP mock, preserved Local participation, and rejected invalid workspace admission without residue. Bounded events/history/transcript/recap reads passed with gap-free tail-first paging and 404 workspace isolation. A controlled slow turn kept every persisted read available through observed `stopping` and final `stopped` states. Busy input produced durable queue/stage results, interrupt canceled all three queued entries, and explicit approval plus denial completed through UDS/CLI with stable negative responses.
- **Bugs filed/updated:** none. RT-018's stale prompt-in-progress 409 phrase was aligned with the current durable default-queue contract already owned by RT-019 and the composer scenarios.
- **Scenarios settled:** RT-022, RT-050 → pass. Backend portions of RT-010/011/012/013/014/015/017/018/019/021/024/051 are green and remain pending their browser-owned steps.
- **Paper cuts:** none
- **Surprises:** Health/recap transport bodies differ only in request-time presence/generated timestamps; persisted read payloads match exactly. A short 4-second busy fixture was excluded as inconclusive and repeated atomically with a deterministic long window.
- **Suggested next charter:** restart with a public provider auth probe command, prove lifecycle convergence across boot, then continue the pending browser session slice when remote debugging is available.

### Isolated provider auth probe slice — Dora

- **Ran:** 2026-07-29T06:49:00Z → 2026-07-29T06:53:00Z (box respected: yes)
- **Findings:** Sequential public config writes added a QA native provider with isolated environment and provider home. After one governed restart, HTTP, UDS, and CLI ran the status command, classified authenticated output, and omitted a sentinel carried by the daemon process. The `auth_mode=none`, missing status command, and unknown provider branches returned their 200/422/404 contracts.
- **Bugs filed/updated:** none for RT-026. The restart independently exposed a shutdown checkpoint/store ordering failure now under root-cause investigation.
- **Scenarios settled:** RT-026 → pass.
- **Paper cuts:** none
- **Surprises:** The provider home resolved directly at the isolated runtime's `providers/qaprobe` directory, and all public probe surfaces agreed after excluding request-time timestamp/duration fields.
- **Suggested next charter:** finish the shutdown-ordering investigation, then reuse the restarted daemon for session lifecycle convergence and remaining provider/catalog cases.

### Unified Marketplace namespace slice — Ada

- **Ran:** 2026-07-29T07:52:00Z → 2026-07-29T08:18:00Z (box respected: yes)
- **Findings:** Search, grouped browse, detail, refresh, deleted-route checks, cursor binding, authored-revision invalidation, and two-workspace native projections ran across CLI, HTTP, UDS, and native tools. The original CLI JSON path added a transport-only field; an unchanged file-catalog refetch invalidated continuation; and generic CLI tool-result sanitization erased public bundle IDs and cursors. All three staged root fixes passed their rebuilt live replays, and cleanup restored the official catalog, zero activations, and the default unverified-extension policy.
- **Bugs filed/updated:** BUG-20260729-marketplace-json-parity; BUG-20260729-marketplace-file-cursor-fence; BUG-20260729-tool-invoke-structural-redaction
- **Scenarios settled:** none — the three original target rows retain Pending until their root fixes have governed commits; ET-cli-tool-invoke-structural-handles is a new tracker-impact scenario for the next QA cycle.
- **Paper cuts:** none
- **Surprises:** Catalog freshness is not catalog identity, and defensive entropy redaction cannot safely classify daemon-authored structural handles without field context.
- **Suggested next charter:** continue CH-agent-marketplace-parity with the curated MCP install and authorization lifecycle.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/022-marketplace-namespace`

### Curated MCP install and scoped authorization slice — Ada

- **Ran:** 2026-07-29T08:58:00Z → 2026-07-29T09:18:00Z (box respected: yes)
- **Findings:** Explicit-null and typed catalog installs, validation order, locked fields, required fields, shared and install-owned Vault refs, apply/readiness separation, three-scope OAuth, automatic/manual PKCE, login alias, bounded timeout, confirmed credential change, targeted logout, redaction, and cleanup ran through CLI, HTTP, and UDS. Workspace-scoped CLI JSON added `resolution_source` outside the daemon contract. Public surfaces do not expose deterministic config/Vault/event fault injection for the remaining rollback-warning branches.
- **Bugs filed/updated:** BUG-20260729-mcp-cli-json-parity. The historical `BUG-20260715-mcp-install-null-values` branch passed its fresh retest.
- **Scenarios settled:** ET-047 → pass. ET-cli-mcp-install and ET-cli-mcp-authorize retain Pending until the structural writer fix has a governed TechSpec and commit; ET-api-mcp-catalog-install retains its fault-injection branches.
- **Paper cuts:** none
- **Surprises:** A short authorization budget can expire during the final real MCP collection probe after the callback already persisted a credential; the CLI correctly exits non-zero, and a fresh status read exposes the committed truth.
- **Suggested next charter:** continue J-mcp-authorize-repair with the raw OAuth endpoints and manual remote-operator exchange while the writer TechSpec decision is pending.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/023-mcp-catalog-install`

### Raw MCP OAuth endpoints and Paste Tour — Ada / Iris

- **Ran:** 2026-07-29T09:24:00Z → 2026-07-29T09:56:00Z (box respected: yes)
- **Findings:** Manual and automatic begin/exchange/callback/logout paths ran over HTTP, UDS, and CLI for global plus two workspace targets. Wrong state and malformed exchange input did not consume sessions; real expiry rejected stale completion; replacement/delete preserved old token records while invalidating pending work; a restarted replacement received no prior bearer; and a blocked refresh could not overwrite a newer exchange. Separate non-loopback and replacement labs both tore down with `clean: true`.
- **Bugs filed/updated:** BUG-20260729-mcp-manual-exchange-timeout. Exchange timeout originally exposed the raw UDS POST diagnostic; the rebuilt CLI now emits the stable authorization timeout used by pending input.
- **Scenarios settled:** none. ET-cli-mcp-auth-manual-exchange retains Pending until its staged root fix has a governed commit. ET-api-mcp-oauth-endpoints retains only the defensive callback-503 state that no public healthy-daemon fault owner can create.
- **Paper cuts:** none
- **Surprises:** Settings correctly reports `auth_expired` without refreshing; the public tool registry is the owner that triggers refresh. Desired config can require login while the pre-restart runtime still operates the old definition, so bearer-withholding parity required a clean restarted envelope.
- **Suggested next charter:** resume the remaining MCP browser management cases or continue the next backend runtime batch while the two TechSpec decisions are pending.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/024-mcp-oauth-endpoints`

### CH-untested-valid-025-operate-daemon-schema-dora — Dora — Doctor diagnostics

- **Ran:** 2026-07-29T10:05:00Z → 2026-07-29T10:17:28Z (box respected: yes)
- **Findings:** HTTP, UDS, and CLI agreed on the full Doctor payload after excluding timing fields. Category filtering, `quiet`, singular `provider`, structured severity counts, and `runtime.memory` passed. The available `doctor.logs.tail` item alone lacked evidence before the rebuilt fix.
- **Bugs filed/updated:** BUG-20260729-doctor-log-tail-evidence. The available branch now reports `evidence.status: "available"` like the existing unavailable branch.
- **Scenarios settled:** none — RT-002 remains Pending until the staged fix has a governed commit.
- **Paper cuts:** none
- **Surprises:** CLI Doctor exits zero even when the aggregate diagnostic status is `error`; RT-002 does not specify process-exit semantics, so no defect was inferred from that observation.
- **Suggested next charter:** continue the runtime batch with structured agent creation only if the current native caller contract is already available.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/025-runtime-doctor`

### CH-untested-valid-001-01-bruno — Bruno — dev-cycle lifecycle

- **Ran:** 2026-07-29T10:19:00Z → 2026-07-29T10:21:18Z (box respected: yes)
- **Findings:** `dev-cycle` began active with exactly two Loops, three default agents, and three extension-host tools. Disabling removed all eight projections; enabling restored the identical sets without a `watch` start source. HTTP and UDS catalog projections agreed in every state.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-052 → pass
- **Paper cuts:** none
- **Surprises:** Repeating enable restarts the extension subprocess while preserving the exact public state; the scenario promises an explicit stable state, not PID identity.
- **Suggested next charter:** RT-029 structured agent authoring, because `compozy__agent_create` is already registered, authorized, and callable in the active daemon.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/026-dev-cycle-lifecycle`

### CH-untested-valid-015-32-ada — Ada — structured agent authoring

- **Ran:** 2026-07-29T10:24:57Z → 2026-07-29T10:26:01Z (box respected: yes)
- **Findings:** HTTP workspace/global, UDS workspace, CLI workspace, and `compozy__agent_create` workspace mutations wrote five real AGENT.md definitions with `acpmock`, `gpt-5.6-sol`, `reasoning_effort: max`, and exact category/prompt fields. Unknown top-level input, invalid scope, and missing workspace returned 400 over both transports without creating files.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-029 → pass
- **Paper cuts:** none
- **Surprises:** The `acpmock` provider has no pre-session model-source rows; agent authoring correctly treats model as persisted runtime intent and does not pretend a session negotiation occurred. CLI JSON still carries the known generic `resolution_source` augmentation, which is outside this scenario's persistence invariant and remains owned by the pending structural writer TechSpec.
- **Suggested next charter:** RT-061 only if the active acpmock driver already emits enough ordered ACP configuration evidence for both Claude-max and Codex-none semantics.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/027-agent-authoring`

### CH-032 — Bruno — reasoning application preflight

- **Ran:** 2026-07-29 (preflight only; no scenario time box opened)
- **Findings:** The active acpmock fixture records ordered model/reasoning configuration and prompt RPCs, but its selectable reasoning values are only `low`, `medium`, and `high`. It cannot represent either required RT-061 branch: Claude `max` or Codex explicit `none`.
- **Bugs filed/updated:** none; this is a fixture-capability gap, not a product failure.
- **Scenarios settled:** none — RT-061 remains Pending and `untested`.
- **Paper cuts:** none
- **Surprises:** The diagnostic transport is sufficient for ordering; only the exact advertised profiles are absent.
- **Suggested next charter:** RT-036 agent heartbeat authoring and dry-run wake, which is executable without changing provider configuration.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/028-reasoning-preflight`

### CH-untested-040-32-bruno — Bruno — Heartbeat authoring and wake

- **Ran:** 2026-07-29T10:32:46Z → 2026-07-29T10:39:00Z (box respected: yes)
- **Findings:** HTTP and UDS agreed on validation and policy reads. V1/v2 CAS writes, stale rejection, two-revision history, UDS rollback, and CLI deletion preserved the managed lifecycle; rollback restored the full v1 file byte-for-byte. A healthy acpmock session was wake-eligible, all three public clients agreed on the dry-run decision, and ACP diagnostics showed no additional prompt. Temporary session IDs left the public catalog/detail and their processes stopped; forensic runtime records remain by design.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-036 → pass
- **Paper cuts:** none
- **Surprises:** An authored acpmock agent needs the fixture command explicitly; two incomplete temporary fixtures failed handshake and were cleaned before the command-backed positive run.
- **Suggested next charter:** continue the next backend Runtime case that does not require provider reconfiguration, browser access, or either pending TechSpec decision.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/029-heartbeat-lifecycle`

### CH-untested-011-11-theo — Théo — session catalog backend slice

- **Ran:** 2026-07-29 (backend-only slice; no browser time box opened)
- **Findings:** Fifteen public sessions paged across three unique cursor pages with stable totals. HTTP/UDS first-page and health-page payloads matched exactly; filters composed correctly, health hydrated only returned rows, CLI agreed, and internal type plus malformed query values returned 400 without changing the catalog.
- **Bugs filed/updated:** none; the ephemeral role-session leak did not reproduce.
- **Scenarios settled:** none — RT-011 retains its Home/Agent browser metrics and continuation UI.
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** execute ET-049 native extensibility registry while the browser remains unavailable.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/030-session-catalog`

### CH-031 — Ada — native extensibility registry

- **Ran:** 2026-07-29T10:49:24Z → 2026-07-29T11:00:57Z (box respected: yes)
- **Findings:** The active contract contained 37 native IDs: 33 skill/extension/bundle/resource/hook/tool/MCP tools plus all four provider-model tools required by J-20. Every descriptor was registered, available, callable, and schema-digested. All public invocations preserved the requested ID; read calls completed and mutation probes reached validation without persistent state. HTTP/UDS catalogs matched byte-for-byte, CLI JSON matched the same catalog, and HTTP/UDS skill invocation results agreed.
- **Bugs filed/updated:** none; the historical BUG-0010 registry-poisoning branch remained fixed because missing MCP backends did not prevent four later native reads.
- **Scenarios settled:** ET-049 → pass
- **Paper cuts:** none
- **Surprises:** The scenario's active J-20/CH-031 ownership widened the historical 33-ID inventory to 37 through the four provider-model tools. The secondary marketplace charter was not needed for settlement, but its read-only native search canary also passed.
- **Suggested next charter:** continue with another backend scenario that requires no provider restart, browser, or pending TechSpec decision.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/031-native-extensibility`

### CH-014 — Théo — session health/status/inspect backend slice

- **Ran:** 2026-07-29 (backend-only slice; no browser time box opened)
- **Findings:** HTTP, UDS, and CLI agreed on stopped/dead health, the compact status projection, inspect health, and the same configuration digest. Optional policy/wake state remained absent rather than fabricated. Invalid boolean and missing session returned exact 400/404 HTTP/UDS bodies, and fresh health reads after both negatives preserved every stable field.
- **Bugs filed/updated:** none
- **Scenarios settled:** none — RT-024 retains its named web session-inspector entry point.
- **Paper cuts:** none
- **Surprises:** Requesting recent wake events for a session with no wake history truthfully omits the empty optional collection; the current OpenAPI contract marks it optional.
- **Suggested next charter:** continue another read-only backend contract while browser access remains unavailable.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/032-session-health-inspect`

### CH-untested-valid-014-31-bruno — Bruno — authored-agent discovery and explorer repair

- **Ran:** 2026-07-29 (backend and isolated browser; browser session closed)
- **Findings:** Global/workspace HTTP, UDS, CLI, and web reads exposed the authored fleet and stable
  effective runtime fields. The shipped explorer was absent because its helper ignored the active
  Compozy home, and its legacy frontmatter could not pass the strict parser. The corrected helper
  installed one valid global explorer; list/detail, 404 negatives, browser list/detail, and zero
  duplicate diagnostics passed.
- **Bugs filed/updated:** BUG-20260729-explorer-active-home-schema
- **Scenarios settled:** none — RT-028 remains failed/Pending until the fix has governed commit
  provenance. RT-083's healthy catalog path passed, but its `sessions_available=false` branch remains
  unexecuted.
- **Paper cuts:** none
- **Surprises:** Browser-only work is now executable through a uniquely named `agent-browser` session
  without attaching to the operator Chrome; the unrelated `agh-network-qa` session remained intact.
- **Suggested next charter:** continue browser-backed Runtime partials using the same isolated-session
  pattern, or execute another backend scenario while the two TechSpec decisions remain pending.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/033-agent-catalog`

### CH-014 — Théo — session inspector completion

- **Ran:** 2026-07-29 (isolated browser; browser session closed)
- **Findings:** The direct route loaded the same stopped session used by the backend slice. The
  inspector exposed all five tabs; Trace and Usage rendered honest empty states, while Memory showed
  exact workspace/root-session lineage, lifecycle metadata, checksum, and eight persisted events.
  Browser page errors and product console errors were absent.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-024 → pass
- **Paper cuts:** none
- **Surprises:** none; the unrelated `agh-network-qa` browser session remained untouched.
- **Suggested next charter:** continue another browser-owned Runtime partial while isolated browser
  sessions remain available.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/032-session-health-inspect`; `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/034-session-inspector`

### CH-untested-011-11-theo — Théo — session catalog UI completion

- **Ran:** 2026-07-29T12:26:34Z → 2026-07-29T12:35:36Z (isolated browser; browser session closed)
- **Findings:** Home rendered the first six agents with daemon-owned catalog metrics. The
  `qa-hook-agent` detail matched 22 active, 51 total, and 26 failed sessions. Its Sessions tab rendered
  the first 50 rows, issued a real cursor request through `Load more sessions`, appended the 51st row,
  and removed the continuation control. No product or browser-console errors appeared.
- **Bugs filed/updated:** none; the accepted-start fixture race was handled independently in batch 035.
- **Scenarios settled:** RT-011 → pass
- **Paper cuts:** none
- **Surprises:** A visible continuation button below the internal scroll region required explicit
  scroll-to-view before the browser generated a pointer click. The product then issued the expected
  cursor request and converged normally.
- **Suggested next charter:** continue the next browser-owned Runtime partial with a uniquely named
  isolated session.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/030-session-catalog`; `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/036-agent-catalog-ui`

### CH-untested-069-operate-home-dashboard-cora — Cora — truthful Home and preferences

- **Ran:** 2026-07-29 (one isolated browser session; browser-local state restored)
- **Findings:** The seven-zone Home rendered in contract order with honest no-attention, no-live-work,
  no-outcome, unknown-cost, and degraded-system states. HTTP and UDS normalized to the same overview
  for 7/30/90-day windows. Each pill issued its matching request; the seven-day retention note appeared
  only for 30/90 days. Selected 90d and expanded System both survived a full reload.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-home-dashboard-zones → pass; RT-home-usage-window-persistence → pass
- **Paper cuts:** none
- **Surprises:** Persistent desktop state initially placed the active client on another desktop; the
  pager truthfully switched to the existing Home desktop before the controls were exercised.
- **Suggested next charter:** use the already-populated agent fleet for a read-only J-31 detail/deep-link batch.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/037-home-dashboard`

### CH-untested-valid-014-31-bruno — Bruno — agent detail metrics and deep links

- **Ran:** 2026-07-29 (one isolated browser session; browser session closed)
- **Findings:** The workspace aggregate and detail page agreed on 0 active of 8 sessions, 1,629
  runtime seconds, 5 failed sessions, and canonical last activity. Overview, Instructions,
  Configuration, and Sessions restored from direct validated search params; invalid values normalized
  to defaults. Settings stayed a modal over detail and closed without a write. The session child route
  made the requested session window the active OS surface while retaining background desktop windows.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-076 → pass; RT-agent-overview-canonical-metrics → pass
- **Paper cuts:** none
- **Surprises:** The OS window manager intentionally retains background windows in the DOM; session
  child replacement is expressed by the route leaf and top active surface, not by destroying other
  desktop windows.
- **Suggested next charter:** continue a read-only agent-fleet or Runtime browser batch with a uniquely
  named isolated session.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/038-agent-detail-deep-links`

### CH-untested-valid-003-12-theo — Théo — session snapshot transport parity

- **Ran:** 2026-07-29 (read-only HTTP/UDS slice plus immediately preceding browser route)
- **Findings:** Scoped snapshot bodies matched byte-for-byte over HTTP and UDS. Scoped and direct
  health snapshots matched after removing only the per-read health freshness timestamp. Invalid
  `include_health` returned matching 400 bodies and missing scoped/direct IDs returned matching 404
  bodies; a post-negative read preserved every stable field. Batch 038 opened the same canonical web
  session route without page or product-console errors.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-012 → pass
- **Paper cuts:** none
- **Surprises:** Health `updated_at` is deliberately recomputed for every read, so freshness-aware
  parity compares the remainder of the payload rather than claiming byte equality for that branch.
- **Suggested next charter:** create one disposable authored agent through the real dialog, then use it
  for the Soul/Heartbeat/Wake lifecycle and clean it through public surfaces.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/038-agent-detail-deep-links`; `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/039-session-snapshot`

### CH-untested-038-31-dora / CH-untested-valid-014-31-bruno — agent creation and authored files

- **Ran:** 2026-07-29 (one disposable authored agent, two public sessions, and one isolated browser)
- **Findings:** Create agent rendered one Simple/Advanced surface, preserved values through Simple and
  Advanced validation, omitted MCP authoring, and persisted the exact category. SOUL and HEARTBEAT
  create/update/history/restore passed. The run reproduced stale mounted-session eligibility and stale
  Wake policy selection after rollback; both repaired live replays are green.
- **Bugs filed/updated:** BUG-20260729-heartbeat-status-stale-eligibility;
  BUG-20260729-heartbeat-wake-rollback-stale-policy
- **Scenarios settled:** MS-web-agent-create-simple-advanced → pass; RT-077 → pending governed fixes
- **Paper cuts:** none
- **Surprises:** Immutable digest-deduped snapshots cannot encode activation order; the authoring
  revision is the current-policy authority after rollback.
- **Suggested next charter:** continue the smallest independent Runtime/browser batch without crossing
  either pending TechSpec boundary.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/040-agent-create-authored-files`

### CH-untested-068-operate-desktop-shell-bruno — shell shortcuts and About

- **Ran:** 2026-07-29 (one read-only desktop flow, healthy status, and an induced failed poll)
- **Findings:** Keyboard traversal opened both shell dialogs and Escape returned focus to their menu
  triggers. The shortcut registry rendered 27 actions across four sections, including every unbound
  action. About matched all eight daemon-published fields, invented no build metadata, and retained
  those rows with a truthful warning when a later status poll failed.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-web-shell-shortcuts-about-dialogs → pass
- **Paper cuts:** none
- **Surprises:** agent-browser 0.26.0 emitted `Unidentified` keyboard events in this environment;
  repository Playwright produced valid native key events and passed the same flow.
- **Suggested next charter:** continue the smallest independent browser batch without crossing either
  pending TechSpec boundary.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/041-shell-shortcuts-about-dialogs`

### CH-015 — session permalink redirect

- **Ran:** 2026-07-29 (existing and missing public permalink reads)
- **Findings:** `/session/$id` resolved a stopped session to its canonical agent-scoped route, rendered
  the session window, and was not revisited by Back. A missing ID returned 404, surfaced `Session not
  found`, kept the desktop usable, and settled to the active Extensions window without an indefinite
  spinner.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-040 → pass
- **Paper cuts:** none
- **Surprises:** The tracker had copied TanStack's internal `/_app` route ID as a public URL; the
  generated route tree and production caller both own `/session/$id`. Vite StrictMode replayed the
  missing read/toast, but the rendered surface showed one overlapping message and no page error.
- **Suggested next charter:** continue an independent route or read-only runtime batch.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/042-session-permalink`

### CH-untested-037-31-bruno — agent fleet listing grammar

- **Ran:** 2026-07-29 (one read-only isolated browser session; browser session closed)
- **Findings:** The workspace catalog returned ten unique agents split evenly between global and
  workspace origins. Rows used one ListingRow per agent, one origin pill, monospace provider/model
  facts, and trail-only status/new-session controls with no sessions Stat or provider pill. Cards used
  ten CatalogCard articles with plain Meta spans and the same catalog. Returning to Rows preserved
  order and count; console and page errors remained empty.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-web-agent-fleet-listing-rows → pass
- **Paper cuts:** none
- **Surprises:** The public catalog contained five workspace-authored agents in addition to the five
  global definitions found during exploration; the browser correctly rendered all ten.
- **Suggested next charter:** continue an independent model-catalog read batch or another read-only
  browser scenario without crossing either pending TechSpec boundary.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/043-agent-fleet-listing`

### CH-031 — Ada — model catalog read parity

- **Ran:** 2026-07-29 (one isolated COMPOZY_HOME; CLI/HTTP/UDS/native plus one cancelled browser dialog)
- **Findings:** Default and explicit curated reads returned 473 identical models on every structured
  surface. All returned 498 identical models and added exactly 25 non-curated deprecated OpenCode
  rows. Complete payload hashes matched per view. Independent cost buckets retained three observed
  shapes; OpenAI HTTP preserved sampled costs exactly and returned its typed invalid-provider error.
  The Web selector hid a deprecated row in browse, found it in search, and preserved missing rates as
  null in the production adapter.
- **Bugs filed/updated:** none
- **Scenarios settled:** MS-042 → pass; MS-045 → pass; MS-053 → pass; MS-055 → pass
- **Paper cuts:** none
- **Surprises:** The live all view already contained 25 deprecated rows, so strict-superset behavior
  was provable without curation or fixture mutation. OpenAI compatibility is intentionally HTTP-only.
- **Suggested next charter:** continue the J-20 mutation/refresh/status/config/docs rows separately;
  this read-only batch must not be reused as proof for them.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/044-model-catalog-read-parity`

### CH-031 — Ada — model catalog mutation and config lifecycle

- **Ran:** 2026-07-29 (fresh disposable COMPOZY_HOME; CLI/HTTP/UDS/native plus restart)
- **Findings:** Provider and global refresh retained successful rows beside typed failures, with
  credential references redacted. Status payloads matched exactly across all four structured
  surfaces. Four serialized curation mutations converged live. The Provider Settings lifecycle
  reproduced discarded pricing deltas and an invalid-pricing HTTP 500; both repaired replays are
  green across config, catalog, restart, and no-mutation validation.
- **Bugs filed/updated:** BUG-20260729-provider-model-pricing-roundtrip;
  BUG-20260729-provider-model-validation-status
- **Scenarios settled:** MS-043 → pass; MS-044 → pass; MS-054 → pass; MS-056 → pending governed fixes
- **Paper cuts:** none
- **Surprises:** The first post-fix validation replay still returned 500 because that daemon had
  started 42 seconds before the rebuilt binary completed; restarting from the final binary produced
  the expected 400 without another source change.
- **Suggested next charter:** continue an independent untested batch outside the two pending TechSpec
  boundaries.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/045-model-catalog-lifecycle`

### CH-016 — Théo — stop a session across tabs

- **Ran:** 2026-07-29 (three isolated browser tabs, then two literal same-browser tabs, plus HTTP and UDS public reads)
- **Findings:** The visible session control stopped the idle session, removed it from the active set,
  preserved the stopped detail and four transcript entries, and classified a repeated stop as 404.
  One tab displayed the requested session, but two fresh tabs stayed on Marketplace after direct entry
  or explicit session selection. The staged topology and per-window live-ownership fixes subsequently
  brought the session to the foreground in both literal tabs. That replay exposed a second failure:
  the UI stop request remained queued in Chrome and never reached the daemon until the owned background
  tab closed, after which the pending request returned 204 immediately.
- **Bugs filed/updated:** BUG-20260729-session-window-cross-tab-focus;
  BUG-20260713-first-prompt-optimistic-stuck (regressed/reopened)
- **Scenarios settled:** RT-013 → pending root fix
- **Paper cuts:** none — the cross-tab foreground failure is recorded as Trust-Damage.
- **Surprises:** All tabs had zero page errors. Browser request `1092.1925` had no response and no
  daemon request until the second tab closed, proving transport admission starvation rather than an
  API/session-manager failure. The stop contract remained correct once admitted.
- **Suggested next charter:** author the required document-transport ownership TechSpec and re-walk
  CH-016; continue RT-014/RT-015 only with independent fixtures so this failed walk is not reused.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/046-session-lifecycle-browser`

### CH-untested-016-15-theo / CH-014 — Théo — delete and attach sessions

- **Ran:** 2026-07-29 (two isolated browser walks, HTTP/UDS/CLI contract probes, disposable fixtures)
- **Findings:** RT-014 passed end to end. Cancel preserved the stopped session, confirmation removed
  its transcript/history/catalog state, and the Web recovered from canonical and permalink routes.
  RT-015 failed twice at its named Web entry: Home remained foreground with `Live layout disconnected`
  and no resume control. Independent HTTP, UDS, and CLI attach flows passed default TTL, explicit
  holder, lock conflict, stopped conflict, over-limit validation, and cleanup.
- **Bugs filed/updated:** BUG-20260729-session-window-cross-tab-focus;
  BUG-20260729-session-attach-openapi-ttl
- **Scenarios settled:** RT-014 → pass; RT-015 → pending structural Web fix
- **Paper cuts:** none — both findings are contract/transport failures, not optional polish.
- **Surprises:** The live endpoint returned matching 400 errors above 24 hours, while OpenAPI omitted
  both that response and the maximum. The declarative source, generated artifacts, and docs are now
  corrected with focused red/green regression evidence; governed commit remains pending.
- **Suggested next charter:** continue an independent backend/runtime batch while the document-
  visibility TechSpec remains isolated.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/047-session-delete-attach`

### CH-untested-008-07-ada — Ada — Loop config and annotations

- **Ran:** 2026-07-29 (fresh disposable COMPOZY_HOME; public CLI, HTTP, and UDS)
- **Findings:** The rebuilt candidate returned an explicit `config: null` and empty annotations for
  both same-name workspace Loops. Alternating HTTP/UDS writes converged in the first workspace while
  the second stayed unchanged. Every missing-name GET/PUT returned 404; creating that name afterward
  proved no detached config or annotations had persisted. Delete returned both sidecar routes to 404,
  and same-name recreation started with null config and empty annotations over both transports.
- **Bugs filed/updated:** BUG-20260729-loop-sidecar-lifecycle
- **Scenarios settled:** none — TA-069 remains failed/Pending until the staged root fix has its one
  logical governed commit; the original-persona repaired replay itself is green.
- **Paper cuts:** none
- **Surprises:** The previous disposable lab had lost its bootstrap `config.toml`, so its attempted
  restart fell back to port 2123. Targeted teardown was clean, and the replay used a brand-new healthy
  manifest instead of repairing contaminated state.
- **Suggested next charter:** continue the adjacent Loop catalog/manageability batch TA-067–TA-075
  with fresh fixtures, leaving the TA-069 commit provenance visible as Pending.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-config-annotations-retest-20260729-20260729-183928-434217-lab/qa-artifacts/qa/evidence/049-loop-config-annotations-retest`

### CH-untested-008-07-ada — Ada — Loop catalog and native definition management

- **Ran:** 2026-07-29 (fresh isolated two-workspace lab; native tools, HTTP, and UDS)
- **Findings:** TA-067 passed counted catalog CRUD, filters, facets, stable pagination, lint/CAS
  negatives, rich terminal projection, and workspace isolation. TA-075's positive authoring verbs
  passed, but the original native CAS and read-only-delete errors discarded actionable semantics and
  the HTTP/UDS delete status diverged from OpenAPI.
- **Bugs filed/updated:** BUG-20260729-loop-native-error-semantics
- **Scenarios settled:** TA-067 → pass; TA-075 → pass after governed fix `103192e4` and the green
  original-persona replay.
- **Paper cuts:** none
- **Surprises:** The generated reason enum correctly co-shipped the two new codes; the combined
  OpenAPI/TypeScript files also contain earlier pending Loop/session contract updates from this QA run.
- **Suggested next charter:** continue TA-073 before closing artifact-only TA-072, then run the
  acpmock/gated TA-074 control matrix.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-catalog-native-20260729-20260729-185423-303044-lab/qa-artifacts/qa/evidence/050-loop-catalog-native`

### CH-untested-009-07-bruno / CH-untested-010-09-ada — Loop contracts and automation bindings

- **Ran:** 2026-07-29 (fresh isolated two-workspace lab; generated contracts, HTTP, UDS, and CLI cleanup)
- **Findings:** TA-072 passed all 24 generated Loop operations, exact status/body/filter/envelope
  coverage, strict enums/defaults, executed definitions/generations, SSE resume fields, lean
  `last_run`, and automation Loop-target/catch-up additions. TA-073 passed alternating HTTP/UDS
  job and trigger create/read/update, counted two-page filters, exact 400/422 negatives without
  residue, and a delegated automation run linked to an independently readable terminal Loop run.
- **Bugs filed/updated:** none
- **Scenarios settled:** TA-072 → pass; TA-073 → pass.
- **Paper cuts:** none
- **Surprises:** The run's effective iteration cap was 50 while the authored definition showed 1.
  Investigation confirmed the documented four-layer precedence: definition, daemon defaults,
  per-Loop config, then per-run overrides. TA-079 owns that separate lifecycle contract.
- **Suggested next charter:** execute TA-074's native catalog and availability matrix in a separate
  isolated acpmock lab, then continue its stateful controls under TA-076.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-automation-contract-20260729-20260729-195531-994564-lab/qa-artifacts/qa/evidence/051-loop-automation-contract`

### CH-untested-010-09-ada — Ada — Native Loop catalog and availability

- **Ran:** 2026-07-29 (fresh isolated two-workspace lab; native tools, HTTP, UDS, and focused race gate)
- **Findings:** TA-074 passed the exact 16-ID `compozy__loops` expansion and live availability for
  every one of the 14 Loop verbs. Native list returned exact `{loops,facets,page}` cursor pages with
  q/kind/category/status filters and isolated same-name workspace catalogs. Inspect, dry-run, real
  run, status, runs, and turns were structured; HTTP and UDS returned byte-identical terminal detail.
- **Bugs filed/updated:** none
- **Scenarios settled:** TA-074 → pass.
- **Paper cuts:** none
- **Surprises:** An operator CLI probe can explicitly target another workspace even when its default
  projection names the first workspace. This is intentional operator authority, not leakage; the
  non-operator adapter boundary rejects the same conflict with `scope_mismatch` before service access.
- **Suggested next charter:** execute TA-076's stateful native pause/resume/stop controls with an
  isolated acpmock provider, then keep approval safety under TA-077.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-native-control-20260729-202726-620346-lab/qa-artifacts/qa/evidence/052-loop-native-catalog`

### CH-untested-008-07-ada — Ada — Stateful native Loop controls

- **Ran:** 2026-07-29 (fresh isolated two-workspace lab; real ACP mock, native tools, HTTP, and UDS)
- **Findings:** Native run, dry-run without residue, status, filtered runs, Pause, Resume, Stop, and
  workspace isolation passed. The original candidate collapsed repeated Pause into `schema_invalid`.
  After a daemon restart, a successful native Resume separately left the run `running` at generation
  1 without a coordinator. The root fix now preserves typed control reasons and atomically reserves a
  generic finisher wake without claiming next-generation identity.
- **Bugs filed/updated:** BUG-20260729-loop-native-error-semantics;
  BUG-20260729-loop-resume-restart-stuck
- **Scenarios settled:** TA-076 → pass after governed fix `103192e4`. The rebuilt original-persona
  replay is green: paused generation 1 survived restart and reached done generation 2 after native
  Resume.
- **Paper cuts:** none
- **Surprises:** Pre-reserving the deterministic generation-2 coordinator was also incorrect because
  the resumed coordinator first acts as generation 1's finisher. The generic wake preserves that
  ownership boundary and eliminates duplicate materialization.
- **Suggested next charter:** after the requested main rebase, bootstrap a fresh isolated lab and
  continue TA-077 approval safety plus TA-078 terminal/result contracts.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-native-controls-20260729-20260729-205221-682163-lab/qa-artifacts/qa/evidence/053-loop-native-controls`

Compozy Impact Audit:

- Native tools: no tool IDs or schemas changed; the existing Pause/Resume/Stop/Approve adapters now
  preserve typed Loop reasons, with canonical native regressions and live receipts.
- Extensibility and hooks: Loop service wiring gained a coordinator-reactivation option; extension,
  hook, bundle, MCP-sidecar, and config lifecycle surfaces were checked and are unchanged.
- Workspace data isolation: control state, decisions, and the generic finisher wake are transacted
  under the requested `workspace_id`; a second workspace listed zero matching runs, and no cache,
  event, SSE, HTTP, UDS, or native read leaked the repaired run.
- Official Compozy skill: `skills/compozy/references/loops.md` documents the exact native control
  reasons; no public command or tool ID changed.

## What Was Fixed

- BUG-20260729-overview-json-parity — `observe overview -o json` injected a field outside the shared payload; root fix and regression are staged, pending the completion gate and governed commit.
- BUG-20260729-resource-docs-protected-kind — the resource guide used service-owned `bundle.activation` for generic CRUD; the guide now uses `automation.job` and directs bundle mutations to the bundle lifecycle service. The fix commit and site gate remain pending.
- BUG-20260729-skill-workspace-error-mapping — scoped skill handlers discarded preserved workspace error semantics; the shared mapper and canonical regression are staged, and the rebuilt daemon passed the original request.
- BUG-20260729-skill-agent-default-selection — the Skills settings view-model used lexical fleet order as default-agent policy; it now prefers the runtime's canonical `general` agent, and the full web gate plus browser replay pass.
- BUG-20260729-skill-policy-normalized-dirty — successful policy saves did not advance the Skills draft baseline and materialized an absent tombstone list; the draft machine now records the confirmed config and adopts the daemon's canonical refetch.
- BUG-20260729-extension-update-partial-error — the native extension adapter discarded committed batch results and an archive identity mismatch was untyped; the staged fix preserves partial progress and maps the domain failure to extension validation.
- BUG-20260729-marketplace-json-parity — Marketplace CLI JSON added workspace-resolution metadata outside the shared daemon contract; search and detail now use the contract-preserving writer and the canonical suite inspects unknown top-level fields.
- BUG-20260729-marketplace-file-cursor-fence — pagination treated file-catalog fetch time as revision identity; continuation now remains stable across identical refetches and invalidates only on a changed authored revision or projection.
- BUG-20260729-tool-invoke-structural-redaction — generic CLI tool rendering applied scalar entropy redaction to public IDs and cursors; structured results now use field-aware JSON redaction while secret-shaped fields remain protected.
- BUG-20260729-mcp-manual-exchange-timeout — the manual MCP exchange returned a raw transport deadline while pending input used the stable authorization timeout; both phases now share the CLI-owned public classification.
- BUG-20260729-doctor-log-tail-evidence — the available log-tail diagnostic omitted its structured capability status; both availability branches now project the authoritative status through the canonical diagnostic constructor.
- BUG-20260729-explorer-active-home-schema — the opt-in explorer helper ignored `COMPOZY_HOME`, and
  its bundled AGENT.md used legacy keys rejected by strict discovery; both contracts are corrected
  and the four public read surfaces are green.
- BUG-20260729-loop-native-error-semantics — native Loop adapters now preserve typed CAS,
  read-only-source, and state-control reasons instead of flattening them through status-only
  fallbacks; the rebuilt definition and control replays are green.
- BUG-20260729-loop-resume-restart-stuck — non-Goal Resume/Approve now reserve a transactional generic
  finisher wake and notify the task backstop after commit; pause → restart → Resume reaches the next
  generation both in the canonical E2E and through public surfaces.
- BUG-20260729-heartbeat-status-stale-eligibility — selected-session Heartbeat status was fetched once
  while prompting and never reconciled; the mounted exact query now polls at its five-second freshness
  boundary and the live view enabled Wake without reload.
- BUG-20260729-heartbeat-wake-rollback-stale-policy — Wake sorted immutable snapshots by creation time
  after rollback reused an older deduped row; current policy now follows the latest authoring revision's
  snapshot activation.
- BUG-20260729-provider-model-pricing-roundtrip — provider reconciliation returned early when curated
  membership was unchanged; it now persists explicit field deltas without materializing unchanged
  catalog enrichment.
- BUG-20260729-provider-model-validation-status — provider validation did not carry the Settings
  validation sentinel; invalid pricing now maps to HTTP 400 and preserves config bytes.
- BUG-20260729-session-attach-openapi-ttl — attach validation and OpenAPI were authored independently;
  shared contract constants now drive the runtime and request maximum, raw integers are bounded before
  duration conversion, the operation declares 400, generated artifacts are synchronized, and the
  runtime reference documents the exact lease boundary. Focused regressions cover the default,
  non-positive, maximum, maximum-plus-one, and largest-integer values without Manager CAS leakage.
- BUG-20260729-loop-sidecar-lifecycle — Loop sidecar APIs now resolve a live definition before access;
  delete stages the definition reversibly, removes config and annotations atomically, and restores all
  three resources through a bounded detached reconciliation when publication fails. The response and
  generated contracts require nullable `config`; focused regressions and the original-persona replay
  are green, while the governed commit remains pending.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- BUG-20260729-overview-json-parity — the CLI added `resolution_source` outside the shared
  `observe-overview/v1` payload while HTTP and UDS agreed.
- BUG-20260729-skill-workspace-error-mapping — a missing workspace on the skills list returned 500 instead of 404 before the staged root fix.
- BUG-20260729-skill-agent-default-selection — entering agent-scoped Skills settings selected `code_implementer` and replaced the page with a 404 before the staged view-model fix.
- BUG-20260729-skill-policy-normalized-dirty — a persisted Skills policy remained labeled `Unsaved changes` after the daemon normalized its duration value.
- BUG-20260729-extension-update-partial-error — a native all-update committed one extension but originally reported only `backend_unhealthy`, hiding the failed target and committed cleanup warning.
- BUG-20260729-marketplace-json-parity — Marketplace search and detail CLI JSON added `resolution_source` while HTTP and UDS agreed exactly.
- BUG-20260729-marketplace-file-cursor-fence — page two rejected a cursor from an unchanged file catalog because a new fetch timestamp changed the projection fence.
- BUG-20260729-tool-invoke-structural-redaction — generic CLI tool invocation replaced reusable public bundle IDs and continuation cursors with `[REDACTED]`.
- BUG-20260729-mcp-cli-json-parity — workspace MCP install, authorize, status, and logout added `resolution_source` outside their daemon-authored payloads; the lifecycle is green, but the third occurrence requires a structural writer TechSpec before correction.
- BUG-20260729-mcp-manual-exchange-timeout — exchange-phase timeout exposed the UDS path and raw HTTP deadline instead of the documented MCP authorization timeout.
- BUG-20260729-doctor-log-tail-evidence — the successful `doctor.logs.tail` item omitted `evidence.status` while every other live Doctor item carried structured evidence.
- BUG-20260729-explorer-active-home-schema — explorer installation targeted the operator default
  home instead of the active global registry, and the installed bytes were invalid under the strict
  AGENT.md schema.
- BUG-20260729-heartbeat-status-stale-eligibility — the mounted agent view kept Wake disabled after
  daemon health had become idle, healthy, and eligible.
- BUG-20260729-heartbeat-wake-rollback-stale-policy — authoring/status restored v1 while dry-run Wake
  still selected the newer-created v2 snapshot.
- BUG-20260729-provider-model-pricing-roundtrip — a Provider Settings PUT reported success but
  discarded five explicit pricing deltas when curated membership did not change.
- BUG-20260729-provider-model-validation-status — a negative provider model rate preserved config
  bytes but returned HTTP 500 instead of caller validation.
- BUG-20260729-session-window-cross-tab-focus — selecting or deep-linking the same session from a
  second tab left its mounted window behind Marketplace until reload; RT-015 re-found the same
  document/window boundary as `Live layout disconnected` before its resume control rendered.
- BUG-20260729-session-attach-openapi-ttl — HTTP and UDS returned the correct over-limit 400, but the
  generated API contract omitted that response and the 86,400-second maximum.
- BUG-20260729-loop-sidecar-lifecycle — missing and deleted Loops accepted or retained detached config
  and annotations, and known Loops without an override omitted the documented nullable field.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

- Structured CLI JSON needs one contract decision before its two-touch TechSpec: preserve raw
  daemon payloads by default and expose workspace resolution only through an explicit diagnostic
  envelope (recommended), or redefine every workspace-aware CLI JSON schema as augmented.
- Graceful shutdown needs one lifecycle decision before its two-touch TechSpec: drain already
  accepted internal Dreams inside the configured shutdown budget (recommended), or cancel them
  immediately with explicit incomplete-checkpoint semantics.
- Hidden browser documents need one transport-ownership decision before the two-touch TechSpec:
  release every product transport and catch up on visibility resume (recommended), or retain only the
  Window Manager control-plane WebSocket while data streams remain visibility-gated.
- ET-050's trust-root public contract will be resolved against daemon truth during its owning
  session before a verdict is assigned.

## Learnings

- Planning repair resolved all 132 blank journey references and all 203 charter-coverage gaps without changing any target verdict.
- Historical charters with non-canonical tours or persona/journey mismatch remain immutable; valid companion charters own the current replay.

## Final Status

- **Exit gate (full automated suite):** pending
- **Issues by user impact:** pending
- **Coverage:** 67/452 scenarios settled; 385 Pending
- **Verdict:** in progress — sixty-five scenarios passed; repaired root failures remain Pending until their governed fixes exist.

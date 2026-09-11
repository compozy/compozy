# QA Run Report — 2026-09-10 — QA execution unblock

- **Scope:** All 357 original handoff rows; adjacent repair canaries only.
- **Cadence tier:** targeted inventory reconciliation and real persona walks
- **Initial build:** ed7f2d7adc2d7ec38071677d28a2d6e87a019c28. Main integrated through fec0e9b08; latest committed fix 06c5e9e10. Daemon includes the Goal parser fix; CLI wait transport repair passed real replay and gate.
- **Started:** 2026-09-10T21:56:04.281977+00:00 · **Status:** in-progress
- **Current provider contract:** Cursor Agent grok-4.6, reasoning high, speed fast; supersedes initial Codex Luna xhigh. Operator native authentication preserved.
- **Environment:** Same isolated bootstrap manifest; production-parity daemon/Web and real Cursor Grok 4.6 High Fast worker/judge runs verified. Earlier Luna evidence remains historical.
- **Inventory:** [Complete reconciliation ledger](2026-09-10-qa-execution-unblock/inventory.csv). Original categories are estimates, never verdicts.

## Personas

Reuse [project personas](../personas.md); per-row assignments below. CLI/HTTP/UDS and desktop Web lanes stay distinct.

## Flows in Scope

- Two legacy rows (ET-051 and TA-021) have no journey field; map their current contracts before execution.
- [J-01](../journeys/J-01-arrive-and-use-run.md)
- [J-03](../journeys/J-03-observe-and-approve.md)
- [J-04](../journeys/J-04-operator-pause-resume.md)
- [J-05](../journeys/J-05-configure-no-fork.md)
- [J-06](../journeys/J-06-fork-and-edit.md)
- [J-07](../journeys/J-07-agent-operated-run.md)
- [J-08](../journeys/J-08-watch-and-maintain.md)
- [J-09](../journeys/J-09-automation-start-bindings.md)
- [J-10](../journeys/J-10-converse-and-decide.md)
- [J-11](../journeys/J-11-return-to-running-session.md)
- [J-12](../journeys/J-12-open-session-fast.md)
- [J-13](../journeys/J-13-follow-a-live-run.md)
- [J-14](../journeys/J-14-read-a-finished-transcript.md)
- [J-15](../journeys/J-15-operate-session-via-cli-api.md)
- [J-16](../journeys/J-16-watch-events-wake.md)
- [J-17](../journeys/J-17-session-create-unified-selector.md)
- [J-20](../journeys/J-20-catalog-curation-agent-surfaces.md)
- [J-22](../journeys/J-22-provider-settings-canary.md)
- [J-23](../journeys/J-23-return-to-network-work.md)
- [J-24](../journeys/J-24-triage-work-at-scale.md)
- [J-25](../journeys/J-25-browse-recover-knowledge.md)
- [J-26](../journeys/J-26-converge-and-control-goal.md)
- [J-27](../journeys/J-27-observe-and-author-goal.md)
- [J-28](../journeys/J-28-recover-context-and-budget.md)
- [J-29](../journeys/J-29-operate-and-recover-goal.md)
- [J-30](../journeys/J-30-scan-agent-fleet.md)
- [J-31](../journeys/J-31-steward-agent-definition.md)
- [J-32](../journeys/J-32-manage-agent-lifecycle.md)
- [J-administer-network-live](../journeys/J-administer-network-live.md)
- [J-administer-provider-auth](../journeys/J-administer-provider-auth.md)
- [J-administer-runtime-settings](../journeys/J-administer-runtime-settings.md)
- [J-administer-window-manager](../journeys/J-administer-window-manager.md)
- [J-agent-manage-window-tabs](../journeys/J-agent-manage-window-tabs.md)
- [J-agent-marketplace-parity](../journeys/J-agent-marketplace-parity.md)
- [J-answer-agent-requests](../journeys/J-answer-agent-requests.md)
- [J-approve-compozy-beta-candidate](../journeys/J-approve-compozy-beta-candidate.md)
- [J-bound-runaway-work](../journeys/J-bound-runaway-work.md)
- [J-complete-task-tree](../journeys/J-complete-task-tree.md)
- [J-complete-web-bridge-setup](../journeys/J-complete-web-bridge-setup.md)
- [J-connect-bridge-provider](../journeys/J-connect-bridge-provider.md)
- [J-create-and-activate-trigger](../journeys/J-create-and-activate-trigger.md)
- [J-cross-workspace-access](../journeys/J-cross-workspace-access.md)
- [J-deliver-long-formatted-reply](../journeys/J-deliver-long-formatted-reply.md)
- [J-deliver-through-public-gateway](../journeys/J-deliver-through-public-gateway.md)
- [J-desktop-agent-headless](../journeys/J-desktop-agent-headless.md)
- [J-desktop-attach-daily](../journeys/J-desktop-attach-daily.md)
- [J-desktop-first-run](../journeys/J-desktop-first-run.md)
- [J-desktop-update-moment](../journeys/J-desktop-update-moment.md)
- [J-diagnose-task-session-health](../journeys/J-diagnose-task-session-health.md)
- [J-digest-sessions-into-memory](../journeys/J-digest-sessions-into-memory.md)
- [J-drain-daemon-safely](../journeys/J-drain-daemon-safely.md)
- [J-edit-reply-context](../journeys/J-edit-reply-context.md)
- [J-enable-coordinated-conversations](../journeys/J-enable-coordinated-conversations.md)
- [J-evaluate-compozy-beta](../journeys/J-evaluate-compozy-beta.md)
- [J-expose-and-pair-gateway](../journeys/J-expose-and-pair-gateway.md)
- [J-extension-agent-authoring](../journeys/J-extension-agent-authoring.md)
- [J-extension-dev-lifecycle](../journeys/J-extension-dev-lifecycle.md)
- [J-extension-distribution](../journeys/J-extension-distribution.md)
- [J-extension-kit-lifecycle](../journeys/J-extension-kit-lifecycle.md)
- [J-extension-newcomer-first-success](../journeys/J-extension-newcomer-first-success.md)
- [J-extension-policy-admin](../journeys/J-extension-policy-admin.md)
- [J-keep-secrets-contained](../journeys/J-keep-secrets-contained.md)
- [J-loop-terminal-recovery](../journeys/J-loop-terminal-recovery.md)
- [J-manage-sandbox-profiles](../journeys/J-manage-sandbox-profiles.md)
- [J-marketplace-acquisition](../journeys/J-marketplace-acquisition.md)
- [J-mcp-authorize-repair](../journeys/J-mcp-authorize-repair.md)
- [J-network-local-default](../journeys/J-network-local-default.md)
- [J-offer-runnable-capabilities](../journeys/J-offer-runnable-capabilities.md)
- [J-one-kickoff-collaboration](../journeys/J-one-kickoff-collaboration.md)
- [J-open-foreign-session](../journeys/J-open-foreign-session.md)
- [J-operate-bounded-task-capacity](../journeys/J-operate-bounded-task-capacity.md)
- [J-operate-daemon-schema](../journeys/J-operate-daemon-schema.md)
- [J-operate-desktop-shell](../journeys/J-operate-desktop-shell.md)
- [J-operate-integrated-terminal](../journeys/J-operate-integrated-terminal.md)
- [J-operate-loop-run-headless](../journeys/J-operate-loop-run-headless.md)
- [J-operate-remote-gateway-cli](../journeys/J-operate-remote-gateway-cli.md)
- [J-operate-terminal-windows](../journeys/J-operate-terminal-windows.md)
- [J-operate-workspace-context](../journeys/J-operate-workspace-context.md)
- [J-prune-missing-workspace](../journeys/J-prune-missing-workspace.md)
- [J-publish-compozy-beta](../journeys/J-publish-compozy-beta.md)
- [J-recover-loop-node-failure](../journeys/J-recover-loop-node-failure.md)
- [J-recover-mid-turn-restart](../journeys/J-recover-mid-turn-restart.md)
- [J-replay-loop-history](../journeys/J-replay-loop-history.md)
- [J-respond-to-agent-attention](../journeys/J-respond-to-agent-attention.md)
- [J-retire-workspace](../journeys/J-retire-workspace.md)
- [J-route-background-work](../journeys/J-route-background-work.md)
- [J-run-bounded-live-collaboration](../journeys/J-run-bounded-live-collaboration.md)
- [J-supervise-agent-terminal](../journeys/J-supervise-agent-terminal.md)
- [J-supervise-loop-request](../journeys/J-supervise-loop-request.md)
- [J-use-terminal-desktop](../journeys/J-use-terminal-desktop.md)
- [J-validate-compozy-hard-cut](../journeys/J-validate-compozy-hard-cut.md)
- [J-watch-agent-work-channel](../journeys/J-watch-agent-work-channel.md)
- [J-worktree-management](../journeys/J-worktree-management.md)

## Session Matrix & Results

All rows initialized Pending before first product interaction. Existing charters are candidates to read and select before each coherent walk. The inventory records every candidate; the matrix shows the first candidate.

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-desktop-agent-headless-cli | J-desktop-agent-headless / APP-agent-cli-app-verbs | Ada | Feature Tour | Pending | | |
| 2 | CH-desktop-update-rehearsal-macos | J-desktop-update-moment / APP-app-auto-update | Bruno | Interrupt Tour | Pending | | |
| 3 | Prerequisite reassessment | J-desktop-first-run / APP-appimage-fuseless-launch | Lea |  | Blocked (needs human verify) | [Platform prerequisite](../scenarios/APP-appimage-fuseless-launch.md) | Exact Linux x64/FUSE3-only runtime required. |
| 4 | CH-electron-offline-first-run-linux | J-desktop-first-run / APP-install-first-run-provision | Lea | Network Tour | Pending | | |
| 5 | CH-native-window-chrome-linux | J-desktop-attach-daily / APP-native-window-controls | Dora | Feature Tour | Pending | | |
| 6 | CH-desktop-attach-quit-linux | J-desktop-attach-daily / APP-quit-contract | Dora | Interrupt Tour | Pending | | |
| 7 | CH-desktop-update-rehearsal-macos | J-desktop-update-moment / APP-runtime-update-app-owned | Bruno | Interrupt Tour | Pending | | |
| 8 | CH-electron-update-headless-authority | J-desktop-agent-headless / APP-single-command-multi-target-update | Ada | Feature Tour | Pending | | |
| 9 | CH-terminal-desktop-fidelity | J-use-terminal-desktop / APP-terminal-desktop-fidelity | Marina | Paste Tour | Pending | | |
| 10 | CH-native-window-chrome-linux | J-desktop-attach-daily / APP-window-geometry-recovery | Dora | Feature Tour | Pending | | |
| 11 | CH-untested-valid-017-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-001 | Bruno | Feature Tour | Pending | | |
| 12 | CH-untested-047-agent-marketplace-parity-bruno | J-agent-marketplace-parity / ET-012 | Bruno | Feature Tour | Pending | | |
| 13 | CH-untested-060-extension-policy-admin-vera | J-extension-policy-admin / ET-013 | Vera | Feature Tour | Pending | | |
| 14 | CH-agent-marketplace-parity | J-agent-marketplace-parity / ET-016 | Ada | Feature Tour | Pending | | |
| 15 | CH-extension-distribution-integrity | J-extension-distribution / ET-017 | Ada | Garbage Tour | Pending | | |
| 16 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / ET-047 | Ada | Paste Tour | Pending | | |
| 17 | CH-untested-060-extension-policy-admin-vera | J-extension-policy-admin / ET-050 | Vera | Feature Tour | Pending | | |
| 18 | Charter mapping pending |  / ET-051 | Vera |  | Pending | | |
| 19 | CH-model-catalog-guidance-parity | J-20 / ET-053 | Ada | Feature Tour | Pending | | |
| 20 | CH-agent-plugin-marketplace | J-marketplace-acquisition / ET-agent-plugin-marketplace-install | Bruno | Back-Button Tour | Pending | | |
| 21 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / ET-api-mcp-oauth-endpoints | Ada | Paste Tour | Pending | | |
| 22 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-cli-extension-sideload-policy-block | Ada | Garbage Tour | Pending | | |
| 23 | CH-mcp-authorize-repair-truth | J-mcp-authorize-repair / ET-cli-mcp-authorize | Ada | Interrupt Tour | Pending | | |
| 24 | CH-cli-tool-structural-handles | J-agent-marketplace-parity / ET-cli-tool-invoke-structural-handles | Ada | Feature Tour | Pending | | |
| 25 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / ET-compozy-extension-contract-identity | Ada | Garbage Tour | Pending | | |
| 26 | CH-gateway-provider-degradation | J-extension-policy-admin / ET-connectivity-provider-trust | Vera | Network Tour | Pending | | |
| 27 | Charter mapping pending | J-operate-desktop-shell / ET-electron-session-copy-debug | Bruno |  | Pending | | |
| 28 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-ext-curated-digest-verify | Ada | Garbage Tour | Pending | | |
| 29 | CH-extension-agent-authoring | J-extension-agent-authoring / ET-extension-agent-guided-authoring | Ada | Feature Tour | Pending | | |
| 30 | CH-extension-agent-observation | J-extension-dev-lifecycle / ET-extension-agent-observer-resolution | Ada | Feature Tour | Pending | | |
| 31 | CH-extension-dev-recovery | J-extension-kit-lifecycle / ET-extension-code-first-authoring | Ada | Interrupt Tour | Pending | | |
| 32 | CH-extension-newcomer-first-success | J-extension-newcomer-first-success / ET-extension-dx-scorecard | Lea | Feature Tour | Pending | | |
| 33 | CH-extension-command-authority | J-extension-policy-admin / ET-extension-manifest-v2-surfaces | Ada | Feature Tour | Pending | | |
| 34 | CH-extension-distribution-integrity | J-extension-distribution / ET-extension-passive-update-discovery | Ada | Garbage Tour | Pending | | |
| 35 | CH-extension-newcomer-first-success | J-extension-newcomer-first-success / ET-extension-quickstart-verbatim | Lea | Feature Tour | Pending | | |
| 36 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-drag-rebalance | Bruno | Back-Button Tour | Pending | | |
| 37 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-gaps-follow-canvas | Bruno | Back-Button Tour | Pending | | |
| 38 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-group-overlap-refused | Bruno | Back-Button Tour | Pending | | |
| 39 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-load-saved-layout | Bruno | Back-Button Tour | Pending | | |
| 40 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-split-orientation | Bruno | Back-Button Tour | Pending | | |
| 41 | CH-untested-043-administer-window-manager-bruno-part-1 | J-administer-window-manager / ET-layout-editor-split-weights | Bruno | Back-Button Tour | Pending | | |
| 42 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-marketplace-kill-switch | Vera | Garbage Tour | Pending | | |
| 43 | CH-truthful-cost-provenance | J-20 / ET-model-source-five-rate-pricing | Ada | Money Tour | Pending | | |
| 44 | CH-approval-grant-memory | J-answer-agent-requests / ET-native-tool-approval-grants | Théo | Interrupt Tour | Pending | | |
| 45 | CH-untested-070-operate-workspace-context-ada | J-operate-workspace-context / ET-native-workspace-scope-isolation | Ada | Back-Button Tour | Pending | | |
| 46 | CH-nested-skill-groups | J-offer-runnable-capabilities / ET-nested-skill-groups | Ada | Feature Tour | Pending | | |
| 47 | CH-site-docs-search-context | J-evaluate-compozy-beta / ET-site-docs-search-context | Dora | Feature Tour | Pending | | |
| 48 | CH-terminal-approval-ladder | J-supervise-agent-terminal / ET-terminal-approval-ladder-grants | Bruno | Feature Tour | Pending | | |
| 49 | CH-terminal-operator-shell | J-operate-integrated-terminal / ET-terminal-session-block-handoff | Bruno | Feature Tour | Pending | | |
| 50 | Platform prerequisite (historical charter superseded) | J-operate-terminal-windows / ET-terminal-windows-parity | Dora | Feature Tour | Blocked (needs human verify) | [Platform prerequisite](../scenarios/ET-terminal-windows-parity.md) | Real Windows runtime required; no cross-compile pass. |
| 51 | CH-artifact-recovery-paging | J-14 / ET-tool-result-artifact-recovery | Rafa | Garbage Tour | Pending | | |
| 52 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-dock-magnification | Bruno | Feature Tour | Pending | | |
| 53 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-ext-policy-block | Bruno | Money Tour | Pending | | |
| 54 | CH-marketplace-under-a-minute | J-marketplace-acquisition / ET-web-extension-detail | Bruno | Money Tour | Pending | | |
| 55 | CH-untested-007-06-bruno | J-06 / ET-web-loop-editor-node-truncate | Bruno | Back-Button Tour | Pending | | |
| 56 | CH-untested-007-06-bruno | J-06 / ET-web-loop-editor-sidebar-tabs | Bruno | Back-Button Tour | Pending | | |
| 57 | CH-untested-007-06-bruno | J-06 / ET-web-loop-editor-topbar | Bruno | Back-Button Tour | Pending | | |
| 58 | CH-marketplace-scope-isolation | J-mcp-authorize-repair / ET-web-marketplace-mcp-authorize-installed | Bruno | Multi-Tab Tour | Pending | | |
| 59 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-marketplace-remove-scope-return | Bruno | Feature Tour | Pending | | |
| 60 | CH-mcp-authorize-repair-truth | J-mcp-authorize-repair / ET-web-mcp-authorize | Bruno | Interrupt Tour | Pending | | |
| 61 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-page-content-gutter | Bruno | Feature Tour | Pending | | |
| 62 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-route-chrome-topbar | Bruno | Feature Tour | Pending | | |
| 63 | CH-foreign-session-deep-link | J-open-foreign-session / ET-web-session-cross-workspace-confirm | Nia | Back-Button Tour | Pending | | |
| 64 | CH-untested-070-operate-workspace-context-ada | J-open-foreign-session / ET-web-session-deep-link-isolation | Théo | Back-Button Tour | Pending | | |
| 65 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-web-settings-extensions-policy | Vera | Garbage Tour | Pending | | |
| 66 | CH-extension-policy-admin-gates | J-extension-policy-admin / ET-web-settings-hooks | Vera | Garbage Tour | Pending | | |
| 67 | CH-untested-024-24-bruno-part-1 | J-24 / ET-web-tasks-mode-url | Bruno | Garbage Tour | Pending | | |
| 68 | CH-plain-scale-legibility | J-operate-desktop-shell / ET-web-ui-resilience | Bruno | Feature Tour | Pending | | |
| 69 | CH-untested-063-marketplace-acquisition-bruno | J-marketplace-acquisition / ET-web-vault-opendesign-listing | Bruno | Feature Tour | Pending | | |
| 70 | CH-window-tabs-agent-parity | J-agent-manage-window-tabs / ET-window-tab-v3-discard | Ada | Feature Tour | Pending | | |
| 71 | CH-cross-workspace-mode-seams | J-cross-workspace-access / ET-workspace-access-mode-matrix | Ada | Feature Tour | Fail | [Provider denial diagnostic](../bugs/BUG-20260910-cursor-denial-hides-workspace-policy.md) | Deny-all branch failed; other reachable seams continue. |
| 72 | CH-cross-workspace-consent-audit | J-cross-workspace-access / ET-workspace-access-prompt-outcomes | Bruno | Interrupt Tour | Pending | | |
| 73 | CH-046 | J-26 / GL-001 | Lea | Feature Tour | Pass | goal-parser-retest-proof.json | Direct 202, one session-origin Run, one canonical judge, completed and retained. |
| 74 | CH-046 | J-26 / GL-002 | Lea | Feature Tour | Pass | goal-first-run-proof.json | Text clauses remain in one agent-judge rubric; approved real run and refreshed Web. |
| 75 | CH-046 | J-26 / GL-003 | Lea | Feature Tour | Pending | | |
| 76 | CH-047 | J-26 / GL-005 | Bruno | Interrupt Tour | Pass | goal-controls-proof.json | Public state, SSE, and refreshed Web confirmed. |
| 77 | CH-047 | J-26 / GL-006 | Bruno | Interrupt Tour | Pass | goal-controls-proof.json | Public state, SSE, and refreshed Web confirmed. |
| 78 | CH-047 | J-26 / GL-007 | Bruno | Interrupt Tour | Fixed | goal-extension-proof.json | Public retake and refreshed Web confirmed. |
| 79 | CH-047 | J-26 / GL-008 | Bruno | Interrupt Tour | Pass | goal-controls-proof.json | Public state, SSE, and refreshed Web confirmed. |
| 80 | CH-046 | J-26 / GL-009 | Lea | Feature Tour | Fixed | busy-goal-feedback-proof.json | Public retake and refreshed Web confirmed. |
| 81 | CH-046 | J-26 / GL-010 | Lea | Feature Tour | Pass | goal-replacement-proof.json; busy-goal-feedback-proof.json | Public retake and refreshed Web confirmed. |
| 82 | CH-046 | J-26 / GL-011 | Lea | Feature Tour | Pass | goal-replacement-proof.json; busy-goal-feedback-proof.json | Public retake and refreshed Web confirmed. |
| 83 | CH-046 | J-26 / GL-012 | Lea | Feature Tour | Pass | goal-blocked-proof.json | Direct start after blocked; approved successor and retained predecessor. |
| 84 | CH-046 | J-26 / GL-013 | Lea | Feature Tour | Pending | | |
| 85 | CH-048 | J-27 / GL-014 | Marina | Feature Tour | Pending | | |
| 86 | CH-048 | J-27 / GL-015 | Marina | Feature Tour | Pending | | |
| 87 | CH-048 | J-27 / GL-016 | Marina | Feature Tour | Pending | | |
| 88 | CH-041 | J-28 / GL-017 | Bruno | Interrupt Tour | Pending | | |
| 89 | CH-041 | J-28 / GL-018 | Bruno | Interrupt Tour | Pending | | |
| 90 | CH-041 | J-28 / GL-019 | Bruno | Interrupt Tour | Pending | | |
| 91 | CH-041 | J-28 / GL-020 | Bruno | Interrupt Tour | Pending | | |
| 92 | CH-042 | J-28 / GL-021 | Bruno | Network Tour | Pending | | |
| 93 | CH-040 | J-27 / GL-022 | Bruno | Back-Button Tour | Pending | | |
| 94 | CH-045 | J-27 / GL-023 | Bruno | Feature Tour | Pending | | |
| 95 | CH-045 | J-27 / GL-024 | Bruno | Feature Tour | Pending | | |
| 96 | CH-043 | J-29 / GL-027 | Ada | Feature Tour | Pending | | |
| 97 | CH-043 | J-29 / GL-028 | Ada | Feature Tour | Pending | | |
| 98 | CH-044 | J-29 / GL-029 | Ada | Interrupt Tour | Pending | | |
| 99 | CH-044 | J-29 / GL-030 | Ada | Interrupt Tour | Pending | | |
| 100 | CH-044 | J-29 / GL-031 | Ada | Interrupt Tour | Pending | | |
| 101 | CH-044 | J-29 / GL-032 | Ada | Interrupt Tour | Pending | | |
| 102 | CH-040 | J-27 / GL-033 | Sol | Back-Button Tour | Pending | | |
| 103 | CH-048 | J-27 / GL-035 | Marina | Feature Tour | Pending | | |
| 104 | CH-043 | J-29 / GL-036 | Ada | Feature Tour | Pending | | |
| 105 | CH-044 | J-29 / GL-037 | Ada | Interrupt Tour | Pending | | |
| 106 | CH-042 | J-28 / GL-038 | Bruno | Network Tour | Pending | | |
| 107 | CH-044 | J-29 / GL-039 | Ada | Interrupt Tour | Pending | | |
| 108 | CH-042 | J-28 / GL-040 | Ada | Network Tour | Pending | | |
| 109 | CH-judge-session-attribution | J-26 / GL-judge-session-contract | Lea | Feature Tour | Pending | | |
| 110 | CH-012 | J-01 / LP-005 | Bruno | Feature Tour | Pending | | |
| 111 | CH-002 | J-03 / LP-009 | Marina | Interrupt Tour | Pending | | |
| 112 | CH-003 | J-04 / LP-014 | Bruno | Interrupt Tour | Pending | | |
| 113 | CH-006 | J-05 / LP-017 | Bruno | Back-Button Tour | Pending | | |
| 114 | CH-006 | J-05 / LP-018 | Bruno | Back-Button Tour | Pending | | |
| 115 | CH-006 | J-05 / LP-019 | Bruno | Back-Button Tour | Pending | | |
| 116 | CH-013 | J-05 / LP-020 | Sol | Back-Button Tour | Pending | | |
| 117 | CH-007 | J-06 / LP-021 | Bruno | Multi-Tab Tour | Pending | | |
| 118 | CH-007 | J-06 / LP-022 | Bruno | Multi-Tab Tour | Pending | | |
| 119 | CH-007 | J-06 / LP-023 | Bruno | Multi-Tab Tour | Pending | | |
| 120 | CH-007 | J-06 / LP-024 | Bruno | Multi-Tab Tour | Pending | | |
| 121 | CH-004 | J-07 / LP-025 | Ada | Feature Tour | Pending | | |
| 122 | CH-004 | J-07 / LP-026 | Ada | Feature Tour | Pending | | |
| 123 | CH-004 | J-07 / LP-027 | Ada | Feature Tour | Pending | | |
| 124 | CH-004 | J-07 / LP-028 | Ada | Feature Tour | Pending | | |
| 125 | CH-026 | J-08 / LP-029 | Bruno | Feature Tour | Pending | | |
| 126 | CH-005 | J-08 / LP-030 | Bruno | Interrupt Tour | Pending | | |
| 127 | CH-009 | J-09 / LP-033 | Marina | Back-Button Tour | Pending | | |
| 128 | CH-009 | J-09 / LP-034 | Marina | Back-Button Tour | Pending | | |
| 129 | CH-009 | J-09 / LP-035 | Marina | Back-Button Tour | Pending | | |
| 130 | CH-010 | J-10 / LP-036 | Bruno | Feature Tour | Pending | | |
| 131 | CH-022 | J-16 / LP-043 | Bruno | Feature Tour | Pending | | |
| 132 | CH-026 | J-01 / LP-046 | Bruno | Feature Tour | Pending | | |
| 133 | CH-025 | J-16 / LP-047 | Bruno | Feature Tour | Pending | | |
| 134 | CH-025 | J-16 / LP-048 | Bruno | Feature Tour | Pending | | |
| 135 | CH-025 | J-16 / LP-049 | Bruno | Feature Tour | Pending | | |
| 136 | CH-025 | J-16 / LP-050 | Bruno | Feature Tour | Pending | | |
| 137 | CH-untested-003-01-lea | J-01 / LP-action-failure-detail | Lea | Feature Tour | Pending | | |
| 138 | CH-loop-death-resume | J-recover-loop-node-failure / LP-crash-death-resume | Bruno | Recovery Tour | Pending | | |
| 139 | CH-runaway-work-bounded | J-recover-loop-node-failure / LP-days-long-node-no-clock | Bruno | Garbage Tour | Pending | | |
| 140 | CH-agent-loop-lifecycle-parity | J-07 / LP-duplicate-event-suppressed | Ada | Feature Tour | Pending | | |
| 141 | CH-agent-loop-lifecycle-parity | J-07 / LP-durable-wait-restart | Ada | Feature Tour | Pending | | |
| 142 | CH-author-loop-failure-contract | J-recover-loop-node-failure / LP-editor-authoring-walk | Lea | Feature Tour | Pending | | |
| 143 | CH-spec-cycle-three-loop-lifecycle | J-01 / LP-implement-tasks-orchestrated-mode | Bruno | Feature Tour | Pending | | |
| 144 | CH-loop-terminal-time-recovery | J-loop-terminal-recovery / LP-invalid-snapshot-boot-isolation | Ada | Interrupt Tour | Pending | | |
| 145 | CH-005 | J-08 / LP-review-round-finalization | Bruno | Interrupt Tour | Pending | | |
| 146 | CH-loop-legibility-run-read-resume | J-operate-loop-run-headless / LP-run-read-agent-journey | Ada | Network Tour | Pending | | |
| 147 | CH-compozy-mixed-runtime-delivery | J-01 / LP-runtime-selection-overrides | Bruno | Feature Tour | Pending | | |
| 148 | CH-loop-quarantine-repair | J-recover-loop-node-failure / LP-sick-target-degrades-one-lane | Bruno | Recovery Tour | Pending | | |
| 149 | CH-task-tree-loop-rollup | J-complete-task-tree / LP-task-rollup-wakes-loop | Bruno | Feature Tour | Pending | | |
| 150 | CH-loop-goal-delete | J-06 / LP-toggle-loop-goal | Bruno | Feature Tour | Pending | | |
| 151 | CH-author-loop-failure-contract | J-recover-loop-node-failure / LP-transient-blip-heals | Lea | Feature Tour | Pending | | |
| 152 | CH-author-loop-failure-contract | J-recover-loop-node-failure / LP-unannotated-escalation | Lea | Feature Tour | Pending | | |
| 153 | CH-agent-loop-lifecycle-parity | J-07 / LP-waiting-inventory-escalation | Ada | Feature Tour | Pending | | |
| 154 | Charter mapping pending | J-05 / LP-web-catalog-badge-budget | Dora |  | Pending | | |
| 155 | CH-untested-006-05-dora | J-05 / LP-web-loop-configure-modal | Dora | Back-Button Tour | Pending | | |
| 156 | Charter mapping pending | J-05 / LP-web-node-dialog-modal-contract | Dora |  | Pending | | |
| 157 | CH-loop-request-lifecycle | J-supervise-loop-request / LP-web-request-answer-card | Bruno | Network Tour | Pending | | |
| 158 | Charter mapping pending | J-05 / LP-web-run-attention-quarantine-routing | Dora |  | Pending | | |
| 159 | CH-loop-time-travel-history | J-replay-loop-history / LP-web-run-diff-view | Bruno | Multi-Tab Tour | Pending | | |
| 160 | Charter mapping pending | J-05 / LP-web-run-form-section-grammar | Dora |  | Pending | | |
| 161 | Charter mapping pending | J-05 / LP-web-run-session-one-click | Dora |  | Pending | | |
| 162 | CH-039 | J-25 / MS-006 | Rafa | Interrupt Tour | Pending | | |
| 163 | CH-039 | J-25 / MS-008 | Dora | Interrupt Tour | Pending | | |
| 164 | CH-untested-valid-019-digest-sessions-into-memory-rafa | J-digest-sessions-into-memory / MS-011 | Rafa | Feature Tour | Fixed | health-retest-proof.json | Browser health and missing-workspace404 verified; gate passed. |
| 165 | CH-dream-pipeline-canary | J-digest-sessions-into-memory / MS-016 | Dora | Feature Tour | Pending | | |
| 166 | CH-untested-061-keep-secrets-contained-dora | J-keep-secrets-contained / MS-041 | Dora | Garbage Tour | Pass | Web typed/simple delete; cancel; disappearing target; HTTP/UDS/reload | |
| 167 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-049 | Dora | Back-Button Tour | Pending | | |
| 168 | CH-033 | J-22 / MS-058 | Marina | Back-Button Tour | Pending | | |
| 169 | CH-039 | J-25 / MS-059 | Rafa | Interrupt Tour | Pending | | |
| 170 | CH-memory-batch-integrity | J-11 / MS-atomic-memory-batch | Ada | Garbage Tour | Pending | | |
| 171 | CH-role-fallback-boundary | J-route-background-work / MS-background-role-fallback | Ada | Network Tour | Pending | | |
| 172 | CH-untested-044-administer-window-manager-bruno-part-2 | J-administer-window-manager / MS-layout-editor-clear-selection | Bruno | Back-Button Tour | Pending | | |
| 173 | CH-provider-settings-model-delta | J-20 / MS-provider-settings-model-delta-roundtrip | Ada | Garbage Tour | Pending | | |
| 174 | CH-settings-roles-live-truth | J-route-background-work / MS-settings-roles-panel | Dora | Back-Button Tour | Pending | | |
| 175 | CH-electron-beta-update-linux | J-desktop-update-moment / MS-settings-update-mutations | Dora | Interrupt Tour | Pending | | |
| 176 | CH-untested-020-22-dora | J-22 / MS-web-provider-auth-gate | Dora | Back-Button Tour | Pending | | |
| 177 | CH-global-scope-regression | J-operate-desktop-shell / MS-web-session-deeplink-global-confirm | Bruno | Feature Tour | Pending | | |
| 178 | CH-untested-020-22-dora | J-22 / MS-web-settings-providers-redesign | Dora | Back-Button Tour | Pending | | |
| 179 | CH-crash-resume-compaction | J-11 / MS-workspace-checkpoint-continuity | Théo | Interrupt Tour | Pending | | |
| 180 | CH-untested-070-operate-workspace-context-ada | J-operate-workspace-context / MS-workspace-resolution-provenance | Ada | Back-Button Tour | Pending | | |
| 181 | CH-untested-valid-016-administer-network-live-ada | J-administer-network-live / NB-001 | Ada | Back-Button Tour | Pending | | |
| 182 | CH-network-admin-lifecycle | J-administer-network-live / NB-002 | Bruno | Multi-Tab Tour | Pending | | |
| 183 | CH-037 | J-23 / NB-004 | Théo | Interrupt Tour | Pending | | |
| 184 | CH-037 | J-23 / NB-005 | Théo | Interrupt Tour | Pending | | |
| 185 | CH-untested-023-23-theo | J-23 / NB-006 | Théo | Network Tour | Pending | | |
| 186 | CH-037 | J-23 / NB-007 | Théo | Interrupt Tour | Pending | | |
| 187 | CH-037 | J-23 / NB-008 | Théo | Interrupt Tour | Pending | | |
| 188 | CH-037 | J-23 / NB-009 | Théo | Interrupt Tour | Pending | | |
| 189 | CH-037 | J-23 / NB-010 | Théo | Interrupt Tour | Pending | | |
| 190 | CH-untested-023-23-theo | J-23 / NB-011 | Théo | Network Tour | Pending | | |
| 191 | CH-037 | J-23 / NB-012 | Théo | Interrupt Tour | Pending | | |
| 192 | CH-037 | J-23 / NB-013 | Théo | Interrupt Tour | Pending | | |
| 193 | CH-untested-023-23-theo | J-23 / NB-014 | Théo | Network Tour | Pending | | |
| 194 | CH-037 | J-23 / NB-015 | Théo | Interrupt Tour | Pending | | |
| 195 | CH-untested-023-23-theo | J-23 / NB-016 | Théo | Network Tour | Pending | | |
| 196 | CH-untested-023-23-theo | J-23 / NB-017 | Théo | Network Tour | Pending | | |
| 197 | CH-037 | J-23 / NB-019 | Théo | Interrupt Tour | Pending | | |
| 198 | CH-037 | J-23 / NB-020 | Ada | Interrupt Tour | Pending | | |
| 199 | CH-037 | J-23 / NB-027 | Omar | Interrupt Tour | Pending | | |
| 200 | CH-mid-turn-bridge-restart | J-recover-mid-turn-restart / NB-031 | Omar | Interrupt Tour | Pending | | |
| 201 | CH-037 | J-23 / NB-032 | Omar | Interrupt Tour | Pending | | |
| 202 | CH-first-slack-response | J-connect-bridge-provider / NB-037 | Tessa | Feature Tour | Pending | | |
| 203 | CH-untested-021-23-bruno | J-23 / NB-045 | Bruno | Network Tour | Pending | | |
| 204 | CH-037 | J-23 / NB-047 | Théo | Interrupt Tour | Pending | | |
| 205 | CH-live-bounds-agent-path | J-run-bounded-live-collaboration / NB-agent-manages-participation | Ada | Interrupt Tour | Pending | | |
| 206 | CH-edit-reply-context | J-edit-reply-context / NB-bridge-edit-reply | Maya | Interrupt Tour | Pending | | |
| 207 | CH-bridge-overload-taxonomy | J-connect-bridge-provider / NB-bridge-overload-recovery | Omar | Network Tour | Pending | | |
| 208 | CH-guided-setup-credentials | J-connect-bridge-provider / NB-bridge-provider-setup | Tessa | Paste Tour | Pending | | |
| 209 | CH-mid-turn-bridge-restart | J-recover-mid-turn-restart / NB-bridge-restart-recovery | Omar | Interrupt Tour | Pending | | |
| 210 | CH-bridge-progress-stress | J-watch-agent-work-channel / NB-bridge-tool-progress | Maya | Garbage Tour | Pending | | |
| 211 | CH-coordination-future-runs | J-enable-coordinated-conversations / NB-coordination-invitation-future-runs | Bruno | Back-Button Tour | Pending | | |
| 212 | CH-untested-054-connect-bridge-provider-omar | J-connect-bridge-provider / NB-indeterminate-bridge-delivery | Omar | Network Tour | Pending | | |
| 213 | CH-long-provider-replies | J-deliver-long-formatted-reply / NB-long-bridge-replies | Omar | Paste Tour | Pending | | |
| 214 | CH-network-admin-lifecycle | J-administer-network-live / NB-network-availability-toggle | Bruno | Multi-Tab Tour | Pending | | |
| 215 | CH-network-admin-lifecycle | J-administer-network-live / NB-network-live-config-lifecycle | Bruno | Multi-Tab Tour | Pending | | |
| 216 | CH-bridge-progress-stress | J-watch-agent-work-channel / NB-provider-progress-rendering | Maya | Garbage Tour | Pending | | |
| 217 | CH-live-bounds-agent-path | J-run-bounded-live-collaboration / NB-run-bounded-live-collaboration | Ada | Interrupt Tour | Pending | | |
| 218 | CH-coordination-future-runs | J-enable-coordinated-conversations / NB-run-conversation-bounds-usage | Bruno | Back-Button Tour | Pending | | |
| 219 | CH-web-bridge-setup | J-complete-web-bridge-setup / NB-web-bridge-setup | Tessa | Back-Button Tour | Pending | | |
| 220 | CH-untested-023-23-theo | J-23 / NB-web-network-head-trail | Théo | Network Tour | Pending | | |
| 221 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-beta-channel-contract | Dora | Garbage Tour | Pending | | |
| 222 | CH-electron-install-docs-canary | J-evaluate-compozy-beta / REL-beta-install-paths | Dora | Feature Tour | Pending | | |
| 223 | CH-electron-install-docs-canary | J-evaluate-compozy-beta / REL-beta-installer-provenance | Dora | Feature Tour | Pending | | |
| 224 | CH-electron-install-docs-canary | J-evaluate-compozy-beta / REL-beta-self-update | Dora | Feature Tour | Pending | | |
| 225 | CH-desktop-release-rehearsal | J-publish-compozy-beta / REL-channel-repair-known-good | Dora | Garbage Tour | Pending | | |
| 226 | CH-untested-072-retire-workspace-bruno | J-retire-workspace / RT-008 | Bruno | Back-Button Tour | Pending | | |
| 227 | CH-016 | J-13 / RT-013 | Théo | Multi-Tab Tour | Pending | | |
| 228 | CH-019 | J-11 / RT-015 | Théo | Back-Button Tour | Pending | | |
| 229 | CH-016 | J-13 / RT-018 | Théo | Multi-Tab Tour | Pending | | |
| 230 | CH-014 | J-11 / RT-024 | Théo | Interrupt Tour | Fixed | HTTP/UDS/Web and restart parity; commit 4f5ae5298 | Both linked bugs fixed |
| 231 | CH-untested-020-22-dora | J-administer-provider-auth / RT-026 | Dora | Back-Button Tour | Pending | | |
| 232 | CH-untested-039-32-ada | J-32 / RT-030 | Ada | Back-Button Tour | Pending | | |
| 233 | CH-untested-039-32-ada | J-32 / RT-032 | Ada | Back-Button Tour | Pending | | |
| 234 | CH-untested-062-manage-sandbox-profiles-dora | J-manage-sandbox-profiles / RT-037 | Dora | Feature Tour | Pending | | |
| 235 | CH-untested-011-11-theo | J-11 / RT-039 | Théo | Network Tour | Pending | | |
| 236 | CH-background-session-switch | J-11 / RT-041 | Théo | Interrupt Tour | Pending | | |
| 237 | CH-014 | J-11 / RT-043 | Théo | Interrupt Tour | Pending | | |
| 238 | CH-untested-valid-003-12-theo | J-12 / RT-044 | Théo | Network Tour | Pending | | |
| 239 | CH-background-session-switch | J-11 / RT-045 | Théo | Interrupt Tour | Pending | | |
| 240 | CH-015 | J-12 / RT-046 | Nia | Network Tour | Pending | | |
| 241 | CH-017 | J-14 / RT-048 | Rafa | Feature Tour | Pending | | |
| 242 | CH-018 | J-15 / RT-051 | Ada | Feature Tour | Pending | | |
| 243 | CH-021 | J-14 / RT-052 | Rafa | Garbage Tour | Pending | | |
| 244 | CH-017 | J-14 / RT-053 | Rafa | Feature Tour | Pending | | |
| 245 | CH-017 | J-14 / RT-055 | Rafa | Feature Tour | Pending | | |
| 246 | CH-017 | J-14 / RT-056 | Rafa | Feature Tour | Pending | | |
| 247 | CH-016 | J-13 / RT-058 | Théo | Multi-Tab Tour | Pending | | |
| 248 | CH-016 | J-13 / RT-059 | Théo | Multi-Tab Tour | Pending | | |
| 249 | CH-029 | J-31 / RT-069 | Bruno | Feature Tour | Pending | | |
| 250 | CH-helix-one-kickoff-bruno | J-one-kickoff-collaboration / RT-073 | Bruno | Feature Tour | Pending | | |
| 251 | CH-untested-valid-013-30-bruno | J-30 / RT-074 | Bruno | Feature Tour | Pending | | |
| 252 | CH-untested-valid-013-30-bruno | J-30 / RT-075 | Bruno | Feature Tour | Pending | | |
| 253 | CH-untested-valid-014-31-bruno | J-31 / RT-078 | Bruno | Back-Button Tour | Pending | | |
| 254 | CH-untested-valid-015-32-ada | J-32 / RT-080 | Ada | Back-Button Tour | Pending | | |
| 255 | CH-untested-036-30-ada | J-30 / RT-083 | Ada | Feature Tour | Pending | | |
| 256 | CH-untested-037-31-bruno | J-31 / RT-agent-detail-runtime-live-edit | Bruno | Back-Button Tour | Pending | | |
| 257 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / RT-compozy-global-database | Bruno | Garbage Tour | Pending | | |
| 258 | CH-gateway-provider-degradation | J-expose-and-pair-gateway / RT-connectivity-provider-route | Dora | Network Tour | Pending | | |
| 259 | CH-drain-without-loss | J-drain-daemon-safely / RT-daemon-drain-admission | Dora | Interrupt Tour | Pending | | |
| 260 | CH-gateway-remote-cli-interruption | J-expose-and-pair-gateway / RT-gateway-browser-stream-reconnect | Iris | Interrupt Tour | Pending | | |
| 261 | CH-gateway-no-device-recovery | J-expose-and-pair-gateway / RT-gateway-no-device-recovery | Iris | Back-Button Tour | Pending | | |
| 262 | CH-gateway-mid-delivery-exposure | J-deliver-through-public-gateway / RT-gateway-offline-delivery-redelivery | Bruno | Network Tour | Pending | | |
| 263 | CH-gateway-provider-degradation | J-expose-and-pair-gateway / RT-gateway-operator-surface-truth | Iris | Network Tour | Pending | | |
| 264 | CH-gateway-no-device-recovery | J-expose-and-pair-gateway / RT-gateway-paired-device | Iris | Back-Button Tour | Pending | | |
| 265 | CH-gateway-no-device-recovery | J-expose-and-pair-gateway / RT-gateway-public-ui-consent | Iris | Back-Button Tour | Pending | | |
| 266 | CH-gateway-remote-cli-interruption | J-operate-remote-gateway-cli / RT-gateway-remote-cli-profile | Iris | Interrupt Tour | Pending | | |
| 267 | CH-untested-valid-023-offer-runnable-capabilities-dora | J-offer-runnable-capabilities / RT-mcp-dead-recovery | Dora | Feature Tour | Pending | | |
| 268 | CH-untested-066-operate-daemon-schema-ada | J-operate-daemon-schema / RT-migrate-memory-stream-when-disabled | Ada | Garbage Tour | Pending | | |
| 269 | CH-prune-missing-workspace | J-prune-missing-workspace / RT-missing-workspace-pruned | Bruno | Interrupt Tour | Pending | | |
| 270 | CH-provider-runtime-strategies | J-17 / RT-openclaw-provider-managed-runtime | Théo | Feature Tour | Pending | | |
| 271 | CH-untested-067-operate-daemon-schema-bruno | J-operate-daemon-schema / RT-preserve-corrupt-database-family | Bruno | Garbage Tour | Pending | | |
| 272 | CH-crash-resume-compaction | J-11 / RT-pressure-context-compaction | Théo | Interrupt Tour | Pending | | |
| 273 | CH-database-refusal-recovery | J-operate-daemon-schema / RT-refuse-ahead-database | Bruno | Garbage Tour | Pending | | |
| 274 | CH-untested-067-operate-daemon-schema-bruno | J-operate-daemon-schema / RT-refuse-cross-stream-legacy-marker | Bruno | Garbage Tour | Pending | | |
| 275 | CH-database-refusal-recovery | J-operate-daemon-schema / RT-refuse-legacy-cli-open | Ada | Garbage Tour | Pending | | |
| 276 | CH-database-refusal-recovery | J-operate-daemon-schema / RT-refuse-legacy-database | Bruno | Garbage Tour | Pending | | |
| 277 | CH-untested-067-operate-daemon-schema-bruno | J-operate-daemon-schema / RT-refuse-legacy-session-database | Bruno | Garbage Tour | Pending | | |
| 278 | CH-gateway-audit-teardown | J-keep-secrets-contained / RT-secret-redaction-boundary | Dora | Feature Tour | Pending | | |
| 279 | CH-crash-resume-compaction | J-11 / RT-session-context-rebuild | Théo | Interrupt Tour | Pending | | |
| 280 | CH-truthful-cost-provenance | J-14 / RT-session-cost-provenance | Rafa | Money Tour | Pending | | |
| 281 | CH-session-affordances-truth | J-11 / RT-session-cwd-resume | Théo | Feature Tour | Pending | | |
| 282 | CH-untested-valid-002-11-bruno | J-11 / RT-session-delete-owned-history | Bruno | Network Tour | Pending | | |
| 283 | CH-session-affordances-truth | J-11 / RT-session-lifecycle-affordances | Théo | Feature Tour | Pending | | |
| 284 | CH-herdr-session-orchestration | J-15 / RT-session-native-stop | Ada | Interrupt Tour | Pending | | |
| 285 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-session-spawn-wake | Cora | Feature Tour | Pending | | |
| 286 | Charter mapping pending | J-15 / RT-spawn-ttl-cleanup | Ada |  | Pending | | |
| 287 | CH-subprocess-health-recovery | J-diagnose-task-session-health / RT-subprocess-health-escalation | Ada | Feature Tour | Pending | | |
| 288 | CH-herdr-attention-signals | J-respond-to-agent-attention / RT-web-attention-toast-delivery | Cora | Feature Tour | Pending | | |
| 289 | CH-background-session-switch | J-11 / RT-workspace-active-session-badge | Théo | Interrupt Tour | Pending | | |
| 290 | CH-workspaces-command-switcher | J-operate-workspace-context / RT-workspace-overview-command-tab | Ada | Feature Tour | Pending | | |
| 291 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-web-nested-navigation | Ada | Feature Tour | Pending | | |
| 292 | CH-untested-valid-022-network-local-default-bruno | J-network-local-default / TA-001 | Bruno | Network Tour | Pending | | |
| 293 | CH-untested-valid-022-network-local-default-bruno | J-network-local-default / TA-004 | Bruno | Network Tour | Pending | | |
| 294 | CH-untested-024-24-bruno-part-1 | J-24 / TA-017 | Bruno | Garbage Tour | Pending | | |
| 295 | CH-untested-024-24-bruno-part-1 | J-24 / TA-018 | Bruno | Garbage Tour | Pass | task-diagnostics-proof.json | CLI/HTTP/UDS and Web Inspect agree after reopen/reload. |
| 296 | CH-untested-024-24-bruno-part-1 | J-24 / TA-019 | Bruno | Garbage Tour | Pass | task-diagnostics-proof.json | CLI/HTTP/UDS and Web Inspect agree after reopen/reload. |
| 297 | Charter mapping pending |  / TA-021 | Bruno |  | Pending | | |
| 298 | CH-untested-024-24-bruno-part-1 | J-24 / TA-022 | Bruno | Garbage Tour | Pending | | |
| 299 | CH-untested-024-24-bruno-part-1 | J-24 / TA-023 | Bruno | Garbage Tour | Pending | | |
| 300 | CH-untested-050-bound-runaway-work-ada | J-bound-runaway-work / TA-024 | Ada | Feature Tour | Pending | | |
| 301 | CH-loop-task-recovery-binding | J-24 / TA-033 | Bruno | Feature Tour | Pending | | |
| 302 | CH-untested-027-24-marina | J-24 / TA-039 | Marina | Garbage Tour | Pending | | |
| 303 | CH-038 | J-24 / TA-040 | Marina | Feature Tour | Pending | | |
| 304 | CH-untested-025-24-bruno-part-2 | J-24 / TA-044 | Bruno | Garbage Tour | Pending | | |
| 305 | CH-untested-025-24-bruno-part-2 | J-24 / TA-047 | Bruno | Garbage Tour | Pending | | |
| 306 | CH-untested-050-bound-runaway-work-ada | J-bound-runaway-work / TA-050 | Ada | Feature Tour | Pending | | |
| 307 | CH-038 | J-24 / TA-052 | Bruno | Feature Tour | Pending | | |
| 308 | CH-automation-manual-trigger | J-24 / TA-053 | Bruno | Feature Tour | Pending | | |
| 309 | CH-038 | J-24 / TA-054 | Bruno | Feature Tour | Pending | | |
| 310 | CH-producer-backed-trigger-events | J-create-and-activate-trigger / TA-057 | Bruno | Garbage Tour | Pending | | |
| 311 | CH-untested-026-24-dora | J-24 / TA-061 | Dora | Garbage Tour | Pending | | |
| 312 | CH-untested-026-24-dora | J-24 / TA-062 | Dora | Garbage Tour | Pending | | |
| 313 | CH-untested-010-09-ada | J-09 / TA-063 | Ada | Feature Tour | Pending | | |
| 314 | CH-untested-010-09-ada | J-09 / TA-064 | Ada | Feature Tour | Pending | | |
| 315 | CH-untested-valid-009-24-ada | J-24 / TA-065 | Ada | Garbage Tour | Pending | | |
| 316 | CH-untested-010-09-ada | J-09 / TA-066 | Ada | Feature Tour | Pending | | |
| 317 | CH-untested-008-07-ada | J-07 / TA-068 | Ada | Feature Tour | Pending | | |
| 318 | CH-untested-008-07-ada | J-07 / TA-075 | Ada | Feature Tour | Pending | | |
| 319 | CH-untested-008-07-ada | J-07 / TA-077 | Ada | Feature Tour | Pending | | |
| 320 | CH-untested-008-07-ada | J-07 / TA-078 | Ada | Feature Tour | Pending | | |
| 321 | CH-untested-008-07-ada | J-07 / TA-079 | Ada | Feature Tour | Pending | | |
| 322 | CH-untested-001-01-ada | J-01 / TA-081 | Ada | Feature Tour | Pending | | |
| 323 | CH-untested-002-01-bruno | J-01 / TA-082 | Bruno | Feature Tour | Pending | | |
| 324 | CH-untested-002-01-bruno | J-01 / TA-083 | Bruno | Feature Tour | Pending | | |
| 325 | CH-untested-005-05-bruno | J-05 / TA-085 | Bruno | Back-Button Tour | Pending | | |
| 326 | CH-untested-007-06-bruno | J-06 / TA-086 | Bruno | Back-Button Tour | Pending | | |
| 327 | CH-untested-032-27-marina-part-1 | J-27 / TA-088 | Marina | Garbage Tour | Pending | | |
| 328 | CH-untested-032-27-marina-part-1 | J-27 / TA-089 | Marina | Garbage Tour | Pending | | |
| 329 | CH-untested-032-27-marina-part-1 | J-27 / TA-090 | Marina | Garbage Tour | Pending | | |
| 330 | CH-untested-032-27-marina-part-1 | J-27 / TA-091 | Marina | Garbage Tour | Pending | | |
| 331 | CH-untested-034-28-bruno | J-28 / TA-092 | Bruno | Feature Tour | Pending | | |
| 332 | CH-untested-029-26-bruno | J-26 / TA-093 | Bruno | Feature Tour | Pending | | |
| 333 | CH-untested-030-26-lea | J-26 / TA-094 | Lea | Feature Tour | Pending | | |
| 334 | CH-untested-030-26-lea | J-26 / TA-095 | Lea | Feature Tour | Pending | | |
| 335 | CH-untested-035-29-ada | J-29 / TA-096 | Ada | Feature Tour | Pending | | |
| 336 | CH-untested-035-29-ada | J-29 / TA-097 | Ada | Feature Tour | Pending | | |
| 337 | CH-untested-035-29-ada | J-29 / TA-098 | Ada | Feature Tour | Pending | | |
| 338 | CH-untested-032-27-marina-part-1 | J-27 / TA-099 | Marina | Garbage Tour | Pending | | |
| 339 | CH-untested-032-27-marina-part-1 | J-27 / TA-101 | Marina | Garbage Tour | Pending | | |
| 340 | CH-untested-031-27-bruno | J-27 / TA-102 | Bruno | Garbage Tour | Pending | | |
| 341 | CH-untested-032-27-marina-part-1 | J-27 / TA-103 | Marina | Garbage Tour | Pending | | |
| 342 | CH-untested-030-26-lea | J-26 / TA-104 | Lea | Feature Tour | Pending | | |
| 343 | CH-untested-032-27-marina-part-1 | J-27 / TA-105 | Marina | Garbage Tour | Pending | | |
| 344 | CH-untested-033-27-marina-part-2 | J-27 / TA-106 | Marina | Garbage Tour | Pending | | |
| 345 | CH-untested-015-14-marina | J-14 / TA-107 | Marina | Feature Tour | Pending | | |
| 346 | CH-runaway-work-bounded | J-bound-runaway-work / TA-action-run-liveness | Ada | Garbage Tour | Pending | | |
| 347 | CH-automation-crud-loop-target | J-24 / TA-automation-crud-loop-target | Bruno | Garbage Tour | Pending | | |
| 348 | CH-suggestions-consent | J-24 / TA-automation-suggestions | Bruno | Feature Tour | Pending | | |
| 349 | CH-loop-quarantine-repair | J-bound-runaway-work / TA-loop-failure-breaker | Ada | Recovery Tour | Pending | | |
| 350 | CH-task-tree-loop-rollup | J-complete-task-tree / TA-parent-rollup-completion | Bruno | Feature Tour | Pending | | |
| 351 | CH-schedule-recovery-guard | J-24 / TA-schedule-catchup-overlap | Bruno | Interrupt Tour | Pending | | |
| 352 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-task-create-async-activation | Bruno | Feature Tour | Pending | | |
| 353 | CH-truthful-cost-provenance | J-24 / TA-task-run-cost-provenance | Bruno | Money Tour | Pending | | |
| 354 | CH-wake-dedup-stress | J-operate-bounded-task-capacity / TA-task-wake-dedup | Ada | Garbage Tour | Pending | | |
| 355 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-terminal-run-inspect | Bruno | Feature Tour | Pending | | |
| 356 | CH-untested-051-complete-task-tree-bruno | J-complete-task-tree / TA-web-task-detail-redesign | Bruno | Feature Tour | Pending | | |
| 357 | CH-workspace-run-capacity | J-operate-bounded-task-capacity / TA-workspace-run-capacity | Ada | Feature Tour | Pending | | |

## Session Debriefs

### Bootstrap and ACP canary — Dora / Ada — completed

- Built the exact worktree with `make build-go` and root `bunx turbo run build --filter=compozy-web`. The latter reused three matching cache records and passed codegen; existing CSS highlight, dynamic-import and chunk-size warnings are retained in the build log, not represented as a zero-warning gate.
- Manifest: `/Users/pedronauck/dev/qa-labs/compozy-qa-execution-unblock-20260910-215445-053583-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- Daemon PID 31065 serves `web/dist` from this worktree at `http://127.0.0.1:60290`; native provider HOME remains the operator HOME. `project/` is the registered root; evidence stays outside it.
- Initial unconfigured `general` session creation refused because no default provider existed; configured `defaults.provider=codex` through CLI and restarted. Unsupported `defaults.model` and `defaults.reasoning_effort` CLI paths were rejected without mutation; runtime fields were instead set on the authored workspace agent using documented flags.
- Public live catalog advertises Luna/xhigh. Created `field-writer` through CLI and an unbound Local session `sess-c39bcd56be145c66`; prompted once with a workspace-read and kickoff-file objective. Public HTTP session detail independently reports effective `codex/gpt-5.6-luna/xhigh` and advertised ACP option `reasoning_effort=xhigh`.
- CLI `session status` and `session inspect` omit runtime selection, unlike the official skill's status guidance. Record as an unresolved guidance/surface finding for the relevant inventory slice; HTTP session detail supplies the current truth.
- Evidence directory: `docs/qa/evidence/2026-09-10-qa-execution-unblock/`. Captured `daemon-start-current-web.json`, `codex-models.json`, `writer-create.json`, `canary-new-configured.json`, `canary-session-http.json`, status and event reads. Prompt completed in 139.604 seconds with end_turn. Native workspace/tool discovery plus terminal file creation and read succeeded. Independent file bytes contain ws_fc0dcf020b8034f1; HTTP detail reports done and Luna/xhigh. Web shows the same completed transcript and survives reload. Additional evidence: canary-prompt.json, canary-tools.json, canary-file-read.json, canary-settled-http.json, canary-history.json, canary-web-settled.png. This proves the provider prerequisite; it does not award unrelated complete scenarios.
- Browser driver: isolated `agent-browser --session qa-execution-unblock`. Setup model selected as Codex / GPT-5.6-Luna / Extra high; native CLI auth; existing disposable workspace retained. This is preparation, not an onboarding scenario pass.


## What Was Fixed

Disabled session memory now returns the existing unsupported capability response and renders a truthful Web state. The local patch and real re-walk are validated; no commit exists yet. See BUG-20260910-disabled-session-ledger.

## Paper Cuts

Not yet assessed.

## Runtime Errors Observed

Not yet assessed.

## Human Verifications Needed

Reserved rows require individual reassessment; handoff categories are not current blockers.

## Decisions for a Human

Not yet assessed.

## Learnings

Runtime isolation and provider home policy are independent. Native terminal command and args are separate fields; shell command strings are executable names, not shell programs.

## Final Status

In progress. No release-readiness claim. Of 357 original rows, 13 have evidence-backed dispositions: 8 verified, 2 fixed-and-verified, 1 unresolved defect, and 2 external runtime blockers. The remaining 344 are pending. The strict whole-run evidence audit is not yet passing. Current cursor and process handles are in progress.json; dated checkpoints below retain historical counts.

## RT-024 interrupted return walk — defect and repair in progress

The real canary completed and was stopped through CLI. HTTP and UDS health/status/inspect agreed on active then stopped state, and invalid boolean queries returned 400. Web Usage/Memory/Files/Vault were opened. The stopped Memory panel exposed [BUG-20260910-disabled-session-ledger](../bugs/BUG-20260910-disabled-session-ledger.md): memory disabled yields 500 `memory.internal`. Evidence: `ledger-disabled-http.json`, `ledger-disabled-uds.json`, and `rt024-ledger-failure.png` under the run evidence directory. The persona walk ended for diagnosis; no full RT-024 pass is claimed. Configuration/policy digest and restart branches remain to be settled.

### Ledger repair verification checkpoint

The factory now returns an absent service interface for disabled/unconfigured memory. The Web recognizes only HTTP501 `memory.unsupported` as unavailable; other server errors stay errors. The regression failed before repair (Go500 rather than501; adapter wrong error type). Race-enabled daemon coverage passes disabled, missing-root and configured cases. Root Turbo session adapter/inspector tests pass92/92, Web typecheck/build pass, and React Doctor reports100/100 without findings. Existing build warnings remain as in the baseline. `make gate` is queued for a machine slot.

After restarting the corrected daemon (PID59080), HTTP/UDS both report501 `memory.unsupported`. The old stopped session survives with consistent health/status/inspect and invalid-query400; reloaded Web shows the honest unavailable explanation (`ledger-fixed-web.png`). Fresh session `sess-f4ff28e4f5f576a6` is repeating the native-terminal read with effective Codex/gpt-5.6-luna/xhigh, and its active Memory panel shows the existing not-yet-materialized placeholder (`ledger-retake-active-web.png`); the active query is deferred. The stopped-state retake is described below. Evidence lives in `docs/qa/evidence/2026-09-10-qa-execution-unblock/`, including `ledger-go-red.txt`, `ledger-web-red.txt`, `ledger-go-green.txt`, `ledger-web-green.txt`, `ledger-build-go.txt`, `ledger-build-web.txt`, `ledger-react-doctor.txt`, and the public-request JSON records.

### Fresh stopped-session retake and terminal-input diagnosis

Fresh session `sess-f4ff28e4f5f576a6` used real Codex Luna xhigh. The agent sent shell command lines as `command` rather than executable plus `args`, producing missing-executable failures. Native `pwd` and `ls` succeeded; operator `cat` with a separate file argument independently read the unchanged workspace ID. This is an agent input failure, not evidence that terminal execution is unavailable. After repeated invalid attempts, the operator canceled the prompt and stopped the session (verified, forced after2s). No successful file-reading completion is claimed for this retake.

The stopped Memory inspector now shows the truthful unavailable state (`ledger-retake-stopped-web.png`), and HTTP/UDS both return501 `memory.unsupported` for the fresh session. This confirms the original ledger defect is repaired locally; commit/gate and the rest of RT-024 remain pending. Evidence includes `ledger-retake-tool-inputs.json`, `ledger-retake-tool-results.json`, `terminal-whole-command-probe.json`, `terminal-operator-read.json`, `ledger-retake-bounded-stop.json`, and `ledger-retake-stop.json`.

### Enabled ledger, delivery gate, and remaining policy defect

The enabled-memory positive control `sess-a8ac593ecc5dd73f` completed a native `cat` with separate arguments in62.992s and effective Luna/xhigh. After stop, Memory showed the expected workspace/root/lifecycle metadata; the60-event ledger was identical over HTTP and UDS and after daemon restart. Evidence: `ledger-enabled-prompt.json`, `ledger-enabled-settled-http.json`, `ledger-enabled-web.png`, `ledger-enabled-stopped-http.json`, `ledger-enabled-stopped-uds.json`, `ledger-positive-restarted-http.json`. Extraction/controller roles were disabled during this preparation; no extraction-role acceptance is claimed.

`make gate` passed all affected lanes: Go lint0issues, race-enabled daemon tests, codegen, Web lint/typecheck and7,189 Web tests across771 files. See `ledger-make-gate.txt`. The first patch has no pending technical validation.

RT-024 remains unfinished because [the profile-agent Heartbeat finding](../bugs/BUG-20260910-profile-agent-heartbeat-missing.md) prevents policy/config correlation. The agent exists and runs, but Heartbeat status cannot find the profile-owned definition. Its repair is the next narrow blocker before the broader containment walks.

Evidence remains local in the designated Skeeper-managed directory. The contract prohibits pushes, so no sidecar sync was attempted. The read-only `skeeper status --json` also failed on this worktree's `worktreeconfig` repository extension; no shared Git configuration was changed.

### User-directed provider change, 2026-09-10 22:49 UTC

Subsequent ordinary-agent walks use Cursor Agent with Grok 4.6 High Fast, superseding the initial Codex Luna xhigh requirement. `cursor-agent` version `2026.09.08-6caf4ff` lists native alias `cursor-grok-4.6-high-fast`. The refreshed Compozy catalog normalizes this to `grok-4.6`, reasoning `high`, speed `fast`. The guarded field-writer definition update persisted these settings; historical sessions retain their original runtime evidence. New session `sess-1b77ee9f015e422e` reached runtime `ready`, with effective settings matching the request and ACP current model `grok-4.6[effort=high,fast=true]` (`cursor-canary-runtime.json`). Its bounded native-tool/file canary is in progress. Default provider was written only in the isolated lab config and requires the next daemon restart. No operator credentials or account state were changed.

The Cursor canary completed in 73.572 seconds (exit 0). Native `compozy__terminal_exec` performed the read/write operations; the agent additionally used Cursor file reads to inspect content. Independent filesystem read confirms `cursor-check.md` contains `ws_fc0dcf020b8034f1`, SHA-256 `2c57444cd2961513b3e3410c4de16a2e0c1e1873a02b40c2fc1fdd56e6e01398`. Evidence: `cursor-canary-prompt.json`, `cursor-canary-tool-calls.json`, `cursor-canary-file-read.json`, `cursor-canary-stop.json`. This establishes the new managed-provider prerequisite, not a blanket scenario verdict.

### Vault deletion walk — MS-041 passed

Dora completed the live Web delete journey with disposable provider/session refs. Empty/wrong typed confirmation could not delete; cancel preserved the provider entry; the exact ref enabled deletion and independent UDS metadata returned 404. Session-scoped deletion displayed a simple Confirm dialog without typing, then returned 404 on metadata read. A second provider target disappeared via UDS DELETE (204) while the Web dialog remained open; confirming/revalidation closed the stale target and the final HTTP list plus browser reload were empty. No unintended or duplicate mutation remained. Evidence: `vault-delete-confirm-empty.txt`, `vault-delete-confirm-wrong.txt`, `vault-after-cancel.json`, `vault-delete-confirm-correct.png`, `vault-retire-deleted.json`, `vault-delete-session-immediate.txt`, `vault-session-delete-confirmed.json`, `vault-disappear-open.txt`, `vault-disappear-delete.json`, `vault-disappear-settled.txt`, `vault-final-list.json`, `vault-final-reload.txt`.

The tracker word “immediate” was reconciled to simple confirmation, matching the unchanged session dialog shipped in original beta commit `8eeb8a3813`; this is a wording correction, not a product relaxation. Adjacent list/store operations were preparation only and their unrelated status files were preserved. Current finalized original inventory rows: 1 verified, 356 pending; RT-024 live repair is verified but delivery bookkeeping/gate remains in progress.

### RT-024 final retest and local repair delivery

Both fixes are committed locally as `4f5ae5298`. The profile source regression passed the full core race suite; `make gate` passed all affected API/daemon and Web lanes with zero lint warnings/errors. The first gate attempt caught formatter drift; the repository formatter corrected it and the retry passed. Pre-commit tasks were run explicitly with `lint-staged --no-stash` to honor the user's no-stash rule; commitlint passed, then automatic hook re-execution was disabled solely to avoid the hook's default Git stash. No required check was skipped and nothing was pushed.

The full live RT-024 retest passed: HTTP and UDS health/status/inspect agree except their per-read timestamps, 400 invalid-bool and 404 missing-session bodies match, the valid missing-policy state exposes configuration correlation, and the inspector's enabled/disabled Memory branches are truthful. Restart preserves inspect state and all 60 ledger events (`profile-patch-parity-summary.json`). Proof includes the actual pre-change Luna runs; the provider change does not relabel them. Current finalized inventory: 1 verified (MS-041), 1 fixed and verified (RT-024), 355 pending. A separate native Heartbeat adjacent canary is running with Cursor to assess the independent native resolution path.

### Native Heartbeat follow-up closed

The real Cursor adjacent canary exposed the native handler's independent unprofiled lookup (`cursor-policy-prompt.json`). Commit `f32f50a9c` carries the caller's profile into the existing native resolver for status/wake. Its canonical native-tools test uses real files with different policies in default/marketing profiles. Focused coverage, Go build, and `make gate` passed (`native-heartbeat-profile-green.txt`, `native-heartbeat-profile-build.txt`, `native-heartbeat-profile-gate.txt`). The managed Cursor retake returned the correct `missing` status with no retry; runtime still confirms Grok4.6/high/fast, and the session was verified stopped. This closes the broader profile-agent finding after the earlier core/API fix. The unchanged package-owned catalog path is not a claimed result of this filesystem-source repair.

Cross-workspace preparation now contains two disposable roots: field-notes and field-archive (`ws_6948d9a869661f6c`). Four agent definitions use the requested Cursor runtime: field-writer (approve-all), archive-helper (approve-all), field-deny (deny-all), field-reader (approve-reads). A ready archive task `task-1a7ec21a5a1987d3` and queued run `run-3f438f979925fc60` were created publicly; the first approve-all crossing prompt is in progress. This is preparation, not a matrix pass.

### Cross-workspace approve-all walk interrupted for diagnosis

Actual Cursor session `sess-35af9e34a61e15e4` inspected the target archive successfully, then reported no claimable task runs for exact queued run `run-3f438f979925fc60`; independent operator read confirmed it remained queued. Native child spawn returned MCP -32602 output-schema validation errors (missing session_id/spawn_role/spawn_depth/ttl_expires_at and extra properties), and the target workspace session list remained empty. A bounded same-surface retry did not create a child. Managed terminal coordination execution reported completion but the agent received no JSON in visible text. Evidence: `cross-all-prompt.json`, `cross-all-message.json`, `cross-all-run-status.json`, `cross-all-target-sessions.json`, `cross-all-stop.json`. No full permission-matrix verdict is claimed. The failed walk ended for developer diagnosis; source findings are not replacement acceptance evidence.

Cross-claim diagnosis correction: the task's background general-role dispatcher was active. It later claimed and completed run `run-3f438f979925fc60` in `sess-bbd30d1a5a881adc`; the original cross caller's no-claim result is not yet a product defect or permission verdict. Actor profile scope is already bound in `autonomyActorContext`; the early missing-profile hypothesis was rejected. The general role's ACP option was Grok4.6/high/fast, but its inherited runtime metadata was underspecified, so the editable global `general` agent is now explicitly pinned to cursor/grok-4.6/high/fast for all later automatic runs (`cursor-general-pinned.json`). The scheduler was paused through CLI after completion to avoid background dispatch racing further fixture preparation. No active task claims remained (`cross-dispatch-pause.json`, `cross-run-current-final.json`). This completed automatic task is retained as a separate observed run, not a substitute for the cross-seam matrix.

## MCP text-result correction checkpoint

The preceding provider-steering acknowledgment rechecked existing evidence but made no new execution progress. This continuation reproduced and repaired the missing structured JSON in hosted MCP text results. The narrow fix preserves existing preview and structured content. See [BUG-20260910-hosted-mcp-text-result](../bugs/BUG-20260910-hosted-mcp-text-result.md) for the owning impact audit and canonical regression. The full MCP race suite, codegen, and Go build passed. No generated drift occurred. The test-conventions heuristic flags the pre-existing `TestRunHostedProxyUsesPrivateTTLCacheForProjectionChanges`, outside the edited helper subtest; no unrelated rewrite was made. Owning Go lint passed.

Fresh Cursor session `sess-ab20fe5bde29aefc` is repeating foreign workspace inspection and a known-file terminal read without provider fallback. Runtime evidence confirms `cursor/grok-4.6/high/fast` and native ACP `grok-4.6[effort=high,fast=true]`. Gate and live retest remain in progress. All 357 rows remain accounted for in the inventory; this adjacent repair does not itself finalize the cross-workspace matrix.

Scope correction: the matrix requires foreign spawn through its supported CLI/HTTP/UDS seams. The current native spawn descriptor has no foreign-workspace selector; the earlier attempt to use that tool for a foreign-only agent does not alone justify adding a new native input. The masked schema diagnostic still needs independent validation. The task run completed after an automatic worker claimed it; the earlier no-claimable response is not a proven defect.

MCP text repair completed in local commit `ea8f4c3f3`. `make gate` passed all affected lanes. Real Cursor retest matched the independently read target file and foreign workspace name/path; session stop verified. Proof: `mcp-text-retest-proof.json`. Provider file reads were limited to its own skill-result artifacts. Pre-commit tasks ran using `lint-staged --no-stash`, and commitlint passed; automatic Husky invocation was disabled only to avoid its forbidden Git stash behavior. A new bounded session `sess-4cbe38a56cc29a66` is isolating the spawn-error diagnostic without requesting a foreign native workspace.

The isolated missing-agent retest reproduced the masked spawn diagnostic and ended. [BUG-20260910-cursor-mcp-error-schema](../bugs/BUG-20260910-cursor-mcp-error-schema.md) records the open interop limitation and matching upstream client issue. Removing structured error data or broadly relaxing server output schemas would change the public contract; no such workaround was applied. The exact embedded Cursor SDK version is unknown. Continue supported CLI/HTTP/UDS branches while retaining this failed native diagnostic branch.

## Managed terminal identity repair

The supported CLI retake `sess-27d6aca7a434c19b` failed spawn with exit64 identity_required and returned coordination data without agent identity. It ended and was stopped before diagnosis. [BUG-20260910-terminal-agent-identity](../bugs/BUG-20260910-terminal-agent-identity.md) owns the scope/impact record. Terminal process starts now project their validated originating Actor into existing managed-session environment fields. Canonical process-boundary coverage reproduced failures for exec, open, and pipe, then passed; the full terminal race suite and test conventions passed. Windows cross-compilation passed, without Windows execution. Build, gate, ACP adjacency and real retake remain pending.


## Main integration and test reconciliation

The managed terminal identity repair completed before this rebase: full terminal race/adjacent ACP checks, Windows cross-compilation, gate, and the real Cursor foreign-workspace CLI spawn/coordination retake passed. Parent/child sessions were stopped. `terminal-identity-retest-proof.json` records the actor-bound public grant evidence. Its original commit `e5a6b2cea` is now `dcb9d25a4` after rebase.

At the user's request, reviewed PRs 607–611 individually and the five incoming commits without associated PRs, then rebased onto main `fec0e9b08631dd9e2d7eaa66fc7f76f5314ae5c0`. No conflicts; all five local commits retain equal patches. The [integration impact report](2026-09-10-qa-execution-unblock/rebase-impact.md) maps each change to affected journeys, backup provenance, commit mappings, and validation evidence.

The same lab upgraded to schema109 with stable pre-existing records verified. Go/Web builds, targeted daemon integration, scoped Dream checks, and a fresh two-turn Cursor Grok4.6 High Fast canary passed. Browser smoke covers the rebuilt mixed transcript disclosure and empty attention state. Initial gate exposed six stale test assumptions from the upstream integration; three existing canonical suites were reconciled against the merged contracts and all188 focused tests passed. Full gate retry passed: all affected lanes green, including 771 Web test files / 7205 tests, typecheck and lint (`rebase-gate-retest.txt`).

This integration checkpoint adds no blanket QA verdicts: 2 original rows remain finalized and355 pending. Full notification journeys and external Tailnet proof remain separate; the Cursor MCP error-schema issue remains open.


## Cross-workspace restricted-provider walk

Ada's deny-all Cursor session `sess-bc315ce9f46d87a7` attempted native foreign workspace/memory reads and the four requested agent CLI commands through native terminal_exec. All were rejected at the provider permission stage with `User rejected`; no pending operator interaction and no workspace.access_denied audit were present. The provider prevented the underlying CLI execution, so no daemon exit77 evidence exists. The turn ended, and stop was requested. This fails the matrix's promised diagnostic path, without showing any access bypass. [Finding](../bugs/BUG-20260910-cursor-denial-hides-workspace-policy.md) records the precise boundary and prohibits a permission-relaxing workaround.

The attempted public config set for autonomy.scheduler.min_queued_age was rejected as unsupported; config get remains 2m0s. No configuration was changed. Scheduler remains paused with no queued runs or claims. Approve-reads session `sess-f5db3f09d3263d2e` is exercising the CLI seams separately. HTTP/UDS and all-mode task claim branches are not yet accepted.


## External platform prerequisites reassessed

Read both complete scenarios and their historical evidence qualifications. Current `uname -srm` reports Darwin25.6.0 arm64; macOS26.6.2. ET-terminal-windows-parity requires actual Windows local/sandbox terminal lifecycle; APP-appimage-fuseless-launch requires the exact released Linux x64 AppImage on FUSE3-only graphical Linux with isolated teardown. Neither can be established by this assigned lab. Their blocked-verify verdicts now reference `platform-prerequisites.json` with exact remaining runtime/artifact conditions. No claimed absence of every other operator machine, and no cross-compilation or older invalid package walk is relabeled as acceptance.

Current inventory dispositions: 1 verified, 1 fixed-and-verified, 1 unresolved defect (matrix branch failed; other seams continue), 2 external-runtime blockers, 352 pending. All357 original rows remain present.


Approve-reads CLI evidence: real session `sess-f5db3f09d3263d2e` completed with spawn, coordination status, and peers exit77 plus the prescribed permission-mode guidance. Independent terminal journal confirms all three command IDs/exits; three workspace.access_denied events name the actor, targetB, source denied, mode approve-reads and spawn/coordination seams (`cross-read-cli-proof.json`). Ordinary MCP/terminal allow-once approvals did not grant workspace consent. The first task-next call timed out while awaiting its second terminal approval; no CLI result is claimed. Its expired interaction still appeared in one later public read and rejecting it returned65; no successful late execution is claimed. One clean fresh-session retry is in progress as `sess-04e085f6d8b83faf`.

Scheduler was resumed publicly with no queued runs/claims, then a new archive task `task-c365b5cc279933d5` and exact queued run `run-5cf0824217f60483` were created. Approve-all session `sess-e4855b551031a4be` was promptly instructed to claim that run natively, before the configured2m scheduler escalation threshold. The task's real output is a single archive-handoff.md line; the operator will verify it through public terminal/file reads and task state. No claim token is requested in the response.

## Task claim identity and denial guidance repair

The clean claim attempt returned no work while its exact run remained queued; the scheduler claimed it only later. Diagnosis isolated stable workspace metadata IDs being passed to registration-ID task queries. A separate clean approve-reads task-next retry returned77 without the promised hint. The bounded repair resolves native claim aliases back to the registration ID and retains the permission error while appending the existing guidance. No permission or schema changes. [Finding and impact audit](../bugs/BUG-20260910-task-claim-workspace-identity.md).

The new real Cursor retakes passed: exact foreign-name run claim, target one-line file, native completion, and CLI denial with canonical guidance. Public task state, file read, terminal journal, grant/denial audits, runtime selection, and stopped sessions are captured in `claim-retest-proof.json`. The full matrix still fails the separate Cursor deny-all preemption branch.

Canonical regressions reproduced red then passed with race detection, full task race suite passed, Go build passed, and make gate passed all affected lanes (`claim-gate-retest.txt`). Initial formatter drift was corrected before retry. Seven pre-existing standalone test-shape heuristic findings in lease_test.go are outside this edit; the added test uses the existing owning suite. No further scenario count change:5 dispositioned,352 pending.

## Memory health walk — September11 continuation

Rafa resumed the MS-011 companion charter after the browser session reset; daemon47583 remained live with zero active managed sessions. Settings → Memory opens and reloads with persistence enabled, zero global memory files, no Dream runs, and a disabled Trigger dream control. CLI/HTTP/UDS valid workspace health agrees on status ok, one workspace file/indexed file, zero orphans, and Dream disabled. Global health includes two workspaces; its global file count remains zero. The Settings summary displays that global count.

The invalid-workspace branch instead returns500 memory.internal: HTTP hides the underlying message, while UDS reports workspace not found. A fresh HTTP retry reproduces it. Evidence: health-http-invalid.json, health-uds-invalid.json, health-http-invalid-retry.json; valid and Web observations use the health-* evidence prefix. The persona walk ends here for diagnosis; no health pass is awarded yet.

## Task and run diagnostics — verified

Bruno completed TA-018 and TA-019 under CH-untested-024-24-bruno-part-1. The archive task executed earlier by real Cursor/Grok remains completed after daemon restart. Task Inspect displays terminal next action, current run, stopped session and active scheduler. Run Inspect shows absent finished lease timestamps, the token hash only, and the correct idempotency key. Independent CLI/HTTP/UDS diagnostics match exactly excluding their as_of timestamps. Reopen/reload retains the display; missing task/run IDs produce404. Evidence: task-diagnostics-proof.json and its linked inspected screenshots. The persona slice ends without mutation. TA-023 run detail was observed but review-bearing behavior is not yet settled; TA-022 filter-before-limit requires its own multi-run walk.

The health gate passed core/HTTP/UDS but hit one TempDir cleanup failure in the unchanged catalog composition test. Three targeted repetitions passed; the gate retry is running. No test was weakened or replaced. Current original inventory:3 verified,1 fixed-and-verified,1 unresolved defect,2 external-runtime blockers,350 pending.

## Memory health repair delivered locally

Rafa's real replay closes MS-011: browser summary/reload, matching valid CLI/HTTP/UDS health after restart, and corrected missing-workspace404 diagnostics. The existing Dream role matrix remains valid for unchanged combinations. Full owning memory race suite and test-shape check passed. The first gate hit a catalog fixture TempDir cleanup failure; three targeted reproductions passed, then the full gate retry passed all affected lanes. No unrelated test change.

Current inventory:3 verified,2 fixed-and-verified,1 unresolved defect,2 external-runtime blockers,349 pending (357total). Claim fix is db933e045. The Goal judge's lab-only delivery runtime defaults are now explicitly cursor/grok-4.6/high/fast through four validated scalar config writes; the unsupported whole-object write changed nothing. No Goal execution is claimed yet.

## First live Goal with Cursor — partial charter

Lea entered the Web composer with field-writer. Bare /goal produced Internal Server Error in the Web, while the CLI reported a missing objective. Independent Goal status stayed null. The valid objective then created exactly one session-origin Run looprun-04abce75fc6d732c, and the browser displayed active Goal turn1/20. The actual worker and separate judge both ran cursor/grok-4.6/high/fast. One canonical agent-judge criterion includes the textual verify and constraints rubric; it is not a command judge. The run settled complete/done with one approved turn, retained after Web reload. Independent field-summary.md contains the workspaceID and three verb-led steps. Judge and origin sessions are stopped.

Evidence: goal-first-run-proof.json, goal-active-web.png, goal-complete-reloaded.png and goal-bare-submitted-web.png. This does not establish two rejected rounds, replacement/pause, all invalid reasons, or the direct202 browser response; those branches remain open. The persona slice ends for diagnosis of the bare-input error.

GL-002 is now verified against its complete scenario contract. This does not close the other CH-046 branches. Inventory: 9 dispositioned, 348 pending. The invalid-input retake uses fresh Web session sess-a6f30575423e1660 on daemon 38013 with the parser fix; Web now renders human guidance, CLI emits typed rejection, and HTTP/UDS return 422. Public origin-filtered Run lists stay empty and queue entries stay zero before a subsequent valid HTTP202 starts looprun-aba12206404b2583.

### Goal parser repair and adjacent acceptance — 2026-09-11

The reserved Goal command now reaches its owning dispatcher even when its objective is invalid. The existing admission suite proves bare and oversized inputs yield structured reasons without invoking the executor. Red-before/green-after race evidence, test-convention audit, Go build, reviewed diff, and make gate all passed. Fresh Web guidance, CLI structured failures, HTTP/UDS422, empty Run lists and empty queue confirm the repair. A valid adjacent start returned202, created one Run and completed with a real Cursor Grok4.6 High Fast worker and separate judge; independent source read and refreshed Web matched. See goal-parser-retest-proof.json and BUG-20260911-goal-parser-preempts-guidance.md for the cross-surface audit.

GL-001 and GL-002 are verified. GL-003 remains pending for its other invalid branches. Current inventory: 10 dispositioned (5 verified, 2 fixed-and-verified, 1 unresolved defect, 2 external runtime blockers), 347 pending. This is continuing QA, not a final delivery claim.

The parser repair is committed locally as 565ebfb32. The strict lab evidence auditor was invoked after the slice and remains failing with ten whole-run evidence requirements (role/channel breadth, structured object correlation/disruption events, and final report/gate indexing); see goal-parser-strict-audit.txt and the manifest qa-audit-report. Focused behavior proofs and make gate remain valid, but no final QA audit pass is claimed. CH-047 now runs in fresh session sess-00a7a5caef013168 with Run looprun-bc34b40550de8f95; Web Pause has been requested during the active prompt and the public Run confirms pause_requested=true.

### Bruno Goal controls — 2026-09-11

Pause settled turn1 and held the worker idle through reload; concurrent HTTP/UDS resume returned the same Run, with one resume transition and one successor segment. Turn2 completed with approved judgment. Terminal Web clear and live CLI clear both hid the snapshot while retaining both Run audits; the live revocation is recorded as ambiguous/goal_control_revoked_in_flight and Run failed with goal_clear cause. No old Goal or late turn reappeared. Screenshots were visually inspected; independent controlled-summary.md matches the requested workspace. GL005/006/008 pass. The 13 dispositioned rows comprise8 verified,2 fixed-and-verified,1 unresolved defect,2 external blockers;344 remain pending.

An adjacent CLI wait failed at30.027s despite --timeout45s (exit69 Client.Timeout exceeded). The current Goal still completed correctly. Persona session ended and origin stopped before source diagnosis; BUG-20260911-session-wait-client-timeout tracks the bounded transport mismatch.

### Session wait repair — 2026-09-11

Commit06c5e9e10 routes wait through the existing long-lived transport. A fresh managed session returned idle immediately; the45s timeout returned exit75 with its resumable server payload after45.03s, and a public stop at35s produced state-reached after35.113s. The session is stopped. Canonical race regression and make gate passed. The test-shape checker has ten identical pre-existing baseline findings and no added findings; no unrelated suites were rewritten. See session-wait-retest-proof.json. The adjacent RT-session-wait-state repair is verified; unchanged semantics retain their prior evidence and do not increase the357-row inventory.

### Turn-limit preparation and missed waiter edge — 2026-09-11

A fresh Goal with max_turns1 paused at turn1 and, on resume, ended blocked/exhausted because its public definition uses on_exhausted:halt. This does not test GL007 approval; the documented escalate policy must be prepared separately. The prior max_turns20 value is restored on disk and will take effect at the next owned restart. No approval was fabricated.

During that walk, an existing90s idle wait timed out even though the visible session had become idle; a fresh read returned idle immediately. The persona session ended and origin stopped before diagnosing BUG-20260911-session-wait-misses-settled-edge. The CLI timeout repair remains valid; this is a separate daemon publication issue. RT-session-wait-state is reopened while its correction is validated.

### Prompt boundary notifications repaired — 2026-09-11

Both runtime activity boundaries now publish one canonical attention transition. The final real Cursor/Grok4.6 High Fast retake registered CLI/HTTP/UDS waits before starting the Goal; all reached running after24s. Catalog SSE retained exactly idle→running→idle. The immediately preceding unchanged finalization path woke all three idle waiters after33s. Final Goal history shows one approved turn and Done/settled survives Web reload; independently read notes agree, and origin stop is confirmed. No fresh final-run idle registration is claimed. Proof: wait-both-edges-proof.json.

The canonical wait suite covers visible idle and unseen done, plus exact hook edge count. Red-before/green-after race checks, conventions and build passed. Final make gate passed all affected lanes in wait-both-edges-gate.txt, including 771 Web files / 7205 tests. RT-session-wait-state is repaired and re-walked without changing the original inventory count. Restored goals.max_turns20 is applied in the current daemon and confirmed by the new Goal.

### Goal turn extension and approval copy — 2026-09-11

Bruno completed GL007 through a workspace-only catalog fork, max_turns1 and on_exhausted:escalate. The first real Cursor turn wrote counter1 and was rejected by the command criterion. Web approval won concurrent CLI approval; exactly one successor segment retained session/binding epoch1, increased the effective limit1→2, and completed approved turn2 with counter2. Done survives reload. Public synthetic prompts, turn audit, SSE and independent files establish the transition; goal-extension-proof.json retains the exact evidence.

The approval fallback incorrectly promised unchanged limits. A one-string repair now states only that approval lets work continue and rejection ends the run. A fresh rebuilt-Web replay reached its own approval at counter1 and displayed the correction after reload; Reject & halt ended blocked with no second turn. Both workers are stopped, with explicit cleanup for the rejection worker. Final gate passed all affected lanes (goal-approval-copy-gate.txt). Inventory:14 dispositioned (8verified,3fixed-and-verified,1unresolveddefect,2externalruntime),343pending of357.

### Expected-Run replacement — 2026-09-11

Lea verified new Goal rejection with exact current Run over CLI/HTTP/UDS, stale identity rejection, and invalid-runtime preparation preserving one paused Run and queue0. A fresh expanded-Goal walk isolated missing replacement feedback only on busy sends; idle send displays Draft replacement. Its nonempty-draft guard prevented overwriting; clearing the authored draft enabled the exact expected-Run prefill. Web replacement switched to one successor, which completed approved with Cursor/Grok4.6 High Fast. Forty sampled public snapshots contain only old or new Run, never null. Old Run is failed (not a claimed cancellation), new Run done; stale old Web command preserves the successor and gives guidance. Refresh retains completion; origins stopped. See goal-replacement-proof.json.

BUG-20260911-busy-goal-replacement-feedback owns the remaining Web boundary failure. The busy adapter discards typed error feedback, while idle chat retains it. Canonical adapter/action regressions are running before the behavior fix. Inventory remains14 dispositioned/343pending while this coherent slice is repaired.

### Busy Goal replacement feedback repaired — 2026-09-11

The HTTP adapter retains typed Goal failures and the busy action publishes them to the existing feedback owner and workspace-scoped Goal envelope without accepting the failed send. Four new canonical cases failed before repair; all101 adapter/action tests then passed. The new test request was corrected to the existing durable messages/identity contract after typecheck; final root Turbo typecheck and build pass. Required gate remains running.

Lea repeated the original failure in fresh session sess-a0ef67c2d1664e37. Both public status reads surrounding Steer report active_prompt true; the expanded Goal displays human rejection and Draft replacement. The same Run/objective remains and queue0/no candidate Run is independently confirmed. Exact Run prefill survives reload and its successor completes approved with Cursor/Grok4.6 High Fast; independent kickoff.md identity agrees, Done survives reload, and session stop is confirmed. Screenshots were visually inspected. Proof: busy-goal-feedback-proof.json, with earlier cross-transport rejections and finite snapshot-continuity observations reused from goal-replacement-proof.json.

GL009 is fixed-and-verified; GL010/011 verified. Inventory:17 dispositioned (10verified,4fixed-and-verified,1unresolveddefect,2externalruntime),340pending. TA094 daemon restart and TA104 streamed-draft branches remain pending.

Busy-feedback delivery gate passed all affected lanes:771 Web files/7209 tests, typecheck and lint (0warnings/0errors). Evidence: busy-goal-feedback-gate.txt. No CI or push was requested.

### Direct start after blocked — 2026-09-11

Lea revisited the retained terminal blocked/exhausted Goal, confirmed no Resume, and observed CLI goal_not_active for resume. A plain Web /goal created a successor without replacement or clear; the old Run remains exhausted. HTTP/UDS/CLI agree, the real Cursor successor completed approved with the independently verified workspace identity, Done survives reload, and session stop is confirmed. Screenshots were visually inspected; goal-blocked-proof.json retains precise observations.

GL012 is verified. The official skill contradicted the existing scenario contract; two sentences now state that live Goals require replacement and terminal blocked permits direct start. This is an editorial correction, reviewed against the real replay, without a prose-only test or additional provider-compliance claim. Inventory:18 dispositioned (11verified,4fixed-and-verified,1unresolveddefect,2externalruntime),339pending.

### Three-turn convergence exposes missing Web history — 2026-09-11

Real session-origin Goal looprun-4e25e5a98540b15d converged with Cursor/Grok4.6 High Fast: count1/rejected (two increments remaining), count2/rejected (one remaining), count3/approved. Independent file reads and total-order CLI/HTTP/UDS evidence agree. The current Run Inspect page exposes no Goal turn timeline across Graph/Nodes/Generations or node details; reload does not restore it. Origin stopped before diagnosis. BUG-20260911-goal-turn-history-missing records PR452 removing the consumer despite its preserved-operator-depth contract. The adjacent previously passing LP-run-detail-story-redesign is regressed; GL004/TA101 stay unfinished. Inventory remains18 dispositioned/339pending.

### Goal history restored and replayed — 2026-09-11

The Web once again reads the existing paged Goal-turn endpoint inside Inspect. Query envelopes preserve server cursors and workspace/Run/Profile identity, and SSE turn events wake that read. No backend, migration, wire, native-tool, config or official-skill contract changed; the bug record owns the impact audit. Two Inspect cases reproduced red;241 focused tests then passed. A numeric item fixture type was corrected and production build/typecheck passed.

Fresh Lea session sess-6c96f86912d1ae77 converged in Run looprun-db7c807f29f666ad with real Cursor/Grok4.6 High Fast worker/judge. Forty public samples independently observed file1/2/3, two exact rejection blockers then approval. Inspect showed pending nullable facts and live updates, and reload preserved all three ordered records and evidence. HTTP limit2/UDS after_seq2 agree with CLI; stop reached stopped. Reviewed screenshots and exact boundaries are in goal-history-retake-proof.json. GL004 is fixed-and-verified; TA101 and the adjacent story scenario still await their broader branches.

Inventory remains18 dispositioned (11verified,4fixed-and-verified,1unresolveddefect,2externalruntime),339pending. GL004 is an adjacent convergence canary outside the original357 and does not increase that count. Required gate initially failed an unchanged Go test during SQLite WAL checkpoint at close; three focused repetitions passed. No weakened assertion or speculative production patch. Gate retry is in progress.

### Draft admission and missing composer handoff — 2026-09-11

Lea resumed CH046/CH-untested030 in fresh sess-ec096d1951e83b23. An idle /goal draft streamed an expanded objective with real Cursor/Grok. CLI --queue and HTTP submissions while busy were rejected without queue growth. UDS arrived after idle and was admitted; no false busy/race proof is assigned. The first Web draft and a separate uncontended Web retry both left the composer empty. Public Run inventory is empty and Goal snapshot null; session stopped. BUG-20260911-goal-draft-empty-composer owns the missing completion-to-composer handoff. GL013/TA104 fail pending repair;18 inventory dispositions and339 pending remain unchanged.

Goal-history gate retry passed all affected lanes (Go plus771 Webfiles/7213tests, typecheck and lint0warnings/0errors), recorded in goal-turn-history-gate-retry.txt. First SQLite-close failure remains documented; no test weakening or unrelated patch was used.

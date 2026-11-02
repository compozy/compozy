# Exhaustive Go directory and frontend-system audit

Original baseline: `de5ab2f4d`. Integrated main: `8c025ff8f`. Implementation PR: [#1141](https://github.com/compozy/compozy/pull/1141).

The original inventory listed **92 internal roots**, not every Go package directory.
The user's coverage check exposed missing explicit SDK, tooling, fixture and nested-package
entries. This expanded inventory enumerates **all 197 tracked Go-containing directories**
(**163 under internal, 34 elsewhere**) and **all 28 frontend systems**. It includes test-only,
generated, build-tagged and nested-module directories, which a host-only `go list ./...`
would not fully represent. The original 92-root table is superseded by the rows below.

**Audited means source inspection is documented**, with named owners/files and a disposition
in the linked report. It does not mean every path was benchmarked or run. Reports distinguish
measured improvements, rejected hypotheses, unchanged code and static-only observations.
[Integrated validation](validation.md) and [browser evidence](browser.md) are separate.

Reproduction: group `git ls-files -- '*.go'` by parent directory and compare every directory
with this table; separately enumerate direct directories in `web/src/systems`. No directory
is inferred covered merely because its parent was assigned. The machine-readable directory inventory and Go/system file-list digests
are in [coverage-inventory.json](coverage-inventory.json); inspected files are named in owner reports.

Current counts: **197/197 Go directories audited**, **zero unassigned or pending directory audits**, **28/28 frontend systems audited**.

The final integration with `8c025ff8f` adds `internal/agentinstance` (14 Go files) and
`internal/agentmemory` (5 Go files). Their complete source audits are recorded in
[Loop/scheduler](loop-loops.md) and [Worktree/workspace](worktree-workspace.md).
The same ten owners reviewed the incoming changes within their existing directories;
each report has a main-integration section. Unchanged directories retain their original
source inspection. The JSON records incoming changed-file counts per directory/system.
These integration reviews do not turn the original measurements into new-base benchmarks;
explicit paired reruns and focused validation are recorded separately.

## Go directories

| Directory | Go files | Role | Owner and evidence | Audit status |
| --- | ---: | --- | --- | --- |
| `.` | 1 | Command entry point | [daemon_runtime](daemon-runtime.md) | Audited |
| `cmd/compozy` | 1 | Command entry point | [daemon_runtime](daemon-runtime.md) | Audited |
| `cmd/compozy-catalog` | 6 | Command entry point | [extension_marketplace](extension-marketplace.md) | Audited |
| `cmd/compozy-codegen` | 25 | Command entry point | [api_tasks](api-tasks.md) | Audited |
| `cmd/compozy-desktop-release` | 1 | Command entry point | [usage_dashboard](usage-dashboard.md) | Audited |
| `cmd/compozy-manifest-check` | 1 | Command entry point | [extension_marketplace](extension-marketplace.md) | Audited |
| `desktop/e2e/fixtures/appstage` | 1 | Test fixture/helper | [terminal_terminal](terminal-terminal.md) | Audited |
| `desktop/e2e/fixtures/runtimeupdate` | 1 | Test fixture/helper | [terminal_terminal](terminal-terminal.md) | Audited |
| `desktop/schema` | 1 | Embedded public contract | [terminal_terminal](terminal-terminal.md) | Audited |
| `extensions/connectivity/tailscale` | 11 | Extension / extension tool | [extension_marketplace](extension-marketplace.md) | Audited |
| `extensions/forge/github` | 10 | Extension / extension tool | [extension_marketplace](extension-marketplace.md) | Audited |
| `extensions/open-design` | 6 | Extension / extension tool | [extension_marketplace](extension-marketplace.md) | Audited |
| `extensions/open-design/scripts` | 1 | Extension / extension tool | [extension_marketplace](extension-marketplace.md) | Audited |
| `extensions/spec-cycle` | 17 | Extension / extension tool | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/acp` | 118 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/admission` | 7 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/agentidentity` | 5 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/agentinstance` | 14 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/agentmemory` | 5 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/api/contract` | 165 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/api/core` | 474 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/api/ginutil` | 2 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/api/httpapi` | 75 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/api/modelcatalogprojection` | 1 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/api/spec` | 160 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/api/spec/testdata/componentnames/collision/session` | 1 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/api/spec/testdata/componentnames/session` | 1 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/api/spec/testdata/componentnames/task` | 1 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/api/testutil` | 42 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/api/udsapi` | 66 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/appstatecontract` | 1 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/attachments` | 13 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/automation` | 90 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/automation/model` | 17 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/cli` | 527 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/cli/docpost` | 7 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/clientstate` | 21 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/cmdpalette` | 36 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/cmdpalette/corecmds` | 11 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/codegen/hostapi` | 3 | Build/dev tooling | [api_tasks](api-tasks.md) | Audited |
| `internal/codegen/jsbin` | 2 | Build/dev tooling | [api_tasks](api-tasks.md) | Audited |
| `internal/codegen/openapits` | 3 | Build/dev tooling | [api_tasks](api-tasks.md) | Audited |
| `internal/codegen/sdkgo` | 10 | Build/dev tooling | [api_tasks](api-tasks.md) | Audited |
| `internal/codegen/sdkts` | 7 | Build/dev tooling | [api_tasks](api-tasks.md) | Audited |
| `internal/codegen/storeschema` | 8 | Build/dev tooling | [api_tasks](api-tasks.md) | Audited |
| `internal/command` | 6 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/config` | 232 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/config/defaults` | 1 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/config/lifecycle` | 2 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/coordinator` | 3 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/daemon` | 854 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/deadentity` | 14 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/demoseed` | 26 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/desktoprelease` | 18 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/diagnosticcontract` | 1 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/diagnostics` | 6 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/doctor` | 7 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/e2elane` | 2 | Runtime/library | [api_tasks](api-tasks.md) | Audited |
| `internal/events` | 14 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/extension` | 271 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/agentplugin` | 14 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/contract` | 32 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/scaffold_templates/connectivity-provider-go` | 1 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/scaffold_templates/loop-watch-source-go` | 1 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/scaffold_templates/tool-provider-go` | 1 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/surfaces` | 2 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/testdata/command-fixture-go` | 1 | Test fixture/helper | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/testdata/palette-fixture-go` | 1 | Test fixture/helper | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extension/testdata/secret-guard` | 2 | Test fixture/helper | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extensionenv` | 1 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extensioninput` | 1 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extensionmcp` | 2 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/extensionprotocol` | 4 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/filesnap` | 6 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/fileutil` | 50 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/frontmatter` | 4 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/gateway` | 54 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/heartbeat` | 36 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/hooks` | 116 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/listcursor` | 2 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/logger` | 5 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/loop` | 293 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/loop/dsl` | 22 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/loop/dsl/refs` | 7 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/loop/gate` | 14 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/loop/goal` | 27 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/loop/watch` | 6 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/managedsidecar` | 3 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/marketplace` | 36 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/marketplace/pluginsource` | 20 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/mcp` | 62 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/mcp/auth` | 20 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/mcp/securehttp` | 5 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/mcppolicy` | 4 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/mention` | 5 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/modelcatalog` | 48 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/monitor` | 12 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/notifications` | 3 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/observe` | 46 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/observe/attention` | 1 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/outboundpolicy` | 4 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/procutil` | 42 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/profile` | 24 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/providerauth` | 2 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/providerenv` | 2 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/providerexec` | 4 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/providers` | 19 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/providerusage` | 42 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/reasoning` | 2 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/redact` | 11 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/registry` | 40 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/registry/github` | 13 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/registry/gitsrc` | 7 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/resources` | 29 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/retention` | 1 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/retry` | 3 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/runtimeoption` | 1 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/scheduler` | 11 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/session` | 473 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/session/contextusage` | 6 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/session/inputqueue` | 9 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/sessionprocess` | 7 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/settings` | 99 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/shellwait` | 6 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/situation` | 23 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/skills` | 67 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/skillscan` | 5 | Runtime/library | [extension_marketplace](extension-marketplace.md) | Audited |
| `internal/soul` | 19 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/speed` | 2 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/sse` | 4 | Runtime/library | [daemon_runtime](daemon-runtime.md) | Audited |
| `internal/store` | 121 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/store/globaldb` | 503 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/store/globaldb/schema` | 1 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/store/globaldb/sqlcgen` | 79 | Generated SQLite adapter | [store_session](store-session.md) | Audited |
| `internal/store/sessiondb` | 58 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/store/sessiondb/schema` | 1 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/store/sessiondb/sqlcgen` | 5 | Generated SQLite adapter | [store_session](store-session.md) | Audited |
| `internal/store/workspacedb` | 10 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/store/workspacedb/schema` | 1 | Runtime/library | [store_session](store-session.md) | Audited |
| `internal/store/workspacedb/sqlcgen` | 3 | Generated SQLite adapter | [store_session](store-session.md) | Audited |
| `internal/subagentid` | 1 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/subprocess` | 29 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/support` | 8 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/task` | 186 | Runtime/library | [loop_loops](loop-loops.md) | Audited |
| `internal/terminal` | 70 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/idgen` | 1 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/journal` | 15 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/pty` | 21 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/testdata/flood` | 1 | Test fixture/helper | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/testdata/hidden_echo` | 1 | Test fixture/helper | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/testdata/mid_escape` | 1 | Test fixture/helper | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/testdata/prompt` | 1 | Test fixture/helper | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/testdata/tui` | 1 | Test fixture/helper | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/vt` | 2 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/terminal/wire` | 8 | Runtime/library | [terminal_terminal](terminal-terminal.md) | Audited |
| `internal/testutil` | 5 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/testutil/acpmock` | 16 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/testutil/acpmock/cmd/acpmock-driver` | 13 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/testutil/e2e` | 32 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/testutil/mcpfixture` | 4 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/testutil/storeseed` | 1 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/testutil/storeseed/global` | 1 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/testutil/storeseed/internal/seedbase` | 1 | Test fixture/helper | [api_tasks](api-tasks.md) | Audited |
| `internal/toolmeta` | 12 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/toolruntime` | 11 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/tools` | 80 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/tools/builtin` | 78 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/transcript` | 38 | Runtime/library | [session_os](session-os.md) | Audited |
| `internal/update` | 47 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/vault` | 16 | Runtime/library | [events_settings](events-settings.md) | Audited |
| `internal/version` | 3 | Runtime/library | [usage_dashboard](usage-dashboard.md) | Audited |
| `internal/windowmanager` | 106 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/workref` | 3 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/workspace` | 40 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/workspaceaccess` | 8 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/workspacefiles` | 5 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `internal/worktree` | 75 | Runtime/library | [worktree_workspace](worktree-workspace.md) | Audited |
| `magefiles` | 44 | Build/dev tooling | [worktree_workspace](worktree-workspace.md) | Audited |
| `scripts` | 2 | Test-only package | [worktree_workspace](worktree-workspace.md) | Audited |
| `scripts/air-state-dir` | 1 | Build/dev tooling | [worktree_workspace](worktree-workspace.md) | Audited |
| `scripts/air-state-lock` | 2 | Build/dev tooling | [worktree_workspace](worktree-workspace.md) | Audited |
| `scripts/demo-seed` | 1 | Build/dev tooling | [worktree_workspace](worktree-workspace.md) | Audited |
| `scripts/dev-http-endpoint` | 1 | Build/dev tooling | [worktree_workspace](worktree-workspace.md) | Audited |
| `scripts/devreadiness` | 1 | Test-only package | [worktree_workspace](worktree-workspace.md) | Audited |
| `sdk/examples/clarify-tool` | 1 | SDK example | [api_tasks](api-tasks.md) | Audited |
| `sdk/examples/notes-commands` | 1 | SDK example | [api_tasks](api-tasks.md) | Audited |
| `sdk/go` | 30 | Public SDK | [api_tasks](api-tasks.md) | Audited |
| `sdk/go/client` | 72 | Public SDK | [api_tasks](api-tasks.md) | Audited |
| `sdk/go/contracts` | 31 | Public SDK | [api_tasks](api-tasks.md) | Audited |
| `sdk/go/extensiontest` | 2 | Public SDK | [api_tasks](api-tasks.md) | Audited |
| `skills` | 3 | Embedded official skill | [extension_marketplace](extension-marketplace.md) | Audited |
| `web/e2e/fixtures/bundled-extension-seeder` | 1 | Test fixture/helper | [daemon_runtime](daemon-runtime.md) | Audited |
| `web/e2e/fixtures/retained-loop-task-seeder` | 1 | Test fixture/helper | [daemon_runtime](daemon-runtime.md) | Audited |
| `web/e2e/fixtures/retired-knowledge-window-seeder` | 1 | Test fixture/helper | [daemon_runtime](daemon-runtime.md) | Audited |
| `web/e2e/fixtures/runtime-database-seeder` | 1 | Test fixture/helper | [daemon_runtime](daemon-runtime.md) | Audited |
| `web/e2e/fixtures/session-pull-request-seeder` | 1 | Test fixture/helper | [daemon_runtime](daemon-runtime.md) | Audited |
| `web/e2e/fixtures/sessions-board-seeder` | 1 | Test fixture/helper | [daemon_runtime](daemon-runtime.md) | Audited |

## Frontend systems

| System | Tracked files | Owner and evidence | Audit status |
| --- | ---: | --- | --- |
| `web/src/systems/agent` | 203 | [usage_dashboard](usage-dashboard.md) | Audited |
| `web/src/systems/automation` | 140 | [loop_loops](loop-loops.md) | Audited |
| `web/src/systems/dashboard` | 65 | [usage_dashboard](usage-dashboard.md) | Audited |
| `web/src/systems/design-system` | 2 | [worktree_workspace](worktree-workspace.md) | Audited |
| `web/src/systems/extensions` | 36 | [extension_marketplace](extension-marketplace.md) | Audited |
| `web/src/systems/gateway` | 50 | [usage_dashboard](usage-dashboard.md) | Audited |
| `web/src/systems/loops` | 455 | [loop_loops](loop-loops.md) | Audited |
| `web/src/systems/marketplace` | 90 | [extension_marketplace](extension-marketplace.md) | Audited |
| `web/src/systems/model-catalog` | 22 | [usage_dashboard](usage-dashboard.md) | Audited |
| `web/src/systems/notifications` | 5 | [store_session](store-session.md) | Audited |
| `web/src/systems/onboarding` | 40 | [events_settings](events-settings.md) | Audited |
| `web/src/systems/os` | 664 | [session_os](session-os.md) | Audited |
| `web/src/systems/profiles` | 71 | [events_settings](events-settings.md) | Audited |
| `web/src/systems/provider-usage` | 30 | [usage_dashboard](usage-dashboard.md) | Audited |
| `web/src/systems/providers` | 6 | [usage_dashboard](usage-dashboard.md) | Audited |
| `web/src/systems/runtime` | 32 | [daemon_runtime](daemon-runtime.md) | Audited |
| `web/src/systems/scheduler` | 18 | [loop_loops](loop-loops.md) | Audited |
| `web/src/systems/session` | 838 | [store_session](store-session.md) | Audited |
| `web/src/systems/settings` | 360 | [events_settings](events-settings.md) | Audited |
| `web/src/systems/skill` | 25 | [extension_marketplace](extension-marketplace.md) | Audited |
| `web/src/systems/status` | 16 | [daemon_runtime](daemon-runtime.md) | Audited |
| `web/src/systems/support` | 7 | [api_tasks](api-tasks.md) | Audited |
| `web/src/systems/tasks` | 276 | [api_tasks](api-tasks.md) | Audited |
| `web/src/systems/terminal` | 137 | [terminal_terminal](terminal-terminal.md) | Audited |
| `web/src/systems/theme` | 13 | [worktree_workspace](worktree-workspace.md) | Audited |
| `web/src/systems/tool-approvals` | 20 | [terminal_terminal](terminal-terminal.md) | Audited |
| `web/src/systems/vault` | 40 | [events_settings](events-settings.md) | Audited |
| `web/src/systems/workspace` | 171 | [worktree_workspace](worktree-workspace.md) | Audited |

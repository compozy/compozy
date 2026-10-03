# BUG-20261002-native-hook-dispatch-missing: Configured hooks do not run around hosted native tools

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, apply a project tool hook
- **Scenarios:** ET-native-workspace-scope-isolation
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Create required synchronous project hook `studio-registration-route` through the public native
hook interface, matching `tool_id: compozy__workspace_info`, then perform its required daemon
restart. The public hook catalog shows the declaration with the registered Studio workspace matcher.
Start a fresh hosted agent in Studio and ask it to resolve the current project registration.
History records a successful `compozy__workspace_info` call with empty input. The configured hook's
pre-call execution history remains empty after the turn ends. The equivalent scoped operator
invocation also did not dispatch its hook in the earlier authoring replay.

The hook attempts a workspace input rewrite; an unchanged result without execution does not prove
the rewrite boundary. This also means a required native tool hook cannot enforce its intended check.
The owned declaration was deleted, both sessions stopped, and the daemon restarted with an empty
tool-pre-call catalog. Graceful drain outlasted the harness's 45-second deadline, then completed;
the later successful restart is separately recorded.

## Evidence and investigation

`native-hook-install.json`, `native-hook-installed-catalog.json`, `native-hook-late-history-operator.json`,
`native-hook-runs-final-operator.json`, `native-hook-remove.json`, `native-hook-cleanup-catalog.json`
and `native-hook-walk-ended.json` under `docs/qa/evidence/2026-10-02-untested/`.

The registry has a pre-call/post-call/error hook interface and input revalidation pipeline, but daemon
boot does not supply a runner. ACP event observation is separate from authoritative native dispatch.
The bounded repair must connect the typed hook runtime to registry dispatch, preserve caller and
workspace authority after patches, and avoid duplicate lifecycle dispatch.

- **Fix commit:** pending
- **Retest:** passed

## Repair in progress

Daemon boot now supplies a native hook adapter to registry dispatch. It uses the existing typed
runtime and the authoritative descriptor, binds patched input again, refuses a changed workspace
(including global scope), and leaves final schema, policy and approval checks in place. Native
post-call hooks receive the canonical tool result envelope; post-error annotations preserve the
original error classification. Active sessions attach their existing hook audit writers so both
hook runs and lifecycle events reach the normal session ledger. ACP observations of native calls
are excluded from a second lifecycle dispatch; provider-native observations retain their path.

The internal `HookRunner` signatures now carry scope and descriptor, with separate post-error
annotation and hook failure returns. All consumers move together. This is an internal contract
change; no public DTO, selector, configuration shape or stored schema changes.

Owning invariants and suites: `TestDaemonBootToolRegistry` covers configured dispatch, input
revalidation, workspace preservation and pre/post/error behavior; the existing ACP hook suite owns
observational duplication; the existing hosted MCP integration test owns the real daemon,
subprocess hook and session audit path. The boot regression first executes a denied call incorrectly
(`native-hook-boot-red.log`), and the observational regression first fires six duplicate hooks
(`native-hook-duplicate-red.log`). The focused suites pass in `native-hook-focused.log`.

The hosted MCP integration already demonstrates the persisted input patch, required-hook refusal,
post-call dispatch and post-error classification (`native-hook-hosted-integration-retry.log`). Its
initial failure was a corrected CLI-array decoding mistake in the new assertion, not a runtime
failure. A final extension also checks session lifecycle events. The test-shape comparison records
zero new findings and the unchanged legacy findings in the touched canonical files:
`native-hook-test-conventions.json`. A fresh original-persona replay and delivery gate remain pending.

## Verified original-persona replay

Ada installs the narrowly matched workspace hook `studio-memory-route` through the public native
interface and restarts the isolated daemon. Fresh hosted operations session `sess-19742af258c1d89a`
submits `memory_propose` for Studio note `release_handoff_guard.md`. The hook runs exactly once and
tries to change only the workspace to Editorial. Dispatch refuses the rewrite with `hook_denied`;
the agent reports the unfinished save and does not bypass it.

Fresh approve-reads reviewer `sess-4539d1947d1b6a4c` submits an explicit Editorial mutation for
`studio_editorial_brief.md`. The hook preserves that already-bound target. The shared workspace
policy prompts for Editorial; Ada rejects the request, and the actual mutating call returns
`workspace_access_denied`. Both complete hook events and one persisted hook run per session are
visible through public reads. Studio and Editorial memory catalogs exactly match their independent
before snapshots. Both caller sessions are stopped, the owned hook is removed, and another daemon
restart confirms an empty pre-call catalog.

Receipts: `native-hook-fixed-*`. The first cleanup delete supplied an unsupported `source` field;
the public descriptor identified that driver mistake, and the corrected request succeeded. No
product workaround was used. The final focused race suites pass in `native-hook-focused-final.log`,
the hosted MCP integration including session lifecycle events passes in
`native-hook-hosted-events-integration.log` (15.054s), and all affected `make gate` lanes pass in
`native-hook-gate.log`. The broader native-boundary scenario remains Pending until its remaining
allowed foreign-mutation and all-scope legs are reconciled.

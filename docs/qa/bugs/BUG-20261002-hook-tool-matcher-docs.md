# BUG-20261002-hook-tool-matcher-docs: Hook examples use unsupported tool matcher fields

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, author a project tool hook
- **Scenarios:** ET-native-workspace-scope-isolation
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Follow the Hook Declaration Format reference and create a required synchronous `tool.pre_call`
declaration through `compozy__hooks_create`, scoped to Studio and matching
`tool_name: compozy__workspace_info`. Both the initial Python executable and the documented
`/usr/bin/env` executor form return `tool_invalid_input` / `hook_validation_failed`; no declaration
is installed. The same reference also recommends `tool_namespace`, which is not a supported field.

## Root cause and repair

The reference describes fields that do not belong to the implemented tool-event contract.
`internal/hooks/matcher.go` allows `tool_id` for tool events; `tool_name` belongs to permission
events. `ToolCallRef` and `ToolCallPatch` expose `tool_id`, without a namespace. Existing matcher
tests already exercise this contract. A temporary expansion of the native hook lifecycle test
reproduced the documented request and exposed the exact validation cause in `hook-matcher-cause.log`.
That diagnostic assertion was removed after identifying its invalid contract assumption; production
validation and existing tests were not weakened.

The declaration reference, its config example, the event payload/patch reference and the official
extension skill now use the implemented field names. This is an editorial repair; it adds no runtime
alias, changes no persisted declaration and needs no migration.

- **Fix commit:** pending

## Verified replay

Changing only the documented matcher to `tool_id` admits the same declaration. After its reported
required restart, the public workspace hook catalog shows the canonical tool ID and registered
workspace matcher. The declaration is then deleted through the native tool, followed by restart and
an empty catalog read-back. Fresh session `sess-29854cb1c9d96b0e` is stopped and verified independently.

Evidence in `docs/qa/evidence/2026-10-02-untested/`: `approval-fixed-hook-create*.json`,
`hook-matcher-cause.log`, `approval-hook-canonical-create.json`, `approval-hook-live-catalog.json`,
`approval-hook-delete.json`, `approval-hook-cleanup-catalog.json`, and `approval-hook-walk-ended.json`.

The scoped operator invocation did not fire this hook, with or without a caller tool-call ID;
both calls returned Editorial and hook history stayed empty. This replay proves authoring and
catalog recovery only. Native agent execution and the hook rewrite boundary remain Pending in the
owning scenario; no hook-input approval verdict is claimed.

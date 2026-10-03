# BUG-20261002-native-hook-dispatch-missing: Configured hooks do not run around hosted native tools

- **Status:** open
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
- **Retest:** pending

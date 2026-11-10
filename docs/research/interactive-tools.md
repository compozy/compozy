# Interactive tools: provider questions reaching the operator

This audit answers one question with code evidence: can a provider agent stop mid-turn,
ask the operator something, and receive the answer? It records the state of `main` before
this change and the smallest cut that makes one mechanism work end to end.

## Verdict

`session/request_permission` already worked, so an agent could always ask *may I do this?*
Nothing answered *what should I do?* — the question shape used by opencode `question`,
Codex `request_user_input`, Claude `AskUserQuestion`, and MCP elicitation. ACP models those
as `elicitation/create`, which this client neither advertised nor served: the request never
reached the operator, and each provider degraded on its own.

## Mechanism matrix

| Mechanism | Wire method | Before | After |
| --- | --- | --- | --- |
| opencode `question` | `elicitation/create` (form) | Unadvertised, unhandled. opencode's `requestedSchema` returns `undefined`, so it calls `cancelUnshown` and the tool returns "The question couldn't be shown to the user in this client. Continue without an answer: make reasonable assumptions…" (`sst/opencode` `packages/cli/src/acp/elicitation.ts`, `capabilities.ts`; verified in the installed `opencode 2.2.7` bundle). The turn continues without an answer. | Advertised and served. Single-select and free-text questions appear in the session clarification dock. |
| Codex `request_user_input` / MCP form elicitation | `elicitation/create` (form) | Capability gate fails, so `handleUserInput` returns `{answers:{}}` (`agentclientprotocol/codex-acp` `src/CodexElicitationHandler.ts`). The model is told the user answered nothing. | Advertised and served for `string` fields and titled `oneOf` selects; the adapter normalizes MCP `enum`/`enumNames` into `oneOf`. |
| Claude `AskUserQuestion` | `elicitation/create` (form) | Unadvertised; the adapter's `ElicitationSupport.form` is false, so the tool answers with empty `answers` (`agentclientprotocol/claude-agent-acp` `src/elicitation.ts`). | Advertised and served for single-select and free text; `multiSelect` forms are declined. |
| ACP `session/request_permission` | `session/request_permission` | Worked. Operator allow/deny, 5 min default wait, repeat-denial memory, durable pending interaction. | Unchanged. |
| Compozy native `compozy__clarify` | in-process broker | Worked. `ClarifyBroker.Ask/Pending/Answer`, session SSE events, Web dock, CLI/HTTP/UDS answer surfaces. | Unchanged; it is now also the backing channel for provider questions. |
| Extension tools with `requires_interaction` | daemon tool approval | Worked as approvals, not as questions. | Unchanged. |

## Audit findings

### 1. ACP layer (`internal/acp/`)

- `AgentProcess.handleInboundRequest` (`internal/acp/handlers.go:105`) switches over the
  client methods the host serves: `fs/read_text_file`, `fs/write_text_file`,
  `session/request_permission`, the five `terminal/*` methods, and the Claude
  `_claude/sdkMessage` extension. Unknown methods return JSON-RPC `-32601`
  (`internal/acp/handlers.go:142-143`). There is no silent fallback.
- `session/request_permission` is served by `handleRequestPermission`
  (`internal/acp/handlers.go:224`), which rejects clone/foreign ACP session ids, then
  routes through `resolvePermissionRequest`: provider interceptor, tool-host policy, or an
  operator prompt with a durable pending entry (`internal/acp/permission_pending.go`).
- The operator wait is `permissionTimeoutOrDefault`
  (`internal/acp/permission_pending.go:188`): the per-session resolver wins when it returns
  a positive value, otherwise `defaultPermissionWait = 5 * time.Minute`
  (`internal/acp/client.go:29`). Expiry resolves as `reject_once` for permission requests,
  it never hangs the process.
- Capabilities were advertised at `internal/acp/start_process.go:245` (`fs`, `terminal`,
  `session.compaction`). `clientCapabilities.elicitation` was absent, which is the single
  reason every provider above stayed on its degraded path.
- opencode `question` was therefore never a permission request: it is an
  `elicitation/create` form with `mode: "form"`, a `requestedSchema`, and the ACP
  `sessionId`. Nothing in Compozy mapped it, and nothing advertised that it could.

### 2. Session and answer surfaces (`internal/session/`, `internal/api/spec/`)

- Permission answers arrive at `POST /api/workspaces/{workspace_id}/sessions/{session_id}/approve`
  (`internal/api/spec/registry_sessions.go:458`) and resolve through
  `Manager.ApprovePermission` (`internal/session/session_pending_resolution.go:36`), which
  either answers the live process or resolves the restart-orphaned stored interaction.
- Clarification answers arrive at
  `POST /api/workspaces/{workspace_id}/sessions/{session_id}/clarifications/{request_id}/answer`
  (`internal/api/spec/clarify.go:37`), with the live projection at
  `GET .../clarifications` (`internal/api/spec/clarify.go:19`). The combined durable view is
  `GET .../pending-interactions` (`internal/api/spec/session_catalog.go:183`).
- Both kinds share one store-backed attention model: `store.PendingInteractionKindPermission`
  and `store.PendingInteractionKindClarify`. `Manager.applyAttentionAgentEvent`
  (`internal/session/session_pending_events.go:30`) commits permission events and
  `Manager.applyClarifyAttentionEvent` (`internal/session/session_pending_events.go:118`)
  commits clarify events, including the resolved answer text.
- There was no bridge from the ACP process to the clarification broker: `internal/acp` could
  only reach the permission channel. That is the gap this change closes.

### 3. Compozy's own mechanism

- `tools.ClarifyBroker` (`internal/tools/clarify.go:211`) owns `Ask`, `Pending`, and
  `Answer`. A question carries free text plus at most `MaxClarifyChoices = 4` choices
  (`internal/tools/clarify.go:14`), and `ClarifyAnswer` carries either `choice` or `text`.
- The daemon broker (`internal/daemon/clarify_bridge.go`) publishes durable pending/resolved
  events, keeps the wait alive with keepalive pings, and honors `tools.clarify.timeout`
  (default `0`, meaning unbounded).
- `compozy__clarify` (`internal/tools/builtin/clarify.go`, `internal/daemon/native_clarify_tool.go`)
  is the model-facing wrapper. `requires_interaction` (`internal/extension/manifest.go:246`)
  marks extension tools that need an approval; it is a different, approval-shaped surface.

### 4. Policy

- `permissions.mode` accepts `deny-all`, `approve-reads`, `approve-all`, `approve-edits`,
  and `auto` (`internal/config/config_agent_session.go:149-153`). There is no `inherit`
  value at the config layer; `inherit` belongs to task orchestration
  (`internal/config/task_orchestration.go:19`). These modes gate
  `session/request_permission`; they never governed elicitation because elicitation did not
  exist.
- Permission lifecycle hooks (`HookEventFamilyPermission`, `internal/hooks/events.go:25`)
  can observe and narrow permission requests. Elicitation dispatches no hook, before or
  after this change.
- Headless behavior before this change: nothing in the ACP layer could ask the operator, so
  Loop/automation/subagent turns could not hang on a provider question — they simply never
  saw one. `compozy__clarify` is gated only by broker availability, and always-on check-in
  sessions explicitly deny it as non-writable (`internal/tools/check_in.go:46`,
  `ReasonCheckInToolNotWritable`). Automation sessions are created as `system`
  (`internal/automation/dispatch_context.go:22`), loops as `system`
  (`internal/daemon/loop_runtime_adapters.go:161`), subagents as `spawned`.

## Scope of this change

In: ACP form elicitation advertised and served for operator-facing sessions, mapped onto the
existing clarification broker and dock, with bounded waits, turn-cancellation handling, and
an explicit decline for sessions that cannot answer.

Out, deliberately:

- `multiselect`/array fields, and `number`/`integer`/`boolean` fields: the clarification
  answer is one choice or one text, so these forms are declined rather than narrowed
  silently.
- URL elicitation (`elicitation/create` `mode: "url"`, `elicitation/complete`): not
  advertised, not served.
- More than four choices on one field: the broker cap rejects them; declining is honest.
- A free-text "other" answer next to choices in the Web dock: the dock renders choices only
  when a question has choices, so the `_custom` companion is answered by API/CLI, not by the
  dock.
- Elicitation hooks and per-tool policy: provider questions are not permission requests, and
  no hook can currently deny or annotate them.
- MCP servers that elicit through Compozy's own hosted MCP proxy; this cut covers the ACP
  client path only.

## How a question travels now

1. The agent sends `elicitation/create` with `mode: "form"`, `sessionId`, `message`, and a
   `requestedSchema` whose single-selects use titled `oneOf` entries.
2. `handleCreateElicitation` (`internal/acp/elicitation.go`) rejects unbound ACP session ids,
   non-form modes, and unrepresentable schemas with an explicit `decline`.
3. `elicitationQuestions` turns each string property into a `ClarifyQuestion`; a titled
   `oneOf` becomes choices (labels shown, provider `const` values kept), and a sibling
   `<key>_custom` field is consumed instead of being asked separately.
4. `Manager.elicitationAsker` (`internal/session/elicitation.go`) refuses headless session
   types with `ErrElicitationUnavailable`, then calls `ClarifyBroker.Ask` with the session's
   workspace, profile, session id, and agent name.
5. The broker publishes the pending clarify event, which the session manager commits as a
   durable pending interaction and the Web dock renders.
6. The operator answers through the dock, CLI, HTTP, or UDS. The answer returns to `Ask`,
   the asker returns it to the ACP handler, and the handler answers `accept` with content
   keyed exactly as the provider's schema expects.
7. Timeout (the session's operator wait, 5 minutes by default), turn cancellation, or a
   headless session answers `decline`/`cancel` instead of leaving the turn blocked.

## Sources

- ACP elicitation types: `github.com/coder/acp-go-sdk` (replace target `compozy/acp-go-sdk`),
  `ClientMethodElicitationCreate`, `UnstableCreateElicitationRequest`,
  `ElicitationCapabilities`.
- opencode: `sst/opencode` `packages/cli/src/acp/elicitation.ts`, `capabilities.ts`,
  `packages/core/src/tool/plugin/question.ts`.
- Codex: `agentclientprotocol/codex-acp` `src/CodexElicitationHandler.ts`,
  `src/ElicitationCapabilities.ts`.
- Claude: `agentclientprotocol/claude-agent-acp` `src/elicitation.ts`.

Nothing here depends on undocumented ACP behavior: the wire shape is the SDK's unstable
elicitation protocol, and each provider's degraded path is visible in its own adapter.

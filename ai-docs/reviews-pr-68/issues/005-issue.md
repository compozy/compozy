# Issue 5 - Review Thread Comment

**File:** `internal/core/run/ui_model.go:286`
**Date:** 2026-04-06 11:28:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** `run.Execute` agora garante um event bus quando a UI está habilitada e o journal não recebeu um bus na fase de preparação. O journal ganhou `SetBus`, e há teste cobrindo o fallback local para manter o fan-out vivo mesmo no caminho `run.Execute(..., nil, runtimeCfg)`.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**A nil bus now disables live UI updates.**

When `bus == nil`, the adapter becomes a no-op and the controller only ever sees the initial `jobQueuedMsg`s. `internal/core/api.go` still calls `run.Execute(..., nil, runtimeCfg)`, so that path will render a static TUI with no lifecycle, session, or shutdown updates. Please either require a bus when UI is enabled or keep a direct fallback.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/ui_model.go` around lines 284 - 286, The early return when
bus == nil in the UI adapter (the if bus == nil { return closeSink, done } block
in internal/core/run/ui_model.go) disables live UI updates; instead provide a
direct fallback bus so lifecycle/session/shutdown messages still flow. Replace
the early return with creating a minimal no-op/fallback Bus that implements the
same message delivery used by the adapter (forwarding lifecycle messages into
closeSink/done and handling subscriptions) so the controller continues to
receive updates when run.Execute(..., nil, runtimeCfg) is used; alternatively,
if UI must not run without a real bus, make run.Execute validate and return an
explicit error when called with nil. Ensure changes reference the bus variable,
closeSink and done return values, and the run.Execute call site behavior.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:30c6a342-10dd-4312-82fe-961af791fe0d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55C9ud`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55C9ud
```

---
*Generated from PR review - CodeRabbit AI*

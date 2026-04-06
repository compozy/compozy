# Issue 2 - Review Thread Comment

**File:** `internal/core/run/journal/journal.go:162`
**Date:** 2026-04-06 10:19:04 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** Havia uma corrida real entre `Submit()` e `Close()`: com `closeRequested` fechado e `inbox` pronto para envio, o `select` podia aceitar um evento depois do início do fechamento. O journal agora usa um gate explícito entre submitters e fechamento, fecha o `inbox` de forma coordenada e impede novos envios a partir do início do close.

## Body

_⚠️ Potential issue_ | _🔴 Critical_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cat -n internal/core/run/journal/journal.go | head -350
```

Repository: compozy/compozy

Length of output: 10511

---

🏁 Script executed:

```shell
# Check if there are any other threads/goroutines that could interact with closeRequested
rg -A 5 -B 5 "closeRequested" internal/core/run/journal/journal.go
```

Repository: compozy/compozy

Length of output: 1465

---

🏁 Script executed:

```shell
# Also check if there's any logic that might allow submissions after closeRequested is closed
rg -A 3 "close\(j.closeRequested\)" internal/core/run/journal/journal.go
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
# Get line numbers around runDrainLoop to understand the full drain lifecycle
sed -n '244,290p' internal/core/run/journal/journal.go | cat -n
```

Repository: compozy/compozy

Length of output: 1448

---

🏁 Script executed:

```shell
# Check the storeResult and result methods to understand the done closure path
rg -A 5 "storeResult\|func.*result" internal/core/run/journal/journal.go | head -40
```

Repository: compozy/compozy

Length of output: 41

---



</details>

**Race condition: `Submit` can succeed after `Close` has started, orphaning events.**

When `closeRequested` is closed, both the `case <-j.closeRequested:` and `case j.inbox <- ev:` become ready in `Submit`'s select statements. Go's select randomly picks one among ready cases, so the send can win, delivering the event to an unmonitored channel. If `runDrainLoop` has already hit its `default` and exited (inbox was empty), no one is draining anymore and the event is lost, yet the caller still gets `nil`.

Also applies to lines 170-178, 269-279.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/journal/journal.go` around lines 130 - 162, Submit can race
with Close: a send to j.inbox can succeed after closeRequested is closed,
orphaning events. Add an explicit "closing"/closed atomic flag (e.g., j.closing
int32) or mutex and use it to prevent sends once Close begins: set the flag at
the start of Close/runDrainLoop, and in Submit check the flag before attempting
to send; after a successful send re-check the flag and if it became set, remove
the just-sent event from j.inbox (non-blocking receive) and return ErrClosed (or
otherwise handle/drop consistently). Update Submit (the select blocks around
j.inbox) and Close/runDrainLoop to set/read that flag so sends cannot win the
race with closeRequested; apply the same change to the other affected regions
noted (lines ~170-178 and ~269-279) that perform j.inbox sends.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4154d49c-3d7e-4c97-8137-5703c38da1a3 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55CGQt`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55CGQt
```

---
*Generated from PR review - CodeRabbit AI*

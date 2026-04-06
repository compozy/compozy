# Issue 1 - Review Thread Comment

**File:** `internal/core/model/model.go:307`
**Date:** 2026-04-06 11:28:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** `SolvePreparation` agora carrega um `JournalHandle` explícito em vez de um `*journal.Journal` cru. O journal é encapsulado por `journal.Owner`, exposto via `prep.Journal()`, fechado via `prep.CloseJournal(ctx)` e a limpeza foi centralizada em `internal/core/preputil`, deixando a ownership de fechamento explícita no boundary do kernel.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Make journal ownership explicit before carrying it in `SolvePreparation`.**

`SolvePreparation` is a copyable DTO, but `Journal` is a closable shared resource. The kernel already keeps cleanup responsibility on `prep.Journal`, and this pointer is now being threaded further downstream, so copying or reusing the preparation object makes close ownership ambiguous very quickly.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/model.go` around lines 303 - 307, SolvePreparation
currently carries a raw *journal.Journal pointer which makes close/ownership
ambiguous; replace the Journal field with an explicit handle interface (e.g.,
type JournalHandle interface { Journal() *journal.Journal; Close() error }) or a
named owner type (e.g., journal.Owner) so ownership/Close() responsibility is
explicit, update all uses of SolvePreparation.Journal to call the handle's
Journal() accessor and ensure the kernel instantiates and is documented as the
Close() owner (do not leave callers closing it), and add a short comment on the
SolvePreparation.JournalHandle field stating the kernel retains cleanup
responsibility.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:30c6a342-10dd-4312-82fe-961af791fe0d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55C9t9`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55C9t9
```

---
*Generated from PR review - CodeRabbit AI*

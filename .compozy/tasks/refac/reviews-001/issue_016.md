---
status: resolved
file: internal/core/model/preparation.go
line: 53
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWk,comment:PRRC_kwDORy7nkc61XmRM
---

# Issue 016: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Protect journal ownership on replace and failed close.**

`SetJournal` can overwrite an existing owner without closing it, and Line 51 clears `JournalHandle` before `Close(ctx)` succeeds. In both cases the journal can remain open with no owner left to retry cleanup. Either reject double assignment or explicitly transfer/close the previous handle, and only nil the field after a successful close.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/preparation.go` around lines 35 - 53, SetJournal
currently overwrites an existing JournalHandle without releasing it and
CloseJournal clears JournalHandle before Close(ctx) completes; update SetJournal
and CloseJournal on SolvePreparation to protect ownership: in SetJournal, if
p.JournalHandle != nil then refuse to replace (no-op) or return without dropping
the old owner (i.e., do not call journal.NewOwner when an owner already exists)
to avoid orphaning the previous handle; in CloseJournal, capture the current
handle, call handle.Close(ctx) first and only set p.JournalHandle = nil after
Close returns successfully (propagate/return the error if Close fails) so the
previous owner remains reachable on failure; reference methods: SetJournal,
CloseJournal, JournalHandle, journal.NewOwner, and handle.Close.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:a60f1eb0-d795-4bb2-8b4d-2afc11c2fe85 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `SetJournal` can silently replace an owned journal handle without closing the previous owner, and `CloseJournal` clears `JournalHandle` before the close succeeds. Both cases can orphan the journal and make cleanup retries impossible. The fix is to preserve ownership on replacement attempts and only clear the handle after a successful close.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.

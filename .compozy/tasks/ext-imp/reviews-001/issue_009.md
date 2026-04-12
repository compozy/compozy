---
status: resolved
file: internal/core/extension/manager.go
line: 199
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iT,comment:PRRC_kwDORy7nkc624f7m
---

# Issue 009: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Normalize session lookup keys the same way they are stored.**

`registerSession()` keys `m.sessions` with `session.runtime.normalizedName()`, but this path only applies `TrimSpace`. That leaves lookup behavior coupled to a duplicate normalization rule, so any future canonicalization change will break active-session resolution here. Please route both store and lookup through the same key-derivation helper.

Based on learnings "Enforce consistency of patterns and conventions across the system".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/manager.go` around lines 190 - 199, The lookup uses
strings.TrimSpace(name) but registerSession() stores keys using
session.runtime.normalizedName(), causing inconsistency; change the lookup to
derive the key via the same helper used by registerSession() (i.e., call the
same normalizedName/key-derivation function used on session.runtime when storing
keys) so both m.sessions insertion (registerSession()) and retrieval use the
identical key function; update the code path that computes normalized to call
that helper (or expose a shared deriveSessionKey(name) and use it here) and then
use m.sessions[derivedKey] for the lookup.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:cfb4e507-6796-4f02-9ecd-afb099e21990 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: session registration and lookup normalize extension names in different places. Today both paths only trim whitespace, but the duplication makes session resolution fragile if canonicalization changes.
  - Fix plan: introduce one session-key helper and route registration and lookup through it.
  - Resolved: centralized extension-session key normalization in `internal/core/extension/manager.go` and reused it for lookups; verified with `make verify`.

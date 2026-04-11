---
status: resolved
file: internal/cli/extension/display.go
line: 116
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVOo,comment:PRRC_kwDORy7nkc621VaU
---

# Issue 003: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Surface matching discovery failures before returning "not found".**

If discovery captured a broken install for this name, these lines return `extension "…" not found` and the failure details added by `appendDiscoveryFailureNotes` never become reachable. That makes `inspect` much less useful for diagnosing malformed manifests or partial installs.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/display.go` around lines 113 - 116, The code currently
returns a generic "extension not found" after calling findEffectiveExtension;
instead, before returning, check whether discovery recorded a failure for that
extension name (the same place where appendDiscoveryFailureNotes would add
diagnostics) and surface that failure/error first. Concretely, in the block
around findEffectiveExtension(result, name) add logic to consult the discovery
failure data stored on result (or call appendDiscoveryFailureNotes(result,
name)) and return that detailed error/notes when present rather than the generic
fmt.Errorf("extension %q not found", name), so inspect returns the discovery
failure details for malformed/partial installs.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d791b4d1-a09d-47ff-aca9-2faf6e21ecb7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - `runInspectCommand` returns `extension %q not found` immediately when `findEffectiveExtension` misses, so any matching entry in `result.Failures` is never surfaced to the user.
  - Root cause: discovery-failure rendering exists only inside `renderInspect`, which is unreachable for malformed or partially installed extensions that never produce an effective discovered entry.
  - Implemented fix: factored the matching discovery-failure formatting into a helper and used it from the not-found branch before returning the generic error.

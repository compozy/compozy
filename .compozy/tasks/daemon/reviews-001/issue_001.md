---
status: resolved
file: cmd/compozy/main.go
line: 77
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mmu,comment:PRRC_kwDORy7nkc65HKX4
---

# Issue 001: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Don't opt in after the first non-flag token.**

`shouldStartUpdateCheck` returns `true` as soon as it sees `tasks` or `daemon`, so paths like `compozy tasks help` still start the update check even though they only print help. Keep scanning the command path before deciding to enable the check.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@cmd/compozy/main.go` around lines 67 - 77, The current loop in
shouldStartUpdateCheck returns true on the first non-flag token, which wrongly
opts in for cases like "compozy tasks help"; instead, iterate through all
non-flag tokens (the args -> value loop) and track whether any non-flag token
exists, while immediately returning false if any token matches the
help/version/completion whitelist ("help", "version", "completion",
"__complete", "__completeNoDesc"); after the loop, return true only if at least
one non-flag token was seen and none were in the whitelist. This keeps the
whitelist check authoritative and delays opting in until the entire command path
is scanned.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:b0da8a0d-7451-4c83-b7b6-de09a795b9a0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `shouldStartUpdateCheck` returns `true` on the first non-flag token, so later command-path tokens such as `help` or shell-completion probes are never inspected.
- Fix plan: scan the full non-flag command path, suppress the update check if any help/version/completion token appears, and only opt in after at least one non-flag token is seen without hitting the whitelist. Add/update command-arg coverage in `cmd/compozy/main_test.go`.
- Resolution: Implemented and verified with `make verify`.

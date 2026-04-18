---
status: resolved
file: internal/cli/archive_command_integration_test.go
line: 13
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm_,comment:PRRC_kwDORy7nkc65HKYK
---

# Issue 017: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

rg -n -C4 'func newShortCLITestHomeDir|func runCLICommand|XDG_CONFIG_HOME|testCLIDaemonHomeEnv|testCLIXDGHomeEnv' --type go internal/cli
```

Repository: compozy/compozy

Length of output: 8344

---



</details>

**Add full environment isolation to match daemon-backed test pattern.**

This test only sets `HOME` but must also isolate `XDG_CONFIG_HOME`, `testCLIDaemonHomeEnv`, and `testCLIXDGHomeEnv` since operator commands run behind the daemon. Mirror the setup from `internal/cli/agents_commands_test.go` lines 66-72:

```go
homeDir := newShortCLITestHomeDir(t)
xdgConfigHome := t.TempDir()
t.Setenv("HOME", homeDir)
t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
t.Setenv(testCLIDaemonHomeEnv, homeDir)
t.Setenv(testCLIXDGHomeEnv, xdgConfigHome)
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/archive_command_integration_test.go` around lines 12 - 13, The
test currently only sets HOME using homeDir := newShortCLITestHomeDir(t) and
t.Setenv("HOME", homeDir); update it to fully isolate environment like the
daemon-backed tests: create an xdgConfigHome (e.g. via t.TempDir()), then call
t.Setenv("XDG_CONFIG_HOME", xdgConfigHome) and also set testCLIDaemonHomeEnv to
homeDir and testCLIXDGHomeEnv to xdgConfigHome (using t.Setenv) so operator
commands running behind the daemon use the isolated HOME/XDG and test-specific
envs.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:b0da8a0d-7451-4c83-b7b6-de09a795b9a0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: the archive integration test only isolates `HOME`, while daemon-backed CLI flows also consult XDG and test-specific daemon-home environment variables.
- Fix plan: mirror the full environment-isolation pattern used by the other daemon-backed CLI tests.
- Resolution: Implemented and verified with `make verify`.

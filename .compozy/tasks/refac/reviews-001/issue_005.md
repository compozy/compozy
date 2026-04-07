---
status: resolved
file: internal/core/agent/registry_launch.go
line: 186
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZV0,comment:PRRC_kwDORy7nkc61XmQW
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail
quoted='"$HOME"'
printf 'preview string: %s\n' "$quoted"
printf 'shell expansion: '
eval "printf '%s\n' $quoted"
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
cat -n internal/core/agent/registry_launch.go | sed -n '170,200p'
```

Repository: compozy/compozy

Length of output: 1080

---

🏁 Script executed:

```shell
grep -n "formatShellArg" internal/core/agent/registry_launch.go
```

Repository: compozy/compozy

Length of output: 237

---

🏁 Script executed:

```shell
# Test what strconv.Quote actually produces
python3 << 'EOF'
import subprocess
result = subprocess.run(["go", "run", "-e", """
package main
import (
    "fmt"
    "strconv"
)
func main() {
    tests := []string{
        "$HOME",
        "test with spaces",
        "test\"with\"quotes",
        "test$VARIABLE",
    }
    for _, t := range tests {
        fmt.Printf("Input: %q -> Output: %s\\n", t, strconv.Quote(t))
    }
}
"""], capture_output=True, text=True, cwd="/tmp")
print(result.stdout)
print(result.stderr)
EOF
```

Repository: compozy/compozy

Length of output: 190

---

🏁 Script executed:

```shell
# Look at actual usage of formatShellCommand
rg -B 3 -A 3 "formatShellCommand" internal/core/agent/registry_launch.go
```

Repository: compozy/compozy

Length of output: 544

---

🏁 Script executed:

```shell
# Search for all usages of formatShellCommand across the codebase
rg "formatShellCommand" --type go
```

Repository: compozy/compozy

Length of output: 418

---

🏁 Script executed:

```shell
# Test strconv.Quote behavior directly with Go's standard library
cat > /tmp/test_quote.go << 'EOF'
package main
import (
	"fmt"
	"strconv"
)
func main() {
	tests := []string{
		"$HOME",
		"test with spaces",
		`test"with"quotes`,
		"test$VARIABLE",
		"a`b",
	}
	for _, t := range tests {
		fmt.Printf("Input: %q -> strconv.Quote output: %s\n", t, strconv.Quote(t))
	}
}
EOF
cd /tmp && go run test_quote.go
```

Repository: compozy/compozy

Length of output: 358

---

🏁 Script executed:

```shell
# Look at the context in client.go where formatShellCommand is used
rg -B 10 -A 10 "formatShellCommand" internal/core/agent/client.go
```

Repository: compozy/compozy

Length of output: 639

---

🏁 Script executed:

```shell
# Search for where command output is actually executed (looking for shell.Command, os/exec, etc.)
rg "exec\.Command|shell\.Command" --type go internal/
```

Repository: compozy/compozy

Length of output: 638

---

🏁 Script executed:

```shell
# Check if there's any shell execution wrapper
rg -B 5 -A 5 "formatShellCommand|launchCommand" internal/core/agent/registry_launch.go | head -60
```

Repository: compozy/compozy

Length of output: 1673

---

🏁 Script executed:

```shell
# Find where launchCommandForPreview is used
rg "launchCommandForPreview" --type go
```

Repository: compozy/compozy

Length of output: 351

---

🏁 Script executed:

```shell
# Look at the full context of launchCommandForPreview usage
rg -B 10 -A 10 "launchCommandForPreview" internal/ | head -80
```

Repository: compozy/compozy

Length of output: 3065

---

🏁 Script executed:

```shell
# Search for what uses the preview string (look for PreviewCommand or similar)
rg "PreviewCommand|getPreviewCommand" --type go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Search for AvailabilityError and how it's used in UI/display context
rg "AvailabilityError" --type go -B 3 -A 3
```

Repository: compozy/compozy

Length of output: 5066

---

🏁 Script executed:

```shell
# Look for any error handling or display code that might show the preview
rg "Error\(\)" internal/core/agent/registry_launch.go -B 5 -A 5
```

Repository: compozy/compozy

Length of output: 780

---



</details>

**Shell expansion vulnerability in preview formatting.**

`strconv.Quote` produces double-quoted Go string literals, but shell double quotes still expand `$...`, backticks, and command substitutions. When users copy and paste the preview command from error messages into their shell, the expansion differs from what the UI displays.

Example: `"$HOME"` appears literal but expands to `/home/user` when executed in bash.

Use shell-safe quoting (e.g., single quotes or proper escaping) instead of `strconv.Quote`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_launch.go` around lines 180 - 186,
formatShellArg currently uses strconv.Quote, which produces Go string literals
but allows shell expansions; replace that with shell-safe quoting: implement a
shell quoting routine (used by formatShellArg) that wraps the arg in single
quotes and escapes any internal single quotes by replacing ' with '\'' (i.e.,
close single quote, insert escaped single quote, reopen single quote) so the
resulting string is safe to paste into a POSIX shell and will not undergo $ or `
expansion; update formatShellArg to call this shell-safe function whenever
strings.ContainsAny detects characters requiring quoting.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `formatShellArg` currently uses `strconv.Quote`, which produces double-quoted shell fragments. Those fragments still allow shell interpolation of `$...` and command substitutions when users copy the preview command, so the preview does not faithfully represent the real argv. The fix is to use shell-safe single-quote escaping for preview rendering and update tests around the preview string behavior.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.

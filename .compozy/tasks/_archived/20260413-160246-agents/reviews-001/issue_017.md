---
status: resolved
file: internal/setup/bundle.go
line: 38
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5t9,comment:PRRC_kwDORy7nkc62zc8t
---

# Issue 017: _⚠️ Potential issue_ | _🟠 Major_

## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# First, let's examine the bundle.go file around lines 35-38
cat -n internal/setup/bundle.go | head -50
```

Repository: compozy/compozy

Length of output: 1922

---

🏁 Script executed:

```shell
# Also check if fmt is already imported
rg "^import" -A 10 internal/setup/bundle.go
```

Repository: compozy/compozy

Length of output: 317

---

🏁 Script executed:

```shell
# Get a broader view of the function containing lines 35-38
sed -n '20,50p' internal/setup/bundle.go
```

Repository: compozy/compozy

Length of output: 1090

---

</details>

**Return partial phase-one results when phase-two installation fails.**

Lines 35-38 return `nil, err` even though bundled skills were already successfully installed (line 30). This discards the `result` object containing successful skill installations and violates the error-wrapping guideline. Return the accumulated `result` with error context instead:

<details>
<summary>Suggested fix</summary>

```diff
 import (
+	"fmt"
 	"io/fs"

 	"github.com/compozy/compozy/agents"
 	"github.com/compozy/compozy/skills"
 )
@@
 	successes, failures, err := InstallBundledReusableAgents(cfg.ResolverOptions)
 	if err != nil {
-		return nil, err
+		return result, fmt.Errorf("install bundled reusable agents: %w", err)
 	}
```

</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
import (
	"fmt"
	"io/fs"

	"github.com/compozy/compozy/agents"
	"github.com/compozy/compozy/skills"
)

// ... (remaining function code)

	successes, failures, err := InstallBundledReusableAgents(cfg.ResolverOptions)
	if err != nil {
		return result, fmt.Errorf("install bundled reusable agents: %w", err)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/bundle.go` around lines 35 - 38, The current code returns nil,
err after calling InstallBundledReusableAgents (successes, failures, err),
discarding the accumulated result of successful installs; instead populate or
update the existing result variable with the successes/failures and return that
result along with a wrapped error (preserving err) so partial phase-one results
are returned on phase-two failure; locate InstallBundledReusableAgents, the
successes/failures variables, and the result variable and change the return from
nil, err to return result, wrappedErr (using the project's error-wrapping
convention) so callers receive both the partial result and the error.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b83f2d8-737c-414c-9d4a-933187b6f725 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `InstallBundledSetupAssets()` returned `nil, err` when bundled reusable-agent installation failed, which discarded successful bundled skill installation results from the first phase.
- Fix: Returned the accumulated setup result together with a wrapped phase-two error, and added a regression test by injecting a reusable-agent install failure after skills were installed successfully.
- Evidence: `go test ./internal/setup`

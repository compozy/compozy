---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/ui/update.go
line: 225
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhqq,comment:PRRC_kwDORy7nkc7LlP35
---

# Issue 017: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify current call sites and locate a cancellable context source on uiModel.
rg -n 'context\.WithTimeout\(context\.Background\(\), 30\*time\.Second\)' internal/core/run/ui/update.go
rg -n 'type uiModel struct' internal/core/run/ui/model.go -A80
```

Repository: compozy/compozy

Length of output: 3477

---

🏁 Script executed:

```shell
#!/bin/bash
# Read the context around lines 224-225 and 251-252
sed -n '215,235p' internal/core/run/ui/update.go
echo "---"
sed -n '245,260p' internal/core/run/ui/update.go
```

Repository: compozy/compozy

Length of output: 1129

---

🏁 Script executed:

```shell
#!/bin/bash
# Check how uiModel is created and whether context is passed
rg -n 'newUIModel\|uiModel\{' internal/core/run/ui/model.go -A3 | head -30

# Check if uiController passes context to uiModel somehow
rg -n 'onJobControl' internal/core/run/ui/model.go -B2 -A2

# Look at uiController's Init method to see if it sets up context passing
rg -n 'func.*uiController.*Init\|func.*uiModel.*Init' internal/core/run/ui/update.go -A10

# Check other tea.Cmd functions in update.go for context handling patterns
rg -n 'return func\(\)' internal/core/run/ui/update.go | head -10
```

Repository: compozy/compozy

Length of output: 686

---

🏁 Script executed:

```shell
#!/bin/bash
# Check the uiController initialization and context handling
rg -n 'func newUIController\|func \(c \*uiController\) Start' internal/core/run/ui/ui.go -A20 | head -40

# Look for where dispatchCtx is created in uiController
rg -n 'dispatchCtx' internal/core/run/ui/ui.go -B3 -A3

# Check what startUIEventAdapter does with the context
rg -n 'func startUIEventAdapter' internal/core/run/ui/events.go -A5
```

Repository: compozy/compozy

Length of output: 264

---

🏁 Script executed:

```shell
#!/bin/bash
# List files in the ui directory
ls -la internal/core/run/ui/

# Find where uiController and uiModel are used
rg -n 'newUIController' internal/core/run/ui/ -l
```

Repository: compozy/compozy

Length of output: 1820

---

🏁 Script executed:

```shell
#!/bin/bash
# Find newUIController definition and how context is handled
rg -n 'func newUIController' internal/core/run/ui/model.go -A30

# Check if uiModel has any context field or access
rg -n 'func \(m \*uiModel\)' internal/core/run/ui/update.go | head -10

# Search for how dispatchCtx is created and used
rg -n 'dispatchCtx\|cancelDispatch' internal/core/run/ui/model.go -B2 -A2
```

Repository: compozy/compozy

Length of output: 1774

---

🏁 Script executed:

```shell
#!/bin/bash
# Check the requestJobControl method signature to see what context it expects
rg -n 'func \(c \*uiController\) requestJobControl' internal/core/run/ui/model.go -A10

# Verify the structure - can uiModel access uiController's context?
# Let's search for any context handling in other tea.Cmd functions
rg -n 'return func\(\) tea\.Msg' internal/core/run/ui/update.go -A5 | head -50
```

Repository: compozy/compozy

Length of output: 1026

---

🏁 Script executed:

```shell
#!/bin/bash
# Check if uiModel could be modified to accept context during initialization
sed -n '85,110p' internal/core/run/ui/model.go

# Check if there are any other async operations in the codebase that handle context properly
rg -n 'context\.WithTimeout.*ctx' internal/core/run/ui/ -B2 -A2

# Look for the dispatchLoop to understand the shutdown flow
rg -n 'func \(c \*uiController\) dispatchLoop' internal/core/run/ui/model.go -A30
```

Repository: compozy/compozy

Length of output: 1692

---

🏁 Script executed:

```shell
#!/bin/bash
# Check if there are any other context patterns in update.go for comparison
rg -n 'context\.' internal/core/run/ui/update.go | head -20

# See if there's a way to access uiController's context from uiModel
rg -n 'uiModel.*ctx\|uiController.*uiModel' internal/core/run/ui/model.go | head -20

# Check if the onJobControl callback could receive the context differently
rg -n 'type uiJobControlRequest' internal/core/run/ui/types.go -A5
```

Repository: compozy/compozy

Length of output: 217

---

🏁 Script executed:

```shell
#!/bin/bash
# Check all callers of newUIModel to understand the impact of adding a context parameter
rg -n 'newUIModel\(' internal/core/run/ui/

# Verify the complete signature of newUIModel
sed -n '85,88p' internal/core/run/ui/model.go

# Check if there are test files that create uiModel
rg -n 'newUIModel' internal/core/run/ui/update_test.go -B2 -A2
```

Repository: compozy/compozy

Length of output: 3677

---



</details>

**Use the UI context instead of `context.Background()` for job-control commands.**

These functions create isolated timeout contexts from `context.Background()`, detaching them from the UI session's cancellation. When the UI shuts down, these operations can continue for the full 30-second timeout instead of being immediately canceled.

Pass the cancellation context from `uiController` to `uiModel` (via a new context field in the struct initialization) and use it as the base for these timeouts. This ensures job-control requests respect the UI/session lifecycle per the coding guideline: "avoid `context.Background()` outside `main` and focused tests; pass `context.Context` across runtime boundaries."

Lines 224–225 and 251–252.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/ui/update.go` around lines 224 - 225, The code creates
timeout contexts using context.Background() which detaches them from the UI
session's lifecycle, allowing operations to continue for the full 30-second
timeout even when the UI shuts down. Add a context field to the uiModel struct,
pass the cancellation context from uiController to uiModel during
initialization, and then replace context.Background() with this passed context
as the base for the timeout contexts in the context.WithTimeout calls at lines
224-225 and 251-252. This ensures job-control requests respect the UI/session
lifecycle.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:9622d9e85b15608f4a8cc98f -->

_Source: Coding guidelines_

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: UI job-control commands create timeout contexts from `context.Background()`, so pause/message calls can outlive UI shutdown instead of being canceled with the UI controller.
- Fix approach: carry the UI dispatch context on `uiModel` and use it as the base for job-control timeout contexts, with fallback only for standalone test-created models. This requires a minimal production touch in `internal/core/run/ui/model.go` outside the initial code-file list because `update.go` must consume context state owned by the model.

## Resolution

- Resolved by parenting pause/message timeouts to the UI dispatch context.
- Verification: `rtk make verify` exited 0 after the code changes.

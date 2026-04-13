---
status: resolved
file: internal/core/run/internal/runshared/config.go
line: 48
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5t7,comment:PRRC_kwDORy7nkc62zc8r
---

# Issue 016: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**`ReusableAgent` context is currently dropped in `NewJobs`.**

Line 48 introduces `Job.ReusableAgent`, but the constructor at Line 118-130 never maps it from `item`, so downstream runtime consumers will always see `nil`. Please propagate this field during job cloning (with deep copy semantics if mutable).



Also applies to: 118-130

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/internal/runshared/config.go` at line 48, The
Job.ReusableAgent field is declared but not propagated in NewJobs, so
Job.ReusableAgent remains nil for cloned jobs; update the NewJobs/job cloning
code that maps from the source "item" (the constructor creating new Job
instances) to copy item.ReusableAgent into the new Job.ReusableAgent, performing
a deep copy if ReusableAgent is a mutable struct or contains pointers/slices
(e.g., call a Clone/DeepCopy method or manually copy nested fields) to avoid
shared mutable state; ensure the new mapping uses the exact symbol names
Job.ReusableAgent, item.ReusableAgent and the NewJobs/job-constructor function
so downstream consumers receive an independent copy.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Analysis: The suggested propagation source does not exist in the current API. `runshared.NewJobs()` clones from `model.Job`, and `model.Job` has no `ReusableAgent` field to propagate.
- Why no change: Reusable-agent execution metadata is attached later in exec job construction, so there is no dropped field in this constructor path.
- Evidence: inspected `internal/core/model/preparation.go`, `internal/core/run/internal/runshared/config.go`, and the exec job construction path

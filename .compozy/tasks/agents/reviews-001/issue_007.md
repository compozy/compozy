---
status: resolved
file: internal/core/agents/agents.go
line: 219
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5ts,comment:PRRC_kwDORy7nkc62zc8a
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don’t let global-home lookup failure break workspace agent discovery.**

`Discover` returns immediately if `r.homeDir()` fails, so a missing/unresolvable home directory makes workspace-local agents unavailable too. Global scope is optional; this should degrade to “no global agents” instead of failing the whole discovery pass.

<details>
<summary>Suggested direction</summary>

```diff
-	globalRoot, err := r.globalAgentsRoot()
-	if err != nil {
-		return Catalog{}, err
-	}
-
-	globalCandidates, globalProblems, err := scanScope(ctx, ScopeGlobal, globalRoot)
-	if err != nil {
-		return Catalog{}, err
-	}
+	var (
+		globalCandidates map[string]agentCandidate
+		globalProblems   []Problem
+	)
+	if globalRoot, err := r.globalAgentsRoot(); err == nil {
+		globalCandidates, globalProblems, err = scanScope(ctx, ScopeGlobal, globalRoot)
+		if err != nil {
+			return Catalog{}, err
+		}
+	}
```
</details>


Based on learnings "Find and document edge cases that the happy path ignores".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/agents.go` around lines 212 - 219, The discovery
currently aborts when r.globalAgentsRoot() (called from Discover) fails, making
workspace agent discovery unavailable; change the logic so Discover treats
failure to resolve the global root as non-fatal: if r.globalAgentsRoot() returns
an error, set globalCandidates and globalProblems to empty (or nil) and proceed
to call scanScope only when a valid root is present, rather than returning the
error; update the handling around r.globalAgentsRoot(), scanScope, and the
variables globalCandidates/globalProblems so the Discover function continues
discovering workspace-local agents even when global lookup fails.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d740b4bc-0bac-4faf-9dba-d2618b9a24f6 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `Discover()` returned immediately when `globalAgentsRoot()` failed, which prevented workspace-local reusable agents from being discovered when global home lookup was unavailable.
- Fix: Treated global root resolution failure as non-fatal and continued workspace discovery; added coverage for a failing `WithHomeDir()` callback.
- Evidence: `go test ./internal/core/agents/...`

# Issue 6 - Review Thread Comment

**File:** `internal/setup/reusable_agent_sources.go:418`
**Date:** 2026-04-13 18:25:17 UTC
**Status:** - [ ] UNRESOLVED

## Body

_⚠️ Potential issue_ | _🔴 Critical_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

echo "Locate reusable-agent parsing and any name validation:"
rg -n -C3 'func parseReusableAgent|parseReusableAgent\(' internal/setup

echo
echo "Search for reusable-agent name sanitization/validation:"
rg -n -C3 'validate.*name|validate.*ReusableAgent|filepath\.Base|path\.Base|strings\.Contains.*\.\.|regexp' internal/setup

echo
echo "Show all call sites that use ReusableAgent.Name as a path segment:"
rg -n -C2 'ReusableAgent\.Name|reusableAgentsInstallDirName|filepath\.Join\(.*reusableAgent' internal/setup
```

Repository: compozy/compozy

Length of output: 10000

---

🏁 Script executed:

```shell
cat -n internal/setup/reusable_agents.go | head -80
```

Repository: compozy/compozy

Length of output: 2869

---

🏁 Script executed:

```shell
rg -n -A 30 'func parseReusableAgent' internal/setup/reusable_agents.go
```

Repository: compozy/compozy

Length of output: 1300

---

🏁 Script executed:

```shell
rg -n -A 15 'func isPathSafe' internal/setup
```

Repository: compozy/compozy

Length of output: 1094

---

🏁 Script executed:

```shell
rg -n 'isPathSafe' internal/setup
```

Repository: compozy/compozy

Length of output: 377

---

🏁 Script executed:

```shell
rg -n -B 5 -A 15 'func prepareReusableAgentInstallTarget' internal/setup/reusable_agent_sources.go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check how extension agents are loaded and if Name validation occurs
rg -n -B 10 -A 10 'Origin.*Extension|AssetOriginExtension' internal/setup/reusable_agents.go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# See where extensions get their names
rg -n -C 5 'ReusableAgent{' internal/setup/reusable_agents.go
```

Repository: compozy/compozy

Length of output: 150

---

🏁 Script executed:

```shell
rg -n 'prepareReusableAgent' internal/setup/
```

Repository: compozy/compozy

Length of output: 298

---

🏁 Script executed:

```shell
cat -n internal/setup/reusable_agent_sources.go | sed -n '240,270p'
```

Repository: compozy/compozy

Length of output: 1330

---

🏁 Script executed:

```shell
# Check extension agent creation/loading
rg -n -A 5 'func.*ReusableAgent.*error' internal/setup/reusable_agent_sources.go | head -50
```

Repository: compozy/compozy

Length of output: 1510

---

🏁 Script executed:

```shell
# See if Name is validated anywhere for agents loaded from extensions
rg -n 'parseReusableAgent\|Name.*=' internal/setup/reusable_agent_sources.go | head -20
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
cat -n internal/setup/reusable_agents.go | sed -n '112,145p'
```

Repository: compozy/compozy

Length of output: 1674

---

🏁 Script executed:

```shell
# Check loadExtensionReusableAgentSources to see how Names are set for extensions
rg -n -A 30 'func loadExtensionReusableAgentSources' internal/setup/reusable_agent_sources.go
```

Repository: compozy/compozy

Length of output: 1131

---

🏁 Script executed:

```shell
# Check the full path construction in verify/preview operations
cat -n internal/setup/reusable_agent_sources.go | sed -n '55,75p'
```

Repository: compozy/compozy

Length of output: 878

---

🏁 Script executed:

```shell
# Check if there's any validation of ReusableAgent.Name field itself
rg -n 'Name.*\.' internal/setup/types.go | head -20
```

Repository: compozy/compozy

Length of output: 392

---

🏁 Script executed:

```shell
# Check what happens with a ".." directory - would filepath.Join be safe?
# Let's create a small test
python3 << 'EOF'
import os

# Simulate what happens if someone creates a ".." directory as a reusable agent
root = "/home/user/.compozy/agents"
problematic_name = ".."

# This is what the code does
target = os.path.join(root, problematic_name)
print(f"filepath.Join('{root}', '{problematic_name}') -> {target}")

# And with "/"? (though filesystem might prevent this)
slash_name = "foo/../../bar"
target2 = os.path.join(root, slash_name)
print(f"filepath.Join('{root}', '{slash_name}') -> {target2}")

# Check if this is within the root
canonical_root = os.path.normpath(root)
canonical_target = os.path.normpath(target)
print(f"Is '{canonical_target}' within '{canonical_root}'? {canonical_target.startswith(canonical_root)}")
EOF
```

Repository: compozy/compozy

Length of output: 314

---

🏁 Script executed:

```shell
# Check if bundled or extension agents could have problematic names
# Look at how directory names are discovered
cat -n internal/setup/reusable_agents.go | sed -n '26,48p'
```

Repository: compozy/compozy

Length of output: 887

---

🏁 Script executed:

```shell
# Check if the review comment's line references are correct
cat -n internal/setup/reusable_agent_sources.go | sed -n '48,62p'
```

Repository: compozy/compozy

Length of output: 616

---

🏁 Script executed:

```shell
# Verify the 4 lines mentioned in the review (55, 151, 250, 417)
cat -n internal/setup/reusable_agent_sources.go | sed -n '55p;151p;250p;417p'
```

Repository: compozy/compozy

Length of output: 346

---

🏁 Script executed:

```shell
# Check if there's any validation in resolveExtensionReusableAgentSource
cat -n internal/setup/reusable_agent_sources.go | sed -n '359,380p'
```

Repository: compozy/compozy

Length of output: 1032

---

🏁 Script executed:

```shell
# Check for any input validation on ExtensionReusableAgentSource
rg -n 'type ExtensionReusableAgentSource' internal/setup/
```

Repository: compozy/compozy

Length of output: 145

---

🏁 Script executed:

```shell
# Let's check the full context around line 55
cat -n internal/setup/reusable_agent_sources.go | sed -n '40,70p'
```

Repository: compozy/compozy

Length of output: 1343

---

🏁 Script executed:

```shell
# Check for any tests that verify name handling
rg -n 'ReusableAgent.*Name\|invalid.*name' internal/setup/reusable_agent_sources_test.go internal/setup/reusable_agents_test.go 2>/dev/null | head -20
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check if there's validation anywhere for the Name field definition
rg -n 'type ReusableAgent struct' internal/setup/
```

Repository: compozy/compozy

Length of output: 113

---

🏁 Script executed:

```shell
# Get the full ReusableAgent struct definition
cat -n internal/setup/types.go | sed -n '53,90p'
```

Repository: compozy/compozy

Length of output: 1148

---

🏁 Script executed:

```shell
# Check all the 4 call sites mentioned - verify that line 55 is actually in PreviewReusableAgentInstall
# (The review comment says lines 55, 151, 250, 417)
# Line 55 in reusable_agent_sources.go
grep -n 'filepath.Join.*Name\|reusableAgentsInstallDirName' internal/setup/reusable_agent_sources.go
```

Repository: compozy/compozy

Length of output: 378

---

🏁 Script executed:

```shell
# Double-check the exact line numbers by counting from file start
wc -l internal/setup/reusable_agent_sources.go
```

Repository: compozy/compozy

Length of output: 103

---

🏁 Script executed:

```shell
# Verify that the review comment references are actually at the stated lines
sed -n '55p;151p;250p;417p' internal/setup/reusable_agent_sources.go
```

Repository: compozy/compozy

Length of output: 318

---

🏁 Script executed:

```shell
# Check if filepath.Base and filepath.Clean actually prevent ".." or "/"
python3 << 'EOF'
import os

# Test what filepath.Base does with problematic inputs
test_cases = [
    "/home/user/agents/name",
    "/home/user/agents/..",
    "/home/user/agents/foo/../bar",
    "/home/user/agents/foo/bar",
    ".",
    "..",
]

for case in test_cases:
    result = os.path.basename(case)
    print(f"filepath.Base('{case}') -> '{result}'")

print("\n--- What about entry.Name() from filesystem? ---")
print("Filesystem directory names cannot contain '/' by definition")
print("And filesystem does not permit creating directories named '..'")
EOF
```

Repository: compozy/compozy

Length of output: 488

---

🏁 Script executed:

```shell
# Check if SourceDir is ever directly user-supplied without validation
cat -n internal/setup/reusable_agent_sources.go | sed -n '359,365p'
```

Repository: compozy/compozy

Length of output: 452

---

🏁 Script executed:

```shell
# Verify what could set SourceDir - check the flow for extensions
rg -n 'SourceDir.*=' internal/setup/reusable_agent_sources.go | head -10
```

Repository: compozy/compozy

Length of output: 270

---

🏁 Script executed:

```shell
# Check if there's any test that validates this isn't exploitable
rg -n 'test.*path\|test.*traversal\|test.*escape' internal/setup/ -i | head -10
```

Repository: compozy/compozy

Length of output: 41

---

</details>

**Add path traversal protection for reusable-agent names before filesystem operations.**

Lines 55, 151, 250, and 417 all use `ReusableAgent.Name` directly in filesystem paths without validation. Although bundled agents are safe (derived from `fs.ReadDir().Name()` which prevents `"/"`), extension agents sourced via `filepath.Base(resolvedPath)` can have `".."` as a basename. This allows escape from the install root during preview, install, and verify operations.

**Fix:** Either reject names containing `".."` or path separators in `parseReusableAgent`, or use `filepath.Base()` in `reusableAgentsInstallDirName` to strip any path components. Additionally, apply `isPathSafe()` checks consistently across preview, install, and verify operations (currently only used in `legacy_cleanup.go`).

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/reusable_agent_sources.go` around lines 416 - 418, The
reusable agent name is used directly in filesystem paths (e.g.,
reusableAgentsInstallDirName) and can contain ".." or separators from extension
sources; update reusableAgentsInstallDirName to return
filepath.Base(reusableAgent.Name) to strip path components, and also enforce
validation in parseReusableAgent to reject names containing ".." or any path
separator characters; finally, ensure isPathSafe() checks are applied in
preview, install, and verify flows (same places that use
reusableAgentsInstallDirName and where legacy_cleanup.go already uses
isPathSafe) so all filesystem operations use validated/sanitized names.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4cdfae9c-22b0-4501-8a36-0aa965d55bf2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: added reusable-agent name validation, sanitized install-directory derivation, enforced safe target-path resolution in preview/install/verify, and added focused regression tests for unsafe names.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56nVMl`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56nVMl
```

---

_Generated from PR review - CodeRabbit AI_

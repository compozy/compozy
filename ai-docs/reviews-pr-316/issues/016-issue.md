# Issue 16 - Review Thread Comment

**File:** `sdk/compozy/constructor.go:158`
**Date:** 2025-10-31 14:57:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

**Verify shallow clone is sufficient for config slices.**

The clone helper functions perform shallow copies of config slices (appending pointers to a new slice). This means the `Engine` will have its own slice but share the underlying config objects. Verify this is intentional and that config objects are treated as immutable after construction.

If configs may be mutated after construction, consider deep cloning individual config objects, or document that configs must not be modified after being passed to `New`.

---

🏁 Script executed:

```shell
#!/bin/bash
# Search for mutations of config objects after engine construction
rg -n -A3 -B3 'engine\.(workflows|agents|tools|memories|mcps)\[' --type go -g '!*_test.go'
```

Length of output: 0

---

```

---

🏁 Script executed:

```shell
#!/bin/bash
# 1. Show the constructor.go file around lines 90-158 and context above to see how clones are used
fd -e go constructor.go | head -5 | xargs -I {} head -200 {} | tail -120
```

Length of output: 3886

---

🏁 Script executed:

```shell
#!/bin/bash
# 2. Search for where these clone functions are called
rg -n 'clone(Workflow|Agent|Tool|Knowledge|Memory|MCP|Schema|Model|Schedule|Webhook)Configs\(' --type go -A2 -B2
```

Length of output: 5200

---

🏁 Script executed:

```shell
#!/bin/bash
# 3. Check if core.CopyMap, core.CloneMap, or core.DeepCopy exist and are used elsewhere
rg -n 'core\.(CopyMap|CloneMap|DeepCopy)' --type go | head -20
```

Length of output: 1308

---

🏁 Script executed:

```shell
#!/bin/bash
# 4. Search for any assignment or mutation patterns on config slices/fields after engine creation
rg -n '\.workflows\s*=|\.agents\s*=|\.tools\s*=|\.memories\s*=' --type go -g '!*_test.go' -B5 -A2
```

Length of output: 10524

---



</details>

**Use `core.DeepCopy` for individual config objects within slice clones, consistent with codebase patterns.**

The shallow clone functions in `sdk/compozy/constructor.go` (lines 90–158) create new slices but share pointer references to the original config objects. This diverges from the project convention: other SDK constructors (workflow, memory, task, model, schedule, tool, schema, knowledge, runtime, project) use `core.DeepCopy` on individual configs. Additionally, config slices are mutated post-construction via `append` in `engine_registration.go` (`e.workflows`, `e.agents`, `e.tools`, `e.memories`). If config objects can be mutated externally after being passed to the engine, both the engine and external code would be affected through shared pointers.

Refactor each clone function to deep-copy individual config objects within the slice:

```go
func cloneWorkflowConfigs(values []*engineworkflow.Config) []*engineworkflow.Config {
	if len(values) == 0 {
		return make([]*engineworkflow.Config, 0)
	}
	cloned := make([]*engineworkflow.Config, 0, len(values))
	for _, cfg := range values {
		c, err := core.DeepCopy(cfg)
		if err != nil {
			// Handle error appropriately
		}
		cloned = append(cloned, c)
	}
	return cloned
}
```

Apply the same pattern to all clone functions for consistency.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/constructor.go around lines 90 to 158, the slice-clone helpers
create new slices but keep pointers to the original config objects; refactor
each function to deep-copy each element using core.DeepCopy (consistent with
other SDK constructors) so the cloned slice contains independent config objects;
for each input slice, allocate a result slice with the same capacity, iterate
over values calling core.DeepCopy(cfg) for each element, handle the error from
DeepCopy appropriately (return it up, log, or panic per project conventions),
append the deep-copied object to the result, and return the result; apply this
change to cloneWorkflowConfigs, cloneAgentConfigs, cloneToolConfigs,
cloneKnowledgeConfigs, cloneMemoryConfigs, cloneMCPConfigs, cloneSchemaConfigs,
cloneModelConfigs, cloneScheduleConfigs, and cloneWebhookConfigs.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFEz`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFEz
```

---
*Generated from PR review - CodeRabbit AI*

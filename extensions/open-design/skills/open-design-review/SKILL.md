---
name: open-design-review
description: Run the explicitly requested independent review and refinement cycle for OpenDesign HTML through the native open-design-review loop. Use for a new brief or existing docs/design artifacts; ordinary edits and _uiux.md input alone do not trigger it.
---

# Review a design

Use this skill only for an explicit request for the independent review and refinement cycle. The `open-design` skill owns the shared craft, project authority, and output contract. Ordinary design work stays with `open-design-designer`.

## Run the existing definition

Inspect `open-design-review` through `compozy__loop_inspect` and read the live descriptors for loop operations. Use the active workspace; the `open-design` profile provides the designer, critic, and linter. If operations are unavailable, report that limitation without claiming an independent cycle ran.

Supply a concise `brief` and a workspace-relative `artifact_path` under `docs/design/`. Preserve existing or explicitly requested paths; otherwise use `docs/design/<slug>/index.html`. For related HTMLs, name the entry file and list the other boards or `_uiux.md` paths in the brief. The designer returns every HTML path for lint and review.

Pass the following `config_overrides` on both calls. Compozy runtime defaults override definition values, so the definition alone does not enforce this workflow's limit. `full_body` also reruns the complete workflow after an action failure. A rejected review always starts a fresh generation.

Dry-run with `compozy__loop_run` using `dry: true`, verify effective `iteration_cap: 3` and `reattempt_strategy: full_body`, then execute with the same inputs and overrides without `dry`:

```json
{
  "name": "open-design-review",
  "workspace": "<active-workspace-id>",
  "config_overrides": {
    "iteration_cap": 3,
    "reattempt_strategy": "full_body"
  },
  "inputs": {
    "artifact_path": "docs/design/session-actions/index.html",
    "brief": "Review and refine the existing session-actions board against the requested selection, confirmation, empty, and failure states. Preserve the product styling and scope."
  }
}
```

Optional `designer` and `critic` inputs select existing agent definitions. Provider and model selection follow Compozy's runtime settings. Do not create a new loop definition, specification, or registry to run this workflow.

## Honor the bounded review contract

Each pass runs designer → native lint → independent critic → native lint verification. The designer refines the same files; the linter reads every returned HTML; the critic evaluates craft, purpose and states, brand, accessibility, and copy. The critic runs as a separate agent session with filesystem and browser tools, owns judgment, and returns a schema-validated verdict with evidence for every file. The designer owns corrections and receives the previous critic verdict, blockers, evidence, and exceptions on the next generation.

The linter must run successfully. Its findings are heuristic: inspect each one, require applicable fixes, and document a specific source-backed exception where appropriate. A lint `passed` value only reports its P0 result; it does not establish visual quality or complete review.

Completion requires an explicit approved verdict, no blocking issues, evidence for every reviewed file, and unchanged lint digests before and after review. The critic decides which lint findings apply and records source-backed exceptions with their original IDs and severities; the completion condition verifies approval and file identity. The native completion condition fails on evaluation errors; missing or invalid critic output cannot become approval.

Use `open-design-browser` for agent-browser inspection when rendered evidence is useful and available. Load the current screenshot through an image-capable harness before making visual claims; text snapshots alone are insufficient. Missing screenshots are not an automatic rejection, but required unavailable evidence remains a disclosed limitation.

With these per-run overrides, the native loop permits at most three generations: the initial pass and up to two refinements, stopping early on approval. Generations repeated after action failures consume this limit; individual node retries follow the native retry policy. A designer or critic inside the loop must return its requested schema and never start another run. Report transport or dependency failures as failures.

## Deliver the observed outcome

Follow the returned run ID with `compozy__loop_status` and inspect its node outputs, especially `critique` and `verify`. Use `compozy loop why <run-id> -o json` when a result needs diagnosis. Keep observing the existing run after a wait timeout rather than starting it again.

Successful completion requires terminal `done` and the recorded approved `critique` result with matching lint verification. Link the actual HTML files, report the terminal outcome and remaining findings or accepted exceptions, and state which browser or visual checks ran. Exhaustion or interruption leaves the latest files on disk; there is no automatic rollback or best-version restoration.

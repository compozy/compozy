---
name: open-design-review
description: Run the explicitly requested independent review and refinement cycle for OpenDesign HTML through the native open-design-review loop. Use for a new brief or existing docs/design artifacts; ordinary edits and _uiux.md input alone do not trigger it.
---

# Review a design

Use this skill only for an explicit request for the independent review and refinement cycle. The `open-design` skill owns the shared craft, project authority, and output contract. Ordinary design work stays with `open-design-designer`.

## Run the existing definition

Inspect `open-design-review` through `compozy__loop_inspect` and read the live descriptors for loop operations. Use the active workspace; the `open-design` profile provides the designer, critic, and linter. If operations are unavailable, report that limitation without claiming an independent cycle ran.

Supply a concise `brief` and a workspace-relative `artifact_path` under `docs/design/`. Preserve existing or explicitly requested paths; otherwise use `docs/design/<slug>/index.html`. For related HTMLs, name the entry file and list the other boards or `_uiux.md` paths in the brief. The designer returns every HTML path for lint and review.

Pass the following `config_overrides` on both calls. Compozy runtime defaults override definition values, so the definition alone does not enforce this workflow's limit. `full_body` reruns design and lint when the critic requests another generation.

Dry-run with `compozy__loop_run` using `dry: true`, verify effective `iteration_cap: 3`, `gate_max_revisions: 2`, and `reattempt_strategy: full_body`, then execute with the same inputs and overrides without `dry`:

```json
{
  "name": "open-design-review",
  "workspace": "<active-workspace-id>",
  "config_overrides": {
    "iteration_cap": 3,
    "gate_max_revisions": 2,
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

Each pass runs designer → native lint → independent critic. The designer refines the same files; the linter reads every returned HTML; the critic evaluates craft, purpose and states, brand, accessibility, and copy. The critic owns judgment, while the designer owns corrections.

The linter must run successfully. Its findings are heuristic: inspect each one, require applicable fixes, and document a specific source-backed exception where appropriate. A lint `passed` value only reports its P0 result; it does not establish visual quality or complete review.

Use `agent-browser` when rendered evidence is useful and available. Load the current screenshot through an image-capable harness before making visual claims; text snapshots alone are insufficient. Missing screenshots are not an automatic rejection, but required unavailable evidence remains a disclosed limitation.

With these per-run overrides, the native loop permits at most three complete passes: the initial pass and two refinements, stopping early on approval. A designer or critic inside the loop must return its requested schema and never start another run. Report transport or dependency failures as failures.

## Deliver the observed outcome

Follow the returned run ID with `compozy__loop_status` and inspect its node/verdict detail. Use `compozy loop why <run-id> -o json` when a result needs diagnosis. Keep observing the existing run after a wait timeout rather than starting it again.

Only terminal `done` establishes successful completion of this review cycle. Link the actual HTML files, report the terminal outcome and remaining findings or accepted exceptions, and state which browser or visual checks ran. Exhaustion or interruption leaves the latest files on disk; there is no automatic rollback or best-version restoration.

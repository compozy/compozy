# Explorer Dispatch Protocol

## Agent contract

Each explorer is dispatched via **`compozy exec` with GLM 5.2 through the `pi` harness** — a plain
`compozy exec`, **not** `--agent explorer`, so the dispatch prompt is fully self-contained and no
external agent definition competes with this skill's seven-section schema. All slices run in
parallel/background; each writes **exactly one file**:
`.compozy/tasks/<slug>/analysis/NN_analysis_<slice-slug>.md`.

### Command

Write each slice's dispatch prompt (below) to
`.compozy/tasks/<slug>/analysis/.prompts/NN_<slice-slug>.txt`, then background one process per slice
and wait for all to exit:

```
compozy exec \
  --ide pi \
  --model openrouter/z-ai/glm-5.2 \
  --access-mode full \
  --prompt-file .compozy/tasks/<slug>/analysis/.prompts/NN_<slice-slug>.txt \
  > .compozy/tasks/<slug>/analysis/.prompts/NN_<slice-slug>.out \
  2> .compozy/tasks/<slug>/analysis/.prompts/NN_<slice-slug>.err
```

Model/harness rules (non-negotiable):

- **GLM runs through `pi`, never `opencode`.** opencode routes GLM through an internal explorer
  subagent that emits little ACP output, starving Compozy's activity watchdog → the run stalls and
  is killed with zero output. pi streams tool-calls continuously and completes. Prereqs: `pi` +
  `pi-acp` on PATH (`npm i -g @earendil-works/pi-coding-agent pi-acp`) and OpenRouter auth at
  `~/.pi/agent/auth.json` (`{"openrouter":{"type":"api_key","key":"sk-or-..."}}`).
- **`--reasoning-effort` is ignored by `pi`.** Control GLM thinking on the model
  (`--model openrouter/z-ai/glm-5.2:xhigh`) or once via `"defaultThinkingLevel": "xhigh"` in
  `~/.pi/agent/settings.json` (pi's default is `medium`).
- **Escalate a slice to Opus** — `--ide claude --model opus --reasoning-effort xhigh` — only when it
  needs maximal citation precision (a subtle multi-file bug trace whose fix hinges on exact lines).
- GLM output can carry cosmetic identifier typos (dropped letters, e.g. `atempt`, `Qued`); the
  parent MUST spot-check every cited `path:line` before promoting a finding into the techspec/tasks.

### Scoped-write contract (embed in every dispatch prompt)

Because dispatch is a plain `compozy exec` with no enforcing agent definition, the prompt itself
carries the contract. Every dispatch prompt MUST state: read-only over source; write **exactly
once**, only to the named target file; never edit any other file or write outside the analysis
directory; no `git`/`make`/package-manager/mutating shell (read-only helpers
`find`/`grep`/`rg`/`wc`/`head`/`cat`/`ls` only); every Evidence citation a real `path:line`; final
message a 5–10 line summary.

## Seven-section schema (mandatory, H2 headings, in order)

1. **Scope & Question** — the slice question restated + exact sources examined.
2. **Architecture Overview** — how the examined system is structured (text diagram welcome).
3. **Key Findings** — numbered findings, each with `path:line` evidence.
4. **Current Behavior Detail** *(owned surfaces)* or **Patterns Worth Copying** *(competitors)* —
   traces, data flows, or the concrete patterns/class-strings to adopt.
5. **Problems, Gaps & Risks** — every defect/smell/gap, each with evidence + a severity guess;
   claims not provable from code alone are marked `UNCONFIRMED`.
6. **Recommendations** — concrete, actionable, priority-ordered.
7. **Reference Index** — exhaustive table: every relevant file path + one-line role.

## Dispatch prompt template

```
Slice NN/<slice-slug> for research prompt: "<the parent research question>"

You are analyzing <ours: /abs/path/to/repo | competitor: /abs/path/.resources/<repo>/ (read-only source)>.
Primary sources: <explicit dirs/files discovered during scouting>.

Investigate and document:
<numbered, slice-specific questions — include reported bug symptoms verbatim when tracing bugs;
demand plausible root causes with evidence, uncertainty marked UNCONFIRMED, never guessed>

Write EXACTLY ONE file: /abs/path/.compozy/tasks/<slug>/analysis/NN_analysis_<slice-slug>.md

Use this seven-section schema (H2 headings, in order): 1. Scope & Question · 2. Architecture
Overview · 3. Key Findings · 4. <Current Behavior Detail | Patterns Worth Copying> · 5. Problems,
Gaps & Risks · 6. Recommendations · 7. Reference Index.

Artifact in English. Cite evidence as repo-relative paths with line numbers (path:line). The
Reference Index must be an exhaustive table (path + one-line role). Be exhaustive and concrete —
this document feeds a techspec and task files authored without re-reading the code.

Scoped-write contract: you are read-only over source. Write EXACTLY ONE file — the target path
above — and nothing else. Never edit any other file, never write outside the analysis directory,
never run mutating shell (`git`, `make`, package managers, `mv`/`rm`/`cp`, output redirection);
read-only helpers only (`find`, `grep`, `rg`, `wc`, `head`, `cat`, `ls`). Every Evidence citation
must be a real, readable `path:line`. Your final message should be a 5-10 line summary of top
findings.
```

Tune §4's heading per slice kind. For bug-trace slices, additionally demand: two numbered
end-to-end traces (happy path T1..Tn and bug path B1..Bn) and a failure-point catalog
(FP-01..FP-nn with trigger, symptom, confidence CONFIRMED-BY-CODE vs UNCONFIRMED).

## Failure recovery (a `compozy exec` can exit non-zero or stall mid-run)

1. On a non-zero exit or a stalled slice (check the slice's `.err` log and process exit), check
   whether its target analysis file exists and is complete (tail it — a complete file ends with the
   Reference Index table).
2. File exists and complete → the failure hit after the write; treat the slice as done.
3. File missing/truncated → **re-dispatch the slice**: a fresh `compozy exec` with the same prompt
   file. `compozy exec` is ephemeral by default, so there is no live session to resume — the prompt
   file is reusable as-is. (To resume an interrupted session instead of restarting, add `--persist`
   to the original dispatch and re-invoke with `--run-id <id>`; a clean re-dispatch is usually
   simpler.)
4. A slice fails twice on GLM → escalate that slice to Opus (`--ide claude --model opus
   --reasoning-effort xhigh`) and re-dispatch. If it still fails, report the gap in the final
   summary rather than fabricating the analysis (Anti-Patterns: no silent scope caps).
5. If every slice stalls with zero output, suspect a harness/auth misconfig — confirm dispatch used
   `--ide pi` (not `opencode`) and that `~/.pi/agent/auth.json` carries the OpenRouter key.

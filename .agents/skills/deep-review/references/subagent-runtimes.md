# Subagent Runtimes (`--subagent`)

How Step 3 review agents (defect cohorts, polish cohorts, sweeps) execute. `native` — the default — uses the Workflow/Agent engines in orchestration.md; every other value runs the same materialized prompts cross-LLM through `compozy exec`, driven by the bundled runner. Step 2 context assembly stays orchestrator-side in every mode.

## Runtime map

| Value | Invocation |
| --- | --- |
| `claude-opus` | `compozy exec --ide claude --model opus --reasoning-effort max` |
| `grok` | `compozy exec --ide cursor-agent --model 'grok-4.5[effort=high,fast=true]'` — effort/fast ride inside the model value (no reasoning flag); requesting `grok-4.5` resolves to the same advertised variant |
| `codex` | `compozy exec --ide codex --model gpt-5.6-sol --reasoning-effort xhigh` |

## Invocation shape (per stage)

The stage scripts already materialized every prompt (schema + output contract embedded — external runtimes have no schema-enforcement layer, so the output-file contract replaces it). Execute a stage's jobs with the bundled runner from the repo root:

```bash
python3 <skill-dir>/scripts/run_jobs.py --out <out> [--jobs-file <out>/<stage>-jobs.json] \
  --command "compozy exec <runtime flags from the map> --format json --timeout 30m --prompt-file {prompt}"
```

The runner owns bounded concurrency (`--workers`, default 4 — each invocation is a full ACP session, not a thread), per-attempt event/err logs under `<out>/runs/`, output validation with one built-in retry, provider-block detection, the source-freeze check, and resume (valid outputs are never re-run). Each job's output file is the agent's only product; the JSONL/stderr logs are operational evidence — never parse them as review output.

## Failure handling

- **Runner exit 2 (blocked)** — a stream matched a block pattern (default `usageLimitExceeded`); `<out>/run-blocker.json` lists the pending jobs. Re-run the same command when the limit clears; add `--block-on <pattern>` for providers that phrase limits differently.
- **Runner exit 1 with FAIL jobs** — the agent kept producing missing/invalid output through its attempts. Read `<out>/runs/<label>.attempt-*.err`, then run that one agent on the `native` path (orchestration.md engines) and record the substitution in review.md — the no-skip invariant outranks runtime purity.
- **`model "X" is not available`** — the error lists the runtime's advertised options. Surface them and stop; never substitute a model silently (L-010).
- **`did not advertise an ACP model option`**, or `compozy` missing from PATH — stop and name the gap; external review has no alternate transport.

## Cost

Every external invocation spends `compozy exec` credit — a large PR fans out dozens of agents. `native` fits exploratory runs; external runtimes earn their spend on gate rounds (e.g. loop Phase D's `codex` lane).

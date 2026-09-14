# Subagent Runtimes (`--subagent`)

How Step 3 review agents (defect cohorts, polish cohorts, sweeps) execute. `native` — the default — uses the Workflow/Agent engines in orchestration.md; every other value runs the same materialized prompts cross-LLM through `compozy exec`, driven by the bundled runner. Step 2 context assembly stays orchestrator-side in every mode.

## Runtime map

| Value | Invocation |
| --- | --- |
| `claude-opus` | `compozy exec --ide claude --model opus --reasoning-effort max` |
| `grok` | `compozy exec --ide cursor-agent --model 'grok-4.5[effort=high,fast=true]'` — effort/fast ride inside the model value (no reasoning flag); requesting `grok-4.5` resolves to the same advertised variant |
| `codex` | `compozy exec --ide codex --model gpt-5.6-sol --reasoning-effort xhigh` |

## Invocation shape (per stage)

Preparation materializes each prompt, direct job contract and draft template. External reviewers use the same compact draft and local submit command as native reviewers. Execute pending jobs from the repo root:

```bash
python3 <skill-dir>/scripts/run_jobs.py --out <out> [--jobs-file <out>/<stage>-jobs.json] \
  --command "compozy exec <runtime flags from the map> --format json --timeout 30m --prompt-file {prompt}"
```

The runner owns bounded concurrency (`--workers`, default 4, maximum 6), unique per-attempt logs/backups under `<out>/runs/`, targeted repair, provider-block detection, source/rule/spec evidence freeze and resume. Valid outputs are preserved. Invalid results receive a repair prompt with their existing artifact and every diagnostic. A completed valid draft/output is accepted even after process timeout or a nonzero exit; the process event is still recorded. A missing result retries the assigned investigation. JSONL/stderr are operational evidence, never review output.

## Failure handling

- **Runner exit 2 (blocked)** — a stream matched a block pattern (default `usageLimitExceeded`); `<out>/run-blocker.json` lists the pending jobs. Re-run the same command when the limit clears; add `--block-on <pattern>` for providers that phrase limits differently.
- **Runner exit 1 with FAIL jobs** — inspect the per-job status and `<out>/runs/<label>.*.attempt-*.err`. Dispatch its repair prompt on the native engine if the selected external engine cannot complete it, and record the substitution. Keep valid findings and evidence; never restart all jobs for a serialization error.
- **`model "X" is not available`** — the error lists the runtime's advertised options. Surface them and stop; never substitute a model silently (L-010).
- **`did not advertise an ACP model option`**, or `compozy` missing from PATH — stop and name the gap; external review has no alternate transport.

## Cost

Every external invocation spends `compozy exec` credit — a large PR fans out dozens of agents. `native` fits exploratory runs; external runtimes earn their spend on gate rounds (e.g. loop Phase D's `codex` lane).

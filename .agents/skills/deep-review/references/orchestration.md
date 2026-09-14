# Orchestration

The orchestrator prepares semantic decisions, dispatches rendered prompts, runs the final gate and reports. It does not review inline or construct reviewer JSON by hand.

## Pipeline

| Stage | Operation | Observable completion |
| --- | --- | --- |
| Prepare | `prepare_review.py --out ...` | Manifest, compact knowledge queue and decision template |
| Apply decisions | Same command with `--decisions ...` | Exit 2 for newly pending references; exit 0 with materialized jobs when complete |
| Review | Native/Workflow or external runner | Each worker submits its assigned draft locally |
| Global gate | `run_jobs.py --validate-only` | Every output valid; source and rule evidence frozen |
| Merge/report | `merge_findings.py`, `render_review.py`, `render_html.py` | Complete two-lane coverage, canonical verdict and reports |

Direct `build_manifest.py`, `build_knowledge.py` and `build_jobs.py` remain useful for focused diagnostics. Normal runs use the preparation helper rather than inventing scripts.

## Cohorts

Both lanes default to 200 files / 15,000 changed lines per job, with no total PR cap. These are configurable batching thresholds: `--max-cohort-files N` / `--max-cohort-lines N` for defects, `--max-polish-files N` / `--max-polish-lines N` for polish. Any positive integer is accepted, including values above the defaults. Preparation retains chosen settings across decision stages, resume and refresh; explicit flags override individual settings. Direct `build_jobs.py` inherits the plan's limits unless overridden.

The automatic planner groups packages/directories, places source/test siblings adjacent and partitions hunks deterministically. Review every selected file; do not sample or omit for size. An oversized file's slices cover each selected hunk line exactly once in each lane, with whole-file context available to every owner.

The optional preparation `--max-context-lines` budget partitions groups by total file size as well as changed lines. A single oversized file remains reviewable. Context-size estimates guide tuning; do not silently lower scope or model effort.

Override `decisions.plan` only when actual source/test/type relationships cross the default grouping. Its `cohorts` contain `id`, `name`, `risk` (`high|normal|low`), `files`, and optional `hunk_scope`: `{path:[{start,lines,side}]}`. The gate rejects unknown paths, unsafe/duplicate IDs, omitted or duplicated ownership and excess limits. A risk tag changes emphasis, never selection.

## Sweeps

Default to none. Each extra sweep needs a concrete cross-cohort hypothesis in `decisions.sweeps`; one agent investigates it across the selected surface. Example:

```json
{"key":"contracts","hypothesis":"The changed producer response crosses API, SDK and UI cohorts; verify all consumers accept the new optional field."}
```

| Key | Boundary that justifies it |
| --- | --- |
| `contracts` | Changed exported/wire/API contract across owners |
| `security` | Input/authz flow crosses cohorts and has a reachable attack hypothesis |
| `migrations` | Schema/code/deployment ordering spans owners |
| `tests` | A failing-capable invariant spans cohorts with no complete local owner; behavior change alone is insufficient |
| `consistency` | A rename or shared invariant has consumers outside its local cohort |
| `config` | Declaration/defaulting/consumption crosses owners |
| `spec-parity` | Always generated when a spec contract is supplied |

Built-in lens text supplies the investigation focus; a custom key requires `lens`. Rules enter a sweep only when their `sweeps` binding names it. Local lanes already own their rule assessments. Keep every cross-cohort survivor and its evidence; no numerical findings quota.

## Engines

Native agents are the default fallback; use up to six concurrently, or the harness's smaller available limit. Read only the compact dispatch index/status, then send each pending row's prompt path. Use `fork_turns=none` when supported and preserve the selected model/effort. Avoid reloading the parent conversation and skill catalog as explicit task context. Each prompt contains the review contract and local submit command.

For Workflow, feed only pending rows from the global status to the existing engine:

```js
export const meta = {name: 'deep-review-jobs', phases: [{title: 'Review'}]}
phase('Review')
return await parallel(args.jobs.map(j => () => agent(
  `Read ${j.prompt}; complete that assignment and its local submit command. Product source is read-only.`,
  {label: j.label, phase: 'Review'})))
```

Workers write their draft and call `run_jobs.py --job LABEL --submit`; local diagnostics and canonical output belong only to that job. They do not run global validation, reload jobs.json or react to other workers' pending outputs. Native reuse is allowed when related jobs benefit from context; do not accumulate unrelated jobs indefinitely. A materialized contract supports a clean worker without reconstructing prior context.

After a batch, the orchestrator runs `run_jobs.py --validate-only`. Valid jobs are reused. An invalid status row's `prompt` points to an automatically generated repair request containing all errors and the existing artifact. Dispatch that row; avoid starting the original review again merely for JSON repairs. Two repair attempts without progress require inspecting the diagnostic and missing evidence, not another blind retry.

For non-native `--subagent`, use [subagent-runtimes.md](subagent-runtimes.md). The same contracts work across engines. Timings/statuses live in prepare-status.json, per-job status and external attempt records; these are diagnostics, not proof of finding quality.

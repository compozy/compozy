# Orchestration

The pipeline stage map, cohort planning rules, sweep triggers, and the execution engines. Prompt wording is single-sourced in `assets/PROMPT.md` (rendered by `build_jobs.py`); output shape in `assets/findings.schema.json` — this file explains how the stages compose and how jobs get executed.

## Pipeline — stages, gates, artifacts

Every stage materializes jobs with lane ownership (`{label, kind, lane, prompt, output, required_hunks, rule_ids}`), executes them on any engine, and passes a script gate. Valid outputs are preserved, so re-running touches only missing or invalid work.

| Stage | Produce | Execute | Gate (exit 0) |
| --- | --- | --- | --- |
| Knowledge | `build_knowledge.py` → knowledge.json + rules.template.json | — | source discovery |
| Plan | `build_jobs.py` → prompts + jobs.json | — | source accounting + ownership |
| Review | — | jobs.json (defect + polish + sweeps) | `run_jobs.py --validate-only` |
| Merge | `merge_findings.py` → findings.json + review-stats.json | — | complete two-lane coverage |
| Report | `render_review.py` → review.md + state.json; `render_html.py` → review.html | — | `render_review.py` |

All job kinds (`cohort`, `polish`, `sweep`) return the same schema: defects, advisories, objective suppressions, hunk coverage, and rule coverage. `hunk` is the assigned canonical range (`<side>:<start>-<end>`), null outside the diff. Defects use the causal certificate; advisories use the improvement certificate.

## Cohort rules (Step 2)

1. Group selected files by package/directory and domain: a source file, its tests, and its types travel together; a file pulled apart from its test loses its reviewer the cheapest evidence.
2. Size: ≤ `--max-cohort-files` files (default `100`) **and** ≤ ~6,000 changed lines per cohort, whichever binds first. Pass the same value to `build_jobs.py`; a single oversized file becomes its own cohort.
3. **Oversized-file split** — when one file alone exceeds ~6,000 changed lines, divide the search across sibling reviewers: same file, disjoint slices of its manifest hunks (`hunk_scope`), one cohort per slice. Every slice reviewer reads the whole file for context but judges only its slice; build_jobs.py proves the merged slices cover every hunk line exactly once.
4. Tag each cohort `risk: high|normal|low` — high when it touches storage/migrations, security/auth, public contracts, or concurrency; low for docs/config-only. Risk feeds reviewer emphasis, not selection.
5. Every selected file in exactly one cohort (or, when sliced, every hunk line in exactly one slice) — build_jobs.py rejects any other shape. `plan.json`:

```json
{ "cohorts": [
    { "id": "c01", "name": "store: task queue", "risk": "high",
      "files": ["internal/store/queue.go", "internal/store/queue_test.go"] },
    { "id": "c02a", "name": "loop/action.go — hunks 1-14", "risk": "high",
      "files": ["internal/loop/action.go"],
      "hunk_scope": { "internal/loop/action.go": [{"start": 12, "lines": 40, "side": "new"}] } }
  ],
  "sweeps": ["contracts", {"key": "layering", "lens": "custom lens text"}] }
```

Sweeps are bare keys from the table below (built-in lens text) or `{key, lens}` objects for a custom lens.

`build_jobs.py` derives a second polish partition automatically: ≤20 files and ≤1,200 changed lines, splitting oversized hunks when needed. Every selected hunk line therefore has one defect owner and one polish owner without complicating plan.json.

## Sweep triggers

Sweeps are **opt-in and rare** — default to none. Each sweep is one extra agent that sees the manifest, not one cohort; include it only when its trigger clearly fires, and prefer at most one or two per round:

| Key | Trigger | Looks for |
| --- | --- | --- |
| `contracts` | exported/wire/API symbol changed contract | breaking changes, drift between spec/impl/clients, missing codegen co-ship |
| `security` | new endpoint/input path/authz surface/secret handling | injection, missing authn/authz, secret leakage, cross-tenant access |
| `migrations` | schema/migration files in diff | destructive ops, missing migration for model change, ordering/identity hazards |
| `tests` | any behavior change | new behavior without a failing-capable test, tests asserting mocks, weakened assertions |
| `consistency` | renames or repeated patterns in diff | incomplete renames, sibling paths not mirroring a fix, duplicated logic |
| `config` | config keys/flags/env vars changed | unwired or undocumented keys, dead flags, default mismatches |
| `spec-parity` | `--spec` provided (always included then) | field-by-field conformance with every artifact in the context pack's Spec contract section |

## Engines

The jobs contract makes engines interchangeable — pick one per run, record it in walkthrough.md's Review details (`Mode: workflow | agent-fallback | subagent:<runtime>`), and always close the loop with `run_jobs.py --validate-only`. Validation rejects missing coverage rows, unaccounted rules, wrong-lane results, and silent suppressions.

**Workflow (default).** One generic script executes any stage's pending jobs — pass the pending list from the validate-only status file as `args.jobs`:

```js
export const meta = {
  name: 'deep-review-jobs',
  description: 'Execute pending deep-review jobs; each agent reads a prompt file and writes one output file',
  phases: [{ title: 'Execute' }],
}
// args: { jobs: [{label, prompt, output}] } — PENDING jobs only
phase('Execute')
const acks = await parallel(args.jobs.map(j => () =>
  agent(`Read ${j.prompt} and follow it exactly. It defines the review task, the JSON ` +
        `schema, and the single file you write (${j.output}). Repo files are read-only. ` +
        `Reply with one sentence once the artifact is written.`,
    { label: j.label, phase: 'Execute' })))
return { dispatched: args.jobs.length, returned: acks.filter(Boolean).length }
```

After the workflow returns, run the validate-only gate; re-invoke with the still-pending jobs (interrupted runs can also resume via `resumeFromRunId`). Two re-dispatches without progress → inspect a failing output by hand before continuing.

**Agent fallback (`--no-workflow` or no Workflow tool).** Same contract through the Agent tool: dispatch each pending job's prompt file to a subagent ("Read `<prompt>` and follow it exactly…"), at most 6 concurrent, then the validate-only gate.

**External runtimes (`--subagent` ≠ `native`).** `run_jobs.py --command` drives `compozy exec` per subagent-runtimes.md — the runner owns concurrency, retries, output validation, provider-block detection, and the freeze check.

The orchestrator never reviews inline, regardless of PR size: reviewers spend their own context on their cohort; the orchestrator plans, dispatches, gates, and reports.

---
name: deep-review
description: "Review branch diffs, working trees, or PRs in depth with defect, polish and cross-contract coverage. Includes incremental and spec-conformance reviews; excludes fixes, spec-document reviews and quick single-file feedback."
disable-model-invocation: true
argument-hint: "[--pr N | --base REF | --staged | --worktree] [--files p1,p2] [--spec PATH] [--subagent native|claude-opus|grok|codex] [--max-cohort-files N] [--max-cohort-lines N] [--max-polish-files N] [--max-polish-lines N] [--publish] [--full] [--out DIR]"
---

# Deep Review

Find actionable defects and improvements with complete selected-hunk and project-rule accounting. Scripts handle discovery, partitions, artifact assembly and validation; the model supplies applicability decisions and evidence-based review. Keep source read-only; write review artifacts under `<out>` (default `.deep-review/<target>/`). Resolve `<skill-dir>` from this file and run bundled commands from the reviewed repository root.

## Contract

- No total file or line cap on the PR. Every selected hunk has exactly one defect owner and one polish owner. Both default to broad cohorts of **200 files / 15,000 changed lines per job**, adjustable with `--max-cohort-files`, `--max-cohort-lines`, `--max-polish-files` and `--max-polish-lines`. Oversized files are sliced without losing hunks. An optional `--max-context-lines N` preparation budget accounts for whole-file reading while preserving selection.
- Every defect has causal evidence; every advisory has an observed premise, concrete improvement and bounded fix. Investigated rejections stay in the suppression ledger. No quota; advisories never block SHIP.
- Every applied rule comes from material actually read, with verbatim text, source, scope and explicit assessment. Root/nested instructions retain scope and precedence. Metadata is a triage queue, not an instruction to load every skill.
- Relevant linters run first or reuse evidence for the same frozen inputs. Record unavailable lanes; suppress overlapping findings. Never install tools to fill a lane.
- The checkout, rule evidence and job contracts are pinned. Valid outputs are reused only for their unchanged contracts. Missing coverage, stale evidence and malformed results prevent completion.
- Publish only with `--publish` or explicit session authorization. Product fixes are outside this skill.

## 1. Prepare

```bash
python3 <skill-dir>/scripts/prepare_review.py --out <out> \
  [--pr N | --base REF | --staged | --worktree] [--files p1,p2] \
  [--spec PATH] [--full] [--max-cohort-files N] [--max-cohort-lines N] \
  [--max-polish-files N] [--max-polish-lines N]
```

The helper creates the manifest, `knowledge.md` and `decisions.template.json`. Inspect the manifest summary for correct base/head and file selection, especially after a rebase. Empty selection ends as “nothing reviewable.” For PRs, fetch the missing head/base/history using the manifest's diagnostic and retry.

Read `knowledge.md`, not the raw registry. Use its metadata to classify routers; read applicable instructions and selected skill sections. Copy the template with `cp <out>/decisions.template.json <out>/decisions.json`, then edit only the semantic decisions, preserving fingerprints. Use compact source IDs or grouped exclusions; supply actual reasons, exact rule spans, change intent and linter evidence. Read [context-pack.md — Decision file](references/context-pack.md#decision-file) for this input contract, including rule lane/lens binding.

```bash
python3 <skill-dir>/scripts/prepare_review.py --out <out> --decisions <out>/decisions.json
```

Exit 2 means decisions remain: applied skill parents reveal their reference routes on the next compile. The refreshed `decisions.template.json` already preserves prior decisions and additional fields; copy it over `decisions.json` and edit the pending entries. Assess references from their routing conditions. Excluding a parent accounts for its dependent references; shared references remain pending while an applicable parent needs them. Missing required references block preparation.

Complete decisions compile `rules.json`, `context-pack.md`, a package-oriented `plan.json`, per-job contracts and `jobs.json`. Do not write preparation scripts, manually enumerate partitions, or author a walkthrough before dispatch. Adjust generated groups only for real semantic boundaries. Sweeps default to none; use [orchestration.md — Sweeps](references/orchestration.md#sweeps) only for a concrete cross-cohort hypothesis. `--spec` adds the conformance sweep automatically.

## 2. Review

Use native agents by default, at most six concurrent. A worker receives only its rendered prompt: “Read `<prompt>` and complete its assigned review; submit its draft with the local command in that prompt.” Start without inherited conversation where supported (`fork_turns=none`). The worker does not load this skill, global jobs/manifest files or orchestrator references. Its contract includes the necessary context, rules, hunk IDs, evidence grammar and output shape.

Workers fill the provided draft template with explicit assessments, then run:

```bash
python3 <skill-dir>/scripts/run_jobs.py --out <out> --job <label> --submit
```

This validates only that job and expands IDs into canonical output. Missing or invalid fields produce complete local diagnostics; repair the existing draft. No blanket `clear`/`compliant`, global validation in workers, or new investigation merely to reconstruct JSON.

The orchestrator alone runs the global gate after a batch, or to resume an interrupted round:

```bash
python3 <skill-dir>/scripts/run_jobs.py --out <out> --validate-only
```

Dispatch only pending/invalid status rows using their `prompt` field: invalid rows already point to a targeted repair prompt. Reuse valid outputs. Repeated failure without progress requires inspecting the actual diagnostic, not looping the same review request. For Workflow execution, read [orchestration.md — Engines](references/orchestration.md#engines); for non-native `--subagent`, read [subagent-runtimes.md](references/subagent-runtimes.md). External calls spend `compozy exec` credit; preserve the user-selected model/effort.

## 3. Report

After global validation exits 0:

```bash
python3 <skill-dir>/scripts/merge_findings.py --out <out>
python3 <skill-dir>/scripts/render_review.py --out <out> [--rework "<structural rationale>"]
python3 <skill-dir>/scripts/render_html.py --out <out>
```

The renderer builds the walkthrough from the plan, intent and reviewer summaries at report time. It derives **SHIP / FIX_BEFORE_SHIP / REWORK** from defects, preserves advisories/suppressions and writes state. State the verdict only after render succeeds; include counts, every Critical/Major defect, coverage and the `review.html` path. Use ReportFindings when available, defects first and advisories afterward. Read [output-contracts.md](references/output-contracts.md) only when changing presentation or resolving a report diagnostic.

## Scope, configuration and recovery

`--base`, `--staged`, `--worktree`, `--files`, `--pr`, `--full` retain the manifest builder's scope semantics. Worktree review includes uncommitted/untracked files and is always full. Prior state scopes normal rounds incrementally; `--full` requests a complete round. `--spec` accepts a file/directory and additional canonical artifacts can be named in decisions.

Batch limits accept any positive integer and never truncate the PR. Preparation preserves selected limits across decision stages, resume and refresh in `prepare-request.json`; passing a limit again overrides only that setting. Direct `build_jobs.py` inherits plan limits and accepts the same four batch flags.

Repo `.deep-review.yaml` keys `path_filters`, `path_instructions`, `request_changes_workflow` fall back individually to `.coderabbit.yaml` (`reviews.*`). Built-in generated/vendor/lock/snapshot exclusions remain. Path instructions have highest rubric precedence, followed by deepest applicable project instructions, root instructions, routed skills and learnings.

Re-run preparation without `--refresh` to resume pinned inputs. A changed scope/source needs `--refresh` and fresh applicability evidence; never bless the old snapshot manually. Provider block is runner exit 2 with `run-blocker.json`; resume the same command when available. Runner/global freeze exit 3 means stale inputs. Invalid input exit 1 needs artifact repair, never bypassed coverage. `prepare-status.json`, per-job statuses and external attempt logs expose timing without additional model work.

With `--publish`, read [publish-github.md](references/publish-github.md) and execute its GitHub recipes after local rendering. When a user rebuts a finding, read [state-and-learnings.md](references/state-and-learnings.md) for the correction workflow. Read [taxonomy.md](references/taxonomy.md) only when an evidence/severity decision is unclear; the normal reviewer prompt already contains its required grammar.

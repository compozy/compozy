---
name: agent-exploration
description: >-
  Coordinate native explorers for explicitly requested multi-area research with
  scoped written results and an evidence-backed synthesis.
disable-model-invocation: true
argument-hint: "[--path <dir>] [--agents <num>] [--prompt <text>] [--model <model>]"
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Agent Exploration

Use when explicitly requested for research across independent areas with written results. Use the current harness's native explorers and configured model/effort; an explicit user override wins. Do not invoke an external runtime or create an agent registry for this workflow.

## Scope and dispatch

1. Resolve the question and output path from the request or active task. Ask only for genuinely missing scope; creating the requested directory needs no additional confirmation. Default an unspecified artifact location to `.audits/exploration/<timestamp>/` and state it.
2. Reuse current grounding. Scout only enough to identify independent questions; no tool-call or source-count quota. Choose as many useful slices as capacity and scope justify, up to 8; `--agents` is an upper bound, not a target to pad.
3. Read `references/dispatch-rules.md` and `assets/explorer-prompt.md` once when preparing dispatch. Send each explorer its question, source scope, exact output path, and the scoped-write contract once. Include `assets/analysis-template.md` when a structured artifact is needed; do not duplicate the contract in several embedded documents.
4. Create the output directory, then dispatch independent slices as capacity permits. Prompts need durable copies only for a long/resumable run. Every worker is read-only except for its named analysis file; no commits or unrelated edits.
5. Inspect each completed artifact and its material citations. A failed slice does not invalidate successful siblings. Continue or correct the affected worker; do not launch a fresh whole round for a missing heading or confirmation sentence.
6. Write `analysis/summary.md` with the answer, evidence links, disagreements, remaining gaps, and actionable conclusions. Distinguish completed and unavailable slices; never claim an unperformed investigation succeeded.

Use `analysis/NN_analysis_<slug>.md` for each slice. Preserve externally required schemas; otherwise keep sections as short as their evidence permits. `references/checklist.md` is an inspection aid, not an extra approval stage. Routine single-file lookups need no delegation or research artifact.

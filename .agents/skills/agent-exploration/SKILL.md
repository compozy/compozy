---
name: agent-exploration
description: >-
  Dispatches scoped-write explorer subagents in parallel — always through the
  current harness's native subagent facility — for multi-area research that
  must produce written artifacts: one seven-section analysis file per slice
  plus a parent-authored summary.
disable-model-invocation: true
argument-hint: "[--path <dir>] [--agents <num>] [--prompt <text>] [--model <model>]"
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Agent Exploration

Generic parallel-research workflow. Use when a question requires deep reads across multiple distinct areas and the operator needs written artifacts (not chat output). The skill dispatches scoped-write `explorer` subagents in parallel; each invocation writes one analysis file. The parent then synthesizes a final summary.

**Every slice runs as a native subagent of the current harness**, inheriting the session's model and reasoning effort unless the operator pins a model. There is no agent registry, no external CLI, and nothing to install: the explorer's role, contract, and schema travel inside each slice prompt.

## Required Reading Router

Match your step to the row. Read the listed files **in full before** producing output. They are not appendices — they are load-bearing. Inline content in this SKILL.md is a pointer, not a substitute.

| Step                                                    | MUST read                                                                                      |
| ------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Step 3 — composing every slice prompt                   | `references/dispatch-rules.md` + `assets/explorer-prompt.md` + `assets/analysis-template.md`   |
| Step 4 — verifying outputs                              | `references/checklist.md` + `assets/analysis-template.md`                                      |
| Step 5 — synthesizing `summary.md`                      | every `<path>/analysis/NN_analysis_<slug>.md` from this round                                  |
| Any contract violation, fabricated evidence, or retry   | `references/dispatch-rules.md` (re-read; do not paraphrase from memory)                        |

## Reference Index

- `references/dispatch-rules.md` — the scoped-write contract: what the dispatched subagent may write, may read, may run; tool allow/forbid lists; parent responsibilities; parallelism cap; failure handling. **Must be embedded verbatim in every slice prompt.**
- `references/checklist.md` — seven-section output validation checklist (runtime, inputs, scout, dispatch, files, schema, summary). Run before authoring `summary.md`.
- `assets/analysis-template.md` — the canonical seven-section schema every dispatched subagent fills (Overview, Mechanisms/Patterns, Relevant Sources, Transferable Patterns, Risks/Mismatches, Open Questions, Evidence) plus a Scope header.
- `assets/explorer-prompt.md` — the explorer role prompt (the scoped-write contract and workflow from the dispatched subagent's perspective). **Embedded verbatim at the top of every slice prompt.**

## Bundled Path Rule

Resolve every bundled file relative to the directory that holds this `SKILL.md`. When a path appears below as `references/<name>` or `assets/<name>`, expand it to `<agent-exploration-dir>/references/<name>` — the absolute skill directory — before reading or embedding it.

## Required Inputs

- `--path <dir>` (required): Output directory. Analysis files are written under `<path>/analysis/`. Any project-relative or absolute directory works (for example `docs/research/<topic>/`, `tasks/<slug>/`, or a path outside the repo). The skill is not tied to any specific project layout.
- `--agents <num>` (optional, default 3, hard cap 8): Number of explorer subagents to dispatch in parallel.
- `--prompt <text>` (required): The research question. Quoted multi-line strings are supported. If omitted, the parent asks the operator before continuing.
- `--model <name>` (optional; default: inherit the session's model): Per-slice model override, forwarded through the harness's native per-subagent model parameter. If the harness exposes no such parameter, stop and tell the operator — never substitute a different model than requested.

If `--path` or `--prompt` is missing, the parent asks the operator a single clarification before continuing. Never invent defaults for either. Apply the documented defaults for `--agents` and `--model` silently when omitted.

## Output Layout

```
<path>/analysis/
├── 01_analysis_<slug-a>.md
├── 02_analysis_<slug-b>.md
├── 03_analysis_<slug-c>.md
└── summary.md
```

- File numbering is zero-padded to two digits (`01`, `02`, …, `08`).
- Each slug is a short kebab-case identifier the parent assigns during the scout (Step 2), reflecting that slice's focus.
- `summary.md` is parent-authored synthesis, not a dispatched output.

## Procedures

**Step 1: Resolve inputs**

1. Confirm the current harness exposes a native subagent facility. If it does not, stop and tell the operator — this skill dispatches slices only as native subagents.
2. Parse `--path`, `--agents`, `--prompt`, `--model` from the invocation. If `--path` or `--prompt` is missing, ask the operator and stop.
3. Default `--agents` to `3` when omitted. Reject values below 1 or above 8 — ask the operator to choose a value in range.
4. Do not validate `--model` ahead of time — the harness surfaces incompatibilities at dispatch; when one does, report it to the operator and re-dispatch the slice with the model they choose.
5. Resolve `--path` to an absolute path. If the directory does not exist, ask the operator whether to create it before continuing; if creation fails, stop and report the filesystem error.
6. Create `<path>/analysis/` if absent. The dispatched subagents refuse to write into a missing directory.

**Step 2: Parent-led initial scout (MANDATORY — do not skip)**

The scout is the load-bearing step that prevents wasted parallel dispatch. The parent must do this work itself before any slice is launched.

1. Perform a brief read-only exploration of the problem space using path globs, content searches, and targeted file reads. The scout's job is to learn enough about the territory to divide it well — not to produce analysis content. Cap the scout at 8–15 tool calls; deep reading belongs to the dispatched subagents.
2. From the scout, identify exactly `--agents` distinct slices that are:
   - **Non-overlapping** — two slices should not require reading the same primary files for the same purpose.
   - **Independently answerable** — a slice's analysis must not depend on another slice's output.
   - **Aligned with the operator's `--prompt`** — every slice serves the original research question.
3. For each slice, assign:
   - A two-digit ordinal (`01`..`08`).
   - A short kebab-case slug (≤ 4 words) reflecting that slice's focus (e.g. `state-machine`, `event-bus`, `auth-boundaries`).
   - A focused per-slice prompt that names the slice question, the primary source paths/URLs to read, and any cross-references the dispatched subagent should use.
4. Briefly tell the operator the slice list (one line per slice: `NN – slug – focus`) before dispatching. Do not ask for approval unless the slices look thin or overlap; just announce and proceed.

If the scout reveals that fewer than `--agents` non-overlapping slices exist, reduce the dispatch count and tell the operator. Do not pad slices to hit the requested count.

**Step 3: Dispatch explorer subagents in parallel**

Gist tripwires — the contract items the parent must enforce in every dispatched prompt:

- The prompt names three things: slice scope, slug+ordinal, exact target file path. If any is missing, the dispatched subagent must refuse and ask back.
- The dispatched subagent gets exactly one file-write — at the named target path — and nothing else. No edits, no `git`/`make`/package managers, no writes outside `<path>/analysis/`.
- All slices dispatch in parallel — one subagent call per slice, issued in a single batch. Wait for every slice to finish before verification.

**STOP. Read `references/dispatch-rules.md` in full before composing any slice prompt.** That file contains the complete scoped-write contract, tool allow/forbid lists, parent responsibilities, and failure handling. The bullets above are tripwires, not the contract — the contract must be embedded verbatim in every slice prompt.

**STOP. Read `assets/explorer-prompt.md` in full before composing any slice prompt.** That file is the explorer's role prompt — it must open every slice prompt verbatim so the dispatched subagent knows its contract.

**STOP. Read `assets/analysis-template.md` in full before composing any slice prompt.** That file is the canonical seven-section schema every dispatched subagent fills. The schema must be embedded in the prompt; do not paraphrase it.

Compose one slice prompt per slice. Every prompt MUST include, in this order:
- `assets/explorer-prompt.md` content embedded verbatim (the explorer's role and contract).
- The operator's original `--prompt` verbatim, prefixed by a short orientation line.
- The slice's focused question and the primary sources to read.
- The exact target path: `<path>/analysis/NN_analysis_<slug>.md` (absolute path).
- `references/dispatch-rules.md` content embedded verbatim (copy-paste, do not paraphrase).
- The seven-section schema from `assets/analysis-template.md`.

Write each composed prompt to its own file at `<path>/.dispatch/prompts/NN_<slug>.txt` — the round's audit trail and the exact text to re-dispatch on a slice failure. The file basename (without extension) is the slice id.

Issue one subagent call per slice — all in a single parallel batch, using whatever async/parallel subagent facility the harness exposes. Each call passes the slice prompt file content verbatim as the subagent's prompt, grants read/search access plus file-write capability (the embedded contract confines the subagent to one write), and applies a model override only when the operator pinned one. Wait for every subagent to complete before Step 4. A subagent that errors, or returns without its written-path confirmation, is a failed slice — re-dispatch it with the contract restated. Never synthesize a missing slice's analysis as if its dispatch succeeded.

**Step 4: Verify outputs**

Gist tripwires — the floor items that catch most failures:

- Every slice finished clean — its subagent returned the written-path confirmation (path written, seven sections). Anything else is a slice failure, not a warning.
- Exactly `N` files at the expected `NN_analysis_<slug>.md` paths under `<path>/analysis/`.
- All seven schema sections present in each file; no empty sections without a gap-note + Open Question.
- At least one cited source per file sample-checked (read local paths; check URL well-formedness).

**STOP. Read `references/checklist.md` in full before declaring outputs verified.** That file is the seven-section output validation checklist (runtime, inputs, scout, dispatch, files, schema, summary). Every item must pass; failing items trigger a re-dispatch of the offending slice. The bullets above are tripwires, not the contract.

If a section is empty, a file is missing, a cited path is fake, or the schema is incomplete, re-dispatch the offending slice with the schema embedded and a request to fill the gap. The parent never authors the missing analysis content — the dispatched subagent owns the write.

**Step 5: Synthesize `summary.md`**

1. Read every `<path>/analysis/NN_analysis_<slug>.md` in full.
2. Author `<path>/analysis/summary.md` with these sections:
   - **Research Question** — the operator's `--prompt`, verbatim.
   - **Slice Map** — table mapping each `NN – slug` to its slice question and one-line finding.
   - **Convergences** — patterns or risks that appear in two or more analyses, with cross-citations to the slice files.
   - **Divergences** — places where slices disagree or where one slice surfaces a finding the others miss.
   - **Risks & Open Questions** — consolidated, deduplicated list pulled from each analysis's Open Questions and Risks/Mismatches sections.
   - **Recommended Next Steps** — short, actionable list. Each step cites the slice file(s) that support it.
   - **Index** — bullet list of `<path>/analysis/NN_analysis_<slug>.md` paths so a future reader can drill in.
3. `summary.md` is parent-authored. Do not dispatch a slice for this step.

## When Not To Use

- **Single-file lookups** ("where is X defined?", "what does function Y return?"): answer directly with the harness's search and file-reading tools. This skill is overkill.
- **Edits to existing code**: the explorer is scoped-write — it can only create new analysis files, not modify anything else.
- **Tightly scoped competitor / reference-repo research** in projects that already ship a more specialized variant (for example a project-local skill that mirrors a fixed competitor catalog). Use that variant when it exists; use this skill as the generic fallback.

## Error Handling

Input and scout failures are handled inline where they occur — each item in Steps 1–2 names its own recovery. Contract violations, fabricated evidence, schema-incomplete analyses, and retries route through the Required Reading Router: **STOP, re-read `references/dispatch-rules.md` in full**, then re-dispatch the offending slice — the dispatched subagent owns the write. One round-level rule lives only here:

- **Network/disk error during dispatch:** fail the round entirely — a half-set of analyses is unacceptable. Re-dispatch after the error is resolved.

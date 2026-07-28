---
name: agent-exploration
description: >-
  Dispatches scoped-write explorer agents in parallel via `compozy exec` for
  multi-area research that must produce written artifacts — one seven-section
  analysis file per slice plus a parent-authored summary. Use when a research
  question spans several distinct areas and chat output is not enough. Do not
  use for single-file lookups (use Explore) or edits to existing code.
trigger: explicit
argument-hint: "[--path <dir>] [--agents <num>] [--prompt <text>] [--ide <ide>] [--model <model>] [--reasoning <effort>]"
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Agent Exploration

Generic parallel-research workflow. Use when a question requires deep reads across multiple distinct areas and the operator needs written artifacts (not chat output). The skill dispatches `explorer` agents in parallel through `compozy exec`; each invocation writes one analysis file. The parent then synthesizes a final summary.

The skill is self-contained and harness-agnostic. The explorer is a Compozy agent — a single definition (`assets/AGENT.md`) installed once at `~/.compozy/agents/explorer/AGENT.md`, discoverable by `compozy exec --agent explorer` from any harness (Claude Code, Codex CLI, Cursor, Droid, OpenCode, Pi, Gemini, Copilot, …). The parent runtime is selected per invocation via `--ide`, `--model`, and `--reasoning`. Parallel dispatch uses whatever async/background tool calls your harness exposes — the skill does not prescribe a specific tool.

## Required Reading Router

Match your step to the row. Read the listed files **in full before** producing output. They are not appendices — they are load-bearing. Inline content in this SKILL.md is a pointer, not a substitute.

| Step                                                    | MUST read                                                              |
| ------------------------------------------------------- | ---------------------------------------------------------------------- |
| Step 4 — composing every slice prompt                   | `references/dispatch-rules.md` + `assets/analysis-template.md`         |
| Step 5 — verifying outputs                              | `references/checklist.md` + `assets/analysis-template.md`              |
| Step 6 — synthesizing `summary.md`                      | every `<path>/analysis/NN_analysis_<slug>.md` from this round          |
| Any contract violation, fabricated evidence, or retry   | `references/dispatch-rules.md` (re-read; do not paraphrase from memory)|

## Reference Index

- `references/dispatch-rules.md` — the scoped-write contract: what the dispatched agent may write, may read, may run; tool allow/forbid lists; parent responsibilities; parallelism cap; failure handling. **Must be embedded verbatim in every slice prompt.**
- `references/checklist.md` — seven-section output validation checklist (installation, inputs, scout, dispatch, files, schema, summary). Run before authoring `summary.md`.
- `assets/analysis-template.md` — the canonical seven-section schema every dispatched agent fills (Overview, Mechanisms/Patterns, Relevant Sources, Transferable Patterns, Risks/Mismatches, Open Questions, Evidence) plus a Scope header.
- `assets/AGENT.md` — the Compozy explorer agent definition (frontmatter: `title`, `description`, `ide`, `model`, `reasoning_effort`, `access_mode`; body: the scoped-write contract and workflow). Installed by `scripts/install-explorer.sh` to `~/.compozy/agents/explorer/AGENT.md`.
- `scripts/install-explorer.sh` — bootstrap helper. Writes the bundled `assets/AGENT.md` to the Compozy global registry. Refuses to overwrite.
- `scripts/dispatch-slices.sh` — parallel dispatch runner. Takes `--ide`/`--model`/`--reasoning` plus 1-8 prompt files, backgrounds one `compozy exec` per file, waits via `wait $pid`, captures per-slice stdout/stderr/exit, and reports a summary. Zero external dependencies (native bash + the `compozy` binary).

## Bundled Path Rule

Resolve every bundled helper relative to the directory that holds this `SKILL.md`. When a command appears below as `scripts/<name>`, treat the actual invocation as `<agent-exploration-dir>/scripts/<name>` — expand `<agent-exploration-dir>` to the absolute skill directory before running.

## Required Inputs

- `--path <dir>` (required): Output directory. Analysis files are written under `<path>/analysis/`. Any project-relative or absolute directory works (for example `docs/research/<topic>/`, `tasks/<slug>/`, or a path outside the repo). The skill is not tied to any specific project layout.
- `--agents <num>` (optional, default 3, hard cap 8): Number of explorer invocations to dispatch in parallel.
- `--prompt <text>` (required): The research question. Quoted multi-line strings are supported. If omitted, the parent asks the operator before continuing.
- `--ide <ide>` (optional, default `claude`): Compozy runtime for each dispatched invocation. Forwarded to `compozy exec --ide`. Accepted values mirror `compozy exec`: `codex`, `claude`, `cursor-agent`, `droid`, `opencode`, `pi`, `gemini`, `copilot`.
- `--model <name>` (optional, default `opus`): Forwarded to `compozy exec --model`. See **Model Options** for the two supported profiles (Opus default; Grok 4.5 High Fast via `--ide cursor-agent --model 'grok-4.5[effort=high,fast=true]'`). When the chosen IDE does not support the requested model, `compozy` surfaces the error and the slice fails — re-dispatch with a compatible model.
- `--reasoning <effort>` (optional, default `xhigh`): Forwarded to `compozy exec --reasoning-effort`. Accepted values: `low`, `medium`, `high`, `xhigh`.

If `--path` or `--prompt` is missing, the parent asks the operator a single clarification before continuing. Never invent defaults for either. Apply the documented defaults for `--ide`, `--model`, `--reasoning` silently when omitted; reject an invalid `--ide` rather than falling back.

## Model Options

Two dispatch profiles are supported. Opus is the default; Grok 4.5 High Fast is the fast alternative for breadth work. Pick per run via `--ide`/`--model`.

| Profile                                | Flags                                                          | When                                                       |
| -------------------------------------- | -------------------------------------------------------------- | ----------------------------------------------------------- |
| Opus (default, high-fidelity)           | `--ide claude --model opus --reasoning xhigh`                   | Hardest slices; when citation precision must be maximal.    |
| Grok 4.5 High Fast (via cursor-agent)   | `--ide cursor-agent --model 'grok-4.5[effort=high,fast=true]'`  | Breadth research, cost-sensitive runs, high slice counts.   |

Grok profile notes:

- Reasoning effort and fast mode are pinned inside the model string (`[effort=high,fast=true]`); the `--reasoning` flag does not control this profile.
- Single-quote the model string — the square brackets are shell glob characters.

## Output Layout

```
<path>/analysis/
├── 01_analysis_<slug-a>.md
├── 02_analysis_<slug-b>.md
├── 03_analysis_<slug-c>.md
└── summary.md
```

- File numbering is zero-padded to two digits (`01`, `02`, …, `08`).
- Each slug is a short kebab-case identifier the parent assigns during the scout (Step 3), reflecting that slice's focus.
- `summary.md` is parent-authored synthesis, not a dispatched output.

## Procedures

**Step 1: Verify the explorer agent is installed**

1. Confirm the `compozy` binary is on `PATH` (e.g. `command -v compozy`). If missing, abort with a one-line message instructing the operator to install Compozy from `/Users/pedronauck/dev/compozy/looper`. Do not fall back to harness-native subagents.
2. Check for the explorer agent in the Compozy registry:
   - Global (preferred): `~/.compozy/agents/explorer/AGENT.md`.
   - Workspace override (optional, takes precedence): `<repo>/.compozy/agents/explorer/AGENT.md`.
   At least one must be present.
3. If both are absent, ask the operator a single question: "The `explorer` Compozy agent is not installed at `~/.compozy/agents/explorer/AGENT.md`. Install it now? [yes/no]". Do not proceed silently.
4. On "yes", run the bundled bootstrap helper: `<agent-exploration-dir>/scripts/install-explorer.sh`. The helper installs to `~/.compozy/agents/explorer/AGENT.md` and refuses to overwrite an existing file.
5. On "no", abort the dispatch with a one-line message — the agent definition is never inlined into a slice prompt.
6. After installation, re-check that the file exists before continuing.
7. (Optional sanity check) Run `compozy agents inspect explorer` and confirm the entry is discoverable. Any error here blocks dispatch.

**Step 2: Resolve inputs**

1. Parse `--path`, `--agents`, `--prompt`, `--ide`, `--model`, `--reasoning` from the invocation. If `--path` or `--prompt` is missing, ask the operator and stop.
2. Default `--agents` to `3` when omitted. Reject values below 1 or above 8 — ask the operator to choose a value in range.
3. Default `--ide` to `claude`, `--model` to `opus`, `--reasoning` to `xhigh` when omitted. Validate `--ide` against the accepted list (`codex`, `claude`, `cursor-agent`, `droid`, `opencode`, `pi`, `gemini`, `copilot`) and `--reasoning` against `low`, `medium`, `high`, `xhigh`; reject invalid values with a clear message instead of silent fallback. Do not validate `--model` ahead of time — let `compozy` surface incompatibilities.
4. Resolve `--path` to an absolute path. If the directory does not exist, ask the operator whether to create it before continuing; if creation fails, stop and report the filesystem error.
5. Create `<path>/analysis/` if absent. The dispatched agents refuse to write into a missing directory.

**Step 3: Parent-led initial scout (MANDATORY — do not skip)**

The scout is the load-bearing step that prevents wasted parallel dispatch. The parent must do this work itself before any slice is launched.

1. Perform a brief read-only exploration of the problem space using `Glob`, `Grep`, and targeted `Read` calls. The scout's job is to learn enough about the territory to divide it well — not to produce analysis content. Cap the scout at 8–15 tool calls; deep reading belongs to the dispatched agents.
2. From the scout, identify exactly `--agents` distinct slices that are:
   - **Non-overlapping** — two slices should not require reading the same primary files for the same purpose.
   - **Independently answerable** — a slice's analysis must not depend on another slice's output.
   - **Aligned with the operator's `--prompt`** — every slice serves the original research question.
3. For each slice, assign:
   - A two-digit ordinal (`01`..`08`).
   - A short kebab-case slug (≤ 4 words) reflecting that slice's focus (e.g. `state-machine`, `event-bus`, `auth-boundaries`).
   - A focused per-slice prompt that names the slice question, the primary source paths/URLs to read, and any cross-references the dispatched agent should use.
4. Briefly tell the operator the slice list (one line per slice: `NN – slug – focus`) before dispatching. Do not ask for approval unless the slices look thin or overlap; just announce and proceed.

If the scout reveals that fewer than `--agents` non-overlapping slices exist, reduce the dispatch count and tell the operator. Do not pad slices to hit the requested count.

**Step 4: Dispatch explorer agents in parallel**

Gist tripwires — the contract items the parent must enforce in every dispatched prompt:

- The prompt names three things: slice scope, slug+ordinal, exact target file path. If any is missing, the dispatched agent must refuse and ask back.
- The dispatched agent gets exactly one file-write — at the named target path — and nothing else. No edits, no `git`/`make`/package managers, no writes outside `<path>/analysis/`.
- All slices dispatch in parallel via `compozy exec`, with `--ide`/`--model`/`--reasoning-effort` forwarded from the operator's inputs. Wait for every process to exit (code 0) before verification.

**STOP. Read `references/dispatch-rules.md` in full before composing any slice prompt.** That file contains the complete scoped-write contract, tool allow/forbid lists, parent responsibilities, and failure handling. The bullets above are tripwires, not the contract — the contract must be embedded verbatim in every slice prompt.

**STOP. Read `assets/analysis-template.md` in full before composing any slice prompt.** That file is the canonical seven-section schema every dispatched agent fills. The schema must be embedded in the prompt; do not paraphrase it.

Compose one slice prompt per slice. Every prompt MUST include:
- The operator's original `--prompt` verbatim, prefixed by a short orientation line.
- The slice's focused question and the primary sources to read.
- The exact target path: `<path>/analysis/NN_analysis_<slug>.md` (absolute path).
- `references/dispatch-rules.md` content embedded verbatim (copy-paste, do not paraphrase).
- The seven-section schema from `assets/analysis-template.md`.

Write each composed prompt to its own file at `<path>/.dispatch/prompts/NN_<slug>.txt`. The file basename (without extension) becomes the slice id used for per-slice log file naming.

**Recommended dispatch path: `scripts/dispatch-slices.sh`.** The bundled script backgrounds one `compozy exec` per prompt file, waits for every PID, captures per-slice stdout/stderr/exit under `<logs-dir>`, and exits non-zero if any slice failed. Zero external dependencies; portable across any harness that can run a bash script.

```
<agent-exploration-dir>/scripts/dispatch-slices.sh \
  --ide <ide> --model <model> --reasoning <reasoning> \
  --logs <path>/.dispatch/logs \
  -- <path>/.dispatch/prompts/01_<slug-a>.txt \
     <path>/.dispatch/prompts/02_<slug-b>.txt \
     <path>/.dispatch/prompts/03_<slug-c>.txt
```

- `<ide>`, `<model>`, `<reasoning>` are the resolved operator inputs (defaults `claude`, `opus`, `xhigh`).
- The script prints `dispatched: <slug> pid=<N>` per launch and `exited: <slug> rc=<N>` per completion, ending with a `summary: total=Xs ok=N/M failed=K/M` line.
- Each prompt is passed through `compozy exec --prompt-file <file>` (no shell-escaping risk for long prompts).
- The script hard-caps at 8 slices per invocation, matching the parallelism cap in `references/dispatch-rules.md`.

**Manual alternative** (if you cannot run a bash script — e.g., a harness that prefers issuing N parallel tool calls itself): invoke each slice with the command shape below. Use whatever async/background facility your harness exposes; wait for every invocation to exit before continuing.

```
compozy exec \
  --agent explorer \
  --ide <ide> \
  --model <model> \
  --reasoning-effort <reasoning> \
  --prompt-file <path>/.dispatch/prompts/NN_<slug>.txt
```

Notes that apply to both paths:
- `compozy exec` already defaults `--access-mode` to `full`, so no extra runtime-permission flag is required.
- **When `--ide cursor-agent` (Grok profile), reasoning effort is pinned in the model string** (`grok-4.5[effort=high,fast=true]`); the forwarded `--reasoning-effort` value does not control it (see Model Options).
- Do not pin `--timeout` in the dispatch template. The Compozy default is an **activity timeout** (job canceled only when no output is received within the period), which the dispatched agent's normal tool-call streaming keeps reset. If a specific slice legitimately needs a longer silent window (e.g., synthesising over 25+ sources), the operator can append `--timeout 30m` (or higher) to that single invocation.
- Treat any non-zero exit code as a slice failure and re-dispatch that slice with the contract restated. Never synthesise a missing slice's analysis as if its dispatch succeeded.

**Step 5: Verify outputs**

Gist tripwires — the floor items that catch most failures:

- Every `compozy exec` exited 0. Non-zero exits are slice failures, not warnings.
- Exactly `N` files at the expected `NN_analysis_<slug>.md` paths under `<path>/analysis/`.
- All seven schema sections present in each file; no empty sections without a gap-note + Open Question.
- At least one cited source per file sample-checked (`Read` for local paths, well-formedness for URLs).

**STOP. Read `references/checklist.md` in full before declaring outputs verified.** That file is the seven-section output validation checklist (installation, inputs, scout, dispatch, files, schema, summary). Every item must pass; failing items trigger a re-dispatch of the offending slice. The bullets above are tripwires, not the contract.

If a section is empty, a file is missing, a cited path is fake, or the schema is incomplete, re-dispatch the offending slice via a fresh `compozy exec` with the schema embedded and a request to fill the gap. The parent never authors the missing analysis content — the dispatched agent owns the write.

**Step 6: Synthesize `summary.md`**

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

- **Single-file lookups** ("where is X defined?", "what does function Y return?"): use `Explore` or direct `Grep`/`Read`. This skill is overkill.
- **Edits to existing code**: the explorer is scoped-write — it can only create new analysis files, not modify anything else.
- **Tightly scoped competitor / reference-repo research** in projects that already ship a more specialized variant (for example a project-local skill that mirrors a fixed competitor catalog). Use that variant when it exists; use this skill as the generic fallback.

## Error Handling

Input, installation, and scout failures are handled inline where they occur — each item in Steps 1–3 names its own recovery. Contract violations, fabricated evidence, schema-incomplete analyses, and retries route through the Required Reading Router: **STOP, re-read `references/dispatch-rules.md` in full**, then re-dispatch the offending slice — the dispatched agent owns the write. One round-level rule lives only here:

- **Network/disk error during dispatch:** fail the round entirely — a half-set of analyses is unacceptable. Re-dispatch after the error is resolved.

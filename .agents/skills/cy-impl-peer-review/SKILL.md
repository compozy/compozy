---
name: cy-impl-peer-review
description: Runs an optional cross-LLM peer review of an implemented change via compozy exec --ide claude --model opus --reasoning-effort xhigh and packages findings for user-directed remediation. Use after any implementation pass (feature, bug fix, refactor) when the user explicitly asks for an external Opus review of the diff before commit or PR. Do not use for TechSpec review (use cy-spec-peer-review), automatic remediation, batched provider review fetching (use cy-fix-reviews), manual self-review without an external LLM (use cy-review-round), or auto-looped review cycles.
trigger: explicit
argument-hint: "[--files path1,path2] [--context path1,path2] [--base ref] [--out dir]"
---

# Implementation Peer Review

Claude Opus pressure-tests an implementation diff via `compozy exec`. This skill runs that pressure-test only when the user explicitly asks for a review round after an implementation pass. It is decoupled from any PRD/task tracking system — the scope is the diff itself plus any optional context files the user names. The skill never auto-runs, auto-incorporates findings, auto-commits, or auto-loops additional rounds.

## User Decisions

When this skill instructs the agent to ask whether to incorporate findings or run another round, it MUST use the runtime's dedicated interactive question tool — the tool or function that presents a question to the user and pauses execution until the user responds.

If the runtime does not provide such a tool, present the question as the complete assistant message and stop generating. Do not answer the question on the user's behalf.

## Optional Inputs

All inputs are optional. Defaults make the common path `cy-impl-peer-review` with no arguments.

- `--files <path1,path2,...>` — scope the review to explicit paths instead of the full branch diff.
- `--context <path1,path2,...>` — additional context files to feed Opus (e.g., a spec, ADR, design doc, RFC, README). The skill never assumes any of these exist.
- `--base <git-ref>` — base ref for the diff. Defaults to `main`. Use `--base HEAD~N` or `--staged` for narrower scopes.
- `--out <dir>` — output directory for round artifacts. Defaults to `.peer-reviews/<UTC-timestamp>/` at repo root.

## Procedures

**Step 1: Validate Input and Compute Scope**

1. Confirm the user has just completed (or paused) the implementation pass and explicitly asked to review the current state. Do not run review rounds during active editing.
2. Resolve the diff scope:
   - If `--files` is provided, verify each path exists and limit the diff to those paths.
   - If `--staged` is provided as the base, use `git diff --staged`.
   - Otherwise run `git diff <base>...HEAD --name-only` (default `<base>` is `main`) to compute the changed file set. If the diff is empty, abort and tell the user there is nothing to review.
3. Resolve the artifact directory:
   - Use `--out` if provided.
   - Otherwise default to `.peer-reviews/<UTC-timestamp-YYYYMMDDTHHMMSSZ>/` at the repository root.
   - Create the directory if it does not exist.
4. Read `.agents/skills/cy-impl-peer-review/references/readiness-checks.md` and verify every readiness marker passes (build/tests green, no committed `.tmp/` or `ai-docs/`, diff is non-empty, no obvious WIP markers in changed files, codegen co-ship if contracts touched, migration co-ship if schema touched, reviewable size). If any marker fails, report the failed markers and abort — Opus review on a broken or incomplete change wastes credit and produces noise.
5. Determine the next review round number by listing existing `impl-review-result-round*.json` files in the artifact directory. Start at `round1` when none exist.

**Step 2: Compose the Review Prompt**

1. Read `.agents/skills/cy-impl-peer-review/references/impl-review-prompt.md` for the canonical Opus prompt template.
2. Capture the diff payload:
   - Run `git diff <base>...HEAD -- <changed-files>` (or `git diff --staged -- <changed-files>` when the user named `--staged`) and write the raw patch to `<out>/impl-review-diff-roundN.patch`.
   - Run `git log --oneline <base>...HEAD -- <changed-files>` and capture the commit list (empty string if `--staged`).
3. Substitute the placeholders in the prompt template:
   - `{scope_summary}` — one-paragraph description of what was implemented. Derive from the user's brief, the commit messages, or — if the user passed `--context` — the linked spec/PRD summary.
   - `{context_paths}` — newline-separated repo-root paths from `--context`, or the literal string `none` when not provided.
   - `{changed_files}` — newline-separated repo-root paths.
   - `{diff_path}` — repo-root path to the patch file from step 2.
   - `{commit_list}` — captured `git log --oneline` output, or `none` if `--staged`.
4. Write the assembled prompt to `<out>/impl-review-prompt-roundN.md`.

**Step 3: Execute the Cross-LLM Review**

1. Run `compozy exec --ide claude --model opus --reasoning-effort xhigh --format json --prompt-file <out>/impl-review-prompt-roundN.md`.
2. Capture stdout to `<out>/impl-review-result-roundN.json` and stderr to `<out>/impl-review-result-roundN.err`.
3. If the command returns a non-zero exit code, fail loudly. Do not retry silently. Inspect the stderr for model misconfiguration (see Error Handling).

**Step 4: Summarize Findings**

1. Parse the JSON output. Expect four sections: `blockers`, `risks`, `nits`, `verdict`.
2. Write `<out>/impl-review-summary-roundN.md` with:
   - verdict (`SHIP` / `FIX_BEFORE_SHIP` / `REWORK`)
   - one-line rationale per blocker (file:line + issue + suggested fix shape)
   - risks list (latent or non-blocking concerns)
   - nits list
   - files most likely affected by remediation
3. Present a concise user-facing summary of the review. Include the verdict, blocker/risk/nit counts, the main themes, and the artifact paths written for the round.
4. Do NOT modify any source code, tests, configs, docs, or commits yet.

**Step 5: User-Directed Remediation**

1. Ask the user which findings to incorporate:
   - A) all blockers
   - B) selected blockers/risks/nits
   - C) nothing — keep the review as a record only
   - D) manual edits before any remediation
2. Apply only the findings the user selected. Do not silently apply all blockers, all risks, or all nits.
3. Re-run the project's verification gate (`make verify` in this repo, or whatever command the user names) after applying any code change. Do not declare remediation done if verification fails — fix the new failure or surface it back to the user.
4. Record the remediation decision in `<out>/impl-review-remediation-roundN.md`, listing:
   - incorporated items with the new commit/diff range
   - deferred items
   - files changed
   - verification command and outcome with timestamp
5. Show the user what changed and what remains deferred. Do not commit or push without an explicit user instruction.

**Step 6: Optional Additional Rounds**

1. Ask whether the user wants another peer-review round against the updated code or wants to stop with the current state.
2. If the user requests another round, re-run from Step 2 against the new diff and create a fresh `roundN+1` artifact set in the same `<out>` directory.
3. Do not auto-loop. The user explicitly requests further rounds.

## Critical Rules

- This skill never commits, pushes, opens PRs, or invokes provider review fetchers. Remediation is local-only; commit/PR steps belong to the user or `cy-fix-reviews`.
- The skill is not bound to any task-tracking directory layout. Every artifact lives under the resolved `<out>` directory and is versioned with `-roundN`. Never overwrite a prior round.
- The `compozy exec` call is the only place this skill spends external review credit. Do not invoke it more than once per round.
- The bundled helper paths used by this skill (`references/impl-review-prompt.md`, `references/readiness-checks.md`) are **read-only** templates — the skill reads them, never edits them.

## Error Handling

- **Model misconfiguration (`The model 'X' does not exist`):** stop and surface the configured model. The IDE may be set to a stale name like `gpt-5.5`. Do not mutate the call to substitute a model — verify with the user. (See `docs/_memory/lessons/L-010-model-name-validation.md`.)
- **`compozy exec` not found:** the skill assumes Compozy CLI is on `PATH`. If absent, fail with the install hint rather than swallowing.
- **Readiness markers missing:** if Step 1 readiness checks fail, do not run Opus. Print the failed markers and exit so the user can fix the underlying problem first.
- **Empty diff:** if `git diff` yields no changes, abort. There is nothing to review.
- **Oversized diff (`> 5000` changed lines or `> 80` files):** warn the user, ask whether to scope down with `--files`, and proceed only on explicit confirmation. Opus review on a sprawling diff produces shallow findings.
- **Empty Opus output:** treat empty `blockers`/`risks`/`nits`/`verdict` as suspect (likely a prompt or model issue). Re-prompt the user before declaring `SHIP`.
- **Existing peer-review files for the round:** never overwrite. Increment to the next `roundN` instead.
- **Verification failing during remediation:** stop and surface the new failure. Do not commit broken code to "preserve the review trail."

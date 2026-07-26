---
name: cy-spec-peer-review
description: Cross-LLM peer review of a TechSpec via compozy exec — scoped Markdown findings for user-directed incorporation. Use when a TechSpec draft has already been approved and the user wants an external review round, especially for autonomy/network/memory-impacting designs. Do not use for PRDs, automatic approval gates, code review batches, or auto-looped review cycles.
trigger: explicit
argument-hint: "[techspec-path] [--ide] [--model] [--reasoning]"
---

# Spec Peer Review

An independent reviewer runtime pressure-tests an approved TechSpec via `compozy exec`. This skill runs that pressure-test only when the user explicitly asks for a review round after approving the current draft. It does not auto-run, auto-incorporate findings, or auto-loop additional rounds.

The review result is a direct-written Markdown findings file. `compozy exec` stdout/stderr is operational evidence only; never parse it as the review source of truth.

## User Decisions

When this skill instructs the agent to ask whether to incorporate findings or run another round, it MUST use the runtime's dedicated interactive question tool — the tool or function that presents a question to the user and pauses execution until the user responds.

If the runtime does not provide such a tool, present the question as the complete assistant message and stop generating. Do not answer the question on the user's behalf.

## Bundled Path Rule

Resolve bundled helper paths relative to the directory that contains this `SKILL.md`. When invoking the validator from a repository root, use the full repo-relative path:

```bash
bash .agents/skills/cy-spec-peer-review/scripts/validate-findings.sh --kind techspec --round <N> --path <findings-path>
```

The validator is a read-only helper: it inspects the findings artifact and exits non-zero on structural contract violations.

## Required Inputs

- **techspec-path** (optional): explicit path to the `_techspec.md` under review. When omitted, resolve to the most recently modified `.compozy/tasks/<slug>/_techspec.md` whose sibling `_meta.md` shows `Pending: > 0` or no `_meta.md` exists yet.
- `--ide <ide>` (default `codex`) — forwarded to `compozy exec --ide`. Secondary option: `--ide claude --model opus --reasoning max`.
- `--model <model>` (default `gpt-5.6-sol`) — forwarded to `compozy exec --model`.
- `--reasoning <effort>` (default `xhigh`) — forwarded to `compozy exec --reasoning-effort`. Accepted: `low`, `medium`, `high`, `xhigh`, `max`.

## Findings Artifact Contract

Each review round has exactly one authoritative findings file:

```
.compozy/tasks/<slug>/qa/peer-review-findings-roundN.md
```

The reviewer may write exactly that file and no other file. If the target path is missing, ambiguous, unwritable, or outside the named `.compozy/tasks/<slug>/qa/` directory, the reviewer must refuse and stop. It must not print findings to stdout as a fallback.

The findings file MUST use this structure:

```markdown
---
schema_version: 1
review_kind: techspec
round: N
readiness: READY|BLOCKED|NEEDS_REWORK
reviewer_runtime: <resolved --ide>
reviewer_model: <resolved --model>
generated_at: <ISO-8601 timestamp>
---

# Summary

# Blockers

# Nits

# Evidence

# Deferred Or Follow-Up
```

Every blocker and nit must include an ID, a real section/path reference, the issue, and a concrete suggested fix. Blockers must also include the rationale for why the issue blocks approval.

## Procedures

**Step 1: Validate Input and Context**

1. Resolve `techspec-path`. If omitted, list candidate paths and pick the freshest.
2. Confirm the user has already approved the current draft or explicitly asked to review the saved spec as-is.
3. Read the spec and confirm it is a final-shape TechSpec (has `Architectural Boundaries`, `Implementation Steps`, `Test Strategy` sections) — not a draft.
4. Read `references/quality-markers.md` and verify the spec carries the six markers (boundary statement, listed boundaries, Go interface signatures, data-model field rationale, side-table-vs-JSON decisions, lease/safety invariants enumerated). If any marker is missing, abort and report the missing markers — external review is wasted on incomplete specs.
5. Resolve the slug from the path; ensure `.compozy/tasks/<slug>/` exists and is writable.
6. Ensure `.compozy/tasks/<slug>/qa/` exists before dispatch.
7. Determine the next review round number by listing existing `qa/peer-review-findings-round*.md`, `qa/peer-review-summary-round*.md`, and legacy `qa/peer-review-result-round*.json*` files (prior local output only — not a compatibility path). Start at `round1` when none exist.

**Step 2: Compose the Review Prompt**

1. Read `references/peer-review-prompt.md` for the canonical executable reviewer prompt template. The assembled prompt must start with the reviewer instructions, not with a Markdown wrapper describing the template.
2. Define the round artifact paths:
   - Findings target: `.compozy/tasks/<slug>/qa/peer-review-findings-roundN.md`.
   - Operational event log: `.compozy/tasks/<slug>/qa/peer-review-events-roundN.jsonl`.
   - Operational stderr log: `.compozy/tasks/<slug>/qa/peer-review-result-roundN.err`.
   - Pre-run status snapshot: `.compozy/tasks/<slug>/qa/peer-review-status-before-roundN.txt`.
   - Post-run status snapshot: `.compozy/tasks/<slug>/qa/peer-review-status-after-roundN.txt`.
   - Validation error, only when needed: `.compozy/tasks/<slug>/qa/peer-review-validation-error-roundN.md`.
3. Substitute the placeholders:
   - `{techspec_path}` — exact path to the TechSpec under review.
   - `{adr_paths}` — any `adrs/*.md` siblings, or `none`.
   - `{related_research}` — any `analysis/*.md` siblings, or `none`.
   - `{findings_path}` — exact absolute path to `.compozy/tasks/<slug>/qa/peer-review-findings-roundN.md`.
   - `{round}` — numeric review round `N`.
   - `{reviewer_runtime}` — resolved `--ide` (default `codex`).
   - `{reviewer_model}` — resolved `--model` (default `gpt-5.6-sol`).
4. Write the assembled prompt to `.compozy/tasks/<slug>/qa/peer-review-prompt-roundN.md`.

**Step 3: Execute the Cross-LLM Review**

1. Capture the pre-run status snapshot:

   ```bash
   git status --short > .compozy/tasks/<slug>/qa/peer-review-status-before-roundN.txt
   ```

2. Run (substitute the resolved `--ide`/`--model`/`--reasoning`, defaults `codex`/`gpt-5.6-sol`/`xhigh`):

   ```bash
   compozy exec --ide <ide> --model <model> --reasoning-effort <reasoning> --format json --prompt-file .compozy/tasks/<slug>/qa/peer-review-prompt-roundN.md > .compozy/tasks/<slug>/qa/peer-review-events-roundN.jsonl 2> .compozy/tasks/<slug>/qa/peer-review-result-roundN.err
   ```

3. Capture the post-run status snapshot:

   ```bash
   git status --short > .compozy/tasks/<slug>/qa/peer-review-status-after-roundN.txt
   ```

4. If the command returns a non-zero exit code, fail loudly. Do not retry silently. Inspect stderr for model misconfiguration (see Error Handling).
5. Treat `peer-review-events-roundN.jsonl` as operational evidence only. Do not parse it for readiness or findings.
6. Require the findings target file to exist after the command exits. If missing, the round is invalid even when `compozy exec` exited 0.

**Step 4: Validate and Summarize Findings**

1. Run the bundled read-only validator:

   ```bash
   bash .agents/skills/cy-spec-peer-review/scripts/validate-findings.sh --kind techspec --round N --path .compozy/tasks/<slug>/qa/peer-review-findings-roundN.md
   ```

2. Manually inspect the findings file and verify the semantic contract:
   - every finding has a real section/path reference;
   - blockers include a rationale tied to project rules, lessons, or architecture constraints;
   - no `TBD`, placeholder text, invented paths, or stdout-only findings;
   - comparing the pre/post status snapshots shows no changes outside the expected review artifact/log paths.
3. If validation fails, write `.compozy/tasks/<slug>/qa/peer-review-validation-error-roundN.md` with the failed checks, command, exit status, and artifact paths. Do not summarize the round as `READY`.
4. Write `.compozy/tasks/<slug>/qa/peer-review-summary-roundN.md` from the validated findings file with:
   - readiness verdict (`READY` / `BLOCKED` / `NEEDS_REWORK`);
   - one-line rationale per blocker;
   - nits list;
   - recommended sections and ADRs likely affected;
   - operational artifact paths.
5. Present a concise user-facing summary of the review. Include the verdict, blocker/nit counts, the main themes, and the artifact paths written for the round.
6. Do NOT modify the TechSpec or ADRs yet.

**Step 5: User-Directed Incorporation**

1. Ask the user which findings to incorporate:
   - A) all blockers
   - B) selected blockers/nits
   - C) nothing
   - D) manual edits before any incorporation
2. Apply only the findings the user selected. Do not silently apply all blockers or all nits.
3. If incorporation requires an ADR update, update only the ADRs tied to the selected findings.
4. Record the incorporation decision in `.compozy/tasks/<slug>/qa/peer-review-incorporation-roundN.md`, listing:
   - incorporated items;
   - deferred items;
   - files changed.
5. Show the user what changed and what remains deferred.

**Step 6: Optional Additional Rounds**

1. Ask whether the user wants another peer-review round or wants to stop with the current saved spec.
2. If the user requests another round, re-run from Step 2 against the updated TechSpec and create a fresh `roundN+1` artifact set.
3. Do not auto-loop. The user explicitly requests further rounds.

## Critical Rules

- This skill never commits, pushes, opens PRs, auto-approves specs, or invokes provider review fetchers.
- Prompt, event log, findings, summary, incorporation, and status snapshot artifacts are versioned with `-roundN`. Never overwrite a prior round.
- The `compozy exec` call is the only place this skill spends external review credit. Do not invoke it more than once per round unless the round is explicitly invalid and the user requests a rerun.
- The bundled helper paths used by this skill (`references/peer-review-prompt.md`, `references/quality-markers.md`, `scripts/validate-findings.sh`) are read-only templates/helpers — the skill reads or runs them, never edits them during a review round.

## Error Handling

- **Model misconfiguration (`The model 'X' does not exist`):** stop and surface the configured model. The IDE may be set to a stale name like `gpt-5.5`. Do not mutate the call to substitute a model — verify with the user. (See `docs/_memory/lessons/L-010-model-name-validation.md`.)
- **`compozy exec` not found:** the skill assumes Compozy CLI is on `PATH`. If absent, fail with the install hint rather than swallowing.
- **Quality markers missing:** if the Step 1 quality-marker check fails, do not run the reviewer. Print the missing markers and exit so the user can amend the spec first.
- **`--ide` invalid:** list accepted values (`codex`, `claude`, `cursor-agent`, `droid`, `opencode`, `pi`, `gemini`, `copilot`) and ask the user to choose — do not fall back.
- **Missing findings file:** treat this as an invalid round, not a clean review. Write a validation-error artifact and ask whether to rerun.
- **Malformed findings frontmatter or missing required sections:** treat this as an invalid round. Do not infer readiness from stdout.
- **Empty or placeholder findings:** treat empty `# Blockers` or `# Nits` sections as acceptable only when the section explicitly says `None.`; reject `TBD`, `TODO`, or vague placeholders.
- **Existing peer-review files:** never overwrite. Prompt, event log, findings, summary, and incorporation files are all versioned with `-roundN`.

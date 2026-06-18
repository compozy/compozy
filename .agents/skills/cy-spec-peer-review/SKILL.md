---
name: cy-spec-peer-review
description: Runs an optional cross-LLM peer review of a TechSpec via compozy exec --ide claude --model opus --reasoning-effort xhigh and requires the reviewer to write a scoped Markdown findings artifact for user-directed incorporation. Use when a TechSpec draft has already been approved by the user and they want an external review round, especially for complex or high-risk technical designs. Do not use for PRDs, automatic approval gates, code review batches, or auto-looped review cycles.
trigger: explicit
argument-hint: "[--spec path] [--context path1,path2] [--out dir]"
---

# Spec Peer Review

The primary LLM authors TechSpecs; Claude Opus pressure-tests them via `compozy exec`. This skill runs that pressure-test only when the user explicitly asks for a review round after approving the current draft. It is decoupled from any task-tracking metadata — the scope is the spec file plus optional context the user names. The skill never auto-runs, auto-incorporates findings, auto-commits, or auto-loops additional rounds.

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

## Optional Inputs

All inputs are optional. Defaults make the common path `cy-spec-peer-review --spec .compozy/tasks/<slug>/_techspec.md`.

- `--spec <path>` — explicit path to the TechSpec under review. When omitted, resolve to the most recently modified `.compozy/tasks/<slug>/_techspec.md`, excluding paths under `.compozy/tasks/_archived/`. If multiple candidates tie or none exist, list candidates and ask the user to choose.
- `--context <path1,path2,...>` — additional context files to feed Opus (e.g., a PRD, RFC, README). The skill never assumes any of these exist.
- `--out <dir>` — output directory for round artifacts. When omitted, apply the hybrid default (see Step 1).

## Findings Artifact Contract

Each review round has exactly one authoritative findings file:

```
<out>/spec-review-findings-roundN.md
```

The reviewer may write exactly that file and no other file. If the target path is missing, ambiguous, unwritable, or outside the resolved `<out>` directory, the reviewer must refuse and stop. It must not print findings to stdout as a fallback.

The findings file MUST use this structure:

```markdown
---
schema_version: 1
review_kind: techspec
round: N
readiness: READY|BLOCKED|NEEDS_REWORK
reviewer_runtime: claude
reviewer_model: opus
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

1. Resolve `--spec`:
   - If provided, verify the path exists and is readable.
   - If omitted, find the most recently modified `_techspec.md` under `.compozy/tasks/`, excluding `_archived/`. If ambiguous, ask the user.
2. Confirm the user has already approved the current draft or explicitly asked to review the saved spec as-is.
3. Read the spec and confirm it is a final-shape TechSpec aligned with `.agents/skills/cy-create-techspec/references/techspec-template.md`. At minimum, verify these sections exist (or an equivalent subsection with the same intent):
   - `Executive Summary`
   - `System Architecture` or `Component Overview`
   - `Implementation Design` or `Core Interfaces`
   - `Testing Approach`
   - `Development Sequencing` or `Build Order`
   Specs that omit applicable sections must state why (per the template). If required sections are missing without justification, abort.
4. Read `.agents/skills/cy-spec-peer-review/references/quality-markers.md` and verify the spec carries all six markers (scope boundary, component boundaries, concrete interface signatures, data model rationale, testing strategy, build order and dependencies). If any marker is missing, abort and report the missing markers — Opus review is wasted on incomplete specs.
5. Resolve the artifact directory `<out>`:
   - Use `--out` if provided.
   - Otherwise, if the spec path matches `.compozy/tasks/<slug>/_techspec.md`, default to `.compozy/tasks/<slug>/qa/`.
   - Otherwise, default to `.peer-reviews/<UTC-timestamp-YYYYMMDDTHHMMSSZ>/` at the repository root.
   - Create the directory if it does not exist.
6. Determine the next review round number by listing existing `spec-review-findings-round*.md`, `spec-review-summary-round*.md`, and legacy `peer-review-*-round*` files in `<out>`. Start at `round1` when none exist.
7. Auto-discover sibling context (do not require these to exist):
   - ADRs: `dirname(spec)/adrs/*.md`
   - PRD: `dirname(spec)/_prd.md` — include in context paths when present
   - User `--context` paths — append when provided

**Step 2: Compose the Review Prompt**

1. Read `.agents/skills/cy-spec-peer-review/references/peer-review-prompt.md` for the canonical executable Opus prompt template. The assembled prompt must start with the reviewer instructions, not with a Markdown wrapper describing the template.
2. Define the round artifact paths:
   - Findings target: `<out>/spec-review-findings-roundN.md`.
   - Operational event log: `<out>/spec-review-events-roundN.jsonl`.
   - Operational stderr log: `<out>/spec-review-result-roundN.err`.
   - Pre-run status snapshot: `<out>/spec-review-status-before-roundN.txt`.
   - Post-run status snapshot: `<out>/spec-review-status-after-roundN.txt`.
   - Validation error, only when needed: `<out>/spec-review-validation-error-roundN.md`.
3. Substitute the placeholders:
   - `{techspec_path}` — exact repo-root path to the TechSpec under review.
   - `{adr_paths}` — newline-separated repo-root paths to sibling ADRs, or `none`.
   - `{context_paths}` — newline-separated repo-root paths from auto-discovered PRD, `--context`, and any sibling analysis docs, or `none`.
   - `{findings_path}` — exact repo-root path to `<out>/spec-review-findings-roundN.md`.
   - `{round}` — numeric review round `N`.
4. Write the assembled prompt to `<out>/spec-review-prompt-roundN.md`.

**Step 3: Execute the Cross-LLM Review**

1. Capture the pre-run status snapshot:

   ```bash
   git status --short > <out>/spec-review-status-before-roundN.txt
   ```

2. Run:

   ```bash
   compozy exec --ide claude --model opus --reasoning-effort xhigh --format json --prompt-file <out>/spec-review-prompt-roundN.md > <out>/spec-review-events-roundN.jsonl 2> <out>/spec-review-result-roundN.err
   ```

3. Capture the post-run status snapshot:

   ```bash
   git status --short > <out>/spec-review-status-after-roundN.txt
   ```

4. If the command returns a non-zero exit code, fail loudly. Do not retry silently. Inspect stderr for model misconfiguration (see Error Handling).
5. Treat `spec-review-events-roundN.jsonl` as operational evidence only. Do not parse it for readiness or findings.
6. Require the findings target file to exist after the command exits. If missing, the round is invalid even when `compozy exec` exited 0.

**Step 4: Validate and Summarize Findings**

1. Run the bundled read-only validator:

   ```bash
   bash .agents/skills/cy-spec-peer-review/scripts/validate-findings.sh --kind techspec --round N --path <out>/spec-review-findings-roundN.md
   ```

2. Manually inspect the findings file and verify the semantic contract:
   - every finding has a real section/path reference;
   - blockers include a rationale tied to project rules or architecture constraints when applicable;
   - no `TBD`, placeholder text, invented paths, or stdout-only findings;
   - comparing the pre/post status snapshots shows no changes outside the expected review artifact/log paths under `<out>`.
3. If validation fails, write `<out>/spec-review-validation-error-roundN.md` with the failed checks, command, exit status, and artifact paths. Do not summarize the round as `READY`.
4. Write `<out>/spec-review-summary-roundN.md` from the validated findings file with:
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
   - C) nothing — keep the review as a record only
   - D) manual edits before any incorporation
2. Apply only the findings the user selected. Do not silently apply all blockers or all nits.
3. If incorporation requires an ADR update, update only the ADRs tied to the selected findings.
4. Record the incorporation decision in `<out>/spec-review-incorporation-roundN.md`, listing:
   - incorporated items;
   - deferred items;
   - files changed.
5. Show the user what changed and what remains deferred. Do not commit or push without an explicit user instruction.

**Step 6: Optional Additional Rounds**

1. Ask whether the user wants another peer-review round or wants to stop with the current saved spec.
2. If the user requests another round, re-run from Step 2 against the updated TechSpec and create a fresh `roundN+1` artifact set in the same `<out>` directory.
3. Do not auto-loop. The user explicitly requests further rounds.

## Critical Rules

- This skill never commits, pushes, opens PRs, auto-approves specs, or invokes provider review fetchers.
- Prompt, event log, findings, summary, incorporation, and status snapshot artifacts are versioned with `-roundN` under `<out>`. Never overwrite a prior round.
- The `compozy exec` call is the only place this skill spends external review credit. Do not invoke it more than once per round unless the round is explicitly invalid and the user requests a rerun.
- The bundled helper paths used by this skill (`references/peer-review-prompt.md`, `references/quality-markers.md`, `scripts/validate-findings.sh`) are read-only templates/helpers — the skill reads or runs them, never edits them during a review round.

## Error Handling

- **Model misconfiguration (`The model 'X' does not exist`):** stop and surface the configured model. The IDE may be set to a stale model name. Do not mutate the call to substitute a model — verify with the user.
- **`compozy exec` not found:** the skill assumes Compozy CLI is on `PATH`. If absent, fail with the install hint rather than swallowing.
- **Quality markers missing:** if the Step 1 quality-marker check fails, do not run Opus. Print the missing markers and exit so the user can amend the spec first.
- **Missing findings file:** treat this as an invalid round, not a clean review. Write a validation-error artifact and ask whether to rerun.
- **Malformed findings frontmatter or missing required sections:** treat this as an invalid round. Do not infer readiness from stdout.
- **Empty or placeholder findings:** treat empty `# Blockers` or `# Nits` sections as acceptable only when the section explicitly says `None.`; reject `TBD`, `TODO`, or vague placeholders.
- **Existing peer-review files:** never overwrite. Prompt, event log, findings, summary, and incorporation files are all versioned with `-roundN`.

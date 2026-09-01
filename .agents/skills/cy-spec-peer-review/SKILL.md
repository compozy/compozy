---
name: cy-spec-peer-review
description: Cross-LLM peer review of a Spec's technical design by a persistent reviewer session — scoped Markdown findings for user-directed incorporation; follow-up rounds continue the same reviewer instead of spawning a new one. Use when a spec has already been approved and the user wants an external review round, especially for autonomy/network/memory-impacting designs. Do not use for Stage 1 product drafts, automatic approval gates, code review batches, or auto-looped review cycles.
trigger: explicit
argument-hint: "[spec-path] [--reviewer] [--model] [--reasoning]"
---

# Spec Peer Review

An independent **reviewer session** pressure-tests an approved Spec (Part II focus). One review program = one reviewer session: round 1 spawns it and reads the full corpus; every later round is a continuation prompt to that same live session, so the reviewer re-reads only what changed. This skill runs that pressure-test only when the user explicitly asks for a review round after approving the current draft. It does not auto-run, auto-incorporate findings, or auto-loop additional rounds.

The review result is a direct-written Markdown findings file. Reviewer terminal/stream output is operational evidence only; never parse it as the review source of truth.

## User Decisions

When this skill instructs the agent to ask whether to incorporate findings or run another round, it MUST use the runtime's dedicated interactive question tool — the tool or function that presents a question to the user and pauses execution until the user responds.

If the runtime does not provide such a tool, present the question as the complete assistant message and stop generating. Do not answer the question on the user's behalf.

## Bundled Path Rule

Resolve bundled helper paths relative to the directory that contains this `SKILL.md`. When invoking the validator from a repository root, use the full repo-relative path:

```bash
bash .agents/skills/cy-spec-peer-review/scripts/validate-findings.sh --kind spec --round <N> --path <findings-path>
```

The validator is a read-only helper: it inspects the findings artifact and exits non-zero on structural contract violations.

## Required Inputs

- **spec-path** (optional): explicit path to the `_spec.md` under review. When omitted, resolve to the most recently modified `.compozy/tasks/<slug>/_spec.md` whose sibling `_meta.md` shows `Pending: > 0` or no `_meta.md` exists yet.
- **Reviewer selection** (default: Codex on `gpt-5.6-sol`, reasoning `xhigh`): how the reviewer session runs depends on the dispatch substrate —
  - **CompozyOS session** (canonical): a reviewer agent definition carries provider/model/reasoning; pass it via `--reviewer <agent-name>`.
  - **herdr worker TUI** (when the operator orchestrates through herdr): model flags ride the worker launch, e.g. `--kind codex -- --yolo -m gpt-5.6-sol -c model_reasoning_effort=xhigh`.
  - `--model` / `--reasoning` override the defaults on either substrate. Never substitute a different model than the user requested.

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
review_kind: spec
round: N
readiness: READY|BLOCKED|NEEDS_REWORK
reviewer_runtime: <reviewer runtime, e.g. codex>
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

1. Resolve `spec-path`. If omitted, list candidate paths and pick the freshest.
2. Confirm the user has already approved the current draft or explicitly asked to review the saved spec as-is.
3. Read the spec and confirm it is final-shape (complete Part I and Part II, with `Architectural Boundaries`, `Development Sequencing`, and `Testing Approach` sections) — not a draft.
4. Read `references/quality-markers.md` and verify the spec carries the six markers (boundary statement, listed boundaries, Go interface signatures, data-model field rationale, side-table-vs-JSON decisions, lease/safety invariants enumerated). If any marker is missing, abort and report the missing markers — external review is wasted on incomplete specs.
5. Resolve the slug from the path; ensure `.compozy/tasks/<slug>/` exists and is writable.
6. Ensure `.compozy/tasks/<slug>/qa/` exists before dispatch.
7. Determine the next review round number by listing existing `qa/peer-review-findings-round*.md`, `qa/peer-review-summary-round*.md`, and legacy `qa/peer-review-result-round*.json*` files (prior local output only — not a compatibility path). Start at `round1` when none exist.

**Step 2: Compose the Review Prompt**

1. Read `references/peer-review-prompt.md` for the canonical executable reviewer prompt template. The assembled prompt must start with the reviewer instructions, not with a Markdown wrapper describing the template.
2. Define the round artifact paths:
   - Findings target: `.compozy/tasks/<slug>/qa/peer-review-findings-roundN.md`.
   - Operational evidence log (when the dispatch substrate produces one): `.compozy/tasks/<slug>/qa/peer-review-log-roundN.txt`.
   - Pre-run status snapshot: `.compozy/tasks/<slug>/qa/peer-review-status-before-roundN.txt`.
   - Post-run status snapshot: `.compozy/tasks/<slug>/qa/peer-review-status-after-roundN.txt`.
   - Validation error, only when needed: `.compozy/tasks/<slug>/qa/peer-review-validation-error-roundN.md`.
3. Substitute the placeholders:
   - `{spec_path}` — exact path to the `_spec.md` under review.
   - `{surface_paths}` — any `_dx.md`/`_uiux.md` siblings, or `none`.
   - `{adr_paths}` — any `adrs/*.md` siblings, or `none`.
   - `{related_research}` — any `analysis/*.md` siblings, or `none`.
   - `{findings_path}` — exact absolute path to `.compozy/tasks/<slug>/qa/peer-review-findings-roundN.md`.
   - `{round}` — numeric review round `N`.
   - `{reviewer_runtime}` — the reviewer runtime (default `codex`).
   - `{reviewer_model}` — resolved `--model` (default `gpt-5.6-sol`).
4. Write the assembled prompt to `.compozy/tasks/<slug>/qa/peer-review-prompt-roundN.md`.
5. Round 1 uses the full template. **Round N+1 composes a continuation prompt instead** — the reviewer session already holds the corpus. It contains only: the exact list of changed files (with a one-line summary of what changed in each), the incorporation record path, the instruction to verify each prior finding is genuinely resolved and to sweep the new text for fresh regressions, and the new findings target with frontmatter `round: N+1` (new finding IDs continue the sequence). Same scoped-write contract and findings format; never resend the full corpus.

**Step 3: Execute the Review Round in the Reviewer Session**

1. Capture the pre-run status snapshot:

   ```bash
   git status --short > .compozy/tasks/<slug>/qa/peer-review-status-before-roundN.txt
   ```

2. **Round 1 — spawn the reviewer session** on the chosen substrate and deliver the full prompt:
   - **CompozyOS session (canonical)**:

     ```bash
     compozy session new --cwd "$PWD" --agent <reviewer-agent> --name spec-review-<slug>   # capture the session id
     compozy session prompt <session-id> "$(cat .compozy/tasks/<slug>/qa/peer-review-prompt-round1.md)"
     compozy session wait <session-id> --until idle --timeout 1800s
     ```

   - **herdr worker TUI** (operator-orchestrated): create a labeled tab, `herdr agent start <name> --kind codex --pane <pane> -- --yolo -m <model> -c model_reasoning_effort=<reasoning>`, then `herdr agent prompt <name> "$(cat …/peer-review-prompt-round1.md)"` and wait on `done` in check-in intervals.

   **Round N+1 — continue the same reviewer session**: deliver the Step 2.5 continuation prompt (`session prompt <session-id> …` / `herdr agent prompt <name> …`).

3. Capture the post-run status snapshot:

   ```bash
   git status --short > .compozy/tasks/<slug>/qa/peer-review-status-after-roundN.txt
   ```

4. If the dispatch fails (session refuses the prompt, worker rejects launch flags, wait surfaces an error), fail loudly. Do not retry silently. Inspect the error for model/agent misconfiguration (see Error Handling).
5. Treat any captured session/worker output log as operational evidence only. Do not parse it for readiness or findings.
6. Require the findings target file to exist after the round settles. If missing, the round is invalid even when the session settled cleanly.

**Step 4: Validate and Summarize Findings**

1. Run the bundled read-only validator:

   ```bash
   bash .agents/skills/cy-spec-peer-review/scripts/validate-findings.sh --kind spec --round N --path .compozy/tasks/<slug>/qa/peer-review-findings-roundN.md
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
6. Do NOT modify the spec or ADRs yet.

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
2. If the user requests another round, run Step 2.5 and Step 3 (round N+1) with a fresh `roundN+1` artifact set.
3. If the reviewer session is gone or invalid, follow Error Handling → *Reviewer session lost between rounds*.
4. Do not auto-loop. The user explicitly requests further rounds.
5. When the review program ends (user stops or the final round is incorporated), retire the reviewer session (`compozy session stop <session-id>` / close the herdr worker tab) after recording anything the registry needs.

## Critical Rules

- This skill never commits, pushes, opens PRs, auto-approves specs, or invokes provider review fetchers.
- Prompt, event log, findings, summary, incorporation, and status snapshot artifacts are versioned with `-roundN`. Never overwrite a prior round.
- The reviewer dispatch is the only place this skill spends external review credit — one prompt per round unless the round is explicitly invalid and the user requests a rerun.
- The bundled helper paths used by this skill (`references/peer-review-prompt.md`, `references/quality-markers.md`, `scripts/validate-findings.sh`) are read-only templates/helpers — the skill reads or runs them, never edits them during a review round.

## Error Handling

- **Model misconfiguration (`The model 'X' does not exist`):** stop and surface the configured model. The reviewer agent definition or worker flags may carry a stale name like `gpt-5.5`. Do not mutate the call to substitute a model — verify with the user. (See `docs/_memory/lessons/L-010-model-name-validation.md`.)
- **Dispatch substrate unavailable** (daemon not running for `compozy session`, herdr absent for a worker): fail with the start hint (`compozy daemon start` / `herdr status`) rather than swallowing.
- **Reviewer session lost between rounds** (stopped, daemon restarted, worker retired, corrupted state): note the loss in the round summary, spawn a fresh reviewer with the full round-1 template, and continue the round numbering.
- **Quality markers missing:** if the Step 1 quality-marker check fails, do not run the reviewer. Print the missing markers and exit so the user can amend the spec first.
- **Reviewer selection invalid** (unknown agent definition or worker kind): list what exists (`compozy agent list` / herdr-supported kinds) and ask the user to choose — do not fall back.
- **Missing findings file:** treat this as an invalid round, not a clean review. Write a validation-error artifact and ask whether to rerun.
- **Malformed findings frontmatter or missing required sections:** treat this as an invalid round. Do not infer readiness from stdout.
- **Empty or placeholder findings:** treat empty `# Blockers` or `# Nits` sections as acceptable only when the section explicitly says `None.`; reject `TBD`, `TODO`, or vague placeholders.
- **Existing peer-review files:** never overwrite. Prompt, event log, findings, summary, and incorporation files are all versioned with `-roundN`.

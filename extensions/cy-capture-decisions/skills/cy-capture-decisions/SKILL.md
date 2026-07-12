---
name: cy-capture-decisions
description: Reconciles a finished workflow's planned decisions (its Accepted ADRs) against the settled reality (git diff, review issues, and task status), then promotes the proven, cross-feature-durable ones into a durable project decision log at .compozy/DECISIONS.md plus .compozy/decisions/AD-NNN.md. Use when a workflow has finished its full pipeline (review round, reviews fix, and final verify) and you want to capture its durable decisions as the final step, or when re-running capture to refresh the log after further changes. Do not use for capturing decisions mid-implementation before review remediation, for PRD or TechSpec authoring, for PR review remediation, or for generic note-taking.
argument-hint: [slug]
---

# Capture Decisions

Reconcile a finished workflow's plan (its `Accepted` ADRs) against the settled reality (the code
diff, review issues, and task status), then promote the proven, cross-feature-durable decisions into
a durable, project-scoped decision log. Run this **manually, as the final step of the pipeline** —
after `/cy-review-round`, `compozy reviews fix`, and `/cy-final-verify` — so capture sees the
remediated, verified state rather than a mid-flight guess.

The log is two tiers: a terse index (`.compozy/DECISIONS.md`) that is `@import`ed into agent memory,
and rich per-decision bodies (`.compozy/decisions/AD-NNN.md`) read on demand. Both live at the
**workspace root**, not under `.compozy/tasks/<slug>/`, so they survive `compozy archive`.

The `<slug>` argument names the **source workflow** to reconcile from. The skill is **idempotent**:
re-running on an unchanged workflow is a no-op.

## Required Inputs

- `<slug>`: the workflow slug identifying the source `.compozy/tasks/<slug>/` directory to reconcile.
- Optional: nothing else. The skill discovers every input from the workspace.

Read the three references before writing anything:

- `references/reconciliation-guide.md` — the relevance gate, evidence rules, classification, provenance,
  supersession, degraded mode, and fresh-eyes discipline. This is the procedural core.
- `references/decision-record-template.md` — the exact `AD-NNN.md` frontmatter schema, body sections,
  `Reconciliation` section, and filename pattern.
- `references/index-format.md` — the exact `DECISIONS.md` line grammar and membership rule.

## Example

Invoking `/cy-capture-decisions feat-orders` on a finished, verified workflow whose plan had one
cross-feature-durable ADR (`adrs/adr-002.md`, event-sourcing) and two feature-local ADRs writes
`.compozy/decisions/AD-001.md` and adds one line to `.compozy/DECISIONS.md`:

```
AD-001 | Event-sourcing for orders | proven | [orders, async] | audit + replay | feat-orders
```

and prints a run summary:

```
Captured from feat-orders:
  PROMOTED  AD-001  NEW  Event-sourcing for orders (proven; evidence: verify p99<200ms; diff abc123)
  SKIPPED   adr-001      feature-local table naming (obvious from schema)
  SKIPPED   adr-003      pagination default (not cross-feature-durable)
```

Re-running the same command with no further changes prints `no changes` and writes nothing.

## Workflow

1. Resolve inputs and confirm the final-step position.
   - Reject a missing or malformed `<slug>` argument. Do not guess a target.
   - Derive the source directory `.compozy/tasks/<slug>/`. If it does not exist, report
     "slug not found"; if only an archived variant exists, report "already archived" and write nothing.
   - Confirm this is being run as the final pipeline step (after `/cy-review-round`,
     `compozy reviews fix`, `/cy-final-verify`). If review artifacts are absent, note it and continue —
     capture still runs on diff + verify signal (see step 3 and `references/reconciliation-guide.md`).
   - Resolve the durable log paths at the workspace root: `.compozy/DECISIONS.md` (index) and
     `.compozy/decisions/` (bodies). Never write the log under `.compozy/tasks/<slug>/`.

2. Read the existing log.
   - Read `.compozy/DECISIONS.md` if it exists and list `.compozy/decisions/AD-*.md`.
   - Build an in-memory map of already-promoted decisions keyed by `source_slug` + `source_adr`
     provenance (the idempotent re-match key), plus their `title`/`status` for the semantic fallback.
   - If the log does not exist yet, treat it as empty; the first NEW decision becomes `AD-001`.

3. Gather reconciliation evidence, in priority order.
   - **Ground truth (primary):** the code diff for the slug — `git diff main...HEAD` (the scoping
     convention from `cy-review-round`) — plus fresh-eyes review issues
     (`.compozy/tasks/<slug>/reviews-NNN/issue_*.md`) and the verify proxy (task files'
     `status: completed`, set only after a clean `/cy-final-verify`).
   - **Plan:** the workflow's `Accepted` ADRs (`.compozy/tasks/<slug>/adrs/adr-*.md`). These are the
     candidate decisions to reconcile.
   - **Hint (secondary, unverified self-report):** `.compozy/tasks/<slug>/memory/MEMORY.md` — used only
     to locate candidate decisions, never as proof.
   - If the diff cannot be scoped (unknown base, broken range, detached history), switch to **degraded
     mode**: fall back to memory + review issues, and mark every entry "unverified against code" so it
     is written as `candidate`, not `proven` (see `references/reconciliation-guide.md`). Do not fabricate
     evidence; if neither diff nor reviews are available, promote nothing.

4. Apply the relevance gate to each plan ADR.
   - Read the gate in `references/reconciliation-guide.md`. Promote an ADR only if it passes all three:
     cross-feature-durable AND non-obvious AND future-relevant. When in doubt, do **not** promote — a
     missed decision can be captured on a later re-run; a wrongly-promoted one is permanent noise.
   - Record each skipped ADR with its gate reason for the run summary. Skipped ADRs stay workflow-local.

5. Classify each surviving decision against the existing log.
   - Match by `source_adr` provenance first (exact), then LLM semantic fallback (robust to ADR
     renumbering). Assign one classification (see `references/reconciliation-guide.md`):
     - **NEW** — no match in the log → assign the next `AD-NNN` (step 7), write body + index line.
     - **UPDATE** — already promoted (provenance or strong semantic match) → amend the existing body in
       place (status, new evidence); **no new number**.
     - **SUPERSEDE** — conflicts with / reverses an active decision from another slug → create a new
       record and mark the old one superseded (step 6).
   - A weak-only semantic match with no `source_adr` match is **NEW** with a low-confidence note, never
     an incorrect UPDATE.
   - If provenance matches and nothing changed, this decision is a **no-op** (idempotency).

6. Reconcile each decision against reality and write its record.
   - Read `references/decision-record-template.md` for the exact frontmatter schema and body layout.
   - Set `status: proven` only when a cited diff/review/verify evidence backs the decision; otherwise
     `status: candidate` with the missing-evidence reason. Populate `evidence` with the concrete
     citations (verify result, diff ref, resolved `issue_NNN`).
   - Preserve the original ADR body sections (Context / Decision / Alternatives / Consequences) and add
     a `## Reconciliation` section describing what execution proved vs. planned. When the shipped result
     diverged from the plan, mark it with a `[DEVIATION]` marker and cite the evidence; when it matched,
     state "implemented as designed" with no `[DEVIATION]`.
   - **SUPERSEDE handling** — never rewrite an accepted decision in place to reverse it. Instead: set the
     old record's `superseded_by: AD-NNN` (it drops from the index but keeps its file), and set the new
     record's `supersedes: [AD-OOO]`. Keep the links bidirectional. In a chain A→B→C, only the tail (C)
     is active.
   - Tag every promoted record with `tags: [area, domain]` in frontmatter.

7. Assign numbers deterministically (shell, not LLM).
   - Process NEW/SUPERSEDE records **one at a time**: compute the id, write its body, then recompute for
     the next record. The derivation below reads on-disk state, so an id is only reserved once its file
     exists — never batch-assign ids before writing (two records computed against the same directory
     snapshot resolve to the same `AD-NNN` and collide).
   - For each NEW/SUPERSEDE record, compute the next id from the **maximum existing suffix** — not the file
     count, which reuses an id whenever the numbering has a gap (`AD-001`, `AD-003`, `AD-004` counts 3 and
     would reselect the existing `AD-004`, overwriting a real decision body). A gap can arise from a manual
     repair, an import, or an interrupted prior capture (US-006.EC-2). Derive from the max instead:

     ```bash
     max=$(ls .compozy/decisions/AD-*.md 2>/dev/null | sed -E 's#.*/AD-0*([0-9]+)\.md#\1#' \
       | sort -n | tail -1); printf 'AD-%03d\n' "$((${max:-0} + 1))"
     ```

   - Deriving from the max keeps ids unique even across a gap — the example above yields `AD-005`, never a
     reused `AD-004`, honoring the "unique across the whole log. Never reused" contract in
     `references/decision-record-template.md`. Because each body is written before the next id is computed,
     two NEW decisions in one run still take consecutive ids with no collision. UPDATE records keep their
     existing id.
   - Write each body to `.compozy/decisions/AD-NNN.md` (zero-padded 3 digits). Create `.compozy/decisions/`
     if missing.

8. Update the terse index.
   - Read `references/index-format.md` for the exact grammar. Write one line per **active, proven**
     decision to `.compozy/DECISIONS.md`:

     ```
     AD-NNN | Title | status | [tag, tag] | one-line rationale | source_slug
     ```

   - Exclude every `candidate` and every `superseded` record — they exist in their `AD-NNN.md` files but
     never appear in the index. On an empty result, write the documented empty-state index, not an
     empty file with a dangling reference.

9. Print the run summary.
   - List: decisions promoted (each with NEW / UPDATE / SUPERSEDE and its `AD-NNN`), decisions skipped by
     the relevance gate (with reason), and any written as `candidate` (with the missing-evidence reason).
   - On an unchanged re-run, state "no changes" explicitly.

10. Verify before completion.
    - Use the installed `cy-final-verify` skill before claiming capture is complete.
    - Re-read the written bodies and index; confirm frontmatter parses, `status` is a valid enum value,
      the index contains only active-proven lines, and every supersession link is bidirectional.
    - Confirm a second run on the now-unchanged workflow would be a no-op (provenance match).

## Critical Rules

- **Never rewrite an accepted decision in place to change its meaning — supersede it.** Amending in place
  is only for UPDATE (same decision, new status/evidence). Reversal or conflict is SUPERSEDE.
- **The index carries active-`proven` records only.** `candidate` and `superseded` records live in their
  files but are excluded from `DECISIONS.md` (ADR-003).
- **Filenames and index lines must match the grammar** in `references/decision-record-template.md` and
  `references/index-format.md` exactly — a validator (task_02) parses them.
- **Write the log at the workspace root** (`.compozy/DECISIONS.md`, `.compozy/decisions/AD-NNN.md`), never
  under `.compozy/tasks/<slug>/` (ADR-002).
- **Cite evidence for every `proven` record.** A decision without cited diff/review/verify evidence is
  `candidate`, not `proven`.
- **Be idempotent.** A provenance match with no change is a no-op; do not re-add or renumber.
- **Do not fabricate evidence or promote everything.** Prefer not promoting when the relevance gate is
  unsure (ADR-008).
- **Do not silently edit the user's `.gitignore`** or `CLAUDE.md` / `AGENTS.md`. Consumption wiring and
  gitignore negations are documented (task_03 README), not applied by this skill.

## Error Handling

- **Missing/malformed `<slug>`** → reject and stop; do not guess a target or write anything.
- **Slug directory not found / already archived** → report the specific condition; write nothing.
- **No `Accepted` ADRs, or all are feature-local** → no-op: zero promotions with reasons, and do not
  create empty log files (on a fresh project still emit the documented empty-state index only if the log
  is being created).
- **One source ADR malformed/unreadable** → skip that decision with a warning and continue processing the
  others.
- **Unscopable diff** → degrade to memory + reviews, mark entries "unverified against code" as
  `candidate` (never overclaim `proven`). A broken commit range is reported, not crashed.
- **Unwritable `.compozy/`** → fail the write step without partial or truncated files; leave any
  pre-existing log unchanged.
- **Interrupted mid-write / a second run** → re-running reconciles via provenance to a consistent log
  (no duplicate or half-written `AD`). Capture is a single, final, serial step; concurrent runs on one
  workspace are unsupported.
- **Pre-existing local edit to the log** → surface it as a normal VCS diff/merge; never silently
  overwrite a user's manual correction — reconcile to the corrected state via provenance.

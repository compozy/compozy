# Reconciliation Guide

This is the procedural core of capture: how to decide what to promote, how to ground it in evidence, how
to classify it against the existing log, and how to degrade gracefully when reality is unscopable. Read
it before writing any record.

The governing idea (planning ADR-005): capture **reconciles plan vs. reality**. It reads the plan-time
ADRs _and_ evidence of what was actually built, then writes each promoted decision to reflect the
**proven outcome**, explicitly flagging deviations from the plan. It is not a verbatim copy of the ADRs.

## 1. Relevance Gate

Do not promote every ADR. Most ADRs are feature-local scaffolding. Promote an ADR **only if it passes
all three tests** (planning ADR-008):

1. **Cross-feature-durable** — will a _future, unrelated_ feature need this to avoid a mistake?
2. **Non-obvious** — is it NOT already obvious from reading the codebase or its conventions?
3. **Future-relevant** — does it still matter beyond the workflow that produced it?

- A "no" on any test → skip; the decision stays workflow-local.
- **When in doubt, do not promote.** A missed decision can be captured on a later re-run; a
  wrongly-promoted one is permanent noise in every future agent session.
- Record each skipped ADR with its gate reason in the run summary (e.g. "adr-001 skipped: feature-local
  table naming, obvious from schema").

Examples that pass: an event-sourcing choice with cross-cutting replay/audit implications; an idempotency
convention other features must honor. Examples that fail: a pagination default, a local file name, a
one-off migration ordering.

## 2. Evidence Rules

Ground "reality" in verification/review evidence + the diff, in **priority order** (planning ADR-006):

1. **Ground truth (primary):** the code **diff** for the slug (`git diff main...HEAD`), the fresh-eyes
   **review issues** (`.compozy/tasks/<slug>/reviews-NNN/issue_*.md`), and the **verify signal** (task
   files' `status: completed`, set only after a clean `/cy-final-verify`).
2. **Hint (secondary):** `memory/MEMORY.md`, explicitly treated as _unverified self-report_ — used to
   locate candidate decisions, never as proof.

Discipline:

- **Cite evidence.** Every promoted decision cites the test/verify/diff/issue that proves it, in the
  `evidence` frontmatter field. A decision without proof is written as `candidate`, not `proven`.
- **No fabrication.** Never invent an evidence citation. If nothing proves a decision, it is `candidate`
  (or, if it also fails the relevance gate, not promoted at all).
- **Post-remediation state only.** Because capture runs after `compozy reviews fix`, reconcile against
  the _remediated_ diff and the _resolved_ review issues — not the pre-fix state. A clean review (no
  issues) still proceeds on diff + verify and is noted as a clean review.

## 3. Classification and Provenance

Read the existing `DECISIONS.md` + `decisions/` first, then classify each gated-in decision (planning
ADR-007). One workflow ADR maps 1:1 to one project `AD`.

- **NEW** — no match in the log → assign the next `AD-NNN`; write body + index line.
- **UPDATE** — already promoted (matched by `source_adr` provenance, semantic fallback) → amend the body
  in place (new status, new evidence); **no new number**.
- **SUPERSEDE** — conflicts with / reverses an active decision from another slug → new record + the old
  record's `superseded_by` set, dropping it from the index (see §4).

**Match key:** provenance first (`source_slug` + `source_adr`, exact), LLM semantic fallback second
(robust to ADR renumbering). A weak-only semantic match with no provenance match is classified **NEW**
(with a low-confidence note), never an incorrect UPDATE — a false UPDATE would silently mutate an
unrelated decision.

**Idempotency:** re-running on an unchanged slug produces a provenance match with no change → **no-op**.
Do not renumber, duplicate, or rewrite unchanged records.

## 4. Supersession

- Never rewrite an accepted decision in place to reverse it. Supersede it: create the new record, set the
  new record's `supersedes: [AD-OOO]`, and set the old record's `superseded_by: AD-NNN`.
- Keep the links **bidirectional**. The superseded record keeps its file (history is preserved) but drops
  from the index.
- In a chain A→B→C, only the active tail (C) is `proven` and in the index; A and B are `superseded`.

## 5. Degraded Mode

When the diff cannot be scoped to the slug (unknown base branch, broken commit range, detached or
squashed history):

- Fall back to `memory/MEMORY.md` + review issues as the reconciliation basis.
- Mark every affected entry **"unverified against code"** in its `Reconciliation` section and write it as
  `candidate`, not `proven` — never overclaim.
- Report the scoping failure in the run summary; do not crash.
- If **neither** a scopable diff **nor** review issues are available, promote nothing — there is no
  honest basis for a `proven` or even an evidence-linked `candidate`.

## 6. Fresh Eyes

Reconcile as an independent reviewer, not the implementer. Trust the diff, the review issues, and the
verify signal over the workflow's own `memory/MEMORY.md` narrative. The memory file is a _hint_ about
what decisions were made, not evidence that they were made correctly. When memory claims an outcome the
diff does not show, the decision is `candidate` (unverified), and the discrepancy belongs in the
`Reconciliation` section.

## 7. Tagging

Every promoted `AD-NNN` carries `tags: [area, domain, ...]` in frontmatter (planning ADR-008): the
subsystem or concern the decision governs (e.g. `orders`, `async`, `api`, `auth`). Tags are the
forward-compat hook for scoped loading (a later phase) and make the terse index scannable. Keep them
lowercase, few, and specific.

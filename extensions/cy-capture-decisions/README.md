# cy-capture-decisions

A skill-only Compozy extension that captures a finished workflow's **durable** decisions into a
project-scoped decision log, so future features start informed by what past features already decided.

Run it as the **final step** of a workflow. It reconciles the plan (the workflow's `Accepted` ADRs)
against the settled reality (the code diff, review issues, and task status), then promotes only the
proven, cross-feature-durable decisions into a two-tier log at your workspace root.

## Quick start

From install to a log every future session reads automatically, in four steps. Each links to its
detailed section below. Steps 3–4 are one-time setup you apply by hand — the skill never edits your
`.gitignore` or memory files for you.

1. **Install** the extension and its skill — [details](#install):

   ```bash
   compozy ext install --yes compozy/compozy --remote github --ref <tag> --subdir extensions/cy-capture-decisions
   compozy ext enable cy-capture-decisions
   compozy setup
   ```

2. **Capture** as the workflow's final step, after a clean `/cy-final-verify` — [details](#usage):

   ```bash
   /cy-capture-decisions <slug>
   ```

3. **Keep the log committed** — only if your repo ignores `.compozy/**`; add the gitignore negations —
   [details](#make-the-decision-log-durable-gitignore).

4. **Wire the index into agent memory** so every session (planning included) reads it. Use `@import`
   for import-capable agents, or the documented `AGENTS.md` read instruction for Codex —
   [details](#wire-the-index-into-agent-memory):

   ```text
   @.compozy/DECISIONS.md
   ```

## What it ships

- `/cy-capture-decisions <slug>` — the capture skill (Markdown instructions + `references/`).
- **Nothing else.** This is a skill-only extension: no runtime process, no hooks, no core changes.
  It adds only the skill and its reference docs to your agents' skill directories.

## What it produces

Two tiers, both at the **workspace root** (a sibling of `.compozy/tasks/`, so they survive
`compozy archive`):

- `.compozy/DECISIONS.md` — a terse index, one line per active, `proven` decision. This is the file you
  `@import` into agent memory (see [Wire the index into agent memory](#wire-the-index-into-agent-memory)).
- `.compozy/decisions/AD-NNN.md` — one rich body per decision (the original ADR sections plus a
  `## Reconciliation` note), read on demand when a new decision touches that area.

The exact file grammar lives with the skill and is enforced by CI (see
[Decision-record schema](#decision-record-schema)).

## Install

```bash
compozy ext install --yes compozy/compozy --remote github --ref <tag> --subdir extensions/cy-capture-decisions  # fetch the bundle into the extensions dir
compozy ext enable cy-capture-decisions  # user/workspace bundles are disabled by default
compozy setup  # install the skill into your agents' skill dirs
```

`compozy setup` is idempotent: re-running it re-installs the same skill without creating duplicates.

## Usage

Invoke the skill as the **last step** of a workflow, after review remediation and a clean
`/cy-final-verify`:

```text
/cy-create-prd → /cy-create-techspec → /cy-create-tasks → compozy tasks run <slug>
  → /cy-review-round → compozy reviews fix → /cy-final-verify → /cy-capture-decisions <slug>
```

Capture runs **after `/cy-final-verify`** on purpose: it must reconcile against the remediated,
verified state, not a pre-review snapshot. Capturing earlier risks promoting a decision that review
later reverses (a wrong instruction in the log is worse than none).

```bash
/cy-capture-decisions feat-orders
```

`<slug>` names the source workflow (`.compozy/tasks/<slug>/`) to reconcile from. The skill is
idempotent: re-running it on an unchanged workflow is a no-op. It prints a run summary of what it
promoted, updated, superseded, or skipped, and never touches your `.gitignore`, `CLAUDE.md`, or
`AGENTS.md` — the two setup steps below are yours to apply.

Capture is a single-writer operation: run one capture at a time in a workspace. Concurrent captures
against the same decision log are unsupported. If a serial run is interrupted, re-run it; provenance
reconciliation repairs the log without duplicating an `AD`.

## Make the decision log durable (gitignore)

The log is only useful if it is committed and shared. What you need depends on your repo:

- **Vanilla project** (commits `.compozy/`, ignores only things like `.DS_Store`): nothing to do —
  the log is committed by default.
- **No `.gitignore` at all**: nothing to do — git tracks the log by default.
- **Ignore-heavy project** (a `.gitignore` that ignores `.compozy/**`, e.g. skeeper-managed repos,
  including Compozy itself): the log would be silently uncommitted. Re-include it by adding these
  negations to your `.gitignore`:

```gitignore
# Keep the durable decision log committed even though .compozy/** is ignored.
!.compozy/DECISIONS.md
!.compozy/decisions/
!.compozy/decisions/**
```

The middle line (`!.compozy/decisions/`) re-includes the directory itself, which git requires before
either `!.compozy/decisions/**` or the index negation can take effect — git will not re-include a file
whose parent directory is still excluded. Verify the result with the `-v` form:
`git check-ignore -v .compozy/DECISIONS.md` (and the same for `.compozy/decisions/AD-001.md`) prints the
matching negation rule — e.g. `.gitignore:2:!.compozy/DECISIONS.md` — and exits 0, confirming the path
is no longer ignored. Plain `git check-ignore` prints nothing and exits 1 for a re-included path, so its
silence is the only "tracked" signal; the `-v` form is the unambiguous check.

If you skip this step in an ignore-heavy repo, capture still writes the log, but it stays uncommitted
and non-durable — and the "review the diff before sharing" flow does not apply, because there is no
tracked diff to review.

## Wire the index into agent memory

The read side is a documentation convention, not a runtime hook: interactive planning skills run inside
the coding agent, outside Compozy's Go runtime, so no extension hook can reach them. Wire the terse index
through the project-memory mechanism your agent supports so **every** session — planning included — reads
it automatically, with no manual step per feature.

For agents that expand file imports, add this line to their project-memory file (for example,
`CLAUDE.md`):

```text
@.compozy/DECISIONS.md
```

- Codex reads `AGENTS.md` but does not expand `@file` imports. Add this instruction to `AGENTS.md`:

  ```text
  Before planning or implementation, read .compozy/DECISIONS.md once in the fresh session.
  Read a matching .compozy/decisions/AD-NNN.md body only when the current work needs its details.
  ```

- If a repository serves both Claude and Codex, keep the `@` import for Claude and the explicit read
  instruction for Codex. Add each once.
- Only the terse index is imported or read at startup. Rich `.compozy/decisions/AD-NNN.md` bodies are
  read on demand, so context cost stays bounded as the log grows.
- If neither wiring form is present, the index is not consumed automatically; usage degrades to reading
  `.compozy/DECISIONS.md` manually.

Because the index carries only active, `proven` decisions, what loads into every session stays terse
and trustworthy.

## Decision-record schema

The produced files follow a fixed, machine-checked grammar. The canonical definitions ship with the
skill:

- `skills/cy-capture-decisions/references/decision-record-template.md` — the `AD-NNN.md` frontmatter
  schema, body sections, and filename pattern.
- `skills/cy-capture-decisions/references/index-format.md` — the `DECISIONS.md` line grammar and the
  active-`proven`-only membership rule.

A Go validator (`decisionlog`, shipped beside this extension) parses fixture logs of this exact
shape in `make verify`, so the documented grammar and its examples stay self-consistent and any
regression in the format contract fails CI. It is a test-only asset (ADR-004) that guards the
format definition — it does not run over the log a project actually produces, so a clean
`make verify` proves the grammar is stable, not that a given `.compozy/DECISIONS.md` is well-formed.

The opt-in behavioral harness under `evals/` installs the shipped extension into an isolated Compozy
home and executes the full reconciliation and consumption matrix against a real model three times.
Run it with `COMPOZY_EVAL_MODEL=<model> make eval-cy-capture-decisions`; it is intentionally outside
`make verify` because it consumes a paid, nondeterministic model.

## Acknowledgements

Inspired by Felipe Rodrigues's [`tlc-spec-driven`](https://github.com/tech-leads-club/agent-skills/tree/main/packages/skills-catalog/skills/%28development%29/tlc-spec-driven) skill (Tech Leads Club, CC-BY-4.0) — specifically the durable `AD-NNN` decision-log pattern with supersession and a project-durability relevance gate. `cy-capture-decisions` is an independent reworking of that idea: a post-pipeline reconciliation-and-promotion step with evidence gating, a two-tier `@import` index, and a Go grammar validator. It reuses the concept, not the source text.

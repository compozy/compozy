---
name: spec-peer-review
description: >-
  Cross-LLM peer review of a spec — TechSpec, design doc, RFC, or detailed PRD — run via
  `compozy exec`, producing one scoped Markdown findings artifact for user-directed
  incorporation. Use when the user has approved a spec draft and explicitly wants an external
  review round, especially for autonomy/network/security/migration-impacting designs.
  Project-agnostic: any repo, any language. Don't use for implementation/diff review (use
  impl-peer-review) or as an automatic/looping approval gate.
trigger: explicit
argument-hint: "[spec-path] [--context p1,p2] [--out dir] [--ide <ide>] [--model <model>] [--reasoning <effort>]"
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Spec Peer Review

An authoring model drafts a spec; a second, independent model pressure-tests it. Run this
cross-LLM **peer review** only when the user has approved the current draft and explicitly asks
for a round — each round is one user-initiated pass, never auto-run, auto-incorporated, or
auto-looped.

**Compozy is the review engine**: each round is one `compozy exec` against a configurable
reviewer runtime (`--ide`/`--model`/`--reasoning`). The **findings file** the reviewer writes
is the source of truth; `compozy exec` stdout/stderr is operational evidence only.

## Bundled files

Resolve `scripts/<name>` and `references/<name>` relative to this SKILL.md's directory (expand
to an absolute path before reading or running). All three are read-only — read or run them,
never edit:

- `references/quality-markers.md` — the six tech-agnostic markers a spec must carry; the
  Step 1 readiness gate. The markers are defined there, not inlined here.
- `references/peer-review-prompt.md` — the canonical reviewer prompt template, including the
  scoped-write contract and the exact findings-file format. Substitute its placeholders
  verbatim.
- `scripts/validate-findings.sh` — read-only structural validator for the findings file
  (frontmatter, required sections, no placeholders); Step 4 runs it.

## User decisions

When a step tells you to ask the user (incorporation choice, another round), use the runtime's
interactive question tool — the one that presents a question and pauses for the user's answer
(e.g. AskUserQuestion). If the runtime has none, make the question your complete assistant
message and stop generating, and let the user answer.

## Inputs

- **spec-path** (optional): explicit path to the spec under review (a TechSpec, design doc,
  RFC, or detailed PRD). When omitted, auto-resolve to the most recently modified
  `.compozy/tasks/<slug>/_techspec.md` **if** that layout exists; otherwise ask the user for
  a path. Never invent a path.
- `--context <p1,p2,...>` (optional): additional context files for the reviewer (ADRs, related
  specs, RFCs, research, design docs).
- `--out <dir>` (optional): output directory for round artifacts (see Output Directory).
- `--ide <ide>` (optional, default `claude`): reviewer runtime, forwarded to `compozy exec
  --ide`. Accepted values mirror `compozy exec`: `codex`, `claude`, `cursor-agent`, `droid`,
  `opencode`, `pi`, `gemini`, `copilot`. Reject an invalid value rather than falling back.
- `--model <name>` (optional, default `opus`): forwarded to `compozy exec --model`. Let
  `compozy` surface incompatibilities rather than pre-validating.
- `--reasoning <effort>` (optional, default `xhigh`): forwarded to `compozy exec
  --reasoning-effort`. Accepted: `low`, `medium`, `high`, `xhigh`.

## Output Directory

Resolve `<out>` in this order, and create the directory if it does not exist:

1. `--out` if provided.
2. Otherwise, if the spec resolves under a `.compozy/tasks/<slug>/` layout, use that task's
   `qa/` directory: `.compozy/tasks/<slug>/qa/`.
3. Otherwise, `.peer-reviews/<UTC-timestamp-YYYYMMDDTHHMMSSZ>/` at the repository root.

## Findings file

Each round has exactly one authoritative findings file: `<out>/peer-review-findings-roundN.md`.
The reviewer writes it — and only it — per the format and scoped-write contract in
`references/peer-review-prompt.md`, which `scripts/validate-findings.sh` enforces. All round
artifacts are versioned `-roundN`; never overwrite a prior round.

## Procedures

**Step 0: Verify Compozy is available**

1. Confirm the `compozy` binary is on `PATH` (e.g. `command -v compozy`). If missing, abort
   with a one-line install hint. Cross-LLM independence is the point — the reviewer runs
   through `compozy exec`, never a harness-native subagent.
2. Call `compozy exec` directly (no `--agent`); no agent install is required.

**Step 1: Validate input and context**

1. Resolve `spec-path` (see Inputs). If omitted and no `.compozy/tasks/<slug>/_techspec.md`
   exists, ask the user for a path and stop.
2. Confirm the user has approved the current draft or asked to review the saved spec as-is.
3. Read the spec; confirm it is a final-shape design (boundaries, plan, verification strategy),
   not a rough draft.
4. **STOP. Read `references/quality-markers.md` in full before assessing readiness** — the six
   markers are defined there, not here. Verify the spec carries all six. If any is missing,
   report which and ask the user whether to amend the spec first or proceed anyway.
5. Resolve `<out>` (see Output Directory); ensure it exists and is writable.
6. Set the round number: list existing `<out>/peer-review-findings-round*.md` and
   `<out>/peer-review-summary-round*.md`. Start at `round1` when none exist.

**Step 2: Compose the review prompt**

1. **STOP. Read `references/peer-review-prompt.md` in full before composing** — it is the
   canonical reviewer prompt template. Substitute its placeholders verbatim; do not paraphrase
   it. The assembled prompt must start with the reviewer instructions, not a wrapper describing
   the template.
2. Define this round's artifact paths under `<out>/`: `peer-review-findings-roundN.md`,
   `peer-review-events-roundN.jsonl` (event log), `peer-review-result-roundN.err` (stderr),
   `peer-review-status-before-roundN.txt`, `peer-review-status-after-roundN.txt`, and
   `peer-review-validation-error-roundN.md` (only when needed).
3. Discover project rule files for `{project_rules}`: root-level `CLAUDE.md`, `AGENTS.md`,
   `.cursor/rules/*`, `.cursorrules`, `CONTRIBUTING.md`, plus nested `CLAUDE.md`/`AGENTS.md` in
   the areas the spec touches, plus repo memory/directive docs (e.g. `docs/_memory/`, standing
   directives, lessons indexes) — where load-bearing invariants usually live.
4. Substitute the placeholders:
   - `{spec_path}` — exact path to the spec under review.
   - `{context_paths}` — newline-separated `--context` paths, or the literal `none`.
     **Spec-corpus rule:** when the spec resolves under a spec directory (e.g.
     `.compozy/tasks/<slug>/`), also resolve and include its sibling corpus —
     requirements/use-case documents, canonical example documents, input tables, QA seeds,
     test contracts, analysis summaries, ADRs — even when the user passed no `--context`. A
     round that omits the sibling corpus is invalid: reviewing a spec in isolation from its own
     corpus lets paraphrase drift and requirement contradictions pass unseen. (Real incident: a
     task decomposition paraphrased its spec's canonical example document without linking it;
     the implementer built from the paraphrase, seven implementation-review rounds reached
     `SHIP`, and the shipped result contradicted the product contract wholesale. Spec review is
     the earliest gate that catches a decomposition that fails to wire its canonical artifacts.)
   - `{project_rules}` — newline-separated discovered rule-file paths, or `none`.
   - `{findings_path}` — absolute path to `<out>/peer-review-findings-roundN.md`.
   - `{round}` — numeric review round `N`.
5. Write the assembled prompt to `<out>/peer-review-prompt-roundN.md`.

**Step 3: Execute the cross-LLM review**

1. Snapshot pre-run status:

   ```bash
   git status --short > <out>/peer-review-status-before-roundN.txt
   ```

2. Run (substitute the resolved `--ide`/`--model`/`--reasoning`, defaults `claude`/`opus`/`xhigh`):

   ```bash
   compozy exec --ide <ide> --model <model> --reasoning-effort <reasoning> --format json --prompt-file <out>/peer-review-prompt-roundN.md > <out>/peer-review-events-roundN.jsonl 2> <out>/peer-review-result-roundN.err
   ```

3. Snapshot post-run status:

   ```bash
   git status --short > <out>/peer-review-status-after-roundN.txt
   ```

4. Non-zero exit → fail loudly; do not retry silently; inspect stderr for model
   misconfiguration (see Error Handling).
5. `peer-review-events-roundN.jsonl` is operational evidence only — do not parse it for
   readiness or findings.
6. Require the findings file to exist after the command exits; missing = invalid round even on
   exit 0.

**Step 4: Validate and summarize findings**

1. Run the validator:

   ```bash
   bash <skill-dir>/scripts/validate-findings.sh --kind techspec --round N --path <out>/peer-review-findings-roundN.md
   ```

2. Inspect the findings file for the semantic contract:
   - every finding cites a real section/path reference;
   - each blocker's rationale ties to a project rule or architecture constraint;
   - no `TBD`, placeholder text, invented paths, or stdout-only findings;
   - when sibling corpus artifacts were in `{context_paths}`, the findings explicitly assess
     the spec's consistency with them (requirements honored, canonical contracts not diluted,
     concrete artifacts wired as required reading for implementers) — a `READY` verdict with no
     corpus-consistency assessment is an invalid round;
   - the pre/post status snapshots show no changes outside the expected review artifact/log
     paths.
3. Validation fails → write `<out>/peer-review-validation-error-roundN.md` with the failed
   checks, command, exit status, and artifact paths; do not summarize the round as `READY`.
4. Write `<out>/peer-review-summary-roundN.md` from the validated findings: the readiness
   verdict (`READY` / `BLOCKED` / `NEEDS_REWORK`); one-line rationale per blocker; the nits
   list; sections/ADRs likely affected; the operational artifact paths.
5. Present a concise user summary: verdict, blocker/nit counts, main themes, and the artifact
   paths written for the round.
6. Leave the spec and any ADRs unchanged until Step 5.

**Step 5: User-directed incorporation**

1. Ask the user which findings to incorporate (see User decisions): (A) all blockers,
   (B) selected blockers/nits, (C) nothing, (D) manual edits before any incorporation.
2. Apply only what the user selected.
3. If incorporation needs an ADR or related-doc update, touch only the docs tied to the
   selected findings.
4. Record the decision in `<out>/peer-review-incorporation-roundN.md`: incorporated items,
   deferred items, files changed.
5. Show the user what changed and what stays deferred.

**Step 6: Optional additional rounds**

1. Ask whether the user wants another round or wants to stop with the current saved spec.
2. On request, re-run from Step 2 against the updated spec into a fresh `roundN+1` artifact set
   in the same `<out>` directory.
3. Run further rounds only when the user asks — never auto-loop.

## Guardrails

- This skill only reviews and, on request, incorporates selected findings — it never commits,
  pushes, opens PRs, or approves a spec.
- Spend external review credit once per round: one `compozy exec`, rerun only when the round is
  invalid and the user asks.

## Error Handling

- **Model misconfiguration (`The model 'X' does not exist`):** surface the configured model,
  verify with the user, and record the failure in the round artifacts. The runtime may hold a
  stale name — do not substitute a model on your own.
- **`--ide` invalid:** list the accepted values and ask the user to choose.
- **Missing, malformed, or placeholder findings:** treat as an invalid round — write
  `peer-review-validation-error-roundN.md` and ask whether to rerun. Infer readiness only from
  the validated findings file, never from stdout.

# cy-capture-decisions — Behavioral Eval Runbook

`cy-capture-decisions` ships no compilable logic; its correctness is **reconciliation quality**, which is
validated by running the skill against curated fixture workflows and asserting **structural properties**
of the produced log (which `AD` ids exist, their `status`/frontmatter, index membership) — never exact
prose. This runbook is the eval suite: it maps every assigned test case to a fixture and the log
properties the run must produce. It is a documented suite, not part of `make verify`'s unit path (which
covers only the deterministic format check owned by task_02).

## Layout

```
evals/
  README.md                      # this runbook
  fixtures/<slug>/
    workflow/                    # staged as .compozy/tasks/<slug>/ (adrs/, memory/, reviews-NNN/, task_*.md)
    diff.patch                   # the code change; apply so `git diff main...HEAD` is scopable
    diff-phase2.patch            # (feat-search only) second-phase evidence for the candidate lifecycle
    seed-log/                    # (feat-auth only) pre-existing .compozy/DECISIONS.md + decisions/ to supersede
    expected.md                  # the expected outcome properties for that fixture
```

## Harness (per eval)

1. Create a scratch git repo with a `main` branch and a working `HEAD` branch (so `git diff main...HEAD`
   resolves). A coding-agent runtime (e.g. Claude Code) is required — the skill uses the agent's own
   file/shell tools.
2. Copy `fixtures/<slug>/workflow/` to `.compozy/tasks/<slug>/`.
3. For a supersession eval, copy `fixtures/<slug>/seed-log/` contents to `.compozy/DECISIONS.md` +
   `.compozy/decisions/`. Otherwise start with no log (or the empty-state index for fresh-project cases).
4. Apply `fixtures/<slug>/diff.patch` (and commit on `HEAD`) so the diff is scopable. For degraded-mode
   evals, deliberately make the diff unscopable instead (see feat-telemetry).
5. Run `/cy-capture-decisions <slug>` (or `compozy exec "/cy-capture-decisions <slug>"`).
6. Assert the properties in `fixtures/<slug>/expected.md` and in the matrix below. Assert on structure
   (ids, `status`, frontmatter fields, index membership), not on wording.

## Assigned case → fixture → expected properties

| Case    | Journey                         | Fixture                                               | Setup                                            | Expected log property to assert                                                                                                                                                           |
| ------- | ------------------------------- | ----------------------------------------------------- | ------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| E2E-001 | promotion                       | feat-orders                                           | canonical                                        | log created; `AD-001` carries `source_slug`/`source_adr` provenance and `proven`.                                                                                                         |
| E2E-002 | reconciliation / deviation      | feat-payments (deviation) + feat-orders (as-designed) | canonical                                        | payments `AD` body has `## Reconciliation` + `[DEVIATION]` + cited evidence; orders body says "implemented as designed", no `[DEVIATION]`.                                                |
| E2E-003 | relevance gate                  | feat-orders                                           | canonical                                        | exactly one `AD` promoted (adr-002); summary lists adr-001 + adr-003 skipped with gate reasons.                                                                                           |
| E2E-004 | classification / numbering      | feat-orders                                           | empty log; also stage a 2nd durable ADR          | empty log → NEW `AD-001`; two NEW in one run → `AD-001`, `AD-002`, no collision.                                                                                                          |
| E2E-005 | fresh-eyes evidence             | feat-search                                           | phase 1                                          | diff-proven decision `proven`; memory-only decision `candidate`; runs even with no memory file.                                                                                           |
| E2E-006 | idempotency                     | feat-orders                                           | run twice unchanged; then change                 | re-run unchanged → zero file changes, "no changes"; changed → only the affected `AD` updated in place (same id).                                                                          |
| E2E-007 | terse index                     | feat-search + feat-auth                               | after promote                                    | `DECISIONS.md` lists only active-proven; a `candidate` (search adr-002 ph1) and a `superseded` (auth AD-001) are absent.                                                                  |
| E2E-008 | rich body                       | feat-orders                                           | canonical                                        | opening `AD-001.md` shows frontmatter + 4 original ADR sections + `Reconciliation`; a deviation case is explicit with evidence (see feat-payments).                                       |
| E2E-009 | candidate lifecycle             | feat-search                                           | phase 1 → phase 2                                | no-evidence decision → `candidate`, absent from index; later capture with evidence → same `AD` becomes `proven` in place and appears in the index.                                        |
| E2E-010 | supersession                    | feat-auth                                             | seed-log staged                                  | old `AD-001` gets `superseded_by`, leaves index, keeps file; new `AD` has `supersedes`; chain A→B→C → only C in index.                                                                    |
| E2E-013 | capture as final step           | feat-orders                                           | full pipeline (reviews + verify present)         | reconciles against post-remediation diff, incorporates `reviews-001/issue_001`, uses `status: completed`; a clean (no-issue) review still proceeds on diff+verify and is noted.           |
| E2E-014 | timing (before vs after review) | feat-orders                                           | remove `reviews-001/`, run; then restore, re-run | before review → log from diff+verify only; re-run after review → affected `AD` updated in place (E2E-006).                                                                                |
| E2E-015 | VCS review of output            | feat-orders                                           | capture, edit `AD-001.md`, re-capture            | capture is a reviewable diff; editing/reverting a wrong entry then re-capturing reconciles to the corrected state via provenance (no blind re-add).                                       |
| E2E-016 | degraded mode                   | feat-telemetry                                        | unscopable diff                                  | degrades to memory + reviews, marks entries "unverified against code" as `candidate`; with neither diff nor reviews → promotes nothing; broken range → reports scoping failure, no crash. |
| E2E-017 | no promotable decisions         | feat-noop                                             | existing log; then fresh project                 | no `Accepted`/all feature-local → no-op, zero promotions with reasons, no empty files; fresh project → valid empty-state index.                                                           |
| E2E-018 | bad slug                        | (none)                                                | run `no-such-slug`; run empty slug               | non-existent/archived slug → clear "not found / already archived", nothing written; omitted/malformed slug → rejected, no guessed target.                                                 |
| E2E-019 | malformed source ADR            | feat-payments                                         | canonical                                        | malformed `adrs/adr-002.md` skipped with a warning; adr-001 still processed.                                                                                                              |
| E2E-020 | weak semantic match → NEW       | feat-search                                           | stage a weakly-similar-titled prior AD           | no `source_adr` match + only weak semantic match → classified NEW (not an incorrect UPDATE), with a low-confidence note.                                                                  |
| E2E-021 | interrupted / concurrent        | feat-orders                                           | write a half `AD` then re-run                    | re-running yields a consistent log (no duplicate/half-written `AD`); two captures on one slug do not corrupt (single-serial-run constraint verified by the reconciled outcome).           |
| IT-001  | capture wiring                  | feat-orders                                           | canonical                                        | writes `.compozy/DECISIONS.md` (one active-proven line) + `.compozy/decisions/AD-001.md` with `source_slug=feat-orders`, `source_adr=adrs/adr-002.md`.                                    |
| IT-005  | unwritable target               | feat-orders                                           | `chmod -w .compozy` before run                   | write step fails; the pre-existing log is unchanged (no partial/truncated files).                                                                                                         |
| IT-006  | VCS reviewability               | feat-orders                                           | capture, then `git status`/`git diff`            | the log change is a normal reviewable diff; a pre-existing local edit is surfaced as a diff/merge, not silently overwritten.                                                              |
| E2E-011 | consumption: auto-loaded index  | feat-orders (seed a proven log, wire the `@import`)   | consume harness (below)                          | a fresh agent session (e.g. starting `/cy-create-prd`) has the proven index in context with no manual step; an empty index adds negligible context.                                       |
| E2E-012 | consumption: on-demand bodies   | feat-orders (seed a proven log incl. a superseded AD) | consume harness (below)                          | during a new feature touching a tagged area the relevant `decisions/AD-NNN.md` body is read on demand; unrelated bodies are not loaded; a superseded body surfaces its active successor.  |

## Consumption evals (US-011, US-012 — the read side)

E2E-011 and E2E-012 validate the _read_ side, so they use a **consume harness**, not the capture harness
above — no `/cy-capture-decisions` run is involved. The deterministic core of both (importing the index
surfaces its text; bodies under `decisions/` are not pulled by the index import) is guarded in
`make verify` by IT-002 in the `packaging` package; these evals cover the behavioral, in-session half.

Consume harness (per eval):

1. Stage a seeded log: write a proven `.compozy/DECISIONS.md` index plus its `.compozy/decisions/AD-NNN.md`
   bodies at the workspace root (reuse `fixtures/feat-orders/expected.md` shape, or a hand-authored seed).
   For E2E-012, include a `superseded` body whose successor is active in the index.
2. Wire consumption exactly as the extension README documents: add `@.compozy/DECISIONS.md` to `CLAUDE.md`
   (and/or `AGENTS.md`).
3. Start a **fresh** coding-agent session in that workspace (e.g. begin `/cy-create-prd`); do no manual
   copy step.
4. Assert the properties:
   - **E2E-011:** the proven index lines are present in the session's context with no manual action; an
     empty-state index adds only its header (negligible context). If the `@import` line is absent, the
     index is simply not auto-loaded (US-011.EC-1) — the documented degrade, not a failure.
   - **E2E-012:** when the new work touches a tagged area, the matching `decisions/AD-NNN.md` body is read
     on demand; bodies for unrelated areas stay unloaded; opening a superseded body surfaces its active
     successor (`superseded_by` → the index's active AD).

Assert on structure (which ids/lines are present in context, which bodies were opened), never on prose.

## Pass criteria

An eval passes when every property in its `expected.md` and its matrix row holds. Because reconciliation
is LLM behavior, run each eval 3× and require the structural properties (ids, status, index membership)
to hold on every run; treat prose variation as noise. The deterministic format grammar these files
follow (required fields, `status` enum, active-proven-only index, bidirectional supersession) is guarded
in `make verify` by the task_02 `decisionlog` validator — but that validator parses hand-authored fixtures
of this same shape, not the eval-produced logs here, whose format is asserted structurally by running this
suite.

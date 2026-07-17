# cy-capture-decisions — Behavioral Eval Runbook

`cy-capture-decisions` ships an LLM-driven reconciliation workflow, so its behavioral contract is tested
by an executable, opt-in model-backed harness. The harness installs the exact extension under test into
an isolated Compozy home, invokes the installed skill against scratch Git repositories, validates every
produced log with the deterministic grammar validator, and then asserts case-specific structural
properties. Assertions cover ids, provenance, status, evidence, supersession, index membership, and
file-system safety — never exact prose.

The matrix contains the 21 E2E cases plus IT-001, IT-005, and IT-006: 24 cases total. Every selected case
runs three times by default, so the full matrix produces 72 independently asserted trials.

## Layout

```
evals/
  README.md                      # this runbook
  cases.go                       # executable case journeys and structural assertions
  harness.go                     # isolated install, model runner, artifacts, and 3x orchestration
  cmd/cy-capture-decisions-eval/ # opt-in CLI entrypoint
  fixtures/<slug>/
    workflow/                    # staged as .compozy/tasks/<slug>/ (adrs/, memory/, reviews-NNN/, task_*.md)
    diff.patch                   # the code change; apply so `git diff main...HEAD` is scopable
    diff-phase2.patch            # (feat-search only) second-phase evidence for the candidate lifecycle
    seed-log/                    # (feat-auth only) pre-existing .compozy/DECISIONS.md + decisions/ to supersede
    expected.md                  # the expected outcome properties for that fixture
  examples/fitnesshub-web/       # sanitized-path, real-world supersession example
  results/                       # generated raw events, output logs, workspaces, and summaries
```

## Run the executable harness

The paid model run is deliberately opt-in and is not part of `make verify`:

```bash
COMPOZY_EVAL_MODEL=gpt-5.6-luna \
  COMPOZY_EVAL_REASONING_EFFORT=medium \
  make eval-cy-capture-decisions
```

For a fast harness smoke test, select one case and one repetition directly:

```bash
make build
go run ./extensions/cy-capture-decisions/evals/cmd/cy-capture-decisions-eval \
  --model gpt-5.6-luna --reasoning-effort medium \
  --cases E2E-001 --repetitions 1
```

The command exits non-zero if any structural assertion fails. It always writes `summary.json` and
`summary.md`; each trial also retains raw ACP JSONL, stderr, generated decision artifacts, and the Git
diff under `evals/results/<case>/run-<n>/`. IT-005 is reported as `SKIP`, never `PASS`, when the host
permission model does not enforce the read-only sandbox (for example, a privileged process or Windows).

## Harness behavior (per eval)

1. Build Compozy, create an isolated `COMPOZY_HOME`, install and enable the extension from this checkout,
   start an isolated daemon, and install the shipped skill into each scratch workspace.
2. Digest-compare the complete installed skill tree (`SKILL.md` plus every reference) with the shipped
   source so a stale local skill cannot satisfy the eval accidentally.
3. Create a scratch git repo with a `main` branch and a working `HEAD` branch so
   `git diff main...HEAD` resolves, then copy `fixtures/<slug>/workflow/` to
   `.compozy/tasks/<slug>/`.
4. For a supersession eval, copy `fixtures/<slug>/seed-log/` contents to `.compozy/DECISIONS.md` +
   `.compozy/decisions/`. Otherwise start with no log (or the empty-state index for fresh-project cases).
5. Apply `fixtures/<slug>/diff.patch` and commit on `HEAD` so the diff is scopable. For degraded-mode
   evals, deliberately make the diff unscopable instead (see feat-telemetry).
6. Invoke Compozy with the selected IDE, model, and reasoning effort using the natural-language skill
   request `Use the cy-capture-decisions skill to capture the finished <slug> workflow.` ACP runtimes do
   not interpret a skill name as a slash command when it is passed through `compozy exec`.
7. Validate the complete generated log and assert the properties in the matrix below.

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
| E2E-017 | no promotable decisions         | feat-noop                                             | existing log; then fresh project                 | no `Accepted`/all feature-local → no-op with reasons; an existing log stays unchanged and a fresh project gets no unnecessary empty log files.                                            |
| E2E-018 | bad slug                        | (none)                                                | run `no-such-slug`; run empty slug               | non-existent/archived slug → clear "not found / already archived", nothing written; omitted/malformed slug → rejected, no guessed target.                                                 |
| E2E-019 | malformed source ADR            | feat-payments                                         | canonical                                        | malformed `adrs/adr-002.md` skipped with a warning; adr-001 still processed.                                                                                                              |
| E2E-020 | weak semantic match → NEW       | feat-search                                           | stage a weakly-similar-titled prior AD           | no `source_adr` match + only weak semantic match → classified NEW (not an incorrect UPDATE), with a low-confidence note.                                                                  |
| E2E-021 | interrupted serial recovery     | feat-orders                                           | write a half `AD` then re-run serially           | re-running yields a valid consistent log with exactly one provenance-matched decision and no duplicate or half-written `AD`; concurrent capture is explicitly outside the contract.       |
| IT-001  | capture wiring                  | feat-orders                                           | canonical                                        | writes `.compozy/DECISIONS.md` (one active-proven line) + `.compozy/decisions/AD-001.md` with `source_slug=feat-orders`, `source_adr=adrs/adr-002.md`.                                    |
| IT-005  | unwritable target               | feat-orders                                           | `chmod -w .compozy` before run                   | write step fails; the pre-existing log is unchanged (no partial/truncated files).                                                                                                         |
| IT-006  | VCS reviewability               | feat-orders                                           | capture, then `git status`/`git diff`            | the log change is a normal reviewable diff; a pre-existing local edit is surfaced as a diff/merge, not silently overwritten.                                                              |
| E2E-011 | consumption: automatic index    | feat-orders (seed a proven log, wire agent memory)    | consume harness (below)                          | a fresh agent session reads the proven index with no manual user step; an empty index adds negligible context.                                                                            |
| E2E-012 | consumption: on-demand bodies   | feat-orders (seed a proven log incl. a superseded AD) | consume harness (below)                          | during a new feature touching a tagged area the relevant `decisions/AD-NNN.md` body is read on demand; unrelated bodies are not loaded; a superseded body surfaces its active successor.  |

## Consumption evals (US-011, US-012 — the read side)

E2E-011 and E2E-012 validate the _read_ side, so their executable cases use a **consume journey**, not a
capture invocation. The deterministic core of both (importing the index
surfaces its text; bodies under `decisions/` are not pulled by the index import) is guarded in
`make verify` by IT-002 in the `packaging` package; these evals cover the behavioral, in-session half.

Consume harness (per eval):

1. Stage a seeded log: write a proven `.compozy/DECISIONS.md` index plus its `.compozy/decisions/AD-NNN.md`
   bodies at the workspace root (reuse `fixtures/feat-orders/expected.md` shape, or a hand-authored seed).
   For E2E-012, include a `superseded` body whose successor is active in the index.
2. Wire consumption exactly as the extension README documents: use `@.compozy/DECISIONS.md` for an
   import-capable agent, or put the read-on-start instruction in `AGENTS.md` for Codex.
3. Start a **fresh** coding-agent session in that workspace (e.g. begin `/cy-create-prd`); do no manual
   copy step.
4. Assert the properties:
   - **E2E-011:** the proven index lines are consumed in a fresh session with no manual user action; an
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

## Concurrency contract

The extension guarantees interruption recovery for a **single serial writer**. Concurrent captures on
the same workspace are unsupported. The skill, extension README, this runbook, and E2E-021 all use that
same contract. E2E-021 proves the supported recovery path by seeding a half-written record, running one
capture, validating the whole log, and asserting that only one provenance-matched `AD` exists.

This is an intentional guarantee boundary, not a claim that concurrent writers are safe. A future
concurrent-writer guarantee would require an implementation-level lock or transactional writer and a
separate parallel-process integration test.

## Real-world supersession example

`examples/fitnesshub-web/` contains the self-contained subset from a production project that demonstrates
a two-to-one supersession: AD-011 and AD-014 are retained as `superseded`, both point to AD-016, and AD-016
links back to both. The reduced `DECISIONS.md` contains only active AD-016, exactly as the index contract
requires. AD-015 is not part of this chain and is intentionally excluded.

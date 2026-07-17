# Expected outcome — feat-search

Exercises: proven-vs-candidate by evidence (E2E-005), candidate excluded from index (E2E-007),
candidate→proven lifecycle in place (E2E-009), weak-semantic-match → NEW (E2E-020).

## Phase 1 — apply `diff.patch` only (no ranking code)

- `adr-001` (inverted index) has diff evidence → promoted `proven`; appears in the index.
- `adr-002` (BM25) is claimed only in `memory/MEMORY.md`, not in the diff → promoted as
  `status: candidate` with a missing-evidence note in `## Reconciliation`. It is written to its
  `AD-NNN.md` file but is **absent** from `.compozy/DECISIONS.md`.

## Phase 2 — additionally apply `diff-phase2.patch` (adds `rank.go`), re-run capture

- `adr-002` now has diff evidence → the **same** `AD` id flips `status: candidate → proven` in place
  (no new number) and now appears in the index.
- `adr-001` is unchanged (provenance match) → no-op.

## Weak-match sub-case (E2E-020)

- A decision whose `source_adr` matches nothing in the log and only weakly matches an existing title by
  wording → classified **NEW** with a low-confidence note, never an incorrect UPDATE.

## Assertions

- After phase 1: index has exactly the `adr-001` line; the `adr-002` candidate body exists but is not
  indexed.
- After phase 2: the `adr-002` body id is unchanged and its status is `proven`; it is now indexed.

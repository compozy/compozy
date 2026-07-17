# Expected outcome — feat-auth

Exercises: supersession (E2E-010) and index proven-only after supersession (E2E-007).

## Setup

Stage `seed-log/` as the pre-existing `.compozy/DECISIONS.md` + `.compozy/decisions/` (an active
`AD-001` from feat-sessions: stateless JWT sessions), then run capture on `feat-auth`.

## After capture

- A new body (next id, `AD-002`) is created for the server-side sessions decision with
  `supersedes: [AD-001]`, `status: proven`.
- The old `AD-001` body is updated: `superseded_by: AD-002`, `status: superseded`. Its file is kept.
- `.compozy/DECISIONS.md` shows only the new `AD-002` line; the superseded `AD-001` line is gone.

## Chain sub-case

- A third workflow reversing `AD-002` again → `AD-003` with `supersedes: [AD-002]`; `AD-002` becomes
  `superseded`. In the chain A→B→C only the tail `AD-003` appears in the index.

## Assertions

- Supersession links are bidirectional (`supersedes` / `superseded_by` point at each other).
- The superseded record keeps its file but leaves the index.
- Never rewrite the old decision's meaning in place — a new record supersedes it.

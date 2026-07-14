# Architecture audit behavioral scenario catalog

Use this catalog with the fixture workspaces and the installed skill evaluation. Every non-deferred
contract must have concrete execution evidence before Task 3's corresponding checkbox is marked complete.
The artifact assertions in `skill_e2e_test.go` are the deterministic oracle for report publication and
the depth-map grammar; inspect the agent transcript and fixture state for conversational behavior.

| Scenario | Fixture / setup                                         | Required observable                                                             |
| -------- | ------------------------------------------------------- | ------------------------------------------------------------------------------- |
| E2E-001  | TypeScript target `apps/checkout`                       | Audits only the target and writes `apps-checkout` reports.                      |
| E2E-002  | Either fixture, omit the target                         | Asks for the target before scanning.                                            |
| E2E-003  | Either fixture, `does/not/exist`                        | Prints `target not found`; no artifact.                                         |
| E2E-004  | TypeScript file `apps/checkout/src/place-order.ts`      | Audits the enclosing module and says so.                                        |
| E2E-005  | Copy TypeScript area as `Apps/Web `                     | Shows and uses normalized `apps-web`.                                           |
| E2E-006  | Empty fixture directory                                 | Prints `nothing to audit`; no artifact.                                         |
| E2E-007  | Two normalized-collision targets                        | Warns before overwriting an existing slug.                                      |
| E2E-008  | Fixture with more than 50 source files                  | Offers narrow or sampled scope.                                                 |
| E2E-009  | TypeScript checkout                                     | Candidates name modules, give deletion-test verdicts, and rank by fix-value.    |
| E2E-010  | Go client                                               | Reports healthy zero-candidate outcome.                                         |
| E2E-011  | Go client                                               | Does not flag the functional-options constructor as shallow.                    |
| E2E-012  | TypeScript with a matching `.compozy/DECISIONS.md`      | Names a settled-decision conflict callout when friction warrants reopening.     |
| E2E-013  | TypeScript checkout                                     | Merges or cross-references overlapping candidate evidence.                      |
| E2E-014  | Ambiguous fixture evidence                              | Uses `Speculative`, never `Strong`, without a confident verdict.                |
| E2E-015  | TypeScript checkout                                     | Creates and publishes the HTML report.                                          |
| E2E-016  | Published TypeScript report offline                     | Report content remains readable without CDN enhancements.                       |
| E2E-017  | Headless fixture environment                            | Prints absolute HTML path when opening fails.                                   |
| E2E-018  | TypeScript with existing report then induced failure    | Preserves the prior HTML byte-for-byte.                                         |
| E2E-019  | TypeScript checkout                                     | Markdown has matching candidates, Mermaid fences, escapes, and anchors.         |
| E2E-020  | TypeScript with `.compozy/**` ignored                   | Writes reports and warns that they are untracked.                               |
| E2E-021  | TypeScript checkout                                     | One dominant CTA leads; ties use the deterministic blast-radius tiebreak.       |
| E2E-022  | Single-candidate TypeScript fixture                     | Uses that candidate as the pick without `1 of 1`.                               |
| E2E-023  | TypeScript checkout                                     | Produced `apps/checkout` map section parses through `archmap.Parse`.            |
| E2E-024  | Seed map with `apps/api` and an `avoid` entry           | Re-audit preserves the other raw section and rejection history.                 |
| E2E-025  | Go client                                               | Writes the dated no-opportunities map line.                                     |
| E2E-026  | Seed renamed area                                       | Reconciles and records the move.                                                |
| E2E-027  | Seed interrupted map state                              | Re-run produces one consistent area section.                                    |
| E2E-028  | deferred                                                | Deferred to V1.1; no nested `AGENTS.md` behavior is evaluated in V1.            |
| E2E-029  | TypeScript checkout with `grill-me`                     | Grills the selected pick and explores answerable questions.                     |
| E2E-030  | TypeScript checkout without `grill-me`                  | Ships core artifacts and prints the one-line skip notice.                       |
| E2E-031  | TypeScript checkout, abandon grilling                   | Leaves no decision record.                                                      |
| E2E-032  | TypeScript checkout, decline with durable reason        | Writes dated `avoid` entry and does not re-propose it.                          |
| E2E-033  | TypeScript checkout, decline `not now`                  | Does not write an `avoid` entry.                                                |
| E2E-034  | Seed a rejected deepening then supersede it             | Retains a comment provenance line.                                              |
| E2E-035  | TypeScript with capture companion and durable outcome   | Routes to workflow draft ADR; never writes durable decision records.            |
| E2E-036  | TypeScript checkout, accept a new term                  | Writes a non-duplicate `.compozy/GLOSSARY.md`; no `docs/adr`.                   |
| E2E-037  | TypeScript with active, empty, and superseded decisions | Respects only active settled decisions.                                         |
| E2E-038  | TypeScript with malformed decisions index               | Soft-warns and continues.                                                       |
| E2E-039  | Either fixture after extension install, enable, setup   | Bare install performs the rigorous audit using bundled vocabulary.              |
| E2E-040  | Either fixture without both optional companions         | Delivers core artifacts and isolates companion failures.                        |
| E2E-041  | Either fixture                                          | Leaves `.gitignore`, `CLAUDE.md`, and `AGENTS.md` unchanged.                    |
| E2E-042  | TypeScript with imported map                            | Guidance remains usable if a report link is missing and is advisory when stale. |

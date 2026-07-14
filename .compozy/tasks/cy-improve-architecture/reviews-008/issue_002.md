---
provider: manual
pr:
round: 8
round_created_at: 2026-07-14T06:43:59Z
status: resolved
file: extensions/cy-improve-architecture/evals/skill_e2e_test.go
line: 908
severity: medium
author: claude-code
provider_ref:
---

# Issue 002: Candidate map oracle accepts avoid-only guidance

## Review Comment

The non-empty evaluation branch requires only `len(area.Entries) > 0`. `Area.Entries` includes
`avoid` records, so an avoid-only section passes even though the fresh TypeScript workspace has no
prior rejection history. An unrelated `deep` or `seam` entry also passes without proving that any
published candidate was distilled into actionable depth-map guidance.

`archmap.Parse` establishes grammar, not semantic linkage. This leaves the evaluator weaker than the
skill contract, which requires rebuilding the audited section from the published markdown report,
and weaker than US-006's requirement that audit findings become useful deep-module or seam guidance.

At minimum, require the candidate fixture to emit a `deep` or `seam` entry and add an avoid-only
regression case. Stronger coverage should extract candidate module names from the stable markdown
records and prove that at least one emitted guidance target or note corresponds to those candidates.

## Triage

- Decision: `VALID`
- Root cause: `assertArtifacts` accepts every non-empty `area.Entries` slice. The parser correctly preserves active `avoid` records, but an avoid-only entry is retained history rather than fresh `deep` or `seam` guidance for the current candidate audit.
- Fix approach: require at least one parsed `deep` or `seam` entry for the non-empty candidate path, and cover an avoid-only map as a regression case in the existing evaluator suite.
- Verification: `env -u NO_COLOR make verify` exited 0 after the remediation. The formatter and linter reported 0 warnings and 0 errors; all configured typecheck and test tasks passed.

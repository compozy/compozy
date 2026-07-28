# Goal stop-reason context and budget profile matrix

- Legacy ID: AB-011
- Source: J-28 / GL-017..GL-021, GL-038, GL-040 / `_tests.md` runtime 5..7
- Why automate: recovery semantics require deterministic ACP profiles; the manual acpmock matrix should graduate into the serialized runtime E2E lane.
- Suggested layer: E2E runtime (`make test-e2e-runtime`) plus focused GlobalDB integration for restart baselines and grants.
- Spec sketch: profiles emit all five stop reasons, reporting/silent/stale usage, effective/ineffective compaction, and token/wall crossings before and inside a turn; assert judge/no-judge routing, exact grant scopes, one reseed, and no post-crossing effect. True end state: public snapshot and turn audit survive restart with no replay.
- Status: proposed

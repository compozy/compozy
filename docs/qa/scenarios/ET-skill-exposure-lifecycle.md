---
id: ET-skill-exposure-lifecycle
area: ET
title: Expose and unexpose skills through the CLI
persona: Dora
journey: J-layer-profile-resources
expected: Eligible user and workspace skills expose only into enabled preset roots, repeated operations are idempotent, failures preserve foreign entries, and every inspection surface reports the reconciled link state
entry_points: compozy skill create --expose; compozy skill expose; compozy skill unexpose; compozy skill info; compozy skill where
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-manage-skill-source-policy
---

Create a workspace skill with `--expose agents`, verify the provider link, inspect its healthy state, repeat expose and confirm the no-change result, then unexpose and verify that only the owned link disappears. Walk missing, broken, disabled-target, name-conflict, multi-target rollback, and foreign-conflict paths; confirm the four public states and that Compozy never changes a foreign entry. Repeat the refusal with a profile-owned skill and confirm no shared provider-root entry is created.

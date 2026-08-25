---
id: ET-skill-exposure-lifecycle
area: ET
title: Expose and unexpose skills through the CLI
persona: Dora
journey: J-share-skills-with-other-tools
expected: Eligible user and workspace skills expose only into enabled preset roots, repeated operations are idempotent, failures preserve foreign entries, and every inspection surface reports the reconciled link state
entry_points: compozy skill create <name> --expose agents|claude; compozy skill expose <name> --to agents|claude; compozy skill unexpose <name> --to agents|claude; compozy skill info; compozy skill where; POST /api/skills/{name}/expose over HTTP or UDS; POST /api/skills/{name}/unexpose over HTTP or UDS; GET /api/skills/{name}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-skill-expose-panel; ET-skill-source-agent-parity; ET-manage-skill-source-policy
---

Create a workspace skill with `--expose agents`, verify the provider link, inspect its healthy state, repeat expose and confirm the no-change result, then unexpose and verify that only the owned link disappears. Walk missing, broken, disabled-target, name-conflict, multi-target rollback, and foreign-conflict paths; confirm the four public states and that Compozy never changes a foreign entry. Repeat the refusal with a profile-owned skill and confirm no shared provider-root entry is created.

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-share-skills-with-other-tools`. Entry points now carry the HTTP/UDS expose and unexpose routes and the skill detail read, so the CLI and the wire contract are settled in one walk instead of leaving the routes uncovered. `--to` is required and the only accepted targets are `agents` and `claude`: `compozy` is always-on and not exposable, and a custom source must fail `expose_target_invalid`. Also walk removal ordering — a skill with an uncleanable owned link must refuse with `skill_remove_blocked` and preserve the canonical directory. Charter: `CH-skill-expose-lifecycle-trust`.

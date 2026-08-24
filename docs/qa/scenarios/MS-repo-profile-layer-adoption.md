---
id: MS-repo-profile-layer-adoption
area: MS
title: Adopt a dormant repository profile layer by name
persona: Dora
journey: J-layer-profile-resources
expected: A repository named-profile config, MCP, agent, and skill layer stays dormant while its profile name is absent, reports an actionable diagnostic, activates when that profile is created or selected, and becomes dormant again after rename without mutating repository files.
entry_points: <workspace>/.compozy/profiles/<name>/; compozy profile create|rename|use; compozy config show; compozy agent list; compozy skill list; Settings source badges; status diagnostics
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-cli-lifecycle; MS-layered-config-write-truth; MS-profile-memory-tier-scope
---

Flagged by Profiles task 08. Task 13 owns the isolated repository-adoption walk and verdict.

Plant a named layer before the catalog contains that profile. Prove config and MCP entries do not
apply, resources do not resolve, and `config_profile_layer_orphaned` names the dormant path and create
action. Create and select the matching profile, then prove project-profile → project base → personal
profile → user shadow order and `LAYER`/`SHADOWS` output. Rename away and back, confirming wake/dormancy
transitions and byte-identical repository files. Also prove personal profile roots placed inside a
registered workspace are rejected.

Expected evidence: file hashes, diagnostics, profile events, config/MCP effective projections, agent
and skill shadow output, and Settings provenance captures.

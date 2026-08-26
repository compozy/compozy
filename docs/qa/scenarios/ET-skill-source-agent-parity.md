---
id: ET-skill-source-agent-parity
area: ET
title: Operate skill sources entirely through structured surfaces
persona: Ada
journey: J-operate-skill-sources-headless
expected: CLI, HTTP, UDS, native tools, and the extension Host API return the same field names and values for the same persisted source state, every refusal is a matchable code, and no step needs the web UI
entry_points: compozy__skill_list; compozy__skill_search; compozy__skill_view; compozy__config_get; compozy__config_set; compozy__config_unset; GET /api/settings/skills over HTTP or UDS; PATCH /api/settings/skills over HTTP or UDS; GET /api/skills; GET /api/skills/{name}; POST /api/skills/{name}/expose over HTTP or UDS; POST /api/skills/{name}/unexpose over HTTP or UDS; extension Host API skills/list
qa_status: pass
bug_ids: BUG-20260825-skill-detail-rejects-workspace-id;BUG-20260825-skill-source-agent-write-doc-mismatch;BUG-20260825-skill-source-profile-write-rejected;BUG-20260825-workspace-skills-non-source-field-written
fix_status: fixed
retest_status: pass
fix_commits: 2643f4aba;740d868cf;84913fa33;b346b36d4
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/tool-skill-list.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/tool-skill-view.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/parity-after.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-skill-origin-attribution; ET-skill-exposure-lifecycle; ET-manage-skill-source-policy; ET-skill-source-observe-ledger
---

Drive the whole feature with no browser. Read the catalog and the source read model through
`compozy__skill_list`, `compozy__skill_search`, and `compozy__skill_view`, and confirm each carries
`origin` and `owner_scope`, that `owner_scope` stays inside `user|workspace|profile|workspace_profile`,
and that `skill_view` carries `exposures[]` whose `status` stays inside
`healthy|missing|broken|foreign_conflict`. Confirm a Compozy-native skill reports an explicit empty
origin rather than an invented provider label.

Exercise the workspace-parameter asymmetry deliberately, because it is easy to get wrong from a
generated client: `GET /api/skills` takes `workspace` and accepts a reference the caller already
resolved, while `GET /api/skills/{name}` takes the canonical `workspace_id` only and must refuse
`workspace` by pointing at the canonical id. Expose and unexpose carry no workspace query parameter
at all — `workspace_id` travels in the body and the response echoes the resolved id.

Write both keys at user, exact profile, and workspace scope through `compozy config set|unset` and
through `PATCH /api/settings/skills`, and confirm the response states live apply semantics
(`restart_required: false`) and returns the refreshed source read model. At workspace scope, prove
the presence-aware tri-state: an absent field is untouched, `null` clears the override, and an array
sets it. Submit each invalid class and match the code, not the sentence: `unknown_skill_source` with
its `valid` list and `suggestion`, `duplicate_skill_source`, `invalid_source_path`, and
`workspace_scope_field_forbidden` naming the offending field. Confirm a failing expose returns
exactly one `expose_failed` envelope with per-target `results[]` whether one target failed or
several.

Read the same persisted state through the CLI, HTTP, and the socket and compare field names and
values directly rather than eyeballing them, then read `skills/list` through the extension Host API
and confirm its `SkillSummary` reports the same `origin` for the same skill as the native tools do.

**Walk the agent-write question first and record the answer explicitly.** The shipped official skill
(`skills/compozy/references/configuration.md`) states that `compozy__config_set` and
`compozy__config_unset` deny `skills.sources` and `skills.custom_sources` with
`config_trust_root_forbidden`. The code as read on this branch does not do that: both paths sit in
the agent-mutable allowlist (`internal/config/tool_surface.go:155-156`), and that lookup returns
before the trust-root check (`:278-281` versus `:294-296`), so an agent can write them. Assert the
**shipped** behavior, record which side is wrong with evidence, and treat the mismatch as a defect
to file — the product decision (deny in code, or correct the documentation) belongs to a human, not
to this walk.

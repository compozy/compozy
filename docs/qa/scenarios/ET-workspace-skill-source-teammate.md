---
id: ET-workspace-skill-source-teammate
area: ET
title: Open a teammate's repo and get its committed skills
persona: Bruno
journey: J-absorb-skills-from-other-tools
expected: A cloned repository's committed skill folders and source configuration load for anyone who opens that workspace with default personal settings, stay scoped to it, and nothing personal is written back into the repository
entry_points: <ws>/.compozy/config.toml [skills] sources|custom_sources; <ws>/.agents/skills/<name>/SKILL.md; compozy skill list --workspace <ref>; compozy skill sources --workspace <ref>; GET /api/settings/skills?scope=workspace&workspace_id=<id> over HTTP or UDS; session composer `/` picker in that workspace
qa_status: pass
bug_ids: BUG-20260825-workspace-native-skill-missing
fix_status: fixed
retest_status: pass
fix_commits: df739b0
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/origin-native-summary.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/list-workspace.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/workspace-other.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-manage-skill-source-policy; ET-live-skill-source-reload; ET-skill-origin-attribution
---

Clone a repository that commits `.agents/skills/review-checklist/SKILL.md` and a `[skills]` block in
`<ws>/.compozy/config.toml`, as a second operator whose personal configuration is untouched
defaults. Open that workspace and confirm `review-checklist` is available in its sessions with no
local setup step, and that a different workspace does not see it — the skill is workspace-scoped, not
machine-scoped.

Walk the workspace-relative path rule from both sides. A workspace-relative custom source such as
`./tools/skills` is valid in the workspace config and loads; the same relative path submitted at user
scope must be refused with `invalid_source_path` explaining that workspace-relative paths require
workspace scope. Confirm the committed config's presence semantics hold per key: a key the repository
sets replaces the personal list for that workspace, a key it omits inherits, and an empty list means
that key's configured roots are off for this workspace even when the personal configuration enables
more.

Then prove the repository stays clean. After opening the workspace, running sessions there, and
reading every surface, the repository's tracked files must be byte-identical — no personal
configuration, no cache, no ownership record written into the working tree. Check `git status` in the
clone as the evidence, not just an inspection of the config file.

Finish on the branch case: check out a branch that deletes the committed skill directory and confirm
the skill leaves the catalog on the next refresh and stops being offered in sessions, without a
restart and without an error the operator has to dismiss. Confirm the reverse too — a global skill
with the same name is shadowed by the workspace copy while the branch carries it, and reappears as the
winner when the branch removes it.

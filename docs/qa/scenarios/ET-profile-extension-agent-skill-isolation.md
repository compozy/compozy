---
id: ET-profile-extension-agent-skill-isolation
area: ET
title: Keep profile extension Agents and local Skills in their winning layer
persona: Ada
journey: J-layer-profile-resources
expected: A Profile-only extension Agent appears only in its owning Profile, and Agent-local Skill reads select distinct global, default-Profile, non-default-Profile, and Workspace+Profile winners without leakage.
entry_points: compozy agent list; compozy skill list --for-agent; compozy skill where; compozy skill view; GET /api/agents over HTTP; GET /api/skills?for_agent= over HTTP; GET /api/skills/{name} over HTTP
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: 3bfa4da4;1e6a6315;7c53637c
evidence: qa-artifacts/qa/public-cli-api/agents-default-final-2.json;qa-artifacts/qa/public-cli-api/agents-finance-final-2.json;qa-artifacts/qa/public-cli-api/skill-global-view.json;qa-artifacts/qa/public-cli-api/skill-default-view.json;qa-artifacts/qa/public-cli-api/skill-finance-view.json;qa-artifacts/qa/public-cli-api/skill-workspace-profile-repeat-view.json;qa-artifacts/qa/qa-audit-report.json;qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-profile-extension-agent-skill-isolation.md
overlaps: ET-001;ET-skill-source-agent-parity
---

Start from an isolated home and workspace. Install and enable a local extension whose manifest binds
one Agent declaration only to a non-default `finance` Profile. Through the public CLI and HTTP API,
prove that the Agent is absent from the default Profile and present in `finance`. Repeat the reads so
the result is stable across daemon reconciliation rather than a one-shot startup artifact.

Plant the same Agent name and same Agent-local Skill name at four increasingly specific layers:
global, default Profile, non-default `finance` Profile, and Workspace+Profile. Give every Skill a
distinct description and body. Read the Agent-scoped Skill catalog and detail after each layer is
introduced, and assert the exact winning description, source path, and body instead of only checking
that one Skill exists. The expected order is global < default Profile < non-default Profile <
Workspace+Profile.

Keep every read public: CLI commands must use the built `compozy` binary and API evidence must come
from the isolated daemon's HTTP listener. Do not inspect SQLite or call Go internals during the walk.
Finish with the repository QA teardown command and accept evidence only when `qa/teardown.json`
records `clean: true` with no surviving process or socket.

QA 2026-09-01: build `7c53637c` passed the isolated CLI and HTTP walk. The Profile-only extension
Agent was absent from `default` and present exactly once in `finance` on both repeated reads. The
same-named Agent-local Skill selected distinct global, default Profile, finance Profile, and
Workspace+Profile descriptions, source paths, and bodies. Empty `for_agent` and an unknown Agent
were rejected without disturbing the successful reads. The strict evidence audit passed, and the
literal bootstrap teardown receipt recorded `clean: true` with no survivors or socket.

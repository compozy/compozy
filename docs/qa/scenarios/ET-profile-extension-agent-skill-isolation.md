---
id: ET-profile-extension-agent-skill-isolation
area: ET
title: Keep profile extension Agents and local Skills in their winning layer
persona: Ada
journey: J-layer-profile-resources
expected: A Profile-only extension Agent appears only in its owning Profile, and Agent-local Skill reads select distinct global, default-Profile, non-default-Profile, and Workspace+Profile winners without leakage.
entry_points: compozy agent list; compozy skill list --for-agent; compozy skill where; compozy skill view; GET /api/agents over HTTP; GET /api/skills?for_agent= over HTTP; GET /api/skills/{name} over HTTP
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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

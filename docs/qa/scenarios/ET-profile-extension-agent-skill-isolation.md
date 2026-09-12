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

## Web profile entry and definition operations

With the bundled `open-design` extension enabled, select its Profile in an isolated workspace.
Open Agents and verify both extension agents; open the designer detail and create a session from
the normal picker. The session must bind the selected Profile and load `open-design` and
`open-design-browser` through the native skill reader. Switching to `default` must show only its
own effective catalog, and returning to `open-design` must restore the extension entries.

Duplicate the extension designer into a disposable authored agent. The copied definition must be
editable in the selected Profile and must not appear in `default`. Edit and delete the authored
copy, checking the selected Profile's detail/cache each time. Update the original extension
AGENT.md with its current digest and verify the customization survives an unchanged-bundle restart.
Individual deletion must be rejected; managed SOUL.md and HEARTBEAT.md remain protected. A pending mutation
retains its original Profile for both the HTTP request and completion cache, including when it is
paused offline and the operator switches Profiles before reconnecting.

QA 2026-09-11 continuation: the current Web build passed catalog/detail/picker reads, default vs
open-design isolation, and native skill loading in `sess-2dd9be79c4d1d76e`. Evidence resides in
`/Users/pedronauck/dev/qa-labs/compozy-open-design-20260911-203142-042642-20260911-223928-759363-lab/qa-artifacts/qa`
(`profile-agents-final.png`, `profile-default-snapshot.txt`, `ui-session-history.json`). Definition
mutation re-walk passed: UI duplicate/edit/delete and cross-profile isolation, plus 19 public HTTP
steps independently verifying the authored copy. The original update-denial expectation was later
identified by CI as a regression against supported extension customization and is superseded by
the round-one re-walk below. Receipts:
`ui-copy-edited.txt`, `ui-copy-absent-default.txt`, `ui-copy-deleted.txt`,
`managed-agent-api-qa-summary.json`. The existing mutation suite covers in-flight and offline
profile switches. Final gate, strict evidence audit and teardown passed; `teardown.json` records
`clean: true`. This supplements the unchanged CLI/API precedence evidence from 2026-09-01.

QA 2026-09-11 round one: existing daemon integration tests exposed the incorrect blanket
AGENT.md update prohibition. Production now preserves package-agent customization with digest CAS;
the unchanged customization/restart, published-session, review-and-fix, and implement-tasks
journeys pass. The isolated public re-walk accepted customization, rejected stale CAS and individual
deletion, and retained the exact definition digest after restart. Evidence:
`/Users/pedronauck/dev/qa-labs/compozy-open-design-r1-20260911-234610-059353-lab/qa-artifacts/qa/bundled-agent-after-restart.json`.

Round-one final strict audit passed without blockers or warnings; the local gate passed all
affected lanes (7,277 Web tests). Lab teardown completed with `clean: true` and no surviving
owned process.

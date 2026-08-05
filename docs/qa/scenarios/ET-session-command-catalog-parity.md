---
id: ET-session-command-catalog-parity
area: ET
title: Read one effective session command catalog across public surfaces
persona: Bruno
journey: J-use-session-slash-commands
expected: Web, CLI JSON, and HTTP expose the same session revision, path-free command ids, tokens, lanes, sources, scopes, and availability, while a wrong workspace cannot read the catalog.
entry_points: web session composer; compozy session commands <session-id> -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/cli-command-catalog.json;/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/http-command-catalog.json;/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/http-wrong-workspace.json;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/cli-command-catalog.json;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/http-command-catalog.json;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/screenshots/07-effective-skills-narrow.png
last_report: docs/qa/reports/2026-08-05-session-slash-commands.md
overlaps: ET-native-workspace-scope-isolation
---

QA impact 2026-08-05: the session-effective command catalog is a new public HTTP, UDS-backed CLI, Web, and native-tool surface. This targeted non-agent scenario settles Web/CLI/HTTP parity and workspace fencing; native-tool exact-source behavior remains covered by the real registry integration suite.

QA verdict 2026-08-05: passed in the isolated lab. CLI JSON and HTTP returned revision `6e956b4162ebdd3e9be40a00248a77dddf4f7293dc4be27d44da2a901bbfc572` with the same `/goal`, workspace `/browser-qa`, and bundled `/compozy` commands; the agent-disabled `/hidden-skill` was absent. The same session id under `ws_wrong` returned 404 with no catalog.

QA verdict 2026-08-05 (reviewed head): passed again with revision `4932bd30db7b9a2d60442ec259142c0245197fe97604fdbd658f7bb8d37c4c0d`. CLI, HTTP, and the 320 px Web menu agreed on `/goal`, `/browser-qa`, and `/compozy`; `/hidden-skill` remained unavailable for `slash-operator`.

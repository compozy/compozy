---
id: ET-session-command-catalog-parity
area: ET
title: Read one effective session command catalog across public surfaces
persona: Bruno
journey: J-use-session-slash-commands
expected: Web, CLI JSON, and HTTP expose the same session revision, path-free command ids, tokens, lanes, sources, scopes, and availability, while a wrong workspace cannot read the catalog.
entry_points: web session composer; compozy session commands <session-id> -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/command-catalog-parity.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/web-proxy-parity.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/workspace-fence-http.status;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/workspace-fence-uds.status
last_report: docs/qa/reports/2026-08-13-extension-agent-session-skills.md
overlaps: ET-native-workspace-scope-isolation
---

QA impact 2026-08-05: the session-effective command catalog is a new public HTTP, UDS-backed CLI, Web, and native-tool surface. This targeted non-agent scenario settles Web/CLI/HTTP parity and workspace fencing; native-tool exact-source behavior remains covered by the real registry integration suite.

QA verdict 2026-08-05: passed in the isolated lab. CLI JSON and HTTP returned revision `6e956b4162ebdd3e9be40a00248a77dddf4f7293dc4be27d44da2a901bbfc572` with the same `/goal`, workspace `/browser-qa`, and bundled `/compozy` commands; the agent-disabled `/hidden-skill` was absent. The same session id under `ws_wrong` returned 404 with no catalog.

QA verdict 2026-08-05 (reviewed head): passed again with revision `4932bd30db7b9a2d60442ec259142c0245197fe97604fdbd658f7bb8d37c4c0d`. CLI, HTTP, and the 320 px Web menu agreed on `/goal`, `/browser-qa`, and `/compozy`; `/hidden-skill` remained unavailable for `slash-operator`.

QA impact 2026-08-05 (composer redesign): reset — menu presentation changed to a single categorized list with humanized built-in/agent titles, kebab skill identities, canonical token trailing text, and skill scope labels. Re-verify catalog parity against `compozy session commands` output.

QA impact 2026-08-12 (GitHub #349 / PR #350): reset remains in force. Session command and prompt resolution now falls back to the workspace-fenced daemon catalog for extension-published agents only after the authored-agent lookup reports the agent absent. Re-walk Web, CLI, HTTP/UDS, prompt, native command/skill reads, refresh, and wrong-workspace fencing with the bundled `dev-cycle` reviewer.

QA verdict 2026-08-12: passed in the fresh isolated lab. The unbound `reviewer` session exposed 11 commands and revision `9fce06ed445f6e2e51a6110ca3dcc54c4e8e22ef7f9e8b031e8344b64cd74512` through CLI, HTTP, and direct UDS, including all nine `dev-cycle` extension skills. The live Codex agent used `compozy__command_list` and source-qualified `compozy__skill_view`, slash expansion persisted the extension source in the user event, the Web composer retained the catalog after refresh, and a foreign workspace request returned 404.

QA impact 2026-08-13: reset because prompt, native skill tools, and command catalogs now share extension-agent session skill resolution. Re-walk the extension-agent catalog and workspace fence.

QA verdict 2026-08-13: blocked for human browser verification. CLI, HTTP, direct UDS, and the isolated Vite Web proxy returned the same 11-command payload at revision `9fce06ed445f6e2e51a6110ca3dcc54c4e8e22ef7f9e8b031e8344b64cd74512`, including all nine `dev-cycle` skills; HTTP and UDS foreign-workspace reads both returned 404. The bootstrap reported neither `browser-use` nor `agent-browser`, so the rendered Web command menu was not observed and this scenario is not marked pass.

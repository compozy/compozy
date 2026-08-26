---
id: ET-session-command-catalog-parity
area: ET
title: Read one effective session command catalog across public surfaces
persona: Bruno
journey: J-use-session-slash-commands
expected: Web, CLI JSON, and HTTP expose the same session revision, path-free command ids, tokens, lanes, sources, scopes, and availability, while a wrong workspace cannot read the catalog.
entry_points: web session composer; compozy session commands <session-id> -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands
qa_status: pass
bug_ids: BUG-20260826-namespaced-skill-label-collapses
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/commands-summary.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/commands-cli.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/commands-http.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/commands-uds.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-native-workspace-scope-isolation
---

QA impact 2026-08-05: the session-effective command catalog is a new public HTTP, UDS-backed CLI, Web, and native-tool surface. This targeted non-agent scenario settles Web/CLI/HTTP parity and workspace fencing; native-tool exact-source behavior remains covered by the real registry integration suite.

QA verdict 2026-08-05: passed in the isolated lab. CLI JSON and HTTP returned revision `6e956b4162ebdd3e9be40a00248a77dddf4f7293dc4be27d44da2a901bbfc572` with the same `/goal`, workspace `/browser-qa`, and bundled `/compozy` commands; the agent-disabled `/hidden-skill` was absent. The same session id under `ws_wrong` returned 404 with no catalog.

QA verdict 2026-08-05 (reviewed head): passed again with revision `4932bd30db7b9a2d60442ec259142c0245197fe97604fdbd658f7bb8d37c4c0d`. CLI, HTTP, and the 320 px Web menu agreed on `/goal`, `/browser-qa`, and `/compozy`; `/hidden-skill` remained unavailable for `slash-operator`.

QA impact 2026-08-05 (composer redesign): reset — menu presentation changed to a single categorized list with humanized built-in/agent titles, kebab skill identities, canonical token trailing text, and skill scope labels. Re-verify catalog parity against `compozy session commands` output.

QA impact 2026-08-12 (GitHub #349 / PR #350): reset remains in force. Session command and prompt resolution now falls back to the workspace-fenced daemon catalog for extension-published agents only after the authored-agent lookup reports the agent absent. Re-walk Web, CLI, HTTP/UDS, prompt, native command/skill reads, refresh, and wrong-workspace fencing with the bundled `spec-cycle` reviewer.

QA verdict 2026-08-12: passed in the fresh isolated lab. The unbound `reviewer` session exposed 11 commands and revision `9fce06ed445f6e2e51a6110ca3dcc54c4e8e22ef7f9e8b031e8344b64cd74512` through CLI, HTTP, and direct UDS, including all nine `spec-cycle` extension skills. The live Codex agent used `compozy__command_list` and source-qualified `compozy__skill_view`, slash expansion persisted the extension source in the user event, the Web composer retained the catalog after refresh, and a foreign workspace request returned 404.

QA impact 2026-08-13: reset because prompt, native skill tools, and command catalogs now share extension-agent session skill resolution. Re-walk the extension-agent catalog and workspace fence.

QA evidence correction 2026-08-13: the preceding pass/blocked claims use a build that predates PR #372 and are historical only.

QA verdict 2026-08-13 (fresh native-CLI lab): blocked-verify. CLI, HTTP, and direct UDS returned an identical canonicalized ten-skill catalog (`compozy` plus the nine `spec-cycle` skills); foreign-workspace HTTP and UDS reads both returned 404 with no command payload. The built Web root rendered through `agent-browser`, but the concrete reviewer-session route returned “Route not found”, so menu parity was not observable. This route limitation is outside the exercised daemon command-resolution contract, but it prevents a Web parity pass.

QA impact 2026-08-25 (skill sources): reset because the session command catalog contract changed again. The projection is now built from pre-overlay candidates carrying a stable opaque root identity, physical homonyms across two roots each keep a deterministic qualified token, skill rows carry an origin label in the trailing slot, and the catalog is keyed by the session's immutable profile id rather than the remembered profile. Re-walk Web, CLI, and HTTP/UDS parity on one revision, confirm both homonyms are listed and distinguishable, confirm switching the remembered profile does not change an existing session's catalog, and re-walk the wrong-workspace fence. Charter: `CH-skill-session-suppression-matrix`.

QA execution 2026-08-26: CLI, HTTP, and UDS retained the distinct canonical collision tokens. The
production Web picker initially collapsed both visible labels to the authored display name; after
`BUG-20260826-namespaced-skill-label-collapses`, E2E-010 verified one `commit-hygiene` row and one
`agents:commit-hygiene` row with the correct `agents` origin.

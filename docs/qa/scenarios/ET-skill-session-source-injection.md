---
id: ET-skill-session-source-injection
area: ET
title: Keep native skill injection provider-aware and explicitly invocable
persona: Dora
journey: J-use-absorbed-skills-in-a-session
expected: A session omits only skills its provider already reads natively from the winning root, while the command picker, qualified invocation, exact profile ownership, verification, and drift rejection remain available and deterministic
entry_points: session startup prompt; session prompt input; session composer `/` picker; compozy session commands <session-id> -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands over HTTP or UDS; slash command submission; harness diagnostics
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-command-catalog-parity; ET-skill-origin-attribution; ET-session-composer-skill-chip; ET-managed-session-skill-loading
---

Start sessions under Claude, OpenClaw, Hermes, their canonical aliases, and an unknown provider. Verify startup and turn catalogs suppress only winning preset roots the active provider reads, including native-home overrides and isolated homes, while commands remain listed with origin labels and every suppression has structured diagnostics. Invoke a suppressed skill explicitly, invoke both physical homonyms through their stable qualified forms, switch the remembered profile and confirm the existing session keeps its original profile catalog, then disable a selected source and verify deterministic drift rejection without partial injection or transcript mutation.

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-use-absorbed-skills-in-a-session`. The session-commands entry point is corrected to the real workspace-fenced route and its CLI equivalent, so picker rows and the structured catalog are compared against the same session revision in one walk. Provider-home policy is part of this scenario, not an environment detail: the isolated- and relocated-home lanes must both be walked, because a relocated provider home means the operator-home folders stop being that session's native roots and its skills must be injected normally. Charter: `CH-skill-session-suppression-matrix`.

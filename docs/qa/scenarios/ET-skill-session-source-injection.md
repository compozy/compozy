---
id: ET-skill-session-source-injection
area: ET
title: Keep native skill injection provider-aware and explicitly invocable
persona: Dora
journey: J-layer-profile-resources
expected: A session omits only skills its provider already reads natively from the winning root, while the command picker, qualified invocation, exact profile ownership, verification, and drift rejection remain available and deterministic
entry_points: session startup prompt; session prompt input; GET /api/sessions/{id}/commands; slash command submission; harness diagnostics
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-session-command-catalog-parity, ET-skill-origin-attribution
---

Start sessions under Claude, OpenClaw, Hermes, their canonical aliases, and an unknown provider. Verify startup and turn catalogs suppress only winning preset roots the active provider reads, including native-home overrides and isolated homes, while commands remain listed with origin labels and every suppression has structured diagnostics. Invoke a suppressed skill explicitly, invoke both physical homonyms through their stable qualified forms, switch the remembered profile and confirm the existing session keeps its original profile catalog, then disable a selected source and verify deterministic drift rejection without partial injection or transcript mutation.

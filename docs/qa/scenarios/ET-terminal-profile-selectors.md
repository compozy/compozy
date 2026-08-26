---
id: ET-terminal-profile-selectors
area: ET
title: Refuse ambiguous or unscoped terminal requests instead of guessing
persona: Ada
journey: J-switch-profile-terminal-scope
expected: Every structured terminal surface resolves exactly one profile scope or an explicitly labelled all-profiles read; asking for both, asking for the aggregate on a change, contradicting a session's own profile, or calling with no project at all is each refused with its own reason, and an unresolvable scope returns nothing rather than everything.
entry_points: compozy terminal list|journal|input-requests --profile and --all-profiles; compozy terminal get|attach|quote --profile; compozy terminal open|exec|kill|signal|respond|record; HTTP and UDS terminal routes with profile= and all_profiles=; terminal catalog stream; native terminal tools in a session with no project
qa_status: pass
bug_ids: BUG-20260826-terminal-attach-profile-scope
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps: ET-terminal-profile-segmentation; ET-profile-selection-precedence
---

Planned by integrated-terminal task 09 for the structured selector contract.
`ET-terminal-profile-segmentation` owns the operator-visible segmentation walk; this file owns the
selector grammar and its refusals across the command line, both transports, the stream, and the agent
tools. Task 10 owns the walk, evidence, and verdict.

Walk:

1. Call each aggregate read with no selector, with an explicit profile, and with the all-profiles form;
   confirm the default is the resolved profile and the aggregate labels every row's owner.
2. Call the single-owner reads and confirm they accept one profile and offer no aggregate form.
3. Call every change verb with the aggregate form and confirm each is refused.
4. Send both selectors together on a read and confirm the refusal names the two selectors and which one
   that surface accepts.
5. From inside a session, pass a profile that disagrees with the session's own and confirm the session
   wins as a veto with its own reason, rather than the flag silently overriding it.
6. From a session with no project, try to open a terminal and to run a command; confirm both are
   refused with the no-project reason and that no terminal appears anywhere afterwards.
7. Pass an unknown profile, an archived one, an empty one, and one differing only in case; confirm each
   answer is a typed refusal or an empty result, never an unfiltered list.
8. Open a terminal under an archived profile and under a profile held by a lifecycle operation; confirm
   each refusal names the profile and its condition while reads of existing terminals keep working.
9. Compare the same query across the command line, both transports, the catalog stream, and a native
   read from inside a session; confirm all five agree on scope and on owner labels.
10. Fill one profile's per-project terminal budget and confirm another profile in the same project can
    still open terminals.

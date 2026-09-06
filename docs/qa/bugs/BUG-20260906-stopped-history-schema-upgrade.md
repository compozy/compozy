# BUG-20260906-stopped-history-schema-upgrade: Upgraded daemon cannot read retained stopped histories

- **Status:** fixed — verified in isolated daemon restart
- **Impact:** Data-Unavailable
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Journey:** J-14 read a finished transcript
- **Scenarios:** RT-session-context-rebuild, ET-web-session-transcript-calm-grammar
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

After upgrading the isolated lab from session migration 7 to 8 and restarting the daemon, both Release notes and Operator handbook returned HTTP 500 for retained transcript reads. The daemon log reports ErrSchemaBehind: read-only session databases remained at version 7. Completed sessions bypassed the mutable crash-repair path, leaving their valid forward migrations unapplied.

The repair asks the Manager to prepare retained session databases during the existing boot inventory phase, before read-only interaction recovery and public routes. A current database opens read-only; only ErrSchemaBehind invokes the existing owned writable opener/Goose stream. Ahead, corrupt, unreadable and foreign databases retain their refusal. SQL transformation belongs exclusively to the appended migration. Events, IDs, archives and sequence/generation fences remain unchanged.

Canonical checks: TestBootSessionRepair and TestManagerOpenQueryRecorderValidationAndCleanup. The separate SessionDB projection suite owns migration data preservation and repeated reopen; no duplicate transformation assertions in boot tests.

Real restart verified: the same stopped handbook opens through public HTTP and Web after daemon PID97015 starts. The public transcript retains the same authored guide ID/text and contains 32 tool parts with corrected final labels. Evidence: integrated lab evidence/handbook-transcript-migrated.json, evidence/handbook-reopened.png, and daemon.log. Original schema-behind failures remain at 08:03–08:04 America/Sao_Paulo.

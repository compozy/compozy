# BUG-20260912-context-turn-identifiers: Opaque turn IDs overflow the Context row

- **Status:** fixed and visually verified (enclosing delivery commit records the revision)
- **Impact (user-side):** Confusion
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Rafa
- **Journey Step:** J-14, inspect individual turn receipts
- **Scenarios:** ET-web-session-context-sidebar
- **Found:** 2026-09-12 · **Report:** ../reports/2026-09-12-session-context.md

## Reproduction

1. Complete a real session turn, producing an ID such as `turn-ea76af9f25086e7d`.
2. Open the Context sidebar and inspect Turns.
3. Compare with the numeric illustrative IDs used by the existing stories.

**Expected:** The full ID remains accessible while the visual row keeps its bounded columns.
**Actual:** The fixed ID column allowed the opaque ID to wrap; the subsequent numeric nowrap repair would allow it to overlap the receipt details.

## Evidence

`.cache/session-context/unknown-stopped-live.png` shows the real stopped session. The original numeric-only visual fixtures did not expose this data shape.

## Fix and verification

The ID column uses the existing 56px spacing token and truncates long identifiers while preserving the full text node and exact raw ID in its native title. Three-digit numeric identifiers remain complete. The existing inspector stories now include the actual opaque ID. Four affected VC-08 pairs were re-captured and inspected; the complete visual validator passes 44/44. Root Turbo typecheck passed in 4.133s. Fresh manifest teardown confirms clean=true. Evidence: `.compozy/tasks/session-context/evidence/visual/task_03/VC-08/states/opaque-turn-ids/` and `.cache/session-context/qa-opaque-turn-typecheck.log`.

## TC-INT-003: Run Stream Overflow and Reconnect Semantics

**Priority:** P1 (High)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 12 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `bunx vitest run --config web/vitest.config.ts web/src/routes/-runs.integration.test.tsx web/src/systems/runs/hooks/use-run-stream.test.tsx web/src/systems/runs/lib/stream.test.ts`
**Automation Notes:** The route and hook-level suites currently prove stream open, overflow banner handling, snapshot invalidation, reconnect scheduling, and manual reconnect behavior without requiring a flaky browser-only reproduction.

### Objective

Verify that run detail remains trustworthy when the SSE stream overflows, reconnects, or requires a fresh snapshot, matching the explicit streaming contract from the TechSpec.

### Preconditions

- [ ] The run detail route can load a snapshot.
- [ ] The stream factory override remains available for deterministic integration testing.
- [ ] The typed stream decoder still supports heartbeat/overflow semantics.

### Test Steps

1. Run the focused stream suite listed above.
   **Expected:** The vitest run exits `0` and the reconnect/overflow specs pass.

2. Confirm the route integration tests inject an overflow event.
   **Expected:** The UI renders an overflow notice and invalidates/refetches the snapshot.

3. Confirm the manual reconnect action closes the previous stream and opens a new one.
   **Expected:** The route test observes a new controller/session after clicking reconnect.

4. Confirm the lower-level stream tests still parse and normalize stream events correctly.
   **Expected:** Heartbeats, overflow frames, and event payloads are decoded consistently.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Overflow | `run.overflow` event | Overflow banner renders and snapshot refetch happens |
| Manual reconnect | Operator clicks reconnect | Previous stream closes and a new one opens |
| Heartbeat-only window | Heartbeats without new events | Stream remains healthy without false failure |
| Decoder branch | Legacy/edge stream frame | Parsing remains stable |

### Related Test Cases

- `TC-FUNC-007`
- `TC-INT-004`

### Traceability

- TechSpec: "Streaming Contract"
- Task reference: `task_10`
- Shared memory note: canonical run SSE contract is `run.snapshot`, `run.event`, `run.heartbeat`, and `run.overflow`

### Notes

- Keep this case separate from `TC-FUNC-007`. A visible stream banner is not sufficient proof that overflow/reconnect semantics still work.

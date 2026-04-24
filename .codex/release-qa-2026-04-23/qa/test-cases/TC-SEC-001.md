## TC-SEC-001: Browser API Security Headers And Localhost Boundary

**Priority:** P1 (High)
**Type:** Security
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-23
**Last Updated:** 2026-04-23
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:** `go test ./internal/api/httpapi -run 'Test.*(Security|Browser|Transport|Static)' -count=1`
**Automation Notes:** Existing HTTP API tests cover browser middleware/security/static behavior; live curl/browser smoke should confirm headers on a running daemon.

### Objective

Verify daemon HTTP/browser surfaces enforce expected local-browser security behavior and do not expose unsafe cross-origin defaults.

### Preconditions

- Daemon HTTP listener is running.

### Test Steps

1. Run focused HTTP API tests.
   **Expected:** Security, browser middleware, transport, and static tests pass.
2. Request the Web UI root with `curl -i`.
   **Expected:** Response includes expected security headers and successful status.
3. Request an API endpoint from local origin.
   **Expected:** API responds only under supported local/browser context.

### Edge Cases

| Variation         | Input                   | Expected Result                                                |
| ----------------- | ----------------------- | -------------------------------------------------------------- |
| Non-local origin  | Foreign `Origin` header | Request is rejected or guarded per browser middleware contract |
| Static asset path | Existing Web UI asset   | Served with safe headers                                       |

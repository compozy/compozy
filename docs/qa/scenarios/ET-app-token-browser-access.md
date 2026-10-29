---
id: ET-app-token-browser-access
area: ET
title: Grant and withdraw access from a real browser origin
persona: Dora
journey: J-authorize-browser-app
expected: Only the token own allowed origin can call app-access operations, privileged and Gateway requests are denied, and revocation closes open streams within five seconds.
entry_points: /docs/apps/browser; browser fetch and SSE; local HTTP; Gateway listener
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-app-token-management; RT-sdk-stream-recovery
---

Use a real app origin and an active token for it. Observe browser preflight 204 and authorized sessions fetch and transcript stream. Try another origin, a token for a different origin, missing bearer, revoked bearer, and an opaque null origin. Request app-token management and privileged Terminal/filesystem/settings routes; confirm 403 route-forbidden without handler effects. Replace origins and verify new requests from the old origin fail. Revoke during streaming, time closure, and confirm the next request returns app_token_revoked. Try the same secret on Gateway and verify no last-use change. Recheck same-origin Web and tokenless local callers.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.

QA impact 2026-10-18: an allowed browser origin can read `/api/status/identity`, while
operator diagnostics at `/api/status` return 403 with app_token_route_forbidden. Exercise
per-origin rate isolation through the identity route; preserve all invalid-token, origin,
revocation, and tokenless local-process checks.

2026-10-29 transport regression: the denied-settings-mutation check validates
uptime drift with the existing sampled-uptime helper, then compares the complete
settings envelope with only that volatile counter normalized. A controlled
reproduction differed solely in `runtime.uptime_seconds` (17 to 18); no settings
changed. Forbidden response codes and terminal non-creation remain asserted.

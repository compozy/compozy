---
id: RT-cloud-gate-grace
area: RT
title: Keep local work running during a sign-in incident
persona: Iris
journey: J-sign-in-and-use-compozy
expected: Verified, time-limited grace permits local work and expires without interrupting running turns.
entry_points: Staging admin console; official desktop and CLI; local automation scheduler
qa_status: pending
---

1. In staging, set a signed grace policy with a deadline. Start with a fresh signed-out home. Check the app's banner, the CLI's stderr deadline, and a recurring job's successful run. Verify Computers still returns `sign_in_required`.
2. Switch the policy to enforcement. Observe the next refresh within 15 minutes and verify the operator stream closes with `4401`, the next recurring occurrence is skipped, and an already-running turn finishes.
3. Upgrade a 0.7.x home while blocking access to the cloud. Check the seven-day deadline and restart midway through the window to confirm the deadline persists. Verify the gate enforces after the deadline. Repeat with a fresh home and confirm it starts enforced.
4. Connect a test instance with a policy signed by an unknown key. Verify the daemon enforces sign-in.
5. While a signed grace policy is active, inject a rate-limited policy response and then a JWKS outage. Verify the banner keeps its original deadline and the operator stream stays open. On an existing home without a cached policy, follow the transient responses with a Cloud outage and verify offline grace is available.

Use UT-134 and IT-112 for the related automated evidence.

---
id: RT-sign-in-first-run
area: RT
title: Sign in to CompozyOS for the first time
journey: J-sign-in-first-run
expected: A first sign-in binds the operator-confirmed grant to one installation and its key. The installation can refresh and sign out with explicit rejection reasons.
entry_points: CompozyOS sign-in; Compozy Cloud activation; hosted browser sign-in
qa_status: planned
---

Run this scenario on staging after the cloud and daemon sign-in slices are integrated.

1. Start sign-in with Google and then with GitHub on fresh installations. Check that the cloud displays the correct computer, platform, location, and code. Confirm that both providers link to the same account when the verified email matches.
2. Cancel before continuing and repeat after the provider callback. Check that the old grant cannot enroll a device. Wait until expiry on a separate attempt and check that the operator can start again.
3. Open the same code in two browser contexts. Confirm that a mismatched callback cookie is refused and that only one browser completes sign-in. Submit five wrong codes from one browser and verify that the next request returns `429` with `Retry-After`.
4. Block Turnstile, then unblock it and continue. Check that the failed attempt stays on the Compozy Cloud page without creating an OAuth request or a pending browser key. In staging, also verify the Cloudflare zone limits for device-flow starts, activation, callbacks, and browser authorization.
5. Sign out and sign in again with the same account on the same installation. Check that sign-out revokes the session and the next sign-in reuses the device. Revoke the last admin device from an authorized staging surface and recover with a fresh Google sign-in. An ordinary sign-in must leave the device without admin access.
6. Open an owned hosted route from two origins in the same browser, then repeat with the browser binding cookie cleared. Check that a registered key stays with its original device and that the hosted route can start again with a new key.

The S1b HTTP and Postgres integration suite covers the cloud portion with fake provider and Turnstile endpoints. Keep the staging walk pending until the real OAuth apps, Turnstile widget, and hosted listener are available.

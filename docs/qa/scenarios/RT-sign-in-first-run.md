---
id: RT-sign-in-first-run
area: RT
title: Sign in to CompozyOS for the first time
journey: J-sign-in-first-run
expected: A first sign-in binds the operator-confirmed grant to one installation and its key. The installation can refresh and sign out with explicit rejection reasons.
entry_points: CompozyOS sign-in; compozy account; local HTTP and UDS; Compozy Cloud activation; hosted browser sign-in
qa_status: planned
---

Run this scenario on staging after the cloud and daemon sign-in slices are integrated.

1. Start sign-in with Google and then with GitHub on fresh installations. Check that the cloud displays the correct computer, platform, location, and code. Confirm that both providers link to the same account when the verified email matches.
2. Cancel before continuing and repeat after the provider callback. Check that the old grant cannot enroll a device. Wait until expiry on a separate attempt and check that the operator can start again.
3. Open the same code in two browser contexts. Confirm that a mismatched callback cookie is refused and that only one browser completes sign-in. Submit five wrong codes from one browser and verify that the next request returns `429` with `Retry-After`.
4. Block Turnstile, then unblock it and continue. Check that the failed attempt stays on the Compozy Cloud page without creating an OAuth request or a pending browser key. In staging, also verify the Cloudflare zone limits for device-flow starts, activation, callbacks, and browser authorization.
5. Sign out and sign in again with the same account on the same installation. Check that sign-out revokes the session and the next sign-in reuses the device. Revoke the last admin device from an authorized staging surface and recover with a fresh Google sign-in. An ordinary sign-in must leave the device without admin access.
6. Sign out with an account stream open and check that it closes. Sign in to a different account and resume with the prior account's event ID. Confirm the daemon returns `stream_reset` and only the new account's approvals are visible.
7. Stop the staging cloud during a signed-in run. Confirm an account action reports `account_cloud_unreachable`, the installation still reports signed in, and a local session can be started.
8. Open an owned hosted route from two origins in the same browser, then repeat with the browser binding cookie cleared. Check that a registered key stays with its original device and that the hosted route can start again with a new key.
9. Require device approval on the hosted computer. Sign in from a fresh browser and leave it waiting for six minutes. From an approved account admin installation, approve the browser key. Check that the browser signs in again and opens the sessions list. Repeat in a new browser and deny its request. Check that the browser shows the denial.

10. Leave product news unchecked during one enrollment and opt in during a separate fresh enrollment. Confirm the second account appears in the staging news segment. Open its unsubscribe link, check that opening it leaves the setting enabled, then confirm and verify the contact is removed.
11. Enroll a second device. Check the sign-in email and both installations' notices. Dismiss on one installation and check the other still shows the notice. Reuse the device key and confirm there is only one email. Open the email link with an ordinary web session, complete a fresh Google sign-in, and revoke the device.
12. Delete a fresh Free account from a trusted admin device. Check that the caller signs out immediately and local work stays on disk. Verify another installation is signed out on its next credential refresh, the host is unlinked, and the Resend contact is removed.
13. In the staged admin scenario, suspend an account with a linked computer and an open terminal stream. Check that the stream closes and sign-in reports the suspension reason. Lift the suspension and confirm the linked computer recovers on its next revalidation while the device signs in again.

The S1b HTTP and Postgres integration suite covers the cloud portion with fake provider and Turnstile endpoints. Keep the staging walk pending until the real OAuth apps, Turnstile widget, and hosted listener are available.

The daemon and Postgres integration suite covers enrollment, sign-out, cloud unavailability, and account stream isolation (IT-010, IT-011, IT-012, IT-099, IT-103).

The hosted browser backend integration covers key activation, account ownership, peer discovery, and approval retry (UT-114, IT-072, IT-073, IT-077, IT-080). The hosted sign-in screens are exercised by E2E-006 after UI integration.

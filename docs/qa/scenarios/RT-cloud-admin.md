---
id: RT-cloud-admin
area: RT
title: Operate the Compozy Cloud admin console
journey: J-cloud-admin
expected: Only the operator can access the console. Account actions use the same services as the cloud API and leave an attributable security event.
entry_points: Cloudflare Access; Compozy Cloud admin; Settings Account; hosted browser sign-in
qa_status: planned
---

Run on staging after the Remote Computers slices are integrated. Use synthetic accounts, a staging hosted route, and a staging Resend segment.

1. Open `/admin` in a private browser window. Sign in through Cloudflare Access as the allowed Google account. In a separate browser profile, verify that a different identity is refused.
2. Search by email, account ID, Stripe customer, and hosted hostname. Open the account and compare its installations, devices, sessions, computers, tunnels, subscriptions, and consent with the staging records. Compare the overview with the cloud metrics.
3. Open a terminal stream from the installation and a session from a phone on the hosted route. Choose **Suspend** and check the account email in the confirmation. Verify that the streams close, the tunnels stop, and the next credential refresh reports suspension. Open the audit receipt and check the actor. Choose **Unsuspend** and verify that a linked host recovers after revalidation and the installation can sign in again.
4. Grant complimentary Pro with a future end date. Verify that the account and the host's next lease show Pro. Raise its hosted-route cap to 12 and verify that an 11th hosted tunnel can be provisioned for that account.
5. Find a disposable hosted route by hostname. Choose **Delete tunnel** and confirm. Verify the hostname stops answering within a minute and trying to re-enable that hosted route returns `hosted_route_taken_down`.
6. Revoke a device session and verify that its next cloud request is refused. Revoke a different device and verify that its active remote streams close.
7. Export JSON and check that it contains only the chosen account and does not disclose credential hashes, PKCE secrets, keys, or access tokens. Export contacts CSV and check that it contains only opted-in accounts with consent time and source.
8. Open **Detach identity**. Confirm the written request is required. Use a synthetic request to detach a linked identity and verify that the receipt records it. Start deletion of a disposable paid account and verify that Stripe confirms cancellation before cloud records are removed.
9. Set the gate to **Grace** with a future UTC deadline and a test message. Verify that a signed-out installation sees that message. Restore **Enforce**, verify the signed public policy, and check the security events for both changes.
10. Opt in a synthetic contact, send staging product news using the cloud operator command, and inspect the delivered message. Verify the configured address in the footer and the cloud URL in `List-Unsubscribe`. Use the message's one-click action and check that Settings shows opt-out and that the contact leaves the staging Resend segment.

The Postgres and HTTP harness exercises the admin routes (IT-108) and fake Resend sync and delivery (UT-142). Staging evidence is collected after integration.
